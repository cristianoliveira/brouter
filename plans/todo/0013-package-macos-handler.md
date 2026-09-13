---
id: TASK-0013
title: Package and install the macOS URL handler
status: doing
depends_on: [TASK-0011]
tags: [macos, packaging]
---

## Problem
The feasibility app is not yet a reproducible user installation.

## Outcome
A packaged macOS app receives OS URL events and routes them through shared Go logic.

## Acceptance criteria
- Provide reproducible bundle build with HTTP/HTTPS declarations and the integration chosen by TASK-0006.
- Route events while app is closed or already running, including multiple sequential URLs, without stale configuration behavior.
- Resolve user config consistently without depending on shell working directory or interactive PATH.
- Provide installation, explicit default-browser selection, uninstall and previous-handler restoration instructions.
- Surface invalid config and launch failures for GUI-originated events; no silent failure hidden in absent terminal output.
- Document signing/notarization status and supported OS/architectures honestly.

## Verification
Local build command and real external-app click smoke test; confirm no silent default change and usable failure feedback.

## Non-goals
Settings GUI, automatic updates, or adding routing logic to the native event layer.
