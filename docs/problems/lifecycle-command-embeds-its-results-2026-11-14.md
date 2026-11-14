# A lifecycle command embeds a copy of what its own batch writes

Written 2026-11-14, station R1 (second item) of
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
| `Revision` | replay, applied-turn, tests | Yes, when the batch writes the revision: the object at the revision's canonical id has the revision's own JSON as its body and revisions are immutable (no path rewrites one; `PatchRevisionMetadata` refuses lifecycle revisions). Read by id, never by head. | No, checked at commit time byte for byte (`bytes.Equal(obj.Body, json.Marshal(revision))`) | **Dropped** when the batch writes it, named by `RevisionID`. Embedded otherwise. |
| `Revision` of a command that only reads the head (settle, archive, continue: `&head`) | same | The head was written by an earlier batch, not this one; it is immutable too, but proving it needs a read at commit | not applicable | **Kept embedded.** Not a second write in the same batch; no measured cost. |
| `Document` | replay, applied-turn, tests | **No.** One mutable object overwritten by every revision (`CurrentRevisionID`, `UpdatedAt`); an earlier state is not kept anywhere but the tape. A rebuild would return the later document. | Yes | **Kept.** Small (no body). |
| `Trajectory` | same | **No.** One mutable object, versioned in place (`LifecycleVersion`, `ReducerSeq`) | Yes | Kept. |
| `WorkItem`, `TargetWorkItems`, `Agent` | same | No, mutable in place | Yes | Kept. |
| `Update`, `Controls` | same | Update packets are mutable (delivery marks them); no earlier state kept | Yes | Kept. |
| `Events` | replay, applied-turn, activation failure | Rebuildable by `ResultEventRefs` (events are immutable objects of the same batch) | No, if checked | **Kept.** Events carry no body (ids, refs, a reason); the saving is small and the activation failure reader would need a second read path. Left for a later step if the counters show it matters. |
| `TextureTurn` | replay, applied-turn | Nowhere else stored; it is the command's own decision | Yes | Kept. |
| `Schema` | replay | a constant | no | Kept (tiny). |

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

1. A new Texture command body still contains the revision body. (Test:
   `TestLifecycleCommandNamesItsRevisionInsteadOfEmbeddingIt`,
   `TestTextureTurnCommandNamesItsRevisionAndStillRecoversByCallerRun`.)
2. Replay of an old command after a later revision returns the later
   revision or document. (`TestLifecycleReplayAfterLaterRevisionReturnsTheOriginalResult`;
   already true before the change, kept as the guard that a by-head rebuild
   would break.)
3. Replay of a tape that mixes old full and new slim commands differs from a
   whole-write reference, or replay rewrites a revision or command object.
   (`TestLifecycleReplayOfOldAndNewCommandsIsByteIdentical`.)
4. A restore from a snapshot taken before the change, followed by new slim
   commands, fails to replay either form.
   (`TestLifecycleRestoreFromSnapshotPlusTailAcrossTheChange`.)
5. `GetAppliedTextureTurnByCallerRun` returns no revision for a slim turn.
