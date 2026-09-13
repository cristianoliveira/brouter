# Development

## Setup

The pinned toolchain comes from the Nix flake; `flake.lock` pins the exact
nixpkgs revision, so every machine resolves the same Go and Make versions.

```sh
nix develop
```

Without Nix, use any Go toolchain at or above the minimum declared in
`go.mod`.

## Toolchain policy

- `go.mod` declares the minimum supported Go version; do not introduce
  language or stdlib features beyond it.
- The dev shell pins the tool actually used day to day and in CI via
  `flake.lock`. Bump it by updating the lock file deliberately.
- No `toolchain` directive in `go.mod`: it would silently download a
  different toolchain instead of using the pinned one.
- External dependencies: none yet. `go.sum` is committed when the first
  dependency lands; additions need a stated reason.

## Build and test

```sh
go build ./cmd/brouter    # build; outputs ./brouter
go test ./...            # run all tests
```

## Quality gate: `make check`

`make check` is the one normal lint/type-and-test gate. Hooks, the watcher,
and CI call the same command (wired in TASK-0004).

Contract:

- On success stdout is exactly `true` and the exit status is zero.
- On failure stdout begins with `false`, followed by bounded diagnostics
  (first 20 lines of the failing step) and a pointer to the full log at
  `.tmp/check.log`; the exit status is nonzero. The log always retains the
  complete output of every step.
- Steps run in pinned order, failing fast: `lint` → `build` → `test`.
- `test` runs `go test -vet=off ./...`. The implicit vet pass is disabled
  on purpose: go vet is a separate command (TASK-0005), and the gate must
  stay independent of toolchain vet defaults.
- The entry script pins `GOTOOLCHAIN=local` before `go` resolves the
  toolchain (a too-old toolchain fails with an explicit version error
  instead of downloading) and `LC_ALL=C` for stable output ordering.
- Missing prerequisites (`go`, `golangci-lint`) fail with setup guidance;
  nothing is auto-downloaded. `golangci-lint` is a self-contained pinned
  binary and runs with `GOTOOLCHAIN=local` exported, so neither it nor any
  `go` invocation it spawns can trigger a toolchain download.

The gate is a POSIX shell script (`scripts/check.sh`) by decision: no Go
program orchestrates or lints Go. Its behavior is covered by
controlled-executable tests in `scripts/check_test.go` (fake `go` and
`golangci-lint` injected via PATH).

Individual targets mirror the gate steps: `make build`, `make test`,
`make lint`. Formatting, go vet, race, coverage, security, and
architecture checks are separate commands (TASK-0005) and must not be
added to the gate.

### Pinned lint rule set

`golangci-lint` **2.13.2**, provided by the locked Nix development shell
(pinned transitively via `flake.lock`; bump by updating the lock file
deliberately). Configuration lives in `.golangci.yml` with `default: none`
and exactly two enabled linters:

- **funlen** — maximum 100 lines per function. golangci-lint 2.13.2
  always enforces a statement cap and cannot disable it (`statements: 0`
  is ignored), so the cap is documented and aligned at 100 statements:
  a function may reach 100 lines or 100 statements. 90 statements across
  90 lines do not fire; the alignment is proven by
  `TestPinnedLintRulesFireOnControlledViolations`.
- **cyclop** — maximum cyclomatic complexity 10 per function, package
  average 5.0.

Rule semantics are pinned by a controlled fixture test that runs the real
linter and asserts each documented threshold fires (and nothing else does).
go vet and staticcheck-grade analyzers are deliberately not enabled here;
TASK-0005 must not duplicate this invocation. New rules require a
rationale and an explicit change to `.golangci.yml` plus this document.

## Conventions

- Tests are colocated with the code they exercise (`*_test.go` next to the
  package) and use colocated `testdata/` fixtures when files are needed.
- Packages are created when code needs them; no empty scaffolding.
- Local artifacts (build output, coverage, reports) are git-ignored; source
  fixtures never are. See `.gitignore`.
