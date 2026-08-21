# Staging Evidence: Assigned CoSuper Model Policy and Provider Fallback Recovery

- Date: 2026-08-21
- Mutation class: Red
- Computer: `computer-03335285269bdba4f94377e56879f9e6`
- Source repair: `3704dd789cd9a2099ddb94f3a20fcaae1cf6a718`
- CI: `32433080337`
- Staging deploy: Node B succeeded; `/health` reported `3704dd789cd9a2099ddb94f3a20fcaae1cf6a718` at `2026-08-21T00:56:10Z`

## Problem

Assigned CoSuper runs created by `startAssignedCoSuperForParent` constructed
`RunRecord.Metadata` directly and activated the run without the model-policy
enrichment used by the other run-creation paths. Their public metadata therefore
had empty `llm_provider`, `llm_model`, and `llm_policy_source` fields. The host
gateway defaulted the first provider call to DeepSeek
`deepseek-v4-flash`, which returned HTTP 402 Payment Required.

The tool loop already classified HTTP 402 as a provider-availability failure,
but `ProviderPreconditionFallbackSelections` returned no candidates when the
primary selection had an empty model. The availability error consequently
terminated the CoSuper at tool-loop iteration 0 instead of trying an active
alternative.

Observed pre-repair staging CoSuper runs repeatedly ended with:

```text
tool loop iteration 0: gateway call failed: gateway client: deepseek:
status 402 Payment Required (sanitized)
```

Their public run metadata had no provider or model selection.

## Repair

`3704dd78` now:

1. enriches assigned CoSuper metadata with `modelpolicy.Manager.EnrichMetadata`
before the durable assignment bind and actor activation; and
2. returns the terminal platform fallback `chatgpt/gpt-5.6-luna` even when the
primary selection is empty, preserving a recovery path for provider-availability
errors from a gateway default selection.

Focused proof passed locally:

- `go test ./internal/modelpolicy/... -run 'TestProviderPreconditionFallback'`
- `go test ./internal/toolregistry/... -run 'TestRunToolLoopTriesMultipleProviderPreconditionFallbacks|TestRunToolLoopTriesProviderPrecondition'`
- `go test ./internal/agentcore -run 'TestSelfDevelopmentPersistentSuperPassesAssignCoSuperGate'`
- `go test ./internal/agentcore -run 'TestPersistentSuperContinuesFromCoSuperSystemCancellation|TestPersistentSuperRewakeReceivesPendingCoSuperCancellationReports'`

CI run `32433080337` passed the selected race shards, build/vet, SBOM, and
staging deploy jobs.

## OAuth recovery boundary

The local ChatGPT OAuth record was already refreshed before this repair:
`last_refresh=2026-08-20T04:27:53.537924Z`. The local
`~/.codex/auth.json` and Node B `/var/lib/go-choir/codex-auth.json` had the same
SHA-256 (`8c00337941acc9f0ce250112ca93abcc32b7f32753ca972c5297aba77457f230`),
without exposing token values. Gateway calls to ChatGPT
`gpt-5.6-luna` succeeded repeatedly after deployment, proving OAuth was not the
remaining 402 cause.

The operator procedure is documented in `docs/README.md` and the comments of
`nix/deploy-provider-creds.sh`. That helper copies an already-refreshed local
OAuth file; it does not refresh OAuth itself, and it also regenerates the broad
provider EnvironmentFile.

## Staging verification boundary

The owner-scoped refresh request used idempotency key
`model-fallback-refresh-2026-08-21T0057Z`. The request returned HTTP 502 while
vmctl was restarting, but the product status afterward reported the retained
computer active at realization epoch 347 with runtime `ready`. Node B vmctl logs
showed the old event appender temporarily unavailable during the service restart
and then a route-authorized primary VM reattach.

After the refresh, persistent Super run
`faed9f4f-1140-44a3-b3a6-32bae0712264` remained `pending`. Boot logs recorded
that its lifecycle spawn work was skipped with empty `trajectory` and
`requested_by` fields, followed by passivation of the stale run. No new
post-repair CoSuper has entered the tool loop, so this receipt does **not** claim
live fallback acceptance. Existing reactivation evidence remains applicable:
`effects-red-passivated-super-missing-trajectory-blocks-reactivation-2026-08-20.md`.

Effects remain OFF. Candidate authorship, freeze, qualified consensus,
promotion, play, falsification, supersession, and restore remain unpaid.

## Rollback

- Platform rollback: revert `3704dd78` through `origin/main` and the normal CI/deploy loop.
- Computer rollback: pre-A checkpoint `99949fe2` remains the immutable restore fence.
- OAuth rollback: restore the prior root-owned mode-0600 Node B auth file only through the documented credential procedure; no token values are retained here.
