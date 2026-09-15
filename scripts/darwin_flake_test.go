package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The flake package is platform-dispatched: on Darwin it ships the CLI
// plus a macOS app bundle at $out/Applications/BrouterHandler.app; on
// Linux it ships the CLI plus the desktop wrapper and entry. These
// tests verify the built output through nix where nix is available.

func nixAvailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix is not available on this host")
	}
}

func buildFlakePackage(t *testing.T) string {
	t.Helper()
	outLink := filepath.Join(t.TempDir(), "result")
	build := exec.Command("nix", "build", ".#brouter-handler", "--out-link", outLink)
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("nix build .#brouter-handler failed: %v\n%s", err, out)
	}
	return outLink
}

func TestDarwinFlakePackageShipsAppAndCLI(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the Darwin flake output can only be built on darwin")
	}
	nixAvailable(t)

	out := buildFlakePackage(t)

	// The app bundle lives at $out/Applications and is complete:
	// metadata, the native shim, and the embedded CLI.
	app := filepath.Join(out, "Applications", "BrouterHandler.app")
	for _, artifact := range []string{
		"Contents/Info.plist",
		"Contents/MacOS/BrouterHandler",
		"Contents/MacOS/brouter",
		"Contents/Resources/menu-icon.png",
		"Contents/Resources/menu-icon@2x.png",
	} {
		assertBundleArtifact(t, app, artifact)
	}

	// The CLI stays available as bin/brouter for terminal use.
	assertExecutable(t, filepath.Join(out, "bin", "brouter"))

	// No Linux artifacts leak into the Darwin output.
	for _, leaked := range []string{
		"bin/brouter-handler",
		"share/applications/brouter-handler.desktop",
	} {
		if _, err := os.Stat(filepath.Join(out, leaked)); err == nil {
			t.Errorf("Darwin output contains the Linux artifact %s", leaked)
		}
	}

	assertDropdownEligibilityMetadata(t, filepath.Join(app, "Contents/Info.plist"))

	// Deterministic arm64 target and an ad-hoc signature on both
	// executables, matching the hand-built bundle contract.
	for _, binary := range []string{
		filepath.Join(app, "Contents/MacOS/BrouterHandler"),
		filepath.Join(app, "Contents/MacOS/brouter"),
	} {
		assertArm64AndSigned(t, binary)
	}

	// The whole bundle must carry a valid seal: symlinked bundle
	// contents invalidate it ("a sealed resource is missing or
	// invalid") even when every binary verifies on its own.
	if out, err := exec.Command("codesign", "--verify", "--deep", "--strict", app).CombinedOutput(); err != nil {
		t.Errorf("bundle signature invalid: %v\n%s", err, out)
	}
}

func assertBundleArtifact(t *testing.T, app, artifact string) {
	t.Helper()
	full := filepath.Join(app, artifact)
	info, err := os.Stat(full)
	if err != nil {
		t.Fatalf("bundle artifact %s missing: %v", artifact, err)
	}
	executable := strings.HasSuffix(artifact, "BrouterHandler") ||
		strings.HasSuffix(artifact, "brouter")
	if executable && info.Mode()&0o111 == 0 {
		t.Errorf("%s is not executable", artifact)
	}
}

func assertDropdownEligibilityMetadata(t *testing.T, plistPath string) {
	t.Helper()
	plist := readFileOrFatal(t, plistPath)
	for _, marker := range []string{
		"<string>http</string>",
		"<string>https</string>",
		"public.html",
		"public.xhtml",
	} {
		if !strings.Contains(plist, marker) {
			t.Errorf("Info.plist is missing the dropdown-eligibility marker %q", marker)
		}
	}
	if out, err := exec.Command("plutil", "-lint", plistPath).CombinedOutput(); err != nil {
		t.Errorf("Info.plist invalid: %v\n%s", err, out)
	}
}

func assertArm64AndSigned(t *testing.T, binary string) {
	t.Helper()
	archs, err := exec.Command("lipo", "-archs", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("lipo failed for %s: %v\n%s", binary, err, archs)
	}
	if fields := strings.Fields(string(archs)); len(fields) != 1 || fields[0] != "arm64" {
		t.Errorf("%s architectures = %q, want exactly [arm64]", binary, archs)
	}
	if out, err := exec.Command("codesign", "--verify", binary).CombinedOutput(); err != nil {
		t.Errorf("%s is not properly signed: %v\n%s", binary, err, out)
	}
}

func TestLinuxFlakePackageStillEvaluates(t *testing.T) {
	nixAvailable(t)

	// Regression guard for the Linux output: the dispatch must keep the
	// x86_64-linux package evaluable with its desktop integration.
	// This is evaluation-only: building the Linux output requires a
	// Linux host, which this darwin machine does not have, so actual
	// Linux build evidence is recorded there (see docs) rather than
	// here.
	eval := exec.Command("nix", "eval", ".#packages.x86_64-linux.brouter-handler.name", "--raw")
	eval.Dir = ".."
	if out, err := eval.CombinedOutput(); err != nil {
		t.Fatalf("Linux package no longer evaluates: %v\n%s", err, out)
	}

	// x86_64-darwin must not be an advertised package: the Darwin
	// handler is arm64-only.
	if out, err := exec.Command("nix", "eval",
		".#packages.x86_64-darwin.brouter-handler.name", "--raw").CombinedOutput(); err == nil {
		t.Errorf("x86_64-darwin package is still advertised: %s", out)
	}
}

func assertExecutable(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("%s missing: %v", path, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("%s is not executable", path)
	}
}
