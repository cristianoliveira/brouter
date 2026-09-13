package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeModule builds a temporary module with go.mod and the given files,
// mirroring the pinned module version in go.mod.
func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	writeFixtureFile(t, dir, "go.mod", "module fixture\n\ngo 1.27\n")
	for name, content := range files {
		writeFixtureFile(t, dir, name, content)
	}
	return dir
}

func runCheck(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()

	cmd := exec.Command("/bin/sh", append([]string{mustAbs("../scripts/" + name)}, args...)...)
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestFmtCheckReportsNonMutatingListAndFixRewrites(t *testing.T) {
	// Given a misformatted file, when the check runs, it fails naming the
	// file and leaves the bytes untouched; the fix command mutates, and
	// afterwards the check passes.
	bad := "package fixture\n\nfunc A() int {\nx := 1\nreturn x\n}\n"
	dir := writeModule(t, map[string]string{"bad.go": bad})
	before, _ := os.ReadFile(filepath.Join(dir, "bad.go"))

	out, err := runCheck(t, "fmt-check.sh", dir)
	if err == nil {
		t.Fatalf("fmt-check passed on misformatted file: %s", out)
	}
	if !strings.Contains(out, "bad.go") {
		t.Errorf("fmt-check output = %q, want the offending file named", out)
	}

	after, _ := os.ReadFile(filepath.Join(dir, "bad.go"))
	if string(before) != string(after) {
		t.Error("fmt-check mutated the file; it must be non-mutating")
	}

	if out, err := runCheck(t, "fmt-fix.sh", dir); err != nil {
		t.Fatalf("fmt-fix failed: %v: %s", err, out)
	}

	if out, err := runCheck(t, "fmt-check.sh", dir); err != nil {
		t.Fatalf("fmt-check still failing after fix: %v: %s", err, out)
	}
}

func TestVetRejectsSuspiciousCode(t *testing.T) {
	// Given a Printf verb mismatch, when vet runs, it fails with the
	// diagnostic; clean code passes.
	bad := writeModule(t, map[string]string{
		"bad.go": "package fixture\n\nimport \"fmt\"\n\nfunc A() {\n\tfmt.Printf(\"%d\", \"not a number\")\n}\n",
	})
	if out, err := runCheck(t, "vet.sh", bad); err == nil {
		t.Errorf("vet passed on Printf mismatch: %s", out)
	}

	good := writeModule(t, map[string]string{
		"ok.go": "package fixture\n\nimport \"fmt\"\n\nfunc A() {\n\tfmt.Printf(\"%d\", 1)\n}\n",
	})
	if out, err := runCheck(t, "vet.sh", good); err != nil {
		t.Errorf("vet failed on clean module: %v: %s", err, out)
	}
}

func TestRaceDetectsUnsynchronizedAccess(t *testing.T) {
	// Given a test with unsynchronized concurrent writes, when the race
	// check runs, it fails reporting the race.
	racy := writeModule(t, map[string]string{
		"race_test.go": `package fixture

import (
	"sync"
	"testing"
)

func TestRace(t *testing.T) {
	var n int
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); for i := 0; i < 1000; i++ { n++ } }()
	go func() { defer wg.Done(); for i := 0; i < 1000; i++ { n++ } }()
	wg.Wait()
	if n == 0 {
		t.Log("zero")
	}
}
`,
	})
	if out, err := runCheck(t, "race.sh", racy); err == nil {
		t.Errorf("race check passed on racy test: %s", out)
	}

	clean := writeModule(t, map[string]string{
		"ok_test.go": `package fixture

import "testing"

func TestOk(t *testing.T) {
	if testing.Short() {
		t.Log("short")
	}
}
`,
	})
	if out, err := runCheck(t, "race.sh", clean); err != nil {
		t.Errorf("race check failed on clean module: %v: %s", err, out)
	}
}

func TestCoverageEnforcesPerPackageBudget(t *testing.T) {
	// Given a package whose tests cover half its statements, when the
	// coverage check runs against a budget of 80, it fails naming the
	// package; against a budget of 40 it passes.
	files := map[string]string{
		"code.go": `package fixture

func Covered() int { return 1 }

func Uncovered() int { return 2 }
`,
		"code_test.go": `package fixture

import "testing"

func TestCovered(t *testing.T) {
	if Covered() != 1 {
		t.Fatal("unexpected")
	}
}
`,
	}

	strict := writeModule(t, files)
	budget := filepath.Join(t.TempDir(), "budget")
	if err := os.WriteFile(budget, []byte("fixture 80\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runCheck(t, "coverage.sh", strict, budget); err == nil {
		t.Errorf("coverage passed below budget: %s", out)
	} else if !strings.Contains(out, "fixture") {
		t.Errorf("coverage output = %q, want the package named", out)
	}

	lenient := writeModule(t, files)
	budget2 := filepath.Join(t.TempDir(), "budget")
	if err := os.WriteFile(budget2, []byte("fixture 40\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runCheck(t, "coverage.sh", lenient, budget2); err != nil {
		t.Errorf("coverage failed above budget: %v: %s", err, out)
	}
}

func TestArchitectureEnforcesStdlibOnlyDomain(t *testing.T) {
	// Given internal/domain imports a sibling non-stdlib package, when the
	// boundary check runs, it fails naming the violation; a stdlib-only
	// domain passes, and an absent domain passes trivially.
	violating := writeModule(t, map[string]string{
		"internal/domain/domain.go": `package domain

import "fixture/internal/infra"

func Use() string { return infra.Helper() }
`,
		"internal/infra/infra.go": `package infra

func Helper() string { return "h" }
`,
	})
	if out, err := runCheck(t, "architecture.sh", violating); err == nil {
		t.Errorf("architecture passed on domain importing infra: %s", out)
	} else if !strings.Contains(out, "internal/domain") || !strings.Contains(out, "fixture/internal/infra") {
		t.Errorf("architecture output = %q, want the violation named", out)
	}

	clean := writeModule(t, map[string]string{
		"internal/domain/domain.go": `package domain

import "fmt"

func Use() string { return fmt.Sprintf("%s", "ok") }
`,
	})
	if out, err := runCheck(t, "architecture.sh", clean); err != nil {
		t.Errorf("architecture failed on stdlib-only domain: %v: %s", err, out)
	}

	empty := writeModule(t, nil)
	if out, err := runCheck(t, "architecture.sh", empty); err != nil {
		t.Errorf("architecture failed with no domain packages: %v: %s", err, out)
	}
}
