---
id: TASK-0008
title: Load and validate portable router configuration
status: todo
depends_on: [TASK-0002]
tags: [config]
---

## Problem
Users need one version-controlled configuration with predictable lookup and useful errors.

## Outcome
A file maps named browser targets and ordered rules into validated routing inputs.

## Acceptance criteria
- Implement agreed file format and deterministic path lookup with explicit override; document precedence and avoid implicit merges.
- Known-browser targets require no absolute path; accept optional profile identifiers and document machine-local portability limits.
- Validate syntax, unknown fields, duplicate target definitions, target references, default target, matcher count, and regex compilation.
- Errors identify affected field/rule without dumping full configuration or URL secrets.
- Support generic browser executable targets without shell command strings; reserve router-as-target rejection for launcher validation.
- Keep file parsing in infrastructure and domain inputs independent of parser dependencies.

## Verification
Tests first: valid minimal/profile configs, missing files, malformed syntax, invalid regex, duplicate/unknown references and config-path precedence. Fixtures are colocated.

## Non-goals
Launching browsers, GUI editing, dynamic executable interpolation, or live reload.
