// Package scripts hosts the controlled-executable tests for the shell
// quality gate (check.sh). It intentionally contains no program logic:
// the gate is a POSIX shell script by design decision on TASK-0003.
package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const noiseLines = 50

// pinnedGolangci is the golangci-lint version the gate enforces; mirrors
// DEVELOPMENT.md and the CI workflow.
const pinnedGolangci = "2.13.2"

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

// fakeGolangci creates a controlled golangci-lint that reports the pinned
// version and then behaves according to body.
func fakeGolangci(t *testing.T, dir, body string) {
	t.Helper()

	writeFake(t, dir, "golangci-lint", `if [ "$1" = "version" ]; then echo "golangci-lint has version `+pinnedGolangci+` built with go1.0"; exit 0; fi;`+body)
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
	fakeGolangci(t, fakeDir, "exit 0")

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
	fakeGolangci(t, fakeDir, "exit 0")

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
	fakeGolangci(t, fakeDir, body)

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

func TestGateRefusesUnpinnedGolangciVersion(t *testing.T) {
	// Given an installed golangci-lint at a different version, when the
	// gate runs, it refuses with actionable guidance instead of risking
	// hooks-pass-CI-fails drift.
	fakeDir := t.TempDir()
	writeFake(t, fakeDir, "go", "exit 0")
	writeFake(t, fakeDir, "golangci-lint", `if [ "$1" = "version" ]; then echo "golangci-lint has version 9.9.9 built with go1.0"; exit 0; fi; exit 0`)

	stdout, _, err := runGate(t, fakeDir)

	if err == nil {
		t.Fatal("gate succeeded with unpinned golangci-lint, want refusal")
	}
	if !strings.HasPrefix(stdout, "false\n") {
		t.Errorf("stdout = %q, want prefix %q", stdout, "false\n")
	}
	if !strings.Contains(stdout, "expected "+pinnedGolangci) || !strings.Contains(stdout, "9.9.9") {
		t.Errorf("stdout = %q, want version mismatch diagnosis", stdout)
	}
	if !strings.Contains(stdout, "nix develop") || !strings.Contains(stdout, "go install") {
		t.Errorf("stdout = %q, want both recovery options", stdout)
	}
}

// TestPinnedLintRulesFireOnControlledViolations proves the documented
// thresholds behave as documented, using the real pinned golangci-lint on
// a controlled fixture. It skips (visibly) where the pinned tool is not
// installed; the gate itself owns the missing-tool contract.
func TestPinnedLintRulesFireOnControlledViolations(t *testing.T) {
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not on PATH; run inside `nix develop` to prove rule semantics")
	}

	fixture := writeRulesFixture(t)
	output := runLinter(t, fixture)

	expectOutputContains(t, output,
		"funlen",
		"cyclop",
		"complexity for function Complex is 15",
		"average complexity for the package fixture",
	)
	expectOutputOmits(t, output, "Dense")
}

// writeRulesFixture builds a temporary module with one violating file per
// documented rule plus one clean-but-dense file proving no hidden cap.
func writeRulesFixture(t *testing.T) string {
	t.Helper()

	fixture := t.TempDir()
	copyFile(t, filepath.Join("..", ".golangci.yml"), filepath.Join(fixture, ".golangci.yml"))
	writeFixtureFile(t, fixture, "go.mod", "module fixture\n\ngo 1.27\n")
	writeFixtureFile(t, fixture, "long.go", longFunctionSource(101))
	writeFixtureFile(t, fixture, "complex.go", complexFunctionSource(14))
	writeFixtureFile(t, fixture, "dense.go", denseFunctionSource(90))
	return fixture
}

// longFunctionSource returns a function with n statements spread over
// more than 100 lines, violating funlen's lines cap only.
func longFunctionSource(n int) string {
	var b strings.Builder
	b.WriteString("package fixture\n\nfunc Long() int {\n\tx := 0\n")
	for i := 0; i < n; i++ {
		b.WriteString("\tx++\n")
	}
	b.WriteString("\treturn x\n}\n")
	return b.String()
}

// complexFunctionSource returns a function with branch count b+1, so
// complexity b+1 exceeds cyclop's per-function ceiling of 10 and lifts the
// package average above 5.0.
func complexFunctionSource(b int) string {
	var branches strings.Builder
	for i := 1; i <= b; i++ {
		branches.WriteString(fmt.Sprintf("\tif x < %d {\n\t\tx += %d\n\t}\n", i, i))
	}
	return fmt.Sprintf("package fixture\n\nfunc Complex(x int) int {\n%s\treturn x\n}\n", branches.String())
}

// denseFunctionSource returns n statements across n lines: below the
// documented 100 alignment in both dimensions, so nothing may fire.
func denseFunctionSource(n int) string {
	var b strings.Builder
	b.WriteString("package fixture\n\nfunc Dense() int {\n\tx := 0\n")
	for i := 0; i < n; i++ {
		b.WriteString("\tx++\n")
	}
	b.WriteString("\treturn x\n}\n")
	return b.String()
}

func runLinter(t *testing.T, dir string) string {
	t.Helper()

	cmd := exec.Command("golangci-lint", "run")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected lint failures on fixture, got clean:\n%s", out)
	}
	return string(out)
}

func expectOutputContains(t *testing.T, output string, want ...string) {
	t.Helper()

	for _, w := range want {
		if !strings.Contains(output, w) {
			t.Errorf("lint output missing %q:\n%s", w, output)
		}
	}
}

func expectOutputOmits(t *testing.T, output, forbidden string) {
	t.Helper()

	if strings.Contains(output, forbidden) {
		t.Errorf("statement cap fired below the documented 100 alignment:\n%s", output)
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()

	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Dir(dst), filepath.Base(dst), string(data))
}

func writeFixtureFile(t *testing.T, dir, name, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
