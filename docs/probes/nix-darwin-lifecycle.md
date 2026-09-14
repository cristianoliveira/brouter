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

Benign external URL delivery (visible destination = example.com
renders):

```sh
open "https://example.com/"   # cold: selected browser launches
open "https://example.com/"   # warm: repeat while the browser runs
```

- [ ] cold: the selected browser launches and example.com renders
- [ ] warm: the repeated link opens in the running session, page
      renders again

Invalid-config and missing-browser failures — verified with an
**isolated config** passed via `--config`, so the user's real config
is never touched. The handler surfaces failures **in the diagnostics
log only** on macOS (`~/Library/Logs/brouter-handler.log`; the shim
has no notification implementation).

```sh
work=$(mktemp -d)

# Validation failure — malformed TOML (validate parses and
# semantic-checks; it does NOT test profile existence):
printf 'default = "x"\n[browsers\n' > "$work/broken.toml"
brouter validate --config "$work/broken.toml"   # exits nonzero: parse error
```

- [ ] `brouter validate` exits nonzero naming the parse/semantic
      problem

Open-time failures (validate passes; the CLI open and the handler
record the failure):

```sh
printf 'default = "ghost"\n\n[browsers.ghost]\nbrowser = "brave"\nprofile = "does-not-exist"\n' > "$work/profile.toml"
brouter validate --config "$work/profile.toml"   # passes: profile existence is an open-time check
brouter open --config "$work/profile.toml" "https://example.com/"   # fails visibly in the terminal
tail -3 ~/Library/Logs/brouter-handler.log   # only if this URL went through the handler

printf 'default = "ghost"\n\n[browsers.ghost]\ncommand = "/nonexistent/browser %%u"\n' > "$work/ghost.toml"
brouter open --config "$work/ghost.toml" "https://example.com/"   # launch failure, visible
```

- [ ] CLI open with the isolated missing-profile config fails visibly
      (profile-resolution error)
- [ ] CLI open with the isolated missing-browser config fails visibly
      (launch error); nothing launches silently

GUI-with-user-config failure (the handler resolving the user's real
`~/.config` config through a GUI-originated event): **UNTESTED** —
pending a safe dedicated test environment (e.g. a launchd-visible
test-only `XDG_CONFIG_HOME`); intentionally not exercised against the
real user config.

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

Post-update forwarding proof — capture `before` BEFORE the switch and
`after` afterwards; they are separate commands with separate state:

```sh
# BEFORE the update rebuild:
readlink "/Applications/Nix Apps/BrouterHandler.app" | tee /tmp/brouter-before.link
```

```sh
# AFTER the update rebuild:
after=$(readlink "/Applications/Nix Apps/BrouterHandler.app")
before=$(cat /tmp/brouter-before.link)
[ "$before" != "$after" ] && echo "link moved to the new build"
"$after/Contents/MacOS/brouter" validate --config ~/.config/brouter/config.toml

# No stale registrations: every BrouterHandler registration must
# point at an existing directory (or use the read-only probe).
/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister \
  -dump | grep "^path:" | grep BrouterHandler \
  | sed 's/^path: *//; s/ (0x[0-9a-f]*)$//' \
  | while read -r p; do [ -d "$p" ] && echo "live: $p" || echo "STALE: $p"; done
./scripts/probes/macos-dropdown-eligibility.sh    # read-only; flags STALE itself
open "https://example.com/"   # forwarded by the NEW embedded binary
```

- [ ] link target moved (`before != after` across the rebuild)
- [ ] the NEW embedded CLI validates the config
- [ ] no `STALE:` lines; probe reports no dangling registrations
- [ ] a clicked URL is delivered by the updated bundle (page renders)

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

Ordinary links after restoration:

```sh
open "https://example.com/"   # must open in the RESTORED default
```

- [ ] the link opens in the restored default browser (Chrome today),
      not BrouterHandler

## Signing, Gatekeeper, and distribution limits

- The bundle is **ad-hoc signed**: valid on the machine that built it;
  no Developer ID identity, no notarization. Other machines may need
  `xattr -dr com.apple.quarantine` (or right-click → Open) for
  downloaded/copied bundles.
- **Same-machine only**: the Nix store paths, the `/Applications/
  Nix Apps` link, and the LaunchServices registration are per-machine.
  Transferring the bundle or store paths to another machine, or
  redistributing cached builds, is unsupported — rebuild from the
  flake on the target machine instead.

## 5. Known baseline finding

The manually installed `/Applications/BrouterHandler.app` predates the
PR20 metadata fix (scheme-only, absent from the dropdown). Case A's
Nix-managed install supersedes it: remove the manual copy (user
action) once the Nix Apps link is in place, so one installation owns
the URL scheme.
