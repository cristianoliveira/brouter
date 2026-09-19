package installedwebapp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMacRootShortcutGetsNarrowCompatibilityScope(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Chrome Apps.localized")
	bundle := filepath.Join(root, "ChatGPT.app")
	writeFile(t, filepath.Join(bundle, "Contents", "Info.plist"), `<?xml version="1.0"?><plist><dict>
<key>CFBundleIdentifier</key><string>com.google.Chrome.app.chatgpt</string>
<key>CFBundleExecutable</key><string>app_mode_loader</string>
<key>CrBundleIdentifier</key><string>com.google.Chrome</string>
<key>CrAppModeShortcutURL</key><string>https://chatgpt.com/</string>
</dict></plist>`)
	writeExecutableFile(t, filepath.Join(bundle, "Contents", "MacOS", "app_mode_loader"))
	a := NewAdapter(Environment{
		GOOS:         "darwin",
		MacRoots:     []string{root},
		LookPath:     func(name string) (string, error) { return "/usr/bin/" + name, nil },
		ReadDir:      os.ReadDir,
		ReadFile:     os.ReadFile,
		Lstat:        testLstat,
		Stat:         testStat,
		EvalSymlinks: testEvalSymlinks,
	})
	entries := a.Discover().Catalog.Entries()
	if len(entries) != 1 || entries[0].Scope.Path != "/" {
		t.Fatalf("entries = %#v, want one root-scoped app", entries)
	}
}

func TestMacNonRootShortcutWithoutScopeIsSkipped(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Chrome Apps.localized")
	bundle := filepath.Join(root, "ChatGPT.app")
	writeFile(t, filepath.Join(bundle, "Contents", "Info.plist"), `<?xml version="1.0"?><plist><dict>
<key>CFBundleIdentifier</key><string>com.google.Chrome.app.chatgpt</string>
<key>CFBundleExecutable</key><string>app_mode_loader</string>
<key>CrBundleIdentifier</key><string>com.google.Chrome</string>
<key>CrAppModeShortcutURL</key><string>https://chatgpt.com/work</string>
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
		t.Fatalf("entries = %d, want non-root shortcut skipped", got)
	}
}
