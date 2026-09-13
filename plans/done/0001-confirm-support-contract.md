---
id: TASK-0001
title: Record initial support and reversible configuration contract
status: done
depends_on: []
priority: high
tags: [product]
---

## Problem
NixOS desktop integration, macOS CPU support, and matching semantics remain open. Implementing against unstated assumptions risks an unusable first release.

## Outcome
A decision ledger separates confirmed support constraints from reversible implementation defaults and keeps unresolved product choices visible. The Go bootstrap can start without treating proposals as approvals.

## Acceptance criteria
- Record the confirmed Linux baseline: NixOS with Nix-installed browsers on Sway (Wayland); exclude other distributions and Flatpak/Snap initially. Keep Home Manager usage unknown and defer the declarative integration choice.
- Record current macOS with Apple Silicon-first support; Intel is unverified and not initially required. Record the observed local host (arm64/macOS 26.6.2) and provisional macOS 13 arm64 deployment floor.
- Record reversible working defaults: TOML, ordered first-match rules, exact host/subdomain and full-URL regex semantics, explicit config lookup, and explicit fallback target. Label them as unapproved defaults.
- Record the confirmed Go module path `github.com/cristianoliveira/brouter` and provisional Go toolchain (pinned to 1.27.1 by TASK-0003/0004).
- Keep optional profiles as a desired, separately verified capability; leave first-release timing open rather than silently approving or dropping it.
- Keep macOS signing/notarization and any local-only unsigned release concession open.
- Include concrete personal/work routing examples and explicit unsupported cases.

## Verification
The decision ledger is documented in `plans/README.md`; unresolved items remain explicitly labeled. Board readiness is verified with the kanban helper and dependency-cycle check.

## Non-goals
Implementation and broad platform parity.
