# M4 deployed proof — Jev transport: live VM→gateway→OpenRouter round-trip

**Date:** 2026-09-30 · **Station:** `choir-signal-jev-transport-2026-09-29` ·
**Build:** `2404e7d2` on staging (`choir.news`) · **Method:** `POST
/provider/v1/judgments` against the deployed `go-choir-gateway` (host loopback
`127.0.0.1:8084`) using persisted per-VM guest bearer tokens from
`/var/lib/go-choir/vm-state/<vm>/persist/gateway-token`.
**Mutation class:** red (gateway provider route + bearer registry).

## Request/response identity

```
POST http://127.0.0.1:8084/provider/v1/judgments
Authorization: Bearer <fleet VM gateway-token>
{"state":{"hypothesis":"t","evidence":["x"],"outcome":"p"},
 "questions":{"q":{"type":"choice","instructions":"i",
   "criteria":{"yes":"a","no":"b"},"choices":["yes","no"]}}}
```

**Response — HTTP 200, pinned model, typed distribution:**

```json
{"model":"typesafe/jev-1.13-20260917",
 "answers":{"q":{"type":"choice","choice":"no",
   "probabilities":{"no":0.62,"yes":0.38},"confidence":0.24}},
 "usage":{"input_tokens":331,"output_tokens":31,"cost":0.000013902},
 "id":"gen-dec-1790752546-goXXmYQShuRvCu08QXrl","provider":"TypeSafe"}
```

The pinned `typesafe/jev-1.13` model is enforced server-side — the caller cannot
select a model. Response carries a typed decision distribution, usage/cost, and a
provider idempotency id.

## Refusal probes

| Probe | Observed | Contract |
|---|---|---|
| missing `Authorization` | **401** | unauthenticated refused |
| fleet bearer via `localhost` (peer-IP mismatch) | **403** | `BindJevPeer` — bearer bound to the VM's peer route; wrong route is not authorized |
| VM-A bucket exhausted (>60/min) | **429** `Retry-After` | per-`computerID:judgments` bucket trips |

The `BindJevPeer` check binds a VM's bearer to its observed peer address. A token
presented from a different route (the cross-VM case) is refused 403 — this is the
per-VM isolation mechanism the agent's earlier in-VM A/B run also observed as 403.

## Independent rate buckets

- Bucket key: `rateLimitBucketKey(computerID, "judgments")` = `computerID:judgments`
  (internal/gateway/handlers.go:1002).
- VM-A `candidate-fleet-e15cb89f25d963c220319b7b`: calls 1–59 → 200; call 60 → 429
  (default `GATEWAY_RATE_LIMIT_MAX_REQUESTS` = 60 / 60s window).
- VM-B `candidate-fleet-49ee3bd0ec6f366a164c02d2`: **200** immediately after VM-A
  tripped — its bucket is independent of VM-A's exhaustion.

## Rollback path

- `GATEWAY_JEV_JUDGMENTS_ENABLED=0` in `/var/lib/go-choir/gateway-provider.env` +
  `systemctl restart go-choir-gateway` → `HandleJudgment` returns 503
  (`jevTransport == nil || !Enabled()`), settling in-flight callers with a clean
  refusal. Already-emitted receipts are durable and unaffected.
- Retire the station-added `OPENROUTER_API_KEY` + per-VM `gateway-token`
  provisioning for the judgments scope; `deploy-provider-creds.sh` still carries
  them for other routes.
- Git revert `internal/gateway/jev.go` route registration (handlers.go:1062) +
  `gatewayruntime/jev.go` client if the transport itself is abandoned.

## Residuals

- The per-VM bearer↔peer-IP `BindJevPeer` is registry-bound; a host-restart
  re-binds on first request (the runtime reconciles the token-hash store after
  gateway restart, `internal/vmmanager/manager.go:502`).
- The gateway writes the authoritative request/distribution receipt internally;
  this doc records transport status + returned distribution only.
