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
