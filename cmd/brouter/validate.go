package main

import (
	"fmt"
	"io"

	"github.com/cristianoliveira/brouter/internal/infra/config"
)

// runValidate implements the body of `brouter validate`: load the
// configuration and report health or every validation problem. It never
// launches anything and never claims a browser is installed. Flag
// parsing lives in the Cobra command layer (cli.go).
func runValidate(configPath string, stdout, stderr io.Writer) int {
	path, ok := resolveConfigPath(configPath, stderr)
	if !ok {
		return exitFailure
	}

	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "config invalid:\n%v\n", err)
		return exitFailure
	}

	if cfg.RouteCommand != nil {
		// Structure was validated by config.Load. The command is never
		// resolved or executed by validate: side-effect-free by design.
		fmt.Fprintf(stdout, "route_command: %q configured (not executed by validate)\n", cfg.RouteCommand[0])
	}
	fmt.Fprintf(stdout, "config OK: targets %d, rules %d, default %q\n",
		len(cfg.Targets), len(cfg.Rules), cfg.Default)
	return exitSuccess
}

// resolveConfigPath honors the explicit -config override; without it the
// documented per-OS location is used. There is no other fallback.
func resolveConfigPath(override string, stderr io.Writer) (string, bool) {
	if override != "" {
		return override, true
	}
	path, err := config.DefaultPath()
	if err != nil {
		fmt.Fprintf(stderr, "cannot determine config path: %v\n", err)
		return "", false
	}
	return path, true
}
