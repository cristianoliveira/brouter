---
id: TASK-0001
title: Confirm initial support and configuration contract
status: todo
depends_on: []
priority: high
tags: [product]
---

## Problem
Linux packaging, macOS support, and matching semantics remain open. Implementing against unstated assumptions risks an unusable first release.

## Outcome
A short approved contract separates required behavior from deferred support.

## Acceptance criteria
- Record Linux desktop/distribution and initial package formats; macOS minimum and CPU architectures.
- Confirm TOML, first-match order, exact host/subdomain and full-URL regex semantics, config lookup, and fallback behavior.
- Record Go module path/toolchain and whether optional profiles are a first-release requirement.
- Record signing/notarization expectations and any local-only first-release concession.
- Include concrete personal/work routing examples and explicit unsupported cases.

## Verification
Review decisions with user; leave unresolved items visible rather than manufacturing approval.

## Non-goals
Implementation and broad platform parity.
