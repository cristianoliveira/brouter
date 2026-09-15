# Raw evaluation outputs (collected verbatim, Apple Silicon macOS 26.6)

All harness sources live next to this file. They are reference inputs
only and are excluded from the repository toolchain. From the repository
root, reproduce the Go probes in a fresh throwaway module with:

```
rm -rf .tmp/eval && mkdir -p .tmp/eval
cp docs/lua-runtime-evaluation-evidence/gopher_test.go .tmp/eval/
cp docs/lua-runtime-evaluation-evidence/spawn_test.go .tmp/eval/
cd .tmp/eval
go mod init example.com/brouter-lua-eval
go get github.com/yuin/gopher-lua@v1.1.1
go test -tags ignore -run 'Test(Sandbox|Timeout|ErrorMapping)$' -v
go test -tags ignore -run TestSubprocessSpawnLatency -v
```

The `ignore` tag includes the reference files without changing them. The
recorded outputs were collected in that throwaway module; its setup is
not committed.

## gopher-lua v1.1.1 — `go test -tags ignore -run 'Test(Sandbox|Timeout|ErrorMapping)$' -v` (gopher_test.go)

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

## C Lua 5.4.7 — `./lua54_harness` (lua54_harness.c.txt)

Tarball: `lua-5.4.7.tar.gz`, sha256
`9fbf5e28ef86c69858f6d3d34eccc32e911c1a28b4120ff3e84aaa70cfbf1e30`.
The committed `.c.txt` reference is copied to a `.c` filename before
compilation because clang otherwise treats `.txt` as linker input.
Build: `cp lua54_harness.c.txt .tmp/eval/lua54_harness.c && clang
-arch arm64 -O2 -I lua-5.4.7/src .tmp/eval/lua54_harness.c
lua-5.4.7/src/liblua.a -lm -o .tmp/eval/lua54_harness`.

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

## Subprocess baseline — `go test -tags ignore -run TestSubprocessSpawnLatency -v`

```
spawn_test.go:19: subprocess spawn+exit: 9.05 ms/roundtrip (100 iterations)
```

This is a raw `/usr/bin/true` fork/exec baseline — NOT a Lua runner
measurement. FD inheritance and rlimit behavior of a real runner are
unprobed (see the evaluation's unknowns). The command is reproducible after the throwaway setup above; its
setup was not run by repository CI and the result remains only a raw
baseline, not runner evidence.
