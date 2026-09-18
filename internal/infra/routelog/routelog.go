// Package routelog writes the opt-in local routing log: one timestamped
// line per web-URL open with per-stage durations and a redacted URL.
// It is evidence, not behavior: emit failures never change routing or
// exit codes, and a disabled logger is a nil receiver no-op.
package routelog

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cristianoliveira/brouter/internal/infra/config"
)

// URLRedacted is the only URL value this log ever writes. Full URLs,
// paths, queries, and credentials never enter the log; the hostname
// appears only when the user explicitly sets host = true.
const URLRedacted = "REDACTED"

// Defaults for unset bounded-log settings: small enough to stay a log,
// large enough to hold a useful diagnostic window.
const (
	defaultMaxBytes = int64(1 << 20) // 1 MiB
	defaultMaxFiles = 3
	minMaxBytes     = int64(1024)
)

// Entry is one resolved-and-launched web URL. Error may carry a short
// launch failure description; callers must already keep URLs out of it
// (launch errors are redacted by construction).
type Entry struct {
	Target    string
	Outcome   string // "launched" or "launch-failed"
	ResolveMS int64
	LaunchMS  int64
	RawURL    string
	Error     string
}

// Logger writes bounded redacted lines. A nil *Logger is valid and
// does nothing: disabled logging has no call-site cost.
type Logger struct {
	path string
	host bool
	max  int64
	want int
	now  func() time.Time
	warn io.Writer
}

// New resolves the validated [log] config into a Logger. It returns
// nil (a no-op) when logging is absent or disabled; the only runtime
// resolution is the default state-dir path and ~ expansion, so a bad
// explicit path surfaces here as a one-time warning — never a routing
// failure. warn receives redacted diagnostics.
func New(cfg *config.LogConfig, warn io.Writer) *Logger {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	path, err := resolvePath(cfg.Path, os.UserHomeDir, stateDir)
	if err != nil {
		fmt.Fprintf(warn, "brouter: routing log disabled: %v\n", err)
		return nil
	}
	max := cfg.MaxBytes
	if max == 0 {
		max = defaultMaxBytes
	}
	want := cfg.MaxFiles
	if want == 0 {
		want = defaultMaxFiles
	}
	return &Logger{
		path: path,
		host: cfg.Host,
		max:  max,
		want: want,
		now:  time.Now,
		warn: warn,
	}
}

// resolvePath applies ~ expansion to an explicit path or derives the
// default from the user state root. It is a pure function of its
// inputs; production wiring passes os.UserHomeDir and stateDir.
func resolvePath(explicit string, homeDir func() (string, error), state func(home string) string) (string, error) {
	if explicit == "" {
		home, err := homeDir()
		if err != nil || home == "" {
			return "", fmt.Errorf("cannot determine the user state directory: set $XDG_STATE_HOME or $HOME")
		}
		return filepath.Join(state(home), "brouter", "routing.log"), nil
	}
	if strings.HasPrefix(explicit, "~/") {
		home, err := homeDir()
		if err != nil || home == "" {
			return "", fmt.Errorf("cannot expand ~ in the configured log path")
		}
		return filepath.Join(home, explicit[2:]), nil
	}
	return explicit, nil
}

// stateDir returns the user state root: $XDG_STATE_HOME when absolute,
// otherwise ~/.local/state — mirroring the config-path convention so
// GUI invocations without a shell resolve the same location.
func stateDir(home string) string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" && filepath.IsAbs(xdg) {
		return xdg
	}
	return filepath.Join(home, ".local", "state")
}

// Emit writes one line. Every failure is visible on warn and swallowed
// otherwise: logging must never break an open. The URL is always
// REDACTED; the hostname is included only when opted in.
func (l *Logger) Emit(e Entry) {
	if l == nil {
		return
	}
	line := l.format(e, l.now())
	if err := l.append(line); err != nil {
		fmt.Fprintf(l.warn, "brouter: routing log write failed: %v\n", err)
	}
}

// format renders one single-line record. Hostname extraction never
// emits path, query, port, userinfo, or raw-URL bytes.
func (l *Logger) format(e Entry, now time.Time) string {
	var b strings.Builder
	b.WriteString(now.Format("2006-01-02T15:04:05.000Z07:00"))
	fmt.Fprintf(&b, " target=%s outcome=%s resolve_ms=%d launch_ms=%d url=%s",
		sanitizeField(e.Target), sanitizeField(e.Outcome), e.ResolveMS, e.LaunchMS, URLRedacted)
	if l.host {
		if host := hostnameOf(e.RawURL); host != "" {
			b.WriteString(" host=" + host)
		}
	}
	if msg := sanitizeMessage(e.Error); msg != "" {
		b.WriteString(" error=" + msg)
	}
	return b.String()
}

// append keeps the log bounded: the file is rotated before the write
// that would cross the byte threshold, and at most want rotated files
// exist. MkdirAll before the first write makes the default state path
// self-healing; failures surface through the caller's warning.
func (l *Logger) append(line string) error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return err
	}
	if info, err := os.Stat(l.path); err == nil && info.Size()+int64(len(line))+1 > l.max {
		if err := rotate(l.path, l.want); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}

// rotate shifts path.1..path.(want-1) up one slot, dropping the oldest,
// then renames the live file to path.1. Best effort per step: a step
// failure is returned and the caller warns; routing is unaffected.
func rotate(path string, want int) error {
	oldest := fmt.Sprintf("%s.%d", path, want)
	if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) {
		return err
	}
	for i := want - 1; i >= 1; i-- {
		prev := fmt.Sprintf("%s.%d", path, i)
		next := fmt.Sprintf("%s.%d", path, i+1)
		if err := os.Rename(prev, next); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Rename(path, path+".1")
}

// hostnameOf lowercases the URL hostname and caps its length; any
// parse failure yields "" so nothing unintended is written.
func hostnameOf(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" || len(host) > 253 {
		return ""
	}
	return host
}

// sanitizeField strips whitespace/control bytes from enum-like fields
// so the line stays single-line and parseable.
func sanitizeField(s string) string {
	if s == "" {
		return `"-"`
	}
	return sanitizeMessage(s)
}

// sanitizeMessage keeps at most 200 bytes of a single-line, control-free
// error description.
func sanitizeMessage(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
