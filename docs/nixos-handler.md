# NixOS desktop handler

`brouter-handler` is a Nix flake package for NixOS + Sway (Wayland): it
ships the brouter binary, a desktop entry declaring the http and https
schemes, and a thin wrapper that hands each URL to brouter with the
user's own configuration. All packaging is declarative and offline;
routing logic lives only in the Go binary — never in Nix expressions
and never in the wrapper.

Build (from the repository root):

```sh
nix build .#brouter-handler
```

## What the package contains

- `bin/brouter` — the CLI binary built from this repository.
- `bin/brouter-handler` — the desktop-entry wrapper. Plumbing only: it
  resolves the config path exactly like the CLI
  (`brouter/config.toml` under an **absolute** `$XDG_CONFIG_HOME`;
  a relative or unset value falls back to `$HOME/.config`; missing
  HOME with no absolute XDG fails visibly before any launch) and
  forwards structured arguments (`open --config <path> <url>`).
- `share/applications/brouter-handler.desktop` — declares
  `x-scheme-handler/http` and `x-scheme-handler/https` with
  `Exec=<package>/bin/brouter-handler %u` (absolute, no shell, single
  URL per invocation).

## Installation (declarative)

Add the package and an explicit opt-in default in the NixOS
configuration:

```nix
environment.systemPackages = [ brouter.packages.${system}.brouter-handler ];

xdg.mime.defaultApplications = {
  "x-scheme-handler/http"  = "brouter-handler.desktop";
  "x-scheme-handler/https" = "brouter-handler.desktop";
};
```

`nixos-rebuild switch` materializes the defaults; no `xdg-mime
default` call and no other imperative mutation is involved. Nothing is
changed behind the Nix configuration.

## Rebuild and upgrade behavior

The desktop-entry file name (`brouter-handler.desktop`) is stable, so a
registration made through the declarative options survives rebuilds.
The `Exec` path is the immutable store path of that specific build and
is regenerated with each rebuild; the entry itself is reinstalled with
the package. No user config is ever bound to a store hash: the wrapper
resolves the config from the environment at runtime, independent of
any shell startup files.

## Config location contract (TASK-0020)

`brouter/config.toml` beneath one root, identical for the CLI and both
desktop handlers:

1. `--config <path>` always wins.
2. `$XDG_CONFIG_HOME` when it is set to an **absolute** path.
3. Otherwise `$HOME/.config` — including on macOS; there is no
   Application Support fallback and no automatic migration. If you
   kept your config at the old macOS location, move it yourself:

   ```sh
   mkdir -p ~/.config/brouter
   mv ~/Library/"Application Support"/brouter/config.toml ~/.config/brouter/
   ```

## Failure channel

GUI launchers hide stderr and ignore exit status (TASK-0007 evidence),
so the wrapper appends all diagnostics to
`$XDG_STATE_HOME/brouter/handler.log` (default
`~/.local/state/brouter/handler.log`) and, on failure, raises one
bounded desktop notification whose text contains no URL. If
notification delivery fails, the log still records the failure. The
wrapper exits with brouter's status; the launcher may ignore it.

## Changing, removing, restoring the default

All three are explicit edits to the same declarative options:

- Change: point the scheme keys at another desktop entry ID.
- Restore: point them back at the previous handler (for example
  `brave-browser.desktop`, as recorded on the TASK-0007 target).
- Remove: delete the keys; the xdg defaults file is regenerated on the
  next rebuild and the package can be dropped from
  `environment.systemPackages`.

## Supported and unsupported

Supported (evidence-based): NixOS with Sway on Wayland, where the
TASK-0007 probe proved `xdg-open` selects a desktop handler for these
schemes without involving portals, and browser forwarding preserves
URLs exactly.

Not supported and not promised: other distributions, Flatpak/Snap or
other sandboxes, GNOME/KDE or X11 sessions (untested), Home Manager
modules (not required — the NixOS xdg options are sufficient). The
package builds on darwin hosts for development, but desktop handling
is a Linux feature.

## Smoke template for the agreed desktop (user-attested)

On the NixOS + Sway target, after installing and selecting the handler:

```sh
nix build .#brouter-handler
ls result/share/applications/        # brouter-handler.desktop present
grep ^Exec result/share/applications/brouter-handler.desktop

# 1. real external click: open a link from another app (e.g. a chat
#    message) and confirm the URL lands in the browser chosen by the
#    user's brouter config, with the config's routing applied.

# 2. warm browser: click a second link while the browser is open and
#    confirm delivery into the existing session.

# 3. failure case: point brouter config at a target that fails (for
#    example a profile directory that does not exist), click a link,
#    confirm the Mako notification appears and the log records it:
tail -5 "${XDG_STATE_HOME:-$HOME/.local/state}/brouter/handler.log"

# 4. defaults intact: confirm the declaration, not a mutation, drives
#    the default:
cat ~/.config/mimeapps.list | grep x-scheme-handler
```

Record the observed behavior for each case in the evidence doc.
