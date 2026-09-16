---
id: TASK-0030
title: Integrate the opt-in external routing command
status: done
depends_on: [TASK-0029]
tags: [routing, command, config]
---

# Integrate the opt-in external routing command

## Problem
A user-selected command must extend routing without changing configurations that omit it or hiding command failures behind implicit static fallback.

## Outcome
An explicit TOML command array enables a language-independent route decision before static rules. `open`, `explain`, and `validate` use clear command semantics; no-command configurations remain unchanged.

## Acceptance criteria
- [x] Implement the smallest config shape: `[route_command] command = ["/path/to/program", "arg"]`. Arrays only; reject bare shell strings and empty elements. An absent command means byte-compatible current static routing; do not migrate or rewrite existing TOML.
- [x] Invoke argv directly with no shell interpolation. Pass the original URL bytes plus one LF on stdin; accept one exact target ID or reserved `@default` (explicit defer) with optional terminal LF/CRLF only. Reject extra lines, empty output, control bytes, and malformed responses visibly.
- [x] Preserve pipeline order: an exact target short-circuits static rules; `@default` defers to declaration-order rules and default. Unknown target, timeout, nonzero exit, malformed output, output cap, or unavailable command is a fixed redacted failure with no silent fallback.
- [x] Reject `@default` as a configured target when route command is active. Validate URLs/userinfo/control bytes before spawn. Preserve original bytes for accepted inputs; do not expose credentials under the settled userinfo policy.
- [x] Define `validate` as structure-only and never execute user code. Define `explain` as static-only and report command presence without execution; do not claim external commands are side-effect-free.
- [x] Load current TOML per URL with bounded observed-change detection and selected-byte immutability. Document stationary-partial and independently saved-file limitations plus atomic-save recommendation; no watcher or retry loop.
- [x] Enforce hard process-group/job timeout and bounded stdout/stderr. Keep resident macOS forwarding unchanged except per-event invocation; do not alter browser defaults, LaunchServices, installed apps, or headless behavior when command is absent.

## Verification
Use current TOML fixtures as golden no-command behavior. Add command fixtures for target, `@default`, unknown target, timeout/descendant cleanup, nonzero exit, malformed/extra-line/empty output, output cap, missing executable, control bytes, userinfo, edit/reload, redaction, and validate/explain no-execution.

## Completion evidence

Implemented by merged PR #34 (`6c0393f`, 2026-09-15), including `open`, `validate`, and `explain` integration, absent-command compatibility, explicit `@default` semantics, and deterministic command/config behavior. PR #35 (`ff0baf0`) subsequently fixed relative executable resolution. No live configuration or platform defaults were changed.

## Non-goals
Embedded Lua, JSON framing, jq, sandbox guarantees, arbitrary shell command strings, implicit migration, watcher-owned state, browser outcome claims, or live personal-config edits.
