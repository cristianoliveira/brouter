---
id: TASK-0018
title: Document declarative nix-darwin installation
status: done
depends_on: [TASK-0017]
priority: normal
tags: [nix-darwin, docs]
---

## Problem
A package alone does not tell users how to expose a macOS application through nix-darwin or keep dotfiles configuration across rebuilds.

## Outcome
One minimal, verified nix-darwin example installs the handler without managing browser defaults or requiring Home Manager.

## Acceptance criteria
- Provide a flake input example and environment.systemPackages reference for aarch64-darwin using the actual public package output. Explain flake.lock updates rather than floating runtime downloads.
- Verify installed app discovery in the nix-darwin-managed Applications location. State any explicit registration or app-link step actually needed; do not assume adding systemPackages automatically makes it the default browser.
- Keep TOML at `~/.config/brouter/config.toml` (or an absolute `$XDG_CONFIG_HOME/brouter/config.toml`), outside the immutable application output. Show optional linking to an existing dotfiles file without overwriting user data or requiring Home Manager; do not fall back to the old macOS Application Support path.
- Include config validation using standard Cobra --config, app discovery, explicit user default-browser selection, rebuild/update, removal and restoring the prior default.
- No new custom module or automatic activation scripts unless demonstrated necessary; document the problem before proposing either.
- Warn that Nix store content can be readable by other local users; do not put private routing config or secrets in a package or public example.
- Keep local source-build instructions valid; clearly distinguish Nix-managed app paths from manually copied /Applications bundles.

## Verification
Evaluate/build a minimal nix-darwin configuration using pinned documented inputs. Check commands against the actual output; no grep assertions on prose. Real application discovery/default-selection observations are completed in TASK-0019, not implied by evaluation.

## Non-goals
Managing the user's whole nix-darwin setup, Home Manager module, silently changing defaults, public release publication, or a GUI editor.
