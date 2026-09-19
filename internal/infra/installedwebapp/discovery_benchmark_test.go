package installedwebapp

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkDiscoverLinuxNoApps(b *testing.B) {
	root := b.TempDir()
	a := NewAdapter(Environment{
		GOOS:         "linux",
		LinuxAppDirs: []string{root},
		ReadDir:      os.ReadDir,
		ReadFile:     os.ReadFile,
		Lstat:        os.Lstat,
		Stat:         os.Stat,
		LookPath:     func(name string) (string, error) { return "/usr/bin/" + name, nil },
	})
	for i := 0; i < b.N; i++ {
		a.Discover()
	}
}

func BenchmarkDiscoverLinuxCold(b *testing.B) {
	root := b.TempDir()
	writeBenchmarkDesktop(b, root)
	for i := 0; i < b.N; i++ {
		a := NewAdapter(Environment{
			GOOS:         "linux",
			LinuxAppDirs: []string{root},
			ReadDir:      os.ReadDir,
			ReadFile:     os.ReadFile,
			Lstat:        os.Lstat,
			Stat:         os.Stat,
			LookPath:     func(name string) (string, error) { return "/usr/bin/" + name, nil },
		})
		a.Discover()
	}
}

func BenchmarkDiscoverLinuxCached(b *testing.B) {
	root := b.TempDir()
	writeBenchmarkDesktop(b, root)
	a := NewAdapter(Environment{
		GOOS:         "linux",
		LinuxAppDirs: []string{root},
		ReadDir:      os.ReadDir,
		ReadFile:     os.ReadFile,
		Lstat:        os.Lstat,
		Stat:         os.Stat,
		LookPath:     func(name string) (string, error) { return "/usr/bin/" + name, nil },
	})
	a.Discover()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Discover()
	}
}

func writeBenchmarkDesktop(b testing.TB, root string) {
	b.Helper()
	path := filepath.Join(root, "chatgpt.desktop")
	if err := os.WriteFile(path, []byte(`[Desktop Entry]
Type=Application
X-WebApp-URL=https://chatgpt.example/
X-WebApp-Scope=https://chatgpt.example/
Exec=google-chrome --app=https://chatgpt.example/
`), 0o644); err != nil {
		b.Fatal(err)
	}
}
