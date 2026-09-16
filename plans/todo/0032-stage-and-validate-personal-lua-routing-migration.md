---
id: TASK-0032
title: Stage and validate personal Lua routing migration
status: todo
depends_on: [TASK-0031, TASK-0015]
priority: normal
tags: []
---

# Stage and validate personal Lua routing migration

## Problem
The personal configuration is a symlink into dotfiles, so migration must prove routing parity and safe edit/reload behavior before touching the live target.

## Context

The external command protocol supersedes embedded-Lua runtime work for the current product path. Preserve this migration as a future evaluation only; no live configuration is authorized.

This task is blocked behind the existing release-acceptance gate (TASK-0015) as a conservative authorization boundary until product acceptance criteria and safe staging approval are explicit.


## Acceptance criteria
- [ ] Define explicit product acceptance criteria for a Lua migration, including routing parity, failure behavior, privacy, and rollback.
- [ ] Obtain approval for an isolated staging exercise and prove it does not touch dotfiles or the live configuration target.
- [ ] Validate parity and reload behavior in staging before proposing any live edit; document unavailable evidence as UNTESTED.
- [ ] Keep the external command path as the supported implementation unless product acceptance explicitly reopens Lua.

## Notes

**Status: BLOCKED / not authorized.** TASK-0031 is complete, but TASK-0015 remains an existing acceptance gate. The next smallest decision is product approval of concrete Lua acceptance criteria and safe staging scope. Do not edit dotfiles, symlink targets, or live configuration.

