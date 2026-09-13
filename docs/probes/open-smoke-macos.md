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

Same-target protocol (brave cold → warm), unlike the macOS runs which
covered different targets (warm Brave, cold Chrome). A local receiver
provides attributable receipts; the fragment never reaches the server
(it is client-side), so also note the address bar. `.example` pages do
not serve — the receipt/address bar is the success signal, not page
load. No checkout cycle is needed if the tree already sits at the
reviewed c5707e4 (later commits are docs-only).

One-time setup — creates the config before any use, then starts the
receiver (it stays up for both runs):

```sh
printf 'default = "brave"\n\n[browsers.brave]\nbrowser = "brave"\n' \
  > /tmp/brouter-smoke.toml
cat > /tmp/brouter-smoke-recv.py <<'EOF'
import http.server, socketserver
log = open("/tmp/brouter-smoke-receipts.log", "a", buffering=1)
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        log.write(f"{self.command} {self.path} {self.headers.get('User-Agent','')}\n")
        self.send_response(200); self.end_headers(); self.wfile.write(b"ok")
    def log_message(self, *a): pass
socketserver.TCPServer.allow_reuse_address = True
socketserver.TCPServer(("127.0.0.1", 18081), H).serve_forever()
EOF
python3 /tmp/brouter-smoke-recv.py & recv_pid=$!
```

T1 — cold: fully quit Brave first (`pgrep -a brave` prints nothing).
The command is wrapped in `timeout 20s` because a cold launch may block
(platform-dependent). While it runs, watch Brave's address bar — that
observation is the user-attested delivery evidence; the fragment
(`#cold`) only appears there, never in the receipt:

```sh
timeout 20s nix develop -c go run ./cmd/brouter open \
  -config /tmp/brouter-smoke.toml \
  'http://127.0.0.1:18081/direct-cold?case=1#cold'
echo "cold rc=$? (124 = timed out while browser ran: blocking, not failure)"
cat /tmp/brouter-smoke-receipts.log
```

T2 — warm: Brave was left open by T1 (no quit in between):

```sh
timeout 20s nix develop -c go run ./cmd/brouter open \
  -config /tmp/brouter-smoke.toml \
  'http://127.0.0.1:18081/direct-warm?case=2#warm'
echo "warm rc=$?"
cat /tmp/brouter-smoke-receipts.log
kill "$recv_pid" 2>/dev/null   # stop the receiver
```

Record for each run: rc (including a 124 timeout on T1 — blocking is a
platform finding, not a launch failure), the receipt line showing the
query (`case=1` / `case=2`) delivered by Brave, and the address-bar
fragment (`#cold` / `#warm`) as user observation. Expected: direct
launch of the stable Brave executable (`/run/current-system/sw/bin/brave`
per TASK-0007); warm run reusing the running instance. Do not reuse
TASK-0007 probe receipts — this is direct-launch evidence.
