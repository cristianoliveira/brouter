---
id: TASK-0030
title: Plan opt-in scripted routing integration
status: todo
depends_on: [TASK-0029]
tags: [lua, routing, design]
---

# Plan opt-in scripted routing integration

## Problem
Adding scripted routing to the TOML pipeline could accidentally change static-rule precedence, fallback behavior, explain output, or configurations that do not opt in.

## Outcome
A compatibility-first integration proposal defines where an optional script hook could enter the existing pipeline while keeping current configurations unchanged.

## Acceptance criteria
- [ ] Specify an explicit opt-in configuration shape as a proposal only, with validation, path/packaging implications, and no automatic migration; TOML targets and static rules remain authoritative when absent.
- [ ] Show precedence and fallback traces for script nil, valid target, unknown target, timeout, and runtime error, including `explain` behavior and safe diagnostics.
- [ ] Define behavior for multiple URL events, concurrency, and reload/update: each new URL loads current TOML/script files without restart; in-flight work keeps its immutable snapshot; no watcher or compiled stale cache is required; symlink replacement is observed on the next URL.
- [ ] Identify compatibility risks and a rollback/defer path; leave final product approval and implementation authorization visible. Missing, malformed, invalid, or unstable edited files must fail visibly for that URL—never silently use last-known-good or route to a wrong browser—unless a future product decision explicitly changes this policy.

## Verification
Use current routing examples as golden behavior and document proposed traces. Verify current per-launch TOML loading before claiming restart is required. Include dotfile symlink replacement, ordinary edits, atomic rename, partial-write/unstable-read handling, and the practical boundary that independently edited files are not a multi-file atomic transaction. No config parser or routing code changes.

## Non-goals
Implementing the hook, migrating dotfiles, choosing a runtime, or changing current static routing.
