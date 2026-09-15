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
- [ ] Compare a small candidate set against the contract: standard-library allowlist, filesystem/network/process isolation, deterministic time injection, timeout enforcement, memory limits, error mapping, arm64/Linux builds, maintenance, licensing, and offline reproducibility.
- [ ] Use temporary/pinned evaluation or source inspection only; do not add a dependency, change `go.mod`, execute user scripts, or expose host capabilities.
- [ ] Identify hard safety requirements and unresolved product choices, including timeout mechanism and whether a separate process is required.
- [ ] Recommend explicit next options (including defer/decline) with risks and evidence; document that no runtime/config schema/whole-Lua mode is approved by this task.

## Verification
Attach reproducible comparison evidence and a threat-model checklist. Treat unavailable platform or sandbox evidence as unknown, not as support.

## Non-goals
Lua implementation, dependency addition, config migration, script execution in production, or choosing an engine by default.
