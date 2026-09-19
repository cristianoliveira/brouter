package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/cristianoliveira/brouter/internal/domain"
	"github.com/cristianoliveira/brouter/internal/infra/config"
	infraapps "github.com/cristianoliveira/brouter/internal/infra/installedwebapp"
)

// runExplain implements the body of `brouter explain URL`: it prints
// the deterministic routing decision for one URL using the same domain
// evaluation the open command uses. It never launches a browser and
// never writes to disk. When a route_command is configured, explain
// reports it WITHOUT executing it: commands may be impure or carry
// side effects, so only real `open` traffic runs them. Flag parsing
// lives in the Cobra command layer (cli.go); args holds zero or one
// positional URL.
func runExplain(configPath string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return runExplainWithDiscovery(configPath, args, stdin, stdout, stderr, infraapps.System())
}

func runExplainWithDiscovery(configPath string, args []string, stdin io.Reader, stdout, stderr io.Writer, apps appDiscovery) int {
	rawURL, ok := singleInput("explain", args, stdin, stderr)
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

	if cfg.RouteCommand != nil {
		fmt.Fprintf(stdout, "route_command: %q configured (not executed by explain)\n", cfg.RouteCommand[0])
	}

	appSnapshot := apps.Discover()
	router, err := domain.NewRouter(cfg.Rules, cfg.Default, appSnapshot.Catalog)
	if err != nil {
		fmt.Fprintf(stderr, "config invalid:\n%v\n", err)
		return exitFailure
	}

	decision, err := router.Evaluate(rawURL)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitFailure
	}

	writeExplain(stdout, decision, cfg)
	return exitSuccess
}

// singleInput takes one positional argument or, when the argument is
// missing, one line from stdin. Empty input is a usage error. Error
// messages name the command they came from.
func singleInput(command string, args []string, stdin io.Reader, stderr io.Writer) (string, bool) {
	if len(args) > 1 {
		fmt.Fprintf(stderr, "%s takes exactly one URL argument\n", command)
		return "", false
	}

	if len(args) == 1 {
		return strings.TrimSpace(args[0]), true
	}

	data, err := io.ReadAll(io.LimitReader(stdin, 8192))
	if err != nil {
		fmt.Fprintf(stderr, "cannot read stdin: %v\n", err)
		return "", false
	}
	line := firstLine(string(data))
	if line == "" {
		fmt.Fprintf(stderr, "%s requires a URL argument, or one URL on stdin\n", command)
		return "", false
	}
	return line, true
}

func firstLine(s string) string {
	if index := strings.IndexByte(s, '\n'); index >= 0 {
		return strings.TrimSpace(s[:index])
	}
	return strings.TrimSpace(s)
}

// writeExplain renders the decision as stable, human-readable lines.
// URL-regex patterns are redacted: they often embed route secrets. Rule
// names, matcher kinds, and outcomes remain fully explained.
func writeExplain(stdout io.Writer, decision domain.Decision, cfg *config.Config) {
	fmt.Fprintf(stdout, "url: %s\n", decision.URL)

	if decision.InstalledApp != nil {
		fmt.Fprintf(stdout, "target: installed web app %q\n", decision.InstalledApp.ID)
		fmt.Fprintln(stdout, "fallback: no (an installed web app matched)")
	} else if decision.Matched {
		fmt.Fprintf(stdout, "target: %s (rule %q)\n", decision.Target, decision.MatchedRule)
		fmt.Fprintln(stdout, "fallback: no (a rule matched)")
	} else {
		fmt.Fprintf(stdout, "target: %s (default)\n", decision.Target)
		fmt.Fprintln(stdout, "fallback: yes (no rule matched)")
	}

	if target, ok := cfg.Targets[string(decision.Target)]; ok && target.Profile != "" {
		fmt.Fprintf(stdout, "profile: %s\n", target.Profile)
	}

	fmt.Fprintf(stdout, "rules evaluated: %d\n", len(decision.Reasons))
	for index, reason := range decision.Reasons {
		outcome := "no match"
		if reason.Matched {
			outcome = "matched"
		}
		pattern := strconv.Quote(reason.Pattern)
		if reason.Kind == domain.URLRegex {
			pattern = "(pattern redacted)"
		}
		fmt.Fprintf(stdout, "  %d. %s (%s %s): %s — %s\n",
			index+1, reason.Rule, reason.Kind, pattern, outcome, reason.Detail)
	}
	for index, name := range decision.Skipped {
		fmt.Fprintf(stdout, "  skipped %d. %s: not evaluated (an earlier rule matched)\n", index+1, name)
	}

	fmt.Fprintln(stdout, "no browser was launched.")
	fmt.Fprintln(stdout, "note: URLs may contain sensitive data; brouter does not log them.")
}
