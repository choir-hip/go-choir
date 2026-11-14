# A restore on d@e cannot finish inside its ten-minute request

Recorded 2026-11-14T22:57Z. Not repaired.

## Problem

`choir computer restore` is one blocking request with no async form and no
status route. Every budget on its path is ten minutes (proxy
`PROXY_REPLAY_COMPLETENESS_TIMEOUT=10m` in `nix/node-b.nix:961`; the guest's
write deadline in `internal/agentcore/api_self_development.go:31`). The work
is the scan, the install of the newest snapshot at or below the checkpoint,
the replay of the tail at about 113 events a second, then the witness check,
then the flip. A checkpoint that sits just below a snapshot falls back to the
snapshot before it. On d@e that was 74,000 events: 16 to 20 minutes of work.
The client gives up, the cancel aborts the replay, nothing is flipped, and
the caller is told nothing.

Three smaller defects seen on the way:

- A witness mismatch is not in `rematerializeRefusalStatus`
  (`rematerialize.go:374`), so it would answer 500, and only after the whole
  replay. Read from code, not observed.
- The restore holds `currentReleaseMu` for its whole run; the hourly cadence
  checkpoint waited on it and logged `held appends 6m17s`. That figure is the
  bind's wall time, not an append hold (`checkpoint_cadence.go:295`).
- The recovery bound is in events (720,000, `recoveryplan/recovery_plan.go:22`)
  with no matching time budget.

## Evidence

- Step 7 of the step 2 proof, run by an operator agent on d@e under N7.
  Checkpoint 189a91ed..., 21:34:02Z, sequence 108,784, `owner_recovery`,
  verdict `unverified`/`no_base`. d@e's own key carries `owner:acts`.
- Request 1, 22:38:33Z: Caddy status 499 after 74.9 s (the CLI's default
  timeout). Request 2, 22:43:29Z with a nine-minute timeout: 499 after 539.9 s.
- Guest log: staged store opened 22:47:35Z; `replay applied 33332 events
  through seq=68048 in 4m54s` at 22:52:29Z, the second the client gave up;
  the replay started after sequence 34,716. No flip, no witness line.
- After: d@e active, realization epoch 13706 unchanged, the revision written
  after the checkpoint still present, same process since 22:28:21Z.
- Guest memory read 1.2 to 1.7 GB against a 1,024 MiB ceiling during the
  second request; vmctl reclaimed each time; no kill.

## Belief

The two requests did nothing durable beyond the restore intent event
(idempotent per checkpoint; not checked on the tape). A third request with
the same checkpoint would fail the same way. A checkpoint at or above the
newest snapshot would replay a short tail and could finish; the cadence bind
published one at sequence 110,778 at 22:52:48Z.

Even with time, the restore's witness is the strict comparison, and d@e's
replay differs from live on the event index
(`replay-of-de-differs-on-the-event-index-only-2026-11-14.md`). The restore
may refuse on that. Not observed.

## Remaining error

- Whether the staging directory was removed on the guest; what request 1
  reached before the cancel.
- The repair: restore as an accepted act with a status to poll, its budget
  tied to the events to replay, and a witness mismatch answered as a refusal.
  Not designed.
- Steps 7d to 7f of the step 2 proof are not done.

## Third request, with a checkpoint above the snapshot (2026-11-14T23:05Z)

Claim C89, entered first; true.

- Checkpoint df408396..., 22:52:48Z, sequence 110,778. The newest snapshot is
  `interval snapshot at head 108786`, 21:34:58Z, so the tail was 1,992 events.
- Restore 22:58:30Z to 23:02:39Z: http 500, `rematerialize: reconstructed
  computer_event_index does not match checkpoint`. Replay took 15.6 s; the
  3m44s before the staged store opened is the scan and the snapshot install.
- Nothing flipped: epoch 13706 before and after, the revision written after
  the checkpoint still answers 200.
- d@e's console also shows snapshots refused for free space under the 2 GiB
  floor from 20:06Z to 21:17Z, before the boot prune freed the volume.

So on d@e today no checkpoint can be restored: the restore's witness compares
the event index strictly and a replayed index differs from the live one
(`replay-of-de-differs-on-the-event-index-only-2026-11-14.md`). Making the
probe tolerant is not a repair of this; the witness that a checkpoint records
and a restore recomputes needs the same canonical form, versioned so an old
checkpoint still fails closed. The refusal also answers 500 where it should be
a refusal status. Whether the paper or any other computer is in the same state
is not known; nothing was run there.

Steps 7d to 7f of the step 2 proof stay open behind this.


## Why the base snapshot was so old (2026-11-14T23:08Z; code and console logs, read-only)

- A snapshot is due when the head is 540,000 events past the newest one or
  seven days have passed (`internal/agentcore/snapshot_producer.go`,
  `snapshotDefaultTailEvents`, `snapshotDefaultInterval`); the producer ticks
  every ten minutes. It is not linked to the checkpoint cadence, which is a
  separate loop (`checkpoint_cadence.go` names no snapshot).
- d@e's snapshot fell due on the seven-day rule about 2026-11-13T22:10Z and was
  refused every ten minutes until 21:17Z on 11-14: 0.9 GB free against the
  2 GiB floor. The boot prune freed 10.6 GB at 21:31Z and the next tick took
  it (head 108,786, 11 s). That the 34,716 snapshot was the only older one is
  inferred; logs before 11-12 are gone.
- The 21:34Z checkpoint sits two events below that snapshot only because the
  cadence waits two minutes after boot and the producer three. Not a rule:
  the binds at 18:29Z, 19:29Z and 20:31Z had no snapshot after them.
- The worst case by design: 540,000 events of tail, about 80 minutes of replay
  at d@e's 113 events a second, against a ten-minute request.
- The paper's newest snapshot is a day old because none is due: about 282,000
  events since (inferred from the effective release's head, not the live
  head). No producer line in its retained log (from 17:43Z).
- A refusal that lasted 23 hours was visible only in the console log.

Repairs, not designed: restore as an accepted act with a status, refusing up
front when tail events over the measured replay rate exceed its bound; a
snapshot trigger in time to replay, not events alone; the snapshot refusal
surfaced where someone reads it.


## The canonical witness (lane/restore-witness, 2026-11-14; red change)

Repair of the 23:02Z refusal (`rematerialize: reconstructed
computer_event_index does not match checkpoint`, http 500) and of review
round 1 of `lane/event-index-diff`. Evidence so far is code reading and
store-level tests; nothing is proven on d@e or staging until the landing loop
has run.

### What changed

- Witness version 4. A checkpoint records the event-index part of the witness
  in a canonical form: a null in `payload_pin_receipt_digests_json` is read as
  `[]`, and finalized `projection_batch_recorded` rows at or below the trim
  boundary of the checkpoint head are left out. The witness carries
  `event_index_trim_boundary`; the derivability digest, and so the checkpoint
  digest (which is over the whole request), commits to the version, the form
  name `canonical-v1` and the boundary. A version 3 witness is unchanged byte
  for byte and is compared strictly; a version 3 witness with a boundary, or a
  version relabelled either way, does not validate.
- The bind (`liveCheckpointWitness`), the restore gate
  (`reconstructedWitness`) and the replay-completeness probe (both sides) call
  one function, `canonicalSnapshotDigests`, over one extractor,
  `canonicalWitnessExtractor`, with the lane's `EventIndexWitnessFilter`,
  `ProjectionBatchTrimBoundary` (the trim's own function) and extractor
  options. The restore refuses a boundary that is not the trim boundary of
  the checkpoint's head sequence.
- Snapshot records stay strict over raw pages (`LiveStoreWitnessTx`,
  `VerifySnapshotFile`): a record carries no witness version and old
  snapshots must still verify. A snapshot root is not a checkpoint witness.
- A witness mismatch and an unjudgeable witness version answer 409 with the
  reason in the body (`ErrWitnessMismatch`, `ErrWitnessVersionRefused` in
  `rematerializeRefusalStatus`). The proxy copies the upstream status and
  body (read, `computer_lifecycle.go`); the CLI path was not run.
- A store whose index holds more than one `computer_id` is refused by the
  canonical extraction: the trim is per computer and the filter is not.

### Soundness of leaving those rows out

Nothing reads them: replay and restore read the host tape, and head and
continuity come from `computer_event_projection_heads` (`event_index_trim.go`).
The trim deletes exactly these rows on every live store, as a pure function of
the head sequence, so a store may legitimately have them or not at a given
moment; after a restore the staged store's own catch-up deletes any that
remain. Excluding them cannot hide a lost event: the event is on the tape.
The predicate keeps every row above the boundary and every other kind and
status (tests: an extra row above the boundary, a finalized batch row at
41,000, a prepared batch row at the boundary, an old desk row, a changed
column, a non-null pin list, a missing row). What it cannot see is a live
store that lost batch rows below the boundary which the tape says exist;
those are bookkeeping rows the trim deletes anyway.

### Red ceremony

- Conjecture delta: before, a checkpoint's witness was a function of the
  store's history (null versus `[]`, when a catch-up ran); after, it is a
  function of the tape up to the head. If wrong, a restore refuses a
  checkpoint it should accept (fails closed) or, if the exclusion is too
  wide, accepts a store that differs only in disposable rows.
- Protected surfaces: checkpoint and restore, the checkpoint digest and
  witness, the restore gate, the replay-completeness probe, the refusal status
  of restore.
- Evidence class: code reading, store-level tests that bind and restore, one
  full restore of a checkpoint of a store holding null pin lists, negative
  tests. Not yet staging or d@e.
- Rollback: a plain revert. A checkpoint bound in version 4 and kept: the
  reverted code refuses it. `RefuseWitnessVersion` there accepts only version
  3, and `CheckpointFromRequest` validates the witness through it on every
  bind, restore and route-projection path, so the restore fails with "witness
  version 4 is not judgeable" before anything is read. It is never misread as
  strict. Read from the old code (`witness_v3.go`, `restore.go`); not run
  against the old binary. Checkpoints bound before stay version 3 and restore
  on either code.
- Heresy delta: a witness that names its own form is a kind of switch. It is
  a version of a commitment, fixed by what the checkpoint says, not a setting
  and not off by default. The strict form stays only to read old checkpoints
  and can be deleted when none remain.

### Review round 1 of lane/event-index-diff, answered

- Blocking 1 and 2: the bind and the restore gate use the canonical form; a
  pre-change checkpoint is version 3 and is compared strictly, failing closed.
  Snapshot verification stays strict.
- Cold recovery (`autoputer/projection_base.go`) installs a snapshot, verified
  strictly, then replays the tail with no witness gate; with the old
  `Prepare` that wrote null for no-payload events, so it is the likely
  producer of null rows. Read from code.
- Diagnostics: the live row lists are read in the probe's snapshot, which
  cannot be after the digests are compared, because the replay does not exist
  yet. They are bounded instead (`ObjectGraphDiagnosticRowLimit` 200,000
  rows, `EventIndexDiagnosticRowLimit` 60,000; about 140 MB and 90 MB per
  side), and a store past the bound skips them and says so in
  `diagnostics_skipped`; the verdict does not depend on them. The trim keeps
  the index small, so a trimmed store is far below the bound; with millions of
  index rows the diagnostic is skipped.
- The replay side uses the replay head's boundary and the report carries both
  (`trim_boundary`, `replay_trim_boundary`). The one-computer invariant is
  checked.
- A probe at a non-zero boundary end to end is not tested: it needs a tape of
  more than 20,000 events. The wiring is tested at the one function the probe,
  the bind and the restore share, at head 50,000.
