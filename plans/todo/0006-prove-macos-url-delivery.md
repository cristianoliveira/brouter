---
id: TASK-0006
title: Prove macOS URL receipt and browser forwarding
status: todo
depends_on: [TASK-0002]
priority: high
tags: [macos, spike]
---

## Problem
A Go CLI alone does not receive macOS default-browser URL events. This integration is the highest platform uncertainty.

## Outcome
A minimal macOS experiment establishes the smallest viable integration.

## Acceptance criteria
- Build a minimal app bundle for current macOS on Apple Silicon that declares HTTP/HTTPS handling and receives a URL through the OS event mechanism. Intel support is not required for this experiment.
- Forward an unchanged test URL to Brave or Chrome when target and router are closed and already running; exercise sequential events.
- Compare direct Go/native interop with a thin native helper only as needed; record complexity, cgo and build requirements, and recommendation.
- Separate process-exit success from evidence that the browser actually received the URL.
- Record an explicit profile-launch probe if feasible, otherwise leave profile behavior unverified.
- Prove an observable failure channel for GUI-originated invalid-config/launch errors and record the chosen mechanism; assess notification permission and log discoverability rather than assuming a terminal exists.
- Provide safe temporary registration and restore instructions; no silent default-browser change.

## Verification
Record real macOS version, commands, observed events and destination. Use benign URLs. Mock-only evidence is insufficient.

## Non-goals
Production installer, signing, or settled helper choice before the experiment.
