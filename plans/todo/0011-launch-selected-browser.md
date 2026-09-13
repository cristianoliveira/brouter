---
id: TASK-0011
title: Open routed URLs through safe browser launch adapters
status: todo
depends_on: [TASK-0008, TASK-0009, TASK-0006, TASK-0007]
tags: [launch]
---

## Problem
A correct decision is useless if launching loses URL characters, invokes a shell, or loops through the default handler.

## Outcome
brouter open URL resolves and launches the selected browser safely on both supported platforms.

## Acceptance criteria
- Implement known Brave/Chrome target resolution based on platform experiments and generic explicit executable targets.
- Pass URL and options as structured process arguments, never shell interpolation; validate HTTP/HTTPS before launch.
- Preserve query strings, fragments, spaces/encoding, ampersands and non-ASCII input as defined by the URL contract.
- Reject direct router-as-target configuration using resolved executable identity/path; never delegate browser launch back to the system default handler.
- Document that generic wrapper executables must launch a browser directly, not call the default handler or router. Arbitrary wrapper recursion cannot be guaranteed detectable; do not claim otherwise.
- Missing executables and launch errors are visible failures; never silently switch work targets to default.
- Wire dependencies at CLI composition root; keep matching independent of process operations.
- Do not accept a profile then silently ignore it: report unsupported until TASK-0012 is available.

## Verification
Deterministic process-fake tests cover argv, missing target, launch failure and no shell. Run platform smoke checks for cold/warm browser states.

## Non-goals
Profile support in this deliverable or proof of page load from process exit alone.
