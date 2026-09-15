# Route scripts (opt-in, TASK-0029 first slice)

`brouter open` can consult an optional Lua script before the static
config rules. Add this to your config file:

```toml
[lua]
script = "route.lua"   # relative to the config file, or absolute
```

The script defines one function:

```lua
function route(ctx)
  if ctx.url.host == "example.com" then
    return "work-browser"   -- a target defined in this config
  end
  return nil                 -- defer to the static rules
end
```

`ctx.url` carries `original`, `scheme`, `host`, `port`, `path`,
`query`, `fragment` (raw bytes, no decoding; userinfo is never
exposed). `ctx.utils.epoch()` and `ctx.utils.utc()` read one frozen
clock sample per decision (UTC only). The script's return must be a
string target or nil.

## What the script can and cannot do

- No filesystem, network, process, environment, package, or debug
  access: those libraries are absent from the isolated helper, and a
  hard 8 MiB allocation cap plus an instruction budget bound every
  run.
- No output channel either: `print` and `warn` are unavailable, so a
  script cannot corrupt the helper's framed responses. Scripts return
  decisions; they do not log.
- Invalid URLs never reach the script: only well-formed http/https
  URLs are evaluated, with the scheme/host normalized exactly like
  the static router (lowercase, one trailing host dot stripped).
- Returning an unknown target, raising an error, timing out, or
  exceeding the memory cap falls back to the static rules for that
  URL and records a safe, URL-free category. A browser launching is
  never proof a page loaded.
- A broken script (syntax error, missing `route`) is a visible
  failure: the open stops with `script-load-error` / `missing-route-
  function` rather than silently routing.

## Live edits

The script is re-read for every URL — edits apply on the next open
without restarting anything. There is no cache and no last-known-good
fallback: a broken edit fails visibly until you fix it.

## Isolation

Each decision runs in a short-lived helper process (`brouter-lua-
helper`, vendored Lua 5.4.7, MIT — see `third-party/lua-5.4.7`), so
runaway scripts are killed by an instruction budget and a wall-clock
backstop, and the Go router stays CGO-free. Diagnostics record only
safe categories, never URLs or script text. Quitting browsers or
editing files outside your config directory is never part of routing.
