# Browser router delivery plan

## Problem
One default browser does not represent separate work contexts. Users need predictable link routing without maintaining a GUI configuration or guessing why a rule matched.

## Confirmed decisions
- Implementation language: Go.
- Supported operating systems: NixOS with Nix-installed browsers on Sway (Wayland), and current macOS (Darwin), Apple Silicon first. Intel is unverified, not an initial support requirement. Other Linux distributions and Flatpak/Snap are outside initial support.
- Repository and Go module path: `github.com/cristianoliveira/brouter` (verified from origin).
- Configuration is a file suitable for dotfiles version control.
- The main workflow routes links to different browsers. Optional browser profiles are desired, but remain a separate capability.
- Favor Brave and Chrome-based browsers where concessions are necessary; do not design out other browsers.
- Provide diagnostics for configuration and regex matching.

## Working defaults — reversible, not product approval
These defaults unblock implementation while preserving the unresolved decisions below. A later decision can change the adapter or parser without changing the pure routing contract.

- Use TOML with named targets, ordered first-match rules, and an explicit default target.
- Support exact-host, explicit-subdomain, and full-URL Go-regex matchers. A rule has exactly one matcher in the first version; combinations wait.
- Evaluate regexes against the original URL string. Preserve the original URL for forwarding. Reject malformed URLs and non-HTTP(S) schemes.
- Use `brouter open URL`, `brouter validate`, and `brouter explain URL`; no GUI or always-on daemon unless OS evidence requires one.
- Accept an explicit config path. The fallback is `$XDG_CONFIG_HOME/brouter/config.toml` on Linux and `~/Library/Application Support/brouter/config.toml` on macOS. Do not merge files or override fields through the environment.
- Launch failures are visible; there is no silent cross-profile fallback. Do not persist URLs or emit telemetry by default.
- Pin Go 1.24 during bootstrap. Use macOS 13 (Ventura) arm64 as the provisional deployment floor; TASK-0006 may revise it from real event-integration evidence. No Intel support is claimed.
- Prefer a flake package for Linux first. Defer choosing a NixOS module versus Home Manager integration until TASK-0014; Home Manager usage is unknown.
- Treat optional profiles as desired but provisional for the first usable release. TASK-0012 must prove them or document an explicit deferral; core routing does not wait on profiles.
- Use conventional commits, portable Make commands, and deterministic tests before logic changes.

## Deferred decisions (visible and intentionally not approved)
1. TOML and the initial matcher semantics need product confirmation. The working defaults above are implementation choices only.
2. The NixOS declarative integration (NixOS module or Home Manager) waits for TASK-0007/TASK-0014 evidence.
3. macOS signing/notarization and any local-only unsigned first release remain open. TASK-0006 records feasibility; TASK-0013 records packaging status.
4. Whether profiles ship in the first release remains open; no task may silently drop the capability or claim release support.

These open decisions do not block the Go skeleton or isolated domain/config work. Tasks that require a final product decision must keep the assumption visible and update this ledger when it changes. No platform support is claimed until its smoke checks run.

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

## Routing examples and explicit exclusions
- Personal: `github.com` defaults to Brave; `work.example.com` selects Chrome; an explicit subdomain rule prevents `other.example.com` from matching.
- Work profile (provisional): a work target may select a machine-local Chrome/Brave profile identifier; display names are not portable.
- Regex: a narrowly scoped full-URL rule can route a supported issue tracker, while unmatched HTTP(S) URLs use the explicit default.
- Unsupported initially: non-HTTP(S) schemes, other Linux distributions, Flatpak/Snap browsers, Intel macOS, GUI chooser/editor, source-application rules, browser extensions, native-app routing, cloud sync, telemetry, and arbitrary in-browser navigation interception.

## Sources and limitations
Product references: https://sindresorhus.com/velja and https://choosy.app/.
OS handler routing only sees links delegated to the OS. Go needs macOS URL-event integration and an app bundle; whether a thin native helper is useful is a feasibility decision, not settled architecture.
