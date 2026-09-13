#!/bin/sh
# One normal quality gate: pinned golangci-lint, then go build, then
# go test with explicit vet control (TASK-0005 owns vet/static analysis
# beyond the pinned lint configuration). Hooks, watcher, and CI call this
# script.
#
# Contract: success prints exactly "true" (exit 0). Failure prints "false",
# bounded diagnostics, and a pointer to the full log (exit nonzero). The
# log always retains every step's complete output.
set -u

LOG_DIR=.tmp
LOG_FILE=$LOG_DIR/check.log
BOUNDED=20

# Missing prerequisites fail before any step, with actionable setup
# guidance; nothing is ever downloaded implicitly.
missing=""
command -v go >/dev/null 2>&1 || missing="$missing go"
command -v golangci-lint >/dev/null 2>&1 || missing="$missing golangci-lint"
if [ -n "$missing" ]; then
	echo "false"
	echo "missing required tools:$missing"
	echo "Install Go at or above the version in go.mod (https://go.dev/dl/)"
	echo "golangci-lint is pinned in the development shell: nix develop"
	exit 1
fi

# Pin before go resolves the toolchain: GOTOOLCHAIN=local turns a too-old
# local toolchain into an explicit version error instead of a download.
# The fixed locale keeps tool output ordering stable.
GOTOOLCHAIN=local
export GOTOOLCHAIN
LC_ALL=C
export LC_ALL

mkdir -p "$LOG_DIR" || {
	echo "false"
	echo "cannot create log directory $LOG_DIR"
	exit 1
}
: > "$LOG_FILE" || {
	echo "false"
	echo "cannot write log $LOG_FILE"
	exit 1
}

run_step() {
	step_name=$1
	shift
	printf '=== %s\n$ %s\n' "$step_name" "$*" >>"$LOG_FILE"
	step_output=$("$@" 2>&1)
	step_rc=$?
	if [ -n "$step_output" ]; then
		printf '%s\n' "$step_output" >>"$LOG_FILE"
	fi
	if [ "$step_rc" -ne 0 ]; then
		echo "false"
		echo "step \"$step_name\" failed: exit status $step_rc"
		echo "first $BOUNDED line(s) of output:"
		printf '%s\n' "$step_output" | head -n "$BOUNDED"
		echo "full log: $LOG_FILE"
		exit 1
	fi
}

run_step lint golangci-lint run
run_step build go build ./...
run_step test go test -vet=off ./...

echo "true"
