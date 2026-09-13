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
covered different targets (warm Brave, cold Chrome). `spike.example`
intentionally does not serve pages: success is the URL reaching the
selected browser (visible in the address bar), not page load. If the
cold command stays in the foreground while Brave runs, record that
foreground blocking as UX evidence - it is not a failure. Code under
test is the reviewed c5707e4 (no new checkout needed; later commits are
docs-only).

Setup and cold run (T1) - build once via the pinned toolchain, create
the config, fully quit Brave (`pgrep -a brave` prints nothing), then:

```sh
nix develop -c go build -o /tmp/brouter-smoke ./cmd/brouter
cat >/tmp/brouter-smoke.toml <<'EOF'
default = "brave"

[browsers.brave]
browser = "brave"
EOF
nix develop -c /tmp/brouter-smoke open -config /tmp/brouter-smoke.toml \
  'https://spike.example/direct-cold'
```

Record for T1: whether the command blocked in the foreground or
returned (and its rc), and whether the URL appeared in Brave's address
bar. Leave Brave open - do not quit it, and do not kill the T1 command
until observations are noted (Ctrl-C afterwards).

Warm run (T2) - in a second terminal, with the same literal config
path, while Brave remains open:

```sh
nix develop -c /tmp/brouter-smoke open -config /tmp/brouter-smoke.toml \
  'https://spike.example/direct-warm'
```

Record for T2: rc (expected: prompt exit 0 via running-instance reuse)
and whether the URL appeared. Do not reuse TASK-0007 probe receipts -
this is direct-launch evidence.
