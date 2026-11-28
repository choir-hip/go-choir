# Snapshot in one pass: no scratch copy, no logical witness at capture (2026-11-22)

Standing: governing | kind: decided | settled-by: owner 2026-11-22 in the authority field of definitions/choir-storage-efficiency-2026-11-22.md (stations S2 and S3 are this design); stage A1 landed as 9e3e2060f (restore verifies a version 2 record by its page digest) | note: replaces snapshot-without-scratch-copy-2026-11-20.md; the capture half (station S3, no scratch copy) is not built, docs/changes.md says no capture writes the version 2 record yet

Class of this file: green (a design). It supersedes
[snapshot-without-scratch-copy-2026-11-20.md](snapshot-without-scratch-copy-2026-11-20.md),
which kept the witness inside the read transaction; a witness of the paper
took 55 minutes at capture (a capture measurement, not a restore
measurement), so that design pins the WAL for an hour. Problem records:
[the snapshot aborts and its scratch copy blocks deploys](../problems/paper-snapshot-aborts-at-its-time-budget-and-its-scratch-copy-blocks-deploys-2026-11-22.md),
[the staged compact copy](../problems/compact-copy-staged-on-a-51-gib-store-holds-node-b-under-the-deploy-floor-2026-11-22.md).
Code read at `origin/main` 5182b4781, from the lane `snapshot-one-pass`.
Statements about code cite file and line; nothing here was run against the
paper.

## Decision

A snapshot record gains a page digest in place of the logical witness. The
capture streams the state file's pages inside the head's read transaction to
the uploader, hashing them as it reads; there is no scratch copy and no
witness. Restore checks the page digest as the pages are written and then the
restored file's head row. Layout compaction's age trigger is deleted
(section B of the lead's brief; not part of this note).

## Stage A1: the reader (this landing)

- `snapshot.Record` gets `PageDigest`. `PageDigest` is SHA-256 over a domain
  tag, the page size (4 bytes), the page count (8 bytes) and then the plaintext
  pages in page order (`snapshot.NewPageDigest`). The geometry is inside the
  digest.
- Record versions: **1** carries `WitnessRoot` and `SchemaHash` and no digest;
  **2** carries `PageDigest`, the page geometry, and no witness. A record whose
  form disagrees with its version, or that carries neither, is refused in
  `restoreOne` (`checkRecordForm`), not in `Verify`, which an install may omit.
  Any other version is refused as before.
- `assemble` hashes each chunk as it is written, before anything opens the
  file. A mismatch refuses the candidate; the install falls back to the next
  older snapshot and leaves no file.
- `VerifySnapshotFile` branches on the version. Version 1: the logical witness
  is recomputed, as before. Version 2: the restored file's projection head row
  (`store.ProjectionHeadOf`) must equal the record's `StoreHeadSequence` and
  `StoreHead` (a file with no head row matches only a record of sequence 0 and
  an empty head). No logical witness is computed.
- `Install` refuses a version 2 record when `RestoreConfig.Verify` is nil: the
  head tie lives in `Verify`. Version 1 with no `Verify` installs as before.
- The telemetry partitions are not covered by the page digest: they are
  checked by their chunk seal only, as in version 1.
- No capture writes version 2 yet.

## Stage A2: the capture (second commit, after A1 is reviewed)

- The state file goes through `streamFile` (today used for the telemetry
  partitions) inside the head's read transaction, with the page digest taken
  inline before the page is copied into the chunk buffer. The record is
  version 2.
- Delete `copyPages`, `crossCheckCopy`, `streamScratch`, `ErrCopyDigest`, the
  scratch directory and the capture's store-copy hold; keep the boot sweep of an
  old scratch directory.
- The free-space check becomes the floor plus WAL headroom, not store size plus
  floor.
- A pin deadline (about 30 minutes) ends the read transaction, runs the WAL
  checkpoint, keeps the uploaded chunks, and a retry skips them. Backpressure is
  the uploader's: 4 jobs queued and 9 buffers of 4 MiB, so the reader is never
  more than about 36 MiB ahead of the uploads, and a stalled upload pins the
  transaction until `runCtx` ends (today 90 minutes) unless the deadline does.
- `snapshotDue`, the capture floor, the backoff and the 540,000-event ceiling
  are not touched (three review rounds on 2026-11-15 failed there).
- Tests that assert capture witness equals restore witness are rewritten in A2.

## Order and rollback (read before A2)

- **A1 must be deployed on every computer before A2 lands.** A binary older
  than A1 refuses a version 2 record (`record is version 2 of computer ...`,
  `restoreOne`).
- **Rollback consequence, by path.** Boot reads the tape with `snapshot.Scan`,
  which keeps only the newest four snapshot events (`maxCandidates`,
  `snapshot.go` `scan`, `projection_base.go:165`). After four version 2
  captures an older binary's boot has no version 1 candidate at all, whatever
  roots the host still holds. `Install` then fails every candidate with
  `record is version 2` and returns `snapshot: no usable snapshot: ...`, which
  boot raises as `BootRefusalKindBaseUnavailable` (`projection_base.go:206`):
  that computer does not boot from a lost disk. The staged (rematerialize)
  path uses `ScanAll` and `PlanCandidates`, so it can reach older version 1
  roots within `recoveryplan.MaxRecoveryTailEvents` (a base too old is refused,
  not replayed past the bound), but only while the host still holds them.
- **So a rollback of A2 is a revert of the capture only: the reader stays.**
  Reverting the capture stops new version 2 records; the version 2 records
  already on the tape keep restoring through the A1 reader. Rolling the binary
  back past A1 after A2 has written four version 2 records is not a rollback.
- A1 alone is safe to roll back: nothing writes version 2.
- Fallback to an older snapshot after a failed candidate is best effort in the
  staged path: `openStagedStore` sets `CandidateBudget` 2 minutes
  (`restore_snapshot.go:93,165`) and a digest failure arrives only after the
  whole file is downloaded and hashed, so on a large store the budget is spent
  and the older candidate is not started (`Install`, `snapshot.go:1316`). Boot
  sets no budget. Recorded as a residual; the budget is not changed here.

## What was checked against the investigation (corrections)

- The witness has exactly one reader, `VerifySnapshotFile`
  (`restore_snapshot.go:213`), reached from `openStagedStore` (rematerialize and
  the replay probe) and `autoputer/projection_base.go:198` (boot). The host and
  `internal/bundle` never read the record; the host sees only
  `RegisterSnapshotRoot(root, headSeq, chunks)`.
- `restoreRequestBudget` (`snapshot_producer.go:105`) bounds nothing; only a test
  uses it. The real bound on a restore is the caller's context: the proxy's
  ten-minute request for rematerialize, and `bootstrapCtx` of 30 minutes at
  `autoputer/run.go:339` for boot (that it reaches `Install` is inferred).
  `CandidateBudget` only stops starting another candidate after a failure; a
  running candidate is never cut off by it.
- `RematerializeFromTape` also computes a second whole-store logical digest after
  replay (`reconstructedWitness` against the checkpoint's witness). A1 and A2 do
  not change it. Out of scope; recorded as a residual in the problem record.
- Dropping the capture witness loses the only read of every table at capture:
  a corruption canary on the source. The page digest proves the bytes uploaded
  are the bytes read, which the sealed record and the manifest already imply;
  what it adds is detection of a capture-side sealing or buffer-reuse bug. A
  quick_check at restore or capture is unmeasured (`compact.go:64` records
  integrity_check at 58 minutes on a 20 GB copy) and is not added.
- The restored head is compared by sequence and canonical head only: the record
  holds those two fields, not the whole head row.
- `VerifySnapshotFile` opens the file read-write with `journal_mode(WAL)`, so the
  digest is taken before anything opens it.
