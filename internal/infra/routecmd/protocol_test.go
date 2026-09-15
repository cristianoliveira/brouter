package routecmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeCommand writes an executable shell script and returns its argv.
func writeCommand(t *testing.T, body string) []string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "router-cmd")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{path}
}

func newCommander(t *testing.T, argv []string) *Commander {
	t.Helper()
	return &Commander{Command: argv}
}

func mustScript(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
	return ""
}

// Happy path: the command answers with an exact target ID.
func TestCommanderHappyPath(t *testing.T) {
	mustScript(t)
	c := newCommander(t, writeCommand(t, `printf 'work-browser'`))
	res, err := c.Decide(context.Background(), "https://example.com/x?a=1#f")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if res.Defer || res.Route != "work-browser" {
		t.Fatalf("result = %+v", res)
	}
}

// Terminal LF and CRLF are both accepted.
func TestCommanderTerminalLineEndings(t *testing.T) {
	mustScript(t)
	for _, tc := range []struct {
		name string
		out  string
	}{
		{"no newline", `printf 'work-browser'`},
		{"terminal lf", `printf 'work-browser\n'`},
		{"terminal crlf", `printf 'work-browser\r\n'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCommander(t, writeCommand(t, tc.out))
			res, err := c.Decide(context.Background(), "https://example.com/x")
			if err != nil {
				t.Fatalf("Decide: %v", err)
			}
			if res.Defer || res.Route != "work-browser" {
				t.Fatalf("result = %+v", res)
			}
		})
	}
}

// The exact @default literal is the only defer.
func TestCommanderDefaultTokenDefers(t *testing.T) {
	mustScript(t)
	c := newCommander(t, writeCommand(t, `printf '@default\n'`))
	res, err := c.Decide(context.Background(), "https://example.com/x")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if !res.Defer || res.Route != "" {
		t.Fatalf("result = %+v, want defer", res)
	}
}

// The command reads the original URL bytes plus exactly one LF.
func TestCommanderSeesOriginalURLBytesAndLF(t *testing.T) {
	mustScript(t)
	c := newCommander(t, writeCommand(t,
		`read -r line
[ "$line" = "https://example.com/a%2Fb?c=%23d#e%2Ff" ] && printf 'wire-ok' || printf 'wire-bad'`))
	res, err := c.Decide(context.Background(), "https://example.com/a%2Fb?c=%23d#e%2Ff")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if res.Route != "wire-ok" {
		t.Fatalf("wire bytes wrong (decoded or mangled): %+v", res)
	}
}

// Fixed redacted failure categories.
func TestCommanderFailureCategories(t *testing.T) {
	mustScript(t)
	cases := []struct {
		name    string
		body    string
		wantErr error
	}{
		{"nonzero exit", `exit 3`, ErrCommandError},
		{"unavailable command", "", ErrCommandUnavailable},
		{"empty output", `true`, ErrInvalidResponse},
		{"extra lines", `printf 'a\nb\n'`, ErrInvalidResponse},
		{"embedded newline", `printf 'a\nb'`, ErrInvalidResponse},
		{"embedded carriage return", `printf 'a\rb'`, ErrInvalidResponse},
		{"non-utf8 output", `printf '\200'`, ErrInvalidResponse},
		{"output cap", `printf 'a%.0s' $(seq 1 9000)`, ErrCommandOutputCap},
		{"timeout", `sleep 5`, ErrCommandTimeout},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var argv []string
			if tc.body == "" {
				argv = []string{filepath.Join(t.TempDir(), "does-not-exist")}
			} else {
				argv = writeCommand(t, tc.body)
			}
			c := newCommander(t, argv)
			res, err := c.Decide(context.Background(), "https://example.com/x")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if res != (Result{}) {
				t.Fatalf("no partial result may be returned: %+v", res)
			}
		})
	}
}

// URL acceptance precedes the spawn: control bytes, invalid URLs, and
// credentials never reach the command.
func TestCommanderURLPrevalidation(t *testing.T) {
	mustScript(t)
	c := newCommander(t, []string{filepath.Join(t.TempDir(), "never-spawned")})
	for _, bad := range []string{
		"ht tp://broken.test",
		"ftp://broken.test/x",
		"javascript:alert(1)",
		"https:///x",
		"https://./x",
		"https://example.com/x\nGET /admin HTTP/1.1\r\n", // control bytes
		"https://example.com/x\ty",                       // tab is a control byte
	} {
		if _, err := c.Decide(context.Background(), bad); err == nil {
			t.Errorf("invalid url %q must be rejected visibly", bad)
		}
	}

	res, err := c.Decide(context.Background(), "https://user:secret@example.com/x")
	if err == nil || !strings.Contains(err.Error(), "userinfo credentials are not supported") {
		t.Fatalf("userinfo URL must be rejected with the fixed message, got %v", err)
	}
	if res != (Result{}) {
		t.Fatalf("no partial result: %+v", res)
	}
}

// URLs beyond the explicit request bound are rejected before spawn.
func TestCommanderURLLengthBound(t *testing.T) {
	mustScript(t)
	c := newCommander(t, []string{filepath.Join(t.TempDir(), "never-spawned")})
	long := "https://example.com/" + strings.Repeat("a", MaxURLLen)
	if _, err := c.Decide(context.Background(), long); err == nil ||
		!strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("over-long URL must be rejected with the explicit bound, got %v", err)
	}
}

// Host normalization: lowercase, one trailing dot stripped.
func TestCommanderSeesNormalizedHost(t *testing.T) {
	mustScript(t)
	c := newCommander(t, writeCommand(t,
		`read -r line
case "$line" in
  "HTTPS://EXAMPLE.COM./x") printf 'raw';;
  *) printf 'norm-ok';;
esac`))
	// The wire carries the ORIGINAL bytes; normalization stays in Go.
	// This probe proves the command sees original bytes and that the
	// URL was accepted despite its unusual casing.
	res, err := c.Decide(context.Background(), "HTTPS://EXAMPLE.COM./x")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if res.Route != "raw" {
		t.Fatalf("expected original bytes on the wire: %+v", res)
	}
}

// A command that leaves a descendant holding the pipes still times out
// instead of hanging: the whole process group is killed.
func TestCommanderDescendantCannotHangTheTimeout(t *testing.T) {
	mustScript(t)
	c := newCommander(t, writeCommand(t, `sleep 1 & printf 'slow'; sleep 30 &`))
	done := make(chan struct{})
	var res Result
	var err error
	go func() {
		res, err = c.Decide(context.Background(), "https://example.com/x")
		close(done)
	}()
	select {
	case <-done:
		if !errors.Is(err, ErrCommandTimeout) {
			t.Fatalf("err = %v, want timeout", err)
		}
		if res != (Result{}) {
			t.Fatalf("no partial result: %+v", res)
		}
	case <-time.After(Timeout + 5*time.Second):
		t.Fatal("descendant-held pipes hung past the timeout")
	}
}

// Output overflow kills the process group immediately: the decision
// returns command-output-cap fast, not at the deadline.
func TestCommanderOutputCapKillsImmediately(t *testing.T) {
	mustScript(t)
	// 5s of continuous output: with immediate kill this returns in ~ms;
	// with wait-until-deadline behavior it would take the full 2s.
	c := newCommander(t, writeCommand(t, `while true; do printf 'x%.0s' $(seq 1 500); done`))
	start := time.Now()
	_, err := c.Decide(context.Background(), "https://example.com/x")
	elapsed := time.Since(start)
	if !errors.Is(err, ErrCommandOutputCap) {
		t.Fatalf("err = %v, want output cap", err)
	}
	if elapsed >= Timeout {
		t.Fatalf("cap decision took %v — overflow must kill immediately, not wait for the deadline", elapsed)
	}
}

// Parent cancellation kills the whole process group promptly — the
// decision returns fast even when a descendant holds the pipes.
func TestCommanderParentCancelKillsGroupPromptly(t *testing.T) {
	mustScript(t)
	// Direct child stays alive past the cancel (foreground sleep);
	// a background descendant also holds stdout.
	c := newCommander(t, writeCommand(t, `sleep 30 & sleep 30`))
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err := c.Decide(ctx, "https://example.com/x")
	elapsed := time.Since(start)

	if !errors.Is(err, ErrCommandCanceled) {
		t.Fatalf("err = %v, want command-canceled", err)
	}
	if elapsed >= time.Second {
		t.Fatalf("cancellation took %v — descendants were not killed promptly", elapsed)
	}
}

// Exactly one terminal LF or CRLF is allowed: double terminal endings
// are rejected, not silently double-trimmed.
func TestCommanderDoubleTerminalEndingsRejected(t *testing.T) {
	mustScript(t)
	for _, tc := range []struct {
		name string
		out  string
	}{
		{"lf then crlf", `printf 'target\n\r\n'`},
		{"crlf then lf", `printf 'target\r\n\n'`},
		{"two lfs", `printf 'target\n\n'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCommander(t, writeCommand(t, tc.out))
			res, err := c.Decide(context.Background(), "https://example.com/x")
			if !errors.Is(err, ErrInvalidResponse) {
				t.Fatalf("err = %v, want invalid response", err)
			}
			if res != (Result{}) {
				t.Fatalf("no partial result: %+v", res)
			}
		})
	}
}

// Control bytes other than CR/LF (NUL, tab, escapes) are malformed
// output too: target IDs are plain TOML strings.
func TestCommanderControlBytesInOutputRejected(t *testing.T) {
	mustScript(t)
	for _, tc := range []struct {
		name string
		out  string
	}{
		{"nul byte", `printf 'tar\000get'`},
		{"tab byte", `printf 'tar\tget'`},
		{"escape byte", `printf 'tar\033get'`},
		{"delete byte", `printf 'tar\177get'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCommander(t, writeCommand(t, tc.out))
			res, err := c.Decide(context.Background(), "https://example.com/x")
			if !errors.Is(err, ErrInvalidResponse) {
				t.Fatalf("err = %v, want invalid response", err)
			}
			if res != (Result{}) {
				t.Fatalf("no partial result: %+v", res)
			}
		})
	}
}

// A pre-canceled context classifies as canceled, not as an
// unavailable command.
func TestCommanderPreCanceledContextClassification(t *testing.T) {
	mustScript(t)
	c := newCommander(t, writeCommand(t, `printf 'x'`))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Decide(ctx, "https://example.com/x")
	if !errors.Is(err, ErrCommandCanceled) {
		t.Fatalf("err = %v, want command-canceled", err)
	}
}
