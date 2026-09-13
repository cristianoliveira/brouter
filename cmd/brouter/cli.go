package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// flagError marks a parser error and the command that produced it, so
// run() can re-emit it in the pre-migration std-flag wording and usage
// block.
type flagError struct {
	command string
	message string
}

func (e *flagError) Error() string { return e.message }

// cli owns the Cobra command tree and the exit code produced by one
// invocation. Construction is injectable: tests and main hand in the
// streams; nothing here touches os.Stdout/os.Stderr directly.
type cli struct {
	root     *cobra.Command
	stdout   io.Writer
	stderr   io.Writer
	exitCode int
}

// newCLI builds the command tree. The parser migrates to Cobra/pflag,
// but every observable behavior of the std-flag CLI is preserved:
// usage text, exit codes 0/1/2, stdout/stderr routing, the historical
// single-dash -config spelling, and legacy help/error channels.
func newCLI(stdin io.Reader, stdout, stderr io.Writer) *cli {
	c := &cli{stdout: stdout, stderr: stderr, exitCode: exitSuccess}

	root := &cobra.Command{
		Use:           "brouter",
		Short:         "Route URLs to browsers using a config file you own.",
		SilenceUsage:  true, // usage is printed by our help func, on our terms
		SilenceErrors: true, // errors are printed by run(), on our terms
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true, // the migration adds no features
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(c.stdout, usage)
			return nil
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetIn(stdin)
	// Parser errors carry their command so run() can re-emit them with
	// the pre-migration std-flag wording and usage block.
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return &flagError{command: cmd.Name(), message: err.Error()}
	})
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		// One usage text for every help surface, byte-identical to the
		// pre-migration CLI, always on stdout with exit 0.
		fmt.Fprint(cmd.OutOrStdout(), usage)
	})

	// The historical help command ignores extra arguments and always
	// prints the usage text with exit 0 — including unknown flags.
	root.SetHelpCommand(&cobra.Command{
		Use:                "help",
		Short:              "Show this usage text.",
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(c.stdout, usage)
			return nil
		},
	})

	var (
		validateConfig string
		explainConfig  string
		openConfig     string
	)

	validate := &cobra.Command{
		Use:   "validate",
		Short: "Check the configuration and report problems.",
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
	// The pre-migration std-flag parser stopped at the first positional
	// argument; flags after it were positionals. Keep that contract.
	validate.Flags().SetInterspersed(false)
	root.AddCommand(validate)

	explain := &cobra.Command{
		Use:   "explain",
		Short: "Show which target a URL routes to and why.",
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
	// The pre-migration std-flag parser stopped at the first positional
	// argument; flags after it were positionals. Keep that contract.
	explain.Flags().SetInterspersed(false)
	root.AddCommand(explain)

	open := &cobra.Command{
		Use:   "open",
		Short: "Route the URL and launch the selected browser with it.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("open takes exactly one URL argument")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.exitWith(runOpen(openConfig, args, stdin, stdout, stderr))
		},
	}
	open.Flags().StringVar(&openConfig, "config", "",
		"path to config.toml (default: per-OS config location)")
	// The pre-migration std-flag parser stopped at the first positional
	// argument; flags after it were positionals. Keep that contract.
	open.Flags().SetInterspersed(false)
	root.AddCommand(open)

	c.root = root
	return c
}

// run executes one invocation and returns its exit code.
func (c *cli) run(args []string) int {
	args = normalizeLegacyArgs(args)

	// Unknown leading tokens keep the legacy guidance instead of Cobra's
	// default wording: same channel, same exit code, same two lines. The
	// help-flag spellings still route into the tree for the usage text.
	if len(args) > 0 {
		first := args[0]
		if first != "--help" && first != "-h" && !isKnownCommand(first) {
			fmt.Fprintf(c.stderr, "brouter: unknown command %q\nRun 'brouter help' for usage.\n", first)
			return exitUsage
		}
	}

	// Subcommand help keeps the std-flag channel: guidance on stderr
	// with the usage exit code, exactly as before the migration.
	if c.legacySubcommandHelp(args) {
		return exitUsage
	}

	c.root.SetArgs(args)
	if err := c.root.Execute(); err != nil {
		var ferr *flagError
		if errors.As(err, &ferr) {
			c.emitLegacyFlagError(ferr)
			return exitUsage
		}
		fmt.Fprintf(c.stderr, "%v\n", err)
		return exitUsage
	}
	return c.exitCode
}

// emitLegacyFlagError re-renders a parser error with the pre-migration
// std-flag wording and usage block: `flag provided but not defined: -x`
// and `flag needs an argument: -config` followed by the command's flag
// summary, all on stderr.
func (c *cli) emitLegacyFlagError(ferr *flagError) {
	message := ferr.message
	switch {
	case strings.Contains(message, "unknown shorthand flag: "):
		// pflag: unknown shorthand flag: 'b' in -bogus; the legacy parser
		// reported the full remaining cluster: -bogus
		if marker := "in "; strings.Contains(message, marker) {
			cluster := message[strings.LastIndex(message, marker)+len(marker):]
			message = fmt.Sprintf("flag provided but not defined: %s", cluster)
		}
	case strings.Contains(message, "unknown flag: "):
		name := strings.TrimPrefix(message, "unknown flag: ")
		message = fmt.Sprintf("flag provided but not defined: -%s", strings.TrimLeft(name, "-"))
	case strings.Contains(message, "flag needs an argument:"):
		message = strings.Replace(message, "--", "-", 1)
	}
	fmt.Fprintf(c.stderr, "%s\n", message)
	fmt.Fprintf(c.stderr, "Usage of %s:\n", ferr.command)
	fmt.Fprintf(c.stderr, "  -config string\n    \tpath to config.toml (default: per-OS config location)\n")
}

// exitWith records a business exit code (0/1) from a command body. Only
// parser-level errors flow through Execute as errors, so they alone map
// to the usage code.
func (c *cli) exitWith(code int) error {
	c.exitCode = code
	return nil
}

// normalizeLegacyArgs rewrites the historical single-dash -config
// spelling to pflag's --config. Without this shim pflag would parse
// -config as a shorthand cluster and reject invocations that worked
// before the migration.
func normalizeLegacyArgs(args []string) []string {
	normalized := make([]string, 0, len(args))
	for _, arg := range args {
		switch {
		case arg == "-config":
			normalized = append(normalized, "--config")
		case strings.HasPrefix(arg, "-config="):
			normalized = append(normalized, "--config="+strings.TrimPrefix(arg, "-config="))
		default:
			normalized = append(normalized, arg)
		}
	}
	return normalized
}

func isKnownCommand(name string) bool {
	switch name {
	case "validate", "explain", "open", "help":
		return true
	}
	return false
}

// legacySubcommandHelp reports whether args invoke a subcommand's help
// flag and, if so, prints the std-flag-style guidance to stderr: the
// pre-migration channel for subcommand help.
func (c *cli) legacySubcommandHelp(args []string) bool {
	if len(args) == 0 || args[0] == "help" || !isKnownCommand(args[0]) {
		return false
	}
	if !leadingHelpFlag(args) {
		return false
	}
	fmt.Fprintf(c.stderr, "Usage of %s:\n", args[0])
	fmt.Fprintf(c.stderr, "  -config string\n    \tpath to config.toml (default: per-OS config location)\n")
	return true
}

// leadingHelpFlag reports whether a help flag (-h or --help) appears
// among the leading flags of a subcommand invocation, before any
// positional argument and honoring that -config consumes a value.
func leadingHelpFlag(args []string) bool {
	for index := 1; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "-h" || arg == "--help":
			return true
		case arg == "-config" || arg == "--config":
			index++ // the flag consumes the next element as its value
		case strings.HasPrefix(arg, "-config="), strings.HasPrefix(arg, "--config="):
			// inline value, nothing to skip
		case strings.HasPrefix(arg, "-"):
			// other flags: unknown ones are rejected later by pflag
		default:
			return false // positional reached; a later -h is positional
		}
	}
	return false
}
