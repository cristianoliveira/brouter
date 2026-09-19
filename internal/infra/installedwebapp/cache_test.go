package installedwebapp

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestAdapterConcurrentDiscoveryReturnsWholeSnapshots(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "chatgpt.desktop"), `[Desktop Entry]
Type=Application
X-WebApp-URL=https://chatgpt.example/
X-WebApp-Scope=https://chatgpt.example/
Exec=google-chrome --app=https://chatgpt.example/
`)
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
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := len(a.Discover().Catalog.Entries()); got != 1 {
				t.Errorf("entries = %d, want 1", got)
			}
		}()
	}
	wg.Wait()
}
