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
go test ./...             # run all tests
```

A single normal quality gate (`make check`) arrives with TASK-0003; until
then these are the documented commands. Do not add competing gates.

## Conventions

- Tests are colocated with the code they exercise (`*_test.go` next to the
  package) and use colocated `testdata/` fixtures when files are needed.
- Packages are created when code needs them; no empty scaffolding.
- Local artifacts (build output, coverage, reports) are git-ignored; source
  fixtures never are. See `.gitignore`.
