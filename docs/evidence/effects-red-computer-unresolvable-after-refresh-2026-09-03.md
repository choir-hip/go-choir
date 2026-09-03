# Problem Receipt: Retained Computer Unresolvable After Refresh 502s (2026-09-03)

- Date: 2026-09-03
- Mutation class of this receipt: green (documentation); any repair is red
- Status: documented before any fix; no fix attempted beyond two product-path refresh retries
- Computer: `computer-03335285269bdba4f94377e56879f9e6` (pre-A fence `99949fe2`; no state mutation observed)

## The Problem

The retained staging computer is unreachable through every product path. All
computer-scoped requests (`/api/runs`, `/api/runtime/observability`, `choir
computer refresh`) fail at proxy autoputer resolution:

```json
{"error":"failed to resolve user autoputer"}
```

(`internal/proxy/handlers.go:1260`, via `writeResolveError`: proxy could not
`resolveComputerURL` through vmctl.) Proxy `/health` is `ok`, `vmctl: ok`,
`deployed_commit: e3edfc6e`. The failure is below the proxy, at
proxy→vmctl→guest resolution/routing.

## Timeline (UTC 2026-09-03)

- 15:55:30Z — owner-authorized refresh to `bf6c51c0` succeeds (epoch 860→861,
  receipt `01a067fb`). Guest healthy on `bf6c51c0`; boot log shows clean
  rewarm (candidate `aa111cdd` pending=0, did-not-enter-selection, Super sweep
  skipped). No new Super rows.
- 16:50:33Z — CI `33776433866` (resume-watchdog fix `e3edfc6e`) succeeds;
  Deploy to Staging (Node B) succeeds. Proxy reports `deployed_commit: e3edfc6e`.
- ~16:53Z — refresh to `e3edfc6e` (new idempotency key): **HTTP 502, empty
  body, after 343s**. No lifecycle receipt returned; epoch advance unknown.
- ~17:00–17:20Z — all computer paths return "failed to resolve user autoputer"
  (8 consecutive observability probes + runs query).
- ~17:2xZ — refresh retry (new idempotency key): **HTTP 502, empty body, after
  393s**. Proxy and vmctl still report ok.

## What This Is Not (ruled out)

- Not the resume-hang jam: the failure is at URL resolution, before any agent
  run or mailbox is consulted. No run rows changed (unobservable directly, but
  the error precedes store access).
- Not proxy-deploy breakage in the ordinary sense: `/health` serves the new
  commit with `vmctl: ok`.

## Candidate Causes ([INFERENCE], unordered)

1. Realization replacement stuck: the first refresh dispatched server-side
   (hence 502-as-timeout, not rejection), tore down epoch-861 VM, and the
   epoch-862 replacement never became routable.
2. Stale vmctl route-slot post-deploy: proxy resolves through a cached route
   the deploy invalidated.
3. Coincidental Node B infrastructure fault (timing overlaps the e3edfc6e
   deploy window).

## Next Safe Probes (owner/Node B)

1. vmctl-side realization state for the computer (exists? booting? failed?).
   Product path cannot observe this while resolution fails.
2. Node B proxy/vmctl logs around the two 502s (request reached vmctl? guest
   dial timeout? route miss?).
3. SSH journal read-only inspection is the established diagnostic path; no SSH
   mutations (never repair routability by hand-editing Node B state).

## Impact on In-Flight Work

- Resume-watchdog fix (`e3edfc6e`): implemented, unit-tested, CI-green,
  proxy-deployed — but **staging verification is blocked** until the guest is
  reachable (needs a boot on the new binary to exercise rewarm paths).
- Definition 1 follow-up re-proof: blocked on the same recovery.
- No data-loss signal: fence `99949fe2` untouched; last observed guest state
  (epoch 861) was healthy with zero active Super rows.
