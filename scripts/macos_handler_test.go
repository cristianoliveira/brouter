package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The macOS URL handler is built as a minimal app bundle: a thin native
// shim that receives OS URL events and forwards them, as structured
// arguments, to the brouter binary embedded beside it. These tests run
// only on darwin; on other platforms the build is asserted to skip.

func TestMacosHandlerBuildSkipsOffPlatform(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("darwin-only negative check")
	}
	// On non-darwin hosts the build script refuses to run: the bundle is
	// a macOS artifact and the pinned Linux CI must not pretend to build it.
	cmd := exec.Command("bash", "build-macos-handler.sh")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("build script succeeded on %s, want refusal", runtime.GOOS)
	}
	if !strings.Contains(string(out), "macOS") {
		t.Errorf("refusal %q does not name macOS", out)
	}
}

func TestMacosHandlerBundleIsReproducibleAndForwardsURLs(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the macOS handler bundle can only be built and verified on darwin")
	}

	dist := filepath.Join("..", "dist")
	app := filepath.Join(dist, "BrouterHandler.app")

	// Given the build script, when it runs twice, the bundle is rebuilt
	// reproducibly (same inputs, fresh outputs, no stale state).
	for round := 1; round <= 2; round++ {
		build := exec.Command("bash", "build-macos-handler.sh")
		build.Dir = "." // script location; it changes into the repo root itself
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build round %d failed: %v\n%s", round, err, out)
		}
	}

	// The bundle structure is complete: a valid Info.plist declaring the
	// routed schemes, the native shim, and the embedded brouter binary.
	for _, path := range []string{
		"Contents/Info.plist",
		"Contents/MacOS/BrouterHandler",
		"Contents/MacOS/brouter",
	} {
		full := filepath.Join(app, path)
		info, err := os.Stat(full)
		if err != nil {
			t.Fatalf("bundle artifact %s missing: %v", path, err)
		}
		if strings.HasSuffix(path, "BrouterHandler") || strings.HasSuffix(path, "brouter") {
			if info.Mode()&0o111 == 0 {
				t.Errorf("%s is not executable", path)
			}
		}
	}

	if out, err := exec.Command("plutil", "-lint", filepath.Join(app, "Contents/Info.plist")).CombinedOutput(); err != nil {
		t.Fatalf("Info.plist invalid: %v\n%s", err, out)
	}
	schemes, err := os.ReadFile(filepath.Join(app, "Contents/Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	for _, scheme := range []string{"http", "https"} {
		if !strings.Contains(string(schemes), "<string>"+scheme+"</string>") {
			t.Errorf("Info.plist does not declare the %s scheme", scheme)
		}
	}

	// The forwarding path is verified with a stub brouter: the shim hands
	// the URL over as structured arguments — [open, --config, cfg, URL] —
	// never through a shell, and records the event in its diagnostics log.
	stubDir := t.TempDir()
	stub := filepath.Join(stubDir, "brouter")
	stubLog := filepath.Join(stubDir, "argv.log")
	script := "#!/bin/sh\nfor arg in \"$@\"; do printf '%s\\n' \"$arg\" >> " + stubLog + "\ndone\nexit 0\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	testApp := filepath.Join(stubDir, "BrouterHandler.app")
	if err := os.MkdirAll(filepath.Join(testApp, "Contents/MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"BrouterHandler", "brouter", "Info.plist"} {
		src := filepath.Join(app, "Contents/MacOS", name)
		if name == "Info.plist" {
			src = filepath.Join(app, "Contents/Info.plist")
		}
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if name != "Info.plist" {
			mode = 0o755
		}
		if err := os.WriteFile(filepath.Join(testApp, "Contents/MacOS", name), data, mode); err != nil {
			t.Fatal(err)
		}
	}
	// The stub replaces the embedded brouter so the forwarded arguments
	// can be asserted without launching a real browser.
	if err := os.WriteFile(filepath.Join(testApp, "Contents/MacOS/brouter"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	handlerBin := filepath.Join(testApp, "Contents/MacOS/BrouterHandler")
	rawURL := "https://company.example/page?q=1&tag=handler#top"
	run := exec.Command(handlerBin, rawURL)
	run.Env = append(os.Environ(), "BRROUTER_HANDLER_LOG="+filepath.Join(stubDir, "handler.log"))
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("handler run failed: %v\n%s", err, out)
	}

	// The shim spawns the embedded brouter asynchronously and exits; the
	// child writes the log afterwards. Poll briefly instead of racing.
	argv := func() []string {
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, err := os.Stat(stubLog); err == nil {
				return readArgvFile(t, stubLog)
			}
			if time.Now().After(deadline) {
				t.Fatalf("stub argv log never appeared at %s", stubLog)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()
	want := []string{"open", "--config", configPathForHandlerTest(), rawURL}
	if len(argv) != len(want) {
		t.Fatalf("stub argv = %q, want %q", argv, want)
	}
	for i := range want {
		if i == 2 {
			// The config path is expanded to an absolute location under
			// the user home; only its shape is asserted here.
			if !strings.HasSuffix(argv[i], "brouter/config.toml") {
				t.Errorf("config arg = %q, want the user config path", argv[i])
			}
			if !strings.HasPrefix(argv[i], "/") {
				t.Errorf("config arg = %q, want an absolute path", argv[i])
			}
			continue
		}
		if argv[i] != want[i] {
			t.Errorf("argv[%d] = %q, want %q", i, argv[i], want[i])
		}
	}

	handlerLog := filepath.Join(stubDir, "handler.log")
	data, err := os.ReadFile(handlerLog)
	if err != nil || !strings.Contains(string(data), rawURL) {
		t.Errorf("handler log = %q (%v), want the URL recorded for diagnostics", string(data), err)
	}
}

func readArgvFile(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

func configPathForHandlerTest() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Application Support", "brouter", "config.toml")
}
