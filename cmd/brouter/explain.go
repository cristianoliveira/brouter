package main

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/cristianoliveira/brouter/internal/domain"
	"github.com/cristianoliveira/brouter/internal/infra/config"
)

// runExplain implements `brouter explain URL`: it prints the deterministic
// routing decision for one URL using the same domain evaluation the future
// open command will use. It never launches a browser and never writes to
// disk.
func runExplain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "path to config.toml (default: per-OS config location)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	rawURL, ok := singleInput("explain", fs.Args(), stdin, stderr)
	if !ok {
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

	router, err := domain.NewRouter(cfg.Rules, cfg.Default)
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

	if decision.Matched {
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
