---
id: TASK-0036
title: Automatic cross-platform installed web-app routing
status: todo
depends_on: []
tags: [routing, web-app, cross-platform]
---

# Automatic cross-platform installed web-app routing

## Problem

The current router needs a configured target or a user-maintained route rule
for every origin. Chromium-family installed web apps already carry an origin
and scope, but brouter does not discover them. Requiring another configuration
layer would duplicate browser state, while guessing from application names or
paths could send a URL to the wrong app.

## Outcome

HTTP(S) routing can discover installed Brave and Chrome web apps on the
currently supported macOS and Linux platforms without any new user
configuration. A parsed, deterministic catalog participates in routing only
after explicit routing decisions and before the configured default. Launch
failures stay visible and never silently open a different browser. Entries without authoritative scope are skipped: `CrAppModeShortcutURL` and
`start_url` are launch URLs, not manifest scopes, and brouter never fetches a
manifest. The one macOS compatibility rule is a root `CrAppModeShortcutURL`:
its normalized scope is the root of that exact origin because no narrower
manifest scope can contain root; non-root shortcuts still require explicit
scope.

## Existing contracts this plan must preserve

- The explicit route command target remains the highest-priority decision.
  Route-command failures remain terminal; they do not fall through to app
  discovery.
- Existing ordered static rules remain ahead of discovery and retain their
  first-match behavior.
- The exact `@default` route-command response remains a defer response, not a
  browser target. Before implementation, document the expanded pipeline as
  `@default -> static rules -> installed-app discovery -> configured default`;
  do not reinterpret it as a new target or add a second ambiguous fallback
  token. Existing `route.py` examples that return `@default` must continue to
  run unchanged.
- The intentional consequence of the expanded defer pipeline (a previously
  defaulted URL may now select a matching installed app) must be called out in
  the route-command documentation and migration notes. Add compatibility tests
  for explicit command targets, static matches, and `@default`. If strict
  configured-default behavior is required for existing commands, stop before
  implementation and choose a versioned protocol/token change rather than
  silently changing `@default` semantics.
- The configured browser default remains the final no-match fallback. It is not
  a fallback after a selected installed-app launch fails.
- No new TOML keys, profile settings, route-command syntax, default-handler
  changes, URL logging, telemetry, or URL persistence are introduced.

## Domain and routing contract

- Model an `InstalledWebApp` as domain data containing a normalized origin,
  normalized scope, stable app ID, and an opaque platform launcher handle. The
  domain must not know `.app` paths, `.desktop` files, browser flags, or shell
  commands.
- Match parsed URL components, never substrings: scheme, host, and effective
  port define same-origin; scope is a path boundary (`/work` does not match
  `/workspace`) with an explicit trailing-slash policy. Credentials are never
  part of the origin or scope decision. Tests must cover userinfo/lookalike
  inputs such as `good.example@evil.example`, ports, case, query/fragment,
  encoded paths, and a host that merely contains the origin text.
- A candidate is eligible only when origin and scope both match. The longest
  matching scope wins. Equal normalized scopes use a documented stable
  tie-break (stable app ID, then adapter-defined stable launcher identity),
  independent of filesystem enumeration order.
- Return a decision trace that identifies explicit command, static rule,
  discovered app, and configured-default outcomes without exposing credentials
  or writing URLs. The same decision path must be usable by `open` and
  diagnostics.
- If catalog metadata is invalid, unreadable, incomplete, or unsupported, skip
  that entry with a bounded diagnostic and continue. If a selected launcher is
  missing, refresh the catalog once before deciding. A refreshed no-match may
  use the configured default; a launcher that still exists but fails to launch
  is a visible failure with no silent switch to another browser.

## Platform discovery and launch adapters

Both adapters are required for the initial completion; neither platform may be
left as a fixture-only placeholder.

- **macOS:** discover generated Chromium-family/Brave and Chrome `.app`
  metadata in the supported user/system application locations. Inventory real
  generated metadata first, then parse only the stable origin, authoritative
  scope, app ID, known browser bundle identity, and launch information that
  the browser actually emits. A root `CrAppModeShortcutURL` has the narrowly
  safe exact-origin compatibility scope; non-root shortcut URLs without
  explicit scope remain unsupported. Launch through a structured platform adapter using an opaque bundle/app
  handle; the domain must not construct shell commands or infer identity from
  a display name. A moved app must remain identifiable by its stable app ID,
  while an uninstalled app must not remain launchable from stale metadata.
- **Linux:** discover generated Brave and Chrome `.desktop` launchers in the
  supported XDG application locations. Parse freedesktop Exec grammar without
  a shell, require an authoritative scope plus an explicit URL/scope field or
  URL in `Exec`, validate argv safely, and reject shell expressions, field-code
  ambiguity, relative executables, or arbitrary entries. Chromium app-id-only
  launchers are explicitly unsupported because they do not expose scope.
  Support the existing NixOS + Sway/Wayland baseline only; do not imply
  support for other distributions, desktops, portals, Flatpak, or Snap.
  Launch through an opaque adapter handle with structured arguments.
- Capture sanitized checked-in fixtures representing Brave and Chrome on both
  platforms, including cold and already-running launch forms, moved metadata,
  and removed launchers. Fixtures must not contain private paths or personal
  URLs. Record metadata fields that are absent or browser-version-specific as
  explicit limitations rather than guessing.
- Keep browser-specific metadata parsing and process/OS operations in
  `internal/infra`; keep catalog matching and precedence in pure domain code.
  Reuse the existing launch safety boundary: no shell interpolation, no
  router recursion, and no implicit default-browser invocation.

## Catalog lifecycle and performance

- Build a catalog service with a bounded cache that stores app metadata and
  launcher fingerprints only, never URLs. Cache lifetime must cover repeated
  events in the macOS handler and avoid a full metadata scan for every Linux
  click; one-shot CLI behavior must use a bounded directory/metadata
  fingerprint rather than trusting an indefinitely stale snapshot.
- Invalidate on relevant root/entry metadata changes, missing launchers, and
  browser install/uninstall changes. Refresh atomically so a partial scan
  cannot replace a valid catalog. Concurrent refreshes must not race or return
  entries from mixed snapshots.
- Add a benchmark using the current main routing fixture and the same machine
  measurement method. Establish the current median before comparing; the
  cached discovery path must stay within a documented non-material budget
  (target: no more than 10% above baseline, with a 1 ms floor), and cold-scan
  cost must have a bounded, separately reported budget. A benchmark failure is
  visible rather than silently disabling discovery.

## Acceptance criteria

- [ ] No user configuration is required; no new configuration field or
      platform default-handler mutation is added.
- [ ] Precedence is covered end to end: explicit route-command target, static
      user rule, longest-scope installed app, then configured default.
- [ ] `@default` compatibility and migration behavior are documented and
      tested as described above; command failures still stop the open.
- [ ] Same-origin and scope matching has deterministic longest-scope and
      equal-scope tie behavior, with credential/lookalike/port/path boundary
      tests.
- [ ] A shared catalog/launcher contract has contract tests, and real-shaped
      Brave/Chrome `.app` and `.desktop` fixtures exercise both macOS and Linux
      adapters at initial completion. Fixtures prove missing authoritative
      scope is skipped and do not claim universal Chromium metadata support.
- [ ] Cache invalidation handles moved and uninstalled apps safely, avoids
      per-click full scans, and never launches a stale or mixed catalog entry.
- [ ] Launch uses platform adapters with structured arguments. Missing,
      malformed, or failed launchers produce defined, URL-safe visible errors;
      no wrong-browser fallback occurs after a selected app launch failure.
- [ ] The performance benchmark records the current median, cached-path
      budget, cold-scan budget, and result; it demonstrates no material median
      regression.
- [ ] Privacy tests prove that discovery, cache, diagnostics, and existing
      opt-in routing logs do not add full URLs, credentials, or telemetry.
- [ ] Documentation states supported macOS and Linux baselines, generated-app
      limitations (including app-id-only Linux launchers and non-root macOS
      bundles with only `CrAppModeShortcutURL`), the root exact-origin rule,
      refresh behavior, failure/fallback policy,
      and explicit non-support for Windows, app creation/installation, other
      engines, and default-handler changes.

## Verification

Run focused domain, catalog, adapter, cache, and CLI tests first. Run the
shared contract suite against both adapters, then platform fixture integration
checks on macOS and the supported Linux desktop. Run the benchmark on the
same fixture and host used for the current baseline. Use real installed Brave
and Chrome smoke checks for cold and already-running apps where available;
report unavailable combinations as unverified rather than inferring support
from mocks. Run the normal quality gate and the existing separate release
checks without changing their scope.

## Non-goals

Windows support before it is an officially supported platform; creating or
installing web apps; GUI configuration; new user configuration; all browser
engines; arbitrary `.app` or `.desktop` discovery; Flatpak/Snap/portal
integration; default-handler changes; source-application routing; URL history,
telemetry, or privacy-policy changes.
