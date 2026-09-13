---
id: TASK-0007
title: Prove Linux desktop URL receipt and browser forwarding
status: todo
depends_on: [TASK-0002]
priority: high
tags: [linux, spike]
---

## Problem
Linux desktop handlers and browser package formats can change launch behavior despite a portable Go binary.

## Outcome
A minimal experiment verifies the agreed Linux baseline.

## Acceptance criteria
- Register a temporary desktop HTTP/HTTPS handler on the agreed desktop and demonstrate OS-delivered URL receipt.
- Forward an unchanged URL to an installed Brave or Chrome, closed and already running.
- Record desktop/session, browser version, package format, executable resolution, and sandbox limitations.
- Probe profile selection if feasible and report uncertainty explicitly.
- Prove an observable failure channel for desktop-originated errors on the supported desktop; record behavior when notifications are unavailable and how users inspect failures without persistent URL logging.
- Preserve previous handler settings and document restoration; no silent system-default changes.

## Verification
Record real desktop smoke evidence. Headless CI or process mocks alone do not establish desktop support.

## Non-goals
All distributions, every package format, production installation, or source-app detection.
