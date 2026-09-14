---
id: TASK-0021
title: Deliver macOS URL events to the handler
status: doing
depends_on: []
priority: normal
tags: [macos, events, privacy]
---

# Deliver macOS URL events to the handler

## Problem
The installed macOS URL-handler process stays alive but ordinary LaunchServices URL opens time out because the shim registers Apple Events without initializing a Cocoa application event loop; its diagnostics also log raw URLs.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Criterion 1
- [ ] Criterion 2

## Notes

