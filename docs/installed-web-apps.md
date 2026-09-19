# Installed web-app routing

Brouter automatically discovers supported installed web apps. No additional
configuration is required.

## Decision order

For HTTP(S) URLs, routing order is:

1. An explicit `route_command` target.
2. The first matching static rule.
3. The most specific matching installed web-app scope.
4. The configured browser default.

An exact `@default` response from `route_command` defers to this complete
pipeline. A route-command error remains terminal. If the selected installed
app disappears between discovery and launch, brouter refreshes once and
re-evaluates. A normal process-launch failure is reported and is not silently
sent to another browser.

## Matching

Matching uses parsed URL components. Scheme, normalized hostname, and
effective port must match exactly. Scope uses path-segment boundaries, so
`/work` does not match `/workspace`; the longest scope wins. Equal scopes use
stable app ID and launcher identity ordering. Credentials, malformed URLs,
non-HTTP(S) URLs, and ambiguous metadata are not used to select an app.

The clicked URL is passed as one structured argument. No manifest is fetched,
network request is made, or full URL is written to the routing log or cache.
The opt-in routing log identifies an installed-app target with a short opaque
stable hash, never its path, origin, or display name.

## Platform support and limitations

- **macOS:** generated Chrome and Brave `.app` bundles are scanned only in
  `Chrome Apps.localized` and `Brave Browser Apps.localized` roots under the
  standard system and user application locations. The adapter requires a
  known Chromium-family bundle identifier, the expected parent browser ID,
  `CFBundleExecutable=app_mode_loader`, a regular executable, and bounded,
  safe `Info.plist` metadata. A root `CrAppModeShortcutURL` uses the root scope of that exact
  origin. A non-root shortcut URL is only eligible when explicit scope
  metadata is present. `CrAppModeShortcutURL` is otherwise a launch URL, not a
  manifest scope; brouter does not fetch a manifest or infer a broader scope.
- **Linux:** generated Chrome and Brave `.desktop` entries are scanned in the
  supported XDG application locations. Entries must provide explicit URL and
  authoritative scope metadata, or a recognized URL in `Exec` plus explicit
  scope metadata. `Exec` is parsed according to freedesktop grammar without a
  shell, and ambiguous field codes, relative/untrusted executables, and
  arbitrary desktop entries are rejected. The final canonical browser
  executable must be regular, non-writable, and under `/usr`, `/opt`,
  `/run/current-system/sw`, `/nix/store`, or `/snap`; PATH is resolved only
  during discovery and is never consulted again at launch.
- Chromium Linux entries containing only `--app-id` do not expose origin or
  scope. They are intentionally unsupported; brouter does not inspect browser
  LevelDB files, guess from app IDs, or fetch network metadata.
- The current baseline is macOS and NixOS with the existing Sway/Wayland
  support. Windows, other browser engines, Flatpak/Snap/portal integration,
  app installation, and default-handler changes are outside this feature.

Discovery metadata is treated as user-writable untrusted input. Scans are
bounded, cache snapshots are invalidated by metadata changes, launch identity
is checked again before spawning, and all process execution bypasses shells.
The cache is process-local and is not persisted. A same-user attacker can
still race a file between the final check and OS launch; exact paths,
canonical executable checks, fingerprints, and trusted roots bound that risk
but cannot eliminate filesystem TOCTOU.
