package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/cristianoliveira/brouter/internal/infra/config"
)

// runValidate implements `brouter validate`: load the configuration and
// report health or every validation problem. It never launches anything
// and never claims a browser is installed.
func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "path to config.toml (default: per-OS config location)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "validate takes no positional arguments; got %q\n", fs.Arg(0))
		return exitUsage
	}

	path, ok := resolveConfigPath(*configPath, stderr)
	if !ok {
		return exitFailure
	}

	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "config invalid:\n%v\n", err)
		return exitFailure
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
