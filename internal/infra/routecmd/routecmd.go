// Package routecmd implements the TASK-0029 external routing command
// protocol: an opt-in executable, invoked directly by argv (never
// through a shell), that answers one bounded versioned JSON request
// per URL with either an exact configured target ID or an explicit
// null (defer to static rules).
//
// Boundaries, per plans/todo/0029 (48e3071):
//   - The runner is TRUSTED local code and is NOT sandboxed: it runs
//     with the user's host permissions. This is an explicit product
//     tradeoff, documented in docs/route-command.md.
//   - Per-URL failures are visible and never silently fall back; only
//     an explicit route:null defers to the ordered static rules.
//   - Everything is bounded: hard wall-clock timeout, stdout/stderr
//     caps, strict single-value JSON decoding. Command output is never
//     echoed into responses; failures carry fixed redacted categories.
package routecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Bounds for the protocol (v1 constants; not user-configurable).
const (
	// Timeout is the hard wall-clock budget per invocation. The child
	// is killed when it expires.
	Timeout = 2 * time.Second
	// OutputCap bounds stdout and stderr alike; a command exceeding it
	// is killed and its decision rejected.
	OutputCap = 4 << 10 // 4 KiB
	// MaxURLLen bounds the URL accepted from the CLI (and therefore
	// the request written to the command). Real-world URLs are far
	// smaller; the cap keeps the protocol request bounded by design.
	MaxURLLen = 8 << 10 // 8 KiB
	// ProtocolVersion is the only accepted request/response version.
	ProtocolVersion = 1
)

// Fixed redacted failure categories. Command output never appears in
// errors: the command is trusted, but its output may still contain
// anything, and responses feed automated flows.
var (
	ErrCommandError        = errors.New("route_command: command failed (exit nonzero)")
	ErrCommandTimeout      = errors.New("route_command: command timed out")
	ErrCommandOutputCap    = errors.New("route_command: command output exceeded the size limit")
	ErrInvalidResponse     = errors.New("route_command: invalid response")
	ErrUserinfoUnsupported = errors.New(
		"invalid url: userinfo credentials are not supported for scripted routing")
)

// clockUTC is one frozen clock sample injected into the request.
type clockUTC struct {
	Epoch            int64 `json:"epoch"`
	Year, Month, Day int
	Hour, Min, Sec   int
}

// urlFields are the contract fields sent to the command. They mirror
// the static router's normalization: scheme/host lowercased, one
// trailing host dot stripped, path/query/fragment kept as written
// (escaped), no userinfo ever included.
type urlFields struct {
	Original string `json:"original"`
	Scheme   string `json:"scheme"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Path     string `json:"path"`
	Query    string `json:"query"`
	Fragment string `json:"fragment"`
}

type request struct {
	Version int        `json:"version"`
	URL     urlFields  `json:"url"`
	Utils   utilsBlock `json:"utils"`
}

type utilsBlock struct {
	Epoch int64    `json:"epoch"`
	UTC   utcBlock `json:"utc"`
}

type utcBlock struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
	Hour  int `json:"hour"`
	Min   int `json:"min"`
	Sec   int `json:"sec"`
}

// response is the decoded command decision. UnmarshalJSON enforces
// the v1 shape exactly: both keys present, no extras, version 1, and
// route either a string or an explicit JSON null (a MISSING route key
// is invalid — it is not a defer).
type response struct {
	Version int
	Route   *string
}

func (r *response) UnmarshalJSON(data []byte) error {
	var raws map[string]json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return err
	}
	allowed := map[string]bool{"version": true, "route": true}
	for k := range raws {
		if !allowed[k] {
			return fmt.Errorf("unknown key %q", k)
		}
	}
	versionRaw, ok := raws["version"]
	if !ok {
		return errors.New("missing version")
	}
	if err := json.Unmarshal(versionRaw, &r.Version); err != nil {
		return fmt.Errorf("version must be a number: %w", err)
	}
	routeRaw, ok := raws["route"]
	if !ok {
		return errors.New("missing route (v1 requires an explicit target or null)")
	}
	if string(routeRaw) == "null" {
		return nil // r.Route stays nil = explicit defer
	}
	var target string
	if err := json.Unmarshal(routeRaw, &target); err != nil {
		return fmt.Errorf("route must be a string or null: %w", err)
	}
	r.Route = &target
	return nil
}

// Result is one command decision: Defer is true only for an explicit
// route:null; otherwise Route is the raw target ID string (validated
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

// Commander invokes the configured routing command.
type Commander struct {
	// Command is the direct argv (executable first); never a shell
	// string.
	Command []string
	// Clock freezes the time sample exposed to the command. Defaults
	// to time.Now.
	Clock func() time.Time
	// Runner spawns the process; defaults to the exec-based runner.
	Runner Runner
}

// frozenClock collapses a clock function into the request's utils
// block, in UTC.
func frozenClock(clock func() time.Time) clockUTC {
	t := clock().UTC()
	return clockUTC{
		Epoch: t.Unix(),
		Year:  t.Year(), Month: int(t.Month()), Day: t.Day(),
		Hour: t.Hour(), Min: t.Minute(), Sec: t.Second(),
	}
}

// Decide evaluates one URL through the configured command.
//
// URL acceptance precedes any spawn: malformed, non-http(s), empty
// host, and userinfo-bearing URLs are visible errors and the command
// never runs. The returned error for command failures is always one
// of the fixed category errors above.
func (c *Commander) Decide(ctx context.Context, rawURL string) (Result, error) {
	fields, err := contractURL(rawURL)
	if err != nil {
		return Result{}, err
	}

	if c.Clock == nil {
		c.Clock = time.Now
	}
	if c.Runner == nil {
		c.Runner = execRunner{}
	}

	req := request{
		Version: ProtocolVersion,
		URL:     fields,
		Utils:   utilsFromClock(frozenClock(c.Clock)),
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return Result{}, fmt.Errorf("route_command: encode request: %w", err)
	}

	stdout, err := c.Runner.Run(ctx, c.Command, reqBytes)
	if err != nil {
		return Result{}, err // fixed category errors from the runner
	}

	return decodeDecision(stdout)
}

// contractURL applies the URL acceptance rules shared with the static
// router, before anything is spawned.
func contractURL(rawURL string) (urlFields, error) {
	if len(rawURL) > MaxURLLen {
		return urlFields{}, fmt.Errorf("invalid url: exceeds the %d-byte limit", MaxURLLen)
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
	return urlFields{
		Original: rawURL,
		Scheme:   scheme,
		Host:     host,
		Port:     u.Port(),
		Path:     u.EscapedPath(),
		Query:    u.RawQuery,
		Fragment: u.EscapedFragment(),
	}, nil
}

// utilsFromClock adapts the frozen sample to its JSON block.
func utilsFromClock(c clockUTC) utilsBlock {
	return utilsBlock{
		Epoch: c.Epoch,
		UTC: utcBlock{
			Year: c.Year, Month: c.Month, Day: c.Day,
			Hour: c.Hour, Min: c.Min, Sec: c.Sec,
		},
	}
}

// decodeDecision strictly parses the single JSON response: exact
// version, known keys only, route is a string or explicit null, and
// no trailing bytes.
func decodeDecision(stdout []byte) (Result, error) {
	if len(bytes.TrimSpace(stdout)) == 0 {
		return Result{}, fmt.Errorf("%w: empty output", ErrInvalidResponse)
	}
	dec := json.NewDecoder(bytes.NewReader(stdout))
	dec.DisallowUnknownFields()
	var resp response
	if err := dec.Decode(&resp); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}
	if dec.More() {
		return Result{}, fmt.Errorf("%w: trailing data", ErrInvalidResponse)
	}
	if resp.Version != ProtocolVersion {
		return Result{}, fmt.Errorf("%w: unsupported version %d", ErrInvalidResponse, resp.Version)
	}
	if resp.Route == nil {
		return Result{Defer: true}, nil
	}
	return Result{Route: *resp.Route}, nil
}
