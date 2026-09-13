# Development

## Policy → command → enforcement

Every process rule maps to a command and an enforcement point. Nothing is
prose-only.

| Policy | Command | Enforcement |
|---|---|---|
| One normal gate: lint + type checks + deterministic tests | `make check` | Enforced by versioned hooks (`make hooks-install`), the fzz watcher (`.watch.yaml`), and CI (`.github/workflows/ci.yml`) — all invoke the same command |
| Conventional commits referencing task IDs | see `.githooks/commit-msg` | Enforced by the versioned commit-msg hook (merge/revert exempt) |
| Deterministic formatting via gofmt | `gofmt -l ./...` (check), `gofmt -w` (fix) | Separate command; release enforcement in TASK-0005 |
| Static analysis beyond the pinned lint rules | `go vet ./...` | Separate command; TASK-0005 |
| Race/coverage/security/architecture budgets | TASK-0005 targets | Separate commands; release enforcement |

## Lifecycle: hooks, watcher, CI

One gate command everywhere; nothing competes with it.

```sh
make hooks-install     # once per clone: core.hooksPath=.githooks
make hooks-check       # verify installation (prints true / false + guidance)
make hooks-uninstall   # restore: removes core.hooksPath (nothing was overwritten)
fzz check              # validate .watch.yaml
fzz run gate           # run the gate job once
fzz                    # watch: reruns make check on gate-relevant changes
```

- pre-commit and pre-push run `make check`; commit-msg enforces
  `<type>(<scope>)?: TASK-XXXX <summary>` (merge/revert exempt).
- Install never overwrites: a foreign `core.hooksPath` or active legacy
  hooks in `.git/hooks` cause a refusal naming what was found.
- CI (`.github/workflows/ci.yml`) runs `make check` on Linux and macOS
  with Go from `go.mod` and golangci-lint pinned at 2.13.2 — the same
  versions as the dev shell. Remote CI evidence is pending until a real
  push is observed; until then CI is documented, not claimed.

## Failure modes → recovery

| Failure mode | Recovery |
|---|---|
| `go` missing from PATH | Install from <https://go.dev/dl/> or run `nix develop` |
| `golangci-lint` missing | Run `nix develop` (pinned 2.13.2; never install floating versions) |
| Gate prints `false`, exit nonzero | Read `.tmp/check.log` (full output); fix the first failing step; rerun `make check` |
| `go: requires go >= X` | Local toolchain older than `go.mod`; update the local toolchain or bump the locked dev shell deliberately |
| `golangci-lint version mismatch` | Run `nix develop`, or install the exact pin: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2` |
| commit-msg hook rejects a commit | Rewrite the subject: `<type>(<scope>)?: TASK-XXXX <summary>`; `git commit --amend` for the head commit or `git rebase -i` only on unpushed work |
| pre-commit/pre-push rejects a push | Run `make check`, read `.tmp/check.log`, fix the failing step |
| Hook install refuses (existing hooks) | Migrate the named hooks into `.githooks/`, then rerun `make hooks-install` |
| Hook uninstall refuses (foreign hooksPath) | The other configuration is preserved; remove it manually only if intended: `git config --unset core.hooksPath` |
| Gate blocked mid-run | Rerun `make check`; the log is recreated fresh each run |

## DO NOT

- Do not add checks to the normal gate (formatting, vet, race, coverage,
  security, architecture stay separate; TASK-0005 owns them).
- Do not create competing gates or parallel quality commands.
- Do not bypass the gate (no `--no-verify` style escapes without an
  explicit, recorded decision).
- Do not auto-download tools or toolchains; missing tools fail with
  guidance.
- Do not use destructive git operations (rebase/reset/amend on shared
  branches) unless explicitly requested.

## Setup

The pinned toolchain comes from the Nix flake; `flake.lock` pins the exact
nixpkgs revision, so every machine resolves the same Go and Make versions.

```sh
nix develop
```

Without Nix, use any Go toolchain at or above the minimum declared in
`go.mod`.

## Toolchain policy

- `go.mod` declares the minimum supported Go version (**1.27**); do not
  introduce language or stdlib features beyond it.
- The exact toolchain is pinned at **1.27.1** — the official latest
  stable at the time Cristian set the direction (go.dev/dl, checked
  2026-09-13). The dev shell provides it via `go_1_27` from the locked
  nixpkgs (`flake.lock`); CI pins `go-version: '1.27.1'`. Minimum (what
  may build the project) and pinned toolchain (what we use day to day)
  are distinct on purpose.
- `GOTOOLCHAIN=local` everywhere: a toolchain older than the minimum
  fails with an explicit version error instead of downloading.
- No `toolchain` directive in `go.mod`: it would silently download a
  different toolchain instead of using the pinned one.
- External dependencies: none yet. `go.sum` is committed when the first
  dependency lands; additions need a stated reason.
- Toolchains below the minimum cannot build or test the module — run
  `nix develop` (failure table below covers the symptom).

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
- The gate refuses a golangci-lint whose version differs from the pin
  (2.13.2): drift is reported with both recovery options (`nix develop`
  or the exact `go install` command) instead of risking hooks-pass-
  CI-fails mismatches.

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

### Test style

- Prefer descriptive, behavior-oriented subtests: `t.Run("shows usage on
  stdout", ...)` — subtest names read as specifications.
- Use table-driven tests for input variations; every row exercises the
  same behavior contract.
- Given/When/Then comments where they clarify intent (Given context,
  When action, Then outcome).
- Subtest-name quality has **no automated checker**: standard
  golangci-lint has none, and the pinned configuration deliberately adds
  no regex or custom analyzer for prose. This convention is enforced by
  code review only — do not claim automated enforcement for it.
