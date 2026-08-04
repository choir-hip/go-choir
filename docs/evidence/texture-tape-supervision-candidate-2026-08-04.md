# Texture Tape Supervision — Frozen Pre-Implementation Candidate

**Captured:** 2026-08-04T04:33:15Z  
**Source base:** `d163a4aaa732e54ad56cbb7fc8a08d3aa8722268` (`origin/main`)  
**Staging observation:** `choir.news /health` still reports deployed commit
`794b99c9bf1526ee74a72fec8ba31e0c21df6d16`, built
`20260801171937`, deployed `2026-08-01T17:40:57Z`  
**Definition:**
[`choir-texture-tape-supervision-2026-08-03.md`](../definitions/choir-texture-tape-supervision-2026-08-03.md)  
**Candidate status:** frozen design and bounded model; no protected runtime code
has changed and no event, effect, migration, deployment, or external mutation
has occurred.

## Gate and ceremony

This is the code-free first gate for a **red** mission. The protected surfaces
are `ComputerEventAppender`, immutable/private event payloads, corpusd head CAS,
embedded Dolt, canonical Texture writes, lifecycle reducers, trajectory
obligations, assignment/attempt lineage, update dispositions, settlement,
future capsule candidate identity, privacy keys, public API/CLI, reconstruction,
and deployment identity.

The governing conjecture remains: an agentic Texture can present the owner's
human-bandwidth document while Super supervises concurrent CoSupers when every
material claim, obligation, result, dissent, decision, and intent revision is a
typed projection of one per-computer tape. The fastest falsifiers are an
alternate production writer, a non-reconstructible payload, branch authority,
lost attempt lineage, stale semantic merge, settlement with unresolved state,
or direct promotion of a branch result.

Admissible evidence at this gate is read-only caller inventory, source-bound
schema/reducer design, and bounded model checking. It cannot prove code
conformance, crash recovery, deployed reconstruction, human usability, or
product acceptance. Rollback is deletion of this unimplemented candidate only.
Heresy delta: `discovered=[H032]`, `introduced=[]`, `repaired=[]`.

## Existing substrate and root cause

The replacement substrate already exists:

- `internal/computerevent/appender.go` pins immutable artifacts, durably
  prepares the local event projection, performs one corpusd canonical-head CAS,
  verifies the signed receipt, finalizes locally, recovers prepared events, and
  reconstructs from the externally retained chain.
- `internal/computerevent/reducer.go` currently reduces only compact canonical,
  desired, effective, pending-transition, commitment, reducer-version, and key
  epoch heads. It has no multi-object supervision reducer.
- `internal/store/computer_events.go` currently prepares/finalizes only the
  embedded event index and projection head.
- `internal/store/lifecycle.go` has the reusable durable-work invariants—typed
  commands, deterministic digests, command replay, object preconditions,
  settlement predicates, and atomic object batches—but commits them directly
  to embedded Dolt with a separate `LifecycleEvent` tape.
- `internal/objectgraph/dolt_store.go` has the serializable conditional batch
  seam needed by a local reducer, but it cannot yet join an externally owned SQL
  transaction used by event finalization.

The repair therefore connects the existing appender and lifecycle reducer. It
does not add a log, reconcile two writers, or patch Texture UI around H032.

## Production mutation caller inventory

The post-cutover classes are closed:

- **canonical transaction caller** — may submit a typed private supervision
  transaction to the sole guest `ComputerEventAppender` service;
- **reducer-private projection** — may update only while finalizing or replaying
  an acknowledged canonical event;
- **migration-only** — may read old state and construct one content-addressed
  import; it cannot continue ordinary writes;
- **derived compatibility projection** — may be regenerated from event-derived
  state and never author meaning; or
- **deterministic refusal** — returns `ErrSupervisionAuthorityRequired` or
  `ErrSupervisionImportRequired` before mutation.

### Texture and prompt ingress

| Production caller | Current write | Frozen disposition |
|---|---|---|
| `internal/textureowner/texture.go:HandleTextureCreateDocument` | `Store.CreateDocument` | canonical `trajectory_started + intent_revised + texture_revision` transaction for new documents; pre-cutover state requires explicit import |
| `handleTextureImportMarkdownLineage`, aliased import | repeated `CreateDocument/CreateRevision/UpsertDocumentAlias` | migration-only importer producing one sorted source manifest and one import transaction; aliases become derived projection |
| `handleTextureCreateRevision`, `createRebasedUserRevision` | lifecycle `CommitLifecycleArtifactHead` or raw `CreateRevision` | canonical `texture_revision`; strict artifact/intent/lifecycle heads; raw path refuses |
| restore and accepted merge (`texture.go`, `texture_merge.go`) | raw `CreateRevision`, then generic event | canonical `texture_revision`; lifecycle identity is mandatory; generic event becomes derived |
| title update | `UpdateTextureDocumentTitleAuthority` or raw `UpdateDocument` | canonical `texture_revision` metadata mutation with head CAS; current title-only CAS is removed from production authority |
| delete/archive | `ArchiveTextureDocumentAuthority` or legacy update | canonical `artifact_archived`; history/source/evidence retained; raw path refuses |
| `internal/textureowner/tools_texture.go:commitTextureToolEdit` | `QueueLifecycleUpdate`, then `ApplyLifecycleUpdateWithSourceGraph`, then mutation/checkpoint/event writes | one transaction containing Researcher/CoSuper packet, Texture revision/source projection, update/work dispositions, and exact refs; queue/apply split removed |
| `texture_agent_revision.go` event and mutation helpers | `AppendEvent`, mutation stale/complete writes | canonical semantic action first; agent mutation, delivery, controller checkpoint, and generic event are reducer-derived/recovery-only projections |
| `coagent_route.go:StartTextureSourceLifecycle`, `texture_handoff.go` | `StartLifecycle` | canonical start/import-aware transaction |
| `texture_controller.go` | `ReplaceLifecycleActivation` plus mutation stale write | activation/mutation state is actor-recovery projection derived from canonical messages/attempts; it cannot author trajectory meaning |

### Runtime, work, messaging, and settlement

| Production caller | Current write | Frozen disposition |
|---|---|---|
| `internal/agentcore/runtime.go:createRunWithMetadata`, `StartCoagentRun`, `ensureSpawnedCoagentWorkItem` | lifecycle start/open/replace or legacy trajectory/work/run | Super assignment transaction before execution; attempt identity before slot/run claim; legacy lifecycle-shaped writes refuse |
| `completePromptBarDecisionRun` | unconditional legacy trajectory/run/events | Texture-owned start/intent transaction or deterministic refusal; no legacy lifecycle bypass |
| `runtime_persistence.go:persistLifecycleSubmittedRun` | mutation row, then activation, then event | actor recovery rows are derived after canonical transaction; no semantic claim before append |
| `tools_worker_update.go:newUpdateCoagentTool` | lifecycle queue or legacy update | canonical `researcher_packet_recorded` or `attempt_result`; legacy lifecycle target refuses |
| `super_controller.go` completion | generic work status only outside lifecycle | derived run delivery; Super disposition transaction owns assignment/work closure |
| `Store.ReconcileLifecycleSettlementForTerminalRun` | later replace/settle transaction | canonical Super settlement proposal plus Texture/owner settlement; recovery may retry exact command only |
| `ClaimCoSuperSlot` / `ReleaseCoSuperSlotClaim` | direct `co_super_slots` SQL | scheduling projection only after typed assignment/attempt authority; a slot never creates or settles work |
| generic `AppendChannelMessage`, actor update log, inbox delivery | separate delivery state | canonical `actor_message_recorded` owns semantic message; narrow actor log/delivery markers remain recovery projections |
| generic `AppendEvent` / `EmitProductEvent` | object-graph event plus bus | derived Trace/notification projection carrying canonical event ref; no independent semantic event |
| `CreateTextureDecision`, `CreateEvidence` | independent object graph | Super/owner decision and Researcher packet are canonical mutations; immutable evidence may be pinned first and referenced, but cannot claim incorporation |

### Legacy generic and publication paths

| Production caller | Current write | Frozen disposition |
|---|---|---|
| `Store.CreateTrajectoryIfAbsent`, `CreateWorkItem`, `UpdateWorkItemStatus/Details`, trajectory subject/status APIs | legacy object graph; work APIs lack complete lifecycle guards | every lifecycle/supervision identity deterministically refuses; non-supervision ledgers remain outside this mission |
| `agentcore/trajectory.go`, `email_lifecycle.go`, `api_trajectory.go` generic creation | legacy trajectory/run | non-Texture/non-supervision work remains outside scope; any supervision ComputerID/trajectory refuses or uses the transaction service |
| `wire_publication.go` generic processor/story/publication work | legacy work/status/subject refs | publication ledger remains separate; Texture trajectory obligations/refs use canonical transaction; lifecycle-shaped generic work refuses |
| `wire_platform_publish.go:persistWirePlatformPublicationRef` | lifecycle `RecordLifecycleRefs` or legacy revision metadata | canonical ref mutation for supervised Texture; unbound publication metadata is a derived publication projection or migration-only |
| raw graph/store writers in `internal/store/texture.go`, `graph_store.go`, `trajectory.go` | document/revision/source/decision/evidence/work objects | unexport or guard; reducer-private apply, migration read, unrelated-ledger use, or pre-mutation refusal only |
| `LifecycleEvent` and lifecycle command receipt writers/readers | independent event/receipt objects | mechanically derived compatibility rows binding the canonical event digest, or deleted; no production writer outside finalization/replay |

The implementation gate must add a source-level caller detector for these
classes. Completion requires re-inventory proving every production caller uses
the service or refuses, and that `LifecycleEvent` has no independent writer.

## Frozen canonical transaction

### Outer event

Add one outer event kind, `supervision_transaction`, to the existing computer
event envelope. The event exposes only mechanical routing and commitments:
`ComputerID`, canonical sequence/previous head, UUIDv7 event ID, idempotency key,
trajectory ID, actor profile and authority ref, reducer version, privacy class,
and one output artifact ref. Titles, prose, objectives, findings, source claims,
scopes, decisions, and result content never enter the plaintext envelope.

Every supervision event has exactly one owner-private output artifact with media
type `application/vnd.choir.supervision-transaction.v1+json`. It is encrypted
once and pinned before the event. Existing evidence/artifact refs are named
inside that transaction; mixed-class payload sets are forbidden. This avoids
the current platform validator's uniform-privacy conflict and makes retry byte
identity explicit.

### `SupervisionTransactionV1`

The canonical plaintext payload contains:

```text
schema = choir.supervision_transaction.v1
reducer = supervision/v1
transaction_id = event_id
owner_id, computer_id, trajectory_id
command_id = event.idempotency_key
command_digest
actor { actor_id, role, authority_ref }
expected {
  lifecycle_version
  intent_revision_id
  artifact_head_revision_id
  settlement_proposal_id
}
observed_base {                 # required only for worker attempt/result
  canonical_event_head
  intent_revision_id
  artifact_head_revision_id
}
mutations[] { kind, body }      # non-empty, ordered tagged union
```

`command_digest` is the SHA-256 of canonical JSON with that field empty.
`event.OccurredAt` supplies reducer time; reducers never call `time.Now()`.
Every referenced identity is owner/computer/trajectory scoped and every object
records its creating/updating canonical event ref. Duplicate mutation targets,
unknown fields/kinds, non-canonical ordering, cross-scope refs, or role-invalid
combinations refuse before pin/CAS.

Idempotent retry first looks up the embedded/canonical request by
`(ComputerID, command_id)`. Same key and digest returns the original event and
receipt without regenerating UUIDs, timestamps, nonces, or ciphertext. A
changed digest conflicts. `AppendNew` must not silently rebase an existing key.

### Closed mutation vocabulary

| Mutation kind | Authorizer | Projection / rule |
|---|---|---|
| `projection_imported` | migration authority | establishes the first supervision reducer state from one content-addressed legacy snapshot |
| `trajectory_started` | Texture | creates trajectory, initial obligation, V0 artifact and Texture actor identity |
| `intent_revised` | owner through Texture, or Texture within granted intent | immutable intent parent/delta; material live change opens a rebase obligation |
| `texture_revision` | owner or Texture | immutable structured revision/narrative with exact refs; advances artifact head under CAS |
| `actor_message_recorded` | Texture or Super within role envelope | addressed message to Researcher, Super, CoSuper, Texture, or owner attention |
| `researcher_packet_recorded` | Researcher | existing sourced packet plus uncertainty/conflicts; cannot edit Texture |
| `assignment_opened` | Super | existing work identity plus parent decision, intent, observed base, scope/capability/policy digests, obligations, and idempotency commitment |
| `attempt_started` | Super authorization plus runtime receipt | stable assignment/attempt/ordinal/run identity and observed base; execution may overlap |
| `attempt_result` | assigned CoSuper | outcome digest and evidence/artifact refs; retains original base and mechanically marks cancellation-late arrival |
| `super_belief_recorded` | Super | current belief, uncertainty, evidence refs, and superseded belief ref |
| `super_finding_recorded` | Super | stable fingerprint, invariant/subject, severity, state, evidence, expected response |
| `dissent_recorded` | Researcher, CoSuper, verifier, or Super | minority stance/evidence retained separately from selected decision |
| `super_reconciliation_recorded` | Super | reconciles findings, evidence, disagreement, assignments, and obligations |
| `super_decision_proposed` | Super | options, selected proposal, evidence/dissent refs, and reserved authority |
| `owner_decision_recorded` | owner through Texture | exact owner-reserved decision; no effect acceptance in this mission |
| `assignment_cancelled` | Super | closes future assignment authority but preserves attempts/results |
| `disposition_recorded` | Super | one current disposition for assignment/attempt/result/update/premise: `preserved`, `invalidated`, `superseded`, `compensation_required`, `cancelled`, `late`, `incorporated`, or `rejected` |
| `rebase_opened` | reducer from material `intent_revised` | binds old/new intent heads and enumerates every affected assignment/work/belief/artifact premise |
| `settlement_proposed` | Super | names current heads and complete obligation/disposition/evidence set |
| `trajectory_settled` | Texture/owner | requires current proposal and deterministic settlement query success |
| `artifact_archived` | owner/Texture | only after non-live trajectory; retains complete history/evidence |

Mandatory Texture control blocks are reducer outputs, not authorable mutations:
current intent/delta; fulfillment state; Super belief changes; material
obligations/blockers; dissent; irreversible gates; owner-only decisions;
attention requests; exact drill-down refs; and an honest bounded overflow count.
A Texture narrative is a `texture_revision`; it cannot hide or override these
blocks.

Existing `Document`, `Revision`, `TrajectoryRecord`, `WorkItemRecord`,
`CoagentSourcePacket`, source graph, command receipt, digest, and object
precondition rules are reused. New projection records are limited to
`IntentRevision`, `Attempt`, `SuperBelief`, `Finding`, `Dissent`,
`Reconciliation`, `Decision`, `Disposition`, `RebaseObligation`, and
`SettlementProposal`. `WorkItemRecord` becomes the assignment projection rather
than adding a competing work object.

### Freshness and settlement

The appender sequencing head and worker observed base remain distinct:

- import, intent, artifact, assignment, cancellation, disposition,
  reconciliation, settlement, archive, and future desired-state transitions
  require current semantic heads and refuse stale input;
- attempt starts/results append against the current canonical sequencing head
  while retaining their original intent/artifact/canonical observed base;
- a cancelled attempt may deliver a retained `late` result, but that result
  clears any prior attempt disposition and opens a new Super disposition
  obligation; and
- settlement refuses while any required assignment/attempt/result/update,
  finding, dissent, rebase target, compensation, artifact head, evidence ref,
  or owner-attention decision is unresolved.

## Atomic prepare/finalize and reconstruction

The frozen local protocol is:

1. validate/canonicalize the plaintext transaction and all semantic heads;
2. encrypt once and pin its exact envelope; bind the pin receipt and artifact
   ref into the outer event request;
3. pin the event;
4. `ProjectionStore.Prepare` verifies/decrypts the exact payload and persists a
   reducer-versioned prepared transaction/plan without changing visible
   semantic objects;
5. corpusd performs the sole canonical head CAS;
6. verify the signed head receipt; and
7. `ProjectionStore.Finalize` opens one serializable embedded transaction that
   revalidates conditions, applies the complete object/edge batch, writes the
   event index/head and derived compatibility receipt/event rows, then commits
   all or none.

`internal/objectgraph` gains a transaction-scoped conditional batch primitive
so finalization owns the SQL transaction; no lifecycle method commits a second
batch. A failed pre-CAS prepare is discarded. A post-CAS crash finalizes the
persisted exact plan. Any plan/version/digest disagreement returns
`ErrNeedsProjectionRepair` and keeps the computer stopped; it never overwrites
or skips the event.

Reconstruction adds an authenticated, read-only corpusd artifact fetch for
content-addressed event payloads. The guest verifies the artifact digest and
pin metadata, decrypts by the envelope's historical key digest, revalidates the
transaction/event join, and replays from sequence zero. Current-key-only private
decryption is insufficient: the compatibility floor must retain a guest-owned
historical decrypt keyring across rotations. corpusd receives ciphertext and
commitments only.

Snapshot reads use one serializable embedded read and report canonical sequence,
event head, reducer version, projection digest, and semantic fields. Digest
equality never substitutes for field-level comparison.

## Content-addressed import

Before any ordinary supervision event for existing state, a migration reader
constructs `ProjectionImportV1`:

```text
schema, reducer_version, owner_id, computer_id
source_dolt_commit, source_projection_digest
legacy_lifecycle_watermark
sorted objects[] { kind, canonical_id, content_hash, canonical_body }
sorted edges[]
explicit refusals[]
projection_digest
```

The import transaction binds this artifact digest and establishes the first
supervision lifecycle/intent/artifact heads. It does not replay old
`LifecycleEvent` rows as if they were canonical computer events. The importer
is additive and idempotent. A changed source digest conflicts. After import,
every old mutation API refuses; `LifecycleEvent` may survive only as a derived
row binding the canonical event digest.

New empty computers start event-native. Existing unimported state receives
`ErrSupervisionImportRequired` before mutation—never a legacy fallback.

## Compatibility floor and cutover

Before the first `supervision_transaction` is emitted, freeze and deploy an
immutable updater `ReleaseManifest` that declares:

- the exact `ComputerID`, accepted event head, `CodeRef`,
  `ArtifactProgramRef`, release digest, and sorted file hashes;
- supported computer-event schema/reducer versions including the new outer
  event and `supervision/v1` payload;
- payload fetch, historical private-key decryption, full reducer replay, and
  field/digest comparison; and
- one global fail-closed `CHOIR_SUPERVISION_WRITES_DISABLED` safety switch
  honored at the canonical service and every legacy ingress.

The switch disables all supervision/Texture/lifecycle semantic writes. It never
routes to legacy code and is therefore not a permanent old/new mode. The final
release retains this rollback switch. Temporary staging computer selection, if
needed for the disposable proof, has a deletion clock in this Definition and
must be removed before terminal closure.

Cutover is event-native-or-refuse: imported/new computers use only the canonical
service; existing unimported computers refuse. The floor rollback rehearsal
sets the global disable before deployment, deploys the exact retained floor,
rebuilds from externally pinned events, and compares all semantic fields. It
emits no `rollback_requested`, `rollback_applied`, checkpoint, route, desired
state, effect, or materialization event. On mismatch, writes stay disabled and
the computer stays stopped.

## Public projection and acceptance seam

Reuse the owner-authenticated product surfaces:

- prompt/Texture writes through `/api/prompt-bar` and `/api/texture/*`;
- extend `/api/trajectories/{id}`, `/events`, and `/stream` with the typed
  supervision snapshot, canonical head/cursor, attempts/dispositions, dissent,
  rebase, decisions, mandatory control blocks, overflow, and exact refs;
- extend `choir lifecycle snapshot/events` (or one clearly named supervision
  command group) with the same DTO and no raw Trace requirement;
- use `/api/compute/status`, exact-computer `/api/computers/{id}/lifecycle/*`,
  and `/api/acceptance/execution-identity` for no-SSH identity/restart evidence.

There is no current `/api/current-computer` route. The candidate does not invent
acceptance evidence from that absent path. Generic `/api/trace`, raw event
mutation, internal routes, SSH, and raw vmctl remain inadmissible.

## Bounded model receipt

`specs/texture_supervision.tla` and `.cfg` map the appender recovery boundary,
assignment/attempt bases, retry/cancel/late disposition, rebase and settlement
blockers, write-disable/rebuild behavior, and the future composed-current-base
candidate with at most one pending transition. The model contains no rollback
action and keeps `rollbackEventCount=0`.

Local TLC 2.19 (TLA tools 1.7.4), JDK 17, final bounded configuration:

- assignments: 2; attempts: 2; results: 2; candidates: 1;
- maximum acknowledged events: 9;
- `31,354,454` states generated;
- `10,691,808` distinct states explored;
- complete depth: 13;
- elapsed: 146 seconds; and
- all declared invariants passed with zero states left on the queue.

The first model run found that dissent could be recorded after settlement; the
model was repaired to make settlement terminal and the complete bounded run was
repeated. This receipt proves only the finite abstract transition system. Code
conformance tests, generative N=3+ scenarios, crash injection, and deployed
reconstruction remain mandatory.

## Frozen implementation order

1. Land the compatibility reader/floor: event kind and version declaration,
   private payload fetch/keyring, pure transaction decode/reducer plan,
   transaction-scoped embedded finalize/replay, write-disable refusal, model and
   exact schema tests. New writes remain disabled.
2. Independently review that exact floor candidate. Deploy and pin the retained
   release before any new event.
3. Add the canonical transaction service, import, and event-first callers;
   delete/guard every legacy writer and add the caller detector.
4. Add assignment/attempt/rebase/settlement projection and minimal desktop/CLI
   view. Effects remain OFF.
5. Run disposable staging import/fan-out/failure/rebase/restart/rebuild proof,
   then the declared cutover and compatibility-floor rollback rehearsal.

Any change to the vocabulary, authorizer matrix, import digest, privacy model,
atomic finalize boundary, compatibility rule, or promotion seam invalidates
this frozen candidate and requires a new digest and independent adjudication.
