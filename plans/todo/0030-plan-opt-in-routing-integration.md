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
- [ ] Define behavior for multiple URL events, concurrency, reload/update, missing script, and script changes without introducing implicit state or default/browser changes.
- [ ] Identify compatibility risks and a rollback/defer path; leave final product approval and implementation authorization visible.

## Verification
Use current routing examples as golden behavior and document proposed traces. No config parser or routing code changes.

## Non-goals
Implementing the hook, migrating dotfiles, choosing a runtime, or changing current static routing.
