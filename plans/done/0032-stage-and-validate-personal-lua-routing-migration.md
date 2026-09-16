---
id: TASK-0032
title: Complete personal routing migration via external command (Lua superseded)
status: done
depends_on: []
priority: normal
tags: [routing, migration]
---

# Complete personal routing migration via external command (Lua superseded)

## Problem
The personal configuration is a symlink into dotfiles, so migration must prove routing parity and safe edit/reload behavior before touching the live target.

## Outcome

The PO-approved migration uses the language-independent external Python route command. Embedded Lua remains superseded and is not resurrected.

## Acceptance criteria
- [x] Adopt the supported external command boundary rather than adding or enabling an embedded Lua runtime.
- [x] Preserve the existing TOML routing contract while adding the approved `route_command` configuration and colocated Python entry points.
- [x] Record the PO-reported migration evidence and keep live-target mutation outside this board reconciliation.
- [x] Preserve an explicit GUI caveat: CLI/command evidence does not establish real browser or LaunchServices acceptance.

## Verification

The PO reported the approved dotfiles migration in commits `c594c65`, `fd0e5a1`, and `5b97aeb`, with an equivalent `route_command` configuration and colocated `route.py` symlinks present in the personal setup. This reconciliation does not inspect, edit, or reload the live dotfiles target.

The external command implementation and contract tests are covered by merged PR #34 (`6c0393f`) and PR #35 (`ff0baf0`). Temporary-HOME checks on current `origin/main` passed; real browser, Finder, LaunchServices, and menu-bar GUI behavior remains separate and is not claimed here.

## Notes

**Status: DONE / superseded by external command.** Lua runtime work remains historical design/evaluation only. No release/Lua gate remains on this task, and no live configuration was changed during reconciliation.
