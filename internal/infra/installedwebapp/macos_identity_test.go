package installedwebapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMacLaunchUsesTheVerifiedPathForDuplicateBundleIDs(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Chrome Apps.localized")
	for _, name := range []string{"first", "second"} {
		bundle := filepath.Join(root, name+".app")
		writeFile(t, filepath.Join(bundle, "Contents", "Info.plist"), `<?xml version="1.0"?><plist><dict>
<key>CFBundleIdentifier</key><string>com.google.Chrome.app.duplicate</string>
<key>CFBundleExecutable</key><string>app_mode_loader</string>
<key>CrBundleIdentifier</key><string>com.google.Chrome</string>
<key>CrAppModeShortcutURL</key><string>https://example.test/</string>
<key>CrAppModeScope</key><string>https://example.test/</string>
</dict></plist>`)
		writeExecutableFile(t, filepath.Join(bundle, "Contents", "MacOS", "app_mode_loader"))
	}
	var calls [][]string
	a := NewAdapter(Environment{
		GOOS:         "darwin",
		MacRoots:     []string{root},
		ReadDir:      os.ReadDir,
		ReadFile:     os.ReadFile,
		Lstat:        testLstat,
		Stat:         testStat,
		EvalSymlinks: testEvalSymlinks,
		LookPath:     func(name string) (string, error) { return "/usr/bin/" + name, nil },
		Run: func(name string, args ...string) error {
			calls = append(calls, append([]string{name}, args...))
			return nil
		},
	})

	entries := a.Discover().Catalog.Entries()
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want duplicate-ID entries retained for deterministic tie-breaking", len(entries))
	}
	if err := a.Launch(entries[0].Launch, "https://example.test/deep"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || len(calls[0]) != 4 || calls[0][1] != "-a" || calls[0][2] != filepath.Join(root, "first.app") || calls[0][3] != "https://example.test/deep" {
		t.Fatalf("launch argv = %q, want verified first.app path and URL", calls)
	}
	if strings.Contains(strings.Join(calls[0], "\x00"), "com.google.Chrome.app.duplicate") {
		t.Fatal("launch used bundle-ID lookup instead of the verified path")
	}
}
