---
id: TASK-0012
title: Support explicit Brave and Chrome profiles
status: todo
depends_on: [TASK-0011]
tags: [profiles]
---

## Problem
Users sometimes separate work with profiles inside the same browser, not separate browsers.

## Outcome
An optional configured profile reaches the intended existing Brave/Chrome profile.

## Acceptance criteria
- Support explicit existing profile identifiers on declared platform/browser combinations; document how users find identifiers versus display names.
- Prove behavior for already-running browsers and cold launches on both platforms.
- Missing or unverifiable profiles fail explicitly rather than silently opening a default profile or creating a new one.
- No-profile targets retain browser-only behavior; explain includes the configured profile.
- Document machine-local profile mapping and any platform/package limitations.

## Verification
Tests cover argument construction, known/missing profiles and failure paths. Real browser smoke evidence confirms selected profile; a successful spawn alone is insufficient.

## Non-goals
Profile creation, migration, synchronization, GUI discovery, or unsupported browser profile parity.
