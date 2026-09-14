---
id: TASK-0022
title: Show the running macOS handler in the menu bar
status: todo
depends_on: [TASK-0021]
tags: [macos, ui]
---

## Problem
Users currently need process commands to know whether BrouterHandler is running. A live process also does not establish default-browser selection or successful URL delivery.

## Outcome
The existing macOS handler has a recognizable, accessible menu-bar icon while its GUI application is running.

## Acceptance criteria
- Use the existing AppKit application/event loop and one retained native status item, not another daemon or tray framework.
- Show a template-style icon legible in light/dark menu bars, with accessible name and tooltip identifying Brouter. Do not add a Dock icon or steal focus on URL delivery.
- Initial menu identifies the application as running and provides Quit. Running must not be labeled healthy, routing successfully, or default unless separately verified; no new default-status polling in this slice.
- Normal GUI startup and cold OS URL delivery initialize the icon; repeated OS opens keep one item in that app instance. CLI open/validate/explain stay headless and keep their exit behavior.
- Quit exits the GUI handler without killing browsers, rewriting config or changing OS defaults. Document that future OS URL delivery may relaunch a selected handler; Quit is not a persistent disable switch.
- Keep existing direct-argv and cold/warm Apple Event forwarding working. Document instance/lifetime behavior without global process-killing or locking schemes.

## Verification
Test lifecycle/menu actions with focused native seams where feasible, then observe actual status-item visibility, Quit and OS URL delivery on Apple Silicon. Assert accessible/user-facing behavior, not internal icon geometry. No default changes or user app termination without permission.

## Non-goals
Linux tray, login/startup registration, browser chooser, routing toggle, config GUI, health polling, or new routing logic in Objective-C.
