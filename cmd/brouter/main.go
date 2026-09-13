// Command brouter routes URLs to the right browser based on a
// version-controlled configuration. This bootstrap build only exposes
// help; routing commands arrive with later tasks.
package main

import (
	"fmt"
	"io"
	"os"
)

const usage = `Usage: brouter [command]

brouter routes URLs to browsers using configurable rules.

Commands:
  help    Show this usage text.

Run 'brouter help' for this text. Routing commands are not implemented yet.
`

// run is the CLI composition root. It owns wiring and process concerns;
// it must stay free of domain routing logic.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}

	fmt.Fprintf(stderr, "brouter: unknown command %q\nRun 'brouter help' for usage.\n", args[0])
	return 1
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
