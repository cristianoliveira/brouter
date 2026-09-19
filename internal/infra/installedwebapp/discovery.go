// Package installedwebapp discovers and launches installed web apps without
// exposing platform metadata to the routing domain. OS metadata is treated as
// untrusted input: scans are bounded, launchers are allowlisted, and no shell
// is ever involved.
package installedwebapp

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unicode"

	domainapp "github.com/cristianoliveira/brouter/internal/domain/installedwebapp"
)

var ErrStaleLaunchPlan = errors.New("installed web app launch plan is stale")

const (
	maxEntries    = 256
	maxFileBytes  = 128 << 10
	maxDesktopLen = 64 << 10
)

// Environment is the filesystem/process seam for deterministic adapter tests.
type Environment struct {
	GOOS         string
	Home         string
	MacRoots     []string
	LinuxAppDirs []string
	ReadDir      func(string) ([]fs.DirEntry, error)
	ReadFile     func(string) ([]byte, error)
	Lstat        func(string) (fs.FileInfo, error)
	Stat         func(string) (fs.FileInfo, error)
	LookPath     func(string) (string, error)
	Run          func(string, ...string) error
	ConvertPlist func(string) ([]byte, error)
}

// Result contains one immutable catalog snapshot and fixed, redacted
// diagnostics. Diagnostics never contain paths, URLs, or metadata values.
type Result struct {
	Catalog     domainapp.Catalog
	Diagnostics []string
}

// Adapter is the injected discovery/launch boundary used by the CLI.
type Adapter struct {
	mu       sync.RWMutex
	env      Environment
	cached   Result
	finger   string
	cacheSet bool
	plans    map[string]launchSpec
}

type launchSpec struct {
	kind       string
	bundleID   string
	argv       []string
	urlIndex   int
	urlFlag    string
	sourcePath string
}

// NewAdapter builds an adapter for the supplied platform environment.
func NewAdapter(env Environment) *Adapter {
	if env.GOOS == "" {
		env.GOOS = runtime.GOOS
	}
	if env.Home == "" {
		env.Home, _ = os.UserHomeDir()
	}
	if env.ReadDir == nil {
		env.ReadDir = os.ReadDir
	}
	if env.ReadFile == nil {
		env.ReadFile = os.ReadFile
	}
	if env.Lstat == nil {
		env.Lstat = os.Lstat
	}
	if env.Stat == nil {
		env.Stat = os.Stat
	}
	if env.LookPath == nil {
		env.LookPath = exec.LookPath
	}
	if env.Run == nil {
		env.Run = func(name string, args ...string) error {
			return exec.Command(name, args...).Run()
		}
	}
	return &Adapter{env: env, plans: make(map[string]launchSpec)}
}

// System returns the production adapter. It intentionally supports only the
// currently supported macOS and Linux baselines.
func System() *Adapter { return NewAdapter(Environment{}) }

// Discover returns a cached catalog when bounded metadata fingerprints are
// unchanged. A missing or changed launcher invalidates the snapshot.
func (a *Adapter) Discover() Result {
	a.mu.Lock()
	defer a.mu.Unlock()
	finger := a.fingerprint()
	if a.cacheSet && finger == a.finger {
		return a.cached
	}
	a.plans = make(map[string]launchSpec)
	var entries []domainapp.Entry
	var diagnostics []string
	switch a.env.GOOS {
	case "darwin":
		entries, diagnostics = a.discoverMac()
	case "linux":
		entries, diagnostics = a.discoverLinux()
	}
	catalog, err := domainapp.NewCatalog(entries)
	if err != nil {
		// A malformed individual entry should have been skipped earlier. This
		// fixed diagnostic is a final defense against serving a partial model.
		catalog = domainapp.Catalog{}
		diagnostics = append(diagnostics, "installed-app catalog rejected malformed metadata")
	}
	result := Result{Catalog: catalog, Diagnostics: stableDiagnostics(diagnostics)}
	a.cached, a.finger, a.cacheSet = result, finger, true
	return result
}

// Launch executes an opaque adapter plan with the clicked URL as data.
func (a *Adapter) Launch(plan domainapp.LaunchPlan, rawURL string) error {
	if err := validateLaunchURL(rawURL); err != nil {
		return err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	spec, ok := a.plans[plan.Token]
	if !ok {
		return ErrStaleLaunchPlan
	}
	if spec.sourcePath != "" {
		if spec.kind == "mac" {
			if !a.safePath(spec.sourcePath) {
				return ErrStaleLaunchPlan
			}
		} else if !a.regular(spec.sourcePath) {
			return ErrStaleLaunchPlan
		}
	}
	switch spec.kind {
	case "mac":
		if spec.bundleID == "" {
			return errors.New("installed web app bundle identity is missing")
		}
		if _, err := a.env.LookPath("open"); err != nil {
			return errors.New("macOS open launcher is unavailable")
		}
		return a.env.Run("open", "-b", spec.bundleID, "--args", rawURL)
	case "linux":
		argv := append([]string(nil), spec.argv...)
		if spec.urlIndex >= 0 {
			if spec.urlFlag != "" {
				argv[spec.urlIndex] = spec.urlFlag + rawURL
			} else {
				argv[spec.urlIndex] = rawURL
			}
		} else {
			argv = append(argv, rawURL)
		}
		if len(argv) == 0 {
			return errors.New("installed web app launcher is empty")
		}
		if filepath.IsAbs(argv[0]) {
			info, err := a.env.Stat(argv[0])
			if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
				return errors.New("installed web app launcher is unavailable")
			}
		} else {
			resolved, err := a.env.LookPath(argv[0])
			if err != nil {
				return errors.New("installed web app launcher is unavailable")
			}
			argv[0] = resolved
		}
		return a.env.Run(argv[0], argv[1:]...)
	default:
		return errors.New("installed web app launch plan is unsupported")
	}
}

func (a *Adapter) discoverMac() ([]domainapp.Entry, []string) {
	roots := a.macRoots()
	var entries []domainapp.Entry
	var diagnostics []string
	count := 0
	for _, root := range roots {
		paths := a.appPaths(root)
		for _, appPath := range paths {
			if count >= maxEntries {
				return entries, append(diagnostics, "installed-app discovery entry cap reached")
			}
			count++
			entry, spec, ok := a.readMacApp(appPath)
			if !ok {
				continue
			}
			a.plans[entry.Launch.Token] = spec
			entries = append(entries, entry)
		}
	}
	return entries, diagnostics
}

func (a *Adapter) readMacApp(appPath string) (domainapp.Entry, launchSpec, bool) {
	if !a.safePath(appPath) || !strings.HasSuffix(appPath, ".app") {
		return domainapp.Entry{}, launchSpec{}, false
	}
	plistPath := filepath.Join(appPath, "Contents", "Info.plist")
	if !a.regular(plistPath) {
		return domainapp.Entry{}, launchSpec{}, false
	}
	data, err := a.env.ReadFile(plistPath)
	if err != nil || len(data) > maxFileBytes {
		return domainapp.Entry{}, launchSpec{}, false
	}
	if bytes.HasPrefix(data, []byte("bplist00")) && a.env.ConvertPlist != nil {
		data, err = a.env.ConvertPlist(plistPath)
		if err != nil || len(data) > maxFileBytes {
			return domainapp.Entry{}, launchSpec{}, false
		}
	}
	values, ok := parsePlistStrings(data)
	if !ok {
		return domainapp.Entry{}, launchSpec{}, false
	}
	bundleID := values["CFBundleIdentifier"]
	shortcut := values["CrAppModeShortcutURL"]
	if bundleID == "" || shortcut == "" || !knownMacBundle(bundleID) {
		return domainapp.Entry{}, launchSpec{}, false
	}
	scope := values["CrAppModeScope"]
	if scope == "" {
		scope = values["Scope"]
	}
	// CrAppModeShortcutURL is normally a launch URL, not an authoritative
	// manifest scope. Product-approved compatibility is limited to a root
	// shortcut: root is the only manifest scope that can contain root, so it
	// cannot broaden matching. Non-root shortcuts without explicit scope are
	// skipped rather than guessed or fetched from the network.
	origin, err := originOf(shortcut)
	if err != nil {
		return domainapp.Entry{}, launchSpec{}, false
	}
	if scope == "" && shortcutHasRootPath(shortcut) {
		scope = origin + "/"
	}
	if scope == "" {
		return domainapp.Entry{}, launchSpec{}, false
	}
	token := "mac:" + bundleID + ":" + appPath
	entry, err := domainapp.NewEntry(bundleID, origin, scope, domainapp.NewLaunchPlan(token))
	if err != nil {
		return domainapp.Entry{}, launchSpec{}, false
	}
	return entry, launchSpec{kind: "mac", bundleID: bundleID, sourcePath: appPath}, true
}

func (a *Adapter) discoverLinux() ([]domainapp.Entry, []string) {
	var entries []domainapp.Entry
	var diagnostics []string
	count := 0
	for _, dir := range a.linuxDirs() {
		items, err := a.env.ReadDir(dir)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				diagnostics = append(diagnostics, "installed-app discovery skipped an unreadable applications directory")
			}
			continue
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Name() < items[j].Name() })
		for _, item := range items {
			if count >= maxEntries {
				return entries, append(diagnostics, "installed-app discovery entry cap reached")
			}
			if item.IsDir() || !strings.HasSuffix(item.Name(), ".desktop") {
				continue
			}
			count++
			path := filepath.Join(dir, item.Name())
			entry, spec, ok := a.readDesktop(path, item.Name())
			if !ok {
				continue
			}
			a.plans[entry.Launch.Token] = spec
			entries = append(entries, entry)
		}
	}
	return entries, diagnostics
}

func (a *Adapter) readDesktop(path, filename string) (domainapp.Entry, launchSpec, bool) {
	if !a.safePath(path) || !a.regular(path) {
		return domainapp.Entry{}, launchSpec{}, false
	}
	data, err := a.env.ReadFile(path)
	if err != nil || len(data) > maxDesktopLen {
		return domainapp.Entry{}, launchSpec{}, false
	}
	fields, ok := parseDesktop(data)
	if !ok || fields["Type"] != "Application" {
		return domainapp.Entry{}, launchSpec{}, false
	}
	execLine := fields["Exec"]
	argv, urlIndex, urlFlag, execURL, ok := parseExec(execLine)
	if !ok || len(argv) == 0 || !knownBrowser(argv[0]) || !trustedExecutable(argv[0]) {
		return domainapp.Entry{}, launchSpec{}, false
	}
	metadataURL := firstNonEmpty(fields, "X-WebApp-URL", "X-WebApp-Url", "X-Chromium-WebApp-URL", "X-Chromium-WebApp-Url")
	scope := firstNonEmpty(fields, "X-WebApp-Scope", "X-WebApp-scope", "X-Chromium-WebApp-Scope", "X-Chromium-WebApp-scope")
	if metadataURL == "" {
		metadataURL = execURL
	}
	if metadataURL == "" {
		// App-id-only Chromium launchers do not expose a trustworthy scope.
		return domainapp.Entry{}, launchSpec{}, false
	}
	if scope == "" {
		// A start/launch URL is not an authoritative scope.
		return domainapp.Entry{}, launchSpec{}, false
	}
	origin, err := originOf(metadataURL)
	if err != nil {
		return domainapp.Entry{}, launchSpec{}, false
	}
	id := firstNonEmpty(fields, "X-WebApp-ID", "X-Chromium-WebApp-ID")
	if id == "" {
		id = filename
	}
	token := "linux:" + path
	entry, err := domainapp.NewEntry(id, origin, scope, domainapp.NewLaunchPlan(token))
	if err != nil {
		return domainapp.Entry{}, launchSpec{}, false
	}
	return entry, launchSpec{kind: "linux", argv: argv, urlIndex: urlIndex, urlFlag: urlFlag, sourcePath: path}, true
}

func (a *Adapter) appPaths(root string) []string {
	items, err := a.env.ReadDir(root)
	if err != nil {
		return nil
	}
	var paths []string
	for _, item := range items {
		if strings.HasSuffix(item.Name(), ".app") && !item.IsDir() {
			continue
		}
		if strings.HasSuffix(item.Name(), ".app") {
			paths = append(paths, filepath.Join(root, item.Name()))
		}
	}
	return paths
}

func (a *Adapter) macRoots() []string {
	if len(a.env.MacRoots) > 0 {
		return unique(a.env.MacRoots)
	}
	roots := []string{"/Applications", "/Applications/Chrome Apps.localized", "/Applications/Brave Browser Apps.localized"}
	if a.env.Home != "" {
		roots = append(roots,
			filepath.Join(a.env.Home, "Applications"),
			filepath.Join(a.env.Home, "Applications", "Chrome Apps.localized"),
			filepath.Join(a.env.Home, "Applications", "Brave Browser Apps.localized"))
	}
	return unique(roots)
}

func (a *Adapter) linuxDirs() []string {
	if len(a.env.LinuxAppDirs) > 0 {
		return unique(a.env.LinuxAppDirs)
	}
	var dirs []string
	if a.env.Home != "" {
		dirs = append(dirs, filepath.Join(a.env.Home, ".local", "share", "applications"))
	}
	dirs = append(dirs, "/usr/local/share/applications", "/usr/share/applications", "/run/current-system/sw/share/applications")
	return unique(dirs)
}

func (a *Adapter) fingerprint() string {
	var parts []string
	if a.env.GOOS == "darwin" {
		for _, root := range a.macRoots() {
			parts = append(parts, a.fingerprintDir(root, ".app")...)
		}
	} else if a.env.GOOS == "linux" {
		for _, root := range a.linuxDirs() {
			parts = append(parts, a.fingerprintDir(root, ".desktop")...)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

func (a *Adapter) fingerprintDir(root, suffix string) []string {
	items, err := a.env.ReadDir(root)
	if err != nil {
		return []string{root + ":missing"}
	}
	parts := []string{root}
	for _, item := range items {
		if !strings.HasSuffix(item.Name(), suffix) {
			continue
		}
		path := filepath.Join(root, item.Name())
		info, err := a.env.Stat(path)
		if err != nil {
			parts = append(parts, path+":missing")
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%t:%d:%d", path, info.IsDir(), info.Size(), info.ModTime().UnixNano()))
		if suffix == ".app" && info.IsDir() {
			plist := filepath.Join(path, "Contents", "Info.plist")
			if plistInfo, plistErr := a.env.Stat(plist); plistErr == nil {
				parts = append(parts, fmt.Sprintf("%s:%d:%d", plist, plistInfo.Size(), plistInfo.ModTime().UnixNano()))
			} else {
				parts = append(parts, plist+":missing")
			}
		}
	}
	return parts
}

func (a *Adapter) safePath(path string) bool {
	info, err := a.env.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	return true
}

func (a *Adapter) regular(path string) bool {
	info, err := a.env.Lstat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() <= maxFileBytes
}

func parseDesktop(data []byte) (map[string]string, bool) {
	fields := make(map[string]string)
	group := ""
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSuffix(raw, "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			group = line[1 : len(line)-1]
			continue
		}
		if group != "Desktop Entry" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || !validDesktopKey(key) {
			return nil, false
		}
		fields[key] = desktopUnescape(value)
	}
	return fields, group == "Desktop Entry"
}

func parseExec(line string) ([]string, int, string, string, bool) {
	var argv []string
	var token strings.Builder
	quoted := false
	escaped := false
	flush := func() {
		if token.Len() > 0 {
			argv = append(argv, token.String())
			token.Reset()
		}
	}
	for _, r := range line {
		switch {
		case escaped:
			if r == '%' || unicode.IsControl(r) {
				return nil, -1, "", "", false
			}
			token.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '"':
			quoted = !quoted
		case unicode.IsSpace(r) && !quoted:
			flush()
		case r == '%' || unicode.IsControl(r):
			return nil, -1, "", "", false
		case r == '\'' && !quoted:
			return nil, -1, "", "", false
		default:
			token.WriteRune(r)
		}
	}
	if escaped || quoted {
		return nil, -1, "", "", false
	}
	flush()
	if len(argv) == 0 {
		return nil, -1, "", "", false
	}
	urlIndex := -1
	urlFlag := ""
	execURL := ""
	for i, arg := range argv[1:] {
		index := i + 1
		var candidate, flag string
		switch {
		case strings.HasPrefix(arg, "--app="):
			candidate, flag = strings.TrimPrefix(arg, "--app="), "--app="
		case isHTTPURL(arg):
			candidate = arg
		}
		if candidate == "" {
			continue
		}
		if !isHTTPURL(candidate) || urlIndex >= 0 {
			return nil, -1, "", "", false
		}
		urlIndex, urlFlag, execURL = index, flag, candidate
	}
	return argv, urlIndex, urlFlag, execURL, true
}

func parsePlistStrings(data []byte) (map[string]string, bool) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	values := make(map[string]string)
	var key string
	inDict := false
	depth := 0
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return values, inDict
			}
			return nil, false
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "dict":
				depth++
				if depth > 16 {
					return nil, false
				}
				inDict = true
			case "key":
				var value string
				if err := decoder.DecodeElement(&value, &element); err != nil {
					return nil, false
				}
				key = value
			case "string":
				var value string
				if err := decoder.DecodeElement(&value, &element); err != nil {
					return nil, false
				}
				if key != "" {
					values[key] = value
					key = ""
				}
			}
		case xml.EndElement:
			if element.Name.Local == "dict" {
				depth--
			}
		}
	}
}

func validateLaunchURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("installed web app cannot launch this URL")
	}
	return nil
}

func shortcutHasRootPath(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Path == "" || parsed.Path == "/") && parsed.RawPath == ""
}

func originOf(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("web app metadata has an invalid URL")
	}
	parsed.Path, parsed.RawPath, parsed.RawQuery, parsed.Fragment = "", "", "", ""
	return parsed.String(), nil
}

func knownBrowser(executable string) bool {
	base := strings.ToLower(filepath.Base(executable))
	switch base {
	case "brave", "brave-browser", "chrome", "chromium", "chromium-browser", "google-chrome", "google-chrome-stable":
		return true
	default:
		return false
	}
}

func knownMacBundle(bundleID string) bool {
	return strings.HasPrefix(bundleID, "com.google.Chrome.app.") || strings.HasPrefix(bundleID, "com.brave.Browser.app.")
}

func trustedExecutable(executable string) bool {
	return filepath.IsAbs(executable) || !strings.ContainsAny(executable, `/\\`)
}

func isHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.User == nil && parsed.Hostname() != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func validDesktopKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-') {
			return false
		}
	}
	return true
}

func desktopUnescape(value string) string {
	var out strings.Builder
	escaped := false
	for _, r := range value {
		if escaped {
			switch r {
			case 's':
				out.WriteByte(' ')
			case 'n':
				out.WriteByte('\n')
			case 't':
				out.WriteByte('\t')
			case 'r':
				out.WriteByte('\r')
			default:
				out.WriteRune(r)
			}
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		out.WriteRune(r)
	}
	if escaped {
		out.WriteByte('\\')
	}
	return out.String()
}

func firstNonEmpty(fields map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fields[key]); value != "" {
			return value
		}
	}
	return ""
}

func stableDiagnostics(values []string) []string {
	sort.Strings(values)
	return values
}

func unique(values []string) []string {
	seen := make(map[string]bool, len(values))
	var result []string
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
