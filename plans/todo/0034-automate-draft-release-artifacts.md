---
id: TASK-0034
title: Automate validated draft release artifacts
status: doing
depends_on: [TASK-0004, TASK-0005, TASK-0013, TASK-0014, TASK-0017]
priority: normal
tags: [release, automation, packaging, security]
---

# Automate validated draft release artifacts

## Problem
Release checks and supported-platform packaging are currently manual, so a versioned release cannot be reproduced consistently without risking an unvalidated tag, an unsupported artifact, or accidental publication.

## Outcome
A validated `vX.Y.Z` tag runs the pinned release checks and produces deterministic, version-stamped supported artifacts plus a SHA-256 manifest in a DRAFT GitHub release. The workflow does not create tags or publish releases.

## Acceptance criteria
- [x] A `vX.Y.Z` tag is validated before its version is used; malformed tags fail closed and no tag value is evaluated as shell code.
- [x] Tag-triggered jobs run the normal gate on Linux and macOS plus the release-required formatting, vet, static analysis, race, coverage, vulnerability and architecture checks with pinned tool versions.
- [x] Build only the supported artifacts: Linux x86_64 and aarch64 CLI archives, macOS arm64 CLI archive, and macOS arm64 app-bundle archive. Intel and other platform artifacts are not produced or implied.
- [x] Archives have deterministic names and contents, include the validated version, and are checked before a sorted SHA-256 manifest is generated.
- [x] CLI and macOS app metadata receive version stamping from the validated tag; local builds remain identifiable as development builds.
- [x] Only the final release job has `contents: write`; it creates a DRAFT release with `--verify-tag`, and no workflow step creates tags or publishes releases.
- [x] Archive, checksum, version, malformed-tag and failure paths have focused local tests; workflow YAML is syntax-checked and tool-validated.

## Verification

Implemented in `.github/workflows/release.yml`, `scripts/release-artifacts.sh`, and the CLI version stamp. The helper uses `trimpath`, a normalized tar/gzip writer, validated target allowlists, archive-content checks and sorted SHA-256 output. macOS app builds reuse `scripts/build-macos-handler.sh` and retain ad-hoc signing.

Automated and package-build evidence is build-only; it does not establish runtime behavior, cross-compiled platform behavior, Developer ID signing, notarization, Gatekeeper transfer, or release approval.

## Remaining gates

- Run the workflow from an authorized real `vX.Y.Z` tag only after explicit release approval; this task does not create tags.
- Inspect the resulting DRAFT release and archive contents/checksums on GitHub.
- Keep real-platform GUI, browser/default-handler, install/update/remove/restore and nix-darwin lifecycle acceptance in TASK-0015, TASK-0019 and TASK-0024.
- Developer ID signing, notarization, Gatekeeper transfer, publication, and a full release matrix remain unverified and out of scope.

## Non-goals

Intel macOS, unsupported platforms, automatic publication, automatic tag creation, live installation/default changes, user configuration changes, and claiming runtime acceptance from cross-compilation.
