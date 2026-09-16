package localfile

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// TASK-0033: local documents are opened by the configured browser
// directly, never routed and never handed back to the OS default
// handler. This package is the whole policy: what counts as a local
// document, which types are browser-viewable, and how a path becomes
// a correctly encoded file URL.

// Sentinel errors keep diagnostics redacted: callers print the error
// text, which names the category but never the path.
var (
	ErrMissing         = errors.New("local document does not exist (path redacted)")
	ErrNotRegularFile  = errors.New("local document is not a regular file (path redacted)")
	ErrUnreadable      = errors.New("local document is not readable (path redacted)")
	ErrUnsupportedType = errors.New("local document type is not supported by browser routing (path redacted)")
)

// viewableExtensions is the deliberate allowlist of document types the
// configured browsers render natively — markup, PDF, SVG, common
// raster images, and plain text. Everything else is rejected with a
// visible error: there is no wildcard and no fallback to the OS
// default opener, so an unsupported file can never recurse.
var viewableExtensions = map[string]bool{
	".html": true,
	".htm":  true,
	".pdf":  true,
	".svg":  true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
	".bmp":  true,
	".txt":  true,
}

// IsLocalFile reports whether raw refers to a local document rather
// than a web URL: a filesystem path or a file:// URL. http(s) URLs and
// anything else with a different scheme are not local files — they
// stay on the normal routing path untouched.
func IsLocalFile(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "file://") {
		return true
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		// Not parseable as a URL at all: treated as a path, where the
		// resolve step decides existence and type.
		return true
	}
	if parsed.Scheme == "" {
		return true
	}
	return false
}

// Resolve validates raw as a local document and returns the file URL
// to hand to a browser. The checks run in order — existence, regular
// file, readability, allowlisted extension — and every failure names
// its category with the path redacted. Relative paths and a leading
// ~ are resolved against the current user's home.
func Resolve(raw string) (string, error) {
	path, err := pathFromInput(raw)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrMissing
		}
		return "", ErrUnreadable
	}
	if !info.Mode().IsRegular() {
		return "", ErrNotRegularFile
	}
	file, err := os.Open(path)
	if err != nil {
		return "", ErrUnreadable
	}
	file.Close()

	ext := strings.ToLower(filepath.Ext(path))
	if !viewableExtensions[ext] {
		return "", ErrUnsupportedType
	}

	return (&url.URL{Scheme: "file", Path: path}).String(), nil
}

// FileURLPath decodes the path of a file URL produced by Resolve,
// closing the encoding round trip for tests and probes.
func FileURLPath(fileURL string) (string, error) {
	parsed, err := url.Parse(fileURL)
	if err != nil {
		return "", err
	}
	return parsed.Path, nil
}

// pathFromInput turns a path or file URL into an absolute filesystem
// path. A file URL must be local — empty host or exactly localhost —
// and its path is taken decoded; anything else is unsupported.
func pathFromInput(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(trimmed), "file://") {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return "", ErrUnsupportedType
		}
		if parsed.Host != "" && parsed.Host != "localhost" {
			return "", ErrUnsupportedType
		}
		if parsed.Path == "" {
			return "", ErrMissing
		}
		trimmed = parsed.Path
	}

	if strings.HasPrefix(trimmed, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", ErrUnreadable
		}
		trimmed = filepath.Join(home, strings.TrimPrefix(trimmed, "~"))
	}
	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		return "", ErrUnreadable
	}
	return absolute, nil
}
