---
id: TASK-0026
title: Preview Brouter menu-bar icon variations
status: doing
depends_on: []
priority: normal
tags: [macos, icon, preview]
---

# Preview Brouter menu-bar icon variations

## Problem
The menu-bar handler needs a recognizable lowercase b monogram with a continuous curved arrow stroke, but selecting a final mark without comparing actual menu-bar rendering risks poor legibility in light and dark appearances.

## Context
(Optional: approach, links, related tasks.)

## Acceptance criteria
- [ ] Produce 2–3 distinct SVG mark variations under `docs/assets/menu-bar-icon-previews/`, each a lowercase `b` monogram whose curved stroke ends in an arrowhead as one continuous shape; do not use a generic forked routing icon.
- [ ] Render every variation at actual menu-bar size and enlarged on both light and dark backgrounds, preserving template-style monochrome rendering; keep previews easy to compare and inspect.
- [ ] Include `docs/assets/menu-bar-icon-previews/README.md` with a short comparison covering legibility at actual size, recognizable `b` silhouette, arrow direction/continuity, and accessibility implications. Keep the existing accessible label `Brouter` unchanged.
- [ ] Keep this preview-only: do not replace the app/menu icon, alter `Info.plist`, change package assets, integrate into native code, or introduce broad branding/system changes.
- [ ] Kelly’s review checks light/dark contrast, silhouette legibility, anti-aliasing at menu-bar size, and that each candidate remains distinguishable without relying on color.

## Notes
User approved preview comparison only. Final mark selection and app integration require a later explicit choice; preserve the current template icon until then.
