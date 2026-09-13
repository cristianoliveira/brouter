#!/bin/sh
# Install the versioned hooks for this repository by pointing
# core.hooksPath at .githooks. Never overwrites existing hook
# configuration: foreign hooksPath or active legacy hooks cause a refusal
# with recovery guidance instead of a silent replacement.
set -u

repo=$(git rev-parse --show-toplevel 2>/dev/null) || {
	echo "not inside a git repository" >&2
	exit 1
}

current=$(git config core.hooksPath 2>/dev/null || true)
if [ "$current" = ".githooks" ]; then
	echo "hooks already installed (core.hooksPath=.githooks)"
	exit 0
fi

if [ -n "$current" ]; then
	echo "refusing: core.hooksPath is already set to '$current'" >&2
	echo "review and migrate those hooks into .githooks/, then rerun: make hooks-install" >&2
	exit 1
fi

active=""
for hook in "$repo"/.git/hooks/*; do
	[ -f "$hook" ] || continue
	case "$hook" in
	*.sample) continue ;;
	esac
	if [ -x "$hook" ]; then
		active="$active $(basename "$hook")"
	fi
done
if [ -n "$active" ]; then
	echo "refusing: .git/hooks contains active hook(s):$active" >&2
	echo "review and migrate them into .githooks/, then rerun: make hooks-install" >&2
	exit 1
fi

for hook in pre-commit pre-push commit-msg; do
	if [ ! -x "$repo/.githooks/$hook" ]; then
		echo "refusing: .githooks/$hook is missing or not executable" >&2
		exit 1
	fi
done

git config core.hooksPath .githooks
echo "hooks installed: core.hooksPath=.githooks (pre-commit, pre-push run 'make check'; commit-msg enforces conventional commits)"
