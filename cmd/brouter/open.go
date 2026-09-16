package main

import (
	"context"
	"fmt"
	"io"

	"github.com/cristianoliveira/brouter/internal/domain"
	"github.com/cristianoliveira/brouter/internal/infra/config"
	"github.com/cristianoliveira/brouter/internal/infra/launch"
	"github.com/cristianoliveira/brouter/internal/infra/localfile"
	"github.com/cristianoliveira/brouter/internal/infra/routecmd"
)

// runOpen implements the body of `brouter open URL`: it evaluates the
// URL with the same domain router as explain, resolves the selected
// target to a real browser executable, and launches it with the URL as
// one structured argument. There is no fallback to the system default
// handler and no silent target switch: every failure is reported and
// stops the open. Flag parsing lives in the Cobra command layer
// (cli.go); args holds zero or one positional URL.
func runOpen(configPath string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	rawURL, ok := singleInput("open", args, stdin, stderr)
	if !ok {
		return exitUsage
	}

	path, cfg, code, ok := loadConfigForOpen(configPath, stderr)
	if !ok {
		return code
	}

	// Local documents bypass all URL routing (TASK-0033): they go
	// straight to the default configured browser as encoded file URLs.
	// Every other input keeps the exact routing order below.
	if code, handled := launchLocalDocument(cfg, rawURL, stderr, stdout); handled {
		return code
	}

	router, err := domain.NewRouter(cfg.Rules, cfg.Default)
	if err != nil {
		fmt.Fprintf(stderr, "config invalid:\n%v\n", err)
		return exitFailure
	}

	// Opt-in external routing command: command-first, per plan
	// TASK-0029. Only the exact @default line defers to the static
	// rules; every command failure is visible and stops the open —
	// never a silent fallback.
	if cfg.RouteCommand != nil {
		res, decided, cerr := decideByCommand(cfg, path, rawURL, stderr)
		if cerr != nil {
			return exitFailure
		}
		if decided {
			return launchCommandTarget(cfg, res, rawURL, stderr, stdout)
		}
		// @default — fall through to the static router.
	}

	decision, err := router.Evaluate(rawURL)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitFailure
	}

	target, ok := cfg.Targets[string(decision.Target)]
	if !ok {
		// Config validation guarantees defined targets; this is defense
		// in depth, kept visible like every other failure.
		fmt.Fprintf(stderr, "target %q is not defined in browsers\n", decision.Target)
		return exitFailure
	}

	return launchTarget(target, string(decision.Target), rawURL, stderr, stdout)
}

// launchCommandTarget validates a command decision against the config
// snapshot and launches it. An unknown target is a visible failure —
// it never silently falls back to static rules.
func launchCommandTarget(cfg *config.Config, res routecmd.Result, rawURL string, stderr, stdout io.Writer) int {
	def, defined := cfg.Targets[res.Route]
	if !defined {
		fmt.Fprintf(stderr, "route_command: unknown target (not defined in browsers)\n")
		return exitFailure
	}
	return launchTarget(def, res.Route, rawURL, stderr, stdout)
}

// loadConfigForOpen resolves the config path and loads the config.
// Any failure is already reported; ok is false and code carries the
// process exit status.
func loadConfigForOpen(configPath string, stderr io.Writer) (string, *config.Config, int, bool) {
	path, ok := resolveConfigPath(configPath, stderr)
	if !ok {
		return "", nil, exitFailure, false
	}
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "config invalid:\n%v\n", err)
		return "", nil, exitFailure, false
	}
	return path, cfg, exitSuccess, true
}

// launchTarget resolves and launches one already-validated target.
func launchTarget(def config.TargetDefinition, name, rawURL string, stderr, stdout io.Writer) int {
	plan, err := launch.Resolve(def, name, launch.SystemEnv())
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitFailure
	}

	if err := launch.Launch(plan, rawURL, nil); err != nil {
		fmt.Fprintf(stderr, "open failed: %v\n", err)
		return exitFailure
	}

	fmt.Fprintf(stdout, "launched: %s (%s)\n", name, plan.Detail)
	return exitSuccess
}

// launchLocalDocument opens a validated local document in the default
// configured browser. Local files stay outside both the route_command
// protocol and the static rules (TASK-0033): a document has no URL
// identity to route on, and there is no recursion into the OS default
// handler — the browser is resolved and launched here or the open
// fails visibly with a redacted diagnostic.
func launchLocalDocument(cfg *config.Config, rawInput string, stderr, stdout io.Writer) (int, bool) {
	if !localfile.IsLocalFile(rawInput) {
		return 0, false
	}

	fileURL, err := localfile.Resolve(rawInput)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitFailure, true
	}

	name := string(cfg.Default)
	def, defined := cfg.Targets[name]
	if !defined {
		fmt.Fprintf(stderr, "target %q is not defined in browsers\n", name)
		return exitFailure, true
	}

	plan, err := launch.Resolve(def, name, launch.SystemEnv())
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitFailure, true
	}
	if err := launch.LaunchFile(plan, fileURL, nil); err != nil {
		fmt.Fprintf(stderr, "open failed: %v\n", err)
		return exitFailure, true
	}

	fmt.Fprintf(stdout, "launched: %s (%s)\n", name, plan.Detail)
	return exitSuccess, true
}

// decideByCommand consults the external routing command. decided is
// true when the command answered with a final target; a null decision
// defers (decided=false, err=nil); any error is terminal and already
// reported on stderr.
func decideByCommand(cfg *config.Config, configPath, rawURL string, stderr io.Writer) (routecmd.Result, bool, error) {
	commander := &routecmd.Commander{Command: cfg.RouteCommand, ConfigPath: configPath}
	res, err := commander.Decide(context.Background(), rawURL)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return routecmd.Result{}, false, err
	}
	return res, !res.Defer, nil
}
