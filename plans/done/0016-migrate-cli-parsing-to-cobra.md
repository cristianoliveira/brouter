---
id: TASK-0016
title: Migrate CLI parsing to Cobra
status: done
depends_on: [TASK-0011]
priority: normal
tags: []
---

# Migrate CLI parsing to Cobra

## Problem
The CLI parser is growing across commands; migrate parsing and command wiring to Cobra without changing existing command, flag, exit-status, output, redaction, or diagnostic side-effect contracts.

## Context
Strict Cobra-native scope per product direction: standard flags and parser surfaces only — no legacy compatibility shims, no byte-parity emulation, no parser-error rewrites.

## Acceptance criteria
- [x] Parser and command wiring use github.com/spf13/cobra (v1.9.1, pinned direct requirement in go.mod) with injectable construction: streams are passed in, the command layer never touches os.Stdout/Stderr.
- [x] Standard long flag `--config` on validate/explain/open; no single-dash compatibility shim.
- [x] Cobra-native help surfaces: root and per-command -h/--help and the help command print help to stdout with exit 0.
- [x] Cobra-native parser errors on stderr with the usage exit code (2); unknown commands rejected via root Args=NoArgs.
- [x] Completion command disabled: the migration adds no features.
- [x] Business contracts preserved: exit codes 0/1/2, stdout/stderr routing, redaction, no diagnostic side effects; domain/config/launch packages unchanged.
- [x] Full battery green in the pinned toolchain: make check (lint + tests), race suite, go mod verify, git diff --check.

## Notes
QA: Kelly final PASS on the pre-strict state (a3252ba); strict-scope re-QA requested at 7b64059.
