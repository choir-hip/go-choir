# Stale open self-development operations block all engineering on a computer (2026-11-02)

Mutation class of this doc: green. Evidence read-only (CLI self-dev list/head, node-b console.log, code).

## Problem
Any self-development operation in an OpenStates state (internal/selfdev/operations.go:49) holds every
engineering admission on its computer: `openOperationHold` (internal/agentcore/carried_patch.go:418) feeds
`operation_open` (internal/agentcore/engineering_admission.go:197-205), retried every 60 s with no cap. The cause is
visible only in the console. No owner act can close an operation that will not settle.

## Evidence
- d@e (computer-59c902c5...): `rollback-4dca8039...` degraded since 2026-10-27 (updated 2026-10-30), target = applied
  `selfdev-4585da37...` (release 63a13ae9, gen 1). Head: desired fcddd72b (the rollback event) != effective ce1737e7,
  pending empty. Console (node-b vm-70070cf7.../console.log): 106 lines of
  "rollback-4dca... in degraded needs retry: rollback release unavailable: updater: pinned release unavailable", one
  per 60 s retry wake (not several per second).
- b@c: a 6-day-old awaiting_approval operation blocked engineering until the orchestrator rejected it.
- Pinned release availability inside the guest (`<updater root>/releases/63a13ae9...`) is not readable from the host
  (guest data.img). Host guest-releases/ holds VM images, not the updater store. The updater itself never deletes
  releases (no RemoveAll of releases in internal/updater/updater.go), so "retention removed it" is a hypothesis.

## Why no existing act settles d@e
- A degraded rollback leaves degraded only via `rollbackSelfDevelopmentOperation`
  (internal/agentcore/self_development_materializer.go:789), whose first step is `ReadPinnedManifest`
  (internal/updater/updater.go:822-830). It fails, so the operation can never reach rolled_back/failed.
- `choir self-dev rollback --to-operation X` (cmd/choir/selfdev.go) creates a NEW rollback operation
  (selfdev.StartRollback, operations.go:393); it does not touch 4dca. 4dca stays open, so engineering stays blocked.
  A rollback to any applied op may even succeed (if that release is retained) and still leave the hold.
- The "manual re-bind" named in the cause string (self_development_materializer.go:271) does not exist as a command
  or route; grep finds only that string and a comment at :966.

## Cluster (3 symptoms, one substrate)
awaiting_approval for days (b@c), degraded with unavailable release (d@e), rebase blocked behind a lock
(docs/problems/selfdev-blocked-behind-unperformed-rebase-2026-10-27.md): an open operation with no terminal path
that the owner or management can invoke, plus an unbounded hold whose cause is not on the ledger.
The state machine already allows the closing edges: awaiting_approval->failed, accepted->failed, degraded->failed
(operations.go:675-698), but only the materializer writes them, and only from journaled outcomes.

## Recommendation
1. Smallest fix for d@e: in the materializer, a degraded/rollback_pending rollback whose
   `ReadPinnedManifest` fails closes `degraded -> failed` (existing edge) with TerminalError "pinned release unavailable;
   computer stays on its current release", at self_development_materializer.go:789. Risk: head desired(fcddd) !=
   effective(ce17) stays; later ops bind to the live head so this is tolerated, but verify on d@e.
2. Common substrate fix: an owner act `self-dev abandon --operation --reason` (route beside rollbacks at
   api_self_development.go:204; CLI beside cmd/choir/commands.go:646, scope owner:acts) that moves any open,
   non-materializing operation to `failed` with the reason recorded on the tape (as recordMaterializationFailed does).
3. Surface the hold: after N retries in deferEngineeringAdmission (engineering_admission.go ~:213) write the
   blocker to the management/Texture commitment ledger, so it is not console-only.
Do 1 now; 2 and 3 are the class fix. Retention never deleting a pinned release is not shown to be the cause.

## Journal-first settlement (round 3 design, 2026-11-02)
Review rounds 1-2 found three wedges in the settle paths themselves: a failed-settle discharged pending before its
guarded row move (a lost race left a live row with no pending); "no journal" was read as "nothing in flight" though
the updater writes its journal only after staging, and the client gives up after 2 min; and the rollback re-drive
read the pinned manifest before the journal, so a terminal `applied` journal behind an unusable pinned release could
never be recorded or abandoned.

Invariant:
1. The updater journal is the authority for whether anything was installed. Every settle path reads it first.
2. In flight is provable: `Apply` holds a per-key advisory lock (`operations/<key>.lock`, flock) from before it
   reads the journal to its return. A settle takes that lock without waiting; held means in flight (refuse, 409).
3. "Nothing installed" is made durable before a settle acts on it: under the lock, a missing journal, or a dead one
   that never swapped `current` (phase `prepared`), is replaced by a terminal `refused` fence. A later `Apply` for
   the key (a queued or timed-out request) returns the fence and never stages or swaps.
4. Every settle runs under `selfdevMaterializeMu` (the materializer holds it per pass; abandon takes it for every
   state), moves the row first with its expected-state CAS, and only then discharges pending, and only a pending
   that is this operation's decision. A lost CAS discharges nothing.
5. No settle path leaves pending bound to a non-open operation: a discharge that fails after the row move, or a
   decision that lands after it, is discharged by the post-move re-read, the abandon replay, or the reconciler's
   orphan sweep (pending whose decision belongs to a failed operation, after the same fence).
6. A terminal `applied` journal settles forward, never failed: the applied event is recorded from the journal's
   signed receipts, and the row moves to `applied`/`rolled_back`. Only the tail (checkpoint, route) may be skipped,
   and then the row says so.

Table (open state x journal x pending -> terminal path). "Fence" = rule 3; discharge = materialization_failed.
| state | journal | pending | terminal path |
|---|---|---|---|
| requested..verified | (no decision, no key) | not theirs | abandon: row -> failed; crash boot closes them |
| awaiting_approval, no decision on tape | none | empty | abandon: row -> failed; a decide that loses the row CAS leaves pending -> post-move re-read / orphan sweep: fence, discharge |
| awaiting_approval, decision on tape | none | the decision | materializer recovers it to accepted (abandon 409: "let it settle") |
| accepted | none | own decision | abandon: fence, row -> failed, discharge |
| materializing / rollback_pending | lock held | own | 409; the updater finishes; the materializer records the terminal outcome |
| " | none, or dead `prepared` | own | fence; materializer: failed + discharge (pinned unusable, refusal); abandon: same |
| " | dead past the swap (`pointer_swapped`, `restart_requested`, `recovering`) | own | not crash-interrupted: the materializer's next `Apply` resumes the journal. Crash-interrupted: held; the owner's abandon re-admits it for one resume (an owner act, not a boot-time resume) |
| " | terminal refused/failed | own | failed + discharge (the journal's result on the event) |
| " | terminal applied, event unrecorded | own | materializer: applied event + tail; if the installed release is unusable, settle forward without the tail. Abandon: settle forward without the tail |
| " | terminal applied, event recorded (tail stuck) | empty | same: tail retried; abandon settles forward |
| degraded (applied, tail unfinished) | terminal applied | empty | rollback: journal-first tail (d@e); if the installed release is unusable, settle forward; abandon settles forward |
| degraded | absent / non-applied | empty | abandon: row -> failed (nothing to discharge) |
| failed (any) | any | its decision (orphan) | orphan sweep: fence, discharge; never when the journal is applied or in flight |

Out of scope (stated, not wedges this change creates): a corrupt (undecodable) journal file retries; a rollback whose
journal is dead past the swap and whose pinned release is also gone needs the release to resume (only reachable by
tampering with the updater store); a settle-forward row has no checkpoint or route receipt, so a later rollback *to*
it is refused at start (a clean 409). The updater daemon ships in the base image: the fence and lock are complete once
the base image carries this build; until then the fence still stops any `Apply` that reads the journal after it.
