# macOS URL handler

`dist/BrouterHandler.app` is a minimal macOS application that receives
http and https URLs from the operating system and forwards them, as
structured arguments, to the brouter binary embedded inside the bundle.
The native shim contains no routing logic: everything is decided by
brouter (the same binary as the CLI), using the user's own
configuration.

Build (macOS only, from the repository root):

```sh
make macos-handler        # produces dist/BrouterHandler.app
```

Nix packaging (TASK-0017): `nix build .#brouter-handler` on Darwin
produces the CLI at `result/bin/brouter` and the same app bundle at
`result/Applications/BrouterHandler.app` — deterministic arm64,
ad-hoc signed, with the dropdown-eligibility document types. Copy the
bundle from `result/Applications/` (never the store path itself) to
`/Applications` and follow the registration steps below. The Linux
output of the same flake remains the desktop wrapper and entry.

## What it does

- Declares http and https URL schemes in its Info.plist.
- Receives URLs while running (Apple Events) and when launched (argv);
  sequential URLs are each forwarded in order.
- Spawns the embedded brouter as `brouter open --config <user config>
  <url>` — structured argv, never a shell, independent of working
  directory and PATH. The config path follows the standardized Unix
  contract (TASK-0020): absolute `$XDG_CONFIG_HOME` wins, otherwise
  `$HOME/.config/brouter/config.toml` — also on macOS; there is no
  Application Support fallback and no automatic migration. If your
  config lives at the old macOS location, move it:

  ```sh
  mkdir -p ~/.config/brouter
  mv ~/Library/"Application Support"/brouter/config.toml ~/.config/brouter/
  ```

  GUI caveat: apps launched by LaunchServices do not inherit shell
  startup variables, so an `XDG_CONFIG_HOME` exported only in a shell
  rc file is invisible to the handler — the effective default is then
  `$HOME/.config/brouter/config.toml`. To make XDG apply to GUI
  launches as well, set it in a launchd-visible way (for example
  `launchctl setenv XDG_CONFIG_HOME /absolute/path`) and rely on the
  same absolute path everywhere.
- Appends every event and failure to a diagnostics log:
  `~/Library/Logs/brouter-handler.log` (override with
  `BRROUTER_HANDLER_LOG` for testing).

Known limitation (documented, deliberate): the child brouter process is
spawned without waiting, so a launch failure after spawn is visible in
the log rather than as the handler's exit status. Config and URL
validation failures still print to the same log before any launch.

## Installation

1. Build the bundle (`make macos-handler`).
2. Copy it to a stable location:
   `cp -R dist/BrouterHandler.app /Applications/` (or `~/Applications`).
3. Register it with LaunchServices (makes it eligible as a handler):
   `/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister -f /Applications/BrouterHandler.app`
4. Choose it explicitly as the default browser: System Settings →
   Desktop & Dock → Default web browser → BrouterHandler. Selection
   is manual and explicit: the bundle never requests handler status on
   its own, and brouter never changes the default silently.

   **Status correction (PR18 defect follow-up):** the version of this
   bundle merged in PR18 declared only URL schemes and did **not**
   appear in the Default Web Browser dropdown on macOS 26 — that
   instruction was unverified as written. LaunchServices populates the
   dropdown from apps claiming html/xhtml document types alongside the
   schemes (it derives the browser-category UTI itself; verified
   against Finicky, whose registered claims show exactly this
   pattern). The bundle now declares those document types; run
   `scripts/probes/macos-dropdown-eligibility.sh` after reinstalling
   to confirm the registered claims, then verify the dropdown
   visually and record it as user-attested evidence.

## Restoration and uninstall

The previous default browser is not modified by installing the bundle;
selection happens only through step 4. To restore:

1. Set the previous browser as default in System Settings (its bundle
   identifier can be read before switching:
   `defaults read com.apple.LaunchServices/com.apple.launchservices.secure LSHandlers`).
2. Remove the app: `rm -rf /Applications/BrouterHandler.app`.
3. Unregister:
   `lsregister -u /Applications/BrouterHandler.app` (optional; stale
   registrations are cleaned by LaunchServices).
4. Optionally remove the diagnostics log:
   `~/Library/Logs/brouter-handler.log`.

## Signing status

The bundle is **ad-hoc signed** (`codesign --force --sign -`):
unsigned binaries are refused by LaunchServices, but an ad-hoc
signature carries no identity. On the build machine the bundle runs
directly; copying it to another Mac may require removing the
quarantine attribute (`xattr -dr com.apple.quarantine`) or right-click
→ Open. Proper Developer ID signing and notarization are deliberately
out of scope until a signing identity exists; this status is honest and
unchanged by packaging.

## Supported platforms

Target and tested: **arm64 only**, macOS 26.x on Apple Silicon. The
build pins `GOARCH=arm64` and `clang -arch arm64` so the bundle is
deterministically arm64 even when built from a shell running under
Rosetta (a translated shell would otherwise emit an x86_64 shim beside
the arm64 Go binary). Running the bundle under Rosetta or on Intel is
untested and unsupported. The bundle declares a minimum of macOS 13;
older versions are untested. Linux and other platforms use their own
delivery mechanisms and are out of scope for this handler.

## Dropdown eligibility and repair

If the app is missing from the Default Web Browser dropdown:

1. `scripts/probes/macos-dropdown-eligibility.sh` — read-only report
   of every registration (live and dangling), claimed schemes and
   document types, and the specific missing markers. It never mutates
   state.
2. Rebuild (`make macos-handler`) and replace the installed copy (this
   is a user action; brouter never touches an installed bundle).
3. Re-register: `lsregister -f /Applications/BrouterHandler.app`
   (targeted registration; never reset the whole LaunchServices
   database).
4. Re-run the probe: it must report the browser-category claim derived
   from the html/xhtml document types at a live path. The final
   dropdown check is visual (System Settings) and user-attested.

## Recent Activity (TASK-0023)

The menu's Recent Activity submenu shows the last 20 events of this
session, newest first: receipt of each URL event (or argv hand-off),
preflight, and dispatch. Every entry is built from allowlisted fields
— time, session-local request ID, entry point, event kind, optional
error category — so URLs, hosts, query strings, profiles, and paths
can never appear in the menu.

What the menu can and cannot observe:

- receipt: a URL event arrived (recorded before validation).
- preflight failed: missing-embedded-binary or
  unavailable-config-directory (directly observable shim-side).
- dispatch started, outcome unknown: the child brouter was spawned.
  Config validation, browser resolution, and page load happen in the
  child and are not observable by the shim; they remain unknown here.
- dispatch failed: spawn-failed (with errno), when the OS refused the
  spawn itself.

Clear Recent Activity empties the in-memory list only; it never
touches diagnostic files and cannot cancel already-dispatched
children. Reveal Config and Reveal Diagnostic Log open Finder
selections for the already-resolved paths; they never create or
overwrite files, and show "(missing)" when the target does not exist.
There is no persistent activity history, no export, and no telemetry.

## Menu bar presence (TASK-0022)

While running, the handler shows one menu-bar status item (a template
icon that adapts to light and dark menu bars). Its menu lists
"Brouter — running" and "Quit Brouter". The label claims only that the
process is running — not health, routing success, or default-browser
status.

- Quit ends the handler process only. It never terminates browsers,
  rewrites the config, or changes OS defaults. Quit is not a
  persistent disable switch: a later OS URL delivery may relaunch the
  selected handler.
- One status item exists per app instance; repeated OS opens to the
  running instance never add items. Launching the handler twice (for
  example an installed copy plus a dev build) yields one item per
  instance — quit the old instance before installing a new build.
- CLI `open`, `validate`, and `explain` remain headless: they never
  create a status item.

## Event loop and logging limitations (TASK-0021)

- LaunchServices delivers URL Apple Events only to an initialized
  NSApplication; the shim runs `[NSApplication sharedApplication]` and
  its event loop. A bare run loop does not service the event queue —
  `open -a` fails with the LaunchServices timeout `-1712` (the defect
  this fixes).
- While running, the app stays alive to receive further events; quit
  it like any app (e.g. `osascript -e 'tell application id
  "com.cristianoliveira.brouter.handler" to quit'`).
- The shim's own log lines are URL-free (event markers and status
  codes only); query strings and fragments are never written to the
  log. The child brouter's stderr is appended to the same log and may
  contain config paths, not URLs.
- AppKit is linked; the shim remains a UIElement app (no Dock icon).

## Diagnostics

All events and failures are appended to
`~/Library/Logs/brouter-handler.log`: URL forwarded, spawn failures,
missing embedded binary. Because the shim does not wait for the child
browser, a browser-side failure after spawn appears in the browser, not
the log; the log records that the URL was handed over. Config and URL
validation errors print to the same log before any browser launch.
