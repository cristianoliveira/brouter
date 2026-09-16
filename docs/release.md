# Release artifacts

Release automation is deliberately narrow. An authorized `vX.Y.Z` tag runs
`.github/workflows/release.yml`; no command in the workflow creates tags.
Malformed tags fail before the value is used for version stamping or artifact
names.

## Checks

The workflow runs `make check` on Ubuntu and macOS with Go 1.27.1 and
`golangci-lint` 2.13.2. A pinned Ubuntu job also runs `make fmt-check`, `make
vet`, `make analyze`, `make race`, `make coverage`, `make security`, and `make
architecture`.

## Supported artifacts

Only these build-only artifacts are produced:

- `brouter-vX.Y.Z-linux-x86_64.tar.gz`: Linux x86_64 CLI.
- `brouter-vX.Y.Z-linux-aarch64.tar.gz`: Linux aarch64 CLI.
- `brouter-vX.Y.Z-darwin-arm64.tar.gz`: macOS arm64 CLI.
- `brouter-handler-vX.Y.Z-darwin-arm64.tar.gz`: macOS arm64 app bundle.

Each archive contains a `VERSION` file and normalized metadata. The workflow
checks archive paths and version contents, then adds a sorted `SHA256SUMS`
manifest. The CLI receives the version through Go linker stamping. The app
bundle receives the version in `CFBundleShortVersionString` and
`CFBundleVersion` before its final ad-hoc signature.

Cross-compilation and package construction prove only build properties. They
do not prove runtime behavior, browser/default-handler behavior, GUI behavior,
transfer, Developer ID signing, notarization, Gatekeeper acceptance, or a
complete platform matrix. The macOS app is arm64-only; Intel artifacts are
not produced.

## Release side effect

The final job has `contents: write` and uses `gh release create --draft
--verify-tag`. It creates a DRAFT release only. It does not publish the
release, create a tag, alter defaults, install anything, or modify user
configuration. Release publication and signing remain explicit manual gates.
