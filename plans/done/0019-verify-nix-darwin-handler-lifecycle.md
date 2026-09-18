---
id: TASK-0019
title: Verify the nix-darwin application lifecycle on macOS
status: done
depends_on: [TASK-0017, TASK-0018]
priority: normal
tags: [nix-darwin, acceptance]
---

## Problem
A successful Nix build does not prove LaunchServices can discover and launch an app through nix-darwin links or after a store-path-changing update.

## Outcome
A real Apple Silicon nix-darwin installation has bounded evidence for installation, URL routing, rebuilds and safe removal.

## Acceptance criteria
- On a real Apple Silicon macOS session, record OS/nix-darwin version, package revision and application discovery path. User-attested results count; no mandatory recording/artifact bureaucracy.
- Inspect and record previous defaults first. Installation/rebuild does not silently change them. Obtain explicit permission before selecting the handler; preserve user work and config.
- Test a benign external-app URL with the handler and selected browser cold, then repeated URLs while running; use the same browser target and existing config. Confirm visible destination, not only process exit.
- Verify behavior from the graphical session without shell PATH/cwd assumptions. Invalid config and missing-browser failures are discoverable without persisting full URLs/secrets.
- Perform an update or controlled changed derivation that yields a new store path. Verify app discovery and forwarding use the new embedded binary without stale registration; preserve config and chosen defaults.
- Restore previous default before removal, remove package through declarative config, and rebuild. Confirm old handler is no longer selected and ordinary links still work. Do not manually delete Nix store paths or unrelated app copies.
- Record signing/Gatekeeper limitations and distinguish same-machine builds from untested transfer/cached-distribution scenarios.

## Verification
Use a short fixed-URL manual checklist, no new stateful probe framework. Report each case passed, failed or untested; request user assistance only when the actual nix-darwin session is unavailable. Mock/CI success is not graphical lifecycle evidence. Fix concrete failures in the relevant packaging/docs PR; do not count PR merge alone as acceptance.

## Closure — USER-ACCEPTED DEFERRAL (2026-09-16)

This task is closed by explicit user triage/deferral, not by a QA pass. No unchecked acceptance criterion was marked complete, and no nix-darwin installation, defaults, or runtime state was changed.

### Verified evidence retained

- Release evidence remains available from the successful `v0.1.0` tag run targeting main SHA `5c7d5d331a16920e8683a3b0fef09d5305f220c5`: [run 35130454143](https://github.com/cristianoliveira/brouter/actions/runs/35130454143).
- The release was explicitly/manual-published as `isDraft=false`, `publishedAt=2026-09-16T18:48:36Z`: [v0.1.0](https://github.com/cristianoliveira/brouter/releases/tag/v0.1.0). It contains the four platform archives plus `SHA256SUMS` (all five expected assets); downloaded checksums pass, archive `VERSION` files are `0.1.0`, and content roots inspect correctly.
- The macOS app archive contains `_CodeSignature`, `CFBundleShortVersionString=0.1.0`, and `CFBundleVersion=010`; the workflow log records ad-hoc signing. This is packaging evidence, not nix-darwin lifecycle evidence.
- The user attested that the installed handler opened a harmless web URL, HTML, and PDF in the intended browser, showed menu activity, and Quit. This session evidence was not agent- or Kelly-observed.

### Deferred / unverified gates

- Real nix-darwin installation, update/rebuild with a changed store path, removal, restore, and ordinary-link behavior remain unverified.
- Previous defaults, explicit handler selection, application discovery, stale registration, and selected-browser cold/warm behavior remain unverified.
- Graphical-session behavior without shell PATH/cwd assumptions, failure discoverability, transfer/cached-distribution behavior, and signing/Gatekeeper limitations remain unverified. The release body’s stale “unpublished DRAFT” wording was not changed under this scope.

## Non-goals
Intel validation, profile re-certification, notarization, automatic default restoration logic or destructive garbage-collection experiments.
