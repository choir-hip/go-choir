# The forward restore event: a restore that survives boot and append (2026-11-15)

Class of this document: green (design and failure list, no code). The changes
it orders are red (tape format, reducer, the platform event authority, boot
recovery, restore). Two review seats before any slice lands (CLAUDE.md,
"Review"). Author: a design builder; not a review seat.

Owner decision, 2026-11-15: the forward restore event is to be built; tape
format changes are welcome. This document is the design and the failure list
the code and its tests are written from. Nothing here is built.

## 0. Sources, and what I did not read

Read in full or in the relevant part:

- `docs/problems/restore-act-three-review-rounds-structural-assessment-2026-11-15.md`
  and its addendum, and
  `docs/problems/restore-bases-design-three-review-rounds-structural-assessment-2026-11-15.md`.
- The forward-restore design of 2026-10-30 is
  `docs/design/computer-history-restore-audit-2026-10-30.md` (section 1.2 G1
  and section 2, "Restore is forward"). It chose three events
  (`restore_requested`, `restore_applied`, `restore_failed`) and an epoch
  root whose effect is `reconstruct(T) + carried records`. This design keeps
  its idea (forward, nothing deleted, undo is another restore) and changes the
  mechanism: one event, no carried records, and the replay skips instead of
  reconstructing. The problem record is
  `docs/problems/restore-is-neither-forward-nor-durable-2026-10-30.md`.
  `docs/design/branching-tape-2026-10-31.md` was read for context; it
  concerns split-brain branches with a primary ref, not an owner restore, and
  this design does not use it.
- `git show origin/lane/restore-boot-install:docs/design/restore-installs-at-boot-2026-11-15.md`
  (unreviewed, parked). Its section 0 is the same finding as section A below.
  Its boot-install step is not needed by this design for correctness (B.9);
  it is an optimization (slice 8).
- Commit 304a81277 (canonical checkpoint witness v4): `reconstructedWitness`
  (`internal/agentcore/canonical_witness.go:90`) and the replay of the tape
  reproducing the witness.
- The code paths cited inline. A `~` before a line number means the line was
  read but the number is approximate.

Not read, so marked where used: the proxy path from `choir computer restore`
to the guest and where the owner-act receipt is verified for it; the file
root (`file_root_committed`) restore path; `internal/ledgerwitness`,
`internal/tapefollow`, `internal/taperepack`, `internal/routekinds`,
`internal/store/event_bodies.go` and `internal/agentcore/api_trajectory.go`
beyond their use of the event source; vmctl's route-slot code; the updater
prune and restage bodies beyond the lines the lane document already cites.
Everything marked **(invented)** is a proposal with no precedent in the code.

Letters used throughout: H is the tape head before the restore, N the
restore target (an event sequence, with digest `digest(N)`), R = H+1 the
restore event, C the checkpoint, W its content witness, A the restore
authorization (B.5).

## A. Today: what a restore does, and exactly where it is undone

### A.1 The act, step by step (main at 718ea5ed4)

1. The CLI sends `choir computer restore` as an owner act with the checkpoint
   artifact (`cmd/choir/commands.go:462`, route `/lifecycle/restore`); the
   guest's handler is `restoreComputer`
   (`internal/agentcore/rematerialize.go:323`, routed from
   `api_self_development.go:113`).
2. `RestoreFromRequest` validates the checkpoint and its witness version
   (`rematerialize.go:~344`); a witness version this release does not judge is
   a 409 before anything is written.
3. `appendRestoreIntent` (`rematerialize.go:360`, body `:397-442`) appends a
   `restore_requested` event at the tape head: sequence H+1, actor `owner`,
   authority `platform-control:restore`, `ProposedEffectRef` = checkpoint
   digest, `DecisionRef` = the checkpoint's accepted head, idempotency key
   `restore-intent-<checkpoint digest>` (`:399`). A second restore to the same
   checkpoint finds that key in the local index and appends nothing
   (`:401-404`): the second restore leaves no event on the tape.
4. `RematerializeFromTape` (`rematerialize.go:64`) plans the target
   (`planRestoreTarget`, `restore_snapshot.go:108`: one pass over the whole
   tape to find the target's sequence and the snapshots below it), installs
   the newest snapshot at or below the target into a staged store, and replays
   the tape into it through the target (`ReconstructThroughTarget`,
   `rematerialize.go:163`, `appender.go:1829`). It checks the tail count
   (`:170`) and the witness against the checkpoint's (`:180`).
5. It restages the pinned release as `current` (`:217`,
   `updater.RestagePinnedRelease`), closes the live store, renames the live
   marker, workspace, state file, WAL and SHM into a quarantine directory and
   the staged ones into place (ten renames, `:255`), reopens (`:261`) and
   rebinds the appender's projection (`:264`).
6. The answer is 200 with `witness_matched: true`. If the release execs a
   different binary, a planned-restart marker is written and the computer
   restarts when idle (`requestRestoreRestart`,
   `self_development_restore_restart.go:138`).

### A.2 Where and why it is undone

After step 5 the guest's store head is the checkpoint's head (N) and the
platform's canonical head is H+1.

- **Append, in the running process.** `appendLockedInner` compares the host
  head with the embedded head (`internal/computerevent/appender.go:1052`),
  finds them different, and runs in-place recovery
  (`recoverPreparedInPlaceLocked`, which calls `catchUpLocked` at `:1301`;
  `internal/computerevent/catchup.go`). Catch-up fetches the host's events
  after the local head and applies each one that extends the local chain
  (`catchup.go:130`, `event.PreviousHead != headCanonical(current)`). The
  host's chain is one linear chain, so event N+1 does extend the restored
  store's head `digest(N)`. Catch-up therefore re-applies every event
  N+1..H+1 onto the restored store. The restore is undone at the next append
  of any kind (and `HeadsAligned`, `appender.go:308-321`, already reports
  `ErrNeedsProjectionRepair` in between). This is the owner's "the next
  append catches the store up to the tape head".
- **Boot.** `materializeProjectionBaseIfNeeded` (`internal/autoputer/run.go:435`)
  calls `PlanRecovery(empty=false, localSeq=N, chainExists, 0, targetSeq=H+1)`
  (`internal/autoputer/projection_base.go:143`): "a store that holds events
  resumes from its own head". Then `runReplayPhase` calls
  `appender.Reconstruct` (`run.go:859`), which replays (N, H+1] from the
  host (`appender.go:1411-1650`). The store ends at the pre-restore state plus
  the intent event. The only part of the restore that survives is the
  `current` release pointer, which lives in the updater root.
- **The 720,000 refusal.** `PlanRecovery` refuses when `TailEvents > 720000`
  (`internal/recoveryplan/recovery_plan.go:22`, refusal at `:131`). The tail
  is `target - start` in sequence numbers (`tailLen`). A checkpoint more than
  720,000 events behind H, restored, makes the next boot refuse
  (`BootRefusalKindTailExcess`, `projection_base.go:~60-70`): the computer
  stays down. Nothing at acceptance checks this distance.

### A.3 What the platform authority believes afterwards

- Its head row (`computer_event_heads`, `internal/platform/computer_events.go`)
  is H+1, canonical head `digest(restore_requested)`. `restore_requested` is
  a causal event (`reducer.go:~194-206`, `isCausalEvent`): it moves no
  desired, effective or pending field. So the authority still believes the
  pre-restore effective release commitment while `current` names the restored
  release.
- It has no record that any guest store was ever at N. Nothing on the tape
  distinguishes "restored" from "asked".
- It publishes a checkpoint only when the accepted head is the current head
  (`internal/platform/checkpoints.go:59`). A store restored to N cannot bind a
  fresh checkpoint at N because N is not the head, and a checkpoint at H+1
  would carry a witness the store does not have.
- The 2026-10-30 gap G1 is exactly this. The two d@e restores of 2026-11-15
  proved replay and the witness, not durability (assessment addendum).

## B. The mechanism: a restore is an event; replay follows lines

### B.1 One sentence

A restore appends one event, `computer_restored`, at the tape head. Its
meaning is: "the computer's state from here is state(N) (the checkpoint's
head); events N+1..H are an abandoned branch that replay skips; later events
apply on top." The tape is never truncated, rewritten or reordered.

### B.2 The line of a position (the one definition everything uses)

Every tape position p has a **line**: the ordered list of events whose
application yields state(p).

- If p is a `computer_restored` event with target X: `line(p) = line(X) ++ [p]`.
- Otherwise: `line(p) = line(p-1) ++ [p]`. `line(0)` is empty.

State is the replay of a line, in order. A restore event applies no
projection ops and moves no table; its only effects are on the head (B.4).
The abandoned events are exactly those not in `line(head)`.

As segments: walk back from p; find the largest restore event R_j at or below
the current position s; emit the segment [R_j .. s]; continue at
s := target(R_j); stop with the segment [1 .. s] when no restore event lies at
or below s. Cost: the number of restore events on the path, small. Define it
once, as a pure type, `computerevent.Line` **(invented)**: `Segments`,
`Live(seq)`, `Ordinal(seq)` (the count of line events at or below seq),
`Count(a, b)`, `NextAfter(seq)`. Everything that today does sequence
arithmetic on the tape (tail bounds, snapshot cadence, replay progress,
`planRestoreTarget`) uses `Ordinal` and `Count`; everything that serves or
selects events uses `Live`. There is no second code path for "a tape with a
restore" and "a tape without one": a tape with no restore event has the single
segment [1..head] and `Ordinal(seq) = seq`.

### B.3 The event's fields (invented, except where a field exists)

`event_kind: "computer_restored"`. Two new envelope fields, both `omitempty`
so the canonical bytes of every existing event are unchanged (`CanonicalJSON`
marshals the struct, `canonical.go:19`; golden test over real historic
events):

| Field | Value |
| --- | --- |
| `sequence` | R = H+1, never rewound |
| `previous_head` | `digest(H)`: the chain stays linear and gapless |
| `restore_target_head` (new) | `digest(N)` |
| `restore_target_sequence` (new) | N, an integer, N < R |
| `proposed_effect_ref` | the checkpoint digest C |
| `decision_ref` | the restore authorization digest A (B.5) |
| `resulting_effective_state_commitment` | the effective commitment at N |
| `actor_profile`, `authority_ref` | `owner`, `platform-control:restore` (the values the intent event uses today, `rematerialize.go:413-415`) |
| `idempotency_key` | `restore-<A>`; A is single use, so a second restore to the same checkpoint is a new event (repairs A.1 step 3) |
| `expected_*`, `RequireExpectedHead` | the head's values; `RequireExpectedHead` is set (`event.go:133`, `appender.go:951`): a restore is never rebased onto a newer head, a lost CAS is a refusal |
| payload | one pinned **restore manifest** JSON, class `owner`: checkpoint digest, target sequence and head, witness digest and version, release digest, frontend identity, operand scopes, authorization digest, `restored_from_head` (`digest(H)`), the snapshot the act's dry run used, counts of abandoned events by kind (B.13), counts of rows in the non-tape tables (B.9) |

`Event.Validate` requires the two new fields for this kind and forbids them
for every other kind. The old kind stays valid: `restore_requested` remains in
`validEventKinds` and decodable forever; the act stops writing it (B.5).

### B.4 What the event does to the head (the reducer)

The head after R is `head(N)` with R's sequence and digest:

- `sequence = R`, `canonical_event_head = digest(R)`.
- `desired_event_head`, `effective_event_head`, `pending_transition_ref`,
  `desired_state_commitment`, `effective_state_commitment` = their values at
  N. The platform reads them from its stored head receipt for N (the receipt
  carries all five; `EventsPage` already reads them back,
  `event_replay.go:~100-110`).
- `credential_revocation_epoch` unchanged. A restore across a credential
  revocation is refused (B.5), so the epoch counted over the whole physical
  tape (`receiptKeyRevokedCountSQL`, `event_replay.go:39`) equals the epoch of
  every line that is a restore target.
- `reducer_version` unchanged.

Two callers, one rule, `restoredHead(target Head, event, digest)` **(invented)**:

1. The platform CAS (`compareAndSwapTx`, `computer_events.go`) calls
   `Reduce(current = head(H), event, input)`. A new
   `TransitionInput.RestoreTarget *Head` carries head(N) (a nil pointer,
   `omitempty`, so every existing pin-intent commitment is unchanged;
   `request.go:12`). The CAS verifies that claim against its own stored
   receipt for N before reducing, as `admitEffectAccepted` verifies authority
   inside the same transaction (`computer_events.go:~225`). The usual checks
   hold against head(H): sequence, `previous_head`, the `expected_*` fields,
   and `current.PendingTransitionRef == ""` (a pending transition refuses, as
   `EventEffectAccepted` does, `reducer.go:~83`).
2. Replay (`applyDurableEvent`, `appender.go:1663`) has the local head at N
   (the replay skipped to it). For this kind it checks
   `local.canonical_event_head == event.restore_target_head` and
   `local.sequence == event.restore_target_sequence`, else a typed
   `ErrRestoreBase` (B.8), then computes the head with the same
   `restoredHead` and compares it to the receipt's `Next` (`sameHead`), as
   every other event does. `Reduce` must have an explicit case for the kind:
   `isCausalEvent` (`reducer.go:~194`) is a default that would silently treat
   it as a head-neutral causal event; a test asserts every entry of
   `validEventKinds` has an explicit decision.

The store's finalize check (`internal/store/computer_events.go:297`,
`currentSequence+1 != sequence || currentHead != previousHead`) is relaxed for
this kind only to: `currentHead == restore_target_head && currentSequence ==
restore_target_sequence && sequence > currentSequence`. The local event index
therefore holds events of the line then R; the digests of N+1..H are absent
from it, and R's index row carries a `previous_head` (`digest(H)`) that is not
in the index. That gap is deliberate; failure 28 lists the readers to check.

### B.5 Who may append it

An owner act, and only that. CLAUDE.md: a grant that spends the owner's money
is an owner act; a restore of the owner's computer abandons the owner's work.

- Today the restore route is `ownerAct(...)` on the CLI side
  (`commands.go:462`, scope `owner:acts`) but `appendRestoreIntent` carries no
  evidence of it into the platform: the platform sees an `owner` actor and an
  authority string. `ownerActMode` is `observe` in production (per the
  2026-10-30 audit, `nix/node-b.nix:162`), while `effect_accepted` enforces
  regardless of mode for every authority but the platform update
  (`owner_acts.go:317-346`). The restore event follows that precedent.
- **(invented)** A single-use `restore_authorizations` row, modeled on
  `platform_update_offers` (`owner_acts.go`, DDL near line 100): minted by
  corpusd when it verifies an owner-act receipt for a `computer_restore` act,
  naming computer, checkpoint digest, target head, target sequence, the tape
  head at mint, an expiry covering the dry run (one hour), and
  `admitted_event_digest` once used. The guest's restore request carries A
  from the start. The CAS admission (`admitRestore`, beside
  `admitEffectAccepted`) refuses, **always, in observe mode too** (there is
  no restore traffic to observe, the reason the consensus branch gives,
  `owner_acts.go:319-322`), an R whose `decision_ref` is not an unexpired,
  unused authorization for this computer, whose `proposed_effect_ref` or
  target fields differ from it, or whose `previous_head` is not the head the
  authorization was minted against or a descendant with no restore, apply or
  revocation between. Where the proxy forwards `/lifecycle/restore` and
  verifies the owner-act receipt for it was **not read**; the mint step goes
  there (open question 1).
- Further refusals at admission, each a typed error with the numbers:
  1. a pending transition at head(H) or at head(N);
  2. any `key_revoked` event with sequence above N, on any branch
     (conservative default: a restore must not re-enable a revoked
     credential, and the head's `credential_revocation_epoch` is
     deliberately not restored);
  3. N not a position on the tape, or N >= R;
  4. the checkpoint C not published for this computer at head(N), or judged
     `not_derivable` (the check the act makes today, `ErrCheckpointNotDerivable`);
  5. a live capsule, run or apply on the computer is refused by the guest
     before it asks (the `restoreRestartBusy` conditions,
     `self_development_restore_restart.go:121-130`; the lane's refusal beside
     live work is "not in dispute" in the assessment).
- The act stops appending `restore_requested`. A failed or refused restore
  leaves nothing on the tape, which is what the comment at
  `rematerialize.go:~346` already promises for a refused witness version. The
  owner-act receipt table is the audit of attempts.

### B.6 Replay, reconstruct and the recovery plan

The plan becomes: *base snapshot at or before N on the line; events to N;
skip to the restore event; continue.* Concretely:

- **The platform serves lines.** `EventsPage` (`event_replay.go:15`) returns
  the events of `line(head)` after `after`, in order. When `after` is the last
  event of a segment, the next page starts at the next segment's first event
  (a restore event). When `after` is not on `line(head)` it refuses with a
  typed `ErrNotOnLine` (the gapless check at `event_replay.go:64` becomes
  "gapless within segments"). The full physical walk moves to an explicit
  `AllEventsPage` used only by audit, export, sweep and compaction, which
  already walk the physical chain (`receipt_compaction.go:6`,
  `tape_sweep.go:771`). Making the safe behavior the default is deliberate:
  every consumer that reconstructs state (replay, catch-up, `snapshot.Scan`,
  `planRestoreTarget`, replay-completeness) gets lines without being edited
  and cannot forget to. A new `GET /internal/computers/events/lineage`
  **(invented)** returns the computer's restore events (sequence, digest,
  target) with their receipts, for planners that need ordinals before
  fetching.
- **The guest replays what it is served.** `reconstruct`
  (`appender.go:1426`) is unchanged in shape: it applies each served event
  through `applyDurableEvent`. The first event after a skip is always a
  restore event, whose branch of `applyDurableEvent` is B.4.2. The skipped
  events are never fetched, never prefetched (`prefetchReplayPayloads`), never
  applied, never indexed. The guest does not re-verify the hash chain across
  the skipped range: it trusts R's signed head receipt for `previous_head`,
  the same trust it places in a snapshot (open question 11 asks whether to
  fetch the skipped envelopes to check it).
- **A retained store must be on the line.** `PlanRecovery` stays pure, but its
  callers (`projection_base.go:143` and `:211`, `run.go:435`, the restore
  plan, replay-completeness) pass **ordinals** for local, watermark and target
  (`Line.Ordinal`), so `TailEvents` counts replayed events, not sequence
  distance, and `MaxRecoveryTailEvents` keeps meaning "events a boot may
  apply" (C.3). A caller also passes whether the local head is on the line. A
  local store whose head is not on `line(head)` is not a resume candidate: it
  takes the install path (B.9). Without this, a store at an abandoned head
  would resume, apply abandoned events, and mis-apply R (failures 6 and 7).
- **Snapshots as bases.** A snapshot is a valid base for a target iff both its
  recording event and its **store head** are on the target's line, and its
  store head is at or below the target position. Both conditions, because the
  producer captures a store at head h and records it later: a snapshot
  captured at an abandoned head h and recorded after R is a live event
  describing abandoned state. `snapshot.Scan` and `PlanCandidates`
  (`snapshot.go:1317-1347`) therefore filter on the store head with `Live`,
  and the platform serving lines already hides recordings in abandoned
  ranges. A snapshot taken after the restore has store head >= R and covers
  the new line.
- **Catch-up** (`catchup.go`) is unchanged and correct: it applies served
  (live) events that extend the local chain. A running store that is not at
  head(N) cannot extend through R; the existing "divergent chain" refusal is
  the safety net (B.7).

### B.7 The act, and the running process

Order (each step idempotent or refusable; the crash analysis is B.14):

1. Refuse beside live work (B.5.5) and take the act lock and the append
   fence.
2. Validate the checkpoint; check the target's release is pinned and not
   pruned, and that it is at or above the floor R will raise (B.12). Refuse
   here, before anything is written.
3. **Dry run**: build the staged store at N and check the canonical witness
   (today's A.1 step 4, up to `rematerialize.go:~190`), unchanged. This is the
   proof that the restore is derivable; it is what returns `witness_matched`.
   Keep it: once R is on the tape, a restore that cannot reproduce W would be
   irreversible.
4. Raise the store floor (B.12).
5. Append R through the appender in a **tape-only mode** **(invented)**: it
   does the pins and the CAS and does **not** finalize R into the running
   store, because the running store holds state(H) and R says state(N). It
   then sets a fence: every later append returns `ErrRestorePending`.
6. Restart the guest now, without a planned-restart marker. The boot after R
   is a crash-class boot by design (CLAUDE.md "Restarts": a crash restart
   never resumes work): nothing the abandoned line had open may resume. Open
   obligations at N, and the runs live at H, are closed "interrupted by a
   restart" by the boot's ordinary rule, as events after R. If the restored
   release execs another binary, the boot restages it first (B.9).

The running process never swaps a store, never reopens, never rebinds a
projection. That is the whole of the in-process state machine the three review
rounds could not settle; it does not exist here. The act does not touch
`current`.

### B.8 The typed refusal

`ErrRestoreBase`: a restore event reached with the local head not equal to its
target. At boot it means the plan was wrong and the store is rebuilt (B.9); in
catch-up it wedges as "divergent chain"; in the staged dry run it is a defect.
No path applies a restore event to a store that is not at its target.

### B.9 The boot after R (the store follows the tape)

The decision is derived from the tape alone, so every boot reaches the same
verdict and no second actor needs a record (the flaw of both lane designs):

1. Boot reads the canonical head and the lineage. If the local store's head is
   on `line(head)`, resume as today, on ordinals.
2. If it is not (the store holds the abandoned branch): the store is **not a
   base**. If the newest restore event's release differs from `current`,
   **restage `current` first** (`RestagePinnedRelease`, `updater.go:987`; the
   restored release is at or above the floor by B.12), and if it execs another
   binary, exit for the updater to restart into it (a normal boot; nothing is
   resumed). Then move the old state file, WAL, SHM and marker to a quarantine
   name beside the store (the shape of the compact rebase's `.precompact`,
   `internal/store/compact.go:595-614`, removed at `telemetry_retire.go:281-295`:
   the original is the rollback copy until the replay is done), and take the
   existing empty-store-with-chain path: install the newest snapshot on the
   target's line (B.6) and replay the line (`projection_base.go:152-215`). That
   path already clears and rebuilds the state file and the telemetry
   partitions (`snapshot.Install`, `PartitionDir`). It is the disk-lost
   recovery, already crash-resumable.
3. At the restore event's boundary in the replay (B.4.2) the boot **checks the
   canonical witness of the store at N against the manifest's W** whenever
   this boot applied events (a rebuild, or a resume from below N). The boot
   may be running the restored release's binary, an older one than the act
   that did the dry run; this check is what catches replay skew between the
   two (failure 24). A mismatch refuses the boot, loudly, keeps the quarantine,
   and names the table.
4. After `replay_done` (`run.go:911`) the quarantine is removed, in the place
   the compact rebase removes its rollback copy.

A kill at any point repeats the same decision from the same facts: the
restage is idempotent, the rename is guarded by "the original exists and the
quarantine name does not", and the install path already survives a crash.
There is no record that could disagree with the tape. What this deletes from
the parked lanes: the act record's phases, the in-process flip and undo, the
reopen and rebind, ten renames in a running process, the panic cases,
"restored, restart required" as a special state. What it costs: the boot
replays the line a second time (the dry run was the first); slice 8 removes
that by installing the verified staged store as the base (the lane document's
install half) and is optional.

Rows in the two tables the tape does not carry (`co_super_slots`,
`agent_evidence`, `compact.go:~700-716`, `replay_eligibility.go:69,90`) are
not restored by any replay and would be dropped silently at the rebuild. The
act refuses when either table is non-empty, with the counts (open question
5): a silent loss at an irreversible step is what the conservative default
prevents.

### B.10 How nested restores compose

`line(p)` is defined for every position, including positions on abandoned
branches, so nested restores are defined, not refused:

- **Restore to a point before an earlier restore event.** Target X < R_prev:
  `line(X)` is the prefix; R_prev falls in (X, R2) and is abandoned with
  everything else there.
- **Restore into an abandoned branch (a re-instate).** Target X inside
  (N, R_prev): `line(X) = line(N) ++ [N+1..X]`, because R_prev lies after X
  and does not count in X's line. After R2 the new line is that, then R2.
  This is the undo of the first restore: restore to the head you had at H
  (position H, which is in the abandoned range). Its snapshots are valid
  bases exactly when B.6's both-conditions test says so on the new line.
- **Restore to a restore event.** `line(R_prev) = line(N) ++ [R_prev]`;
  state(R_prev) = state(N). Allowed.
- **Depth.** `line` costs one step per restore on the path. The lineage is
  read as one indexed query on `(computer_id, event_kind, sequence)`.
- **Refusals that remain:** the credential refusal (B.5.2), a pending
  transition, N >= R, a release below the floor (B.12) and "the target's line
  has no base inside the replay bound" (C.3). The platform computes the line;
  the guest recomputes it from the served lineage and refuses an event whose
  target is not the head it reached.

### B.11 What happens to the surfaces

| Surface | Behaviour across R |
| --- | --- |
| Sequence numbers | Never rewound. R = H+1; later events H+2 onward. A line has gaps in sequence numbers; ordinals count line events. |
| Hash chain, `canonical_event_head` | One linear physical chain: R's `previous_head` is `digest(H)`. After R the canonical head is `digest(R)` and the guest's head equals it, so `HeadsAligned` and `sameHead` need no change. Only the guest's local index has a gap, and finalize is relaxed for R only (B.4). |
| Head fields | Desired, effective, pending and commitments are N's (B.4). The effective release commitment moves back with the restored release pointer, ending the divergence of A.3. |
| Checkpoints | One bound before R at a head on the line stays restorable. One whose accepted head is abandoned is restorable as a re-instate (B.10). A checkpoint taken after R binds at head >= R and its witness is of the store at that head. `checkpoints.go:59` is unchanged (C.2). |
| Snapshots | B.6: base iff recording event and store head are both on the line. The platform's retention must keep the newest base on the target's line until a snapshot at or after R exists (failure 23). Slice 6 has the guest queue a snapshot at R once its replay is done; until then the producer's cadence, on ordinals, does it. |
| Receipts | Unchanged: one head receipt per event (`computer_event_append_receipts`). R's receipt carries N's heads. `VerifyEventHeadReceipt` at replay is unchanged. |
| Run-memory retire ops and object retire ops | Ordinary ops inside projection batches (`ProjectionOpRunMemoryRetire`, `ProjectionOpObjectRetire`, `projection_batch.go:40-44`). A retire op in the abandoned range is skipped with its batch; one on the line is replayed at the same position as before; one after R acts on the rows the line holds. Retire ops name time slices and never touch the tape or blobs (`object_retire.go:1-30`), so a skipped retire loses nothing. The trim boundary is a function of head sequence (`event_index_trim.go`, store epoch 4): R's finalize trims at `R - window`, so the index rows of the line below it go, deterministically, live and replay alike (failure 19). |
| Idempotency keys | Unique per computer today (primary key of `computer_event_append_receipts`, `computer_events.go`). A key that an abandoned event used and that the new line re-derives would be answered with the abandoned event's receipt (`existingComputerEventReceipt`): a lost write. Uniqueness becomes "among events on the line": **(invented)** a column `abandoned_at_sequence`, set in R's admission transaction on the rows of the previous line in (N, R), and the primary key replaced by a unique index on `(computer_id, idempotency_key)` where it is null. Only an index column changes; no event body, digest or receipt JSON is rewritten. |
| Store schema floor | Epoch 7 **(invented)**: "the tape may hold a restore event; the line is not 1..head." The floor is written before the first R request (B.12). |

### B.12 Old binaries, the floor, and the release

- A restore event on the tape reaches every reader through
  `DecodeHistoricEvent` (`decode.go:123`, no `DisallowUnknownFields`) and then
  `Event.Validate`: an old binary has no `computer_restored` in
  `validEventKinds` (`event.go:~91-99`) and refuses with "unknown event kind".
  The kind check is the fence, not the new fields: an old decoder silently
  drops unknown fields and would compute a different digest (failure 2).
- The guest writes the store floor (`storeschema.WriteFloor`, `floor.go:70`,
  written "before it first writes data an older binary would mishandle") at
  epoch 7 before the first R request. The updater already refuses Apply and
  `restorePrior` to a release below the floor or one that declares no epoch
  (`floor.go` header). `RestagePinnedRelease` checks the same floor
  (`updater.go:987`).
- **Consequence stated plainly:** a restore addresses a release (A.1 step 5:
  the restore set includes the release pointer, computer-ontology
  "Restore-set boundary"). A release that does not know `computer_restored`
  cannot read the tape that holds one, so **a checkpoint whose release
  predates the slice that introduces the event cannot be restored**: the act
  refuses it before writing anything, naming the floor. Every checkpoint that
  exists the day slice 2 deploys is in that class. The first restorable
  checkpoint is one bound after that deploy (the d@e drill binds one, F.2).

### B.13 What is not undone

The restore set is event head, release pointer, VM-local SQLite state and the
computer-surface frontend (`docs/computer-ontology.md`, "Restore-set
boundary"). Plainly, these are **not** undone, and the product says so in the
act's answer and in `choir` output, which lists counts by kind from the
abandoned range (the manifest carries the same counts). The kinds to count
are taken from the code at slice 7 (mail send, publish and provider-call
events are the ones I expect; I did not enumerate them, **invented** until
read):

- Objects published to the shared Autopaper object graph (captures,
  publications). Others may have adopted them. The restored computer's state
  no longer records having published them.
- Mail sent, and any other effect on an outside party. The Mail desk's state
  at N may show an item unsent that was sent. Double-send risk is real;
  custodian desks reconcile on the restore event (open question 6).
- Provider calls and their cost, including Jev calls. The spend stays spent.
- Releases the updater applied in the abandoned range stay in the updater root
  until pruned; `current` follows R.
- Files on the data volume outside SQLite (the file tree and its file CAS).
  The restore of the file root from `file_root_committed` was **not read**;
  until it is, the act refuses a target whose last file root differs from the
  head's (failure 51).
- Anything another computer read from this one.
- The abandoned events themselves: they stay on the tape, readable by audit
  (`AllEventsPage`), subject to the same retention, and shown in history
  marked abandoned (failure 54).

### B.14 Crash cases

The tape is the record. "A crash never resumes work" applies to obligations,
not to deciding which state a computer should hold; the rebuild is the boot's
ordinary recovery of state from the tape, like a lost disk. A planned restart
may resume once with a marker; the restore boot deliberately has none.

| Crash point | Result |
| --- | --- |
| Before R is durable on the host (including during the dry run or the floor write) | Nothing happened on the tape. The staged store is swept (`sweepStagingArtifacts`, `projection_base.go:~255`); `current` is untouched because the act restages nothing. The floor is raised, harmlessly, and forever. The owner retries. |
| R durable, response lost (timeout) | The idempotent resend returns R's existing receipt (`resendUnclearCASLocked`, `appender.go:1257`; `existingComputerEventReceipt`); the act proceeds to the restart. |
| R durable, process dies before the restart | The next boot is a crash boot; the tape holds R; the local store is not on the line; B.9 rebuilds. The act's success is determined by the tape, not by the process that asked. |
| During B.9 (after the restage, between renames, mid snapshot install, mid replay) | The same decision on the next boot from the same facts. Restage and rename are idempotent; the snapshot install and the replay resume as for a lost disk. The quarantine keeps the old store until `replay_done`. |
| After the rebuild, before the quarantine is removed | The next boot finds a valid store on the line and a leftover quarantine; it removes it after `replay_done`. |
| The restart after R never comes | The fence keeps the process refusing appends (`ErrRestorePending`); `/health` reports restore pending. The host's existing stall detector restarts it; no new timer (open question 3). |
| The restored release will not boot | The runtime unit's boot-loop guard (`nix/autoputer-vm.nix:~188`) and the updater's rollback run as for any release. Recovery is another restore (a re-instate to H), which needs a release at or above the floor. |

## C. What changes in the platform authority

### C.1 Accepting the restore event

`Reduce` gets the explicit case (B.4). `compareAndSwapTx`
(`computer_events.go`) gets: the `admitRestore` gate (B.5), the lineage checks
(B.5 refusals 1 to 4), the `restore_authorizations` consume and the
idempotency re-marking (B.11), all inside the one serializable transaction
that already updates the head and inserts the receipt, so R is atomic with
them. A CAS lost to a concurrent append is a refusal because
`RequireExpectedHead` is set.

The restore-target claim in `TransitionInput` is not trusted: the CAS reads
the stored receipt at N (fields and digest) and compares. Nothing else in the
CAS changes; the head update `WHERE sequence=? AND canonical_event_head=?` is
the same.

### C.2 Does `checkpoints.go:59` change?

No. The event makes ancestor checkpoints **unnecessary for restoring**: a
restore needs C (published when its head was the head, so `AcceptedEventHead`
was current then) and the tape; it does not need a new checkpoint at an
ancestor. The check at `:59` governs *publishing*, and publishing at the head
is still right. What the line gives that the restore-bases assessment (item 3)
asked for is the **proof that a head behind the current one is on the
chain**: a head's membership of `line(head)` is a lookup of its receipt row
plus the lineage. A later slice (9, optional) may let the authority accept a
checkpoint whose accepted head is an ancestor on the line, verifying pending,
effective and release fields against the stored receipt for that ancestor
instead of the head row. That unblocks "restore bases" capture at a copy's
head and is a separate red change with its own review. It is not required by
this one.

### C.3 The 720,000-event refusal

- The refusal's meaning is kept ("replayed events a boot may apply") and its
  arithmetic is fixed: `tailLen` runs on ordinals (B.6). After R the boot's
  tail is the events of `line(head)` after the base, `(N - base) + 1` plus
  later events, not `R - base`. A restore far behind the head is no longer a
  boot that refuses: the distance between N and H is irrelevant to the next
  boot.
- The refusal that does remain moves to **acceptance**: the target's line has
  no valid snapshot (B.6) within `MaxRecoveryTailEvents` ordinals below N and
  N's ordinal is above the bound. `planRestoreTarget` already refuses this
  (`restore_snapshot.go:~140-146`); it keeps that and counts ordinals. The act
  refuses with the numbers instead of a boot refusing later.
- Retention must keep that base: the platform's snapshot pruning ("the host
  keeps a chain of older roots and prunes the ones between",
  `restore_snapshot.go:~134`) must not prune the newest valid base of the
  current line below R until a snapshot at or after R exists (failure 23,
  slice 6).

## D. Failure list (tests are written from it before code)

62 entries. Prevent is what makes the failure impossible or refused; detect is
what shows it if prevention fails. "T:" names the test that must exist first.

### D.1 Tape, reducer, chain

| # | Failure | Prevent | Detect |
| --- | --- | --- | --- |
| 1 | An old guest or platform binary meets the event. | Kind absent from the old `validEventKinds` (`event.go:~91`): refused at `Validate`. Floor 7 written before the first R. Deploy order (E). | Boot refusal log. T: a binary built from the previous commit refuses a tape with R. |
| 2 | An old decoder drops the new fields and recomputes a different digest. | The kind check comes first; the fields are not the fence (`decode.go:123` does not disallow unknown fields). | T: decode R with a struct lacking the fields; the digest differs and `Validate` refuses first. |
| 3 | The new envelope fields change the canonical bytes of old events. | `omitempty`. | T: golden digests over real historic events (a recorded fixture of kinds and fields). |
| 4 | `TransitionInput.RestoreTarget` changes old pin-intent commitments (`request.go:12`). | Nil pointer, `omitempty`. | The same golden test for `ComputePinIntentCommitment` and `ComputeRequestCommitment`. |
| 5 | The kind falls into the reducer's causal default (`reducer.go:~194`): head unchanged. | Explicit case. | T: exhaustiveness over `validEventKinds`. |
| 6 | Replay applies R to a store not at N. | `applyDurableEvent` checks head digest and sequence; `ErrRestoreBase`. | T: apply R with local head at N-1, N+1 and an abandoned head; each refuses. |
| 7 | A retained store at an abandoned head resumes and applies abandoned events. | `PlanRecovery` callers pass "local on line"; the platform refuses `after` not on the line (`ErrNotOnLine`). | T: boot with a store at H after R rebuilds and never resumes. |
| 8 | The store's finalize rejects R (`computer_events.go:297`). | The relaxation for this kind only. | T: finalize R at N; finalize R at another head refuses. |
| 9 | The relaxation lets any event skip a range. | Only R, only with the target equal to the current head and to the event's own target fields. | T: a forged non-restore event with a gap refuses. |
| 10 | A restore with a pending transition at H or N leaves a dangling pending ref. | Admission refuses both; the reducer refuses at H. | T: both. |
| 11 | A restore re-enables a revoked credential. | Refused when any `key_revoked` has sequence > N; the epoch is not restored (B.5.2). | T: restore across a revocation refuses; across none passes. |
| 12 | A second restore to the same checkpoint appends nothing (`rematerialize.go:399-404` today). | Key `restore-<A>`, A single use. | T: restore twice to one checkpoint gives two events. |
| 13 | A key re-derived after the restore is answered with an abandoned event's receipt. | The partial unique index and the `abandoned_at_sequence` marking (B.11). | T: append key K, restore past it, append K again: a new receipt on the line. |
| 14 | A concurrent append between the guest's read and the CAS is silently abandoned. | `RequireExpectedHead` (`appender.go:951`). | T: a lost CAS is a refusal, not a rebase. |
| 15 | Two restores at once, or a restore beside a self-development apply. | The act lock; the reducer refuses with a pending transition. | T: both. |
| 16 | The receipt compaction (`receipt_compaction.go`) assumes a chain it cannot expand. | The physical chain is unchanged by R; verify, do not assume. | T: compact, expand and verify a tape with R. |
| 17 | A forged R from a compromised guest. | `admitRestore`: single-use platform-minted A, always enforcing; target claims checked against the platform's own receipts. | T: R without A, with a used A, with an expired A, for another computer, checkpoint or target: each refuses. |
| 18 | An authorization minted at head H is used after a deploy, an apply or a revocation. | A names the tape head at mint; R's previous head must equal it or have no restore, apply or revocation between. | T. |

### D.2 Replay, plan, snapshots, ordinals

| # | Failure | Prevent | Detect |
| --- | --- | --- | --- |
| 19 | The trim boundary jumps at R's finalize (`event_index_trim.go`). | It is a pure function of head sequence applied in finalize, so live and replay agree. Do not special-case. | T: replay equals direct by canonical witness across R with the trim window crossed. |
| 20 | Code that treats sequence difference as an event count: `tailLen` (`recovery_plan.go:~60`), the replay observer's `tailReceipt` (`rematerialize.go:170`, `replay_completeness.go:401`), `PlanCandidates(target, bound)` (`snapshot.go:1333`), the snapshot cadence, replay progress, health displays. | All pass ordinals (B.6). | T: each with a tape holding a restore whose gap exceeds the bound. The grep list is a slice 3 deliverable. |
| 21 | A snapshot from the abandoned branch is chosen as a base. | Base iff recording event and store head are on the target line; the platform hides abandoned recordings (B.6). | T: a snapshot recorded after R with an abandoned store head is never a candidate; one with store head <= N on the line is. |
| 22 | A snapshot captured at an abandoned head is recorded after R by the old process. | The append fence (B.7.5) stops appends; the head-on-line test stops the rest. | T. |
| 23 | The newest valid base of the line is pruned and the next boot has no base inside the bound. | Retention keeps it until a snapshot at or after R exists (C.3). | T: prune with a restore; the act refuses at acceptance when the base is gone. |
| 24 | The replay of `line(N)` differs between the act's dry run and the boot (the boot may run an older release's binary). | The witness check at the R boundary (B.9.3). | T: skew test with a deliberately different replay refuses. |
| 25 | A replay page straddles the skip and applies abandoned events. | The platform serves lines; the guest never receives them. | T: page sizes 1, 2 and larger across the gap. |
| 26 | The platform's page-serving cost grows with the lineage. | One indexed query; computed once per request. | Page timing in the existing replay log line. |
| 27 | Replay progress and `/health` show events behind as sequence difference. | Ordinals. | T. |
| 28 | A reader assumes the local event index is contiguous or that each row's `previous_head` is present (`computer_event_index`). | Audit in slice 2: the witness's event-index digest, `NewestEventOfKind`, the trim, `EventByIdempotency`, the snapshot manifest, replay-completeness. | T: witness equality live versus replay across R (the canonical witness of 304a81277 is the oracle). |
| 29 | The witness of the store at R is compared to the checkpoint's W for N and differs by design. | W is checked at N (B.9.3). After R the oracle is replay-completeness: live equals a replay of `line(head)`. | T. |

### D.3 The act, the process, the boot

| # | Failure | Prevent | Detect |
| --- | --- | --- | --- |
| 30 | The running process serves or appends after R with state(H). | Tape-only append, then the fence (`ErrRestorePending`); catch-up's divergent-chain refusal. | T: after R an append refuses; `HeadsAligned` reports. |
| 31 | The act restages `current` before R and R is never written. | The restage happens at the boot that sees R (B.9.2), not in the act. | T: kill before R; `current` unchanged. |
| 32 | A restore resumes the abandoned line's open work. | No planned-restart marker (B.7.6). | T: open run at H; after the restore boot it is closed "interrupted", as an event after R, and not resumed. |
| 33 | The restore boot is treated as planned and the compact rebase swaps a staged copy over the rebuilt store. | No marker, so `planned=false`; this avoids the collision the lane found (restore-boot-install 2.7). | T: a staged compact copy present at the rebuild boot. |
| 34 | The restored release is pruned between the act and the boot (`updater/prune.go:20-61`). | Checked at the act and again at the restage; failure there refuses the boot loudly with the old store in quarantine, and the owner restores to another checkpoint. | T. |
| 35 | The restored release is below the floor or predates the event. | Refused at the act (B.12). | T. |
| 36 | The restored release execs another binary and the rebuild runs under the wrong one. | Restage first, restart, then rebuild (B.9.2). | T: exec-change restore across two boots with kills between. |
| 37 | A crash between the quarantine rename and the install leaves a store-less volume. | The install path is the existing lost-disk path; the quarantine is a rename the next boot sees. | T: kill at each rename and in the snapshot install. |
| 38 | The old store is destroyed before the rebuilt one is proven. | Quarantine removed only after `replay_done`. | T. |
| 39 | Disk: staged dry run plus live plus rebuild plus quarantine. | The act's floor counts the staged copy, the workspace and any quarantine (the lane's round-3 caveat); the dry run's staging is removed before R. | T: ENOSPC at each step. |
| 40 | The dry run takes longer than the request (the ten-minute budget, `restore_snapshot.go` `restoreCandidateBudget` comment). | Out of scope here: slice 7 keeps the synchronous route, enough for d@e-class stores; the asynchronous accepted-act surface of `lane/restore-act` is the follow-on, reusable because its install half is deleted here. | The first restore of a larger store times out loudly. |
| 41 | The boot after R exceeds the boot deadline. | `MaxRecoveryTailEvents` on ordinals; the act refuses at acceptance (C.3). | T. |
| 42 | The act's answer is read as "done" while the rebuild is pending. | The 200 says `restore_committed: true, state: pending_rebuild`; the CLI polls through 502 and 503 until the head is R and the witness at N matched. | T on the CLI. |
| 43 | Rows in `co_super_slots` or `agent_evidence` are dropped by the rebuild. | The act refuses with counts when non-empty (B.9, open question 5). | T. |
| 44 | Stale realizations (a cold-recovered copy, `vmctl/cold_recover.go`; a sibling fork) hold heads on the old branch. | A cold recover is a disk-lost boot and takes the line. A stale writer is refused by the CAS (head moved). A fork has its own identity and tape. | T: cold recover after R. |
| 45 | Route projection or the serving route stays on the pre-restore release. | **Not read** (vmctl route CAS). The drill checks the served release equals C's release after the restore (F.2). | Drill. |
| 46 | The telemetry partitions and derived-key marks of the old store leak into the rebuilt one. | The install path empties and reinstalls partitions (`snapshot.Install`, `PartitionDir`); the marks follow the head. | T: witness across the rebuild. |
| 47 | The restore boot leaves a staging directory or quarantine behind. | `sweepStagingArtifacts` and the post-replay removal; the floor counts them. | `df` and the drill. |

### D.4 Outside the store (stated, not prevented)

| # | Failure | Prevent | Detect |
| --- | --- | --- | --- |
| 48 | Published objects remain in the shared graph. | Not undone (B.13). The act's answer lists counts; the manifest records them. | The answer. |
| 49 | Mail sent and unsent-looking at N: double send. | Custodian desks see R and reconcile (open question 6). Until built, the answer names mail events in the abandoned range. | Manifest counts. |
| 50 | Spend (provider and Jev calls) is not refunded. | Stated. | Ledger. |
| 51 | The file tree outside SQLite differs from the file root at N. | **Not read.** Refuse a target whose last `file_root_committed` differs from the head's until the file restore is read and specified. | T. |
| 52 | The updater's applies in the abandoned range leave releases and journals. | Prune handles them; `current` is set at the rebuild boot. | Updater root listing. |
| 53 | An update offer minted for a base in the abandoned range is used later. | The offer's base head must equal the new event's `previous_head` (`owner_acts.go:~375`), so it is refused. | T. |
| 54 | History shows the abandoned branch as if live (`api_trajectory.go`, the UI). | Serve lines by default; an explicit "abandoned" view from `AllEventsPage`. **Not read**: consumers beyond `EventsPage`. | Review of each consumer. |
| 55 | Audit readers (`ledgerwitness/walk.go`) count abandoned batches as the computer's records. | Walk the line or label by line. **Not read.** | Consumer audit in slice 3. |
| 56 | Retention and sweep treat abandoned events as live or as free to delete. | They walk the physical chain and apply the same windows; abandoned events are history. | `tape_sweep` tests with R. |

### D.5 Versioning, deploy and rollback

| # | Failure | Prevent | Detect |
| --- | --- | --- | --- |
| 57 | Replay skew between the release that did the dry run and the restored release that rebuilds. | The witness at the R boundary at boot (the same as 24, kept as its own line because it needs its own test on an exec-change restore). | B.9.3. |
| 58 | A host (platform) rollback to a binary that predates R makes the computer's replay fail. | **(invented)** The deploy refuses a host release older than the newest tape kind on any computer, the host-side analogue of the store floor; or the rollback recipe is "none". Open question 7. | The deploy script. |
| 59 | A guest rollback below the floor. | The updater's floor check (`floor.go`). | T. |
| 60 | The partial unique index migration on a live Postgres table. | Online index build, then the swap, in the slice that needs it, deployed before any writer exists. | Migration test and deploy log. |
| 61 | A tape exported from one platform and imported to another loses R's semantics. | The line is a function of the tape; export walks the physical chain (`receipt_compaction.go:6`). | T: export, import, replay equal. |
| 62 | Docs state the old restore semantics. | `docs/computer-ontology.md` (restore-set paragraph), `docs/design/computer-history-restore-audit-2026-10-30.md` G1 and the restore problem record are updated in the same change as the writer. | Review. |

## E. Old binaries and the deploy order

What an old binary does on meeting the event:

- **Platform, old:** `Reduce` calls `event.Validate()` first (`reducer.go:~56`),
  which returns "unknown event kind" (`event.go:~180`); the CAS refuses the
  append, and a page read decodes with `DecodeHistoricEvent` and then
  `Validate` through `Digest()`, refusing at the event. It never serves past R
  wrongly.
- **Guest, old:** the replay's `applyDurableEvent` calls `Reduce`
  (`appender.go:1668`), which refuses; boot fails with `boot.fatalf("autoputer:
  reconstruct computer event authority: ...")` (`run.go:~890`). The old guest
  also cannot open a store the new one wrote: the floor is at epoch 7.
- Neither can run a store or a tape past R silently. This is the existing
  op-kind and floor mechanism; nothing in this design adds a path that tries to
  continue past an unknown kind.

Deploy order (each step is a landing with its own proof):

1. Guest and platform that **know** the kind and the lines, with **no writer**
   (slices 1 to 3), and with the platform CAS refusing every
   `computer_restored` append (slice 2).
2. The idempotency index and `admitRestore` on the platform (slice 4). Still no
   guest writer.
3. The boot rebuild (slice 5) and the retention (slice 6), before the writer,
   so a restore is never written to a guest that cannot follow it.
4. The writer (slice 7), last, on d@e only first.

The platform can always admit what the guest writes, because step 4 follows
step 2; and no release predating step 1 can be restored (B.12).

## F. Proof

### F.1 Repo tests, written before the code of each slice

All through the real replay path with the canonical witness of 304a81277 as
the oracle, and a model-based property test for `Line` against a brute-force
walk of a small tape.

1. `Line`: segments, `Live`, `Ordinal`, `Count` on random sequences of
   restores (including nested, to a restore event, into an abandoned range,
   before an earlier restore) equal a naive replay-by-definition.
2. **Replay of a tape with R equals direct state.** Build a store by direct
   application of the line's events (events 1..N, then the events after R; the
   direct path never sees the abandoned events) and a store by replaying the
   whole physical tape through the line-serving source. Equal by canonical
   witness and by event-index digest. Variants: R at the head; two restores;
   nested (before an earlier restore, into an abandoned range, to a restore
   event); a retire op, a run-memory retire op and a partition retire on each
   side of R; the trim window crossed; the abandoned range holding a snapshot
   recording, a `key_rotated` and a lifecycle event.
3. Reducer: the head after R; every refusal in B.5 and B.4; the exhaustiveness
   test; the golden digests and commitments of old events.
4. Plan: ordinals, local-on-line, a snapshot with an abandoned store head and
   a live recording, base retention, the acceptance refusal with numbers.
5. Old binary: a binary at the previous commit refuses the tape at R by name.
6. Crash cases through the real boot function with a SIGKILLed child at each
   step: before R; R durable and the process killed before the restart; during
   the restage; between the renames; mid snapshot install; mid replay; after
   `replay_done` before the quarantine removal. Each ends in the same store as
   the uninterrupted run (witness), with no resumed obligation.
7. Platform: the `admitRestore` table of refusals (failures 17 and 18), the
   idempotency re-marking (13), the lost CAS (14), the transaction atomicity.
8. Act: tape-only append and fence (30), no restage before R (31), a release
   below the floor refused (35), refusal beside live work, non-empty non-tape
   tables (43), the answer's counts of abandoned effects.

### F.2 The deployed drill: d@e only

Never the largest computer, never the owner's. This is staging
(`https://choir.news`). Preconditions: the d@e rows in `co_super_slots` and
`agent_evidence` are counted first (43); the deployed commit is verified.

1. Bind and publish a checkpoint C at head N with a release that knows the
   event (`choir computer checkpoints`). Record W, the release digest and the
   served route's release.
2. Write new work W1 (an owner edit and an agent run) so the head is H > N.
3. `choir computer restore` to C as an owner act. Expect 200,
   `witness_matched: true`, `restore_committed: true`. Read the tape: R at H+1
   with the fields of B.3.
4. Wait for the restart and the rebuild. Verify: the head is R; the store
   witness at the R boundary matched W (the boot log line); `current` is C's
   release and the served route says so; W1's edit is absent; the abandoned
   events are on the tape and absent from the served page.
5. Write new work W2. Restart the guest (a crash restart, then a planned one).
   Verify: the state is `line(N) + R + W2` by canonical witness, equal to a
   fresh replay-completeness run (`replay_completeness.go`) from a base on the
   line.
6. A second restore: to C again (a new event), then to a head inside the
   abandoned range (a re-instate of W1's state) and back. Verify B.10.
7. Adversarial kills (d@e only): kill the guest VM after R before the restart;
   during the restage; during the snapshot install; during the replay; after
   `replay_done` before the quarantine removal. After each, the computer boots
   to the same witness and nothing is resumed.
8. Refusal drills: a checkpoint of a release below the floor; a target beyond
   a revocation; a restore beside a live run; a replayed authorization.

Claims to precommit in `docs/calibration/claude-ledger.md` before the writer
lands, each an outcome the author cannot write: "a restore survives a restart
on d@e" (the witness after step 5), "a restore leaves the abandoned events off
the served page", "the dry run's witness equals the boot's witness at R".

## G. What this deliberately does not do; rollback; open questions

Does not do:

- No undo by deletion; no truncation or rewriting of the tape; no restore of
  the shared graph, mail, spend, other computers, or the data volume's files.
- No second mechanism: no fallback to the in-process flip, no setting, no
  allowlist, no selector. The route that restored in-process is replaced.
- No restore to a release that does not know the event.
- No change to `checkpoints.go:59` (C.2).
- No asynchronous accepted-act surface in this change (failure 40).
- No installation of the verified staged store (slice 8, optional).

Rollback of the change: slices 1 to 6 revert by order; nothing on any tape
depends on them. After the writer has run, **a tape that holds R cannot be
rolled back past slice 2**: old binaries refuse it (E). So the writer lands
last, on d@e first, and the floor is raised only by the first R request;
reverting slice 7 stops new restores and leaves the reading code in place.
Each slice's commit subject carries its class and rollback; every non-docs
landing carries `Ring-Expedite:` naming the defect it unblocks (a restore that
does not survive boot or append); each expedite is named in the next report.

Open questions, each with its conservative default (nothing blocks on them):

1. Where the owner-act receipt for `/lifecycle/restore` is verified and where
   A is minted (proxy or corpusd; not read). Default: corpusd verifies the
   owner-act receipt with the existing `owneract.Verifier` and mints A; no
   restore without it.
2. The event's name and the two new envelope fields (invented). Default: as in
   B.3.
3. A watchdog for "R durable, restart never comes". Default: none; the fence
   and `/health` make it visible and the host's stall detector restarts it.
4. Whether the first restorable checkpoint after the deploy is acceptable, or
   the owner wants a platform-bound "restore anchor" checkpoint taken at the
   deploy of slice 2 for each computer. Default: acceptable; d@e binds its own.
5. Non-tape tables: refuse when non-empty (default), or restore and report the
   loss.
6. How custodian desks (Mail, Autopaper) react to R. Default: nothing beyond
   the counts in the answer and the manifest; a desk-level reconcile is a
   later precommitment-record slice.
7. A host-release floor so a platform rollback past the first R is refused
   (failure 58). Default: the deploy of slice 2 documents that rollback past
   slice 2 is unavailable once a restore has run.
8. The witness at the R boundary on a large store: its cost is not measured.
   Default: required for every boot that applied events; measure on d@e and
   restate.
9. Credential refusal granularity: refuse on any `key_revoked` above N
   (default) versus only revocations of credentials the store at N holds.
10. Whether to witness the store once at the end of the first replay after R
    as a checkpoint of record. Default: the next scheduled checkpoint covers it.
11. Whether the guest fetches the skipped envelopes (not payloads) to verify
    the hash chain across the abandoned range. Default: no; trust R's signed
    head receipt, as a snapshot's trust.

## H. Slices, in order, each reviewable alone

Each slice: its mutation class, the proof that lands with it, the rollback.

1. **`computerevent.Line` and its property tests** (yellow; revert). Pure,
   unused by production. Proof: the model-based test. No behavior change.
2. **Event kind, fields, reducer, replay interpretation, store finalize,
   floor declaration, no writer** (red; two seats; revert, by order, since no
   tape holds the kind). Includes the explicit refusal of every
   `computer_restored` append in the platform CAS, the golden-digest tests,
   the old-binary refusal test and the replay-equals-direct tests on synthetic
   tapes (F.1 2, 3, 5). Declares epoch 7 in `storeschema` without writing the
   floor. Proof: those tests; a deployed no-op (nothing writes R).
3. **Lines everywhere a tape is read for state** (red; two seats; revert).
   Platform `EventsPage` serves lines and refuses `ErrNotOnLine`;
   `AllEventsPage` and the lineage endpoint; ordinals in the `PlanRecovery`
   callers, the snapshot scan and candidate filter, cadence,
   `planRestoreTarget`, replay observers, health. With no restore event on any
   tape the behavior is identical. Proof: the whole uncached suites of every
   touched package and importers; replay-completeness on d@e unchanged; the
   grep list of sequence-arithmetic sites in the commit message; the consumer
   audit of failures 54 and 55.
4. **Platform admission** (red; two seats; revert): `restore_authorizations`
   and the mint, `admitRestore` always enforcing, the lineage checks, the
   idempotency partial unique index and re-marking, the head fields from the
   stored receipt. Still no guest writer (an authorization can be minted in a
   test only). Proof: F.1 7; the online index migration on staging.
5. **The boot rebuild** (red; two seats; revert): the plan's "local not on the
   line" branch, the restage-first step, the quarantine rename and its removal
   at `replay_done`, the witness check at the R boundary. Exercised by a test
   tape and a d@e store forced onto an abandoned head, with no writer. Proof:
   F.1 4 and 6 with a synthetic R.
6. **Snapshot retention on the line** (orange; one seat; revert): the platform
   keeps the newest valid base below R until a snapshot at or after R exists;
   the guest queues a snapshot at R once its replay is done. Proof: the prune
   test and the d@e snapshot after a restore.
7. **The restore act writes the event** (red; two seats; revert): tape-only
   append, the fence, the restart without a marker, the floor write, removal
   of `appendRestoreIntent`, the new answer and CLI poll, the refusals of B.5
   and B.12, and the non-tape-table refusal. This is the writer and the
   switch-on; there is no setting. Proof: F.1 8; the deployed drill F.2 on d@e
   only; the replay-completeness equivalent.
8. **Optional: install the verified staged store as the base** (red; two
   seats; revert): the lane document's install half, reduced to "a staged
   store whose head is N and whose witness is the manifest's W is a base like
   a snapshot". Removes the second replay. Proof: the kill drills with the
   staged store present and absent.
9. **Optional: the checkpoint authority accepts an ancestor on the line**
   (red; two seats; revert): C.2. Unblocks restore-bases capture; separate
   proof and review.
