# External routing command (TASK-0029)

Opt-in routing logic in any language: brouter invokes your executable
directly by argv (never through a shell) once per URL, feeds it the
URL on stdin, and reads back a single-line decision.

## Configuration

```toml
[route_command]
command = ["/path/to/my-router", "--flag"]
```

The command is an argv array: the first element is the executable.
Absolute paths are used directly, and bare names are resolved via `$PATH`
at first use. Explicit relative paths such as `./route.py` resolve from
the selected config file's logical directory, not the process working
directory. This remains true when the config is a symlink, so a
colocated `route.py` symlink can point to a mutable script beside the
tracked config. Arguments are passed unchanged; there is no shell, tilde,
or environment-variable expansion. Configs without this section behave
exactly as before.

## Protocol (v1)

brouter spawns the command once per URL and writes the URL's original
bytes followed by exactly one LF to stdin, then closes stdin. The
bytes are never JSON-encoded, decoded, normalized, or shell-escaped;
control bytes, userinfo credentials, over-long URLs, and malformed
URLs are rejected before the command is spawned.

The command answers on stdout with exactly one line:

```
work-browser
```

- The line is one configured target ID with no newline, one terminal
  LF, or one terminal CRLF.
- The reserved literal `@default` means explicit defer to the ordered
  static TOML rules. It cannot be configured as a browser ID while
  `route_command` is active.
- Anything else — empty output, extra lines, embedded CR/LF,
  non-UTF-8 bytes, wrong target — is a visible failure. No `jq` or
  other helper is required: `read line` in shell, `sys.stdin.read()`
  in Python, `io.read()` in Lua all work.

## Bounds and failures

- Hard 2s wall-clock timeout: the command is killed.
- 4 KiB stdout/stderr capture cap: exceeding it is a failure.
- Fixed redacted categories: `route_command: command failed (exit
  nonzero)`, `route_command: command unavailable`, `route_command:
  command timed out`, `route_command: command output exceeded the
  size limit`, `route_command: invalid response`,
  `route_command: canceled` (caller-initiated cancellation), and
  `route_command: unknown target`. URLs and command output never
  appear in errors. Target IDs must be plain printable strings —
  control bytes are rejected.
- Every failure is visible and stops the open. Only the exact
  `@default` line defers to the ordered static rules; failures never
  silently do so.

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
