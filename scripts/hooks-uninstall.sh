#!/bin/sh
# Restore the pre-install hook configuration. Only removes core.hooksPath
# when this repository owns it (.githooks); a foreign value is preserved
# and reported, never destroyed.
set -u

current=$(git config core.hooksPath 2>/dev/null || true)
if [ -z "$current" ]; then
	echo "core.hooksPath was not set; nothing to uninstall"
	exit 0
fi

if [ "$current" != ".githooks" ]; then
	echo "refusing: core.hooksPath is '$current', not managed by this repository" >&2
	echo "remove it manually only if intended: git config --unset core.hooksPath" >&2
	exit 1
fi

git config --unset core.hooksPath
echo "hooks uninstalled (core.hooksPath removed)"
