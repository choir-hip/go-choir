# Mission Ledger: Texture As A Long-Running Agent v0

Parallax ledger for `docs/mission-texture-long-running-agent-v0.md`. The paradoc
holds current Parallax State; this ledger records dated checkpoints, evidence,
and conjecture/heresy deltas.

## 2026-06-17 - Paradoc created (green)

Created the paramission from the deployed-fix falsification recorded in
`docs/mission-texture-product-loop-recovery-v0.ledger.md`. The owner-selected
direction after that falsification is to invert the core invariant: make the
Texture agent `texture:<docID>` a single long-running logical actor that writes an
immediate from-weights V1 and then deepens the document across many canonical
revisions as evidence streams in.

Belief state at creation:

- Confirmed by code review: one write per run is a hard DB-backed block
  (`tools_texture.go:566-573`, `store/texture.go:1753-1776`); Texture warm
  injection is disabled (`super_controller.go:434-436`, shipped in `68d09cc3`);
  the "persistent super" pattern is wake-driven ephemeral runs, not an immortal
  run; there is no park-and-wait primitive, a bare `maxToolLoopIterations=200`
  ceiling, no cumulative cost budget, and doc-delete does not cancel runs
  (`texture.go:1048-1061`).
- Confirmed by staging probe: prior increment `68d09cc3` deployed (staging health
  + sandbox both on the SHA) and produced V1-only at ~49s - necessary but not
  sufficient, and it reinforced the cap by disabling warm injection.

Mutation class of this checkpoint: green (docs only). Planned execution is red.

Remaining error / open edges: cost/runaway and cancellation are the top risks of
a long-lived actor; the budget kill-switch and the doc-delete->cancel gap must
close with the lifecycle change. The collapse of many revisions into one run must
keep trajectory/work-item attribution and the per-revision supervision narrative
legible (verifier and Trace at N:1).

Next move: hand the goal string to Codex to one-shot as far as it can safely
prove; then critically review (codex review + reading + deployed staging probe)
and iterate. Codex must leave precise, file-cited blockers for unfinished ramp
items.

Lineage: supersedes/folds in `mission-texture-product-loop-recovery-v0`. The
`68d09cc3` injector change is reverted/superseded by T1.

## 2026-06-17 - Read-only T1 audit before runtime mutation (red planned, green checkpoint)

Claim under test: the current source tree still carries the superseded
one-write-per-run cadence workaround rather than the long-running Texture actor
specified by this paradoc.

Move: probe. Read the current Texture runtime, prompt overlay, mutation store,
and cadence tests before editing protected code.

Expected ΔV: -0 implementation, +1 observer evidence packet. Actual ΔV: no ramp
item landed, but the route is now precise enough for the T1 construct.

Receipts:

- `internal/runtime/super_controller.go:417-438` still returns `nil` for Texture
  warm injection and explicitly says Texture consumes only a cold-prepended
  batch, one canonical revision per run.
- `internal/runtime/tools_texture.go:549-576` still rejects a second
  `patch_texture`/`rewrite_texture` in the same run once the run's
  `texture_agent_mutations` row is no longer `pending`.
- `internal/runtime/textureprompts/overlays/run_system.yaml:12` still instructs
  Texture not to write again in the same revision run after a write succeeds.
- `internal/runtime/texture_controller.go:25-44` implements leading coalesced
  wakes, which was a useful prior repair but preserves separate revision runs
  instead of making `texture:<docID>` one resident logical actor.
- `internal/runtime/texture_test.go:1433-1468` and
  `internal/runtime/texture_test.go:7431-7510` encode the old expectations:
  Texture warm injection disabled and second same-run writes rejected.

Open edge: the mutation row currently carries a single `revision_id`, so T1
needs a narrow replacement for duplicate-write protection that does not recreate
the one-write cap. Candidate repair: keep `texture_agent_mutations` as run
liveness/idempotency state, stop completing it on each write, and let each
canonical revision carry `loop_id`, parent revision, operation metadata, and
worker-update consumption metadata as the per-revision commit record.

Next move: documentation-first checkpoint commit, then T1 runtime construct.

## 2026-06-17 - T1/T2 local construct (red runtime, not settled)

Claim under test: Texture can stop being capped at one canonical write per
revision run without losing stale-write safety, and an initial appagent revision
can honestly be marked as model-prior/interim before worker evidence arrives.

Move: construct. Re-enabled Texture warm update injection and replaced the
per-write completed mutation gate with a pending run-liveness mutation plus
per-revision metadata.

Expected ΔV: land T1 locally and part of T2; leave T3-T8 blocked by file-cited
remaining architecture work. Actual ΔV: T1 locally repaired, T2 metadata/prompt
policy locally repaired, settlement still open.

Receipts:

- `internal/runtime/super_controller.go:424-438` no longer excludes Texture from
  warm coagent update injection.
- `internal/runtime/tools_texture.go:564-688` keeps the Texture mutation pending
  across canonical writes, records the latest revision after each write, and
  advances worker-update delivery only through the consumed message sequence.
- `internal/store/texture.go:1744-1790` adds
  `RecordAgentMutationRevision`, preserving the mutation row as run
  liveness/idempotency state until run completion.
- `internal/runtime/runtime.go:2655-2689` completes a written Texture mutation at
  run completion and marks no-write completions so they do not immediately
  re-wake the same pending work forever.
- `internal/runtime/runtime.go:2868-2926` marks no-worker appagent revisions as
  `model_prior_interim` / `revision_grounding=model_prior`.
- `internal/runtime/runtime.go:2959-3072` computes the consumed worker-update
  sequence from injected `worker_update_ids` and emits per-revision consumption
  metadata.
- `internal/runtime/textureprompts/overlays/revision_policy.yaml:22-48` and
  `internal/runtime/textureprompts/overlays/run_system.yaml:12-25` allow a fast,
  explicitly uncertain model-prior scaffold while preserving the no-grounded
  facts-from-recall invariant.

Verification:

- `nix develop -c go test ./internal/runtime -run 'TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce' -count=1`
- `nix develop -c go test ./internal/store -run 'TestTextureAgentMutation' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestCoagentUpdateTurnInjectorSupportsTexture|TestTextureAgentRevisionMutationCompletedOnlyOnce|TestBuildAppagentRevisionMetadataCarriesForwardDurableKeys|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestTexturePromptInitialRevisionUsesSingleWriterLoop|TestTexturePromptForFactualFirstRevisionForbidsUngroundedContent|TestResearcherFailureSynthesizesCheckpointAfterSearch|TestRunSupportsCoagentUpdateInjectionIncludesTexture|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite' -count=1`
- `nix develop -c scripts/go-test-runtime-shards`

Open blockers / remaining error:

- T3 remains open: no uniform park-and-wait primitive and no cumulative
  per-actor budget/kill switch. The loop is still bounded by
  `maxToolLoopIterations=200` in `internal/runtime/toolloop.go:203-209`.
- T4 remains open: `internal/runtime/texture_controller.go:24-90` still contains
  separate wake/reconcile scaffolding. This construct proves multi-revision
  capability inside one run but does not yet make the doc actor a parked
  resident lifecycle.
- T5 remains open: restart passivation still marks pending Texture mutations
  stale in `internal/runtime/runtime.go:1261-1302`; a real sleep/resume model
  must preserve a multi-write actor safely across vmctl refresh.
- T6 remains open: document delete still only calls `DeleteDocument` and does
  not cancel the resident actor in `internal/runtime/texture.go:1048-1060`.
- T7 remains open: the verifier still checks per-revision causality but not the
  full one-run-to-many-revisions lifecycle in
  `internal/runtime/texture_workflow_verifier.go:527-593`; heresy detector docs
  have not been updated.
- T8 remains open: no CI/deploy/staging cadence proof or RunAcceptanceRecord has
  been produced for this partial construct yet.

Rollback ref: revert this runtime construct commit after the prior checkpoint
commit `54b71842` if the local proof falsifies in staging or review.
