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
- Rollback: a plain revert of the code, with the truth per order of
  rollback below. The short form: a version 4 checkpoint is never restored by
  code that does not know it; where the refusal happens, and what the
  operator reads, depends on which side rolled back.
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
  yet. They are bounded instead (`ObjectGraphDiagnosticRowLimit` 100,000
  rows, `EventIndexDiagnosticRowLimit` 30,000). The two-sided peak, when the
  result is not equivalent, is the live list, the replay list and the
  comparison maps: about 165 MB for the object graph and 105 MB for the
  index, about 270 MB together, on a guest whose ceiling is 1,024 MiB. A store
  past a bound skips that diagnostic and says so in `diagnostics_skipped`; the
  verdict does not depend on it. The live lists could be read lazily only if
  the snapshot stayed open across the replay (minutes), which pins the
  write-ahead log of a computer that keeps appending on a volume that may be
  short; I did not do that. The trim keeps the index small, so a trimmed store
  is far below the index bound; d@e's 122,000 objects are over the object
  bound, so its object graph rows are not diffed (a digest verdict is).
- The replay side uses the replay head's boundary and the report carries both
  (`trim_boundary`, `replay_trim_boundary`). The one-computer invariant is
  checked.
- The probe at a non-zero boundary is tested end to end on a 20,010-event
  tape (`TestProbeAtANonzeroTrimBoundary`): a null pin list on the live side
  only is equivalent, and the bound witness has the probed head's boundary and
  content root. A trim backlog row on the live side only, and a real
  difference above the boundary, are tested at the function the probe, bind
  and restore share, at head 50,000; they are not injected into the
  20,000-event tape (every sequence there is a tape event). Named residual,
  with the test to write if the probe's composition changes.

### Rollback by order (review round 2)

The version 4 witness JSON carries `witness_version: 4` and
`event_index_trim_boundary`. "Old" is the code before this change. Read from
the structs and decoders, not run against an old binary.

- **Full revert (guest and host old).** Checkpoints bound as version 4 stay in
  the host store. The old host serves one decoded into its struct, which keeps
  `witness_version: 4` and drops `event_index_trim_boundary` (unknown field).
  The old guest's `RestoreFromRequest` validates the witness first and refuses
  by name: `/lifecycle/restore` answers 400 `witness version 4 is not
  judgeable`; `/lifecycle/rematerialize-from-tape` answers 500 with the same
  text (the old status map does not know the refusal). Version 3 checkpoints
  restore as before.
- **Guest new, host old (a guest updated before the host, or the host rolled
  back first).** A new guest binds version 4 and publishes it. The old host's
  `CheckpointFromRequest` refuses it by name, so nothing is stored: the bind
  and the cadence checkpoint fail with `witness version 4 is not judgeable`,
  and no checkpoint is made on that guest until the host is new. Fail closed;
  a lost checkpoint, not a wrong one.
- **Host new, guest old.** The old guest binds version 3, which the new host
  accepts. It cannot restore a version 4 checkpoint a new guest published
  earlier: refused by name, as in the full revert.
- **Host-only automatic rollback mid-deploy, after new guests published
  version 4 to the new host.** The stored version 4 checkpoints are now served
  by the old host through its struct, which drops the boundary. A new guest
  restoring one recomputes the derivability digest with boundary 0 and gets
  `witness derivability digest does not match`: 400 on `/lifecycle/restore`
  and a 500 on rematerialize (new code does not map that validation text to a
  status). This is not "version 4 is not judgeable"; the operator is told the
  checkpoint is corrupt, and it is not: it is unrestorable until the host is
  new again. The old host also certifies a route projection for such a
  checkpoint, because it never validated the witness there; the new host
  refuses it (`RouteProjectionFromRequest` rebuilds the checkpoint). That
  certification is a route, not a restore, and the checkpoint still cannot be
  restored. Version 4 checkpoints published in a skew window are therefore
  refused (fail closed) until both sides are on the new code.
- **What this change does about it.** The host now serves a stored checkpoint
  verbatim (`StoredCheckpointResponse` returns the stored bytes, never a
  decoded and re-encoded copy), so a field a release does not know survives
  transit, and the next field added will not be lost this way. It cannot
  change the old host. The route projection rebuilds the checkpoint, so an
  unjudgeable version or a digest that does not follow from the request is
  refused as a route.
- **The `Prepare` nuance.** `Prepare` now writes `[]` where an old replay
  wrote `null`. For an ordinary store that fixes version 3 restores (live
  rows are `[]`; the old replay's `null` was the mismatch). For a store
  written entirely by an old replay (null in every no-payload row), a version
  3 checkpoint that restored on the old binary now refuses (fail closed); a
  new version 4 bind canonicalizes null and recovers. So "version 3
  checkpoints restore on either code" holds for stores whose live rows are
  `[]`, not for that class.

## Investigation for the replay-time trigger (lane/snapshot-by-replay-time; code, read-only)

Written before the build. Mutation class of the build: orange (runtime
behavior of the producer; the snapshot and its tape record are unchanged).

(i) How chunks are stored, shared and pruned.

- `internal/store/snapshot_to.go` is the capsule store copy
  (`SnapshotTo`, `:77`), not the snapshot capture. The capture is
  `internal/snapshot/snapshot.go`. It writes no copy of the store: one
  read-only transaction streams the pages of the WAL snapshot
  (`snapshot.go:532`).
- Chunks are content-addressed and shared across successive snapshots of one
  computer. The page stream is cut into fixed page ranges, 4 MiB
  (`ChunkSize`, `snapshot.go:66`; chunk i is pages `[i*N,(i+1)*N)`,
  `:19`). Each is sealed with `filecas.SealChunk`, deterministic per key and
  plaintext (`snapshot.go:441`), so an unchanged range has the same digest.
  The uploader HEADs every digest and PUTs only the ones the host does not
  hold (`snapshot.go:450-475`; stats `ChunksHeld`, `ChunksUploaded`,
  `BytesUploaded`). A new snapshot therefore costs the changed ranges, not a
  full copy. A range counts as changed if any page in it changed, so a
  store that rewrites pages all over (the paper) shares little: the comment
  at `snapshot_producer.go:56-60` records 20 GB a day in October and the
  paper's 11-13 record gives 4.8 GB of sealed chunks added by a 46.7 GB,
  37 min 40 s snapshot.
- Retention, host side: each snapshot registers a root (digest list)
  (`snapshot/http_chunks.go:100`), and `pruneSnapshotRoots`
  (`internal/platform/snapshot_roots.go:203`) keeps the newest 4 roots the
  tape names, 2 when the store is over 2048 chunks (8 GiB)
  (`snapshot_roots.go:42-49`). Chunks no live root names become collectable
  by the artifact sweep (`platform/artifact_gc.go:182`, after its grace).
  Restore considers at most the newest 4 snapshots (`snapshot.MaxCandidates`,
  `snapshot.go:896-899`), so on a large store a checkpoint older than the
  second newest snapshot has no base.
- Retention, guest side: nothing is kept locally. Only the
  `snapshot_recorded` event stays on the tape, and the producer removes a
  leftover VACUUM INTO copy (`snapshot_producer.go`, `snapshotLegacyTempDirName`).

(ii) What the replay rate depends on, and whether it is measured.

- The tail replay (`computerevent/appender.go:1396-1430`) pages events from
  the host in 1,024-event pages (fetch), then reduces, verifies receipts,
  projects rows and commits. The log line breaks the time into fetch,
  payload prefetch, reduce, verify, prepare, finalize and commit. So the rate
  depends on host fetch latency, payload fetches, the per-event projection
  (which grows with the store's indexes and the cold page cache) and the
  commit cadence. Measured: 206 to 380 events/s on d@e in October and early
  November, 113 events/s on 2026-11-14 (restore of 33,332 events in 4 m 54 s).
  The paper's replay rate has not been measured in the records.
- At runtime it is a log line only: `replay applied N events ... (X events/s`
  (`appender.go:1423`) is computed in a deferred function and kept nowhere.
  `ReplaySnapshot().AppliedRows` is zeroed per replay (`:1404`). No counter
  of the rate exists. The build adds one.

(iii) Cost of a capture as a function of store size.

- Time: the stream and the witness both read every page, so both scale with
  the store. The paper's 46.7 GB took 37 min 40 s; the witness alone was 22 to
  35 minutes on 21 GiB (`snapshot_producer.go:79-84`); d@e's 1 GiB took 11 s.
- Held: one read transaction on the read pool for the whole capture
  (`ReadTxTime`). Writers and appends do not wait, but the transaction pins
  the WAL, which grew 3.7 GB in 30 minutes under load (comment at
  `snapshot_producer.go:81-84`), so free space is the resource held; the
  producer refuses under 2 GiB (`snapshotFreeFloor`) and the capture checks
  it during the stream. Memory is a few 4 MiB buffers.
- Written: sealed bytes to the host for the changed ranges only
  (`BytesUploaded`), plus a probe per chunk, and the host keeps unreferenced
  chunks through its grace.

Consequence for the trigger: the cost of a capture (seconds on d@e, 37 minutes
and several GB on the paper) is known after the fact from `Stats`, so the
trigger can derive its own floor between captures from the last capture.

## Repair: snapshot due by replay time (lane/snapshot-by-replay-time)

The producer estimates a restore's replay as tail events over a replay rate
(this computer's last measured replay of at least 1,000 events, otherwise 113
events/s, d@e's slowest recent reading) and takes a snapshot when that exceeds
four minutes (27,120 events at 113/s): ten minutes of request, four of scan and
install, one for the witness and flip, one of margin. The 540,000-event ceiling
and the seven-day interval stay and fire regardless. Replay-time captures keep a
floor from the last capture's own cost: the larger of ten times its duration and
its sealed bytes spread over 8 GiB a day. d@e (11 s, a small store): minutes to
under an hour, so the bound decides (about every four hours at 1.8 events/s).
The paper (37 min 40 s, 4.8 GB, 3.3 events/s): 13.4 hours, so the floor decides,
about two captures a day against one every 45 hours by the ceiling, 4.6% of wall
time, and its restore tail stays above the bound; the producer logs
"snapshot floor-limited" and the observability block says `floor_limited`.
Observability `snapshot_producer` carries the state, including how long a due
snapshot has been refused. Not verified on a live computer.

### Review round 1 repairs (same lane)

- The floor survives a restart: the last capture's duration and sealed bytes
  are written beside the store (`snapshot-producer.json` on the data volume)
  and read back for the newest snapshot only if the snapshot event falls
  inside that capture's window. A snapshot with no matching record has an
  unknown floor, which binds replay-time captures until the 540,000-event
  ceiling or the interval takes one and measures it (`capture_known:false` in
  the block). Whether the data-volume directory survives a restore flip was
  not verified.
- Host retention: the newest k roots stay, and so does a chain beyond them
  covering the recovery bound (the newest root at least 720,000 events behind
  the newest root, plus roots at least 360,000 events apart above it), so a
  base exists for every checkpoint a restore accepts and the nearest base is
  inside the bound. This is more than "k plus one anchor": a single anchor
  cannot advance, because the newest k roots span hours, not the bound, and
  the root that will be the next anchor has been dropped before it is needed.
  At most three extra roots; on the paper about two (about 9.6 GB of unique
  chunks at 4.8 GB each). The rows are the collector's liveness, so their
  chunks outlive the 24 hour upload grace.
- The replay rate used is the slowest of 113 events/s and the slowest replay
  measured since the process started, boot and restore alike (the restore's
  dry-run copy reports to the live appender). It is process memory and
  re-learns downward after a restart.
- A replay-time capture also needs free space above the floor for the WAL it
  pins: 2 MiB/s of the last capture's duration (paper: about 4.4 GiB more).
- Still open, not in this lane: the restore's up-front refusal. For a
  floor-limited computer (the paper) the four-minute scan and install figure
  is d@e's on a 1 GiB store; it is not shown to hold for a 46.7 GB store, and
  the replay bound there is probably optimistic.

### Review round 2 repair: one selection (same lane)

Round 1's chain could not be reached: `snapshot.Scan` kept the newest four
`snapshot_recorded` events at or below the target from the tape, which knows
nothing of the roots the host pruned, so with frequent snapshots the four were
usually pruned rows and a live chain root a few events lower was never tried.

- The plan now offers every snapshot event on the tape at or below the target
  and not further than `MaxRecoveryTailEvents` behind it (`ScanAll`,
  `PlanCandidates`), newest first, at most 1,024. The install probes the host
  (the existing chunk HEAD route, `ChunkProber`) for the chunks of a candidate
  (the state file's and the telemetry partitions') before downloading any. As
  first landed (round 2) this was false of production: `platformRestoreTape`
  had no `HasChunk`, so the probe was skipped silently and a pruned candidate
  downloaded until its first missing chunk (round 3 found it). Now the tape
  forwards `HasChunk`, and a pruned root costs a record fetch and probes that
  stop at the first missing chunk (8 concurrent HEADs; at most one HEAD per
  chunk of the candidate, about 1,150 on the paper's 4.8 GB-per-capture store,
  about 11,400 for its whole file), then the next older one is tried. The
  failed candidates share a two-minute budget (`restoreCandidateBudget`), after
  which no further candidate is started, so a run of failures cannot starve the
  replay. The 1,024 cap only keeps the list finite: the base up to 360,000
  events below a checkpoint is reached within it iff the cadence in the window
  is at least 352 events, and the producer's fastest cadence is the tick bound
  (about 1,100 events at d@e's 1.8/s). A store that cannot probe logs that once. No new guest-host route: the
  guest had no way to ask the host for its live roots, and a list-roots route
  would have been a new one; the probe gives the same answer, the live roots
  at or below the target, from the route that exists.
- What a lying or stale host can and cannot cause. Candidates come only from
  the tape, and a record opens only under the event identity it was sealed
  for, its manifest root verifies, every chunk opens under the computer's key
  at its digest, and the unsealed file is checked against the record's witness
  before install. A host cannot hand the guest a root the tape does not
  record (tested: a host that holds the chunks and the sealed record of an
  unrecorded snapshot installs nothing), nor wrong state. It can deny: say
  "not held" for a held chunk (an older base, longer replay, or a refusal) or
  fail downloads (a refusal). An unreachable host or a probe error is "unknown"
  and the download decides; an empty answer leaves the tape's candidates, as
  before.
- A target whose only snapshots are older than the bound now refuses up front;
  before, the old plan installed that base and `PlanRecovery` then refused the
  tail over the bound, after the install.

Named residual, not fixed here: a restore from a chain root replays up to
360,000 events (the chain gap), about 53 minutes at 113 events/s, far over the
ten-minute request. The chain does not meet the restore budget. Old
checkpoints become restorable in principle with this change, and in practice
only when restore is an accepted act with a status (the next lane), which must
also refuse up front when its replay estimate exceeds its budget.

Open for the owner: the chain keeps up to three extra roots per computer, about
9.6 to 14.4 GB of unique chunks on the paper (4.8 GB each; not measured). The
host's free space on the paper belongs in the landing proof.

### Round 3 structural note

Twice the tests modelled production instead of running it. Round 2: the
retention tests passed because they asserted a base existed, not that the
guest's selection could reach it. Round 3: the probe tests passed because their
fakes implemented `HasChunk` and production's `platformRestoreTape` did not.
From now each restore-path claim in this lane has a test through the production
wiring: the production tape type (built by `newPlatformRestoreTape`, the one
constructor the runtime also uses), the real HTTP clients, a fake host that only
speaks HTTP, and a compile-time assertion that the production type is a
`ChunkProber`. The older `restoreTapeFake` has no `HasChunk`, as production did
not; tests that need the probe use the production type.

## After the snapshot producer deployed (2026-11-15T05:44Z)

- d@e: restore to a version 4 checkpoint answered 200 twice (04:10Z in 283 s
  on 304a81277; 05:43Z in 363 s on cc8ea803d), `witness_matched: true` both
  times; replay completeness after the first read `equivalent`.
- The paper is not restorable inside one request and the new producer does
  not change that yet. Its block at 05:36Z: 314,364 tail events, 46m22s
  estimated replay, `floor_limited`: the cost of the last capture is not
  recorded for its newest snapshot (2026-11-13T22:36Z), so a replay-time
  capture waits for the 540,000-event ceiling or the interval to take one and
  measure it. At about 7,400 events an hour the ceiling is roughly 30 hours
  away. Until then a paper restore needs the async restore act (lane 3) or a
  capture someone decides to take. Not acted on: the last paper capture took
  37 minutes with the WAL pinned, which is what the floor protects.

## Repair design: restore as an accepted act (lane/restore-act, 2026-11-15)

Recorded before the build. Mutation class: red (restore, checkpoint and route
projection, the vmctl busy reading). Nothing here is proven on a live computer.

### Why this and not a longer request

The paper cannot be restored: 314,364 tail events, 46m22s at 113 events a
second, and the producer is floor-limited for about 30 hours (above). By design
a tail reaches 540,000 events (about 80 minutes). Every request budget on the
path is ten minutes. Making the work independent of the request costs no
storage and lets snapshots stay rare, which is the point of the storage
mission. The restore engine is unchanged; what changes is who waits for it.

### The act

- A restore (and `rematerialize-from-tape`, the same engine) is an accepted act
  on the guest. `POST /lifecycle/restore` validates what it can without the
  tape, appends the restore intent to the tape (the existing
  `restore_requested` event, idempotent per checkpoint), writes the act record
  and answers 202 with the act id and a status URL. The work runs in the
  guest's background.
- States: `accepted`, `fetching_base` (plan the target, install a snapshot),
  `replaying`, `verifying` (witness, vocabulary fence), `installing` (restage
  the pinned release, flip), then `succeeded`, `failed` or `interrupted`.
- Where it is recorded, and by whom: the tape holds the intent (existing
  event, existing authority). The act's progress is a record on the data
  volume beside the store, `restore-acts/<act id>.json`, written atomically by
  the guest runtime, the same shape as the capsule job records
  (`internal/capsule/jobs.go`, `closeInterruptedJobs`). It is not in the store
  because the flip replaces the store: a record in it would be quarantined with
  the state it describes. It is not a new kind of tape event: the status is
  progress of one process, and the tape carries the fact that was asked. No
  parallel store: one small JSON file per act, the newest 20 kept.
- Auth: the status route is `GET /lifecycle/restore-status[?act=ID]`. The
  proxy gates it as restore is gated: the owner act (`owner:acts`), the
  computer's lifecycle scope and ownership. The guest route itself checks only
  the computer binding (as every guest route does); `product_api` refuses the
  path so a desk cannot read it. The result carries filesystem paths and
  witness detail an agent has no use for.
- One at a time: a second request while an act runs answers 409 naming the
  running act. The same idempotency key (default: the checkpoint digest, the
  key the intent already uses) returns the running act.
- The acceptance answer carries the estimate: tail events and seconds, from
  the replay rate measured on this computer when one is recorded (appender
  `ReplayRate`), else the assumed 113 events a second, and says which. The tail
  is read from the local event index (target and newest snapshot at or below
  it); when the index does not know the target, the answer says the tail is
  unknown until the plan. No refusal for being long. The roots the host still
  holds are probed in `fetching_base`, before any download; a refusal there is
  a `failed` act naming it, not a 202 that promised more (the scan is a pass
  over the tape and is not bounded by a request).
- Correctness refusals stay: witness version and mismatch, not derivable, base
  refused, unavailable. One is added on the owner's brief, a disk floor: the
  live store's size plus the 2 GiB snapshot floor must be free, because the
  staged store grows beside the live one.
- The status read returns: state, tail events total and applied, the rate
  (measured in this act once it has applied events, else the estimate's) and
  its source, estimated seconds remaining, accepted, started, updated and
  finished times, and on the end the report the synchronous route returned
  (`witness_matched` and the rest) or the refusal by name and the HTTP status
  the synchronous route would have used.
- The synchronous path is deleted. Callers named: `choir computer restore` and
  `choir computer rematerialize-from-tape` (both move to submit and poll), the
  proxy's ten-minute client for those two routes (deleted; the replay probe
  keeps its own), and the guest's ten-minute write deadline for them (deleted).
  `RematerializeFromTape` stays as the engine the act runs; it is not a route.

### Restart table (CLAUDE.md "Restarts")

| Event | What happens | Live state |
| --- | --- | --- |
| Crash or kill in `accepted`, `fetching_base`, `replaying`, `verifying` | The boot closes the act `interrupted` ("interrupted by a restart"), removes the staged store, never resumes | untouched: the restore writes only into the staging directory until the flip (the one write to the live computer before is the intent event, a tape record) |
| Crash in `installing`, before any rename | same | untouched, except `current` may name the restored release (below) |
| Crash in `installing`, mid-flip | the boot, before it opens the store, undoes the renames the act recorded, closes the act `interrupted` | the pre-restore state |
| Crash in `installing`, flip complete | the boot leaves the renamed realization and closes the act `interrupted`, saying the flip had finished | the restored state (not undone: that would be a second restore) |
| Planned restart | refused while an act runs: `restore_in_flight` is unsafe work whatever the host asks (`allow_busy` included), like a streaming snapshot | n/a |
| Planned restart after success (the exec changed) | requested after the act record says `succeeded` | restored |

An act never resumes after a restart; a restart is not a reason to start work
nobody asked for. The owner re-submits (the intent is idempotent).

### Problems found while reading, recorded with the fix

1. A crash in a restore strands its staging directory (`rematerialize-staging-*`)
   on the data volume for ever: nothing removes it at boot. On the paper that is
   up to the size of a store. The boot now removes it.
2. `RestagePinnedRelease` moves `current` to the restored release before the
   flip. A crash between leaves the live store with the restored release's SPA.
   The act records the prior release digest before the restage and the boot puts
   it back (best effort, as the in-process failure path already does).
3. The restore holds `currentReleaseMu` for its whole run. At 46 minutes that
   blocks every checkpoint bind (which reads `current` under it) for 46
   minutes. It is now held for the validation and for the restage-and-flip, not
   the replay. The prior release is read again under the lock before the restage.
4. Nothing told vmctl a computer was mid-restore. Its idle sweep, host stop and
   crash restart read running runs, self-development operations and live
   capsules; a restore is none of them. `/health` now reports `restore_in_flight`
   and each reading treats it as busy.
5. The replay probe (a checkpoint bind runs one every hour) and a restore both
   install a replay observer on the one appender; a probe finishing clears the
   restore's, and the restore then fails its exactly-once tail receipt. The
   restore now observes through its own dry-run copy and never touches the
   appender's.

### Residuals named before the build

- The replay-completeness probe has the same ten-minute shape. It shares the
  scratch and observer machinery but its result is a large report; it is not
  converted here (named residual unless the machinery proves small).
- A deploy that is not a planned refresh (a host reboot, an OOM kill, a vmctl
  kill) interrupts the act by design; the table above is its answer.
- Estimates assume the best base. A pruned root costs a longer tail than the
  estimate says; `tail_events_total` becomes exact at the plan.

### Failure list (each has a test; written before the code, seen failing)

Guest, through the real router (`NewAPIHandler(rt).HandleComputersRouter`):

1. F1 Restore answers before the work ends: 202, an act id, a status URL, state
   `accepted`, an estimate. `TestRestoreActAnswersAtOnceWhileTheReplayRuns`
2. F2 Status shows progress while replaying: state, tail total and applied,
   rate, estimated seconds remaining. `TestRestoreActStatusReportsProgress`
3. F3 On success the status carries the report the synchronous route returned
   and the store is the restored one. `TestRestoreActSucceedsWithTheReport`
4. F4 A refusal in the background (witness mismatch) ends `failed` naming the
   refusal and its HTTP status; the live store and the served release are
   untouched. `TestRestoreActFailsByNameAndLeavesLiveState`
5. F5 One at a time: another key answers 409 naming the running act; the same
   key returns that act, appends no second intent and starts no second run.
   `TestRestoreActOneAtATime`
6. F6 The estimate uses the measured rate when one is recorded, else the
   assumed one, and says which; a long tail is not refused.
   `TestRestoreActEstimateNamesItsRate`
7. F7 A disk below the floor refuses up front, before the intent is appended.
   `TestRestoreActRefusesBelowDiskFloorBeforeTheIntent`
8. F8 The refusals that were up front stay up front and append no intent
   (bad scope 400, unjudgeable witness 409). `TestRestoreActKeepsUpFrontRefusals`
9. F9 A kill mid-replay (a real SIGKILL of a child process) leaves the live
   store and the served release as they were; the boot closes the act
   `interrupted` and does not resume; the staged store is gone.
   `TestRestoreKilledMidReplayLeavesTheLiveStateUntouched`
10. F10 A crash mid-flip is undone by the boot; a completed flip is left and
    the act says so. `TestBootUndoesAHalfFlipAndLeavesACompleteOne`
11. F11 A crash after the release was restaged and before the flip puts the
    prior release back at boot. `TestBootPutsBackThePriorRelease`
12. F12 A planned restart is refused mid-restore whatever `allow_busy` says,
    and `/health` reports `restore_in_flight`.
    `TestPlannedRestartIsRefusedMidRestore`, `TestHealthReportsRestoreInFlight`
13. F13 `currentReleaseMu` is free during the replay.
    `TestRestoreActDoesNotHoldTheReleaseLockDuringTheReplay`
14. F14 A replay probe that runs during a restore does not touch its
    observation. `TestReplayProbeDuringARestoreDoesNotTouchItsObservation`
15. F15 Act records: unknown id 404, newest 20 kept, a record of another
    computer is not served. `TestRestoreActRecords`
16. F16 No production capability is missing from the test wiring: the act runs
    on `resolveRestoreTape` and the production `ReplayObserver`.
    `TestRestoreActWiring`

Host (proxy and vmctl):

17. F17 The proxy forwards restore and rematerialize on the ordinary client and
    answers the guest's 202 unchanged; `restore-status` is a GET gated as
    restore is (owner act, lifecycle scope, ownership).
    `TestRestoreRoutesUseTheOrdinaryClientAnd202`, `TestRestoreStatusRouteIsAnOwnerAct`
18. F18 vmctl's idle reading, host-stop reading and crash-restart refusal treat
    `restore_in_flight` as busy; the deploy refusal names it.
    `TestRestoreInFlightIsBusyForVmctl`

CLI:

19. F19 `computer restore` submits, polls, prints progress to stderr and the
    result to stdout (exit 0), a failed or interrupted act exits 1, `--no-wait`
    prints the act id, `restore-status` reads one, a poll that meets a 502
    while the guest restarts keeps polling to the deadline.
    `TestComputerRestoreWaits`, `TestComputerRestoreNoWait`,
    `TestComputerRestoreStatus`, `TestComputerRestoreFailedExitsOne`,
    `TestComputerRestoreToleratesATransientPollError`

### As built (lane/restore-act)

Not run on a live computer. What is proven is in the tests named above; the
deployed proof is a list of commands for d@e (below), run after landing.

- Guest: `internal/agentcore/restore_act.go` (the act, its record, the boot
  repair), `rematerialize.go` (the engine reports its state and progress to the
  act through the context; the two routes accept), `restore_snapshot.go` (the
  observer's per-event callback). Host: the proxy forwards the two POSTs on the
  ordinary client and the new `GET .../lifecycle/restore-status` as an owner
  act; vmctl reads `restore_in_flight` from `/health` in its idle sweep, host
  stop and automatic restart, and names it in a deploy refusal. CLI:
  `computer restore` and `computer rematerialize-from-tape` wait by default,
  `--no-wait` returns the act, `computer restore-status` reads one.
- Both routes are acts, not only restore: `rematerialize-from-tape` is the
  same engine and had the same ten-minute shape. The synchronous routes are
  deleted. The CLI still prints a plain answer that carries no act id as the
  result, which is what a guest not yet refreshed onto this release sends: that
  is the client tolerating an old server, not a synchronous path kept.
- The replay observer moved off the shared appender
  (`ReconstructThroughTargetObserved`): a replay probe's `SetReplayObserver(nil)`
  used to clear the restore's observer mid-run, and the exactly-once receipt
  then refused the restore. A checkpoint bind runs a probe every hour.
- A flip that fails after the live store is closed (the quarantine directory
  cannot be made, or a rename fails) now reopens the live store on the
  original realization. It used to return with the store closed, answering
  nothing until the next restart. Read from code; not observed.
- The release lock (`currentReleaseMu`) is held for the check at the start and
  for the restage-and-flip, not for the replay. The prior release is read again
  under it before the restage.
- The CLI's wait is `--timeout` when it is given (or `$CHOIR_TIMEOUT`), else two
  hours; a poll is a small read with its own 60 second bound. `--interval` is
  the poll (default 5 s, floor 100 ms).

#### Rollback

A plain revert. The truth by order:

- Guest reverted, host and CLI new: the CLI reads a 200 with no act id as the
  result, as before. The status route does not exist on the old guest (404).
- CLI old, guest new: an old CLI sends the restore, receives the 202 body and
  prints it as the result with exit 0. It looks like a success and is not: the
  restore is running. Use a CLI from this release or read the status route.
- A restore act in flight at the revert is interrupted by the restart that
  deploys the revert (the planned refresh is refused while it runs; a forced
  one closes it at the next boot only on a guest that has this code). An old
  guest booting after such a kill finds a staged store it never removes: remove
  `rematerialize-staging-*` beside the store by hand, and put `current` back if
  it names the restored release (`restore-acts/<id>.json` records the prior
  digest).
- The act records (`restore-acts/`) are ignored by old code.

#### Residuals

- The replay-completeness probe has the same ten-minute shape and is not
  converted: its answer is a large diagnostic report, the proxy keeps its
  budget for it, and the accepted-act machinery here is shaped around a result
  of one fixed report and a one-writer slot.
- The roots the host still holds are probed in `fetching_base`, before any
  download; a refusal there is a `failed` act, not a refused request. The scan is
  a pass over the tape and is not bounded by a request.
- The estimate is the best case (the newest snapshot at or below the target).
  A snapshot the host pruned means a longer tail; `tail_events_total` is exact
  once the plan has run, and the act's own measured rate replaces the assumed
  one as soon as an event has applied.
- A desk can read the status through `product_api` (it is a GET; the guest does
  not distinguish a desk from the owner's proxy). It carries quarantine paths
  and witness detail, not secrets.
- A new guest under an old vmctl keeps itself safe from a planned restart (the
  guest refuses) but not from the idle sweep or host stop, which need vmctl on
  this release.
- The disk floor is the live store plus 2 GiB. On the paper that is about 49 GB
  free; whether the paper's volume has it is not known. A refusal says the
  numbers.
- The act record's progress is persisted every five seconds; a crash shows the
  last of them, not the exact count.

## Review round 1 repair: the flip as a durable protocol (lane/restore-act, 2026-11-15)

Both seats said REPAIR on the same root: the flip's state was inferred at boot
from which files exist, against a record that was neither written durably nor
checked. The inference failed three ways (a record lost to ENOSPC or a power
cut with the renames surviving; a completed flip undone because the reopened
store recreated `-wal` at the original path; a corrupt record skipped and the
staging deleted), and a post-flip failure reported `failed` over a restored
store. One mechanism, redone before any patch.

### The protocol

The act's record carries `install_phase`, a durable fact. The install facts
(not the progress ticks) are written file fsync, atomic rename, directory
fsync, and a failed write stops the install where it is.

1. `prepared`: before anything live is touched. Holds the prior and restored
   release digests, the quarantine directory and the full list of renames.
   A failed write fails the restore with nothing touched.
2. The release is restaged to the checkpoint's (the floor check can refuse here,
   with nothing flipped), then `release_restaged` is written.
3. The store is closed, the quarantine directory made, `flip_started` written;
   a failed write reopens the store, puts the prior release back, fails.
4. The renames. A failed rename undoes itself, reopens, puts the release back.
5. `flip_completed`, written and synced right after the last rename, before
   the reopen. A failed write undoes the renames in the process and fails; if
   that undo fails too the act says `unresolved` and the next boot (which still
   sees `flip_started`) undoes it.
6. Reopen, rebind, selfdev store. A failure here is `restored`, not `failed`:
   the computer is on the restored store and the restored release, and says a
   restart is required. The release is never put back over a flipped store.
7. The restart marker is requested, then `succeeded` is written.

Boot decides from the fact alone, never from which files exist:

- before `flip_completed` (`prepared` to `flip_started`): undo. Every recorded
  rename is undone last first, each only when its destination exists and its
  source does not; `current` is put back on the prior release if it moved. The
  computer is on exactly its pre-restore state. The act ends `interrupted`
  with `live_state: pre_restore`.
- `flip_completed` and later: roll forward. Nothing is undone; `current` is
  made the restored release if it is not. The act ends `restored` with
  `live_state: restored` ("restored; the process ended before the result was
  recorded"). The store and the release are always the same side.

Why roll forward is not a boot-time resumer (CLAUDE.md "Restarts": a crash
restart never resumes work; a planned restart may resume once, with a marker):
what is resumed is nothing. No replay, no fetch, no staging is read; by
`flip_completed` the work is finished and durably so, and the boot makes at most
one pointer swap that step 2 already made. The alternative, discarding the
restored state, would rename a store that has been serving (the reopened store
may already have taken writes, and the quarantine holds the old one), which
throws away the owner's 46 minutes and the writes after the flip for the sake of
a rule about work that is not being done. I chose roll forward. The repair is
idempotent (every step checks before it moves) and is tested killed between its
steps.

Closed-loud rules: a record in `restore-acts/` that cannot be read, or whose
paths are not under the data volume, is not touched and is reported
`unresolved`; while any record is unresolved and a staging or quarantine
directory exists, nothing is deleted, the computer refuses new restore acts
(409, `restore_unresolved`) and `/health` names the unresolved act. A boot with
an open act whose install ended before `prepared` removes its staged store as
before.

### Beside other work

- Acceptance refuses (409, `blocked_by_live_work`) while a self-development
  operation is mid-transaction or an engineering capsule is live.
- A restore in flight blocks a snapshot capture (the producer's busy check is
  the planned-restart count, which includes it) and a clean rebase; apply and
  capsule admission are not changed. Instead the restore re-checks under the
  release lock immediately before `prepared` and fails untouched
  (`blocked_by_live_work`) if one is live, and at boot a compaction rebase staged
  before a restore is refused after it (read from `store/compact.go`).
- Events appended during the replay, read from the code: they are on the tape
  and in the live store; the flip replaces that store with the state at the
  checkpoint's head, which does not hold them. The appender then finds its local
  head behind the host's and catches up in place on the next append
  (`catchup.go`), and a boot replays to the canonical head; both apply the events
  after the target, including `restore_requested`. So nothing is lost, and the
  restored state does not stay: this is the open problem recorded 2026-10-30
  (`restore-is-neither-forward-nor-durable-2026-10-30.md`), which this lane does
  not fix and now matters more, because the window is long. Not observed; the
  deployed proof below reads the head after the restore, after one write and
  after a restart.

### The crash table

| Crash point | `install_phase` | On disk | Boot does | Test |
| --- | --- | --- | --- | --- |
| during the replay | none | staging; live untouched | close `interrupted`, remove staging | `TestRestoreKilledMidReplayLeavesTheLiveStateUntouched` |
| after `prepared` | prepared | live untouched | close `interrupted` (pre_restore), remove staging | `TestRestoreCrashPoints/after_prepared` |
| after `release_restaged` | release_restaged | `current` on the restored release | put the release back, close `interrupted` | `.../after_release_restaged` |
| after `flip_started` | flip_started | store closed, nothing renamed | undo (nothing), close | `.../after_flip_started` |
| mid flip | flip_started | some renames made | undo them, put the release back | `.../mid_flip` |
| after the last rename, before `flip_completed` | flip_started | all renames made | undo them (the fact says not done) | `.../before_flip_completed` |
| after `flip_completed` | flip_completed | restored store, restored release | roll forward, close `restored` | `.../after_flip_completed` |
| after reopen, a `-wal` at the original path | flip_completed | the same plus `state.db-wal` | roll forward, never undo | `.../after_reopen` |
| repair killed between steps | any | partly undone | the next run finishes | `TestRepairIsIdempotentAndSafeWhenInterrupted` |
| unreadable record, staging present | unknown | everything | touch nothing, `unresolved`, refuse acts | `TestRepairRefusesWhatItCannotRead` |
| record writes fail (ENOSPC) | n/a | live untouched | n/a | `TestRestoreInstallRecordWriteFailureTouchesNothing` |

### Failure list additions

20. A failed install-record write touches nothing; each phase write has a test.
21. The crash table, through the real engine flip, a real SIGKILL at each point.
22. The repair twice and killed between its steps.
23. An unreadable record or an out-of-volume path is loud and deletes nothing.
24. `failed` only on the pre-restore state; `restored` after a failed reopen; the
    CLI's pre-restore sentence only when `live_state` proves it.
25. Acceptance and the pre-prepared re-check refuse live apply or capsule work.
26. The fixture drains a running act (the second act of `TestRestoreActOneAtATime`
    leaked and failed the cleanup).
27. A desk's product_api cannot read the status route.
28. The tail estimate from the local index (written after its function, found by
    review; `internal/store/restore_tail_estimate_test.go`).

### Round 1 repair as built, and the answers to the review's lists

- The install protocol is `install_phase` in the act's record (`restore_act.go`
  `install`, `rematerialize.go`). Order as built: `prepared`, then the release
  is restaged (its floor check can refuse with nothing flipped), then
  `release_restaged`, `flip_started`, the renames, `flip_completed`. The release
  is restaged before the flip, not after, so a floor refusal leaves nothing
  changed; boot makes the store and the release agree from the fact in either
  direction, so the order costs nothing.
- Boot undoes only at `flip_started`; before it the record's rename list is a
  plan and nothing moved. A step was made when its source is gone and its
  destination is there; a source still present means not made, whatever the
  destination holds (the live store's `-wal` and `-shm` are present before the
  flip, which the first version of the repair read as a conflict).
- An unreadable record is `unresolved` only when a staging or quarantine
  directory exists to protect; with none, boot logs it.
- Events appended during the replay: see the note above; the restored state is
  not forward-durable, the open problem of 2026-10-30. Not changed here.
- An ineligible computer: up-front validation does not read replay eligibility
  (that judgement, `replay_eligibility.go`, belongs to the probe that runs at a
  checkpoint bind). A checkpoint exists only for a state the probe judged
  equivalent, so a computer that never was eligible has no checkpoint to submit;
  one that stopped being eligible after its checkpoint gets a `failed` act,
  `witness_mismatch`, named, after the replay.
- Should-fix and nits, one line each. Act deadline: declined, a replay legitimately
  runs 80 minutes and a stall is the owner's `choir computer restart`, named as a
  residual (a stalled tape fetch holds `restore_in_flight`). `succeeded` before the
  marker: fixed, the marker is requested first and a race rolls forward at boot.
  Status authority: `product_api` refuses the route, the guest route relies on the
  proxy's owner-act gate and the in-guest binding. Test after the function:
  recorded as failure item 28. Both-exist steps: now "not made", with the reason
  above. Version skew: a field an old CLI rejects was declined (it would break the
  accepted answer for every client); the rollout order is vmctl and proxy with the
  platform release, guests on refresh, CLI from the same release, and an old vmctl
  does not read `restore_in_flight`. `restored_release_digest` in the old boot test:
  the test was replaced and the field is now real. `restoreActStatus`'s running
  branch not filtering by computer: one runtime per guest, left. `pruneLocked`
  string sort: harmless, left. `dryRunCopy` reading `replayObserver` unlocked:
  pre-existing, left. A 200 with a `state` and no `act_id` printing as a result:
  no route produces one, left. The 2 h default wait: `--timeout` exists, left.
  Callers of the deleted synchronous path: `scripts/reeng_tracer.mjs` was one and
  now reads the act.

## Review round 2 repair: one repair routine (lane/restore-act, 2026-11-15)

Both seats said REPAIR again; every round-1 blocker was closed and the new
defects were all in the gaps between two repair implementations that disagreed:
the in-process failure paths (which undid the renames and deleted the staging
and quarantine directories) and the boot repair (which read the record). A kill
between the in-process deletion and the in-process record write left boot
unable to repair and `current` on the wrong release; a terminal record with an
unresolved live state was never repaired; the undo inferred "step made" from
which files exist. The frame changes, not the patches.

### One routine, driven by the durable record

`undoInstall(updaterRoot, facts)` is the only code that undoes an install. Its
input is the install facts the record holds (phase, the renames, the prior
release); the boot calls it with the record it read, and the engine's failure
path calls it with the same facts it has just written. It is idempotent, safe
to kill between any two steps, and it deletes nothing:

1. At `flip_started` only: undo the renames, last first. A step was made when
   its source is gone and its destination is there; the list holds only the
   renames whose source existed when `flip_started` was written, so a step
   whose source never existed is not in it and a sidecar the live store left is
   never moved by inference.
2. Sync the directories the renames touched.
3. From `release_restaged` on: make `current` the prior release again, and
   check it is. A release that cannot be put back, or an empty prior digest, is
   an error, and an error is `unresolved`.

Everything else follows from who calls what:

- The engine, on any failure, returns after calling `undoInstall` and
  reopening the live store if it closed it. It deletes nothing: no staging, no
  quarantine. A panic is a failure (recovered in the engine).
- The one place that removes the staged store and the empty quarantine is the
  end of an act, after the outcome is durably recorded (`finish`), and the boot
  after it has closed what it found. A kill anywhere before that leaves the
  directories for the boot, whose repair is the same routine.
- An act is never written terminal while its live state is unresolved. The
  record stays open (`installing`, `live_state: unresolved`), so the boot repairs
  it and, with no boot needed, acceptance refuses: acceptance reads the records
  (any open record, any unreadable record beside a staging or quarantine
  directory) and the boot's list, not the list alone. `/health` reports the
  same set.
- Roll forward at `flip_completed` is bookkeeping only: it closes the record
  `restored` and does not swap `current` (the release was restaged durably
  before the flip), which settles the resumer question: it moves nothing.
- If the `flip_completed` write fails, the engine first writes a corrective
  `flip_started` fact. If that succeeds it undoes through the routine. If it
  fails too (a double write failure) it does not undo: the files stay flipped, the
  store stays closed, the act stays open and unresolved, and whichever fact the
  disk holds agrees with the files (`flip_started` is undone at boot,
  `flip_completed` rolled forward).
- A flip's renames are made durable before `flip_completed` is: the data
  directory, the quarantine and the staging directories are synced after the
  last rename and before the fact, and after the undo before the close record.
  A directory sync that fails is a failed install, handled as above.
- The first `accepted` record is written durably too (a torn first record read
  as unreadable, which with staging present refuses restores).
- A `restored` act (reopen failed) requests the restart as a success does and
  sets the restaged flag, so a migration cannot move `current` under the
  restored store.
- A success says, in its reason, that the restored state is not forward-durable
  (the 2026-10-30 problem), so no act reads clean over a state the next
  catch-up changes.

### The crash table, additions

| Crash or fault | On disk | Boot (or the same routine in-process) | Test |
| --- | --- | --- | --- |
| kill after the in-process undo, before the record | renames undone, staging present, `current` restored release, record `flip_started` | undo finds nothing to do, puts the release back, closes `interrupted`, removes staging | `TestRestoreCrashPoints/after_undo` |
| panic between `flip_started` and the renames | nothing moved | the engine undoes (nothing), reopens, act `failed` on the pre-restore state, no deletion by the engine | `TestRestoreEnginePanicsAreSettledByTheRoutine/before_renames` |
| panic after the renames | all renames made | the engine undoes them, reopens, `failed` pre_restore | `.../after_renames` |
| live `-wal` present when `flip_started` is written, kill before any rename | sidecar at the original path, staged sidecar never existed | the live sidecar is left where it is | `TestRestoreCrashPoints/live_wal_at_flip_started` |
| `flip_completed` write fails, corrective write fails (double failure) | files flipped, store closed, record open | act open and unresolved, acceptance refuses; the next boot undoes from the fact on disk | `TestRestoreDoubleWriteFailureIsUnresolvedNotTerminal` |
| prior release cannot be put back | store undone, `current` on the restored release | `unresolved`, record open | `TestRepairIsUnresolvedWhenThePriorReleaseCannotBePutBack` |
| unresolved act, no boot since | open record | a new restore is refused from the record | same test |
| reopen failed, then boot | restored store, restored release | `restored` stays, nothing moved | `TestRestoreReopenFailureIsRestoredNotFailed` (extended) |
| first record unwritable | nothing | acceptance refused | `TestRestoreAcceptRefusedWhenItsFirstRecordCannotBeWritten` |
| directory syncs | n/a | renames synced before `flip_completed` | `TestRestoreSyncsTheFlipsDirectoriesBeforeItsFact` |

### The runbook for an unresolved act

`/health` lists `restore_unresolved`, and `choir computer restore-status` shows
an act `installing` with `live_state: unresolved`. The computer refuses new
restore acts. Nothing is deleted while any act is unresolved.

1. Read the act's record (`restore-acts/<id>.json` beside the store) and note
   `install_phase`, `flip` (the renames) and the two release digests.
2. Look at the data volume: `rematerialize-staging-*` (the staged store),
   `rematerialize-quarantine-*` (the pre-restore store, or empty), and the
   store at its normal path.
3. If the record says `flip_started`: the pre-restore store is wanted. For each
   rename in `flip`, last first, where the source is missing and the
   destination exists, move the destination back to the source. Make `current`
   the release in `prior_release_digest` (the updater's restage, or re-point
   `current` by hand).
4. If the record says `flip_completed`: the restored store is wanted. Leave
   the files; `current` must be `restored_release_digest`.
5. Set the record's `state` to `interrupted` (undone) or `restored` (kept),
   `live_state` to match, and give it a `finished_at`; or delete the record.
6. Restart the computer. The boot repair finds nothing open, removes the staged
   store and empty quarantine, and restore acts are accepted again.

If step 3 or 4 cannot be done, the quarantine holds the pre-restore store and
the staging directory the restored one: neither is deleted by anything.

### Round 2 should-fix and nit answers

- Roll forward's unconditional release swap: deleted (bookkeeping only).
- Directory fsync around the renames and after the undo: done; failure is a
  failed install.
- Corrective record before an undo after a failed `flip_completed` write: done,
  with the double failure left open (above).
- First progress record unsynced: the `accepted` record is durable; later
  progress ticks stay plain writes (a torn tick leaves the older record).
- `ensureReleaseOf` silent: returns an error and verifies; an empty prior
  release is an error.
- Boot proceeds to open a mixed store when unresolved: declined. The guest still
  refuses restore acts and `/health` reports it; refusing to serve would turn a
  rare unresolved restore into an outage, and an absent state file is the
  existing lost-disk boot.
- Admission-side gate for apply and capsule: declined and named. An apply or
  capsule admitted between the pre-flip check and the store close fails (the
  close waits 10 s for the writer); it does not corrupt. The restore's replay
  time is lost in that case.
- A restore leaves a store-sized quarantine each time (pre-existing, nothing
  prunes it): the disk floor does not account for it. A restore is a rare owner
  act; pruning is left to the storage mission.
- Two readers of `restore-acts/`: the strict one decides repair and acceptance,
  the lenient one serves status; left.
- `restoreInstallPhaseKnown("")` for a pre-protocol record: unreachable, no
  deployed release wrote one; commented.
- The `mid_flip` branch in the engine is a nil-hook call; left. The seam
  variables are plain: run under `-race`, below.
- `repairRestoreAct`'s `stopped` return skips the rest on purpose (a kill);
  commented.

### Failure list additions

29. A kill after the in-process undo and before the record leaves the boot able
    to finish (one routine, nothing deleted by the engine).
30. A terminal record never holds an unresolved live state; acceptance checks
    records.
31. A sidecar the live store left is never moved by inference.
32. The renames' directories are synced before the fact.
33. A double write failure leaves the act open, unresolved, refused.
