---
id: TASK-0003
title: Add the single Go lint and test gate
status: todo
depends_on: [TASK-0002]
priority: high
tags: [go, guardrails]
---

## Problem
Without one local quality command, automated checks can disagree or hide failures.

## Outcome
make check provides a deterministic, bounded lint/type-and-test contract.

## Acceptance criteria
- Define local build, test, lint, and check targets; document the exact minimal lint rule set and pin tools.
- Compilation/type checks and deterministic unit tests run in the normal gate; separate go vet/static analysis from this gate. Explicitly control go test's implicit vet behavior.
- Success prints exactly true; failure starts with false, returns nonzero, and bounds diagnostics while retaining a local full log.
- Missing tools fail with actionable setup guidance, without auto-downloading floating versions.
- Tests for the gate exercise successful commands, lint/test failures, and missing prerequisites using controlled executables; no document-string tests.

## Verification
Run focused gate harness tests and one local smoke invocation. Record evidence.

## Non-goals
Formatting, coverage, race testing, security scans, or architecture checks inside the normal gate.
