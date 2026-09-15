---
id: TASK-0031
title: Implement Lua compatibility and diagnostics tests
status: todo
depends_on: [TASK-0029, TASK-0030]
tags: [lua, tests, diagnostics]
---

# Implement Lua compatibility and diagnostics tests

## Problem
A scripted-routing feature can appear correct while changing static routing, leaking URL data, or confusing script failures with browser delivery.

## Outcome
A focused contract-test suite proves compatibility, deterministic context, capability denial, privacy, resource limits, and live-edit behavior before Lua is enabled for personal configuration.

## Acceptance criteria
- [ ] Cover no-script regression plus defer, valid target, unknown target, runtime error, timeout, malformed script, missing script, and invalid/unstable snapshot cases.
- [ ] Assert exact URL-field normalization, original preservation, one-sample UTC helpers, immutable per-URL context, deterministic results, and concurrency/in-flight snapshot stability.
- [ ] Assert no filesystem/network/process/environment/package/debug capability and no URL, credentials, script text, target, profile, or private path in diagnostics or Recent Activity.
- [ ] Test TOML/script edits, symlink replacement, atomic rename, partial writes, next-URL reload without restart, and the documented non-atomic cross-file boundary.
- [ ] Separate script decision evidence from browser/process/page-load outcomes; preserve current CLI exit/output and no-script behavior.

## Verification
Run focused Go/native tests, race tests, source/Nix package checks, and available supported-platform checks. Record unavailable GUI/Linux/resource runs as UNTESTED; no user-script or live-config test may write to the real dotfiles target.

## Non-goals
Claiming browser/page-load success, broad platform support, implicit migration, or changing the personal config before TASK-0032.
