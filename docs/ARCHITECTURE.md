# Architecture

## Layout

Folder roles follow the guardrails template (domain / lib / cli / infra /
fixtures) mapped onto Go conventions; packages are created only when code
needs them — no empty scaffolding:

- `cmd/brouter` — CLI entry point and composition root (cli). Owns
  process concerns and wiring. Contains no domain routing logic.
- `internal/domain` — pure routing decisions (domain), including parsed
  installed-web-app scope matching and precedence.
- `internal/infra` — platform, config, and process adapters (infra), including
  the injected macOS/Linux installed-web-app catalog and launcher adapters.
- Shared helpers (lib) live in `internal/` packages only when a second
  consumer appears.
- Test fixtures use colocated `testdata/` directories next to the code
  under test, never a separate fixtures tree.
- `scripts/check.sh` — the quality gate entry point (POSIX shell, by
  decision: no Go program orchestrates or lints Go). Its pinned lint
  configuration lives in `.golangci.yml`; tests in `scripts/check_test.go`
  exercise it with controlled executables.

## Boundary rules

1. `internal/domain` imports the standard library only — never `cmd/`,
   `internal/infra/`, or third-party packages. Domain decisions stay
   testable without filesystem, process, or network access.
2. `internal/infra` may import `internal/domain`, the standard library, and
   third-party libraries for adapters.
3. `cmd/brouter` wires concrete adapters into the domain at startup.
   Adapters enter the domain as narrow interfaces defined where they are
   consumed.
4. Tests are colocated with their package; test fixtures live in colocated
   `testdata/` directories. Deterministic: no clock, network, or
   environment dependence in tests.
5. Shared packages are added only when more than one package needs the
   code. No empty framework scaffolding.

## Current state

`cmd/brouter` exposes `validate` and `explain` diagnostics: validate
reports configuration health with actionable per-field/rule errors;
explain prints the deterministic routing decision for one URL (arg or
stdin), including static rules, installed-app matches, fallback, and an
explicit "no browser was launched" statement. `open` gives explicit route
commands and static rules precedence, then uses the injected installed-app
catalog before the configured default. Exit codes: 0 success, 1 validation
failure, 2 usage error. Routing decisions come from `internal/domain`;
configuration and OS discovery from `internal/infra`.
