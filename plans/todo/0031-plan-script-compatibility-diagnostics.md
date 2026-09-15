---
id: TASK-0031
title: Test the external routing command line contract
status: todo
depends_on: [TASK-0029, TASK-0030]
tags: [routing, command, tests, diagnostics]
---

# Test the external routing command line contract

## Problem
A language-independent command can break static routing, leak URL data, hang the handler, or produce ambiguous outcomes unless its line protocol and trust boundary are tested directly.

## Outcome
Contract tests prove no-command compatibility, deterministic bounded command behavior, exact target validation, redaction, and the distinction between explicit `@default` defer and visible command failure.

## Acceptance criteria
- [ ] Cover absent command, exact target, `@default`, unknown target, nonzero exit, timeout/kill including descendants, malformed/empty output, extra lines, embedded control bytes, CRLF policy, missing executable, and stdout/stderr caps.
- [ ] Assert stdin is exactly original URL bytes plus one LF; reject raw CR/LF/control-byte URLs before spawn and preserve accepted percent-escapes/userinfo policy without shell interpolation.
- [ ] Assert fixed redacted diagnostics: URL, credentials, command output, argv secrets, and private paths never leak to errors or Recent Activity. Validate reserved `@default` collision behavior.
- [ ] Test TOML observed-change rejection and selected-byte immutability, while documenting that bounded checks cannot identify every stationary syntactically valid partial write or guarantee cross-file generations; recommend atomic saves.
- [ ] Test `validate` never executes user code and `explain` remains static-only. Document and test that external commands are trusted and may have filesystem, network, process, environment, and configuration side effects; do not assert a sandbox.
- [ ] Preserve current static routing and CLI exit/output contracts when the command is absent; separate routing evidence from browser/process/page-load outcomes.

## Verification
Run focused Go tests, race tests, source/Nix package checks, and available supported-platform checks. Record unavailable GUI/runtime evidence as UNTESTED. No test may write to the real dotfiles target or invoke an unreviewed user command.

## Non-goals
Embedded Lua, JSON framing, jq, sandbox claims, implicit migration, browser/page-load success claims, or changing the personal configuration before TASK-0032.
