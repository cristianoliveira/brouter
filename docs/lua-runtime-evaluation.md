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

This is a **feasibility recommendation, not a security or runtime
acceptance** decision. Claim taxonomy used throughout: **measured** =
run in this evaluation; **proposed** = mechanism exists in the pinned
artifact but was not exercised here; **unknown** = no evidence
obtained. Untested limits and platforms are always marked, never
assumed.

## Candidates

| # | Candidate | Kind | Pin |
|---|-----------|------|-----|
| 1 | gopher-lua | pure-Go Lua 5.1 VM, in-process | `github.com/yuin/gopher-lua v1.1.1`, MIT |
| 2 | C Lua 5.4 via cgo | upstream interpreter, in-process | `lua-5.4.7` (lua.org sha256 `9fbf5e28…0bf1e30`), MIT |
| 3 | arnodel/golua | pure-Go Lua 5.4 VM, in-process | `github.com/arnodel/golua v0.3.0`, Apache-2.0 |
| 4 | WASM Lua via wazero | WebAssembly sandbox, in-process | wazero v1.12.0 (MIT); **Lua image pin unresolved** |
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
  GC-step checks inside the same hook — **mechanism unprobed** in this
  evaluation (no cap was built or measured); the cap policy is an open
  choice.
- Error redaction: the harness surfaces raw error strings; whether a
  production binding must sanitize script-authored text before it
  reaches diagnostics was **not probed** — recorded as unknown.
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
  pure Go (MIT, actively maintained; probe pinned against v1.12.0,
  the latest release tag at evaluation time) and supports context
  deadlines and memory limits — the contract's timeout/memory map
  natively. wazero itself was NOT exercised end-to-end here because
  the Lua image pin is the missing piece.
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
  test-load conditions (100 iterations of `/usr/bin/true`). This is a
  **raw fork/exec baseline only — not a Lua runner measurement**; a
  real runner adds engine init, JSON marshal/unmarshal, and teardown.
- FD inheritance and rlimit behavior of an actual runner are
  **unprobed** (close-on-exec policy, limit availability on Linux vs
  macOS) — unknown.
- arm64/Linux: inferred from Go's cross-platform process spawning plus
  `GOOS=linux` build evidence of the harness (build-only, no Linux
  execution) — recorded as build-evidence, not run-evidence.
- Costs: a second shipped binary (nix can pin it), JSON contract
  maintenance, and per-call latency. Timeout is enforcement-by-kill —
  the strongest guarantee of all candidates.

## Comparison against the contract

| Criterion | 1 gopher-lua | 2 C Lua 5.4 | 3 golua | 4 WASM/wazero | 5 subprocess |
|---|---|---|---|---|---|
| Allowlist stdlib | measured ✓ | measured ✓ | unknown | proposed (unexercised) | proposed (unexercised) |
| No I/O/process/env | ✓ (stripped) | ✓ (never loaded) | unknown | proposed (no imports; unexercised) | unknown (child capability probe unrun) |
| Injected deterministic UTC | measured ✓ | measured ✓ | unknown | proposed (unexercised) | proposed (host feeds runner; unexercised) |
| Hard timeout | **✗ measured gap** | **measured ✓** | unknown | proposed (context; unexercised) | proposed (kill; unexercised) |
| Memory limit | ✗ (no knob) | proposed (allocator cap/hook; unprobed) | unknown | proposed (limiter; unexercised) | proposed (rlimits/kill; unprobed) |
| Error mapping | measured ✓ | measured ✓ | unknown | proposed (unexercised) | unknown (unprobed) |
| arm64/Linux | ✓ measured compile | macOS measured; Linux **unknown** | build-only evidence; run **unknown** | unknown (Lua image unresolved; wazero not exercised) | build-only evidence; run **unknown** |
| Maintenance | MIT, low churn, 1 maintainer | MIT, reference, ultra-stable | Apache-2.0, small project | MIT, active | n/a (self-owned) |
| Offline reproducibility | proposed (module pin; offline rerun unverified) | proposed (tarball hash; offline rerun unverified) | proposed (module pin; offline rerun unverified) | **blob provenance unresolved** | unknown (runner not pinned/built) |
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
(covered by TASK-0029/30/31 snapshot/coherence plans). The snapshot
read itself must be robust to real editor behavior — atomic-rename
writes, symlinked config paths, and files rewritten mid-read — and any
coherence scheme must define which side of an unstable read wins;
those mechanics are TASK-0029/0030 acceptance material, listed here so
no candidate's design assumes a stable-file world.

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

## Threat-model checklist

Status key per candidate: **measured** (exercised here), **proposed**
(mechanism exists in the pinned artifact, unexercised), **unknown**
(no evidence), **n/a** (not applicable to the isolation shape).

| Threat | Requirement | 1 gopher | 2 C Lua | 3 golua | 4 WASM | 5 subprocess |
|---|---|---|---|---|---|---|
| Script reads/writes files, spawns processes, reads env | allowlist denies os/io/load/require; no host capability surface | measured ✓ | measured ✓ | unknown | proposed (unexercised) | unknown (child capability probe unrun) |
| Runaway script starves CPU | hard per-invocation timeout | **measured ✗** | measured ✓ | unknown | proposed (context) | proposed (kill) |
| Script exhausts memory (tables/strings) | bounded allocation | measured ✗ (no knob) | proposed (allocator cap; unprobed) | unknown | proposed (limiter) | proposed (rlimits; unprobed) |
| Secrets leak via error strings into diagnostics | redaction sweep before any sink | unprobed | unprobed | unprobed | unprobed | unprobed |
| Stale or half-written config/script is served after an edit | fresh immutable per-URL snapshot; unstable reads rejected | n/a (host obligation, engine-independent) — host-side, all candidates inherit it | | | | |
| Malicious/typo'd target name silently routes | unknown-target fails closed, name unlogged | contract-level ✓ (all candidates inherit) | | | | |
| Supply chain substitutes the engine or image | pinned artifact + hash, offline build | module pin (offline unverified) | tarball sha256 (offline unverified) | module pin (offline unverified) | **unresolved image pin ✗** | unknown (runner not pinned/built) |
| Host escape (interpreter bug) | memory-safe VM or process boundary | pure Go (VM bugs possible, no memory unsafety) | C (memory-unsafe surface) | pure Go | Wasm sandbox | process boundary |
| Engine/network access left on by mistake | no network API reachable | ✓ (stdlib stripped) | ✓ (socket lib never loaded) | unknown | proposed (no host imports; unexercised) | unknown (FD inheritance unprobed) |

The checklist is acceptance material for whichever provider a later
task selects: every unmeasured cell must become measured (or the
threat must be redesign-excluded) before any runtime ships.

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
- `curl https://www.lua.org/ftp/lua-5.4.7.tar.gz` — full sha256
  `9fbf5e28ef86c69858f6d3d34eccc32e911c1a28b4120ff3e84aaa70cfbf1e30`;
  `make generic`, harness `lua54_harness.c` with `-arch arm64`; raw
  output quoted in
  `docs/lua-runtime-evaluation-evidence/RESULTS.md`.
- Harness sources committed for independent re-running:
  `docs/lua-runtime-evaluation-evidence/gopher_test.go`,
  `lua54_harness.c.txt`, `spawn_test.go`, `RESULTS.md` (verbatim raw
  outputs). The Go sources require the `ignore` build tag in the
  throwaway module; the C reference must be copied to a `.c` filename
  before clang compilation. Setup is documented but not run by CI.
- `go get github.com/arnodel/golua@v0.3.0` — Apache-2.0; probe
  abandoned (API drift) — unknown recorded.
- `GOOS=linux GOARCH=arm64 go build ./...` and
  `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...` — pure-Go
  cross-compile evidence.
- Spawn latency harness `spawn_test.go` — 9.05 ms/roundtrip; raw
  baseline only, with reproduction setup described in RESULTS.md.
