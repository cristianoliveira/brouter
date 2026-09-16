---
id: TASK-0034
title: Automate validated draft release artifacts
status: done
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
- [x] Archives have deterministic names and contents, include the validated version, and every downloaded archive is revalidated for safe paths, entry types, required root/VERSION and stamped version before a sorted SHA-256 manifest is generated.
- [x] CLI and macOS app metadata receive version stamping from the validated tag; local `--version` builds identify as `dev-<full VCS revision>` (or `-dirty`), falling back to `dev` when Go build metadata is unavailable.
- [x] Every checkout disables persisted credentials; repository helpers run before token exposure, and only the final fixed `gh release create` command receives `GH_TOKEN`.
- [x] Only the final release job has `contents: write`; it creates a DRAFT release with `--verify-tag`, and no workflow step creates tags or publishes releases.
- [x] Archive, checksum, version, malformed-tag and failure paths have focused local tests; workflow YAML is syntax-checked and tool-validated.

## Verification

Implemented in `.github/workflows/release.yml`, `scripts/release-artifacts.sh`, and the CLI version stamp. Local CLI version resolution uses Go embedded build metadata (`dev-<full revision>` with explicit `-dirty` status); linker-stamped release versions remain unchanged and unavailable metadata falls back to `dev`. Focused tests cover revision-present/absent and Cobra `--version` output. The helper uses `trimpath`, a normalized tar/gzip writer, validated target allowlists, archive-content checks and sorted SHA-256 output. The final job revalidates downloaded archives before manifest generation. macOS app builds reuse `scripts/build-macos-handler.sh` and retain ad-hoc signing.

Automated and package-build evidence is build-only; it does not establish runtime behavior, cross-compiled platform behavior, Developer ID signing, notarization, Gatekeeper transfer, or release approval.

## Actual outcome

The first authorized tag run completed successfully from `main`:

- Tag: `v0.1.0`.
- Target commit: `5c7d5d331a16920e8683a3b0fef09d5305f220c5`.
- Workflow run: [35130454143](https://github.com/cristianoliveira/brouter/actions/runs/35130454143).
- Initial draft release: [Brouter v0.1.0](https://github.com/cristianoliveira/brouter/releases/tag/untagged-27ace58a8a5339e71fc0).
- The release contains all five expected assets: `brouter-v0.1.0-linux-x86_64.tar.gz`, `brouter-v0.1.0-linux-aarch64.tar.gz`, `brouter-v0.1.0-darwin-arm64.tar.gz`, `brouter-handler-v0.1.0-darwin-arm64.tar.gz`, and `SHA256SUMS`.

Downloaded assets were inspected: `sha256sum -c SHA256SUMS` passed; every archive contained its expected target root, executable or app payload, and `VERSION` set to `0.1.0`; and the manifest contained sorted checksums for all four archives. The macOS workflow log records ad-hoc signing and the app archive contains the code-signature resources plus `CFBundleShortVersionString=0.1.0` and `CFBundleVersion=010`. After draft inspection, publication was explicit and manual outside the workflow; the current release is published (`isDraft=false`, `publishedAt=2026-09-16T18:48:36Z`) at [v0.1.0](https://github.com/cristianoliveira/brouter/releases/tag/v0.1.0). No tag mutation was performed.

No protected `v*` tag ruleset was found through the repository API, and no repository settings were changed. Publication, production signing/notarization, runtime GUI, browser, and nix-darwin acceptance remain separate gates in TASK-0015, TASK-0019, and TASK-0024.

## Remaining gates

- Configure protected `v*` tag rules so only authorized maintainers can create, move, or delete release tags before any future release; this task does not change repository settings.
- Keep real-platform GUI, browser/default-handler, install/update/remove/restore and nix-darwin lifecycle acceptance in TASK-0015, TASK-0019 and TASK-0024.
- Developer ID signing, notarization, Gatekeeper transfer, publication, and a full release matrix remain unverified and out of scope.

## Non-goals

Intel macOS, unsupported platforms, automatic publication, automatic tag creation, live installation/default changes, user configuration changes, and claiming runtime acceptance from cross-compilation.
