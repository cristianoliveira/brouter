---
id: TASK-0005
title: Add explicit Go safety coverage and architecture checks
status: done
depends_on: [TASK-0003]
tags: [guardrails, security]
---

## Problem
Unit tests alone do not reveal race conditions, vulnerable dependencies, or eroded domain boundaries.

## Outcome
Separate runnable commands expose release risks without slowing every normal gate.

## Acceptance criteria
- Add separate format-check, static-analysis (`go vet` and any pinned analyzer not already run by the normal gate), race, coverage, security (govulncheck), and architecture targets. Do not duplicate the pinned `golangci-lint` invocation or its `funlen`/`cyclop` rules from TASK-0003.
- Formatting checks do not mutate source; offer a separate fix command.
- Enforce a documented coverage budget for routing/config behavior and a no-unexplained-regression rule. Publish coverage per relevant package, not only an aggregate.
- Enforce domain import boundaries using Go package metadata; permit standard-library dependencies, reject platform/infrastructure imports.
- Missing prerequisites and security-network failures are visible failures, never a clean scan.
- Document which checks are manual and which the release gate requires, with local equivalents and tool versions.

## Verification
Demonstrate checks on the skeleton and controlled failure fixtures where applicable; report current coverage without inventing future coverage.

## Non-goals
Run these checks inside make check or add brittle source/document regex tests.
