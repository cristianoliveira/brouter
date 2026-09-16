---
id: TASK-0033
title: Handle local HTML files safely on macOS
status: todo
depends_on: []
priority: high
tags: [macos, local-files, bug]
---

# Handle local HTML files safely on macOS

## Problem
On macOS, opening a local HTML file through the system file handler dispatches to BrouterHandler and shows an unsupported HTML-format dialog instead of opening the document in an appropriate browser. The document-association versus app-open-file cause is not yet verified.

## Context

Observed behavior: macOS dispatches a local `index.html` document to
`BrouterHandler`, then shows an unsupported-format dialog instead of opening
the document in an appropriate browser. The original local path and screenshot
are intentionally not versioned.

Generalized reproduction:

```sh
mkdir -p /tmp/brouter-example
printf '%s\n' '<!doctype html><title>brouter example</title><p>Example</p>' \\
  > /tmp/brouter-example/index.html
open /tmp/brouter-example/index.html
```

The failure mechanism is not yet established. Investigate both LaunchServices
document association and the app's open-file handling before choosing a fix.

## Acceptance criteria

- [ ] Reproduce the behavior with the generalized `/tmp/brouter-example/index.html`
      fixture and record the dispatch and failure evidence.
- [ ] Identify whether the cause is document association, app open-file
      handling, or both; do not infer the cause from the dialog alone.
- [ ] Define a safe local-HTML policy that opens HTML in an appropriate browser
      without recursively dispatching through the default HTTP(S) handler.
- [ ] Preserve HTTP(S) routing, privacy guarantees, and rejection of arbitrary
      unsupported file formats.
- [ ] Add deterministic regression coverage or a safe macOS probe for the
      selected policy, without automating assertions by grepping documentation.

## Notes

This is a registration-only bug task. Do not change LaunchServices defaults,
app assets, installation, or runtime behavior as part of registration.
