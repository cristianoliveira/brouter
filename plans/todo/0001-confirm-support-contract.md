---
id: TASK-0001
title: Confirm initial support and configuration contract
status: todo
depends_on: []
priority: high
tags: [product]
---

## Problem
NixOS desktop integration, macOS CPU support, and matching semantics remain open. Implementing against unstated assumptions risks an unusable first release.

## Outcome
A short approved contract separates required behavior from deferred support.

## Acceptance criteria
- Linux baseline is confirmed: NixOS with Nix-installed browsers on Sway (Wayland); exclude other distributions and Flatpak/Snap initially. Confirm preferred declarative integration; Home Manager usage remains unknown.
- Current macOS with Apple Silicon-first support is confirmed; Intel is unverified and not initially required. Local host is arm64/macOS 26.6.2; record an explicit deployment minimum during bootstrap.
- Confirm TOML, first-match order, exact host/subdomain and full-URL regex semantics, config lookup, and fallback behavior.
- Use confirmed Go module path github.com/cristianoliveira/brouter; choose toolchain and confirm whether optional profiles are a first-release requirement.
- Record signing/notarization expectations and any local-only first-release concession.
- Include concrete personal/work routing examples and explicit unsupported cases.

## Verification
Review decisions with user; leave unresolved items visible rather than manufacturing approval.

## Non-goals
Implementation and broad platform parity.
