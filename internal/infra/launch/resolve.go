package launch

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cristianoliveira/brouter/internal/infra/config"
)

// Plan is the resolved launch plan for one configured target: an
// executable path plus fixed base arguments that precede the URL.
// Args stays empty until profiles arrive (TASK-0012); it exists so the
// launch contract does not change then.
type Plan struct {
	Executable string
	Args       []string
	Detail     string
}

// candidate is one place a known browser may live: a bare command name
// to look up on PATH, or a literal path to stat.
type candidate struct {
	lookPath bool
	value    string
}

// Resolve computes the launch plan for one configured target. It never
// consults the system default handler: known browsers resolve to real
// browser executables, executable targets resolve to the configured
// path. Failures name the target and every candidate tried; there is no
// fallback to another target.
func Resolve(def config.TargetDefinition, name string, env *Env) (Plan, error) {
	if def.ProfileSet {
		return Plan{}, fmt.Errorf("target %q: profile %q is not supported yet (profiles arrive in a later task); remove it or drop the target", name, def.Profile)
	}

	var (
		plan Plan
		err  error
	)
	switch def.Kind {
	case "executable":
		plan, err = resolveExecutable(def.Command, env)
	case "known":
		plan, err = resolveKnown(def.Browser, env)
	default:
		err = fmt.Errorf("unknown target kind %q", def.Kind)
	}
	if err != nil {
		return Plan{}, fmt.Errorf("target %q: %w", name, err)
	}

	if err := rejectRouterTarget(plan.Executable, env); err != nil {
		return Plan{}, fmt.Errorf("target %q: %w", name, err)
	}
	return plan, nil
}

func resolveExecutable(command string, env *Env) (Plan, error) {
	if command == "" {
		return Plan{}, fmt.Errorf("executable target is empty")
	}

	resolved, detail, ok := "", "", false
	if strings.ContainsRune(command, '/') {
		if env.exists(command) {
			resolved, detail, ok = command, "explicit executable path", true
		}
	} else if path, hit := env.lookup(command); hit {
		resolved, detail, ok = path, fmt.Sprintf("%q found on PATH", command), true
	}
	if !ok {
		return Plan{}, fmt.Errorf("executable %q does not exist; fix the target command", command)
	}
	return Plan{Executable: resolved, Detail: detail}, nil
}

func resolveKnown(browser string, env *Env) (Plan, error) {
	candidates, ok := knownCandidates(browser, env)
	if !ok {
		return Plan{}, fmt.Errorf(
			"known browser %q has no resolved launcher in this build (brave and chrome are supported; use an executable target for anything else)",
			browser)
	}

	tried := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		var (
			path  string
			found bool
		)
		if candidate.lookPath {
			path, found = env.lookup(candidate.value)
			tried = append(tried, candidate.value)
		} else {
			path, found = candidate.value, env.exists(candidate.value)
			tried = append(tried, candidate.value)
		}
		if found {
			source := fmt.Sprintf("installed at %s", path)
			if candidate.lookPath {
				source = fmt.Sprintf("%q found on PATH", candidate.value)
			}
			return Plan{Executable: path, Detail: fmt.Sprintf("known %s browser: %s", browser, source)}, nil
		}
	}

	return Plan{}, fmt.Errorf(
		"known browser %q was not found (tried %s); install it or use an executable target",
		browser, strings.Join(tried, ", "))
}

// knownCandidates lists where a browser is installed, evidence-first:
// the NixOS spike recorded a stable profile path (/run/current-system/sw)
// and warned against persisting /nix/store build paths; the macOS path
// follows the documented bundle layout. Bare names resolve through PATH.
func knownCandidates(browser string, env *Env) ([]candidate, bool) {
	switch browser {
	case "brave":
		switch env.GOOS {
		case "darwin":
			candidates := []candidate{
				{value: "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"},
			}
			if home, err := env.Home(); err == nil {
				candidates = append(candidates,
					candidate{value: filepath.Join(home, "Applications/Brave Browser.app/Contents/MacOS/Brave Browser")})
			}
			return candidates, true
		default:
			return []candidate{
				{lookPath: true, value: "brave"},
				{lookPath: true, value: "brave-browser"},
				{value: "/run/current-system/sw/bin/brave"},
				{value: "/usr/bin/brave"},
			}, true
		}
	case "chrome":
		switch env.GOOS {
		case "darwin":
			candidates := []candidate{
				{value: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},
			}
			if home, err := env.Home(); err == nil {
				candidates = append(candidates,
					candidate{value: filepath.Join(home, "Applications/Google Chrome.app/Contents/MacOS/Google Chrome")})
			}
			return candidates, true
		default:
			return []candidate{
				{lookPath: true, value: "google-chrome"},
				{lookPath: true, value: "google-chrome-stable"},
				{value: "/usr/bin/google-chrome"},
				{value: "/opt/google/chrome/google-chrome"},
			}, true
		}
	default:
		return nil, false
	}
}

// rejectRouterTarget refuses plans whose executable is brouter itself,
// compared by resolved identity (symlinks evaluated on both sides), so a
// misconfigured target can never make the router launch the router.
// Recursion through arbitrary wrapper scripts cannot be detected here;
// wrapper targets must launch a browser directly (documented contract).
func rejectRouterTarget(resolved string, env *Env) error {
	self, err := env.Executable()
	if err != nil {
		return fmt.Errorf("cannot verify the target is not brouter itself: %w", err)
	}

	identity := func(path string) string {
		if real, evalErr := env.EvalSymlinks(path); evalErr == nil {
			return real
		}
		return path
	}

	if identity(resolved) == identity(self) {
		return fmt.Errorf(
			"target resolves to brouter itself (%s); routing to the router would recurse — point the target at a browser",
			resolved)
	}
	return nil
}
