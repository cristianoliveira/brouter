package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeFakeGo places a controlled `go` executable on a PATH. The gate under
// test must invoke tools through PATH so tests can substitute them.
func writeFakeGo(t *testing.T, script string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "go")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSuccessfulGatePrintsExactlyTrue(t *testing.T) {
	// Given every gate step succeeds, when the gate runs, it prints
	// exactly "true" and exits zero.
	fakePath := writeFakeGo(t, "echo '$0' >/dev/null; exit 0")
	t.Setenv("PATH", fakePath)

	logPath := filepath.Join(t.TempDir(), "check.log")
	var stdout bytes.Buffer

	code := runCheck("/bin/sh", testSteps(), logPath, &stdout)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := stdout.String(); got != "true\n" {
		t.Errorf("stdout = %q, want exactly %q", got, "true\n")
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("full log not written: %v", err)
	}
}

func TestFailedStepPrintsBoundedDiagnosticsAndFullLog(t *testing.T) {
	// Given a step fails after emitting many lines, when the gate runs,
	// it starts with "false", bounds the diagnostics, keeps the full log,
	// and exits nonzero.
	fakePath := writeFakeGo(t, "i=1; while [ $i -le 50 ]; do echo \"noise $i\"; i=$((i+1)); done; exit 3")
	t.Setenv("PATH", fakePath)

	logPath := filepath.Join(t.TempDir(), "check.log")
	var stdout bytes.Buffer

	code := runCheck("/bin/sh", testSteps(), logPath, &stdout)

	if code == 0 {
		t.Fatal("exit code = 0, want nonzero")
	}
	out := stdout.String()
	if !strings.HasPrefix(out, "false\n") {
		t.Errorf("stdout = %q, want prefix %q", out, "false\n")
	}
	if got := strings.Count(out, "noise"); got != boundedLines {
		t.Errorf("stdout contains %d diagnostic lines, want bounded %d", got, boundedLines)
	}
	if !strings.Contains(out, filepath.Base(logPath)) {
		t.Errorf("stdout = %q, want pointer to full log %q", out, logPath)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(log), "noise"); got != 50 {
		t.Errorf("log contains %d lines, want unbounded 50", got)
	}
}

func TestFailingTestStepReportsLintThenBuildThenTest(t *testing.T) {
	// Given steps run in pinned order, when an early step fails, later
	// steps do not run.
	fakePath := t.TempDir()
	script := "#!/bin/sh\necho \"$@\" >> " + fakePath + "/calls\nexit 1\n"
	if err := os.WriteFile(filepath.Join(fakePath, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakePath)

	var stdout bytes.Buffer
	code := runCheck("/bin/sh", testSteps(), filepath.Join(t.TempDir(), "check.log"), &stdout)

	if code == 0 {
		t.Fatal("exit code = 0, want nonzero")
	}
	data, err := os.ReadFile(filepath.Join(fakePath, "calls"))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(got) != 1 || !strings.Contains(got[0], "lint") {
		t.Errorf("steps invoked = %v, want only the lint step", got)
	}
}

func TestCheckScriptFailsWithActionableGuidanceWhenGoIsMissing(t *testing.T) {
	// Given go is not installed, when the entry script runs, it fails
	// with actionable setup guidance instead of a raw tool error.
	cmd := exec.Command("/bin/sh", "../../scripts/check.sh")
	cmd.Env = []string{"PATH=/nonexistent-brouter-test"}
	out, err := cmd.Output()

	if err == nil {
		t.Fatal("check.sh succeeded without go, want failure")
	}
	stdout := string(out)
	if !strings.HasPrefix(stdout, "false\n") {
		t.Errorf("stdout = %q, want prefix %q", stdout, "false\n")
	}
	if !strings.Contains(stdout, "go: not found") {
		t.Errorf("stdout = %q, want missing-go diagnosis", stdout)
	}
	if !strings.Contains(stdout, "nix develop") || !strings.Contains(stdout, "https://go.dev/dl/") {
		t.Errorf("stdout = %q, want actionable setup guidance", stdout)
	}
}

func TestCheckScriptDelegatesToGateThroughPath(t *testing.T) {
	// Given go is on PATH, when the entry script runs, it delegates to
	// the gate harness and preserves its output and exit status.
	fakePath := writeFakeGo(t, `if [ "$1" = "run" ]; then echo "true"; exit 0; fi; exit 97`)
	t.Setenv("PATH", fakePath)

	cmd := exec.Command("/bin/sh", "../../scripts/check.sh")
	cmd.Env = []string{"PATH=" + fakePath}
	out, err := cmd.Output()

	if err != nil {
		t.Fatalf("check.sh failed: %v", err)
	}
	if string(out) != "true\n" {
		t.Errorf("check.sh stdout = %q, want %q", out, "true\n")
	}
}

func testSteps() []step {
	return []step{
		{name: "lint", command: "go run ./tools/lint ."},
		{name: "build", command: "go build ./..."},
		{name: "test", command: "go test -vet=off ./..."},
	}
}
