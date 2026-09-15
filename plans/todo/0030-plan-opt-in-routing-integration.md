---
id: TASK-0030
title: Integrate the opt-in external routing command
status: todo
depends_on: [TASK-0029]
tags: [routing, command, config]
---

# Integrate the opt-in external routing command

## Problem
A user-selected command must extend routing without changing configurations that omit it or silently hiding command failures.

## Outcome
An explicit TOML command array enables a language-independent route decision before static rules. `open`, `explain`, and `validate` use clearly defined command semantics; no-script configurations remain unchanged.

## Acceptance criteria
- [ ] Implement the smallest explicit config shape for an executable plus argv. An absent command means byte-compatible current static routing; do not migrate or rewrite existing TOML.
- [ ] Preserve pipeline order: command target short-circuits static rules; explicit null defers to declaration-order rules and default. Unknown target, timeout, nonzero exit, malformed output, protocol/version error, or unavailable command is a visible fixed redacted failure with no silent static fallback.
- [ ] Define `validate` as command/config shape and executable-path validation only; it must not execute user code. Define `explain` explicitly (execute the command or report it as unavailable) and preserve CLI output/privacy contracts. Do not claim external commands are side-effect-free.
- [ ] Invoke argv directly with no shell interpolation. Pass one bounded JSON context, cap stdout/stderr, enforce hard timeout, and ensure child termination. Do not expose credentials if the selected URL policy rejects userinfo; otherwise document the trusted-code exposure explicitly.
- [ ] Load current TOML for each URL, retain selected bytes immutably during that URL, support next-URL edits without restart, and document bounded observed-change detection and independently saved-file limitations.
- [ ] Keep resident macOS forwarding unchanged except per-event invocation; do not alter browser defaults, LaunchServices, installed applications, or headless behavior when command is absent.

## Verification
Use current TOML fixtures as golden no-command behavior. Add command fixtures for defer, target, unknown target, timeout, nonzero exit, malformed/trailing output, oversized output, version mismatch, missing executable, edit/reload, redaction, and side-effect boundaries. Run focused source/Nix checks on available targets and label unavailable platform evidence.

## Non-goals
Embedded Lua, sandbox guarantees, arbitrary shell command strings, implicit migration, watcher-owned mutable state, browser outcome claims, or live personal-config edits.
