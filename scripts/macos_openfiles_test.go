package scripts

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TASK-0033 warm path: kAEOpenDocuments delivery to a running handler
// reaches OpenDocumentsDelegate's application:openURLs:. This test drives
// that selector directly through a native harness compiled against the
// shim source — deterministic, no LaunchServices, no live install, no
// GUI. Real double-click/Open With delivery through LaunchServices on
// an installed bundle stays UNTESTED here and is recorded as such.

type openFilesHarness struct {
	binary       string
	stubArgvLog  string
	handlerLog   string
	documentsDir string
}

func compileOpenFilesHarness(t *testing.T) *openFilesHarness {
	t.Helper()
	work := t.TempDir()
	harness := filepath.Join(work, "openfiles_selftest")
	arch := runtime.GOARCH
	shimObj := filepath.Join(work, "shim.o")
	selfObj := filepath.Join(work, "selftest.o")
	steps := [][]string{
		{"clang", "-arch", arch, "-fobjc-arc", "-Dmain=unused_shim_main", "-c",
			filepath.Join("..", "native", "macos", "main.m"), "-o", shimObj},
		{"clang", "-arch", arch, "-fobjc-arc", "-c",
			filepath.Join("..", "native", "macos", "openfiles_selftest.m"), "-o", selfObj},
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

	stubArgvLog := filepath.Join(work, "stub-argv.log")
	handlerLog := filepath.Join(work, "handler.log")
	// EmbeddedBrouterPath resolves beside argv[0], mirroring the bundle:
	// the recorder stub takes the embedded binary's place.
	stub := "#!/bin/sh\nfor arg in \"$@\"; do printf '%s\\n' \"$arg\" >> " + stubArgvLog + "\ndone\nexit 0\n"
	if err := os.WriteFile(filepath.Join(work, "brouter"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	documentsDir := filepath.Join(work, "my docs")
	if err := os.MkdirAll(documentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Names avoid decomposable unicode (ü, é): Foundation hands file
	// URLs in NFD while the disk stores NFC, which would make byte
	// comparison flaky. En dash and # are stable and still exercise
	// percent-encoding; decomposable-unicode handling is covered by
	// internal/infra/localfile tests and the cold argv probe.
	for _, name := range []string{"report page #3.html", "deck – translated.pdf"} {
		if err := os.WriteFile(filepath.Join(documentsDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &openFilesHarness{
		binary:       harness,
		stubArgvLog:  stubArgvLog,
		handlerLog:   handlerLog,
		documentsDir: documentsDir,
	}
}

func TestOpenFilesDelegateForwardsWarmDocuments(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the macOS handler documents path is darwin-only")
	}
	h := compileOpenFilesHarness(t)

	cmd := exec.Command(h.binary, "warm", h.documentsDir)
	cmd.Env = append(os.Environ(),
		"STUB_ARGV_LOG="+h.stubArgvLog,
		"BRROUTER_HANDLER_LOG="+h.handlerLog,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("open-files harness failed: %v\n%s", err, out)
	}

	// The delegate forwarded one encoded file URL per document.
	assertForwardedOncePerFixtureDocument(t, h.stubArgvLog, h.documentsDir)

	// Redaction on the warm path: the event is logged, the path is not.
	handlerLog := readFileOrFatal(t, h.handlerLog)
	if !strings.Contains(handlerLog, "forwarding URL event") {
		t.Errorf("handler log = %q, want the redacted event marker", handlerLog)
	}
	if strings.Contains(handlerLog, h.documentsDir) {
		t.Errorf("handler log = %q leaks the documents directory", handlerLog)
	}
}

// countSpawns counts embedded-brouter invocations recorded in a stub
// log (each spawn's argv starts with the literal "open").
func countSpawns(lines []string) int {
	spawns := 0
	for _, line := range lines {
		if line == "open" {
			spawns++
		}
	}
	return spawns
}

// assertForwardedOncePerFixtureDocument checks the stub log: every
// fixture document in dir arrives exactly once as a file URL whose
// decoded path names the on-disk file. The comparison decodes each
// forwarded URL: APFS may spell a name in a different unicode
// normalization than the test created it with, so byte-comparing
// encoded strings would flake while the contract — same document —
// holds.
// decodedFixtureURLs returns the log lines that are file URLs naming
// an on-disk fixture, decoded to filesystem paths.
func decodedFixtureURLs(lines []string, onDisk map[string]bool) []string {
	var found []string
	for _, line := range lines {
		parsed, err := url.Parse(line)
		if err != nil || parsed.Scheme != "file" {
			continue
		}
		// parsed.Path is already percent-decoded by url.Parse.
		if onDisk[parsed.Path] {
			found = append(found, parsed.Path)
		}
	}
	return found
}

func assertForwardedOncePerFixtureDocument(t *testing.T, stubLog, dir string) {
	t.Helper()
	// The whole warm event arrives as ONE batched spawn: brouter
	// preflights every document before any browser starts.
	if spawns := countSpawns(readArgvFile(t, stubLog)); spawns != 1 {
		t.Fatalf("document spawns = %d, want exactly one batched spawn", spawns)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("fixture documents: %v", err)
	}
	onDisk := map[string]bool{}
	for _, entry := range entries {
		onDisk[filepath.Join(dir, entry.Name())] = true
	}

	forwarded := map[string]int{}
	for _, path := range decodedFixtureURLs(readArgvFile(t, stubLog), onDisk) {
		forwarded[path]++
	}
	if len(forwarded) != 2 {
		t.Fatalf("forwarded documents = %v, want both fixture documents", forwarded)
	}
	for path, count := range forwarded {
		if count != 1 {
			t.Errorf("document %q forwarded %d times, want one spawn per document", path, count)
		}
	}
}

func TestOpenFilesRealAppleEventDispatchReachesOpenURLs(t *testing.T) {
	// REAL dispatch evidence: the harness builds a genuine
	// kAEOpenDocuments Apple Event (direct object = typeFileURL list)
	// and dispatches it through NSAppleEventManager's public raw
	// dispatch — the same machinery NSApplication services for the OS.
	// AppKit's installed odoc handler then routes to the delegate's
	// -application:openURLs:. This is not a direct selector call; the
	// only hop outside reach remains the live LaunchServices handoff
	// from an installed bundle (UNTESTED).
	if runtime.GOOS != "darwin" {
		t.Skip("the macOS handler documents path is darwin-only")
	}
	h := compileOpenFilesHarness(t)

	cmd := exec.Command(h.binary, "warm-dispatch", h.documentsDir)
	cmd.Env = append(os.Environ(),
		"STUB_ARGV_LOG="+h.stubArgvLog,
		"BRROUTER_HANDLER_LOG="+h.handlerLog,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("warm-dispatch harness failed: %v\n%s", err, out)
	}

	assertForwardedOncePerFixtureDocument(t, h.stubArgvLog, h.documentsDir)
}
