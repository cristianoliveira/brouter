package main

import (
	"strings"
	"testing"
)

// This file locks the CLI's parser contract ahead of the Cobra
// migration. Every test here passes against the pre-migration std-flag
// implementation and must keep passing unchanged afterwards: the
// migration swaps the parser, not the behavior.

func TestContractDoubleDashConfigFlagIsAccepted(t *testing.T) {
	// Given the documented --config spelling, when each command runs, the
	// flag is accepted exactly like the single-dash form.
	path := writeConfig(t, twoRuleConfig)

	for _, args := range [][]string{
		{"validate", "--config", path},
		{"explain", "--config", path, "https://company.example/"},
	} {
		code, stdout, stderr := runCapture(t, "", args...)
		if code != 0 {
			t.Errorf("%v: exit code = %d, stderr = %q", args, code, stderr)
		}
		if strings.Contains(stdout, "no browser was launched") && !strings.Contains(stdout, "url: https://company.example/") {
			t.Errorf("%v: stdout = %q, want the URL explained", args, stdout)
		}
	}
}

func TestContractSingleDashConfigFlagKeepsWorking(t *testing.T) {
	// Given the historical single-dash spelling, when commands run, it is
	// accepted. pflag would reject it as a shorthand cluster without the
	// compatibility shim; this test is the shim's guard.
	path := writeConfig(t, twoRuleConfig)

	code, _, stderr := runCapture(t, "", "validate", "-config", path)
	if code != 0 {
		t.Fatalf("validate -config: exit code = %d, stderr = %q", code, stderr)
	}

	code, _, stderr = runCapture(t, "", "explain", "-config="+path, "https://company.example/")
	if code != 0 {
		t.Fatalf("explain -config=: exit code = %d, stderr = %q", code, stderr)
	}
}

func TestContractUnknownFlagIsUsageErrorWithGuidance(t *testing.T) {
	// Given an undefined flag, when validate runs, the exit code is the
	// usage code and stderr carries flag guidance.
	code, _, stderr := runCapture(t, "", "validate", "-bogus")

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "bogus") {
		t.Errorf("stderr = %q, want the offending flag named", stderr)
	}
}

func TestContractUnknownCommandKeepsLegacyMessage(t *testing.T) {
	// Given an unrecognized command, when the CLI runs, stderr names the
	// command and points to brouter help with the usage exit code.
	code, stdout, stderr := runCapture(t, "", "frobnicate")

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, `unknown command "frobnicate"`) || !strings.Contains(stderr, "brouter help") {
		t.Errorf("stderr = %q, want the legacy unknown-command guidance", stderr)
	}
}

func TestContractNoCompletionCommand(t *testing.T) {
	// Given Cobra auto-generates a completion command, when the CLI runs
	// it, that is still an unknown command: the migration adds no
	// features.
	code, _, stderr := runCapture(t, "", "completion")

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, `unknown command "completion"`) {
		t.Errorf("stderr = %q, want the legacy unknown-command guidance", stderr)
	}
}

func TestContractSubcommandHelpKeepsLegacyChannel(t *testing.T) {
	// Given a subcommand invoked with -h or --help, when it runs, the
	// usage guidance goes to stderr with the usage exit code, matching
	// the pre-migration std-flag behavior.
	for _, args := range [][]string{
		{"validate", "-h"},
		{"validate", "--help"},
		{"explain", "-h"},
		{"open", "--help"},
	} {
		code, stdout, stderr := runCapture(t, "", args...)
		if code != exitUsage {
			t.Errorf("%v: exit code = %d, want %d", args, code, exitUsage)
		}
		if stdout != "" {
			t.Errorf("%v: stdout = %q, want empty", args, stdout)
		}
		if !strings.Contains(stderr, "config") {
			t.Errorf("%v: stderr = %q, want flag guidance", args, stderr)
		}
	}
}

func TestContractValidatePositionalMessagePreserved(t *testing.T) {
	// Given validate receives a positional argument, when it runs, the
	// rejection names the argument with the historical wording.
	path := writeConfig(t, twoRuleConfig)

	code, _, stderr := runCapture(t, "", "validate", "-config", path, "unexpected-arg")

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "validate takes no positional arguments") || !strings.Contains(stderr, "unexpected-arg") {
		t.Errorf("stderr = %q, want the historical rejection", stderr)
	}
}

func TestContractExplainMultipleURLMessagePreserved(t *testing.T) {
	// Given two URLs, when explain runs, the historical one-URL guidance
	// is preserved on stderr with the usage exit code.
	path := writeConfig(t, twoRuleConfig)

	code, _, stderr := runCapture(t, "", "explain", "-config", path, "https://a.example/", "https://b.example/")

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "explain takes exactly one URL argument") {
		t.Errorf("stderr = %q, want the historical guidance", stderr)
	}
}

func TestContractOpenMultipleURLMessagePreserved(t *testing.T) {
	// Given two URLs, when open runs, the same one-URL guidance applies.
	dir := t.TempDir()
	browser, _ := writeFakeBrowser(t, dir, 0)
	path := writeConfig(t, executableTargetConfig(browser))

	code, _, stderr := runCapture(t, "", "open", "-config", path, "https://a.example/", "https://b.example/")

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "open takes exactly one URL argument") {
		t.Errorf("stderr = %q, want the historical guidance", stderr)
	}
}
