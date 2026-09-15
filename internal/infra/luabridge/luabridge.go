// Package luabridge implements the TASK-0029 first slice: an opt-in
// route(ctx) evaluated by an isolated C Lua 5.4 helper process, with
// per-URL immutable snapshots read on the Go side.
//
// Boundaries, per docs/route-script-contract.md:
//   - The Go router stays CGO-free; Lua runs only in the helper child.
//   - The script and config are re-read for every URL (no watchers, no
//     caches): each request sees one immutable snapshot, and edited or
//     broken files take effect on the next URL — visibly.
//   - The helper receives no filesystem, network, process, or
//     environment capability: only the framed request fields.
package luabridge

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Result statuses.
const (
	StatusOK    = "ok"    // script returned a target string
	StatusDefer = "nil"   // script deferred: static rules decide
	StatusError = "error" // failure: Category carries the safe class
)

// Pair is one framed key/value field.
type Pair struct {
	Key   string
	Value string
}

// Result is the classified outcome of one helper run.
type Result struct {
	Status   string
	Target   string // set only when Status is StatusOK
	Category string // set only when Status is StatusError
}

// Runner executes one framed helper request. Injectable for tests; the
// production runner spawns the helper with a wall-clock budget.
type Runner func(ctx context.Context, request []byte) ([]byte, error)

// Provider evaluates route(ctx) through the helper process.
type Provider struct {
	HelperPath string // brouter-lua-helper binary
	ScriptPath string // route script; re-read per URL
	Clock      func() time.Time
	Budget     time.Duration // wall-clock budget for the child process
	Runner     Runner        // defaults to the exec-based runner
}

// NewProvider returns a provider with production defaults. The clock
// and runner are replaceable for deterministic tests.
func NewProvider(helperPath, scriptPath string) *Provider {
	return &Provider{HelperPath: helperPath, ScriptPath: scriptPath, Clock: time.Now}
}

// readFile is the script source seam; tests replace it to simulate
// unstable mid-read edits.
var readFile = os.ReadFile

// ReadScript loads one immutable script snapshot with unstable-read
// rejection: the file is read twice and a disagreement between the two
// reads means a writer is mid-edit — the read is rejected visibly so a
// torn partial script is never served. An atomic-rename edit racing an
// open may therefore reject that one open; the next URL applies the
// new script. There is deliberately no cache: every URL re-reads.
func ReadScript(path string) ([]byte, error) {
	first, err := readFile(path)
	if err != nil {
		return nil, fmt.Errorf("read route script: %w", err)
	}
	second, err := readFile(path)
	if err != nil {
		return nil, fmt.Errorf("read route script: %w", err)
	}
	if !bytes.Equal(first, second) {
		return nil, fmt.Errorf("route script changed while reading; retry the open")
	}
	return second, nil
}

// Decide evaluates one URL: it takes a fresh script snapshot, injects
// the contract fields, runs the helper, and classifies the outcome.
//
// Returned errors are visible configuration failures (unreadable
// script, missing helper) and must stop the open. Returned Results
// with StatusError are per-URL runtime failures: the caller falls back
// to static rules and records the category.
// errUserinfoUnsupported is fixed text: it never echoes the URL or
// its credentials.
var errUserinfoUnsupported = errors.New(
	"invalid url: userinfo credentials are not supported for scripted routing")

// parseContractURL applies the contract's URL acceptance rules before
// anything else: the URL must be well-formed http(s) with a non-empty
// normalized host — identical to the static router's rejection.
func parseContractURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("invalid url: unsupported scheme %q (only http and https are routed)", scheme)
	}
	host := strings.ToLower(u.Hostname())
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return nil, fmt.Errorf("invalid url: empty host")
	}
	return u, nil
}

func (p *Provider) Decide(ctx context.Context, rawURL string) (Result, error) {
	// URL validation precedes everything: malformed or non-http(s)
	// URLs never reach the script or the helper (contract-safe
	// handling, identical to the static router's rejection).
	u, err := parseContractURL(rawURL)
	if err != nil {
		return Result{}, err
	}

	// Credentials present: scripted routing is rejected visibly with
	// a fixed, redacted message before the script is read or the
	// helper spawns. No fallback that would silently change routing
	// semantics for the affected URL; no credential ever leaves Go.
	if u.User != nil {
		return Result{}, errUserinfoUnsupported
	}

	script, err := ReadScript(p.ScriptPath)
	if err != nil {
		return Result{}, err
	}

	// Contract normalization, identical to the static router's host
	// matching: scheme lowercased, host lowercased with one trailing
	// dot stripped, no IDN conversion.
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	host = strings.TrimSuffix(host, ".")
	port := u.Port()

	epoch := p.Clock().Unix()
	pairs := []Pair{
		{"script", string(script)},
		{"epoch", fmt.Sprintf("%d", epoch)},
		{"url.original", rawURL},
		{"url.scheme", scheme},
		{"url.host", host},
		{"url.port", port},
		{"url.path", u.EscapedPath()},
		{"url.query", u.RawQuery},
		{"url.fragment", u.EscapedFragment()},
	}

	budget := p.Budget
	if budget <= 0 {
		budget = 5 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	runner := p.Runner
	if runner == nil {
		runner = execRunner(p.HelperPath)
	}
	out, err := runner(runCtx, frameRequest(pairs))
	if err != nil {
		if ctxErr := runCtx.Err(); ctxErr == context.DeadlineExceeded {
			// The wall-clock kill is the backstop behind the helper's
			// instruction budget; the observable outcome is the same.
			return Result{Status: StatusError, Category: "script-timeout"}, nil
		}
		return Result{}, fmt.Errorf("run helper: %w", err)
	}

	return parseResponse(out)
}

func execRunner(helperPath string) Runner {
	return func(ctx context.Context, request []byte) ([]byte, error) {
		cmd := exec.CommandContext(ctx, helperPath)
		cmd.Stdin = bytes.NewReader(request)
		return cmd.Output()
	}
}

// frameRequest encodes pairs as [uint32 LE count]{[uint32 LE klen][key]
// [uint32 LE vlen][value]} — little-endian to match the helper's
// native-endian writes on supported targets.
func frameRequest(pairs []Pair) []byte {
	size := 4
	for _, p := range pairs {
		size += 4 + len(p.Key) + 4 + len(p.Value)
	}
	buf := make([]byte, 0, size)
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(pairs)))
	for _, p := range pairs {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(p.Key)))
		buf = append(buf, p.Key...)
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(p.Value)))
		buf = append(buf, p.Value...)
	}
	return buf
}

// parseResponse decodes the helper's framed response and maps it to a
// Result. Unknown statuses and malformed frames are visible bridge
// failures, never silently-treated fallbacks.
func parseResponse(data []byte) (Result, error) {
	fields, err := readFramedPairs(data)
	if err != nil {
		return Result{}, err
	}

	switch fields["status"] {
	case StatusOK:
		if _, ok := fields["target"]; !ok {
			return Result{}, fmt.Errorf("helper returned ok without target")
		}
		return Result{Status: StatusOK, Target: fields["target"]}, nil
	case StatusDefer:
		return Result{Status: StatusDefer}, nil
	case StatusError:
		category := fields["category"]
		if category == "" {
			return Result{}, fmt.Errorf("helper returned error without category")
		}
		// Load-level failures mean the configured script itself is
		// broken: they are visible configuration failures, not per-URL
		// fallbacks — the contract's no-stale/no-silent rule.
		if category == "script-load-error" || category == "missing-route-function" {
			return Result{}, fmt.Errorf("script load failed (%s)", category)
		}
		return Result{Status: StatusError, Category: category}, nil
	default:
		return Result{}, fmt.Errorf("helper returned unknown status %q", fields["status"])
	}
}

// readFramedPairs decodes the helper's pair framing into a map.
func readFramedPairs(data []byte) (map[string]string, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("helper response too short")
	}
	count := int(binary.LittleEndian.Uint32(data))
	if count == 0 || count > 16 {
		return nil, fmt.Errorf("helper response has invalid pair count %d", count)
	}
	off := 4
	fields := make(map[string]string, count)
	for i := 0; i < count; i++ {
		if off+4 > len(data) {
			return nil, fmt.Errorf("helper response truncated in key length")
		}
		klen := int(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4
		if off+klen > len(data) {
			return nil, fmt.Errorf("helper response truncated in key")
		}
		key := string(data[off : off+klen])
		off += klen
		if off+4 > len(data) {
			return nil, fmt.Errorf("helper response truncated in value length")
		}
		vlen := int(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4
		if off+vlen > len(data) {
			return nil, fmt.Errorf("helper response truncated in value")
		}
		fields[key] = string(data[off : off+vlen])
		off += vlen
	}
	return fields, nil
}

func parseResponseStatus(fields map[string]string) (Result, error) {
	switch fields["status"] {
	case StatusOK:
		if _, ok := fields["target"]; !ok {
			return Result{}, fmt.Errorf("helper returned ok without target")
		}
		return Result{Status: StatusOK, Target: fields["target"]}, nil
	case StatusDefer:
		return Result{Status: StatusDefer}, nil
	case StatusError:
		category := fields["category"]
		if category == "" {
			return Result{}, fmt.Errorf("helper returned error without category")
		}
		// Load-level failures mean the configured script itself is
		// broken: they are visible configuration failures, not per-URL
		// fallbacks — the contract's no-stale/no-silent rule.
		if category == "script-load-error" || category == "missing-route-function" {
			return Result{}, fmt.Errorf("script load failed (%s)", category)
		}
		return Result{Status: StatusError, Category: category}, nil
	default:
		return Result{}, fmt.Errorf("helper returned unknown status %q", fields["status"])
	}
}
