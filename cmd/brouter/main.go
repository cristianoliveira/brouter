// Command brouter routes URLs to the right browser based on a
// version-controlled configuration. The open command resolves a URL to
// its configured target and launches that browser directly with the URL
// as one structured argument; validate and explain remain diagnostics.
package main

import (
	"io"
	"os"
)

// Stable exit codes, documented in the root command's long help and
// DEVELOPMENT.md.
const (
	exitSuccess = 0
	exitFailure = 1 // invalid config, invalid URL, or launch failure
	exitUsage   = 2 // unknown command or missing/invalid arguments
)

// run is the CLI composition root. It owns wiring and process concerns;
// routing decisions live in internal/domain. Parser/wiring is Cobra
// (cli.go); the observable contract is unchanged.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return newCLI(stdin, stdout, stderr).run(args)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
