# Packaged macOS handler — verification evidence (TASK-0024)

Scope: bounded packaged verification of the source bundle and the
offline `aarch64-darwin` Nix package. All automated checks below were
executed on Apple Silicon (macOS 26.6, arm64). Graphical observations
that require an interactive session are listed as **user-attested** —
they are not claimed as automated results.

Revision under verification: main after #27 (TASK-0023), which is the
source of both the source-built bundle and the Nix package.

## Automated checks (all passing)

- Nix package builds offline (`nix build --offline .#brouter-handler`).
- Both executables are arm64-only Mach-O (`file`):
  `Contents/MacOS/BrouterHandler`, `Contents/MacOS/brouter`.
- Final signature valid: `codesign --verify --deep --strict` passes.
- Bundle metadata is byte-identical to the source `Info.plist`
  (plutil round-trip diff): bundle id
  `com.cristianoliveira.brouter.handler`, `LSUIElement` true,
  http/https URL schemes, `LSHandlerRank Alternate` document types,
  `LSMinimumSystemVersion 13.0`.
- Icon assets: the bundle intentionally declares **no**
  `CFBundleIconFile`; the menu icon is a runtime template SF Symbol.
  No `.icns` is required or shipped. Locked by
  `TestMacosHandlerBundleMetadata`.
- Headless CLI from the package (`bin/brouter`): `validate` exits 0 on
  a valid fixture config, 1 on missing config; `explain` exits 0 and
  launches nothing; invalid scheme exits 1. Output and exit codes
  unchanged from the CLI contract.
- Real OS-open delivery from a **temporary copied bundle** (no
  installation, no `lsregister`, no defaults writes): cold `open -a
  <copy> https://example.com/brouter-qa-cold` → exit 0, handler
  started, presence installed; a second warm open reached the same
  instance (log shows two "forwarding URL event" entries, one process
  throughout). The previously observed `-1712` timeout cannot recur
  (TASK-0021 fix, regression-locked).
- Downstream failure stays **outcome unknown**: both QA URLs were
  delivered to the real embedded brouter under an isolated
  `HOME`/`XDG_CONFIG_HOME` with no config file. The child's failure
  appears only in the diagnostics log; the shim recorded receipt →
  preflight → dispatch and stayed alive. No success was claimed
  anywhere.
- Safety: `defaults read com.cristianoliveira.brouter.handler` returns
  nothing before and after (domain never created); no files were
  created in the isolated config directory; the user's real config,
  `/Applications`, and login items are untouched; the test instance
  was quit gracefully via AppleScript (no process kills). Source-level
  guards (`TestMacosHandlerSourceShipsNoDefaultsOrRegistrationWrites`)
  forbid `NSUserDefaults`, `lsregister`, default-handler registration,
  and shell-outs in the shim.
- Linux regression: the full scripts suite (including NixOS handler
  and Linux flake tests) passes; no Linux tray or UI is claimed.

## User-attested (graphical, not automated)

To be confirmed in an interactive session (this verification host
auto-hides the menu bar, so icon pixels are not capturable here):

- Menu icon visibility and legibility in light and dark appearance.
- Accessible label/tooltip via Accessibility Inspector; keyboard
  navigation to the status item and menu.
- One menu item per running instance, seen visually; no Dock icon and
  no focus stealing on URL delivery.
- Recent Activity rows observed in the open menu after real opens;
  Clear empties the list; Reveal Config / Reveal Diagnostic Log open
  Finder and show "(missing)" for absent files (no-create behavior is
  covered by code inspection and the Reveal fix in #27, but the visual
  click-through is interactive).
- Observed destination browser for a delivered URL (requires a config
  that resolves to a real browser; deliberately not exercised here to
  avoid opening browser tabs on the user's session).

## Manual checklist

1. Quit any installed copy; install a rebuilt bundle (explicit user
   action) or run a copied bundle from any path.
2. Confirm the menu-bar icon appears; check light/dark appearance.
3. Open the menu: "Brouter — running" (disabled), Recent Activity
   submenu, Reveal Config, Reveal Diagnostic Log, Quit Brouter.
4. Trigger an OS open (System Settings default selection is the user's
   explicit act); watch a receipt → preflight → dispatch started row
   appear.
5. Clear Recent Activity: rows disappear; diagnostic files unchanged.
6. Reveal items: Finder selects the file; rename it temporarily to see
   "(missing)".
7. Quit: process exits; no browsers are killed; defaults untouched.
