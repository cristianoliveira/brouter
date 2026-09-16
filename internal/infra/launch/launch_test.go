package launch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// writeFakeBrowser writes an executable shell script that records its
// argv, one entry per line, into logPath, and exits with exitCode. It
// models a real browser process without a GUI.
func writeFakeBrowser(t *testing.T, dir string, logPath string, exitCode int) string {
	t.Helper()
	path := filepath.Join(dir, "fake-browser")
	script := "#!/bin/sh\n" +
		"for arg in \"$@\"; do\n" +
		"	printf '%s\\n' \"$arg\" >> " + logPath + "\n" +
		"done\n" +
		"printf 'argc=%s\\n' \"$#\" >> " + logPath + "\n" +
		"exit " + itoa(exitCode) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("cannot write fake browser: %v", err)
	}
	return path
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

func TestLaunchPassesURLAsOneStructuredArgument(t *testing.T) {
	// Given a fake browser that records argv, when launching a URL with
	// query, fragment, spaces, ampersands, and non-ASCII characters, the
	// URL arrives as exactly one argument, byte-for-byte.
	dir := t.TempDir()
	log := filepath.Join(dir, "argv.log")
	browser := writeFakeBrowser(t, dir, log, 0)

	plan := Plan{Executable: browser, Detail: "test"}
	rawURL := "https://example.com/path?a=1&b=2#frag with spaces & more/héllo?x=%20"

	if err := Launch(plan, rawURL, nil); err != nil {
		t.Fatalf("Launch failed: %v", err)
	}

	lines := readLines(t, log)
	if len(lines) != 2 {
		t.Fatalf("recorded argv = %q (%d lines), want exactly [url, argc=1]", lines, len(lines))
	}
	if lines[0] != rawURL {
		t.Errorf("url argument = %q, want %q", lines[0], rawURL)
	}
	if lines[1] != "argc=1" {
		t.Errorf("argc line = %q, want argc=1 (URL must be one argument, never shell-split)", lines[1])
	}
}

func TestLaunchNeverUsesAShell(t *testing.T) {
	// Given a URL containing shell metacharacters, when launching, the
	// metacharacters reach the browser verbatim: a shell would have
	// interpreted ; and $( ) and split on spaces.
	dir := t.TempDir()
	log := filepath.Join(dir, "argv.log")
	browser := writeFakeBrowser(t, dir, log, 0)

	plan := Plan{Executable: browser, Detail: "test"}
	rawURL := `https://example.com/a;rm -rf /;echo $(whoami) 'quoted' "double"`

	if err := Launch(plan, rawURL, nil); err != nil {
		t.Fatalf("Launch failed: %v", err)
	}

	lines := readLines(t, log)
	if len(lines) != 2 || lines[0] != rawURL {
		t.Fatalf("argv = %q, want the raw URL as one unsplit argument", lines)
	}
}

func TestLaunchSurfacesBrowserExitStatus(t *testing.T) {
	// Given a fake browser that exits 7, when launching, the failure is
	// visible and names the exit status; the URL is never retried
	// elsewhere and no fallback runs.
	dir := t.TempDir()
	log := filepath.Join(dir, "argv.log")
	browser := writeFakeBrowser(t, dir, log, 7)

	plan := Plan{Executable: browser, Detail: "test"}
	err := Launch(plan, "https://example.com/", nil)
	if err == nil {
		t.Fatal("Launch succeeded, want visible failure")
	}
	if !strings.Contains(err.Error(), "7") {
		t.Errorf("error %q does not name the exit status", err)
	}
}

func TestLaunchRejectsNonHTTPSchemesBeforeExecuting(t *testing.T) {
	// Given a non-http(s) URL, when launching, it is rejected before any
	// process starts.
	dir := t.TempDir()
	log := filepath.Join(dir, "argv.log")
	browser := writeFakeBrowser(t, dir, log, 0)

	plan := Plan{Executable: browser, Detail: "test"}
	for _, rawURL := range []string{"ftp://example.com/x", "file:///etc/passwd", "", "example.com/no-scheme"} {
		if err := Launch(plan, rawURL, nil); err == nil {
			t.Errorf("Launch(%q) succeeded, want rejection", rawURL)
		}
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Error("a process was started for a rejected URL")
	}
}

func TestLaunchReportsMissingExecutableAsVisibleFailure(t *testing.T) {
	// Given the resolved executable vanished between resolve and launch,
	// when launching with the real runner, the exec error surfaces.
	plan := Plan{Executable: filepath.Join(t.TempDir(), "vanished"), Detail: "test"}

	err := Launch(plan, "https://example.com/", nil)
	if err == nil {
		t.Fatal("Launch succeeded, want visible failure")
	}
}

func TestLaunchUsesInjectedRunner(t *testing.T) {
	// Given an injected runner, when launching, it receives the command
	// with the browser path as argv[0] and the URL as argv[1].
	var captured *exec.Cmd
	runner := func(cmd *exec.Cmd) error {
		captured = cmd
		return nil
	}

	plan := Plan{Executable: "/usr/bin/brave", Args: []string{}, Detail: "test"}
	if err := Launch(plan, "https://example.com/", runner); err != nil {
		t.Fatalf("Launch failed: %v", err)
	}
	if captured == nil {
		t.Fatal("runner never received a command")
	}
	if captured.Path != "/usr/bin/brave" {
		t.Errorf("cmd.Path = %q, want /usr/bin/brave", captured.Path)
	}
	if len(captured.Args) != 2 || captured.Args[1] != "https://example.com/" {
		t.Errorf("cmd.Args = %q, want [browser, url]", captured.Args)
	}
}

func TestLaunchWrapsRunnerErrors(t *testing.T) {
	// Given a runner that fails, when launching, the error is wrapped with
	// context instead of being swallowed.
	plan := Plan{Executable: "/usr/bin/brave", Detail: "test"}
	runner := func(cmd *exec.Cmd) error { return errors.New("boom") }

	err := Launch(plan, "https://example.com/", runner)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %v, want wrapped boom", err)
	}
}

func TestLaunchCarriesProfileArgumentBeforeURL(t *testing.T) {
	// Given a resolved profiled plan, when launching, the profile
	// argument precedes the URL and both reach the browser as discrete
	// argv elements.
	dir := t.TempDir()
	log := filepath.Join(dir, "argv.log")
	browser := writeFakeBrowser(t, dir, log, 0)

	plan := Plan{Executable: browser, Args: []string{"--profile-directory=Work"}, Detail: "test"}
	if err := Launch(plan, "https://example.com/?q=1", nil); err != nil {
		t.Fatalf("Launch failed: %v", err)
	}

	lines := readLines(t, log)
	if len(lines) != 3 {
		t.Fatalf("argv = %q, want [profile-arg, url, argc=2]", lines)
	}
	if lines[0] != "--profile-directory=Work" || lines[1] != "https://example.com/?q=1" || lines[2] != "argc=2" {
		t.Errorf("argv = %q, want profile argument then URL as separate elements", lines)
	}
}

func TestLaunchFilePassesEncodedFileURLAsOneArgument(t *testing.T) {
	// Given a browser plan and a correctly encoded file URL, when
	// launching, the URL arrives as one structured argv element and the
	// process starts exactly once.
	dir := t.TempDir()
	log := filepath.Join(dir, "argv.log")
	browser := writeFakeBrowser(t, dir, log, 0)

	fileURL := "file:///tmp/my docs/report%20%E2%80%93%20%233.html"
	plan := Plan{Executable: browser, Detail: "test"}
	if err := LaunchFile(plan, fileURL, nil); err != nil {
		t.Fatalf("LaunchFile() error = %v", err)
	}

	lines := readLines(t, log)
	if len(lines) != 2 || lines[0] != fileURL || lines[1] != "argc=1" {
		t.Errorf("argv log = %q, want the URL alone as argc=1", lines)
	}
}

func TestLaunchFileRejectsNonFileSchemesBeforeExecuting(t *testing.T) {
	// Given anything that is not a file URL, when launching through the
	// file path, it is rejected before any process starts. The http(s)
	// gate of Launch stays the only door for web URLs.
	dir := t.TempDir()
	log := filepath.Join(dir, "argv.log")
	browser := writeFakeBrowser(t, dir, log, 0)

	plan := Plan{Executable: browser, Detail: "test"}
	for _, raw := range []string{
		"https://example.com/x",
		"ftp://example.com/x",
		"",
		"not a url",
	} {
		if err := LaunchFile(plan, raw, nil); err == nil {
			t.Errorf("LaunchFile(%q) succeeded, want rejection", raw)
		}
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Error("a process was started for a rejected input")
	}
}
