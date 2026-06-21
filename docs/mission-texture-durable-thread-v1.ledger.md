# Ledger: Texture Durable Thread v1

## 2026-06-20 - Clean Successor Created

Claim: the old `mission-texture-long-running-agent-v0.md` and especially its
ledger are too failure-shaped to be the next source program. They should remain
cautionary evidence, not the pattern a fresh agent matches.

Move: created `docs/mission-texture-durable-thread-v1.md` as the compact current
source program; retained the old mission as superseded historical evidence.

Expected ΔV: 0 implementation descent; this is mission-source cleanup.

Actual ΔV: 0. V starts at 12.

Receipts:
- `docs/mission-texture-durable-thread-v1.md`
- `docs/mission-texture-durable-thread-v1.ledger.md`
- `docs/mission-graph.yaml`
- `docs/mission-texture-long-running-agent-v0.md`

Open edge: v1 still needs code execution. The first high-information move is R0a
tests for runtime-owned `update_id` and owner-visible work-state revisions.

## 2026-06-21 - R0a Update Identity Cutover Started

Claim: the `update_coagent` identity half of R0a can be repaired locally without
strengthening the run-centric spine if durable identity is derived from the
normalized delivery envelope and payload after target resolution, while the model
schema stops requiring `update_id`.

Move: changed model-facing `update_coagent` so `update_id` is optional,
deprecated, and ignored for durable identity; runtime now derives an `upd-*`
handle from owner, sender, target, channel, trajectory, role/kind, structured
payload, and inline evidence content. Updated focused comprehensive tests to
submit without `update_id`, prove repeat idempotency, and prove a reused human
label with a different payload mints a distinct runtime-owned update. Removed
live-test prompt wording that instructed models to provide `update_id`.

Expected ΔV: -3 for variant items 1, 2, and 3 if tests prove the schema,
runtime idempotency, and no human-label collision.

Actual ΔV: -3. V is now 9. This repairs H024b locally but does not repair the
owner-visible work-state half of R0a or settle the durable-thread mission.

Receipts:
- `internal/runtime/tools_worker_update.go`
- `internal/runtime/agent_tools_test.go`
- `internal/runtime/texture_live_llm_workflow_test.go`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestResearcherSubmitCoagentUpdatePersistsEvidenceAndDedupes|TestSubmitWorkerUpdatePersistsStructuredNonPatchUpdate' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestAgentToolProfiles|TestToolRegistry|TestRuntimeToolRegistryAccessor' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestUpdateCoagent|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestSubmitWorkerUpdateWakeUsesSameDebouncedPath' -count=1`

Open edge: direct owner-triggered Texture work-state revisions remain unproven.
Next pass should test and implement that behavior without exact first
`patch_texture` forcing or role choreography.

## 2026-06-21 - Direct Revise Stops Exact First Patch

Claim: H024a pressure can be reduced without semantic role choreography by
distinguishing direct user-authored revise requests from prompt-bar first-paint
and grounded integrate wakes. Direct revise should require some durable action,
but not exact `patch_texture`, so Texture can choose an honest work-state
revision, delegation, blocker, or decision.

Move: `submitTextureAgentRevisionRun` now records `current_author_kind` in run
metadata. `initialTextureToolChoice` returns generic `required` for direct
`revise` requests against a user-authored current revision, while preserving
`function:patch_texture` for prompt-bar first-paint and update-coagent integrate
wakes. Added a unit case pinning this distinction.

Expected ΔV: -1 for variant item 5. This does not prove item 4 because no
end-to-end direct work-state revision test has yet asserted the canonical
revision content.

Actual ΔV: -1. V is now 8.

Receipts:
- `internal/runtime/runtime.go`
- `internal/runtime/texture_agent_revision.go`
- `internal/runtime/texture_prompt_unit_test.go`
- `nix develop -c go test ./internal/runtime -run 'TestInitialTextureToolChoiceRequiresPatchBeforeContinuation|TestAgentToolProfiles|TestToolRegistry|TestRuntimeToolRegistryAccessor|TestUpdateCoagent|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestSubmitWorkerUpdateWakeUsesSameDebouncedPath' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestResearcherSubmitCoagentUpdatePersistsEvidenceAndDedupes|TestSubmitWorkerUpdatePersistsStructuredNonPatchUpdate' -count=1`

Open edge: owner-visible work-state is still not proven. The next discriminator
is a focused direct revise test where pending/delegated work produces a canonical
Texture work-state revision rather than a trivial cleanup patch or hidden Trace
state.

## 2026-06-21 - Direct Work-State Revision Proven Locally

Claim: the remaining R0a owner-visible work-state obligation can be proven
without adding semantic role choreography by exercising the direct document
`/revise` path on a user-authored current revision. The first provider request
should be generic `required`, not exact `patch_texture`, and the resulting
canonical document state should honestly name pending/delegated work before
researcher delegation.

Move: added `TestDirectTextureReviseWritesWorkStateBeforeDelegatingResearch`.
The test submits a direct Texture revise request against a user-authored
document, verifies the first Texture tool choice is generic `required`, verifies
the appagent writes a canonical revision containing "Working revision while
researcher evidence is pending", and verifies a researcher run is delegated after
that work-state revision. Fixed the runtime metadata mismatch by defaulting blank
`request_intent` to `revise` in `submitTextureAgentRevisionRun`; the prompt layer
already used that default, but `initialTextureToolChoice` was reading the blank
metadata field.

Expected ΔV: -1 for variant item 4. Item 5 was already reduced by the prior
direct-revise policy slice.

Actual ΔV: -1. V is now 7. R0a has focused local evidence, but the mission is
not settled: durable same-thread mailbox, event-driven delivery, passivation /
resume, always-deep research, deletion of old scaffolding, and staging proof
remain open.

Receipts:
- `internal/runtime/texture_test.go`
- `internal/runtime/texture_agent_revision.go`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestTextureAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory|TestDirectTextureReviseWritesWorkStateBeforeDelegatingResearch' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestInitialTextureToolChoiceRequiresPatchBeforeContinuation|TestAgentToolProfiles|TestToolRegistry|TestRuntimeToolRegistryAccessor|TestUpdateCoagent|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestSubmitWorkerUpdateWakeUsesSameDebouncedPath' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestResearcherSubmitCoagentUpdatePersistsEvidenceAndDedupes|TestSubmitWorkerUpdatePersistsStructuredNonPatchUpdate|TestTextureAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory|TestDirectTextureReviseWritesWorkStateBeforeDelegatingResearch' -count=1`

Open edge: R0a is not a settlement boundary. Next move should inspect and test
the durable per-document thread/mailbox replacement for pending-update
rediscovery and cold reconstruction.

## 2026-06-21 - Durable Mailbox Cursor Substrate

Claim: the next safe cutover step toward one durable Texture thread per document
is a first-class processed cursor for the addressed actor mailbox. The durable
log remains `worker_updates`; the cursor must represent the highest contiguous
delivered worker-update message sequence and must not skip an undelivered older
update.

Move: added `coagent_mailboxes` to the runtime store schema with
`owner_id`, `agent_id`, `channel_id`, and `processed_message_seq`. Delivery
marking now refreshes this cursor transactionally after `worker_updates` rows
are marked delivered. Added store tests for ordinary terminal delivery and for
out-of-order delivery: marking sequence 2 first leaves the cursor at 0 until
sequence 1 is delivered, then advances to 2.

Expected ΔV: -1 for variant item 6 at store/data-model scope. This does not
claim event-driven delivery cutover or same-thread resume.

Actual ΔV: -1. V is now 6.

Receipts:
- `internal/store/store.go`
- `internal/store/store_test.go`
- `nix develop -c go test ./internal/store -run 'TestUpdateRunAndMarkWorkerUpdatesDelivered|TestCoagentMailboxCursorRequiresContiguousDeliveredUpdates' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestInitialTextureToolChoiceRequiresPatchBeforeContinuation|TestAgentToolProfiles|TestToolRegistry|TestRuntimeToolRegistryAccessor|TestUpdateCoagent|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestSubmitWorkerUpdateWakeUsesSameDebouncedPath' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestResearcherSubmitCoagentUpdatePersistsEvidenceAndDedupes|TestSubmitWorkerUpdatePersistsStructuredNonPatchUpdate|TestTextureAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory|TestDirectTextureReviseWritesWorkStateBeforeDelegatingResearch' -count=1`

Open edge: runtime still discovers pending work from `worker_updates.delivered_at
IS NULL`; next pass should route Texture wake/delivery through the mailbox cursor
and prove event-driven delivery without cold reconstruction.

## 2026-06-21 - Cursor-Backed Backlog Reads Wired

Claim: the mailbox cursor can become the actor-facing read boundary before the
full same-thread Texture actor is settled, as long as the bridge does not claim
to replace event-driven delivery. Runtime should ask for a coagent mailbox
backlog, not generic pending worker rows, whenever it decides whether to wake an
actor or inject addressed update packets.

Move: added `ListCoagentMailboxBacklog` and `ListCoagentMailboxBacklogAll`.
These APIs read updates whose `message_seq` is after the durable
`coagent_mailboxes.processed_message_seq`; during this bridge slice they still
filter `delivered_at IS NULL` so already-consumed rows are not replayed. Routed
boot sweep, persistent super wake, generic coagent wake, Texture wake, warm
turn injection, parked-wait readiness, cold initial packet prep, Texture
delivery marking, and pending evidence collation through those backlog APIs.
Extended store tests to prove the per-actor cursor keeps out-of-order delivery
conservative and that the all-actor boot backlog skips rows already processed
for one actor while still surfacing later rows for that actor and initial rows
for another actor.

Expected ΔV: 0. This reduces pending-row rediscovery pressure but does not
complete variant item 7 because update delivery still uses `delivered_at` as a
compatibility guard and still injects packets into runs rather than appending
them as literal turns in one durable Texture thread.

Actual ΔV: 0. V remains 6.

Receipts:
- `internal/store/store.go`
- `internal/store/store_test.go`
- `internal/runtime/runtime.go`
- `internal/runtime/super_controller.go`
- `internal/runtime/texture_controller.go`
- `internal/runtime/texture_evidence_sources.go`
- `nix develop -c go test ./internal/store -run 'TestUpdateRunAndMarkWorkerUpdatesDelivered|TestCoagentMailboxCursorRequiresContiguousDeliveredUpdates|TestCoagentMailboxBacklogAllUsesActorCursors' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestUpdateCoagent|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestSubmitWorkerUpdateWakeUsesSameDebouncedPath|TestInitialTextureToolChoiceRequiresPatchBeforeContinuation' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestResearcherSubmitCoagentUpdatePersistsEvidenceAndDedupes|TestSubmitWorkerUpdatePersistsStructuredNonPatchUpdate|TestTextureAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory|TestDirectTextureReviseWritesWorkStateBeforeDelegatingResearch' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestTextureRevisionRunParksAndConsumesUpdateWithoutColdWake|TestCoagentRewarmUsesResidentActivationNotActiveRunProxy|TestCoagentRewarmIgnoresBlockedHistoricalActivation' -count=1`

Open edge: the next pass must stop treating `delivered_at` as the actor-facing
mailbox authority and prove new worker updates become same-thread turns for a
resident or resumed Texture document thread.

## 2026-06-21 - Cursor Authority And Same-Run Restart Reactivation

Claim: the next useful discriminator is whether the mailbox cursor, not the
legacy delivered audit column, decides actor-visible backlog, and whether a
passivated interrupted Texture activation can resume as the same durable run
with the new mailbox packet appended to its existing memory.

Move: removed `delivered_at IS NULL` from the coagent mailbox backlog APIs.
Out-of-order delivered audit marks no longer hide rows after the cursor: if the
cursor is still behind, backlog returns those rows for contiguous actor
processing. Added a latest-passivated-run lookup and a Texture-specific
reactivation path in `reconcileTextureAgentWake`; it reopens a stale mutation,
marks the passivated run pending, preserves budget continuity metadata, skips
cold-prepend delivery, and injects fresh mailbox packets into the existing
run-memory log before the next provider call. Updated the restart recovery test
to require the original Texture `loop_id` to complete, no replacement Texture
revision run to be minted, the previous memory entry to remain first, and the
mailbox packet to be persisted as a resumed user turn.

Expected ΔV: 0. This directly narrows variant items 7 and 8, but does not settle
them: normal idle quiescence still completes the Texture run, so a later update
can still require a new activation seeded from prior memory.

Actual ΔV: 0. V remains 6.

Receipts:
- `internal/store/store.go`
- `internal/store/texture.go`
- `internal/store/store_test.go`
- `internal/runtime/runtime.go`
- `internal/runtime/texture_controller.go`
- `internal/runtime/texture_test.go`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestRestartRecoveryReactivatesInterruptedTextureRun' -count=1`
- `nix develop -c go test ./internal/store -run 'TestUpdateRunAndMarkWorkerUpdatesDelivered|TestCoagentMailboxCursorRequiresContiguousDeliveredUpdates|TestCoagentMailboxBacklogAllUsesActorCursors' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestUpdateCoagent|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestSubmitWorkerUpdateWakeUsesSameDebouncedPath|TestInitialTextureToolChoiceRequiresPatchBeforeContinuation' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestTextureRevisionRunParksAndConsumesUpdateWithoutColdWake|TestCoagentRewarmUsesResidentActivationNotActiveRunProxy|TestCoagentRewarmIgnoresBlockedHistoricalActivation|TestRestartRecoveryReactivatesInterruptedTextureRun' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestResearcherSubmitCoagentUpdatePersistsEvidenceAndDedupes|TestSubmitWorkerUpdatePersistsStructuredNonPatchUpdate|TestTextureAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory|TestDirectTextureReviseWritesWorkStateBeforeDelegatingResearch' -count=1`

Open edge: idle quiescence must become passivation/sleep for Texture, not normal
completion. The next proof should create a Texture run that reaches idle, verify
it remains a sleeping/passivated durable thread with no provider calls, then send
`update_coagent` and require the same `loop_id` to resume and write the next
revision.

## 2026-06-21 - Idle Quiescence Becomes Texture Sleep

Claim: normal Texture idle quiescence must be a sleeping durable actor state,
not a completed run that later needs reconstruction from document head and
channel history. A same-loop mailbox packet may also be consumed without an
immediate canonical write, so checkpoint advancement cannot depend only on the
Texture write path.

Move: taught the tool loop to return an explicit passivation signal when an
idle park waiter expires, and made Texture idle expiry persist the run as
`passivated` with `actor_sleep_state=idle` while moving the agent mutation to
`sleeping`. Sleeping mutations can reactivate the same `loop_id`; reactivation
injects mailbox packets into existing run memory instead of cold-prepending a
new activation. Same-loop consumed Texture updates now advance the controller
checkpoint monotonically even when no revision write occurs before the actor
sleeps again.

Expected ΔV: -1 for variant item 8 at focused runtime scope. This narrows item
7 but does not settle the full event-driven cutover because cold-prepend and
replacement wake compatibility paths still exist.

Actual ΔV: -1. V is now 5.

Receipts:
- `internal/runtime/toolloop.go`
- `internal/runtime/runtime.go`
- `internal/runtime/super_controller.go`
- `internal/store/texture.go`
- `internal/runtime/texture_test.go`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestTextureUpdateCoagentDuringActiveRevisionTriggersSameRunFollowUp|TestTextureIdlePassivatesAndResumesSameRun|TestTextureRevisionRunParksAndConsumesUpdateWithoutColdWake|TestRestartRecoveryReactivatesInterruptedTextureRun' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoopParkWaiterBlocksWithoutProviderCallsUntilInjectedTurn|TestUpdateCoagent|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestSubmitWorkerUpdateWakeUsesSameDebouncedPath|TestInitialTextureToolChoiceRequiresPatchBeforeContinuation' -count=1`
- `nix develop -c go test ./internal/store -run 'TestUpdateRunAndMarkWorkerUpdatesDelivered|TestCoagentMailboxCursorRequiresContiguousDeliveredUpdates|TestCoagentMailboxBacklogAllUsesActorCursors' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestResearcherSubmitCoagentUpdatePersistsEvidenceAndDedupes|TestSubmitWorkerUpdatePersistsStructuredNonPatchUpdate|TestTextureAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory|TestDirectTextureReviseWritesWorkStateBeforeDelegatingResearch' -count=1`

Open edge: remove the remaining cold-prepend/replacement wake scaffolding for
Texture delivery, then delete the classifier and exact-first-tool residues that
still encode semantic workflow policy in runtime code.

## 2026-06-21 - Sandbox Preserves Texture Sleep Config

Claim: the focused runtime sleep/passivation repair does not reach staging if
the sandbox runtime config drops `TextureActorParkIdle` while copying the
`LoadConfig` result into the sandbox-specific config.

Move: preserved `TextureActorParkIdle` through `cmd/sandbox`'s
`buildRuntimeConfig` and added a regression assertion beside the existing
runtime-config handoff test. This is a staging-facing config repair, not a new
thread primitive.

Expected ΔV: 0. This removes a staging drift blocker for item 12 but does not
settle staging acceptance.

Actual ΔV: 0. V remains 5.

Receipts:
- `cmd/sandbox/main.go`
- `cmd/sandbox/main_test.go`
- `nix develop -c go test ./cmd/sandbox -run TestBuildRuntimeConfigPreservesHostServiceURLs -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestTextureUpdateCoagentDuringActiveRevisionTriggersSameRunFollowUp|TestTextureIdlePassivatesAndResumesSameRun|TestTextureRevisionRunParksAndConsumesUpdateWithoutColdWake|TestRestartRecoveryReactivatesInterruptedTextureRun' -count=1`

Open edge: keep the first-cold activation path only as a compatibility bridge
for documents that genuinely have no resident or sleeping Texture actor; ordinary
post-activation updates should route to the same sleeping thread.

## 2026-06-21 - Runtime Shards Pass After Sleep Slice

Claim: the idle sleep/passivation and sandbox config repairs should not regress
the broader non-comprehensive runtime suite.

Move: ran the runtime shard script sequentially for all four shards. The first
plain invocation completed shard `0/4`; shards `1/4`, `2/4`, and `3/4` were run
explicitly with `SHARD_INDEX` to avoid overclaiming the first receipt.

Expected ΔV: 0. This is verification for the current slice, not mission
settlement, and any further runtime mutation requires fresh verification.

Actual ΔV: 0. V remains 5.

Receipts:
- `nix develop -c scripts/go-test-runtime-shards` (shard `0/4`, pass)
- `nix develop -c env SHARD_INDEX=1 TOTAL_SHARDS=4 scripts/go-test-runtime-shards` (pass)
- `nix develop -c env SHARD_INDEX=2 TOTAL_SHARDS=4 scripts/go-test-runtime-shards` (pass)
- `nix develop -c env SHARD_INDEX=3 TOTAL_SHARDS=4 scripts/go-test-runtime-shards` (pass)

Open edge: comprehensive Texture tests, docs/store/sandbox focused tests, commit,
push, CI, deploy identity, and staging product proof remain before settlement.

## 2026-06-21 - Texture Update Wakes Use Activation Mailbox Turns

Claim: Texture should not deliver fresh `update_coagent` wakes by smuggling
typed packets into prompt-prefix context before run memory exists. Even when a
fresh Texture activation is still needed, pending updates should be represented
as durable mailbox turns in the activation's run-memory log.

Move: added an `activation_mailbox_turn` delivery phase and taught Texture
activation wakes to append pending coagent packets as the first mailbox turn
after run-memory initialization. The existing warm injector now accepts an
initial delivery phase, so fresh, resident, sleeping, and restart-reactivated
Texture delivery all share the same run-memory user-turn substrate. Cold
activation packet prep remains only as non-Texture compatibility behavior.
The findings-wake test now proves that the fresh wake is not cold-prepend
eligible, that the activation mailbox packet is persisted in run memory, and
that the durable evidence handle linked from the runtime-owned update id
resolves.

Expected ΔV: 0. This removes Texture cold-prepend delivery and strengthens
variant item 7, but replacement wake scaffolding still exists when there is no
resident or sleeping Texture actor, so event-driven durable-thread delivery is
not fully settled.

Actual ΔV: 0. V remains 5.

Receipts:
- `internal/runtime/coagent_update_packet.go`
- `internal/runtime/runtime.go`
- `internal/runtime/super_controller.go`
- `internal/runtime/texture_agent_revision.go`
- `internal/runtime/texture_test.go`
- `docs/choir-prompting-invariants.md`
- `docs/texture-agentic-invariants-2026-06-13.md`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestSubmitResearchFindingsWakeUsesSameDebouncedPath' -count=1`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestSubmitResearchFindingsWakeUsesSameDebouncedPath|TestTextureRevisionRunParksAndConsumesUpdateWithoutColdWake|TestTextureIdlePassivatesAndResumesSameRun|TestRestartRecoveryReactivatesInterruptedTextureRun|TestTextureUpdateCoagentDuringActiveRevisionTriggersSameRunFollowUp' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoopParkWaiterBlocksWithoutProviderCallsUntilInjectedTurn|TestUpdateCoagent|TestTextureWarmInjectedUpdateIsConsumedByRevisionWrite|TestSubmitWorkerUpdateWakeUsesSameDebouncedPath|TestInitialTextureToolChoiceRequiresPatchBeforeContinuation' -count=1`
- `nix develop -c scripts/go-test-runtime-shards` (shard `0/4`, pass)
- `nix develop -c env SHARD_INDEX=1 TOTAL_SHARDS=4 scripts/go-test-runtime-shards` (pass)
- `nix develop -c env SHARD_INDEX=2 TOTAL_SHARDS=4 scripts/go-test-runtime-shards` (pass)
- `nix develop -c env SHARD_INDEX=3 TOTAL_SHARDS=4 scripts/go-test-runtime-shards` (pass)

Open edge: delete replacement wake behavior for ordinary document-owned Texture
threads. A document with an existing Texture thread should receive addressed
updates through the resident or sleeping actor; a document without a thread
needs an explicit thread creation path, not an implicit run reconstruction path.

## 2026-06-21 - Parked Texture Delegation Stays In Same Thread

Claim: replacement wake runs persist because a Texture actor can treat semantic
delegation (`spawn_agent`, `request_super_execution`, email handoff) as a
terminal tool success, completing the document thread before researcher/super
evidence returns. Once a document has Texture revision-thread history, ordinary
addressed `update_coagent` backlog should enter a resident or sleeping actor;
it should not mint a second Texture run.

Move: for Texture revision actors with idle parking enabled, removed the
semantic delegation tools from terminal-tool exits so the tool loop reaches the
park/passivate path after delegation. Tightened `reconcileTextureAgentWake` so
the fresh integrate-run creation path is legal only when the document has no
prior Texture revision-thread history. Added focused tests that direct revise
writes work-state before delegation then sleeps, researcher evidence writes V2
in the same Texture `loop_id`, and completed historical thread records do not
spawn replacement wake runs.

Expected ΔV: -1. This should close the named replacement-wake edge for
established Texture threads while keeping explicit first activation for documents
without a thread.

Actual ΔV: -1. V is now 4.

Receipts:
- `internal/runtime/runtime.go`
- `internal/runtime/texture_controller.go`
- `internal/runtime/texture_test.go`
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestDirectTextureReviseWritesWorkStateBeforeDelegatingResearch|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureWakeDoesNotMintReplacementForExistingThreadHistory|TestTextureRevisionRunParksAndConsumesUpdateWithoutColdWake|TestTextureIdlePassivatesAndResumesSameRun|TestTextureUpdateCoagentDuringActiveRevisionTriggersSameRunFollowUp|TestSubmitResearchFindingsWakeUsesSameDebouncedPath' -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestRunToolLoopTerminalToolSuccessStopsWithoutExtraProviderTurn|TestRunToolLoopParkWaiterBlocksWithoutProviderCallsUntilInjectedTurn|TestUpdateCoagent|TestUpdateCoagentWarmActivationInjectsPendingTurn|TestCoagentRewarmUsesResidentActivationNotActiveRunProxy|TestCoagentRewarmIgnoresBlockedHistoricalActivation|TestUpdateCoagentDeliveryRequiresSuccessfulActivation' -count=1`
- `nix develop -c scripts/go-test-runtime-shards` (all four shards sequential, pass)
- `git diff --check`

Open edge: classifier/exact-first-tool/model-prior completion guard residues
still encode semantic choreography. The next deletion should preserve
owner-visible work-state and evidence-path behavior without hard role forcing.

## 2026-06-21 - Parked Delegation Same-Thread Slice Deployed and Staging-Proven

Claim: commit `1c202e525f77a2a6169c0bf0ac49b986b75047b7` should preserve the
Texture durable-thread behavior on staging: semantic delegation must not
terminal-complete a parked Texture revision actor, researcher findings must wake
the same Texture `loop_id`, and established document threads must not mint
replacement wake runs for ordinary addressed update backlog.

Move: pushed the docs checkpoint and runtime repair to `origin/main`, monitored
CI/deploy, verified `choir.news` build identity, submitted an authenticated
Comet UI prompt through the logged-in product session, then ran an instrumented
public-product API probe for structured trajectory, Trace, revision, and
RunAcceptanceRecord evidence. Computer Use/Comet was used for the account-backed
browser submission; Playwright was used only for structured public API evidence
because the Comet accessibility surface exposed revision state but not enough
Trace/product ids to synthesize acceptance.

Expected ΔV: 0. This is landing proof for the prior ΔV=-1 implementation, not a
new variant deletion.

Actual ΔV: 0. V remains 4.

Receipts:
- Commits:
  - `a78f972a` (`docs: record texture same-thread delegation invariant`);
  - `1c202e525f77a2a6169c0bf0ac49b986b75047b7`
    (`runtime: keep parked Texture delegation in one thread`).
- Push: `origin/main` advanced from `d161c2e4` to `1c202e52`.
- GitHub Actions:
  - CI run `27893155637`: success, including `Deploy to Staging (Node B)` job
    `82540310386`.
  - Docs Truth Check run `27893155620`: success.
  - FlakeHub run `27893155633`: success.
- Staging identity: `/health` reported proxy and sandbox both at
  `1c202e525f77a2a6169c0bf0ac49b986b75047b7`, deployed at
  `2026-06-21T04:16:34Z`, with `status=ok`, `upstream=ok`, and
  `vmctl_status=ok`.
- Authenticated Comet UI proof: submitted
  `CHOIR_DURABLE_THREAD_PROOF_20260621_001` through the logged-in
  `choir.news` UI. The product opened a Texture document at `v0`, wrote `v1`,
  and remained in `Revising...`/`Continuing...` state. This proved the logged-in
  product path was active, but did not expose enough Trace ids for structured
  acceptance.
- Structured deployed probe:
  - prompt: `What's going on with Anthropic and the US government?`;
  - trajectory/submission: `a893f0ca-a8b6-41de-a73b-1e8b05c7c80d`;
  - doc: `d296fdfc-98f0-44d6-b984-c0ad7d2098ee`;
  - initial Texture loop id:
    `c3cb6b21-6220-4d6f-a226-641906ea56b9`;
  - revisions: V0 user at +0.258s, V1 appagent at +18.303s
    (`99ca1f5a-89ae-4ae2-a2b0-4b2ca2080bb5`, 1167 chars), V2 appagent at
    +88.860s (`29bcfccf-429c-40a6-92bb-55d7e43543ea`, 3514 chars);
  - Trace counts: `web_search=20`, `spawn_agent=2`, `update_coagent=2`,
    `moment_count=243`, `agent_count=3`, `delegation_count=1`;
  - trajectory state: `passivated`.
- Same-thread Trace evidence: the Texture actor
  `texture:d296fdfc-98f0-44d6-b984-c0ad7d2098ee` used loop
  `c3cb6b21-6220-4d6f-a226-641906ea56b9` for `spawn_agent`,
  `park_wait_started`, `park_wait_finished`, the V2 `patch_texture`,
  `texture.document_revision.created`, and final `activation.passivated`. The
  researcher `update_coagent` packet arrived on loop
  `33779ab4-bfef-4565-b996-a1848acbac50` and woke the same Texture loop rather
  than causing a replacement Texture activation.
- RunAcceptanceRecord:
  `runacc-26cfe15a6fbd4fd6be6f`, target mission
  `mission-texture-durable-thread-v1`, trajectory
  `a893f0ca-a8b6-41de-a73b-1e8b05c7c80d`, deployment/health commit
  `1c202e525f77a2a6169c0bf0ac49b986b75047b7`, CI run `27893155637`, deploy
  run `27893155637:82540310386`, acceptance level `staging-smoke-level`, state
  `blocked`. The blocked state is expected because continuation-level evidence
  and the remaining classifier/exact-first-tool/model-prior deletion are not
  settled.

Result: the parked-delegation same-thread slice is deployed and supported by
staging product evidence. It does not settle the mission. The next realism axis
is still to delete classifier/exact-first-tool/model-prior guard residues while
preserving owner-visible work state and evidence-path behavior.

## 2026-06-21 - Exact First-Tool Deletion Target Selected

Claim: prompt-bar first-paint exact `patch_texture` forcing is now the narrowest
high-value guard residue to delete. It is a runtime trust channel because
`initialTextureToolChoice` converts ordinary initial Texture work into
provider-level `function:patch_texture` and `RunToolLoop` narrows the first-call
tool list to that single tool. The durable-thread invariant should instead be:
Texture must make owner-visible canonical progress, reject no-op prompt-copy
patches, and open evidence paths when needed, without hidden semantic
choreography in tool choice.

Move: document this problem before the runtime change. The intended behavior
change is to leave prompt-bar initial Texture runs unconstrained on the first
provider call, while preserving grounded `update_coagent` integration safeguards
and the existing no-op/progress guards.

Expected ΔV: -1 if tests and staging show ordinary prompt-bar Texture starts
with no exact first-tool choice, still creates a useful appagent revision, and
still handles researcher/super evidence updates.

Admissible evidence: focused unit/integration tests around
`initialTextureToolChoice`, prompt-bar Texture starts, no-op prompt-copy retry,
researcher/super V2 update paths, full runtime shards, then deployed product
proof on `choir.news` with Trace evidence that the first provider call is not
exact-tool forced.

Rollback: revert the runtime commit that changes `initialTextureToolChoice` and
its test inversions. Docs may remain as problem evidence if the rollback proves
the deletion is not yet safe.

## 2026-06-21 - Exact First-Tool Deletion Deployed and Staging-Proven

Claim: commit `b4adb70ff4a01ea6be92ce30a062a66a824f89a9` should delete ordinary
prompt-bar first-paint exact `patch_texture` forcing without weakening durable
Texture progress. Initial prompt-bar Texture runs should enter the provider with
empty `tool_choice` and the full Texture tool surface; grounded
`update_coagent` integration may still use narrow mechanical continuation
behavior.

Move: changed `initialTextureToolChoice` so ordinary prompt-bar first paint is
unconstrained, kept update-backed integration exact, inverted prompt-bar and
Texture tests around the full initial tool surface, ran focused runtime tests
and runtime shards, pushed to `origin/main`, monitored CI/deploy, verified
staging build identity, used Comet for a logged-in UI proof, then used
authenticated public product APIs for Trace fields that the Comet UI does not
render.

Expected ΔV: -1.

Actual ΔV: -1. V is now 3.

Receipts:
- Commits:
  - `39b163ba` (`docs: record texture exact first-tool deletion target`);
  - `b4adb70ff4a01ea6be92ce30a062a66a824f89a9`
    (`runtime: remove Texture first-paint exact patch forcing`).
- Focused tests:
  - `nix develop -c go test ./internal/runtime -run 'TestInitialTextureToolChoiceOnlyConstrainsMechanicalContinuations|TestHandlePromptBarExplicitNoWorkerDecisionStartsWithTexture|TestInitialTextureRunWritesBeforeSpawningResearcher|TestTextureModelPriorCompletionGuardOpensProbePath|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureCreatedSuperEvidenceWakesTextureV2|TestInitialTextureRunDefaultsMinimalEditContextFromActivation|TestInitialTextureDecisionPromptRejectsPrematureEditBeforeDecision|TestTextureRevisionRunParksAndConsumesUpdateWithoutColdWake|TestInitialTextureNoOpPatchRetriesIntoUsefulDraft' -count=1`
- Runtime suite: `nix develop -c scripts/go-test-runtime-shards` passed all
  four sequential shards.
- GitHub Actions:
  - CI run `27893821509`: success, including deploy job `82542050100`.
  - Docs Truth Check run `27893821503`: success.
  - FlakeHub run `27893821512`: success.
- Staging identity: `/health` reported proxy and sandbox both at
  `b4adb70ff4a01ea6be92ce30a062a66a824f89a9`, deployed at
  `2026-06-21T04:48:51Z`, with `status=ok`, `upstream=ok`, and
  `vmctl_status=ok`.
- Authenticated Comet UI proof: submitted
  `CHOIR_EXACT_TOOL_DELETION_PROOF_20260621_001` through the logged-in
  `choir.news` session. The product opened a Texture document, advanced to
  `v1`, rendered a concise proof note containing deployed commit
  `b4adb70ff4a01ea6be92ce30a062a66a824f89a9`, and the visible activity stream
  included `patch_texture` and `record_texture_decision` calls.
- Structured deployed proof:
  - prompt marker:
    `CHOIR_EXACT_TOOL_DELETION_ACCEPT_1782018076150`;
  - trajectory/submission: `20bf8cd1-ff91-4bae-87a0-96d348d7b3ae`;
  - doc: `2355c0ea-ca1e-4449-b96e-f220a3952afb`;
  - initial Texture loop id:
    `c0dfb742-9797-42e7-931f-6951d63cddbc`;
  - first provider-call moment:
    `96b5c6e7-a1b9-4c18-965c-bc8cf4ba224c`;
  - first provider-call payload:
    `tool_choice=""`, `tools=8`, tool names included `patch_texture`,
    `record_texture_decision`, `spawn_agent`, and `request_super_execution`;
  - first tool batch chose `record_texture_decision`; second tool batch chose
    `patch_texture`;
  - tool results: `record_texture_decision returned` and
    `patch_texture returned`;
  - appagent V1:
    `cb641f7b-c821-43f3-82fe-2c5611479e19`, 205 chars, containing the marker;
  - retry evidence: no `initial_tool_choice` retry and no
    `exact_initial_tool_choice_precondition` fallback.
- RunAcceptanceRecord:
  `runacc-73a11f1381124ee76315`, target mission
  `mission-texture-durable-thread-v1`, trajectory
  `20bf8cd1-ff91-4bae-87a0-96d348d7b3ae`, deployment/health commit
  `b4adb70ff4a01ea6be92ce30a062a66a824f89a9`, acceptance level
  `staging-smoke-level`, state `blocked`. The blocked state is expected for
  this narrow slice because export-level and continuation-level evidence are
  intentionally not claimed.

Result: prompt-bar first-paint exact `patch_texture` forcing is no longer a
semantic trust channel. The remaining realism axis is classifier/model-prior
guard deletion plus always-deep research proof beyond the current V2/V3
cadence.

## 2026-06-21 - Deterministic Initial-Super Handoff Target Selected

Claim: deterministic initial-super request parsing is the next highest-value
classifier/guard residue to delete. The current path recognizes prompt phrases
such as "ask downstream super execution to", persists
`texture_initial_super_request_required` metadata, and calls
`requestPersistentSuperExecution` from runtime before the Texture provider turn.
That means runtime, not Texture, creates a semantic super delegation from prompt
text.

Move: document the problem before the runtime change. The intended behavior
change is to leave execution-shaped prompt-bar requests inside Texture-owned
artifact state, expose `request_super_execution` as an available affordance, and
let Texture decide whether and when to call it. Prompt and hard-requirement
language may still describe execution obligations, but must not deterministically
open downstream super work.

Expected ΔV: -1 if focused tests prove execution-shaped prompt-bar requests open
Texture first without automatic super metadata/run creation, while Texture still
sees the `request_super_execution` tool and can call it agentically.

Admissible evidence: focused prompt-bar/Texture tests for no automatic
`texture_initial_super_request_required` metadata, no pre-provider super run,
presence of the super-execution tool affordance, no regression to explicit
no-worker decision recording, full runtime shards, then deployed staging proof
on `choir.news` if runtime behavior changes land.

Rollback: revert the runtime commit that deletes the deterministic parser and
recorder. The docs checkpoint may remain as problem evidence if the deletion
shows an uncovered product need for visible command grammar.

## 2026-06-21 - Deterministic Initial-Super Handoff Deployed and Staging-Proven

Claim: commit `1e0166474e17369828a1e8a7bfd655c34ae1454b` should delete
runtime-created initial super work from Texture prompt text while preserving
Texture's agentic `request_super_execution` affordance.

Move: removed the deterministic initial-super parser/recorder, inverted focused
prompt-bar and Texture tests, ran runtime shards, pushed to `origin/main`,
monitored CI/deploy, verified staging build identity, used Comet for a
logged-in UI proof, and used authenticated public product APIs for Trace fields
not visible in the UI.

Expected ΔV: -1.

Actual ΔV: -1. V is now 2.

Receipts:
- Commits:
  - `aa19d1c0` (`docs: record texture initial-super handoff target`);
  - `1e0166474e17369828a1e8a7bfd655c34ae1454b`
    (`runtime: stop auto-opening super from Texture prompt text`);
  - `b823ff89` (`docs: record texture initial-super staging proof`).
- Focused tests covered prompt-bar explicit-super text no longer stamping
  `texture_initial_super_request_*` metadata or creating automatic super work,
  plus manual `request_super_execution` from Texture context still working.
- Runtime suite: `nix develop -c scripts/go-test-runtime-shards` passed all
  four sequential shards.
- GitHub Actions:
  - CI run `27894521422`: success, including deploy job `82543893990`.
  - Docs Truth Check run `27894521436`: success.
  - FlakeHub run `27894521431`: success.
  - Docs proof commit Docs Truth Check run `27894825543`: success.
- Staging identity: `/health` reported proxy and sandbox both at
  `1e0166474e17369828a1e8a7bfd655c34ae1454b`, deployed at
  `2026-06-21T05:23:22Z`, with `status=ok`.
- Authenticated Comet UI proof marker:
  `CHOIR_INITIAL_SUPER_DELETION_PROOF_20260621_001`; the Texture window reached
  `v1`, preserved the old execution phrase as document text, and showed Texture
  tool activity.
- Structured deployed proof:
  - prompt marker:
    `CHOIR_INITIAL_SUPER_DELETION_ACCEPT_1782019773071`;
  - trajectory/submission: `a2e96589-dc7a-4be4-9e5d-556427f5afc2`;
  - doc: `8789314a-fc2c-498d-9634-f9af72de6f56`;
  - initial Texture loop id:
    `ad8443b9-c5a2-4171-b59d-7cc06db91885`;
  - appagent V1:
    `652e8729-0172-4420-935d-f747876ba8ea`;
  - Trace roles: `conductor + texture` only;
  - first Texture tool event: `record_texture_decision`;
  - no `super` agent, no `request_super_execution` tool call, no
    `texture_initial_super_request_*` metadata, no `delegation_opened`.
- RunAcceptanceRecord:
  `runacc-db363e8ce03c8e65f0fe`, target mission
  `mission-texture-durable-thread-v1`, trajectory
  `a2e96589-dc7a-4be4-9e5d-556427f5afc2`, deployment/health commit
  `1e0166474e17369828a1e8a7bfd655c34ae1454b`, acceptance level
  `staging-smoke-level`, state `blocked`. The blocked state is expected for
  this narrow slice because export-level and continuation-level evidence are
  intentionally not claimed.

Result: execution-shaped prompt text is no longer a runtime-owned super
delegation trigger. The remaining realism axis is the model-prior/world-
knowledge completion guard plus always-deep research proof beyond the current
V2/V3 cadence.

## 2026-06-21 - Model-Prior Completion Guard Target Selected

Claim: `textureModelPriorCompletionGuard` and
`texturePromptNeedsWorldKnowledge` are now the highest-value remaining
classifier/guard residue. The current path scans prompt text for markers such
as "latest", "today", "news", or "government" and injects a completion-guard
retry after a model-prior/interim first revision if no evidence path has opened.
That means runtime, not Texture, still chooses a semantic Probe/Execute
obligation from prompt keywords.

Move: document the problem before the runtime change. The intended behavior
change is to keep model-prior/interim metadata and prompt obligations, keep
`spawn_agent`/`request_super_execution` available, but delete the
Texture-specific completion guard and its world-knowledge keyword classifier.
Texture should open researcher/super work through ordinary tool choice, record
an audit decision, or stop with an honest interim/blocker without a runtime
retry instruction.

Expected ΔV: -1 if focused tests prove no completion-guard retry is injected,
current-events prompts can still open researcher work agentically, model-prior
metadata/no-op protections remain intact, runtime shards pass, and staging
shows researcher path opening without a guard retry.

Admissible evidence: focused tool-loop/Texture tests showing no
`texture_model_prior_interim_needs_evidence_path` guard, no
`texturePromptNeedsWorldKnowledge` classifier, retained model-prior metadata
and no-op guard behavior, researcher/super tool affordances still present, full
runtime shards, then deployed staging proof on `choir.news`.

Rollback: revert the runtime commit that removes the Texture-specific
completion guard and classifier. The docs checkpoint may remain as problem
evidence if deletion shows that prompt obligations alone are not sufficient to
preserve agentic deepening.

## 2026-06-21 - Model-Prior Completion Guard Deleted and Staging-Proven

Claim: commit `cd79ed2d6f7ed629328daf658ab988baf42edad7` should delete the
Texture-specific model-prior completion guard and world-knowledge keyword
classifier while preserving honest model-prior/interim metadata, no-op
protection, and agentic researcher/super tool affordances.

Move: removed `WithCompletionGuard(rt.textureModelPriorCompletionGuard(rec))`
from Texture revision runs, deleted `textureModelPriorCompletionGuard`,
`textureRunOpenedEvidencePath`, and `texturePromptNeedsWorldKnowledge`, updated
the current-events Texture test to require ordinary `spawn_agent` tool choice
without the old guard instruction, ran focused tests and runtime shards, pushed
to `origin/main`, monitored CI/deploy, verified staging build identity, used
Computer Use with the logged-in Comet session for visible product proof, and
used authenticated public product APIs for Trace fields not visible in the UI.

Expected Delta V: -1.

Actual Delta V: -1. V is now 1.

Receipts:
- Commits:
  - `cc03d089` (`docs: record texture model-prior guard target`);
  - `cd79ed2d6f7ed629328daf658ab988baf42edad7`
    (`runtime: remove Texture model-prior completion guard`).
- Focused tests:
  `nix develop -c go test ./internal/runtime -run 'TestTextureCurrentEventsPromptCanOpenProbePathWithoutCompletionGuard|TestInitialTextureRunWritesBeforeSpawningResearcher|TestTextureCreatedResearcherEvidenceWakesTextureV2|TestTextureCreatedSuperEvidenceWakesTextureV2|TestPromptOnlyInitialUserPromptRevisionIsMarkedModelPrior|TestInitialTextureNoOpPatchRetriesIntoUsefulDraft|TestHandlePromptBarExplicitSuperExecutionStartsWithTextureWithoutAutomaticSuper|TestHandlePromptBarExplicitNoWorkerDecisionStartsWithTexture' -count=1`
  passed.
- Guard deletion grep:
  `rg -n "textureModelPriorCompletionGuard|func texturePromptNeedsWorldKnowledge|WithCompletionGuard\\(rt\\.texture" internal/runtime -g '*.go'`
  returned no matches.
- Diff hygiene: `git diff --check` passed.
- Runtime suite: `nix develop -c scripts/go-test-runtime-shards` passed all
  four sequential shards.
- GitHub Actions:
  - CI run `27895027562`: success, including deploy job `82545223168`.
  - Docs Truth Check run `27895027563`: success.
  - FlakeHub run `27895027559`: success.
- Staging identity: `/health` reported proxy and sandbox both at
  `cd79ed2d6f7ed629328daf658ab988baf42edad7`, deployed at
  `2026-06-21T05:47:50Z`, with `status=ok`.
- Authenticated Comet UI proof:
  - submitted marker
    `CHOIR_MODEL_PRIOR_GUARD_DELETION_PROOF_20260621_001` through the logged-in
    `choir.news` session in Comet;
  - visible Texture run handle prefix/suffix `273cdee7-b...0ff678`;
  - the marked Texture reached `v3`;
  - the visible activity stream showed Texture calling `spawn_agent`, receiving
    tool output, then calling `patch_texture`;
  - V3 rendered a grounded evidence brief for Anthropic and US government AI
    procurement, including an explicit note that durable source-entry retrieval
    failed and clickable URLs still need attachment for publication-quality
    citations.
- Structured deployed proof:
  - prompt marker:
    `CHOIR_MODEL_PRIOR_GUARD_API_PROOF_20260621_1782021104911`;
  - trajectory/submission: `780dc749-ab6c-4d4b-9594-721c02b8f60e`;
  - doc: `fc398877-517c-4eff-9fb4-2a17d8f1f736`;
  - initial Texture loop id:
    `0f44a44a-1b0d-4b6e-bdc0-2fdf5c41a42e`;
  - Trace roles: `conductor + texture + researcher`;
  - Trace analysis: `spawn_agent_count=4`, `patch_texture_count=12`,
    `retry_moment_count=0`, `guard_reason_present=false`,
    `completion_guard_present=false`, and `old_guard_instruction_present=false`;
  - V1 appagent revision:
    `fc33eecf-f8dc-404c-ab5a-ce11e7aba928`, 2559 chars, metadata
    `model_prior_interim=true`, `revision_grounding=model_prior`;
  - V2 appagent revision:
    `2ff153df-d469-4e36-8af5-7621b575c1fc`, 5721 chars, consumed researcher
    worker update seq 1;
  - V3 appagent revision:
    `b7f981fa-76fa-44c2-a00c-1dbaef0d055c`, 7159 chars, consumed researcher
    worker update seq 2;
  - assertions passed: deployed SHA matched, Texture decision opened, researcher
    role seen, `spawn_agent` tool result seen, no completion-guard retry, no old
    guard reason, no old guard instruction, and model-prior metadata present.
- RunAcceptanceRecord:
  `runacc-d8bf901c9bbb56c5d583`, target mission
  `mission-texture-durable-thread-v1`, trajectory
  `780dc749-ab6c-4d4b-9594-721c02b8f60e`, deployment/health commit
  `cd79ed2d6f7ed629328daf658ab988baf42edad7`, acceptance level
  `staging-smoke-level`, state `blocked`. The blocked state is expected for
  this narrow slice because export-level and continuation-level evidence are
  intentionally not claimed.

Result: current-events/world-knowledge prompt handling no longer depends on a
runtime-authored completion-guard retry or keyword classifier. Texture can write
an honest model-prior interim revision, choose `spawn_agent` normally, and
incorporate researcher evidence into later canonical revisions. The remaining
realism axis is always-deep/source-evidence robustness beyond this narrow V3
current-events path, especially durable source attachment for owner-visible
grounded citations.

## 2026-06-21 - Source Attachment Gap Documented Before Fix

Claim: the next source-evidence repair must target a typed runtime attachment
gap, not reintroduce role choreography or prose scraping. The C12 staging proof
at commit `cd79ed2d6f7ed629328daf658ab988baf42edad7` produced grounded V3
research synthesis through Texture and researcher, but the Comet-visible V3
also said durable source-entry retrieval failed and clickable URLs still needed
attachment for publication-quality citations.

Move: audited the source collation path before code edits. `update_coagent`
supports inline `evidence`, `evidence_ids`, and `refs`; inline evidence is
persisted and appended to `EvidenceIDs`, but
`evidenceSourceEntitiesFromPendingUpdates` currently builds Texture
`source_entities` only from `EvidenceIDs`. Source-search results and fallback
researcher checkpoints already use typed `source_service_item:<item_id>` refs,
and Texture has a first-class `source_service_item` source entity type with
source-service enrichment. Therefore a researcher packet can carry a durable
source ref in `refs` while the pending-update collation supplies no source
entity to the canonical Texture revision.

Expected Delta V: 0 for this documentation checkpoint. The next runtime commit
can reduce V only if it proves typed refs become source entities without parsing
free-form findings.

Actual Delta V: 0. V remains 1.

Admissible next evidence: focused runtime tests showing
`source_service_item:...`, `content_id:...`, and evidence refs in
`update_coagent.refs` are collated into source entities; a negative test showing
free-form prose refs are ignored; runtime shards; then deployed Comet/product
proof that grounded Texture output has durable source handles instead of the
C12 source-entry caveat.

Protected surfaces: Texture revision metadata `source_entities`,
`update_coagent` evidence/ref delivery, source-service projection, citation
validation, and owner-visible grounded citation behavior.

Rollback: revert the runtime source-ref collation commit. This docs checkpoint
should remain as evidence of the discovered gap unless later proof falsifies the
diagnosis.

## 2026-06-21 - Source Ref Collation Landed With Partial Staging Proof

Claim: commit `06e729734cfff7a6ee33715ea5fde2e6bf7e05e5` should let typed
researcher/source refs carried in `update_coagent.refs` become Texture source
entities without returning to regex prose scraping or semantic role
choreography.

Move: added runtime collation for typed worker-update refs:
`source_service_item:...` / `source_service_item=...`, `content_id:...`, and
evidence refs. Unsupported or free-form prose refs are ignored. Added a focused
test that creates source-service, content-item, and evidence handles, sends them
through a pending worker update, and verifies source entities are produced while
free-form prose is not scraped.

Expected Delta V: -1 only if deployed proof shows native durable source handles
attached cleanly. Otherwise the change narrows the source-evidence risk but does
not settle the last variant.

Actual Delta V: 0. V remains 1.

Receipts:
- Problem checkpoint commit:
  `7a817c9d14803916352e9752ec78f2780878cf74`
  (`docs: record Texture source attachment gap`), pushed to `origin/main`;
  Docs Truth Check run `27895441834` succeeded.
- Runtime commit:
  `06e729734cfff7a6ee33715ea5fde2e6bf7e05e5`
  (`runtime: attach Texture source refs from coagent updates`), pushed to
  `origin/main`.
- Focused tests:
  `nix develop -c go test ./internal/runtime -run 'TestPendingUpdateRefsBecomeSourceEntities|TestEvidenceRecordToSourceEntity|TestEvidenceDerivedEntityFeedsCitationValidator' -count=1`
  passed.
- Runtime coverage:
  `nix develop -c scripts/go-test-runtime-shards` passed, and explicit shard 3
  confirmation with
  `nix develop -c env SHARD_INDEX=3 TOTAL_SHARDS=4 scripts/go-test-runtime-shards`
  passed.
- Diff hygiene: `git diff --check` passed before commit.
- GitHub Actions for runtime commit:
  - CI run `27895604243`: success, including deploy job `82546715436`;
  - FlakeHub run `27895604242`: success.
- Staging identity: `/health` reported proxy build, proxy deployed commit, and
  upstream sandbox commit all at
  `06e729734cfff7a6ee33715ea5fde2e6bf7e05e5`, with deployed/built time
  `2026-06-21T06:14:52Z`, `status=ok`, and vmctl enabled/ok.
- Comet UI proof using the logged-in `yusefnathanson@me.com` session:
  - submitted marker
    `CHOIR_SOURCE_REF_ATTACHMENT_PROOF_20260621_001`;
  - visible initial draft refused model-prior-only claims and asked for
    researcher/source-search evidence;
  - activity stream showed Texture run
    `96289132-1911-4917-ba6e-2229b2ed3c22` spawning researcher worker
    `02d2624e-fc28-418f-a787-4beb4e63b43b`;
  - worker activity showed `web_search`, `source_search`, `save_evidence`, and
    `update_coagent`;
  - UI reported `Coagent update ready. Role: researcher. Kind: findings.
    Summary: Grounded evidence for a current public AI model release`;
  - Texture then called `rewrite_texture` and created `v2`;
  - v2 title was `Concise evidence note: OpenAI GPT-5.5 public model release`;
  - v2 carried the deployed commit under test, said it used
    researcher/source-search evidence only, and rendered inline clickable links
    for OpenAI model docs, TechCrunch coverage, and the surfaced OpenAI
    announcement handle;
  - v2 did not contain the old "durable source-entry retrieval failed" caveat.
- Negative deployed observations:
  - the native Texture `Sources` button remained disabled on v2;
  - the window still showed `Revising...` and disabled publish/revise controls
    after extended polling;
  - the activity stream showed tool-error entries around the worker and parent
    run despite the accepted `update_coagent` and created v2;
  - opening `/api/trace/trajectories/96289132-1911-4917-ba6e-2229b2ed3c22/logs`
    in a raw Comet tab returned `{"error":"authentication required"}`, so the
    run error was not inspected through raw public API without exposing browser
    tokens.

Result: typed source refs are covered by local runtime tests, landed, deployed,
and the staging product path now reaches researcher source-search evidence,
accepted `update_coagent` delivery, and a source-linked Texture v2 without the
prior retrieval-failed caveat. The slice is not settled because native
source-panel attachment and clean run settlement were not proven on staging.

Next realism axis: diagnose the disabled native Sources state and visible
tool-error / `Revising...` residue after source-backed rewrite. The next fix
must again document the problem first if it changes runtime behavior.

## 2026-06-21 - Texture Passivation Stream Settlement Local Repair

Claim: the disabled Sources / continuing-state residual after
`CHOIR_NATIVE_SOURCE_ENTITY_PROOF_20260621_004` is at least partly a stream
settlement gap. The Texture actor can successfully create a source-backed v2 and
then park, but the browser document stream does not receive a document-scoped
completion event for idle passivation, so the editor can keep `agentPending`
true and leave toolbar actions disabled.

Move: added document stream metadata to Texture revision actor idle
passivation: `doc_id`, `current_revision_id`, and `loop_id` now ride on
`EventRunPassivated`. The Texture document stream maps Texture-owned
`EventRunPassivated` to `synth_completed`, and its payload decoder now tolerates
mixed JSON metadata so numeric token fields do not hide the string correlation
fields. Added focused comprehensive tests for the stream mapping and the emitted
passivation payload, while preserving non-Texture passivation filtering.

Expected Delta V: -1 only after deployed product proof shows a source-backed v2
settles to idle with native inline source handles and an inspectable Sources
panel. Local tests alone do not settle the source-panel axis.

Actual Delta V: provisional 0. V remains 1 pending commit, deploy, and Comet
proof on `https://choir.news`.

Receipts:
- Focused passivation stream tests:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestTextureStreamEventMaps(TexturePassivationToSynthCompleted|ProgressSeparatelyFromStarted)$|TestTextureIdlePassivationEventCarriesDocumentStreamCompletionPayload$' -count=1`
  passed.
- Adjacent resident actor tests:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestTexture(DocumentStreamEmitsHeadChangeAfterAgentRevision|IdlePassivatesAndResumesSameRun|RevisionRunParksAndConsumesUpdateWithoutColdWake)$' -count=1`
  passed.
- Source/evidence regression tests:
  `nix develop -c go test ./internal/runtime -run 'TestResearcher(FailureSynthesizesCheckpointAfterSearch|CompletionSynthesizesCheckpointAfterSavedEvidence)|Test(EvidenceRecordToSourceEntity|EvidenceDerivedEntityFeedsCitationValidator|EvidenceSummaryEntityAllowsNativeCitationWithoutQuoteMatch|PendingUpdateRefsBecomeSourceEntities|TextureCoagentSourceRefsSurviveInjectionAndDelivery|TextureCoagentEvidenceSummarySourceCanPatchWithNativeCitation)$' -count=1`
  passed.
- Runtime coverage:
  `nix develop -c scripts/go-test-runtime-shards` passed locally.

Open edge: staging must prove the frontend receives the new `synth_completed`
after idle passivation and that this enables the toolbar `Sources` panel without
manual cancellation.

## 2026-06-21 - Texture Passivation Stream Settlement Deployed Proof

Claim: commit `fce827ca2a43994d1d67312f33fe4fef1d97f4d3` should close the
source-panel settlement gap discovered in
`CHOIR_NATIVE_SOURCE_ENTITY_PROOF_20260621_004`: after a source-backed Texture
actor writes v2 and idles, the document stream should receive a document-scoped
completion event, clear toolbar pending state, and make native Sources
inspectable without manual cancellation.

Move: pushed `runtime: settle Texture passivation stream` to `origin/main`,
monitored CI/deploy, verified staging identity, and used Computer Use against
the logged-in Comet browser session for `yusefnathanson@me.com` instead of a
fresh Playwright context. Submitted marker
`CHOIR_NATIVE_SOURCE_ENTITY_PROOF_20260621_005` against deployed commit
`fce827ca2a43994d1d67312f33fe4fef1d97f4d3`.

Expected Delta V: -1 if the deployed product path writes source-backed Texture
v2, settles out of visible pending state after idle passivation, enables the
toolbar `Sources` control, and shows represented source artifacts in the native
Sources panel.

Actual Delta V: -1. V is now 0 for the source-panel settlement axis. The proof
does not claim export-level, promotion-level, or continuation-level acceptance.

Receipts:
- Runtime commit:
  `fce827ca2a43994d1d67312f33fe4fef1d97f4d3`
  (`runtime: settle Texture passivation stream`), pushed to `origin/main`.
- GitHub Actions:
  - CI run `27897682907`: success, including deploy job `82552403986`;
  - Docs Truth Check run `27897682904`: success;
  - FlakeHub run `27897682928`: success.
- Staging identity: `https://choir.news/health` reported `status=ok`, upstream
  ok, vmctl enabled/ok, proxy commit/deployed commit
  `fce827ca2a43994d1d67312f33fe4fef1d97f4d3`, upstream sandbox
  commit/deployed commit `fce827ca2a43994d1d67312f33fe4fef1d97f4d3`, and
  deployed/built time `2026-06-21T07:50:24Z`.
- Comet UI proof:
  - marker `CHOIR_NATIVE_SOURCE_ENTITY_PROOF_20260621_005`;
  - initial visible run fragment `bdd7b42d-6...936a00`;
  - researcher run `16eb2f1a-5f53-45a4-b2f1-aa211d896a91`;
  - Texture loop `33ac2a66-6c63-4bf6-8d8a-e0f965133a5b`;
  - activity showed researcher completion, Texture `patch_texture`, "Texture
    created a new revision", and tool output receipt;
  - v2 document carried deployed commit
    `fce827ca2a43994d1d67312f33fe4fef1d97f4d3`;
  - v2 rendered inline native source chip
    `Source: GPT-5.5 Model | OpenAI API`;
  - evidence ledger preserved source handle `src_b430c59e225afa4c`, content id
    `67d9c15a-cedb-4b70-a559-b340d29ffc2f`, evidence id
    `2e421b69-fed6-4d99-b2f7-0220831b7d25`, and fetched URL
    `https://developers.openai.com/api/docs/models/gpt-5.5`;
  - after a bounded idle wait, the toolbar no longer showed `Revising...`,
    `Revise` was enabled, `Sources` was enabled, and `Publish v2` was enabled;
  - opening `Sources` showed `1 represented source`, source row
    `GPT-5.5 Model | OpenAI API`, `CONTENT ITEM`, `AVAILABLE SOURCE`, source
    artifact title `GPT-5.5 Model | OpenAI API`, and URL
    `https://developers.openai.com/api/docs/models/gpt-5.5`.
- Residual observation: the Comet accessibility tree still retained one stale
  `Continuing...` text node below the `_005` document even after visible
  toolbar state and source-panel controls settled. It did not block owner
  revision, publish, or source inspection controls. Treat this as
  accessibility/live-region polish, not as a blocker for the repaired
  passivation stream settlement.

Rollback: revert `fce827ca2a43994d1d67312f33fe4fef1d97f4d3` if the
document-stream passivation mapping regresses other Texture lifecycle states.
The problem checkpoint `a8a62b29` should remain as durable evidence of the
discovered stream-settlement gap.

Result: source-backed Texture revision settlement now has local tests, runtime
shards, CI, deploy identity, and deployed Comet proof through the logged-in
owner browser. Next realism axis is settlement review and optional cleanup of
the stale accessibility/live-region `Continuing...` node; any new runtime fix
must begin with a problem checkpoint if staging evidence shows behavior impact.

## 2026-06-21 - Texture Settlement Review Checkpoint And Acceptance Record

Claim: the `_005` source-panel proof should be captured in durable product
acceptance without inflating its level. The visible Texture id
`33ac2a66-6c63-4bf6-8d8a-e0f965133a5b` is a document/loop-facing id, not a
stored run id for the run-acceptance synthesizer; the correct synthesis key is
the trace trajectory for the proof.

Move: used the logged-in Comet browser session for `yusefnathanson@me.com` to
renew `/auth/session`, query `/api/trace/trajectories`, identify trajectory
`c3e06265-48a7-4f00-91d1-068c3706ff58` for
`CHOIR_NATIVE_SOURCE_ENTITY_PROOF_20260621_005`, and then POST
`/api/run-acceptances/synthesize` with that trajectory, CI run `27897682907`,
deploy job `27897682907:82552403986`, and staging URL `https://choir.news`.
The earlier browser-authenticated attempt using only the visible
`33ac2a66-6c63-4bf6-8d8a-e0f965133a5b` loop id returned `record not found`,
which explains why the mission needed a trace-trajectory lookup rather than a
runtime code change.

Expected Delta V: -1 on the settlement-record/document-drift obligation if the
product synthesizer stores a record at the honest evidence level and the
Parallax State no longer claims stale `planned, V=12` status.

Actual Delta V: -1 for the settlement-record/document-drift obligation. Current
exit V is 3: reconcile the original witness clauses, obtain the independent
prover/widest checker required before exit, and decide the stale accessibility
`Continuing...` residual.

Receipts:
- Comet-authenticated synthesis returned HTTP `202` and
  RunAcceptanceRecord `runacc-21e9c87d45c3965bba1d`.
- Record fields included target mission
  `mission-texture-durable-thread-v1`, source prompt objective
  `CHOIR_NATIVE_SOURCE_ENTITY_PROOF_20260621_005 passivation stream
  source-panel settlement proof`, trajectory
  `c3e06265-48a7-4f00-91d1-068c3706ff58`, loop id
  `c3e06265-48a7-4f00-91d1-068c3706ff58`, authority profile
  `texture > conductor > researcher`, deployment commit
  `fce827ca2a43994d1d67312f33fe4fef1d97f4d3`, CI run `27897682907`,
  deploy run `27897682907:82552403986`, acceptance level
  `staging-smoke-level`, and state `blocked`.
- The `blocked` state is the correct product-level result for this proof:
  source-panel settlement exercised conductor/Texture/researcher but did not
  exercise super worker, AppChangePackage adoption, promotion, rollback, or
  continuation checkpoints.
- `docs/mission-texture-durable-thread-v1.md` now has a compact current
  Parallax State, records the acceptance id, names the exit V=3 obligations, and
  updates the Suggested Goal String away from the stale `planned, V=12` route.

Open edge: no runtime behavior changed in this pass. Mission exit still needs
the Parallax handoff-tier review; under the current no-subagent operating
constraint, no independent prover was run here.

## 2026-06-21 - Settlement Audit Finds Stale Researcher Verifier

Claim: the next settlement-review move should reconcile the original durable
thread witness clauses against current evidence before calling the mission
settled. A focused verifier pass is admissible because it checks the same-thread
researcher/source-delivery bridge that the mission still needs for exit.

Move: inspected the current `docs/mission-texture-durable-thread-v1.md` state,
runtime code for `update_coagent` delivery, mailbox reactivation, passivation
stream metadata, and the Texture prompt/guard deletion surface. Then ran focused
local verifier commands for same-thread researcher/super delivery,
update-coagent mailbox delivery, passivation/resume, and passivation stream
settlement.

Expected Delta V: -1 if the audit proved the original witness clauses and left
only independent prover / live-region residual. Otherwise, name the unsupported
edge without touching runtime behavior.

Actual Delta V: +1. The audit found a verifier gap, not a product-path runtime
fix candidate yet. `TestTextureCreatedResearcherEvidenceWakesTextureV2` fails
alone on current `main` because its stub provider waits for the deleted
`model_prior_interim` completion-guard reminder before spawning researcher. That
guard dependency is stale after the model-prior completion guard deletion.
`TestTextureCurrentEventsPromptCanOpenProbePathWithoutCompletionGuard` passes
alone, but in a broader parallel focused batch it can fail with
`choices = []string{"", ""}` because it asserts the async researcher provider
call has already appended a third tool-choice observation after the researcher
run starts. The current verifier therefore cannot serve as settlement evidence
until the stale guard dependency and async assertion are repaired or retired.

Receipts:
- Failing stale verifier:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run '^TestTextureCreatedResearcherEvidenceWakesTextureV2$' -count=1 -v`
  failed with `Texture-created researcher run was not found`.
- Stale mechanism: `textureResearchEvidenceLoopProvider` only emits
  `spawn_agent` when the last user message contains `model_prior_interim`; that
  was the old completion-guard route this mission intentionally deleted.
- Healthy replacement path in isolation:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run '^TestTextureCurrentEventsPromptCanOpenProbePathWithoutCompletionGuard$' -count=1 -v`
  passed and showed a researcher coagent run started by ordinary tool choice.
- Broader focused batch excluding the stale researcher test still failed under
  parallel execution at
  `TestTextureCurrentEventsPromptCanOpenProbePathWithoutCompletionGuard`, with
  `texture choices = []string{"", ""}`, while its run-alone result passed. This
  points to verifier synchronization/assertion drift rather than a confirmed
  product regression.
- Other same-thread/passivation/update-coagent tests in that batch passed before
  the failing parallel assertion ended the package run, including update-coagent
  backlog delivery, rewarm selection, successful activation gating, idle
  passivation/resume, active-revision follow-up, super-evidence V2 wake, and
  passivation stream mapping.

Open edge: first repair or retire the stale/flaky verifier evidence in a
test-only yellow commit. Do not claim mission settlement or start a runtime fix
from this evidence alone.

## 2026-06-21 - Same-Thread Researcher Verifier Repair

Claim: the stale/flaky local verifier can be repaired without changing runtime
behavior by aligning the test stubs with the post-guard Texture route and by
removing a race against async researcher provider observation.

Move: updated `textureResearchEvidenceLoopProvider` so the
`TestTextureCreatedResearcherEvidenceWakesTextureV2` stub opens researcher after
the first Texture write via ordinary `spawn_agent`, rather than waiting for the
deleted `model_prior_interim` completion-guard reminder. Updated
`TestTextureCurrentEventsPromptCanOpenProbePathWithoutCompletionGuard` to assert
that all observed conductor/Texture tool choices are unconstrained without
requiring the async researcher provider turn to append a third observation
before the tested Texture path settles.

Expected Delta V: -1 if the stale researcher verifier and parallel batch
assertion are repaired, with no runtime implementation changes.

Actual Delta V: -1. Current exit V returns to 3: reconcile original scope,
obtain independent prover/widest checker before exit, and decide the stale
accessibility/live-region residual.

Receipts:
- Specific repaired tests:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run '^TestTextureCreatedResearcherEvidenceWakesTextureV2$|^TestTextureCurrentEventsPromptCanOpenProbePathWithoutCompletionGuard$' -count=1 -v`
  passed.
- Focused settlement verifier set:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run '^TestInitialTextureRunWritesBeforeSpawningResearcher$|^TestTextureCurrentEventsPromptCanOpenProbePathWithoutCompletionGuard$|^TestTextureCreatedResearcherEvidenceWakesTextureV2$|^TestTextureCreatedSuperEvidenceWakesTextureV2$|^TestTextureRevisionRunParksAndConsumesUpdateWithoutColdWake$|^TestTextureIdlePassivatesAndResumesSameRun$|^TestTextureUpdateCoagentDuringActiveRevisionTriggersSameRunFollowUp$|^TestTextureStreamEventMapsTexturePassivationToSynthCompleted$|^TestTextureIdlePassivationEventCarriesDocumentStreamCompletionPayload$|^TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce$|^TestUpdateCoagentDeliveryRequiresSuccessfulActivation$|^TestUpdateCoagentDeliveryIgnoresStrayWorkerUpdateMetadata$|^TestUpdateCoagentWarmActivationInjectsPendingTurn$|^TestCoagentRewarmUsesResidentActivationNotActiveRunProxy$|^TestCoagentRewarmIgnoresBlockedHistoricalActivation$' -count=1 -v`
  passed.

Open edge: this is still not handoff-tier settlement. The mission still needs a
scope reconciliation and an independent prover / widest checker before exit.

## 2026-06-21 - Scope Reconciliation

Claim: the original mission claim can now be reconciled without new runtime
construction by separating supported scoped evidence from residual edges. The
mission should not require impossible universal proof before it can hand off,
but it also must not overclaim promotion/continuation or every possible Texture
entry path.

Move: reconciled the original witness clauses against current receipts in the
mission state and ledger. The result is scoped support rather than blanket
settlement: runtime-owned `update_id`, owner-visible work-state revisions,
durable mailbox delivery, same-run passivation/resume for established Texture
documents, event-driven `update_coagent` delivery, deletion of the named
first-paint/initial-super/model-prior scaffolds, and source-backed researcher
evidence incorporation all have focused-test plus staged proof receipts.

Expected Delta V: -1 if the audit can name the supported scope and residual
edges clearly enough that the next pass no longer needs to re-litigate the
original witness clauses.

Actual Delta V: -1. Current exit V is 2: obtain the independent
prover/widest checker required before any Parallax exit, and decide the stale
Comet accessibility/live-region `Continuing...` residual.

Receipts:
- Current Parallax State now names supported scope and residual edges instead
  of treating the original mission wording as either fully settled or fully
  open.
- Product acceptance remains `runacc-21e9c87d45c3965bba1d`,
  `staging-smoke-level` / `blocked`, tied to deployed commit
  `fce827ca2a43994d1d67312f33fe4fef1d97f4d3` and trajectory
  `c3e06265-48a7-4f00-91d1-068c3706ff58`.
- Residuals explicitly named: no universal proof over every possible Texture
  document entry path; always-deep means obligation/prompt-driven
  probe-and-incorporate loops with receipts, not exhaustive research for all
  prompts; and no promotion-level or continuation-level acceptance is claimed.

Open edge: Parallax still forbids self-checked exit. Under the current
no-subagent operating constraint, the independent prover obligation remains
open unless the owner explicitly authorizes a separate prover or an equivalent
non-authoring checker.
