# Probe: NixOS + Sway (Wayland) URL delivery — TASK-0007

Status: **prepared, not executed.** This spike requires a real NixOS
machine with a graphical Sway session. It was authored on a macOS host and
MUST NOT be counted as desktop-support evidence until the checklist below
is filled with observed output from the target machine.

## Prerequisites (exact access needed)

1. A NixOS machine with an active Sway (Wayland) graphical session —
   login on the machine, not SSH alone: the probe needs
   `WAYLAND_DISPLAY`/session environment and a real portal stack.
2. Brave or Chrome installed via Nix (desktop files present), to test
   browser forwarding cold and warm.
3. A terminal inside that Sway session to run the probe script, and one
   other graphical app able to open a link (for the external-app click).

## Tool

`scripts/probes/nixos-url-spike.sh` — POSIX shell, self-contained.
Everything it changes is recorded in a backup and restorable with one
command. It refuses to run outside Linux or outside a graphical session.

## Procedure

Run inside the Sway session, from the repository checkout:

```sh
scripts/probes/nixos-url-spike.sh backup     # 1. record current handlers (mimeapps.list + queries)
scripts/probes/nixos-url-spike.sh install    # 2. register temporary logging handler
scripts/probes/nixos-url-spike.sh report     # 3. session, portal, and browser resolution facts
scripts/probes/nixos-url-spike.sh probe https://spike.example/probe-1     # 4. OS opener hand-off
scripts/probes/nixos-url-spike.sh probe https://spike.example/probe-2     # 5. sequential delivery
cat brouter-spike/received.log               # 6. the receipt: argv, env, invoker
```

Browser forwarding (restore the real browser default first via
`restore`, then explicitly set the browser desktop id):

```sh
scripts/probes/nixos-url-spike.sh restore
scripts/probes/nixos-url-spike.sh backup
scripts/probes/nixos-url-spike.sh browser brave-browser https://spike.example/cold   # Brave CLOSED
scripts/probes/nixos-url-spike.sh browser brave-browser https://spike.example/warm   # Brave RUNNING
```

Failure channel (deliberately broken handler):

```sh
# point the installed handler's Exec at a nonexistent binary, then:
scripts/probes/nixos-url-spike.sh probe https://spike.example/broken
# record: xdg-open exit code, any on-screen error, portal/journal messages:
journalctl --user -u xdg-desktop-portal* --no-pager | tail -20
scripts/probes/nixos-url-spike.sh restore
```

External-app click (user-assisted): with the logging handler installed,
click a link inside another graphical app (e.g. a README link in a GTK
editor) and record whether `received.log` gains an entry.

## Evidence checklist (fill from observed output)

| TASK-0007 acceptance item | Evidence location | Observed |
|---|---|---|
| Temporary handler registered for http/https | `install` output + `.desktop` file | ☐ |
| OS-delivered URL receipt with full argv | `brouter-spike/received.log` | ☐ |
| Graphical-session opener/portal path recorded | `report.txt` portals section + received.log env line | ☐ |
| Browser resolution from session (Nix store vs profile paths) | `report.txt` executable resolution | ☐ |
| Forwarding to Brave/Chrome, closed and running | browser commands above, observed browser state | ☐ |
| Sequential delivery (two URLs) | probe 1 + 2 entries in received.log | ☐ |
| Failure channel when handler is broken | broken-handler procedure output + journal excerpt | ☐ |
| Previous handler preserved and restored | backup/restore outputs + `xdg-mime query` checks | ☐ |

## Rules

- No completion claim, no platform support statement, and no default
  changes may be recorded without the checklist filled from a real run.
- The spike workspace (`brouter-spike/`) is host-local evidence; paste
  relevant excerpts into the TASK-0007 report, do not commit logs.
- Restore is mandatory before declaring the spike session finished.
