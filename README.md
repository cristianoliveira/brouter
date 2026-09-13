# brouter

`brouter` routes URLs to the right browser based on a version-controlled
configuration — separate work and personal contexts without hand-editing GUI
settings or guessing why a rule matched.

Status: bootstrap plus the first launch capability: `open` routes a URL
and launches the selected browser directly. Default-handler integration
(handling URLs from the OS) arrives in later tasks.

## Open a URL

```sh
brouter open URL        # or: echo URL | brouter open
brouter open -config path/to/config.toml URL
```

Exit codes: 0 launched, 1 validation or launch failure, 2 usage error.

Safety contract:

- The URL is passed to the browser as one structured process argument.
  No shell parses it, so query strings, fragments, spaces, ampersands,
  and non-ASCII characters survive byte-for-byte.
- Only http and https URLs are routed, and the launcher re-validates the
  scheme as a last line of defense.
- brouter launches the resolved browser executable directly. It never
  delegates to the system default handler, and a target that resolves to
  brouter itself is rejected (symlink identity is compared).
- Failures are visible: a missing executable, an unsupported browser, an
  unsupported profile, or a browser process failure stops `open` with a
  named error on stderr. brouter never silently switches to another
  target.
- Known browsers resolve per platform: brave and chrome are resolved
  from PATH (including the NixOS profile path observed in
  docs/probes/nixos-sway-url-delivery.md) and macOS app bundles; other
  known browsers report as unsupported — use an executable target.
- A profile on a target is reported as unsupported until profile support
  ships (TASK-0012); it is never accepted and silently ignored.
- Generic executable targets must launch a browser directly. A wrapper
  that calls the system default handler (or brouter) creates recursion
  brouter cannot detect; the wrapper contract is documented here, not
  enforced by inspection.

## Quick start

Prerequisites: either [Nix](https://nixos.org/) with flakes, or any Go
toolchain at or above the version in `go.mod`.

```sh
nix develop               # pinned toolchain (flake.lock)
make check                # the one quality gate: prints "true" on success
go build ./cmd/brouter    # produces ./brouter
go test ./...             # runs the tests
```

See [DEVELOPMENT.md](DEVELOPMENT.md) for the gate contract, the pinned lint
rule set, and the toolchain policy, and
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for package boundaries.

## Supported platforms (planned)

- NixOS with Nix-installed browsers on Sway (Wayland)
- macOS, Apple Silicon first

Other distributions and package formats are outside initial support.
