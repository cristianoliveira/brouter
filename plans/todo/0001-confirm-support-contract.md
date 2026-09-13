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
- Linux distribution/package source is confirmed: NixOS with Nix-installed browsers; exclude other distributions and Flatpak/Snap initially. Confirm desktop/compositor and preferred declarative integration.
- Confirm current macOS baseline and CPU architectures. Local host is arm64/macOS 26.6.2; Apple Silicon first with Intel unverified is proposed, not yet approved.
- Confirm TOML, first-match order, exact host/subdomain and full-URL regex semantics, config lookup, and fallback behavior.
- Use confirmed Go module path github.com/cristianoliveira/brouter; choose toolchain and confirm whether optional profiles are a first-release requirement.
- Record signing/notarization expectations and any local-only first-release concession.
- Include concrete personal/work routing examples and explicit unsupported cases.

## Verification
Review decisions with user; leave unresolved items visible rather than manufacturing approval.

## Non-goals
Implementation and broad platform parity.
