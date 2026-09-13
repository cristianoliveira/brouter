---
id: TASK-0004
title: Wire hooks watcher and CI to the same quality gate
status: done
depends_on: [TASK-0003]
tags: [guardrails, ci]
---

## Problem
A local gate is ineffective if development and CI bypass it or run competing checks.

## Outcome
All normal lifecycle checks invoke make check.

## Acceptance criteria
- Version pre-commit/pre-push hooks that call make check, and a commit-msg hook enforcing documented conventional commits.
- Provide explicit hook installation and verification commands; do not overwrite unrelated hooks silently.
- Configure or document the fzz watcher to run the same gate without adding watcher tools to production dependencies.
- Run the gate in Linux and macOS CI with pinned checkout/tool setup actions and the agreed Go version.
- Document failure recovery and local equivalents; no CI-only normal checks.

## Verification
Exercise hook install/check and controlled hook failures in a temporary Git repository. Confirm workflow commands match local commands; report remote CI as pending until observed.

## Non-goals
Push code or change hosted branch protection without authorization.
