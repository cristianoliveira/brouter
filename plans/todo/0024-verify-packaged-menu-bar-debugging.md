---
id: TASK-0024
title: Verify packaged macOS menu-bar and diagnostic behavior
status: doing
depends_on: [TASK-0022, TASK-0023]
tags: [macos, acceptance, packaging]
---

## Problem
Native UI tests or passing CLI checks alone do not prove a packaged menu-bar handler survives real macOS URL events, app updates and failure paths.

## Outcome
The source-built and Nix-packaged Apple Silicon app expose the same small menu/debug feature without regressing routing or privacy.

## Acceptance criteria
- Build source bundle and aarch64-darwin Nix package; verify required icon assets are bundled, arm64 binaries, final signature and existing metadata/eligibility remain valid.
- On real macOS, confirm menu icon visibility in light/dark appearance, accessible label, keyboard navigation, one item per running instance and no unwanted focus/Dock presence.
- With fixed benign URLs verify cold handler plus cold browser, repeated warm URL delivery while menu is open, observed destination and ordered activity entries. Record foreground-child lifetime without blocking the UI.
- Verify Quit/relaunch behavior and distinguish app-running state from default-handler selection. Defaults stay untouched unless user explicitly selects the app.
- Exercise safe isolated directly observed preflight/spawn failures, clear/retention and missing config/log reveal behavior. Downstream config/browser failures remain outcome unknown in the menu rather than falsely successful; existing file diagnostics are separate. No modification of user's real config, no historical raw log exposure and no URL/secret leakage.
- Verify CLI commands remain headless and retain existing output/exit behavior; regression-check Linux packaging without claiming a Linux tray feature.
- Document how to inspect recent activity, what dispatch/unknown means, memory-only retention, and how to reach existing file diagnostics. Installing a rebuilt bundle is an explicit user action.

## Verification
Kelly reports exact revision and actual GUI observations; user attestation is sufficient for their own session. Separate passing automated/package checks from unavailable graphical evidence. Use a short manual checklist, not another probe framework. Report untested cases honestly; a docs PR merge alone does not close acceptance.

## Non-goals
Nix-darwin system activation/removal retest, changing defaults, login items, Intel support, Linux UI, automatic installation, or private browsing-history capture.
