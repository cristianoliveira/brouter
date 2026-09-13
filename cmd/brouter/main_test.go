package main

import (
	"bytes"
	"strings"
	"testing"
)

func runCapture(t *testing.T, args ...string) (int, string, string) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
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
			code, stdout, stderr := runCapture(t, path.args...)

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

func TestUnknownCommandFailsWithGuidance(t *testing.T) {
	// Given an unimplemented command name, when the CLI runs, it fails
	// visibly and points to help instead of failing silently.
	code, stdout, stderr := runCapture(t, "bogus")

	t.Run("exit code is nonzero", func(t *testing.T) {
		if code == 0 {
			t.Errorf("exit code = 0, want nonzero")
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
