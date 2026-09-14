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

func skipUnlessDarwin(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("the macOS handler bundle can only be built and verified on darwin")
	}
}

func TestMacosHandlerBuildRefusesOffPlatform(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("darwin-only negative check")
	}
	// On non-darwin hosts the build script refuses to run: the bundle is
	// a macOS artifact and the pinned Linux CI must not pretend to build it.
	build := exec.Command("bash", "build-macos-handler.sh")
	build.Dir = "."
	out, err := build.CombinedOutput()
	if err == nil {
		t.Fatalf("build script succeeded on %s, want refusal", runtime.GOOS)
	}
	if !strings.Contains(string(out), "macOS") {
		t.Errorf("refusal %q does not name macOS", out)
	}
}

func TestMacosHandlerBundleIsReproducible(t *testing.T) {
	skipUnlessDarwin(t)

	app := filepath.Join("..", "dist", "BrouterHandler.app")

	// Given the build script, when it runs twice, the bundle is rebuilt
	// reproducibly: same inputs, fresh outputs, no stale state.
	for round := 1; round <= 2; round++ {
		buildBundle(t)
	}

	// The bundle structure is complete: a valid Info.plist declaring the
	// routed schemes, the native shim, and the embedded brouter binary.
	assertBundleStructure(t, app)
}

func TestMacosHandlerForwardsURLsThroughStubBrouter(t *testing.T) {
	skipUnlessDarwin(t)

	app := filepath.Join("..", "dist", "BrouterHandler.app")

	// The build under test must exist; the structure test builds it.
	if _, err := os.Stat(filepath.Join(app, "Contents/MacOS/BrouterHandler")); err != nil {
		t.Fatalf("bundle not built: %v", err)
	}

	stubDir := t.TempDir()
	stubScript := stubBrouterScript(filepath.Join(stubDir, "argv.log"))
	testApp := assembleTestBundle(t, app, stubDir, stubScript)

	handlerBin := filepath.Join(testApp, "Contents/MacOS/BrouterHandler")
	rawURL := "https://company.example/page?q=1&tag=handler#top"
	run := exec.Command(handlerBin, rawURL)
	run.Env = append(os.Environ(), "BRROUTER_HANDLER_LOG="+filepath.Join(stubDir, "handler.log"))
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("handler run failed: %v\n%s", err, out)
	}

	// The shim spawns the embedded brouter asynchronously and exits; the
	// child writes the log afterwards. Poll briefly instead of racing.
	argv := readArgvFile(t, waitForFile(t, filepath.Join(stubDir, "argv.log")))
	assertForwardedArgv(t, argv, rawURL)

	handlerLog := readFileOrFatal(t, filepath.Join(stubDir, "handler.log"))
	if !strings.Contains(handlerLog, rawURL) {
		t.Errorf("handler log = %q, want the URL recorded for diagnostics", handlerLog)
	}
}

func buildBundle(t *testing.T) {
	t.Helper()
	build := exec.Command("bash", "build-macos-handler.sh")
	build.Dir = "." // script location; it changes into the repo root itself
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
}

// assertBundleStructure checks the reproducible bundle: all artifacts
// present and executable where required, the Info.plist valid, and both
// routed schemes declared.
func assertBundleStructure(t *testing.T, app string) {
	t.Helper()
	artifacts := map[string]bool{
		"Contents/Info.plist":           false,
		"Contents/MacOS/BrouterHandler": true,
		"Contents/MacOS/brouter":        true,
	}
	for path, mustBeExecutable := range artifacts {
		full := filepath.Join(app, path)
		info, err := os.Stat(full)
		if err != nil {
			t.Fatalf("bundle artifact %s missing: %v", path, err)
		}
		if mustBeExecutable && info.Mode()&0o111 == 0 {
			t.Errorf("%s is not executable", path)
		}
	}

	plist := filepath.Join(app, "Contents/Info.plist")
	if out, err := exec.Command("plutil", "-lint", plist).CombinedOutput(); err != nil {
		t.Fatalf("Info.plist invalid: %v\n%s", err, out)
	}
	declared := readFileOrFatal(t, plist)
	for _, scheme := range []string{"http", "https"} {
		if !strings.Contains(declared, "<string>"+scheme+"</string>") {
			t.Errorf("Info.plist does not declare the %s scheme", scheme)
		}
	}

	// Browser-dropdown eligibility: LaunchServices populates the Default
	// Web Browser dropdown from apps claiming html/xhtml document types
	// alongside the http/https schemes — it derives the browser-category
	// UTI itself (proven against Finicky on macOS 26; a scheme-only
	// bundle was absent from the dropdown — PR18 defect, and a merged
	// dict also declaring the category UTI directly was dropped by
	// LaunchServices entirely, yielding no claims).
	for _, contentType := range []string{"public.html", "public.xhtml"} {
		if !strings.Contains(declared, "<string>"+contentType+"</string>") {
			t.Errorf("Info.plist does not claim %s for dropdown eligibility", contentType)
		}
	}
	assertDocumentTypesDeclareViewerRole(t, declared)
}

func assertDocumentTypesDeclareViewerRole(t *testing.T, declared string) {
	t.Helper()
	docTypes := declared[strings.Index(declared, "CFBundleDocumentTypes"):]
	if !strings.Contains(docTypes[:strings.Index(docTypes, "</array>")], "<string>Viewer</string>") {
		t.Errorf("Info.plist document types do not declare a Viewer role")
	}
}

// assembleTestBundle copies the built bundle into stubDir and swaps the
// embedded brouter for a stub that records its arguments, so forwarding
// can be asserted without launching a real browser. It returns the
// assembled bundle path.
func assembleTestBundle(t *testing.T, app, stubDir, stubScript string) string {
	t.Helper()
	testApp := filepath.Join(stubDir, "BrouterHandler.app")
	if err := os.MkdirAll(filepath.Join(testApp, "Contents/MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	copies := map[string]string{
		"BrouterHandler": filepath.Join(app, "Contents/MacOS/BrouterHandler"),
		"brouter":        filepath.Join(app, "Contents/MacOS/brouter"),
	}
	for name, src := range copies {
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(testApp, "Contents/MacOS", name)
		if err := os.WriteFile(dst, data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(testApp, "Contents/MacOS/brouter"), []byte(stubScript), 0o755); err != nil {
		t.Fatal(err)
	}
	return testApp
}

func testAppMacOSDir(stubDir string) string {
	return filepath.Join(stubDir, "BrouterHandler.app", "Contents/MacOS")
}

func stubBrouterScript(logPath string) string {
	return "#!/bin/sh\nfor arg in \"$@\"; do printf '%s\\n' \"$arg\" >> " + logPath + "\ndone\nexit 0\n"
}

func waitForFile(t *testing.T, path string) string {
	t.Helper()
	// The shim spawns the embedded brouter asynchronously and exits; the
	// child writes logs afterwards. Poll briefly instead of racing.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return path
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected file never appeared at %s", path)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// assertForwardedArgv locks the forwarding contract: the shim hands the
// URL to the embedded brouter as [open, --config, <abs user config>,
// <url>] — structured arguments, never a shell. The config path is
// HOME-expanded, so only its shape is asserted.
func assertForwardedArgv(t *testing.T, argv []string, rawURL string) {
	t.Helper()
	want := []string{"open", "--config", userConfigPath(), rawURL}
	if len(argv) != len(want) {
		t.Fatalf("stub argv = %q, want %q", argv, want)
	}
	for i := range want {
		if i == 2 {
			assertUserConfigPath(t, argv[i])
			continue
		}
		if argv[i] != want[i] {
			t.Errorf("argv[%d] = %q, want %q", i, argv[i], want[i])
		}
	}
}

func userConfigPath() string {
	// The standardized Unix location: $HOME/.config on macOS too
	// (TASK-0020). os.UserHomeDir reads $HOME, matching the shim.
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "brouter", "config.toml")
}

func assertUserConfigPath(t *testing.T, got string) {
	t.Helper()
	want := userConfigPath()
	if got != want {
		t.Errorf("config arg = %q, want %q", got, want)
	}
}

func TestMacosHandlerBundleTargetsArm64(t *testing.T) {
	skipUnlessDarwin(t)

	app := filepath.Join("..", "dist", "BrouterHandler.app")

	// The declared target is arm64: the build script pins the Go target
	// and the clang -arch flag so a translated build shell cannot produce
	// a mixed-architecture bundle. Both executables must be arm64 only.
	for _, binary := range []string{"Contents/MacOS/BrouterHandler", "Contents/MacOS/brouter"} {
		full := filepath.Join(app, binary)
		out, err := exec.Command("lipo", "-archs", full).CombinedOutput()
		if err != nil {
			t.Fatalf("lipo failed for %s: %v\n%s", binary, err, out)
		}
		if archs := strings.Fields(string(out)); len(archs) != 1 || archs[0] != "arm64" {
			t.Errorf("%s architectures = %q, want exactly [arm64]", binary, string(out))
		}
	}
}

func TestMacosHandlerShimResolvesConfigPerContract(t *testing.T) {
	skipUnlessDarwin(t)

	// The built shim spawns the embedded real brouter; with no config
	// file at the resolved location brouter's own error names the path
	// it was given, so the handler log doubles as the resolution
	// evidence. Each case pins one contract clause.
	cases := []struct {
		name      string
		home      string
		xdg       string
		wantInLog string
	}{
		{"absolute xdg wins", "IGNORED", "/tmp/bh-xdg-abs", "/tmp/bh-xdg-abs/brouter/config.toml"},
		{"relative xdg falls back", "/tmp/bh-home-rel", "relative/xdg", "/tmp/bh-home-rel/.config/brouter/config.toml"},
		{"unset xdg uses home", "/tmp/bh-home-unset", "", "/tmp/bh-home-unset/.config/brouter/config.toml"},
		{"missing home is visible", "", "", "cannot determine the user config directory"},
	}

	app := filepath.Join("..", "dist", "BrouterHandler.app")
	if _, err := os.Stat(filepath.Join(app, "Contents/MacOS/BrouterHandler")); err != nil {
		t.Fatalf("bundle not built: %v", err)
	}
	handlerBin := filepath.Join(app, "Contents/MacOS/BrouterHandler")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logFile := filepath.Join(t.TempDir(), "handler.log")
			env := []string{
				"PATH=/usr/bin:/bin",
				"BRROUTER_HANDLER_LOG=" + logFile,
			}
			if tc.home == "IGNORED" {
				env = append(env, "HOME=/unused-home")
			} else {
				env = append(env, "HOME="+tc.home)
			}
			env = append(env, "XDG_CONFIG_HOME="+tc.xdg)

			run := exec.Command(handlerBin, "https://contract.example/")
			run.Env = env
			_ = run.Run() // brouter fails on the missing config; that is the evidence

			// The shim exits after spawning; the child writes its
			// diagnostics afterwards. Poll briefly instead of racing.
			log := ""
			deadline := time.Now().Add(5 * time.Second)
			for {
				data, err := os.ReadFile(logFile)
				if err == nil && strings.Contains(string(data), tc.wantInLog) {
					log = string(data)
					break
				}
				if time.Now().After(deadline) {
					data := []byte("<missing>")
					if data2, err := os.ReadFile(logFile); err == nil {
						data = data2
					}
					t.Fatalf("handler log does not mention %q:\n%s", tc.wantInLog, data)
				}
				time.Sleep(50 * time.Millisecond)
			}
			_ = log
		})
	}
}
