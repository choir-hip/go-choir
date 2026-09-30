# M1 ActingPack isolation + legacy grandfathering — verification (2026-09-30)

Objective: M1 items 2 and 3 — (2) the ActingPack injected into an acting cell's
frame carries no score fields; (3) pre-typed (legacy) commitment records stay
`Prediction.Hypothesis`-only and unscoreable (grandfathered), while remaining
readable.

Deployed build under test: `build.commit = 37882e1d` (guest epoch 979,
computer-03335285269bdba4f94377e56879f9e6). The typed-commitment tape leg
completed on this build: run `run:assignment-062f03d9-0e02-5ccf-b1c8-7933561387de`
(completed 16:54Z) staged `choir.Precommit` + `choir.Resolve` rows via one
`capsule_go_eval` cell — typed `commitment_record` objects now exist on the
deployed tape.

## Item 2 — ActingPack carries no score fields: VERIFIED (type-level + wire)

The boundary is compile-time, not convention:

- `DeskSessionWorker.EvalCell` accepts `pack *types.ActingPack`
  (`internal/yaegikernel/desk_session_worker.go:294`). A `SupervisionPack`
  (the score-carrying variant) cannot be passed without a type error.
- The only construction seam is `actingCommitmentPackForDesk`
  (`internal/agentcore/tools_desk.go:462`) → `types.BuildActingPack` →
  injected as `SessionFrame.Pack` (line 189). `BuildSupervisionPack` is
  defined (`commitment_views.go:485`) but referenced only from `_test` —
  nothing injects it into a desk frame.
- `ActingPackItem` (`commitment_views.go:84-`) has fields
  `record_id, claim, discrepancy, observation_excerpt, resolved_at` — no
  score/distribution/disagreement keys exist to populate.

Test pin: `TestActingPackCarriesNoScoreFields`
(`internal/types/commitment_views_test.go:139`) marshals a built pack and
asserts the JSON carries none of `"scores"`, `"scored_at"`,
`"scorer_model_id"`, `"distribution"`, `"probabilities"`, `"disagreement"` —
the epistemic boundary holds on the wire, not just in the struct list.

## Item 3 — legacy records grandfathered unscoreable: VERIFIED (code level)

- `types.CommitmentRecord` (`commitment.go:161-163`): older bodies have nil
  typed fields and retain `Prediction.Hypothesis` unchanged; "a typed Resolve
  may target that legacy record, but it remains unscoreable because no
  distribution is [present]".
- Scoring requires a frozen `Distribution`; `typedCommitmentPrecommit`
  (`rlm_reduce.go:269-283`) rejects a precommit body with empty
  `Distribution`. Legacy rows carry `Prediction.Hypothesis` only — no
  `Precommit`/distribution — so no scorer verdict can attach.
- Readability preserved: `isResolutionRecord` (`commitment_views.go:148-156`)
  recognizes a legacy resolution by "established link plus ResolvedAt stamp"
  when `rec.Resolve` is nil; `commitmentClaimText` falls back to
  `Prediction.Hypothesis` for the claim. Legacy rows join `ResolveCommitments`
  and enter `BuildActingPack` normally — grandfathered, not orphaned.

## Residual — deployed-tape confirmation pending

The code+test proof is complete; the *deployed* confirmation (reading the
actual `commitment_record` OG objects staged by run `062f03d9` to show a real
ActingPack JSON and a real legacy row unscoreable) requires a guest-internal
read of the Dolt OG store, which is not reachable via the host API surface
(`choir lifecycle`, `run status`, texture routes expose trajectories/events,
not OG objects). The readable deployed evidence — the run completed and
staged typed intents — is already on the tape; the pack/legacy content
assertion is proven by the type boundary + pinned tests, which is the
admissible evidence class for these two inspection items.

Blocked companion items sharing the live-guest gate: M-SUB deployed proof and
M0a proof wait on run `362febb2` (researcher `cfa90b87`) reactivating — its
`coagent_result` wake was re-armed by the 23:16 migration (`minted 1778`) but
is queued behind a serial drain racing the platform-dolt OOM reboot cycle.
