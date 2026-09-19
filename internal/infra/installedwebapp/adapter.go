// Package installedwebapp discovers and launches installed web apps without
// exposing platform metadata to the routing domain. OS metadata is treated as
// untrusted input: scans are bounded, launchers are allowlisted, and no shell
// is ever involved.
package installedwebapp

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	domainapp "github.com/cristianoliveira/brouter/internal/domain/installedwebapp"
)

var ErrStaleLaunchPlan = errors.New("installed web app launch plan is stale")

const (
	maxEntries    = 256
	maxFileBytes  = 128 << 10
	maxDesktopLen = 64 << 10
)

// Environment is the filesystem/process seam for deterministic adapter tests.
type Environment struct {
	GOOS         string
	Home         string
	MacRoots     []string
	LinuxAppDirs []string
	ReadDir      func(string) ([]fs.DirEntry, error)
	ReadFile     func(string) ([]byte, error)
	Lstat        func(string) (fs.FileInfo, error)
	Stat         func(string) (fs.FileInfo, error)
	EvalSymlinks func(string) (string, error)
	LookPath     func(string) (string, error)
	Run          func(string, ...string) error
	ConvertPlist func(string) ([]byte, error)
}

// Result contains one immutable catalog snapshot and fixed, redacted
// diagnostics. Diagnostics never contain paths, URLs, or metadata values.
type Result struct {
	Catalog     domainapp.Catalog
	Diagnostics []string
}

// Adapter is the injected discovery/launch boundary used by the CLI.
type Adapter struct {
	mu       sync.RWMutex
	env      Environment
	cached   Result
	finger   string
	cacheSet bool
	plans    map[string]launchSpec
}

type launchSpec struct {
	kind                  string
	bundleID              string
	argv                  []string
	urlIndex              int
	urlFlag               string
	sourcePath            string
	sourceFingerprint     string
	executablePath        string
	executableFingerprint string
}

// NewAdapter builds an adapter for the supplied platform environment.
func NewAdapter(env Environment) *Adapter {
	env = withFilesystemDefaults(env)
	env = withProcessDefaults(env)
	return &Adapter{env: env, plans: make(map[string]launchSpec)}
}

func withFilesystemDefaults(env Environment) Environment {
	if env.GOOS == "" {
		env.GOOS = runtime.GOOS
	}
	if env.Home == "" {
		env.Home, _ = os.UserHomeDir()
	}
	if env.ReadDir == nil {
		env.ReadDir = os.ReadDir
	}
	if env.ReadFile == nil {
		env.ReadFile = os.ReadFile
	}
	if env.Lstat == nil {
		env.Lstat = os.Lstat
	}
	if env.Stat == nil {
		env.Stat = os.Stat
	}
	if env.EvalSymlinks == nil {
		env.EvalSymlinks = filepath.EvalSymlinks
	}
	return env
}

func withProcessDefaults(env Environment) Environment {
	if env.LookPath == nil {
		env.LookPath = exec.LookPath
	}
	if env.Run == nil {
		env.Run = func(name string, args ...string) error {
			return exec.Command(name, args...).Run()
		}
	}
	if env.ConvertPlist == nil && env.GOOS == "darwin" {
		env.ConvertPlist = func(path string) ([]byte, error) {
			return exec.Command("/usr/bin/plutil", "-convert", "xml1", "-o", "-", path).Output()
		}
	}
	return env
}

// System returns the production adapter. It intentionally supports only the
// currently supported macOS and Linux baselines.
func System() *Adapter { return NewAdapter(Environment{}) }

// Discover returns a cached catalog when bounded metadata fingerprints are
// unchanged. A missing or changed launcher invalidates the snapshot.
func (a *Adapter) Discover() Result {
	a.mu.Lock()
	defer a.mu.Unlock()
	var finger string
	if a.cacheSet {
		finger = a.fingerprint()
		if finger == a.finger {
			return a.cached
		}
	}
	a.plans = make(map[string]launchSpec)
	var entries []domainapp.Entry
	var diagnostics []string
	switch a.env.GOOS {
	case "darwin":
		entries, diagnostics = a.discoverMac()
	case "linux":
		entries, diagnostics = a.discoverLinux()
	}
	catalog, err := domainapp.NewCatalog(entries)
	if err != nil {
		// A malformed individual entry should have been skipped earlier. This
		// fixed diagnostic is a final defense against serving a partial model.
		catalog = domainapp.Catalog{}
		diagnostics = append(diagnostics, "installed-app catalog rejected malformed metadata")
	}
	result := Result{Catalog: catalog, Diagnostics: stableDiagnostics(diagnostics)}
	// The first one-shot discovery deliberately avoids a second full root
	// fingerprint pass. A reused adapter computes the fingerprint before its
	// next scan; the empty initial fingerprint therefore causes one safe
	// refresh before subsequent calls become cache hits.
	a.cached, a.finger, a.cacheSet = result, finger, true
	return result
}

// Launch executes an opaque adapter plan with the clicked URL as data.
