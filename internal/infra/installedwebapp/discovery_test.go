package installedwebapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMacAdapterDiscoversAndSafelyLaunchesGeneratedApp(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "ChatGPT.app")
	plist := filepath.Join(bundle, "Contents", "Info.plist")
	writeFile(t, plist, `<?xml version="1.0"?><plist><dict>
<key>CFBundleIdentifier</key><string>com.google.Chrome.app.chatgpt</string>
<key>CrAppModeShortcutURL</key><string>https://chatgpt.com/</string>
<key>CrAppModeScope</key><string>https://chatgpt.com/</string>
</dict></plist>`)

	var gotName string
	var gotCalls [][]string
	a := NewAdapter(Environment{
		GOOS:     "darwin",
		MacRoots: []string{root},
		LookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil },
		Run: func(name string, args ...string) error {
			gotName = name
			gotCalls = append(gotCalls, append([]string(nil), args...))
			return nil
		},
	})

	result := a.Discover()
	if len(result.Catalog.Entries()) != 1 {
		t.Fatalf("discovered %d apps, want 1 (diagnostics: %v)", len(result.Catalog.Entries()), result.Diagnostics)
	}
	entry := result.Catalog.Entries()[0]
	if entry.ID != "com.google.Chrome.app.chatgpt" {
		t.Fatalf("entry ID = %q", entry.ID)
	}
	for _, rawURL := range []string{"https://chatgpt.com/share/123", "https://chatgpt.com/share/456"} {
		if err := a.Launch(entry.Launch, rawURL); err != nil {
			t.Fatal(err)
		}
	}
	if gotName != "open" || len(gotCalls) != 2 {
		t.Fatalf("launch = %s %q", gotName, gotCalls)
	}
	want := [][]string{
		{"-b", "com.google.Chrome.app.chatgpt", "https://chatgpt.com/share/123"},
		{"-b", "com.google.Chrome.app.chatgpt", "https://chatgpt.com/share/456"},
	}
	if strings.Join(gotCalls[0], "\x00") != strings.Join(want[0], "\x00") || strings.Join(gotCalls[1], "\x00") != strings.Join(want[1], "\x00") {
		t.Fatalf("launch calls = %q, want %q", gotCalls, want)
	}
}

func TestLinuxAdapterRequiresExplicitURLAndRejectsAmbiguousExec(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "chatgpt.desktop"), `[Desktop Entry]
Type=Application
Name=ChatGPT
Exec=google-chrome --app-id=chatgpt
`)
	writeFile(t, filepath.Join(root, "explicit.desktop"), `[Desktop Entry]
Type=Application
Name=ChatGPT
X-WebApp-URL=https://chatgpt.com/
X-WebApp-Scope=https://chatgpt.com/
Exec=google-chrome --app=https://chatgpt.com/
`)
	writeFile(t, filepath.Join(root, "hostile.desktop"), `[Desktop Entry]
Type=Application
Name=Hostile
X-WebApp-URL=https://evil.test/
Exec=sh -c 'google-chrome https://evil.test/'
`)

	a := NewAdapter(Environment{
		GOOS:         "linux",
		LinuxAppDirs: []string{root},
		LookPath:     func(name string) (string, error) { return "/usr/bin/" + name, nil },
		ReadDir:      os.ReadDir,
		ReadFile:     os.ReadFile,
		Lstat:        os.Lstat,
		Stat:         os.Stat,
	})
	result := a.Discover()
	entries := result.Catalog.Entries()
	if len(entries) != 1 || entries[0].ID != "explicit.desktop" {
		t.Fatalf("entries = %#v diagnostics=%v, want only explicit.desktop", entries, result.Diagnostics)
	}
}

func TestAdapterRefreshRemovesMovedOrUninstalledEntries(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "chatgpt.desktop")
	writeFile(t, path, `[Desktop Entry]
Type=Application
X-WebApp-URL=https://chatgpt.com/
X-WebApp-Scope=https://chatgpt.com/
Exec=google-chrome --app=https://chatgpt.com/
`)
	a := NewAdapter(Environment{
		GOOS:         "linux",
		LinuxAppDirs: []string{root},
		LookPath:     func(name string) (string, error) { return "/usr/bin/" + name, nil },
		ReadDir:      os.ReadDir,
		ReadFile:     os.ReadFile,
		Lstat:        os.Lstat,
		Stat:         os.Stat,
	})
	if got := len(a.Discover().Catalog.Entries()); got != 1 {
		t.Fatalf("initial entries = %d, want 1", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := len(a.Discover().Catalog.Entries()); got != 0 {
		t.Fatalf("after removal entries = %d, want 0", got)
	}
}

func TestAdapterLaunchPreservesClickedURLWithoutShell(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "chatgpt.desktop"), `[Desktop Entry]
Type=Application
X-WebApp-URL=https://chatgpt.com/
X-WebApp-Scope=https://chatgpt.com/
Exec=google-chrome --app=https://chatgpt.com/
`)
	var args []string
	a := NewAdapter(Environment{
		GOOS:         "linux",
		LinuxAppDirs: []string{root},
		LookPath:     func(name string) (string, error) { return "/usr/bin/" + name, nil },
		ReadDir:      os.ReadDir,
		ReadFile:     os.ReadFile,
		Lstat:        os.Lstat,
		Stat:         os.Stat,
		Run: func(name string, got ...string) error {
			args = append([]string{name}, got...)
			return nil
		},
	})
	entry := a.Discover().Catalog.Entries()[0]
	if err := a.Launch(entry.Launch, "https://chatgpt.com/x?a=1&b=2"); err != nil {
		t.Fatal(err)
	}
	want := []string{"/usr/bin/google-chrome", "--app=https://chatgpt.com/x?a=1&b=2"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("argv = %q, want %q", args, want)
	}
}

func TestAdapterUsesDomainCatalogForMatching(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "app.desktop"), `[Desktop Entry]
Type=Application
X-WebApp-URL=https://example.test/work/
X-WebApp-Scope=https://example.test/work/
Exec=google-chrome --app=https://example.test/work/
`)
	a := NewAdapter(Environment{
		GOOS:         "linux",
		LinuxAppDirs: []string{root},
		LookPath:     func(name string) (string, error) { return "/usr/bin/" + name, nil },
		ReadDir:      os.ReadDir,
		ReadFile:     os.ReadFile,
		Lstat:        os.Lstat,
		Stat:         os.Stat,
	})
	entry, ok, err := a.Discover().Catalog.Match("https://example.test/work/page")
	if err != nil || !ok || entry.ID != "app.desktop" {
		t.Fatalf("Match() = (%#v, %v, %v)", entry, ok, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
