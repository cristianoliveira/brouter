#!/usr/bin/env bash
# Build the macOS URL handler bundle at dist/BrouterHandler.app.
#
# The bundle contains the native event shim (native/macos/main.m), the
# Go brouter binary compiled from this repository, and an Info.plist
# declaring the http and https URL schemes. Signing is ad-hoc; see
# docs/macos-handler.md for the honest signing and distribution notes.
set -euo pipefail
cd "$(dirname "$0")/.."

if [[ "$(uname -s)" != "Darwin" ]]; then
	echo "refusing: the macOS handler bundle builds only on macOS" >&2
	exit 1
fi
command -v clang >/dev/null 2>&1 || { echo "clang is required (xcode-select --install or nix develop)" >&2; exit 1; }
command -v go >/dev/null 2>&1 || { echo "go is required" >&2; exit 1; }

app="dist/BrouterHandler.app"
rm -rf "$app"
mkdir -p "$app/Contents/MacOS"

# The bundle targets arm64 explicitly: without these pins, a build
# shell running under Rosetta produces an x86_64 shim beside an arm64
# Go binary. Deterministic single-arch output; see docs/macos-handler.md.
target_arch="arm64"

echo "== building embedded brouter ($target_arch)"
GOOS=darwin GOARCH=$target_arch go build -o "$app/Contents/MacOS/brouter" ./cmd/brouter

echo "== building native event shim ($target_arch)"
clang -arch $target_arch -fobjc-arc -framework Foundation -framework CoreServices -framework AppKit \
	-o "$app/Contents/MacOS/BrouterHandler" \
	native/macos/main.m

	echo "== assembling bundle metadata"
mkdir -p "$app/Contents/Resources"
cp native/macos/Info.plist "$app/Contents/Info.plist"
# User-selected menu icon (TASK-0026 candidate C): 1x/2x template
# monochrome PNGs; see docs/assets/menu-bar-icon-previews/.
cp native/macos/menu-icon.png "$app/Contents/Resources/menu-icon.png"
cp native/macos/menu-icon@2x.png "$app/Contents/Resources/menu-icon@2x.png"

echo "== ad-hoc signing (unsigned binaries are refused by LaunchServices)"
codesign --force --sign - "$app" >/dev/null

echo "built $app"
echo "note: ad-hoc signature; see docs/macos-handler.md before distributing"
