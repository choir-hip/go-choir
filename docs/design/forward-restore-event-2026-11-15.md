# The forward restore event: a restore that survives boot and append (2026-11-15)

Class of this document: green (design and failure list, no code). The changes
it orders are red (tape format, reducer, the platform event authority, boot
recovery, restore). Two review seats before any slice lands (CLAUDE.md,
"Review"). Author: a design builder; not a review seat. Revision 2, after
review round 1 (section I lists every finding and its disposition).

Owner decision, 2026-11-15: the forward restore event is to be built; tape
format changes are welcome. This document is the design and the failure list
the code and its tests are written from. Nothing here is built.

## 0. Sources

- `docs/problems/restore-act-three-review-rounds-structural-assessment-2026-11-15.md`
  and its addendum, and
  `docs/problems/restore-bases-design-three-review-rounds-structural-assessment-2026-11-15.md`.
- The forward-restore design of 2026-10-30 is
  `docs/design/computer-history-restore-audit-2026-10-30.md` (section 1.2 G1
  and section 2, "Restore is forward"). It chose three events and an epoch
  root whose effect is `reconstruct(T) + carried records`. This design keeps
  its idea (forward, nothing deleted) and changes the mechanism: one event, no
  carried records, replay skips instead of reconstructing. The problem record
  is `docs/problems/restore-is-neither-forward-nor-durable-2026-10-30.md`.
- `git show origin/lane/restore-boot-install:docs/design/restore-installs-at-boot-2026-11-15.md`
  (unreviewed, parked): its section 0 is the finding of section A below; its
  install step is not needed for correctness here (B.9).
- Commit 304a81277 (canonical checkpoint witness v4): `reconstructedWitness`
  (`internal/agentcore/canonical_witness.go:90`).
- Anchors are against origin/main 5743fd19f. There is no diff under
  `internal/` between 718ea5ed4 (the first revision's anchors) and 5743fd19f,
  so every `file:line` below was re-checked and is unchanged. A `~` means the
  line was read but the number is approximate.

Everything marked **(invented)** is a proposal with no precedent in the code.
Every reader of the tape in the repo was read for this revision (section
B.6.2); nothing in this document depends on an unread file except where it
says "to be measured".

Letters: H is the tape head before the restore, N the restore target (an
event sequence, digest `digest(N)`), R = H+1 the restore event, C the
checkpoint, W its content witness, A the restore authorization (B.5).

## A. Today: what a restore does, and exactly where it is undone

### A.1 The act, step by step

1. The CLI sends `choir computer restore` with the checkpoint artifact
   (`cmd/choir/commands.go:462`, route `/lifecycle/restore`). The proxy
   authenticates the owner (`authorizeOwnerAct`, `internal/proxy/owner_acts.go:25`,
   called for the restore path at `internal/proxy/computer_lifecycle.go:~391`:
   the signed-in session or an API key with the `owner:acts` scope) and
   forwards to the guest. **It mints no owner-act receipt for restore**
   (receipts exist for mode writes, package publish and listing, verifier
   repin, platform yank: `internal/proxy/self_development.go:~313`,
   `packages.go:249`, `verifier_repin.go:160`); the platform sees an `owner`
   actor and an authority string and nothing signed. The guest's handler is
   `restoreComputer` (`internal/agentcore/rematerialize.go:323`, routed from
   `api_self_development.go:113`).
2. `RestoreFromRequest` validates the checkpoint and its witness version
   (`rematerialize.go:~344`); a version this release does not judge is a 409
   before anything is written.
3. `appendRestoreIntent` (`rematerialize.go:360`, body `:397-442`) appends a
   `restore_requested` event at the tape head: sequence H+1, actor `owner`,
   authority `platform-control:restore`, idempotency key
   `restore-intent-<checkpoint digest>` (`:399`). A second restore to the same
   checkpoint finds that key in the local index and appends nothing
   (`:401-404`).
4. `RematerializeFromTape` (`rematerialize.go:64`) plans the target
   (`planRestoreTarget`, `restore_snapshot.go:108`: one pass over the tape),
   installs the newest snapshot at or below it into a staged store and replays
   the tape through the target (`ReconstructThroughTarget`,
   `rematerialize.go:163`, `appender.go:1829`), then checks the tail count
   (`:170`) and the witness against the checkpoint's (`:180`).
5. It restages the pinned release as `current` (`:217`), closes the live
   store, renames marker, workspace, state file, WAL and SHM into a
   quarantine directory and the staged ones into place (ten renames, `:255`),
   reopens (`:261`) and rebinds the appender's projection (`:264`).
6. The answer is 200 with `witness_matched: true`. If the release execs a
   different binary a planned-restart marker is written and the computer
   restarts when idle (`requestRestoreRestart`,
   `self_development_restore_restart.go:138`).

### A.2 Where and why it is undone

After step 5 the guest's store head is the checkpoint's head (N) and the
platform's canonical head is H+1.

- **Append, in the running process.** `appendLockedInner` compares the host
  head with the embedded head (`internal/computerevent/appender.go:1052`),
  finds them different and runs in-place recovery (`recoverPreparedInPlaceLocked`,
  which calls `catchUpLocked` at `:1301`; `catchup.go`). Catch-up fetches the
  host's events after the local head and applies each one that extends the
  local chain (`catchup.go:130`). The host's chain is one linear chain, so
  event N+1 extends the restored store's head `digest(N)`: catch-up re-applies
  N+1..H+1 onto the restored store, at the next append of any kind
  (`HeadsAligned`, `appender.go:308-321`, already reports
  `ErrNeedsProjectionRepair` in between).
- **Boot.** `materializeProjectionBaseIfNeeded` (`internal/autoputer/run.go:435`)
  calls `PlanRecovery(empty=false, localSeq=N, chainExists, 0, targetSeq=H+1)`
  (`projection_base.go:143`): "a store that holds events resumes from its own
  head". `runReplayPhase` then calls `appender.Reconstruct` (`run.go:859`),
  which replays (N, H+1] (`appender.go:1411-1650`). The store ends at the
  pre-restore state plus the intent event. Only `current`, in the updater root,
  stays restored.
- **The 720,000 refusal.** `PlanRecovery` refuses when `TailEvents > 720000`
  (`internal/recoveryplan/recovery_plan.go:22`, refusal at `:131`), where the
  tail is `target - start` in sequence numbers (`tailLen`). A checkpoint more
  than 720,000 events behind H, restored, makes the next boot refuse
  (`BootRefusalKindTailExcess`, `projection_base.go:~60-70`): the computer
  stays down. Nothing at acceptance checks this distance.

### A.3 What the platform authority believes afterwards

- Its head row (`computer_event_heads`, `internal/platform/computer_events.go`)
  is H+1, canonical head `digest(restore_requested)`. That kind is causal
  (`reducer.go:~194-206`, `isCausalEvent`): it moves no desired, effective or
  pending field. The authority still believes the pre-restore effective
  release commitment while `current` names the restored release.
- It has no record that any guest store was ever at N.
- It publishes a checkpoint only when the accepted head is the current head
  (`internal/platform/checkpoints.go:59`): a store restored to N cannot bind a
  checkpoint at N, and one at H+1 would carry a witness the store lacks.

## B. The mechanism

### B.1 One sentence

A restore appends one event, `computer_restored`, at the tape head. Its
meaning is: "the computer's state from here is state(N); events N+1..H are an
abandoned branch that replay skips; later events apply on top." The tape is
never truncated, rewritten or reordered.

### B.2 The one fact: which events are on the line

The **line** of the head is the events of the tape that are not abandoned.
An event is abandoned when a restore event R' with target N' < its sequence
< R' was admitted while it was on the line. One column records it **(invented)**:
`computer_event_append_receipts.abandoned_at_sequence`, NULL for an event on
the line, R' for an abandoned one. It is set once, in R's admission
transaction (C.1), on the rows in (N, R) that are still NULL. It is an index
column over the receipts table: no event body, digest or receipt JSON is
rewritten (and the table already takes bulk updates of its stored receipt
column, `receipt_compaction.go:511`). It is derived data: a pure function of
the restore events on the tape, rebuilt on import (reader table, row 13).

Two numbers follow from it, and everything else uses only these:

- `live(s)`: row s exists and its column is NULL.
- `ordinal(s)` = `s - COUNT(abandoned rows with sequence <= s)`, the number of
  line events at or below s. A tape with no restore has `ordinal(s) = s`.

Nested restores need nothing more (B.10). There is no segment list, no
lineage structure and no `Line` type: a restore event's effect on the line is
exactly its marking, and replay needs only "serve live events after `after`".

### B.3 The event's fields (invented, except where a field exists)

`event_kind: "computer_restored"`. Two new envelope fields, both `omitempty`
so the canonical bytes of every existing event are unchanged
(`CanonicalJSON` marshals the struct, `canonical.go:19`):

| Field | Value |
| --- | --- |
| `sequence`, `previous_head` | R = H+1 and `digest(H)`: the physical chain stays linear and gapless |
| `restore_target_head` (new) | `digest(N)` |
| `restore_target_sequence` (new) | N, an integer, N < R |
| `proposed_effect_ref` | the checkpoint digest C |
| `decision_ref` | the restore authorization digest A (B.5) |
| `resulting_effective_state_commitment` | the effective commitment at N |
| `actor_profile`, `authority_ref` | `owner`, `platform-control:restore` (as the intent event today, `rematerialize.go:413-415`) |
| `idempotency_key` | `restore-<A>`; A is single use, so a second restore to the same checkpoint is a new event (repairs A.1 step 3) |
| `expected_*`, `RequireExpectedHead` | the head's values (H's); `RequireExpectedHead` set (`event.go:133`, `appender.go:951`): never rebased, a lost CAS is a refusal |
| payload | one pinned **restore manifest**, class `owner`, with four fields: the checkpoint digest, the target sequence and head, W (digest and version), the release digest (the boot needs it to restage), A |

`Event.Validate` requires the two new fields for this kind and forbids them
for every other kind. `restore_requested` stays valid and decodable forever;
the act stops writing it.

### B.4 The R rule: what each party verifies

The head after R is `head(N)` with R's sequence and digest: `desired_event_head`,
`effective_event_head`, `pending_transition_ref`, `desired_state_commitment`,
`effective_state_commitment` are their values at N (the platform reads them
from its stored head receipt for N, which carries all five; `EventsPage`
already reads them back, `event_replay.go:~100-110`); `reducer_version` and
`credential_revocation_epoch` are unchanged (the epoch is safe because B.5.2
refuses a restore across a revocation).

Three code paths see R. One rule, `restoredHead(target Head, event, digest)`
**(invented)**, computes the head in all three.

**1. Platform CAS** (`compareAndSwapTx`, `computer_events.go`). `Reduce(current
= head(H), event, input)` runs its generic checks first and unchanged
(`reducer.go:~74-79`): sequence is H+1, `previous_head` is `digest(H)`, the
`expected_*` fields equal head(H)'s. `input.RestoreTarget *Head` (a nil
pointer, `omitempty`, so every old pin-intent commitment is unchanged,
`request.go:12`) carries head(N); the CAS compares it to its own stored
receipt for N inside the admission (as `admitEffectAccepted` verifies
authority in the same transaction, `computer_events.go:~225`). The kind then
has an explicit `Reduce` case: pending at H must be empty (as
`EventEffectAccepted`, `reducer.go:~83`) and the next head is
`restoredHead(head(N), ...)`. `isCausalEvent` (`reducer.go:~194`) is a
default that would treat the kind as head-neutral; a test asserts every entry
of `validEventKinds` has an explicit decision.

**2. Replay and boot** (`applyDurableEvent`, `appender.go:1663`, called by
`reconstruct`, `appender.go:1426`). The local head is N. For this kind the
function does **not** call `Reduce` (whose sequence and `previous_head` checks
are H's, not N's): it verifies

1. `local.canonical_event_head == event.restore_target_head` and
   `local.sequence == event.restore_target_sequence`, else `ErrRestoreBase`
   (B.8);
2. `event.sequence > local.sequence`;
3. `restoredHead(local, event, digest)` equals the receipt's `Next` (`sameHead`);
4. the signed head receipt (`VerifyEventHeadReceipt`, unchanged).

**3. Catch-up** (`catchup.go:130`). The check there is
`event.Sequence != headSeq(current)+1 || event.PreviousHead != headCanonical(current)`.
For this kind it is replaced by the same four checks as in 2 (one shared
function). A store not at N therefore wedges as "divergent chain" and
repairs by the boot rebuild (B.9); a store at N applies R and ends at head R.
Every other kind keeps the existing check.

What a replaying guest **cannot** verify for R: `previous_head` (`digest(H)`),
`sequence == H+1` and the `expected_*` values, because they describe the
abandoned tape head, which the guest never held. What vouches for them:
(a) the platform's admission, against head(H), inside the serializable
transaction that stores R; (b) the signed head receipt, whose fields include
`previous_head` and `sequence` (`eventHeadReceiptFields`,
`computer_events.go:~385`) and which the guest verifies in step 2.4; (c) the
physical hash chain on the platform, which every export re-verifies
(`bundle/export.go:189-192`, unchanged because R's `previous_head` is
`digest(H)`); (d) the witness check of the store at N against W at the R
boundary at boot (B.9.3), which vouches for the state itself. It also does
verify that the event after R chains to R: for R+1, `previous_head ==
digest(R)` is checked by the ordinary rule.

The store's finalize check (`internal/store/computer_events.go:297`,
`currentSequence+1 != sequence || currentHead != previousHead`) is relaxed for
this kind only to: `currentHead == restore_target_head && currentSequence ==
restore_target_sequence && sequence > currentSequence`. The local event index
then holds the line's events and R; R's index row names a `previous_head` that
is not in the index; failure 28 lists the readers to check.

### B.5 Who may append it

An owner act, and only that (CLAUDE.md: a grant that spends the owner's money
is an owner act; a restore abandons the owner's work).

- **The proxy mints, corpusd exchanges (invented, grounded in A.1.1).** After
  `authorizeOwnerAct` the proxy mints an owner-act receipt, new kind
  `ActComputerRestore = "computer.restore"` (`internal/owneract/owneract.go:70-91`),
  whose subject binds computer, checkpoint digest and target head, with the
  existing `ownerActSigner.Mint` (`proxy/self_development.go:317` is the
  pattern). A receipt lives `TTL = 5 minutes` (`owneract.go:48`) and the dry
  run measured 283 to 363 seconds on d@e (lane document), so the receipt
  cannot be carried to the append. corpusd therefore verifies it
  (`Verifier.Verify`, `owneract.go:255`; `recordOwnerAct` consumes it once) and
  mints a **single-use authorization A** with a one-hour expiry in a new
  `restore_authorizations` table modeled on `platform_update_offers`
  (`owner_acts.go`, DDL near line 100), stamped with the tape head at mint and
  the target. The proxy forwards A to the guest with the request. (The review
  suggested the receipt id could serve as A and the table be cut; the 5-minute
  TTL against a measured dry run contradicts it.)
- **Admission** (`admitRestore`, beside `admitEffectAccepted`,
  `owner_acts.go:317`) refuses, **always, in observe mode too**
  (`owner_acts.go:319-322`: no restore traffic exists to observe), an R whose
  `decision_ref` is not an unexpired, unused A for this computer, whose
  `proposed_effect_ref` or target fields differ from A's, or whose
  `previous_head` is not the head A was minted at or a descendant with no
  restore, apply or revocation between. Further refusals, typed, with the
  numbers:
  1. a pending transition at head(H) or at head(N);
  2. any `key_revoked` event with sequence above N (a restore must not
     re-enable a revoked credential; the epoch is deliberately not restored);
  3. N not a row on the line (`live(N)` false, so a target inside an
     abandoned range is refused: B.10), or N >= R;
  4. C not published for this computer at head(N), or judged `not_derivable`
     (`ErrCheckpointNotDerivable`);
  5. (guest side, before it asks) a live capsule, run or apply
     (`restoreRestartBusy`, `self_development_restore_restart.go:121-130`).
- The act stops appending `restore_requested`. A refused or failed restore
  leaves nothing on the tape; the owner-act receipt table audits attempts.
- **The platform stamps what was abandoned**: in the admission transaction it
  counts the rows it is about to mark by event kind and stores the counts on
  A's row. The act's answer and `choir` output read them from there. H may
  move between the guest's dry run and the CAS, so a count computed by the
  guest would be stale; the manifest carries none.

### B.6 Replay, reconstruct and the recovery plan

#### B.6.1 The rules

- **Serving.** `EventsPage` (`internal/platform/event_replay.go:14`) returns
  the events with `sequence > after` and `abandoned_at_sequence IS NULL`, in
  order, up to the page size, for **any** `after`, whether or not `after` is
  itself on the line. A page spans the skipped range: it is full unless the
  line ends. There is no `ErrNotOnLine`. The gapless check
  (`event_replay.go:64`) becomes "no gap between consecutive served events
  except across rows marked abandoned". The query is a range over a partial
  index on live rows `(computer_id, sequence) WHERE abandoned_at_sequence IS
  NULL`; the point-lookup technique of `event_replay.go:24` was a Dolt planner
  workaround and the platform is PostgreSQL (`receipt_walk.go:89`); the plan is
  to be measured in slice 3.
- **The guest replays what it is served.** `reconstruct` ends a replay on
  `len(page) < pageSize` (`appender.go:1599`): that stays correct because a
  short page now means the line ended, never a segment boundary. The skipped
  events are never fetched, prefetched, applied or indexed.
- **A retained store must be on the line.** `PlanRecovery` stays pure but its
  callers (`projection_base.go:143` and `:211`, the restore plan, the snapshot
  cadence) pass ordinals for local, watermark and target, and know whether the
  local head is live, from one small read **(invented)**
  `GET /internal/computers/events/line?seq=...` returning `{seq, live,
  ordinal}` per sequence. `MaxRecoveryTailEvents` thus keeps its meaning,
  "events a boot may apply" (C.3). A local store whose head is not live is
  not a resume candidate; it takes the rebuild (B.9). (A store whose head is
  not live and that did resume would be served R first and refuse it:
  `ErrRestoreBase`. That is the safety net, not the plan.)
- **Snapshots as bases.** A snapshot is a valid base iff its recording event
  is served (so on the line) and its **store head is live** and at or below
  the target. Both, because the producer records a snapshot after capturing at
  head h: one captured at an abandoned head and recorded after R is a live
  event describing abandoned state. `snapshot.Scan` and `PlanCandidates`
  (`snapshot.go:1317-1347`) filter on `live(store head)`.
  *A base whose store head is at or below N but whose recording event sits in
  the abandoned range is **not** a usable base* (review should-fix 4 asked
  whether it should be): `Scan` reads recordings from the served page, and
  making abandoned recordings visible would be a second serving rule. The cost
  is a longer replay, bounded to the window of one capture around N (the
  producer captures at h and records shortly after). Retention (slice 6) counts
  only visible bases, so an unusable base is not kept as if it were one.
- **Catch-up** (`catchup.go`) applies served events that extend the local
  chain and, for R, applies B.4.3.

#### B.6.2 Every reader of the tape

Read for this revision. Categories: **S** gets the line from the served page
unchanged; **C** needs the change named; **P** reads the physical tape on
purpose and is correct to; **N** is not a tape reader.

| # | Reader | Cat | Disposition |
| --- | --- | --- | --- |
| 1 | `appender.reconstruct` (`appender.go:1554`) | S | Applies served events; short page means end of line. R takes B.4.2. |
| 2 | `store.finalizedEventBody` (`store/event_bodies.go:116`): `EventsPage(after=sequence-1, 1)` | S | **Review B1.** Under the any-`after` serving rule, for R it asks after H and is served R; for any line event it is served itself; for an abandoned sequence it is served another event and the existing digest and sequence check (`:121`) refuses it, correctly. Unchanged. Slice 3's test: read R's body with a cold cache. (The first revision's `ErrNotOnLine` would have broken it; that rule is deleted.) |
| 3 | `snapshot.scan`, `Scan`, `ScanAll` (`snapshot.go:1354`, via `projection_base.go:165`, `restore_snapshot.go:116`) | S+C | Pages are lines. Candidate filter adds `live(store head)` (B.6.1). |
| 4 | `platformRestoreTape.EventsPage` (`restore_snapshot.go:39`) and `planRestoreTarget` (`:108`) | S+C | Finds the target in the served line: a target not served (abandoned or absent) is refused, which is B.10's refusal for free. Counts ordinals. |
| 5 | `replay_completeness` (`replay_completeness.go:391-393`, `:401`) | S+C | Replays through the appender: line. `tailReceipt` (`restore_snapshot.go:313`) compares applied count to `target - base`: ordinals. |
| 6 | `http_client.go` `EventsPage` / `Events` | S | The client of row 7. |
| 7 | `platform.EventsPage` / `HandleComputerEventReplay` (`event_replay.go:14`, `event_handlers.go:329`) | C | The serving rule of B.6.1; slice 3. |
| 8 | `catchup.go:130` | C | B.4.3. |
| 9 | `applyDurableEvent` (`appender.go:1663`) | C | B.4.2. |
| 10 | `store.finalizeBatch` (`store/computer_events.go:297`) | C | B.4 relaxation. |
| 11 | `PlanRecovery` callers (`projection_base.go:143, :211`), `PlanCandidates` (`snapshot.go:1333`), snapshot cadence (`agentcore/snapshot_producer.go:76`, `lastSeq` and head arithmetic at `:479-493`) | C | Pass ordinals and `live`. |
| 12 | `ledgerwitness.Walk` (`ledgerwitness/walk.go:102`, `curr = event.PreviousHead`; sole caller `cmd/ledger-witness/main.go:31`) | C | **Review B2.** It walks the physical chain from the head and would report a record in an abandoned batch as `Found`. Change: at an event that carries `restore_target_head`, `curr = event.RestoreTargetHead`, else `PreviousHead`. One line plus its test (a tape with R and a record in the abandoned range reports it `Missing`; one before N reports it `Found` with its position). Slice 3. |
| 13 | `bundle` import (`bundle/import.go:473` inserts the receipt rows; `ReceiptRow`, `bundle.go:125`, has no marking column) | C | The column is derived, so import recomputes it after inserting rows: for each restore row in order, mark the rows in (N, R) still NULL. Export and verify carry rows unchanged. Test: export, import, serve page equal. (Surprise of this pass.) |
| 14 | `bundle` named-root reads (`bundle/store.go:105`, `bundle/follow.go:74`) | C | Add `abandoned_at_sequence IS NULL` on the naming receipt and a live root head; slice 6. |
| 15 | `platform/snapshot_roots.go:154` (`snapshotRootNamedSeqSQL`, used at `:227`) and `pruneSnapshotRoots` | C | **Review should-fix 4.** `MAX(a.sequence)` over `snapshot:<root>` names counts an abandoned naming event. Add the same clause; retention keeps the newest visible base on the line (C.3). Slice 6. |
| 16 | `platform/checkpoints.go:226, :254` (verifier evidence event looked up by digest) | C | Add the clause: a checkpoint is not evidenced by an abandoned event. `:90, :119` look up the genesis event, always on the line: P. |
| 17 | `platform/clean_rebase.go:181` (prior decision "on this computer's tape") | C | Add the clause: a decision abandoned by a restore is not a prior decision to rebase. |
| 18 | `bundle/export.go:189-192`, `verify.go`, `scan.go`, `readReceipts` (`store.go:38`) | P | Physical chain verification and copy: R chains to H, `projectHead` (`follow.go:~55`) reads R's receipt fields, which carry N's heads. Correct unchanged. |
| 19 | `tapefollow` (`store.go`, `syncer.go`) and `cmd/desktop/sync*.go` | P | Copies and verifies bundles; no `Reduce` or replay anywhere in them (grep). A future follower that reconstructs state must apply B.6.1; named in G. |
| 20 | `platform/tape_sweep.go` (`:705-852`), `tape_purge.go` (`:82, :328`), `receipt_compaction.go` (`:462-522`), `receipt_walk.go` | P | Retention, sweep, purge and compaction work on the physical chain on purpose; abandoned events are history under the same windows. |
| 21 | `platform/verifier_key_pin.go:174` (`MAX(sequence)`) | P | Key windows are in physical sequence, the platform's clock. |
| 22 | `platform/receipt_indexes.go` (`receiptKeyRevokedCountSQL`, kind and sequence lookups) | P | Physical. The revoked-count equals the line's because B.5.2 forbids any revocation in an abandoned range (test, failure 11). |
| 23 | `platform/owner_acts.go:395` (offer base head equals `previous_head`) | P | Physical; an offer minted on an abandoned head cannot match a post-R `previous_head`. |
| 24 | `platform/checkpoint_list.go:98` (sequence by digest) | P | Physical lookup of a digest; the act refuses abandoned targets (B.5.3). |
| 25 | `event_handlers.go:279` (payload by digest) | P | Content-addressed. |
| 26 | `agentcore/api_trajectory.go`, `routekinds/routekinds.go` | N | `streamLifecycleEvents` reads the local store; routekinds' `EventSource` is the browser SSE API. **Neither reads the tape.** |
| 27 | `appdesk/required.go:156`, `agentcore/desk_install.go:191` | N | Desk logs, not the tape. |
| 28 | `internal/vmctl`, `internal/proxy` | N | No tape reads (grep for the receipts table, `EventsPage`, `PreviousHead`). Route projection joins checkpoint certificates, not the tape. |
| 29 | `autoputer/file_sync.go:750` | N | Appends `file_root_committed` (a causal event with no projection effect); `HydrateIfNeeded` (`:214`) reads the file CAS, not the tape. |
| 30 | `taperepack` (`engine.go`) | N | Repacks legacy tape files by digest and pin receipt; no sequence or chain walk. |

Count of the 30 rows: 3 are S (1, 2, 6); 3 are S with a change (3, 4, 5);
11 are C (7 to 17); 8 are P (18 to 25); 5 are N (26 to 30). Rows 26 to 30 were
"not read" in the first revision. Surprises: rows 26 and 27 are not tape
readers at all (the names suggested otherwise); row 2 looked broken and is
fine under the serving rule; row 13 (bundle import loses the derived column)
and the 5-minute owner-act TTL (B.5) were not visible before reading.

### B.7 The act, and the running process

1. Refuse beside live work (B.5.5); take the act lock and the append fence.
2. Validate the checkpoint; check the target's release is pinned and not
   pruned and is at or above the floor R will raise (B.12). Refuse before
   anything is written.
3. **Dry run**: build the staged store at N and check the canonical witness
   (A.1 step 4, up to `rematerialize.go:~190`), unchanged. It is the proof
   that the restore is derivable; once R is on the tape a restore that cannot
   reproduce W would be irreversible.
4. Raise the store floor (B.12).
5. Append R in a **tape-only mode** **(invented)**: the pins and the CAS, and
   neither `projection.Prepare` nor finalize. The running store holds state(H)
   and R says state(N); skipping Prepare also means the old store never holds a
   prepared row for R, so no `RecoverPrepared` can try to finalize it there
   (review should-fix 3). Then set the fence: every later append returns
   `ErrRestorePending`.
6. Restart the guest now, without a planned-restart marker. The boot after R
   is a crash-class boot by design (CLAUDE.md "Restarts": a crash restart
   never resumes work): open obligations and the runs live at H are closed
   "interrupted by a restart" as events after R, and nothing resumes.

The running process never swaps a store, reopens or rebinds. The act does not
touch `current`.

### B.8 The typed refusal

`ErrRestoreBase`: R reached with the local head not equal to its target. At
boot it means the plan was wrong and the store is rebuilt; in catch-up it
wedges as "divergent chain"; in the staged dry run it is a defect.

### B.9 The boot after R

The decision is derived from the tape alone (`live(local head)`), so every
boot reaches the same verdict. **Order**: the decision runs in
`materializeProjectionBaseIfNeeded` (`run.go:435`), before the replay phase
(`run.go:859`) and so before `RecoverPrepared` (called first by `reconstruct`,
`appender.go:~1440`) ever sees the old store. A prepared row left in the old
store by an in-flight append of the abandoned line is moved away with it.

1. Local head live: resume as today, on ordinals.
2. Local head not live: the store is **not a base**. If the newest restore
   event's release differs from `current`, **restage `current` first**
   (`RestagePinnedRelease`, `updater.go:987`) and, if it execs another binary,
   exit for a normal restart into it (nothing resumes). Then move the state
   file, WAL, SHM and marker to the quarantine name (the shape of the compact
   rebase's `.precompact`, `store/compact.go:595-614`, removed at
   `telemetry_retire.go:281-295`) and take the existing empty-store-with-chain
   path: install the newest valid base (B.6.1) and replay the line
   (`projection_base.go:152-215`; it clears and rebuilds the state file and
   telemetry partitions through `snapshot.Install`). It is the disk-lost
   recovery, already crash-resumable.
   **If the quarantine name already exists** (an earlier rebuild stalled before
   `replay_done` and a second R landed; review should-fix 2): it is deleted,
   then the current store is renamed into it. Reason: the quarantine is a
   convenience copy, never an authority; the tape is, and everything in a
   store except the two non-tape tables is derivable from it. Those two
   tables are empty by the act's refusal (below), so nothing is lost. The
   boot therefore depends on the tape and on "the state file exists", not on
   whether a quarantine is lying around.
3. At R's boundary in the replay (B.4.2) the boot **checks the canonical
   witness of the store at N against the manifest's W** whenever this boot
   applied events. The boot may run the restored release's binary, older than
   the one that did the dry run; this catches replay skew (failure 24). A
   mismatch refuses the boot loudly, keeps the quarantine and names the table.
4. After `replay_done` (`run.go:911`) the quarantine is removed where the
   compact rebase removes its rollback copy.

Rows in the two tables the tape does not carry (`co_super_slots`,
`agent_evidence`; `compact.go:~700-716`, `replay_eligibility.go:69,90`) are
dropped silently by any replay. **Decided**: the act refuses when either
table is non-empty, with the counts. A refusal is a correctness refusal, not
a switch; the d@e drill counts them first.

### B.10 How nested restores compose

With one marking column the compositions are all consequences of B.2:

- **Restore to a point before an earlier restore event** (N' below R_prev and
  on the line): R2 marks the rows in (N', R2) still NULL, which includes
  R_prev and everything after N'. Served pages then run 1..N', R2.
- **Restore to a restore event** R_prev: it is on the line; allowed;
  state(R_prev) is state(N).
- **Restore to a point inside an abandoned range**: refused (B.5.3). This is
  the one case of "refused or defined" decided as refused. It means a restore
  cannot be undone by restoring to the pre-restore head; the owner restores
  to a checkpoint on the line. Undo by re-instating an abandoned branch is
  later work in one line: serve `line_of=<seq>` pages so the dry run can read
  a line that is not the head's (G).
- **Depth**: no cost; each restore is one UPDATE and one row.
- **Refusals that remain**: credential (B.5.2), a pending transition, N >= R,
  an abandoned target, a release below the floor (B.12), "no valid base
  within the replay bound" (C.3).

### B.11 What happens to the surfaces

| Surface | Behaviour across R |
| --- | --- |
| Sequence numbers | Never rewound. R = H+1; later events H+2 onward. A line has gaps in sequence; ordinals count line events. |
| Hash chain, `canonical_event_head` | One linear physical chain. After R the canonical head is `digest(R)` and the guest's head equals it, so `HeadsAligned` and `sameHead` need no change. Only the guest's local index has a gap and finalize is relaxed for R only (B.4). |
| Head fields | N's (B.4); the effective release commitment moves back with the release pointer, ending the divergence of A.3. |
| Checkpoints | One bound at a head on the line stays restorable. One at an abandoned head is not (B.10). A checkpoint after R binds at head >= R. `checkpoints.go:59` is unchanged (C.2). |
| Snapshots | B.6.1. Slice 6 has the platform keep the newest visible base on the line below R until a snapshot at or after R exists, and the guest queue a snapshot at R once its replay is done. |
| Receipts | One head receipt per event, unchanged; R's carries N's heads. |
| Run-memory retire ops and object retire ops | Ordinary ops in projection batches (`projection_batch.go:40-44`). Skipped with an abandoned batch; replayed in place on the line; after R they act on the rows the line holds. They name time slices and never touch tape or blobs (`object_retire.go:1-30`). The trim boundary is a pure function of head sequence (`event_index_trim.go`): R's finalize trims at `R - window`, deterministically, live and replay alike (failure 19). |
| Idempotency keys | Unique per computer today (primary key of the receipts table). A key that an abandoned event used and the new line re-derives would be answered with the abandoned event's receipt (`existingComputerEventReceipt`): a lost write. Uniqueness becomes "among events on the line": the primary key is replaced by a unique index on `(computer_id, idempotency_key) WHERE abandoned_at_sequence IS NULL` **(invented)**, and the same lookup carries the same clause. |
| Store schema floor | Epoch 7 **(invented)**: "the tape may hold a restore event; the line is not 1..head." |

### B.12 Old binaries, the floor, and the release

- An old binary refuses the kind at `Validate` ("unknown event kind",
  `event.go:~180`) after `DecodeHistoricEvent` (`decode.go:123`), which does
  not disallow unknown fields: the kind check is the fence, not the new
  fields (an old decoder would drop them and compute another digest).
- The guest writes the store floor at epoch 7 (`storeschema.WriteFloor`,
  `floor.go:70`, written "before it first writes data an older binary would
  mishandle") before the first R request. The updater refuses Apply and
  `restorePrior` to a release below the floor or declaring no epoch;
  `RestagePinnedRelease` (`updater.go:987`) checks the same floor.
- **Consequence:** a restore addresses a release (A.1 step 5; computer-ontology
  "Restore-set boundary"). A release that does not know the event cannot read
  a tape that holds one, so **a checkpoint whose release predates slice 2
  cannot be restored**; the act refuses it before writing anything. Every
  checkpoint that exists the day slice 2 deploys is in that class; the first
  restorable one is bound after the deploy (the drill binds one).

### B.13 What is not undone

The restore set is event head, release pointer, VM-local SQLite state and the
computer-surface frontend (`docs/computer-ontology.md`, "Restore-set
boundary"). Not undone, and the act's answer says so with the counts by kind
from A's row (B.5):

- Objects published to the shared Autopaper object graph; others may have
  adopted them.
- Mail sent and any other effect on an outside party. The Mail desk's state at
  N may show an item unsent that was sent: double-send risk. The kinds to
  count (mail send, publish, provider call) are taken from the code at slice 7.
- Provider calls and their cost, including Jev. The spend stays spent.
- Releases the updater applied in the abandoned range (pruned later).
- **The files tree on the data volume.** Read: `file_root_committed` is a
  causal event carrying a manifest of the tree synced to the file CAS
  (`file_sync.go:750`); it moves no store table, and the tree lives outside
  SQLite. A restore leaves the tree as it is at the restart (the first
  revision's refusal on a differing root is dropped: it would refuse every
  restore after any file edit). The next sync commits the tree as a new root
  on the line. `HydrateIfNeeded` runs only on a fresh volume.
- Anything another computer read from this one.
- The abandoned events themselves: they stay on the tape and are retained
  under the same windows.

### B.14 Crash cases

The tape is the record. "A crash never resumes work" applies to obligations,
not to deciding which state a computer holds; the rebuild is the boot's
ordinary recovery from the tape, as for a lost disk. A planned restart may
resume once with a marker; the restore boot deliberately has none.

| Crash point | Result |
| --- | --- |
| Before R is durable on the host (dry run, floor write) | Nothing happened on the tape. The staged store is swept (`sweepStagingArtifacts`, `projection_base.go:~255`); `current` is untouched. The floor stays raised, harmlessly. The owner retries. |
| R durable, response lost | The idempotent resend returns R's receipt (`resendUnclearCASLocked`, `appender.go:1257`); the act proceeds to the restart. |
| R durable, process dies before the restart | The next boot sees `live(local head)` false and rebuilds (B.9). Success is decided by the tape. |
| During B.9 (after restage, between renames, mid snapshot install, mid replay) | The same decision from the same facts; restage and rename are idempotent, install and replay resume as for a lost disk. An existing quarantine is replaced (B.9.2). |
| After the rebuild, before the quarantine is removed | The next boot finds a store on the line and a quarantine; it removes it after `replay_done`. |
| The restart after R never comes | The fence keeps refusing appends; `/health` reports restore pending; the host's existing stall detector restarts the guest. |
| The restored release will not boot | The runtime unit's boot-loop guard (`nix/autoputer-vm.nix:~188`) and the updater's rollback run as for any release; recovery is another restore to a checkpoint on the line. |

## C. What changes in the platform authority

### C.1 Accepting the restore event

`Reduce` gets the explicit case (B.4.1). `compareAndSwapTx` gets `admitRestore`
(B.5), the lineage checks (refusals 1 to 4), the consumption of A, the counts
stamp and the marking `UPDATE ... SET abandoned_at_sequence=R WHERE
computer_id=? AND sequence>N AND sequence<R AND abandoned_at_sequence IS
NULL`, all in the one serializable transaction that already updates the head
and inserts the receipt, so R is atomic with them. The marking touches H - N
rows; that cost under a SERIALIZABLE transaction is to be measured on a copy
before slice 4 lands (failure 60). A CAS lost to a concurrent append is a
refusal (`RequireExpectedHead`).

### C.2 Does `checkpoints.go:59` change?

No. The event makes ancestor checkpoints unnecessary for restoring: a restore
needs C (published when its head was the head) and the tape. The check at `:59`
governs publishing, and publishing at the head is still right. That the
column answers "is this head on the chain" is what the restore-bases
assessment (item 3) asked for; letting the authority accept a checkpoint at an
ancestor on the line is later work (G).

### C.3 The 720,000-event refusal

- The meaning is kept ("replayed events a boot may apply") and the arithmetic
  is fixed: `tailLen` runs on ordinals (B.6.1). After R the boot's tail is the
  line's events after the base, `(N - base) + 1` plus later events, not
  `R - base`; the distance between N and H no longer matters to the next boot.
- The refusal that remains moves to **acceptance**: the target's line has no
  valid base within `MaxRecoveryTailEvents` ordinals below N and N's ordinal is
  above the bound (`planRestoreTarget` already refuses, `restore_snapshot.go:~140`;
  it counts ordinals). The act refuses with the numbers.
- Retention must keep that base: the platform's pruning of snapshot roots
  (`pruneSnapshotRoots`, `snapshot_roots.go`, ordered by the naming sequence
  of B.6.2 row 15) keeps the newest visible base below R until a snapshot at
  or after R exists (slice 6).

## D. Failure list (tests are written from it before code)

Each has prevent and detect. "T:" names the test that must exist first.

### D.1 Tape, reducer, chain

| # | Failure | Prevent | Detect |
| --- | --- | --- | --- |
| 1 | An old guest or platform binary meets the event. | Kind absent from the old `validEventKinds` (`event.go:~91`); floor 7 before the first R; deploy order (E). | T: a binary at the previous commit refuses a tape with R by name. |
| 2 | An old decoder drops the new fields and recomputes another digest. | The kind check precedes (`decode.go:123` does not disallow unknown fields). | T: decode R without the fields; the digest differs and `Validate` refuses first. |
| 3 | The new envelope fields change the canonical bytes of old events. | `omitempty`. | T: golden digests over real historic events. |
| 4 | `TransitionInput.RestoreTarget` changes old pin-intent commitments (`request.go:12`). | Nil pointer, `omitempty`. | Golden `ComputePinIntentCommitment` and `ComputeRequestCommitment`. |
| 5 | The kind falls into the causal default (`reducer.go:~194`). | Explicit case. | T: exhaustiveness over `validEventKinds`. |
| 6 | Replay applies R to a store not at N. | B.4.2 checks head digest and sequence; `ErrRestoreBase`. | T: R with local head at N-1, N+1 and an abandoned head. |
| 7 | A retained store at an abandoned head resumes. | The plan reads `live(local)` (B.6.1); served pages hand it R first, which refuses. | T: boot with a store at H after R rebuilds. |
| 8 | The store's finalize rejects R (`computer_events.go:297`). | Relaxed for this kind only. | T: finalize R at N; at another head refuses. |
| 9 | The relaxation lets any event skip a range. | Only R, only with the target equal to the current head and the event's own target fields. | T: a forged non-restore event with a gap refuses. |
| 10 | A pending transition at H or N leaves a dangling pending ref. | Admission refuses both; the reducer refuses at H. | T. |
| 11 | A restore re-enables a revoked credential, or the epoch count diverges. | Refused when any `key_revoked` has sequence > N. | T: restore across a revocation refuses; and after any admitted restore, the physical `key_revoked` count equals the count over served events (no abandoned range holds one), including a restore before an earlier restore. |
| 12 | A second restore to the same checkpoint appends nothing (`rematerialize.go:399-404`). | Key `restore-<A>`, A single use. | T: two restores, two events. |
| 13 | A key re-derived after the restore is answered with an abandoned event's receipt. | The partial unique index and the lookup clause (B.11). | T: append K, restore past it, append K again: a new receipt on the line. |
| 14 | A concurrent append between the guest's read and the CAS is silently abandoned. | `RequireExpectedHead`. | T: a lost CAS refuses, not rebases. |
| 15 | Two restores at once, or a restore beside an apply. | The act lock; the reducer refuses a pending transition. | T. |
| 16 | The receipt compaction assumes a chain it cannot expand. | The physical chain is unchanged by R. | T: compact, expand, verify a tape with R. |
| 17 | A forged R from a compromised guest. | `admitRestore`: single-use A minted by corpusd from a proxy-signed owner-act receipt; always enforcing; target claims checked against the platform's receipts. | T: R without A, with a used A, an expired A, for another computer, checkpoint or target: each refuses. |
| 18 | A is used after a deploy, an apply or a revocation. | A names the tape head at mint (B.5). | T. |
| 18a | The owner-act receipt expires (5-minute TTL, `owneract.go:48`) before the append. | The receipt is exchanged for A at the proxy, before the dry run. | T: a dry run longer than 5 minutes still admits. |

### D.2 Replay, plan, snapshots, ordinals

| # | Failure | Prevent | Detect |
| --- | --- | --- | --- |
| 19 | The trim boundary jumps at R's finalize. | A pure function of head sequence in finalize; live and replay agree; do not special-case. | T: replay equals direct by canonical witness across R with the window crossed. |
| 20 | Sequence difference used as an event count: `tailLen`, the replay observer's `tailReceipt`, `PlanCandidates`, snapshot cadence, progress, health. | All pass ordinals (B.6.2 rows 5, 11). | T each, with a restore whose gap exceeds the bound. |
| 21 | A snapshot from the abandoned branch is chosen as a base. | Recording served and store head live (B.6.1). | T: recorded after R with an abandoned head: never; head <= N, recorded on the line: yes; head <= N, recorded in the abandoned range: not (named cost). |
| 22 | A snapshot captured at an abandoned head is recorded after R by the old process. | The append fence; the live-head test. | T. |
| 23 | The newest valid base is pruned; the next boot has none in bound. | Slice 6 retention counts visible bases (B.6.2 row 15). | T: prune after a restore; the act refuses at acceptance when none is left. |
| 24 | The replay differs between the dry run and the boot (an older release may run it). | Witness at R's boundary (B.9.3). | T: skew test refuses. |
| 25 | A page straddles the skip and ends early (a short page read as end of replay, `appender.go:1599`) or applies abandoned events. | Pages span the skip; short means end of line (B.6.1). | T: page sizes 1, 2 and larger, with a page ending exactly at N, across the gap, through `reconstruct`, catch-up, `Scan` and `finalizedEventBody`. |
| 26 | Page serving cost on the range query. | Partial index on live rows. | Plan measured in slice 3. |
| 27 | Replay progress and `/health` show events behind as sequence difference. | Ordinals. | T. |
| 28 | A reader assumes the local event index is contiguous or each row's `previous_head` is present. | Audit in slice 2: the witness's event-index digest, `NewestEventOfKind`, trim, `EventByIdempotency`, the snapshot manifest. | T: witness equality live versus replay across R (304a81277's canonical witness is the oracle). |
| 29 | The store's witness at R is compared to W for N and differs by design. | W is checked at N (B.9.3); after R the oracle is replay-completeness. | T. |
| 30 | `finalizedEventBody` cannot read R after the rebuild (review B1). | Any-`after` serving (B.6.1); no `ErrNotOnLine`. | T: cold-cache read of R and of an event after R. |
| 31 | `ledgerwitness.Walk` follows `previous_head` into the abandoned range (review B2). | Jump via `restore_target_head` (B.6.2 row 12). | T: record in the abandoned range is `Missing`; one before N is `Found`. |
| 32 | The reducer or catch-up rejects R because its sequence and `previous_head` are H's (review B3). | B.4.2 and B.4.3 verify the four named checks; the generic check stays for every other kind. | T: replay and catch-up to R from a store at N and below; a store above N wedges. |
| 33 | Bundle import loses the marking and a followed tape serves abandoned events. | Import recomputes the column (row 13). | T: export, import, served pages equal. |
| 34 | The snapshot-roots naming SQL counts an abandoned naming event. | The clause (rows 14 and 15). | T: a root named only by an abandoned event is unnamed for retention. |
| 35 | A checkpoint is evidenced by an abandoned event, or a clean rebase cites an abandoned decision. | Rows 16, 17. | T. |

### D.3 The act, the process, the boot

| # | Failure | Prevent | Detect |
| --- | --- | --- | --- |
| 36 | The running process serves or appends after R with state(H). | Tape-only append, then the fence; catch-up's divergent-chain refusal. | T. |
| 37 | The act restages `current` before R and R is never written. | Restage happens at the boot that sees R. | T: kill before R; `current` unchanged. |
| 38 | A restore resumes the abandoned line's open work. | No planned-restart marker. | T: an open run at H is closed "interrupted" after the restore boot, as an event after R. |
| 39 | A staged compact copy is swapped over the rebuilt store. | No marker, so `planned=false` (the collision the lane found, restore-boot-install 2.7). | T. |
| 40 | The restored release is pruned between the act and the boot (`updater/prune.go:20-61`). | Checked at the act and at the restage; the boot then refuses loudly with the old store in quarantine and the owner restores to another checkpoint. | T. |
| 41 | The release is below the floor or predates the event. | Refused at the act. | T. |
| 42 | The restored release execs another binary and the rebuild runs under the wrong one. | Restage, restart, then rebuild. | T: across two boots with kills between. |
| 43 | A crash between the quarantine rename and the install leaves a store-less volume. | The install path is the existing lost-disk path. | T: kill at each rename and in the install. |
| 44 | The old store is destroyed before the rebuilt one is proven. | The quarantine is removed only after `replay_done`. | T. |
| 45 | A second R lands while the first rebuild's quarantine exists (review should-fix 2). | B.9.2: the quarantine is replaced, never consulted. | T: R1, kill before `replay_done`, R2, boot twice. |
| 46 | The tape-only append leaves a prepared row for R (review should-fix 3). | It calls neither `Prepare` nor finalize; the rebuild precedes `RecoverPrepared` (B.9). | T: kill after R; boot; no prepared row and no finalize attempt. |
| 47 | Disk: dry run plus live plus rebuild plus quarantine. | The act's floor counts the staged copy, the workspace and any quarantine; the staging is removed before R. | T: ENOSPC at each step. |
| 48 | The dry run outlives the request. | Out of scope: slice 7 keeps the synchronous route (d@e-class stores); the asynchronous act of `lane/restore-act` is the follow-on and reusable because its install half is deleted here. | A larger store times out loudly. |
| 49 | The boot after R exceeds the boot deadline. | Ordinals; refusal at acceptance (C.3). | T. |
| 50 | The answer is read as "done" while the rebuild is pending. | The 200 says `restore_committed: true, state: pending_rebuild`; the CLI polls through 502/503 until head is R and W matched. | T. |
| 51 | Rows in the non-tape tables are dropped. | The act refuses with counts when non-empty (B.9). | T. |
| 52 | Stale realizations (a cold recover, `vmctl/cold_recover.go`; a fork) hold abandoned heads. | A cold recover is a disk-lost boot; a stale writer is refused by the CAS; a fork has its own tape. | T: cold recover after R. |
| 53 | The serving route stays on the pre-restore release. | vmctl and the proxy read no tape (B.6.2 row 28); the drill checks the served release equals C's. | Drill. |
| 54 | Telemetry partitions and derived-key marks of the old store leak into the rebuilt one. | `snapshot.Install` empties and reinstalls partitions; marks follow the head. | T: witness across the rebuild. |
| 55 | The marking UPDATE (H - N rows) stalls the SERIALIZABLE admission. | Measured on a copy before slice 4; if too slow, mark in chunks inside the transaction ordered by sequence. | Staging measurement; the append-latency metric. |

### D.4 Outside the store (stated, not prevented)

| # | Failure | Prevent | Detect |
| --- | --- | --- | --- |
| 56 | Published objects remain in the shared graph. | Not undone (B.13); the answer lists counts. | The answer. |
| 57 | Mail sent and unsent-looking at N: double send. | The counts are named; custodian-desk reconciliation on R is open question 2. | A's counts. |
| 58 | Spend is not refunded. | Stated. | Ledger. |
| 59 | The files tree stays as at the restart. | Stated (B.13). | The answer. |
| 60 | Updater applies in the abandoned range leave releases and journals. | Prune handles them. | Updater root listing. |
| 61 | An update offer minted on an abandoned head is used later. | Its base head must equal `previous_head` (`owner_acts.go:395`). | T. |
| 62 | History shows the abandoned branch as live. | The served page is the line; `api_trajectory` and routekinds do not read the tape (B.6.2 rows 26-27). | T on the history view with R. |
| 63 | Retention treats abandoned events as live or deletable. | Sweep, purge and compaction are physical on purpose (row 20). | `tape_sweep` tests with R. |

### D.5 Versioning, deploy, rollback

| # | Failure | Prevent | Detect |
| --- | --- | --- | --- |
| 64 | A host (platform) revert past the readers after R exists. | Rollback story (G): only the writer is revertible; readers are permanent; the deploy's `-previous` binary after slice 7 contains the readers. Residual named. | Page decode refuses at R for that computer (loud). |
| 65 | A guest rollback below the floor. | The updater's floor check. | T. |
| 66 | The partial unique index and column migration on live PostgreSQL. | Online index build before any writer exists (slice 4); the column is nullable with no default rewrite. | Migration test and deploy log. |
| 67 | Docs state the old semantics. | `docs/computer-ontology.md` (restore-set), `computer-history-restore-audit-2026-10-30.md` G1 and the restore problem record updated with slice 7. | Review. |

## E. Old binaries and the deploy order

- **Platform, old:** `Reduce` calls `event.Validate()` first
  (`reducer.go:~56`), which returns "unknown event kind"; the CAS refuses the
  append; a page read decodes with `DecodeHistoricEvent` and fails at `Validate`
  through `Digest()`. It never serves past R wrongly.
- **Guest, old:** `applyDurableEvent` calls `Reduce` (`appender.go:1668`),
  which refuses; boot fails with `boot.fatalf("autoputer: reconstruct computer
  event authority: ...")` (`run.go:~890`). It also cannot open the store: the
  floor is at epoch 7.
- No path continues past an unknown kind.

Deploy order, each a landing with its own proof:

1. Readers with no writer: slices 1 to 3, the platform CAS refusing every
   `computer_restored` append (slice 2).
2. Admission, marking and the idempotency index (slice 4).
3. The boot rebuild (slice 5) and retention (slice 6).
4. The writer (slice 7), last, on d@e only first.

## F. Proof

### F.1 Repo tests, written before the code of each slice

All through the real replay path with the canonical witness of 304a81277 as
the oracle.

1. Marking and ordinals: a model-based property test of the UPDATE and of
   `live`/`ordinal` against a brute-force definition of "state(p) is the replay
   of the line" on random tapes with restores (before an earlier restore, to a
   restore event; the refused abandoned target).
2. **Replay of a tape with R equals direct state.** A store built by applying
   the line's events directly (events 1..N, then the events after R) against a
   store built by replaying the whole physical tape through the served page.
   Equal by canonical witness and event-index digest. Variants: R at the head;
   two restores; a retire op, a run-memory retire op and a partition retire on
   each side of R; the trim window crossed; a snapshot recording, a
   `key_rotated` and a lifecycle event in the abandoned range; page sizes 1, 2,
   large, one ending exactly at N.
3. Reducer and catch-up: the head after R; B.4.1-3, including a catch-up from
   a store at N and wedge from one above N; every refusal in B.5; exhaustiveness;
   golden digests and commitments.
4. Plan: ordinals, `live(local)`, snapshot filtering, retention, the acceptance
   refusal with numbers, `finalizedEventBody` with a cold cache, `ledgerwitness`.
5. Old binary refuses the tape at R by name.
6. Crash cases through the real boot with a SIGKILLed child at each step of
   B.14, plus R1, kill, R2 (failure 45) and the prepared-row case (46); each
   ends in the same store as the uninterrupted run, with no resumed obligation.
7. Platform: `admitRestore` refusals, the proxy-to-A exchange with a dry run
   longer than the receipt TTL, the marking and idempotency re-keying, a lost
   CAS, atomicity, bundle import recompute.
8. Act: tape-only append and fence, no restage before R, a release below the
   floor, refusal beside live work, non-tape tables, the answer's counts.

### F.2 The deployed drill: d@e only

Never the largest computer, never the owner's. Staging (`https://choir.news`).
Preconditions: the d@e rows in `co_super_slots` and `agent_evidence` counted
(51); the deployed commit verified.

1. Bind and publish a checkpoint C at head N with a release that knows the
   event. Record W, the release digest and the served route's release.
2. Write new work W1 so the head is H > N.
3. `choir computer restore` to C. Expect 200, `witness_matched: true`,
   `restore_committed: true`; read R at H+1 with the fields of B.3 and the
   counts on A's row.
4. After the restart and rebuild: head is R; the boot logged the witness at
   R's boundary equal to W; `current` and the served route are C's release; W1
   is absent; the abandoned events are on the tape and absent from the served
   page.
5. Write W2. Restart the guest (crash, then planned). The state is
   `line(N) + R + W2` by canonical witness, equal to a fresh replay-completeness
   run from a base on the line.
6. A second restore to C (a new event), and to a point before the first R.
   Refused: a target inside the first abandoned range.
7. Adversarial kills (d@e only): after R before the restart; during the
   restage; during the install; during the replay; after `replay_done` before
   the quarantine removal. Each boots to the same witness, nothing resumed.
8. Refusals: a checkpoint of a release below the floor; a target beyond a
   revocation; a restore beside a live run; a replayed A.

Claims to precommit in `docs/calibration/claude-ledger.md` before the writer
lands, each an outcome the author cannot write: a restore survives a restart on
d@e (the witness after step 5); a restore leaves the abandoned events off the
served page; the dry run's witness equals the boot's at R.

## G. What this does not do; rollback; open questions

Does not do:

- No undo by deletion, truncation or rewrite of the tape; no restore of the
  shared graph, mail, spend, other computers or the files tree.
- No fallback to the in-process flip, no setting, no allowlist, no selector.
- No restore to a release that does not know the event.
- No restore into an abandoned range, and so no undo of a restore.
- No asynchronous accepted-act surface (failure 48).

Later work, one line each (the reader table is complete and none of it
depends on them, so they leave this design):

- **Install the verified staged store as the base** (formerly slice 8): removes
  the second replay; the lane document's install half reduced to "a staged
  store at N with the manifest's W is a base like a snapshot".
- **The checkpoint authority accepts an ancestor on the line** (formerly slice
  9): `live(N)` plus the stored receipt supply the proof; unblocks restore-bases.
- **Re-instate an abandoned branch**: `line_of=<seq>` pages for the dry run.
- **A future follower that reconstructs state** must apply B.6.1 (today
  `tapefollow` only copies).

**Rollback of the change** (review should-fix 5; no new mechanism). Slices 1 to
6 revert by order while no R exists on any tape. After the first R, the
readers are permanent, like every tape kind before it: a tape kind is an
append-only commitment. What is revertible, and real, is the **writer**:
reverting slice 7 is a normal forward deploy that removes the route's write and
leaves slices 1 to 6, so the host that runs it still reads R. The deploy's
automatic rollback goes to the `<service>-previous` binary
(`.github/scripts/node-b-ci-deploy:407-419`) and the CLAUDE.md landing loop
makes every deploy a forward commit on `origin/main`; slices 2 to 6 deploy
before slice 7, so the previous binary of any deploy after slice 7 contains the
readers. The one path to a host that cannot decode R is a human reverting
slices 2 to 6 after the first R. Named residual: that makes the page read for
that one computer refuse at R and its boot fail loudly (failure 1); the old
platform's CAS reads only the head row and would still accept appends chained
on R, so no data is lost, and the repair is to roll forward. A host-release
floor was considered and **not** added: it would be a new mechanism guarding a
human error that already fails loud and loses nothing, and the review's
alternative is a slice of its own that can be added if a seat judges the
residual unacceptable. Every non-docs landing carries `Ring-Expedite:` naming
the defect (a restore that does not survive boot or append); each expedite is
named in the next report.

Open questions (the rest were decided and left the list; section I):

1. Non-tape tables: the act refuses when non-empty (decided default); the owner
   may prefer to restore and report the loss.
2. How custodian desks (Mail, Autopaper) react to R; until decided, only the
   counts in the answer.
3. The witness at R's boundary on a large store: its cost is to be measured on
   d@e; required for every boot that applied events until then.
4. Credential refusal granularity: any `key_revoked` above N (decided default)
   versus only revocations of credentials the store at N holds.

## H. Slices, in order, each reviewable alone

Each: class, the proof that lands with it, the rollback.

1. **Marking and ordinals** (yellow; revert): the nullable column, the marking
   function and the `live`/`ordinal` queries, with the property test. Nothing
   writes the column yet; no behavior change.
2. **Event kind, fields, reducer, replay and catch-up, store finalize, floor
   declaration, no writer** (red; two seats; revert by order, since no tape
   holds the kind): includes the platform CAS refusing every `computer_restored`
   append, golden digests, the old-binary refusal, replay-equals-direct on
   synthetic tapes (F.1 2, 3, 5) and the index-readers audit (failure 28).
3. **Readers that need the line** (red; two seats; revert): `EventsPage` serving
   and the partial index, the `line` read, ordinals in `PlanRecovery` callers,
   snapshot filter and cadence, `planRestoreTarget`, replay observers, the
   `ledgerwitness` jump, the `finalizedEventBody` test. With no restore on any
   tape behavior is identical. Proof: whole uncached suites of every touched
   package and importers; replay-completeness on d@e unchanged; the plan
   measured.
4. **Platform admission** (red; two seats; revert): `ActComputerRestore` and the
   proxy mint, corpusd exchange and `restore_authorizations`, `admitRestore`
   always enforcing, the refusals, the counts stamp, the marking UPDATE, the
   idempotency unique index, bundle import recompute, the checkpoint-evidence
   and clean-rebase clauses. No guest writer (A mintable in a test only). Proof:
   F.1 7; the online migration and the marking cost measured on staging.
5. **The boot rebuild** (red; two seats; revert): the plan's "not live" branch,
   restage-first, the quarantine (replace if it exists) and its removal at
   `replay_done`, the witness at R's boundary. Exercised by a test tape and a
   d@e store forced onto an abandoned head, with no writer. Proof: F.1 4 and 6.
6. **Snapshot retention on the line** (orange; one seat; revert): the
   snapshot-roots SQL and `pruneSnapshotRoots`, `bundle` named-root reads, and
   the guest queueing a snapshot at R. Proof: the prune test; the d@e snapshot
   after a restore.
7. **The restore act writes the event** (red; two seats; revert, the one
   revertible step after the first R): tape-only append, fence, restart without
   a marker, floor write, removal of `appendRestoreIntent`, the new answer and
   CLI poll, the refusals of B.5 and B.12 and the non-tape tables. The writer
   and the switch-on; no setting. Proof: F.1 8; the drill F.2 on d@e only.

## I. Review round 1

Reviewer: the Sonnet seat (REPAIR: three blockers, seven should-fixes). The
DeepSeek seat timed out and reads round 2. Each finding was checked against
origin/main 5743fd19f.

| Finding | Verified | Disposition |
| --- | --- | --- |
| B1 `finalizedEventBody` reads R with `EventsPage(after=sequence-1, 1)` (`store/event_bodies.go:116`) and `ErrNotOnLine` refuses `after=H`; "every consumer gets lines unedited" is false | Yes, the code is as described | Repaired by subtraction: `ErrNotOnLine` is deleted; serving is "live events with `sequence > after`" for any `after`, so the reader is unchanged and correct (B.6.1, row 2, failure 30). The false claim is replaced by the complete reader table (B.6.2), 30 rows, none unread. Carried by slice 3. |
| B2 `ledgerwitness.Walk` follows `previous_head` into the abandoned range (`walk.go:102`) | Yes | Required change in slice 3: jump via `restore_target_head` (row 12, failure 31). |
| B3 The reducer and `catchup.go:130` reject R (sequence and `previous_head` are H's) | Yes (`reducer.go:~74-79`, `catchup.go:130`) | B.4 writes R explicitly for the reducer, replay and catch-up, and states what a replaying guest verifies and cannot, and what vouches (failure 32). |
| 1 `reconstruct` ends on a short page (`appender.go:1599`) | Yes | Adopted: pages span the skip; short means end of line; test sizes 1, 2, larger and exact-at-N (failure 25). |
| 2 Quarantine already exists | Yes, a real loop | Decided: replace it; it is never an authority (B.9.2, failure 45). |
| 3 Tape-only append may leave a prepared row; boot order | Yes in principle | Adopted: no `Prepare`, no finalize; the rebuild precedes `RecoverPrepared` (B.7.5, B.9, failure 46). |
| 4 Snapshot-root SQL and `bundle/follow.go` read the physical tape; a base at or below N recorded in the abandoned range is hidden | Yes (`snapshot_roots.go:154`, `bundle/store.go:105`, `follow.go:74`) | Adopted into slice 6 (rows 14-15, failure 34). The hidden-base case is **rejected as a defect and recorded as a named cost** (B.6.1): the platform serves only the line, and the extra rule to expose it is a mechanism the finding does not need. |
| 5 Platform rollback after R | Partly: an old CAS accepts appends chained on R; only page reads that cross R fail | A real rollback story with no new mechanism (G): only the writer is revertible; readers are permanent; the deploy's `-previous` binary contains them. Residual named. A host floor not added; reason given. |
| 6 Abandoned counts computed by the guest | Yes | The platform stamps them at admission (B.5); the manifest carries none. |
| 7 Credential epoch | Yes, correct only because of B.5.2 | Test added (failure 11), including a restore before an earlier restore. |
| Cut: lineage endpoint and `AllEventsPage` | Partly | `AllEventsPage` and the lineage structure are cut (no physical consumer uses `EventsPage`: rows 18-25 read SQL or tape files directly). A small `line` read stays: the plan needs `live` and ordinals before it fetches, which `EventsPage` cannot give. |
| Cut: manifest fields | Yes | Cut to checkpoint digest, N and head, W, release digest (the boot restages from it) and A. |
| Cut: `restore_authorizations` (receipt id as A) | **Contradicted by the code**: an owner-act receipt lives 5 minutes (`owneract.go:48`) and the dry run measured 283 to 363 seconds | Kept; the proxy mints a receipt that corpusd exchanges for a one-hour single-use A (B.5, failure 18a). |
| Cut: slices 8 and 9, questions 3, 10, 11 | Yes | Slices 8 and 9 are later work, one line each (G). Questions 3, 10, 11 removed. |
| Cut: `Line` type, segments, re-instate | My own subtraction, not the review's | One marking column replaces them; a restore into an abandoned range is refused (B.10), which also removes the dry-run problem of serving a line that is not the head's. |

Recount: failure list 67 entries plus 18a (68). Open questions: 4, down from 11
(decided and removed: owner-act path now read and decided; event naming; no
watchdog; restore anchor not needed; host-release floor, answered by the
rollback story; end-of-replay witness; skipped-envelope fetch, no). Slices:
7, down from 9.
