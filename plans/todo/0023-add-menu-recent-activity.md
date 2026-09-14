---
id: TASK-0023
title: Show a small privacy-safe recent activity log in the menu
status: todo
depends_on: [TASK-0022]
tags: [macos, diagnostics]
---

## Problem
A running icon cannot explain missing URL delivery or a forwarding failure. Users should not need Terminal for the first diagnostic step.

## Outcome
The menu answers whether a URL event arrived, whether dispatch was attempted, and what observable failure occurred.

## Proposed first-version limits
- Recent Activity submenu: latest 20 entries, bounded field lengths, newest first.
- In-memory for the current handler session; empty state says no activity this session.
- Clear Recent Activity removes these entries only, not existing diagnostic files.
- No full URL, hostname, query, fragment, regex, custom rule/target name, profile identifier or filesystem path in activity entries by default.

## Acceptance criteria
- Store structured allowlisted records: timestamp, stable event kind, session-local monotonic request ID, entry point (OS event or direct argv), and safe error category when present. Inject clock/IDs in tests.
- Record receipt before validation/dispatch, and distinguish dispatch started, observed handler/child failure, and unknown outcome. A spawn or exit code is not proof of browser receipt or page load.
- Capture safe categories only for directly observable preflight failures (missing embedded binary, unavailable config directory) and spawn errors. The current fire-and-forget child does not expose config validation, browser resolution or launch outcomes to the shim: these remain outcome unknown in this slice. Do not parse human stderr or render it as activity.
- Show dispatch started, outcome unknown after successful spawn. Do not wait for a long-lived child, block AppKit, or change CLI waiting semantics. Existing file diagnostics remain available for downstream investigation.
- Do not re-evaluate routing with a second rules engine or duplicate explain invocation. A structured nonblocking child outcome bridge is a separately justified future deliverable, not required for this small menu.
- Menu remains responsive during bursts and long-running launches. Bounded retention evicts oldest entries deterministically; clear does not cancel pending URL forwarding.
- Add Reveal Config and Reveal Diagnostic Log using existing resolved paths, without creating or overwriting files. Missing file gives actionable non-secret feedback. No auto-open of private log contents or export/copy feature by default.
- Existing file diagnostics remain separate: never ingest historical raw logs into the menu or erase them as a side effect of Clear. New events use structured safe fields; no persistent browsing history or telemetry.

## Verification
Tests first for empty/history/clear/eviction, fixed clocks and IDs, repeated events, safe error mapping and untrusted URL/error strings. Use query/fragment/credential/path markers and verify they are absent from every new sink. Exercise real preflight/spawn failures; isolated invalid-config and missing-browser cases must not produce a false success claim even though their downstream outcomes remain unknown in the menu. Never mutate user dotfiles. Record what the menu can and cannot observe.

## Non-goals
Searchable history, full-URL debug mode, log export, automatic file rotation retrofit, remote telemetry, metrics/tracing stack, profile editor, or proof a webpage loaded.
