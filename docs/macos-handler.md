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

## Diagnostics

All events and failures are appended to
`~/Library/Logs/brouter-handler.log`: URL forwarded, spawn failures,
missing embedded binary. Because the shim does not wait for the child
browser, a browser-side failure after spawn appears in the browser, not
the log; the log records that the URL was handed over. Config and URL
validation errors print to the same log before any browser launch.
