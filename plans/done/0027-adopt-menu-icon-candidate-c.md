---
id: TASK-0027
title: Adopt the selected Brouter menu-bar icon
status: done
depends_on: []
tags: [macos, icon]
---

# Adopt the selected Brouter menu-bar icon

## Problem
The current menu-bar item still uses a generic system branch symbol even though the user selected preview candidate C, a recognizable lowercase `b` with one continuous curved arrow stroke and an ascending stem.

## Outcome
The existing macOS menu-bar item uses candidate C while preserving template monochrome rendering, accessibility, event handling, and package integrity.

## Acceptance criteria
- [ ] Use the selected `candidate-c-ascending-stem.svg` from the preview deliverable as the sole source mark; do not add alternate marks or broad branding changes.
- [ ] Bundle/load the asset through the existing AppKit status item without changing the NSApplication/event loop, Recent Activity behavior, URL routing, or headless CLI.
- [ ] Preserve template-style monochrome behavior and accessible `Brouter` label/tooltip; do not rely on color, add Dock presence, steal focus, or change defaults.
- [ ] Verify actual menu-bar-size legibility and light/dark rendering on Apple Silicon, plus source/Nix package asset inclusion, arm64 binaries, and strict final signature.
- [ ] Keep preview assets and comparison history intact; final integration must not mutate user config, LaunchServices defaults, or installed `/Applications` copies.

## Verification
Run focused native/package tests and both source/Nix builds. Inspect the selected asset at actual size and enlarged light/dark previews. User selection is candidate C; no further mark choice is introduced by implementation.

## Non-goals
Recent Activity changes, Lua runtime/config work, Linux tray support, login items, browser chooser, app-wide branding, default-browser changes, or installed-app replacement.

## Notes
PR30 preview candidate C: `docs/assets/menu-bar-icon-previews/candidate-c-ascending-stem.svg`. PR30 remains the provenance for the selected asset; the implementation delivered the asset through PR31. Static asset/package checks passed, while live GUI light/dark/menu interaction remains explicitly untested because replacing/launching the installed app was not authorized.
