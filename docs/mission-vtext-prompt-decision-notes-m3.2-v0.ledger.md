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
