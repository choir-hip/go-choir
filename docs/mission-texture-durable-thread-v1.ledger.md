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
