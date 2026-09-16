
<img width="50" height="50" align="right" alt="brouter logo" src="docs/assets/readme-logo.svg" />

`brouter` routes URLs to the right browser based on a version-controlled
configuration — separate work and personal contexts without hand-editing GUI
settings or guessing why a rule matched.



## Open a URL

```sh
brouter open URL        # or: echo URL | brouter open
brouter open --config path/to/config.toml URL
```

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

For a safe, self-contained Python route command, see
[examples/python-route-command](examples/python-route-command).

## Supported platforms (planned)

- NixOS with Nix-installed browsers on Sway (Wayland)
- macOS, Apple Silicon first

Other distributions and package formats are outside initial support.
