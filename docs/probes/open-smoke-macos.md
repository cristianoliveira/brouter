# Smoke: `brouter open` on macOS — TASK-0011

Status: **executed on macOS host; Linux/NixOS run pending (commands below).**

Run date: 2026-09-13. Environment: macOS 26.6.2 (Apple Silicon), Brave
151.1.93.138, Chrome 152.0.7977.83. Binary built from branch
`task-0011-browser-launch` at c5707e4 (`go build ./cmd/brouter`).

## Method

A local HTTP receiver on `127.0.0.1:18081` logged each request line, so
delivery is attributable to the browser (User-Agent) and the URL
contract is checked byte-for-byte on the wire. Each run states the
pre-existing browser state before `brouter open`.

```sh
go build -o /tmp/brouter-smoke/brouter ./cmd/brouter
python3 receiver.py /tmp/brouter-smoke/receipts.log &   # logs "GET <path> <UA>"
```

## Warm Brave (instance already running)

- Pre-state: Brave running (pid 1031, user session).
- Config: known browser `brave`. Resolved:
  `/Applications/Brave Browser.app/Contents/MacOS/Brave Browser`.
- Command: `brouter open -config warm.toml "http://127.0.0.1:18081/warm?kept=1&tag=smoke-macos"`
- Result: exit 0 in ~1s; pid unchanged (running instance reused).
- Receipt: `GET /warm?kept=1&tag=smoke-macos` from Brave (UA
  `...Chrome/151.0.0.0...`) — query delivered intact, one argument.

## Cold Chrome (default-profile instance not running)

- Pre-state: no Chrome instance with the default user-data-dir (only
  separate automation instances with their own `--user-data-dir`, which
  Chrome's process singleton does not join).
- Config: known browser `chrome`. Resolved:
  `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`.
- Command: `brouter open -config cold.toml "http://127.0.0.1:18081/cold?kept=2&tag=smoke-macos"`
- Result: receipt `GET /cold?kept=2&tag=smoke-macos` from Chrome/152 —
  URL delivered intact to a freshly launched instance.
- Blocking finding: on macOS the exec'd binary is the long-lived app
  process, so `brouter open` did not return while Chrome ran (observed
  >20s; stopped via watchdog). Warm launches return immediately. This
  is a documented platform difference: on macOS cold starts, process
  exit is not a completion signal — consistent with the TASK-0007
  finding that exit status must not be treated as page-load proof.

## URL contract observed on the wire

Both receipts show the full query (`kept=N&tag=smoke-macos`) with no
re-encoding, splitting, or loss; no shell was involved (direct argv, no
default-handler delegation — brouter exec'd the resolved browser
binaries itself).

## Linux/NixOS run (pending, user-attested)

Same-target protocol (brave cold -> warm), unlike the macOS runs which
covered different targets (warm Brave, cold Chrome). Fixed benign URLs;
`spike.example` intentionally does not serve pages - success is the
exact URL (query and fragment) appearing in the browser address bar,
NOT page load. No timeout and no receiver: if the cold command stays in
the foreground while Brave runs, that is recorded as UX evidence, not
treated as failure. Code under test is the reviewed c5707e4 (no new
checkout needed; later commits are docs-only).

Setup - creates the config before any use:

```sh
printf 'default = "brave"\n\n[browsers.brave]\nbrowser = "brave"\n' \
  > /tmp/brouter-smoke.toml
```

T1 - cold: fully quit Brave first (`pgrep -a brave` prints nothing),
then run in a first terminal. The command may stay in the foreground
while Brave runs: leave it running and record that behavior - do not
kill it, and do not quit Brave.

```sh
nix develop -c go run ./cmd/brouter open -config /tmp/brouter-smoke.toml \
  'https://spike.example/direct-cold?case=1#cold'
```

T2 - warm: in a second terminal, with the same explicit config path and
Brave left open from T1:

```sh
nix develop -c go run ./cmd/brouter open -config /tmp/brouter-smoke.toml \
  'https://spike.example/direct-warm?case=2#warm'
```

Record for each run: whether the command returned (and its rc) or is
still foreground-blocking (T1 blocking is UX evidence for cold starts),
and the address-bar observation (`direct-cold?case=1#cold` /
`direct-warm?case=2#warm` with query and fragment intact). Expected:
direct launch of the stable Brave executable
(`/run/current-system/sw/bin/brave` per TASK-0007); warm run reusing
the running instance and returning promptly. Afterwards T1 may be
stopped with Ctrl-C; Brave can then be quit normally. Do not reuse
TASK-0007 probe receipts - this is direct-launch evidence.
