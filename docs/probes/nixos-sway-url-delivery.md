# Probe: NixOS + Sway (Wayland) URL delivery — TASK-0007

Status: **prepared, not executed.** This spike requires a real NixOS
machine with a graphical Sway session. It was authored on a macOS host and
MUST NOT be counted as desktop-support evidence until the checklist below
is filled with observed output from the target machine.

## Prerequisites (exact access needed)

1. A NixOS machine with an active Sway (Wayland) graphical session —
   login on the machine, not SSH alone. The tool gates on `ID=nixos` in
   `os-release`, `WAYLAND_DISPLAY`, and `SWAYSOCK` (or a successful
   `swaymsg` check); X11 sessions and other compositors are refused.
2. Brave or Chrome installed via Nix (desktop files present), to test
   browser forwarding cold and warm.
3. A terminal inside that Sway session, and one other graphical app able
   to open a link (for the external-app click).

## Tool

`scripts/probes/nixos-url-spike.sh` — POSIX shell, self-contained.
Strict session gate: requires `WAYLAND_DISPLAY` plus an unconditional
`swaymsg -t get_version` validation — X11, other compositors, and stale
`SWAYSOCK` values are refused. Opener failures propagate through exit
status. An existing handler with the spike's ID blocks installation (a
copy is saved, the file is never overwritten); in `--system` mode an
install-time safety trap auto-restores a partial failure, and restore
verifies the persisted before/after handler state (exits nonzero on
mismatch) and reinstalls any pre-existing desktop file. Receiver logs
never contain raw argv (argument count only); `selftest` verifies the
receipt-redaction guarantees on any host.

## Two modes

| Mode | What it touches | Use |
|---|---|---|
| default (isolated) | Registrations live in `brouter-spike/xdg-*` via redirected `XDG_DATA_HOME`/`XDG_CONFIG_HOME`. **System defaults are never modified** — nothing to restore. | OS-delivery and handler-receipt evidence |
| `--system` (opt-in) | Mutates the real `mimeapps.list` to prove the actual default-handler path. Strict backup (including absence marker and handler-state snapshot), explicit verified restore, session-less recovery. | Only when isolated evidence is insufficient |

## Procedure (isolated mode — no system changes)

```sh
scripts/probes/nixos-url-spike.sh install                                # isolated registrations
scripts/probes/nixos-url-spike.sh report                                 # session, portal, Exec resolution
scripts/probes/nixos-url-spike.sh probe https://spike.example/probe-1    # OS opener hand-off
scripts/probes/nixos-url-spike.sh probe https://spike.example/probe-2    # sequential delivery
cat brouter-spike/received.log                                           # the receipt
```

Browser forwarding (isolated mode registers the real browser desktop id
inside the spike environment, so the browser binary launches with the
URL while system defaults stay untouched):

```sh
scripts/probes/nixos-url-spike.sh browser brave-browser https://spike.example/cold   # Brave CLOSED
scripts/probes/nixos-url-spike.sh browser brave-browser https://spike.example/warm   # Brave RUNNING
```

## System mode (explicit opt-in)

```sh
scripts/probes/nixos-url-spike.sh --system backup
scripts/probes/nixos-url-spike.sh --system install
scripts/probes/nixos-url-spike.sh --system probe https://spike.example/real-default
scripts/probes/nixos-url-spike.sh --system restore
```

- `backup` snapshots `mimeapps.list` (or its absence), the current http
  and https handler ids, and refuses nothing — it is mandatory before
  system mutation.
- `restore` reinstalls the snapshot, removes spike-generated files,
  verifies the persisted before/after handler state, and exits nonzero
  on any mismatch.
- If the Sway session dies mid-spike: `restore` runs WITHOUT a graphical
  session. For fully manual repair, compare
  `~/.config/mimeapps.list` against
  `brouter-spike/mimeapps.list.backup` (restore the copy, or delete the
  generated file when `mimeapps.list.absent` exists), then remove
  `~/.local/share/applications/brouter-spike-handler.desktop`.

## Failure channel (deliberately broken handler)

Point the installed handler's `Exec` at a nonexistent binary, run
`probe https://spike.example/broken`, and record the opener exit code,
any on-screen error, and `journalctl --user -u xdg-desktop-portal*`.
Restore afterwards.

External-app click (user-assisted): with the handler installed, click a
link in another graphical app and check `received.log`.

## URL secret handling

`received.log` and probe output never log full URLs: scheme, host, and
path are kept for correlation; query strings, fragments, and userinfo
are redacted (byte count + checksum preserved). URLs carrying
`user:pass@` credentials are rejected outright.

## Evidence checklist (fill from observed output)

| TASK-0007 acceptance item | Evidence location | Observed |
|---|---|---|
| Temporary handler registered for http/https | `install` output + `.desktop` file | ☐ |
| OS-delivered URL receipt with full argv | `brouter-spike/received.log` | ☐ |
| Graphical-session opener/portal path recorded | `report.txt` portals + received.log env line | ☐ |
| Browser resolution: Nix store Exec lines vs profile paths | `report.txt` Exec resolution section | ☐ |
| Forwarding to Brave/Chrome, closed and running | browser commands, observed browser state | ☐ |
| Sequential delivery (two URLs) | probe 1 + 2 entries in received.log | ☐ |
| Failure channel when handler is broken | broken-handler output + journal excerpt | ☐ |
| Previous handler state preserved and verified restored | `--system` backup/restore + verified comparison | ☐ |

## Rules

- No completion claim, no platform support statement, and no default
  changes may be recorded without the checklist filled from a real run.
- The spike workspace (`brouter-spike/`) is host-local evidence; paste
  relevant excerpts into the TASK-0007 report, do not commit logs.
- In `--system` mode, restore is mandatory before declaring the spike
  session finished, and its verification must have printed success.
