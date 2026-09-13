---
id: TASK-0010
title: Expose validate and explain diagnostics
status: done
depends_on: [TASK-0008, TASK-0009]
tags: [cli, diagnostics]
---

## Problem
Users cannot safely debug regex/configuration by repeatedly opening real browser tabs.

## Outcome
Users inspect syntax and routing decisions without launching anything.

## Acceptance criteria
- validate checks configuration and returns actionable field/rule errors with nonzero exit status.
- explain URL uses the same routing decision function as open and shows evaluated rules, match reasons, target, optional profile, and fallback use.
- Clearly mark rules not evaluated after the first match rather than reporting fabricated match results.
- Both commands perform no browser launch or network request; explain states that no browser was launched.
- Keep config validity separate from machine/browser availability; do not claim a validated browser is installed.
- Define stable success/usage/config-error exit behavior and stdout/stderr use.
- No automatic disk logging of URLs; warn that explicitly requested URL output may contain sensitive data.

## Verification
CLI tests cover success, invalid regex/config/URL, unmatched URLs, and launch dependency never being invoked. Assert meaningful output, not entire fragile snapshots.

## Non-goals
Persistent tracing, telemetry, or a standalone diagnostic engine.
