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
  on purpose: static analysis is a separate concern (TASK-0005), and the
  gate must stay exactly build+tests regardless of toolchain vet defaults.
- Toolchain pinning happens in the entry script before `go` resolves the
  toolchain: `scripts/check.sh` exports `GOTOOLCHAIN=local` (so an older
  local toolchain fails with an explicit version error instead of
  downloading), and the harness re-pins the same variable plus `LC_ALL=C`
  for child steps, keeping output ordering stable.

Individual targets mirror the gate steps: `make build`, `make test`,
`make lint`. Formatting, static analysis (go vet), race, coverage,
security, and architecture checks are separate commands (TASK-0005) and
must not be added to the gate.

### Pinned lint rule set (v1, 2026-09-13)

Implemented by `tools/lint` (in-repo, standard library only):

- **L001** — no `fmt.Print`/`fmt.Printf`/`fmt.Println` or builtin
  `print`/`println` outside `cmd/`, `tools/`, and `_test.go`. Library code
  must not write to stdout/stderr; printing belongs to CLI composition.
- **L002** — no empty `interface{}` literals; use `any`.

New rules require a documented rationale and a version bump of this list.

Missing prerequisites fail with setup guidance (`go` via PATH, POSIX
shell); nothing is auto-downloaded. The toolchain comes from `nix develop`
or a local install at or above the `go.mod` minimum.

## Conventions

- Tests are colocated with the code they exercise (`*_test.go` next to the
  package) and use colocated `testdata/` fixtures when files are needed.
- Packages are created when code needs them; no empty scaffolding.
- Local artifacts (build output, coverage, reports) are git-ignored; source
  fixtures never are. See `.gitignore`.
