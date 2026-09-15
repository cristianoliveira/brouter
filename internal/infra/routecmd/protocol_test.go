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
	return &Commander{
		Command: argv,
		Clock:   func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}
}

func mustScript(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
	return ""
}

// Happy path: the command returns an exact target ID.
func TestCommanderHappyPath(t *testing.T) {
	mustScript(t)
	c := newCommander(t, writeCommand(t,
		`read -r line
case "$line" in
  *'"host":"example.com"'*) printf '{"version":1,"route":"work-browser"}';;
esac`))
	res, err := c.Decide(context.Background(), "https://example.com/x?a=1#f")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if res.Defer || res.Route != "work-browser" {
		t.Fatalf("result = %+v", res)
	}
}

// Explicit null defers to static rules.
func TestCommanderExplicitNullDefers(t *testing.T) {
	mustScript(t)
	c := newCommander(t, writeCommand(t,
		`read -r line; printf '{"version":1,"route":null}'`))
	res, err := c.Decide(context.Background(), "https://example.com/x")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if !res.Defer || res.Route != "" {
		t.Fatalf("result = %+v, want defer", res)
	}
}

// The request the command sees carries the full versioned contract.
func TestCommanderSeesVersionedRequest(t *testing.T) {
	mustScript(t)
	c := newCommander(t, writeCommand(t,
		`read -r line
case "$line" in
  '{"version":1,"url":{"original":"https://example.com/x?a=1#f","scheme":"https","host":"example.com","port":"","path":"/x","query":"a=1","fragment":"f"},"utils":{"epoch":1700000000,"utc":{"year":2023,"month":11,"day":14,"hour":22,"min":13,"sec":20}}}'*)
    printf '{"version":1,"route":"ok"}';;
  *) printf '{"version":1,"route":"mismatch"}';;
esac`))
	res, err := c.Decide(context.Background(), "https://example.com/x?a=1#f")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if res.Route != "ok" {
		t.Fatalf("request did not match the contract exactly: %+v", res)
	}
}

// Percent escapes survive as written (no decoding).
func TestCommanderSeesEscapedFields(t *testing.T) {
	mustScript(t)
	c := newCommander(t, writeCommand(t,
		`read -r line
case "$line" in
  *'"path":"/a%2Fb"'*'"fragment":"e%2Ff"'*) printf '{"version":1,"route":"escaped-ok"}';;
  *) printf '{"version":1,"route":"decoded"}';;
esac`))
	res, err := c.Decide(context.Background(), "https://example.com/a%2Fb?c=%23d#e%2Ff")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if res.Route != "escaped-ok" {
		t.Fatalf("fields decoded or missing: %+v", res)
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
		{"malformed output", `printf 'not json'`, ErrInvalidResponse},
		{"wrong version", `printf '{"version":2,"route":"x"}'`, ErrInvalidResponse},
		{"extra keys", `printf '{"version":1,"route":"x","extra":1}'`, ErrInvalidResponse},
		{"trailing data", `printf '{"version":1,"route":"x"}{}'`, ErrInvalidResponse},
		{"output cap", `printf '{"version":1,"route":"%s"}' "$(printf 'a%.0s' $(seq 1 9000))"`, ErrCommandOutputCap},
		{"timeout", `sleep 5`, ErrCommandTimeout},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newCommander(t, writeCommand(t, tc.body))
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

// URL acceptance precedes the spawn: the command never runs for
// invalid URLs, and credentials never reach it.
func TestCommanderURLPrevalidation(t *testing.T) {
	mustScript(t)
	c := newCommander(t, []string{filepath.Join(t.TempDir(), "never-spawned")})
	for _, bad := range []string{
		"ht tp://broken.test",
		"ftp://broken.test/x",
		"javascript:alert(1)",
		"https:///x",
		"https://./x",
	} {
		if _, err := c.Decide(context.Background(), bad); err == nil {
			t.Errorf("invalid url %q must be rejected visibly", bad)
		}
	}

	res, err := c.Decide(context.Background(), "https://user:secret@example.com/x")
	if err == nil || !strings.Contains(err.Error(), "userinfo credentials are not supported") {
		t.Fatalf("userinfo URL must be rejected with the fixed message, got %v", err)
	}
	if errors.Is(err, ErrCommandError) {
		t.Fatalf("userinfo rejection must not surface as a command failure")
	}
	if res != (Result{}) {
		t.Fatalf("no partial result: %+v", res)
	}
}

// Host normalization: lowercase, one trailing dot stripped.
func TestCommanderSeesNormalizedHost(t *testing.T) {
	mustScript(t)
	c := newCommander(t, writeCommand(t,
		`read -r line
case "$line" in
  *'"scheme":"https","host":"example.com"'*) printf '{"version":1,"route":"norm-ok"}';;
  *) printf '{"version":1,"route":"raw"}';;
esac`))
	res, err := c.Decide(context.Background(), "HTTPS://EXAMPLE.COM./x")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if res.Route != "norm-ok" {
		t.Fatalf("host not normalized: %+v", res)
	}
}
