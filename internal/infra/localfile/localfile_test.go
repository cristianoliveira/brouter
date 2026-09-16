package localfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsLocalFileDistinguishesFilesFromWebURLs(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want bool
	}{
		{name: "absolute path", raw: "/tmp/report.html", want: true},
		{name: "relative path", raw: "docs/report.html", want: true},
		{name: "home-relative path", raw: "~/notes/report.pdf", want: true},
		{name: "path with spaces", raw: "/tmp/my docs/report page.html", want: true},
		{name: "file URL", raw: "file:///tmp/report.html", want: true},
		{name: "file URL localhost host", raw: "file://localhost/tmp/report.html", want: true},
		{name: "http URL", raw: "https://example.com/page", want: false},
		{name: "http URL with query", raw: "http://example.com/a?b=c#d", want: false},
		{name: "empty", raw: "", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsLocalFile(tc.raw); got != tc.want {
				t.Errorf("IsLocalFile(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestResolveEncodesSupportedDocument(t *testing.T) {
	// Given a readable document whose name needs URL encoding — spaces,
	// unicode, and reserved characters — when resolved, the result is a
	// file URL whose path survives a parse round trip byte for byte.
	dir := t.TempDir()
	name := "report page #3 – ~100%.html"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve(path)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !strings.HasPrefix(got, "file://") {
		t.Fatalf("Resolve() = %q, want a file URL", got)
	}
	if !strings.Contains(got, "%23") {
		t.Errorf("Resolve() = %q, want the reserved # percent-encoded", got)
	}
	parsed, err := FileURLPath(got)
	if err != nil {
		t.Fatalf("FileURLPath(%q) error = %v", got, err)
	}
	if parsed != path {
		t.Errorf("round trip = %q, want %q", parsed, path)
	}
}

func TestResolveAcceptsFileURLInput(t *testing.T) {
	// Given a file:// URL handed over by the OS, when resolved, the same
	// checks and encoding apply as for a plain path.
	dir := t.TempDir()
	path := filepath.Join(dir, "deck.pdf")
	if err := os.WriteFile(path, []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := "file://" + path

	got, err := Resolve(raw)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v", raw, err)
	}
	parsed, err := FileURLPath(got)
	if err != nil {
		t.Fatalf("FileURLPath(%q) error = %v", got, err)
	}
	if parsed != path {
		t.Errorf("round trip = %q, want %q", parsed, path)
	}
}

func TestResolveRejectsWithRedactedVisibleErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(t *testing.T) string
		wantErr error
	}{
		{name: "missing file", prepare: func(t *testing.T) string {
			return filepath.Join(t.TempDir(), "gone.html")
		}, wantErr: ErrMissing},
		{name: "directory", prepare: func(t *testing.T) string {
			dir := t.TempDir()
			sub := filepath.Join(dir, "bundle.app")
			if err := os.Mkdir(sub, 0o755); err != nil {
				t.Fatal(err)
			}
			return sub
		}, wantErr: ErrNotRegularFile},
		{name: "unreadable file", prepare: func(t *testing.T) string {
			p := filepath.Join(t.TempDir(), "secret.png")
			if err := os.WriteFile(p, []byte("x"), 0o000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
			return p
		}, wantErr: ErrUnreadable},
		{name: "unsupported extension", prepare: func(t *testing.T) string {
			return writeDoc(t, "installer.dmg")
		}, wantErr: ErrUnsupportedType},
		{name: "file URL with remote host", prepare: func(t *testing.T) string {
			return "file://server.example.com/x.html"
		}, wantErr: ErrUnsupportedType},
		{name: "executable document", prepare: func(t *testing.T) string {
			p := writeDoc(t, "report.html")
			if err := os.Chmod(p, 0o755); err != nil {
				t.Fatal(err)
			}
			return p
		}, wantErr: ErrExecutable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := tc.prepare(t)
			_, err := Resolve(raw)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Resolve() error = %v, want %v", err, tc.wantErr)
			}
			// Diagnostics are redacted: the path itself must not leak,
			// while the category stays visible enough to act on.
			if strings.Contains(err.Error(), raw) && filepath.IsAbs(raw) {
				t.Errorf("error %q leaks the input path", err)
			}
		})
	}
}

// writeDoc creates a readable document with the given extension in a
// fresh temp directory and returns its path.
func writeDoc(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestResolveAcceptsBrowserViewableTypes(t *testing.T) {
	// The allowlist is deliberate: only document types the configured
	// browsers render natively — markup, PDF, SVG, common rasters, and
	// plain text.
	for _, ext := range []string{
		".html", ".htm", ".xhtml", ".pdf", ".svg",
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp",
		".txt",
	} {
		if _, err := Resolve(writeDoc(t, "doc"+ext)); err != nil {
			t.Errorf("Resolve(%q) error = %v, want accepted", ext, err)
		}
	}
}

func TestResolveRejectsNonViewableTypes(t *testing.T) {
	// Anything outside the allowlist is rejected, never opened by
	// recursion into the OS default handler.
	for _, ext := range []string{".md", ".docx", ".zip", ".dmg", ".exe", ".sh", ".json"} {
		if _, err := Resolve(writeDoc(t, "doc"+ext)); !errors.Is(err, ErrUnsupportedType) {
			t.Errorf("Resolve(%q) error = %v, want ErrUnsupportedType", ext, err)
		}
	}
}

func TestResolveRejectsScriptTypesEvenWhenNotExecutable(t *testing.T) {
	// Script types are rejected by the extension rule regardless of the
	// executable bit: they are never launched, never executed, never
	// handed to the OS opener.
	path := filepath.Join(t.TempDir(), "tool.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(path); !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("Resolve() error = %v, want ErrUnsupportedType", err)
	}
}

func TestResolveRejectsExecutableModeRegardlessOfType(t *testing.T) {
	// An executable bit turns the file into code, whatever the
	// extension claims: it is never launched, never executed — it is
	// rejected like any other unsupported input.
	for _, name := range []string{"report.html", "deck.pdf", "image.png"} {
		p := writeDoc(t, name)
		if err := os.Chmod(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := Resolve(p); !errors.Is(err, ErrExecutable) {
			t.Errorf("Resolve(%q) error = %v, want ErrExecutable", name, err)
		}
	}
}
