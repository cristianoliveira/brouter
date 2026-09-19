package installedwebapp

import (
	"errors"
	"path/filepath"

	domainapp "github.com/cristianoliveira/brouter/internal/domain/installedwebapp"
)

func (a *Adapter) Launch(plan domainapp.LaunchPlan, rawURL string) error {
	if err := validateLaunchURL(rawURL); err != nil {
		return err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	spec, ok := a.plans[plan.Token]
	if !ok || !a.launchSourceExists(spec) {
		return ErrStaleLaunchPlan
	}
	switch spec.kind {
	case "mac":
		return a.launchMac(spec, rawURL)
	case "linux":
		return a.launchLinux(spec, rawURL)
	default:
		return errors.New("installed web app launch plan is unsupported")
	}
}

func (a *Adapter) launchSourceExists(spec launchSpec) bool {
	if spec.sourcePath == "" {
		return true
	}
	if spec.kind == "mac" {
		if !a.safePath(spec.sourcePath) {
			return false
		}
		plistPath := filepath.Join(spec.sourcePath, "Contents", "Info.plist")
		values, ok := a.readMacPlist(plistPath)
		return ok && values["CFBundleIdentifier"] == spec.bundleID && a.macExecutableValid(spec.sourcePath) && a.fileFingerprint(plistPath) == spec.sourceFingerprint
	}
	return a.regular(spec.sourcePath) && a.fileFingerprint(spec.sourcePath) == spec.sourceFingerprint &&
		a.secureExecutable(spec.executablePath) && a.fileFingerprint(spec.executablePath) == spec.executableFingerprint
}

func (a *Adapter) launchMac(spec launchSpec, rawURL string) error {
	if spec.bundleID == "" {
		return errors.New("installed web app bundle identity is missing")
	}
	if _, err := a.env.LookPath("open"); err != nil {
		return errors.New("macOS open launcher is unavailable")
	}
	// The URL is a LaunchServices operand, not an --args value. Chromium's
	// app shim receives this through application:openURLs:, including when
	// the app is already running. Use the verified bundle path rather than
	// bundle-ID lookup so a duplicate registered ID cannot redirect launch.
	return a.env.Run("open", "-a", spec.sourcePath, rawURL)
}

func (a *Adapter) launchLinux(spec launchSpec, rawURL string) error {
	argv := append([]string(nil), spec.argv...)
	if len(argv) == 0 {
		return errors.New("installed web app launcher is empty")
	}
	argv = replaceLaunchURL(argv, spec.urlIndex, spec.urlFlag, rawURL)
	resolved, err := a.resolveLinuxExecutable(argv[0])
	if err != nil {
		return err
	}
	argv[0] = resolved
	return a.env.Run(argv[0], argv[1:]...)
}

func replaceLaunchURL(argv []string, index int, flag, rawURL string) []string {
	if index < 0 {
		return append(argv, rawURL)
	}
	if flag != "" {
		argv[index] = flag + rawURL
	} else {
		argv[index] = rawURL
	}
	return argv
}

func (a *Adapter) resolveLinuxExecutable(executable string) (string, error) {
	if !filepath.IsAbs(executable) || !knownBrowser(executable) || !allowedLinuxExecutable(executable) || !a.secureExecutable(executable) {
		return "", errors.New("installed web app launcher is unavailable")
	}
	return executable, nil
}
