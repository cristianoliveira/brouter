---
id: TASK-0031
title: Plan scripted-routing compatibility and diagnostics tests
status: todo
depends_on: [TASK-0029, TASK-0030]
tags: [lua, tests, diagnostics]
---

# Plan scripted-routing compatibility and diagnostics tests

## Problem
A scripted-routing feature can appear correct while changing static routing, leaking URL data, or making failures indistinguishable from successful browser delivery.

## Outcome
A bounded, engine-neutral test matrix makes compatibility, privacy, determinism, and diagnostic limits executable before runtime implementation.

## Acceptance criteria
- [ ] Cover static-only regression, script defer/target/failure/timeout, unknown target, malformed URL, repeated events, fixed time, concurrency, and deterministic ordering, plus edits taking effect on the next URL without restarting.
- [ ] Assert `explain` and diagnostics contain only safe categories and never full URLs, credentials, query/fragment data, script text, or private target/profile/path values.
- [ ] Separate script decision evidence from browser/process/page-load outcomes; downstream fire-and-forget results remain unknown.
- [ ] Define supported local/CI platform matrix, pinned fixtures, timeout/resource-boundary tests, and explicit unavailable/UNTESTED cases, including in-flight snapshot stability and no stale-cache behavior.
- [ ] Keep the matrix compatible with current CLI output/exit contracts and a no-script configuration. Test current TOML reload behavior and future script/config cases for symlink replacement, atomic rename, partial writes, unstable reads, missing files, and non-atomic cross-file edits; require visible load failure without last-known-good or wrong-target fallback.

## Verification
Review the matrix against the approved contract and proposed integration traces. No runtime, dependency, or production test harness is added.

## Non-goals
Implementing Lua, adding a dependency, changing config, claiming browser/page-load success, or expanding platform support.
