package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TASK-0023 Recent Activity: a bounded, in-memory, privacy-safe record
// of what the macOS handler observed — receipt, preflight, dispatch
// started/failed — with injectable clock and request IDs, safe
// allowlisted categories, and no downstream success claims. These
// tests drive a native harness compiled against the shim source; a
// second compile step links the shim with its entry point renamed.
// The harness is the seam Kelly's GUI QA will see rendered in the
// Recent Activity submenu.

type activityHarness struct {
	binary string
	stub   string // path where the harness looks for the embedded brouter
}

func compileActivityHarness(t *testing.T) *activityHarness {
	t.Helper()
	work := t.TempDir()
	harness := filepath.Join(work, "activity_selftest")
	arch := runtime.GOARCH
	shimObj := filepath.Join(work, "shim.o")
	selfObj := filepath.Join(work, "selftest.o")
	steps := [][]string{
		{"clang", "-arch", arch, "-fobjc-arc", "-Dmain=unused_shim_main", "-c",
			filepath.Join("..", "native", "macos", "main.m"), "-o", shimObj},
		{"clang", "-arch", arch, "-fobjc-arc", "-c",
			filepath.Join("..", "native", "macos", "activity_selftest.m"), "-o", selfObj},
		{"clang", "-arch", arch, shimObj, selfObj,
			"-framework", "AppKit", "-framework", "Foundation", "-framework", "CoreServices",
			"-o", harness},
	}
	names := []string{"shim object", "harness object", "harness link"}
	for i, step := range steps {
		if out, err := exec.Command(step[0], step[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%s step failed: %v\n%s", names[i], err, out)
		}
	}
	// EmbeddedBrouterPath resolves beside argv[0], mirroring the bundle.
	return &activityHarness{binary: harness, stub: filepath.Join(work, "brouter")}
}

func (h *activityHarness) run(t *testing.T, args []string, env []string) string {
	t.Helper()
	cmd := exec.Command(h.binary, args...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("activity harness failed: %v\n%s", err, out)
	}
	return string(out)
}

func TestActivityHarnessBoundedEvictionAndClear(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Recent Activity is macOS-only")
	}
	h := compileActivityHarness(t)
	out := h.run(t, []string{"unit"}, nil)
	if strings.Contains(out, "FAIL") {
		t.Fatalf("unit assertions failed:\n%s", out)
	}
}

// forwardScenario runs ForwardURLs in the harness under a controlled
// preflight environment and returns the printed activity lines.
func (h *activityHarness) forwardScenario(t *testing.T, stubMode, url string, env []string) string {
	t.Helper()
	switch stubMode {
	case "executable":
		writeStubBrouter(t, h.stub, 0o755)
	case "not-executable":
		// A directory passes the executable-bit preflight but
		// posix_spawn rejects it (EACCES): the only deterministic way
		// to reach a real spawn failure from the harness.
		os.Remove(h.stub)
		if err := os.Mkdir(h.stub, 0o755); err != nil {
			t.Fatal(err)
		}
	case "missing":
		os.Remove(h.stub)
	}
	return h.run(t, []string{"forward", "os-event", url}, env)
}

// writeStubBrouter installs a recorder stand-in for the embedded
// brouter with the requested permission bits.
func writeStubBrouter(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(path, []byte(script), mode); err != nil {
		t.Fatal(err)
	}
}

// Untrusted markers that must never surface in any activity sink.
var privacyMarkers = []string{
	"https://", "secret", "query", "fragment", "credential",
	"/private", "/tmp", "profile-x", "target-y",
}

func assertNoMarkers(t *testing.T, lines string) {
	t.Helper()
	for _, marker := range privacyMarkers {
		if strings.Contains(lines, marker) {
			t.Errorf("activity output contains untrusted marker %q:\n%s", marker, lines)
		}
	}
}

func TestActivityRecordsDispatchStartedWithoutSuccessClaim(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Recent Activity is macOS-only")
	}
	h := compileActivityHarness(t)
	url := "https://example.test/open?query=1&credential=secret#fragment"
	out := h.forwardScenario(t, "executable", url, nil)

	for _, want := range []string{"receipt", "preflight", "dispatch started"} {
		if !strings.Contains(out, want) {
			t.Errorf("activity output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "outcome success") || strings.Contains(out, "delivered") {
		t.Errorf("activity must not claim downstream success:\n%s", out)
	}
	if !strings.Contains(out, "outcome unknown") {
		t.Errorf("dispatch started must state outcome unknown:\n%s", out)
	}
	assertNoMarkers(t, out)
}

func TestActivityRecordsMissingEmbeddedBinaryPreflight(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Recent Activity is macOS-only")
	}
	h := compileActivityHarness(t)
	out := h.forwardScenario(t, "missing",
		"https://example.test/missing", nil)
	if !strings.Contains(out, "missing-embedded-binary") {
		t.Errorf("expected missing-embedded-binary category:\n%s", out)
	}
	if !strings.Contains(out, "preflight failed") {
		t.Errorf("expected preflight failure phrasing:\n%s", out)
	}
	if strings.Contains(out, "dispatch started") {
		t.Errorf("no dispatch attempt may be claimed when preflight fails:\n%s", out)
	}
	if strings.Contains(out, "unavailable-config-directory") {
		t.Errorf("wrong category for this scenario:\n%s", out)
	}
	assertNoMarkers(t, out)
}

func TestActivityRecordsSpawnFailureWithoutSuccessClaim(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Recent Activity is macOS-only")
	}
	h := compileActivityHarness(t)
	out := h.forwardScenario(t, "not-executable",
		"https://example.test/spawnfail", nil)
	if !strings.Contains(out, "spawn-failed") {
		t.Errorf("expected spawn-failed category:\n%s", out)
	}
	if strings.Contains(out, "outcome success") {
		t.Errorf("spawn failure must not be a success claim:\n%s", out)
	}
	assertNoMarkers(t, out)
}

func TestActivityRecordsUnavailableConfigDirectory(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Recent Activity is macOS-only")
	}
	h := compileActivityHarness(t)
	out := h.forwardScenario(t, "executable",
		"https://example.test/noconfig", []string{"HOME=", "XDG_CONFIG_HOME="})
	if !strings.Contains(out, "unavailable-config-directory") {
		t.Errorf("expected unavailable-config-directory category:\n%s", out)
	}
	assertNoMarkers(t, out)
}
