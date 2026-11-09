# Provider retry audit, 2026-11-09

Owner, 2026-11-09: "the pools ... need to be intelligent enough that if a
particular provider is not working, we don't keep retrying it. We should be
doing exponential back off and having some kind of memory. This just needs to
be like a system-wide policy to always be intelligent about not retrying."

Method: read the code paths (not traces); every claim below is a code reading,
a hypothesis about production behaviour until a ledger confirms it. Columns:
M = consults a shared availability memory before calling; E = backs off
exponentially on failure; R = memory survives a process restart; H = a hard
failure (401/403/404) is held longer than a transient one; F = something
retries on a fixed beat regardless of memory.

## Outbound call paths

| Path | M | E | R | H | F | Fix in this change |
|---|---|---|---|---|---|---|
| Tool loop LLM calls, guest (`toolregistry.RunToolLoop`, both sites: desk runs in `agentcore/runtime.go`, research children in `research_children.go`): QuotaLedger | yes (per computer) | yes, 1 min doubling to 15 min; quota holds to the provider reset | no | no | yes: the every-model incident `Forget`s the whole chain each round and re-asks | persisted (a), incident keeps future holds (b), hard holds (c) |
| Tool loop transient retries per call (`callToolLoopProviderWithPoolRetries`) | no (per call, bounded by attempts) | rate-limit backoff | n/a | no | bounded | none; bounded per call and the ledger sits above it |
| Gateway per-model breaker (`gateway/circuit_breaker.go`) for every gateway LLM call | yes (per model, per gateway) | yes, doubles per failed probe to 10 min; quota 5 min to 1 h | no (gateway restart closes all) | no: 4xx other than 408/429 counted nothing, so a refused key was called at full rate | no | hard failures open at once, 15 min doubling to 6 h (e) |
| Non-tool-loop LLM calls on a computer: intake labels, support judge, candidate judge, frontimportance scorer (all through `gatewayruntime.JudgmentClient` to `/provider/v1/judgments` -> `gateway/jev.go` -> OpenRouter Decisions) | no: Jev has no breaker; each caller times out on its own | no | no | no | per-caller fixed schedules | gateway-side `upstreamHold` on the Jev transport (d): 1 min doubling to 15 min, hard 15 min to 6 h, success clears, 503 + Retry-After while held |
| Model-policy verifier `modelpolicy/tools_model_verify.go` (CallWithTools) | no | no | no | no | one shot per verify | residual (another builder owns modelpolicy) |
| Search providers (Brave, Exa, Tavily, Serper, Keenable, Parallel, NewsAPI, SerpAPI) via `gateway/searchplane` | yes (SQLite `FileHealthStore`) | yes (`BackoffPolicy`, per outcome class, 2x) | yes (SQLite) | yes: auth_error 15 min doubling to 6 h | no | none needed |
| Claude delegate (`claudedelegate`) | no | no | no | no | per run | residual (another builder owns it) |
| Source polls (`sources`, `sourcefetch`, source schedules) | no shared provider memory; the schedule tick is the cadence | no | schedule survives (`recoverSourceSchedulesAtBoot`), failure memory does not | no | the tick | residual |
| Mail (Resend), embeddings/blob/host calls (`maild`, `hostfetch`, `vmctl`) | not LLM/search providers | no | no | no | n/a | out of scope |

## Gaps closed here

a. QuotaLedger persisted to `<store>.provider-availability.json` (holds,
   sources, strikes; load at boot, atomic rewrite on change, 64 members max).
b. The every-model incident no longer `Forget`s held members; after its wait
   only holds that ended are dropped, so a member whose reset is ahead is not
   re-asked.
c. Hard failures (401, 403, 404, model not found) hold 15 min doubling to
   6 h in the ledger and in the gateway breaker; a success ends the hold.
   No gateway credential reload signal exists in the tree, so a rotated key
   is seen at the next probe (at most 6 h, usually less).
d. Jev (all non-tool-loop LLM judgments) is behind a gateway hold.
e. The gateway breaker already doubled on non-quota failures; a test now
   pins it, and hard failures were added.

## Residuals

- The gateway breaker and the Jev hold are in memory: a gateway restart
  re-probes each dead upstream once.
- Claude delegate, model-policy verifier and source polls have no failure
  memory.
- No credential reload signal clears a hard hold early.
- When every pool member is held and the incident wait is cut by its bound,
  the run still makes one probe call of the active member per round.
