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

func TestHelpIsTheSuccessPath(t *testing.T) {
	// Given no arguments, when the CLI runs, it shows usage and succeeds.
	for _, args := range [][]string{nil, {"help"}, {"--help"}, {"-h"}} {
		code, stdout, stderr := runCapture(t, args...)

		if code != 0 {
			t.Errorf("args %v: exit code = %d, want 0", args, code)
		}
		if stderr != "" {
			t.Errorf("args %v: stderr = %q, want empty", args, stderr)
		}
		if !strings.Contains(stdout, "Usage:") {
			t.Errorf("args %v: stdout = %q, want usage text", args, stdout)
		}
	}
}

func TestUnknownCommandFailsWithGuidance(t *testing.T) {
	// Given an unimplemented command, when the CLI runs, it fails
	// visibly and points to help instead of failing silently.
	code, stdout, stderr := runCapture(t, "bogus")

	if code == 0 {
		t.Errorf("exit code = 0, want nonzero")
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, `unknown command "bogus"`) {
		t.Errorf("stderr = %q, want unknown-command error", stderr)
	}
	if !strings.Contains(stderr, "brouter help") {
		t.Errorf("stderr = %q, want pointer to brouter help", stderr)
	}
}
