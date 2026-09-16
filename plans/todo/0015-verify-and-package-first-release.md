---
id: TASK-0015
title: Verify and package the first supported release
status: doing
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

## Bounded release slice (2026-09-16)

All checks below used detached current `origin/main` `be0ccc3`, temporary HOME/config paths, temporary fake browser/launcher scripts, and no live configuration.

### End-to-end CLI matrix

A temporary four-target config and fake executables verified:

- `validate`: PASS (`config OK: targets 4, rules 3, default "default"`).
- exact-host, subdomain, URL-regex, and unmatched default routing: PASS; each fake browser received exactly one original URL argument.
- repeated CLI launches: PASS; two cold process invocations delivered both URLs to the rule-selected target.
- default-path resolution: PASS with `$HOME/.config/brouter/config.toml` and unset `XDG_CONFIG_HOME`.
- malformed TOML: PASS as a visible exit 1 with config path/syntax category.
- missing executable target: PASS as visible exit 1; no fallback.
- missing known-browser profile: PASS as visible exit 1; no profile creation or fallback.

`go test -vet=off ./scripts -run '^TestOpenFilesRealAppleEventDispatchReachesOpenURLs$' -count=1 -v`: PASS. This is a temporary Apple-event harness, not LaunchServices/default-browser GUI acceptance.

### Quality and security checks

- `go test -race -vet=off ./cmd/brouter ./internal/...`: PASS after removing unrelated exhausted temporary test directories.
- `make fmt-check`, `make vet`, `make architecture`: PASS.
- Budgeted coverage from the prior bounded run: cmd/brouter 86.5%, domain 95.8%, config 95.0%: PASS.
- `GOTOOLCHAIN=go1.27.0 go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...`: PASS, `No vulnerabilities found.`
- `make analyze`: **FAIL** on existing source finding `scripts/macos_handler_test.go:195:6: func testAppMacOSDir is unused (U1000)`. Per instruction, no source change was made; this is a release blocker requiring follow-up.
- `make check`: **FAIL before tests** because installed golangci-lint is 2.12.2 (built with Go 1.25.12) while the repository expects 2.13.2. This is a toolchain/environment blocker distinct from the staticcheck source finding.

### Temporary artifact/signing evidence

Two `GOOS=darwin GOARCH=arm64 go build -trimpath` CLI builds were byte-identical (SHA-256 `d4f00dd9c568f67ad21d075a423ca1c45bb2bb23e9d59a609b260aa41d279562`). A temporary arm64 app bundle assembled from `native/macos` passed `codesign --verify --deep --strict`; `file` reported both executables as Mach-O arm64. `codesign -dv` reported identifier `com.cristianoliveira.brouter.handler`, `Signature=adhoc`, and `TeamIdentifier=not set`. Temporary SHA-256 manifest output was recorded; no artifact was published.

`nix flake check --no-update-lock-file` and `nix build .#packages.aarch64-darwin.brouter-handler --no-link` also pass. Artifact version stamping, release archive/checksum publication, Developer ID signing/notarization/Gatekeeper transfer, and tags remain unperformed.

### Remaining consent and release gates

Still **UNTESTED**: real browsers/profiles and visible destinations; real LaunchServices cold/warm/repeated delivery; clean-environment install, explicit default selection, update, uninstall/restore; menu-bar/accessibility behavior; multi-platform release matrix; and published versioned artifacts. Exact manual actions require user consent to inspect defaults, install/rebuild/remove, explicitly select/restore the handler, open one fixed benign URL cold and warm, observe the visible browser destination, inspect diagnostics without exposing the URL, and verify clean restore. Do not infer GUI or publication acceptance from these headless/temporary checks. No release or tag was published.

## Non-goals
Public release authorization, automatic updates, or expanding the support matrix.

## Scope note
Profiles are included provisionally. If TASK-0001 defers them beyond the first usable release, update this dependency and release scope explicitly rather than skipping verification.
