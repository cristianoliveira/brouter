#!/bin/sh
# Entry point for the one normal quality gate. Hooks, watcher, and CI call
# this script (TASK-0004); it only forwards to the gate harness after
# checking that the Go toolchain exists, so missing tools fail with
# actionable guidance instead of a raw interpreter error.
set -u

if ! command -v go >/dev/null 2>&1; then
	echo "false"
	echo "go: not found in PATH."
	echo "Install Go at or above the version in go.mod (https://go.dev/dl/)"
	echo "or enter the pinned development shell: nix develop"
	exit 1
fi

exec go run ./tools/gate "$@"
