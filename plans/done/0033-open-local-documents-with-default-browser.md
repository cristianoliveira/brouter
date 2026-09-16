---
id: TASK-0033
title: Open local documents with the default browser
status: done
depends_on: [TASK-0030]
tags: [macos, local-files, browser]
---

# Open local documents with the default browser

## Problem

On macOS, local HTML and other supported documents must reach the default browser without being misclassified as routed HTTP(S) URLs or producing an unsupported-format dialog.

## Outcome

Merged PR #38 (`265f9af`, 2026-09-16) classifies supported local documents, separates them from HTTP(S) routing, and forwards them through the default browser while preserving the existing URL path.

## Acceptance criteria

- [x] Classify supported local documents, including HTML, through the explicit local-file path; reject unsupported formats visibly.
- [x] Preserve HTTP(S) routing and split mixed document/HTTP batches without changing per-URL forwarding semantics.
- [x] Preflight batches before dispatch and retain honest partial-dispatch reporting when an OS dispatch fails after spawning.
- [x] Forward local documents through the default browser without recursively routing them through the HTTP(S) handler.
- [x] Keep the runtime allowlist explicit (`internal`, `infra`, `localfile`) and avoid changing browser defaults or installed applications.
- [x] Add deterministic classification, forwarding, warm/mixed/multi-HTTP, and Apple-event harness coverage.

## Verification

Focused Go and macOS harness tests landed in PR #38, including warm delivery, real Apple-event dispatch, mixed-array splitting, multi-HTTP forwarding, preflight atomicity, and partial-dispatch wording. CI evidence covered the available source/package checks.

**UNTESTED:** live LaunchServices delivery and the effective extension/UTI intersection of the declarative bundle metadata. The runtime allowlist is the sole authority over what brouter accepts; metadata declarations do not prove effective modern LaunchServices matching. Real browser rendering and Finder default behavior remain untested.

## Non-goals

Changing LaunchServices defaults, installing applications, editing personal configuration, or claiming browser/page-load success from routing or dispatch evidence.
