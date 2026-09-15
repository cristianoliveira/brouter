---
id: TASK-0031
title: Test the external routing command contract and diagnostics
status: todo
depends_on: [TASK-0029, TASK-0030]
tags: [routing, command, tests, diagnostics]
---

# Test the external routing command contract and diagnostics

## Problem
A language-independent command can break static routing, leak URL data, hang the handler, or produce ambiguous outcomes unless its protocol and trust boundary are tested directly.

## Outcome
Contract tests prove no-command compatibility, deterministic bounded command behavior, exact target validation, redaction, and the distinction between explicit defer and visible command failure.

## Acceptance criteria
- [ ] Cover absent command, explicit null, valid target, unknown target, nonzero exit, timeout/kill, malformed JSON, trailing output, wrong version, missing executable, and stdout/stderr/output-size limits.
- [ ] Assert exact JSON context fields, one clock sample, immutable per-URL config bytes, next-URL reload, observed unstable-read rejection, and documented stationary-partial/independent-save limitations.
- [ ] Assert direct argv execution without shell interpolation, no accidental environment/working-directory assumptions, fixed redacted diagnostics, and no URL/credential/command stderr leakage to logs or Recent Activity.
- [ ] Test `validate` never runs user code and exercise the chosen `explain` semantics. Document and test that external commands are trusted and may have side effects; do not assert a Lua-like sandbox.
- [ ] Preserve current static routing and CLI exit/output contracts when the command is absent; separate routing decision evidence from browser/process/page-load outcomes.

## Verification
Run focused Go tests, race tests, source/Nix package checks, and available supported-platform checks. Record unavailable GUI/runtime evidence as UNTESTED. No test may write to the real dotfiles target or invoke an unreviewed user command.

## Non-goals
Embedded Lua, sandbox claims, implicit migration, browser/page-load success claims, or changing the personal configuration before TASK-0032.
