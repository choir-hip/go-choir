# Capture through a static copy: design section and failure list before code

Date: 2026-11-15. Stage one of the lane `lane/capture-static-copy`: no
production code. Class: red (section 4). Origin: item 2 of
`docs/problems/restore-bases-design-three-review-rounds-structural-assessment-2026-11-15.md`.
The set-aside package design
(`docs/design/restore-bases-store-once-2026-11-15.md`, "Landing 2") is read
here as a hypothesis; every claim about code below was read from the code, with
file and line. A claim marked *(inferred)* was not read or measured.

Rule for this lane (from the assessment): no mechanism beyond this one. The
only additions that are not the copy itself are the ones the copy cannot be
correct without: a cleanup that runs before the free-space measurement, an
exclusion with the compact copy (both are store-sized files on one volume), and
the metrics the proof needs. Section 5 lists what was left out.

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
3. **Experiment (this lane's first test).**
   `internal/snapshot/static_copy_consistency_test.go`, run on the build VM
   (Linux, go1.26.4, modernc sqlite from this module): a store whose main file is
   4,096 bytes while 17.6 MB sits in the WAL (autocheckpoint off, so the copy
   cannot be a read of the file), a writer committing constantly, a head read
   first and 2,008 pages copied with a 5 ms pause every 50 pages. 54 commits
   landed during the copy; the live head moved 600 to 654; none leaked. The copy
   opened with `dbx.OpenSQLite(..., ReadOnly: true)` held exactly 600 x 10 rows,
   its head row was 600, `quick_check` was `ok`, and the main file's SHA-256 differed from the copy's (so
   the WAL content did reach the copy). Opening, querying and closing the copy
   read-only left its SHA-256 unchanged. Two sidecars (`-wal`, `-shm`) were
   left beside the copy after close, so the cleanup must remove them with it. A
   copy missing its last page was refused when opened (`database disk image is
   malformed`), before any cross-check; a same-size copy of the wrong head is
   only caught by the cross-check below.

Alternatives, rejected on what the code says: `VACUUM INTO` renumbers pages (the
`compact.go` copy: "each B-tree written contiguously", `:388-391`), so a copy
never dedupes against live-file chunks; the SQLite backup API restarts whenever
another connection writes (`wal_bound.go:28-31`).

### 2.2 The write

- The scratch is a directory `snapshot-tmp` under the data dir (the same name
  the legacy cleanup uses, `snapshot_producer.go:137`), holding one file
  `state.db`. It is on the data volume, beside the store, never `/tmp`
  (`replay_scratch.go:17-23`). Created `0700`/`0600`, `O_EXCL`, in a directory
  that was removed first.
- Reusing the legacy name has one reason: after a rollback (section 4) the
  previous binary's attempt start removes it (`:553`), so a rollback needs no
  operator step for a scratch left by a new binary.
- Pages are written through a 4 MiB buffered writer. Every 64 MiB
  (`sourceDropBytes` is already that size, `snapshot.go:612`) the writer calls
  `fdatasync` and then drops that range from the page cache with the existing
  `adviseDropFileCache` (`internal/snapshot/cache_linux.go:14-21`). Reason
  *(inferred)*: `FADV_DONTNEED` drops only clean pages, 40 GB of dirty pages
  would fill a guest's memory, and a synchronous commit of the live WAL must not
  wait behind more than one 64 MiB writeback. The cost (sync per chunk) is
  measured in the d@e proof, section 4.
- Every 64 MiB the copy also calls `cfg.FreeSpace()` (section 2.5).
- A SHA-256 over the bytes written is computed in a second goroutine fed by the
  writer (so it does not extend the pin when the disk is the slower side). On the
  build VM Go's SHA-256 ran at about 450 MB/s single core, 2 GiB in 4.57 s,
  against 2 GiB written and fsynced in 3.65 s and read in 0.94 s *(build VM,
  one run, not a guest)*. The digest is checked again while the upload reads
  the file (2.4), so "the bytes uploaded are the bytes witnessed" is kept without
  a second pass over the disk. It is a mechanism only because the copy removed the
  one-transaction construction that gave the property for free; section 5 lists
  it as an open question.
- The transaction ends as soon as `pages == page_count` and the writer has
  closed the file. The "read transaction" metric then equals head read plus copy.

### 2.3 How the copy is tied to the head

Before the witness, on the opened copy:

1. `PRAGMA page_size` and `PRAGMA page_count` equal the values read in the
   transaction (`streamFile` reads both, `snapshot.go:662-667`), and the file
   size equals `pageSize x pageCount`.
2. The head row read from the copy equals the head read first in the
   transaction, on every column `Head` returns. The check calls `cfg.Head` with
   a read transaction of the copy's own DB, so it reuses `store.HeadTx` and adds
   no second query.
3. The pages written equal the transaction's `page_count` (today's equivalent,
   `snapshot.go:749`).
4. The copy is opened read-only (`dbx.OpenSQLite(path, {ReadOnly: true,
   ReadConns: 1})`, `dbx.go:99-130`). The experiment showed that this does not
   change the file.

A mismatch fails the attempt with the name of the check; nothing was uploaded,
nothing registered, the scratch is removed.

The witness is then `LiveStoreWitness(ctx, copyDB)`, i.e. the code
`VerifySnapshotFile` runs on an installed base (`restore_snapshot.go:243-246`).
The recorded witness and the install's witness are the same function over the
same bytes. (`VerifySnapshotFile` opens read/write; the capture opens read-only.
Both read the same pages; failure F7 pins the equality.)

### 2.4 Order of an attempt (new)

In `evaluate` (`snapshot_producer.go:432-`):

1. After `tick.TryLock` (`:436`), before `Ready` and before anything is
   measured: remove the scratch directory, and any `.compact-staging` file when
   no compaction is running (2.6). The tick mutex means no capture of this
   process is live; nothing else writes the data dir's scratch. This is the
   cleanup the free-space check must follow (the leftover of a crashed attempt
   is removed even when the snapshot is not due, so 40 GB never waits for the
   next due capture).
2. `Runtime.Start` also removes it, next to `removeStaleReplayScratch`
   (`runtime.go:1278`), so a crash's scratch is gone before boot-time disk users
   (rebase, restore staging) need the space.
3. Busy, free-space rule (2.5), acquire the exclusion (2.6), spend the attempt,
   `inFlight = true`, `Take`.
4. `capture`: hold the partitions, begin the transaction, head, page size and
   count, partition list, copy (2.2), end the transaction, call a new
   `Config.ReadReleased` hook (production: `store.CheckpointAfterLongRead`, moved
   from the `defer` on `Take`, `snapshot_producer.go:798`, to here), open the
   copy, cross-check (2.3), witness, close the copy's DB.
5. Upload from the file: read the scratch in `ChunkSize` blocks (the same cut:
   `snapshot.go:660-672`), hashing as read, seal, probe, PUT with the existing
   uploader (`:420-470`). At the end compare the hash with the copy's; mismatch
   fails the attempt before `RegisterSnapshotRoot`.
6. Delete the scratch directory immediately after the upload (not at the end of
   `Take`), so the partitions, the append and the next disk user do not wait for
   it. A `defer` also deletes it on every error path.
7. Partitions (unchanged), manifest, `RegisterSnapshotRoot`, append
   (`snapshot.go:341`, `:365`), release the exclusion.

`inFlight` stays true for the whole `Take` (`snapshot_producer.go:559-562`). A
planned restart still waits for the whole capture, not only the copy, because a
restart after the copy would discard the scratch and the witness for nothing
(`planned_restart.go:516-519` gives the same reasoning for the stream).

### 2.5 Free-space rule, every trigger kind

The old replay-only headroom (`:536-543`) is deleted; one rule applies to
first, ceiling, interval and replay, because every capture copies:

```
need = S + snapshotFreeFloor + copyHeadroom
S            = page_count x page_size read from the read pool at evaluation
copyHeadroom = copyEstimate x snapshotWALBytesPerSecond
copyEstimate = 2 x the last copy duration of this computer, when the persisted
               capture record has one; otherwise S / 50 MiB/s; at most the copy budget
```

- `snapshotWALBytesPerSecond` (2 MiB/s) is kept: it is the existing measured
  figure (`snapshot_producer.go:120-124`). The 50 MiB/s and the factor 2 are
  inferred defaults, to be replaced by the first measured copy; they are
  conservative, not measured.
- A small store: S = 1.1 GB gives about 2 GiB + 1.1 GB + 22 s x 2 MiB/s x 2,
  about 3.2 GB (the figure "about 3.1 GB" appears in the set-aside design).
- Unreadable `page_count` or free space fails closed, as today (`:527-530`).
- During the copy: `FreeSpace()` is called every 64 MiB written and fails the
  attempt under the floor (the minute-interval goroutine, `snapshot.go:492-517`,
  stays for the witness and upload). Reason: the copy writes at disk speed, so a
  one-minute interval can be 10 GB or more between readings, against a 2 GiB
  floor *(inferred from the write rate measured on the build VM above)*.
- Staged compact copy (`state.db.compact`, complete): no separate rule. Its bytes
  are already missing from the measured free space; the refusal text names it
  when it exists so the operator sees the cause. The capture does not delete it
  (compaction does that itself, `compact.go:312-316`).
- Compact copy in progress (`state.db.compact-staging`): the exclusion of 2.6.
  A leftover staging file with no compaction running is removed at the attempt
  start (2.4), which is exactly what `prepareCompactCopy` does first anyway
  (`compact.go:267-269`).
- The replay probe's scratch (`replay_scratch.go:124-142`) and rematerialize
  staging share the volume; they admit by measured free space, so a capture's
  scratch makes them refuse by name, and theirs makes the capture refuse by name.
  No mechanism is added for that *(inferred from `admitReplayScratch`)*.

### 2.6 The exclusion with compaction

Both are store-sized writes to one volume. Today nothing orders them (1.5), and
the disk check is a one-time reading before each starts. Mechanism: a single
in-process token on `store.Store`, `TryHoldStoreCopy(kind string) (release func(),
holder string, ok bool)`, taken by `prepareCompactCopy` around staging (from
before `removeSQLiteFiles(staging)` at `compact.go:269` until the staged file is
renamed or removed) and by the capture around `Take`. The second to ask is
refused with the holder's name: the capture reports "refused: a compact copy is
being written" and the next tick retries (10 minutes, `snapshotTickEvery`);
compaction returns `skipped: a snapshot capture holds its scratch copy` and runs
at its next pass (24 hours, `telemetry_retire.go:30`). Being in-process, a crash
releases it. This is the only new shared state in the lane.

### 2.7 What still pins the WAL, and for how long

- The state store's read transaction: head read plus copy. This is the pin the
  lane shortens. From today's witness plus stream to the copy time. The copy
  time is bounded below by the read rate and above by the guest disk's write
  rate (unmeasured on a guest; the build VM wrote about 560 MB/s). WAL growth is
  proportional: the set-aside design cites 2.6 to 3.7 GB over 37 minutes,
  about 1.2 to 1.7 MiB/s, so a 3 to 13 minute copy would pin about 0.2 to 1.3 GB
  *(inferred from figures taken from that design, not re-measured here)*.
- Each telemetry partition's read transaction pins that partition's own WAL for
  that partition's stream and upload, one after another, after the state
  transaction has ended and after the state upload (1.3). They are unchanged;
  the lane adds a metric for the longest of them (section 4). Their size is not
  recorded anywhere I read.
- The partition hold (`gcMu.RLock`) lasts from before the transaction to the
  end of `capture`, as today (`snapshot.go:528-531`), so partition garbage
  collection waits for the whole capture, now including the witness and upload.
  It already did (the witness and stream were inside the same function).
- After the transaction: nothing of the capture pins the state WAL. The explicit
  PASSIVE checkpoint moves to the moment of release (2.4 step 4); ordinary
  autocheckpoints continue behind it. The file's size stays at its high-water
  mark until a writer restarts the WAL and truncates to 64 MiB
  (`wal_bound.go:20-24`).
- Other long readers are unchanged: the compaction copy (1.5) and the checkpoint
  bind's witness (outside this lane).

### 2.8 Crash at every step

| Step | State left | Next attempt or boot |
| --- | --- | --- |
| before the copy | nothing | normal |
| during the copy (transaction open) | partial scratch, no WAL pin (process gone), attempt counter at +1 | `Start` removes the scratch (2.4 step 2); the first tick removes it again before the free-space check |
| after the copy, before or during the witness | full-size scratch | same; the witness is repeated from a new copy (nothing resumes: CLAUDE.md "A crash restart never resumes work") |
| during the upload | scratch, some chunks on the host | same; the host sweeps unregistered chunks after its grace (`snapshot_producer.go:111-114`) |
| after the scratch delete, before `RegisterSnapshotRoot` or the append | no scratch, chunks on the host | existing behaviour (`snapshot.go:341`, `:365`) |
| after the append | complete | `clearGuestAttempt` runs when `Take` returns; a crash between append and return leaves the attempt counted (existing) |
| a failed delete of the scratch | scratch | logged; the next attempt start retries before measuring |
| rollback to the previous binary | `snapshot-tmp/` | removed at its attempt start (`:553`); that check follows its free-space check, so a rollback on a volume too full for the previous rule to pass needs the operator to delete the directory *(inferred; a volume that full has other problems)* |

A crash costs the attempt counter: three crashed starts close the producer
(`snapshot_producer.go:134`, `:547-550`). That is existing behaviour and is not
changed; a deploy restart is not a crash because the planned restart waits for
`inFlight`.

### 2.9 What is deleted

- `LiveStoreWitnessTx` and `Config.Witness`'s transaction form
  (`restore_snapshot.go:251-262`; `snapshot.go:240-244`, `:571`); `Config.Witness`
  takes the copy's `*sql.DB`. `DigestsTx` stays (`Digests` uses it).
- The state file's use of the `sqlite_dbpage` loop in `streamFile`
  (`snapshot.go:696-760`) is replaced by a file reader for the state file; the
  loop remains once, in the copy, and for partitions. The in-transaction witness
  and its ordering comment (`:562-570`) go.
- The replay-only WAL headroom (`snapshot_producer.go:536-543`) and the single
  "the stream writes no copy" comments (`snapshot_producer.go:38-41`, `:91-92`;
  `snapshot.go:13-14`; `wal_bound.go:28-31`) are rewritten. The producer header
  failure item 4 ("the stream writes a temporary copy of the store (it writes
  nothing...)") is rewritten as "the scratch copy is on the data volume, removed
  at the start of every attempt and of every boot".
- The 90-minute budget comment (`snapshot_producer.go:70-80`) is rewritten: the
  budget no longer bounds a pin. `CHOIR_SNAPSHOT_MAX_DURATION` is not touched
  (existing environment value, `:164-178`); a new constant bounds the copy
  (section 5, question 3). No new environment variable.
- `snapshotLegacyTempDirName` is renamed `snapshotScratchDirName`, same value;
  the one `RemoveAll` at `:553` becomes the start-of-attempt cleanup. The doc
  comment at `:135-137` changes.
- Not deleted: `VerifySnapshotFile`, `Restore`, `Install`, the record, the chunk
  cut and every existing restore path. A record from the new code is the record
  of today's code.

## 3. Failure list (tests are written from this list, before the code)

Test 0 already exists: `TestStaticCopyIsAConsistentImageOfThePinnedHead` (section
2.1). It is the lane's first test. Each below is a scenario one test pins.

Consistency and the tie to the head
1. Writers commit throughout the copy; the copy opens as exactly the head read
   first (test 0).
2. A copy whose head row differs from the transaction's head (a hook rewrites it
   between copy and cross-check) is refused; nothing uploaded, no root, no event,
   scratch gone.
3. A copy that is short, long or of another page size than the transaction read
   is refused by the cross-check or by open (test 0's short copy).
4. Pages copied differ from `page_count`: the attempt fails by name.
5. The copy's SHA-256 recorded at write time differs from the one computed while
   uploading (a hook flips a byte after the witness): the attempt fails before
   `RegisterSnapshotRoot`.
6. The record's `StoreHeadSequence` is the head row of the copy; a restore of the
   base replays exactly (head, target] (`tailReceipt`, `restore_snapshot.go:331-357`).
7. Equality of witnesses: the witness computed on the read-only copy equals
   `VerifySnapshotFile` on the reassembled file, equals `LiveStoreWitness` of the
   live store at a quiet head.
8. A snapshot from the new code installs through the unchanged `Restore` with
   `VerifySnapshotFile` and its chunk digests equal those of the page sequence cut
   by `ChunkSize/pageSize` (dedupe against earlier bases is preserved).

The pin
9. In the witness callback the WAL can be truncated (`wal_checkpoint(TRUNCATE)`
   reports busy 0, `assertWALReleased`, `snapshot_pages_test.go:120-129`), i.e. no
   read transaction of the capture is open during the witness and the upload.
10. `ReadTxTime` is the head read plus the copy and not the witness; `CopyTime`,
    `WitnessTime`, `UploadTime`, `ScratchBytes` are reported.
11. `ReadReleased` runs after the transaction ends and before the witness starts,
    and the producer's checkpoint is not deferred to the end of `Take`.
12. Appends and writes proceed during the copy, the witness and the upload (the
    no-hold test, `snapshot_nohold_test.go`, extended to the scratch phases).
13. A telemetry partition retired after the head and before its read is kept by
    the hold until it is read (the existing partitions test), and the longest
    partition transaction is reported.

Space and cleanup
14. A scratch directory left by a crash is removed at the start of an attempt
    BEFORE the free-space measurement: with free space equal to
    `need - leftover`, the attempt proceeds; with the old order it would refuse
    for ever (the wedge the review found).
15. The same leftover is removed by `Runtime.Start` and by an evaluation that
    finds the snapshot not due.
16. The scratch's `-wal` and `-shm` sidecars are removed with it (test 0 observed
    them left after close).
17. The free-space rule table: for each of first, ceiling, interval, replay, free
    just under `S + floor + headroom` refuses with the amounts named, and just
    over it takes the capture; nothing is created on refusal.
18. Free space falling under the floor mid-copy (checked every 64 MiB) aborts the
    copy; the transaction is released, the scratch removed, no root, no event.
19. `ENOSPC` or `EIO` on write or sync names the error; transaction released,
    scratch removed.
20. A staged compact copy (`state.db.compact`) lowers measured free space and the
    refusal names it; the capture does not delete it.
21. A leftover `.compact-staging` file with no compaction running is removed at
    the attempt start; one with a compaction running is not touched.
22. Exclusion: a capture refuses while compaction holds the token and compaction
    skips while a capture holds it; both release on success, error, panic and
    context cancellation.
23. The scratch is deleted right after the upload and on every error path
    (witness error, upload error, `RegisterSnapshotRoot` error, append error,
    cancellation), including before the partitions are read.

Budgets, restarts, observability
24. The copy budget ends the copy with `ErrTimeBudget` naming the copy phase;
    the transaction is released and the scratch removed; the producer backs off
    (`snapshotBackoffWait`).
25. `MaxDuration` still ends a slow witness or upload with nothing registered, and
    a witness aborted by the budget uploads and probes nothing (the existing W1
    test, `snapshot_witness_first_test.go`, rewritten for the new order; the W2
    test about the witness sharing the transaction is deleted with the mechanism it
    pinned).
26. `inFlight` is true from before the copy until `Take` returns, and a planned
    restart is refused during the witness (`planned_restart.go:566-568`).
27. Context cancellation during the copy releases the transaction, closes the file
    and removes the scratch.
28. The persisted capture record carries the copy seconds, a restart reads them,
    and the headroom uses them (and uses `S / 50 MiB/s` when absent).
29. Observability: `snapshot_producer` reports `last_read_tx_seconds`,
    `last_copy_seconds`, `last_witness_seconds`, `last_upload_seconds`,
    `last_partition_pin_max_seconds` and `last_scratch_bytes` after a capture and
    zeros before the first.
30. The scratch directory name equals the previous binary's cleanup name
    (`snapshotScratchDirName == "snapshot-tmp"`), so a rollback removes it.
31. Geometry: a page size that does not divide `ChunkSize` or a zero page count is
    refused before any byte is written (`snapshot.go:669-671`).

## 4. Class, surfaces, rollback, proof

- **Mutation class: red.** The change writes the base every restore starts from
  and the record that names it on the tape (`snapshot_recorded`,
  `snapshot.go:365`), and changes disk use on every computer's data volume.
  Orange behaviour (runtime) with a red surface (tape and evidence: the
  snapshot is part of the restore chain; checkpoint and route projection are not
  touched). Two review seats before landing; the author is not one
  (`CLAUDE.md`, Review).
- **Conjecture delta.** Today: a capture's head, pages and witness are one state
  because one read transaction produced all three. New: they are one state
  because the pages were copied in the transaction that read the head, the copy's
  head row and geometry are checked equal before the witness, and a digest taken
  at write time is checked at upload. The witness on the copy is the install-time
  check.
- **Protected surfaces.** Snapshot capture and `snapshot_recorded`; the
  guest data volume and its free-space floor; the store's WAL; compaction's
  staging; telemetry partition GC (the hold). Unchanged and listed so a reviewer
  does not look for change: the record, the chunk cut and sealing, the file
  CAS, `Restore`, `Install`, `VerifySnapshotFile`, the checkpoint bind, vmctl,
  the platform's snapshot roots.
- **Heresy delta.** The header comment's "no temporary copy" is retired; the
  producer's failure item 4 is rewritten. Nothing is added as an off switch;
  the free-space and exclusion refusals are correctness refusals
  (`CLAUDE.md`, Delivery).
- **Rollback.** Revert the commit. The record is unchanged, so an older binary
  restores a base the new binary wrote (`restoreOne` reads the same fields,
  `snapshot.go:989-1031`). A scratch left by a new binary is under the old name
  and is removed by the old binary's attempt start (`snapshot_producer.go:553`).
  The persisted capture record gains a field an older reader ignores
  (`json.Unmarshal` into a struct, `:333`).
- **Expedite.** Every non-docs landing in this lane carries `Ring-Expedite:` with
  the defect it unblocks (the pinned WAL on the largest computers). The path
  `internal/agentcore/` is already an expedited deploy path
  (`CLAUDE.md`, Delivery). Stage one is docs plus one test file and ships none
  to a computer.

### Deployed proof (d@e only; never the paper)

Metric: `GET /api/runtime/observability`, `snapshot_producer`. The block today
exposes `last_capture_seconds` and the text `last_detail`, which contains
"stream ..., witness ..., read transaction ..."
(`snapshot_producer.go:608`, status struct `:665`), but no structured
read-transaction length, so this lane adds the structured fields of failure 29.
The existing WAL block is the second witness of the pin: `wal.last_checkpoint`
with `reason: "snapshot capture"` carries `wal_bytes_before`, the WAL's size when
the pin ended (`wal_bound.go:86-93`, `api.go:1167-1169`).

Procedure, after the deployed commit on d@e is verified:
1. Before the deploy, read `snapshot_producer.last_detail` and `wal` of d@e and
   record the old read transaction, witness and `wal_bytes_before` (read only).
2. After the deploy, let two captures occur naturally (d@e takes them by the tail
   rule; the producer's `due` and `due_why` say when). No computer is forced.
3. Predicted, per capture: `last_read_tx_seconds <= last_copy_seconds + 2` and
   `last_read_tx_seconds` at most half the pre-deploy read transaction;
   `last_witness_seconds` reported separately and nonzero; `wal.last_checkpoint.
   wal_bytes_before` below the pre-deploy value; head sequence advancing during
   the capture; the scratch directory absent afterwards (`data_volume` free bytes
   back to the pre-capture value within the WAL's size).
4. A restore to a checkpoint above the second capture reports the witness matched
   (the set-aside design's proof command; the CLI wording is not verified here).
5. Adversarial drill on d@e only: kill the runtime mid-copy and mid-witness; the
   next boot removes the scratch before the first tick and the next capture
   succeeds.
6. Measure on d@e the append latency during the copy against outside it; the
   floor-and-sync parameters of 2.2 are justified only if it does not worsen
   (there is no existing append-latency field I found in the observability block;
   if none exists the test records head-sequence timestamps from the event index).

Precommitted claim (to be entered in `docs/calibration/claude-ledger.md` at the
landing, not before): on d@e, the first capture after the deploy has
`last_read_tx_seconds <= last_copy_seconds + 2` and at most half of the old read
transaction. Probability 0.8. The scoring outcome is a field of the deployed
observability block, which the author cannot write. Not entered in stage one
(no code is deployed).

## 5. Open questions and the conservative default

1. **A staged compact copy can starve captures on a large computer.** The
   compact copy is staged "for the next planned restart" and kept until one
   (`compact.go:11-18`, `:249-250`). If it is of the order of the store, then
   `S + staged + floor + headroom` can exceed the volume and captures are refused
   for as long as it stays. The set-aside design's paper figures (39.6 GB store,
   85.9 GB available, 42 GB compact copy; taken from that document, not
   re-measured) put the sum at about 81.6 GB plus floor and headroom against
   85.9 GB. Default: refuse, with the amounts and the staged copy named, record
   it as a residual with the d@e proof not covering it, and do not delete the
   staged copy. The way out (using the staged copy as the static copy, the set-aside
   design's landing 5) is a separate lane.
2. **Partitions keep their own transactions.** Their pins are unchanged and
   unmeasured. Default: unchanged, with `last_partition_pin_max_seconds` added;
   copying them too would be the same mechanism applied to other files, and is
   decided after that number exists.
3. **Copy budget.** The set-aside design proposed 20 minutes (unmeasured). Default:
   30 minutes as a constant, at which the pin at 1.2 to 1.7 MiB/s of WAL growth is
   about 2.2 to 3.1 GB, no worse than today's 3.7 GB. Replace by the d@e and
   paper measurements. It is a deadline, not a switch.
4. **The write-time digest.** It re-establishes "the bytes uploaded are the bytes
   witnessed". Cost: one goroutine, about 450 MB/s on the build VM. Default:
   include. If a reviewer finds the failure it prevents (a disk fault between
   copy and upload) too remote, it is the first thing to delete.
5. **fdatasync and cache drop every 64 MiB, no throttle.** Default as in 2.2;
   if append latency worsens during the copy on d@e (proof step 6), the answer is
   a smaller sync interval, not a rate limit.
6. **Headroom constants.** 50 MiB/s assumed copy rate and the factor 2 are
   inferred. Default as in 2.5, overwritten by the first measured copy.
7. **The exclusion's shape.** An in-process token on `Store` (2.6) against two
   `stat` calls on file names. Default: the token, because a name check is wrong
   after a crash (the leftover staging file blocks for a day, `telemetry_retire.go:30`).
8. **Reading the copy read-only versus the install's read/write open.** Default:
   read-only (the experiment shows it leaves the bytes unchanged); failure 7 pins
   the equality of the two witnesses.
9. **The legacy `snapshot-tmp` cleanup becomes the scratch cleanup** and keeps its
   name. Default: yes; the rollback story depends on it.
10. **`inFlight` over the whole `Take`.** The set-aside design says a restart
    "waits only for the copy"; the code sets `inFlight` around `Take`
    (`snapshot_producer.go:559-562`). Default: keep the code's behaviour; a
    restart during the witness would discard 40 GB of scratch and the witness.
11. **Existing `CHOIR_SNAPSHOT_DISABLE` and `CHOIR_SNAPSHOT_MAX_DURATION`**
    (`snapshot_producer.go:56-58`, `:163-181`) are switches by the rule's own
    wording; not touched here (different lane); stated so a reviewer does not
    attribute them to this change.
12. **The ledger entry** of section 4 is made at the landing commit. Default: made
    then.
13. **Page cache pressure on small guests during the witness.** The witness reads
    the copy through SQLite with the live store's page cache already warm.
    Default: after the witness, drop the whole scratch file's cache
    (`dropSourceCache`, `snapshot.go:621-626`) and drop ranges while uploading; no
    cache control during the witness itself *(inferred; measured on d@e as free
    memory in the guest memory block if one exists)*.

## 6. What the set-aside design and the reviews have wrong, against the code

- "A planned restart waits only for the copy (the existing `inFlight`)" is
  wrong: `inFlight` is set around the whole `Take` (`snapshot_producer.go:559-562`).
- A review (deepseek B3) gives `snapshot_producer.go:553` as "precedent" for
  cleaning before the free-space check. It is the opposite: the legacy removal
  runs after the free-space refusals (`:527-543`) and after `spendGuestAttempt`
  (`:547`). A cleanup before the measurement is new. The other review's "boot
  deletes ... as `snapshotLegacyTempDirName` does today" is also not today's
  behaviour: it is removed at an attempt, not at boot; the boot sweep is
  `removeStaleReplayScratch` (`runtime.go:1278`), a different precedent.
- The review's "refuse while a staged or staging copy exists" and "gate on both:
  2 x 42 GB against 83.9 GB" is replaced by the measured free space (which
  already nets out a staged file) plus an in-process exclusion for the copy being
  written. The staged copy still starves large captures in practice (question 1).
- The set-aside design says the "WAL headroom rule (`:124`) is computed from the
  copy's duration". Today it applies to the replay trigger only (`:536-543`);
  first, ceiling and interval captures get the floor alone, so the rule here is
  a change for three of the four kinds.
- `wal_bound.go:28-31` and the producer and snapshot headers state the opposite
  of the new design ("not shortened", "no temporary copy"); they are in the
  delete list (2.9). The set-aside design does not list the `wal_bound.go` one.
- The set-aside design's "Record, restore and `VerifySnapshotFile` are unchanged"
  holds, but it leaves out that nothing in restore compares the file's head row
  with `StoreHeadSequence` (1.8): the head tie must be made at capture.
- The review's Claude "page-cache" point is right for the scratch but the code's
  existing drop is of the *source* (`snapshot.go:612-626`, `:733`); the
  scratch needs its own (2.2).
