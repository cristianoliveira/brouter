package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianoliveira/brouter/internal/infra/localfile"
)

// writeFakeBrowser creates an executable script that records its argv,
// one entry per line, into a log file inside dir, and exits with
// exitCode. It stands in for a real browser so open runs end to end
// without a GUI.
func writeFakeBrowser(t *testing.T, dir string, exitCode int) (script string, log string) {
	t.Helper()

	log = filepath.Join(dir, "argv.log")
	script = filepath.Join(dir, "fake-browser")
	body := fmt.Sprintf("#!/bin/sh\nfor arg in \"$@\"; do\n\tprintf '%%s\\n' \"$arg\" >> %s\ndone\nexit %d\n", log, exitCode)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script, log
}

func executableTargetConfig(browserPath string) string {
	return fmt.Sprintf(`
default = "fake"

[browsers.fake]
command = %q
`, browserPath)
}

func readArgvLog(t *testing.T, log string) []string {
	t.Helper()
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("cannot read argv log: %v", err)
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

func TestOpenLaunchesTheSelectedBrowserThroughARule(t *testing.T) {
	// Given a rule that routes a URL to an executable target, when open
	// runs, the browser process receives the URL as its one argument and
	// the command reports success without touching stderr.
	dir := t.TempDir()
	browser, log := writeFakeBrowser(t, dir, 0)
	path := writeConfig(t, executableTargetConfig(browser)+`
[[rules]]
name = "docs"
matcher = "exact-host"
pattern = "docs.example"
target = "fake"
`)

	code, stdout, stderr := runCapture(t, "", "open", "--config", path, "https://docs.example/page?a=1&b=2")

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	argv := readArgvLog(t, log)
	if len(argv) != 1 || argv[0] != "https://docs.example/page?a=1&b=2" {
		t.Errorf("browser argv = %q, want exactly the URL as one argument", argv)
	}
	if !strings.Contains(stdout, "launched: fake") {
		t.Errorf("stdout = %q, want a launched report", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty on success", stderr)
	}
}

func TestOpenFallsBackToTheDefaultTarget(t *testing.T) {
	// Given no rule matches, when open runs, the default target launches;
	// fallback never switches to the system default handler.
	dir := t.TempDir()
	browser, log := writeFakeBrowser(t, dir, 0)
	path := writeConfig(t, executableTargetConfig(browser))

	code, _, stderr := runCapture(t, "", "open", "--config", path, "https://anything.example/")

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	if argv := readArgvLog(t, log); len(argv) != 1 {
		t.Errorf("browser argv = %q, want the URL delivered once", argv)
	}
}

func TestOpenReadsURLFromStdinWhenArgumentIsMissing(t *testing.T) {
	// Given no URL argument, when open runs, it reads one URL from stdin,
	// matching explain's input contract.
	dir := t.TempDir()
	browser, log := writeFakeBrowser(t, dir, 0)
	path := writeConfig(t, executableTargetConfig(browser))

	code, _, stderr := runCapture(t, "https://stdin.example/x\n", "open", "--config", path)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	if argv := readArgvLog(t, log); len(argv) != 1 || argv[0] != "https://stdin.example/x" {
		t.Errorf("browser argv = %q, want the stdin URL", argv)
	}
}

func TestOpenFailsExplicitlyWhenProfileDirectoryIsMissing(t *testing.T) {
	// Given a known target whose profile directory does not exist on this
	// machine, when open runs, it fails visibly instead of silently
	// opening the default profile or creating a new one. The unlikely
	// profile name keeps the failure deterministic on any machine.
	path := writeConfig(t, `
default = "brave-work"

[browsers.brave-work]
browser = "brave"
profile = "task-0012-missing-profile"
`)

	code, stdout, stderr := runCapture(t, "", "open", "--config", path, "https://example.com/")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "task-0012-missing-profile") || !strings.Contains(stderr, "never creates profiles") {
		t.Errorf("stderr = %q, want an explicit missing-profile report", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing on failure", stdout)
	}
}

func TestOpenFailsVisiblyWhenTheExecutableIsMissing(t *testing.T) {
	// Given an executable target whose path does not exist, when open
	// runs, the failure names the target and nothing is launched.
	missing := filepath.Join(t.TempDir(), "no-browser")
	path := writeConfig(t, executableTargetConfig(missing))

	code, stdout, stderr := runCapture(t, "", "open", "--config", path, "https://example.com/")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "fake") || !strings.Contains(stderr, missing) {
		t.Errorf("stderr = %q, want the target and missing path named", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing on failure", stdout)
	}
}

func TestOpenSurfacesBrowserLaunchFailure(t *testing.T) {
	// Given a browser that exits nonzero, when open runs, the exit status
	// surfaces as a failure; there is no silent fallback.
	dir := t.TempDir()
	browser, _ := writeFakeBrowser(t, dir, 3)
	path := writeConfig(t, executableTargetConfig(browser))

	code, _, stderr := runCapture(t, "", "open", "--config", path, "https://example.com/")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "3") {
		t.Errorf("stderr = %q, want the browser exit status named", stderr)
	}
}

func TestOpenRejectsRouterAsTarget(t *testing.T) {
	// Given an executable target pointing at the running brouter binary,
	// when open runs, the recursion is rejected before launch. The test
	// binary stands in for the brouter executable.
	path := writeConfig(t, executableTargetConfig(os.Args[0]))

	code, _, stderr := runCapture(t, "", "open", "--config", path, "https://example.com/")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "brouter itself") {
		t.Errorf("stderr = %q, want the recursion rejection", stderr)
	}
}

func TestOpenRejectsNonHTTPSchemes(t *testing.T) {
	// Given a URL the router would never route, when open runs, it fails
	// with the validation exit code before any process starts.
	dir := t.TempDir()
	browser, log := writeFakeBrowser(t, dir, 0)
	path := writeConfig(t, executableTargetConfig(browser))

	code, _, stderr := runCapture(t, "", "open", "--config", path, "ftp://example.com/file")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr == "" {
		t.Error("stderr = empty, want a scheme rejection")
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Error("a browser process started for a rejected URL")
	}
}

func TestOpenRejectsMultipleURLArguments(t *testing.T) {
	// Given two URL arguments, when open runs, it is a usage error.
	dir := t.TempDir()
	browser, _ := writeFakeBrowser(t, dir, 0)
	path := writeConfig(t, executableTargetConfig(browser))

	code, _, _ := runCapture(t, "", "open", "--config", path, "https://a.example/", "https://b.example/")

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// route_command happy path: the command answers with an exact target
// ID and that target is launched even though no static rule matches.
func TestOpenRouteCommandAnswersTarget(t *testing.T) {
	// Given a route_command that answers the single line fake, when
	// open runs, the fake browser receives the URL and no rule was
	// needed.
	dir := t.TempDir()
	browser, log := writeFakeBrowser(t, dir, 0)
	routerCmd := filepath.Join(dir, "router.sh")
	if err := os.WriteFile(routerCmd, []byte("#!/bin/sh\nprintf 'fake'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := writeConfig(t, executableTargetConfig(browser)+fmt.Sprintf(`
[route_command]
command = [%q]
`, routerCmd))

	code, stdout, stderr := runCapture(t, "", "open", "--config", path, "https://example.com/x")

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	argv := readArgvLog(t, log)
	if len(argv) != 1 || argv[0] != "https://example.com/x" {
		t.Errorf("browser argv = %q, want exactly the URL", argv)
	}
	if !strings.Contains(stdout, "launched: fake") {
		t.Errorf("stdout = %q, want a launched report", stdout)
	}
}

// An exact @default line defers to the ordered static rules.
func TestOpenRouteCommandNullDefersToStaticRules(t *testing.T) {
	// Given a command that defers, when open runs a URL matched by a
	// static rule, the static rule's target is launched.
	dir := t.TempDir()
	browser, log := writeFakeBrowser(t, dir, 0)
	routerCmd := filepath.Join(dir, "router.sh")
	if err := os.WriteFile(routerCmd, []byte("#!/bin/sh\nprintf '@default'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := writeConfig(t, executableTargetConfig(browser)+fmt.Sprintf(`
[[rules]]
name = "docs"
matcher = "exact-host"
pattern = "docs.example"
target = "fake"

[route_command]
command = [%q]
`, routerCmd))

	code, stdout, stderr := runCapture(t, "", "open", "--config", path, "https://docs.example/page")

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	argv := readArgvLog(t, log)
	if len(argv) != 1 || argv[0] != "https://docs.example/page" {
		t.Errorf("browser argv = %q, want the static rule's launch", argv)
	}
	if !strings.Contains(stdout, "launched: fake") {
		t.Errorf("stdout = %q, want a launched report", stdout)
	}
}

// Unknown targets and command failures are visible and never silently
// fall back to static rules.
func TestOpenRouteCommandFailuresAreVisible(t *testing.T) {
	dir := t.TempDir()
	browser, _ := writeFakeBrowser(t, dir, 0)
	unknownCmd := filepath.Join(dir, "unknown.sh")
	if err := os.WriteFile(unknownCmd, []byte("#!/bin/sh\nprintf 'not-a-target'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	failCmd := filepath.Join(dir, "fail.sh")
	if err := os.WriteFile(failCmd, []byte("#!/bin/sh\nexit 7\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := executableTargetConfig(browser) + `
[[rules]]
name = "would-also-match"
matcher = "exact-host"
pattern = "docs.example"
target = "fake"

[route_command]
command = [%q]
`
	path := writeConfig(t, fmt.Sprintf(base, unknownCmd))
	code, stdout, stderr := runCapture(t, "", "open", "--config", path, "https://docs.example/page")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "unknown target") {
		t.Fatalf("unknown target must fail visibly: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	path = writeConfig(t, fmt.Sprintf(base, failCmd))
	code, stdout, stderr = runCapture(t, "", "open", "--config", path, "https://docs.example/page")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "route_command: command failed") {
		t.Fatalf("command failure must fail visibly: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

// URL acceptance precedes the spawn; credentials never appear anywhere.
func TestOpenRouteCommandURLPrevalidation(t *testing.T) {
	dir := t.TempDir()
	browser, _ := writeFakeBrowser(t, dir, 0)
	neverSpawned := filepath.Join(dir, "never-spawned")
	path := writeConfig(t, executableTargetConfig(browser)+fmt.Sprintf(`
[route_command]
command = [%q]
`, neverSpawned))

	code, stdout, stderr := runCapture(t, "", "open", "--config", path, "https://user:secret@example.com/x")

	if code != 1 {
		t.Fatalf("exit code = %d, want visible failure", code)
	}
	if !strings.Contains(stderr, "userinfo credentials are not supported") {
		t.Errorf("stderr = %q, want the fixed redacted message", stderr)
	}
	if strings.Contains(stderr, "secret") {
		t.Errorf("credentials must never appear: %q", stderr)
	}
	_ = stdout
}

func TestOpenSendsLocalDocumentStraightToTheDefaultBrowser(t *testing.T) {
	// Given a local document with a browser-viewable extension, when
	// open runs, the default configured browser receives the correctly
	// encoded file URL as its one argument — the static rules and the
	// route_command protocol are never consulted for local files.
	dir := t.TempDir()
	browser, log := writeFakeBrowser(t, dir, 0)
	path := writeConfig(t, executableTargetConfig(browser)+fmt.Sprintf(`
[[rules]]
name = "docs"
matcher = "exact-host"
pattern = "docs.example"
target = "fake"

[route_command]
command = [%q]
`, writeAnsweringScript(t, dir, "fake")))

	doc := writeLocalDocument(t, dir, "my docs/report page #3 – ~100%.html", "<html></html>")

	code, stdout, stderr := runCapture(t, "", "open", "--config", path, doc)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	wantURL, err := localfile.Resolve(doc)
	if err != nil {
		t.Fatal(err)
	}
	if wantURL == "file://"+doc {
		t.Fatalf("expected encoding for %q", doc)
	}
	argv := readArgvLog(t, log)
	if len(argv) != 1 || argv[0] != wantURL {
		t.Errorf("browser argv = %q, want exactly the encoded file URL %q", argv, wantURL)
	}
	if !strings.Contains(stdout, "launched: fake") {
		t.Errorf("stdout = %q, want a launched report", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty on success", stderr)
	}
}

func TestOpenSendsAFileURLInputToTheDefaultBrowser(t *testing.T) {
	// Given a file:// URL, when open runs, it is treated as a local
	// document and encoded canonically before the launch.
	dir := t.TempDir()
	browser, log := writeFakeBrowser(t, dir, 0)
	path := writeConfig(t, executableTargetConfig(browser))

	doc := filepath.Join(dir, "deck.pdf")
	if err := os.WriteFile(doc, []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runCapture(t, "", "open", "--config", path, "file://"+doc)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	argv := readArgvLog(t, log)
	if len(argv) != 1 || argv[0] != "file://"+doc {
		t.Errorf("browser argv = %q, want the canonical file URL", argv)
	}
}

func TestOpenRejectsUnsupportedLocalDocumentsWithRedactedErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		prepare  func(dir string) string
		wantText string
	}{
		{
			name: "missing file",
			prepare: func(dir string) string {
				return filepath.Join(dir, "gone.html")
			},
			wantText: "does not exist",
		},
		{
			name: "directory",
			prepare: func(dir string) string {
				sub := filepath.Join(dir, "bundle.app")
				if err := os.Mkdir(sub, 0o755); err != nil {
					t.Fatal(err)
				}
				return sub
			},
			wantText: "not a regular file",
		},
		{
			name: "unsupported extension",
			prepare: func(dir string) string {
				p := filepath.Join(dir, "installer.dmg")
				if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				return p
			},
			wantText: "not supported",
		},
		{
			name: "executable",
			prepare: func(dir string) string {
				p := filepath.Join(dir, "tool.sh")
				if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
					t.Fatal(err)
				}
				return p
			},
			wantText: "not supported",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			browser, log := writeFakeBrowser(t, dir, 0)
			path := writeConfig(t, executableTargetConfig(browser))

			raw := tc.prepare(dir)
			code, _, stderr := runCapture(t, "", "open", "--config", path, raw)

			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if !strings.Contains(stderr, tc.wantText) {
				t.Errorf("stderr = %q, want the %q category", stderr, tc.wantText)
			}
			if strings.Contains(stderr, dir) {
				t.Errorf("stderr = %q leaks the input path — diagnostics must stay redacted", stderr)
			}
			if _, err := os.Stat(log); !os.IsNotExist(err) {
				t.Error("a browser process was started for a rejected document")
			}
		})
	}
}

func TestOpenKeepsHTTPRoutingUntouchedForLocalDocuments(t *testing.T) {
	// Given the local-document path exists, when an http URL is opened,
	// the routing behavior is exactly as before: rules and route_command
	// see web URLs only; local policy never intercepts them.
	dir := t.TempDir()
	browser, log := writeFakeBrowser(t, dir, 0)
	path := writeConfig(t, executableTargetConfig(browser)+fmt.Sprintf(`
[[rules]]
name = "docs"
matcher = "exact-host"
pattern = "docs.example"
target = "fake"

[route_command]
command = [%q]
`, writeAnsweringScript(t, dir, "fake")))

	code, _, stderr := runCapture(t, "", "open", "--config", path, "https://docs.example/page")

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	argv := readArgvLog(t, log)
	if len(argv) != 1 || argv[0] != "https://docs.example/page" {
		t.Errorf("browser argv = %q, want the routed URL", argv)
	}
}

// writeAnsweringScript creates an executable route_command script that
// always answers with the given single-line target.
func writeAnsweringScript(t *testing.T, dir, answer string) string {
	t.Helper()
	script := filepath.Join(dir, "router.sh")
	body := fmt.Sprintf("#!/bin/sh\nprintf '%s'\n", answer)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return script
}

// writeLocalDocument creates a file under dir (subdirectories allowed
// in the name) with the given content and returns its full path.
func writeLocalDocument(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
