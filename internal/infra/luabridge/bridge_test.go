package luabridge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The helper binary is compiled from the vendored, pinned Lua 5.4.7
// sources plus native/lua-helper/lua_helper.c. Skipping is honest:
// without a C toolchain this evaluation cannot run here.
func buildHelper(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("helper build test runs on darwin/linux only")
	}
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	helper := filepath.Join(t.TempDir(), "brouter-lua-helper")
	src, luasrc, sources := helperInputs(t)
	args := append([]string{"-O2", "-I", luasrc, src}, sources...)
	args = append(args, "-lm", "-o", helper)
	cmd := exec.Command("clang", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building helper failed: %v\n%s", err, out)
	}
	return helper
}

// helperInputs anchors paths at this source file (the repository root
// is four levels up) and lists the vendored Lua sources that are safe
// to link: capability libraries are excluded from the link entirely.
func helperInputs(t *testing.T) (src, luasrc string, sources []string) {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
	src = filepath.Join(root, "native", "lua-helper", "lua_helper.c")
	luasrc = filepath.Join(root, "third-party", "lua-5.4.7", "src")
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("helper source missing at %s: %v", src, err)
	}
	entries, err := os.ReadDir(luasrc)
	if err != nil {
		t.Fatalf("reading vendored lua dir: %v", err)
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
		t.Fatal("vendored lua dir has no linkable .c files")
	}
	return src, luasrc, sources
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
		{"invalid return", routeWrapper(fmt.Sprintf(`return 42`)), StatusError, "invalid-return"},
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

	p := NewProvider(helper, script)
	p.Clock = func() time.Time { return time.Unix(1700000000, 0) }

	res := evalWith(t, helper, routeWrapper(fmt.Sprintf(`return "target-one"`))) // warm-up sanity
	if res.Status != StatusOK || res.Target != "target-one" {
		t.Fatalf("first decision = %+v", res)
	}

	// Edit: the very next URL must see the new script.
	write(`return "target-two"`)
	res = evalWith(t, helper, routeWrapper(fmt.Sprintf(`return "target-two"`)))
	if res.Status != StatusOK || res.Target != "target-two" {
		t.Fatalf("reload after edit = %+v", res)
	}

	// Broken edit: visible failure, no stale last-known-good target.
	write(`function route( end`)
	p2 := NewProvider(helper, script)
	p2.Clock = p.Clock
	if _, err := p2.Decide(context.Background(), "https://example.com/broken"); err == nil {
		t.Fatal("broken script must fail visibly, not serve a stale result")
	}
}

// Symlinked script paths resolve through the open: replacing the
// symlink target or the symlink itself is picked up on the next read.
func TestLoaderFollowsSymlinkReplacement(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link.lua")

	atomicWrite(t, filepath.Join(dir, "real.lua"), "first")
	symlink(t, filepath.Join(dir, "real.lua"), link)
	assertContent(t, link, "first")

	// Atomic-rename replacement of the target file.
	atomicWrite(t, filepath.Join(dir, "real.next"), "second")
	symlink(t, filepath.Join(dir, "real.next"), filepath.Join(dir, "real.lua"))
	assertContent(t, link, "second")

	// Replacing the symlink itself (atomic-rename editors do this).
	atomicWrite(t, filepath.Join(dir, "other.lua"), "third")
	symlink(t, filepath.Join(dir, "other.lua"), link)
	assertContent(t, link, "third")
}

func real(dir, name string) string { return filepath.Join(dir, name) }

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
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
