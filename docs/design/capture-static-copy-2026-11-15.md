# Capture through a static copy: design section and failure list before code

Date: 2026-11-15. Stage one of the lane `lane/capture-static-copy`: no
production code. Class: red (section 4). Origin: item 2 of
`docs/problems/restore-bases-design-three-review-rounds-structural-assessment-2026-11-15.md`.
The set-aside package design
(`docs/archive/design/restore-bases-store-once-2026-11-15.md`, "Landing 2") is read
here as a hypothesis; every claim about code below was read from the code, with
file and line. A claim marked *(inferred)* was not read or measured.

Rule for this lane (from the assessment): no mechanism beyond this one. The
additions that are not the copy itself are the ones the copy cannot be correct
without: a cleanup that runs, under an exclusion token, before the free-space
measurement and at the earliest boot step; the exclusion with the compact copy
(both are store-sized files on one volume); and the metrics the proof needs.
Section 5 lists residuals; section 7 is review round 1's findings and what was
done with each.

## 1. What the code does today

Line numbers are of this tree (`origin/main` at 43f568b90).

### 1.1 The one read transaction

- Production caller: `snapshotDepsForRuntime` builds `snapshot.Config` with
  `DB: store.ReadDB()`, `Head`, `Witness`, `FreeSpace`, `Partitions` and
  `MaxDuration` (`internal/agentcore/snapshot_producer.go:795-828`). It is the
  only non-test caller of `TakeSnapshot` in `internal/agentcore`
  (grep of `TakeSnapshot` over `internal` and `cmd`).
- `store.ReadDB()` is the read pool of the store (`internal/store/computer_events.go:28-36`),
  which `dbx.OpenSQLite` opens with `mode=ro` (`internal/dbx/dbx.go:99-130`, DSN at
  `:77-97`).
- `capture` opens it: `cfg.DB.BeginTx(runCtx, ReadOnly)` at
  `internal/snapshot/snapshot.go:533`. The head is the first statement
  (`cfg.Head`, `:549`), which for production is `store.HeadTx` reading
  `computer_event_projection_heads` (`internal/store/computer_events.go:53-59`,
  SQL at `:60`). The telemetry partition list is read in the same transaction
  (`:553-559`).
- Inside the transaction, in this order: the witness (`cfg.Witness`, `:571`),
  then the page stream (`streamFile`, `:577`), which reads
  `SELECT pgno, data FROM sqlite_dbpage` (`:696`), checks the pages arrive in
  order and of page size (`:725`), cuts them into chunks of
  `ChunkSize/pageSize` pages and hands each to four upload workers that seal,
  probe and PUT (`:408-470`, `:704`, `:730`). So the transaction stays open while
  the chunks upload (chunks are handed over a bounded channel, `:412`).
- The transaction ends at `rollback()` after the stream (`:582`); `ReadTxTime`
  is stamped there (`:542`). The comment at `:564-570` says why the witness is
  first: so an attempt that cannot finish the witness dies "with nothing
  uploaded".
- The header comment says "There is no temporary copy of the store and no size
  cap" (`:13-14`). It is the sentence this lane makes false.
- The read transaction therefore lasts witness plus stream. The producer
  comment says the witness alone measured 22 to 35 minutes on the paper
  (`snapshot_producer.go:72-80`; the same text in `snapshot.go:564-570`).
  Those figures are the code's own comments, not re-measured here.

### 1.2 The witness

- `LiveStoreWitnessTx(ctx, tx)` calls `replayStateExtractor(nil).DigestsTx`
  inside the caller's transaction (`internal/agentcore/restore_snapshot.go:251-262`).
- `replayStateExtractor` is the one extractor configuration; `Database:
  "texture"` is a label (`restore_snapshot.go:222-238`). `LiveStoreWitness(ctx,
  db)` runs the same extractor through `extractor.Digests`, which opens its own
  read-only transaction on `db` and calls `DigestsTx`
  (`restore_snapshot.go:243-246`; `internal/computerversion/sqlite_state_extractor.go:152-177`).
- So the witness on a file is already production code: `VerifySnapshotFile` opens
  the file with `dbx.OpenSQLite(path, {ReadConns: 1})`, calls `LiveStoreWitness`
  on `db.Read` and compares `ContentRoot` and `SchemaHash` with the record
  (`restore_snapshot.go:203-216`).

### 1.3 Partition stores

- `capture` takes `Partitions.Hold()` before the transaction and releases it on
  return (`snapshot.go:528-531`). The hold is `gcMu.RLock` on the telemetry
  partition set (`internal/store/telemetry_partition.go:929-935`); partition
  GC takes `gcMu.Lock` (`:768-769`).
- Each partition is read after the state file's transaction has ended, in its
  own read transaction, whose first statement is `SELECT 1`, then `streamFile`
  with the same sqlite_dbpage loop (`snapshot.go:790-815`, called from
  `:598-606`). It pins that partition file's own WAL for that partition's
  stream. The record comment says a partition read later may hold rows above the
  head, which `Open` drops at restore and replay writes again
  (`snapshot.go:593-598`, `:124-135`).
- The producer's `CheckpointAfterLongRead` is on the state store's write
  connection (`internal/store/wal_bound.go:86-93`) and is not run for the
  partitions.

### 1.4 Free space

- Before the attempt: `Busy` (`snapshot_producer.go:519`), `MkdirAll` of the
  data dir (`:524`), `FreeBytes` (`:527`), refusal under the 2 GiB
  `snapshotFreeFloor` (`:531-533`, constant `:93`). For `ev.Kind == "replay"`
  only, a further headroom of the last capture's duration times
  `snapshotWALBytesPerSecond` (2 MiB/s, `:124`) is required (`:536-543`).
  First, ceiling and interval captures get the floor alone.
- During the attempt: a goroutine calls `cfg.FreeSpace()` every minute
  (`snapshot.go:492-517`; `FreeSpaceEvery` default one minute), and the
  producer's function refuses below the same floor (`snapshot_producer.go:813-823`).
- `freeBytesOnVolume(dataDir)` is `Statfs` `Bavail*Bsize`
  (`internal/agentcore/snapshot_freespace_unix.go:9-15`); `dataDir` is
  `filepath.Dir(store.StatePath())` (`snapshot_producer.go:762`, `dataDir := filepath.Dir(store.StatePath())`).

### 1.5 Temp paths and cleanup today

- The only snapshot temp path is the legacy `snapshot-tmp` directory beside the
  store, from the VACUUM INTO era. It is removed with `os.RemoveAll` at
  `snapshot_producer.go:553`, which is AFTER the free-space refusals
  (`:527-543`) and after `spendGuestAttempt` (`:547-550`), so a refusal never
  reaches it. Nothing else removes it: `snapshotLegacyTempDirName` is referenced
  only there and in a test (`:137`, `snapshot_producer_test.go:229`).
- Boot cleans other scratch: `rt.removeStaleReplayScratch()` is the first
  action of `Runtime.Start` (`internal/agentcore/runtime.go:1278`), for the replay
  probe's roots in `replay-probe/` beside the store
  (`internal/agentcore/replay_scratch.go:201-222`, parent `:69-84`). Its comment
  says scratch lives on the persistent volume, "never in /tmp, which in a guest
  is a RAM tmpfs" (`replay_scratch.go:17-23`). That is the precedent for the
  location and for a start-of-boot sweep.
- Compaction's files beside `state.db`: `.compact-staging` (being written),
  `.compact` (staged), `.compact.json` sidecar (`internal/store/compact.go:108-114`).
  `prepareCompactCopy` deletes a leftover staging file before anything else
  (`:267-269`: "A copy from a crashed earlier attempt is never trusted") and drops
  a stale staged copy when it is the disk the new one needs (`:312-316`). It runs
  once per retirement pass, every 24 hours (`internal/store/telemetry_retire.go:30`,
  call at `:342`). It requires `1.2 x live` free (`compact.go:54`, `:311-321`),
  guards the floor every 10 seconds while copying (`:69-72`, `:425-436`), and its
  own read transaction (VACUUM INTO) pins the WAL like the capture's
  (`:388-445`; budget 90 minutes, `:60`).
- Nothing in the snapshot producer knows about compaction, and compaction does
  not look at `inFlight` (grep: `snapshots.inFlight` is read only by
  `planned_restart.go:582`, `memory_cache_return.go:296`).

### 1.6 The WAL after the transaction

- The transaction pins the WAL because a checkpoint cannot copy frames past the
  oldest reader's mark (`wal_bound.go:3-12`). After the read ends,
  `CheckpointAfterLongRead` runs a PASSIVE checkpoint on the write connection
  (`:86-93`). The producer defers it on `Take`'s return
  (`snapshot_producer.go:798`), i.e. after the whole capture, not at the end of
  the read. SQLite never shrinks the WAL unless `journal_size_limit` is set; the
  writer sets 64 MiB so the first commit after a restart truncates
  (`dbx.go:68-75`, `:92-94`).
- `wal_bound.go:28-31` argues the read "is not shortened" because "the head, the
  pages and the witness of a snapshot must be one state" and the SQLite backup
  API restarts whenever another connection writes. That comment is stale after
  this lane (the backup-API half of it stays true; section 2.1).

### 1.7 A crash mid-capture

- `spendGuestAttempt` is called before `Take` and `clearGuestAttempt` after it
  returns (`snapshot_producer.go:547-550`, `:564-567`); three starts without a
  return close the producer (`snapshotAttemptLimit`, `:134`; refusal text
  `:549`). The comment names the crash-loop case (`:46-47`, failure 6).
- A crash kills the process and its read transaction; SQLite's own recovery
  handles the WAL *(inferred; standard WAL behaviour)*. The chunks already PUT
  stay on the host until its grace sweep (comment at `:81-89`). Nothing
  is registered or appended before `RegisterSnapshotRoot` and the append, which
  are the last two acts (`snapshot.go:341`, `:365`). `inFlight` is set around
  `Take` only (`snapshot_producer.go:559-562`), and a planned restart refuses to
  cut it: `unsafe()` includes `SnapshotInFlight` (`planned_restart.go:566-568`),
  with the reason at `:516-519`.

### 1.8 What a restore verifies

- `Restore`/`Install` fetches the record, checks version and computer id,
  `MaxSequence`, the manifest root, geometry (`snapshot.go:989-1031`), probes
  that the host still holds every chunk (`:1036`), assembles the chunks into a
  temp file in the destination's directory, calls `cfg.Verify` on it (`:1082`),
  installs the partitions, removes a stale `-wal`/`-shm`, renames, fsyncs the
  directory (`:1067-1105`). The production `Verify` is `VerifySnapshotFile`
  (`restore_snapshot.go:166` for the staged restore).
- The verification is the witness only: nothing compares the installed file's
  head row to `Record.StoreHeadSequence`. The head and the witness belong
  together today by construction (one transaction, `snapshot.go:549-571`);
  replay then resumes after `StoreHeadSequence` and `tailReceipt` checks the tail
  is contiguous (`restore_snapshot.go:331-357`). Consequence for this lane: the
  construction is what the copy breaks, so the copy needs its own tie (section 2.3).

## 2. The mechanism

### 2.1 How the pages are copied, and why the copy is the pinned head

Inside the read transaction that already exists, after the head, copy every
page of the transaction's snapshot to a scratch file, then end the transaction.

API: the same `SELECT pgno, data FROM sqlite_dbpage` that `streamFile` reads
today (`snapshot.go:696`). Reasons it is a consistent image:

1. The head is read as the first statement of the transaction
   (`snapshot.go:549`), which fixes the WAL snapshot; every later statement of
   the transaction sees that snapshot (this is what the code's own comments rely
   on: `snapshot.go:240-244`, `:545-548`).
2. `sqlite_dbpage` returns pages as the pager sees them at that snapshot, WAL
   frames included, and page 1 with its header. The production restore
   already assembles and opens exactly these bytes
   (`snapshot.go:1067-1105`, `restore_snapshot.go:203-216`), so the property
   holds today; the lane changes where the bytes are written, not which bytes.
3. **Experiment (this lane's first test, updated in review round 1).**
   `internal/snapshot/static_copy_consistency_test.go`, run on the build VM
   (Linux, go1.26.4, modernc sqlite from this module), in two configurations.
   (a) The pinned state lives only in the WAL: main file 4,096 bytes, 17.6 MB
   in the WAL, autocheckpoint off, so the copy cannot be a read of the file.
   (b) Production defaults: autocheckpoint on, a concurrent
   `wal_checkpoint(PASSIVE)` every 20 ms, and a freelist from deleted rows
   (main file 7.7 MB, WAL 4.1 MB). In both: a writer commits constantly, the head
   (ten columns, like `store.Head`) is read first, 2,009 pages are copied, and
   the copy loop paces on writer progress (it waits for one more commit every 50
   pages, 30 s limit) instead of sleeping. 42 commits landed during each copy
   (head 600 to 642); none leaked. (The first version of the test, with 5 ms
   sleeps, saw 54 commits in the author's run and 1,294 in a reviewer's: the
   count depends on the machine, which is why pacing is on progress now.) The
   copy opened with `dbx.OpenSQLite(..., ReadOnly: true)` held exactly the rows
   of head 600, its ten head columns equalled the transaction's, `quick_check`
   was `ok`, and in (a) the main file's SHA-256 differed from the copy's (the WAL
   content reached the copy). Opening, querying and closing the copy read-only
   left its SHA-256 unchanged. `-wal` and `-shm` were left beside the copy after
   close. A copy missing its last page was refused when opened (`database disk
   image is malformed`) before any cross-check. A same-size copy of the same
   sequence whose `effective_state_commitment` alone differs is refused by the
   cross-check, and a nil head is equal only to a nil head. Not covered by this
   test, so listed in section 3: auto_vacuum/`incremental_vacuum` truncation,
   page sizes other than 4096, a store past 1 GiB (the lock-byte page), and the
   read/write open that `VerifySnapshotFile` uses. (A reviewer's throwaway check
   of the read/write open, `computerversion` digests on the copy and a copy after
   `incremental_vacuum` all matched; that was not mine and is not in the tree,
   so failures 7 and 8 pin it.)

Alternatives, rejected on what the code says: `VACUUM INTO` renumbers pages (the
`compact.go` copy: "each B-tree written contiguously", `:388-391`), so a copy
never dedupes against live-file chunks; the SQLite backup API restarts whenever
another connection writes (`wal_bound.go:28-31`).


### 2.2 The write

- The scratch is a directory `snapshot-tmp` under the data dir (the name the
  legacy cleanup uses, `snapshot_producer.go:137`), holding one file `state.db`.
  It is on the data volume beside the store, never `/tmp`
  (`replay_scratch.go:17-23`). Directory `0700`, file `0600`, `O_EXCL`, in a
  directory that was removed first; an existing file at that path is an error,
  never reused.
- Rollback caveat, stated plainly: reusing the name means a rolled-back binary
  removes a scratch left by the new one, but its removal runs after its own
  free-space refusal (`snapshot_producer.go:527-553`) and it has no boot sweep. A
  rollback onto a volume with less than the old rule's floor free, with a
  leftover scratch, needs an operator to delete `snapshot-tmp`. See 2.8 and
  section 4.
- Pages go through a 4 MiB buffered writer. Every 64 MiB (`sourceDropBytes`,
  `snapshot.go:612`, made a package variable `scratchSyncBytes` so a test can
  shrink it) the code flushes the buffer, calls `Sync` (an `fsync`, which covers more than `fdatasync` would) on the file, drops
  that range with `adviseDropFileCache` (`internal/snapshot/cache_linux.go:14-21`;
  a no-op elsewhere, `cache_other.go`, so the proof runs on Linux guests) and
  calls `cfg.FreeSpace()`. Reason *(inferred)*: `FADV_DONTNEED` drops only clean
  pages, 40 GB of dirty pages would fill a guest's memory, and a synchronous
  commit of the live WAL must not wait behind more than one 64 MiB writeback.
- The SHA-256 of the bytes written is computed **inline**, on the same buffer,
  by the goroutine that writes. It is not a second goroutine: a bounded queue
  would make the slower side (hash, about 450 MB/s on the build VM, 2 GiB in
  4.57 s, against 2 GiB written and fsynced in 3.65 s there) the pin, and an
  unbounded one holds memory. The cost is therefore a bounded, named addition to
  the pin: `S / 450 MB/s`, about 88 s for 39.6 GB *(build VM figure, one run; a
  guest may differ)*. The digest is compared at the end of the upload (2.4 step
  6); it is what re-establishes "the bytes uploaded are the bytes witnessed"
  once one transaction no longer produces both.
- The pin (`PinTime`) is the interval from the `BeginTx` return to the return of
  the transaction's rollback, taken by `capture` with its own clock, not by the
  copy loop. `CopyTime` is the page loop alone. They are different timers; a
  test pins that `PinTime >= CopyTime` and that an outside probe sees the reader
  for `PinTime` (failure 12).
- The transaction ends when `pages == page_count` and the writer is flushed,
  synced and closed.
- Writes go through a small interface (`scratchWriter`: `Write`, `Sync`,
  `Close`) so a test can inject `ENOSPC` and `EIO`, which cannot be forced
  portably.

### 2.3 How the copy is tied to the head

Before the witness, on the opened copy (opened read-only,
`dbx.OpenSQLite(path, {ReadOnly: true, ReadConns: 1})`, `dbx.go:99-130`; the
experiment shows it does not change the file):

1. `PRAGMA page_size` and `PRAGMA page_count` equal the values read in the
   transaction (`streamFile` reads both, `snapshot.go:662-667`), and the file
   size equals `pageSize x pageCount`.
2. The head read from the copy equals the head read first in the transaction on
   **every field `Head` carries**: `ComputerID`, `Sequence`,
   `CanonicalEventHead`, `DesiredEventHead`, `EffectiveEventHead`,
   `PendingTransitionRef`, `DesiredStateCommitment`, `EffectiveStateCommitment`,
   `ReducerVersion`, `CredentialRevocationEpoch`
   (`internal/computerevent/reducer.go:16-27`; the columns of the `SELECT`,
   `internal/store/computer_events.go:60`). The comparison is by value over the
   whole struct (`*got == *want`; every field is a string or integer, so the
   struct is comparable); a check of the sequence alone is a defect. It reuses
   `cfg.Head` with a read transaction on the copy's own DB, so it adds no
   second query. `Head` returns nil for an empty store (`scanProjectionHead`,
   `computer_events.go:62-67`): nil equals nil and nothing else; nil on one side
   only fails the attempt.
3. The pages written equal the transaction's `page_count` (today's equivalent,
   `snapshot.go:749`).

A mismatch fails the attempt with the name of the check; nothing was uploaded or
registered, the scratch is removed. What this check catches is a writer-side
defect (a dropped or duplicated page, a wrong size, a wrong file), not a mix of
two snapshots inside one transaction, which cannot happen. Things produced at a
different instant than the copy are unchanged: the partitions (read after,
`snapshot.go:598-606`), `CreatedAt`, the manifest time, `RegisterSnapshotRoot` and
the append (`snapshot.go:341`, `:365`). `FileTreeRoot` is the one record field that
could name yet another instant; it is unset in production (the producer's
`snapshot.Config` sets no `FileTreeRoot`, `snapshot_producer.go:795-828`).

The witness is then `LiveStoreWitness(ctx, copyDB)`, the code `VerifySnapshotFile`
runs on an installed base (`restore_snapshot.go:243-246`, `:203-216`).
`VerifySnapshotFile` opens read/write and the capture read-only; both read the
same pages, and failures 7 and 8 pin the equality.

### 2.4 Order of an attempt (new)

In `evaluate` (`snapshot_producer.go:432-`), the due evaluation and the backoff
gate (`:440-515`, `Ready` at `:440`) are reads only and stay first. Then, as the first act of an
attempt that is going to run (the coordinator asked for "right after the tick
lock"; the lock is taken at `:436`, and the evaluation between them only reads and
cannot touch a file; taking the token on a not-due tick would refuse and count a
streak for a snapshot nobody wanted):

1. **Take the store-copy token**: `release, holder, ok :=
   TryHoldStoreCopy("capture")` (2.6), with `defer release()` on the next line, so
   that every return from `evaluate` after this point (a free-space read error,
   the `spendGuestAttempt` refusal at `snapshot_producer.go:547-549`, a
   `TakeSnapshot` configuration error, a panic) gives the token back. `release` is
   idempotent. Refused: return `snapshotRefused` naming the holder; nothing is
   deleted, measured or started. The token is therefore held until `evaluate`
   returns, which includes the partitions, the manifest and the append after the
   scratch is gone (a few minutes more than the scratch's own life; accepted, one
   rule instead of two).
2. **Clean**: `MkdirAll` of the data dir (today between `Busy` and `FreeBytes`,
   `snapshot_producer.go:524-527`; moved here, because the directory must exist
   for the copy anyway and nothing is measured before it), then `RemoveAll` of
   `snapshot-tmp` (scratch and, inside it, the copy's
   `-wal`/`-shm`), and, because the token proves no compaction in this process is
   writing it, `removeSQLiteFiles(state.db.compact-staging)`
   (`compact.go:239-243`, the same call `prepareCompactCopy` makes first,
   `:267-269`). A removal that fails fails the attempt closed (log, no capture);
   nothing proceeds on top of a file it could not remove. Only the never-trusted
   staging file is removed; the capture never touches `state.db.compact` or its
   sidecar (2.5).
3. **Measure**: `Busy`, then free space and `page_count` (2.5). The old order
   put the free-space refusals before the only cleanup (`snapshot_producer.go:527-543`
   against `:553`); this is the new order.
4. `spendGuestAttempt`, `inFlight = true`, `Take`.
5. In `Take`/`capture`: hold the partitions, begin the transaction, head, page
   size and count, partition list, copy (2.2). Then the transaction ends:
   `rollback()` is the one place that ends it (`snapshot.go:538-546`; it is
   idempotent through `txOpen`, and `defer rollback()` runs it on every return).
   **The `ReadReleased` hook fires from that closure, once, on every path**: a
   successful copy, a copy error, a cancel, the time budget, `ENOSPC`, a head or
   cross-check failure. It is called with `context.WithoutCancel(ctx)` of the
   caller's context (not `runCtx`, which the budget or a cancel may already have
   ended, which is when the WAL is largest), bounded by the capture's existing
   deadline value: `context.WithTimeout(context.WithoutCancel(ctx),
   cfg.MaxDuration)` (a fresh span of that length from the hook's start, because
   the deadline of the run may already have passed in the cases the hook exists
   for; no new constant). When `MaxDuration` is zero the hook gets no deadline at all
   (the same `if cfg.MaxDuration > 0` guard `capture` uses, `snapshot.go:484-488`;
   `WithTimeout(ctx, 0)` would be expired at once, which is wrong); production never
   passes zero (`snapshot_producer.go:80`). The caller cannot cancel it, the deadline can. On timeout the
   hook logs and returns; it never fails the attempt (the checkpoint is best
   effort, as `CheckpointAfterLongRead` already treats an error, `wal_bound.go:94-98`).
   The goroutine is started after `PinTime` has been taken, so neither its start
   nor its run is inside the timed pin. In production it runs
   `store.CheckpointAfterLongRead` (`wal_bound.go:86-93`) and reads the head
   sequence for the proof (4). It runs in its own goroutine, so the witness
   does not wait for it (the witness reads the copy's own DB, not the live
   store); `capture` joins it before it returns (at most `MaxDuration` after the
   hook starts), so `Take` returning still means the checkpoint finished or timed
   out, as the old `defer` did (`snapshot_producer.go:798`).
   It does take the store's single write connection for its duration, so appends
   queue behind a PASSIVE checkpoint of the pinned WAL, exactly as they did at
   the end of every old capture; the proof measures that (4).
6. Open the copy read-only, cross-check (2.3), witness, close the copy's DB.
   Upload from the file: read the scratch in `ChunkSize` blocks (the same cut,
   `snapshot.go:660-672`), hash as read, seal, probe, PUT with the existing
   uploader (`:408-470`). At the end compare the hash with the copy's; a mismatch
   fails the attempt before `RegisterSnapshotRoot`.
7. **Delete the scratch right after the upload** (`RemoveAll` of the directory) and
   on every error path (a `defer`), then call the `ScratchRemoved` hook once on
   every path. `ScratchRemoved` is a test and observability seam for the scratch
   only; it does not release the token (step 1's `defer` does).
8. Partitions (unchanged), manifest, `RegisterSnapshotRoot`, append
   (`snapshot.go:341`, `:365`).

`inFlight` stays true for the whole `Take` (`snapshot_producer.go:559-562`). A
planned restart still waits for the whole capture, because a restart after the
copy would discard the scratch and the witness for nothing
(`planned_restart.go:516-519` gives the same reason for the stream).

**Boot (second defence first).** `autoputer.Run` does the compact rebase at
`internal/autoputer/run.go:213` (`RebaseOntoCompactCopy`), opens the store at
`:236` and replays the tape (`:900-910`) before `rt.Start` (`:786`, `:817`), so
`Runtime.Start`'s sweep (`runtime.go:1278`, where `removeStaleReplayScratch`
runs) comes after the disk users the sweep is for. The scratch removal therefore
moves to the earliest boot step: a store helper
`RemoveSnapshotScratch(storePath)` (the directory is
`filepath.Dir(StateDBPath(storePath))/snapshot-tmp`, `state_path.go:13-15`) called in
`autoputer.Run` before `RebaseOntoCompactCopy`. `Runtime.Start` calls the same
helper too, as a second defence (it is the earlier, simpler call site for tests
that construct a runtime without `autoputer.Run`). Nothing runs in the
`Start`-less window that could be writing a scratch (no capture runs before a
boot has settled: `Ready`, `snapshot_producer.go:440`).

### 2.5 Free-space rule, every trigger kind

One rule for first, ceiling, interval and replay, because every capture copies:

```
need = S + snapshotFreeFloor          (2 GiB, snapshot_producer.go:93)
S    = page_count x page_size from the read pool, read after the token and cleanup
```

- The old replay-only headroom (`snapshot_producer.go:536-543`) is **deleted**,
  and no new headroom is added (review round 1 deleted the constants 50 MiB/s and
  the factor 2 I had invented). The headroom existed to avoid "a mid-capture
  abort after gigabytes of upload" (`:116-124`); an abort during the copy now
  uploads nothing, and the 64 MiB guard (below) stops the copy before the floor.
- Unreadable `page_count` or free space fails closed, as today (`:527-530`).
- During the copy: `FreeSpace()` runs every 64 MiB written and fails the attempt
  under the floor. The minute-interval goroutine (`snapshot.go:492-517`) stays
  for the witness and upload. Reason *(inferred)*: the copy writes at disk speed,
  so one minute can be 10 GB or more against a 2 GiB floor. A `statfs` error
  during the copy fails closed (as `:816`).
- **The new disk requirement.** A computer with free space above the floor but
  below `S` can no longer capture at all; today it captures with the floor alone
  because nothing is written locally. This is a regression for such a computer
  and is a named residual (section 5, R1, and failure 17): its tail grows until
  the ceiling, then until the recovery bound, with the refusal visible in
  `snapshot_producer.refusal` and `consecutive_refusals` (`snapshot_producer.go:665`,
  status). No computer in the records I read is in that state; the paper's
  numbers from the set-aside design (39.6 GB store, 85.9 GB available at
  2026-11-15T05:49:12Z; not re-measured here) leave it 46 GB.
- **Staged compact copy (`state.db.compact`, complete).** The capture **never
  deletes it** or its sidecar. The admission test is `free < S + floor`, where
  `free` is measured after the staged file exists; its bytes are already missing
  from `free` and are not added again. When the test fails and a staged copy is
  present, the refusal names `free`, `need = S + floor`, and the staged file's
  size by `os.Stat` of `state.db.compact` (not the sidecar's `Size`, which the
  boot rewrites, `compact.go:659-670`). The band this refuses in is residual R4.
  Figures from the set-aside design (39.6 GB store, about 42 GB staged, 85.9 GB
  available before staging; none re-measured here, and the staged size is
  inferred): `free` about 43.9 GB against `need` 41.7 GB, admitted by about 2 GB;
  a staged copy a little larger, or a store a little larger, refuses.
  **Floor against WAL growth during the pin.** Admission is `S + floor`, with no
  WAL term; while the pin is open the WAL grows (the code's 2 MiB/s,
  `snapshot_producer.go:120-124`, an unmeasured figure), and the 64 MiB guard
  aborts the copy once `free` falls under the floor. So the band
  `[S + floor, S + floor + WAL growth over the copy)` is **admitted and then
  aborted**, not refused: the copy time is lost, nothing is uploaded, the scratch is
  removed, and the producer backs off (`snapshotBackoffWait`). On the figures above
  (a 2.15 GB margin) that is a copy that runs longer than about 18 minutes at
  2 MiB/s, i.e. an effective copy rate under about 37 MB/s for S = 39.6 GB. This
  floor is in addition to step 8's two rates. No WAL term is added to admission
  (round 1 deleted the headroom on purpose: it protected an upload abort that no
  longer exists); the abort band is stated instead and is part of R4.
- **Capture and compaction exclude each other.** The token's scope is one
  statement: **the whole of `evaluate` after step 1 of 2.4** (it is released by
  `defer`). So a compaction can wait for a capture's pin, witness, upload, hook,
  partitions and append; and a capture is refused for a compaction's whole
  `prepareCompactCopy` (2.6): a due capture, including a replay-triggered one that protects the
  recovery bound, is refused while compaction holds the token, which is up to its
  own copy budget of 90 minutes plus its check (`compact.go:60`, `:343-385`); and
  compaction waits while a capture holds it. Refusals are counted for status only
  and touch neither the failure counter nor the backoff (`recordOutcome`,
  `snapshot_producer.go:643-658`; the refusal returns at `:519-545` before the
  failure bookkeeping), so a long compaction escalates nothing. The refusal log
  names the holder. No priority rule is added.
- Compact copy in progress: the token (2.6); a leftover staging file is removed
  at step 2 of 2.4 once the token is held.
- The replay probe's scratch admits by measured free space
  (`admitReplayScratch`, `replay_scratch.go:124-142`, called from
  `replay_completeness.go:302`). Rematerialize staging does **not**: it creates
  `rematerialize-staging-*` beside the store (`rematerialize.go:93`) and
  `admitReplayScratch` has no other caller. A restore during a capture can
  therefore fill the volume the capture's guard is watching; the capture's
  guard aborts the capture (the copy is lost, nothing is uploaded) and the
  restore proceeds. Named residual R2; no mechanism is added.

### 2.6 The exclusion with compaction

Both are store-sized writes to one volume and nothing orders them today (1.5).
An in-process token on `store.Store`, with two entry points:
`TryHoldStoreCopy(kind) (release func(), holder string, ok bool)` (never blocks;
the capture uses it) and `HoldStoreCopy(ctx, kind) (release func(), err error)`
(waits for the holder, bounded by the context; compaction uses it). `release` is
idempotent (a `sync.Once`) and both callers `defer` it at acquisition.

- Compaction takes it at the top of `prepareCompactCopy`, **before**
  `removeSQLiteFiles(staging)` (`compact.go:267-269`), and holds it until the
  staged file is renamed or removed (the end of the function). It waits rather
  than skipping, so a collision does not cost a 24-hour pass
  (`telemetry_retire.go:30`; the retirement goroutine already runs for tens of
  minutes in this call, `compact.go:306-385`). Its wait is bounded by its context
  (a cancelled wait returns without taking the token), and its own copy budget
  (`compactCopyBudget`, `compact.go:60`, applied at `:330`) starts after the wait.
- The capture takes it with `TryHold` at step 1 of 2.4 and holds it until
  `evaluate` returns. A refused capture is `snapshotRefused` with the
  holder's name and retries at the next tick (10 minutes).
- Because the token is taken before any deletion and any reading of free space, a
  compaction that starts after the capture took it waits; one that started first
  made the capture refuse before it deleted anything. The race the first draft
  had (the capture unlinking the file compaction is writing, or measuring space
  compaction is about to consume) cannot occur.
- Being in-process, a crash releases it. One agentcore process runs per guest
  *(assumption, from `Runtime`/`autoputer.Run`)*.

### 2.7 What still pins the WAL, and for how long

- The state store's read transaction: head read, page count, partition list and
  copy, plus the inline hash. This is the pin the lane shortens, from today's
  witness plus stream to the copy. The stages are serial on one goroutine (read, hash, buffered write, sync), so
  its per-byte time is the sum of theirs, not the slowest (one run each of a throwaway Go program on `choir-agents`, 2026-11-15, 2 GiB, the
  program not kept: 450 MB/s SHA-256 (4.57 s), 560 MB/s write plus fsync (3.65 s),
  2.1 GB/s read (0.94 s, page cache warm);
  a guest is unmeasured). WAL growth is proportional to the pin; the set-aside
  design's cited growth (2.6 to 3.7 GB over a 37-minute capture) is a different
  rate from the code's 2 MiB/s (`snapshot_producer.go:120-124`), and neither is
  measured here. The proof therefore reports growth normalised by the events
  appended during the pin (4), and predicts nothing in bytes.
- Each telemetry partition's read transaction pins that partition's own WAL for
  its stream and upload, sequentially, after the state transaction and after the
  state upload (1.3). Unchanged and still unmeasured; no metric is added for it
  (round 1: the proof does not need it).
- The partition hold (`gcMu.RLock`) lasts from before the transaction to the end
  of `capture`, as today (`snapshot.go:528-531`); partition GC waits for the
  whole capture, as it already did, and now also for the read-released hook,
  because `capture` joins it before returning: up to `MaxDuration` longer than
  today in the worst case (the hook is bounded by a fresh `MaxDuration`).
- After the transaction nothing of the capture pins the state WAL. The PASSIVE
  checkpoint runs at release (2.4 step 5); ordinary autocheckpoints continue.
  The file keeps its high-water size until a writer restarts the WAL and
  truncates to 64 MiB (`wal_bound.go:20-24`).
- Other long readers are unchanged: the compaction copy (1.5) and the checkpoint
  bind's witness (outside this lane).

### 2.8 Crash at every step

| Step | State left | Next attempt or boot |
| --- | --- | --- |
| before the token | nothing | normal |
| token held, cleanup or measurement | the token dies with the process | normal |
| during the copy (transaction open) | partial scratch, no WAL pin (process gone), attempt counter at +1 | `autoputer.Run` removes the scratch before the rebase and the store open (2.4); `Start` again; the attempt's step 2 again |
| after the copy, before or during the witness | full-size scratch | same; the witness is repeated from a new copy (nothing resumes: CLAUDE.md "A crash restart never resumes work") |
| during the upload | scratch, some chunks on the host | same; the host sweeps unregistered chunks after its grace (comment `snapshot_producer.go:81-89`) |
| during or after the scratch delete | none or partial directory | same sweep; a partial `RemoveAll` is finished by the next one |
| after the delete, before `RegisterSnapshotRoot` or the append | no scratch, chunks on the host | existing behaviour (`snapshot.go:341`, `:365`) |
| after the append | complete | `clearGuestAttempt` runs when `Take` returns; a crash between append and return leaves the attempt counted (existing) |
| a failed delete of the scratch | scratch | logged; the next attempt fails closed at step 2 until it can be removed (2.4) |
| **boot with a leftover scratch and low free space** | scratch of up to S bytes | removed first in `autoputer.Run`, before the rebase (`run.go:213`), the store open (`:236`) and the replay; without the early removal the rebase, open or replay could fail on a full volume before any sweep |
| rollback to the previous binary | `snapshot-tmp/` | removed at the old attempt start (`:553`), which follows the old free-space refusal (`:527-543`): on a volume too full for the old floor an operator deletes the directory; the old binary has no boot sweep |

A crash costs the attempt counter: three crashed starts close the producer
(`snapshot_producer.go:134`, `:547-550`), existing behaviour. A deploy restart is
not a crash because the planned restart waits for `inFlight`. The d@e drill
(section 4) therefore kills at most twice between normal attempts.

### 2.9 What is deleted

- `LiveStoreWitnessTx` and `Config.Witness`'s transaction form
  (`restore_snapshot.go:251-262`; `snapshot.go:240-244`, `:571`); `Config.Witness`
  takes the copy's `*sql.DB`. `DigestsTx` stays (`Digests` uses it).
  Callers to change, because the gate runs every package importing the changed
  one: `internal/agentcore/snapshot_producer.go:808`,
  `internal/agentcore/snapshot_stream_test.go:185-192`,
  `internal/agentcore/restore_snapshot_test.go:254-261`,
  `internal/agentcore/restore_probe_production_test.go:151-157`,
  `internal/autoputer/projection_base_test.go:213-223` and `:274-280`, the
  fixtures in `internal/snapshot/*_test.go` (12 files call `TakeSnapshot`), and the
  comment at `internal/agentcore/canonical_witness.go:40`.
- The state file's use of the `sqlite_dbpage` loop in `streamFile`
  (`snapshot.go:696-760`) is replaced by a file reader for the state file; the loop
  remains once, in the copy, and for partitions. The in-transaction witness and its
  ordering comment (`:564-570`) go.
- The replay-only WAL headroom (`snapshot_producer.go:536-543`).
- The comments that say the opposite of the new design are rewritten, not left:
  `snapshot.go:13-14` ("no temporary copy"), `snapshot_producer.go:38-41` (failure
  item 4) and `:91-92` ("The stream writes no copy"), `wal_bound.go:28-31` ("not
  shortened"; the backup-API half stays true) and the 90-minute reason at
  `snapshot_producer.go:72-79`.
- `CHOIR_SNAPSHOT_MAX_DURATION` stays and stays the one deadline for the whole
  capture, which also bounds the pin: the 30-minute copy budget I had proposed is
  **deleted** (it was an invented number; the existing 90 minutes already gives a
  pin no worse than today's, and the 64 MiB free-space guard is the real bound).
  No new environment variable, no new constant for a budget.
- `snapshotLegacyTempDirName` is renamed `snapshotScratchDirName`, same value;
  the one `RemoveAll` at `:553` becomes the step-2 cleanup and the boot helper.
- Not added (round 1 removed them): the persisted copy-seconds field and its
  headroom use, `last_partition_pin_max_seconds`, the tick-level cleanup on a
  not-due evaluation.
- Not deleted: `VerifySnapshotFile`, `Restore`, `Install`, the record, the chunk
  cut and every restore path. A record from the new code is the record of today's
  code; `DurationSeconds` in the persisted capture record stays the whole
  capture's, because `snapshotCaptureFloor` and `snapshotDue` read it
  (`snapshot_producer.go:~345-410`, `:590-606`).

## 3. Failure list (tests are written from this list, before the code)

Test 0 exists: `TestStaticCopyIsAConsistentImageOfThePinnedHead` (two
configurations, section 2.1). Seams the tests need, named so the builder writes
them first: `Config.pageHook` (exists) called in the copy loop; unexported
`Config.afterCopy(path)` and `Config.afterWitness(path)` hooks; the
`scratchSyncBytes` variable (2.2); the `scratchWriter` interface (2.2);
`Config.ReadReleased(ctx)` and `Config.ScratchRemoved()`; an unexported `afterGates` hook in `evaluate` between the read-only gates and the token (so a compaction can be started exactly there); an injectable `hash.Hash` for the inline digest; the producer's existing `FreeBytes` dependency and `Config.FreeSpace` for every space case (injected, keyed on file state; never real `statfs` margins), with sparse files (`Truncate`) standing in for a 42 GB `.compact`; the boot phase names of `autoputer.Run` (`objectgraph.BootPhase`, `run.go:212`) for the boot order; `Config.MaxDuration`
(exists) for budgets; `FreeSpace` (a func, exists) for space.

Consistency and the tie to the head
1. Writers commit throughout the copy, in both configurations (WAL-only state;
   production defaults with concurrent PASSIVE checkpoints and a freelist); the
   copy opens as exactly the head read first (test 0).
2. A copy whose head differs from the transaction's in **one column other than
   the sequence** (a hook rewrites `effective_state_commitment`, then another
   test `canonical_event_head`) is refused; nothing uploaded, no root, no event,
   scratch gone. A test per column of the ten is a table test.
3. A copy that is short, long or of another page size than the transaction read is
   refused by the cross-check or by open.
4. Pages copied differ from `page_count`: the attempt fails by name.
5. A byte flipped in the scratch after the copy and before the witness (hook
   `afterCopy`), and one flipped after the witness (`afterWitness`), each fail the
   attempt through the digest compare before `RegisterSnapshotRoot`.
6. The record's `StoreHeadSequence` is the head row of the copy; a restore of the
   base replays exactly (head, target] (`tailReceipt`,
   `restore_snapshot.go:331-357`).
7. Witness equality, under concurrent writers: the witness of the copy equals
   `DigestsTx` run inside the capture transaction at the pinned head while writers
   commit, and equals `LiveStoreWitness` of the live store at a quiet head.
8. The copy opened **read/write** as `VerifySnapshotFile` does
   (`restore_snapshot.go:203-216`): main-file SHA unchanged, geometry, head and
   `quick_check` unchanged, witness equal to the read-only open's; the same after
   `auto_vacuum=INCREMENTAL` with `incremental_vacuum` shrinking the file before the
   capture (stale in-header size).
9. A store whose page size is 512, 8192 and 65536 copies and restores (the last
   partial chunk, `snapshot.go:660-672`); a store past 1 GiB (the lock-byte page)
   copies and restores equal (a test that creates a file just over 1 GiB, skipped
   under `-short`, run once on the build VM).
10. A snapshot from the new code installs through the unchanged `Restore` with
    `VerifySnapshotFile`, and its chunk digests equal those of the page sequence cut
    by `ChunkSize/pageSize` (dedupe against earlier bases is preserved).
11. A nil head on both sides is accepted and recorded as today; nil on one side
    refuses.

The pin
12. An outside probe observes the pin without a time threshold, on ORDER: after
    the transaction is open the test commits one write behind it, and a probe
    goroutine then repeats `wal_checkpoint(PASSIVE)` on the live write connection
    (PASSIVE never truncates, so it does not change the WAL the proof reads); a
    reader at the head is visible as `checkpointed < log` once a write has landed
    behind it. The last "pinned" observation precedes the `ReadReleased` call, the
    first "unpinned" follows it, and none is "pinned" during the witness callback or
    the upload; `PinTime >= CopyTime`. (Round 1's 1 ms `TRUNCATE` probe and its
    50 ms tolerance are gone: `TRUNCATE` changes the WAL and sees nothing for a
    reader at the head.) The test holds the live handle itself.
13. `ReadReleased` fires exactly once, from the rollback closure, on: a successful
    copy, a copy error, cancellation during the copy, `MaxDuration` during the
    copy, an injected `ENOSPC`, an injected `EIO`, a cross-check failure and a head
    read failure; each time with a context that is not cancelled even though
    `runCtx` is; and the producer's checkpoint is not deferred to the end of `Take`.
14. The witness starts while the hook's checkpoint is still running (the hook
    blocks on a channel; the witness must not wait), and `capture` joins the hook
    before returning. The hook's goroutine start and run are outside `PinTime`.
15. Appends and writes proceed during the copy, the witness and the upload (the
    no-hold test, `snapshot_nohold_test.go`, extended to the scratch phases), and
    during the PASSIVE checkpoint no append is lost (only delayed).
16. A telemetry partition retired after the head and before its read is kept by
    the hold until it is read (the existing partitions test).

Space, ordering and cleanup
17. A capture on a volume with `floor <= free < S` is refused with the amounts
    named, for every kind (first, ceiling, interval, replay), creates nothing,
    and the refusal appears in `snapshot_producer.refusal` (the named
    regression R1). The same table: just under `S + floor` refuses, just over
    takes the capture.
18. Order: a scratch directory left by a crash is removed BEFORE the free-space
    measurement. With `free = need - leftover` the attempt proceeds; with the old
    order it refuses for ever.
19. The token is taken before any deletion or measurement. A compaction holding the
    token when the capture starts makes the capture refuse with the holder named
    and **nothing deleted** (a `.compact-staging` file written by that compaction
    is intact afterwards). The atomic property is tested with the token
    pre-held by a compaction stand-in (no seam exists between the cleanup and the
    measurement, and none is needed: after step 1 nothing else can hold the token).
    A compaction that starts at the `afterGates` seam (between the gates and the
    token) either holds the token first (the capture refuses, nothing deleted) or
    waits for the capture's token and proceeds when `evaluate` returns.
20. A leftover `.compact-staging` file with no compaction running is removed at
    step 2 using `removeSQLiteFiles` (its `-wal`, `-shm` and `-journal` too); one
    with a compaction holding the token is not touched.
21. The scratch's `-wal` and `-shm` are removed with the directory (test 0 saw them
    left after close).
22. Boot: the scratch sweep is a boot phase that is recorded before the
    `compact-rebase` phase (`run.go:212-213`), the store open and the replay, and
    its helper removes a leftover directory (with its sidecars); `Runtime.Start`
    removes it again if it exists. This pins the ORDER; a boot on a genuinely full
    volume is not simulated by any seam I could name, so the claim that the rebase,
    open and replay then succeed rests on the order and is exercised by the d@e
    drill (section 4, step 4), not by a unit test.
23. A scratch whose removal fails, a `.compact-staging` whose removal fails, and an
    existing file at the scratch path with `O_EXCL` each fail the attempt closed:
    no copy starts.
24. Free space falling under the floor mid-copy (checked every
    `scratchSyncBytes`) aborts the copy; the transaction is released, the scratch
    removed, no root, no event; a `statfs` error mid-copy fails closed.
25. `ENOSPC` or `EIO` on write or sync (injected writer) names the error;
    transaction released, scratch removed, hook fired once.
26. A staged compact copy lowers measured free space and is not double counted:
    with a sparse `state.db.compact` present and the injected free `= need`, the
    capture takes; with injected free `= need - 1` the capture refuses, **nothing is
    removed** (`state.db.compact` and `.compact.json` are byte-identical afterwards),
    the refusal names `free`, `need` and the staged file's `os.Stat` size, and the
    refusal is visible in `snapshot_producer.refusal`. A compaction running (token
    held) is never touched.
27. The scratch is deleted right after the upload and on every error path (witness
    error, upload error, `RegisterSnapshotRoot` error, append error, cancellation),
    including before the partitions are read; `ScratchRemoved` fires once on each.
28. The token is released, through the `defer release()` taken at acquisition and
    idempotently, on success, error, panic and context cancellation of both the
    capture and compaction; `ScratchRemoved` does not release it.
29. A restore or replay probe refused during a capture's scratch, and a capture
    aborted by a restore filling the volume (R2), leave no scratch and no registered
    root; the capture is not wedged afterwards.

Budgets, restarts, observability
30. `MaxDuration` ending the copy releases the transaction and removes the scratch;
    ending the witness or upload leaves no chunk registered, and a witness aborted
    by the budget uploads and probes nothing (the existing W1 test,
    `snapshot_witness_first_test.go`, rewritten for the new order; the W2 test about
    the witness sharing the transaction is deleted with the mechanism it pinned).
31. `inFlight` is true from before the copy until `Take` returns, and a planned
    restart is refused during the witness (`planned_restart.go:566-568`).
32. Context cancellation during the copy releases the transaction, closes the file
    and removes the scratch.
33. `DurationSeconds` in the persisted capture record is the whole capture's and
    `snapshotCaptureFloor` still reads it (consequence, stated: it now includes the
    copy, so replay-time captures are spaced further apart than after an old
    capture of the same store; the field is not split); a record written by the new code is read
    by the old reader (an extra field is ignored; here none is added).
34. Observability: `snapshot_producer` reports `last_pin_seconds`,
    `last_copy_seconds`, `last_witness_seconds`, `last_upload_seconds`,
    `last_scratch_bytes`, `last_pin_wal_growth_bytes` and `last_pin_events` after
    a capture and zeros before the first; `last_detail` uses the word "pin" for
    the new value and never the old "read transaction", so the two are not
    conflated by a reader.
35. The scratch directory name equals the previous binary's cleanup name
    (`snapshotScratchDirName == "snapshot-tmp"`).
36. Geometry: a page size that does not divide `ChunkSize` or a zero page count is
    refused before any byte is written (`snapshot.go:669-671`).
37. The hash is inline: a slow hasher (the injected `hash.Hash`) lengthens `PinTime` by its own
    time and holds no extra buffer beyond the write buffer (no queue to bound).
38. A flush happens before each sync (an injected writer records that `Sync` is
    never called with unflushed buffered bytes).

Added in review round 2
39. A return between the token (step 1) and `Take` (a free-space or `page_count`
    read error, the `spendGuestAttempt` refusal, a configuration error, a panic)
    gives the token back: a compaction waiting on it proceeds, and the next
    capture runs.
40. A `ReadReleased` hook that blocks past `MaxDuration`: `Take` returns, `inFlight`
    is false, the attempt's outcome is unchanged (the timeout is logged only).
41. A staged compact copy present and `free < need`: nothing removed, the refusal is
    visible and names the amounts (failure 26's refusal case, kept as its own entry
    for the status fields).
42. A compaction waiting on the token while a capture holds it, cancelled by its
    context, returns without taking the token and without disturbing the capture;
    when it is not cancelled it proceeds when the capture releases, and its own
    copy budget starts after the wait.
43. Many consecutive refusals because compaction holds the token leave the failure
    counter and the backoff untouched (`snapshotRefused` does not reach them).
44. A copy whose first 64 MiB already leaves free space under the floor fails at the
    first check.
45. After the read-only witness closes, `state.db-wal` beside the copy is zero length
    or absent before the upload's digest compare.
46. A page that arrives out of order or short is refused by name (the existing
    check, `snapshot.go:725`).
47. A copy that exhausts `MaxDuration`: the witness fails by `ErrTimeBudget`, nothing
    is uploaded, the scratch is gone (the completion floor is R5).

## 4. Class, surfaces, rollback, proof

- **Mutation class: red.** The change writes the base every restore starts from
  and the record that names it on the tape (`snapshot_recorded`,
  `snapshot.go:365`), and changes disk use on every computer's data volume. Orange
  behaviour with a red surface (tape and evidence; the snapshot is part of the
  restore chain; checkpoint and route projection are not touched). Two review seats
  before landing; the author is not one (`CLAUDE.md`, Review).
- **Conjecture delta.** Today: a capture's head, pages and witness are one state
  because one read transaction produced all three. New: they are one state because
  the pages were copied in the transaction that read the head, the copy's page
  geometry and ten head columns are checked equal before the witness, and a digest
  taken at write time is checked at upload. The witness on the copy is also the
  install-time check.
- **Protected surfaces.** Snapshot capture and `snapshot_recorded`; the guest data
  volume and its free-space floor; the store's WAL; compaction's staging file and
  its token (the staged copy is read for its size only); telemetry partition GC (the hold);
  the boot sequence in `autoputer.Run` (one added call). Unchanged and listed so a
  reviewer does not look for change: the record, the chunk cut and sealing, the file
  CAS, `Restore`, `Install`, `VerifySnapshotFile`, the checkpoint bind, vmctl, the
  platform's snapshot roots.
- **Heresy delta.** The "no temporary copy" and "not shortened" comments are
  retired. Nothing is added as an off switch; the free-space and exclusion refusals
  are correctness refusals (`CLAUDE.md`, Delivery). No new limit on any model call.
- **Rollback.** Revert the commit. The record is unchanged, so an older binary
  restores a base the new binary wrote (`restoreOne` reads the same fields,
  `snapshot.go:989-1031`). A scratch left by a new binary is under the old name and
  the old attempt start removes it (`snapshot_producer.go:553`), after its own
  free-space refusal; on a volume below the old floor, an operator deletes
  `snapshot-tmp` (section 2.8, 2.2). No persisted field is added.
- **Expedite.** Every non-docs landing in this lane carries `Ring-Expedite:` with
  the defect it unblocks (the pinned WAL on the largest computers);
  `internal/agentcore/` is already an expedited deploy path (`CLAUDE.md`, Delivery).
  Stage one ships no code to a computer.

### What d@e can and cannot prove

**d@e is Linux, and the cache drop and the digest's re-read rely on it**
(`cache_other.go` is a no-op elsewhere, so on a non-Linux build the digest compare
is weaker because the upload may read the page cache, not the disk; the proof is
on the Linux guest only). d@e's store is about 1.1 GB (the set-aside design's figure; not re-measured). That
is two orders of magnitude under the computers this lane is for. d@e therefore
proves **correctness** (the copy is the head, the witness matches, restore matches,
the scratch is removed, crash and space paths work, appends continue). It does
**not** prove that the pin is short at tens of GB: on a store this small the copy
plus fsync plus hash can take longer than the old witness plus stream, and the pin
may be longer than before. Nothing below predicts otherwise on d@e. Constructing a
large store on d@e is not done (it would mutate a live computer, and its disk size
is unknown to me). The size effect is first seen on the largest computer's next
capture, read-only from the console; the build-VM bench of step 8 below is the one
measurement that can fail before the deploy and is a recorded precondition of it.

### Deployed proof

Metric (all in `GET /api/runtime/observability`, `snapshot_producer`, added by
failure 34): `last_pin_seconds` is the primary metric, the interval with an open
reader on the state store, timed by `capture` independently of `last_copy_seconds`.
`last_pin_wal_growth_bytes` (the `-wal` size at release minus at the pin's start)
and `last_pin_events` (head sequence at release minus the capture's head) give WAL
growth normalised by the write rate: bytes of WAL per event during the pin. The
existing `wal.last_checkpoint` (`wal_bound.go:101`; reason "snapshot capture")
can be overwritten by a compact copy that runs after, so it is read at once or
not used; the new fields are the record.

On d@e (correctness; read-only reads plus the capture's own writes):
1. Before the deploy, read `last_detail` (or, if the process restarted since, the
   `runtime: snapshot taken:` log line of its newest capture, which carries the same
   text, `snapshot_producer.go:608`, `:843-846`) and `wal` of d@e **and of the
   largest computer (read-only)**: the old read transaction, witness and stream
   (the old wording "read transaction"). These are the baselines for the claim.
   A computer with no recorded old capture has no baseline and the claim for it is
   not made.
2. After the deploy, two captures occur naturally (the producer's `due` and
   `due_why` say when; no computer is forced). For each: `last_witness_seconds`
   nonzero, `last_scratch_bytes` equal to `page_count x page_size` of that head,
   `data_volume` free bytes back within the WAL's size after, head sequence
   advanced during the capture.
3. A restore to a checkpoint above the second capture:
   `choir computer restore --host=<host> --computer=<d@e id>
   --checkpoint-file=<file>` (`cmd/choir/commands.go:462`; use in
   `cmd/choir/main_test.go:1029`); the response must contain `"witness_matched":true`.
4. Adversarial drill on d@e only: kill the runtime mid-copy, then mid-witness: the
   next boot removes the scratch before the rebase, and the next capture takes.
   Two kills at most between normal attempts (`snapshotAttemptLimit` 3,
   `snapshot_producer.go:134`).
5. Append rate during the pin, with a baseline and a tolerance: sample
   `head_sequence` every 10 s for the hour before and through each pin and
   checkpoint; the events per second inside the pin and inside the checkpoint
   must be at least 0.8 of the preceding hour's mean. (0.8 is inferred, flagged for
   the seats; no append-latency field exists in the observability block, which is
   why the rate is used.)
6. Whatever `last_pin_seconds` reads on d@e is recorded next to the old read
   transaction; no direction is predicted for it there.

On the largest computer (read-only, the console only), at its next capture after
the deploy:
7. `last_pin_seconds` at most half of the pre-deploy read transaction seconds of
   the **same computer** recorded in step 1 (primary). Secondary, reported and not
   decisive: `last_pin_seconds <= 0.5 x last_witness_seconds` of the same capture
   (the new witness runs on a copy whose cache was dropped, so it may be slower than
   the old one on a warm store and flatter the ratio), and
   `last_pin_wal_growth_bytes / last_pin_events` within 1.5x of the per-event
   growth of the last old capture. The 0.5, 1.5x and 0.8 of step 5 are proof
   thresholds chosen by the author, not runtime limits and not measured.
8. **Precondition of the deploy, recorded before it:** the build-VM bench on a
   synthetic store reports the copy rate in
   MB/s of the writer as built (read, inline hash, buffered write, sync). Two
   acceptance rates, from the code's own numbers, computed per S at the time:
   completion, `rate >= S / (MaxDuration - witness - upload)` (with the 90 minutes
   of `snapshot_producer.go:80`, a 35 minute witness from the code comment `:72-79`
   and a 3.5 minute upload from the set-aside design, S = 39.6 GB gives about
   12.8 MB/s; inputs unverified), below which a capture never completes; and no-worse
   pin, `rate >= S / (pre-deploy read transaction of that computer)` (about 17.5
   MB/s on the same inputs), below which a capture completes with a longer pin than
   today; a third rate is the abort band's (2.5). The bench needs `S_bench` (the
   store) plus `S_bench` (the scratch) plus the 2 GiB floor free on its volume: the
   10 GB first named here cannot run on a VM with about 16 GB free (10 + 10 + 2.1 =
   22.1 GB), so the bench size is `(free - 2.1 GB) / 2` rounded down, and the rates
   are the bench's MB/s applied to the target computer's own S, not to the
   bench's. The bench is on a build VM, not a guest disk; a pass is necessary, not
   sufficient. Its results are recorded in section 9.
9. **Rollback trigger:** if the largest computer's first capture after the deploy
   fails, is refused (R4's band included), is admitted and aborted, or its
   `last_pin_seconds` exceeds its pre-deploy read transaction, then either revert
   (the record is unchanged and the revert restores today's behaviour) or record R4
   as the result and stop: the owner's decision, made when the first outcome is
   read, not a timer. A refusal produces no measurement, so it is itself the
   outcome; the trigger does not wait for one.
**Precommitted claim, re-stated so that it can fail** (entered in
`docs/calibration/claude-ledger.md` **before the deploy**, as the contract's
"precommit, then score" requires; settle-by: the largest computer's first due
capture after the deploy, or the owner's stop under step 9): on the largest
computer's first capture after the deploy, `last_pin_seconds` is at most half of that
computer's pre-deploy read transaction seconds recorded in step 1. Probability 0.5.
It fails when the guest disk's effective copy rate is below about
`S / (0.5 x pre-deploy read transaction)`; the threshold is computed at the landing
from `last_scratch_bytes` and the recorded baseline, not from the unverified S of any
document (the code comment says 21 GiB, the set-aside design 39.6 GB). A separate
claim for d@e, probability 0.9: the two captures after the deploy both record,
`witness_matched` is true, and the scratch is gone after each. The outcomes are
fields of the deployed observability block and a response of the restore command;
the author cannot write them. Neither claim says the change helps on a store of any
particular size before the bench and the largest computer's capture have run.

## 5. Open questions, residuals and the conservative default

Residuals (named, not solved here)
- **R1. A computer with `floor <= free < S` takes no snapshot** (2.5, failure 17).
  Consequence: its tail grows to the ceiling and then toward the recovery bound,
  visible as `refusal` and `consecutive_refusals`. Today it captures. It is the
  price of a local copy; the data-volume bound (the set-aside design's T7) is the
  guard. Default: accept, say so in the report, do not add a fallback to the
  in-transaction path (that would keep both mechanisms).
- **R2. A restore or rematerialize during a capture's scratch** competes for the
  volume; rematerialize staging does not admit by free space (2.5). Default: the
  capture's own guard aborts the capture first; no mechanism added.
- **R3. Partition pins** are unchanged and unmeasured (2.7). Default: a separate
  lane after the state pin is measured.
- **R4. A staged compact copy can make the largest computer's captures refuse.**
  The band: `free < S + floor` while `state.db.compact` sits beside the store
  (compaction stages "for the next planned restart", `compact.go:11-18`, and keeps
  it until one). On the set-aside design's unverified figures the margin is about
  2 GB (2.5). Consequence: captures refuse, or are admitted and aborted in the band
  of 2.5 (visible in `refusal` or in `last_detail`) until a planned
  restart consumes the staged copy or compaction's next pass drops it, and the tail
  grows meanwhile (R1's consequence). The d@e proof does not cover it. Default:
  accept; the capture never deletes `state.db.compact`. A drop-the-staged-copy
  policy, if the owner wants one, is its own lane: reviewers found that the capture's
  deletion has no replacement (unlike `compact.go:312-316`, which deletes the stale
  copy because it is about to write a fresh one), can starve compaction and waste a
  full-store VACUUM per pass, suppresses partition GC while the file exists
  (`telemetry_partition.go:756-765`), and could void a pending planned-restart
  rebase (`PlannedRestartMarkerPending`, `run.go:213`).
- **R5. The pin can be longer than today's, and a capture can fail to complete.**
  `MaxDuration` (90 minutes) now bounds copy plus witness plus upload, not witness
  plus stream. Below the completion rate of proof step 8 a capture never completes
  (each attempt a pin of up to 90 minutes and nothing uploaded); between that rate
  and the no-worse rate it completes with a longer pin. The bench and the rollback
  trigger of section 4 are the guards; no per-phase budget is added. The outer bound
  of an attempt is not `MaxDuration`: the hook's fresh `MaxDuration` lets `Take`
  return up to about twice `MaxDuration` after the pin starts in the worst case, and
  `inFlight` (so a planned restart) waits that long.
- **R6. A disk-image fork or VM checkpoint while the scratch exists** would carry it.
  `SnapshotInFlight` is read by restart-class paths only as far as I could verify
  (`planned_restart.go:567`, `clean_rebase.go:96`, `capsule_pause.go:247`,
  `memory_cache_return.go:279`; a `vmctl/owner_update.go:191` cite in an earlier
  draft matches no reader in `internal` or `cmd` and is dropped); whether a fork or
  checkpoint consults it I did not establish. Default: named, not solved.
- **R7. The digest covers write to upload, not write to witness.** A transient read
  corruption seen only by the witness is not detected by it. Remote; recorded.

Questions
1. **Staged compact copy.** The capture refuses and never deletes it (R4). Round 2
   reversed the round-1 default.
2. **Partitions:** unchanged (R3).
3. **Copy budget:** deleted; `MaxDuration` bounds the whole capture (2.9).
4. **Write-time digest:** included, inline (2.2); the first thing to delete if a seat
   finds the remaining failure (a disk fault between copy and upload) too remote.
5. **`Sync` (`fsync`) and cache drop every 64 MiB, no throttle:** default as in 2.2; if
   the proof's step 5 shows the rate dropping, the answer is a smaller interval, not a
   rate limit.
6. **Headroom constants:** deleted (2.5).
7. **Exclusion shape:** the in-process token with a blocking and a non-blocking
   entry (2.6).
8. **Read-only versus read/write open of the copy:** read-only; failure 8 pins the
   equality.
9. **Name `snapshot-tmp`:** kept, with the rollback caveat (2.2, 2.8).
10. **`inFlight` over the whole `Take`:** kept.
11. **`CHOIR_SNAPSHOT_DISABLE` and `CHOIR_SNAPSHOT_MAX_DURATION`:** existing, not
    touched (`snapshot_producer.go:56-58`, `:163-181`).
12. **Ledger entry:** at the landing; the claim is in section 4.
13. **Page-cache pressure on small guests during the witness:** after the witness the
    copy's whole range is dropped (`dropSourceCache`, `snapshot.go:621-626`) and
    ranges while uploading; none during the witness *(inferred)*.
14. **The pending-byte page (a store past 1 GiB)** and page sizes other than 4096 are
    covered by failure 9 only on the build VM; d@e's store (about 1.1 GB) is just
    past 1 GiB, so its capture is also that test in production.

## 6. What the set-aside design and the reviews have wrong, against the code

- "A planned restart waits only for the copy (the existing `inFlight`)" is wrong:
  `inFlight` is set around the whole `Take` (`snapshot_producer.go:559-562`).
- A first-round review gave `snapshot_producer.go:553` as precedent for cleaning
  before the free-space check. It is the opposite: the legacy removal runs after the
  free-space refusals (`:527-543`) and after `spendGuestAttempt` (`:547`). The other
  review's "boot deletes ... as `snapshotLegacyTempDirName` does today" is not
  today's behaviour either: it is removed at an attempt, not at boot, and the boot
  sweep that exists (`runtime.go:1278`) is for the replay probe and runs after the
  rebase, open and replay (`autoputer/run.go:213`, `:236`, `:900-910`, `:786`).
- My own first draft said `Runtime.Start` runs "before boot-time disk users
  (rebase, restore staging)". That was false for the rebase, the store open and the
  replay; fixed (2.4).
- The WAL headroom rule today applies to the replay trigger only
  (`snapshot_producer.go:536-543`). The set-aside design says it is "computed from
  the copy's duration"; it is deleted here, and the rule `S + floor` is a change for
  all four kinds.
- `wal_bound.go:28-31` and the producer and snapshot headers state the opposite of
  the new design; they are in the delete list (2.9). The set-aside design does not
  list the `wal_bound.go` one.
- "Record, restore and `VerifySnapshotFile` are unchanged" holds, but nothing in
  restore compares the file's head row with `StoreHeadSequence` (1.8): the head tie
  must be made at capture, on every column.
- The set-aside design and my first draft said rematerialize and the replay probe
  both admit by free space. Only the replay probe does (`replay_scratch.go:124`,
  `replay_completeness.go:302`); `rematerialize.go:93` does not.
- The set-aside design's "2 x 42 GB against 83.9 GB" and my first draft's
  "`S + staged + floor + headroom` against the volume" both count the staged file
  twice: measured free space is net of it (2.5).
- The existing drop is of the *source* (`snapshot.go:612-626`, `:733`); the scratch
  needs its own (2.2).

## 7. Review round 1: findings and dispositions

Two seats (Claude: REPAIR; DeepSeek v4.1 flash: REPAIR). Both found the mechanism
sound. Where a review and the code differ, the code is cited.

### Blocking (all five closed)

| # | Finding | Disposition | Where |
| --- | --- | --- | --- |
| B1 | Token after cleanup and after measurement; compaction races | Adopted. Token first, then cleanup, then measure; held for the whole of `evaluate` after step 1 (round 2 corrected the first draft's "to scratch removal"); refused token deletes nothing; compaction takes it before `removeSQLiteFiles(staging)` (`compact.go:267-269`) and holds to rename/remove; `removeSQLiteFiles` for sidecars | 2.4 steps 1-3 and 7, 2.6; failures 19, 20, 28 |
| B2 | `ReadReleased` only on the happy path; cancelled context | Adopted. Fires from the rollback closure once on every path (`snapshot.go:538-546`), with `context.WithoutCancel`; runs in its own goroutine so the witness does not wait; joined before `capture` returns | 2.4 step 5; failures 13, 14 |
| B3 | Proof metric cannot fail; d@e small | Adopted. Primary metric is `last_pin_seconds`, independently timed; WAL growth per event; d@e is correctness only, size effect on the largest computer, read-only; claim restated | 2.2, 4 (two subsections); failure 12, 34 |
| B4 | Boot cleanup after rebase/open/replay | Verified true in `internal/autoputer/run.go:213` (rebase), `:236` (open), `:786` (`rt.Start`), `:900-910` (replay). Scratch removal moved to the first boot step; `Start` second defence | 2.4 "Boot"; 2.8; failure 22 |
| B5 | Head tie sequence-only in the tests | Adopted. Ten fields named in 2.3 and failure 2; test helper compares the whole struct, nil handled; lane test updated | 2.3; failures 2, 11; test 0 |

### Should-fix (both seats)

| Finding | Disposition |
| --- | --- |
| Hash goroutine can extend the pin / backpressure (both) | Adopted: inline; the extra pin is `S / 450 MB/s` (88 s at 39.6 GB, build VM), named. 2.2, failure 37 |
| Nil head (Claude 2) | Adopted. 2.3, failure 11, test 0 |
| Test 0 pacing and production pragmas (Claude 3, 4) | Adopted: paced on writer progress; second configuration with default autocheckpoint, concurrent PASSIVE checkpoints and a freelist. Not covered and added to the list: `incremental_vacuum`, page sizes, 1 GiB (failures 8, 9) |
| Seams the list assumes (Claude 5) | Adopted: named at the head of section 3 |
| Callers of `Config.Witness` (Claude 6) | Adopted: named in 2.9 |
| Recovery paths need the same space (Claude 7) | Adopted as R2 plus failure 29; the rematerialize claim corrected (DeepSeek, verified `rematerialize.go:93`) |
| `ReadTxTime` meaning changed; do not reuse "read transaction" (Claude 8) | Adopted: `PinTime`, `last_detail` says "pin". Failure 34 |
| Proof step 6 needs baseline and tolerance (Claude 9, DeepSeek) | Adopted as the event rate against the preceding hour, 0.8 (inferred; there is no append-latency field) |
| Crash drill vs the 3-start limit (Claude 10) | Adopted: at most two kills. 2.8, 4 step 4 |
| New disk requirement `free < S` is unstated (DeepSeek) | Adopted as R1 with its consequence and failure 17 |
| Staged-copy arithmetic double counts (DeepSeek) | Adopted. 2.5 |
| Staged-copy default: refuse (Claude, DeepSeek both against) | Adopted: remove only when it is why `free < need`. Reversible; flagged question 1. **This is the one place I went beyond "repair text only"**: both reviewers asked for the policy and it adds one branch to an existing step, not a mechanism **Reversed in round 2 (section 8, finding 1): the capture never deletes the staged copy.** |
| `ReadReleased` blocks appends on the write connection (DeepSeek) | Adopted: stated in 2.4, measured by proof step 5, failure 15 |
| Read/write open of the copy; stale in-header size, shrink (DeepSeek) | Adopted: failure 8 |
| Witness equality under writers, not at quiet head (DeepSeek) | Adopted: failure 7 |
| Compaction can lose a 24 h pass (DeepSeek) | Adopted: compaction waits for the token (2.6). Claude's "skip" in the first draft is replaced |
| Rollback claim "no operator step" false (DeepSeek, Claude's rollback caveat) | Adopted: removed; 2.2, 2.8, 4 |
| `copyHeadroom` fallback `max(...)` (DeepSeek) | Rejected as moot: the headroom is deleted (Claude verdict 6; it protected against an upload abort, which no longer exists) |
| Flush before `fdatasync`; cache drop no-op off Linux (DeepSeek) | Adopted: 2.2, failure 38 |
| Proof command unverified (DeepSeek) | Adopted: `choir computer restore --host --computer --checkpoint-file` (`cmd/choir/commands.go:462`, `main_test.go:1029`) |
| `wal.last_checkpoint` can be overwritten (DeepSeek) | Adopted: the new fields are the record (4) |
| Copy budget 30 min invented; 3.75 GB arithmetic (Claude 3; DeepSeek 3) | Adopted: budget deleted; `MaxDuration` stays the one deadline |
| Test 0 numbers stale: 54 commits vs 1,294 (DeepSeek) | Adopted: the count is machine dependent; section 2.1 states it and the test paces on progress |
| "37 minutes on a 38.8 GB store" conflicts with the code's 21 GiB, 22-35 min (DeepSeek) | Adopted: the doc cites the code's comment (`snapshot_producer.go:72-80`) and calls the set-aside figures unverified |
| "Bounded below by the read rate" loose (DeepSeek) | Adopted: 2.7 says the slower of read, write and hash |
| Drop `last_partition_pin_max_seconds` (Claude) vs add it (DeepSeek) | Claude's: dropped, R3. The proof does not need it; the lane that touches partitions measures them |
| Drop the persisted copy-seconds field and failure 28 (Claude) | Adopted: removed with the headroom; failure 33 keeps `DurationSeconds` whole-capture |
| Tick-level cleanup before `Ready` duplicates others (Claude) | Adopted for the not-due path (removed); the cleanup runs at step 2 of an attempt, at boot and at `Start` |
| Copy seconds vs capture seconds in `snapshotDue` (Claude 6) | Adopted: failure 33 |
| Missing entries 1-17 (DeepSeek) and Claude's 6 list | Covered by failures 2, 5, 7-9, 13-15, 17-29, 33, 37, 38; the `incremental_vacuum` and page-size cases are in 8 and 9 |
| A crash drill at each witness and upload step on d@e (Claude 6) | Partly: mid-copy and mid-witness (4 step 4). Mid-upload adds nothing new on d@e (the scratch is already complete); not added |
| A reviewer may want the staged-copy removal in a separate lane (Claude dissent) | Not taken: both seats asked for the policy in this lane; flagged question 1 **Reversed in round 2 (section 8, finding 1): the capture never deletes the staged copy.** |

### Open-question verdicts

1 staged copy: both disagreed with refuse; adopted (above). 2 partitions: agree.
3 copy budget: delete (Claude) adopted; DeepSeek "deadline, 25 min" not taken (invented).
4 digest: agree, inline. 5 fdatasync: agree with the flush fix. 6 headroom: delete.
7 token: agree, with the order. 8 read-only: agree. 9 name: agree with the caveat.
10 `inFlight`: agree. 11 switches: agree. 12 ledger: agree, restated. 13 cache drop:
agree.

## 8. Review round 2: findings and dispositions

Two seats (Claude: REPAIR; DeepSeek v4.1 flash: REPAIR). Both closed all five
round-1 blockers. This is the last text round; nothing was added in it: every
edit below removes behaviour, tightens a sentence or names a test.

### Blocking

| # | Finding | Disposition | Where |
| --- | --- | --- | --- |
| 1 | The capture deleting `state.db.compact` was itself a second mechanism with no replacement (both seats; `compact.go:312-316` deletes because compaction is about to write a fresh copy) | Reversed. The capture never deletes `state.db.compact` or its sidecar; it refuses when `free < S + floor`, naming `free`, `need` and the staged size by `os.Stat`. R4 names the band and the consequence. The "safe subset" (delete a staged copy the boot would drop anyway) is **declined**: one mechanism. Capture and compaction exclude each other for the length of the other's copy, stated | 2.5, 2.6, 4 (protected surfaces), 5 (R4, question 1); failures 26, 41 |
| 2 | Token released only by `ScratchRemoved`, which exists only inside `capture`; leaks on every earlier return (DeepSeek B1) | `release, holder, ok := TryHoldStoreCopy(...)` then `defer release()` at acquisition, idempotent; `ScratchRemoved` is only a scratch hook; the token is held until `evaluate` returns | 2.4 step 1 and 7, 2.6; failures 28, 39 |
| 3 | `ReadReleased` uncancellable and joined with no bound (DeepSeek B2) | Uncancellable by the caller, bounded by `cfg.MaxDuration` as a fresh timeout from the hook's start (no new constant); on timeout log and return, never fail the attempt; `capture` joins at most that long; the hook's start is outside `PinTime` | 2.4 step 5; failures 14, 40 |
| 4 | Claim baseline: the new witness is on a cache-dropped copy, so a witness-relative claim can pass without improvement (Claude 2) | Primary claim: pin at most half of the same computer's pre-deploy read-transaction seconds, recorded before the deploy for d@e and for the largest computer (read-only); witness-relative figure secondary; the build-VM bench is a recorded precondition with two acceptance rates; a rollback trigger is stated | 4 (steps 1, 7, 8, 9, claim) |

### Should-fix, missing entries, verdicts

| Finding | Disposition |
| --- | --- |
| Proof thresholds 0.8, 1.5x, 0.5, 50 ms, 30 s unexplained (Claude) | Adopted: labelled proof thresholds, not runtime limits (section 4 step 7; 50 ms is gone with failure 12's rewrite) |
| Other readers of the volume while the scratch exists; fork/checkpoint refused through `SnapshotInFlight` (Claude) | Partly: `SnapshotInFlight` readers named; that a fork or checkpoint consults it is not established by those lines, so it is residual R6, not a claim |
| Hook start relative to the pin timer (Claude) | Adopted (2.4 step 5, failure 14) |
| Hash covers write to upload only (Claude) | Adopted as R7 |
| Weaker digest on non-Linux (Claude) | Adopted: stated; the proof is Linux only (section 4) |
| Failure entries: staged copy with nothing removed; compaction wait cancelled; consecutive refusals; first 64 MiB below floor (Claude) | Adopted: failures 41, 42, 43, 44 |
| Crash between staged-copy removal and sidecar removal (Claude) | Moot: the capture removes neither |
| Failure 12's 50 ms probe depends on timing (Claude) and `TRUNCATE` changes the WAL (DeepSeek) | Adopted: order-based, PASSIVE after a write behind the reader |
| Failure 19 needs a seam (Claude, DeepSeek) | Adopted: the `afterGates` seam, plus the token pre-held for the atomic property |
| Failure 22 needs an injection to simulate low free space (Claude, DeepSeek) | Declined as a unit test: no seam I could name simulates a full volume for the rebase; the test pins the boot phase ORDER and the d@e drill covers the rest (failure 22 says so) |
| Failure 37 needs a hasher seam; 18/26 need a free-space seam keyed on file state and a sparse `.compact` (DeepSeek) | Adopted (seam list, failures 26, 37) |
| `MkdirAll` placement (DeepSeek 6) | Adopted (2.4 step 2) |
| Serial-stage arithmetic: "lower bound is the slower of read, write and hash" is false (DeepSeek 2) | Adopted: 2.7 says the sum of serial stages |
| Completion bound of `MaxDuration` now shared (DeepSeek 3) | Adopted as R5 and the bench's two rates (no per-phase budget) |
| The bench is the only pre-deploy size evidence; state its acceptance (DeepSeek 4) | Adopted (section 4 step 8) |
| Token priority: a compaction can refuse a replay-triggered capture (Claude (b), DeepSeek 5) | Adopted: named in 2.5, no priority rule (it would be a second mechanism) |
| `DurationSeconds` grows by the copy and spaces captures further (DeepSeek 8) | Adopted: stated under failure 33 |
| Staged size by `os.Stat`, not the sidecar (DeepSeek 9) | Adopted (2.5) |
| Keep the `FileTreeRoot` unset sentence (DeepSeek 10) | Adopted: it is in section 2.3's list of instants as the one field that is unset in production (`snapshot_producer.go:799-826`); no new text needed |
| Missing: `-wal` empty after witness close; zero or out-of-order page; whole capture fits `MaxDuration` (DeepSeek 3, 4, 5) | Adopted: failures 45, 46, 47 |
| Missing: a staged copy with no readable sidecar deleted without arithmetic (DeepSeek 6) | Moot with finding 1 |
| Missing: compaction wait's own budget starts after the wait; boot replay also succeeds (DeepSeek 7, 8) | Adopted in failure 42; the boot replay claim is in failure 22's wording as an order-and-drill claim |
| Q12: the 0.5 threshold depends on an unverified S (DeepSeek) | Adopted: computed at the landing from `last_scratch_bytes` and the recorded baseline |

Open-question verdicts: 1 reversed (refuse, never delete); 2 partitions, 3 no copy
budget (with R5), 4 inline digest, 5 sync and drop, 6 no headroom, 7 token with the
`defer`, 8 read-only open, 9 name `snapshot-tmp`, 10 `inFlight`, 11 env values
untouched, 12 ledger with the new baseline, 13 cache drop, 14 lock-byte page: all as
in round 1, which both seats agreed with.

## 9. The build-VM bench (precondition of the deploy, section 4 step 8)

Run 2026-11-15 on `choir-agents` (a 4 GiB build VM, not a guest disk), with
`TestBenchCopyRate` (`internal/snapshot/snapshot_bench_copy_test.go`, skipped
unless `SNAPSHOT_BENCH_BYTES` and `SNAPSHOT_BENCH_DIR` are set), through the real
`TakeSnapshot` with a chunk store that keeps nothing. The size is the section 4
rule `(free - 2.1 GB) / 2`: the volume had 14 GB free, so about 6 GB, not the 10 GB
first named (that needed 22.1 GB).

| store | pages | copy (page loop, inline hash, write, sync) | pin | witness | upload |
| --- | --- | --- | --- | --- | --- |
| 6,007,803,904 bytes | 1,466,749 | 41.0 s = **146 MB/s** | 41.0 s | 24.1 s | 17.3 s = 348 MB/s |

Applied to the paper's S = 39.6 GB (the set-aside design's figure, unverified), the
copy would take about 271 s (4.5 minutes) at 146 MB/s. The three rates of step 8
and 2.5, on the same unverified inputs: the completion rate (about 12.8 MB/s), the
no-worse rate (about 17.5 MB/s) and the abort-band rate (about 37 MB/s) are all
under 146 MB/s, so on this VM's disk the bench passes all three. It is necessary,
not sufficient: a guest disk may write far slower than this VM's, and the synthetic
store is one 4 KiB random page then zero pages per megabyte, so its witness (24 s)
is cheap and says nothing about the paper's 22 to 35 minutes. On this store the
pin (41 s) is longer than the witness (24 s); the claim of step 7 is against the
computer's own pre-deploy read transaction, not against this witness. One run; no
repeat.

## 10. Failure list to tests (stage two)

Tests are in `internal/snapshot/snapshot_static_copy_test.go` (S),
`internal/snapshot/static_copy_consistency_test.go` (C, the lane's test 0),
`internal/agentcore/snapshot_producer_copy_test.go` (P),
`internal/store/store_copy_test.go` (T) and
`internal/autoputer/boot_scratch_order_test.go` (A). "Existing" names a test that
was already in the tree and pins the preserved property.

| # | Test |
| --- | --- |
| 1 | C `TestStaticCopyIsAConsistentImageOfThePinnedHead`; existing `TestWritesInsideTheStreamStayOutOfTheSnapshot` |
| 2 | S `TestACopyWithADifferentHeadColumnIsRefused` (ten columns) |
| 3 | S `TestACopyOfAnotherSizeIsRefused` |
| 4 | none: the order and count checks are reused from the old stream (`page stream out of order`, `the page stream held N pages`) and no seam can make `sqlite_dbpage` lie |
| 5 | S `TestAByteChangedAfterTheCopyOrTheWitnessIsRefused` |
| 6 | S `TestTheUploadFromTheCopyIsTheLivePageSequence` (restore with the witness check); `tailReceipt` is existing |
| 7 | S `TestTheCopyOpenedReadWriteIsUnchangedAndWitnessesAlike` (copy equals live at a quiet head); under writers, existing `TestWritesInsideTheStreamStayOutOfTheSnapshot` (the in-transaction `DigestsTx` it would be compared to no longer exists) |
| 8 | S `TestTheCopyOpenedReadWriteIsUnchangedAndWitnessesAlike`, `TestAnIncrementallyVacuumedStoreCopiesAndRestores` |
| 9 | S `TestOtherPageSizesCopyAndRestore` (512, 8192, 65536), `TestAStorePastOneGiBCopiesAndRestores` (skipped under `-short`) |
| 10 | S `TestTheUploadFromTheCopyIsTheLivePageSequence` |
| 11 | S `TestANilHeadIsEqualOnlyToANilHead` |
| 12 | S `TestThePinEndsBeforeTheWitness` (order-based probe) |
| 13 | S `TestReadReleasedFiresOnceOnEveryPath` (success, head failure, cancel, budget, ENOSPC, EIO, cross-check) |
| 14 | S `TestTheWitnessDoesNotWaitForTheReadReleasedHook` |
| 15 | S `TestThePinEndsBeforeTheWitness` (a write during the witness); existing `snapshot_nohold_test.go` |
| 16 | existing `TestSnapshotCarriesPartitionsAndDedupsClosedOnes` and the partitions tests |
| 17 | P `TestEveryTriggerKindNeedsTheStoreSizePlusTheFloor` |
| 18 | P `TestALeftoverScratchIsRemovedBeforeFreeSpaceIsMeasured` |
| 19 | P `TestAHeldTokenRefusesBeforeAnythingIsDeletedOrMeasured` (with the `afterGates` seam); T `TestCompactionWaitsForTheTokenAndTouchesNothingWhileItWaits` |
| 20 | T `TestTheCaptureCleanupNeverTouchesTheStagedCopy` |
| 21 | S `TestTheScratchIsRemovedOnEveryPath` (the directory and its sidecars) |
| 22 | A `TestTheScratchIsSweptBeforeTheRebaseTheOpenAndTheReplay` (source order; no full-volume boot is simulated), P `TestRuntimeStartSweepsTheSnapshotScratch`, T `TestTheBootSweepRemovesOnlyTheScratch` |
| 23 | P `TestAFailedCleanupFailsTheAttemptClosed`; S `TestAnExistingScratchFileFailsTheAttemptClosed` |
| 24 | S `TestFreeSpaceIsCheckedEveryScratchSyncBytes` (low and statfs error) |
| 25 | S `TestReadReleasedFiresOnceOnEveryPath/ENOSPC...`, `/EIO...` |
| 26 | P `TestAStagedCopyIsNamedInTheRefusalAndNeverRemoved`; T `TestTheCaptureCleanupNeverTouchesTheStagedCopy` |
| 27 | S `TestTheScratchIsRemovedOnEveryPath`, `TestTheScratchIsRemovedBeforeThePartitionsAreRead` |
| 28 | T `TestTheStoreCopyTokenIsExclusiveAndReleasesIdempotently`, `TestTheTokenSurvivesConcurrentUse`; P `TestTheTokenIsReleasedOnEveryReturn` |
| 29 | none: R2 (a restore filling the volume) cannot be simulated; the capture's own abort is failure 24 |
| 30 | S `TestMaxDurationEndsTheCopyAndTheWitnessWithNothingUploaded`; existing `TestABudgetAbortInTheWitnessUploadsNothing` |
| 31 | P `TestInFlightSpansTheWholeTake`; the planned-restart refusal is existing |
| 32 | S `TestReadReleasedFiresOnceOnEveryPath/cancel...`, `TestTheScratchIsRemovedOnEveryPath/cancellation` |
| 33 | P `TestThePersistedCaptureDurationIsTheWholeCapture` |
| 34 | P `TestTheStatusReportsThePinAndThePhases` |
| 35 | P `TestTheScratchDirectoryNameIsThePreviousBinarysCleanupName` |
| 36 | none: a fresh WAL store already has one page (a test tried and skipped), page sizes are powers of two up to 64 KiB and `ChunkSize` is 4 MiB, so no unusable geometry is reachable through SQLite; the check (`storeGeometry`) runs before any byte is written |
| 37 | S `TestTheInlineHashLengthensThePin` |
| 38 | S `TestSyncNeverRunsWithUnflushedBytes` |
| 39 | P `TestTheTokenIsReleasedOnEveryReturn` |
| 40 | S `TestABlockedReadReleasedHookIsBoundedByMaxDuration` (also `MaxDuration == 0`) |
| 41 | P `TestAStagedCopyIsNamedInTheRefusalAndNeverRemoved` |
| 42 | T `TestAWaiterIsCancelledWithoutTakingTheTokenAndAnotherProceedsOnRelease`, `TestCompactionWaitsForTheTokenAndTouchesNothingWhileItWaits` |
| 43 | P `TestRefusalsForAHeldTokenEscalateNothing` |
| 44 | S `TestFreeSpaceIsCheckedEveryScratchSyncBytes` (the first check) |
| 45 | S `TestTheCopyOpenedReadWriteIsUnchangedAndWitnessesAlike` |
| 46 | none: as failure 4 |
| 47 | S `TestMaxDurationEndsTheCopyAndTheWitnessWithNothingUploaded/witness` |
