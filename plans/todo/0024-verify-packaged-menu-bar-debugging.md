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

## USER-ATTESTED evidence (2026-09-16)

The user reports that the installed handler opened a harmless web URL, an HTML document, and a PDF in the intended browser, observed menu activity, and used Quit. This is session-specific user evidence, not agent-observed evidence. It supports intended-browser destination observations for those three opens and a Quit interaction only; it does not attest the remaining acceptance criteria.

## Remaining unverified

- Light/dark appearance, accessible label, keyboard navigation, one item per running instance, unwanted focus, and Dock behavior.
- Cold-versus-warm sequencing, repeated URL delivery while the menu is open, ordered activity entries, and foreground-child lifetime; the user did not assert these details.
- Relaunch behavior or distinction between app-running state and default-handler selection.
- Isolated directly observed preflight/spawn failures, clear/retention behavior, missing config/log reveal behavior, and downstream config/browser failure presentation.
- Source-bundle/Nix-package parity, required icon assets, arm64 binaries, final signature and metadata/eligibility checks.
- CLI regression behavior and Linux packaging checks.

## Non-goals
Nix-darwin system activation/removal retest, changing defaults, login items, Intel support, Linux UI, automatic installation, or private browsing-history capture.
