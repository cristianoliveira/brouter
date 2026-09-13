package launch

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// Runner executes a prepared command. The production runner is
// (*exec.Cmd).Run; tests inject fakes. Injected as a plain function so
// call sites stay independent of the exec package.
type Runner func(*exec.Cmd) error

// Launch executes the plan with rawURL as one structured argv element.
// The URL is passed verbatim: query strings, fragments, spaces,
// ampersands, and non-ASCII characters survive because no shell parses
// them. The scheme is validated here even though the router already
// rejects non-http(s) input: the launcher is the last line of defense.
// Failures are returned, never swallowed and never retried on another
// target: process exit status is visible evidence, not proof of page
// load.
func Launch(plan Plan, rawURL string, run Runner) error {
	if err := validateHTTPURL(rawURL); err != nil {
		return err
	}
	if plan.Executable == "" {
		return fmt.Errorf("launch plan has no executable")
	}

	// exec.Command builds argv directly: nothing is interpolated into a
	// shell command line, so shell metacharacters in the URL are data.
	cmd := exec.Command(plan.Executable, append(plan.Args, rawURL)...)
	cmd.Env = os.Environ()

	if run == nil {
		run = (*exec.Cmd).Run
	}
	if err := run(cmd); err != nil {
		return fmt.Errorf("browser process failed (%s): %w", plan.Executable, err)
	}
	return nil
}

// validateHTTPURL rejects anything that is not an http or https URL with
// a host. Error text keeps the URL out of the message when it is empty
// or unparseable; the caller already has the input.
func validateHTTPURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("cannot launch an empty URL")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("cannot launch a malformed URL (input redacted): %v", err)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		if parsed.Host == "" {
			return fmt.Errorf("cannot launch a URL without a host (input redacted)")
		}
		return nil
	default:
		return fmt.Errorf("cannot launch scheme %q (only http and https are routed)", strings.ToLower(parsed.Scheme))
	}
}
