package installedwebapp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckedInCrossPlatformFixtures(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "installedwebapp")
	for _, tc := range []struct {
		name string
		goos string
		dir  string
		want string
	}{
		{name: "macOS", goos: "darwin", dir: filepath.Join(root, "macos"), want: "com.google.Chrome.app.chatgpt-fixture"},
		{name: "Linux", goos: "linux", dir: filepath.Join(root, "linux"), want: "chatgpt-fixture"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := NewAdapter(Environment{
				GOOS:         tc.goos,
				MacRoots:     []string{tc.dir},
				LinuxAppDirs: []string{tc.dir},
				ReadDir:      os.ReadDir,
				ReadFile:     os.ReadFile,
				Lstat:        os.Lstat,
				Stat:         os.Stat,
				LookPath:     func(name string) (string, error) { return "/usr/bin/" + name, nil },
			})
			entries := a.Discover().Catalog.Entries()
			if len(entries) != 1 || entries[0].ID != tc.want {
				t.Fatalf("entries = %#v, want %q", entries, tc.want)
			}
		})
	}
}
