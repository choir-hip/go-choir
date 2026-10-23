# The front-importance scorer failed silently for 13 hours, and the paper lost its sections (2026-10-23)

Found 2026-10-23 13:1x UTC by FP0, after the owner asked where the
Autopaper sections went. Mutation class of this record: green.

## What happened

- [V] From 23:52 UTC on 2026-10-22 to 13:18 UTC on 2026-10-23, every run
  of `go-choir-front-importance.service` (the Node B timer, every 10
  minutes, `corpusd front-importance`, internal/frontimportance/scorer.go)
  failed every story with `gateway answered 502: {"error":"Jev judgment
  provider unavailable"}`. The 13:18 run read "100 stories, 0 kept, 0
  scored, 80 failed, 20 deferred".
- [V] Cause: the OpenRouter account was out of credit (total credits 85.75,
  total usage 85.96 at 13:2x; the key's own $50 limit had $37 left, so the
  account balance, not the key limit, refused). Jev runs on OpenRouter
  (`~typesafe/jev-latest`, internal/gateway/jev.go). The owner replenished
  the account at about 13:25.
- [V] Effect: new stories got no section and no importance score. The
  section switcher shows only when at least half the front stories carry a
  section (frontend/src/lib/autopaper.ts, `usesSections`); 1 of the 60
  newest did, so the World, U.S. Politics, Business & Finance and Tech & AI
  sections disappeared, and new stories ranked by recency only.
- [V] Recovery needed no deploy: the 13:28:40 run scored 80, failed 0, and
  all 60 of the newest front stories carry a section.

## Why it was silent

- [V] The scorer is best effort by design: a failure logs and keeps the
  previous scores file, and a missing score ranks by recency. Nothing
  alerts on a run that scores nothing.
- [V] The gateway logs `host judgment from host:front-importance failed`
  without the provider's status, so the journal never said "out of
  credit". (handlers: internal/gateway/host_judgment.go:66-73 drops the
  error.)
- The owner noticed by reading the paper, about 13 hours later.

## Options (not chosen)

1. Alert when a scorer run has `scored == 0 && failed > 0` for N runs in a
   row (an ops-status line, or the status page). Cheap, observe only.
2. Log the provider's HTTP status and error class (not the body) on a
   failed Jev decision, so the journal names "402/insufficient credit".
3. Show a visible fallback state on the paper: when sections cannot be
   filled, say so to the operator view rather than hide the switcher.
4. Watch the OpenRouter balance (`/api/v1/credits`) from the host, like the
   Synthetic quota line, and warn below a floor.

Options 1 and 2 are the smallest and answer this failure directly.
