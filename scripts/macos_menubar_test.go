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
