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
