# TASK-0019: nix-darwin lifecycle verification — baseline and checklist

Status: **baseline verified on the real host (non-mutating); install,
update, and removal cases are user-assisted and pending attestation.**

Run date: 2026-09-14. Host: Apple Silicon macOS 26.6.2 arm64, Nix
2.28.5, nix-darwin generation `25.05.d6c2f8a`. Package pinned to
main revision `08d24ba` (post-PR23). This report contains no code
changes; every command below was either executed non-mutatingly during
this task or is prepared for the user to execute with visible expected
results. Nothing in this task ran `darwin-rebuild`, changed
LaunchServices defaults, or created/overwrote config.

## 1. Verified baseline (executed, non-mutating)

| Item | Command (abridged) | Observed |
| --- | --- | --- |
| OS / arch | `sw_vers`, `uname -m` | macOS 26.6.2 (25G83), arm64 |
| Nix | `nix --version` | 2.28.5 |
| nix-darwin generation | `readlink /run/current-system` | `darwin-system-25.05.d6c2f8a` |
| Package in system? | `ls /run/current-system/sw/bin/brouter` | absent — brouter is not yet in `systemPackages` |
| Manual app copy | `lipo -archs`, `codesign --verify --deep --strict` | `/Applications/BrouterHandler.app` exists, arm64, signature valid |
| Installed metadata | `plutil -p …/Info.plist` | **scheme-only (PR18-era); no public.html/xhtml claims** — stale relative to the PR20 dropdown fix |
| LS defaults | `defaults read …LSHandlers` (read-only) | `http` and `https` → `com.google.chrome` |
| Config | `ls` both locations | no `~/.config/brouter/config.toml`, no legacy Application Support config |

Pinned package output (built into the store, out-link in /tmp only —
no activation):

- rev `08d24ba`: `bin/brouter` +
  `Applications/BrouterHandler.app/Contents/MacOS/{brouter,BrouterHandler}`
- the packaged plist **does** carry the dropdown-eligibility document
  types (`public.html`) — unlike the installed manual copy.

## 2. Exact configuration needed for the lifecycle cases

```nix
# flake.nix inputs (plus existing nix-darwin/nixpkgs inputs)
inputs.brouter.url =
  "git+https://github.com/cristianoliveira/brouter?rev=08d24ba84976ab23f1b8c374090c641bd2de7dc3";

# inside darwinConfigurations.<host> module
{
  system.stateVersion = 7;              # as hinted by nix-darwin
  system.primaryUser = "cristianoliveira";
  environment.systemPackages = [
    inputs.brouter.packages.aarch64-darwin.brouter-handler
  ];
}
```

## 3. User-assisted checklist (each result is user-attested)

Safety recap: the only mutations are the ones the user chooses to run
below. `darwin-rebuild switch` and the registration step require the
GUI session (App Management permission; see nix-darwin's
`system/applications.nix` checks).

### Case A — install

```sh
darwin-rebuild switch --flake .#<host>          # with the config above
ls -la /run/current-system/sw/bin/brouter       # exists (sw bin)
ls -la "/Applications/Nix Apps/BrouterHandler.app"   # activation link
/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister \
  -f "/Applications/Nix Apps/BrouterHandler.app"
mkdir -p ~/.config/brouter
$EDITOR ~/.config/brouter/config.toml           # user writes their config
brouter validate --config ~/.config/brouter/config.toml
./scripts/probes/macos-dropdown-eligibility.sh  # read-only report
```

- [ ] `/run/current-system/sw/bin/brouter` present after switch
- [ ] `/Applications/Nix Apps/BrouterHandler.app` link present
- [ ] probe reports the browser-category claims at the live path
- [ ] `brouter validate` prints `config OK: …`
- [ ] BrouterHandler appears in the Default Web Browser dropdown
- [ ] user selects it explicitly; dropdown/default change is theirs

### Case B — update

```sh
# bump the brouter input rev to a newer main, then:
darwin-rebuild switch --flake .#<host>
readlink "/Applications/Nix Apps/BrouterHandler.app"   # new store path
```

- [ ] link target changes to the new store path
- [ ] default selection and registration survive (stable bundle id +
      desktop identity; dropdown state unchanged unless the user
      changed it)
- [ ] `~/.config/brouter/config.toml` untouched

### Case C — remove / restore

```sh
# remove the systemPackages entry, then:
darwin-rebuild switch --flake .#<host>
ls "/Applications/Nix Apps/BrouterHandler.app" 2>/dev/null   # gone
# restore the previous default in System Settings (e.g. Chrome), then:
/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister \
  -u "/Applications/Nix Apps/BrouterHandler.app"
defaults read com.apple.LaunchServices/com.apple.launchservices.secure LSHandlers | grep -c BrouterHandler   # 0
```

- [ ] link gone after rebuild
- [ ] default restored to the previous handler (Chrome today)
- [ ] no BrouterHandler entries remain in LSHandlers
- [ ] `~/.config/brouter/config.toml` still present (removal never
      deletes user config)

## 4. Known baseline finding

The manually installed `/Applications/BrouterHandler.app` predates the
PR20 metadata fix (scheme-only, absent from the dropdown). Case A's
Nix-managed install supersedes it: remove the manual copy (user
action) once the Nix Apps link is in place, so one installation owns
the URL scheme.
