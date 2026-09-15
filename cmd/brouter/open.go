package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/cristianoliveira/brouter/internal/domain"
	"github.com/cristianoliveira/brouter/internal/infra/config"
	"github.com/cristianoliveira/brouter/internal/infra/launch"
)

// runOpen implements the body of `brouter open URL`: it evaluates the
// URL with the same domain router as explain, resolves the selected
// target to a real browser executable, and launches it with the URL as
// one structured argument. There is no fallback to the system default
// handler and no silent target switch: every failure is reported and
// stops the open. Flag parsing lives in the Cobra command layer
// (cli.go); 	args holds zero or one positional URL.

// errOpenExit signals that a phase helper has already reported the
// failure on stderr and the open must stop with the failure code.
var errOpenExit = errors.New("open: failure already reported")

func runOpen(configPath string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	rawURL, ok := singleInput("open", args, stdin, stderr)
	if !ok {
		return exitUsage
	}

	cfg, router, err := loadRouter(configPath, stderr)
	if err != nil {
		return exitFailure
	}

	decision, handled, err := decideRoute(cfg, router, rawURL, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "lua route: %v\n", err)
		return exitFailure
	}
	if !handled {
		decision, err = evaluateStatic(router, rawURL)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return exitFailure
		}
	}

	target, ok := cfg.Targets[string(decision.Target)]
	if !ok {
		// Config validation guarantees defined targets; this is defense
		// in depth, kept visible like every other failure.
		fmt.Fprintf(stderr, "target %q is not defined in browsers\n", decision.Target)
		return exitFailure
	}

	plan, err := launch.Resolve(target, string(decision.Target), launch.SystemEnv())
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitFailure
	}

	if err := launch.Launch(plan, decision.URL, nil); err != nil {
		fmt.Fprintf(stderr, "open failed: %v\n", err)
		return exitFailure
	}

	fmt.Fprintf(stdout, "launched: %s (%s)\n", decision.Target, plan.Detail)
	return exitSuccess
}

// loadRouter resolves, loads, and validates the configuration, then
// builds the domain router over it. Failures are reported on stderr;
// errOpenExit signals the caller to exit with the failure code.
func loadRouter(configPath string, stderr io.Writer) (*config.Config, *domain.Router, error) {
	path, ok := resolveConfigPath(configPath, stderr)
	if !ok {
		return nil, nil, errOpenExit
	}
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "config invalid:\n%v\n", err)
		return nil, nil, errOpenExit
	}
	router, err := domain.NewRouter(cfg.Rules, cfg.Default)
	if err != nil {
		fmt.Fprintf(stderr, "config invalid:\n%v\n", err)
		return nil, nil, errOpenExit
	}
	return cfg, router, nil
}
