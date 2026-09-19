// Package installedwebapp contains the platform-neutral model and matching
// rules for installed web applications. Discovery and launch remain in
// infrastructure packages; the launch plan is intentionally opaque here.
package installedwebapp

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// LaunchPlan is an opaque handle owned by the discovery adapter. The domain
// never interprets Token; only the adapter that created it may launch it.
type LaunchPlan struct {
	Token string
}

// NewLaunchPlan creates an opaque adapter handle. Empty handles are rejected
// when the containing entry is validated.
func NewLaunchPlan(token string) LaunchPlan { return LaunchPlan{Token: token} }

// Entry describes one installed web app without exposing platform metadata.
type Entry struct {
	ID     string
	Origin url.URL
	Scope  url.URL
	Launch LaunchPlan
}

// NewEntry validates and normalizes app metadata before it reaches routing.
func NewEntry(id, originRaw, scopeRaw string, launch LaunchPlan) (Entry, error) {
	if err := validateEntryIdentity(id, launch); err != nil {
		return Entry{}, err
	}
	origin, err := normalizedOrigin(id, originRaw)
	if err != nil {
		return Entry{}, err
	}
	scope, err := normalizedScope(id, origin, scopeRaw)
	if err != nil {
		return Entry{}, err
	}
	return Entry{ID: id, Origin: origin, Scope: scope, Launch: launch}, nil
}

func validateEntryIdentity(id string, launch LaunchPlan) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("installed web app id is empty")
	}
	if launch.Token == "" {
		return fmt.Errorf("installed web app %q has no launch plan", id)
	}
	return nil
}

func normalizedOrigin(id, raw string) (url.URL, error) {
	origin, err := parseMetadataURL(raw, "origin")
	if err != nil {
		return url.URL{}, fmt.Errorf("installed web app %q: %w", id, err)
	}
	if origin.Path != "" && origin.Path != "/" {
		return url.URL{}, fmt.Errorf("installed web app %q: origin must not contain a path", id)
	}
	origin.Path, origin.RawPath, origin.RawQuery, origin.Fragment = "", "", "", ""
	return origin, nil
}

func normalizedScope(id string, origin url.URL, raw string) (url.URL, error) {
	scope, err := parseMetadataURL(raw, "scope")
	if err != nil {
		return url.URL{}, fmt.Errorf("installed web app %q: %w", id, err)
	}
	if !sameOrigin(origin, scope) {
		return url.URL{}, fmt.Errorf("installed web app %q: scope has a different origin", id)
	}
	if scope.RawQuery != "" || scope.Fragment != "" {
		return url.URL{}, fmt.Errorf("installed web app %q: scope must not contain query or fragment", id)
	}
	scope.Path, scope.RawPath, err = normalizeScopePath(scope)
	if err != nil {
		return url.URL{}, fmt.Errorf("installed web app %q: %w", id, err)
	}
	scope.RawQuery, scope.Fragment = "", ""
	return scope, nil
}

// Catalog is an immutable, deterministically ordered installed-app snapshot.
type Catalog struct {
	entries []Entry
}

// NewCatalog validates and snapshots entries. Enumeration order never affects
// equal-scope selection.
func NewCatalog(entries []Entry) (Catalog, error) {
	copyEntries := append([]Entry(nil), entries...)
	for i, entry := range copyEntries {
		validated, err := NewEntry(entry.ID, entry.Origin.String(), entry.Scope.String(), entry.Launch)
		if err != nil {
			return Catalog{}, err
		}
		copyEntries[i] = validated
	}
	sort.Slice(copyEntries, func(i, j int) bool {
		left, right := copyEntries[i], copyEntries[j]
		leftLen, rightLen := len(left.Scope.EscapedPath()), len(right.Scope.EscapedPath())
		if leftLen != rightLen {
			return leftLen > rightLen
		}
		if left.Scope.Path != right.Scope.Path {
			return left.Scope.Path < right.Scope.Path
		}
		if left.ID != right.ID {
			return left.ID < right.ID
		}
		return left.Launch.Token < right.Launch.Token
	})
	return Catalog{entries: copyEntries}, nil
}

// Match selects the most specific same-origin scope. Credentials are rejected
// before matching so they cannot be confused with a hostname.
func (c Catalog) Match(rawURL string) (Entry, bool, error) {
	target, err := parseRequestURL(rawURL)
	if err != nil {
		return Entry{}, false, err
	}
	for _, entry := range c.entries {
		if !sameOrigin(entry.Origin, target) {
			continue
		}
		if scopeMatches(entry.Scope.EscapedPath(), target.EscapedPath()) {
			return entry, true, nil
		}
	}
	return Entry{}, false, nil
}

// Entries returns a copy for adapter diagnostics and tests.
func (c Catalog) Entries() []Entry { return append([]Entry(nil), c.entries...) }

func parseMetadataURL(raw, label string) (url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return url.URL{}, fmt.Errorf("invalid %s metadata", label)
	}
	if parsed.User != nil {
		return url.URL{}, fmt.Errorf("%s metadata must not contain credentials", label)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return url.URL{}, fmt.Errorf("%s metadata must use http or https", label)
	}
	if parsed.Hostname() == "" || parsed.Host == "" {
		return url.URL{}, fmt.Errorf("%s metadata has no host", label)
	}
	if parsed.Port() != "" {
		port, portErr := strconv.ParseUint(parsed.Port(), 10, 16)
		if portErr != nil || port == 0 {
			return url.URL{}, fmt.Errorf("%s metadata has an invalid port", label)
		}
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = normalizedHost(parsed.Hostname(), parsed.Port(), parsed.Scheme)
	return *parsed, nil
}

func parseRequestURL(raw string) (url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return url.URL{}, fmt.Errorf("invalid url: input redacted")
	}
	if parsed.User != nil {
		return url.URL{}, fmt.Errorf("invalid url: userinfo credentials are not supported")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return url.URL{}, fmt.Errorf("invalid url: unsupported scheme")
	}
	if parsed.Hostname() == "" || parsed.Host == "" {
		return url.URL{}, fmt.Errorf("invalid url: empty host")
	}
	if parsed.Port() != "" {
		port, portErr := strconv.ParseUint(parsed.Port(), 10, 16)
		if portErr != nil || port == 0 {
			return url.URL{}, fmt.Errorf("invalid url: invalid port")
		}
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = normalizedHost(parsed.Hostname(), parsed.Port(), parsed.Scheme)
	return *parsed, nil
}

func normalizeScopePath(scope url.URL) (string, string, error) {
	path := scope.EscapedPath()
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		return "", "", fmt.Errorf("scope path must be absolute")
	}
	if strings.Contains(path, "%") {
		return "", "", fmt.Errorf("scope path contains unsupported percent encoding")
	}
	if strings.Contains(path, "//") || strings.Contains(path, "/./") || strings.Contains(path, "/../") || strings.HasSuffix(path, "/..") || strings.HasSuffix(path, "/.") {
		return "", "", fmt.Errorf("scope path contains ambiguous dot or empty segments")
	}
	if path != "/" {
		path = strings.TrimRight(path, "/") + "/"
	}
	return path, "", nil
}

func sameOrigin(left, right url.URL) bool {
	return left.Scheme == right.Scheme && left.Host == right.Host
}

func normalizedHost(host, port, scheme string) string {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if strings.Contains(host, ":") {
		if port == "" {
			return "[" + host + "]"
		}
		return "[" + host + "]:" + port
	}
	if port == "" {
		return host
	}
	return host + ":" + port
}

func scopeMatches(scope, target string) bool {
	if scope == "/" {
		return strings.HasPrefix(target, "/")
	}
	base := strings.TrimSuffix(scope, "/")
	return target == base || strings.HasPrefix(target, scope)
}
