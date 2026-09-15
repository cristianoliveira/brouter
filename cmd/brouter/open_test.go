package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
