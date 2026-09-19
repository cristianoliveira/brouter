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
	if !ok || len(argv) == 0 || !knownBrowser(argv[0]) || !trustedExecutable(argv[0]) || !a.launcherAvailable(argv[0]) {
		return nil, -1, "", "", false
	}
	return argv, urlIndex, urlFlag, execURL, true
}

func (a *Adapter) newDesktopEntry(path, id, origin, scope string, argv []string, urlIndex int, urlFlag string) (domainapp.Entry, launchSpec, bool) {
	token := "linux:" + path
	entry, err := domainapp.NewEntry(id, origin, scope, domainapp.NewLaunchPlan(token))
	if err != nil {
		return domainapp.Entry{}, launchSpec{}, false
	}
	return entry, launchSpec{kind: "linux", argv: argv, urlIndex: urlIndex, urlFlag: urlFlag, sourcePath: path, sourceFingerprint: a.fileFingerprint(path)}, true
}

func desktopURLs(fields map[string]string, execURL string) (string, string) {
	metadataURL := firstNonEmpty(fields, "X-WebApp-URL", "X-WebApp-Url", "X-Chromium-WebApp-URL", "X-Chromium-WebApp-Url")
	if metadataURL == "" {
		metadataURL = execURL
	}
	scope := firstNonEmpty(fields, "X-WebApp-Scope", "X-WebApp-scope", "X-Chromium-WebApp-Scope", "X-Chromium-WebApp-scope")
	return metadataURL, scope
}
