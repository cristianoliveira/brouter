---
id: TASK-0009
title: Implement deterministic URL matching and fallback
status: doing
depends_on: [TASK-0002]
tags: [domain]
---

## Problem
A wrong or ambiguous rule can open a work URL in a personal browser.

## Outcome
Pure routing decisions are deterministic and explainable without filesystem or process access.

## Acceptance criteria
- Implement approved exact-host, explicit subdomain, and full-URL Go regex matching; one matcher per rule initially.
- First match wins; unmatched URLs select the explicit default.
- Reject unsupported schemes and malformed URLs; define host case, ports, trailing dots and internationalized-host handling explicitly.
- Match hosts by parsed labels, not substring: company.example.evil.test and evilcompany.example do not match company.example.
- Preserve original URL for eventual forwarding; document which representation regex evaluates.
- Return ordered evaluation reasons and selected target as data, usable by open and explain.

## Verification
Table-driven tests cover boundaries, overlapping rules, fallback, invalid URLs, normalization and stable output; add bounded fuzz targets for parser/matcher robustness. No I/O or clock-dependent tests.

## Non-goals
Config parsing, redirects, HTTP requests, or source-app matching.
