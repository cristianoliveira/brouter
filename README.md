
<img width="50" height="50" align="right" alt="brouter logo" src="docs/assets/readme-logo.svg" />

`brouter` routes URLs to the right browser based on a version-controlled
configuration — separate work and personal contexts without hand-editing GUI
settings or guessing why a rule matched.



## Open a URL

```sh
brouter open URL        # or: echo URL | brouter open
brouter open --config path/to/config.toml URL
```

## Routing log (opt-in)

`brouter open` can record one timestamped line per web-URL launch with
the selected target, outcome, and per-stage durations — evidence for
latency questions. It is local-only, disabled by default, and full URLs
are never written: the URL field is always `REDACTED`.

```toml
[log]
enabled = true
# path = "/absolute/or/~/path/routing.log"   # default: $XDG_STATE_HOME/brouter/routing.log
# host = false       # set true to include the lowercased hostname only
# max_bytes = 1048576 # rotate threshold
# max_files = 3      # rotated files kept
```

Write or rotation failures print one redacted warning and never change
routing or exit codes.

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
[examples/python-route-command](examples/python-route-command). See
[installed web-app routing](docs/installed-web-apps.md) for automatic
Chromium-family app discovery and platform limitations.

## Supported platforms (planned)

- NixOS with Nix-installed browsers on Sway (Wayland)
- macOS, Apple Silicon first

Other distributions and package formats are outside initial support.
