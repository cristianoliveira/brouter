---
id: TASK-0014
title: Package and install the Linux desktop handler
status: todo
depends_on: [TASK-0011]
tags: [linux, packaging]
---

## Problem
Users need persistent desktop integration beyond an experiment or shell command.

## Outcome
The agreed Linux baseline has a reproducible user-level installation.

## Acceptance criteria
- Install executable and valid desktop entry declaring HTTP/HTTPS handling with safe desktop Exec argument behavior.
- Use platform experiment evidence for browser resolution and supported package formats.
- Config lookup is independent of launching app working directory and interactive shell PATH.
- Provide explicit opt-in default-handler change, uninstall and previous-handler restoration behavior.
- Surface invalid config and launch failures when invoked outside a terminal.
- Unsupported package formats and desktops are documented rather than implicitly promised.

## Verification
Test generated launch behavior and install paths in temporary directories. Run real external-app click, warm-browser, and failure smoke cases on the agreed desktop.

## Non-goals
All Linux desktops/package managers, privileged global installation, or routing logic in installer scripts.
