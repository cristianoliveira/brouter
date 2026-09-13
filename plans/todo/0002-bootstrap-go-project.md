---
id: TASK-0002
title: Bootstrap a minimal Go executable and package boundaries
status: todo
depends_on: [TASK-0001]
priority: high
tags: [go, foundation]
---

## Problem
The repository has no buildable code or repeatable development baseline.

## Outcome
A newcomer can build and run a minimal executable with a documented Go toolchain.

## Acceptance criteria
- Add go.mod using github.com/cristianoliveira/brouter and reproducible dependency/tool version policy; commit go.sum when dependencies exist.
- Provide a minimal pinned Nix development environment for the Go toolchain and project tools; keep runtime packaging in the platform deliverables.
- Add a minimal CLI entry point with a tested help/success and invalid-invocation path.
- Document setup and package boundaries in README, DEVELOPMENT.md, and docs/ARCHITECTURE.md.
- Keep domain behavior free of OS/process/config dependencies; CLI composition owns wiring.
- Colocate Go tests and testdata; create packages only when code needs them.
- Ignore local reports, binaries, and coverage artifacts without ignoring source fixtures.

## Verification
Run focused CLI tests and documented build command. No claimed platform integration yet.

## Non-goals
Router logic, GUI, and speculative shared libraries.
