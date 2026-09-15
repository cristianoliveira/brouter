# Raw evaluation outputs (collected verbatim, Apple Silicon macOS 26.6)

All harness sources live next to this file. They are reference inputs
only and are excluded from the repository toolchain. From the repository
root, reproduce the Go probes in a fresh throwaway module with:

```
rm -rf .tmp/eval && mkdir -p .tmp/eval
sed '1{/^\/\/go:build ignore$/d;}' docs/lua-runtime-evaluation-evidence/gopher_test.go > .tmp/eval/gopher_test.go
sed '1{/^\/\/go:build ignore$/d;}' docs/lua-runtime-evaluation-evidence/spawn_test.go > .tmp/eval/spawn_test.go
cd .tmp/eval
go mod init example.com/brouter-lua-eval
go get github.com/yuin/gopher-lua@v1.1.1
go test -run 'Test(Sandbox|ErrorMapping)$' -v
timeout_rc=0; go test -run TestTimeout -v || timeout_rc=$?
test "$timeout_rc" -eq 1
go test -run TestSubprocessSpawnLatency -v
```

Equivalent one-line variants (identical effect): the Go probes run
with `go test -v -tags ignore` in a module with the pinned dependency
and the sources left unstripped; the C probe compiles directly from
the committed name with `clang -arch arm64 -O2 -x c -I lua-5.4.7/src
lua54_harness.c.txt lua-5.4.7/src/liblua.a -lm -o lua54_harness`.

The `sed` copies remove only the reference build constraint in the
throwaway module; ordinary `go test` then runs only these copied files.
`TestTimeout` is expected to exit 1 because the candidate lacks a hard
timeout, and the `test` assertion makes an unexpected result fail. The
recorded outputs were collected in that throwaway module; its setup is
not committed.

## gopher-lua v1.1.1 — `go test -run 'Test(Sandbox|ErrorMapping)$' -v` (gopher_test.go)

```
--- PASS: TestSandbox (0.00s)
--- PASS: TestErrorMapping (0.00s)
PASS
```

The timeout probe is intentionally run separately because it is expected
to fail for this candidate:

```
--- FAIL: TestTimeout (2.20s)
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
unprobed (see the evaluation's unknowns). The command is reproducible
after the throwaway setup above; its setup was not run by repository CI
and the result remains only a raw baseline, not runner evidence.
