---
id: TASK-0021
title: Deliver macOS URL events to the handler
status: doing
depends_on: []
priority: normal
tags: [macos, events, privacy]
---

# Deliver macOS URL events to the handler

## Problem
The installed macOS URL-handler process stays alive but ordinary LaunchServices URL opens time out because the shim registers Apple Events without initializing a Cocoa application event loop; its diagnostics also log raw URLs.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Initialize a Cocoa application before registering the `GURL` Apple Event handler and run the application event loop so a normal LaunchServices `open -a ... <fixed URL>` reaches `ForwardURLs`; preserve argv URL forwarding.
- [ ] Add a regression test using a temporary app/log/receiver that exercises a real OS-open event while the handler is already running and after cold launch; no defaults writes, process kills, reinstall, or user-config mutation.
- [ ] Keep diagnostics privacy-safe: never write the raw URL, query, or fragment to the handler log on forwarding or spawn failure; retain actionable status/path information without full URL history.
- [ ] Preserve current config-path semantics, embedded executable resolution, and visible failure logging; do not add routing logic or browser/default registration.
- [ ] Verify with pinned macOS build/tests and a benign fixed URL; document any host-only or GUI evidence limitations honestly.

## Verification

Merged PR #25 (`9ce31a6`) added `NSApplication` initialization, the application event loop, URL-free handler logging, and focused Apple-event coverage. On current `origin/main` `be0ccc3`, focused handler/event tests pass with temporary HOME/config paths, including warm forwarding, real Apple-event dispatch through the test harness, mixed URL/document batches, and privacy-safe diagnostics.

Still **UNTESTED** as GUI acceptance: a user-selected/registered installed handler receiving a real LaunchServices `open -a` event in the user's graphical session, cold and warm visible browser destinations, and behavior across a real launchd environment. Manual consent checklist: inspect defaults without writing; use a copied temporary bundle only (or explicitly approve an installed bundle), open a fixed benign URL cold and warm, inspect the forwarding log for receipt without the raw URL, verify the browser destination, then quit gracefully. No defaults, registration, installation, or live config was touched by this reconciliation.

## Notes
Historical baseline before PR #25: `/Applications/BrouterHandler.app` stayed alive and strict codesign passed, but `/usr/bin/open -a` with `https://example.com/brouter-fixed-check` returned LaunchServices timeout `-1712` and added no forwarding log line. The pre-fix shim registered `NSAppleEventManager` then ran `NSRunLoop` without initializing `NSApplication`; its logs also included raw URL text. PR #25 addresses that baseline.
