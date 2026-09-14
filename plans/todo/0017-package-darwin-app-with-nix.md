---
id: TASK-0017
title: Package the macOS app through the Nix flake
status: todo
depends_on: [TASK-0013, TASK-0014]
priority: normal
tags: [nix, macos, packaging]
---

## Problem
The current flake exposes a Darwin handler package but installs a Linux wrapper and desktop entry. A nix-darwin user cannot install the real macOS URL-handler app from it.

## Outcome
On aarch64-darwin, the public handler package contains a functional BrouterHandler.app and CLI built reproducibly from pinned inputs.

## Acceptance criteria
- Dispatch the existing public brouter-handler/default package outputs by platform: preserve Linux behavior and produce the macOS app on aarch64-darwin. Do not advertise an Intel handler as supported.
- Install the bundle at $out/Applications/BrouterHandler.app with its embedded Go executable and native URL-event layer; retain a usable bin/brouter CLI.
- Use existing native layer and routing code unchanged. Reuse build conventions where practical; do not introduce a second routing implementation.
- Pin dependencies/toolchain, supply native build dependencies through Nix, and build in the sandbox without network fetches during build or writes outside declared outputs.
- Both Mach-O executables are arm64, including builds started under a translated shell. Bundle identifiers, metadata and ad-hoc signature are valid after all fixups.
- No Linux .desktop or D-Bus handler wrapper in the Darwin output. No browser registration, default changes, user-config creation or home-directory writes during build.
- Document supported local build/signing limits; do not claim Developer ID signing, notarization or cross-machine installation until verified.

## Verification
Before changes, capture the missing Darwin bundle as a failing packaging assertion. Verify package contents structurally, executable architecture and signature, CLI behavior, and repeated build reproducibility on Apple Silicon. Regression-check existing Linux output through a supported builder; unavailable Linux build evidence remains explicit. Add local commands and equivalent targeted CI checks rather than document-string tests.

## Non-goals
nix-darwin module, activation hooks, default-browser management, Intel/universal packaging, app updates, and browser logic changes.

## Dependency note
TASK-0013/0014 implementation PRs are merged, but their board cards may need lead reconciliation. Do not redo those implementations or declare platform smoke complete solely because PRs merged.
