---
id: TASK-0029
title: Implement the isolated Lua bridge and per-URL snapshot loader
status: doing
depends_on: [TASK-0028]
tags: [lua, runtime, security]
---

# Implement the isolated Lua bridge and per-URL snapshot loader

## Problem
The approved Lua contract and feasibility evaluation now need a bounded implementation seam that cannot expose host capabilities or serve stale configuration after edits.

## Outcome
A focused provider/bridge slice evaluates an optional `route(ctx)` through an isolated C Lua 5.4 helper process, with explicit Go-side snapshot and result boundaries. Existing no-script routing remains unchanged.

## Acceptance criteria
- [ ] Pin and build the selected C Lua 5.4.7 helper for the supported source/Nix targets; keep the Go router free of cgo and do not claim unsupported platforms.
- [ ] Expose only the contract fields and helpers: immutable `ctx.url`, one-sample UTC `ctx.utils.epoch()`/`utc()`, and a string-or-nil route result. No filesystem, network, process, environment, package, debug, or shell capability is reachable from Lua.
- [ ] Create one immutable TOML-plus-Lua snapshot per URL. Re-read current files for the next URL without restart; an in-flight request keeps its snapshot. Preserve symlink replacement and atomic-rename behavior; reject missing, malformed, invalid, or unstable reads visibly, with no stale last-known-good or wrong-target fallback.
- [ ] Enforce a wall/instruction timeout and bounded allocation in the helper; map timeout, runtime error, invalid return, unknown target, and load failures to safe categories without URL/script/target leakage. Keep downstream browser outcome unknown.
- [ ] Provide injectable clock, fixed URL, helper result, loader, and process/error seams for deterministic tests. Do not mutate user config, defaults, LaunchServices, or installed applications.
- [ ] Document the C-helper/subprocess tradeoff: strongest available timeout/isolation path from the evaluation, but memory/process controls remain measured only where tests prove them.

## Verification
Add focused unit/contract tests for bridge mapping, capability denial, redaction, snapshot identity, symlink/atomic rename/unstable-read cases, and no-script compatibility. Run source and Nix package checks on available supported targets; label unavailable runs explicitly.

## Non-goals
Whole-config Lua, implicit migration, file watchers, silent fallback, browser/page-load claims, arbitrary host APIs, or changing the live personal dotfiles configuration.
