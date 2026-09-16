---
id: TASK-0020
title: Standardize Unix config path
status: done
depends_on: []
priority: normal
tags: [config, cli, macos, nixos]
---

# Standardize Unix config path

## Problem
The CLI and GUI handlers resolve different default configuration locations on macOS, making a valid ~/.config/brouter/config.toml unusable by macOS launches and forcing platform-specific migration.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [x] `--config` remains highest precedence and accepts an explicit path without changing it.
- [x] With no explicit path, use `$XDG_CONFIG_HOME/brouter/config.toml` only when `XDG_CONFIG_HOME` is absolute; otherwise use `$HOME/.config/brouter/config.toml` on macOS and Linux.
- [x] CLI, native macOS handler, and NixOS wrapper share the same default-path semantics without depending on interactive shell startup; no implicit fallback to the old macOS Application Support path.
- [x] Tests cover unset, absolute, and relative `XDG_CONFIG_HOME`, missing `HOME`, explicit overrides, and all handler paths; no config files are moved or created.
- [x] Documentation explains manual migration from the old macOS path and that GUI-launched processes may not inherit shell environment variables.

## Verification

Merged PR #21 (`29000b2`) implemented the shared resolver, native macOS shim, NixOS wrapper, focused path matrix, explicit override coverage, and migration documentation. On current `origin/main` (`be0ccc3`), the config-path and handler-path tests pass in an isolated temporary HOME; no config files were moved or created. Real GUI environment inheritance remains a manual concern, not a reason to alter the contract.

## Notes
Reserved task ID 0020 because TASK-0017 through TASK-0019 are reserved for the nix-darwin planning set.
