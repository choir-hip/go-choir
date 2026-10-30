# S0m blocker: prompt-bar submit fails — Dolt projection batch write cancelled under load

**Date:** 2026-10-02
**Discovered by:** S0m finish-acceptance attempt on staging (deployed `734ce69b`)
**Station:** S0m record-native cutover — blocks the ask→research→resolve acceptance chain

## Symptom

`POST /api/prompt-bar` never returns (client timeouts at 45s / 90s / 120s).
Guest journal shows the server-side commit aborting:

```
texture prompt bar: submit: persist conductor route:
  store: append projection batch: computer event appender:
    prepare embedded projection: computer event projection:
      check idempotency: context canceled
texture prompt bar: submit: persist conductor route:
  objectgraph dolt: scan object: context canceled
texture prompt bar: submit: start initial Texture agent revision:
  persist submitted event: store: context canceled
texture prompt bar: submit: start initial Texture agent revision:
  start Texture revision: load current revision: context canceled
```

## Evidence / state

- Guest autoputer is **healthy** (no crash loop, `runtime_health=ready`,
  `running_runs` 3-5, `desk_pending_mutations` draining 47→43). The
  `e4780c1a` crash-loop fix and `ae47c8a4` dispatch fix are both confirmed live.
- The failure is in `CompletePromptBarDecision`'s transaction: `persist
  conductor route` → `append projection batch` → `computer event appender`
  → `context canceled`. The request context is being cancelled mid-commit.
- Trajectories ARE minting on staging (200 live, newest 16:57) — those come
  from the autonomous desk backlog, not from prompt-bar submit. My repeated
  submits produced no new texture trajectory (no `Firecracker`/`UFFD`/`s0m`
  objective found in `GET /api/trajectories`).

## Hypothesis

Two candidates, not yet distinguished:

1. **Server-side deadline on the projection batch** — the append's context
   carries a deadline that the Dolt object-graph write exceeds under the
   post-refresh backlog load (dozens of concurrent desk reconciles + wake
   re-drives saturating the embedded Dolt writer). The batch is retried or
   the request dies; either way the commit never lands.
2. **A real lock/serialization deadlock** in `projectLifecycleRun` /
   `appendProjectionBatch` contended by the wake-drain and reconcile traffic —
   the context deadline is the symptom, the lock wait the cause.

`context canceled` (not `deadline exceeded`) suggests an explicit `cancel()`
or a request-scope context torn down — possibly the proxy closing the request
on its own timeout while the server still holds the write.

## Repro

```
POST /api/prompt-bar  {"text":"…","command_id":"s0m-askresearch-chain-N"}
```

→ client `curl: (28) Operation timed out` at >45s; server logs the
`context canceled` chain above; no trajectory minted.

## Not a fix here

The commit-path timeout belongs to the store/projection layer and needs a
bounded write deadline + retry policy, or the projection batch needs to run
under `context.WithoutCancel`. This is the substrate-level blocker for the
S0m finish acceptance; flag for the store/dispatch owner (overlaps S0's
lock-substrate cluster — `t.mu`/`m.mu` reentry already documented).
