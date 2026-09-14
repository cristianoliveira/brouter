package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestMacosHandlerBundleMetadata locks the packaged bundle's identity:
// the URL schemes it accepts, its background-app posture, and its icon
// posture (no icon file is declared; the menu icon is a runtime SF
// Symbol, so no .icns asset is required or shipped). Regression for
// the TASK-0024 packaged verification: any accidental Info.plist
// change fails here before packaging.
func TestMacosHandlerBundleMetadata(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("bundle metadata is macOS-only")
	}
	plist := filepath.Join("..", "native", "macos", "Info.plist")
	out, err := exec.Command("plutil", "-convert", "json", "-o", "-", plist).Output()
	if err != nil {
		t.Fatalf("plutil failed: %v", err)
	}
	meta := string(out)

	for _, want := range []string{
		`"CFBundleIdentifier":"com.cristianoliveira.brouter.handler"`,
		`"CFBundleExecutable":"BrouterHandler"`,
		`"CFBundlePackageType":"APPL"`,
		`"LSUIElement":true`,
		`"http"`, `"https"`,
	} {
		if !strings.Contains(meta, want) {
			t.Errorf("bundle metadata missing %s", want)
		}
	}
	if strings.Contains(meta, "CFBundleIconFile") {
		t.Errorf("CFBundleIconFile declared but no icon asset ships; either ship the asset or drop the key")
	}

	// The declared document types must stay handler-ranked Alternate:
	// the bundle may open HTML when picked explicitly, but must never
	// claim default handlers by metadata alone.
	if !strings.Contains(meta, `"LSHandlerRank":"Alternate"`) {
		t.Errorf("document types must keep LSHandlerRank Alternate")
	}
}

// TestMacosHandlerSourceShipsNoDefaultsOrRegistrationWrites guards the
// TASK-0024 safety contract at the source level: the shim never
// touches user defaults, never self-registers with LaunchServices, and
// never shells out.
func TestMacosHandlerSourceShipsNoDefaultsOrRegistrationWrites(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("source-level guard is macOS-only")
	}
	source, err := os.ReadFile(filepath.Join("..", "native", "macos", "main.m"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"NSUserDefaults", "lsregister", "LSSetDefaultHandlerForURLScheme", "system(", "popen("} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("shim source contains forbidden %q", forbidden)
		}
	}
}

// TestMacosHandlerShipsMenuIconAsset locks the user-selected menu icon
// (TASK-0026 candidate C) into the bundle: 1x/2x template PNGs at the
// expected pixel dimensions in Contents/Resources.
func TestMacosHandlerShipsMenuIconAsset(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("menu icon asset is macOS-only")
	}
	resources := filepath.Join("..", "dist", "BrouterHandler.app", "Contents", "Resources")
	for name, want := range map[string]int{
		"menu-icon.png":    16,
		"menu-icon@2x.png": 32,
	} {
		png := filepath.Join(resources, name)
		out, err := exec.Command("sips", "-g", "pixelWidth", "-g", "pixelHeight", png).Output()
		if err != nil {
			t.Errorf("%s missing or unreadable: %v", name, err)
			continue
		}
		if !strings.Contains(string(out), fmt.Sprintf("pixelWidth: %d", want)) ||
			!strings.Contains(string(out), fmt.Sprintf("pixelHeight: %d", want)) {
			t.Errorf("%s pixel size = %s, want %dx%d", name, strings.TrimSpace(string(out)), want, want)
		}
	}
}
