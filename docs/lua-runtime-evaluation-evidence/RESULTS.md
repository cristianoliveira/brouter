# Raw evaluation outputs (collected verbatim, Apple Silicon macOS 26.6)

All harness sources live next to this file. Every command below was
run inside the pinned throwaway module (`.tmp/eval`, uncommitted).

## gopher-lua v1.1.1 — `go test -v` (gopher_test.go)

```
--- PASS: TestSandbox (0.00s)
--- FAIL: TestTimeout (2.20s)
--- PASS: TestErrorMapping (0.00s)
```

TestTimeout failed its 2-second grace after `L.Close()`: the infinite
pure-Lua loop was still running. Verdict: no native hard-timeout
mechanism in v1.1.1.

Sandbox probes (TestSandbox body): `os`, `io`, `require`, `load`,
`dofile` all resolve to nil; `utils.epoch() == 1700000000` passes
(injected host clock).

## C Lua 5.4.7 — `./lua54_harness` (lua54_harness.c)

Tarball: `lua-5.4.7.tar.gz`, sha256
`9fbf5e28ef86c69858f6d3d34eccc32e911c1a28b4120ff3e84aaa70cfbf1e30`.
Build: `clang -arch arm64 -O2 -I lua-5.4.7/src lua54_harness.c
lua-5.4.7/src/liblua.a -lm -o lua54_harness`.

```
sandbox-probe: ok
injected-utils: ok
error-mapping: [string "error('boom')"]:1: boom
timeout: script-timeout (instruction budget exceeded)

real	0m0.365s
user	0m0.049s
sys	0m0.005s
```

## Cross-compile (pure-Go candidates)

```
GOOS=linux GOARCH=arm64 go build ./...   → linux/arm64 build OK
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./... → OK
```

Build evidence only; Linux execution was not available.

## Subprocess baseline — `go test -run TestSubprocessSpawnLatency -v`

```
spawn_test.go:19: subprocess spawn+exit: 9.05 ms/roundtrip (100 iterations)
```

This is a raw `/usr/bin/true` fork/exec baseline — NOT a Lua runner
measurement. FD inheritance and rlimit behavior of a real runner are
unprobed (see the evaluation's unknowns).
