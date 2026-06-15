# Mission M3.2 - VText Prompt Register And Decision Notes Ledger

## 2026-06-15 - Paradoc Creation

Claim/scope: after M3.1 settles the emergency forced-workflow regression, M3
needs one more gate before lifecycle work resumes: VText prompt language must
explain why delegation matters without becoming passive or forced, and VText
needs an off-document decision channel so canonical documents do not become
agent work logs.

Move: construct paradoc and mission graph entry. Expected Delta V: planning
variant established, no implementation V decrease. Actual Delta V: V=6 opened
with typed obligations for checkpoint, schema/store, tool, Trace/log
projection, Sources-panel visibility, and prompt-register tests.

Receipts:
- `docs/mission-vtext-prompt-decision-notes-m3.2-v0.md`
- `docs/mission-vtext-prompt-decision-notes-m3.2-v0.ledger.md`
- `docs/mission-graph.yaml`

Open edge: implementation must still start with Problem Documentation First
because the mission touches protected VText tools/prompts, runtime store schema,
Trace/event projection, logs, and VText UI.

## 2026-06-14 - Problem Checkpoint Before Red-Surface Code

Claim/scope: current VText prompt defaults still include forced-sequence
language for broad task classes, and "record why not" pressure would pollute
canonical documents unless M3.2 provides an off-document decision path.

Move: construct Problem Documentation First checkpoint in the paradoc before
runtime/frontend edits. Expected Delta V: close the first obligation. Actual
Delta V: V=6 to V=5 at docs-level only.

Receipts:
- `docs/mission-vtext-prompt-decision-notes-m3.2-v0.md`
- `internal/runtime/prompt_defaults/vtext.md` inspection

Open edge: schema/tool/API/UI/prompt implementation and all product-path
evidence remain open; this checkpoint documents the hazard but does not repair
it.

## 2026-06-14 - Local M3.2 Implementation And Proof

Claim/scope: the M3.2 witness exists locally: VText can record audit-worthy
decisions off-document in Dolt, Trace/logs can read them, the VText diagnosis
API feeds them to the Sources panel, and prompt defaults/tool descriptions now
use reason-bearing delegation pressure without semantic tool-order forcing.

Move: construct schema/store/tool/event/API/UI/prompt batch and run focused
proof. Expected Delta V: close implementation obligations 1-5, leaving only
landing/staging. Actual Delta V: V=5 to V=1.

Receipts:
- `nix develop -c go test ./internal/store -run TestVTextDecisionRecordsAreOwnerScopedAndDocumentScoped -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords|DefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence|RecordVTextDecisionToolDescriptionKeepsDecisionsOffDocument|InstallDefaultAgentToolsProfiles|InitialVTextToolChoice)' -count=1`
- `nix develop -c go test ./internal/store -run 'TestVText(CreateDocument|DecisionRecordsAreOwnerScopedAndDocumentScoped|UpdateDocument|DocumentAliasRoundTrip)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(VTextPrompt|RecordVTextDecision|VTextDiagnosis|InstallDefaultAgentToolsProfiles|InitialVTextToolChoice)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(Trace|VText|RecordVTextDecision|DefaultVTextPrompt|RecordVTextDecisionToolDescription|InstallDefaultAgentToolsProfiles)' -count=1`
- `npm run build` from `frontend/` (passes; existing UniversalWireApp Svelte warnings remain)
- `npx playwright test tests/vtext-markdown-lineage.spec.js -g "VText Sources panel shows off-document decision notes separately"` from `frontend/`
- in-app Browser loaded `http://localhost:4173/`, saw `data-desktop`, and reported no console errors
- `git diff --check`

Open edge: no landing claim yet. Commit, push, CI/deploy monitoring, staging
identity, and deployed product-path proof remain required before settlement.

## 2026-06-15 - Staging Problem Checkpoint Before Prompt Compliance Repair

Claim/scope: the first deployed product-path proof showed the M3.2 construct
landed and deployed, but deployed VText did not call `record_vtext_decision`
even when the owner prompt explicitly asked for an off-document decision note.

Move: document the staging-discovered problem before changing prompt/tool
behavior. Expected Delta V: reopen the landing-only variant into a small repair
loop. Actual Delta V: V=1 to V=2, with the problem documented and the repair
still pending.

Receipts:
- GitHub Actions CI run `27517539570` passed for
  `890dbe6fafc413f7d301828c83a51cbe10705ad4`, including Node B staging deploy.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=890dbe6fafc413f7d301828c83a51cbe10705ad4`.
- Deployed proof artifact:
  `/tmp/vtext-decision-staging-proof-1781484637787.json`.
- Deployed proof screenshot:
  `/tmp/vtext-decision-staging-proof-1781484637787.png`.
- Proof submission:
  `a81945d4-15df-4e92-8602-012b55366cb3`.
- Proof document:
  `5b15afa6-705c-48cf-84ce-b20ee2b0c124`.
- Trace agents observed: conductor, super, and VText completed.
- Forbidden browser-public internal routes observed: none.
- Diagnosis decisions observed: `0`.
- Trace decision moments observed: `0`.

Open edge: strengthen explicit owner-requested decision-recording pressure
without reintroducing forced semantic researcher/super choreography, rerun
focused prompt/runtime tests, push, monitor CI/deploy, and rerun deployed
product-path proof.

## 2026-06-15 - Local Prompt Compliance Repair

Claim/scope: explicit owner requests to record an off-document VText decision
now create a clear tool obligation without making researcher/super delegation a
forced semantic sequence.

Move: strengthen VText prompt defaults, runtime profile augmentation, tool
description, and prompt tests. Expected Delta V: close the local repair loop.
Actual Delta V: V=2 to V=1, leaving landing/staging proof.

Receipts:
- `internal/runtime/prompt_defaults/vtext.md` now says explicit
  owner-requested off-document decision notes call `record_vtext_decision`
  unless the requested record would be false, unsafe, or outside VText
  authority.
- `internal/runtime/tool_profiles.go` carries the same runtime prompt
  augmentation.
- `internal/runtime/tools_vtext.go` tool description carries the explicit owner
  request obligation.
- `internal/runtime/vtext_prompt_unit_test.go` rejects losing that prompt/tool
  language while still checking for no forced semantic delegation sequence.
- `nix develop -c go test ./internal/runtime -run 'Test(DefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence|RecordVTextDecisionToolDescriptionKeepsDecisionsOffDocument|InstallDefaultAgentToolsProfiles|InitialVTextToolChoice)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords|DefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence|RecordVTextDecisionToolDescriptionKeepsDecisionsOffDocument|InstallDefaultAgentToolsProfiles|InitialVTextToolChoice)' -count=1`

Open edge: commit, push, monitor CI/deploy, verify staging identity, and rerun
deployed product-path proof.

## 2026-06-15 - Staging Super-Request Choke-Point Checkpoint

Claim/scope: the super-request choke-point repair deployed cleanly, but the
deployed proof still failed. The route still created a `super` initial loop
before VText, and neither VText diagnosis nor Trace showed a durable VText
decision record.

Move: document the deployed failure before another runtime route change.
Expected Delta V: close the landing/staging proof. Actual Delta V: V=1 to V=2,
with the staging super-first route still not intercepted.

Receipts:
- Commit `80883c5f34add2de0a77e1e5a193e314a6ca602d` passed CI run
  `27521430465`, Docs Truth Check `27521430463`, and FlakeHub publish
  `27521430481`.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=80883c5f34add2de0a77e1e5a193e314a6ca602d`.
- Deployed proof artifact:
  `/tmp/vtext-decision-staging-proof-1781492755257.json`.
- Deployed proof screenshot:
  `/tmp/vtext-decision-staging-proof-1781492755257.png`.
- Proof submission `09346891-ff2b-468b-9dda-c40d190370da`, document
  `e321e1a4-4271-4e8b-8552-e1a7e217f555`, initial loop
  `b96d830a-4f28-4546-9ec9-9773d3c7d123`.
- Observed diagnosis decisions `0`, Trace decision moments `0`,
  `canonical_contains_reason=false`, revision count `2`, forbidden internal
  routes `[]`.
- Trace agents still included conductor, `super`, and VText; `super` first
  appeared before VText.

Open edge: run a focused public Trace route diagnostic on deployed `80883c5f`
to identify why the new `requestPersistentSuperExecution` redirect did not
intercept the staging super-first route.

## 2026-06-15 - Public Trace Super-Request Redirect Diagnostic

Claim/scope: a focused public Trace diagnostic on deployed `80883c5f` showed
that the first super assignment still comes from the conductor loop using
VText requester identity. The new redirect hook is on the right function, but
its stored-conductor metadata guard is too strict for the deployed prompt-bar
run shape.

Move: capture route-level public product evidence before changing the guard.
Expected Delta V: distinguish wrong hook from over-strict predicate. Actual
Delta V: V=2 remains, but the repair target narrows to the redirect predicate.

Receipts:
- Diagnostic artifact:
  `/tmp/vtext-route-diagnostic-1781493103394.json`.
- Submission `1c68be03-a265-420f-8a32-618be6d37ba4`, document
  `474ab6fa-80d6-40fb-a9e4-4a166477aefc`, initial loop
  `3e2d5235-bec1-47bf-8e1f-67dd119fcd7e`.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=80883c5f34add2de0a77e1e5a193e314a6ca602d`.
- Stream sequence 5 was a `channel.message` on the conductor loop:
  `1c68be03-a265-420f-8a32-618be6d37ba4 -> super:4c`, role VText, kind
  assignment.
- Stream sequence 6 submitted the persistent `super` loop; stream sequence 17
  submitted VText from that super loop.
- Observed decision rows `0`; forbidden internal routes were not used.

Open edge: relax the super-request redirect predicate to rely on owner,
conductor profile, existing VText document channel, and durable no-worker
prompt text, then rerun focused route tests and deployed proof.

## 2026-06-15 - Local Redirect-Predicate Repair

Claim/scope: the super-request redirect no longer depends on stored prompt-bar
app metadata fields that may be absent on the deployed conductor run. It now
requires owner match, conductor profile, an existing VText document channel,
and durable prompt text that matches the explicit no-worker decision route.

Move: relax the over-strict predicate identified by public Trace. Expected
Delta V: close local repair and return to landing/staging proof. Actual Delta
V: V=2 to V=1.

Receipts:
- `internal/runtime/tools_vtext.go` removes the `input_source=prompt_bar` and
  `requested_app=vtext` requirements from the redirect predicate.
- `internal/runtime/prompt_bar_unit_test.go` now omits those metadata fields
  in `TestPromptBarNoWorkerSuperRequestRedirectsToVText`.
- `nix develop -c go test ./internal/runtime -run 'Test(HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ConductorVTextRouteDerivesNoWorkerDecisionFromStoredPrompt|PromptBarNoWorkerSuperRequestRedirectsToVText|HandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|ExplicitNoWorkerDecisionPromptParsesInitialDecision)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(RunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool|RunToolLoopInitialToolChoiceAppliesOnlyFirstCall|RunToolLoopRelaxesExactInitialToolChoiceAfterProviderPrecondition|RunToolLoopRelaxesExactInitialToolChoiceAfterDeepSeekThinkingToolChoiceError|InitialVTextDecisionPromptRejectsPrematureEditBeforeDecision|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ConductorVTextRouteDerivesNoWorkerDecisionFromStoredPrompt|PromptBarNoWorkerSuperRequestRedirectsToVText|HandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|ExplicitNoWorkerDecisionPromptParsesInitialDecision|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|InitialVTextToolChoiceUsesExactTools|RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords|DefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoop' -count=1`

Open edge: commit, push, monitor CI/deploy, verify staging identity, and rerun
deployed product-path proof.

## 2026-06-15 - Staging Route-Carrier Repair Checkpoint

Claim/scope: the route-carrier repair deployed cleanly, but the deployed proof
still failed the durable decision-table requirement. The proof observed a Trace
moment mentioning `no_worker_needed`, but inspection showed it was a
`channel.message` from `super` to VText rather than a
`vtext.decision.recorded` event or diagnosis decision row.

Move: document the deployed failure before another runtime route change.
Expected Delta V: close the landing/staging proof. Actual Delta V: V=1 to V=2,
with a remaining deployed conductor-to-super assignment path.

Receipts:
- Commit `081a411e88a8d81fb35f62f59c6eecae2baf22e6` passed CI run
  `27521119493`, Docs Truth Check `27521119488`, and FlakeHub publish
  `27521119461`.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=081a411e88a8d81fb35f62f59c6eecae2baf22e6`.
- Deployed proof artifact:
  `/tmp/vtext-decision-staging-proof-1781492090148.json`.
- Deployed proof screenshot:
  `/tmp/vtext-decision-staging-proof-1781492090148.png`.
- Proof submission `7903b196-f5f1-4ca2-a3d0-b82bc2faf68f`, document
  `fa2368bd-4f7d-45b4-aac2-b7993c875360`, initial loop
  `2a7fa7db-7e16-4b07-831d-da624d48efce`.
- Observed diagnosis decisions `0`, Trace decision-like moments `1`,
  `canonical_contains_reason=false`, revision count `2`, forbidden internal
  routes `[]`.
- The matching Trace moment `8834185b-fcfd-4f16-aaab-2fe1b11e0a22` was kind
  `channel.message`, agent `super`, summary beginning "VText decision note:
  no_worker_nee..."; it was not a durable VText decision record.
- Trace agents still included conductor, `super`, and VText; `super` remained
  the initial loop.

Open edge: inspect the route that still assigns the initial prompt to `super`
on staging, then repair it so explicit no-worker VText decision prompts start
with VText and persist the decision row before the first edit.

## 2026-06-15 - Local Super-Request Choke-Point Repair

Claim/scope: if a prompt-bar conductor no-worker route still reaches
`requestPersistentSuperExecution`, the runtime now redirects that request to an
initial VText revision run instead of dispatching a persistent-super update.
The redirected VText run records the deterministic `no_worker_needed` decision
before provider execution.

Move: repair the actual super-assignment choke point observed on staging.
Expected Delta V: close local repair and return to landing/staging proof.
Actual Delta V: V=2 to V=1.

Receipts:
- `internal/runtime/tools_vtext.go` detects prompt-bar conductor no-worker
  requests at the start of `requestPersistentSuperExecution` and redirects to
  `submitVTextAgentRevisionRun`.
- `internal/runtime/runtime.go` records redirected initial handoff metadata as
  `vtext_no_worker_redirect` instead of `persistent_super`.
- `internal/runtime/prompt_bar_unit_test.go` adds
  `TestPromptBarNoWorkerSuperRequestRedirectsToVText`, proving the choke-point
  returns a VText run and persists one `no_worker_needed` decision row.
- `nix develop -c go test ./internal/runtime -run 'Test(HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ConductorVTextRouteDerivesNoWorkerDecisionFromStoredPrompt|PromptBarNoWorkerSuperRequestRedirectsToVText|HandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|ExplicitNoWorkerDecisionPromptParsesInitialDecision)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(RunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool|RunToolLoopInitialToolChoiceAppliesOnlyFirstCall|RunToolLoopRelaxesExactInitialToolChoiceAfterProviderPrecondition|RunToolLoopRelaxesExactInitialToolChoiceAfterDeepSeekThinkingToolChoiceError|InitialVTextDecisionPromptRejectsPrematureEditBeforeDecision|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ConductorVTextRouteDerivesNoWorkerDecisionFromStoredPrompt|PromptBarNoWorkerSuperRequestRedirectsToVText|HandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|ExplicitNoWorkerDecisionPromptParsesInitialDecision|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|InitialVTextToolChoiceUsesExactTools|RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords|DefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoop' -count=1`

Open edge: commit, push, monitor CI/deploy, verify staging identity, and rerun
deployed product-path proof.

## 2026-06-15 - Staging Prompt-Bar Route Repair Checkpoint

Claim/scope: the prompt-bar no-worker route repair deployed, but deployed proof
still recorded zero decision rows and zero Trace decision moments. The route
shape changed: VText had two runs and the document had three revisions, while
the private reason still did not leak into canonical text.

Move: document the deployed failure before changing metadata or recording
behavior again. Expected Delta V: reopen landing-only variant into a VText run
metadata/recording repair. Actual Delta V: V=1 to V=2.

Receipts:
- Commit `6be05f87043553e07cebd56940c3d004deaeaebd` passed CI run
  `27520207638`, Docs Truth Check `27520207623`, and FlakeHub publish
  `27520207634`.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=6be05f87043553e07cebd56940c3d004deaeaebd`.
- Deployed proof artifact:
  `/tmp/vtext-decision-staging-proof-1781490150274.json`.
- Deployed proof screenshot:
  `/tmp/vtext-decision-staging-proof-1781490150274.png`.
- Proof submission `0f1b0472-a833-4370-9862-b268d93b6fd9`, document
  `5bbaccb7-fca4-45b9-94e7-f67379dee590`, initial loop
  `2cdcc3f6-00d4-4da0-9574-3c2f2e21f4aa`.
- Observed diagnosis decisions `0`, Trace decision moments `0`,
  `canonical_contains_reason=false`, revision count `3`, forbidden internal
  routes `[]`.
- Trace agents included conductor, `super`, and VText; VText run count was `2`.

Open edge: capture full public Trace for this route shape, identify both VText
runs and why neither records the deterministic no-worker decision, then repair
the metadata propagation or recording boundary.

## 2026-06-15 - Local Super-Execution Detector No-Worker Repair

Claim/scope: the prompt-bar route flag alone did not stop deployed initial
persistent-super preemption. A fresh public Trace diagnostic on deployed
`6be05f87043553e07cebd56940c3d004deaeaebd` showed
`initial_loop_id=a5028aa1-9cfb-46db-88ed-f9d6f2b9e9f9` was still the super run,
and VText was spawned from that run. The repair moves the explicit no-worker
guard into `vtextPromptNeedsSuperExecution` so the phrase "execution worker"
cannot trigger super preemption when the prompt also carries `no_worker_needed`.

Move: make `vtextPromptNeedsSuperExecution` return false for
`promptBarNoWorkerDecisionRoute` before scanning super markers; keep negative
coverage that ordinary operational proof prompts still request persistent super.
Expected Delta V: close the local detector repair. Actual Delta V: V=2 to V=1,
leaving landing/staging proof.

Receipts:
- Public diagnostic proof artifact:
  `/tmp/vtext-route-diagnostic-1781490432255.json`.
- `internal/runtime/runtime.go` excludes explicit no-worker decision routes in
  `vtextPromptNeedsSuperExecution`.
- `internal/runtime/vtext_prompt_unit_test.go` asserts the proof-style prompt
  no longer needs super execution.
- `nix develop -c go test ./internal/runtime -run 'Test(HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|HandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|ExplicitNoWorkerDecisionPromptParsesInitialDecision|InitialVTextDecisionPromptRejectsPrematureEditBeforeDecision|InitialVTextToolChoiceUsesExactTools|RunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(RunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool|RunToolLoopInitialToolChoiceAppliesOnlyFirstCall|RunToolLoopRelaxesExactInitialToolChoiceAfterProviderPrecondition|RunToolLoopRelaxesExactInitialToolChoiceAfterDeepSeekThinkingToolChoiceError|InitialVTextDecisionPromptRejectsPrematureEditBeforeDecision|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|HandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|ExplicitNoWorkerDecisionPromptParsesInitialDecision|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|InitialVTextToolChoiceUsesExactTools|RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords|DefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoop' -count=1`

Open edge: commit, push, monitor CI/deploy, verify staging identity, and rerun
deployed product-path proof.

## 2026-06-15 - Staging Partial Enforcement Checkpoint

Claim/scope: the exact initial tool enforcement repair deployed cleanly and
removed the canonical leak, but deployed proof still failed the core M3.2
decision-record requirement. Staging produced no decision rows or Trace decision
moments, while VText still wrote clean reader-facing revisions and opened a
super branch.

Move: document the deployed partial repair before making the next runtime
change. Expected Delta V: reopen landing-only variant into a first-turn decision
guarantee repair. Actual Delta V: V=1 to V=2, with the remaining guarantee
pending.

Receipts:
- Commit `44851c95d44b4308b21598a90cf3a5022221f17f` passed CI run
  `27518973675`, Docs Truth Check `27518973682`, and FlakeHub publish
  `27518973674`.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=44851c95d44b4308b21598a90cf3a5022221f17f`.
- Deployed proof artifact:
  `/tmp/vtext-decision-staging-proof-1781487682095.json`.
- Proof submission `919e7628-dbe2-4bd5-a9cf-e5b915ba3ece`, document
  `3382ae5d-699d-4d3a-81b8-848134e4e4e4`, initial loop
  `2b6c207c-8fdf-48ca-b0f5-860d26050439`.
- Observed decisions `0`, Trace decision moments `0`,
  `canonical_contains_reason=false`, revision count `3`, forbidden internal
  routes `[]`, Trace agents conductor + `super` + VText, VText run count `2`.

Open edge: inspect product Trace/log evidence for the first VText run to
distinguish missing exact decision selection, provider noncompliance, and
decision-tool execution failure; then repair the remaining first-turn decision
guarantee without turning ordinary VText agency into blanket over-recording.

## 2026-06-15 - Local First-Turn Decision Guarantee Repair

Claim/scope: explicit, owner-supplied `decision_kind no_worker_needed`
decision-note prompts now produce a durable VText decision row before the
provider can create a canonical edit. This is limited to the structured
no-worker decision request shape; ordinary VText decisions still depend on VText
agency and the `record_vtext_decision` affordance.

Move: parse the explicit no-worker decision note from the prompt into initial
VText run metadata, record it idempotently at the start of the VText tool-loop
activation, emit the normal `vtext.decision.recorded` event, and then start the
model on exact `edit_vtext` for the reader-facing revision. Expected Delta V:
close local first-turn decision guarantee. Actual Delta V: V=2 to V=1, leaving
landing/staging proof.

Receipts:
- `internal/runtime/vtext_agent_revision.go` carries explicit initial
  no-worker decision metadata into the VText run.
- `internal/runtime/runtime.go` records the initial VText decision before the
  provider turn and switches that narrow path to initial `edit_vtext`.
- `internal/runtime/vtext_prompt_unit_test.go` parses the deployed proof prompt
  into decision kind, reason, evidence ref, and next action.
- `internal/runtime/prompt_bar_unit_test.go` proves prompt-bar materialization
  creates a VText run with deterministic initial decision metadata and initial
  edit choice.
- `internal/runtime/vtext_test.go` proves the proof-style prompt records one
  `no_worker_needed` decision, creates one reader-facing appagent revision, and
  keeps the private reason out of canonical text.
- `nix develop -c go test ./internal/runtime -run 'Test(ExplicitNoWorkerDecisionPromptParsesInitialDecision|InitialVTextDecisionPromptRejectsPrematureEditBeforeDecision|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|InitialVTextToolChoiceUsesExactTools|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|RunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(RunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool|RunToolLoopInitialToolChoiceAppliesOnlyFirstCall|RunToolLoopRelaxesExactInitialToolChoiceAfterProviderPrecondition|RunToolLoopRelaxesExactInitialToolChoiceAfterDeepSeekThinkingToolChoiceError|InitialVTextDecisionPromptRejectsPrematureEditBeforeDecision|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ExplicitNoWorkerDecisionPromptParsesInitialDecision|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|InitialVTextToolChoiceUsesExactTools|RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords|DefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence)' -count=1`

Open edge: commit, push, monitor CI/deploy, verify staging identity, and rerun
deployed product-path proof.

## 2026-06-15 - Staging Detector-Level Route Repair Checkpoint

Claim/scope: the detector-level no-worker route repair deployed cleanly, but
the deployed product proof still recorded zero VText decisions and zero Trace
decision moments. The private no-worker reason stayed out of canonical text,
but Trace agent summary still showed `super` before VText, so staging still has
a super-first route path before the deterministic VText decision record exists.

Move: document the deployed failure before changing runtime routing again.
Expected Delta V: reopen landing-only variant into a focused deployed route
diagnostic. Actual Delta V: V=1 to V=2, with the remaining super-first route
path pending.

Receipts:
- Commit `9c11fab05c5d5f24e9d869a721a25a1455ce63b5` passed CI run
  `27520566861`, Docs Truth Check `27520566862`, and FlakeHub publish
  `27520566875`.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=9c11fab05c5d5f24e9d869a721a25a1455ce63b5`.
- Deployed proof artifact:
  `/tmp/vtext-decision-staging-proof-1781491001897.json`.
- Deployed proof screenshot:
  `/tmp/vtext-decision-staging-proof-1781491001897.png`.
- Proof submission `c0599cdc-5591-4fba-bb4b-372e066b44a6`, document
  `e3774182-f2c2-40f5-9268-fe5230652643`, initial loop
  `a89ff657-1530-4516-9d24-436407808b91`.
- Observed diagnosis decisions `0`, Trace decision moments `0`,
  `canonical_contains_reason=false`, revision count `2`, forbidden internal
  routes `[]`.
- Trace agent summary included conductor, `super`, and VText; `super` first
  appeared before VText.

Open edge: run a focused public Trace diagnostic for the deployed `9c11` path
to identify which route bypasses the no-worker guard, then repair that route
with local route coverage before another push/deploy/proof loop.

## 2026-06-15 - Public Trace Route Diagnostic

Claim/scope: a focused public Trace diagnostic on deployed `9c11` clarified
the remaining route bypass. The conductor creates the VText document, then the
initial assignment goes to `super`; `super` later wakes VText, and VText edits
without any decision row.

Move: capture route-level public product evidence before touching routing code.
Expected Delta V: identify whether the bypass is inside VText recording or
before VText owns the route. Actual Delta V: V=2 remains, but the repair target
narrows to conductor-to-super assignment.

Receipts:
- Diagnostic artifact:
  `/tmp/vtext-route-diagnostic-1781491412663.json`.
- Diagnostic screenshot:
  `/tmp/vtext-route-diagnostic-1781491412663.png`.
- Submission `005828b7-b74d-4316-99da-20edcf987916`, document
  `31757e3f-3429-42a4-8d68-f4f59a1dbaf0`, initial loop
  `d4d9a803-8bb4-4f41-a64e-e0150a08ff66`.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=9c11fab05c5d5f24e9d869a721a25a1455ce63b5`.
- Observed decision rows `0`, Trace moments `48`, forbidden internal routes
  `[]`.
- Trace order: conductor created the document revision, then emitted an
  assignment channel message to `super`; `super` invoked `update_coagent`;
  VText then started from the super loop and invoked `edit_vtext`.

Open edge: inspect and repair the conductor-to-super assignment path for
explicit no-worker VText decision prompts, then rerun focused local coverage and
the deployed product-path proof.

## 2026-06-15 - Local Route-Carrier Repair

Claim/scope: explicit no-worker VText decision prompts no longer depend on one
prompt-bar metadata flag surviving to the conductor route branch. The route now
derives the no-worker decision from all durable prompt carriers available at
the boundary, stamps the route flag before any persistent-super branch, and
preserves that seed prompt for deterministic VText decision metadata.

Move: repair the conductor-to-super assignment path identified by the public
Trace diagnostic. Expected Delta V: close local repair and return to
landing/staging proof. Actual Delta V: V=2 to V=1.

Receipts:
- `internal/runtime/runtime.go` builds a route prompt from parsed decision
  seed, conductor seed, run prompt, and stored seed metadata before evaluating
  the no-worker route guard.
- `internal/runtime/runtime.go` persists
  `prompt_bar_no_worker_decision_route=true` when the route guard derives from
  durable prompt text.
- `internal/runtime/prompt_bar_unit_test.go` adds
  `TestConductorVTextRouteDerivesNoWorkerDecisionFromStoredPrompt` for the
  deployed diagnostic prompt shape without a pre-stamped handler flag.
- `nix develop -c go test ./internal/runtime -run 'Test(HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ConductorVTextRouteDerivesNoWorkerDecisionFromStoredPrompt|HandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|ExplicitNoWorkerDecisionPromptParsesInitialDecision)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(RunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool|RunToolLoopInitialToolChoiceAppliesOnlyFirstCall|RunToolLoopRelaxesExactInitialToolChoiceAfterProviderPrecondition|RunToolLoopRelaxesExactInitialToolChoiceAfterDeepSeekThinkingToolChoiceError|InitialVTextDecisionPromptRejectsPrematureEditBeforeDecision|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ConductorVTextRouteDerivesNoWorkerDecisionFromStoredPrompt|HandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|ExplicitNoWorkerDecisionPromptParsesInitialDecision|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|InitialVTextToolChoiceUsesExactTools|RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords|DefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoop' -count=1`

Open edge: commit, push, monitor CI/deploy, verify staging identity, and rerun
deployed product-path proof.

## 2026-06-15 - Prompt-Bar Route-Contract Checkpoint

Claim/scope: the local no-worker route-preemption repair deployed cleanly, but
the fourth staging proof still created a `super` run, recorded zero VText
decisions, recorded zero Trace decision moments, and leaked the exact
no-worker reason into canonical VText text. The local predicate test did not
cover the full prompt-bar route that creates the document and selects initial
super versus initial VText.

Move: document the deployed route-contract failure before changing runtime
routing again. Expected Delta V: reopen the landing-only variant into a narrow
route-level repair. Actual Delta V: V=1 to V=2, with route-contract repair
pending.

Receipts:
- Commit `f0335bfedd48ccad5487c0addf7d02449801ab86` passed CI run
  `27518517656`, including Node B deploy.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=f0335bfedd48ccad5487c0addf7d02449801ab86`.
- Deployed proof artifact:
  `/tmp/vtext-decision-staging-proof-1781486690473.json`.
- Proof submission `8b77ba79-36cd-4988-9d80-cfc817e876cb`, document
  `a6c409c2-9113-486b-b252-4f86e084d531`, initial loop
  `8eb718bb-7471-4a64-8f8e-de2142a8912c`.
- Trace agents included conductor, `super`, and VText; observed diagnosis
  decisions `0`, Trace decision moments `0`, `canonical_contains_reason=true`,
  revision count `2`, forbidden internal routes `[]`.

Open edge: add route-level coverage for prompt-bar VText materialization so an
explicit `decision_kind no_worker_needed` prompt creates an initial VText
revision run with exact `record_vtext_decision` and no initial `super` run,
while ordinary execution/verification/mutation prompts still use super.

## 2026-06-15 - Exact Initial Tool Enforcement Checkpoint

Claim/scope: local route-level testing did not reproduce the initial super
preemption, but code inspection found a stronger root cause for the canonical
leak: exact initial tool choice only shapes the provider request. If the
provider returns a different tool call, the tool loop executes it through the
full registry.

Move: document the exact-tool enforcement gap before changing generic tool-loop
behavior. Expected Delta V: refine the route-contract repair into a tool-loop
enforcement repair. Actual Delta V: V=2 remains V=2, with the enforcement patch
pending.

Receipts:
- `RunToolLoop` sets `req.ToolChoice` from `WithInitialToolChoice` and filters
  `req.ToolDefinitions` on the first provider call.
- In the `tool_use` response branch, `executeTools(ctx, registry,
  resp.ToolCalls, emit)` receives the full registry and executes returned calls
  before validating that they match the exact initial choice.
- Therefore a model/provider can return `edit_vtext` during an exact
  `record_vtext_decision` initial VText turn, creating a canonical revision with
  private rationale before any decision record exists.

Open edge: enforce exact initial tool choice after provider response by rejecting
or retrying mismatched returned tool calls before execution, then cover the
behavior in focused tool-loop and VText route tests.

## 2026-06-15 - Local Exact Initial Tool Enforcement Repair

Claim/scope: exact initial tool choice now rejects mismatched returned tool
calls before execution. A provider/model that returns `edit_vtext` during an
exact `record_vtext_decision` initial turn no longer gets that edit executed;
the loop appends a retry reminder and reissues the exact initial tool choice.

Move: validate exact initial tool-choice responses before persisting assistant
tool-call messages or calling `executeTools`, then cover the generic tool loop
and VText prompt-bar route. Expected Delta V: close local enforcement repair.
Actual Delta V: V=2 to V=1, leaving landing/staging proof.

Receipts:
- `internal/runtime/toolloop.go` now checks exact initial tool calls before tool
  execution and emits an `initial_tool_choice` retry event on mismatch.
- `internal/runtime/toolloop_test.go` proves a returned `edit_vtext` call is not
  executed when exact `record_vtext_decision` is required, and the required
  decision tool is retried.
- `internal/runtime/vtext_test.go` proves the proof-style prompt records one
  `no_worker_needed` decision, creates one reader-facing appagent revision, and
  does not leak the private decision reason into canonical text even when the
  provider first tries `edit_vtext`.
- `internal/runtime/prompt_bar_unit_test.go` covers prompt-bar materialization
  for explicit no-worker decision prompts.
- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool|TestHandlePromptBarExplicitNoWorkerDecisionStartsWithVText|TestInitialVTextDecisionPromptRejectsPrematureEditBeforeDecision' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(RunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool|RunToolLoopInitialToolChoiceAppliesOnlyFirstCall|RunToolLoopRelaxesExactInitialToolChoiceAfterProviderPrecondition|RunToolLoopRelaxesExactInitialToolChoiceAfterDeepSeekThinkingToolChoiceError|InitialVTextDecisionPromptRejectsPrematureEditBeforeDecision|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|InitialVTextToolChoiceUsesExactTools|RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords)' -count=1`

Open edge: commit, push, monitor CI/deploy, verify staging identity, and rerun
deployed product-path proof.

## 2026-06-15 - No-Worker Decision Route-Preemption Checkpoint

Claim/scope: the exact initial tool-choice repair deployed cleanly, but the
third staging proof still produced zero decision rows and zero Trace decision
moments. The VText revision count reached two and canonical content contained
the decision rationale, proving the process note still leaked into document
text. Local inspection found a route-preemption cause: broad super-execution
markers such as "staging proof" and "execution" can divert the initial route
before VText records an explicit `no_worker_needed` decision.

Move: document the refined route-preemption problem before changing runtime
routing. Expected Delta V: reopen the landing-only variant into a narrow route
repair. Actual Delta V: V=1 to V=2, with route repair pending.

Receipts:
- Commit `d3b8277ff67459d2de47ab00f3d7f1de83725bbd` passed CI run
  `27518252699`, including Node B deploy.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=d3b8277ff67459d2de47ab00f3d7f1de83725bbd`.
- Deployed proof artifact:
  `/tmp/vtext-decision-staging-proof-1781486103418.json`.
- Proof submission `0e171d1a-e3d6-4109-9938-6a50cea58efb`, document
  `d25ca47e-ddbe-450c-ab7f-ae6b4daedf90`, initial loop
  `9f5af4c3-4303-46c2-9bee-64575a708225`.
- Observed diagnosis decisions `0`, Trace decision moments `0`,
  `canonical_contains_reason=true`, revision count `2`, forbidden internal
  routes `[]`.
- `internal/runtime/runtime.go` checks `vtextPromptNeedsSuperExecution` before
  starting the initial VText revision run; the marker list includes
  `"staging proof"` and `"execution"`.

Open edge: let explicit `decision_kind no_worker_needed` / no-worker
decision-note prompts reach VText decision recording instead of preemptive
initial super execution, without weakening super routing for real code,
artifact, verification, or mutation requests.

## 2026-06-15 - Local No-Worker Route-Preemption Repair

Claim/scope: explicit no-worker decision-note prompts now reach VText decision
recording instead of being preempted by broad initial super-execution markers.
Ordinary debug/fix/verify/product-mutation prompts still trigger super
execution.

Move: guard the initial super handoff with
`!vtextPromptExplicitlyRequestsNoWorkerDecision(combinedPrompt)` and test both
the no-worker bypass and ordinary mutation route. Expected Delta V: close local
route repair. Actual Delta V: V=2 to V=1, leaving landing/staging proof.

Receipts:
- `internal/runtime/runtime.go` skips initial super preemption for explicit
  `decision_kind no_worker_needed` / no-worker decision-note prompts.
- `internal/runtime/vtext_prompt_unit_test.go` proves the proof-style prompt
  still contains broad super markers but is recognized as a no-worker decision
  route, while a debug/fix/verify prompt remains super-worthy.
- `nix develop -c go test ./internal/runtime -run 'Test(ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|InitialVTextToolChoiceUsesExactTools|InitialVTextRunWritesFirstAppagentRevisionThroughEdit|RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|InitialVTextToolChoiceUsesExactTools|DefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence|RecordVTextDecisionToolDescriptionKeepsDecisionsOffDocument|RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords|InstallDefaultAgentToolsProfiles|InitialVTextRunWritesFirstAppagentRevisionThroughEdit)' -count=1`

Open edge: commit, push, monitor CI/deploy, verify staging identity, and rerun
deployed product-path proof.

## 2026-06-15 - Tool-Choice Root Cause Checkpoint

Claim/scope: the prompt/tool-description repair deployed cleanly, but the
second staging proof still saw zero decision records and zero Trace decision
moments before the auth polling session expired. Local code inspection then
identified a stronger root cause: initial VText revision runs force exact
`edit_vtext` as the first provider tool.

Move: document the refined staging/root-cause evidence before changing runtime
tool-choice behavior. Expected Delta V: reopen the landing-only variant into a
narrow runtime repair. Actual Delta V: V=1 to V=2, with the tool-choice repair
pending.

Receipts:
- Commit `9c62cc061114419dd0ce1a36a9f8b27a81fa222a` passed CI run
  `27517926004`, including Node B deploy.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=9c62cc061114419dd0ce1a36a9f8b27a81fa222a`.
- Deployed proof artifact:
  `/tmp/vtext-decision-staging-proof-1781485443266.json`.
- Proof submission completed, then evidence polling observed diagnosis
  decisions `0` and Trace decision moments `0` through the last successful
  sample at `2026-06-15T01:08:59.179Z`.
- `internal/runtime/runtime.go` applies
  `WithInitialToolChoice(initialVTextToolChoice(rec))` to VText revision runs,
  and `initialVTextToolChoice` returns exact `function:edit_vtext` for ordinary
  initial VText runs.
- `internal/runtime/vtext_prompt_unit_test.go` currently expects that exact
  initial `edit_vtext` choice for all initial cases.

Open edge: make explicit owner-requested decision notes select exact
`record_vtext_decision` for the initial VText tool choice, while preserving
exact `edit_vtext` for ordinary first-revision work and leaving worker-woken
turns free to choose.

## 2026-06-15 - Staging Metadata Guarantee Checkpoint

Claim/scope: the deterministic first-turn decision guarantee deployed cleanly,
but the deployed product path still recorded zero VText decisions and zero
Trace decision moments. The canonical leak stayed repaired, so the remaining
gap is the off-document accountability guarantee rather than document-body
pollution.

Move: document the deployed metadata/route failure before making another
runtime change. Expected Delta V: reopen landing-only variant into a
route/metadata repair. Actual Delta V: V=1 to V=2, with the route/metadata
repair pending.

Receipts:
- Commit `f244c5446f387ca0df9ef0ebed2188b75de38d17` passed CI run
  `27519472366`, Docs Truth Check `27519472379`, and FlakeHub publish
  `27519472396`.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=f244c5446f387ca0df9ef0ebed2188b75de38d17`.
- Deployed proof artifact:
  `/tmp/vtext-decision-staging-proof-1781488672918.json`.
- Deployed proof screenshot:
  `/tmp/vtext-decision-staging-proof-1781488672918.png`.
- Proof submission `72ef2f03-b3d5-4157-9166-52b378443e80`, document
  `f0740135-8059-403e-a6b3-6c9c4c003883`, initial loop
  `2c399a26-844b-4207-8d82-2b765c2fe401`.
- Observed diagnosis decisions `0`, Trace decision moments `0`,
  `canonical_contains_reason=false`, revision count `2`, forbidden internal
  routes `[]`.

Open edge: inspect and cover the prompt-bar-to-VText child-run metadata
boundary so explicit `decision_kind no_worker_needed` metadata reaches
`executeWithToolLoop` before provider execution; then rerun focused runtime
tests, push, monitor CI/deploy, verify staging identity, and rerun deployed
product-path proof.

## 2026-06-15 - Local Pre-Activation Decision Repair

Claim/scope: explicit no-worker VText decision records no longer depend on the
VText child goroutine reaching the tool-loop setup. The deterministic record is
created synchronously after the run row exists and before root or child
activation starts; the existing tool-loop hook remains as an idempotent fallback.

Move: move the initial decision record to the pre-activation boundary and
strengthen prompt-bar route coverage to wait for the VText child run and assert
the durable decision row. Expected Delta V: close the local route/metadata
repair. Actual Delta V: V=2 to V=1, leaving landing/staging proof.

Receipts:
- `internal/runtime/runtime.go` calls
  `recordExplicitInitialVTextDecisionIfNeeded` before `startRunAsync` for root
  runs and before the child activation goroutine starts in `StartChildRun`.
- `internal/runtime/prompt_bar_unit_test.go` now proves the prompt-bar route
  creates one durable `no_worker_needed` decision row for the initial VText
  child run, not only metadata on the pending run.
- `nix develop -c go test ./internal/runtime -run 'Test(HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|InitialVTextDecisionPromptRejectsPrematureEditBeforeDecision|ExplicitNoWorkerDecisionPromptParsesInitialDecision|InitialVTextToolChoiceUsesExactTools|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|RunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(RunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool|RunToolLoopInitialToolChoiceAppliesOnlyFirstCall|RunToolLoopRelaxesExactInitialToolChoiceAfterProviderPrecondition|RunToolLoopRelaxesExactInitialToolChoiceAfterDeepSeekThinkingToolChoiceError|InitialVTextDecisionPromptRejectsPrematureEditBeforeDecision|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ExplicitNoWorkerDecisionPromptParsesInitialDecision|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|InitialVTextToolChoiceUsesExactTools|RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords|DefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoop' -count=1`

Open edge: commit, push, monitor CI/deploy, verify staging identity, and rerun
deployed product-path proof for decision row, Trace decision moment, no
forbidden routes, and no private reason in canonical text.

## 2026-06-15 - Staging Pre-Activation Repair Checkpoint

Claim/scope: the pre-activation decision repair deployed and CI/staging deploy
passed, but the deployed product proof still recorded zero VText decisions and
zero Trace decision moments. The private reason stayed out of canonical text,
but Trace again showed `super` before VText.

Move: document the deployed failure before changing the route again. Expected
Delta V: reopen landing-only variant into a route-preemption investigation.
Actual Delta V: V=1 to V=2, with the super/VText ordering gap pending.

Receipts:
- Commit `916885ce5fde61a146a8317353ac6b2096cee4e6` passed CI run
  `27519880134`, including Node B staging deploy.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=916885ce5fde61a146a8317353ac6b2096cee4e6`.
- Deployed proof artifact:
  `/tmp/vtext-decision-staging-proof-1781489484677.json`.
- Deployed proof screenshot:
  `/tmp/vtext-decision-staging-proof-1781489484677.png`.
- Proof submission `68ff0f37-9b14-47ad-a711-ad5ebf0be660`, document
  `f6caec7a-a975-4b38-8a17-6b4804e8a9ec`, initial loop
  `efec393f-bc03-4cc9-871a-2b5caa14d3c9`.
- Observed diagnosis decisions `0`, Trace decision moments `0`,
  `canonical_contains_reason=false`, revision count `2`, forbidden internal
  routes `[]`.
- Trace agents included conductor, `super`, and VText; `super` first appeared
  before the VText run.

Open edge: determine whether the prompt-bar `initial_loop_id` is still a
super handoff, whether VText asks super before the deterministic decision path,
or whether the proof is following a later VText run. Then repair the actual
route and rerun local plus deployed product-path proof.

## 2026-06-15 - Local Prompt-Bar No-Worker Route Repair

Claim/scope: the deployed route gap was initial persistent-super preemption.
Public Trace diagnostic artifact
`/tmp/vtext-route-diagnostic-1781489784153.json` showed
`initial_loop_id=0e66ef35-accd-4113-874f-3d3451d8fb47` was the super run, and
the VText run was spawned from that super run. The repair adds a prompt-bar
metadata flag for explicit no-worker decision prompts and makes conductor VText
materialization honor it before persistent-super preemption.

Move: stamp `prompt_bar_no_worker_decision_route` for prompts containing the
structured `no_worker_needed` marker or "no research or execution worker"; use
that flag in `ensureConductorVTextRoute`; keep negative coverage for ordinary
operational proof prompts that still need persistent super. Expected Delta V:
close the local route-preemption repair. Actual Delta V: V=2 to V=1, leaving
landing/staging proof.

Receipts:
- Public diagnostic proof through `/api/prompt-bar`, `/api/vtext/*`, and
  `/api/trace/*`: `/tmp/vtext-route-diagnostic-1781489784153.json`.
- `internal/runtime/api.go` stamps the prompt-bar no-worker decision route flag.
- `internal/runtime/runtime.go` honors the flag before persistent-super
  preemption and keeps the previous explicit no-worker prompt detector.
- `internal/runtime/prompt_bar_unit_test.go` asserts the flag and durable
  decision row for the proof-style route.
- `internal/runtime/vtext_prompt_unit_test.go` covers the route predicate and a
  negative mutation prompt.
- `nix develop -c go test ./internal/runtime -run 'Test(HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|HandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|ExplicitNoWorkerDecisionPromptParsesInitialDecision|InitialVTextDecisionPromptRejectsPrematureEditBeforeDecision|InitialVTextToolChoiceUsesExactTools|RunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(RunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool|RunToolLoopInitialToolChoiceAppliesOnlyFirstCall|RunToolLoopRelaxesExactInitialToolChoiceAfterProviderPrecondition|RunToolLoopRelaxesExactInitialToolChoiceAfterDeepSeekThinkingToolChoiceError|InitialVTextDecisionPromptRejectsPrematureEditBeforeDecision|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|HandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|ExplicitNoWorkerDecisionPromptParsesInitialDecision|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|InitialVTextToolChoiceUsesExactTools|RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords|DefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoop' -count=1`

Open edge: commit, push, monitor CI/deploy, verify staging identity, and rerun
deployed product-path proof.

## 2026-06-15 - Local Initial Tool-Choice Repair

Claim/scope: explicit owner-requested decision notes now select exact
`record_vtext_decision` for the first initial VText tool call, while ordinary
initial VText work keeps exact `edit_vtext` and worker-woken VText turns remain
free to choose.

Move: add a narrow explicit decision-note detector to `initialVTextToolChoice`
and cover it in prompt/tool-choice tests. Expected Delta V: close the local
tool-choice repair. Actual Delta V: V=2 to V=1, leaving landing/staging proof.

Receipts:
- `internal/runtime/runtime.go` now returns
  `function:record_vtext_decision` for initial VText prompts that explicitly
  request an off-document decision note.
- `internal/runtime/vtext_prompt_unit_test.go` covers the explicit decision
  note case, ordinary initial `edit_vtext`, and worker-wake unconstrained
  behavior.
- `nix develop -c go test ./internal/runtime -run 'Test(InitialVTextToolChoiceUsesExactTools|DefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence|RecordVTextDecisionToolDescriptionKeepsDecisionsOffDocument|RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords|InstallDefaultAgentToolsProfiles)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(InitialVTextRunWritesFirstAppagentRevisionThroughEdit|InitialVTextToolChoiceUsesExactTools|RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords)' -count=1`

Open edge: commit, push, monitor CI/deploy, verify staging identity, and rerun
deployed product-path proof.
