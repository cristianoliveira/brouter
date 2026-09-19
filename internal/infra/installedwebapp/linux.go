package installedwebapp

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	domainapp "github.com/cristianoliveira/brouter/internal/domain/installedwebapp"
)

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
	return a.buildDesktopEntry(path, filename, fields)
}

func (a *Adapter) buildDesktopEntry(path, filename string, fields map[string]string) (domainapp.Entry, launchSpec, bool) {
	argv, urlIndex, urlFlag, execURL, ok := a.desktopLaunch(fields["Exec"])
	if !ok {
		return domainapp.Entry{}, launchSpec{}, false
	}
	metadataURL, scope := desktopURLs(fields, execURL)
	if metadataURL == "" || scope == "" {
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
	return a.newDesktopEntry(path, id, origin, scope, argv, urlIndex, urlFlag)
}

func (a *Adapter) desktopLaunch(line string) ([]string, int, string, string, bool) {
	argv, urlIndex, urlFlag, execURL, ok := parseExec(line)
	if !ok || len(argv) == 0 || !knownBrowser(argv[0]) {
		return nil, -1, "", "", false
	}
	resolved, ok := a.resolveTrustedBrowser(argv[0])
	if !ok {
		return nil, -1, "", "", false
	}
	argv[0] = resolved
	return argv, urlIndex, urlFlag, execURL, true
}

func (a *Adapter) newDesktopEntry(path, id, origin, scope string, argv []string, urlIndex int, urlFlag string) (domainapp.Entry, launchSpec, bool) {
	token := "linux:" + path
	entry, err := domainapp.NewEntry(id, origin, scope, domainapp.NewLaunchPlan(token))
	if err != nil {
		return domainapp.Entry{}, launchSpec{}, false
	}
	return entry, launchSpec{
		kind:                  "linux",
		argv:                  argv,
		urlIndex:              urlIndex,
		urlFlag:               urlFlag,
		sourcePath:            path,
		sourceFingerprint:     a.fileFingerprint(path),
		executablePath:        argv[0],
		executableFingerprint: a.fileFingerprint(argv[0]),
	}, true
}

func desktopURLs(fields map[string]string, execURL string) (string, string) {
	metadataURL := firstNonEmpty(fields, "X-WebApp-URL", "X-WebApp-Url", "X-Chromium-WebApp-URL", "X-Chromium-WebApp-Url")
	if metadataURL == "" {
		metadataURL = execURL
	}
	scope := firstNonEmpty(fields, "X-WebApp-Scope", "X-WebApp-scope", "X-Chromium-WebApp-Scope", "X-Chromium-WebApp-scope")
	return metadataURL, scope
}

func (a *Adapter) resolveTrustedBrowser(executable string) (string, bool) {
	candidate := executable
	if !filepath.IsAbs(candidate) {
		resolved, err := a.env.LookPath(candidate)
		if err != nil {
			return "", false
		}
		candidate = resolved
	}
	if !filepath.IsAbs(candidate) || !knownBrowser(candidate) {
		return "", false
	}
	canonical, err := a.env.EvalSymlinks(candidate)
	if err != nil || !filepath.IsAbs(canonical) || !knownBrowser(canonical) {
		return "", false
	}
	if !allowedLinuxExecutable(canonical) || !a.secureExecutable(canonical) {
		return "", false
	}
	return canonical, true
}

func allowedLinuxExecutable(path string) bool {
	for _, root := range []string{"/usr", "/opt", "/run/current-system/sw", "/nix/store", "/snap"} {
		if pathWithin(root, path) {
			return true
		}
	}
	return false
}

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func (a *Adapter) secureExecutable(path string) bool {
	info, err := a.env.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 || info.Mode()&0o022 != 0 {
		return false
	}
	for dir := filepath.Dir(path); dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		info, err := a.env.Lstat(dir)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || info.Mode()&0o022 != 0 {
			return false
		}
	}
	return true
}
