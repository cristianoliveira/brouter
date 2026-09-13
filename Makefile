# Local development commands. TASK-0004 wires hooks, watcher, and CI to
# `make check`; TASK-0005 adds separate format/static-analysis/race/
# coverage/security/architecture commands. Do not expand these targets.

.PHONY: check build test lint hooks-install hooks-check hooks-uninstall fmt-check fmt-fix vet analyze race coverage security architecture

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

# Advanced checks (TASK-0005). All are explicit and OUTSIDE the normal
# gate; release requires all of them, manual runs are welcome anytime.
fmt-check:
	@sh scripts/fmt-check.sh

fmt-fix:
	@sh scripts/fmt-fix.sh

vet:
	@sh scripts/vet.sh

# staticcheck pinned; none of its checks duplicate the gate's funlen/cyclop.
analyze:
	@go run honnef.co/go/tools/cmd/staticcheck@v0.8.1 ./...

race:
	@sh scripts/race.sh

# Per-package floors in scripts/coverage.budget; lowering one is an
# explained regression (rationale required in the commit).
coverage:
	@sh scripts/coverage.sh . scripts/coverage.budget

# Requires network (vulnerability database); failures are visible by design.
security:
	@go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...

architecture:
	@sh scripts/architecture.sh
