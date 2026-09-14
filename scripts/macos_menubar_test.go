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

// TASK-0022 first slice: the running macOS handler shows a menu-bar
// presence (one retained NSStatusItem on the existing NSApplication
// loop) with an accessible name, a tooltip, and a Quit item. These
// tests assert the user-visible seams that are observable without UI
// scripting: the presence-installed diagnostics marker on the isolated
// log (the same seam the real menu uses) and the AppKit linkage that
// makes the status item possible. Actual icon visibility and menu
// interaction are verified as GUI evidence on Apple Silicon.

func TestMacosHandlerLinksAppKitForPresence(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("menu bar presence is macOS-only")
	}
	shim := filepath.Join("..", "dist", "BrouterHandler.app", "Contents", "MacOS", "BrouterHandler")
	out, err := exec.Command("otool", "-L", shim).CombinedOutput()
	if err != nil {
		t.Fatalf("otool failed: %v", err)
	}
	if !strings.Contains(string(out), "AppKit") {
		t.Errorf("shim does not link AppKit; status item cannot be created:\n%s", out)
	}
}

func TestMacosHandlerInstallsMenuBarPresence(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("menu bar presence is macOS-only")
	}
	distBundle := filepath.Join("..", "dist", "BrouterHandler.app")
	if _, err := os.Stat(filepath.Join(distBundle, "Contents/MacOS/BrouterHandler")); err != nil {
		t.Fatalf("bundle not built: %v", err)
	}

	env := newOpenEventEnv(t, distBundle)

	// Cold OS-open startup must initialize the presence.
	env.openApp(t, "https://events.example/presence")

	deadline := time.Now().Add(30 * time.Second)
	for {
		// The log appears only after the LS-launched app starts writing;
		// absence is a retryable state, not a failure.
		if data, err := os.ReadFile(env.logPath); err == nil &&
			strings.Contains(string(data), "menu bar presence installed") {
			break
		}
		if time.Now().After(deadline) {
			log := "(absent)"
			if data, err := os.ReadFile(env.logPath); err == nil {
				log = string(data)
			}
			t.Fatalf("menu bar presence never reported installed; log = %q", log)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Quit is the graceful NSApplication terminate path — never a kill.
	env.quitGracefully(t)
}

// TestMacosQuitMenuActionDispatchesTermination is the focused seam for
// the Quit item's wiring: a native harness builds the presence menu,
// asserts the Quit item's target responds to its action (the defect QA
// found: action terminate: on a controller implementing only quit:),
// then dispatches it exactly as AppKit does. A correct wiring ends the
// process with exit 0 via terminate:.
func TestMacosQuitMenuActionDispatchesTermination(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("menu bar Quit wiring is macOS-only")
	}

	harness := compileQuitMenuHarness(t)
	cmd := exec.Command(harness)
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code := exitErr.ExitCode()
				if code == 3 {
					t.Fatalf("Quit item wiring is broken (target does not respond to its action)")
				}
				if code == 4 {
					t.Fatalf("Quit action returned without terminating the process")
				}
				t.Fatalf("harness exited %d", code)
			}
			t.Fatalf("harness failed: %v", err)
		}
	case <-time.After(30 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		t.Fatalf("harness hung for 30s")
	}
}

// compileQuitMenuHarness builds the native Quit-wiring self-test: the
// shim object with its entry point renamed plus the harness object.
func compileQuitMenuHarness(t *testing.T) string {
	t.Helper()
	work := t.TempDir()
	harness := filepath.Join(work, "quit_selftest")
	arch := runtime.GOARCH
	shimObj := filepath.Join(work, "shim.o")
	selfObj := filepath.Join(work, "selftest.o")
	steps := [][]string{
		{"clang", "-arch", arch, "-fobjc-arc", "-Dmain=unused_shim_main", "-c",
			filepath.Join("..", "native", "macos", "main.m"), "-o", shimObj},
		{"clang", "-arch", arch, "-fobjc-arc", "-c",
			filepath.Join("..", "native", "macos", "quit_menu_selftest.m"), "-o", selfObj},
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
	return harness
}
