# A store snapshot without the scratch copy (2026-11-20)

Class of this file: green (a design; nothing changes until the landings
below). Problem: [the paper's data image grows by the store's size at every
capture](../problems/paper-data-image-grew-40-gib-in-eleven-minutes-2026-11-16.md).
Code read at `origin/main` 7f5ad41e1. Every claim about code cites the file
and line it was read at. A number marked *(not measured)* is not a fact.

## 1. The decision in five lines

1. The capture stops writing the store into a scratch file. It reads the
   pages from its one read transaction and hands them straight to the
   uploader, as it did before 842e0fda6 (2026-11-15).
2. The witness runs inside that same read transaction, before the pages,
   so the head, the witness and the pages are one state by construction.
3. The cost moves from a store-sized file (48 GiB, held 83 minutes) to the
   WAL that the open reader pins: about 2 MiB/s for the pin, so about 8 GiB
   for a 70-minute pin at the planning rate.
4. "Copy only the changed pages" is rejected. The witness needs a whole,
   frozen, queryable image, and only three things give one: a full copy, a
   pinned WAL, or a filesystem snapshot. Changed pages alone are none of
   them.
5. Three landings: the capture (red), the free-space rule (red), and the
   removal of the dead copy code (orange).

## 2. What the thing is

A snapshot is a cache of replay. It bounds how far a rebuild must replay
the tape (`docs/design/snapshots-cost-the-change-2026-11-13.md`, section 1).
A restore fetches the chunks, assembles the file, recomputes the witness on
it and compares it with the recorded one (`VerifySnapshotFile`,
`internal/agentcore/restore_snapshot.go:203`). So a snapshot must be three
things of one state: the head (where replay resumes), the pages, and the
witness.

### What the code does today

- The producer marks the capture in flight for its whole run
  (`internal/agentcore/snapshot_producer.go:599`), and a planned restart
  refuses while it is (`internal/agentcore/planned_restart.go:567`). vmctl
  reads that refusal as `snapshot_in_flight` and defers the refresh
  (`internal/vmctl/owner_update.go:175-192`). This is why the deploy of
  2026-11-19 reached the paper 28 minutes late.
- The producer refuses a capture unless the data volume has the store's
  size plus 2 GiB free (`snapshot_producer.go:576`, floor at `:98`).
- `capture` opens one read-only transaction (`internal/snapshot/snapshot.go:621`)
  and reads the head as its first statement (`:660`). That fixes the WAL
  snapshot.
- It copies every page through `sqlite_dbpage` into
  `snapshot-tmp/state.db` on the data volume (`copyPages`, called at `:683`,
  query at `:810`), then ends the transaction (`:685`). The pin on the WAL
  lasts the copy only.
- It opens the copy, ties it to the head (`crossCheckCopy`, `:882`), runs the
  witness on it (`:710`), drops its cache and reads the whole copy again
  from disk to upload it (`streamScratch`, `:723`). Only chunks the host
  does not hold are uploaded (HEAD probe, `:243-247`).
- The scratch is removed after the upload (`:736`) and on every error path
  (`:614`). Boot removes a scratch a crash left before anything else needs
  the space (`internal/autoputer/run.go:211-213`).
- The guest's hourly `choir-data-fstrim` (`nix/autoputer-vm.nix:1002-1021`)
  returns the freed blocks to the host by hole-punching the image
  (`internal/vmmanager/data_discard.go:16-17`). That is why the image steps
  up at the copy and back within an hour or two.
- The telemetry partitions are already streamed with no copy, each from its
  own read transaction (`streamFile`, `snapshot.go:1051`, from
  `capturePartition` at `:748`). `streamFile` is the unwired replacement for
  the state file: it is the function the state file used before 842e0fda6
  (`git show 842e0fda6^:internal/snapshot/snapshot.go`, witness in the
  transaction at `:571`, `streamFile` at `:577`).

### Why the copy was added, and why that no longer holds

842e0fda6 moved the witness off the live store so the pin on the WAL would
last minutes, not the 26 to 37 minutes of witness plus stream
(`docs/design/capture-static-copy-2026-11-15.md`, section 1.1;
`docs/problems/restore-bases-design-three-review-rounds-structural-assessment-2026-11-15.md`,
item 2). At the time the store was about 22 GB and the scratch was taken to
be short-lived. The record shows it is not: the scratch lives for the
witness and the upload, which is most of the capture.

## 3. Physical bounds

| Quantity | Value | Source |
|---|---|---|
| State file at the last capture | 51,391,889,408 bytes (47.9 GiB) | console, 2026-11-20 01:25Z, in the record |
| Whole capture | 1 h 22 min 51 s | same line |
| Copy (the pin today) | about 12 min (image grew 00:03Z to 00:15Z) and about 5 min on 11-16 (18:43Z to 18:48:27Z) | record |
| Witness plus upload | about 71 min (11-20) and about 48 min (11-16) | record: copy end to root time |
| Witness alone, 22 GB store, 11-12 | 22 min 37 s (22 to 35 min seen) | `snapshots-cost-the-change` section 2 |
| Stream with upload, 44 GB of pages, 11-12 | 3 min 02 s | same |
| WAL growth while pinned, 11-12 | 3.7 GB in 30 min (about 2 MiB/s) | same; the producer's planning rate |
| Paper guest block writes now | 0.89 MB/s (160 MB in 180 s, `write_bytes` of its firecracker, 06:25Z to 06:28Z on 11-20) | read on Node B for this design; just after a boot, so a low reading |
| Store growth | 41.76 GB (11-16) to 51.39 GB (11-20), about 2.4 GB a day | record |
| Time budget of a capture | 90 min | `snapshot_producer.go:84` |
| Captures | seven in four days, about one every 13 h | record |

What must be consistent: the head, the pages and the witness must be of
one WAL snapshot. Nothing else in the record is tied to that instant (the
partitions are read later on purpose, `snapshot.go:743-747`).

What these numbers say:

- The copy is not the slow part. The witness is. A frozen image is needed
  for as long as the witness runs.
- Holding a frozen image costs either the store's size (a copy) or the
  writes made while it is held (a pinned WAL, or a filesystem snapshot).
  The second is smaller unless the paper writes faster than the store's
  size divided by the hold: 48 GiB over 70 minutes is about 12 MiB/s. The
  planning rate is 2 MiB/s and the reading today is under 1 MB/s. The
  margin is six times at the planning rate.
- The pin under this design is the witness plus the stream: about 71
  minutes now (the record's witness plus upload; the stream and upload
  overlap, as they did on 11-12). At 2 MiB/s that is about 8.3 GiB of WAL.
  At today's reading it is about 3.7 GB.

## 4. Options

| Option | Host disk at the peak | Pin | Code | Verdict |
|---|---|---|---|---|
| A. Today: full scratch copy | store size, 48 GiB, for about 80 min | copy, 5 to 12 min | built | the defect |
| B. Stream from the read transaction, witness in it, no scratch | WAL growth, about 2 MiB/s for the pin: about 8 GiB | witness plus stream, about 71 min | `streamFile` exists; the pre-842e0fda6 capture is in history | **chosen** |
| C. Copy only changed pages | a persistent base copy (store size, held always) or a store-sized assembly for the witness | as A | new | rejected: the witness needs the whole image, so the copy comes back in another place, or is held all the time |
| D. Scratch only for chunks the host lacks, witness through a reader that fetches the rest from the host | the changed chunks (about half at today's 1,024-page cut, `snapshots-cost-the-change` section 3a) | as A | a SQLite reader over sealed chunks: new and large | rejected: big mechanism, gain capped by the cut's poor dedupe |
| E. Host reflink of `data.img` after a guest freeze, attached read-only | blocks overwritten during the hold | seconds of freeze | a second drive per VM in vmctl, a freeze path, WAL recovery on a read-only copy | not now: right physics, many red surfaces (vmctl, guest boot, the store) |
| F. Drop or replace the witness | none | stream only | format change | not mine: the owner may overrule section 7 of `snapshots-cost-the-change`; nothing cheaper catches a mixed-state stream |

B is chosen because it removes the store-sized write, uses code that
already exists (`streamFile`, and the witness inside the transaction,
which ran in production from 0668c545a to 842e0fda6), and its cost scales
with writes, not with the store. It also removes 48 GiB of guest disk
writes per capture. The record shows 20 failed health checks from 00:02Z to
00:18Z, the same window as the copy; whether the copy's writes caused them
is *(not measured)*, and the claim in section 7 tests it.

What B does not fix: the deploy still waits for a capture in flight (now
about 71 minutes instead of 83), and the witness still grows with the
store. Section 8 says what that means for the 90-minute budget.

### How B gets its consistency

One read transaction on the read pool. Its first statement reads the head,
which fixes the WAL snapshot. The witness reads its digests in the same
transaction (`computerversion.SQLiteStateExtractor.DigestsTx`,
`internal/computerversion/sqlite_state_extractor.go:180`). The pages come
from `SELECT pgno, data FROM sqlite_dbpage` in the same transaction
(`snapshot.go:1087`), in order, with the count checked against
`page_count` (`:1114`, `:1136`). All three are one snapshot because SQLite
gives one read transaction one snapshot. There is no copy, so there is no
`crossCheckCopy` and no digest-of-the-copy check; the bytes sealed are the
bytes read. The transaction ends after the last page is handed over; the
uploader finishes after it.

Order inside the transaction: the witness first, then the pages. This
keeps the rule of 0d2f0bd46: an attempt that cannot finish the witness
uploads nothing.

## 5. Landings, in order

### S1. The capture streams from its read transaction (red)

Scope, small enough for one builder in about an hour:

- `internal/snapshot/snapshot.go`, `capture` (`:552-757`): after the head,
  the partition list and the geometry, call the witness on `tx`, then
  `streamFile(runCtx, cfg, dek, cancel, cause, tx)` for the state file,
  then `rollback()`. Delete the calls to `copyPages`, `crossCheckCopy`,
  `dropSourceCache(scratchPath…)` and `streamScratch`, and the scratch
  creation. Leave those functions in place for S3 to delete.
- `Config.Witness` takes a `*sql.Tx` again (its signature before
  842e0fda6, `842e0fda6^:internal/snapshot/snapshot.go:243`).
- `internal/agentcore/restore_snapshot.go`: add `LiveStoreWitnessTx(ctx,
  tx)` calling `replayStateExtractor(nil).DigestsTx` (it existed before
  842e0fda6). `snapshot_producer.go:882` passes it.
- `ReadReleased` (the checkpoint after the long read,
  `snapshot_producer.go:885-890`) still fires at the rollback, which is now
  after the stream.
- The producer's log line (`snapshot_producer.go:647`) adds `wal +N frames (+B bytes)`
  (`Stats.PinWALEnd - PinWALStart`) and keeps `scratch` at 0.
- The free-space rule stays as it is in S1 (store size plus floor). That is
  conservative: it refuses at least as often as today.
- The header comment of the package (`snapshot.go:1-41`) and the comment in
  `internal/store/wal_bound.go:27-34` are corrected in the same change.

Protected surfaces: snapshot capture and the `snapshot_recorded` event
(tape and evidence), restore verification, the WAL of the live store.

Failure list, written before the tests:

1. A write committed during the stream leaks into the pages (a writer
   commits between every page, as `pageHook` allows today): the restored
   file's head and witness are the transaction's.
2. The witness and the pages disagree: a restore of the result through
   `VerifySnapshotFile` matches.
3. A witness that fails or hits the budget uploads no chunk.
4. A stream that fails midway registers no root and appends no event.
5. A short or out-of-order page stream is refused (exists in `streamFile`).
6. No file is created under `snapshot-tmp` on any path.
7. `PinTime >= WitnessTime + stream time`, and the WAL growth over the pin
   is reported.
8. The free-space guard that fires during the pin ends the transaction at
   once (the WAL stops growing) and records nothing.
9. A rolled-back binary (842e0fda6's) restores a snapshot S1 made: same
   format, same chunk cut, so `restoreOne` accepts it.

Deployed proof on d@e, with observables the author cannot write. The WAL
figure is the frames the pin made the WAL hold (the `log` counter of
`wal_checkpoint`, sampled after the read mark is held and again just before it
is released, times the page size), not the WAL file's size: the file stays at
the 64 MiB journal limit's slack and is reused from its start, so a short
capture on d@e reads zero file growth and says nothing (review of `fab0d6a0e`).

- At d@e's next natural capture after the deploy (the producer's `due_why`
  says when; nothing is forced), its console prints `runtime: snapshot
  taken: ...` with `wal +N frames (+B bytes) during the hold, file ...`, and a
  nonzero witness time and upload time, and `GET /api/runtime/observability`
  shows `snapshot_producer.last_scratch_bytes: 0`,
  `last_pin_wal_frames_known: true` and `last_pin_wal_frames` at least
  `last_pin_events` (every event appended under the pin is one commit and so at
  least one frame). `last_pin_wal_growth_bytes` (the file) may read zero and is
  not a condition.
- A restore of d@e to a checkpoint above that capture (`choir computer
  restore --host=<host> --computer=<d@e> --checkpoint-file=<file>`,
  `cmd/choir/commands.go:462`) answers `"witness_matched":true`.
- On the paper, read only, no drill: the hourly growth ledger in the 48
  hours after the deploy shows no data-image step over 15 GiB while its
  console shows a `snapshot taken` line in that window.
- What to watch on the first computer with a large store (the paper, or a test
  computer made large first, see "Measuring S1 before the paper takes it"):
  the WAL frames over the hold; the count and duration of `store_stalled`
  answers from `/health` during the pin and during the checkpoint after it
  (`CheckpointAfterLongRead`, whose log line `store: wal bound: checkpoint after
  snapshot capture: ... wal A -> B bytes in D` and the observability `wal`
  block's `last_checkpoint` carry its duration, frames and the WAL's size
  after it); read latency on the store while the WAL is large; and that the
  WAL file is back under the 64 MiB limit after the next commit.
  `scripts/proofs/snapshot-pin-observe.sh` prints these every sample.

### What stops a pin that holds too much, and the paper's first capture

**The tripwire.** The capture's ticker (every minute, the same one as the
free-space guard) reads the WAL's frame count and cancels the capture when the
frames added since the read mark was held, times the page size, pass 12 GiB
(`snapshotMaxPinWALBytes`, `snapshot.ErrPinWALBound`): the read is released,
nothing is recorded and the producer's failure line says so. 12 GiB is this
design's figure (about 8 GiB for a 71-minute pin at 2 MiB/s, with margin), not a
measurement, and it is a safety refusal, not a setting. Before this tripwire the
only mid-capture stop was the 2 GiB free-space floor, and the pre-check admits a
capture at store-size free, so about 51 GiB of WAL would have been allowed.

**The checkpoint after the pin, and the write connection.** SQLite's own
rules, read in the amalgamation behind modernc v1.49 (`wal.c`, `sqlite3.c`),
decide this; the code states them beside itself (`internal/store/wal_bound.go`):
a PASSIVE checkpoint never calls the busy handler, takes only the checkpoint
lock (plus read lock 0 for the copy), never the WAL write lock, and copies only
frames up to the smallest reader mark, so it blocks no writer and no reader; it
answers BUSY (`busy=1, log=-1`) and does nothing when another checkpoint or a
reader on read lock 0 is in its way; the WAL restarts at the first commit after
every frame is backfilled, by whichever connection backfilled. But
`wal_autocheckpoint` (default 1000 frames, per connection, never overridden)
runs a PASSIVE checkpoint inside the commit on the committing connection, and
the store's single write connection (`SetMaxOpenConns(1)`) is held until it
returns. After a long pin the first commit would backfill the whole WAL there:
about 22 s per GiB on the 2026-11-12 reading (1 min 21 s for 3.67 GB), about 3
minutes for 8 GiB, with every write, append and `/health` (which reads through
that same connection, `textureHandle`) waiting. A side-connection checkpoint
alone loses that race (review of `41fbba5c0`). So, in S1:

1. before the pin, `Store.SuspendAutoCheckpoint` sets `wal_autocheckpoint=0` on the
   write connection; its resume is deferred around the whole `Take`, so it runs
   on every path (error, cancel, tripwire, panic). The pragma belongs to the
   connection: a process that dies comes up with the default;
2. after the pin, `CheckpointAfterLongRead` runs PASSIVE on its own connection
   (opened `mode=rw`, so a missing file is an error, never an empty database) in a
   retry loop: BUSY, or a pass a reader's mark kept from finishing, is retried
   after a growing pause until every frame is backfilled or a 15 minute budget
   (five times the 3 minutes) ends it; each pass logs the frames backfilled and
   its time. It applies to every caller (the compact copy, the capsule copy);
3. only after it, the automatic checkpoint is restored, so the write connection
   does not do the large backfill itself. If the budget ends first the
   restored automatic checkpoint finishes what is left on the write connection:
   that stall is the one case this does not remove, and the budget and the log
   line say when it happened.

SQLite gives no way to stop a PASSIVE checkpoint after a number of frames, so
bounded steps are not possible; retrying a busy pass is. A test builds a 400 MiB
WAL under a pin, releases it, and shows a write and a health-style read through
the write connection each return within 2 seconds while the backfill runs. Not
measured: the same at the paper's 8 GiB, the disk's effect on read latency during
the backfill, and how often the side connection finds the checkpoint lock busy.

**Decision rule for the paper's first capture** (read from `runtime: snapshot
taken`, the observability block, `/health` and the host's growth ledger):

| Observable | Within | Beyond | Action |
|---|---|---|---|
| WAL held by the pin (`last_pin_wal_frame_bytes`) | up to 12 GiB | the tripwire cancels the capture by itself, nothing recorded | the lead reverts S1 if the paper's first capture is cancelled by the tripwire |
| `store_stalled` answers from `/health`, during the pin and the checkpoint after it | at most 5 failed checks in total, none of a run longer than 60 seconds | more than 5, or any run over 60 seconds | the lead reverts S1 |
| Checkpoint after the pin (the log's per-pass lines and `wal.last_checkpoint.duration`, which now covers every pass) | up to 3 minutes for 8 GiB and no `store_stalled` during it | over the 15 minute budget (the log says so), or a `store_stalled` run over 60 seconds during it | the lead reverts S1 |
| Capture duration (`pin` plus `drain`; the checkpoint after the pin has its own 15 minutes) | up to 75 minutes | 75 to 90 minutes: S1 stays, the witness-speed design (problem record `paper-snapshot-will-pass-its-ninety-minute-budget`) is the next step at once; over 90 minutes: the budget aborts it, nothing recorded | the lead escalates, no revert needed |
| Data image step in the hourly growth ledger | under 15 GiB | 15 GiB or more | the lead reverts S1 |
| WAL file back under 64 MiB after the next commit | yes | no, 24 hours later | the lead opens a record; no revert |

Thresholds are the design's own claims (12 GiB, 5 failed health checks) and the
two readings the record holds; none is a measurement of the paper's pin.

**Residuals.** (1) The capture-side check that the bytes uploaded are the bytes
witnessed (the copy's digest, `crossCheckCopy` and `ErrCopyDigest` of
842e0fda6) is gone; only a restore verifies a snapshot. A cheap capture-side
check is not available: the witness is a logical digest of tables, with no
page-level digest to compare a hash of the streamed pages with; and the one
snapshot property it checked holds by construction here. The deployed proof's
one restore of d@e is the only evidence it works. (2) The tripwire leaves the
chunks already stored on the host unrooted until its grace; "nothing recorded"
means no root and no event. (3) A computer whose write rate trips the bound at
every capture would retry at every due tick and never record a snapshot; the
producer's status (`last_outcome`, `consecutive_refusals`) shows it, nothing
alerts on it, and the lead reads it on every computer after the deploy. (4) The
free-space pre-check is changed in S1, not S2: a capture needs the smaller of the
store's size and the 12 GiB bound, plus the 2 GiB floor (14 GiB for the paper,
not about 53); S2 can still make it follow the measured WAL. The database file's
growth during the backfill is not in that term and is not measured.

### Measuring S1 before the paper takes it

The trade in section 3 rests on one reading: 3.67 GB of WAL in 30 minutes
(about 2 MiB/s) on 2026-11-12 under post-restart load on a 22 GB store (an
earlier reading the same day was 1.99 GB, about 1.1 MiB/s), and a 0.89 MB/s
reading after a boot on 2026-11-20. The pin is also longer than the pin of
842e0fda6 (witness plus the upload draining, about 71 minutes). Whether a
multi-GiB WAL held for that long slows reads, and how long the checkpoint after
it stalls writers (1 min 21 s on 2026-11-12, `store_stalled` at 10.7 s on
2026-11-09), is unmeasured. So S1 is unproven until it is measured on a test
computer with a large store under sustained writes, before the paper takes it.

Procedure (the lead runs it on d@e; nothing touches the paper):

1. Deploy S1 to staging so d@e runs it. Note d@e's store size
   (`PRAGMA page_size`, `page_count`) and the capture floor (free space must be
   store size plus 2 GiB, S1 keeps that rule).
2. Make d@e's store large. **Not verified: no route to write synthetic rows into
   a computer's store was found while building S1**; the lead names one (a
   d@e-only drill route, or a synthetic-row writer run inside the guest). Target
   a store of 20 GB or more with real-sized rows, so the witness takes minutes,
   not seconds. A small store cannot answer the question: the pin is the
   witness time and the WAL is what is written meanwhile.
3. Start sustained writes at 2 MiB/s or more into d@e's store for the whole
   capture (the same writer; 1 MiB commits).
4. Run `SNAPSHOT_OBSERVE_HEADER=... scripts/proofs/snapshot-pin-observe.sh
   <d@e runtime url> 30 240` from before the capture is due until after the
   checkpoint, and keep the console.
5. Read: frames and bytes held at the pin's end (`last_pin_wal_frame_bytes`)
   against the writer's rate times the pin; `store_stalled` count and the
   duration of each run of 503s, in the pin and in the minutes after it;
   `ckpt_took` and `ckpt_wal_after`; store read latency while the WAL is large
   (a timed read from the guest, as `/health` is); `wal_file` back under 64 MiB.
6. Settle: the paper takes S1 only if the WAL held is under the free space with
   the floor to spare at the paper's rate, no `store_stalled` run during the pin
   exceeds what the health guard tolerates, and the checkpoint after the pin
   stalls writers for under 10 seconds. Otherwise S1 does not ship to the paper
   and option E (a host reflink after a guest freeze) is reopened.

Rollback: revert S1. The restore format is unchanged, so snapshots taken
under S1 stay restorable by the reverted binary.

A crash in the middle: the read transaction dies with the process. SQLite
recovers the WAL at the next open, as for any crash. Chunks already
uploaded are not rooted; the host keeps them for its 24-hour grace and the
next capture finds them held (HEAD probe). The attempt guard counts the
crash (`spendGuestAttempt`, `snapshot_producer.go:587`; three starts with
no return close the producer). Nothing resumes: per the restart rules, the
next boot closes nothing for the snapshot because a capture is not an
obligation; the producer's next due tick starts a new capture from a new
head. A planned restart never cuts a capture (it waits for `inFlight`), so
no marker is needed and none is added. The boot sweep of `snapshot-tmp`
stays, for a scratch left by a rolled-back binary.

Claim for the ledger: "After S1 is deployed, the paper's first capture
shows no data-image step over 15 GiB in the hourly growth ledger and its
`last_pin_wal_frame_bytes` (or the log line's `wal +N frames (+B bytes)`) is
under 12 GiB."
Probability 0.75. Settle by the growth ledger and the paper's console
within 48 hours of the deploy.

Second claim, for the health checks: "During that capture the host logs at
most 5 failed health checks for the paper (20 were logged during the copy
on 2026-11-20)." Probability 0.6. Same settlement.

### S2. The free-space rule follows the WAL, not the store (red)

Scope: `snapshot_producer.go:572-583`. A capture needs the floor plus the
WAL it will pin: the larger of the last capture's
`PinWALEnd - PinWALStart` times two, and 2 MiB/s times the last capture's
pin; with no capture known, 2 MiB/s times the 90-minute budget (10.5 GiB).
The refusal text names the term. The guard during the capture
(`snapshot_producer.go:893-903`) is unchanged.

Protected surfaces: the snapshot producer's refusals (a capture refused
too often leaves the replay tail past its bound).

Failure list: a store larger than the free space but a small WAL history
is admitted; a computer with no capture history needs 10.5 GiB plus the
floor; a WAL growth larger than predicted is stopped by the guard during
the capture (exists); the refusal names its numbers.

Deployed proof on d@e: `snapshot_producer.refusal` stays empty and the next
capture is taken; the refusal text, if any, names the WAL term. The
observable is d@e's `last_outcome: taken` after the deploy.

Rollback: revert S2 (the store-size rule returns).

A crash in the middle: the rule is a read before the capture; nothing to
leave.

Claim: "In the 7 days after S2, no capture on any computer on Node B is
refused for free space." Probability 0.85. Settle by the guests' consoles
(`snapshot refused` lines) read on Node B.

### S3. Delete the copy (orange)

Scope: remove `copyPages`, `crossCheckCopy`, `streamScratch`, `osScratch`,
`scratchWriter`, `ErrCopyHeadMismatch`, `ErrCopyDigest`, the `afterCopy`,
`afterWitness`, `openScratch` hooks and their tests; keep
`RemoveSnapshotScratch` at boot and `RemoveCaptureLeftovers` (a rollback
can still leave a scratch). Keep the store-copy token: a capture still pins
the WAL while a compaction would pin it too (`internal/store/compact.go`
copies with `VACUUM INTO`). Update the row in
`docs/current-architecture.md`.

Protected surfaces: none beyond S1's; this deletes unreachable code.

Failure list: the package builds with no reference to the scratch path in
`capture`; every test of S1 still passes.

Deployed proof: the d@e capture after the deploy is `taken` with
`last_scratch_bytes: 0`, as after S1.

Rollback: revert S3.

Claim: none worth entering; a deletion of unreachable code is judged by the
build.

## 6. What a crash leaves, all landings together

| Point of the crash | What is left | Next boot |
|---|---|---|
| During the witness or the stream | a WAL grown by the pin; maybe unrooted chunks | SQLite recovers the WAL; the host collects the chunks after 24 h or a retry reuses them; the attempt is counted |
| After the stream, before the event | a registered root with no event | the existing Stage 0 handling of `snapshots-cost-the-change` section 5 |
| A rolled-back binary mid-copy | its scratch | the boot sweep removes it before the store opens |

## 7. What is not measured

- The split of today's 83 minutes into witness and upload. The paper's
  console has rotated since 01:25Z and its producer status was lost at the
  06:17Z refresh. The next capture's log line names both.
- The paper's WAL write rate under load now. The one reading here (0.89
  MB/s of block writes, which counts checkpoint writes too) was taken just
  after a boot.
- Whether the copy's writes caused the failed health checks.
- Whether a 70-minute pin slows the paper's reads. The 11-12 captures
  pinned for 26 minutes; no slowdown was recorded then, and none was
  measured.

## 8. A second problem this design finds

The capture's time grows with the store, and the budget is 90 minutes
(`snapshot_producer.go:84`). At 1.61 minutes per GB (82.85 minutes for
51.39 GB) the capture reaches 90 minutes at about 56 GB, which is about two
days of growth at 2.4 GB a day. Then every capture aborts with nothing
recorded, the replay tail passes its bound, and each failed attempt leaves
uploads for the host to collect. S1 takes the copy out (about 12 minutes),
which moves the wall to about 65 GB, about six days. It does not remove the
wall. The witness is the wall. This needs its own problem record now, before
the next capture, and a design for the witness's speed.

## 9. Questions only the owner can answer

- Should a deploy refresh of the paper cut a capture in flight instead of
  waiting for it (up to about 71 minutes after S1)? Default taken: keep
  waiting, as today. The capture is the only recovery base for the newest
  hours, and a cut capture costs another hour of guest disk reads.
- Section 8: when the capture meets the 90-minute deadline, which gives
  way: the witness (faster, or the owner's overrule of section 7 of
  `snapshots-cost-the-change`), or the deadline? Default taken: neither in
  this design; record the problem and design the witness's speed next.
