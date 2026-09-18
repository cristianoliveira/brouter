package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func logTargetConfig(browserPath, logPath string) string {
	return executableTargetConfig(browserPath) + `
[log]
enabled = true
path = ` + quoteTOML(logPath) + `
`
}

func quoteTOML(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func TestOpenWithLogEnabledWritesOneRedactedLine(t *testing.T) {
	// Given an opted-in routing log, when open launches a URL carrying
	// credentials and a path, then the open succeeds and the log holds
	// exactly one line with target/outcome/durations and REDACTED — no
	// URL bytes anywhere.
	dir := t.TempDir()
	browser, _ := writeFakeBrowser(t, dir, 0)
	logPath := filepath.Join(dir, "routing.log")
	path := writeConfig(t, logTargetConfig(browser, logPath))

	code, stdout, stderr := runCapture(t, "", "open", "--config", path,
		"https://alice:secret@docs.example/private/page?token=1")

	if code != 0 || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "launched: fake") {
		t.Fatalf("stdout = %q", stdout)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	got := string(data)
	for _, forbidden := range []string{"alice", "secret", "docs.example", "/private", "token=1", "https"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("log leaks %q:\n%s", forbidden, got)
		}
	}
	if !strings.Contains(got, " target=fake outcome=launched ") || !strings.Contains(got, "url=REDACTED\n") {
		t.Fatalf("log line missing contract fields:\n%s", got)
	}
}

func TestOpenWithoutLogTableWritesNothing(t *testing.T) {
	// Given a config without [log], when open runs, then no routing log
	// is created and behavior is byte-identical to before the feature.
	dir := t.TempDir()
	browser, _ := writeFakeBrowser(t, dir, 0)
	path := writeConfig(t, executableTargetConfig(browser))

	code, stdout, stderr := runCapture(t, "", "open", "--config", path, "https://anything.example/")

	if code != 0 || stderr != "" {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "launched: fake") {
		t.Fatalf("stdout = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "routing.log")); !os.IsNotExist(err) {
		t.Fatalf("routing log must not exist without opt-in: %v", err)
	}
}

func TestOpenLaunchFailureIsLoggedWithoutChangingStatus(t *testing.T) {
	// Given a failing browser and an opted-in log, when open runs, then
	// the exit status and visible error are unchanged and the log
	// records outcome=launch-failed with a sanitized error.
	dir := t.TempDir()
	browser, _ := writeFakeBrowser(t, dir, 3)
	logPath := filepath.Join(dir, "routing.log")
	path := writeConfig(t, logTargetConfig(browser, logPath))

	code, _, stderr := runCapture(t, "", "open", "--config", path, "https://docs.example/x")

	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "open failed") {
		t.Fatalf("stderr = %q", stderr)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "outcome=launch-failed") || !strings.Contains(got, "url=REDACTED") {
		t.Fatalf("failure not recorded:\n%s", got)
	}
	if strings.Contains(got, "docs.example") {
		t.Fatalf("log leaks the URL:\n%s", got)
	}
}

func TestOpenWithUnwritableLogStillLaunchesAndWarns(t *testing.T) {
	// Given a log path whose directory cannot exist (a file occupies
	// the way), when open runs, then the launch succeeds with the same
	// exit status and stdout, and the failure is a visible redacted
	// warning — never a routing failure.
	dir := t.TempDir()
	browser, argvLog := writeFakeBrowser(t, dir, 0)
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := writeConfig(t, logTargetConfig(browser, filepath.Join(blocker, "routing.log")))

	code, stdout, stderr := runCapture(t, "", "open", "--config", path, "https://docs.example/x")

	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "launched: fake") {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "routing log write failed") {
		t.Fatalf("warning must be visible, stderr = %q", stderr)
	}
	if strings.Contains(stderr, "docs.example") {
		t.Fatalf("warning must not echo the URL: %q", stderr)
	}
	if argv := readArgvLog(t, argvLog); len(argv) != 1 {
		t.Fatalf("browser argv = %q", argv)
	}
}

func TestOpenRejectsInvalidLogConfigVisibly(t *testing.T) {
	// Given [log] enabled with a relative path, when open runs, then
	// the config is rejected visibly before any browser starts.
	dir := t.TempDir()
	browser, _ := writeFakeBrowser(t, dir, 0)
	path := writeConfig(t, logTargetConfig(browser, "relative/routing.log"))

	code, _, stderr := runCapture(t, "", "open", "--config", path, "https://docs.example/x")

	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "log: path must be absolute") {
		t.Fatalf("stderr = %q", stderr)
	}
}
