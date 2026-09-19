package installedwebapp

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
	roots := []string{"/Applications/Chrome Apps.localized", "/Applications/Brave Browser Apps.localized"}
	if a.env.Home != "" {
		roots = append(roots,
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

func (a *Adapter) fileFingerprint(path string) string {
	info, err := a.env.Stat(path)
	if err != nil {
		return "missing"
	}
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
}
