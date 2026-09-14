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

## Notes
Observed baseline on the real host: `/Applications/BrouterHandler.app` stays alive and strict codesign passes, but `/usr/bin/open -a` with `https://example.com/brouter-fixed-check` returned LaunchServices timeout `-1712` and added no forwarding log line. Current `main.m` registers `NSAppleEventManager` then runs `NSRunLoop` without initializing `NSApplication`. Existing logs include raw URL text and must be redacted in this focused fix.

