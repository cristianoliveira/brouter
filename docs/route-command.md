# External routing command (TASK-0029)

Opt-in routing logic in any language: brouter invokes your executable
directly by argv (never through a shell) once per URL and reads a small
versioned JSON decision.

## Configuration

```toml
[route_command]
command = ["/path/to/my-router", "--flag"]
```

The command is an argv array: the first element is the executable
(absolute path, or resolved via `$PATH` at first use). Configs without
this section behave exactly as before.

## Protocol (v1)

brouter spawns the command once per URL and writes one bounded JSON
object to its stdin, then closes it:

```json
{"version":1,"url":{"original":"https://example.com/x?a=1#f","scheme":"https","host":"example.com","port":"","path":"/x","query":"a=1","fragment":"f"},"utils":{"epoch":1700000000,"utc":{"year":2023,"month":11,"day":14,"hour":22,"min":13,"sec":20}}}
```

- `url.original` is the URL byte-for-byte as typed; `scheme`/`host` are
  normalized like the static router (lowercase, one trailing host dot
  stripped); `path`/`query`/`fragment` keep their percent-escapes as
  written; userinfo credentials never appear anywhere.
- `utils` is one clock sample frozen by brouter for this decision.

The command answers with exactly one JSON object on stdout:

```json
{"version":1,"route":"work-browser"}
```

- `"route"` must be an exact configured target ID, or explicit `null`
  to defer to the ordered static rules. Nothing else defers.
- Strict decoding: unknown keys, wrong versions, trailing bytes, or
  non-JSON output are failures. Keep stdout clean — print diagnostics
  to stderr (captured, bounded, never forwarded into responses).

## Bounds and failures

- Hard 2s wall-clock timeout: the command is killed.
- 4 KiB stdout/stderr capture cap: exceeding it is a failure.
- Fixed redacted categories: `route_command: command failed (exit
  nonzero)`, `route_command: command timed out`, `route_command:
  command output exceeded the size limit`, `route_command: invalid
  response`, and `route_command: unknown target`. URLs and command
  output never appear in errors.
- Every failure is visible and stops the open. Only `route:null`
  falls back to static rules; failures never silently do so.

Malformed URLs, non-http(s) schemes, empty hosts, and URLs containing
userinfo credentials are rejected before the command is spawned
(userinfo rejection uses a fixed message that never echoes the URL).

## validate and explain never execute

`brouter validate` checks the section's structure only (no `$PATH`
resolution, no execution). `brouter explain` reports that a command is
configured but never runs it: commands may be impure or carry side
effects; only real `open` traffic invokes them.

## Trusted, unsandboxed code — read this

The routing command runs with **your full user permissions**. It is
not sandboxed: it can read and write files, use the network, launch
processes, and see every URL you open. Only add commands you author or
trust. This is an explicit design tradeoff of the external-command
approach.

## Config snapshot semantics

The TOML config is loaded for every URL with bounded observed-change
detection: the file is read twice and a load that observes the file
changing is rejected visibly. This detects edits racing an open but
cannot distinguish a stationary syntactically-valid partial write;
save configs atomically (write-then-rename, as editors' atomic-save
modes do). There is no watcher and no retry loop.
