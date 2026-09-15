package luabridge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// The helper binary is compiled from the vendored, pinned Lua 5.4.7
// sources plus native/lua-helper/lua_helper.c. Skipping is honest:
// without a C toolchain this evaluation cannot run here.
var (
	helperOnce   sync.Once
	cachedHelper string
	cachedErr    error
)

// buildHelper compiles the helper once per test binary and reuses the
// cached binary for every test: vendored Lua compilation costs seconds
// and would otherwise be paid by each test function.
func buildHelper(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("helper build test runs on darwin/linux only")
	}
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	helperOnce.Do(func() {
		cachedHelper, cachedErr = compileHelper()
	})
	if cachedErr != nil {
		t.Fatalf("building helper failed: %v", cachedErr)
	}
	return cachedHelper
}

func compileHelper() (string, error) {
	dir, err := os.MkdirTemp("", "brouter-lua-helper-")
	if err != nil {
		return "", err
	}
	helper := filepath.Join(dir, "brouter-lua-helper")
	src, luasrc, sources, srcErr := helperInputs(dir)
	if srcErr != nil {
		return "", srcErr
	}
	args := append([]string{"-O2", "-I", luasrc, src}, sources...)
	args = append(args, "-lm", "-o", helper)
	cmd := exec.Command("clang", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("%v: %s", err, out)
	}
	return helper, nil
}

// helperInputs anchors the helper source at this file (repository
// root is four levels up) and points luasrc at the pinned Lua source
// cache, bootstrapped on demand by scripts/fetch-lua.sh (sha256-
// verified). Capability libraries are excluded from the link.
func helperInputs(dir string) (src, luasrc string, sources []string, err error) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
	src = filepath.Join(root, "native", "lua-helper", "lua_helper.c")
	if _, serr := os.Stat(src); serr != nil {
		return "", "", nil, fmt.Errorf("helper source missing at %s: %w", src, serr)
	}
	bootstrap := exec.Command(filepath.Join(root, "scripts", "fetch-lua.sh"))
	bootstrap.Stderr = os.Stderr
	out, berr := bootstrap.Output()
	if berr != nil {
		return "", "", nil, fmt.Errorf("lua source bootstrap failed: %w", berr)
	}
	luasrc = strings.TrimSpace(string(out))
	entries, rerr := os.ReadDir(luasrc)
	if rerr != nil {
		return "", "", nil, fmt.Errorf("reading pinned lua dir: %w", rerr)
	}
	exclude := map[string]bool{
		"lua.c": true, "luac.c": true, "onelua.c": true, "linit.c": true,
		"loadlib.c": true, "liolib.c": true, "loslib.c": true,
		"ldblib.c": true, "lmathlib.c": true, "lutf8lib.c": true,
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".c") && !exclude[e.Name()] {
			sources = append(sources, filepath.Join(luasrc, e.Name()))
		}
	}
	if len(sources) == 0 {
		return "", "", nil, fmt.Errorf("vendored lua dir has no linkable .c files")
	}
	return src, luasrc, sources, nil
}

func evalWith(t *testing.T, helper, script string) Result {
	t.Helper()
	p := NewProvider(helper, writeScript(t, script))
	p.Clock = func() time.Time { return time.Unix(1700000000, 0) }
	res, err := p.Decide(context.Background(), "https://example.com/x?a=1#f")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	return res
}

func writeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "route.lua")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func routeWrapper(body string) string { return "function route(ctx) " + body + " end" }

func TestHelperSandboxCapabilityDenial(t *testing.T) {
	helper := buildHelper(t)
	for _, probe := range []string{
		`return tostring(os)`,
		`return tostring(io)`,
		`return tostring(require)`,
		`return tostring(load)`,
		`return tostring(debug)`,
		`return tostring(print)`,
		`return tostring(warn)`,
		`return tostring(rawset)`,
		`return tostring(setmetatable)`,
	} {
		res := evalWith(t, helper, routeWrapper(probe))
		if res.Status != StatusOK || res.Target != "nil" {
			t.Errorf("capability %q reachable: status=%s target=%q", probe, res.Status, res.Target)
		}
	}
}

func TestHelperContractFieldsAndDeterministicClock(t *testing.T) {
	helper := buildHelper(t)
	res := evalWith(t, helper, routeWrapper(
		`return ctx.url.host .. "|" .. ctx.url.scheme .. "|" .. ctx.url.query .. "|" .. ctx.url.fragment .. "|" .. ctx.url.original .. "|" .. tostring(utils.epoch()) .. "|" .. tostring(utils.utc().year)`))
	if res.Status != StatusOK {
		t.Fatalf("status=%s category=%s", res.Status, res.Category)
	}
	want := "example.com|https|a=1|f|https://example.com/x?a=1#f|1700000000|2023"
	if res.Target != want {
		t.Errorf("fields = %q, want %q", res.Target, want)
	}
}

// Contract normalization: scheme/host lowercased, one trailing dot
// stripped — identical to the static router's host matching.
func TestHelperSeesNormalizedSchemeAndHost(t *testing.T) {
	helper := buildHelper(t)
	p := NewProvider(helper, writeScript(t, routeWrapper(
		fmt.Sprintf(`return ctx.url.scheme .. "|" .. ctx.url.host`))))
	res, err := p.Decide(context.Background(), "HTTPS://EXAMPLE.COM./x?a=1")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if res.Status != StatusOK || res.Target != "https|example.com" {
		t.Errorf("normalized fields = %+v (target %q), want https|example.com", res, res.Target)
	}
}

// A script that prints must not corrupt the framed response: print is
// stripped, so such scripts fail with script-error and the response
// frame still parses exactly.
func TestHelperPrintCannotCorruptFramedResponse(t *testing.T) {
	helper := buildHelper(t)
	res := evalWith(t, helper, routeWrapper(
		fmt.Sprintf(`print("CORRUPT-FRAME-OUTPUT") return "ok-target"`)))
	if res.Status != StatusError || res.Category != "script-error" {
		t.Fatalf("print-using script = %+v, want script-error (print is stripped)", res)
	}
	if strings.Contains(res.Category+res.Target, "CORRUPT") {
		t.Errorf("script output leaked into the response: %+v", res)
	}
}

// URL validation precedes the script and the helper: malformed and
// non-http(s) URLs are visible errors and the helper is never spawned.
func TestProviderRejectsInvalidURLBeforeHelper(t *testing.T) {
	p := NewProvider(filepath.Join(t.TempDir(), "helper-should-not-exist"),
		writeScript(t, routeWrapper(fmt.Sprintf(`return "x"`))))
	for _, bad := range []string{
		"ht tp://broken.test", // url.Parse error
		"ftp://broken.test/x", // parsed, unsupported scheme
		"javascript:alert(1)", // parsed, unsupported scheme
	} {
		if _, err := p.Decide(context.Background(), bad); err == nil {
			t.Errorf("invalid url %q must be rejected visibly", bad)
		}
	}
}

// Contract-safe URL handling: empty hosts and malformed/unsupported
// URLs are visible errors (mirroring the static router), and percent
// escapes in path/query/fragment stay as written — no decoding.
func TestProviderContractSafeURLHandling(t *testing.T) {
	helper := buildHelper(t)

	// Empty and dot-only hosts are invalid after normalization.
	pEmpty := NewProvider(helper, writeScript(t, routeWrapper(fmt.Sprintf(`return "x"`))))
	for _, bad := range []string{"https:///x", "https://./x"} {
		if _, err := pEmpty.Decide(context.Background(), bad); err == nil {
			t.Errorf("empty-host URL %q must be rejected visibly", bad)
		}
	}

	// Percent escapes survive as written; the helper script observes
	// the escaped forms, never decoded separators.
	scriptProbe := writeScript(t, routeWrapper(
		fmt.Sprintf(`return ctx.url.path .. "|" .. ctx.url.query .. "|" .. ctx.url.fragment`)))
	pEsc := NewProvider(helper, scriptProbe)
	res, err := pEsc.Decide(context.Background(), "https://example.com/a%2Fb?c=%23d#e%2Ff")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if res.Status != StatusOK {
		t.Fatalf("status = %s (%s)", res.Status, res.Category)
	}
	want := "/a%2Fb|c=%23d|e%2Ff"
	if res.Target != want {
		t.Errorf("escaped fields = %q, want %q", res.Target, want)
	}
}

// Credentials: URLs carrying userinfo are rejected visibly with fixed
// redacted text before the script is read or the helper spawns — no
// silent fallback, no credential ever echoed.
func TestProviderRejectsUserinfoURLsVisibly(t *testing.T) {
	// A helper path that does not exist proves the helper is never
	// spawned for userinfo URLs.
	p := NewProvider(filepath.Join(t.TempDir(), "helper-never-spawned"),
		writeScript(t, routeWrapper(fmt.Sprintf(`return "x"`))))
	res, err := p.Decide(context.Background(), "https://user:secret-token@example.com/x")
	if err == nil {
		t.Fatal("userinfo URL must be rejected visibly")
	}
	if !strings.Contains(err.Error(), "userinfo credentials are not supported") {
		t.Errorf("error must be the fixed redacted message, got %q", err)
	}
	if strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), "example.com") {
		t.Errorf("error must not echo the URL or credentials: %q", err)
	}
	if res != (Result{}) {
		t.Errorf("no partial result may be returned: %+v", res)
	}
}

// ctx is read-only: writes to existing or new fields are denied, and
// the metatable is hidden ("protected"), so the real table behind the
// proxy is unreachable.
func TestHelperReadOnlyCtx(t *testing.T) {
	helper := buildHelper(t)
	res := evalWith(t, helper, routeWrapper(
		fmt.Sprintf(`ctx.url.host = "changed" return tostring(ctx.url.host)`)))
	if res.Status != StatusError || res.Category != "script-error" {
		t.Errorf("ctx.url write must be denied: %+v", res)
	}
	res = evalWith(t, helper, routeWrapper(
		fmt.Sprintf(`ctx.newfield = 1 return tostring(ctx.newfield)`)))
	if res.Status != StatusError || res.Category != "script-error" {
		t.Errorf("ctx write must be denied: %+v", res)
	}
	res = evalWith(t, helper, routeWrapper(
		fmt.Sprintf(`return tostring(getmetatable(ctx.url))`)))
	if res.Status != StatusOK || res.Target != "protected" {
		t.Errorf("metatable must be hidden (protected), got %+v", res)
	}
}

func TestHelperFailureCategories(t *testing.T) {
	helper := buildHelper(t)
	cases := []struct {
		name     string
		script   string
		status   string
		category string
	}{
		{"runtime error", routeWrapper(fmt.Sprintf(`error("boom")`)), StatusError, "script-error"},
		{"infinite loop", routeWrapper(fmt.Sprintf(`while true do end`)), StatusError, "script-timeout"},
		{"memory flood", routeWrapper(fmt.Sprintf(`local t = {} while true do t[#t+1] = string.rep('x', 100000) end`)), StatusError, "memory-cap"},
		{"invalid return", routeWrapper(fmt.Sprintf(`return 42`)), StatusError, "script-error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := evalWith(t, helper, tc.script)
			if res.Status != tc.status || res.Category != tc.category {
				t.Errorf("status=%s category=%s, want %s/%s", res.Status, res.Category, tc.status, tc.category)
			}
		})
	}
}

// Load-level failures (malformed script, missing route function) are
// visible configuration errors: Decide returns an error, not a
// fallback result.
func TestProviderVisibleScriptLoadFailures(t *testing.T) {
	helper := buildHelper(t)
	for _, tc := range []struct {
		name   string
		script string
	}{
		{"syntax error", `function route( end`},
		{"missing route function", `x = 1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := NewProvider(helper, writeScript(t, tc.script))
			if _, err := p.Decide(context.Background(), "https://example.com/broken"); err == nil {
				t.Fatalf("broken script must fail visibly")
			}
		})
	}
}

func TestHelperNeverLeaksURLOrScriptInResponse(t *testing.T) {
	helper := buildHelper(t)
	secret := "secret-token-abc123"
	res := evalWith(t, helper, routeWrapper(fmt.Sprintf(`error("secret-token-abc123 attempt on https://leak.example/x#frag")`)))
	if strings.Contains(res.Target, secret) || strings.Contains(res.Category, secret) {
		t.Errorf("response carries secret-bearing text: %+v", res)
	}
	if res.Category != "script-error" {
		t.Errorf("category = %q, want script-error", res.Category)
	}
}

func TestProviderVisibleLoadFailure(t *testing.T) {
	p := NewProvider("/nonexistent/brouter-lua-helper", filepath.Join(t.TempDir(), "route.lua"))
	if _, err := p.Decide(context.Background(), "https://example.com/x"); err == nil {
		t.Fatal("missing script must fail visibly")
	}
}

// Per-URL snapshot semantics: the script file is re-read for every URL,
// edits apply on the next call without any restart, and an in-flight
// snapshot is immune to an atomic-rename swap.
func TestProviderScriptSnapshotReload(t *testing.T) {
	helper := buildHelper(t)
	dir := t.TempDir()
	script := filepath.Join(dir, "route.lua")
	write := func(body string) {
		tmp := script + ".tmp"
		if err := os.WriteFile(tmp, []byte(routeWrapper(body)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(tmp, script); err != nil { // atomic replacement
			t.Fatal(err)
		}
	}
	write(`return "target-one"`)

	// One provider instance across URLs: per-URL reload is the
	// provider's own responsibility (script re-read per Decide).
	p := NewProvider(helper, script)
	p.Clock = func() time.Time { return time.Unix(1700000000, 0) }

	res, err := p.Decide(context.Background(), "https://example.com/one")
	if err != nil || res.Status != StatusOK || res.Target != "target-one" {
		t.Fatalf("first decision = %+v err=%v", res, err)
	}

	// Edit: the very next URL must see the new script.
	write(`return "target-two"`)
	res, err = p.Decide(context.Background(), "https://example.com/two")
	if err != nil || res.Status != StatusOK || res.Target != "target-two" {
		t.Fatalf("reload after edit = %+v err=%v", res, err)
	}

	// Broken edit: visible failure, no stale last-known-good target.
	write(`function route( end`)
	if _, err := p.Decide(context.Background(), "https://example.com/broken"); err == nil {
		t.Fatal("broken script must fail visibly, not serve a stale result")
	}
}

// Unstable mid-read edits are rejected visibly: when the two snapshot
// reads disagree, ReadScript fails instead of serving a torn script.
func TestReadScriptRejectsUnstableRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "route.lua")
	if err := os.WriteFile(path, []byte("stable"), 0o644); err != nil {
		t.Fatal(err)
	}
	original := readFile
	calls := 0
	readFile = func(string) ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte("torn-partial"), nil
		}
		return []byte("fully-rewritten-by-editor"), nil
	}
	defer func() { readFile = original }()

	if _, err := ReadScript(path); err == nil || !strings.Contains(err.Error(), "changed while reading") {
		t.Fatalf("unstable read must be rejected visibly, got err=%v", err)
	}
}

// Symlinked script paths resolve through the open: replacing the
// symlink target or the symlink itself is picked up on the next read.
func TestLoaderFollowsSymlinkReplacement(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link.lua")

	atomicWrite(t, filepath.Join(dir, "real.lua"), "first")
	if err := os.Symlink(filepath.Join(dir, "real.lua"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	assertContent(t, link, "first")

	// Atomic-rename replacement of the target file.
	atomicWrite(t, filepath.Join(dir, "real.next"), "second")
	if err := os.Rename(filepath.Join(dir, "real.next"), filepath.Join(dir, "real.lua")); err != nil {
		t.Fatal(err)
	}
	assertContent(t, link, "second")

	// Replacing the symlink itself (atomic-rename editors do this):
	// the new link is created at a temp name and renamed into place.
	atomicWrite(t, filepath.Join(dir, "other.lua"), "third")
	tmpLink := link + ".tmp"
	if err := os.Symlink(filepath.Join(dir, "other.lua"), tmpLink); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Rename(tmpLink, link); err != nil {
		t.Fatal(err)
	}
	assertContent(t, link, "third")
}

func atomicWrite(t *testing.T, path, content string) {
	t.Helper()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

func assertContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := ReadScript(path)
	if err != nil || string(got) != want {
		t.Errorf("content = %q err=%v, want %q", got, err, want)
	}
}
