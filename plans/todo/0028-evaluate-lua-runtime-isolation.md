---
id: TASK-0028
title: Evaluate Lua runtime and isolation options
status: todo
depends_on: [TASK-0025]
priority: normal
tags: [lua, design, security]
---

# Evaluate Lua runtime and isolation options

## Problem
The approved routing contract defines script inputs and safe outcomes, but selecting an embedded Lua engine or process boundary without evidence could add unsafe capabilities, nondeterminism, or an unmaintained dependency.

## Outcome
A bounded, engine-agnostic comparison identifies whether and how the TASK-0025 contract can be implemented safely; it does not silently select a runtime.

## Acceptance criteria
- [ ] Compare a small candidate set against the contract: standard-library allowlist, filesystem/network/process isolation, deterministic time injection, timeout enforcement, memory limits, error mapping, arm64/Linux builds, maintenance, licensing, offline reproducibility, and live edits without router restart.
- [ ] Use temporary/pinned evaluation or source inspection only; do not add a dependency, change `go.mod`, execute user scripts, or expose host capabilities.
- [ ] Identify hard safety requirements and unresolved product choices, including timeout mechanism, whether a separate process is required, and how each new URL obtains a fresh immutable config-plus-script snapshot.
- [ ] Recommend explicit next options (including defer/decline) with risks and evidence; document that no runtime/config schema/whole-Lua mode is approved by this task.

## Verification
Attach reproducible comparison evidence and an explicit threat-model checklist. Commands must be runnable as written after the documented throwaway setup; if setup or a platform run was not reproduced, label it unavailable. Treat unavailable platform, sandbox, load/reload, or resource evidence as unknown, not as support.

## Non-goals
Lua implementation, dependency addition, config migration, script execution in production, or choosing an engine by default.
