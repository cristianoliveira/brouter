// Package scripts hosts the controlled-executable tests for the shell
// quality gate (check.sh). It intentionally contains no program logic:
// the gate is a POSIX shell script by design decision on TASK-0003.
package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const noiseLines = 50

// writeFake creates a controlled executable directory. Each named tool
// runs `body` via /bin/sh; calls append their arguments to a per-test
// calls file so tests can prove which tools ran.
func writeFake(t *testing.T, dir, name, body string) {
	t.Helper()

	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func fakeGo(t *testing.T, dir, callsFile string) {
	t.Helper()

	writeFake(t, dir, "go", "printf '%s\\n' \"$@\" >> "+callsFile+"\nprintf 'GOTOOLCHAIN=%s LC_ALL=%s\\n' \"$GOTOOLCHAIN\" \"$LC_ALL\" >> "+callsFile+"\nexit 0")
}

func runGate(t *testing.T, fakeDir string) (string, string, error) {
	t.Helper()
	return runGateWithEnv(t, fakeDir, fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// runGatePrereq runs the gate with ONLY the fake tools on PATH: enough for
// the prerequisite check, which must happen before any external utility.
func runGatePrereq(t *testing.T, fakeDir string) (string, string, error) {
	t.Helper()
	return runGateWithEnv(t, fakeDir, fakeDir)
}

func runGateWithEnv(t *testing.T, fakeDir, path string) (string, string, error) {
	t.Helper()

	script, err := filepath.Abs("check.sh")
	if err != nil {
		t.Fatal(err)
	}
	workDir := t.TempDir()

	cmd := exec.Command("/bin/sh", script)
	cmd.Dir = workDir
	// Fake tools shadow the real ones (fake dir first); generic utilities
	// (mkdir, head) still resolve from the host environment, like any real
	// user environment.
	cmd.Env = []string{"PATH=" + path}
	out, err := cmd.Output()

	log, _ := os.ReadFile(filepath.Join(workDir, ".tmp", "check.log"))
	return string(out), string(log), err
}

func TestGateSuccessPrintsExactlyTrue(t *testing.T) {
	// Given every gate step succeeds, when the gate runs, it prints
	// exactly "true", exits zero, and records the steps in the log.
	fakeDir := t.TempDir()
	calls := filepath.Join(t.TempDir(), "calls")
	fakeGo(t, fakeDir, calls)
	writeFake(t, fakeDir, "golangci-lint", "exit 0")

	stdout, log, err := runGate(t, fakeDir)

	if err != nil {
		t.Fatalf("gate failed: %v", err)
	}
	if stdout != "true\n" {
		t.Errorf("stdout = %q, want exactly %q", stdout, "true\n")
	}
	for _, step := range []string{"golangci-lint run", "go build ./...", "go test -vet=off ./..."} {
		if !strings.Contains(log, "$ "+step) {
			t.Errorf("log missing step %q:\n%s", step, log)
		}
	}
}

func TestGatePinsToolchainAndLocaleBeforeSteps(t *testing.T) {
	// Given the ambient environment has neither variable, when steps run,
	// go sees GOTOOLCHAIN=local and LC_ALL=C.
	fakeDir := t.TempDir()
	calls := filepath.Join(t.TempDir(), "calls")
	fakeGo(t, fakeDir, calls)
	writeFake(t, fakeDir, "golangci-lint", "exit 0")

	stdout, _, err := runGate(t, fakeDir)

	if err != nil {
		t.Fatalf("gate failed: %v", err)
	}
	if stdout != "true\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "true\n")
	}
	data, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "GOTOOLCHAIN=local LC_ALL=C") {
		t.Errorf("go ran with %q, want GOTOOLCHAIN=local LC_ALL=C", string(data))
	}
}

func TestGateLintFailureBoundsDiagnosticsKeepsFullLogAndFailsFast(t *testing.T) {
	// Given lint fails after emitting many lines, when the gate runs, it
	// starts with "false", bounds stdout diagnostics, keeps the full log,
	// and never runs later steps.
	fakeDir := t.TempDir()
	calls := filepath.Join(t.TempDir(), "calls")
	fakeGo(t, fakeDir, calls)
	body := "i=1; while [ $i -le " + strconv.Itoa(noiseLines) + " ]; do echo \"noise $i\"; i=$((i+1)); done; exit 3"
	writeFake(t, fakeDir, "golangci-lint", body)

	stdout, log, err := runGate(t, fakeDir)

	if err == nil {
		t.Fatal("gate succeeded, want nonzero exit")
	}
	if !strings.HasPrefix(stdout, "false\n") {
		t.Errorf("stdout = %q, want prefix %q", stdout, "false\n")
	}
	if got := strings.Count(stdout, "noise"); got != 20 {
		t.Errorf("stdout has %d diagnostic lines, want bounded 20", got)
	}
	if got := strings.Count(log, "noise"); got != noiseLines {
		t.Errorf("log has %d lines, want full %d", got, noiseLines)
	}
	if callsData, err := os.ReadFile(calls); err == nil {
		t.Errorf("later steps ran despite lint failure: %q", string(callsData))
	}
}

func TestGateMissingToolsFailWithActionableGuidance(t *testing.T) {
	// Given go exists but golangci-lint does not, when the gate runs, it
	// fails before any step with setup guidance instead of a tool error.
	fakeDir := t.TempDir()
	calls := filepath.Join(t.TempDir(), "calls")
	fakeGo(t, fakeDir, calls)

	stdout, _, err := runGatePrereq(t, fakeDir)

	if err == nil {
		t.Fatal("gate succeeded without golangci-lint, want failure")
	}
	if !strings.HasPrefix(stdout, "false\n") {
		t.Errorf("stdout = %q, want prefix %q", stdout, "false\n")
	}
	if !strings.Contains(stdout, "golangci-lint") {
		t.Errorf("stdout = %q, want missing-tool diagnosis", stdout)
	}
	if !strings.Contains(stdout, "nix develop") {
		t.Errorf("stdout = %q, want actionable setup guidance", stdout)
	}
	if _, rerr := os.ReadFile(calls); rerr == nil {
		t.Error("tools ran despite missing prerequisite")
	}
}
