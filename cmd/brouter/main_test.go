package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig writes a config file and returns its path. Configs in tests
// are inline so each case owns its exact bytes.
func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const twoRuleConfig = `
default = "personal"

[browsers.personal]
browser = "chrome"

[[rules]]
name = "work"
matcher = "exact-host"
pattern = "company.example"
target = "personal"

[[rules]]
name = "tickets"
matcher = "url-regex"
pattern = "^https://tickets"
target = "personal"
`

func runCapture(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code := run(args, strings.NewReader(stdin), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestHelpInvocationShowsUsage(t *testing.T) {
	// Given every help-style invocation, when the CLI runs, it succeeds
	// with usage on stdout and nothing on stderr.
	helpPaths := []struct {
		name string
		args []string
	}{
		{name: "no arguments shows usage", args: nil},
		{name: "help command shows usage", args: []string{"help"}},
		{name: "double-dash flag shows usage", args: []string{"--help"}},
		{name: "single-dash flag shows usage", args: []string{"-h"}},
	}

	for _, path := range helpPaths {
		t.Run(path.name, func(t *testing.T) {
			code, stdout, stderr := runCapture(t, "", path.args...)

			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
			if !strings.Contains(stdout, "Usage:") {
				t.Errorf("stdout = %q, want usage text", stdout)
			}
		})
	}
}

func TestUnknownCommandFailsWithUsageCode(t *testing.T) {
	// Given an unrecognized command name, when the CLI runs, it exits with
	// the usage code and points to help instead of failing silently.
	code, stdout, stderr := runCapture(t, "", "bogus")

	t.Run("exit code is the usage code", func(t *testing.T) {
		if code != exitUsage {
			t.Errorf("exit code = %d, want %d", code, exitUsage)
		}
	})

	t.Run("stdout stays empty", func(t *testing.T) {
		if stdout != "" {
			t.Errorf("stdout = %q, want empty", stdout)
		}
	})

	t.Run("stderr names the command and points to help", func(t *testing.T) {
		if !strings.Contains(stderr, `unknown command "bogus"`) {
			t.Errorf("stderr = %q, want unknown-command error", stderr)
		}
		if !strings.Contains(stderr, "brouter help") {
			t.Errorf("stderr = %q, want pointer to brouter help", stderr)
		}
	})
}

func TestValidateReportsHealthyConfig(t *testing.T) {
	// Given a valid config, when validate runs, it succeeds and reports
	// the counted targets and rules on stdout.
	path := writeConfig(t, twoRuleConfig)

	code, stdout, stderr := runCapture(t, "", "validate", "-config", path)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "config OK") {
		t.Errorf("stdout = %q, want a config OK report", stdout)
	}
	if !strings.Contains(stdout, "targets 1") || !strings.Contains(stdout, "rules 2") {
		t.Errorf("stdout = %q, want counted targets and rules", stdout)
	}
	if !strings.Contains(stdout, `default "personal"`) {
		t.Errorf("stdout = %q, want the default target", stdout)
	}
}

func TestValidateFailsWithFieldErrorsOnInvalidConfig(t *testing.T) {
	// Given a config whose rule references an undefined target, when
	// validate runs, it exits 1 and names the offending rule and target on
	// stderr; stdout stays empty.
	path := writeConfig(t, `
default = "personal"

[browsers.personal]
browser = "chrome"

[[rules]]
name = "points-nowhere"
matcher = "exact-host"
pattern = "a.example"
target = "missing-target"
`)

	code, stdout, stderr := runCapture(t, "", "validate", "-config", path)

	if code != exitFailure {
		t.Fatalf("exit code = %d, want %d", code, exitFailure)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on failure", stdout)
	}
	if !strings.Contains(stderr, "points-nowhere") || !strings.Contains(stderr, "missing-target") {
		t.Errorf("stderr = %q, want the offending rule and target named", stderr)
	}
}

func TestValidateFailsWhenConfigFileIsMissing(t *testing.T) {
	// Given a config path that does not exist, when validate runs, the
	// missing file is named on stderr with exit 1.
	missing := filepath.Join(t.TempDir(), "nope.toml")

	code, stdout, stderr := runCapture(t, "", "validate", "-config", missing)

	if code != exitFailure {
		t.Fatalf("exit code = %d, want %d", code, exitFailure)
	}
	if !strings.Contains(stderr, "nope.toml") {
		t.Errorf("stderr = %q, want the missing file named", stderr)
	}
	_ = stdout
}

func TestValidateRejectsMisusedFlags(t *testing.T) {
	// Given an undefined flag, when validate runs, it exits with the
	// usage code.
	code, _, stderr := runCapture(t, "", "validate", "-bogus")

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if stderr == "" {
		t.Error("stderr = empty, want flag guidance")
	}
}

func TestExplainReportsTheRoutingDecision(t *testing.T) {
	// Given a matching URL and a valid config, when explain runs, it
	// prints the deterministic decision report on stdout, states that no
	// browser was launched, and warns about sensitive URL data.
	path := writeConfig(t, twoRuleConfig)

	code, stdout, stderr := runCapture(t, "", "explain", "-config", path, "https://company.example/page?q=1")

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	for _, want := range []string{
		"url: https://company.example/page?q=1",
		`target: personal (rule "work")`,
		"fallback: no (a rule matched)",
		"rules evaluated: 1",
		`1. work (exact-host "company.example"): matched`,
		`skipped 1. tickets: not evaluated (an earlier rule matched)`,
		"no browser was launched.",
		"may contain sensitive data",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty on success", stderr)
	}
}

func TestExplainReportsFallbackToDefault(t *testing.T) {
	// Given a URL no rule matches, when explain runs, the report names the
	// default target, lists every evaluated rule, and records no skips.
	path := writeConfig(t, twoRuleConfig)

	code, stdout, stderr := runCapture(t, "", "explain", "-config", path, "https://other.example/")

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	for _, want := range []string{
		"target: personal (default)",
		"fallback: yes (no rule matched)",
		"rules evaluated: 2",
		"1. work (exact-host \"company.example\"): no match",
		"2. tickets (url-regex (pattern redacted)): no match",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "skipped") {
		t.Errorf("stdout mentions skips with no match:\n%s", stdout)
	}
}

func TestExplainIsDeterministic(t *testing.T) {
	// Given the same invocation twice, when explain runs, the output is
	// byte-identical.
	path := writeConfig(t, twoRuleConfig)

	_, first, _ := runCapture(t, "", "explain", "-config", path, "https://company.example/")
	_, second, _ := runCapture(t, "", "explain", "-config", path, "https://company.example/")

	if first != second {
		t.Errorf("explain output is not deterministic:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestValidateRejectsPositionalArguments(t *testing.T) {
	// Given validate receives a positional argument, when it runs, that is
	// a usage error: validate reads no URL.
	path := writeConfig(t, twoRuleConfig)

	code, _, stderr := runCapture(t, "", "validate", "-config", path, "unexpected-arg")

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "unexpected-arg") {
		t.Errorf("stderr = %q, want the argument named", stderr)
	}
}

func TestValidateReportsUnresolvableConfigLocation(t *testing.T) {
	// Given no usable config directory and no explicit override, when
	// validate runs, the failure is visible and actionable.
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	code, _, stderr := runCapture(t, "", "validate")

	if code != exitFailure {
		t.Fatalf("exit code = %d, want %d", code, exitFailure)
	}
	if !strings.Contains(stderr, "cannot determine config path") {
		t.Errorf("stderr = %q, want the resolution failure", stderr)
	}
}

func TestExplainRejectsMultipleURLArguments(t *testing.T) {
	// Given two URL arguments, when explain runs, that is a usage error:
	// exactly one URL is explained per invocation.
	path := writeConfig(t, twoRuleConfig)

	code, _, stderr := runCapture(t, "", "explain", "-config", path, "https://a.example/", "https://b.example/")

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "exactly one URL") {
		t.Errorf("stderr = %q, want one-URL guidance", stderr)
	}
}

func TestExplainReportsProfiles(t *testing.T) {
	// Given the winning target has a machine-local profile, when explain
	// runs, the profile is reported alongside the target.
	path := writeConfig(t, `
default = "work"

[browsers.work]
browser = "brave"
profile = "Work"

[[rules]]
name = "always"
matcher = "exact-host"
pattern = "company.example"
target = "work"
`)

	code, stdout, stderr := runCapture(t, "", "explain", "-config", path, "https://company.example/")

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "profile: Work") {
		t.Errorf("stdout = %q, want the profile reported", stdout)
	}
}

func TestExplainRejectsInvalidURLsWithExitOne(t *testing.T) {
	// Given an unsupported or malformed URL, when explain runs, it fails
	// with the typed reason on stderr and exit 1.
	path := writeConfig(t, twoRuleConfig)

	for _, tc := range []struct {
		name, url, want string
	}{
		{name: "unsupported scheme", url: "ftp://company.example", want: "unsupported scheme"},
		{name: "malformed url", url: "http://[bad", want: "malformed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := runCapture(t, "", "explain", "-config", path, tc.url)

			if code != exitFailure {
				t.Fatalf("exit code = %d, want %d", code, exitFailure)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr = %q, want %q", stderr, tc.want)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty on failure", stdout)
			}
		})
	}
}

func TestExplainReadsURLFromStdinWhenArgumentIsMissing(t *testing.T) {
	// Given no URL argument, when explain runs, it reads one URL line from
	// stdin; empty stdin is a usage error.
	path := writeConfig(t, twoRuleConfig)

	code, stdout, _ := runCapture(t, "https://company.example/\n", "explain", "-config", path)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "url: https://company.example/") {
		t.Errorf("stdout = %q, want the stdin URL explained", stdout)
	}

	code, _, stderr := runCapture(t, "\n", "explain", "-config", path)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d for empty stdin", code, exitUsage)
	}
	if !strings.Contains(stderr, "URL") {
		t.Errorf("stderr = %q, want URL guidance", stderr)
	}
}

func TestExplainKeepsBrowserAvailabilityOutOfReports(t *testing.T) {
	// Given a config whose browser may or may not be installed, when
	// validate and explain run, neither reports availability claims, and
	// explain explicitly states that no browser was launched.
	path := writeConfig(t, `
default = "firefox-zone"

[browsers.firefox-zone]
browser = "firefox"
profile = "Work"

[[rules]]
name = "always"
matcher = "exact-host"
pattern = "company.example"
target = "firefox-zone"
`)

	code, validateOut, validateErr := runCapture(t, "", "validate", "-config", path)
	if code != 0 {
		t.Fatalf("validate exit = %d stderr = %q", code, validateErr)
	}

	code, explainOut, explainErr := runCapture(t, "", "explain", "-config", path, "https://company.example/")
	if code != 0 {
		t.Fatalf("explain exit = %d", code)
	}

	for name, report := range map[string]string{
		"validate": validateOut + validateErr,
		"explain":  explainOut + explainErr,
	} {
		for _, claim := range []string{"installed", "browser launched", "opening", "is available"} {
			if strings.Contains(strings.ToLower(report), claim) {
				t.Errorf("%s claimed browser availability (%q):\n%s", name, claim, report)
			}
		}
	}

	if !strings.Contains(explainOut, "no browser was launched") {
		t.Errorf("explain must state that no browser was launched:\n%s", explainOut)
	}
}

func TestExplainRedactsSensitiveRegexPatterns(t *testing.T) {
	// Given a url-regex rule whose pattern embeds a secret, when explain
	// runs, the report explains the outcome without echoing the pattern.
	path := writeConfig(t, `
default = "hooks"

[browsers.hooks]
browser = "chrome"

[[rules]]
name = "hooked"
matcher = "url-regex"
pattern = "^https://hooks\\.example/[?]?token=secret-hunter-42"
target = "hooks"
`)

	code, stdout, stderr := runCapture(t, "", "explain", "-config", path, "https://hooks.example/?token=secret-hunter-42")

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "hooked") || !strings.Contains(stdout, "matched") {
		t.Errorf("stdout = %q, want the evaluated rule explained", stdout)
	}
	if strings.Contains(stdout, `token=secret-hunter-42(?`) {
		t.Errorf("stdout = %q, want the config pattern redacted", stdout)
	}
	if !strings.Contains(stdout, "(pattern redacted)") {
		t.Errorf("stdout = %q, want an explicit redaction marker on the rule line", stdout)
	}
}
