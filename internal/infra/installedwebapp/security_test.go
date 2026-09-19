package installedwebapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOversizedMetadataAndSymlinkEntriesAreSkipped(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "oversized.desktop"), strings.Repeat("x", maxDesktopLen+1))
	writeFile(t, filepath.Join(root, "safe.desktop"), `[Desktop Entry]
Type=Application
X-WebApp-URL=https://safe.example/
X-WebApp-Scope=https://safe.example/
Exec=google-chrome --app=https://safe.example/
`)
	if err := os.Symlink(filepath.Join(root, "safe.desktop"), filepath.Join(root, "symlink.desktop")); err != nil {
		t.Fatal(err)
	}
	a := NewAdapter(Environment{
		GOOS:         "linux",
		LinuxAppDirs: []string{root},
		ReadDir:      os.ReadDir,
		ReadFile:     os.ReadFile,
		Lstat:        testLstat,
		Stat:         testStat,
		EvalSymlinks: testEvalSymlinks,
		LookPath:     func(name string) (string, error) { return "/usr/bin/" + name, nil },
	})
	entries := a.Discover().Catalog.Entries()
	if len(entries) != 1 || entries[0].ID != "safe.desktop" {
		t.Fatalf("entries = %#v, want only safe.desktop", entries)
	}
}

func TestMacBundleRequiresKnownChromiumIdentity(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Chrome Apps.localized")
	writeFile(t, filepath.Join(root, "arbitrary.app", "Contents", "Info.plist"), `<?xml version="1.0"?><plist><dict>
<key>CFBundleIdentifier</key><string>com.example.arbitrary</string>
<key>CrAppModeShortcutURL</key><string>https://example.test/</string>
</dict></plist>`)
	a := NewAdapter(Environment{
		GOOS:         "darwin",
		MacRoots:     []string{root},
		ReadDir:      os.ReadDir,
		ReadFile:     os.ReadFile,
		Lstat:        testLstat,
		Stat:         testStat,
		EvalSymlinks: testEvalSymlinks,
	})
	if got := len(a.Discover().Catalog.Entries()); got != 0 {
		t.Fatalf("entries = %d, want arbitrary bundle skipped", got)
	}
}
