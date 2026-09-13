// Command brouter routes URLs to the right browser based on a
// version-controlled configuration. The open command resolves a URL to
// its configured target and launches that browser directly with the URL
// as one structured argument; validate and explain remain diagnostics.
package main

import (
	"io"
	"os"
)

// Stable exit codes, documented in the usage text and DEVELOPMENT.md.
const (
	exitSuccess = 0
	exitFailure = 1 // invalid config or invalid URL
	exitUsage   = 2 // unknown command or missing/invalid arguments
)

const usage = `Usage: brouter [command]

brouter routes URLs to browsers using a config file you own.

Commands:
  validate            Check the configuration and report problems.
  explain URL         Show which target a URL routes to and why. Reads the
                      URL from stdin when the argument is missing.
  open URL            Route the URL and launch the selected browser with
                      it. Reads the URL from stdin when the argument is
                      missing. Never uses a shell or the system default
                      handler; failures are reported, never silently
                      rerouted.
  help                Show this usage text.

Flags:
  -config path        Use path instead of the default config location.

Exit codes:
  0 success
  1 validation failure (invalid config or invalid URL)
  2 usage error

Run 'brouter help' for this text.
`

// run is the CLI composition root. It owns wiring and process concerns;
// routing decisions live in internal/domain. Parser/wiring is Cobra
// (cli.go); the observable contract is unchanged.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return newCLI(stdin, stdout, stderr).run(args)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
