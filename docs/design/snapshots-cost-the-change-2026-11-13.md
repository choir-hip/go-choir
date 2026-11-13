# Snapshots that cost the change (2026-11-13)

Status: revised after review round 1 (both seats REPAIR). The round-1
dispositions are in section 10.

Class: green (design record; nothing changed by this file). Station S4 of
[choir-storage-solutions-2](../definitions/choir-storage-solutions-2-2026-11-12.md)
("snapshots cost the change, keep two"). Design record first, and two seats
for every red stage below. Inputs: `internal/snapshot/snapshot.go`,
`internal/filecas/filecas.go`, `internal/agentcore/snapshot_producer.go`,
`internal/agentcore/restore_snapshot.go`, `internal/recoveryplan`,
`internal/platform/snapshot_roots.go`, `internal/platform/file_cas.go`,
`internal/platform/artifact_gc.go`, `internal/bundle/store.go`,
`internal/store/compact.go`, `internal/store/telemetry_retire.go`;
[chunk drain trace](../evidence/paper-chunk-drain-trace-2026-11-12.md),
[storage frame](storage-pressure-frame-2026-11-12.md),
[append-clustered store](append-clustered-store-2026-10-31.md) (section 6
of that note holds the only page-level churn measurements we have),
[root check problem](../problems/snapshot-root-recorded-without-chunk-check-2026-11-12.md),
[loop problem](../problems/snapshot-loop-blocks-its-own-fix-2026-11-12.md),
and the storage mission ledger.

Tags: **[C]** read in code at the cited line on origin/main b21cf284d;
**[M]** measured, with its source; **[E]** estimate, with the arithmetic
shown; **[I]** inferred; **[?]** not known. Nothing tagged [E], [I] or [?] is
a fact.

## 0. The answer in brief

1. **The brief's figure of 25 GB a snapshot is stale.** It is the size of
   the paper's first snapshot on 2026-10-30, sealed before chunks were
   compressed (f1ebf0e4c and 8492760aa turned compression on, 2026-10-31).
   The paper's latest snapshot, at 2026-11-12 23:28Z, uploaded 5,858 of
   10,524 chunks: 4.04 GB sealed out of 44.1 GB of pages. The witness took
   22 min 37 s and the stream 3 min 02 s [M, storage mission ledger].
   **A snapshot costs about 4 GB a day** (one every 22 hours). The host has
   135 GiB free. The comments that still say 25 GB are stale
   (`snapshot_producer.go:62-69`, `snapshot_roots.go:37-41`).
2. **Sealing is not why chunks fail to dedupe.** The nonce is synthetic:
   `filecas.SealChunk` (`filecas.go:184-215`, `chunkNonce` at 234-248)
   derives it from the plaintext. Two roots taken at one frozen head shared
   10,510 of 10,510 chunks [M].
3. **The cause is a cut of 1,024 pages laid over writes that land all over
   the file.** `streamFile` sets `fs.pagesPerChunk = ChunkSize / pageSize`
   (`snapshot.go:671`, with `ChunkSize` at line 66). A chunk changes if any
   one of its 1,024 pages changes. The writes are scattered for four
   reasons:
   - random index leaves (the canonical id key);
   - the pointer map of `auto_vacuum=INCREMENTAL`, about one page in every
     4 MiB range;
   - the daily `incremental_vacuum`, which moves up to 250,000 pages
     (`compact.go:105`, `telemetry_retire.go:315`);
   - compaction swaps.

   Section 3(a) has the detail.
4. **Stage 0 is a present-day recovery defect I confirmed while answering
   review: the newest snapshot on the tape can lose its protection.**
   `TakeSnapshot` registers the root before it appends `snapshot_recorded`.
   Registering prunes the table at once, keeping the newest roots by head
   and nothing else. Two failed appends in a row therefore evict the newest
   root that is on the tape. Its chunks become collectable, and
   `RecoveryBase` advertises a root that the tape never names. The
   conditions occurred on the paper on 2026-11-12. Section 5, Stage 0, has
   the code path and the fix.
5. **Weighed against about 4 GB a day, most of the original station is not
   worth building.** The full cost is in section 6. What I recommend:
   - Stage 0, now;
   - S4.1, a cheap meter (yellow and orange) that measures what a snapshot
     could cost;
   - S4.2, smaller page groups, only if the meter shows a saving of at
     least 3 GB a snapshot.

   **Dropped:**
   - S4.3 (page root in place of the witness): the witness stays. Section
     7 says why.
   - The 3-hour unrooted window.
   - Keep two roots for every computer.
   - The replay-rate guard.

## 1. What a snapshot is for

A snapshot adds no new information. The tape holds every event, and the
store is a projection of the tape (`docs/computer-ontology.md`, "Guest
store"). A snapshot is a **cache of replay**: it bounds how far a rebuild
has to replay. The bound is `recoveryplan.MaxRecoveryTailEvents = 720000`
(`recovery_plan.go:22`). The producer fires at 540,000 events
(`snapshot_producer.go:70`), which is about 22 hours on the paper.

- Its floor is the bytes that state the store at head N+T, given the
  store at head N: the pages that changed (section 4).
- If T is the interval and D(T) the bytes of the pages that change in it,
  the daily cost is (24 h / T) x D(T). D is concave, so **space snapshots
  as far apart as the recovery bound allows**. A trigger that fires earlier
  costs bytes.
- A snapshot must be checkable before it is installed
  (`VerifySnapshotFile`, `restore_snapshot.go:173-186`). It must also be
  named on the tape: `snapshot.Scan` finds candidates only through
  `snapshot_recorded` events (`snapshot.go:904-932`).

## 2. Today's snapshot, measured

| Quantity | Value | Source |
|---|---|---|
| state.db | 22.38 GB, 4 KiB pages, about 5,337 chunks of 4 MiB | [M] drain trace section 1 |
| telemetry partitions | about 21.7 GB, about 5,187 chunks; a closed partition's bytes do not change | [E] 44.1 GB of pages less 22.4 GB |
| chunks in the latest root | 10,524 | [M] ledger 2026-11-12T23:30Z |
| uploaded at the latest snapshot | 5,858 chunks, 4.04 GB sealed (0.69 MB a chunk, about 6x compression) | [M] same |
| already held | 4,666 | [M] same; the log does not split state chunks from partition chunks |
| witness | 22 min 37 s (22 to 35 min seen) | [M]; `snapshot.go:563-573` |
| stream | 3 min 02 s | [M] |
| read transaction (holds the WAL) | 25 min 51 s; the WAL reached 3.7 GB in 30 min under load | [M] |
| roots kept | 2 when a root names more than 2,048 chunks, else 4 | [C] `snapshot_roots.go:34-43`, prune at 98 |
| window for unrooted chunks | 24 h from the last write or claim | [C] `artifact_gc.go:180`, 677 |
| collector | hourly; at most 50,000 files and 32 GiB a run | [C] `nix/node-b.nix:1129-1134` |

The previous root (19:18Z, head 10,914,341) was 64,433 events earlier, with
a crash boot onto a new binary in between. So this reading does not
measure churn cleanly [?].

**Superseded roots are collected within about an hour.** This holds for
large roots (retention 2). A dropped root's own chunks were last claimed
about two intervals before it is dropped, so they are already past the
24-hour window. A small computer at retention 4 waits four intervals.
Failed attempts are the case where the window does cost bytes. Since
0d2f0bd46 a budget abort in the witness uploads nothing. A failed append
leaves a *registered* root, so its chunks are rooted, not held by the
window (Stage 0).

## 3. The options

Costs are for the paper's state file. Partitions stay at 1,024 pages a chunk
because a closed partition never changes. p is the share of pages that
changed between two snapshots. p_g is the share of g-page groups that
changed. c_g is zstd's ratio at group size g.

### (a) Today: fixed 1,024-page ranges, deterministic sealing

- Bytes: p_1024 x 22.4 GB / 6 ≈ 3.7 to 4.0 GB [M].
- Guest time: about 26 min (witness 87%). The WAL is held for the same
  time. Restore: about 10.5k sequential GETs, the witness again, then
  replay.
- Why dedupe is poor:
  - A range survives with probability (1 - p)^1024 if changes are spread
    out. Then 88% of ranges changing needs only p ≈ 0.2%, about 11,000
    pages [E].
  - Index inserts land on random leaves: the 136-byte canonical id is the
    last column of 13 of the 16 `og_objects` indexes [M, append-clustered
    section 0].
  - The pointer map has one page per about 820 pages, rewritten whenever a
    page in its range is allocated, freed or re-parented. In the synthetic
    store, 1,287 pointer-map pages in 1,211 ranges changed per 1,000
    inserts [M, append-clustered 6.1].
  - The daily `incremental_vacuum` moves up to 250,000 pages.
  - A compaction swap renames a new file into place (`compact.go:373`) and
    renumbers every page.
  - Hot rows are updated in place.
- Ruled out:
  - Sealing (above).
  - Page 1's header: it is in chunk 0 only.
  - Page order: the stream *enforces* page order and fails loudly
    otherwise. The query has no ORDER BY (`snapshot.go:695`, check at
    722-725).
- A latent cause: the digest is SHA-256 of the *sealed* bytes, zstd output
  included (`filecas.go:261`). A zstd library upgrade that changes the
  encoder's output re-keys every chunk once [?].

### (b) Smaller fixed page groups, keyed by content (g < 1,024 pages a chunk)

Same format, sealing, CAS, root check and collector. Only `pagesPerChunk`
changes, and only for the state file.

- **Today's restore already reads this.** `restoreOne` accepts any
  `PagesPerChunk` with `PageSize x PagesPerChunk <= 2 x ChunkSize` and
  the matching chunk count (`snapshot.go:963-976`). `assemble` checks each
  chunk against that size (`snapshot.go:1059-1065`). A rollback to the old
  binary therefore still restores the new snapshots.
- Bytes: p_g x 22.4 GB / c_g. If changes are spread out,
  p_g ≈ 1 - (1 - p)^g. At p = 0.2%: p_128 ≈ 23%, about 0.86 GB at c = 6.
  At p = 16%: about 100%, no gain [E].
- Chunks per root: 5.46 M / g, plus 5.2k for partitions.
  - g = 128: about 48k chunks. That fits `maxSnapshotRootChunks` = 65,536
    and `maxSnapshotRootBody` = 8 MiB (about 3.2 MB at 67 bytes a digest).
  - g = 64: about 90k chunks and 6.0 MB. That needs only
    `maxSnapshotRootChunks` raised.
  - g = 32: about 176k chunks and 11.8 MB. That needs both limits raised.
    It also needs a benchmark of the root stat pass, the Postgres write
    under the 30 s deadline, the `LIKE` live check and a sequential
    restore of 176k GETs. (Seat 1 found these.)
- Tape: the sealed record grows to about 3 MB at g = 128. The caps are
  64 MiB: `MaxPrivatePlaintextBytes` (`privacy.go:51`),
  `EventPayloadMaxResponseBytes` (`event.go:30`), `tapestore.MaxRecordBytes`
  (`format.go:103`).
- Restore at g = 128 is about 48k sequential GETs. It must be measured on
  a paper-sized store (`snapshot_large_test.go` builds one), not on d@e.
  `assemble` gets a prefetch if the time is long.
- One full re-upload when the cut changes, and another on rollback.
- The host learns which 512 KiB regions changed instead of which 4 MiB
  regions. Equality leakage is already accepted (`filecas.go:196-198`).
- Code risk: low.

### (c) Content-defined chunking

A SQLite file does not shift: pages are rewritten in place. CDC is (b) with
boundaries that do not fall on pages, and with a rolling hash to pay for.
After a compaction the bytes themselves change, because each page holds
its children's page numbers. **Rejected.**

### (d) Page diff against the previous root

The guest sends only the changed pages, packed, with a page map. It costs
the bound in bytes. The cost is in collection: a pack stays live while any
live root names a page in it, so the host needs repacking or periodic full
bases. That means a new restore and a log-structured store on the host.
High risk. **Deferred:** worth it only if (b) cannot get under the S4.2
threshold *and* the bytes matter, which section 6 argues they do not.

### (e) Not snapshotting the store

- Replay from genesis: more than 10.9 M events, which breaks the bound.
- A host reflink of `data.img`: it sits on the same disk it protects,
  nothing on the tape names it, it contradicts the ontology ("the host
  keeps no projection bases"), and the copy is plaintext at rest.
- Faster replay to allow a larger bound: the bound belongs to the owner.
  **All rejected.**
- Change the store's write locality instead: the append-clustered layout
  plus a compaction cadence. Synthetic store: 83% to 88% of the 4 MiB
  ranges of a compacted file survive 24 h, against 0% for a grown file
  [M, append-clustered 6.1]. This is the strongest lever on the bound. It
  belongs to the store's own work, not to S4.

### (f) Superseded roots and the unrooted window

- Superseded large roots are already collected within about an hour
  (section 2). Collecting them "immediately" saves less than an hour of
  bytes. **Rejected.**
- A 3-hour unrooted window: since afe50a926 it is *safe*, because a
  capture that outlives the window fails loudly with a 409. It is not
  *worth* it:
  - Its only beneficiary is an attempt that fails after upload and before
    registration. That is rare now that the witness runs first.
  - `snapshotBackoffCeiling` is 3 h (`snapshot_producer.go:89`). After the
    fourth failure, each retry would find the previous attempt's chunks
    collected and upload them again: more bytes sent, to save held bytes.
    (Both seats found this.)

  **Dropped.** If the hourly status ever shows a large unrooted-in-window
  total, revisit with the window set above the backoff ceiling.
- Keep two for every computer: this breaks
  `TestSnapshotRootRetentionCoversEveryRestoreCandidate`
  (`snapshot_roots_test.go:115-124`), whose contract is retention >=
  `snapshot.MaxCandidates` for small roots. It saves nothing for a store of
  200 chunks. **Dropped.**

### Summary

| Option | New bytes a snapshot (paper) | Guest time / WAL hold | Restore | Code risk | Verdict |
|---|---|---|---|---|---|
| (a) today | 3.7 to 4.0 GB [M] | 26 min | 10.5k GETs + witness | none | keep |
| (b) g = 128 | 0.86 to 3.7 GB [E] | about 27 min | 48k GETs + witness | low | build only if the meter shows a saving of at least 3 GB |
| (c) CDC | as (b), boundaries off the pages | + rolling hash | new checks | medium | rejected |
| (d) page diff | the bound | about 26 min | chain or map | high | deferred |
| (e) no snapshot | 0 | 0 | breaks the bound | n/a | rejected |
| (f) 3 h window, keep two everywhere | small | 0 | 0 | low | dropped |

## 4. The physical bound

    B(T) = sum over the distinct pages changed in (N, N+T] of |zstd(page)|
         ≈ p(T) x 22.4 GB / c_1

T is about 22 h (540k events). c_1 is the ratio for changed pages packed
together; it is about 6 at 4 MiB [M] and not known for packs [?].

- p(22 h) is at least about 0.2%: the share that alone makes about 88% of
  the 4 MiB ranges change in 30 minutes [E].
- p(22 h) is at most 1.
- One clone reading from 2026-10-31, at half today's event rate, saw 16%
  of blocks rewritten in 50 minutes. It is not known whether that counted
  repeated checkpoint writes [?].

So **B lies between about 7.5 MB (p = 0.2%) and 3.7 GB (p = 1) per
snapshot.** Today's 3.7 to 4.0 GB is between about 1x and about 500x the
bound. That spread is the whole reason for S4.1's meter.

The distance of the recommendation from the bound is
(p_128 / p) x (c_1 / c_128). With changes spread out at small p, p_128 / p
is near 128. It falls toward 1 as changes cluster. That ratio no longer
gates anything: S4.2 is gated on absolute bytes (section 5), because the
bytes are what the disk pays.

## 5. Recommended stages

Every stage ships on, with no setting. Acts on the paper are not allowed.
The paper's proof is reading its log and the host's tables after it boots a
release in the normal way.

### Stage 0: an unappended root never evicts a root on the tape (red: snapshot root custody, the recovery base advertised to vmctl)

**Confirmed from the code.**

- `TakeSnapshot` calls `RegisterSnapshotRoot` (`snapshot.go:335-343`)
  before `AppendNewPrivatePayload` (`snapshot.go:364`). The event's
  idempotency key is `"snapshot:" + manifest.Root` (`snapshot.go:358`).
- `recordSnapshotRoot` inserts the root and, in the same transaction,
  deletes every row of the computer except the newest `retention` by
  `head_sequence` (`snapshot_roots.go:90-101`). Retention is 2 for any
  root of more than 2,048 chunks (`snapshot_roots.go:41-43`), the paper's
  case.
- `RecoveryBase` returns the newest row by `head_sequence`
  (`file_cas.go:80-92`). It is served as the watermark
  (`file_cas_http.go:192`), and vmctl's empty-store admission reads it
  (`vmctl/recovery_admission.go:668-689`).
- Boot restore takes candidates only from the tape (`snapshot.Scan` in
  `autoputer/projection_base.go:165`) and installs one through
  `snapshot.Install` (`projection_base.go:187-198`).
- A failed append keeps its root. Nothing withdraws it.

So two failed appends in a row leave the two retained rows both off the
tape. The newest on-tape snapshot is pruned, and after the window the
collector takes the chunks it does not share. A lost disk would then fail
twice: vmctl would admit an empty-store boot against a base the tape does
not have, and `Install` would find no restorable candidate. With head
10.9 M, far past the 720k bound, the boot would refuse.

**It had its preconditions on the paper on 2026-11-12.**
- At 16:11:59Z an append failed after the root was registered [M, drain
  trace section 1, item 5].
- Roots registered at 18:55:55Z and 19:18:21Z at head 10,914,341 were not
  on the tape, because the store was wedged [M, same].
- From then until the 23:28Z snapshot, both retained rows were probably
  off the tape [I].
- Whether the previous on-tape root's chunks were collected in that gap is
  not established [?]. The 11-11 snapshot's chunks would have been past
  the 24-hour window.

Check, read only, on Node B:

    SELECT r.computer_id, r.root, r.head_sequence,
      EXISTS (SELECT 1 FROM computer_event_append_receipts a
              WHERE a.computer_id = r.computer_id
                AND a.idempotency_key = 'snapshot:' || r.root
                AND a.event_kind = 'snapshot_recorded'
                AND a.sequence > r.head_sequence) AS on_tape
    FROM computer_snapshot_roots r
    ORDER BY r.computer_id, r.head_sequence DESC;

**The smallest correct fix is host only. It reuses a predicate that already
exists.** `internal/bundle/store.go:86-104` (`readNamedSnapshotRoot`)
already defines "a root the tape names": a `snapshot_recorded` receipt
whose `idempotency_key` is `'snapshot:' || root` and whose sequence is
above the root's head. Receipts and roots share the platform store's
database. The idempotency key is the receipts table's primary key
(`computer_events.go:40-54`), so the lookup is indexed.

1. **Prune** (`recordSnapshotRoot`): keep a row if it is one of the newest
   `retention` *named* roots. Also keep the newest *unnamed* root whose
   head is above the newest named root's head; this is always the row just
   inserted, whose append is about to run. Delete everything else,
   including older unnamed roots: their attempts are over, because the
   producer runs one capture at a time (`snapshot_producer.go:213`).
   Retention counts are unchanged (4 and 2), so the contract test stands.
2. **`RecoveryBase`**: return the newest *named* root. If there is none,
   return `ErrNoWatermark` (404). vmctl handles a missing
   watermark (`recovery_admission.go:673-674`) by planning, and the planner
   refuses an empty-store start with no base as `base_missing` (corrected
   2026-11-13, see the problem record); vmctl therefore admits a chain the
   guest can replay from genesis (at most 720,000 events) and refuses a longer
   one.
3. **`LiveSnapshotRoots`** (`snapshot_roots.go:338`) and the collector's
   live check (`snapshot_roots.go:313-314`): drop `LIMIT
   snapshotRootRetention` and read every row of the computer. The prune
   already bounds the table at retention + 1. Without this change, a small
   computer's oldest named root (the fifth row) would lose its protection.
   Reading more rows is the safe direction for rows that `bundle.Import`
   adds.

No guest change. No schema change.

Failure list (written before code):
1. Two or more failed appends evict a named root (the defect; a test with
   real receipts).
2. The root just inserted is pruned (a test).
3. `RecoveryBase` advertises an unnamed root (a test).
4. The stored receipt key is not `"snapshot:"+root` (a test through the
   real append handler, not a hand-written row).
5. A computer with no named root yet: its first attempt's append failed,
   so `RecoveryBase` returns 404. vmctl and the boot must behave as they do
   for a computer with no snapshot (a test in vmctl's admission suite).
6. The collector misses a kept row because of the old LIMIT (a test with
   retention 4 and one unnamed row).
7. Roots that `bundle.Import` writes without matching receipts
   (`bundle/import.go:460`) look unnamed and are pruned at the next
   registration. Check that the import carries receipts
   (`bundle/import.go:153` counts them) [?].
8. Receipt compaction (`PLATFORM_RECEIPT_COMPACT_COMPUTERS`) removes rows
   rather than bodies [?, read the compaction before landing].
9. The prune query is slow: a few rows per computer, each lookup by
   primary key.

Proof:
- Unit and integration tests in `internal/platform` against Postgres, as
  the root-check lane ran.
- Deployed: after the deploy, the read-only query above shows that every
  computer's `RecoveryBase` root is named, and that each computer keeps at
  least `min(retention, named snapshots taken)` named roots.
- A d@e drill that makes an append fail after registration, only if a
  fault seam exists on staging. I did not find one [?]. Otherwise the
  proof is the tests plus the audit, and the report says so.

Rollback: revert. Rows keep their form, and the old prune resumes.
Conjecture delta: retention counts only snapshots a restore can find.
Evidence class: unchanged. Heresy delta: none.

### S4.1: per-file counts, then a dirty-page meter (yellow, then orange; restore path untouched)

1. **Yellow:** the "snapshot taken" line gets per-file counts. `addStats`
   already accumulates per file (`snapshot.go:774-785`, `803-813`); only
   the log line changes. It answers whether the state file or an open
   partition makes up the 4 GB.
2. **Orange, observation only, in `streamFile`:**
   - The reader keeps the first 8 bytes of SHA-256 of each state-file page
     in a guest-local sidecar, `snapshot-pagehash` beside the store
     (5.46 M pages x 8 B = 44 MB).
   - The sidecar header holds the state file's inode and device, its page
     size and its page count. A compaction swap renames a new file into
     place (`compact.go:373`), so a different inode marks the reading
     "after a compaction" and it is excluded.
   - The stream compares against the previous sidecar in step with the
     pages, reading `min(old, new)` pages. Pages beyond the old count count
     as changed.
   - It logs p, p_g for g in {32, 64, 128, 256, 1024}, and c_1 and c_g
     from a fixed sample of changed pages.
   - The sidecar is written atomically, and only after
     `AppendNewPrivatePayload` succeeds. It is never read for anything but
     the log line.
   - `Record` and the restore code are not touched.

Failure list:
1. The sidecar is used for anything but the log line.
2. It is written for a failed attempt.
3. A change in page count throws away the reading.
4. An interval that spans a compaction is reported as churn.
5. Hashing slows the stream: SHA-256 on one core is about 30 to 90 s over
   22 GB [E]. The stream's time is logged.
6. The 44 MB sidecar lands on a volume that is nearly full. It falls under
   `snapshotFreeFloor`.

Proof:
- d@e takes two snapshots and logs a meter line with plausible values.
- On the paper (read only), its "snapshot taken" line after a normal
  deploy refresh is the first measurement of p(22 h).

Rollback: revert and leave the sidecar (harmless).

### S4.2: page groups of g >= 128 for the state file (red: snapshot format), conditional

**Gate: absolute bytes.** Build only if S4.1's reading on the paper,
outside a compaction interval, predicts p_128 x 22.4 GB / c_128 <= 1.0 GB
sealed per snapshot. That saves at least about 3 GB a snapshot: about
3 GB a day of host writes and backup volume, and about 6 GB held across
the two kept roots. Use the largest g >= 128 that meets the threshold.

- Below 128 the host limits and the restore must change; that is out of
  scope.
- If no g >= 128 meets the threshold, S4 stops here, and the byte lever
  moves to the store's layout (option e).

The gate is a build decision made from a measurement, not a switch.

What changes: `streamFile` takes pages per chunk as an argument; the state
file uses g; partitions stay at 1,024. No host change and no reader change.

Failure list:
1. The producer and an older restore disagree. A test restores a g = 128
   record with the restore code at b21cf284d.
2. The paper's real page count at g pushes the root past 65,536 chunks or
   8 MiB. Check it before landing.
3. The root stat under the reuse lock at about 48k chunks takes too long.
   Measure it on the build VM, as the root-check lane did for 10.5k.
4. The first g-cut snapshot uploads the whole file once (expected; it is
   reported).
5. A superseded root's deletions span two collector runs (fine).
6. Restore time on a paper-sized store (`snapshot_large_test.go`) grows
   past the boot deadline. If it does, `assemble` gets a prefetch.

Proof:
- d@e: two snapshots, and a rematerialize of d@e (the owner act at
  `cmd/choir/commands.go:443`) verifies.
- The paper, read only: two consecutive snapshots, with uploaded bytes
  within 1.5x of the meter's prediction.

Rollback: revert. The old binary restores g-cut records (section 3(b)).
One re-upload follows. Protected surfaces: snapshot chunks and records.

### Dropped stages

- **S4.3, the page root in place of the witness.** The witness stays;
  section 7 says why. With S4.3 gone, the page root has no job: the AEAD
  and the manifest root already prove the restored bytes are the sealed
  bytes. It is dropped from S4.1 as well.
- **S4.4.** The 3-hour window and keep-two-everywhere are covered in 3(f).
  The replay-rate guard is dropped because its premise is false. Normal
  boot replay runs `appender.Reconstruct` with no observer
  (`autoputer/run.go:849-851`). `restoreReplayObserver` is attached only in
  `RematerializeFromTape` (`rematerialize.go:164-167`) and in the probe
  (`replay_completeness.go:360`). Nothing persists a rate. A real guard
  would need a durable rate that the producer reads; it fires only
  earlier, which costs bytes; and nothing shows the paper's replay is
  slower than the bound assumes. Recorded as a residual (section 9).

## 6. Diminishing returns

A completed snapshot costs about 4 GB a day of host writes and backup
volume. It holds about 11 GB on the host: two roots of about 3.7 GB of
state each, plus shared partitions [E]. Node B has 135 GiB free. Stage by
stage:

- **Stage 0: yes, first.** It is a recovery-correctness defect, not a
  saving.
- **S4.1: yes.** It is small, it does not touch the restore path, and it
  ends a 500x uncertainty about the bound. It also tells the store's own
  work how much write locality is worth.
- **S4.2: only behind its gate.** The most it can save is about 3 GB a
  day and about 6 GB held. Each red stage costs review rounds (three for
  afe50a926, five for the parked restart lane). Below the gate it is not
  worth that.
- **Not worth building:** S4.3, the 3-hour window, keep two everywhere,
  and the replay guard. Option (d) only if S4.2 fails its gate and the
  owner still wants the bytes.

The real lever on the snapshot's floor is the store's write locality
(option e), and the meter is what puts a number on it.

## 7. Why the witness stays

The witness is witness v3, computed by SQL inside the capture's read
transaction (`snapshot.go:563-573`). It is recomputed over the restored
file in `VerifySnapshotFile` (`restore_snapshot.go:173-186`). Its only
consumer is that comparison (`restore_snapshot.go:183`).

**What it proves that nothing cheaper does.** The page stream from
`sqlite_dbpage` is the same logical state that the SQL reads saw at
`HeadTx`, for every table that is not derived, non-indexed columns
included. It is the only check that crosses from the page path to the row
path.

| Candidate | Catches | Misses | Cost |
|---|---|---|---|
| page root over the streamed bytes | transport and assembly faults (which the AEAD and manifest root already catch) | a stream at a different or mixed state: it hashes whatever `sqlite_dbpage` returned | about 1 min |
| head row read from the restored file, compared with `StoreHeadSequence` / `StoreHead` | a whole snapshot at another head | a mix of pages from two states; wrong row content at the right head | seconds |
| `PRAGMA quick_check` | broken B-tree structure | index-versus-table mismatches (`compact.go:63-66`); wrong values in a valid structure | not measured on 22 GB [?] |
| `PRAGMA integrity_check` | quick_check plus index-versus-table | wrong values in non-indexed columns | **58 min on a 20.66 GB copy** (`compact.go:63-66`), more than the witness |
| witness v3 | all of the above for state tables | derived tables (excluded on purpose, `restore_snapshot.go:189-200`) | 22 to 35 min |

Nothing cheaper covers a stream that mixes pages from two states, or wrong
content at the right head. Those are exactly the faults that would be
silent: a restore would install a state that replay then builds on. The
witness stays. Its costs are:

- about 26 min of WAL hold (about 3 GB of WAL on the guest's volume);
- a longer window in which a planned restart is refused (the parked
  restart lane addresses that);
- the same time again at restore.

The owner may overrule. A witness-less format would then need the
forward-compatible reader one release ahead, because two witness-less
roots leave an old binary nothing it can restore (`restore_snapshot.go:183`
compares unconditionally). Both seats found this.

## 8. What must be preserved

- **Restore correctness.** Witness v3 stays. S4.2 changes only the cut,
  which today's readers already verify.
- **The root-time chunk check (afe50a926).** Unchanged. Stage 0 changes
  only which rows are kept and advertised.
- **Encryption at rest and key custody.** `SealChunk` under the computer's
  DEK, unchanged. The meter's sidecar holds truncated page hashes and stays
  on the guest.
- **The 720,000-event recovery bound.** Unchanged. The trigger stays at
  540k.
- **Old roots restorable until superseded.** Stage 0 strengthens this:
  superseding now means by a root the tape names.
- **No switches.** None added. S4.2's gate is a build decision.

## 9. Owner questions, each with its conservative default

1. **Stage 0's rule: keep the newest unnamed root above the newest named
   root, besides the named ones?** Default: yes. This is the attempt
   whose append is about to run. Every older unnamed root is dropped.
2. **S4.2's threshold of 1.0 GB sealed per snapshot.** Default: as stated.
   Below it, S4 stops and the byte lever goes to the store's layout.
3. **Finer change locality visible to the host under S4.2** (512 KiB
   regions instead of 4 MiB). Default: accept, the same trade as
   `filecas.go:196-198`.
4. **The witness stays (section 7).** Default: stays. Overruling it means
   a forward-compatible reader first.
5. **The 24-hour unrooted window stays.** Default: stays. Revisit only
   with the window above the backoff ceiling.
6. **The paper's replay rate is unmeasured, so the 720k bound rests on
   d@e's rate.** Default: a residual. Measuring it needs a durable rate
   from a boot replay, which no code records today.

## 10. Review round 1: dispositions

Seat 1 was Gemini 4 Hydrogen; seat 2 was DeepSeek v4.1 flash. I verified
each finding against the code. A finding marked moot was confirmed but no
longer applies, because the stage it was about has been dropped.

| Finding | Disposition |
|---|---|
| S4.2's gate refuses its own win case (seat 2 B1) | Confirmed by the record's own arithmetic. The gate is now absolute bytes (section 5). |
| Section 4's bound range is inconsistent at p = 0.2% (seat 2 B1) | Confirmed. Now 7.5 MB to 3.7 GB, so today is 1x to 500x the bound. |
| S4.3's rollback is not executable (seat 1 B1, seat 2 B2) | Confirmed: `restore_snapshot.go:183` compares unconditionally and the paper keeps 2 (`snapshot_roots.go:43`). S4.3 dropped. |
| quick_check skips index checks; the page root cannot see a stream at another head (seat 1 B2) | Confirmed (`compact.go:63-66`). Section 7: the witness stays. |
| Register before append evicts the on-tape root (seat 1 B3, the coordinator) | Confirmed with file:line. Now Stage 0, built first. |
| The replay guard rests on a false claim (seat 1 B4) | Confirmed (`run.go:849-851`, `rematerialize.go:164-167`). Dropped. |
| Keep two breaks the MaxCandidates contract (seat 1 B5, seat 2 S4) | Confirmed (`snapshot_roots_test.go:115-124`). Dropped. |
| A 3 h window equal to the 3 h backoff ceiling forces re-uploads (seat 2 S3, seat 1 nit 1) | Confirmed (`snapshot_producer.go:89`). The window change is dropped. |
| The sidecar size rule loses readings (seat 1 S1) | Accepted: compare `min(old, new)` pages, and count growth as changed. |
| A compaction swap contaminates the meter (seat 2 S5) | Confirmed (a rename at `compact.go:373`). The sidecar carries the inode, and such intervals are excluded. |
| Partition page root after `RemoveAll` (seat 1 S2); the truncation test does not reach the page root (seat 1 S3); binary framing of the page root (seat 1 S6); S4.1's proof cannot fail (seat 2 S8); quick_check as a new refusal path (seat 2 S7); duplicate owner of the page-root check (seat 2 nit 13) | Moot: the page root is dropped. Each is correct about the earlier text. |
| Bottlenecks below g = 128 (seat 1 S4) | Accepted: S4.2 is limited to g >= 128. Below that, the record names what a benchmark must cover. |
| Payload caps of 64 MiB (seat 1 S4) | Verified (`privacy.go:51`, `event.go:30`, `format.go:103`). Replaces my [?]. |
| The meter need not touch the restore path; store layout is the main lever (seat 1 S5) | Accepted. S4.1 is observation only, and section 6 says so. |
| Restore proof on a paper-sized store (seat 2 S6) | Accepted (`snapshot_large_test.go`). |
| Per-file counts first, as yellow (seat 2 nit 14) | Accepted: S4.1 step 1. |
| g = 64 needs only the chunk limit (seat 2 nit 9) | Correct (6.0 MB < 8 MiB). Fixed. |
| "Returns pages in order" should be "enforces" (seat 2 nit 10) | Correct. Fixed. |
| "Collected within an hour" holds only at retention 2 (seat 2 nit 11, seat 1 nit 2) | Correct. Fixed. |
| A re-upload on cut change and on rollback (seat 2 nit 12) | Accepted. Stated. |
| `artifact_gc.go:678` and the 168 h run grace (seat 1 nit 4) | Verified: the 24 h chunk grace governs unrooted chunks. No change. |
| Missed contributors: hot-row updates, smaller c_g (seat 1 S5) | Accepted. Added to 3(a). |

No finding was rejected.

## 11. Precommitted claim for the first stage (Stage 0)

To be entered in `docs/calibration/claude-ledger.md` when Stage 0's build
starts.

- **Claim:**
  - Stage 0 is accepted by both seats within two review rounds, lands, and
    deploys with no fix commit.
  - The read-only audit query in section 5, run on Node B after the
    deploy and again after the paper's next recorded snapshot, shows every
    computer's `RecoveryBase` root named on the tape.
  - It also shows no computer with fewer named roots than
    `min(retention, named snapshots taken)`.
- **Probability: 0.45.** Red lanes on this path took three rounds
  (afe50a926, d6b88cd63) or more (C64). The change is host only and
  reuses an existing predicate, which helps.
- **Test:** review outputs, CI run, deploy receipt, the audit query.
- **Settle by:** 2026-11-17T12:00Z.

## 12. Residuals

- The split of the 5,858 uploaded chunks between the state file and the
  partitions (S4.1 step 1 logs it).
- Whether zstd's output is stable across `klauspost/compress` versions
  [?].
- Whether the 2026-10-31 reading of 16% in 50 minutes counted repeated
  checkpoint writes [?].
- The paper's replay rate is unmeasured, and the bound rests on d@e's
  rate.
- `prepareHibernate` does not refuse a capture in flight
  (`hibernate.go:354-366`, per the root-check record). With the 24 h
  window kept, this needs a hibernation of more than a day.
