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

## 2026-06-17 - Staging proof falsified full cadence settlement (red evidence, green record)

Claim under test: the local T1/T2 construct is enough to show faster
from-weights first paint and multiple grounded V2+ revisions on staging.

Move: deployed probe. Pushed runtime construct commit
`8dbdd4585417974bc2dd3f3d07b9c5ad58af542b`, monitored GitHub Actions via the
public API, checked staging health identity, then ran the product-path cadence
probe against `https://choir.news`.

Receipts:

- GitHub Actions for `8dbdd4585417974bc2dd3f3d07b9c5ad58af542b`: Docs Truth
  Check #191 success; FlakeHub #876 success; CI #1294 had all test/build jobs
  success, but the `Deploy to Staging (Node B)` job concluded failure at step
  "Deploy to staging". Public API access did not allow fetching private deploy
  logs (`403`).
- Staging `/health` nevertheless reported both proxy and sandbox on
  `8dbdd4585417974bc2dd3f3d07b9c5ad58af542b`, `upstream=ok`,
  `vmctl_status=ok`, `deployed_at=2026-06-18T01:50:01Z`.
- `nix shell nixpkgs#nodejs_22 -c env CHOIR_DEPLOYED_BASE_URL=https://choir.news node scripts/texture_revision_cadence_probe.mjs`
  ran through the public product path. Submission:
  `d33edcc6-7f05-43af-b3a8-063679d68a5e`; doc:
  `893bcb64-d82e-4c99-856e-93d7d97e2f06`.
- Probe result: V0 user at +0.328s; V1 appagent at +47.057s; no V2+ appagent
  revisions; `appagent_revision_count=1`; `total_revision_count=2`;
  `final_head_chars=2483`.
- Trace activity existed despite the single revision:
  `web_search=6`, `source_search=2`, `spawn_agent=2`, `update_coagent=2`,
  `moment_count=120`, `search_attempt_count=36`, `search_success_count=12`,
  `agent_count=3`, `delegation_count=1`.

Result: full mission settlement is falsified. T1/T2 local mechanics are still a
valid partial repair, but staging shows they are not sufficient for fast
from-weights first paint or multiple visible grounded revisions. The next
conjecture is that prompt/tool-loop behavior still waits for research before the
first write and terminates after the first incorporated packet; T3/T4 must add a
real park-and-wait/resident lifecycle, and T2 may need stronger first-tool /
first-write scaffolding to force an honest interim V1 before Probe work.

Next move: documentation-first checkpoint commit for this falsification, then
investigate the deployed trajectory events for why the first write waited 47s
and why warm `update_coagent` activity did not yield V2+ before trajectory
completion.

## 2026-06-17 - Exact first-write T2 repair (red runtime, staging pending)

Claim under test: the `8dbdd458` construct still let Texture choose a terminal
tool before any canonical revision because the initial tool choice was only
`"required"`. If the first Texture turn is constrained to `patch_texture`, V1
should publish before researcher/super delegation, and subsequent integrate
wakes should write V2 from the delivered findings before they can terminate.

Move: construct. Changed initial Texture tool selection from "some durable tool"
to exact first `function:patch_texture` for initial revision runs and
`update_coagent` integrate wakes. The next provider call after the write remains
unconstrained, preserving Texture's ability to delegate, request super
execution, record a decision, hand off email, or end after publishing the
interim revision.

Expected ΔV: repair the staging-observed late-V1 mechanism without adding a
Texture-specific tool-loop branch; leave T3/T4 long-running actor lifecycle
open. Actual local ΔV: first-write ordering is mechanically proven in focused
runtime tests and the shard suite; deployed proof is still pending.

Receipts:

- `internal/runtime/runtime.go:2214-2229` now returns
  `function:patch_texture` for initial Texture revision tasks and
  `request_source=update_coagent` wakes, while scheduled non-coagent runs remain
  unconstrained.
- `internal/runtime/texture_test.go` now asserts the first Texture tool
  definition set contains only `patch_texture`, and the write+research fixture
  proves write-first / delegate-second ordering.
- `internal/runtime/texture_prompt_unit_test.go` now covers the exact
  first-patch policy for ordinary prompts, product work, proof prompts, creative
  document work, explicit decision notes, and update-coagent wakes.
- `internal/runtime/prompt_bar_unit_test.go` now records the product-route
  expectation that explicit no-worker decisions still write before delegating.

Verification:

- `nix develop -c go test ./internal/runtime -run 'TestTextureAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit|TestInitialTextureRunWritesBeforeSpawningResearcher|TestInitialTextureToolChoiceRequiresPatchBeforeContinuation|TestHandlePromptBarOperationalProofInitialRunStartsWithTexture|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureCreatedSuperEvidenceWakesTextureV2|TestInitialTextureRunDefaultsMinimalEditContextFromActivation|TestSubmitResearchFindingsWakeUsesSameDebouncedPath|TestHandlePromptBarExplicitNoWorkerDecisionStartsWithTexture' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TestTexturePromptInitialRevisionUsesSingleWriterLoop|TestTexturePromptForFactualFirstRevisionForbidsUngroundedContent|TestResearcherFailureSynthesizesCheckpointAfterSearch|TestRunSupportsCoagentUpdateInjectionIncludesTexture|TestHandlePromptBarExplicitNoWorkerDecisionStartsWithTexture' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestCoagentUpdateTurnInjectorSupportsTexture|TestTextureAgentRevisionMutationCompletedOnlyOnce|TestBuildAppagentRevisionMetadataCarriesForwardDurableKeys|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit' -count=1`
- `nix develop -c go test ./internal/store -run 'TestTextureAgentMutation' -count=1`
- `nix develop -c scripts/go-test-runtime-shards`

Open blockers / remaining error:

- Staging must still prove the provider honors exact `function:patch_texture`
  and does not relax to "any required tool" through an adapter fallback.
- T3/T4 remain open: this repair can yield fast V1 and wake-driven V2+, but it
  still does not provide a role-uniform park-and-wait primitive, cumulative
  actor budget, or one resident `texture:<docID>` lifecycle.
- If staging still shows late V1 or V1-only, the next documentation-first record
  should capture whether exact tool choice was relaxed, `patch_texture` failed,
  the post-write continuation ended before delegation, or integrate wakes failed
  to consume findings.

Rollback ref: revert this exact-first-write runtime commit and return to the
documented `8dbdd458` partial construct if staging or review shows the exact
first patch policy breaks valid Texture entry paths.

## 2026-06-17 - Staging falsified exact first-write repair (red evidence, green record)

Claim under test: exact initial `function:patch_texture` would force a fast V1
before terminal delegation and allow later V2+ integrate wakes.

Move: deployed probe. Pushed runtime construct commit
`7d462629ca4a5df9b3df3c7b7a707742a8e5b6eb`, monitored GitHub Actions, checked
staging health identity, then ran the product-path cadence probe against
`https://choir.news`.

Receipts:

- GitHub Actions for `7d462629ca4a5df9b3df3c7b7a707742a8e5b6eb`: Docs Truth
  Check #193 success; FlakeHub #877 success; CI #1295 had all test/build jobs
  success, but `Deploy to Staging (Node B)` concluded failure. Public API access
  to deploy logs still returned `403`.
- Staging `/health` nevertheless reported both proxy and sandbox on
  `7d462629ca4a5df9b3df3c7b7a707742a8e5b6eb`, `upstream=ok`,
  `vmctl_status=ok`, `deployed_at=2026-06-18T02:11:15Z`.
- `nix shell nixpkgs#nodejs_22 -c env CHOIR_DEPLOYED_BASE_URL=https://choir.news node scripts/texture_revision_cadence_probe.mjs`
  ran through the public product path. Submission:
  `42fb44c0-a0f0-43c2-a883-6c85a007eb8c`; doc:
  `f01de6d5-c638-414c-8232-db483469da2f`.
- Probe result: V0 user at +0.267s; no appagent revisions;
  `appagent_revision_count=0`; `first_paint_ms=null`; `total_revision_count=1`;
  `final_head_chars=53`.
- Trace summary: `web_search=0`, `source_search=0`, `spawn_agent=0`,
  `update_coagent=0`, `moment_count=29`, `agent_count=2`,
  `delegation_count=0`, trajectory `state=completed`, `live=false`.

Result: exact first-write is locally proven but product-path falsified. The
failure mode changed from late V1 after researcher wake (`8dbdd458`) to no
appagent revision and no delegation at all (`7d462629`). This suggests the live
provider/adapter path may reject or ignore exact `function:patch_texture`, hit a
provider precondition fallback that relaxes or completes without the required
tool, or otherwise let the initial Texture run complete without a canonical
write despite the local stub tests.

Expected ΔV: staged proof of fast V1 and V2+. Actual ΔV: negative for product
path, but useful observer evidence: the first-write obligation must be enforced
in a way compatible with live provider tool-choice semantics.

Next move: before any code fix, inspect live-compatible tool-choice handling and
provider precondition fallback. A candidate repair must prove that an initial
Texture run cannot complete without a canonical write, while preserving
post-write delegation and avoiding exact-tool behavior that collapses the live
provider path.

Rollback ref: revert `7d462629` if the next repair cannot preserve exact
first-write semantics safely. This docs checkpoint is the required
problem-documentation-first commit before the next code change.
