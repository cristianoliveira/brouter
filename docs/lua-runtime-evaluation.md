# Lua runtime and isolation evaluation (TASK-0028)

Status: **evaluation only**. No dependency was added, `go.mod` is
unchanged, no user scripts were executed, no runtime code was written
into the product, and **no runtime is selected or defaulted** by this
document. All evidence was produced in a throwaway module against
pinned versions; commands and raw outputs are quoted so the comparison
is reproducible.

Evaluation target: the approved contract in
`docs/route-script-contract.md` (TASK-0025) — standard-library
allowlist, no filesystem/network/process/environment access,
deterministic injected UTC, per-invocation timeout, bounded memory,
safe error categories, arm64 + Linux builds, maintenance, licensing,
offline reproducibility.

## Candidates

| # | Candidate | Kind | Pin |
|---|-----------|------|-----|
| 1 | gopher-lua | pure-Go Lua 5.1 VM, in-process | `github.com/yuin/gopher-lua v1.1.1`, MIT |
| 2 | C Lua 5.4 via cgo | upstream interpreter, in-process | `lua-5.4.7` (lua.org sha256 `9fbf5e28…0bf1e30`), MIT |
| 3 | arnodel/golua | pure-Go Lua 5.4 VM, in-process | `github.com/arnodel/golua v0.3.0`, Apache-2.0 |
| 4 | WASM Lua via wazero | WebAssembly sandbox, in-process | wazero (MIT); **Lua image pin unresolved** |
| 5 | subprocess runner | OS process boundary per call | any pinned engine; no in-process dependency |

## Measured evidence

All harnesses ran on Apple Silicon (macOS 26.6, arm64). Cross-platform
evidence is limited to compilation where noted; execution on Linux was
not available and is recorded as **unknown**, not as support.

### 1. gopher-lua v1.1.1 (pure Go)

- Sandbox: `lua.NewState(lua.Options{SkipOpenLibs: true})` +
  `OpenBase` only, with `os`, `io`, `dofile`, `loadfile`, `require`,
  `load` set to nil — probes confirm none are reachable; a host-provided
  `utils` global delivers the injected clock (`utils.epoch() ==
  1700000000` passes). No `package`/`require` machinery exists at all.
- Error mapping: `error("boom")` surfaces to the host with location
  text — maps cleanly to the contract's `script-error`.
- **Timeout: FAILS the contract natively.** The documented kill path,
  `L.Close()` from a timer goroutine, did **not** stop an infinite
  pure-Lua loop within 2 s (test `TestTimeout` timed out). There is no
  instruction-count hook or preemption knob in v1.1.1. Enforcing the
  contract would require abandoning the runaway goroutine (leaks the
  script's memory until it finishes — unbounded, unacceptable), a
  maintained VM fork (patch budget loop), or wrapping the engine in a
  subprocess (candidate 5).
- Builds: `GOOS=linux GOARCH=arm64` and `CGO_ENABLED=0 GOOS=linux
  GOARCH=amd64` both compile (pure Go).
- Maintenance/license: MIT; low release churn but the de-facto standard
  Lua for Go; single-maintainer bus factor.

### 2. C Lua 5.4.7 via cgo

- Sandbox: load only `_G`/`table`/`string` via `luaL_requiref` (os/io
  never opened), then nil-out the same globals — probe passes.
- Injected utils: host C function registered as `utils.epoch()` —
  passes.
- Error mapping: `luaL_dostring` returns the error object with chunk
  location — maps to `script-error`.
- **Timeout: PASSES measurably.** `lua_sethook` with `LUA_MASKCOUNT`
  at a 20,000,000-instruction budget stopped an infinite loop with
  "script-timeout (instruction budget exceeded)"; the whole harness
  (compile + run + teardown) took **0.365 s wall**.
- Memory: enforceable via a custom `lua_Alloc` allocator cap or
  GC-step checks inside the same hook (not yet prototyped — the
  *mechanism* exists in the C API; the cap policy is an open choice).
- Builds: compiles arm64 on macOS (`make generic`). Linux/arm64 build
  and cgo integration into this repo's packaging (which today assumes
  `CGO_ENABLED=0`-friendly pure Go) are **unknown — not exercised**;
  this is the candidate's largest integration cost, not a safety
  question.
- Maintenance/license: MIT, reference implementation, maximally stable.
  Offline reproducibility: vendor the tarball (hash pinned above) via
  nix.

### 3. arnodel/golua v0.3.0 (pure Go, Lua 5.4)

- Apache-2.0, pure Go (cross-compiles in principle like candidate 1).
- **Timeout/sandbox mechanics: UNKNOWN.** API drift between v0.1.0 and
  v0.3.0 (runtime construction, compiler options, module registration)
  prevented a working probe within the evaluation budget; the runtime
  context machinery suggests deadline support but there is **no
  measured evidence**. Per the plan's verification rule this is
  recorded as unknown. Also the least adopted of the Go candidates —
  maintenance-bus-factor risk.

### 4. WASM Lua hosted by wazero

- In principle the strongest isolation: the script runs in a WebAssembly
  sandbox with no host imports unless explicitly exposed; wazero is
  pure Go (MIT, actively maintained) and supports context deadlines and
  memory limits — the contract's timeout/memory map natively.
- **Unresolved pin: no provenance-clean, nix-pinnable Lua-to-WASM
  image was identified within the evaluation budget.** The known
  builds live in third-party registries with supply-chain provenance
  that would need review. Additionally the C ABI marshaling of
  `ctx` (strings, nested table) is real glue, and per-call
  instantiation costs are unmeasured. Recorded as a plausible
  candidate with **unknown** evidence, not a recommendation.

### 5. Subprocess runner (process boundary)

- Any engine (including C Lua 5.4.7 unchanged) runs in a short-lived
  child per routed URL: OS-enforced isolation (no inherited fds beyond
  pipes, wall-clock kill, separate address space), and the Go binary
  stays pure-Go. Marshaling is a small JSON document both ways.
- Measured spawn+exit cost on this host: **9.05 ms/roundtrip** under
  test-load conditions (100 iterations of `/usr/bin/true`); even an
  order of magnitude above the raw fork cost is acceptable at brouter's
  per-URL launch volumes.
- Costs: a second shipped binary (nix can pin it), JSON contract
  maintenance, and per-call latency. Timeout is enforcement-by-kill —
  the strongest guarantee of all candidates.

## Comparison against the contract

| Criterion | 1 gopher-lua | 2 C Lua 5.4 | 3 golua | 4 WASM/wazero | 5 subprocess |
|---|---|---|---|---|---|
| Allowlist stdlib | measured ✓ | measured ✓ | unknown | by construction ✓ | by construction ✓ |
| No I/O/process/env | ✓ (stripped) | ✓ (never loaded) | unknown | ✓ (no imports) | ✓ (OS-enforced) |
| Injected deterministic UTC | measured ✓ | measured ✓ | unverified | design ✓ | host feeds runner ✓ |
| Hard timeout | **✗ native gap** | **measured ✓** | unknown | ✓ (context) | ✓ (kill) |
| Memory limit | ✗ (no knob) | mechanism ✓ (policy open) | unknown | ✓ (limiter) | ✓ (rlimits/kill) |
| Error mapping | measured ✓ | measured ✓ | unknown | design ✓ | via JSON ✓ |
| arm64/Linux | ✓ measured compile | macOS measured; Linux **unknown** | presumed ✓ unverified | ✓ (wazero) | ✓ |
| Maintenance | MIT, low churn, 1 maintainer | MIT, reference, ultra-stable | Apache-2.0, small project | MIT, active | n/a (self-owned) |
| Offline reproducibility | module pin ✓ | tarball pin ✓ | module pin ✓ | **blob provenance unresolved** | nix-pinned runner ✓ |
| Integration cost | low (if timeout solved) | high (cgo: CGO_ENABLED=0 conflicts, cross-builds, seal) | low | medium-high (glue + pin) | medium (second binary + JSON) |

## Live-edit requirement (added after PO confirmation)

Confirmed requirement: TOML edits — and any future Lua script edits —
must apply **without router restart**. The current pipeline already
behaves this way for TOML: `runOpen`/`runExplain`/`runValidate` call
`config.Load` per invocation (cmd/brouter/open.go:29, explain.go:28,
validate.go:20), and the macOS shim spawns a fresh `brouter open` per
URL, so every event resolves a fresh, immutable config snapshot; there
is no watcher and no cache. Verified live: editing the fixture config
between two `explain` calls changes the resolved target (alpha →
beta) with no restart, and a missing config after an edit fails
visibly (exit 1) with **no** last-known-good or wrong-target fallback.

Consequence for every candidate above: any runtime adoption must
preserve per-URL fresh immutable snapshots — the script (like the
config) is re-read per event or held to an equivalent coherence
guarantee; a long-lived engine that caches a parsed script across
events must invalidate it on every edit and treat mid-event changes as
out of scope. Edited-but-broken scripts/configs must fail visibly per
the contract's error categories — no stale-cache serving and no
last-known-good fallback unless a future policy explicitly adds one
(covered by TASK-0029/30/31 snapshot/coherence plans).

## Hard safety requirements identified

1. **A hard per-invocation timeout is non-negotiable** (contract:
   `script-timeout`). Candidate 1 cannot provide it natively; any
   in-process adoption of gopher-lua requires either a maintained
   budget mechanism upstream/fork or wrapping it in candidate 5.
2. **The allowlist must be deny-by-default**: open only
   base/table/string, strip os/io/load/require, inject `utils` as a
   host-provided global. Verified working in candidates 1 and 2.
3. **No capability leakage via error text**: error strings may carry
   script-authored text; diagnostics must keep the URL redaction
   contract (already enforced by the TASK-0023 tests' pattern).

## Unresolved product choices (not made here)

- Provider selection (in-process engine vs subprocess boundary).
- Timeout budget value and its measurement basis (instructions vs wall
  clock).
- Memory accounting policy (allocator cap vs GC-step sampling vs
  process rlimit).
- Config schema and opt-in shape for scripts (TASK-0030 territory).
- Whether Lua is adopted at all — a declarative non-Turing-complete
  alternative remains a legitimate defer/decline option.

## Recommended next options (no default selected)

- **Option A — defer**: keep static rules; revisit if real demand for
  scripting materializes. Zero risk; loses the time-based routing
  capability that motivated TASK-0025.
- **Option B — subprocess boundary** (candidates 2 or 1 inside): strongest
  isolation and hard kill-based timeout; costs a pinned second binary
  and JSON bridge (TASK-0029's bridge seams would define it).
- **Option C — in-process C Lua 5.4 via cgo**: everything measured
  green, but takes on cgo build/packaging complexity across the
  existing CGO-free matrix.
- **Option D — keep evaluating**: obtain Linux/arm64 cgo evidence and a
  provenance-clean WASM Lua pin before deciding (both currently
  unknown).
- **Decline**: none of the candidates blocks the product; the static
  router fully covers current behavior.

## Evidence appendix

Pinned artifacts and commands (temporary evaluation module in
`.tmp/eval`, not committed):

- `go get github.com/yuin/gopher-lua@v1.1.1` — MIT (LICENSE quoted in
  module cache). Tests `TestSandbox`, `TestTimeout`, `TestErrorMapping`
  in `gopher_test.go`.
- `curl https://www.lua.org/ftp/lua-5.4.7.tar.gz` (sha256
  `9fbf5e28…`), `make generic`, harness `lua54_harness.c` with
  `-arch arm64`; raw output quoted above under "Measured evidence".
- `go get github.com/arnodel/golua@v0.3.0` — Apache-2.0; probe
  abandoned (API drift) — unknown recorded.
- `GOOS=linux GOARCH=arm64 go build ./...` and
  `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...` — pure-Go
  cross-compile evidence.
- Spawn latency harness `spawn_test.go` — 9.05 ms/roundtrip.
