---
id: TASK-0029
title: Implement the external routing command protocol
status: doing
depends_on: [TASK-0028]
tags: [routing, command, security]
---

# Implement the external routing command protocol

## Problem
Users want routing logic in any language they choose, without embedding or vendoring a language runtime. The runner is trusted local code, so the boundary must be explicit rather than pretending to sandbox it.

## Outcome
Brouter can invoke an opt-in executable plus argv directly for each URL and consume a small versioned JSON protocol. The Lua helper design is superseded for this product slice and must not be merged; its commits and QA remain historical evidence only.

## Protocol
Input is one bounded JSON object on stdin:

```json
{"version":1,"url":{"original":"https://example.com/x?a=1#f","scheme":"https","host":"example.com","port":"","path":"/x","query":"a=1","fragment":"f"},"utils":{"epoch":1700000000,"utc":{"year":2023,"month":11,"day":14,"hour":22,"min":13,"sec":20}}}
```

Output is one bounded JSON object on stdout:

```json
{"version":1,"route":"work-browser"}
```

`route` is either an exact configured target ID or `null` for explicit defer. No extra output, trailing data, stderr diagnostics, or protocol extensions are accepted in v1.

## Acceptance criteria
- [ ] Replace Lua-specific runtime/build/dependency work with direct argv execution; never interpolate a command through a shell. Support Bash, Python, Lua, or any executable selected by the user through the same command array.
- [ ] Bound stdin context and stdout/stderr capture, enforce a hard per-URL timeout, kill the child on timeout, and classify nonzero exit, timeout, malformed/trailing output, protocol/version errors, and unknown target with fixed redacted categories.
- [ ] Validate returned target IDs against the current TOML snapshot. Only explicit `null` defers to ordered static rules; failures are visible and never silently fall back. Preserve byte-exact URL original only according to the resolved credential policy; do not leak secrets in errors or activity logs.
- [ ] Load the current TOML for each URL with bounded observed-change detection. Keep selected config bytes immutable for the URL; document that independently saved files or a stationary syntactically valid partial write cannot be distinguished, and recommend atomic saves. No watcher or indefinite retry.
- [ ] Keep the Go router free of language/runtime dependencies. Do not claim sandboxing: configured commands have the user’s host permissions and may read/write files, access network, launch processes, or mutate configuration.
- [ ] Provide injectable clock, command runner, loader, decoder, and process/error seams for deterministic tests. Do not mutate defaults, LaunchServices, installed applications, or the real personal configuration.

## Verification
Add focused protocol, timeout/output-bound, target-validation, redaction, observed-config-change, URL-field, no-command, and static-compatibility tests. Run source/Nix package checks on available targets and label unavailable platform evidence.

## Non-goals
Embedded Lua, runtime sandboxing, arbitrary shell strings, implicit migration, file watchers, silent fallback, browser/page-load claims, or live personal-config changes.
