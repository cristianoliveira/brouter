# Local development commands. TASK-0004 wires hooks, watcher, and CI to
# `make check`; TASK-0005 adds separate format/static-analysis/race/
# coverage/security/architecture commands. Do not expand these targets.

.PHONY: check build test lint hooks-install hooks-check hooks-uninstall

# The one normal lint/type-and-test gate.
# Contract: success prints exactly "true"; failure prints "false", bounded
# diagnostics, and exits nonzero. Full log: .tmp/check.log
check:
	@sh scripts/check.sh

build:
	@go build ./...

# go vet is deliberately excluded from the gate (TASK-0005 owns static
# analysis); -vet=off keeps the gate exactly build+tests, independent of
# the toolchain's implicit vet defaults.
test:
	@go test -vet=off ./...

lint:
	@golangci-lint run

# Versioned hooks: install once per clone; hooks/watcher/CI all run the
# same `make check` gate. Install refuses to overwrite existing hook
# configuration (see scripts/hooks-install.sh).
hooks-install:
	@sh scripts/hooks-install.sh

hooks-check:
	@sh scripts/hooks-check.sh

hooks-uninstall:
	@sh scripts/hooks-uninstall.sh
