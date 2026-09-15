---
id: TASK-0029
title: Specify the safe Go-to-Lua bridge seams
status: todo
depends_on: [TASK-0028]
tags: [lua, design, security]
---

# Specify the safe Go-to-Lua bridge seams

## Problem
The TASK-0025 contract is engine-agnostic, so implementation needs explicit Go seams for context construction, allowlisted utilities, result validation, and failure mapping before any runtime code is written.

## Outcome
An implementation-ready, engine-neutral bridge proposal preserves URL and time semantics without exposing mutable host state.

## Acceptance criteria
- [ ] Define Go-side interfaces/types for `route(ctx)`, exact URL fields and original bytes, one-sample UTC clock, target-name validation, and safe diagnostic categories.
- [ ] Define ownership, mutability, per-invocation lifetime, concurrency, cancellation, timeout, and error boundaries; no ambient globals or host environment access.
- [ ] Map every contract happy/unhappy case to a deterministic bridge result, including nil/defer, unknown target, script error, timeout, and downstream outcome unknown.
- [ ] Provide test seams for fixed URL, clock, IDs, and runtime errors without selecting an engine or changing current static-rule behavior.

## Verification
Review the proposal against `docs/route-script-contract.md` and write contract-level examples. No source implementation or dependency change is included.

## Non-goals
Runtime selection, Lua embedding, config schema activation, process sandbox implementation, or routing behavior changes.
