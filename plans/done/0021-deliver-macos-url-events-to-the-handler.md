---
id: TASK-0021
title: Deliver macOS URL events to the handler
status: done
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
- [x] Initialize a Cocoa application before registering the `GURL` Apple Event handler and run the application event loop so a normal LaunchServices `open -a ... <fixed URL>` reaches `ForwardURLs`; preserve argv URL forwarding.
- [x] Add a regression test using a temporary app/log/receiver that exercises a real OS-open event while the handler is already running and after cold launch; no defaults writes, process kills, reinstall, or user-config mutation.
- [x] Keep diagnostics privacy-safe: never write the raw URL, query, or fragment to the handler log on forwarding or spawn failure; retain actionable status/path information without full URL history.
- [x] Preserve current config-path semantics, embedded executable resolution, and visible failure logging; do not add routing logic or browser/default registration.
- [x] Verify with pinned macOS build/tests and a benign fixed URL; document any host-only or GUI evidence limitations honestly.

## Verification

### Automated / QA evidence from merged PR #25

Merged PR #25 (`9ce31a6`, source tip `6c36aab`) initializes `NSApplication` before Apple Event registration and runs its event loop; argv forwarding remains unchanged. Its focused QA report recorded:

- `nix develop --command make macos-handler`: PASS.
- `TestMacosHandlerReceivesOSOpenEvents`: PASS with a temporary registered bundle, cold `open -a`, a second event while the instance was running, recorder receipt, redacted log, isolated environment and graceful cleanup.
- Privacy forwarding with query and fragment: PASS; exact argv was preserved and the raw URL was absent from the shim log.
- Offline `nix build .#brouter-handler`, strict codesign verification and arm64 bundle inspection: PASS.
- `git diff --check` and both GitHub CI gates: PASS.

This evidence does not claim a user's installed `/Applications` bundle, defaults, or user configuration was changed.

### USER-ATTESTED evidence (2026-09-16)

The user reports that the installed handler opened one harmless web URL in the intended browser. This is session-specific user evidence, not agent-observed evidence. The user did not attest cold-versus-warm sequencing, repeated-event ordering, defaults, installation lifecycle, or failure paths; those claims are not made here.

## Historical baseline before PR #25

Before PR #25, the real host's `/Applications/BrouterHandler.app` stayed alive and strict codesign passed, but `/usr/bin/open -a` with `https://example.com/brouter-fixed-check` returned LaunchServices timeout `-1712` and added no forwarding log line. The shim registered `NSAppleEventManager` then ran `NSRunLoop` without initializing `NSApplication`, and its logs included raw URL text. PR #25 addressed these defects.
