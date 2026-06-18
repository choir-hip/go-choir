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

## 2026-06-17 - End-turn exact-tool retry repair (red runtime, staging pending)

Claim under test: the `7d462629` no-write staging result happened because the
tool loop accepted a provider `end_turn` as normal completion even while an
exact initial tool choice was active. If `end_turn` is retried as a missing
required initial tool, the live provider path should be unable to complete
Texture V1 without calling `patch_texture`.

Move: construct. Tightened the general tool loop, not Texture-specific
choreography: exact initial tool choice now retries on `end_turn` without a tool,
with the same filtered tool definition set. Also settled failed no-write Texture
mutations before reconcile so a failed integrate wake does not spin by
immediately requeueing the same undelivered packet.

Expected ΔV: repair the no-appagent-revision failure mode from `7d462629` while
preserving the first-write-before-delegation order from the previous construct.
Actual local ΔV: the staging-shaped no-tool `end_turn` path is covered by a new
tool-loop regression test; the previously hanging researcher fallback test now
passes; runtime shards pass. Deployed proof remains pending.

Receipts:

- `internal/runtime/toolloop.go:630-655` retries exact initial tool choice when
  the provider returns `end_turn` without the required tool, emits
  `initial_tool_choice`, and preserves the filtered exact-tool definitions for
  the retry.
- `internal/runtime/toolloop.go:504-520` now bounds repeated wrong-tool initial
  tool-choice retries with the same retry ceiling used for required-next-tool
  loops.
- `internal/runtime/runtime.go:3237-3273` completes a failed Texture mutation
  if it had already written a revision, otherwise marks a no-write failure and
  skips immediate reconcile for that terminal no-write failure.
- `internal/runtime/toolloop_test.go` adds
  `TestRunToolLoopExactInitialToolChoiceRetriesEndTurnWithoutTool`, simulating
  the live failure shape: first provider response is `end_turn`, second response
  calls `patch_texture`, final response completes normally.

Verification:

- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoopExactInitialToolChoiceRetriesEndTurnWithoutTool|TestRunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool|TestRunToolLoopRelaxesExactInitialToolChoiceAfterProviderPrecondition|TestRunToolLoopRelaxesExactInitialToolChoiceAfterDeepSeekThinkingToolChoiceError|TestRunToolLoopFallsBackModelAfterRelaxedInitialToolChoicePrecondition' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestTextureAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit|TestInitialTextureRunWritesBeforeSpawningResearcher|TestInitialTextureToolChoiceRequiresPatchBeforeContinuation|TestHandlePromptBarOperationalProofInitialRunStartsWithTexture|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureCreatedSuperEvidenceWakesTextureV2|TestInitialTextureRunDefaultsMinimalEditContextFromActivation|TestSubmitResearchFindingsWakeUsesSameDebouncedPath|TestHandlePromptBarExplicitNoWorkerDecisionStartsWithTexture' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TestTexturePromptInitialRevisionUsesSingleWriterLoop|TestTexturePromptForFactualFirstRevisionForbidsUngroundedContent|TestResearcherFailureSynthesizesCheckpointAfterSearch|TestRunSupportsCoagentUpdateInjectionIncludesTexture|TestHandlePromptBarExplicitNoWorkerDecisionStartsWithTexture' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestCoagentUpdateTurnInjectorSupportsTexture|TestTextureAgentRevisionMutationCompletedOnlyOnce|TestBuildAppagentRevisionMetadataCarriesForwardDurableKeys|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit' -count=1`
- `nix develop -c scripts/go-test-runtime-shards` returned success for shards 0-2 in visible output; `nix develop -c env SHARD_INDEX=3 TOTAL_SHARDS=4 scripts/go-test-runtime-shards` then explicitly passed shard 3/4 to remove ambiguity.

Open blockers / remaining error:

- Staging must still prove fast V1 and V2+. This repair only prevents silent
  no-write completion and reconcile spin; it does not implement T3/T4
  park-and-wait or one resident actor.
- If staging still fails, use Trace `provider_call`, `tool_loop`,
  `provider_tool_choice`, and `initial_tool_choice` events to classify the live
  provider behavior precisely.

Rollback ref: revert this runtime repair commit, then `7d462629`, if the live
provider still cannot satisfy the first-write obligation and the retry path
creates blocked or failed trajectories instead of fast V1.

## 2026-06-18 - Staging falsified end-turn retry as sufficient (red evidence, green record)

Claim under test: the `58f261c8` tool-loop retry repair would convert the
`7d462629` no-write staging result into a fast initial `patch_texture` V1, or at
least expose a bounded retry/failure path instead of silent completion.

Move: deployed probe. Pushed runtime construct commit
`58f261c801f077e37f04ee480905422cbf925b52`, monitored GitHub Actions, checked
staging health identity, then ran the product-path cadence probe against
`https://choir.news`.

Expected ΔV: staged proof of fast V1 or a new live-provider retry/failure
signature. Actual ΔV: no ramp item landed; observer evidence shows the
no-appagent failure repeats even after the end-turn retry repair.

Receipts:

- GitHub Actions for `58f261c801f077e37f04ee480905422cbf925b52`: Docs Truth
  Check #195 success; CI #1296 had all test/build jobs success, including
  internal/runtime shards 0-3, integration smoke, TLA+, and vet/build, but the
  `Deploy to Staging (Node B)` job concluded failure. Public API access to
  deploy logs still returned `403`.
- Staging `/health` nevertheless reported both proxy and sandbox on
  `58f261c801f077e37f04ee480905422cbf925b52`, `upstream=ok`,
  `vmctl_status=ok`, `deployed_at=2026-06-18T02:33:02Z`.
- `nix shell nixpkgs#nodejs_22 -c env CHOIR_DEPLOYED_BASE_URL=https://choir.news node scripts/texture_revision_cadence_probe.mjs`
  ran through the public product path. Submission:
  `08c13c3b-8f80-4567-a4a5-7656dfee16b4`; doc:
  `3d0ccff6-a89a-4a86-af43-c4a6189e9f28`.
- Probe result: V0 user at +0.332s; no appagent revisions;
  `appagent_revision_count=0`; `first_paint_ms=null`; `total_revision_count=1`;
  `final_head_chars=53`.
- Trace summary: `web_search=0`, `source_search=0`, `spawn_agent=0`,
  `update_coagent=0`, `moment_count=29`, `agent_count=2`,
  `delegation_count=0`, trajectory `state=completed`, `live=false`.

Result: the no-appagent staging failure from `7d462629` repeated at `58f261c8`.
The end-turn retry branch may not be reached in the live product path, or the
run may be completed/settled before the provider response shape covered by the
local regression test. The next construct must not guess at another prompt or
tool-loop tweak before inspecting product evidence around activation, initial
tool-choice selection, provider-call events, retry events, tool definition
filtering, and mutation settlement for this submission/doc.

Next move: inspect the deployed trajectory/Trace events for submission
`08c13c3b-8f80-4567-a4a5-7656dfee16b4` / doc
`3d0ccff6-a89a-4a86-af43-c4a6189e9f28` and classify the exact no-appagent path.

Rollback ref: revert `58f261c8`, then `7d462629`, if the next evidence shows the
exact-first-write line is incompatible with the live provider/tool-selection
path rather than only missing an activation or settlement guard.

## 2026-06-18 - Failed initial write retry repair (red runtime, staging pending)

Claim under test: the repeated V0-only staging result is not because Texture was
never activated or exact tool choice was unavailable. It is because the first
exact `patch_texture` batch can call the right tool but fail to store a revision,
then be treated as satisfied by a duplicate non-error notice.

Move: probe + construct. Ran a fresh product-path Trace diagnostic against
deployed `58f261c8`, then repaired the general sequential tool execution and
exact initial tool-choice loop.

Expected ΔV: classify the live no-appagent branch and repair the first-write
fallthrough without adding semantic role choreography. Actual local ΔV: the
staging-shaped branch is covered by focused tests and the runtime shard suite;
deployed proof remains pending.

Diagnostic receipts:

- Trace diagnostic submission `02be18d3-dfa9-4294-9327-4567e1a4b008`, doc
  `76dee478-d2f5-4c73-86ff-781fd9dadfee`, initial Texture run
  `323ea93b-668a-4355-a202-d2f2046f2537`, on staging health identity
  `58f261c801f077e37f04ee480905422cbf925b52`.
- Trace showed Texture activated and provider calls used exact
  `function:patch_texture` with a one-tool `patch_texture` definition after
  Xiaomi and DeepSeek provider-availability fallbacks.
- ChatGPT returned `stop_reason=tool_use` with two `patch_texture` calls. The
  first returned `tool_error: edit 0: find text not present` because it tried to
  replace a fenced prompt block from prompt framing (`--- ... ---`) that was not
  present in canonical V0. The duplicate call returned a non-error duplicate
  notice: `duplicate Texture write tool patch_texture ... one canonical document
  mutation is allowed per revision run`.
- The following provider turn was unconstrained, returned `end_turn` prose
  saying no canonical revision was created, then Texture emitted
  `Texture run completed without storing a Texture revision`.

Repair receipts:

- `internal/runtime/tools.go` now performs Texture duplicate-write suppression
  during sequential execution, after observing that a prior same-turn Texture
  write returned a structured non-error success. A failed first write no longer
  makes later same-turn Texture writes no-ops.
- `internal/runtime/toolloop.go` now treats an exact initial tool choice as
  unsatisfied when the required tool was called but all results were errors,
  appends an initial-tool reminder, emits `loop.retry` phase
  `initial_tool_choice` with reason `required_initial_tool_failed`, and retries
  with the exact initial tool choice still active.
- `internal/runtime/toolloop_test.go` adds
  `TestRunToolLoopExactInitialToolChoiceRetriesFailedRequiredTool`, matching the
  staging shape: duplicate failed `patch_texture` calls, exact retry, then a
  stored revision.
- `internal/runtime/tools_test.go` adds
  `TestExecuteToolsDoesNotSkipTextureEditAfterFailedAttempt`, proving failed
  Texture writes do not suppress later writes in the same turn.

Verification:

- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoopExactInitialToolChoiceRetriesFailedRequiredTool|TestRunToolLoopExactInitialToolChoiceRetriesEndTurnWithoutTool|TestRunToolLoopExactInitialToolChoiceAcceptsDuplicateSameTool|TestExecuteToolsSkipsDuplicateTextureEditsInSameTurn|TestExecuteToolsDoesNotSkipTextureEditAfterFailedAttempt' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestTextureAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit|TestInitialTextureRunWritesBeforeSpawningResearcher|TestInitialTextureToolChoiceRequiresPatchBeforeContinuation|TestHandlePromptBarOperationalProofInitialRunStartsWithTexture|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureCreatedSuperEvidenceWakesTextureV2|TestInitialTextureRunDefaultsMinimalEditContextFromActivation|TestSubmitResearchFindingsWakeUsesSameDebouncedPath|TestHandlePromptBarExplicitNoWorkerDecisionStartsWithTexture|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TestTexturePromptInitialRevisionUsesSingleWriterLoop|TestTexturePromptForFactualFirstRevisionForbidsUngroundedContent|TestResearcherFailureSynthesizesCheckpointAfterSearch|TestRunSupportsCoagentUpdateInjectionIncludesTexture' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestCoagentUpdateTurnInjectorSupportsTexture|TestTextureAgentRevisionMutationCompletedOnlyOnce|TestBuildAppagentRevisionMetadataCarriesForwardDurableKeys|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit' -count=1`
- `nix develop -c scripts/go-test-runtime-shards`

Open blockers / remaining error:

- Staging must still prove this repair yields a visible V1. If V1 appears but no
  V2+ follows, the next open edge is T3/T4: a real park-and-wait / resident actor
  lifecycle rather than wake-driven integrate runs.
- This repair does not implement park-and-wait, cumulative actor budget,
  passivation-as-sleep, doc-delete cancellation, or verifier N:1 settlement.

Rollback ref: revert this failed-initial-write runtime repair commit if staging
shows retry exhaustion, repeated invalid fenced replacements, or a broader
tool-loop regression.

## 2026-06-18 - Staging partially repaired first paint, falsified settlement (red evidence, green record)

Claim under test: the failed-initial-write retry repair in
`29265caeb836b1f975e13ab5497b6f5f40554c1f` would make the product path produce a
fast first appagent V1 and then continue toward grounded V2+ revisions.

Move: deployed probe + Trace diagnostic. Pushed `29265cae`, monitored CI, checked
staging health identity, ran the cadence probe, then ran a targeted Trace
diagnostic because the cadence probe failed V0-only.

Expected ΔV: staged proof of fast V1 and progress toward V2+. Actual ΔV:
partial descent. The retry repair can produce fast V1 in the live product path,
but it is not reliable enough for the formal cadence probe and it does not yet
produce researcher/delegation or V2+.

Receipts:

- CI #1297 for `29265cae`: all test/build jobs passed, including internal/runtime
  shards 0-3, non-runtime Go tests, integration smoke, TLA+, Docs Truth Check,
  and vet/build. `Deploy to Staging (Node B)` concluded failure again.
- Staging `/health` nevertheless reported proxy and sandbox both on
  `29265caeb836b1f975e13ab5497b6f5f40554c1f`, `upstream=ok`,
  `vmctl_status=ok`, `deployed_at=2026-06-18T02:51:42Z`.
- `scripts/texture_revision_cadence_probe.mjs` on staging submitted
  `00523d55-5dee-4a1b-94e6-bb205ea1618d`; doc
  `771fa753-3c8e-4484-a24e-1c333a95271e`. Probe result: V0 only,
  `appagent_revision_count=0`, `first_paint_ms=null`, trajectory `state=failed`,
  no `web_search`, `source_search`, `spawn_agent`, or `update_coagent`.
- Follow-up Trace diagnostic on the same deployed SHA submitted
  `05ddee7c-8ccb-48f9-bc93-1bc313593d2a`; doc
  `9cdf22a3-4ac5-4f4b-81cd-564a76b69c1a`; initial Texture run
  `580a69d4-98ec-405b-98d1-836c7960bd53`.
- The diagnostic showed the intended retry branch: initial exact
  `function:patch_texture` calls failed twice with `append edit requires text`,
  emitted `loop.retry` reason `required_initial_tool_failed`, retried exact
  `function:patch_texture`, then stored appagent revision
  `cf3d52b0-90c5-4ded-8dd3-ca240e0b2f19` at about +16s.
- The successful V1 diagnostic still ended after that one appagent revision:
  trajectory `state=completed`, `delegation_count=0`, `finding_count=0`,
  no researcher activity, and no V2+.

Result: full mission settlement remains falsified. The new live position is more
specific than the previous no-appagent failure: fast from-weights V1 is possible
and the retry branch works, but the actor can still fail before V1 on malformed
edit arguments and, when V1 succeeds, Texture still ends instead of continuing
into a Probe path for factual/current prompts.

Next move: before any further code, keep this docs checkpoint as the problem
record. The next runtime construct should preserve fast V1 and repair the
post-V1 factual-request transition so a model-prior/interim V1 is followed by
researcher/delegation and later grounded V2+, without adding semantic
role-choreography or pretending this is the T3/T4 park-and-wait lifecycle.

Rollback ref: revert `29265cae` if the first-write retry branch is judged too
stochastic or disruptive; otherwise keep it as a partial repair and continue from
the post-V1 no-delegation edge.

## 2026-06-18 - Local completion guard for model-prior V1 (red runtime, staging pending)

Claim under test: the `29265cae` fast-V1/no-delegation branch can be repaired
without making `edit_texture` force a semantic researcher continuation. A
model-prior/interim V1 for a factual/current prompt is not completion; Texture
must either open an evidence path, request execution where appropriate, follow
up with active workers, or record an audit-worthy decision/blocker before ending.

Move: construct. Added a generic tool-loop completion guard hook and wired a
Texture-specific guard outside the core loop for initial factual/current
model-prior heads.

Expected ΔV: local proof that a successful model-prior V1 no longer silently
ends the run when the owner asked for current/world knowledge, while preserving
Texture agency and the no-forced-continuation invariant. Actual local ΔV: the
branch is covered by focused tests and runtime shards; deployed proof remains
pending.

Repair receipts:

- `internal/runtime/toolloop.go` adds `WithCompletionGuard`, a role-uniform
  hook that may reject `end_turn`, append an ordinary user reminder, emit
  `loop.retry` phase `completion_guard`, and retry under a bounded counter. It
  does not select or force a tool.
- `internal/runtime/runtime.go` installs `textureModelPriorCompletionGuard` only
  for Texture revision runs. The guard fires when the current head is an
  appagent `model_prior_interim` / `revision_grounding=model_prior` revision for
  an initial factual/current prompt and the run has not opened an evidence path
  or recorded a Texture decision.
- `internal/runtime/toolloop_test.go` adds
  `TestRunToolLoopCompletionGuardRetriesEndTurn`.
- `internal/runtime/texture_test.go` adds
  `TestTextureModelPriorCompletionGuardOpensProbePath`, proving the product-path
  shape: V1 is stored and flagged model-prior/interim; a premature `end_turn`
  receives the completion-guard reminder; the next unconstrained turn opens a
  researcher Probe path; Trace has a `completion_guard` retry event.

Verification:

- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoopCompletionGuardRetriesEndTurn|TestTextureModelPriorCompletionGuardOpensProbePath' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoopCompletionGuardRetriesEndTurn|TestRunToolLoopExactInitialToolChoiceRetriesFailedRequiredTool|TestRunToolLoopExactInitialToolChoiceRetriesEndTurnWithoutTool|TestRunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool|TestTextureModelPriorCompletionGuardOpensProbePath|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit|TestInitialTextureRunWritesBeforeSpawningResearcher|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureCreatedSuperEvidenceWakesTextureV2|TestInitialTextureRunDefaultsMinimalEditContextFromActivation|TestEditTextureInitialWorkingRevisionDoesNotSmuggleRequiredContinuation|TestEditTextureExplicitResearcherDoesNotForceSpawnContinuation|TestEditTextureExplicitResearcherDoesNotForceSpawnAfterSuperBase|TestEditTextureExplicitResearcherFromBaseRevisionContentSurvivesWorkerPrompt|TestEditTextureExplicitResearcherFromSeedPromptSurvivesRequestIntent|TestEditTextureExplicitResearcherDoesNotDuplicateExistingResearcher' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestTextureAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit|TestInitialTextureRunWritesBeforeSpawningResearcher|TestTextureModelPriorCompletionGuardOpensProbePath|TestHandlePromptBarOperationalProofInitialRunStartsWithTexture|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureCreatedSuperEvidenceWakesTextureV2|TestInitialTextureRunDefaultsMinimalEditContextFromActivation|TestSubmitResearchFindingsWakeUsesSameDebouncedPath|TestHandlePromptBarExplicitNoWorkerDecisionStartsWithTexture|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TestTexturePromptInitialRevisionUsesSingleWriterLoop|TestTexturePromptForFactualFirstRevisionForbidsUngroundedContent|TestResearcherFailureSynthesizesCheckpointAfterSearch|TestRunSupportsCoagentUpdateInjectionIncludesTexture' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestCoagentUpdateTurnInjectorSupportsTexture|TestTextureAgentRevisionMutationCompletedOnlyOnce|TestBuildAppagentRevisionMetadataCarriesForwardDurableKeys|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit' -count=1`
- `nix develop -c scripts/go-test-runtime-shards`

Open blockers / remaining error:

- Staging must prove the guard survives live provider behavior. The next product
  probe must distinguish no-V1 edit failure, guard-not-fired, researcher-not-
  delivering, and Texture-not-consuming-findings branches.
- This repair is still not T3/T4: there is no park-and-wait primitive, no
  cumulative per-actor budget, no one-resident-run lifecycle, no sleep/resume
  proof, no doc-delete cancellation, and no N:1 verifier settlement.

Rollback ref: revert this completion-guard runtime commit if staging shows
guard-induced loops, excessive failed trajectories, or a semantic forced-role
regression. Otherwise keep it as the local repair for the post-V1 no-delegation
edge and continue toward deployed V2+ proof.

## 2026-06-18 - Staging falsified completion guard as sufficient (red evidence, green record)

Claim under test: deployed `58895d28e56dec72e63852fd9eb35bc9ce441ab7` would
convert the successful-V1/no-delegation branch into a fast model-prior V1
followed by Probe work and later V2+ revisions, while preserving Texture agency.

Move: deployed probe. Pushed `58895d28`, monitored Actions, checked staging
health identity, then ran the public product-path cadence probe against
`https://choir.news`.

Expected ΔV: staged proof that the completion guard reaches the live provider
path after V1, opens evidence work, and begins the V2+ cadence. Actual ΔV: no
mission ramp item landed; the formal probe still fails before V1, so the
completion guard branch remains unproven on staging.

Receipts:

- GitHub Actions for `58895d28`: Docs Truth Check #197 succeeded; FlakeHub #880
  succeeded; CI #1298 had all test/build jobs success, including internal/runtime
  shards 0-3, non-runtime Go tests, integration smoke, TLA+, docs, and vet/build.
  `Deploy to Staging (Node B)` concluded failure.
- Staging `/health` nevertheless reported proxy and sandbox both deployed at
  `58895d28e56dec72e63852fd9eb35bc9ce441ab7`, `upstream=ok`,
  `vmctl_status=ok`, `deployed_at=2026-06-18T03:17:36Z`.
- `nix shell nixpkgs#nodejs_22 -c env CHOIR_DEPLOYED_BASE_URL=https://choir.news node scripts/texture_revision_cadence_probe.mjs`
  submitted `012c7431-3645-4c7b-82a7-8efafedc4c2a`; doc
  `0e0fcfba-ead8-411f-b264-32d495ba51dd`.
- Probe result: V0 user at +0.330s; no appagent revisions;
  `appagent_revision_count=0`; `first_paint_ms=null`; `total_revision_count=1`;
  `final_head_chars=53`.
- Trace summary from the probe: `web_search=0`, `source_search=0`,
  `spawn_agent=0`, `update_coagent=0`, `moment_count=43`, `agent_count=2`,
  `delegation_count=0`, trajectory `state=failed`, `live=false`.

Result: the local completion guard may still be the right repair for the
successful-V1/no-Probe sub-branch, but it does not address the formal probe's
remaining no-V1 failure. The next construct must inspect Trace/provider/tool
events for this deployed failure (or a fresh equivalent diagnostic) before
changing runtime behavior again.

Next move: run a focused product-path Trace diagnostic for the no-V1 branch and
classify whether the failure is malformed edit arguments, exact-tool retry
exhaustion, provider/adapter failure, mutation settlement, or activation/prompt
assembly. Do not add another code fix before that classification.

Rollback ref: revert `58895d28` only if the diagnostic shows the completion guard
causes the no-V1 failure or a semantic forced-role regression. Otherwise keep it
as a local repair for the post-V1 branch and repair the earlier no-V1 branch next.

## 2026-06-18 - Fresh diagnostic found article-revision metadata bypass (green record)

Claim under test: the next discriminator for deployed `58895d28e56dec72e63852fd9eb35bc9ce441ab7`
was still only the no-V1 formal-probe branch.

Move: fresh product-path Trace/Texture diagnostic against `https://choir.news`
using the same factual/current prompt shape as the cadence probe.

Expected ΔV: classify whether the live branch is still malformed edit arguments,
exact-tool retry exhaustion, provider/adapter failure, mutation settlement, or
activation/prompt assembly. Actual ΔV: observer evidence changed the branch. A
successful V1 can be stored on staging, but it is not flagged
model-prior/interim, so the completion guard cannot recognize it.

Receipts:

- Diagnostic submission / trajectory:
  `653300e5-8f29-4094-8e45-00601bd378b0`.
- Texture doc: `16301311-92a1-4e57-b87d-88c4c0f99c45`.
- Initial loop id: `0859bdb0-8be2-45af-8570-dd0a2717b5e5`.
- Staging health identity during the diagnostic:
  `58895d28e56dec72e63852fd9eb35bc9ce441ab7`.
- Trajectory result: `state=completed`, `live=false`, `agent_count=2`,
  `delegation_count=0`, `finding_count=0`, `moment_count=30`.
- Revisions: V0 user plus appagent V1. V1 metadata included
  `artifact_kind=article_revision`, `revision_role=canonical`,
  `texture_version_stage=article_revision`, `source=patch_texture`,
  `texture_edit_operation=apply_edits`, and a rationale that called the revision
  an "honest interim revision"; it did not include `model_prior_interim`,
  `revision_grounding=model_prior`, or `grounding_status=model_prior_interim`.
- Trace had two `patch_texture` invocations/results, no `completion_guard`
  event, no `spawn_agent`, no researcher findings, and no `update_coagent`.

Result: the local guard is structurally unable to fire on this live successful-V1
branch because the metadata builder treats the prompt-bar initial revision as a
canonical Wire article revision instead of an interim prompt-only model-prior
revision. This is a T2 grounding-honesty failure before it is a cadence failure.

Next move: inspect `wirepublish.IsWireArticleRevisionRun`, prompt-bar Texture
run metadata, and `buildAppagentRevisionMetadata`; repair prompt-bar initial
no-worker factual/current revisions so model-prior/interim metadata is present
without changing genuinely sourced Wire article revision semantics.

Rollback ref: docs-only evidence checkpoint. Runtime rollback remains reverting
the later repair commit if it damages sourced article revisions, loops the guard,
or introduces semantic role choreography.

## 2026-06-18 - Local repair for article-shaped prompt-only V1 metadata (red construct)

Claim under test: the `58895d28` diagnostic branch failed because the successful
prompt-bar V1 was classified as a canonical Wire article revision, so the
model-prior completion guard could not see that factual/current V1 as interim.

Move: repaired `buildAppagentRevisionMetadata` so prompt-only initial Texture
revision runs override accidental article metadata when there is no consumed
worker update, scheduled message, or `update_coagent` source. Added a regression
test that constructs a user-prompt run which still satisfies
`wirepublish.IsWireArticleRevisionRun` and requires `model_prior_interim`,
`revision_grounding=model_prior`, `grounding_status=model_prior_interim`, and
non-publishable working/input metadata. Updated completion-guard fixture
providers to use addressed `update_coagent` packets and exact first-write
behavior matching the current runtime.

Expected ΔV: C2/T2 descends from live-metadata falsified to locally repaired
again; C8's next discriminator moves from code inspection to staging proof.
Actual ΔV: local repair and focused verification succeeded; staging remains
unproven.

Receipts:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestBuildAppagentRevisionMetadataMarksUserPromptArticleShapeAsInterim|TestTextureModelPriorCompletionGuardOpensProbePath|TestRunToolLoopCompletionGuardRetriesEndTurn' -count=1`
- `nix develop -c go test ./internal/runtime -run TestRunToolLoopCompletionGuardRetriesEndTurn -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run TestProcessorAndReconcilerProfilesDelegateToTextureOnly -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run TestTextureCreatedResearcherEvidenceWakesTextureV2 -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run TestTextureCreatedSuperEvidenceWakesTextureV2 -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestRunToolLoopCompletionGuardRetriesEndTurn|TestBuildAppagentRevisionMetadataMarksUserPromptArticleShapeAsInterim|TestBuildAppagentRevisionMetadataPreservesDurableKeys|TestProcessorAndReconcilerProfilesDelegateToTextureOnly|TestTextureModelPriorCompletionGuardOpensProbePath|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit|TestInitialTextureRunWritesBeforeSpawningResearcher|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureCreatedSuperEvidenceWakesTextureV2|TestInitialTextureRunDefaultsMinimalEditContextFromActivation|TestEditTextureInitialWorkingRevisionDoesNotSmuggleRequiredContinuation|TestEditTextureExplicitResearcherDoesNotForceSpawnContinuation|TestEditTextureExplicitResearcherDoesNotForceSpawnAfterSuperBase|TestEditTextureExplicitResearcherFromBaseRevisionContentSurvivesWorkerPrompt|TestEditTextureExplicitResearcherFromSeedPromptSurvivesRequestIntent|TestEditTextureExplicitResearcherDoesNotDuplicateExistingResearcher' -count=1`
- `nix develop -c go test ./internal/wirepublish -count=1`
- `nix develop -c go test ./cmd/doccheck -count=1`
- `git diff --check`
- `nix develop -c scripts/go-test-runtime-shards` exited 0; output showed
  shards 0, 1, and 3 passed, with shard 2 output elided by command-output
  truncation but covered by the zero exit status.

Open edge: this does not settle T3/T4/T8. The runtime still has no park-and-wait
primitive, no cumulative per-actor budget, and no one-resident-run lifecycle.
The next product proof must show the live cadence path: fast model-prior V1,
completion guard opening evidence work, and V2+ consuming findings. If staging
instead shows no-V1 or V1-only behavior, document that failure before another
code repair.

Rollback ref: revert the runtime repair commit if staging shows sourced Wire
article revisions lose canonical metadata, the completion guard loops, or
prompt-only V1s still bypass interim model-prior metadata.

## 2026-06-18 - Staging moved failure to V1-plus-research/no-V2 (red evidence, green record)

Claim under test: deployed `f96262421748902f257fd20aadd61477f7727353` would make
prompt-only initial Texture V1s honest model-prior/interim, let the completion
guard open evidence work, and produce V2+ after researcher findings.

Move: monitored the pushed main commit through CI, confirmed staging identity,
then ran the public product-path cadence probe against `https://choir.news`.

Expected ΔV: prove fast V1 plus evidence-opening V2+ cadence on staging. Actual
ΔV: C2/T2 is partially supported on staging because V1 and evidence work now
happen; T8 remains falsified because no V2 was written and the trajectory failed.

Receipts:

- Commit: `f96262421748902f257fd20aadd61477f7727353`.
- GitHub Actions: Docs Truth Check #27735436095 succeeded; FlakeHub #27735436119
  succeeded; CI #27735436104 test/build jobs all succeeded, including
  non-runtime Go tests, integration smoke, docs, TLA+, vet/build, deploy-impact
  detection, and internal/runtime shards 0, 1, 2, and 3. The CI workflow
  concluded failure only because `Deploy to Staging (Node B)` job #82051591616
  exited 1.
- Deploy identity: the deploy job's health probes and public
  `curl https://choir.news/health` both reported proxy and sandbox deployed at
  `f96262421748902f257fd20aadd61477f7727353`, `deployed_at=2026-06-18T03:53:52Z`,
  with `status=ok`, `upstream=ok`, and `vmctl_status=ok`.
- Probe command:
  `nix shell nixpkgs#nodejs_22 -c env CHOIR_DEPLOYED_BASE_URL=https://choir.news node scripts/texture_revision_cadence_probe.mjs`.
- Probe submission / trajectory:
  `bddc8556-602a-4cb1-b2be-134371cbb274`.
- Texture doc: `fff50f6c-93b5-46e8-9a2e-b74cf02a2869`.
- Probe timing: V0 user at +0.386s; appagent V1 at +28.966s; no V2 within the
  probe window; `appagent_revision_count=1`, `total_revision_count=2`,
  `final_head_chars=1035`.
- Trace summary: `web_search=2`, `source_search=2`, `spawn_agent=2`,
  `update_coagent=2`, `moment_count=128`, `search_attempt_count=12`,
  `search_success_count=4`, `agent_count=3`, `delegation_count=1`.
- Final trajectory: `state=failed`, `live=false`.

Result: the branch advanced. The live system no longer failed only as no-V1 or
V1-without-delegation; it produced fast V1 and researcher/supervision evidence
activity, but returned updates did not become a V2 revision before the trajectory
failed. The likely next classes are addressed-update delivery, integrate wake
failure, worker-update consumption metadata, or trajectory failure settlement.

Next move: run a focused product-path diagnostic that prints revision metadata,
terminal Trace/tool results, `update_coagent` targets/payloads, and any integrate
wake runs for this branch. If the original authenticated context is unavailable,
reproduce with a fresh one-off diagnostic. Document that diagnostic before any
next code repair.

Rollback ref: do not revert `f9626242` solely for this result; it moved staging
from no-V1/no-delegation to V1-plus-research. Revert only if the next diagnostic
shows the metadata repair itself caused the trajectory failure or damaged sourced
Wire article revision semantics.

## 2026-06-18 - Focused diagnostic found no-op consumed-evidence V2 (red evidence, green record)

Claim under test: the `f9626242` post-probe blocker was simply that returned
researcher updates were not delivered to Texture before failure.

Move: ran a fresh product-path diagnostic against `https://choir.news` that
printed revision metadata and filtered Trace moments for the same prompt shape.

Expected ΔV: classify the no-V2 branch as update delivery, integrate wake, or
terminal trajectory failure. Actual ΔV: the branch split. A fresh run can deliver
researcher evidence and write V2, but V2 can be a no-op that marks the evidence
consumed without deepening the document.

Receipts:

- Deployed identity during diagnostic: proxy and sandbox
  `f96262421748902f257fd20aadd61477f7727353`, `deployed_at=2026-06-18T03:53:52Z`.
- Diagnostic submission / trajectory:
  `8b935f7f-339b-4934-959e-6070ad71243c`.
- Texture doc: `568d6131-0988-4c77-b886-cb541e70c698`.
- Initial loop id: `2b47db7e-a4a6-48a5-8d0b-ce31e8ba72a6`.
- Revisions: V0 user at +0s; V1 appagent at about +24s; V2 appagent at about
  +73s.
- V1 metadata: `model_prior_interim=true`, `revision_grounding=model_prior`,
  `grounding_status=model_prior_interim`, `texture_version_stage=interim`,
  `revision_role=input`, `worker_updates_consumed=[]`.
- V2 metadata: `artifact_kind=article_revision`, `revision_role=canonical`,
  `texture_version_stage=article_revision`,
  `texture_edit_base_revision_id=6a17822f-2976-40c9-a098-4742d4b42fe0`,
  `worker_updates_checkpoint_seq=0`, `worker_updates_scheduled_seq=1`,
  `worker_updates_consumed` contained researcher seq 1 from loop
  `69dfe453-ff39-46cc-b27b-5b1ea1040cf8`, and `worker_updates_pending=[]`.
- V2 content did not deepen: V1 and V2 both had 794 chars,
  `texture_edit_delta_chars=0`, and the V2 rationale was "Required immediate
  Texture write call in response to the user's instruction; no substantive
  content change intended."
- Trace: trajectory completed, `agent_count=3`, `delegation_count=1`,
  `moment_count=160`, `search_attempt_count=12`, `search_success_count=4`.
  Trace included researcher `update_coagent`, one duplicate `update_coagent`
  error, an integrate Texture run, failed `patch_texture` attempts followed by
  retry after `required_initial_tool_failed`, and then successful `patch_texture`
  results for V2.

Result: C2/T2 is now partially supported on staging: V1 metadata is honest and a
V2 wake can consume researcher evidence. T8 remains falsified because the
grounded/deepening quality is absent; the revision stream can advance with a
no-op patch that burns the worker update.

Next move: inspect the scheduled-message / consumed-worker-update Texture
first-tool path. The next repair should keep the initial V1 exact-write guard,
but prevent evidence-bearing wake runs from satisfying the write obligation with
an empty/no-op patch that neither incorporates nor explicitly accounts for the
consumed findings.

Rollback ref: keep `f9626242`; it repaired V1 metadata and enabled evidence
delivery. Revert a future repair if it regresses initial V1, blocks worker-update
delivery, or forces Texture into a hard-coded researcher workflow.

## 2026-06-18 - Local repair rejects no-op consumed-evidence writes (red construct)

Claim under test: the no-op V2 branch can be repaired mechanically by refusing to
store an unchanged Texture revision when that write would mark worker updates as
consumed.

Move: added a guard in `commitTextureToolEdit` after edit materialization and
before metadata construction / revision creation / worker-update delivery. If
`consumedThroughSeq > 0` and the materialized content exactly equals the current
revision content, `patch_texture` / `rewrite_texture` returns an error instead
of storing a revision or advancing the controller checkpoint. Added a
comprehensive regression test that builds the live-shaped researcher update,
scheduled `update_coagent` Texture wake, and no-op patch rationale, then verifies
the revision history stays at the base revision, the mutation remains pending,
and no Texture controller checkpoint is written.

Expected ΔV: move the V2 defect from "worker update can be burned by a no-op
revision" to "local guard forces the provider to produce a substantive revision
or fail without consuming the update." Actual ΔV: focused local tests passed;
staging remains unproven.

Receipts:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestTextureWorkerUpdateRevisionRejectsNoOpPatch|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureCreatedSuperEvidenceWakesTextureV2|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite' -count=1`
- `git diff --check`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestRunToolLoopCompletionGuardRetriesEndTurn|TestBuildAppagentRevisionMetadataMarksUserPromptArticleShapeAsInterim|TestBuildAppagentRevisionMetadataPreservesDurableKeys|TestProcessorAndReconcilerProfilesDelegateToTextureOnly|TestTextureModelPriorCompletionGuardOpensProbePath|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit|TestInitialTextureRunWritesBeforeSpawningResearcher|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureCreatedSuperEvidenceWakesTextureV2|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestTextureWorkerUpdateRevisionRejectsNoOpPatch|TestInitialTextureRunDefaultsMinimalEditContextFromActivation|TestEditTextureInitialWorkingRevisionDoesNotSmuggleRequiredContinuation|TestEditTextureExplicitResearcherDoesNotForceSpawnContinuation|TestEditTextureExplicitResearcherDoesNotForceSpawnAfterSuperBase|TestEditTextureExplicitResearcherFromBaseRevisionContentSurvivesWorkerPrompt|TestEditTextureExplicitResearcherFromSeedPromptSurvivesRequestIntent|TestEditTextureExplicitResearcherDoesNotDuplicateExistingResearcher' -count=1`
- `nix develop -c go test ./cmd/doccheck -count=1`
- `nix develop -c scripts/go-test-runtime-shards` exited 0.

Open edge: this guard prevents the exact no-op burn branch, but the live provider
may still fail all exact `patch_texture` retries on a worker wake. Staging must
decide whether the model recovers by writing a substantive grounded V2 or exposes
a new retry-exhaustion failure.

Rollback ref: revert the no-op guard commit if legitimate same-content
evidence-accounting revisions are required and no alternative accounting path is
available, or if staging shows it regresses initial V1 / worker update delivery.

## 2026-06-18 - Staging probe supports V2 guard, exposes no-op V1 (red evidence, green record)

Claim under test: rejecting identical consumed-evidence Texture writes is enough
for the live provider to recover into a substantive V2 while preserving fast
from-weights V1.

Move: pushed `157db34f3330e64ff55541a71afc5776ba4e1410`, monitored CI/deploy
identity, and ran the deployed cadence probe against `https://choir.news`.

Expected ΔV: either settle the immediate V1 + substantive V2 slice, or classify
the next live branch after the no-op V2 guard. Actual ΔV: the consumed-evidence
V2 branch improved, but V1 quality/timing is now the active blocker.

Receipts:

- Commit: `157db34f3330e64ff55541a71afc5776ba4e1410`.
- CI: Docs Truth Check and CI test/build jobs passed, including
  internal/runtime shards 0, 1, 2, and 3; overall CI concluded failure only
  because the Node B deploy job exited 1.
- Deploy identity: public `/health` showed proxy and sandbox both at
  `157db34f3330e64ff55541a71afc5776ba4e1410`, `deployed_at=2026-06-18T04:14:54Z`.
- Probe command:
  `nix shell nixpkgs#nodejs_22 -c env CHOIR_DEPLOYED_BASE_URL=https://choir.news node scripts/texture_revision_cadence_probe.mjs`.
- Submission / trajectory: `dfb565f2-affe-4cab-a422-f3049777eff5`.
- Texture doc: `947e4baf-65d0-47c5-97ea-20d04b692840`.
- Revision timing: V0 user at +0.326s, 53 chars; appagent V1 at +49.350s,
  53 chars; appagent V2 at +88.448s, 1336 chars.
- Probe counts: `appagent_revision_count=2`, `total_revision_count=3`,
  `first_paint_ms=49350`, `final_head_chars=1336`.
- Research / trajectory: `web_search=8`, `source_search=2`, `spawn_agent=2`,
  `update_coagent=2`, `moment_count=117`, `search_attempt_count=48`,
  `search_success_count=16`, `agent_count=3`, `delegation_count=1`,
  final trajectory `state=completed`.

Result: the no-op V2 guard has staging support because the formal probe reached
a substantive V2 rather than burning the researcher update with identical
content. The first-paint requirement remains falsified: V1 was prompt-sized,
not a useful from-weights draft, and it landed at 49.350s rather than well under
the prior ~49s first-findings baseline.

Next move: diagnose the initial no-worker exact-write branch on `157db34f` with
metadata/tool-result evidence if needed, then repair it so unchanged prompt-copy
V1 writes do not satisfy the first-paint obligation. The repair must preserve
model-prior/interim metadata and the now-working substantive V2 path.

Rollback ref: keep `157db34f`; it improved the V2 branch. Revert a future V1
repair if it regresses substantive V2, worker-update delivery, or honest
model-prior/interim metadata.

## 2026-06-18 - Local repair rejects prompt-copy initial V1 (red construct)

Claim under test: the no-op V1 branch can be repaired by refusing to store an
unchanged prompt-only model-prior Texture revision, letting the existing exact
initial tool retry path ask the provider for a real draft.

Move: extended the content-equality guard in `commitTextureToolEdit` to reject
unchanged writes whose revision metadata is `model_prior_interim` or
`revision_grounding=model_prior`, after metadata classification and before
revision storage. Added one direct guard test for a current-events prompt-copy
patch and one prompt-bar path test proving the failed no-op `patch_texture`
keeps exact tool choice active and the same run stores a useful V1 on retry.

Expected ΔV: move the active branch from "prompt-copy V1 can satisfy first
paint" to "prompt-copy V1 is a failed tool result that must retry or fail
without storing appagent V1." Actual ΔV: focused local tests passed; staging
remains unproven.

Receipts:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestInitialTextureRevisionRejectsNoOpPromptCopy|TestInitialTextureNoOpPatchRetriesIntoUsefulDraft|TestTextureWorkerUpdateRevisionRejectsNoOpPatch|TestTextureModelPriorCompletionGuardOpensProbePath|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite' -count=1`

Result: local storage and tool-loop behavior now match the desired branch. A
prompt-only no-op V1 does not create an appagent revision or complete the
mutation, and the exact initial `patch_texture` retry can recover into one
useful model-prior V1. The worker-update no-op guard and evidence wake tests
still pass in the same focused batch.

Next move: run the broader focused runtime batch, docs check, whitespace check,
and runtime shards; then commit/push and let staging decide whether the live
provider writes a useful V1 quickly or exposes a new retry-exhaustion branch.

Rollback ref: revert the prompt-copy guard commit if staging shows legitimate
initial model-prior drafts require same-content storage, or if it regresses the
substantive V2 path repaired by `157db34f`.

## 2026-06-18 - Staging prompt-copy guard becomes no-V1 failure (red evidence, green record)

Claim under test: rejecting prompt-copy initial V1 writes lets the live provider
recover through exact `patch_texture` retry into a useful model-prior first
draft.

Move: pushed `84038c4ae972c0aa3a32b18b6b227e763a9be777`, monitored CI/deploy
identity, and ran the deployed cadence probe against `https://choir.news`.

Expected ΔV: either prove useful V1 + substantive V2 on staging, or classify the
live retry branch after the prompt-copy guard. Actual ΔV: the guard prevented the
bad stored V1, but the live provider did not recover into any appagent revision.

Receipts:

- Commit: `84038c4ae972c0aa3a32b18b6b227e763a9be777`.
- CI: Docs Truth Check and CI test/build jobs passed, including
  internal/runtime shards 0, 1, 2, and 3; overall CI concluded failure only
  because the Node B deploy job exited 1.
- Deploy identity: public `/health` showed proxy and sandbox both at
  `84038c4ae972c0aa3a32b18b6b227e763a9be777`, `deployed_at=2026-06-18T04:34:01Z`.
- Probe command:
  `nix shell nixpkgs#nodejs_22 -c env CHOIR_DEPLOYED_BASE_URL=https://choir.news node scripts/texture_revision_cadence_probe.mjs`.
- Submission / trajectory: `e83758bf-5b54-44a7-8838-3c4e686a8b30`.
- Texture doc: `6ed92ad9-c861-458c-b5c4-a09ca48dc529`.
- Revisions: only V0 user at +0.252s, 53 chars.
- Probe counts: `appagent_revision_count=0`, `first_paint_ms=null`,
  `total_revision_count=1`, `final_head_chars=53`.
- Research / trajectory: `web_search=0`, `source_search=0`, `spawn_agent=0`,
  `update_coagent=0`, `moment_count=43`, `agent_count=2`,
  `delegation_count=0`, final trajectory `state=failed`.

Result: the local repair was directionally correct on canonical integrity - it
did not store the prompt-copy V1 - but insufficient for product behavior. The
live path is now a failed no-appagent-V1 branch instead of a stored no-op V1
branch.

Next move: run a focused diagnostic on the same deployed SHA that prints
initial Texture tool results, retry events, and final error state. The next
repair should improve live recovery after rejected no-op/invalid initial patches
without re-allowing prompt-copy V1 storage.

Rollback ref: keep `84038c4a` until the diagnostic proves it blocks all
recoverable initial drafts; it protects canonical history from prompt-copy V1s.
Revert only if the next repair cannot preserve that invariant while restoring
first paint.

## 2026-06-18 - Same-SHA diagnostic proves useful V1 and V2+ branch (red evidence, green record)

Claim under test: the `84038c4a` no-V1 result is deterministic live failure
rather than a stochastic branch after rejected prompt-copy writes.

Move: ran a focused product-path diagnostic and then reran the formal cadence
probe against the same deployed SHA.

Expected ΔV: classify the no-V1 branch by Trace/tool-result evidence. Actual
ΔV: the branch split. The same deployed build can recover into useful V1 and
multiple V2+ revisions; the earlier no-V1 result is residual stochastic risk,
not deterministic failure of the guard.

Receipts:

- Deployed identity during both probes: proxy and sandbox
  `84038c4ae972c0aa3a32b18b6b227e763a9be777`.
- Focused diagnostic submission / trajectory:
  `534bdb39-b582-43d8-9fc9-8a961b8a4fbd`.
- Focused diagnostic doc: `83a9d899-e062-4f49-9052-2f8f15a112f8`.
- Focused diagnostic revisions: V0 user 53 chars; V1 appagent 602 chars with
  `model_prior_interim=true`, `revision_grounding=model_prior`,
  `texture_version_stage=interim`, and no worker updates consumed; V2 appagent
  695 chars consuming researcher seq 1; V3 appagent 2728 chars consuming
  researcher seq 2. Trajectory completed with `agent_count=3`,
  `delegation_count=1`, `moment_count=186`, `search_attempt_count=12`,
  `search_success_count=4`.
- Repeat formal probe command:
  `nix shell nixpkgs#nodejs_22 -c env CHOIR_DEPLOYED_BASE_URL=https://choir.news node scripts/texture_revision_cadence_probe.mjs`.
- Repeat formal probe submission / trajectory:
  `ca430c89-b259-453b-900c-2863c0e38567`.
- Repeat formal probe doc: `37277fa3-7843-41ba-9e69-b6b6e16c37fb`.
- Repeat formal probe revisions: V0 user at +0.330s, 53 chars; V1 appagent at
  +23.795s, 569 chars; V2 appagent at +49.844s, 859 chars; V3 appagent at
  +65.631s, 1509 chars.
- Repeat formal probe counts: `appagent_revision_count=3`,
  `first_paint_ms=23795`, `total_revision_count=4`, `final_head_chars=1509`,
  `web_search=4`, `source_search=0`, `spawn_agent=2`, `update_coagent=4`,
  `moment_count=165`, final trajectory `state=completed`.

Result: `84038c4a` supports the fast useful V1 + multi-revision V2+ cadence on
staging, but not deterministically. The no-prompt-copy invariant holds in the
successful branch, and the same deployed product can deepen across multiple
canonical appagent revisions as researcher updates arrive. The earlier
`e83758bf-5b54-44a7-8838-3c4e686a8b30` no-V1 failure remains a residual
first-write recovery risk.

Next move: either run a small repeated-probe stability sample before another
runtime repair, or proceed to the next ramp item while naming stochastic no-V1 as
residual risk. Do not claim mission settlement; T3-T8 remain open and no
RunAcceptanceRecord should claim staging-smoke-level for the full mission yet.

Rollback ref: keep `84038c4a`; the successful same-SHA branch proves the guard
does not inherently block useful first drafts or V2+ cadence.

## 2026-06-18 - Local T3 tool-loop budget substrate (red construct)

Claim under test: the T3 bounded-cost slice can descend before full
park-and-wait by replacing Texture's inherited bare tool-loop ceiling with a
role-uniform cumulative loop budget and Trace-visible kill switch.

Move: construct. Added `ToolLoopBudget` to the generic `RunToolLoop` option
surface, enforcing provider-call, input-token, output-token, total-token, and
elapsed-time limits for any caller. The loop now emits the configured budget in
`provider_call` progress payloads and emits `tool_loop_budget` progress evidence
when a budget is exhausted. Texture revision runs attach a conservative
actor-labeled budget (`texture:<docID>`) through the existing runtime run setup,
with metadata overrides for provider-call, token, and elapsed limits.

Expected ΔV: repair the "bare maxToolLoopIterations only" part of T3 without
claiming park-and-wait, one-resident lifecycle, or passivation-as-sleep. Actual
ΔV: partial T3 descent. There is now a uniform budget primitive and Texture uses
it, but parked no-billed idle waits and cross-passivation cumulative accounting
remain open.

Receipts:

- `internal/runtime/toolloop.go` defines `ToolLoopBudget`, `WithToolLoopBudget`,
  before-provider provider-call/elapsed checks, after-provider cumulative token
  checks, and `tool_loop_budget` evidence emission.
- `internal/runtime/runtime.go` wires `textureActorToolLoopBudget` into Texture
  revision runs next to the existing initial-tool and completion-guard options.
- `internal/runtime/texture_prompt_unit_test.go` proves default Texture actor
  budget labels and metadata overrides.
- `internal/runtime/toolloop_test.go` proves provider-call budget exhaustion
  stops after two provider calls and cumulative token exhaustion stops before
  returning a final answer.

Verification:

- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoopBudgetLimitsProviderCalls|TestRunToolLoopBudgetLimitsCumulativeTokens|TestRunToolLoopMaxIterations|TestTextureActorToolLoopBudgetDefaultsAndOverrides|TestInitialTextureToolChoiceRequiresPatchBeforeContinuation' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestRunToolLoopBudgetLimitsProviderCalls|TestRunToolLoopBudgetLimitsCumulativeTokens|TestRunToolLoopMaxIterations|TestRunToolLoopCompletionGuardRetriesEndTurn|TestBuildAppagentRevisionMetadataMarksUserPromptArticleShapeAsInterim|TestBuildAppagentRevisionMetadataPreservesDurableKeys|TestProcessorAndReconcilerProfilesDelegateToTextureOnly|TestTextureModelPriorCompletionGuardOpensProbePath|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit|TestInitialTextureRunWritesBeforeSpawningResearcher|TestInitialTextureRevisionRejectsNoOpPromptCopy|TestInitialTextureNoOpPatchRetriesIntoUsefulDraft|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureCreatedSuperEvidenceWakesTextureV2|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestTextureWorkerUpdateRevisionRejectsNoOpPatch|TestTextureActorToolLoopBudgetDefaultsAndOverrides|TestInitialTextureRunDefaultsMinimalEditContextFromActivation|TestEditTextureInitialWorkingRevisionDoesNotSmuggleRequiredContinuation|TestEditTextureExplicitResearcherDoesNotForceSpawnContinuation|TestEditTextureExplicitResearcherDoesNotForceSpawnAfterSuperBase|TestEditTextureExplicitResearcherFromBaseRevisionContentSurvivesWorkerPrompt|TestEditTextureExplicitResearcherFromSeedPromptSurvivesRequestIntent|TestEditTextureExplicitResearcherDoesNotDuplicateExistingResearcher' -count=1`
- `nix develop -c scripts/go-test-runtime-shards`

Open blockers / remaining error:

- T3 is not complete: there is still no park-and-wait primitive that blocks with
  no billed provider calls until a packet/signal or idle deadline, and the new
  budget is per activation rather than cumulative across passivation/rewarm.
- T4 remains open: `texture_controller.go` still owns wake/reconcile scaffolding
  instead of one parked resident `texture:<docID>` actor.
- T5-T8 remain open: sleep/resume semantics, doc-delete cancellation,
  N:1 verifier lifecycle proof, deployed proof, and RunAcceptanceRecord are not
  yet satisfied.

Rollback ref: revert this T3 budget construct commit if staging shows the
conservative Texture budget regresses first paint, V2+ update consumption, or
long-running provider fallback behavior.

## 2026-06-18 - Staging proof for T3 budget substrate (red evidence, green record)

Claim under test: deployed `f5884e08977f74ed463a55a19e9ece3cd24dc06f` preserves
the currently supported fast V1 + V2 cadence while adding the conservative
Texture actor budget.

Move: landing proof. Pushed `f5884e08`, monitored GitHub Actions, confirmed
staging health identity, and ran the public product-path cadence probe against
`https://choir.news`.

Expected ΔV: prove the budget construct does not regress the staging cadence
slice and record that T3 remains incomplete until park-and-wait exists. Actual
ΔV: partial T3 supported on staging. Budget enforcement is deployed and the
cadence probe still reaches fast V1 and one grounded follow-on V2. Full mission
settlement remains open.

Receipts:

- Commit: `f5884e08977f74ed463a55a19e9ece3cd24dc06f`.
- GitHub Actions: Docs Truth Check #27737645943 succeeded; FlakeHub
  #27737645941 succeeded; CI #27737645928 concluded failure only because
  `Deploy to Staging (Node B)` job #82058203814 failed. CI jobs for TLA+,
  Go vet/build, deploy-impact detection, docs, runtime shards 0, 1, 2, and 3,
  non-runtime Go tests, integration smoke, and the aggregate Go vet/test/build
  job all succeeded.
- Staging identity: public `/health` reported proxy and sandbox both at
  `f5884e08977f74ed463a55a19e9ece3cd24dc06f`,
  `deployed_at=2026-06-18T04:59:36Z`, with `status=ok`, `upstream=ok`, and
  `vmctl_status=ok`.
- Probe command:
  `nix shell nixpkgs#nodejs_22 -c env CHOIR_DEPLOYED_BASE_URL=https://choir.news node scripts/texture_revision_cadence_probe.mjs`.
- Probe submission / trajectory: `820581e2-ef7f-430f-80a5-5e148a3552d7`.
- Texture doc: `1e9bae65-6953-49c2-a95d-c75370e3e855`.
- Revision timing: V0 user at +0.291s, 53 chars; appagent V1 at +23.508s,
  880 chars; appagent V2 at +73.013s, 1786 chars.
- Probe counts: `appagent_revision_count=2`, `total_revision_count=3`,
  `first_paint_ms=23508`, `final_head_chars=1786`.
- Research / trajectory: `web_search=2`, `source_search=2`, `spawn_agent=2`,
  `update_coagent=2`, `moment_count=114`, `search_attempt_count=12`,
  `search_success_count=4`, `agent_count=3`, `delegation_count=1`, final
  trajectory `state=completed`.

Result: the conservative Texture actor budget is deployed and did not regress
the fast useful V1 plus grounded V2 product path. Do not overclaim: this probe
does not prove many V2+ revisions, park-and-wait, no-billed idle blocking,
one-resident-run lifecycle, sleep/resume, cancellation, verifier N:1, or a
RunAcceptanceRecord.

Next move: continue T3 with the actual role-uniform park-and-wait primitive and
cumulative budget accounting across sleep/rewarm, then resume the T4-T8 ramp.

Rollback ref: revert `f5884e08` if future staging evidence shows the budget
causes premature Texture failure, provider fallback regressions, or blocked
multi-revision updates.

## 2026-06-18 - Local T3 park-and-wait primitive (red construct)

Claim under test: a uniform tool-loop park waiter can suspend a resident actor
with no provider calls while idle, then resume only after runtime-owned context
is injected from a durable signal.

Move: added `WithParkWaiter` to the generic tool loop, owner+agent waiter
registration in runtime, and a metadata-gated coagent waiter wired to
`update_coagent` signaling. The new waiter is opt-in through
`actor_park_on_idle`; it is not yet the default Texture lifecycle.

Expected ΔV: repair the no-billed-idle primitive slice of T3 without adding a
Texture-specific harness branch. Actual ΔV: local runtime evidence supports the
primitive and warm signal path. Remaining error: cross-passivation cumulative
budget accounting and T4 one-resident-run Texture lifecycle are still open.

Receipts:

- `internal/runtime/toolloop.go` accepts `WithParkWaiter`, emits
  `park_wait_started` / `park_wait_finished`, blocks outside provider calls, and
  resumes only after injected user turns are appended.
- `internal/runtime/runtime.go` keeps an in-memory owner+agent waiter registry
  and wakes waiters when the resident run is signaled or removed.
- `internal/runtime/super_controller.go` notifies waiters when `update_coagent`
  targets a resident agent and exposes an opt-in coagent park waiter.
- `internal/runtime/toolloop_test.go` adds
  `TestRunToolLoopParkWaiterBlocksWithoutProviderCallsUntilInjectedTurn` and
  `TestRuntimeAgentSignalWakesParkWaiter`.

Verification:

- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoopParkWaiterBlocksWithoutProviderCallsUntilInjectedTurn|TestRuntimeAgentSignalWakesParkWaiter|TestRunToolLoopBudgetLimitsProviderCalls|TestRunToolLoopBudgetLimitsCumulativeTokens|TestRunToolLoopMaxIterations' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestRunToolLoopParkWaiterBlocksWithoutProviderCallsUntilInjectedTurn|TestRuntimeAgentSignalWakesParkWaiter|TestRunToolLoopBudgetLimitsProviderCalls|TestRunToolLoopBudgetLimitsCumulativeTokens|TestRunToolLoopMaxIterations|TestRunToolLoopCompletionGuardRetriesEndTurn|TestBuildAppagentRevisionMetadataMarksUserPromptArticleShapeAsInterim|TestBuildAppagentRevisionMetadataPreservesDurableKeys|TestProcessorAndReconcilerProfilesDelegateToTextureOnly|TestTextureModelPriorCompletionGuardOpensProbePath|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit|TestInitialTextureRunWritesBeforeSpawningResearcher|TestInitialTextureRevisionRejectsNoOpPromptCopy|TestInitialTextureNoOpPatchRetriesIntoUsefulDraft|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureCreatedSuperEvidenceWakesTextureV2|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestTextureWorkerUpdateRevisionRejectsNoOpPatch|TestTextureActorToolLoopBudgetDefaultsAndOverrides|TestInitialTextureRunDefaultsMinimalEditContextFromActivation|TestEditTextureInitialWorkingRevisionDoesNotSmuggleRequiredContinuation|TestEditTextureExplicitResearcherDoesNotForceSpawnContinuation|TestEditTextureExplicitResearcherDoesNotForceSpawnAfterSuperBase|TestEditTextureExplicitResearcherFromBaseRevisionContentSurvivesWorkerPrompt|TestEditTextureExplicitResearcherFromSeedPromptSurvivesRequestIntent|TestEditTextureExplicitResearcherDoesNotDuplicateExistingResearcher' -count=1`
- `nix develop -c scripts/go-test-runtime-shards`
- `nix develop -c go test ./cmd/doccheck -count=1`
- `git diff --check`

Open blockers / remaining error:

- T3 is not fully complete: the new budget is still per activation rather than
  cumulative across passivation/rewarm.
- T4 remains open: Texture does not yet enable `actor_park_on_idle` by default
  or replace `texture_controller.go` wake/reconcile scaffolding with one parked
  resident `texture:<docID>` actor.
- T5-T8 remain open: sleep/resume semantics, doc-delete cancellation,
  N:1 verifier lifecycle proof, deployed product proof, and RunAcceptanceRecord
  are not yet satisfied.

Rollback ref: revert the T3 park-waiter construct commit if deployed evidence
shows stalled Texture runs, lost `update_coagent` delivery, extra provider calls
while idle, or broken cadence non-regression.

## 2026-06-18 - Staging proof failure for T3 park primitive deploy (red evidence, green record)

Claim under test: deployed `d7b7ae4929a92623dcdd99e766ffce0c189c0a86`
preserves the currently supported Texture cadence path while carrying the
metadata-gated park waiter primitive.

Move: pushed `d7b7ae49`, monitored GitHub Actions, confirmed staging health
identity, and ran the public product-path cadence probe against
`https://choir.news`.

Expected ΔV: prove no regression in the staging cadence slice, while keeping
park-waiter proof local because `actor_park_on_idle` is not product-default.
Actual ΔV: deploy identity is confirmed and tests passed, but the formal
deployed cadence probe failed with no appagent revision. This repeats the
previously named no-V1 live branch and blocks acceptance for this deploy.

Receipts:

- Commit: `d7b7ae4929a92623dcdd99e766ffce0c189c0a86`.
- GitHub Actions: Docs Truth Check #27738567105 succeeded; FlakeHub
  #27738567106 succeeded; CI #27738567112 concluded failure only because
  `Deploy to Staging (Node B)` job #82060956249 failed. CI jobs for TLA+,
  Go vet/build, deploy-impact detection, docs, runtime shards 0, 1, 2, and 3,
  non-runtime Go tests, integration smoke, and the aggregate Go vet/test/build
  job all succeeded.
- Staging identity: public `/health` reported proxy and sandbox both at
  `d7b7ae4929a92623dcdd99e766ffce0c189c0a86`,
  `deployed_at=2026-06-18T05:25:50Z`, with `status=ok`, `upstream=ok`, and
  `vmctl_status=ok`.
- Probe command:
  `nix shell nixpkgs#nodejs_22 -c env CHOIR_DEPLOYED_BASE_URL=https://choir.news node scripts/texture_revision_cadence_probe.mjs`.
- Probe submission / trajectory: `a69f85b6-25b4-428e-be8f-63a366480383`.
- Texture doc: `c4257df0-bd3b-433a-aa1d-1ac3ed775f69`.
- Revision timing: V0 user at +0.296s, 53 chars; no appagent revisions.
- Probe counts: `appagent_revision_count=0`, `total_revision_count=1`,
  `first_paint_ms=null`, `final_head_chars=53`.
- Research / trajectory: `web_search=0`, `source_search=0`, `spawn_agent=0`,
  `update_coagent=0`, `moment_count=43`, `agent_count=2`,
  `delegation_count=0`, final trajectory `state=failed`.

Result: the park-waiter primitive is deployed but not product-path accepted. The
probe did not enable `actor_park_on_idle`, so it does not directly falsify the
new park primitive; it does show the deployed product path still hits the no-V1
Texture branch.

Next move: inspect Trace/run evidence for
`a69f85b6-25b4-428e-be8f-63a366480383` /
`c4257df0-bd3b-433a-aa1d-1ac3ed775f69` before any code repair. Preserve this
docs checkpoint as the first commit after the staging evidence.

Rollback ref: revert `d7b7ae49` if Trace shows the new park waiter or waiter
signal plumbing caused the no-V1 branch; otherwise treat the failure as the
pre-existing stochastic no-V1 branch and repair that separately.

## 2026-06-18 - Local initial patch retry guidance repair (red construct)

Claim under test: the no-V1 live branch is retry exhaustion on malformed
initial `patch_texture` calls, not a park-waiter regression. The model needs
more precise retry pressure after a failed required initial `patch_texture`.

Move: inspected authenticated product Trace details through
`/api/trace/trajectories/{id}` and
`/api/trace/trajectories/{id}/moments/{moment_id}`, then changed the generic
required-initial-tool reminder text for failed `patch_texture` to direct initial
first-paint drafts away from prompt-text replacement/no-op copies and toward an
append edit with substantive draft content.

Expected ΔV: reduce the live no-V1 branch while preserving the no-op guard and
without adding Texture-specific harness control flow. Actual ΔV: local focused
tests cover the observed error sequence and recover into a useful V1. Deployed
acceptance remains open until the repair is pushed and probed.

Receipts:

- Failure reproduction diagnostic: `038fc8b3-a422-42a3-bf63-dbf5d6d122fe` /
  doc `06274f82-702f-41db-9567-ce7c0e0ccbf1` failed with no appagent revision;
  Trace ended with `tool loop: required initial tool "patch_texture" did not
  succeed after 2 retries`.
- Detail diagnostic: `ff54e889-bb5e-42c9-b679-f89aa9e90c9e` / doc
  `bc97e669-3517-434a-8921-00978e5e4fa7` recovered after initial tool errors:
  `tool_error: edit 0: find text not present`, then
  `tool_error: initial model-prior Texture revision must change prompt content
  before first paint is stored`, then a stored appagent V1.
- `internal/runtime/toolloop.go` now emits a specific retry reminder for
  failed required initial `patch_texture`.
- `internal/runtime/texture_test.go` updates
  `TestInitialTextureNoOpPatchRetriesIntoUsefulDraft` to exercise missing-find,
  prompt-copy/no-op, then append-based recovery and assert the new guidance is
  visible to the provider.

Verification so far:

- `nix develop -c go test ./internal/runtime -run 'TestInitialTextureNoOpPatchRetriesIntoUsefulDraft|TestRunToolLoopRequiredNextToolMaxTokensStopsAfterBoundedRetries|TestRunToolLoopRetriesEndTurnBeforeRequiredNextTool' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestInitialTextureNoOpPatchRetriesIntoUsefulDraft|TestInitialTextureRevisionRejectsNoOpPromptCopy|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureWorkerUpdateRevisionRejectsNoOpPatch|TestRunToolLoopParkWaiterBlocksWithoutProviderCallsUntilInjectedTurn|TestRuntimeAgentSignalWakesParkWaiter|TestTextureActorToolLoopBudgetDefaultsAndOverrides|TestRunToolLoopCompletionGuardRetriesEndTurn|TestTextureModelPriorCompletionGuardOpensProbePath|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit|TestInitialTextureRunWritesBeforeSpawningResearcher' -count=1`

Open blockers / remaining error:

- Full runtime shards, CI, deploy identity, and deployed cadence proof remain to
  run for this repair.
- Even if cadence proof passes, T3/T4-T8 remain open: default parked Texture
  lifecycle, cumulative budget across sleep/rewarm, doc-delete cancellation,
  N:1 verifier proof, and RunAcceptanceRecord are not yet satisfied.

Rollback ref: revert the initial patch retry-guidance repair if deployed
evidence shows it increases initial Texture failures, suppresses useful V1
creation, or weakens the prompt-copy/no-op guard.

## 2026-06-18 - Retry guidance deployed, cadence slice accepted, lifecycle still blocked (red proof + green record)

Claim under test: the initial `patch_texture` retry-guidance repair in
`4da4ffa3fc9d6831e3d5643b6993aaba4ad67d9e` reduces the deployed no-V1 branch
enough for the product-path cadence probe to show fast useful V1 and multiple
V2+ revisions again.

Move: land + deployed probe + acceptance synthesis. Pushed `4da4ffa3`, monitored
GitHub Actions, checked staging health identity, ran the formal cadence probe,
then reran a product-path proof that synthesized a `RunAcceptanceRecord` in the
same authenticated browser session.

Expected ΔV: staged proof for the current cadence slice and a
`staging-smoke-level` acceptance record, while leaving T4-T8 open. Actual ΔV:
the formal cadence probe passed with fast useful V1 and V2/V3; a
staging-smoke-level record was synthesized; the acceptance rerun exposed
remaining weak/late V1 stochasticity and the record state is correctly blocked.

Receipts:

- Commit: `4da4ffa3fc9d6831e3d5643b6993aaba4ad67d9e`
  (`runtime: guide initial texture patch retries`).
- Local verification before commit:
  `nix develop -c go test ./internal/runtime -run 'TestInitialTextureNoOpPatchRetriesIntoUsefulDraft|TestRunToolLoopRequiredNextToolMaxTokensStopsAfterBoundedRetries|TestRunToolLoopRetriesEndTurnBeforeRequiredNextTool' -count=1`;
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestInitialTextureNoOpPatchRetriesIntoUsefulDraft|TestInitialTextureRevisionRejectsNoOpPromptCopy|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureWorkerUpdateRevisionRejectsNoOpPatch|TestRunToolLoopParkWaiterBlocksWithoutProviderCallsUntilInjectedTurn|TestRuntimeAgentSignalWakesParkWaiter|TestTextureActorToolLoopBudgetDefaultsAndOverrides|TestRunToolLoopCompletionGuardRetriesEndTurn|TestTextureModelPriorCompletionGuardOpensProbePath|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit|TestInitialTextureRunWritesBeforeSpawningResearcher' -count=1`;
  `nix develop -c go test ./cmd/doccheck -count=1`;
  `git diff --check`; `nix develop -c scripts/go-test-runtime-shards`.
- GitHub Actions for `4da4ffa3`: Docs Truth Check run `27739387244` succeeded;
  FlakeHub run `27739387274` succeeded; CI run `27739387251` concluded failure
  only because `Deploy to Staging (Node B)` job `82063482453` failed. CI jobs
  for Go vet/build, TLA+, non-runtime Go tests, deploy-impact detection,
  integration smoke, internal/runtime shards 0-3, Docs Truth Check, and the
  aggregate Go vet/test/build job all succeeded.
- Staging identity: public `/health` reported proxy and sandbox both at
  `4da4ffa3fc9d6831e3d5643b6993aaba4ad67d9e`,
  `deployed_at=2026-06-18T05:48:23Z`, with `status=ok`, `upstream=ok`, and
  `vmctl_status=ok`.
- Formal probe command:
  `nix shell nixpkgs#nodejs_22 -c env CHOIR_DEPLOYED_BASE_URL=https://choir.news node scripts/texture_revision_cadence_probe.mjs`.
- Formal probe submission / trajectory:
  `ce488219-549d-439e-8f90-a6c20edf2318`; doc
  `f756294c-3610-4a7d-bf3e-97bc40e55665`.
- Formal probe revisions: V0 user at +0.274s, 53 chars; V1 appagent at
  +23.469s, 670 chars; V2 appagent at +67.701s, 1284 chars; V3 appagent at
  +86.062s, 1831 chars.
- Formal probe counts: `appagent_revision_count=3`, `total_revision_count=4`,
  `first_paint_ms=23469`, `web_search=6`, `source_search=2`, `spawn_agent=2`,
  `update_coagent=4`, `moment_count=166`, `agent_count=3`,
  `delegation_count=1`, final trajectory `state=completed`.
- Acceptance-enabled rerun: trajectory
  `7df99090-4ed4-4571-a63e-cb03ed5b2f78`; doc
  `8f5b2eea-519a-4814-91fc-1a651742df7e`; V1 at +49.355s with 65 chars, V2 at
  +83.212s with 1606 chars; `web_search=2`, `source_search=2`,
  `spawn_agent=2`, `update_coagent=2`, trajectory `state=completed`.
- RunAcceptanceRecord: `runacc-7760011a3b329bc50fb5`, target mission
  `mission-texture-long-running-agent-v0`, trajectory
  `7df99090-4ed4-4571-a63e-cb03ed5b2f78`, deployment/health commit
  `4da4ffa3fc9d6831e3d5643b6993aaba4ad67d9e`, CI run `27739387251`, deploy job
  `82063482453`, acceptance level `staging-smoke-level`, state `blocked`,
  checkpoints `submitted` and `texture_opened` passed.

Result: the cadence slice is product-path proven for one formal staging run and
the mission now has a durable staging-smoke RunAcceptanceRecord. This does not
settle the mission. The acceptance rerun shows the weak/late V1 branch still
exists, and T4-T8 remain open: default parked Texture lifecycle, cumulative
budget across sleep/rewarm, passivation-as-sleep, doc-delete cancellation, N:1
verifier proof, and a non-blocked acceptance record for the long-running
lifecycle are still missing.

Next move: continue T3/T4. Decide how the metadata-gated park waiter becomes the
default `texture:<docID>` lifecycle and how cumulative budget survives
sleep/rewarm, without regressing the deployed multi-revision cadence slice or
pretending the current wake-driven reconcile scaffolding is the final actor
model.

Rollback ref: revert `4da4ffa3` if subsequent deployed evidence shows the retry
guidance increases initial write failures or weakens the no-op guard; otherwise
the broader rollback for this mission remains reverting the mission commits back
through the last accepted checkpoint.

## 2026-06-18 - Bounded default Texture park lifecycle constructed locally (red construct)

Claim under test: the metadata-gated park waiter can become the default
Texture revision lifecycle without regressing the current multi-revision cadence
slice. A `texture:<docID>` revision run should write V1, remain resident and
parked while idle, receive a later `update_coagent` signal, and write V2 in the
same run instead of requiring a cold wake run.

Move: enable `actor_park_on_idle` metadata from runtime config for Texture
revision runs. Production `LoadConfig` now defaults
`RUNTIME_TEXTURE_ACTOR_PARK_IDLE` to two minutes, while hand-constructed test
configs remain zero unless they opt in. Add a comprehensive resident-Texture
test that drives the real `update_coagent` tool into a parked revision run and
asserts V2, delivery marking, no pending update, no second Texture revision run,
and exact-first/unconstrained-follow-up tool choice. Update the legacy
debounced-wake test to use a stored nonresident Texture requester so it still
tests the cold wake path rather than accidentally depending on a live resident.

Expected ΔV: T4 descends from "park waiter exists but is metadata-only" to
"default Texture revision runs are bounded parked residents when config is
loaded." Actual ΔV: local construct passes focused and sharded runtime evidence.
Remaining T4/T5 error: process restart still passivates instead of sleeping and
resuming the same logical actor, budgets are not yet cumulative across
sleep/rewarm, and deployed product proof has not yet shown whether findings are
consumed by a resident parked run rather than a cold wake.

Receipts:

- Changed `internal/runtime/config.go`: added
  `DefaultTextureActorParkIdle = 2 * time.Minute`, config field
  `TextureActorParkIdle`, and `RUNTIME_TEXTURE_ACTOR_PARK_IDLE` loading.
- Changed `internal/runtime/texture_agent_revision.go`:
  `submitTextureAgentRevisionRun` stamps `actor_park_on_idle` and
  `actor_park_idle_seconds` when the config is positive.
- Changed `internal/runtime/texture_test.go`: added
  `TestTextureRevisionRunParksAndConsumesUpdateWithoutColdWake` and adjusted
  `TestSubmitResearchFindingsWakeUsesSameDebouncedPath` to preserve the
  nonresident cold-wake contract.
- Verification:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestTextureRevisionRunParksAndConsumesUpdateWithoutColdWake|TestSubmitResearchFindingsWakeUsesSameDebouncedPath|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureCreatedSuperEvidenceWakesTextureV2|TestRunToolLoopParkWaiterBlocksWithoutProviderCallsUntilInjectedTurn|TestRuntimeAgentSignalWakesParkWaiter|TestTextureActorToolLoopBudgetDefaultsAndOverrides|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit|TestInitialTextureNoOpPatchRetriesIntoUsefulDraft' -count=1`;
  `nix develop -c go test ./internal/runtime -run 'TestLoadConfigDefaultsResearcherCount|TestLoadConfigReadsResearcherCount' -count=1`;
  `nix develop -c go test ./cmd/doccheck -count=1`;
  `git diff --check`;
  `nix develop -c scripts/go-test-runtime-shards`.

Result: bounded T4 is locally constructed but not settled. Next move is land,
push, monitor CI/deploy, verify staging identity, and run the deployed cadence
probe. Settlement remains open until deployed proof plus T5-T8 close the restart,
budget, cancellation, verifier, and acceptance-record gaps.

Rollback ref: revert the bounded default-park commit if staging shows worse
first-paint/cadence behavior or stuck resident Texture runs. The config escape
hatch is `RUNTIME_TEXTURE_ACTOR_PARK_IDLE=0` for emergency disablement if a
deploy config path is faster than code revert, but the durable rollback remains
reverting the commit.

## 2026-06-18 - Bounded default Texture park lifecycle deployed and accepted as blocked staging-smoke (red proof + green record)

Claim under test: commit `68c6e5b0b5dd4315719ee27cc11a861e8eaa70cb` can make
Texture revision actors park by default without regressing the deployed
prompt-bar cadence slice.

Move: push `68c6e5b0`, monitor GitHub Actions, verify staging identity, run the
deployed Texture cadence probe, then rerun the product proof and synthesize a
same-owner `RunAcceptanceRecord`.

Expected ΔV: deployed proof that fast V1 and V2+ cadence still hold with the
bounded default-park lifecycle enabled. Actual ΔV: two staging prompt-bar proofs
reached fast V1 and V2. A staging-smoke RunAcceptanceRecord was synthesized and
correctly remains `blocked` because T5-T8 are not yet settled.

Receipts:

- Commit: `68c6e5b0b5dd4315719ee27cc11a861e8eaa70cb`
  (`runtime: park texture revision actors by default`).
- GitHub Actions:
  - Docs Truth Check run `27740883099`: success.
  - FlakeHub run `27740883077`: success.
  - CI run `27740883113`: aggregate failure only because `Deploy to Staging
    (Node B)` job `82068117891` failed. CI jobs for Docs Truth Check, Go Test
    non-runtime, TLA+, deploy-impact detection, Go vet/build, runtime shards
    0-3, integration smoke, and aggregate Go vet/test/build all succeeded.
- Staging identity: `/health` reported proxy and sandbox both at
  `68c6e5b0b5dd4315719ee27cc11a861e8eaa70cb`, deployed at
  `2026-06-18T06:26:29Z`, with `status=ok`, `upstream=ok`, and
  `vmctl_status=ok`.
- Formal deployed probe command:
  `nix shell nixpkgs#nodejs_22 -c env CHOIR_DEPLOYED_BASE_URL=https://choir.news node scripts/texture_revision_cadence_probe.mjs`.
- Formal deployed probe submission / trajectory:
  `ed344528-ba08-425a-919e-fe813479f56c`; doc
  `dae4b2b1-78d3-4931-a708-f163e5487767`.
- Formal deployed probe revisions: V0 user at +0.317s, 53 chars; V1 appagent
  at +18.409s, 835 chars; V2 appagent at +68.035s, 1372 chars.
- Formal deployed probe counts: `appagent_revision_count=2`,
  `total_revision_count=3`, `first_paint_ms=18409`, `web_search=6`,
  `source_search=2`, `spawn_agent=2`, `update_coagent=2`, `moment_count=146`,
  `agent_count=3`, `delegation_count=1`, trajectory `state=completed`.
- Acceptance-enabled same-session proof: trajectory
  `1b99eff4-272d-4784-85d5-f5e43325cf2d`; doc
  `bc86ad0f-1502-4ba0-98c7-2376c1a77a5d`; V0 user at +0.256s, V1 appagent at
  +18.356s with 742 chars, V2 appagent at +60.096s with 1800 chars;
  `web_search=2`, `source_search=2`, `spawn_agent=2`, `update_coagent=2`,
  `moment_count=140`, trajectory `state=completed`.
- RunAcceptanceRecord: `runacc-60e41bc0a8f6cf708f3e`, target mission
  `mission-texture-long-running-agent-v0`, trajectory
  `1b99eff4-272d-4784-85d5-f5e43325cf2d`, deployment/health commit
  `68c6e5b0b5dd4315719ee27cc11a861e8eaa70cb`, CI run `27740883113`, deploy job
  `82068117891`, acceptance level `staging-smoke-level`, state `blocked`,
  checkpoints `submitted` and `texture_opened` passed.

Result: bounded default Texture parking is deployed and product-path accepted as
a blocked staging-smoke slice. This does not settle the full long-running actor
mission. The acceptance record remains blocked because continuation-level and
full lifecycle evidence are not proven: passivation-as-sleep, cumulative
sleep/rewarm budget, cancellation gaps, verifier N:1 updates, heresy/docs
updates, and a non-blocked lifecycle acceptance record remain open.

Next move: T5. Make a parked Texture actor survive process restart as logical
sleep and rewarm, preserve/charge cumulative budget across that boundary, and
prove no lost foreground updates or duplicate revisions.

Rollback ref: revert `68c6e5b0` if later staging evidence shows stuck resident
Texture runs or degraded first-paint/cadence. Emergency config escape hatch:
`RUNTIME_TEXTURE_ACTOR_PARK_IDLE=0`.

## 2026-06-18 - Texture restart rewarm and budget carry-forward constructed locally (red construct)

Claim under test: a parked/restarted Texture actor can resume as the same
logical `texture:<docID>` actor after process restart by passivating the old run,
staling its pending mutation, seeding the next activation from durable run
memory, and carrying prior provider-call/token budget spend into the new
activation.

Move: extend the role-neutral `ToolLoopBudget` with prior-spend fields and emit
`tool_loop_budget_usage` after each provider response. Texture revision run
submission now looks up the latest completed/passivated run-memory source for
the same owner+agent, derives provider-call/token spend from durable budget
usage events (falling back to provider-call preflight events), and stamps the
replacement activation with `actor_rewarm_source_loop_id` plus
`actor_budget_spent_*` metadata. The restart recovery test now uses a real
durable `update_coagent` row and proves rewarm snapshot, stale mutation,
cumulative budget metadata, update consumption, and recovered canonical revision.

Expected ΔV: T5 descends from "restart passivates then cold-reconciles without
budget continuity" to "replacement activation resumes the logical actor from
run-memory and charged provider/token spend." Actual ΔV: local construct passes
focused and sharded runtime evidence. Remaining T5 error: this is a replacement
activation seeded from memory, not literal same-goroutine continuation; elapsed
time budget remains per activation rather than active actor time across sleeps;
deployed staging proof is not yet run.

Receipts:

- Changed `internal/runtime/toolloop.go`: `ToolLoopBudget` now carries
  `SpentProviderCalls`, `SpentInputTokens`, and `SpentOutputTokens`; budget
  checks and exhaustion payloads include prior spend; `tool_loop_budget_usage`
  records cumulative usage after every provider response.
- Changed `internal/runtime/runtime.go`: `textureActorToolLoopBudget` reads
  prior-spend metadata, and `latestActorToolLoopBudgetSpend` reconstructs the
  latest actor budget baseline from durable events for the same owner+agent.
- Changed `internal/runtime/texture_agent_revision.go`: Texture revision runs
  stamp `actor_rewarm_source_loop_id`, `actor_budget_spent_*`, and
  `actor_budget_spend_source` when prior actor memory exists.
- Changed `internal/runtime/texture_test.go`: restart recovery now dispatches a
  real worker update, seeds interrupted Texture run-memory and budget events,
  and proves the replacement run has `actor_rewarm` memory, cumulative budget
  metadata, the worker update id, no duplicate pending mutation, and a recovered
  appagent revision.
- Verification:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestRunToolLoopBudgetCountsPriorProviderCalls|TestRunToolLoopBudgetLimitsProviderCalls|TestRunToolLoopBudgetLimitsCumulativeTokens|TestTextureActorToolLoopBudgetDefaultsAndOverrides|TestRestartRecoveryClearsInterruptedTextureMutationAndRelaunches' -count=1`;
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestTextureRevisionRunParksAndConsumesUpdateWithoutColdWake|TestSubmitResearchFindingsWakeUsesSameDebouncedPath|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureCreatedSuperEvidenceWakesTextureV2|TestRunToolLoopParkWaiterBlocksWithoutProviderCallsUntilInjectedTurn|TestRuntimeAgentSignalWakesParkWaiter|TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit|TestInitialTextureNoOpPatchRetriesIntoUsefulDraft|TestRestartRecoveryClearsInterruptedTextureMutationAndRelaunches|TestRunToolLoopBudgetCountsPriorProviderCalls' -count=1`;
  `nix develop -c go test ./internal/runtime -run 'TestTextureActorToolLoopBudgetDefaultsAndOverrides|TestRunToolLoopBudgetCountsPriorProviderCalls|TestRunToolLoopBudgetLimitsProviderCalls|TestRunToolLoopBudgetLimitsCumulativeTokens|TestLoadConfigDefaultsResearcherCount|TestLoadConfigReadsResearcherCount' -count=1`;
  `nix develop -c go test ./cmd/doccheck -count=1`;
  `git diff --check`;
  `nix develop -c scripts/go-test-runtime-shards`.

Result: T5 has local construct evidence for run-memory rewarm and cumulative
provider/token budget carry-forward. The mission is not settled. Next move is
land, push, monitor CI/deploy, run deployed cadence proof, then continue T6-T8
for document-deletion cancellation, verifier N:1 lifecycle proof, heresy/docs,
and a non-blocked lifecycle acceptance record.

Rollback ref: revert this T5 construct if CI or staging shows budget exhaustion
false positives, missing first paint, duplicate Texture revision runs, or stuck
pending mutations after restart. Broader mission rollback remains reverting the
mission commits back through the last accepted checkpoint.
