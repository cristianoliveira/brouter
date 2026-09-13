#!/bin/sh
# Verify the versioned hooks are installed and intact. Boolean contract:
# exactly "true" on success; "false" plus guidance and nonzero otherwise.
set -u

current=$(git config core.hooksPath 2>/dev/null || true)
if [ "$current" != ".githooks" ]; then
	echo "false"
	echo "hooks not installed (core.hooksPath='${current:-unset}')."
	echo "Install with: make hooks-install"
	exit 1
fi

for hook in pre-commit pre-push commit-msg; do
	if [ ! -x ".githooks/$hook" ]; then
		echo "false"
		echo "missing or non-executable hook: .githooks/$hook"
		exit 1
	fi
done

echo "true"
