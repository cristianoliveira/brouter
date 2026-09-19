package installedwebapp

import (
	"bytes"
	"path/filepath"
	"strings"

	domainapp "github.com/cristianoliveira/brouter/internal/domain/installedwebapp"
)

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
	values, ok := a.readMacPlist(filepath.Join(appPath, "Contents", "Info.plist"))
	if !ok {
		return domainapp.Entry{}, launchSpec{}, false
	}
	bundleID, origin, scope, ok := macAppMetadata(values)
	if !ok {
		return domainapp.Entry{}, launchSpec{}, false
	}
	token := "mac:" + bundleID + ":" + appPath
	entry, err := domainapp.NewEntry(bundleID, origin, scope, domainapp.NewLaunchPlan(token))
	if err != nil {
		return domainapp.Entry{}, launchSpec{}, false
	}
	plistPath := filepath.Join(appPath, "Contents", "Info.plist")
	return entry, launchSpec{kind: "mac", bundleID: bundleID, sourcePath: appPath, sourceFingerprint: a.fileFingerprint(plistPath)}, true
}

func (a *Adapter) readMacPlist(path string) (map[string]string, bool) {
	if !a.regular(path) {
		return nil, false
	}
	data, err := a.env.ReadFile(path)
	if err != nil || len(data) > maxFileBytes {
		return nil, false
	}
	if bytes.HasPrefix(data, []byte("bplist00")) && a.env.ConvertPlist != nil {
		data, err = a.env.ConvertPlist(path)
		if err != nil || len(data) > maxFileBytes {
			return nil, false
		}
	}
	return parsePlistStrings(data)
}

func macAppMetadata(values map[string]string) (string, string, string, bool) {
	bundleID := values["CFBundleIdentifier"]
	shortcut := values["CrAppModeShortcutURL"]
	if bundleID == "" || shortcut == "" || !knownMacBundle(bundleID) {
		return "", "", "", false
	}
	scope := firstNonEmpty(values, "CrAppModeScope", "Scope")
	origin, err := originOf(shortcut)
	if err != nil {
		return "", "", "", false
	}
	if scope == "" && shortcutHasRootPath(shortcut) {
		scope = origin + "/"
	}
	if scope == "" {
		return "", "", "", false
	}
	return bundleID, origin, scope, true
}
