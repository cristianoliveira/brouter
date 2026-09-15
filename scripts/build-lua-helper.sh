#!/usr/bin/env bash
# Build the isolated C Lua route-script helper (TASK-0029).
#
# The helper compiles against the vendored, pinned Lua 5.4.7 sources
# (third-party/lua-5.4.7 — see its PIN.md) and is a standalone binary:
# the Go router stays CGO-free and spawns this process per routed URL.
# Interpreter libraries that would add host capability surfaces
# (os, io, package loader, debug, math, utf8) are excluded from the
# link entirely; the helper opens only base/table/string.
set -euo pipefail
cd "$(dirname "$0")/.."

luasrc="third-party/lua-5.4.7/src"
out="${1:-dist/brouter-lua-helper}"
arch="$(uname -m)"

command -v clang >/dev/null 2>&1 || { echo "clang is required" >&2; exit 1; }

# Excluded from the link: standalone interpreters, the library
# initializer (the helper registers libs itself), and every library
# that would expose a host capability surface.
exclude=" lua.c luac.c onelua.c linit.c loadlib.c liolib.c loslib.c ldblib.c lmathlib.c lutf8lib.c "

sources=()
for f in "$luasrc"/*.c; do
	base="$(basename "$f")"
	case " $exclude " in
		*" $base "*) continue ;;
	esac
	sources+=("$f")
done

mkdir -p "$(dirname "$out")"
clang -arch "$arch" -O2 -I "$luasrc" \
	native/lua-helper/lua_helper.c "${sources[@]}" -lm -o "$out"

echo "built $out"
