package installedwebapp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxRejectsHomeAndPathHijackedExecutables(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"home.desktop", "path.desktop"} {
		writeFile(t, filepath.Join(root, name), `[Desktop Entry]
Type=Application
X-WebApp-URL=https://example.test/
X-WebApp-Scope=https://example.test/
Exec=brave --app=https://example.test/
`)
	}
	home := t.TempDir()
	hijack := filepath.Join(home, "bin", "brave")
	writeExecutableFile(t, hijack)
	a := NewAdapter(Environment{
		GOOS:         "linux",
		Home:         home,
		LinuxAppDirs: []string{root},
		ReadDir:      os.ReadDir,
		ReadFile:     os.ReadFile,
		Lstat:        os.Lstat,
		Stat:         os.Stat,
		EvalSymlinks: filepath.EvalSymlinks,
		LookPath:     func(string) (string, error) { return hijack, nil },
	})
	if got := len(a.Discover().Catalog.Entries()); got != 0 {
		t.Fatalf("entries = %d, want PATH/home hijack rejected", got)
	}
}

func TestLinuxRejectsExecutableSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "app.desktop"), `[Desktop Entry]
Type=Application
X-WebApp-URL=https://example.test/
X-WebApp-Scope=https://example.test/
Exec=/usr/bin/brave --app=https://example.test/
`)
	target := filepath.Join(t.TempDir(), "brave")
	writeExecutableFile(t, target)
	a := NewAdapter(Environment{
		GOOS:         "linux",
		LinuxAppDirs: []string{root},
		ReadDir:      os.ReadDir,
		ReadFile:     os.ReadFile,
		Lstat:        os.Lstat,
		Stat:         os.Stat,
		EvalSymlinks: func(string) (string, error) { return target, nil },
	})
	if got := len(a.Discover().Catalog.Entries()); got != 0 {
		t.Fatalf("entries = %d, want symlink/escape rejected", got)
	}
}

func TestLinuxLaunchDoesNotReResolvePATHAfterDiscovery(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "app.desktop"), `[Desktop Entry]
Type=Application
X-WebApp-URL=https://example.test/
X-WebApp-Scope=https://example.test/
Exec=google-chrome --app=https://example.test/
`)
	lookups := 0
	var launched []string
	a := NewAdapter(Environment{
		GOOS:         "linux",
		LinuxAppDirs: []string{root},
		ReadDir:      os.ReadDir,
		ReadFile:     os.ReadFile,
		Lstat:        testLstat,
		Stat:         testStat,
		EvalSymlinks: testEvalSymlinks,
		LookPath: func(name string) (string, error) {
			lookups++
			if lookups == 1 {
				return "/usr/bin/" + name, nil
			}
			return filepath.Join(t.TempDir(), name), nil
		},
		Run: func(name string, args ...string) error {
			launched = append([]string{name}, args...)
			return nil
		},
	})
	entry := a.Discover().Catalog.Entries()[0]
	if err := a.Launch(entry.Launch, "https://example.test/deep"); err != nil {
		t.Fatal(err)
	}
	if lookups != 1 || len(launched) == 0 || launched[0] != "/usr/bin/google-chrome" {
		t.Fatalf("lookups=%d launched=%q, want stored canonical executable", lookups, launched)
	}
}
