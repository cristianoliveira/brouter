package launch

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/cristianoliveira/brouter/internal/infra/config"
)

// fakeEnv builds an Env whose filesystem is the given set of existing
// paths. LookPath resolves bare names against them; Stat checks them
// verbatim. Paths outside existing are missing.
func fakeEnv(goos string, home string, existing []string, selfExecutable string) *Env {
	isThere := func(path string) bool {
		for _, candidate := range existing {
			if candidate == path {
				return true
			}
		}
		return false
	}
	return &Env{
		GOOS: goos,
		Home: func() (string, error) {
			if home == "" {
				return "", errors.New("no home in test")
			}
			return home, nil
		},
		LookPath: func(file string) (string, error) {
			if strings.Contains(file, "/") {
				return "", errors.New("LookPath expects a bare name in tests")
			}
			for _, candidate := range existing {
				if strings.HasSuffix(candidate, "/"+file) {
					return candidate, nil
				}
			}
			return "", errors.New("not found: " + file)
		},
		Stat: func(path string) (fs.FileInfo, error) {
			if isThere(path) {
				return fakeFileInfo{}, nil
			}
			return nil, fs.ErrNotExist
		},
		Executable: func() (string, error) {
			if selfExecutable == "" {
				return "", errors.New("no self executable in test")
			}
			return selfExecutable, nil
		},
		EvalSymlinks: func(path string) (string, error) { return path, nil },
	}
}

func knownTarget(browser string) config.TargetDefinition {
	return config.TargetDefinition{Kind: "known", Browser: browser}
}

func TestResolveBraveOnLinuxViaPathLookup(t *testing.T) {
	// Given a linux environment where brave is on PATH, when resolving a
	// known brave target, the PATH hit wins.
	env := fakeEnv("linux", "/home/u", []string{"/usr/bin/brave"}, "/bin/brouter")

	plan, err := Resolve(knownTarget("brave"), "work", env)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if plan.Executable != "/usr/bin/brave" {
		t.Errorf("executable = %q, want /usr/bin/brave", plan.Executable)
	}
	if !strings.Contains(plan.Detail, "PATH") {
		t.Errorf("detail = %q, want it to mention PATH lookup", plan.Detail)
	}
}

func TestResolveBraveOnDarwinUsesApplicationBundle(t *testing.T) {
	// Given a darwin environment with the system Brave bundle, when
	// resolving a known brave target, the bundle executable wins.
	bundle := "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"
	env := fakeEnv("darwin", "/home/u", []string{bundle}, "/Applications/brouter.app/brouter")

	plan, err := Resolve(knownTarget("brave"), "work", env)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if plan.Executable != bundle {
		t.Errorf("executable = %q, want %q", plan.Executable, bundle)
	}
}

func TestResolveBraveOnDarwinFallsBackToUserApplications(t *testing.T) {
	// Given the system bundle is absent but a user-local one exists, when
	// resolving, the user-local bundle wins.
	userBundle := "/home/u/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"
	env := fakeEnv("darwin", "/home/u", []string{userBundle}, "/bin/brouter")

	plan, err := Resolve(knownTarget("brave"), "work", env)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if plan.Executable != userBundle {
		t.Errorf("executable = %q, want %q", plan.Executable, userBundle)
	}
}

func TestResolveChromeOnLinuxPrefersCanonicalNames(t *testing.T) {
	// Given both google-chrome-stable and google-chrome exist, when
	// resolving, the unversioned google-chrome wins.
	env := fakeEnv("linux", "/home/u", []string{
		"/usr/bin/google-chrome-stable",
		"/usr/bin/google-chrome",
	}, "/bin/brouter")

	plan, err := Resolve(knownTarget("chrome"), "work", env)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if plan.Executable != "/usr/bin/google-chrome" {
		t.Errorf("executable = %q, want /usr/bin/google-chrome", plan.Executable)
	}
}

func TestResolveKnownTargetWithoutAnyCandidateFailsNamingCandidates(t *testing.T) {
	// Given nothing exists, when resolving a known target, the error names
	// the target and every candidate tried, and suggests an executable.
	env := fakeEnv("linux", "/home/u", nil, "/bin/brouter")

	_, err := Resolve(knownTarget("brave"), "work", env)
	if err == nil {
		t.Fatal("Resolve succeeded, want failure")
	}
	for _, want := range []string{`"work"`, "brave", "executable target"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestResolveExecutableLiteralPath(t *testing.T) {
	// Given an executable target with a literal path that exists, when
	// resolving, that path is the plan executable.
	env := fakeEnv("linux", "/home/u", []string{"/opt/browsers/brave"}, "/bin/brouter")

	plan, err := Resolve(config.TargetDefinition{Kind: "executable", Command: "/opt/browsers/brave"}, "work", env)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if plan.Executable != "/opt/browsers/brave" {
		t.Errorf("executable = %q, want /opt/browsers/brave", plan.Executable)
	}
}

func TestResolveExecutableBareNameUsesPathLookup(t *testing.T) {
	// Given an executable target that is a bare command name, when
	// resolving, PATH lookup finds it.
	env := fakeEnv("linux", "/home/u", []string{"/usr/local/bin/my-browser"}, "/bin/brouter")

	plan, err := Resolve(config.TargetDefinition{Kind: "executable", Command: "my-browser"}, "work", env)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if plan.Executable != "/usr/local/bin/my-browser" {
		t.Errorf("executable = %q, want /usr/local/bin/my-browser", plan.Executable)
	}
}

func TestResolveExecutableMissingPathFails(t *testing.T) {
	// Given an executable target whose path does not exist, when
	// resolving, the failure names the target and the missing path.
	env := fakeEnv("linux", "/home/u", nil, "/bin/brouter")

	_, err := Resolve(config.TargetDefinition{Kind: "executable", Command: "/opt/browsers/gone"}, "work", env)
	if err == nil {
		t.Fatal("Resolve succeeded, want failure")
	}
	if !strings.Contains(err.Error(), "/opt/browsers/gone") || !strings.Contains(err.Error(), `"work"`) {
		t.Errorf("error %q does not name the missing path and target", err)
	}
}

func TestResolveProfileTargetReportsUnsupported(t *testing.T) {
	// Given a known target with a profile, when resolving, the profile is
	// reported as unsupported instead of being silently ignored.
	env := fakeEnv("linux", "/home/u", []string{"/usr/bin/brave"}, "/bin/brouter")
	def := config.TargetDefinition{Kind: "known", Browser: "brave", Profile: "work", ProfileSet: true}

	_, err := Resolve(def, "work", env)
	if err == nil {
		t.Fatal("Resolve succeeded, want unsupported-profile failure")
	}
	if !strings.Contains(err.Error(), "profile") || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error %q does not report profile as unsupported", err)
	}
}

func TestResolveKnownBrowserOutsideBraveAndChromeReportsUnsupported(t *testing.T) {
	// Given a known firefox target, when resolving, the failure says this
	// build resolves only brave and chrome and suggests an executable.
	env := fakeEnv("linux", "/home/u", []string{"/usr/bin/firefox"}, "/bin/brouter")

	_, err := Resolve(knownTarget("firefox"), "work", env)
	if err == nil {
		t.Fatal("Resolve succeeded, want unsupported-browser failure")
	}
	for _, want := range []string{"firefox", "brave", "chrome", "executable"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestResolveRejectsRouterAsExecutableTarget(t *testing.T) {
	// Given an executable target that resolves to the running brouter
	// binary, when resolving, the recursion is rejected.
	env := fakeEnv("linux", "/home/u", []string{"/usr/local/bin/brouter"}, "/usr/local/bin/brouter")

	_, err := Resolve(config.TargetDefinition{Kind: "executable", Command: "/usr/local/bin/brouter"}, "self", env)
	if err == nil {
		t.Fatal("Resolve succeeded, want router-as-target rejection")
	}
	if !strings.Contains(err.Error(), "brouter itself") {
		t.Errorf("error %q does not name the recursion", err)
	}
}

func TestResolveRejectsRouterIdentityThroughSymlinks(t *testing.T) {
	// Given a target path that is a symlink to the running brouter
	// binary, when symlinks are evaluated, the identity match is rejected.
	env := fakeEnv("linux", "/home/u", []string{"/usr/bin/brave"}, "/proc/self/exe/brouter")
	env.EvalSymlinks = func(path string) (string, error) {
		if path == "/usr/bin/brave" {
			return "/opt/brouter/brouter", nil
		}
		if path == "/proc/self/exe/brouter" {
			return "/opt/brouter/brouter", nil
		}
		return path, nil
	}

	_, err := Resolve(knownTarget("brave"), "work", env)
	if err == nil {
		t.Fatal("Resolve succeeded, want symlinked identity rejection")
	}
	if !strings.Contains(err.Error(), "brouter itself") {
		t.Errorf("error %q does not name the recursion", err)
	}
}

func TestResolveNeverReturnsEmptyExecutable(t *testing.T) {
	// Whatever the environment, a successful Resolve never yields an
	// empty executable path.
	env := fakeEnv("linux", "/home/u", []string{"/usr/bin/brave"}, "/bin/brouter")

	plan, err := Resolve(knownTarget("brave"), "work", env)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if plan.Executable == "" {
		t.Error("executable is empty")
	}
}

// fakeFileInfo satisfies fs.FileInfo for existence checks; only IsDir
// matters to the resolver, which accepts any existing regular path.
type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "fake" }
func (fakeFileInfo) Size() int64        { return 1 }
func (fakeFileInfo) Mode() fs.FileMode  { return 0o755 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }
