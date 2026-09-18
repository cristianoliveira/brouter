package routelog

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cristianoliveira/brouter/internal/infra/config"
)

// deterministicNow pins the timestamp so line assertions stay exact.
var deterministicNow = time.Date(2026, 9, 18, 15, 54, 40, 123000000, time.FixedZone("CEST", 2*3600))

func newTestLogger(t *testing.T, cfg *config.LogConfig) (*Logger, string, *strings.Builder) {
	t.Helper()
	warn := &strings.Builder{}
	logger := New(cfg, warn)
	if logger != nil {
		logger.now = func() time.Time { return deterministicNow }
	}
	path := ""
	if logger != nil {
		path = logger.path
	}
	return logger, path, warn
}

func lineOf(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	return string(data)
}

func TestNilLoggerDoesNothing(t *testing.T) {
	// Given a disabled feature, when Emit is called on the nil
	// receiver, then nothing panics and nothing is written.
	var logger *Logger
	logger.Emit(Entry{Target: "dev", Outcome: "launched", RawURL: "https://secret.example/private?token=1"})
}

func TestAbsentOrDisabledConfigYieldsNoLogger(t *testing.T) {
	for _, cfg := range []*config.LogConfig{nil, {Enabled: false}} {
		logger, path, warn := newTestLogger(t, cfg)
		if logger != nil {
			t.Fatalf("expected nil logger for disabled config")
		}
		if path != "" || warn.Len() != 0 {
			t.Fatalf("disabled config must not resolve a path or warn")
		}
	}
}

func TestEmitWritesRedactedSingleLine(t *testing.T) {
	dir := t.TempDir()
	logger, path, warn := newTestLogger(t, &config.LogConfig{
		Enabled: true, Path: filepath.Join(dir, "routing.log"),
	})
	logger.Emit(Entry{
		Target: "dev", Outcome: "launched",
		ResolveMS: 63, LaunchMS: 214,
		RawURL: "https://user:secret@local.example/path?token=1#frag",
	})

	got := lineOf(t, path)
	want := "2026-09-18T15:54:40.123+02:00 target=dev outcome=launched " +
		"resolve_ms=63 launch_ms=214 url=REDACTED\n"
	if got != want {
		t.Fatalf("line mismatch:\n got %q\nwant %q", got, want)
	}
	if warn.Len() != 0 {
		t.Fatalf("unexpected warnings: %q", warn.String())
	}
}

func TestEmitNeverWritesURLBytes(t *testing.T) {
	// Given any opted-in host mode, when a credential-bearing URL is
	// emitted, then no raw URL, secret, path, query, port, or scheme
	// byte reaches the file — only REDACTED (and the opted-in host).
	dir := t.TempDir()
	logger, path, _ := newTestLogger(t, &config.LogConfig{
		Enabled: true, Path: filepath.Join(dir, "routing.log"), Host: true,
	})
	logger.Emit(Entry{
		Target: "work", Outcome: "launched",
		RawURL: "https://alice:hunter2@Local.Example:8443/private/path?q=secret",
	})

	got := lineOf(t, path)
	for _, forbidden := range []string{"alice", "hunter2", "Local.Example", "8443", "/private", "q=secret", "https"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("log leaks %q:\n%s", forbidden, got)
		}
	}
	if !strings.Contains(got, "url=REDACTED host=local.example\n") {
		t.Fatalf("expected redacted URL with lowercase host, got:\n%s", got)
	}
}

func TestHostnameOmittedWhenUnparsableOrMissing(t *testing.T) {
	dir := t.TempDir()
	logger, path, _ := newTestLogger(t, &config.LogConfig{
		Enabled: true, Path: filepath.Join(dir, "routing.log"), Host: true,
	})
	for _, raw := range []string{"://bad", "https:///no-host", "https://" + strings.Repeat("a", 300) + "/x"} {
		logger.Emit(Entry{Target: "dev", Outcome: "launched", RawURL: raw})
	}
	for _, line := range strings.Split(strings.TrimRight(lineOf(t, path), "\n"), "\n") {
		if strings.Contains(line, "host=") {
			t.Fatalf("host must be omitted, got:\n%s", line)
		}
	}
}

func TestRotationKeepsBoundedFiles(t *testing.T) {
	dir := t.TempDir()
	logger, path, warn := newTestLogger(t, &config.LogConfig{
		Enabled: true, Path: filepath.Join(dir, "routing.log"),
		// One 96-byte line fits per 100-byte file: every emit after the
		// first crosses the threshold, making the rotation invariant crisp.
		MaxBytes: 100, MaxFiles: 2,
	})
	for i := 0; i < 8; i++ {
		logger.Emit(Entry{Target: "dev", Outcome: "launched", ResolveMS: int64(i)})
	}

	if warn.Len() != 0 {
		t.Fatalf("unexpected warnings: %q", warn.String())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected live file plus 2 rotated, got %d: %v", len(entries), entries)
	}
	// The live file holds only the newest line; each rotated file holds
	// at most the lines that fit its threshold.
	for _, name := range []string{"routing.log", "routing.log.1", "routing.log.2"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	if newest := lineOf(t, path); !strings.Contains(newest, "resolve_ms=7 ") || strings.Contains(newest, "resolve_ms=6") {
		t.Fatalf("live file must hold only the newest line, got:\n%s", newest)
	}
}

func TestEmitFailureIsVisibleOnWarnWithoutPanic(t *testing.T) {
	// Given an impossible directory (a file where the dir belongs),
	// when Emit runs, then the failure is warned once and routing data
	// flow is otherwise unaffected.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	logger, _, warn := newTestLogger(t, &config.LogConfig{
		Enabled: true, Path: filepath.Join(blocker, "routing.log"),
	})
	logger.Emit(Entry{Target: "dev", Outcome: "launched", RawURL: "https://secret.example/x"})

	if !strings.Contains(warn.String(), "routing log write failed") {
		t.Fatalf("expected visible failure warning, got %q", warn.String())
	}
	if strings.Contains(warn.String(), "secret.example") {
		t.Fatalf("warning must not echo URLs: %q", warn.String())
	}
}

func TestNewWarnsAndDisablesWhenHomeUnavailableForDefaultPath(t *testing.T) {
	// Given the default path with no HOME and no XDG_STATE_HOME, when
	// New resolves, then it warns once and returns the nil no-op.
	t.Setenv("HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	warn := &strings.Builder{}
	logger := New(&config.LogConfig{Enabled: true}, warn)
	if logger != nil {
		t.Fatalf("expected nil logger when the state dir is undeterminable")
	}
	if !strings.Contains(warn.String(), "routing log disabled") {
		t.Fatalf("expected a visible disable warning, got %q", warn.String())
	}
}

func TestLineShapeIsMachineParseable(t *testing.T) {
	dir := t.TempDir()
	logger, path, _ := newTestLogger(t, &config.LogConfig{
		Enabled: true, Path: filepath.Join(dir, "routing.log"), Host: true,
	})
	logger.Emit(Entry{
		Target: "personal", Outcome: "launch-failed", ResolveMS: 5, LaunchMS: 7,
		RawURL: "https://ok.example/x", Error: "browser process failed (/x): exit status 1",
	})
	shape := regexp.MustCompile(
		`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}[+-]\d{2}:\d{2} ` +
			`target=\S+ outcome=\S+ resolve_ms=\d+ launch_ms=\d+ url=REDACTED host=\S+ error=.+$`)
	if got := strings.TrimRight(lineOf(t, path), "\n"); !shape.MatchString(got) {
		t.Fatalf("line does not match contract:\n%s", got)
	}
}

func TestResolvePathDefaultsAndExpansion(t *testing.T) {
	home := func() (string, error) { return "/home/u", nil }
	state := func(h string) string { return filepath.Join(h, ".local", "state") }
	got, err := resolvePath("", home, state)
	if err != nil || got != filepath.Join("/home/u/.local/state/brouter", "routing.log") {
		t.Fatalf("default path mismatch: %q err=%v", got, err)
	}
	got, err = resolvePath("~/logs/routing.log", home, state)
	if err != nil || got != "/home/u/logs/routing.log" {
		t.Fatalf("~ expansion mismatch: %q err=%v", got, err)
	}
	got, err = resolvePath("/var/log/routing.log", home, state)
	if err != nil || got != "/var/log/routing.log" {
		t.Fatalf("absolute path must pass through: %q err=%v", got, err)
	}
	_, err = resolvePath("", func() (string, error) { return "", errors.New("no home") }, state)
	if err == nil {
		t.Fatalf("expected visible error without HOME")
	}
}
