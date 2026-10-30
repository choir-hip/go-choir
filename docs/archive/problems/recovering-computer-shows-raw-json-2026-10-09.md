# A recovering computer shows the owner raw JSON (2026-10-09)

Found by the first run of the Texture acceptance suite (17:35Z), before
any check ran. Mutation class of the fix: orange (proxy response for
browser requests).

## Evidence

- Account `4b579d04…` (a probe account from 2026-09-23), computer
  `computer-fd097d5d…`, event head 134. Its VM directory
  (`vm-47daf854…`) no longer exists: storage reclaim removed the
  realization, which O21 allows.
- It had never been checkpointed (watermark 0), so recovery was refused:
  `resolve blocked (projection_base_missing): required base is missing
  for an existing chain`, 17:35:38–39. The refusal queued a repair;
  `checkpointd` published the base (target 134) at 17:36:13, about 35 s
  later. The recovery design worked.
- The browser's page load of `/` got the refusal body as the whole page:
  `{"error":"computer recovery blocked","reason":"required base is
  missing for an existing chain","kind":"projection_base_missing",
  "retry_after_seconds":60,...}`. Nothing retried; the page stayed on
  that JSON for 10 minutes (Playwright snapshot).

## Problem

Any owner whose computer is starting, recovering or briefly blocked sees
a raw JSON error on the desktop URL and has to reload by hand. The
refusal already carries `retry_after_seconds`.

## Fix direction

For a browser page request (`Accept: text/html`) to the computer surface,
the proxy answers a typed transient refusal with a small HTML page: "Your
computer is starting up" (or "recovering"), the reason in plain words,
and an automatic retry after `retry_after_seconds` (capped). API callers
keep the JSON. No change to the refusal itself.

## Residual

Computers created before `checkpointd` existed may still have no base;
each pays a one-time ~1 min recovery on its first visit after its disk is
reclaimed. A one-time backfill would remove that; counted, not fixed here.
