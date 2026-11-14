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
| `Revision` | replay, applied-turn, tests | Yes, when the batch writes the revision: the object at the revision's canonical id has the revision's own JSON as its body and revisions are immutable (no live path rewrites one: `PatchRevisionMetadata` refuses lifecycle revisions, and the legacy `PutRevision` writes a different, unscoped canonical id; only the retired vocabulary cutover could re-encode an object body in place). Read by id, never by head. | No, checked at commit time byte for byte (`bytes.Equal(obj.Body, json.Marshal(revision))`) | **Dropped** when the batch writes it, named by `RevisionID`. Embedded otherwise. |
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
  `GetAppliedTextureTurnByCallerRun` scans a trajectory's commands newest
  first but resolves the revision only for the command that matches (kind,
  Texture turn, agent, active run: `texture_turn.go`, the resolve after the
  match), so a missing revision breaks recovery for its own run only. (An
  earlier version of this record said the whole trajectory aborts; that was
  wrong.)
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
