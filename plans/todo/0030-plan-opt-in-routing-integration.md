---
id: TASK-0030
title: Implement opt-in Lua routing integration
status: todo
depends_on: [TASK-0029]
tags: [lua, routing, config]
---

# Implement opt-in Lua routing integration

## Problem
Adding scripted routing can change precedence, fallback, explain output, or existing configurations unless the hook is explicitly opt-in and shares the current loader and router pipeline.

## Outcome
An opt-in TOML shape enables a Lua route hook without changing configurations that omit it, while `open`, `explain`, and `validate` use the same current-file snapshot and safe diagnostics.

## Acceptance criteria
- [ ] Implement and document the smallest explicit script activation shape; absent script means byte-compatible current static routing. Do not migrate or rewrite existing TOML automatically.
- [ ] Preserve pipeline order: script target short-circuits static rules; nil defers to current first-match rules and default. Unknown target, runtime error, timeout, malformed script, or load failure follows the approved visible fail-closed policy without wrong-target or last-known-good fallback.
- [ ] Route `open` and `explain` through the same snapshot/bridge seam; `validate` checks script presence and syntax without executing `route`. Keep CLI exit/output contracts and privacy redaction stable.
- [ ] Support edits for the next URL without restart, with in-flight snapshot stability. Define and test symlink replacement, atomic rename, partial/unstable reads, missing files, and independently edited TOML/script boundaries.
- [ ] Keep resident macOS event forwarding unchanged except for invoking the per-event loader; do not alter URL routing, browser defaults, LaunchServices, installed app state, or headless behavior when script is absent.

## Verification
Use current TOML fixtures as golden no-script behavior. Add opt-in script fixtures for defer, target, error, timeout, unknown target, edit/reload, and redaction. Run focused source/Nix checks on available targets and label unavailable platform evidence.

## Non-goals
Whole-config Lua, implicit migration, watcher-owned mutable state, browser outcome claims, or live personal-config edits.
