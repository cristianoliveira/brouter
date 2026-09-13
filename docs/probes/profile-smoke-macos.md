# Smoke: profiled `brouter open` on macOS — TASK-0012

Status: **executed on macOS host; Linux/NixOS run pending (template below).**

Run date: 2026-09-13. Environment: macOS 26.6.2, Brave 151.1.93.138
(warm), Chrome 152.0.7977.83 (cold). Binary built from branch
`task-0012-profiles` (profile support on top of merged TASK-0011).

## Method

Local HTTP receiver on `127.0.0.1:18081` (kept from the TASK-0011
smoke) logged each request line. Profile selection is attributed three
ways: the brouter resolution detail, the live browser process argv
(cold only), and the browser's own `Local State`
(`profile.last_active_profiles`) after launch.

## Warm Brave, profile `Default`

- Pre-state: Brave running (user session, pid unchanged across run).
- Config: known `brave`, `profile = "Default"`.
- Result: exit 0 in ~1s. Detail: `profile "Default" verified`.
- Receipt: `GET /warm-profile?case=12` from Brave — URL delivered
  intact.
- Profile attribution: Brave `Local State` after the run reports
  `last_active_profiles: ['Default']` — the browser itself confirms the
  configured profile was the active one.

## Cold Chrome, profile `Profile 1`

- Pre-state: no Chrome instance with the default user-data-dir (the
  visible Chrome processes are automation instances with their own
  `--user-data-dir`, which Chrome's process singleton does not join).
- Config: known `chrome`, `profile = "Profile 1"` (an existing
  identifier in the user's default Chrome user-data dir).
- Result: fresh Chrome instance launched; live process argv captured
  `--profile-directory=Profile 1` (direct argv proof); receipt
  `GET /cold-profile?case=12` from Chrome/152 — URL delivered intact.
- Profile attribution: Chrome `Local State` after the run reports
  `last_active_profiles: ['Profile 1']`.
- Blocking finding (same as TASK-0011): on macOS the exec'd binary is
  the long-lived app process, so `brouter open` stays in the foreground
  while a cold browser runs. Recorded as UX evidence; the watchdog
  stopped brouter only (Chrome left open).

## Failure path (unit-verified, deterministic)

A configured profile whose directory does not exist fails before any
launch: `profile directory <expected path> does not exist; brouter
never creates profiles — create or verify it in the browser first`.
Covered by `TestOpenFailsExplicitlyWhenProfileDirectoryIsMissing` (CLI)
and `TestResolveFailsExplicitlyWhenProfileDirectoryIsMissing` /
`TestResolveFailsExplicitlyWhenBrowserUserDataDirIsMissing` (resolver,
including the browser-never-ran case).

## Identifier semantics (documented contract)

`--profile-directory` takes the on-disk profile identifier (for example
`Default`, `Profile 1`), not the user-facing display name. Identifiers
are machine-local: they live inside the browser's user-data dir
(macOS: `~/Library/Application Support/...`; Linux: `~/.config/...`)
and are not portable across machines. Users find identifiers by listing
that directory or the browser's profile settings. brouter verifies the
directory exists and refuses to launch otherwise — it never creates,
migrates, or renames profiles.

## Linux/NixOS run (pending, user-attested)

Same three-way attribution on the NixOS+Sway target; user-data root is
`~/.config/BraveSoftware/Brave-Browser`:

```sh
go build -o /tmp/b12 ./cmd/brouter
ls ~/.config/BraveSoftware/Brave-Browser     # pick an existing identifier
printf 'default = "s"\n\n[browsers.s]\nbrowser = "brave"\nprofile = "<ID>"\n' > /tmp/b12.toml
brouter open --config /tmp/b12.toml 'http://127.0.0.1:18081/profile?case=12'   # with receiver from the TASK-0011 template
pgrep -a brave | grep profile-directory       # argv proof while it runs
cat ~/.config/BraveSoftware/Brave-Browser/Local State | grep -o '"last_active_profiles":[^}]*}'
```

Record: pre-state, rc/blocking, receipt line, argv line, Local State
profiles. Repeat after quitting Brave for the cold case.
