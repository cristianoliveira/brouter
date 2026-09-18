---
id: TASK-0024
title: Verify packaged macOS menu-bar and diagnostic behavior
status: done
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

## Closure — USER-ACCEPTED DEFERRAL (2026-09-16)

This task is closed by explicit user triage/deferral, not by a QA pass. No unchecked acceptance criterion was marked complete, and no packaged app, defaults, configuration, or live release state was changed.

### Verified evidence retained

- The successful `v0.1.0` tag run targeted main SHA `5c7d5d331a16920e8683a3b0fef09d5305f220c5`: [run 35130454143](https://github.com/cristianoliveira/brouter/actions/runs/35130454143).
- The release was explicitly/manual-published as `isDraft=false`, `publishedAt=2026-09-16T18:48:36Z`: [v0.1.0](https://github.com/cristianoliveira/brouter/releases/tag/v0.1.0). All five expected assets are present: the four versioned platform archives and `SHA256SUMS`; downloaded assets pass `sha256sum -c SHA256SUMS`, archive roots and `VERSION=0.1.0` inspect correctly, and the manifest is sorted.
- The macOS app archive contains `_CodeSignature`, `CFBundleShortVersionString=0.1.0`, and `CFBundleVersion=010`; the workflow log records ad-hoc signing. These are packaging facts only.
- The user attested that the installed handler opened a harmless web URL, HTML, and PDF in the intended browser, showed menu activity, and Quit. This session evidence was not agent- or Kelly-observed.

### Deferred / unverified gates

- Light/dark appearance, accessibility label, keyboard navigation, one item per running instance, unwanted focus, and Dock behavior remain unverified.
- Cold/warm ordering, repeated URL delivery while the menu is open, ordered activity entries, foreground-child lifetime, relaunch behavior, and app-running versus default-handler distinction remain unverified.
- Directly observed preflight/spawn failures, clear/retention, missing config/log reveal, downstream config/browser failure presentation, source/Nix package parity, required icons, arm64 binaries, final signature/metadata/eligibility, CLI behavior, and Linux packaging checks remain unverified. The release body’s stale “unpublished DRAFT” wording was not changed under this scope.
- No protected `v*` tag ruleset was found, and no repository settings or user defaults were changed.

## Non-goals
Nix-darwin system activation/removal retest, changing defaults, login items, Intel support, Linux UI, automatic installation, or private browsing-history capture.
