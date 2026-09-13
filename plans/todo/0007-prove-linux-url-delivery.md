---
id: TASK-0007
title: Prove Linux desktop URL receipt and browser forwarding
status: doing
depends_on: [TASK-0002]
priority: high
tags: [linux, spike]
---

## Problem
NixOS desktop handlers and Nix executable paths can change launch behavior despite a portable Go binary. Only NixOS with Nix-installed browsers is in scope.

## Outcome
A minimal experiment verifies the agreed Linux baseline.

## Acceptance criteria
- Register a temporary desktop HTTP/HTTPS handler on NixOS with Sway (Wayland) and demonstrate OS-delivered URL receipt.
- Exercise the actual graphical-session opener and any portal path used by the chosen source app; record required desktop/session environment without assuming a GNOME/KDE session or X11.
- Prove browser resolution from the graphical session, not only a development shell; distinguish stable user/system profile paths from immutable Nix store paths and record implications for declarative installation.
- Forward an unchanged URL to an installed Brave or Chrome, closed and already running.
- Record desktop/session, browser version, package format, executable resolution, and sandbox limitations.
- Probe profile selection if feasible and report uncertainty explicitly.
- Prove an observable failure channel for desktop-originated errors on the supported desktop; record behavior when notifications are unavailable and how users inspect failures without persistent URL logging.
- Preserve previous handler settings and document restoration; no silent system-default changes.

## Verification
Record real desktop smoke evidence. Headless CI or process mocks alone do not establish desktop support.

## Non-goals
Other distributions, Flatpak/Snap, production installation, or source-app detection.
