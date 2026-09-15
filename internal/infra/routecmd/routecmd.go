// Package routecmd implements the TASK-0029 external routing command
// line protocol: an opt-in executable, invoked directly by argv (never
// through a shell), that reads the URL on stdin and answers with one
// line — an exact configured target ID, or the reserved literal
// @default meaning explicit defer to the ordered static rules.
//
// Boundaries, per plans/todo/0029 (6863ca1):
//   - The runner is TRUSTED local code and is NOT sandboxed: it runs
//     with the user's host permissions. This is an explicit product
//     tradeoff, documented in docs/route-command.md.
//   - Per-URL failures are visible and never silently fall back; only
//     an exact @default line defers.
//   - Everything is bounded: hard wall-clock timeout, stdout/stderr
//     caps, strict single-line decoding. Command output is never
//     echoed into responses; failures carry fixed redacted categories.
package routecmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// Bounds for the protocol (v1 constants; not user-configurable).
const (
	// Timeout is the hard wall-clock budget per invocation. The whole
	// process group is killed when it expires.
	Timeout = 2 * time.Second
	// OutputCap bounds stdout and stderr alike; a command exceeding it
	// is killed and its decision rejected.
	OutputCap = 4 << 10 // 4 KiB
	// MaxURLLen bounds the URL accepted from the CLI (and therefore
	// the request written to the command). Real-world URLs are far
	// smaller; the cap keeps the protocol request bounded by design.
	MaxURLLen = 8 << 10 // 8 KiB
	// DefaultToken is the reserved literal meaning explicit defer. It
	// cannot be a configured target ID while a route command is active.
	DefaultToken = "@default"
)

// Fixed redacted failure categories. Command output never appears in
// errors: the command is trusted, but its output may still contain
// anything, and responses feed automated flows.
var (
	ErrCommandError        = errors.New("route_command: command failed (exit nonzero)")
	ErrCommandUnavailable  = errors.New("route_command: command unavailable")
	ErrCommandTimeout      = errors.New("route_command: command timed out")
	ErrCommandOutputCap    = errors.New("route_command: command output exceeded the size limit")
	ErrInvalidResponse     = errors.New("route_command: invalid response")
	ErrUserinfoUnsupported = errors.New(
		"invalid url: userinfo credentials are not supported for scripted routing")
)

// Result is one command decision: Defer is true only for an exact
// @default line; otherwise Route is the raw target ID string (validated
// against the config snapshot by the caller).
type Result struct {
	Route string
	Defer bool
}

// Runner executes the command with the request on stdin and returns
// the bounded stdout. It is the process seam for deterministic tests.
type Runner interface {
	Run(ctx context.Context, argv []string, stdin []byte) (stdout []byte, err error)
}

// Commander invokes the configured routing command. v1 sends only the
// original URL bytes; there is deliberately no clock or context
// payload to freeze.
type Commander struct {
	// Command is the direct argv (executable first); never a shell
	// string.
	Command []string
	// Runner spawns the process; defaults to the exec-based runner.
	Runner Runner
}

// Decide evaluates one URL through the configured command.
//
// URL acceptance precedes any spawn: malformed, non-http(s), empty
// host, over-long, control-byte-containing, and userinfo-bearing URLs
// are visible errors and the command never runs. The returned error
// for command failures is always one of the fixed category errors
// above.
func (c *Commander) Decide(ctx context.Context, rawURL string) (Result, error) {
	fields, err := contractURL(rawURL)
	if err != nil {
		return Result{}, err
	}

	if c.Runner == nil {
		c.Runner = execRunner{}
	}

	stdout, err := c.Runner.Run(ctx, c.Command, append([]byte(fields.Original), '\n'))
	if err != nil {
		return Result{}, err // fixed category errors from the runner
	}

	return decodeDecision(stdout)
}

// contractURL applies the URL acceptance rules shared with the static
// router, before anything is spawned. Accepted URL bytes are preserved
// exactly on the wire: percent-escapes are never decoded, and raw
// control bytes (which would corrupt the line protocol) are rejected.
func contractURL(rawURL string) (urlFields, error) {
	if len(rawURL) > MaxURLLen {
		return urlFields{}, fmt.Errorf("invalid url: exceeds the %d-byte limit", MaxURLLen)
	}
	for _, b := range []byte(rawURL) {
		if b < 0x20 || b == 0x7f {
			return urlFields{}, fmt.Errorf("invalid url: control bytes are not routable")
		}
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return urlFields{}, fmt.Errorf("invalid url: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return urlFields{}, fmt.Errorf("invalid url: unsupported scheme %q (only http and https are routed)", scheme)
	}
	if u.User != nil {
		return urlFields{}, ErrUserinfoUnsupported
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return urlFields{}, fmt.Errorf("invalid url: empty host")
	}
	_ = scheme
	return urlFields{Original: rawURL}, nil
}

// urlFields carries the wire payload: v1 sends only the original
// bytes. Parsing exists for validation, not for transport.
type urlFields struct {
	Original string
}

// decodeDecision enforces the v1 output contract exactly: the whole
// output is one line — a non-empty UTF-8 target ID with an optional
// single terminal LF or CRLF — or the reserved @default literal
// (explicit defer). Empty output, embedded line breaks, extra lines,
// and non-UTF-8 bytes are visible failures.
func decodeDecision(stdout []byte) (Result, error) {
	line := stdout
	line = bytes.TrimSuffix(line, []byte("\r\n"))
	line = bytes.TrimSuffix(line, []byte("\n"))

	if len(line) == 0 {
		return Result{}, fmt.Errorf("%w: empty output", ErrInvalidResponse)
	}
	if bytes.ContainsAny(line, "\r\n") {
		return Result{}, fmt.Errorf("%w: extra lines", ErrInvalidResponse)
	}
	if !utf8.Valid(line) {
		return Result{}, fmt.Errorf("%w: output is not valid UTF-8", ErrInvalidResponse)
	}
	target := string(line)
	if target == DefaultToken {
		return Result{Defer: true}, nil
	}
	return Result{Route: target}, nil
}
