# A lifecycle command embeds a copy of what its own batch writes

Written 2026-11-14, revised after review round 1, station R1 (second item) of
[store-once](../design/store-once-2026-11-14.md). Class: red (tape and
evidence, Texture canonical writes).

## Problem

`commitLifecycleTransition` (`internal/store/lifecycle.go`, the
`storedReceipt.StoredResult = &types.LifecycleStoredResult{...}` block) puts a
copy of the command's whole result inside every `choir.lifecycle_command`
object. For a Texture write the batch already writes the revision as its own
`choir.texture_revision` object (`texture_turn.go:455`, `lifecycle.go:4466`,
`4554`, `4872`, `1532`), so the body (`BodyDoc`, `Content`, `SourceEntities`,
`Provenance`) is on the tape twice in one batch and in the guest store twice.
The tape counter measured the lifecycle command at 20.6 MB of the paper's
window (5% of tape bytes); the revision share of it is not separately
measured (inferred: most, since the revision is the only field that grows
with the document).

## Evidence: readers of the stored result

| Reader | Where | Fields it reads |
| --- | --- | --- |
| Idempotent re-submit of the same command | `replayLifecycleCommand`, `lifecycle.go` (about 1296) | all |
| Applied Texture turn by caller run | `GetAppliedTextureTurnByCallerRun`, `texture_turn.go:832` | all, selected by `TextureTurn` and `Agent.ActiveRunID` |
| Control-activation failure evidence | `lifecycle_control_activation_failure.go:306-315` | `Events[0]` only |
| Tests | `lifecycle_test.go:1992-1998` | `Trajectory`, `Document`, `Revision`, `Events` of a settlement (a head read from an earlier batch) |

No reader outside `internal/store`. Restore, rematerialize and the ledger
witness move the object body as bytes and never decode `StoredResult`
(`internal/selfdevprotocol/rematerialize.go` works on ops; the vocab migration
`vocab_migrate_og.go:126` rewrites only `command_id` spellings). Idempotent
re-submit returns the result to API callers (`StartLifecycle`,
`CommitLifecycleArtifactHead`, `ApplyTextureTurn` and the rest).

The existing rebuild path (the `StoredResult == nil` branch of
`replayLifecycleCommand`, lines 1305-1345) is not a safe general fallback: it
rebuilds `Trajectory`, `Document` and `Revision` from the **latest** snapshot,
so a later revision would leak into an old command's result, and it rebuilds
no `WorkItem`, `Agent`, `Update`, `TextureTurn`, `Controls` or
`TargetWorkItems` at all. It is only exercised by receipts that were written
without a stored result (`lifecycleTransitionReceipt`).

## Per-field table

| Field | Read by | Rebuildable exactly from objects of this or earlier batches? | Dropping it changes returned bytes? | Decision |
| --- | --- | --- | --- | --- |
| `Revision` | replay, applied-turn, tests | Yes, when the batch writes the revision: the object at the revision's canonical id has the revision's own JSON as its body and revisions are immutable (no live path rewrites one: `PatchRevisionMetadata` refuses lifecycle revisions, and the legacy `CreateTextureRevisionOG` writes a different, unscoped canonical id; only the retired vocabulary cutover could re-encode an object body in place). Read by id, never by head. | No, checked at commit time byte for byte (`bytes.Equal(obj.Body, json.Marshal(revision))`) | **Dropped** when the batch writes it, named by `RevisionID`. Embedded otherwise. |
| `Revision` of a command that only reads the head (settle, archive, continue: `&head`) | same | The head was written by an earlier batch, not this one; it is immutable too, but proving it needs a read at commit | not applicable | **Kept embedded.** Not a second write in the same batch; no measured cost. |
| `Document` | replay, applied-turn, tests | **No.** One mutable object overwritten by every revision (`CurrentRevisionID`, `UpdatedAt`); an earlier state is not kept anywhere but the tape. A rebuild would return the later document. | Yes | **Kept.** Small (no body). |
| `Trajectory` | same | **No.** One mutable object, versioned in place (`LifecycleVersion`, `ReducerSeq`) | Yes | Kept. |
| `WorkItem`, `TargetWorkItems`, `Agent` | same | No, mutable in place | Yes | Kept. |
| `Update`, `Controls` | same | Update packets are mutable (delivery marks them); no earlier state kept | Yes | Kept. |
| `Events` | replay, applied-turn, activation failure | Rebuildable by `ResultEventRefs` (events are immutable objects of the same batch) | No, if checked | **Kept.** Events carry no body (ids, refs, a reason); the saving is small and the activation failure reader would need a second read path. Left for a later step if the counters show it matters. |
| `TextureTurn` | replay, applied-turn | Nowhere else stored; it is the command's own decision | Yes | Kept. |
| `Schema` | replay | a constant | no | Kept (tiny). |

## Two steps, and why

Two seats reviewed the first one-commit version and sent it back with two
blocking findings; both are in this record because they shaped the cut.

1. **The reader gate (B1).** `choir.lifecycle_command` is kernel-owned (any
   reader reads it); `choir.texture_revision` is `desk:texture`, and the gate
   enforces outside tests (all three package `TestMain`s set observe, which hid
   it). A rebuild on the caller's context is refused with `ErrUndeclaredReader`
   for `handleRunCompletion` (`context.Background()`) and
   `sleepTextureMutationAfterIdle`, which then marked a committed mutation
   stale. Fixed in `lifecycleStoredRevision`: the read declares the revision's
   custodian. This is not a widening: the caller already read the command, and
   a command written before store-once carried this very revision inside it.
   Both agentcore callers also stop treating a read error other than not-found
   as "no turn committed".
2. **Revert safety (B2).** A binary that predates the `revision_id` field
   ignores it and returns a nil revision with no error, which callers read as
   "no revision". So the writer may land only after a release that reads the
   field has been the deployed one.

- **Step 1** (`lane/stored-result-step1`) is the reader only: the field, the
  custodian read, every reader going through it, the agentcore error handling,
  enforce-mode tests that build slim commands directly. The writer still embeds
  the revision; nothing on any tape changes. It reverts to today's main at any
  time.
- **Step 2** (`lane/stored-result-step2`, on step 1) is the writer under the
  byte check, with a counter of slim and embedded commands. **Step 2 waits
  until step 1 has been the deployed release, and step 2 must never be
  reverted past step 1 once deployed**: its rollback is "revert to step 1",
  which reads slim commands correctly.

## Invariants

- **A revision named by a command is never retired.** Today this is enforced
  by code: `ObjectRetireProjection.Validate` refuses every object kind but
  `choir.event` (`internal/computerevent/projection_batch.go`), pinned by a
  test that it refuses `choir.texture_revision`. Any future retirement of
  revisions must keep each revision named by a `choir.lifecycle_command`
  `stored_result.revision_id`, or retire the command first, and carry a scan
  test.
- **A named revision that cannot be read is an error, never a nil revision**
  (`TestSlimCommandWithMissingRevisionFailsLoudly`), and the error is
  `ErrLifecycleStoredRevisionUnavailable`, which is deliberately not
  `ErrNotFound`: the first version wrapped the read's not-found, and
  `handleRunCompletion` and `sleepTextureMutationAfterIdle` read not-found as
  "no turn committed" and marked a committed mutation stale
  (`TestFailedTurnReadLeavesTheMutationPendingNotStale` found it).
- **The blast radius of a missing revision is one run.**
  `GetAppliedTextureTurnByCallerRun` lists a trajectory's commands by paging
  on canonical id (hash order, not time) and walks the list from its end. It
  resolves the revision only for the command that matches (kind, Texture
  turn, agent, active run: `texture_turn.go`, the resolve after the match), so
  a missing revision breaks recovery for its own run only. (An earlier version
  of this record said the whole trajectory aborts, and another said "newest
  first"; both were wrong.) Residual, pre-existing and not changed here: a run
  with several Texture-turn commands can match and resolve a turn that is not
  its newest, because the walk order is hash order.
- **After a transient read error at idle passivation the mutation stays
  pending** until the next wake or restart reconciles it; completion is never
  blocked and nothing loops. A pending row keeps the host from calling the
  computer idle meanwhile: a cost, not a wedge.
- The custodian read replaces the caller's declared reader, so the lease-read
  audit (`LeaseAdmitsKind`/`NoteLeaseRead`) does not fire for this one read.
  The declared reader is the strictest true one; the audit signal for this
  read is gone and that is stated here rather than hidden.
- `GetAppliedTextureTurnByCallerRun` returns the receipt as stored, so its
  `Receipt.StoredResult` reflects the stored form; no reader uses it.

## Step 2 as built

`commitLifecycleTransition` names the revision when `lifecycleBatchRevisionForm`
returns `slim`: the batch writes an object at the revision's canonical id
whose body equals `json.Marshal(revision)` (the second marshal is the proof;
do not replace it with a cheaper check). Every command that commits through
`commitLifecycleTransition` is counted by form, after its batch and outbox
succeed, in `telemetry_batching.lifecycle_command_forms` (served with the
other tape counters): `slim`, `embedded_mismatch` (the batch writes the
revision but the bytes differ: a silent zero saving would show here),
`embedded_head` (a head written by an earlier batch) and `no_revision`. A
replay, a lost condition or a failed outbox counts nothing. The story-open
start command (`story_open.go`) is written by its own batch: it is neither
counted nor slimmed, so the counter total is not the count of all
`choir.lifecycle_command` objects (its seed revision is empty; a later step may
slim it). Read the counter on a live computer before and after:
`tape_op_bytes_by_object_kind` for `choir.lifecycle_command` over its op count
is the per-command size.

Tests: `TestLifecycleWriterGateKeepsHeadAndMismatchedRevisionsEmbedded` (a real
terminal settlement stays embedded as `embedded_head`; a batch whose revision
body differs stays embedded as `embedded_mismatch`; a writer that slims every
command fails it, and `TestLifecycleCommandFormsAreCounted`),
`TestLifecycleCommandFormsCountOnTheSuccessPathOnly` (a lost condition counts
nothing; counting before the commit fails it).
`TestSlimTextureTurnCompletesItsMutationUnderEnforce` runs the real
`handleRunCompletion` and `sleepTextureMutationAfterIdle` on a slim turn under
the enforced gate. `ApplyLifecycleUpdate` is covered by the existing
`TestLifecycleSettlementWaitsForUpdateDisposition` flow, whose apply replay now
reads a slim command.

Rollback of step 2 is "revert to step 1", never past it once deployed, and
step 2 is a separate deploy after step 1 is the adopted release (see the
landing precondition below).

## Belief

Only the revision is both large and exactly rebuildable. After the change a
new command whose batch writes its revision carries `revision_id` instead of
`revision`. Old commands carry the embedded copy and replay from it exactly as
today (`lifecycleStoredRevision` returns the embedded copy when present). A
command is slim only if the check passed at commit time, so there is no
setting and no mode.

## Remaining error

- `Document` is the next-largest embedded field only if a document carries a
  large field; it does not (title, ids, times). Not measured on the paper.
- A future retirement of superseded revisions (R2) would break the rebuild of
  a slim command's revision: R2 must keep the revision a command names, or
  read it from the archive.
- Heads embedded by settle, archive, continue and commit-act remain a second
  copy across batches.
- The per-op byte reduction was measured on test data only (a 20 KB body: see
  the test's `t.Logf`), not on the paper.

## Failure list (written before the code)

The test names below are the real ones; a name that matches no test makes
`go test -run` print `ok`.

1. A new Texture command body still contains the revision body. (Step 2:
   `TestLifecycleCommandNamesItsRevisionInsteadOfEmbeddingIt`,
   `TestTextureTurnCommandNamesItsRevision`.)
2. Replay of a command after a later revision returns the later revision or
   document. (`TestLifecycleSlimReplayAfterLaterRevisionReturnsTheOriginalResult`;
   true before the change too, kept as the guard that a by-head rebuild would
   break.)
3. Slim and embedded commands replay to different bytes, or replay rewrites a
   revision object. (`TestLifecycleReplayOfSlimAndEmbeddedCommandsIsByteIdentical`.)
4. Slim commands do not survive a store copy and reopen, plus a new command on
   the restored store. (`TestLifecycleRestoreFromSnapshotPlusTailWithSlimCommands`.)
5. `GetAppliedTextureTurnByCallerRun` returns no revision for a slim turn, or
   cannot read it under the enforced gate.
   (`TestSlimTextureTurnReadsUnderTheEnforcedReaderGate`,
   `TestSlimTextureTurnCompletesItsMutationUnderEnforce`.)
6. A failed read of the turn marks a committed mutation stale.
   (`TestFailedTurnReadLeavesTheMutationPendingNotStale`; each guard and the
   error chain was removed in turn and the test failed each time.)

7. A caller maps `ErrLifecycleStoredRevisionUnavailable` to a client error:
   `HandleTextureLifecycleCreate` and `forkDocument` answered 400.
   (`TestCreateReplayWithAnUnreadableNamedRevisionAnswers500`,
   `TestEveryMappingOfAStoreErrorTreatsAnUnavailableStoredRevisionAsServerFault`;
   both failed first.)
8. A create re-submit proves nothing about the stored revision because the
   handler echoed a locally derived revision id.
   (`TestCreateReplayWhoseCommandNamesAnotherRevisionAnswers500`, failed
   first with 201.) Decision: yes, the handler verifies the revision the store
   returns against the derived id (a mismatch is a 500). The id is a pure
   function of the client request id and the command id is too, so any other
   revision from the same command means the stored result does not name this
   create's revision, which is the invariant the check exists for.

## Every path that can receive the error

The error comes from `lifecycleStoredRevision`, reached by any command replay
whose stored result names a revision (`replayLifecycleCommand`,
`GetAppliedTextureTurnByCallerRun`). Only commands whose batch writes a
revision are slim: start, commit-head, apply-update, apply-turn.

| Caller | Where | What it maps to |
| --- | --- | --- |
| `GET`/`POST` lifecycle routes using `writeLifecycleAPIError` | `internal/agentcore/api_trajectory.go` | 500 `lifecycle invariant broken` |
| create `POST /api/texture/lifecycle-documents` | `internal/textureowner/texture_lifecycle_create.go` (`HandleTextureLifecycleCreate`) | 500 (was 400); also a derived-id mismatch |
| fork `POST /api/texture/fork` | `revise_fork.go` `forkStartFailure` | 500 (was 400) |
| owner revise `POST /api/texture/documents/{id}/revise` commit | `texture_agent_revision.go` (`refuseCommit`) | 500 naming it |
| editor save `POST /api/texture/documents/{id}/revisions` (`handleTextureCreateRevision` in `texture.go`, then `reviseDocument` `ReviseEdit`, then `commitOwnerBodyEdit` in `revise_edit.go`) | `revise_edit.go` (`refuseCommit`) | 500 naming it. Unlike the comment, take and merge cases it does not go through `reviseReplay`, so a re-submit with the same `idempotency_key` reaches `CommitLifecycleArtifactHeadWithSourceGraph` and `replayLifecycleCommand` directly: the one revise-shaped route whose replay exercises the command |
| revise with sources, take, merge commit | `revise_edit.go` (`refuseCommit`) | 500 naming it (these cases replay through `reviseReplay` first) |
| the Texture root's background create | `texture_root.go` `createTextureRootDocument` (via `ensureTextureRoot`, driven by `HandleTextureRootDigest`) | calls `startLifecycleTextureDocument`; a start replay that fails this way returns a wrapped error to `ensureTextureRoot` and `HandleTextureRootDigest` re-arms the digest when the root does not yet exist, and returns `false, nil` when it does. No HTTP status of its own |
| prompt bar and ask-story submit | `prompt_bar.go` `promptBarSubmitFailure` (via `EnsureTextureHandoff`, the conductor start in `texture_handoff.go` and the coagent route start in `coagent_route.go`) | 500 naming it (was a generic 500) |
| prompt eval | `api_texture_prompt_eval.go` | 500 `failed to prepare texture prompt eval` |
| Texture cell tools: atomic turn and Write | `texture_cell_author.go`, `texture_turn_runtime.go` | a tool-result error whose text carries the error (no HTTP) |
| evidence embed | `texture_evidence_embed.go` | a tool-result error, not retried |
| run completion and idle passivation | `internal/agentcore/runtime.go` | the mutation stays pending; logged |
| `SleepAgentMutationAfterTextureTurn` | `internal/store/texture.go` | the error is returned to the two callers above |
| `ArchiveLifecycleArtifact` | `internal/store/texture.go` | an archive command embeds its head, so it is never slim |
| `ContinueLifecycleDocument` | `revise_sources.go`, `texture_agent_revision.go` | 409 (the command embeds its head and cannot be slim today) |

The owner revise and the prompt-bar replays are answered before the store's
command replay is reached (revise reads its revision by its derived id; the
prompt bar replays by run), so a broken command cannot be driven through them
over HTTP; their mappings are covered by the table-driven mapping test.

## What the replay and restore tests do, and do not, prove

They rewrite command objects in place in a store and copy the store's
directory and reopen it. The projection copies object bodies as bytes
(`projectObject`), which is the property they lean on. They are not a tape
replay and not a destroy-and-restore: the full proof is the replay and
restore drill on d@e before the paper.

## Landing precondition (read this before deploying step 2)

The work is cut in two steps, not landed, until each is deployed. Step 2 must
be a separate deploy after step 1 is the release computers have adopted: the
deploy's automatic rollback returns a computer to the previous release, and a
previous release without step 1 reads a slim command as a command with no
revision and no error. Landing both in one train makes that previous release
the one before step 1. Step 2 must never be reverted past step 1 once
deployed.

Step 2 cannot be staged on d@e by a normal landing: the deploy auto-expedites
any release that touches `internal/agentcore/` (it does here), and the ring
soak floors are zero (`internal/vmctl/ring_walk.go`,
`.github/scripts/node-b-ci-deploy` lines 639 to 646), so a landed release
reaches every ring at once. The d@e proof therefore has to happen before the
merge. The route (a branch build on d@e only, or another) is the owner's
decision and is open.

## The d@e proof for step 2 (operator procedure)

Run on d@e only, never the paper or the owner's computers. What to deploy
where is the coordinator's step; these are the checks. Step 2 may not reach any
computer until step 1 is the adopted release (landing precondition above). The
steps use the product API with d@e's key as `scripts/chaos/README.md` does
(the key is read from `.env` into a variable by the harness, never printed),
and the existing drills `scripts/chaos/run s1-guest-crash` and
`scripts/chaos/run s3-planned-restart`, run with `--execute` after a dry run.
I have not run this procedure; it is written from the repo's drills and
counters and must be run before step 2 reaches the paper.

1. **Before the step 2 deploy, read the baseline.**
   `GET /api/runtime/observability` on d@e: record
   `telemetry_batching.tape_op_bytes_by_object_kind["choir.lifecycle_command"]`
   and `tape_ops_by_object_kind["choir.lifecycle_command"]` (mean command op
   bytes = bytes / ops), and `lifecycle_command_forms` (absent before step 2).
   Record `GET /api/computers/<d@e>/self-development/replay-completeness`
   `.result.status`.
2. **After the deploy, drive Texture writes.** Create at least three
   documents: `POST /api/texture/lifecycle-documents` with `{client_request_id,
   title, initial_content}` (201, `doc_id`, `revision_id`). Revise each at
   least once: `POST /api/texture/documents/{doc_id}/revise` with
   `{client_request_id, prompt, expected_head_revision_id}` (202,
   `revision_id`; the head id is `current_revision_id` of
   `GET /api/texture/documents/{doc_id}`). Keep every request body and every
   response. For each document record `current_revision_id` from the document
   read and the revision's `content` from `GET /api/texture/revisions/{id}`
   (the export's `content_hash` also works). The prompt bar does not revise
   these documents unless it is focused on them.
3. **Counters, what must read.** `lifecycle_command_forms.slim` increased by
   the number of Texture commands written (one start per document plus one per
   owner revise, plus one per Texture revision turn the desk then writes: read
   the count of Texture commands from the run records, not from the number of
   requests you sent). `embedded_mismatch` must stay 0: any nonzero value means
   the byte check refused a batch and the saving is silently lost for it, so
   stop. `embedded_head` and `no_revision` may rise (settlements, work
   commands). The mean `choir.lifecycle_command` op bytes must fall against
   step 1's baseline by about the revision body size per Texture command; if
   it did not fall while `slim` rose, the counter or the op accounting is
   wrong.
4. **Replay comparison. Each check names what fails it.**
   a. *Create re-submit.* Re-send each create body unchanged (same
      `client_request_id`, title and content). Pass: 201 with `replay: true`
      and the same `doc_id` and `revision_id` as the first response. This
      check has teeth because `startLifecycleTextureDocument` now compares the
      revision the store rebuilt for the command with the id derived from the
      request: a replay whose command names no revision, another revision or
      an unreadable one answers 500 `lifecycle invariant broken: stored
      revision unavailable`. Fails: any non-201, `replay: false`, a different
      id, or any 500.
   b. *Later revision does not leak.* Revise one document once more (a new
      `client_request_id`), then re-send its original create body. Pass: 201,
      `replay: true`, the original `revision_id` (not the new head). Fails: any
      other id, any 500.
   c. *Revise re-submit.* For a document's latest revise, re-send the same
      body (same `client_request_id`, `prompt`, `expected_head_revision_id`)
      while that revise's revision is still the head. Pass: 202, `replay:
      true`, `revision_id` equal to the first response. Fails: a different
      `revision_id`, a 409 `document head changed` (the head moved: redo it
      while it is the head), or any 500. This reads the revision by its
      derived id; the command replay path itself is exercised by (a) and by
      the unit tests, and the record says so rather than claiming (c) proves
      it.
   d. *Editor save re-submit (the replay check that reaches the command).*
      For a document, save a body with `POST /api/texture/documents/{id}/revisions`
      and `{content, parent_revision_id, idempotency_key}` (201, `revision_id`),
      keeping the body. Re-send it unchanged (same `idempotency_key`, same
      `parent_revision_id`). This route does not pass through `reviseReplay`, so
      the re-send reaches `replayLifecycleCommand` and rebuilds the revision by
      id. Pass: 201 with the same `revision_id` as the first save. Fails: a
      different `revision_id`, a 409 (the head moved: do it while this revision
      is the head), or a 500 naming `stored revision unavailable`.
   e. *Rebuilt revision bytes.* For each document, `GET
      /api/texture/revisions/{id}` for the first and the latest revision: the
      `content` equals what step 2 recorded. Fails: any difference or 404.
5. **Replay completeness.** `replay-completeness` `.result.status` must be
   `equivalent`, `.eligibility.unknown_tables` empty. Anything else fails it.
6. **Crash and planned restart.** `scripts/chaos/run s1-guest-crash --execute`
   then `scripts/chaos/run s3-planned-restart --execute`. Both must pass
   their own checks, and after each, repeat steps 4 and 5 and re-read each
   document's head revision id and `content` hash: they must equal step 2's.
7. **Destroy and restore, with the CLI (`cmd/choir/commands.go`).** d@e is
   `computer-59c902c5d46caecc0a180a8557164e72`; D=that id. Both acts below are
   owner acts and destructive (`ownerAct(computerCheckpointed(...))`): the key
   used needs `owner:acts` (admin does not imply it). If d@e's key lacks it,
   the owner runs them; do not use another key. Run only on d@e.
   a. Record each document's head revision id and `content` hash (step 2) and
      the `slim` count.
   b. List checkpoints: `choir computer checkpoints --computer $D`. Pick one
      taken after the step 2 writes whose verdict the host marks restorable.
      Fetch it: `choir computer checkpoints --computer $D --digest <digest>
      > ckpt.json`.
   c. Write one more revision to one document (the post-checkpoint write).
   d. Restore: `choir computer restore --computer $D --checkpoint-file
      ckpt.json`. This restores VM-local state and the served SPA from the
      checkpoint and rematerializes from the tape.
   e. Rebuild from the tape alone: `choir computer rematerialize-from-tape
      --computer $D --checkpoint-file ckpt.json` (the same artifact), and
      wait for `choir computer status --computer $D` to read active.
   f. After d and again after e, repeat 4a, 4c, 4d, 4e and step 5. Pass: every
      request recorded before the checkpoint answers its re-submit with its
      own recorded pre-checkpoint `revision_id`: the create 201 `replay: true`,
      the revise 202 `replay: true`, the editor save 201, and the same
      `content`; replay-completeness `equivalent`. Use the response each
      request got when it was first sent, not step 2's, for a document whose
      later revisions fall before the checkpoint; a revise re-submit also needs
      the document's trajectory unchanged (it hashes the trajectory id) and
      the head where it was, else it answers 409, which is not a failure of the
      stored revision. The editor save (4e) and the create (4a) are the
      re-submits that replay the stored command after restore and after
      rematerialize. After d only, the revision written after the checkpoint
      must be absent (`GET /api/texture/revisions/{id}` 404). Fails: any 500 or
      a `stored revision unavailable` body; a `revision_id` or `content` that
      differs from the recorded one; a post-checkpoint revision still present
      after d. The `slim` counter restarts with the restored process (do not
      compare it); compare documents.
   Byte-level restore of the host's stores, if wanted, is
   `scripts/off-host-restore-drill.sh <snapshot> tape` on Node B under the
   memory cap in its header. I have not run any of this.
8. **Failure means stop.** Any failed check above blocks step 2 from the
   paper; roll d@e back to step 1 (never past it).
