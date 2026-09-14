# Declarative nix-darwin installation

How to install brouter and its macOS URL handler on NixOS-style
declarative macOS management with nix-darwin, using the flake's public
package output. Everything here was executed against the pinned
revision on macOS 26 / aarch64-darwin; commands and observed outputs
are recorded in the Verification section.

## What the package provides

`brouter.packages.aarch64-darwin.brouter-handler` contains:

- `bin/brouter` — the CLI, linked into `/run/current-system/sw/bin` by
  nix-darwin.
- `Applications/BrouterHandler.app` — the URL handler app; nix-darwin's
  activation script links app bundles from `environment.systemPackages`
  into `/Applications/Nix Apps/`.

The bundle is arm64 only (the shim is compiled `-arch arm64`;
`x86_64-darwin` packages are not produced) and ad-hoc signed. The
package carries no Linux wrapper or desktop entry.

## Pinned flake setup

```nix
# flake.nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    nix-darwin = {
      url = "github:nix-darwin/nix-darwin";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    # Pin brouter to an exact revision; `nix flake lock` records it in
    # flake.lock, which is the effective pin for rebuilds.
    brouter.url = "git+https://github.com/cristianoliveira/brouter?rev=<REV>";
  };

  outputs = inputs: {
    darwinConfigurations.example = inputs.nix-darwin.lib.darwinSystem {
      system = "aarch64-darwin";
      modules = [
        {
          system.stateVersion = 7;
          # Recommended: lets nix-darwin manage the Nix Apps folder for
          # your user during darwin-rebuild (GUI-session permission
          # checks).
          system.primaryUser = "youruser";

          environment.systemPackages = [
            inputs.brouter.packages.aarch64-darwin.brouter-handler
          ];
        }
      ];
    };
  };
}
```

Apply with `darwin-rebuild switch --flake .#example`.

## Where things land

- CLI: `/run/current-system/sw/bin/brouter`.
- Handler app: `/Applications/Nix Apps/BrouterHandler.app` — a link
  into the Nix store, created and refreshed by nix-darwin's activation
  script. This is the Nix-managed app. Do not copy it to the
  `/Applications` root and do not keep a second manual copy from the
  non-Nix install flow; one installation should own the URL scheme.
- Config: `~/.config/brouter/config.toml`, or an **absolute**
  `$XDG_CONFIG_HOME` (set in a launchd-visible way, e.g.
  `launchctl setenv XDG_CONFIG_HOME /absolute/path`, if you want it to
  apply to GUI launches). There is no Application Support fallback and
  no store-hash-bound config: the handler resolves the path from the
  environment at runtime, and your TOML never enters the Nix store —
  keep secrets and personal defaults out of any flake.

## Config from a dotfiles repository (optional)

To keep the config in a dotfiles repository, symlink it into place —
do not overwrite a config that already exists:

```sh
cfg=~/.config/brouter/config.toml
mkdir -p ~/.config/brouter
if [ -e "$cfg" ]; then
  echo "refusing: $cfg already exists — merge manually" >&2
else
  ln -s ~/dotfiles/brouter/config.toml "$cfg"
fi
brouter validate --config "$cfg"
```

This is plain shell plus a symlink; no Home Manager and no custom
module is involved.

## Explicit default selection

Registration and selection are manual and explicit; installing or
rebuilding never changes the default browser:

1. Register the linked bundle once after the first install:

   ```sh
   /System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister \
     -f "/Applications/Nix Apps/BrouterHandler.app"
   ```

2. Select it: System Settings → Desktop & Dock → Default web browser →
   BrouterHandler.

Validate your configuration the standard way before pointing anything
at it:

```sh
brouter validate --config ~/.config/brouter/config.toml
# config OK: targets 1, rules 0, default "…"
```
3. Check eligibility at any time (read-only):
   `scripts/probes/macos-dropdown-eligibility.sh` from the repository.

## Rebuild and update semantics

- `darwin-rebuild switch` replaces the store paths and refreshes the
  `/Applications/Nix Apps` link; the desktop-entry file name and
  bundle identifier are stable, so the registration and your default
  selection survive updates.
- `darwin-rebuild --rollback` returns to the previous generation and
  its link targets.
- Store paths of older generations stay valid until garbage-collected;
  the config file is never touched by rebuilds.

## Removal and restoration

1. Remove the package from `environment.systemPackages` and run
   `darwin-rebuild switch` — the `/Applications/Nix Apps` link is
   removed.
2. Restore the previous default browser in System Settings (explicitly,
   e.g. Brave or Safari).
3. Optionally unregister the app path LaunchServices knows — that is
   the Nix Apps link, not the store path itself:
   `lsregister -u "/Applications/Nix Apps/BrouterHandler.app"` — and
   remove the diagnostics log
   (`~/.local/state/brouter/handler.log`).

## Not included (by scope)

No Home Manager or custom nix-darwin module (the standard
`environment.systemPackages` option is sufficient), no automatic
default-browser or config writes, no Intel/universal build, and no
launchd user services. The `make macos-handler` script flow remains
the non-Nix installation alternative; do not maintain both installs
simultaneously.

## Verification record (executed)

```sh
rev=089044940cf87898533be16c7ef565a482b67871
nix build "git+file:///Users/cristianoliveira/other/brouter?rev=$rev#packages.aarch64-darwin.brouter-handler" -o /tmp/t18-pkg
ls /tmp/t18-pkg/bin /tmp/t18-pkg/Applications/BrouterHandler.app/Contents/MacOS
# bin/brouter  Applications/BrouterHandler.app/Contents/MacOS/{brouter,BrouterHandler}
```

A minimal nix-darwin configuration with the pinned input above
evaluated successfully (`config.system.build.toplevel` resolved to a
darwin-system derivation) and the generated activation script contains
the `/Applications/Nix Apps` linking logic. Final dropdown membership
is confirmed visually in System Settings and recorded as
user-attested.
