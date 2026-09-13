package main

import (
	"strings"
	"testing"
)

// This file locks the Cobra-native parser contract established by the
// TASK-0016 migration: long --config flag, standard help and error
// surfaces, exit codes 0/1/2, and no added features.

func TestNativeConfigFlagAccepted(t *testing.T) {
	// Given the standard --config spelling, when each command runs, the
	// flag is accepted.
	path := writeConfig(t, twoRuleConfig)

	for _, args := range [][]string{
		{"validate", "--config", path},
		{"explain", "--config", path, "https://company.example/"},
	} {
		code, stdout, stderr := runCapture(t, "", args...)
		if code != 0 {
			t.Errorf("%v: exit code = %d, stderr = %q", args, code, stderr)
		}
		if args[0] == "explain" && !strings.Contains(stdout, "url: https://company.example/") {
			t.Errorf("%v: stdout = %q, want the URL explained", args, stdout)
		}
	}
}

func TestNativeSingleDashConfigIsRejected(t *testing.T) {
	// Given the parser is Cobra-native, when the single-dash -config
	// spelling is used, it is rejected as an unknown shorthand with the
	// usage exit code.
	path := writeConfig(t, twoRuleConfig)

	code, stdout, stderr := runCapture(t, "", "validate", "-config", path)

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "config") {
		t.Errorf("stderr = %q, want the offending flag named", stderr)
	}
}

func TestNativeRootHelpSurfaces(t *testing.T) {
	// Given every root help invocation, when the CLI runs, help goes to
	// stdout with exit 0 and empty stderr.
	for _, args := range [][]string{
		nil,
		{"help"},
		{"--help"},
		{"-h"},
	} {
		code, stdout, stderr := runCapture(t, "", args...)
		if code != 0 {
			t.Errorf("%v: exit code = %d, want 0", args, code)
		}
		if !strings.Contains(stdout, "Usage:") {
			t.Errorf("%v: stdout = %q, want usage", args, stdout)
		}
		if stderr != "" {
			t.Errorf("%v: stderr = %q, want empty", args, stderr)
		}
	}
}

func TestNativeSubcommandHelpSurfaces(t *testing.T) {
	// Given a subcommand help invocation, when it runs, per-command help
	// goes to stdout with exit 0 (standard Cobra help surface).
	for _, args := range [][]string{
		{"validate", "-h"},
		{"validate", "--help"},
		{"explain", "--help"},
		{"open", "--help"},
	} {
		code, stdout, stderr := runCapture(t, "", args...)
		if code != 0 {
			t.Errorf("%v: exit code = %d, want 0", args, code)
		}
		if !strings.Contains(stdout, "--config") {
			t.Errorf("%v: stdout = %q, want the --config flag documented", args, stdout)
		}
		if stderr != "" {
			t.Errorf("%v: stderr = %q, want empty", args, stderr)
		}
	}
}

func TestNativeUnknownCommandError(t *testing.T) {
	// Given an unrecognized command, when the CLI runs, Cobra's standard
	// error names the command on stderr with the usage exit code.
	code, stdout, stderr := runCapture(t, "", "frobnicate")

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, `unknown command "frobnicate"`) {
		t.Errorf("stderr = %q, want the unknown-command error", stderr)
	}
}

func TestNativeNoCompletionCommand(t *testing.T) {
	// Given Cobra auto-generates a completion command, when the CLI runs
	// it, that is still an unknown command: the migration adds no
	// features.
	code, _, stderr := runCapture(t, "", "completion")

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, `unknown command "completion"`) {
		t.Errorf("stderr = %q, want the unknown-command error", stderr)
	}
}

func TestNativeUnknownFlagError(t *testing.T) {
	// Given an undefined flag, when validate runs, the parser error is
	// reported on stderr with the usage exit code.
	code, stdout, stderr := runCapture(t, "", "validate", "--bogus")

	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "bogus") {
		t.Errorf("stderr = %q, want the offending flag named", stderr)
	}
}

func TestNativePositionalAndURLMessagesPreserved(t *testing.T) {
	// Command-level argument validation keeps its historical messages:
	// they are command UX, not parser emulation.
	path := writeConfig(t, twoRuleConfig)

	code, _, stderr := runCapture(t, "", "validate", "--config", path, "unexpected-arg")
	if code != exitUsage {
		t.Fatalf("validate exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "validate takes no positional arguments") || !strings.Contains(stderr, "unexpected-arg") {
		t.Errorf("validate stderr = %q, want the historical rejection", stderr)
	}

	code, _, stderr = runCapture(t, "", "explain", "--config", path, "https://a.example/", "https://b.example/")
	if code != exitUsage {
		t.Fatalf("explain exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "explain takes exactly one URL argument") {
		t.Errorf("explain stderr = %q, want the one-URL guidance", stderr)
	}
}

func TestNativeFlagsAfterPositionalAreParsed(t *testing.T) {
	// Given Cobra's default interspersed parsing, when a flag follows a
	// positional argument, it is parsed as a flag: this is standard
	// Cobra behavior and is locked here so it is a known, deliberate
	// contract.
	dir := t.TempDir()
	browser, log := writeFakeBrowser(t, dir, 0)
	path := writeConfig(t, executableTargetConfig(browser))

	code, _, stderr := runCapture(t, "", "open", "https://a.example/", "--config", path)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	if argv := readArgvLog(t, log); len(argv) != 1 || argv[0] != "https://a.example/" {
		t.Errorf("browser argv = %q, want the URL delivered once", argv)
	}
}
