# Python route-command example

This self-contained example routes invented work domains during illustrative
local working hours (Monday-Friday, 09:00 inclusive to 17:00 exclusive):

- `meetings.example.com` and `issues.example.com` → `work` during hours.
- The same work URLs → `personal` outside hours and on weekends.
- `localhost` and `dev.example.test` → `dev` at all times.
- Other URLs → `@default`, which lets the normal static-rule, installed-app,
  and configured-default pipeline decide.

The work check uses `datetime.now()`, so it uses the host's local timezone.
Tests inject `datetime` values into `choose_target` for deterministic boundary
and weekend coverage.

## Install beside a config

Brouter invokes route commands directly, never through a shell. The example
uses an executable config-relative command:

```sh
cd /path/to/brouter/examples/python-route-command
chmod +x route.py
brouter validate --config "$PWD/config.toml"
```

When a config is symlinked, place a `route.py` symlink beside the selected
config path if the script lives elsewhere. Relative executable resolution uses
the logical config directory, not the process working directory. Brouter does
not expand `~`, `$HOME`, or shell syntax. Configuration is loaded for each
URL, so changing this script or config needs no brouter process restart once
the installed binary supports `route_command`.

Replace the placeholder browser commands in `config.toml` before opening URLs.
Use a temporary fake browser for safe tests; do not point this example at a
real browser until its routing decisions are verified.

## Protocol

Brouter writes the original URL bytes plus one LF to stdin. The script writes
exactly one target and one LF to stdout. `@default` is the only defer token;
failures are visible and never silently fall back. The command is trusted,
unsandboxed local code, so only run scripts you author or trust.

`brouter validate` checks structure without executing the command, and
`brouter explain` reports routing without executing it. Use `open` only
with a fake browser when exercising the command itself.
