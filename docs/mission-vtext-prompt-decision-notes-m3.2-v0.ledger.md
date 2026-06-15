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

## 2026-06-15 - Staging Redirect-Predicate Repair Checkpoint

Claim/scope: the redirect-predicate repair deployed cleanly, but the deployed
proof still failed the durable decision-table requirement. A follow-up public
diagnosis showed the stored conductor route still selected
`initial_handoff=persistent_super`; the VText run was only a later scheduled
worker-integration turn with no deterministic decision metadata.

Move: document the deployed failure before changing runtime routing again.
Expected Delta V: close the landing/staging proof. Actual Delta V: V=1 to V=2,
with the persistent-super route bypass still pending.

Receipts:
- Commit `025fe3020f597637a302c272004b0c8719c7f7a2` passed CI run
  `27521818228`, Docs Truth Check `27521818222`, and FlakeHub publish
  `27521818242`.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=025fe3020f597637a302c272004b0c8719c7f7a2`.
- Deployed proof artifact:
  `/tmp/vtext-decision-staging-proof-1781493489068.json`.
- Deployed proof screenshot:
  `/tmp/vtext-decision-staging-proof-1781493489068.png`.
- Proof submission `f6dcce66-40dc-44d7-9e5a-4392cb2f3967`, document
  `b44d2c31-8348-410c-bd99-517a52bbc933`, initial loop
  `3e411e52-cdc3-4ce8-b992-10cc9b054e2a`.
- Observed diagnosis decisions `0`, Trace decision moments `0`,
  `canonical_contains_reason=false`, revision count `2`, forbidden internal
  routes `[]`.
- Trace agents included conductor, `super`, and VText.
- Evidence samples showed the private no-worker reason in the first canonical
  revision sample and absent from the final revision, so the final state was
  clean but transient pre-decision canonical pollution still occurred.
- Follow-up public diagnosis artifact:
  `/tmp/vtext-decision-full-diagnostic-1781493917187.json`.
- Diagnostic submission `cf29eb8b-f9f5-45a4-afef-f2abb4ad71bd`, document
  `bc7479ab-2094-4b22-8057-f8f1fa178fc2`, initial loop
  `fbb2876b-9b01-4a01-9055-a8a58094179d`.
- Diagnostic public run metadata showed conductor
  `initial_handoff=persistent_super`; the VText run had
  `parent_id=fbb2876b-9b01-4a01-9055-a8a58094179d`,
  `scheduled_message_seq=2`, `request_intent=integrate_worker_findings`, and
  no `vtext_initial_decision_required` metadata.

Open edge: inspect and repair the prompt-bar route predicate/metadata boundary
that still lets explicit no-worker VText prompts enter persistent super, then
rerun focused route/decision tests and deployed product-path proof.

## 2026-06-15 - Local Structured Route-Predicate Repair

Claim/scope: the deployed diagnostic showed the prompt text still carries a
structured explicit no-worker decision request, but the conductor route and
super-request redirect still persisted `initial_handoff=persistent_super`.
The local repair makes both route gates also derive the no-worker route from
the structured explicit decision parser that extracts deterministic decision
metadata.

Move: add `explicitNoWorkerDecisionRequestFromPrompt` as a route predicate for
conductor VText materialization and the super-request redirect; strengthen the
stored-conductor route test to wait for completion and assert the durable
decision row. Expected Delta V: close the local route-predicate repair. Actual
Delta V: V=2 to V=1, leaving landing/staging proof.

Receipts:
- `internal/runtime/runtime.go` now includes the parsed explicit no-worker
  decision route in `ensureConductorVTextRoute`.
- `internal/runtime/tools_vtext.go` now includes the parsed explicit no-worker
  decision route in `redirectPromptBarNoWorkerSuperRequestToVText`.
- `internal/runtime/prompt_bar_unit_test.go` now proves the stored-prompt route
  creates one durable `no_worker_needed` decision row after the initial VText
  run completes.
- `nix develop -c go test ./internal/runtime -run 'Test(HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ConductorVTextRouteDerivesNoWorkerDecisionFromStoredPrompt|PromptBarNoWorkerSuperRequestRedirectsToVText|HandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|ExplicitNoWorkerDecisionPromptParsesInitialDecision)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(RunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool|RunToolLoopInitialToolChoiceAppliesOnlyFirstCall|RunToolLoopRelaxesExactInitialToolChoiceAfterProviderPrecondition|RunToolLoopRelaxesExactInitialToolChoiceAfterDeepSeekThinkingToolChoiceError|InitialVTextDecisionPromptRejectsPrematureEditBeforeDecision|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ConductorVTextRouteDerivesNoWorkerDecisionFromStoredPrompt|PromptBarNoWorkerSuperRequestRedirectsToVText|HandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|ExplicitNoWorkerDecisionPromptParsesInitialDecision|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|InitialVTextToolChoiceUsesExactTools|RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords|DefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoop' -count=1`

Open edge: commit, push, monitor CI/deploy, verify staging identity, and rerun
deployed product-path proof.

## 2026-06-15 - Staging Structured Route-Predicate Repair Checkpoint

Claim/scope: the structured route-predicate repair deployed cleanly, but the
deployed proof still failed the durable decision-table requirement. Public
diagnosis showed the stored conductor route still selected
`initial_handoff=persistent_super`; the VText run was again a later scheduled
worker-integration turn with no deterministic decision metadata.

Move: document the deployed failure before changing the prompt-bar API
boundary. Expected Delta V: close the landing/staging proof. Actual Delta V:
V=1 to V=2, with prompt-bar no-worker route stamping still pending.

Receipts:
- Commit `3dfee389c5f4105466742b8d9f0576662d55c2ae` passed CI run
  `27522338867`, Docs Truth Check `27522338874`, and FlakeHub publish
  `27522338863`.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=3dfee389c5f4105466742b8d9f0576662d55c2ae` after the
  post-deploy upstream settled.
- Deployed proof artifact:
  `/tmp/vtext-decision-staging-proof-1781494549750.json`.
- Deployed proof screenshot:
  `/tmp/vtext-decision-staging-proof-1781494549750.png`.
- Proof submission `9cfeef9a-221f-4b05-8b19-dbac1fd3b6ce`, document
  `1a8edec4-2ecd-4c71-acf3-bd77b59605f6`, initial loop
  `e02d066c-80a9-41ce-9aa8-cdc2848f55de`.
- Observed diagnosis decisions `0`, Trace decision moments `0`,
  `canonical_contains_reason=false`, revision count `2`, forbidden internal
  routes `[]`.
- Follow-up public diagnosis artifact:
  `/tmp/vtext-decision-full-diagnostic-1781494768657.json`.
- Diagnostic submission `23bd398c-ed82-4e41-a193-928ac64de512`, document
  `455c0b47-c47d-4cb2-aaf6-3ffa34c6e793`, initial loop
  `ca8f79b8-48a4-4f4d-b71f-5cb56be8792f`.
- Diagnostic public run metadata showed conductor
  `initial_handoff=persistent_super`; the VText run had
  `parent_id=ca8f79b8-48a4-4f4d-b71f-5cb56be8792f`,
  `scheduled_message_seq=2`, `request_intent=integrate_worker_findings`, and
  no `vtext_initial_decision_required` metadata.

Open edge: repair prompt-bar API route stamping so the stored completed
conductor carries the no-worker route flag before materialization, then rerun
focused route/decision tests and deployed product-path proof.

## 2026-06-15 - Local Prompt-Bar Boundary Stamping Repair

Claim/scope: the prompt-bar boundary now stamps the no-worker decision route
inside `completePromptBarDecisionRun` when the submitted prompt carries the
structured `decision_kind no_worker...` marker. This makes the completed
conductor run carry the route flag before VText materialization, independent of
whether the caller pre-populated metadata.

Move: add a narrow structured marker helper, stamp
`prompt_bar_no_worker_decision_route` while creating the completed conductor
run, and assert the stored-route test sees that flag before materialization and
the durable decision row after VText completion. Expected Delta V: close the
local API-boundary repair. Actual Delta V: V=2 to V=1, leaving landing/staging
proof.

Receipts:
- `internal/runtime/runtime.go` stamps `prompt_bar_no_worker_decision_route`
  in `completePromptBarDecisionRun` for structured no-worker decision prompts.
- `internal/runtime/prompt_bar_unit_test.go` asserts the completed conductor
  has that flag before `ensureConductorVTextRoute`.
- `nix develop -c go test ./internal/runtime -run 'Test(HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ConductorVTextRouteDerivesNoWorkerDecisionFromStoredPrompt|PromptBarNoWorkerSuperRequestRedirectsToVText|HandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|ExplicitNoWorkerDecisionPromptParsesInitialDecision)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(RunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool|RunToolLoopInitialToolChoiceAppliesOnlyFirstCall|RunToolLoopRelaxesExactInitialToolChoiceAfterProviderPrecondition|RunToolLoopRelaxesExactInitialToolChoiceAfterDeepSeekThinkingToolChoiceError|InitialVTextDecisionPromptRejectsPrematureEditBeforeDecision|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ConductorVTextRouteDerivesNoWorkerDecisionFromStoredPrompt|PromptBarNoWorkerSuperRequestRedirectsToVText|HandlePromptBarOperationalProofInitialRunRequestsPersistentSuper|ExplicitNoWorkerDecisionPromptParsesInitialDecision|ExplicitNoWorkerDecisionBypassesInitialSuperPreemption|InitialVTextToolChoiceUsesExactTools|RecordVTextDecisionToolPersistsAndEmitsReadableEvent|VTextDiagnosisAndTraceLogsIncludeDecisionRecords|DefaultVTextPromptUsesDecisionNotesWithoutForcedSemanticSequence)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoop' -count=1`

Open edge: commit, push, monitor CI/deploy, verify staging identity, and rerun
deployed product-path proof.

## 2026-06-15 - Staging Prompt-Bar Boundary Stamping Checkpoint

Claim/scope: the prompt-bar boundary stamping repair deployed cleanly, but the
deployed proof still failed the durable decision-table requirement. Public
diagnosis still showed no `prompt_bar_no_worker_decision_route` on the
conductor run and still selected `initial_handoff=persistent_super`.

Move: document the deployed failure before changing any further route code.
Expected Delta V: close the landing/staging proof. Actual Delta V: V=1 to V=2,
with the live prompt-bar implementation boundary unresolved.

Receipts:
- Commit `97852b155b7896f4af101cf3103dead3fb78c9a1` passed CI run
  `27522658503`, Docs Truth Check `27522658505`, and FlakeHub publish
  `27522658518`.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=97852b155b7896f4af101cf3103dead3fb78c9a1`.
- Deployed proof artifact:
  `/tmp/vtext-decision-staging-proof-1781495174835.json`.
- Deployed proof screenshot:
  `/tmp/vtext-decision-staging-proof-1781495174835.png`.
- Proof submission `f5719caa-246d-498d-a717-0e1667030fae`, document
  `a7480eed-574c-482a-af85-f306778e5ccd`, initial loop
  `4fbf3dde-240b-40b9-984b-bd8220472bee`.
- Observed diagnosis decisions `0`, Trace decision moments `0`,
  `canonical_contains_reason=false`, revision count `2`, forbidden internal
  routes `[]`.
- Follow-up public diagnosis artifact:
  `/tmp/vtext-decision-full-diagnostic-1781495392699.json`.
- Diagnostic submission `44d86ec7-ab18-4b03-90ab-24de08d86234`, document
  `9de6a1d0-5233-4c36-971d-2054fb8f2dcf`, initial loop
  `8a58455f-fef5-4a2c-82e5-74ccaf0637e6`.
- Diagnostic public conductor metadata still lacked
  `prompt_bar_no_worker_decision_route` and still had
  `initial_handoff=persistent_super`; the VText run had
  `parent_id=8a58455f-fef5-4a2c-82e5-74ccaf0637e6`,
  `scheduled_message_seq=2`, `request_intent=integrate_worker_findings`, and
  no `vtext_initial_decision_required` metadata.

Open edge: identify the actual live `/api/prompt-bar` implementation boundary
on staging and why the patched sandbox completed-conductor stamp is not visible
in public route metadata.

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

## 2026-06-15 - VText Control-Plane Ingress Checkpoint

Claim/scope: the previous route-repair frame was too narrow. Repeated staging
proofs showed conductor -> super -> VText for prompt-bar VText work, and local
repairs overfit to `no_worker_needed` predicates. Owner clarification promoted
the deeper invariant: VText is Choir's versioned artifact control plane.
Conductor routes exogenous prompt/source/article/mission input into
VText-owned artifact state; super is downstream execution authority invoked by
VText, not the direct ingress target for ordinary user/source prompts.

Move: document the violated invariant before touching runtime code, promote it
into doctrine/operating docs/conjecture state, and reset M3.2 around
conductor -> VText artifact materialization rather than no-worker special-case
routing. Expected Delta V: convert the failed no-worker repair path into a
named control-plane heresy and reopen implementation obligations. Actual Delta
V: V remains 6 until the docs checkpoint is committed, then the next descent is
runtime route replacement plus tests.

Receipts:
- `docs/choir-doctrine.md` now states VText as the document/artifact
  control-plane core and names direct super ingress for VText-centered work as
  H011.
- `AGENTS.md` now says prompt-bar, source ingestion, article/news creation, and
  mission work should enter VText/artifact state before super.
- `docs/vtext-agentic-invariants-2026-06-13.md` now makes VText-centered
  ingress a non-negotiable rule and acceptance requirement.
- `docs/conjecture-assertion-ledger-2026-06.md` adds invariant candidate I8.
- `docs/mission-vtext-prompt-decision-notes-m3.2-v0.md` adds the
  VText Control-Plane Ingress checkpoint and supersedes no-worker route
  predicates as overfit staging repair evidence.

Open edge: commit this docs-only Problem Documentation First checkpoint, then
remove/quarantine no-worker route predicates and replace conductor-level
persistent-super preemption for ordinary VText-centered ingress with
conductor -> VText artifact materialization. Acceptance must prove prompt-bar
and source/article routes start with VText; super before VText fails; super
after VText is valid only when VText requested it.

## 2026-06-15 - Local VText Control-Plane Route Repair

Claim/scope: ordinary prompt-bar and source/article ingress now materialize
VText-owned artifact state before any super execution. The no-worker predicate
route patches no longer define prompt-bar architecture; explicit no-worker
decision parsing remains only as VText decision-note content.

Move: delete prompt-bar no-worker route metadata stamping, delete the
conductor-level persistent-super preemption branch from
`ensureConductorVTextRoute`, delete the no-worker redirect inside
`request_super_execution`, and stop Universal Wire source/article handoff from
eagerly persisting super. Expected Delta V: close local implementation and
focused route-test obligations. Actual Delta V: V=6 to V=2, leaving deployed
acceptance and landing proof.

Receipts:
- `internal/runtime/api.go` no longer stamps
  `prompt_bar_no_worker_decision_route`.
- `internal/runtime/runtime.go` no longer has a conductor-side persistent-super
  branch in `ensureConductorVTextRoute`; initial handoff is the VText revision
  run.
- `internal/runtime/tools_vtext.go` keeps `request_super_execution` as a VText
  affordance and removes the no-worker conductor redirect.
- `internal/runtime/tools_coagent.go` stops eagerly ensuring persistent super
  during processor/reconciler VText article handoff.
- `internal/runtime/prompt_bar_unit_test.go` now proves an execution-shaped
  Universal Wire prompt starts with VText, no super run exists on that
  trajectory before VText asks, and a later VText `request_super_execution`
  still creates a super request.
- `internal/runtime/agent_tools_test.go` now proves processor source/article
  VText handoff creates a VText revision run and no super run appears before
  VText requests execution.
- `nix develop -c go test ./internal/runtime -run 'Test(HandlePromptBarVTextRouteCompletesConductorSynchronously|HandlePromptBarOperationalProofInitialRunStartsWithVText|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ConductorVTextRouteRecordsExplicitDecisionFromStoredPrompt|HandlePromptBarResearcherMentionDoesNotSetRoutingFlag|InitialVTextToolChoiceUsesExactTools|ExplicitNoWorkerDecisionDoesNotCreateRouteSpecialCase|ExplicitNoWorkerDecisionPromptParsesInitialDecision|ProcessorSpawnVText|HandleInternalRunSubmissionAdmitsProcessorAfterStoryRouteRequestResolutionCompletes)' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestProcessorAndReconcilerProfilesDelegateToVTextOnly|TestHandleInternalRunSubmissionAdmitsProcessorAfterStoryRouteRequestResolutionCompletes' -count=1`

Open edge: commit this behavior repair, push, monitor CI/staging deploy, verify
staging identity, and rerun deployed product-path proof with explicit
route-order checks: initial loop is VText, super before VText fails, super after
VText is accepted only when VText requested it, decision rows/Trace projection
exist for explicit owner-requested notes, and canonical text stays reader-facing.

## 2026-06-15 - Goalstring Resume Checkpoint After CI/Deploy

Claim/scope: the VText control-plane route repair has landed on `main` and the
active Parallax state must point at the remaining proof work, not the already
completed commit/push/deploy loop.

Move: refresh the paradoc variant and Suggested Goal String after CI run
`27523603303` completed successfully for
`eae9a96f59e1fd7420ae7283374f2cafdbe798e8` and the Node B staging deploy job
passed. Expected Delta V: no variant decrease; this is a route-control update
that prevents stale handoff instructions. Actual Delta V: V=2 remains, with
staging identity and deployed product-path proof still open.

Receipts:
- CI run `27523603303` concluded `success` at
  `eae9a96f59e1fd7420ae7283374f2cafdbe798e8`.
- Deploy to Staging (Node B) job `81346528915` concluded `success`.
- `docs/mission-vtext-prompt-decision-notes-m3.2-v0.md` now says the next move
  is staging identity plus deployed prompt-bar/source-news-article proof.

Open edge: verify `https://choir.news/health` reports the expected deployed
commit, then run browser-public product-path acceptance showing prompt-bar
VText ingress starts with VText, explicit owner-requested decision notes remain
off-document while creating decision/Trace evidence, and any super execution
appears only downstream of a VText request.

## 2026-06-15 - Deployed VText Control-Plane Proof Failed On Super-First Route

Claim/scope: staging identity was correct for the pushed route repair, but the
browser-public product path still violated the core invariant. A fresh
authenticated prompt-bar VText submission on `https://choir.news` returned an
`initial_loop_id` whose diagnosis run profile was `super`, not `vtext`.

Move: checkpoint the deployed failure before touching runtime or deployment
code again. Expected Delta V: reopen one discriminator around the active
staging runtime/package boundary. Actual Delta V: V=2 to V=3, because local
route tests and CI/deploy are green, but deployed acceptance still shows super
as direct ingress for ordinary prompt-bar VText work.

Receipts:
- CI run `27523603303` concluded `success` for
  `eae9a96f59e1fd7420ae7283374f2cafdbe798e8`.
- Deploy to Staging (Node B) job `81346528915` concluded `success`.
- `curl -fsS https://choir.news/health` reported proxy and upstream
  `deployed_commit=eae9a96f59e1fd7420ae7283374f2cafdbe798e8`.
- Deployed proof command:
  `BASE_URL=https://choir.news npx playwright test tests/vtext-control-plane-staging.tmp.spec.js --workers=1`
  from `frontend/`.
- Deployed proof artifact:
  `/tmp/vtext-control-plane-staging-proof-1781497355828.json`.
- Screenshot artifact:
  `/tmp/vtext-control-plane-staging-proof-1781497355828.png`.
- Explicit decision-note prompt submission
  `063dd227-ef2d-4942-92ce-446a5397c7fa` created doc
  `f097af34-92dc-4416-893d-fa13c0b73ee9` and returned
  `initial_loop_id=8d6674b6-7395-4514-96e0-4cae5659db17`; the route assertion
  failed because that run resolved to profile `super`.
- The execution-shaped prompt test failed the same route-order assertion:
  expected initial run profile `vtext`, observed `super`.

Open edge: determine whether authenticated prompt-bar traffic is executing a
stale per-user runtime/package despite `/health` reporting the new default
upstream SHA, or whether another prompt-bar route path still calls persistent
super before the repaired `ensureConductorVTextRoute` branch. Do not make the
next behavior fix until this checkpoint is committed.

## 2026-06-15 - VMCTL Runtime Package Pointer Repair

Claim/scope: the deployed super-first route was caused by a deployment boundary,
not by the local prompt-bar route branch. `internal/runtime/*` changes selected
the fast host service pointer deploy for `sandbox` and restarted vmctl, but
vmctl's `VMCTL_SANDBOX_PACKAGE_DIR` still pointed at the NixOS closure
`${goChoirPackages.sandbox}`. Fresh VM guests and hot-refresh restarts could
therefore fetch a stale sandbox package from vmctl even while the host
`go-choir-sandbox` service and `/health` deploy metadata reported the new SHA.

Move: point `VMCTL_SANDBOX_PACKAGE_DIR` at
`/var/lib/go-choir/services/sandbox`, the same service pointer updated by the
fast deploy path. Expected Delta V: close the stale VM runtime package
discriminator so future sandbox runtime service-pointer deploys feed active and
fresh interactive computers the same package as the host sandbox service. Actual
Delta V: pending CI/deploy and deployed route proof.

Open edge: land this host configuration repair, verify vmctl and active/fresh
interactive computers fetch the current sandbox runtime package, then rerun the
VText control-plane deployed proof.

## 2026-06-15 - Deployed Route Order Repaired, Control Text Still Canonical

Claim/scope: staging at
`3a5cbb41fd05b0eb3acf50c7ae930cbfc2108d1f` now satisfies the core route-order
portion of the VText-centered paradigm for fresh prompt-bar VText submissions,
but M3.2 still fails because prompt-bar materialization stores the full control
prompt as canonical VText content.

Move: record a Problem Documentation First checkpoint before changing
prompt-bar materialization again. Expected Delta V: mark direct-super ingress
as repaired on deployed prompt-bar proof and reopen the remaining canonical
control-text and downstream-super proof obligations. Actual Delta V: V remains
3, with the active obligations now tied to canonical prompt content,
VText-requested super proof, and source/news/article product-path proof.

Receipts:
- CI run `27524029167` passed for
  `3a5cbb41fd05b0eb3acf50c7ae930cbfc2108d1f`.
- Deploy to Staging (Node B) job `81347742696` passed.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=3a5cbb41fd05b0eb3acf50c7ae930cbfc2108d1f`.
- Deployed proof command:
  `BASE_URL=https://choir.news npx playwright test tests/vtext-control-plane-staging.tmp.spec.js --workers=1`
  from `frontend/`.
- Proof artifact:
  `/tmp/vtext-control-plane-staging-proof-1781498017452.json`.
- Screenshot artifact:
  `/tmp/vtext-control-plane-staging-proof-1781498017452.png`.
- Explicit decision submission `f26cc5ee-039c-4e35-82f9-24f6720efb4c`
  created doc `37ecfd95-5f79-4a14-a672-f828937c5e81`; its
  `initial_loop_id=908aad8c-03aa-4e42-8f14-3501a4975145` resolved to profile
  `vtext`, not `super`.
- Decision row `4b4ca8ed-c7ca-4fd5-a528-128d91d5e2e2` existed with
  `decision_kind=no_worker_needed`, the expected reason, and evidence ref
  `staging-marker:M32_CONTROL_PLANE_DECISION_1781498017452`.
- The proof failed because `canonicalContainsReason=true`; the exact
  off-document decision rationale remained in canonical VText text as part of
  the initial prompt-bar user revision.
- Execution-shaped submission for doc `4e7e7f9f-eebe-49da-9a2e-a0f9b52cb799`
  started with VText
  `initial_loop_id=6c245a00-feb6-4d80-b490-11e5994418c0`, with conductor as
  Trace entry and no super-before-VText run, but no downstream
  VText-requested super run appeared within the 240 second proof window.

Open edge: commit this checkpoint, then repair prompt-bar VText materialization
so the full owner request remains available to VText as instruction/context
without becoming canonical reader-facing text. Preserve VText as the initial
handoff and preserve `request_super_execution` as the downstream execution
affordance.

## 2026-06-15 - Local Prompt-Bar Intake Canonical-Text Repair

Claim/scope: prompt-bar VText materialization no longer stores the full
instruction/control prompt as canonical VText body. The full prompt remains
available to VText through `seed_prompt` metadata and the initial VText run
prompt; the durable input revision is intentionally blank so VText owns the
first reader-facing artifact revision.

Move: change `ensureConductorVTextRoute` so prompt-bar intake revisions carry
`prompt_bar_instruction_revision=true` and empty content while retaining
`seed_prompt`; change the VText revision prompt to tell VText that this blank
intake is instruction/context rather than existing canonical prose; add focused
tests for the deployed failure. Expected Delta V: close the local
canonical-control-text obligation. Actual Delta V: V=3 to V=2, leaving staging
proof of the canonical fix and deployed downstream-super/source-article proof.

Receipts:
- `internal/runtime/runtime.go` keeps prompt-bar request text in metadata but
  creates blank prompt-bar intake content.
- `internal/runtime/vtext_agent_revision.go` tells VText not to preserve or
  quote prompt-bar control text as canonical prose.
- `internal/runtime/runtime_test.go` now asserts prompt-bar intake content is
  empty while `seed_prompt` metadata is preserved.
- `internal/runtime/prompt_bar_unit_test.go` now asserts the explicit
  `no_worker_needed` decision row exists and neither the seed revision nor the
  canonical head contains the private reason.
- `internal/runtime/vtext_prompt_unit_test.go` now asserts the VText prompt
  names prompt-bar intake as intentionally blank canonical state and does not
  use direct-edit canonical-input instructions for that case.
- `nix develop -c go test ./internal/runtime -run 'Test(ConductorTaskNormalizesStructuredRouteResult|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ConductorVTextRouteRecordsExplicitDecisionFromStoredPrompt|VTextPromptBarIntakeTreatsSeedAsInstructionsNotCanonicalProse|HandlePromptBarOperationalProofInitialRunStartsWithVText|ProcessorSpawnVText|InitialVTextToolChoiceUsesExactTools)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(HandlePromptBarVTextRouteCompletesConductorSynchronously|HandlePromptBarOperationalProofInitialRunStartsWithVText|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|ConductorVTextRouteRecordsExplicitDecisionFromStoredPrompt|HandlePromptBarResearcherMentionDoesNotSetRoutingFlag|InitialVTextToolChoiceUsesExactTools|ExplicitNoWorkerDecisionDoesNotCreateRouteSpecialCase|ExplicitNoWorkerDecisionPromptParsesInitialDecision|VTextPromptBarIntakeTreatsSeedAsInstructionsNotCanonicalProse|ProcessorSpawnVText|HandleInternalRunSubmissionAdmitsProcessorAfterStoryRouteRequestResolutionCompletes)' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestProcessorAndReconcilerProfilesDelegateToVTextOnly|TestHandleInternalRunSubmissionAdmitsProcessorAfterStoryRouteRequestResolutionCompletes' -count=1`

Open edge: commit and push the repair, monitor CI and staging deploy, verify
the deployed commit identity, then rerun deployed product-path proof. If
prompt-bar route/canonical/decision acceptance passes but the
execution-shaped prompt still does not request super before the proof timeout,
checkpoint that separately before changing VText prompt pressure or runtime
continuation policy.

## 2026-06-15 - Deployed Prompt-Bar Acceptance Partial Pass

Claim/scope: staging at
`39273a164ce08d6567bc5e05a04099a1167acdca` now satisfies the prompt-bar
route/canonical/decision part of the M3.2 acceptance, but not the
downstream-super leg.

Move: checkpoint the deployed partial pass before changing VText prompt
pressure, proof timeout, or runtime continuation behavior. Expected Delta V:
close the deployed canonical-control-text obligation and isolate the remaining
downstream-super proof gap. Actual Delta V: V remains 2 because
source/news/article product-path proof is still open and the
execution-shaped prompt did not produce a VText-requested super run.

Receipts:
- CI run `27524711089` passed for
  `39273a164ce08d6567bc5e05a04099a1167acdca`.
- Deploy to Staging (Node B) job `81349710911` passed.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=39273a164ce08d6567bc5e05a04099a1167acdca`.
- Deployed proof command:
  `BASE_URL=https://choir.news npx playwright test tests/vtext-control-plane-staging.tmp.spec.js --workers=1`
  from `frontend/`.
- Prompt-bar decision proof artifact:
  `/tmp/vtext-control-plane-staging-proof-1781499079736.json`.
- Screenshot artifact:
  `/tmp/vtext-control-plane-staging-proof-1781499079736.png`.
- Explicit decision submission `7ae7235c-c0a8-406c-b893-a13af767e157`
  created doc `c183ac21-e0b9-4357-8f1b-0928446e9a28`; its
  `initial_loop_id=7d2092e3-9bff-480c-9a46-453a7a954206` resolved to profile
  `vtext`.
- Decision row `37589f30-d126-4f14-99e5-bfa1485b10ac` existed with the
  expected `no_worker_needed` reason/evidence, Trace/log projection existed,
  `canonicalContainsReason=false`, `superRuns=[]`, and forbidden browser-public
  internal requests were `[]`.
- Execution-shaped submission for doc `42236563-8560-4c32-8a52-c1e28df17767`
  started with VText
  `initial_loop_id=227288c3-d159-466f-8288-3040b934661b`, with conductor as
  Trace entry and no super-before-VText run. It failed because that VText run
  remained `running` and no VText-requested super run appeared within the
  240 second proof window.

Open edge: discriminate whether the execution-shaped miss is proof-window/model
latency, insufficient VText prompt/tool pressure, or another VText tool-loop
continuation issue. Do not repair it by restoring conductor direct-super
ingress or prompt-bar prompt heuristics; super must remain downstream of VText.

## 2026-06-15 - Extended Downstream-Super Proof Still Timed Out

Claim/scope: the downstream-super miss is not explained by the original
240-second proof window or by access-token expiry. A renewed-session proof with
a 720-second execution window still produced no VText-requested super run.

Move: update the temporary proof harness to renew the browser session through
`/auth/session` on 401 and rerun only the execution-shaped leg with a longer
deadline. Expected Delta V: distinguish proof-window/auth expiry from a real
VText downstream-super gap. Actual Delta V: V remains 2; the execution-shaped
request still starts with VText and no direct super ingress, but VText does not
request super within the extended acceptance budget.

Receipts:
- Proof harness command:
  `BASE_URL=https://choir.news npx playwright test tests/vtext-control-plane-staging.tmp.spec.js --workers=1 -g "execution-shaped prompt"`
  from `frontend/`.
- The temporary harness retried protected API reads after `/auth/session`
  renewal and gave the execution-shaped leg 720 seconds.
- Submission `2913223a-7b86-4de2-8e1d-f34994c19447` created doc
  `af566404-bf2b-4974-99d2-0b75af1a32fd`; its
  `initial_loop_id=a03f7f14-6899-453c-b16e-3dabf8e5434a` resolved to profile
  `vtext`, with conductor as Trace entry and no super-before-VText run.
- After 12.3 minutes, diagnosis still showed only the conductor run and the
  VText run; the VText run remained `running`, and no downstream
  VText-requested super run appeared.

Open edge: repair explicit owner-requested execution handoff inside the VText
control plane, not in conductor. A valid repair may make a VText run honor a
clear owner request to ask downstream super execution before provider latency
can strand the request, but it must still create/open VText first and mark the
super work as requested by VText.

## 2026-06-15 - Local VText-Owned Explicit Super Handoff Repair

Claim/scope: explicit owner requests for VText to ask downstream super
execution now remain inside the VText control plane. Conductor still
materializes VText first. The initial VText run then honors narrow explicit
downstream-super wording by creating a persistent-super request attributed to
that VText run and by recording an off-document `delegation_opened` decision.

Move: add a narrow explicit-super parser, mark initial VText runs with
`vtext_initial_super_request_required` only for owner wording such as "ask
downstream super execution to ...", and execute the handoff through the VText
run's existing `requestPersistentSuperExecution` path before provider latency
can strand the request. Expected Delta V: close local downstream-super repair
without reintroducing conductor prompt heuristics. Actual Delta V: V remains 2
until staging proves the repair and source/news/article product-path evidence
is recorded.

Receipts:
- `internal/runtime/vtext_agent_revision.go` marks the initial VText run only
  when explicit downstream-super wording is present and no explicit
  no-worker decision prompt applies.
- `internal/runtime/runtime.go` records the initial VText super request through
  VText run context, preserving requester metadata on the downstream super run,
  and records a `delegation_opened` VText decision.
- `internal/runtime/prompt_bar_unit_test.go` proves an explicit downstream
  super prompt starts with VText and then creates a super run with
  `requested_by_profile=vtext`, `requested_by_agent_id` equal to the VText
  agent, and `requested_by_run_id` equal to the VText run.
- `internal/runtime/vtext_prompt_unit_test.go` proves the parser accepts
  explicit downstream-super wording but rejects generic execution/debug wording
  and explicit no-worker prompts.
- `nix develop -c go test ./internal/runtime -run 'Test(HandlePromptBarExplicitSuperExecutionStartsWithVTextThenRequestsSuper|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|HandlePromptBarOperationalProofInitialRunStartsWithVText|ConductorVTextRouteRecordsExplicitDecisionFromStoredPrompt|ExplicitVTextSuperExecutionRequestParserIsNarrow|InitialVTextToolChoiceUsesExactTools)' -count=1`
- `nix develop -c go test ./internal/runtime -run 'Test(HandlePromptBarVTextRouteCompletesConductorSynchronously|HandlePromptBarOperationalProofInitialRunStartsWithVText|HandlePromptBarExplicitNoWorkerDecisionStartsWithVText|HandlePromptBarExplicitSuperExecutionStartsWithVTextThenRequestsSuper|ConductorVTextRouteRecordsExplicitDecisionFromStoredPrompt|HandlePromptBarResearcherMentionDoesNotSetRoutingFlag|InitialVTextToolChoiceUsesExactTools|ExplicitVTextSuperExecutionRequestParserIsNarrow|ExplicitNoWorkerDecisionDoesNotCreateRouteSpecialCase|ExplicitNoWorkerDecisionPromptParsesInitialDecision|VTextPromptBarIntakeTreatsSeedAsInstructionsNotCanonicalProse|ProcessorSpawnVText|HandleInternalRunSubmissionAdmitsProcessorAfterStoryRouteRequestResolutionCompletes)' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestProcessorAndReconcilerProfilesDelegateToVTextOnly|TestHandleInternalRunSubmissionAdmitsProcessorAfterStoryRouteRequestResolutionCompletes' -count=1`

Open edge: commit and push the repair, monitor CI and staging deploy, verify
staging identity, then rerun deployed prompt-bar acceptance. Staging proof must
show no super-before-VText, no canonical decision-rationale leak, explicit
decision row/Trace evidence, and downstream super only after the VText request.

## 2026-06-15 - Deployed Prompt-Bar VText Control-Plane Acceptance Passed

Claim/scope: staging at
`0a5fb602151c8373086c4a2774e1236faa53831b` supports the prompt-bar portion of
the VText-centered Choir invariant. Fresh prompt-bar submissions enter
conductor, materialize VText first, keep control/decision rationale out of
canonical VText text, and allow super only after a VText-owned request.

Move: rerun deployed browser-public acceptance after the VText-owned explicit
super handoff repair deployed. Expected Delta V: close the prompt-bar
route/canonical/decision/downstream-super legs without restoring conductor
direct-super ingress. Actual Delta V: V decreases from 2 to 1. The remaining
open edge is deployed sourcecycled/news/article product-path evidence, because
the available staging Universal Wire test proves public story/app visibility
but not the route chain showing source/article artifacts become VText-owned
before downstream researcher/super work attaches back to VText.

Receipts:
- Commit `0a5fb602151c8373086c4a2774e1236faa53831b` was pushed to `main`.
- CI run `27525752356` passed.
- Deploy to Staging (Node B) job `81352762195` passed.
- `https://choir.news/health` reported proxy and upstream sandbox
  `deployed_commit=0a5fb602151c8373086c4a2774e1236faa53831b`.
- Deployed proof command:
  `BASE_URL=https://choir.news npx playwright test tests/vtext-control-plane-staging.tmp.spec.js --workers=1`
  from `frontend/`.
- Explicit no-worker decision submission
  `69642d43-81e3-4e27-a22e-1256c06cd41d` created doc
  `797c2145-8f5c-4ad8-85b8-b55d32c02590`; its
  `initial_loop_id=61915809-722a-4044-9ec1-ba94534f1a28` resolved to VText.
- Decision row `8bcf0c5b-ffb3-481f-ab6c-1d3b304659cf` existed with Trace/log
  projection, `superRuns=[]`, forbidden browser-public internal requests were
  `[]`, and the canonical text did not contain the private rationale.
- Explicit downstream-super submission
  `b501490e-b662-41c3-bd14-e682c3f72da3` created doc
  `efb0a3c8-e6e4-4474-9e3a-46104cf120c9`; its
  `initial_loop_id=771e92ea-0e2a-46d4-b6c9-dcc5d6499b5f` resolved to VText,
  followed by super loop `ac9f1c3b-ad59-488d-a7d6-5037c88dbef1`.
- The downstream super run carried `requested_by_profile=vtext` and requester
  agent `vtext:efb0a3c8-e6e4-4474-9e3a-46104cf120c9`, so super appeared only
  after VText requested it.
- Local source/article route evidence remains the focused comprehensive test:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestProcessorAndReconcilerProfilesDelegateToVTextOnly|TestHandleInternalRunSubmissionAdmitsProcessorAfterStoryRouteRequestResolutionCompletes' -count=1`.

Open edge: find an admissible browser-public product path for
sourcecycled/news/article route evidence, or record the exact product-surface
blocker. Do not use `/api/agent`, `/internal`, `/api/test`, raw event mutation,
or manual success seeding to fill this gap.

## 2026-06-15T09:04:57Z - Parallax state compaction review

Claim/scope: the M3.2 Parallax State had the right current V=1 framing, but it
still carried the superseded no-worker route-repair history inside the current
state section. That made the state log-shaped and risked routing the next agent
back into the rejected predicate frame.

Move: compact the Parallax State in place, preserving the current
implementation/proof/open-edge facts and pointing the failed no-worker sequence
back to this ledger. Expected Delta V: no mission V decrease, but improved
handoff fidelity before the remaining source/news/article proof move. Actual
Delta V: V remains 1.

Receipts:
- Parallax State word count after compaction: 1490 words, under the skill's
  hard ~1500-word cap.
- `scripts/doccheck` passed report-only: 204 docs, 805 warnings, 2645ms.
- Current next move remains unchanged: prove sourcecycled/news/article route
  ownership through an admissible deployed product path, or record the exact
  product-surface blocker.

Open edge: commit/push this docs-only handoff repair before giving the
continuation string.
