# `choir computer refresh` answers 503 "lifecycle durable intent unavailable" with no reason (2026-10-10)

Status: documented, not fixed. Mutation class of the fix: `orange` (proxy
error projection); the lifecycle authority itself is not in question.

## Evidence

Earlier today, refreshing a test computer through its computer-scoped API key
(`choir computer refresh`) returned HTTP 503 `{"error":"lifecycle durable
intent unavailable"}`. The vmctl journal on Node B showed the VM being
refreshed around the same time. `choir computer restart` with a fresh
idempotency key worked and loaded the new guest code (used again at 19:04Z
today for both demo computers).

## What the source shows

`internal/proxy/computer_lifecycle.go`: the 503 is returned when the
**prepare** phase of the durable lifecycle control
(`h.lifecycleControl(..., Phase: "prepare")`) returns any error. The handler
discards that error: it is neither logged nor projected. So a durable
refusal (for example an idempotency or prior-state conflict from corpusd)
and a real outage look identical to the caller.

## Hypotheses (not confirmed by trace)

1. corpusd refused the prepare with a conflict (409-shaped) because the
   prior state or epoch had moved, and the proxy turned that into a 503.
2. The refresh the vmctl journal shows came from another path (the idle
   refresh after a deploy, or an earlier attempt), not from this request.

Either way, the fix is to log the prepare error and project a refusal as a
4xx with its reason, keeping 503 for an unreachable authority. Confirming
which hypothesis held needs the corpusd and proxy logs for that request,
which were not captured.
