---
id: TASK-0014
title: Package and install the NixOS desktop handler
status: todo
depends_on: [TASK-0011]
tags: [linux, packaging]
---

## Problem
Users need persistent desktop integration beyond an experiment or shell command.

## Outcome
NixOS with Sway (Wayland) has a reproducible Nix package and documented declarative desktop integration.

## Acceptance criteria
- Provide a flake package that includes executable and valid desktop entry declaring HTTP/HTTPS handling with safe desktop Exec argument behavior; build offline at runtime without mutable installation scripts.
- Document one supported declarative default-handler setup using the integration chosen in TASK-0001; do not create both NixOS and Home Manager modules unless required.
- Use NixOS experiment evidence for browser resolution; avoid user config bound to a particular Nix store hash and prove rebuild/upgrade behavior.
- Config lookup is independent of launching app working directory and interactive shell PATH.
- Provide explicit opt-in declarative default-handler change, removal and previous-handler restoration behavior; do not mutate managed defaults behind Nix configuration.
- Surface invalid config and launch failures when invoked outside a terminal.
- Unsupported package formats and desktops are documented rather than implicitly promised.

## Verification
Test generated launch behavior and install paths in temporary directories. Run real external-app click, warm-browser, and failure smoke cases on the agreed desktop.

## Non-goals
Other distributions, Flatpak/Snap, imperative mutation of Nix-managed settings, or routing logic in packaging expressions.
