# Fresh-computer prompt-bar submit fails: `invalid genesis` on projection-batch append

**Status:** DIAGNOSED 2026-09-28 — not a new regression; a gate bypass on a
pre-genesis computer. Discovered while running the M11-fix acceptance probe
(`scripts/texture_revision_cadence_probe.mjs`) against deployed `54c96c98`.

## Symptom

Every fresh-computer prompt-bar submission returns HTTP 500:

```
texture prompt bar: submit: persist agent: store: append projection batch:
  invalid computer event transition: invalid genesis
```

- VM `vm-fceb9551589a968667b76a4bb6606202` — 2026-09-28 05:25:03
- VM `vm-f9a5dc9aa2d137136a1485d366c0a552` — 2026-09-28 05:25:26

## Root cause (confirmed)

Fresh computers are pre-genesis by design: the canonical chain starts only
when the owner POSTs `/api/computers/{id}/lifecycle/bootstrap-chain`
(`BootstrapChain`, internal/agentcore/chain_bootstrap.go:39 — also the
`choir computer bootstrap-chain` CLI and, for selfdev, the signed genesis
handler in api_self_development.go:526).

Run admission *is* gated on pre-genesis (`createRunWithMetadata`,
internal/agentcore/runtime.go:827 → "computer is pre-genesis: run admission
refused (bootstrap-chain required)"). But the prompt-bar submit path calls
`completePromptBarDecisionRun` (runtime.go:907), which persists the agent
record via `UpsertAgent` directly — **no admission gate**. The first write
is therefore a projection-batch append bound to a nil head → genesis shape
→ `reduceGenesis` refuses → 500.

So this is two defects, not one:

1. `completePromptBarDecisionRun` bypasses the pre-genesis admission gate —
   an invariant hole independent of this probe (the same bypass would fire
   for any owner submitting to an unbootstrapped computer).
2. `scripts/texture_revision_cadence_probe.mjs` never bootstrapped — it
   registered a fresh owner and submitted immediately. Fixed in this commit:
   the probe now POSTs `lifecycle/bootstrap-chain` before submit (the same
   step `m11_selfdev_episode_probe.mjs:386` has always done).

## Why it looked new

Yesterday's probe VMs ran on computers whose chains were already genesis'd
by earlier selfdev ops — the cadence probe's fresh-owner fresh-computer
combination is what exposed the bypass. The last prior run of this probe
against a fresh computer predates the projection-batch cutover
(`92149d06`, 2026-08-16) or ran against a bootstrapped computer.

## Fix direction (decided)

- `completePromptBarDecisionRun` needs the same pre-genesis admission check
  as `createRunWithMetadata` (or share the gate so every run-admitting path
  enforces it). Left as follow-up — the probe's explicit bootstrap is the
  correct product path; the gate bypass should produce the documented
  "bootstrap-chain required" refusal, not an opaque 500, but that is an
  error-shaping repair, not the blocker.

## Evidence

- Node B journal (vmctl exec), 05:25:03 and 05:25:26, both probe VMs.
- Source: `bindCurrentHeadLocked` (internal/computerevent/appender.go:565)
  binds nil-head appends to sequence-1 zeroed fields; `reduceGenesis`
  (internal/computerevent/reducer.go:150) requires `EventGenesisImported`.
