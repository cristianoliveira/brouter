# Browser router delivery plan

## Problem
One default browser does not represent separate work contexts. Users need predictable link routing without maintaining a GUI configuration or guessing why a rule matched.

## Confirmed decisions
- Implementation language: Go.
- Supported operating systems: NixOS with Nix-installed browsers on Sway (Wayland), and current macOS (Darwin), Apple Silicon first. Intel is unverified, not an initial support requirement. Other Linux distributions and Flatpak/Snap are outside initial support.
- Repository and Go module path: github.com/cristianoliveira/brouter (verified from origin).
- Configuration lives in a file suitable for dotfiles version control.
- Main workflow routes to different browsers. Optional browser profiles are also desired.
- Favor Brave and Chrome-based browsers where concessions are necessary; do not design out other browsers.
- Provide diagnostics for configuration and regex matching.

## Proposed defaults — not yet agreed
- TOML, named targets, ordered first-match rules, explicit default target.
- Exact host, explicit subdomain matching, and full-URL Go regex; no lookaround or backreferences.
- A rule has exactly one matcher in the first version; more complex combinations wait.
- Known browser IDs resolve on each OS; optional profile identifiers are browser-local, not portable display names.
- `brouter open URL`, `brouter validate`, and `brouter explain URL`; no GUI or always-on daemon unless OS evidence requires one.
- Explicit config-path override, otherwise one documented user config location per OS. No implicit file merging or environment-based field overrides initially.
- Launch failure is visible; no silent cross-profile fallback. HTTP/HTTPS only initially.
- No URL persistence or telemetry by default.
- Conventional commits; portable Make commands; deterministic tests before logic changes.

## Decisions needed
1. Preferred declarative installation path (NixOS module or Home Manager); Home Manager usage is not yet confirmed. Desktop is confirmed as Sway, which uses Wayland. Propose flake package first; defer module choice until handler integration is proven.
2. Current macOS, Apple Silicon first, is confirmed. Local machine reports arm64 and macOS 26.6.2; choose an explicit deployment minimum during bootstrap. Release signing/notarization expectations remain open.
3. Confirm TOML and initial matcher semantics.
4. Initial Go toolchain version; module path is confirmed above.
5. Whether optional profiles must ship in first usable release. Planned as a separate deliverable, not forgotten.

Tasks that need these answers state the dependency explicitly. No platform support is claimed until its smoke checks run.

## Delivery slices
- **Foundation:** TASK-0001–0005 — support contract, Go skeleton, normal gate, hooks/CI, advanced checks.
- **Platform feasibility:** TASK-0006–0007 — prove real URL receipt and forwarding, including already-running browsers.
- **Headless router:** TASK-0008–0011 — config, matching, diagnostics, browser launching.
- **Profiles:** TASK-0012 — optional Chrome/Brave profiles, with explicit support limits.
- **Desktop integration:** TASK-0013–0014 — installable macOS and Linux handlers.
- **Release evidence:** TASK-0015 — end-to-end acceptance and installable artifacts.

## Guardrail contract to implement
`make check` is the one normal lint/type-and-test gate. On success stdout is exactly `true`. On failure output begins with `false`, followed by bounded diagnostics, and exit status is nonzero. Hooks, watcher, and CI call this command. Formatting, static analysis, race tests, coverage, security, and architecture checks have explicit separate commands and release enforcement; do not quietly expand the normal gate.

Use Go's normal conventions: CLI composition under `cmd/brouter`, pure domain decisions under `internal/domain`, platform/config/process adapters under `internal/infra`, and colocated `_test.go` and `testdata` fixtures. Add shared packages only when needed. Document boundaries in `docs/ARCHITECTURE.md`; no empty framework scaffolding.

## Board operation
Tasks live in `plans/todo/`; completed files move to `plans/done/`. Folder is lifecycle truth. `depends_on` references stable task IDs, not filenames. Only use `doing` when work has actually started. Each task requires acceptance evidence and a conventional commit referencing its ID before closure.

## Non-goals for initial release
GUI chooser/editor, source-application rules, browser extensions, native-app routing, tracking removal, cloud synchronization, and interception of arbitrary in-browser navigation.

## Sources and limitations
Product references: https://sindresorhus.com/velja and https://choosy.app/.
OS handler routing only sees links delegated to the OS. Go needs macOS URL-event integration and an app bundle; whether a thin native helper is useful is a feasibility decision, not settled architecture.
