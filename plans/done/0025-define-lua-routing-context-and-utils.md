---
id: TASK-0025
title: Define an explicit Lua routing context with allowlisted utilities
status: done
depends_on: []
tags: [lua, design, config]
---

## Problem
Future programmable routing needs convenient helpers without exposing arbitrary Lua operating-system capabilities or hiding dependencies in globals.

## Confirmed direction
The routing function receives a context object. It contains URL data and a limited utils API. User confirmed this shape; current routing does not require time conditions.

```lua
function route(ctx)
  -- ctx.url: parsed URL data
  -- ctx.utils: explicitly provided helpers
  return "work"
end
```

## Outcome
A small reviewed API contract for future Lua integration, not a Lua runtime implementation or migration of existing configuration.

## Approved contract helper surface
- `ctx.utils.epoch()` returns integer Unix seconds in UTC.
- `ctx.utils.utc()` returns `{year, month, day, hour, min, sec}` with integer UTC fields.
- Both helpers take no arguments and observe one clock sample per `route` invocation.
- No extra helper is included merely because the Lua standard library has it. Weekday/year-day fields remain undecided.

## Acceptance criteria
- Document the context's exact URL fields and normalization, helper signatures, supported format/conversion behavior and return values. Preserve original URL separately if needed; never mistake parsed host matching for current substring regex semantics.
- Specify date/time semantics intentionally: the approved contract does not promise full os.date/os.time compatibility. `epoch()` and `utc()` are UTC-only, total, no-argument helpers sampled from one instant; local-time and extra calendar fields remain undecided.
- Helpers are passed through the context, not unrestricted os globals. The host supplies implementations, including replaceable clock/timezone inputs for deterministic tests. Production clock behavior must be explicit; do not introduce a separate ctx.time policy engine.
- The routing result names an existing configured target; no returned shell commands or direct process access. Document explicit fallback versus invalid/unknown target or script failure behavior for the future evaluator.
- Allowlist exposed capabilities. No os.execute, filesystem, process environment, network, io, package loading, debug access or dynamically loaded libraries through helper objects. A restricted helper table alone is not proof of sandbox security.
- Document mutability/isolation so one evaluation cannot alter later contexts/helpers through shared globals or tables. Runtime execution/allocation limits remain a prerequisite of eventual embedding, not an unverified promise here.
- Define how future open/explain evaluation will share the same context/helper provider, with controlled inputs for reproducibility. Tests can use fixed helper implementations without manipulating machine time or user configuration.
- Include examples with a fixed clock and small contract-test scenarios for deterministic UTC values and attempts to access forbidden capabilities. These are acceptance scenarios until a runtime exists, not claims that executable tests passed.

## Verification
Review the contract with lead/developer and record unresolved semantics explicitly. No full code checks apply to this design-only task. A later runtime implementation must execute both happy/unhappy contract tests, privacy checks and resource-limit tests before claiming support.

## Non-goals
Selecting or adding a Lua engine, replacing TOML, deciding whole-config versus optional Lua mode, scheduling rules, migrating current dotfiles, implementing a routing DSL, exporting unrestricted standard libraries, or tying Lua work to menu-bar delivery.

## Follow-up boundary
Lua embedding/configuration mode must be planned separately once chosen. Completion here approves the contract only; it does not mean Lua configurations work in brouter. The confirmed requirement that TOML and future Lua edits apply to the next URL without restart is captured in `docs/route-script-contract.md`; loading/coherence and load-failure mechanics remain undecided.
