# Architecture

## Layout

- `cmd/brouter` — CLI entry point and composition root. Owns process
  concerns and wiring. Contains no domain routing logic.
- `internal/domain` — pure routing decisions (config-independent inputs,
  matching, fallback). Planned for the routing tasks; created when code
  needs it.
- `internal/infra` — platform, config, and process adapters (file loading,
  browser launching). Planned; created when code needs it.

- `tools/` — development-only tooling invoked by the gate and by developers;
  never imported by product code. Standard-library only.

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

Only `cmd/brouter` exists: help/success and invalid-invocation paths, with
tests. Routing, configuration, and launching arrive in later tasks and will
populate the packages above.
