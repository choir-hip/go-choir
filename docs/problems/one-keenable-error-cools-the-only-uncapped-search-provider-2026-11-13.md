# One Keenable error cools the only uncapped search provider

Recorded 2026-11-13T23:40Z. Status: observed, cause of the cooling located
in the logs, code not yet read, no fix.

## Problem

The paper's searches return nothing for minutes at a time. Keenable is the
abundant primary (owner, 2026-11-13: "Keenable is abundant. That should be
working regardless"); every paid provider is quota-limited by design. When
Keenable is marked `error` and cools, no provider is left and each search is
a miss.

## Evidence

Gateway journal on Node B, read at 23:38Z:

- 22:59:28Z `searchplane: keenable call failed class=error http_status=404
  stage=http_status latency_ms=6342`. One line in 90 minutes.
- 23:02Z to 23:03Z: 30 paper searches logged `answered 0 of 0 providers
  (8 cooling, 0 pending)` with `keenable=error` in the skipped list.
- Hourly counts of paper searches answered by no provider: 0 of 236
  (20:35Z), 0 of 330 (21:35Z), 18 of 349 (22:35Z), 32 of 238 (23:35Z).
- Keenable had answered 6,970 searches that day.

## Belief

A single failed Keenable call puts it in the same cooldown as any other
provider, and the cooldown lasts minutes. For a provider with no cap and no
substitute, that turns one upstream 404 into a few dozen empty searches.

## Remaining error

- The cooldown length and whether it grows with repeated errors are not read
  from the code.
- Why Keenable returned a 404 after 6.3 s is unknown (a bad query path, or
  an upstream fault).
- The 22:35Z hour's 18 misses were not traced to a Keenable error line.
- Whether the paper's researcher retries a missed search later is unknown.

## Fix direction

Not a limit to tune blindly. Read the cooldown rule first; the likely change
is that a provider with nothing behind it is retried on the next request
(or after a short, fixed backoff following a real refusal) rather than
skipped for minutes.
