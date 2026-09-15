#!/bin/sh
# Bootstrap the pinned Lua 5.4.7 source used by the isolated route
# helper. The repository does not vendor the Lua tree; this script
# fetches the official release once, verifies its sha256, and extracts
# it into a git-ignored cache. Nothing is downloaded implicitly: a
# cached copy is verified and reused, and an offline bootstrap fails
# with actionable guidance instead of downloading.
#
# Contract: exit 0 and print the extracted source directory when the
# cache is ready; exit nonzero with bounded diagnostics otherwise.
set -eu

LUA_VERSION=5.4.7
LUA_URL="https://www.lua.org/ftp/lua-${LUA_VERSION}.tar.gz"
LUA_SHA256=9fbf5e28ef86c69858f6d3d34eccc32e911c1a28b4120ff3e84aaa70cfbf1e30

repo_root=$(cd "$(dirname "$0")/.." && pwd)
cache_dir=${BROUTER_LUA_CACHE:-"$repo_root/.cache/lua-${LUA_VERSION}"}
marker=$cache_dir/.pin-ok

if [ -f "$marker" ] && [ -f "$cache_dir/src/lua.h" ]; then
	echo "$cache_dir/src"
	exit 0
fi

if ! command -v curl >/dev/null 2>&1; then
	echo "curl is required to bootstrap Lua ${LUA_VERSION}" >&2
	exit 1
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "fetching lua-${LUA_VERSION} (${LUA_URL})" >&2
if ! curl -fsSL "$LUA_URL" -o "$tmp/lua.tar.gz"; then
	echo "download failed. For offline builds use nix (nix develop /" >&2
	echo "nix build), or run this script once with network access." >&2
	exit 1
fi

actual=$(shasum -a 256 "$tmp/lua.tar.gz" | awk '{print $1}')
if [ "$actual" != "$LUA_SHA256" ]; then
	echo "sha256 mismatch: got $actual, want $LUA_SHA256" >&2
	exit 1
fi

mkdir -p "$cache_dir"
tar xzf "$tmp/lua.tar.gz" -C "$cache_dir" --strip-components=1

echo "$LUA_SHA256" > "$marker"
echo "$cache_dir/src"
