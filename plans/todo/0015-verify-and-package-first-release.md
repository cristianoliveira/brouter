---
id: TASK-0015
title: Verify and package the first supported release
status: todo
depends_on: [TASK-0004, TASK-0005, TASK-0010, TASK-0012, TASK-0013, TASK-0014]
tags: [release, acceptance]
---

## Problem
Individual passing components do not prove that installed routing works across supported platforms.

## Outcome
A bounded release candidate has reproducible artifacts and user-facing acceptance evidence.

## Acceptance criteria
- Run end-to-end matrix for default routing, host/subdomain/regex rules, fallback, profiles, invalid config, missing browser/profile, warm/cold launches, and repeated OS URL delivery.
- Validate a committed sample dotfiles config on both platforms, documenting local profile substitutions.
- Run normal and separate release checks locally/CI: formatting, static analysis, race checks where supported, coverage budget, architecture boundary and dependency vulnerabilities. Record unsupported checks explicitly.
- Document chosen release trigger, version stamping, supported platform artifacts, checksums, and macOS signing status with reproducible local commands.
- Confirm install, explicit default selection, debug, and uninstall/restore instructions from a clean user environment.
- Publish no artifacts or tags until explicitly authorized; no full URL history/logging by default.

## Verification
Attach command evidence and real-platform acceptance results; list untested combinations as unsupported or pending. Do not infer GUI acceptance from headless tests.

## Non-goals
Public release authorization, automatic updates, or expanding the support matrix.

## Scope note
Profiles are included provisionally. If TASK-0001 defers them beyond the first usable release, update this dependency and release scope explicitly rather than skipping verification.
