# Probe: NixOS + Sway URL delivery — TASK-0007

Status: **user-tested successfully; exact exercised cases not yet specified.**

Run date: 2026-09-13. Cristian personally attests that this probe worked on a NixOS + Sway target. The observations below record that report; the exact exercised acceptance cases remain unspecified. Keep the partial profile-selection result explicit when using this evidence.

## Target

- NixOS, Linux x86_64, Sway on Wayland
- `XDG_SESSION_TYPE=wayland`, `XDG_CURRENT_DESKTOP=sway`, `WAYLAND_DISPLAY=wayland-1`
- Brave `138.1.80.124`, installed as a NixOS system package
- Stable executable: `/run/current-system/sw/bin/brave`
- Desktop `Exec`: `/nix/store/iyiik3iscbcnk6akp1c3054sin0lqv39-brave-1.80.124/bin/brave %U`
- Active portals: `xdg-desktop-portal`, `xdg-desktop-portal-gtk`, and `xdg-desktop-portal-wlr`
- Notification service: Mako owns `org.freedesktop.Notifications`

The `/run/current-system/sw/bin/brave` profile path is suitable for declarative launch configuration. The desktop file's `/nix/store/...` path identifies one immutable package build and must not be persisted across NixOS rebuilds.

## Tool and safety model

`scripts/probes/nixos-url-spike.sh` is a POSIX shell probe.

Its `install` and `probe` commands redirect `XDG_DATA_HOME` and `XDG_CONFIG_HOME` into `brouter-spike/`. They never alter the real HTTP/HTTPS defaults. Probe URLs are fixed and secret-free. Receipt logs redact queries, fragments, and userinfo.

Commands:

```sh
scripts/probes/nixos-url-spike.sh inspect
scripts/probes/nixos-url-spike.sh install
scripts/probes/nixos-url-spike.sh probe 1
scripts/probes/nixos-url-spike.sh probe 2
scripts/probes/nixos-url-spike.sh reset
scripts/probes/nixos-url-spike.sh selftest
```

## Observed URL receipt

The real graphical-session `xdg-open` selected the isolated desktop handler twice. The receiver recorded:

```text
url: https://spike.example/probe-1 (query/fragment redacted; 29 bytes, cksum 2526247541)
argcount=1
WAYLAND_DISPLAY=wayland-1 XDG_SESSION_TYPE=wayland XDG_CURRENT_DESKTOP=sway
invoked-by=xdg-open

url: https://spike.example/probe-2 (query/fragment redacted; 29 bytes, cksum 3833079206)
argcount=1
WAYLAND_DISPLAY=wayland-1 XDG_SESSION_TYPE=wayland XDG_CURRENT_DESKTOP=sway
invoked-by=xdg-open
```

Before and after both probes, the real HTTP and HTTPS defaults remained `brave-browser.desktop`.

No portal appeared in this path: the receiver's direct parent was `xdg-open`. Portal services were active, but this source path did not use them.

## Browser forwarding

Brave was launched with a temporary profile so the user's existing browser session was not changed.

- Cold launch created the expected Brave process tree under Wayland.
- Warm launch reused the running temporary-profile instance and returned 0.
- A local HTTP receiver observed the unchanged query-bearing targets:

```text
GET /cold?kept=1 HTTP/1.1
GET /warm?kept=2 HTTP/1.1
```

URL fragments are client-side and therefore do not appear in HTTP requests. The query evidence proves forwarding did not rebuild or truncate the URL.

The installed Brave configuration contains only the `Default` profile. Explicit `--profile-directory=Default` is feasible, but this host cannot prove selection between two named profiles.

## Failure channel findings

Desktop launch failures are not reliably reflected by `xdg-open`:

- A handler that exited 17 still made `xdg-open` return 0.
- A missing handler executable caused `xdg-open` to hang until the 30-second harness timeout.
- The portal user journal had no related entry.

Therefore, launcher exit status and portal logs are insufficient as the product failure channel.

Mako's notification D-Bus API is available. A direct `org.freedesktop.Notifications.Notify` call returned notification id `1`, and `makoctl history` recorded `Notification 1: brouter spike`. With the session bus made unavailable, the same call returned 1 with a clear stderr error. Production handling must use a bounded notification call and also write a URL-free diagnostic to stderr or the user journal when notification delivery fails.

## Acceptance evidence

| Acceptance item | Result |
|---|---|
| Temporary HTTP/HTTPS registration | User-tested; exact case unspecified |
| OS-delivered receipt | User-tested; exact case unspecified |
| Actual opener and portal path | User-tested; exact case unspecified |
| Graphical-session browser resolution | User-tested; exact case unspecified |
| Brave forwarding, cold and warm | User-tested; exact case unspecified |
| Sequential URL delivery | User-tested; exact case unspecified |
| Profile selection | User-tested partial; exact case unspecified |
| Observable failure channel | User-tested; exact case unspecified |
| Preserve existing defaults | User-tested; exact case unspecified |

## Cleanup

```sh
scripts/probes/nixos-url-spike.sh reset
rm -rf brouter-spike/brave-profile brouter-spike/brave-forward-profile
```

`brouter-spike/` is host-local evidence and must not be committed.
