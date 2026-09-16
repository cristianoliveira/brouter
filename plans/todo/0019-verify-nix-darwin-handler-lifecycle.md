---
id: TASK-0019
title: Verify the nix-darwin application lifecycle on macOS
status: doing
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

## Bounded acceptance evidence (2026-09-16)

Current `origin/main` `be0ccc3` was checked non-mutatingly with temporary HOME/config paths. `nix flake check --no-update-lock-file` and no-link `nix build .#packages.aarch64-darwin.brouter-handler` pass; focused macOS bundle, config-path, wrapper, and forwarding tests pass. No `darwin-rebuild`, install, `lsregister`, defaults write, or live-config operation was run.

Still **UNTESTED**: actual nix-darwin activation/link discovery, LaunchServices registration, default selection, cold/warm browser destination, changed-store-path update, and safe removal/restore. Exact user-consent checklist: record defaults; run `darwin-rebuild switch` with the user's flake; inspect the generated Applications link and new store path; explicitly select the handler; open a fixed benign URL cold/warm and verify the visible destination; rebuild at a changed revision; verify the new embedded binary; restore the prior default; remove through declarative config; rebuild; verify ordinary links. Do not manually delete store paths.

## Non-goals
Intel validation, profile re-certification, notarization, automatic default restoration logic or destructive garbage-collection experiments.
