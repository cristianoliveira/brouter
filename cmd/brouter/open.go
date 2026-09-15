package main

import (
	"context"
	"fmt"
	"io"

	"github.com/cristianoliveira/brouter/internal/domain"
	"github.com/cristianoliveira/brouter/internal/infra/config"
	"github.com/cristianoliveira/brouter/internal/infra/launch"
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

	path, ok := resolveConfigPath(configPath, stderr)
	if !ok {
		return exitFailure
	}
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "config invalid:\n%v\n", err)
		return exitFailure
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
