package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/cristianoliveira/brouter/internal/infra/localfile"
)

// cli owns the Cobra command tree and the exit code produced by one
// invocation. Construction is injectable: tests and main hand in the
// streams; nothing here touches os.Stdout/os.Stderr directly.
//
// Parser behavior is Cobra-native: the long flag is --config, help and
// error surfaces are Cobra's own, and unknown flags or commands fail
// through the standard parser path. Business failures (invalid config,
// invalid URL, launch problems) keep the historical exit codes.
type cli struct {
	root     *cobra.Command
	stdout   io.Writer
	stderr   io.Writer
	exitCode int
}

// newCLI builds the command tree.
func newCLI(stdin io.Reader, stdout, stderr io.Writer) *cli {
	c := &cli{stdout: stdout, stderr: stderr, exitCode: exitSuccess}

	root := &cobra.Command{
		Use:          "brouter",
		Short:        "Route URLs to browsers using a config file you own.",
		SilenceUsage: true,         // errors print the message; a usage wall helps no one
		Args:         cobra.NoArgs, // unknown commands error instead of falling through to root
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true, // the migration adds no features
		},
		Long: `brouter routes URLs to browsers using a config file you own.

Exit codes:
  0 success
  1 validation failure (invalid config, invalid URL, or launch failure)
  2 usage error

URLs may contain sensitive data; brouter does not log them.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetIn(stdin)

	var (
		validateConfig string
		explainConfig  string
		openConfig     string
	)

	validate := &cobra.Command{
		Use:   "validate",
		Short: "Check the configuration and report problems.",
		Long: `Check the configuration and report problems.

It never launches anything and never claims a browser is installed.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("validate takes no positional arguments; got %q", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.exitWith(runValidate(validateConfig, stdout, stderr))
		},
	}
	validate.Flags().StringVar(&validateConfig, "config", "",
		"path to config.toml (default: per-OS config location)")
	root.AddCommand(validate)

	explain := &cobra.Command{
		Use:   "explain URL",
		Short: "Show which target a URL routes to and why.",
		Long: `Show which target a URL routes to and why. Reads the URL from
stdin when the argument is missing. It never launches a browser and
never writes to disk.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("explain takes exactly one URL argument")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.exitWith(runExplain(explainConfig, args, stdin, stdout, stderr))
		},
	}
	explain.Flags().StringVar(&explainConfig, "config", "",
		"path to config.toml (default: per-OS config location)")
	root.AddCommand(explain)

	open := &cobra.Command{
		Use:   "open URL",
		Short: "Route the URL and launch the selected browser with it.",
		Long: `Route the URL and launch the selected browser with it. Reads the
URL from stdin when the argument is missing. Never uses a shell or the
system default handler; failures are reported, never silently rerouted.`,
		Args: func(cmd *cobra.Command, args []string) error {
			// More than one argument is valid only when every argument is
			// a local document batch (TASK-0033); anything else — URLs
			// especially — stays single-input. The mixed case is rejected
			// here: the OS never mixes documents and URLs in one event.
			if len(args) <= 1 {
				return nil
			}
			for _, arg := range args {
				if !localfile.IsLocalFile(arg) {
					return fmt.Errorf("open takes exactly one URL argument, or multiple local documents")
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.exitWith(runOpen(openConfig, args, stdin, stdout, stderr))
		},
	}
	open.Flags().StringVar(&openConfig, "config", "",
		"path to config.toml (default: per-OS config location)")
	root.AddCommand(open)

	c.root = root
	return c
}

// run executes one invocation and returns its exit code. Cobra renders
// parser errors and help itself (stderr/stdout respectively); any error
// reaching this point is usage-class and maps to the usage exit code.
// Business failures are recorded by the command bodies via exitWith.
func (c *cli) run(args []string) int {
	c.root.SetArgs(args)
	if err := c.root.Execute(); err != nil {
		return exitUsage
	}
	return c.exitCode
}

// exitWith records a business exit code (0/1) from a command body. Only
// parser-level errors flow through Execute as errors, so they alone map
// to the usage exit code.
func (c *cli) exitWith(code int) error {
	c.exitCode = code
	return nil
}
