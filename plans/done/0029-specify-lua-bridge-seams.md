---
id: TASK-0029
title: Implement the external routing command line protocol
status: done
depends_on: [TASK-0028]
tags: [routing, command, security]
---

# Implement the external routing command line protocol

## Problem
Users want routing logic in any language they choose, without embedding a language runtime, JSON tool, or shell-specific convention. The configured runner is trusted local code, so the boundary must be explicit and honest.

## Outcome
Brouter invokes an opt-in executable plus argv directly for each URL and exchanges a minimal line protocol. The earlier Lua and JSON protocols are superseded; preserve their history but do not merge or extend them.

## Protocol (v1)

**Input:** write the accepted URL's original bytes followed by exactly one LF to stdin, then close stdin. Do not JSON-encode, decode, normalize, or shell-escape the payload. Reject raw CR, LF, or other control bytes before spawning the command; preserve the original URL bytes for accepted inputs.

**Output:** accept exactly one configured target ID with no newline, one terminal LF, or one terminal CRLF. The reserved literal `@default` means explicit defer to ordered TOML rules. Empty output, extra lines, embedded CR/LF, malformed bytes, or any other response is a visible failure. Cap stdout and stderr at 4 KiB.

When a route command is active, reject `@default` as a configured target ID so the reserved response cannot collide with a real target. No `jq` or other helper is required.

## Acceptance criteria
- [x] Replace JSON/Lua-specific runtime work with direct argv execution; never interpolate a command through a shell. Support Bash, Python, Lua, or any executable selected by the user through the same command array.
- [x] Enforce a hard 2-second per-URL timeout and terminate the command process group/job, including descendants that inherit stdout/stderr. Bound stdout/stderr at 4 KiB and classify timeout, nonzero exit, output cap, malformed/extra-line output, unavailable command, and unknown target with fixed redacted categories.
- [x] Validate the single output line against the current TOML target IDs. Only exact `@default` defers; every other failure is visible and never silently falls back. Keep downstream browser outcome unknown.
- [x] Reject malformed URL input and raw CR/LF/control bytes before invoking the command. Preserve accepted URL bytes exactly on stdin. Preserve the settled userinfo policy: credentials cause a fixed redacted pre-command failure with no fallback.
- [x] Load current TOML for each URL with bounded observed-change detection. Keep selected config bytes immutable for the URL; document that bounded checks cannot detect every stationary syntactically valid partial write and recommend atomic saves. No watcher or indefinite retry.
- [x] Keep the Go router free of language/runtime dependencies. Do not claim sandboxing: configured commands have the user's host permissions and may read/write files, access network, launch processes, or mutate configuration.
- [x] Provide injectable clock only if the protocol needs it; v1 sends only the original URL, so do not invent a time/context payload. Provide injectable command runner, loader, decoder, and process/error seams for deterministic tests.
- [x] Do not mutate defaults, LaunchServices, installed applications, or the real personal configuration.

## Verification
Add focused line-protocol, timeout/process-group, output-cap, target-validation, redaction, observed-config-change, control-byte, no-command, and static-compatibility tests. Run source/Nix package checks on available targets and label unavailable platform evidence.

## Completion evidence

Implemented and integrated by merged PR #34 (`6c0393f`, 2026-09-15): direct argv protocol, bounded process execution, output/target validation, redaction, config snapshot checks, and focused Go coverage. The Lua/JSON bridge was superseded by this external command boundary; no personal configuration or runtime dependency was added.

## Non-goals
Embedded Lua, JSON framing, jq, runtime sandboxing, arbitrary shell strings, implicit migration, file watchers, silent fallback, browser/page-load claims, or live personal-config changes.
