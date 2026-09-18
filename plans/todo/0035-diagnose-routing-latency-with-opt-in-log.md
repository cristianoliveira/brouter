---
id: TASK-0035
title: Diagnose routing latency with an opt-in local routing log
status: todo
depends_on: [TASK-0030, TASK-0033]
tags: [performance, diagnostics, privacy, log]
---

# Diagnose routing latency with an opt-in local routing log

## Problem
The user reports browser routing "feels slow sometimes", but nothing
measures where per-URL time goes, so any optimization would be
speculative. Warm-machine measurement on 2026-09-18 (n=20 each, fixture
URLs, no browser launch) decomposed the live chain
BrouterHandler → `brouter open` → `route.py` → browser launch:

- brouter Go spawn + config load + static eval: median ~6ms (max ~34ms)
- live `route.py` decision (fresh CPython per URL): median ~57ms, p90 ~105ms
- combined decision path: well under human perception; no network in path

The remaining stage — browser process launch (warm handoff vs cold
start) — is unmeasured in the wild and is the dominant suspect, but no
data exists to confirm it.

## Outcome
A default-off, local-only, size-bounded routing log with per-stage
durations and redacted URLs produces real-world evidence; routing
behavior, exit codes, and configurations without `[log]` are unchanged.

## Acceptance criteria
- [x] Record the measured decision-path breakdown (numbers above) as
      the baseline; do not optimize before real-world data justifies it.
- [x] Implement an opt-in `[log]` config table (`enabled`, `path`,
      `host`, `max_bytes`, `max_files`), disabled by default; absent or
      disabled tables change no behavior and write nothing.
- [x] Default log location is the user state dir
      (`$XDG_STATE_HOME/brouter/routing.log`, defaulting to
      `~/.local/state/...`); an explicit `path` must be absolute or
      `~/`-prefixed and is validated visibly.
- [x] Never log full URLs, paths, queries, or credentials: the URL
      field is always `REDACTED`; the hostname is written only when the
      user explicitly sets `host = true`.
- [x] One timestamped line per web-URL open with target, outcome,
      resolve_ms, launch_ms, and a sanitized short error on failure;
      local-document batches stay out of scope.
- [x] Rotation is bounded and deterministic (`max_bytes` threshold,
      `max_files` rotated files); log write/rotation failures print one
      redacted stderr warning and never change routing or exit codes.
- [x] Tests cover redaction, host opt-in, rotation bounds,
      failure-visible warnings, default-off no-op, and config
      validation; all existing tests pass unchanged.
- [ ] With user consent on location/content, enable `[log]` in the live
      dotfiles config and collect warm/cold distributions (>=20 opens
      each) to rank the real bottleneck before any further change.
