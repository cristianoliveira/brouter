// Package launch turns a validated routing target into a safe browser
// process: explicit target resolution, structured argv, visible
// failures. It never uses a shell, never delegates to the system default
// handler, and never silently falls back to another target.
package launch

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Env abstracts everything Resolve and Launch touch outside the
// process: PATH lookup, filesystem existence, the user home, the running
// executable, and symlink identity. Production code wires SystemEnv at
// the CLI composition root; tests inject fakes so behavior is
// deterministic on every host.
type Env struct {
	GOOS         string
	LookPath     func(file string) (string, error)
	Stat         func(path string) (fs.FileInfo, error)
	Home         func() (string, error)
	Executable   func() (string, error)
	EvalSymlinks func(path string) (string, error)
}

// SystemEnv returns the real process environment.
func SystemEnv() *Env {
	return &Env{
		GOOS: runtime.GOOS,
		LookPath: func(file string) (string, error) {
			return exec.LookPath(file)
		},
		Stat: func(path string) (fs.FileInfo, error) {
			return os.Stat(path)
		},
		Home:         os.UserHomeDir,
		Executable:   os.Executable,
		EvalSymlinks: filepath.EvalSymlinks,
	}
}

// exists reports whether path is present on the fake or real filesystem.
func (e *Env) exists(path string) bool {
	_, err := e.Stat(path)
	return err == nil
}

// lookup resolves a bare command name through PATH.
func (e *Env) lookup(name string) (string, bool) {
	path, err := e.LookPath(name)
	if err != nil {
		return "", false
	}
	return path, true
}
