# Route script contract (design draft, TASK-0025)

Status: **design contract only** — no runtime, engine, dependency, or
config-migration decision is made here, and no source changes are part
of this document. Anything marked **UNDECIDED** is deliberately open.

This contract defines the observable behavior of an optional scripted
routing hook so that implementation work can be judged against a fixed
agreement, independent of how the script is executed.

## Confirmed live-edit requirement

Configuration edits must take effect without restarting the router. This
applies to current TOML configuration and the future optional Lua script:
a new URL event must observe a newly loaded configuration snapshot, while
an in-flight URL uses the immutable snapshot it started with. This is a
confirmed product requirement. The loading, coherence, and failure
mechanics below remain proposed until implementation review.

## Current loading evidence

The current CLI already loads TOML per command invocation: `runOpen` calls
`config.Load(path)` before constructing the router, and `runExplain` follows
the same per-invocation path. The resident macOS shim spawns the embedded
`brouter open` child separately for each URL event, so current TOML edits are
picked up by the next event without a handler restart. There is no current
compiled config cache or file watcher. This evidence does not yet cover a
future Lua loader.

## Pipeline position

For one URL, evaluation runs in this fixed order:

1. `route(ctx)` — the script hook, if a script is configured.
2. Static config rules, in declaration order (current first-match-wins
   semantics, unchanged).
3. The config `default` target (current fallback, unchanged).

The script acts as a pre-rules override: returning a target short-
circuits rules and default for that URL; returning nothing defers to
the existing pipeline, which keeps its semantics untouched.

## `route(ctx)` contract

A configured script defines one global function:

```lua
function route(ctx)
  -- return a string target name, or nil to defer
end
```

- Returns a string: the name is validated against the config's targets
  at apply time. An unknown name is a failure (below), not a silent
  fallback to default — the distinction is observable in diagnostics.
- Returns `nil`: no opinion; evaluation continues with static rules.
- Returns anything else (number, table, boolean, multiple values):
  extra values are ignored; a non-string, non-nil first value is a
  failure.
- Exactly one `ctx` per URL event. There is no ambient second input.

## `ctx.url` fields

| Field      | Content                                                        |
|------------|----------------------------------------------------------------|
| `original` | the URL byte-for-byte, exactly as received (same string url-regex rules see) |
| `scheme`   | lowercased scheme; only `http`/`https` URLs reach the script   |
| `host`     | lowercased, port removed, one trailing dot stripped, compared as received (no IDN/punycode) — identical to existing host normalization |
| `port`     | decimal port as written, or empty string when absent (no default-port elision) |
| `path`     | as written, no percent-decoding, empty string when absent      |
| `query`    | as written without the `?`, empty string when absent           |
| `fragment` | as written without the `#`, empty string when absent           |
| `userinfo` | **not exposed** — credentials never reach scripts              |

No field is percent-decoded or otherwise normalized beyond the table
above; `ctx.url.original` is always available for byte-exact matching.
Invalid URLs never reach the script: the existing invalid-URL rejection
happens before `route` is invoked.

## `ctx.utils` — date and time

- `ctx.utils.epoch()` → integer Unix seconds (UTC).
- `ctx.utils.utc()` → table `{year, month, day, hour, min, sec}`
  (integers, UTC).
- The clock is sampled **once per `route` invocation**: `epoch()` and
  `utc()` within one call observe the same instant, so a script cannot
  produce a self-inconsistent decision mid-flight.
- Timezone: **UTC only**. There is deliberately no local-time API —
  local time is environment-dependent and breaks deterministic
  routing. Scripts that need local semantics must encode the offset.
- Both functions are total: no invalid-input behavior exists (they
  take no arguments and always succeed).

## Failure and fallback semantics

Runtime evaluation failures in this list remain fail-closed: the URL
continues through static rules and default, exactly as if the script
returned `nil`, and one safe diagnostic category is recorded (no URL, no
script text, no target name in the diagnostic):


| Situation                                   | Diagnostic category |
|---------------------------------------------|---------------------|
| Lua runtime error raised by `route`          | `script-error`      |
| Non-string, non-nil return value             | `script-error`      |
| Returned string names no configured target   | `unknown-target`    |
| Time budget exceeded                         | `script-timeout`    |

- A per-invocation time budget exists; exceeding it is `script-timeout`.
  The exact budget value and enforcement mechanism are **UNDECIDED**.
- Failures are per-URL: other URLs keep using the script. There is no
  automatic global disable after a failure.
- Loading a new TOML or script snapshot is a separate failure class. A
  missing, malformed, unstable, or invalid edited file must fail visibly
  for that URL; it must not silently use a last-known-good snapshot or
  route to a different target. Whether this is a hard stop or a chosen
  visible fallback is **UNDECIDED** and requires explicit product approval.
  A script returning an unknown target remains the `unknown-target`
  runtime-evaluation case above unless that policy is explicitly changed.
- A spawned/launched browser remains outside the script's knowledge:
  the script decides a *target*, nothing more; downstream outcome
  reporting (TASK-0023 semantics) is unchanged.

## Per-URL snapshot and edit semantics

The implementation must evaluate each new URL against one immutable
configuration-plus-script snapshot. It must not use a long-lived watcher,
compiled stale cache, or an evaluation that can mix files from different
loads. A symlink is resolved when a new snapshot is loaded; replacing the
symlink affects the next URL, not an in-flight URL.

Proposed mechanics (**UNDECIDED**): read the TOML and script bytes for the
same request, check each file for changes during its read, and reject an
unstable read rather than evaluating partial content. Independently edited
TOML and script files cannot have a universal atomic commit guarantee;
the implementation must document that boundary and recommend atomic
rename/update of each file, or add an explicit coherence marker, rather
than claim cross-file transactions. Bounded retries and the exact
stability/coherence check require design review.

`validate` and `explain` should use the same current-file loader semantics
as `open`; they must never validate or explain a compiled stale snapshot.

## Immutability and isolation

- `ctx` is read-only. Fields are plain values; there is no API to
  mutate the URL or influence later stages beyond the return value.
- One fresh `ctx` per URL event; no documented state carries over.
  Scripts **should** be pure: same `ctx` ⇒ same result. Enforcement of
  purity (fresh function environments, table freezes) is
  **UNDECIDED**.
- Scripts have no I/O: no filesystem, network, processes, or
  environment access. Which standard-library parts are removed or
  stubbed is an implementation decision (**UNDECIDED**).
- Isolation provider (embedded interpreter vs. separate process), the
  config schema that enables a script, and the exact load-failure policy
  are **UNDECIDED**. No config migration is specified in this contract;
  static rules remain the source of truth and keep working without a
  script. Any chosen provider must preserve per-new-URL loading and must
  not require a router restart or watcher-managed mutable state.


## Privacy

- Scripts see the full URL by design (that is their purpose).
- Diagnostics gain only the categories above — the URL redaction
  contract (no URLs, hosts, query strings, or fragments in logs or the
  Recent Activity menu) is unchanged and applies to script failures.
- `explain` remains the authoritative record of the final decision; how
  script decisions render in `explain` is **UNDECIDED**.

## Examples

Happy — work-hours override, otherwise defer:

```lua
function route(ctx)
  local t = ctx.utils.utc()
  local on_hours = t.hour >= 9 and t.hour < 17
    and t.wday ~= 1 and t.wday ~= 7   -- see note
  if on_hours and ctx.url.host == "example.com" then
    return "brave-work"
  end
  return nil
end
```

Note: weekday fields require a contract decision on whether `utc()`
exposes `wday`/`yday`; the minimal table above omits them. If added,
they are integers, UTC, and listed in this table — **UNDECIDED**
(minimal vs. full `os.date("*t")` parity).

Happy — inspect the original URL byte-for-byte:

```lua
function route(ctx)
  if string.find(ctx.url.original, "^https://files%.example%.internal/", false) then
    return "legacy-tool"
  end
  return nil
end
```

Unhappy — each of these falls back to static rules and records one
safe category:

```lua
function route(ctx) error("boom") end          -- script-error
function route(ctx) return 42 end              -- script-error
function route(ctx) return "typo-browser" end  -- unknown-target
while true do end                              -- script-timeout (budget applies)
```

## Verification story (when implemented)

Contract tests, not feature tests: pinned clock and IDs; category
mapping for each failure row; determinism (same ctx ⇒ same result
across repeats); redaction sweeps asserting URLs never reach any
diagnostic sink; fallback equivalence (runtime failure output equals
nil-case output); and live-edit cases proving TOML/script edits apply to
the next URL without restart while an in-flight request keeps its
snapshot. Include symlink replacement, atomic rename, malformed/partial
writes, unstable-read rejection, missing files, and cross-file edit
boundary cases. Engine-agnostic: the suite must pass regardless of the
chosen provider.

