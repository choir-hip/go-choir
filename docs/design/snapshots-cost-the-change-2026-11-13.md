# Snapshots that cost the change (2026-11-13)

Class: green (design record; nothing changed by this file). Station S4 of
[choir-storage-solutions-2](../definitions/choir-storage-solutions-2-2026-11-12.md)
("snapshots cost the change, keep two"), design record first, two seats for
every red stage below. Inputs: `internal/snapshot/snapshot.go`,
`internal/filecas/filecas.go`, `internal/agentcore/snapshot_producer.go`,
`internal/agentcore/restore_snapshot.go`, `internal/recoveryplan`,
`internal/platform/snapshot_roots.go`, `internal/platform/artifact_gc.go`,
`internal/store/compact.go`, `internal/store/telemetry_retire.go`;
[chunk drain trace](../evidence/paper-chunk-drain-trace-2026-11-12.md),
[storage frame](storage-pressure-frame-2026-11-12.md),
[append-clustered store](append-clustered-store-2026-10-31.md) (its
section 6 holds the only page-level churn measurements we have),
[root check problem](../problems/snapshot-root-recorded-without-chunk-check-2026-11-12.md),
[loop problem](../problems/snapshot-loop-blocks-its-own-fix-2026-11-12.md),
the storage mission ledger.

Tags: **[C]** read in code at the cited line on origin/main 77a2a8129;
**[M]** measured, with its source; **[E]** estimate, arithmetic shown;
**[?]** not known. Nothing marked [E] or [?] is a fact.

## 0. The answer in brief

1. **The brief's 25 GB a snapshot is stale.** It is the paper's first
   snapshot of 2026-10-30, sealed before chunks were compressed (25.5 GB
   sealed, data-system build ledger; zstd before sealing landed in
   f1ebf0e4c and 8492760aa on 2026-10-31). The paper's latest snapshot
   (2026-11-12 23:28Z, head 10,978,774) uploaded **5,858 of 10,524 chunks,
   4.04 GB sealed**, from 44.1 GB of pages, in 25 min 51 s: witness 22 min
   37 s, stream 3 min 02 s [M, storage mission ledger]. A completed
   snapshot costs about 4 GB a day at one snapshot every 22 hours, not
   25 GB. The 6 GiB an hour of 2026-11-12 was the failed-attempt loop, fixed
   by 0d2f0bd46. The comments that still say 25 GB
   (`snapshot_producer.go:62-69`, `snapshot_roots.go:37-41`) are stale.
2. **Sealing is not the cause.** `filecas.SealChunk`
   (`internal/filecas/filecas.go:184-215`) uses a synthetic nonce, HMAC-SHA256
   of the plaintext under a key derived from the DEK and the computer ID
   (`chunkNonce`, lines 234-248). Equal plaintext gives equal sealed bytes
   and an equal digest. On the host, two roots taken at the same frozen head
   share 10,510 of 10,510 chunks [M, drain trace section 1].
3. **The cause is a 1,024-page cut over writes scattered across the whole
   file.** `streamFile` sets `fs.pagesPerChunk = ChunkSize / pageSize`
   (`internal/snapshot/snapshot.go:671`, `ChunkSize = 4 << 20` at line 66).
   A 4 MiB range is new if any one of its 1,024 pages changed. If each page
   changes with probability p between snapshots, and changes are spread
   out, a range survives with probability (1 - p)^1024. For 88% of ranges
   to change, p need only be about 0.2% (about 11,000 pages, 45 MB) [E]. The
   paper's writes are spread out for four reasons, each in the code or
   measured:
   - inserts land on random leaves: the 136-byte canonical id is the last
     column of 13 of the 16 `og_objects` secondary indexes [M, append-clustered
     section 0];
   - the paper's file has `auto_vacuum=INCREMENTAL` [M, 2026-10-31], so it
     carries a pointer-map page every 820 pages or so, about 1.25 per 4 MiB
     range. A pointer-map page is rewritten when any page in its range is
     allocated, freed or gets a new parent. In the synthetic grown store,
     1,000 inserts rewrote 1,287 pointer-map pages in 1,211 ranges [M,
     append-clustered section 6.1];
   - the daily retirement (`telemetry_retire.go:300-320`) deletes old rows
     all over the file. `reclaimFreePages` (`compact.go:973-1012`) then
     runs `PRAGMA incremental_vacuum`, which moves up to 250,000 pages
     (`incrementalVacuumMaxPages`, `compact.go:105`) from the end of the
     file into holes across it;
   - a compaction swap (`prepareCompactCopy`, swapped at a planned restart)
     renumbers every page. The snapshot after it is a full one, which is
     expected and rare.

   The page reads are not the cause either: `sqlite_dbpage` returns pages in
   page-number order and the stream refuses anything else
   (`snapshot.go:722-725`). SQLite pages do not move when rows are
   inserted, so the cut points are already stable. Content-defined
   chunking has no shifted content to find.
4. **What we do not know decides the station:** how many distinct pages
   change between two snapshots. Two readings disagree by about 70 times.
   The 88% of ranges in 30 minutes needs only p ≈ 0.2%. The 2026-10-31
   clone reading, 16% of blocks rewritten in 50 minutes, says p ≈ 16% [M,
   append-clustered section 0]. At p = 0.2%, cutting at 64 pages would send
   about 12% of the file instead of 88% [E]. At p = 16%, every cut size
   above a few pages sends almost all of it. **The first stage is a meter
   that measures p, plus a page root.** The page root makes the meter's
   hashing do a second job: it checks restore and can later replace the
   22-minute witness.
5. **Recommendation:** S4.1 page root and dirty-page meter (red, guest).
   S4.2 smaller content-keyed page groups for the state file, built only
   if the meter shows they pay (red, guest, plus host limits only if the
   group is under 128 pages). S4.3 the page root replaces the
   in-transaction witness (red; an owner question). S4.4 the 3-hour
   unrooted-chunk window and keep-two for every computer (red, host).
   Section 5 has the stages.

## 1. What a snapshot is for

A snapshot is not a backup of information. Everything in it is already on
the host: the tape holds every event, and the store is a projection of the
tape (`docs/computer-ontology.md`, "Guest store"). A snapshot is a **cache
of replay**: it bounds how much tape a rebuild must apply. The bound is
`recoveryplan.MaxRecoveryTailEvents = 720000`
(`internal/recoveryplan/recovery_plan.go:22`), about 40 minutes at the
298 events a second measured on d@e. The producer fires at three quarters
of it, `snapshotDefaultTailEvents` = 540,000 (`snapshot_producer.go:70`).
That is about 22 hours on the paper at 24,000 events an hour.

Three things follow:

- The marginal information in a snapshot is zero. Its cost is a price
  paid for not replaying. Its floor is the bytes needed to state the store
  at head N+T, given that the host holds the store at head N. That is the
  pages that differ (section 4).
- If snapshot k changes D(T) distinct pages over an interval of T, the
  daily cost is (24 h / T) x D(T). D is concave, because a hot page
  rewritten twice counts once. So **snapshots should be as far apart as the
  recovery bound allows**. A trigger that fires earlier than the bound
  requires costs more bytes, not fewer. "A replay-cost trigger" (S4.4)
  belongs in this design only as a correctness guard: fire earlier when a
  replay of the tail would be slower than the bound assumes.
- A snapshot must be checkable on its own. A restore verifies before it
  installs (`VerifySnapshotFile`, `restore_snapshot.go:173-186`), and the
  newest usable candidate wins (`snapshot.Install`, `snapshot.go:868-890`).

## 2. Today's snapshot, measured

| Quantity | Value | Source |
|---|---|---|
| state.db | 22.38 GB, 4 KiB pages, about 5,337 chunks of 4 MiB | [M] drain trace section 1 |
| telemetry partitions | about 21.7 GB, about 5,187 chunks; closed ones are stable bytes | [E] 44.1 GB of pages - 22.4 GB |
| chunks in the latest root | 10,524 | [M] ledger 2026-11-12T23:30Z |
| uploaded at the latest snapshot | 5,858 chunks, 4.04 GB sealed (0.69 MB a chunk, about 6x compression) | [M] same |
| held already | 4,666 | [M] same; the log does not split state from partitions |
| witness | 22 min 37 s (22 to 35 min seen) | [M] same; `snapshot.go:563-573` |
| stream | 3 min 02 s | [M] same |
| read transaction (WAL pin) | 25 min 51 s; the WAL reached 3.7 GB in 30 min under load | [M] same; drain trace |
| budget | 90 min (`snapshotDefaultMaxDuration`) | [C] `snapshot_producer.go:79` |
| roots kept per computer | 2 when a root names over 2,048 chunks, else 4 | [C] `snapshot_roots.go:34-43`, prune at 98 |
| unrooted-chunk window | 24 h from last write or claim (a HEAD claims) | [C] `artifact_gc.go:180`, cutoff at 677 |
| collector | hourly, at most 50,000 files and 32 GiB a run | [C] `nix/node-b.nix` `PLATFORM_ARTIFACT_GC_*` |

The previous root (19:18Z, head 10,914,341) was 64,433 events earlier, but
a crash boot onto a new binary came between the two roots. So the 23:28Z
reading is not a clean "64k events changes every range" measurement [?].

**Superseded roots are already collected within about an hour.** A root
leaves retention when the root after next is recorded. Its own chunks were
last written or claimed when it was captured, about two intervals (about
44 h) earlier. They are past the 24 h window, so the next hourly run takes
them. The 24-hour window costs bytes only for **failed attempts**: chunks
that no root names, held 24 h after upload. Since 0d2f0bd46 the witness
runs before upload, so a budget abort uploads nothing. A failure after the
upload (an append failure, or a 409 from the root check) still leaves
about 4 GB for 24 h.

## 3. The options

Costs are for the paper's state file. Partitions stay at 1,024 pages a
chunk in every option: closed partitions never change. "Bytes" means new
sealed bytes per snapshot. p is the share of pages changed between two
snapshots; p_g is the share of g-page groups changed.

### (a) Today: fixed 1,024-page ranges, deterministic sealing

- Bytes: p_1024 x 22.4 GB / 6 ≈ 3.7 to 4.0 GB [M]; p_1024 ≈ 0.88 to 1.0.
- Guest CPU and time: zstd plus XChaCha20 over 44 GB of pages on 4
  workers, inside the 3-minute stream; HMAC-SHA256 per chunk for the nonce.
- WAL pin: the whole read transaction, about 26 min (the witness is 87% of
  it).
- Restore: about 10.5k sequential GETs (`assemble`, `snapshot.go:1044-1082`),
  then the witness again in `VerifySnapshotFile` (22 to 35 min) inside the
  boot deadline, then up to 720k events of replay.
- Code risk: none (it is today).
- Why the dedupe is poor: section 0, item 3. Not sealing: the nonce is
  synthetic (`filecas.go:187-199`). Not a volatile header: page 1 is in
  chunk 0 only. Not page reordering: the stream is in page order. VACUUM
  and incremental vacuum: yes, a real part, daily. Scattered B-tree and
  pointer-map writes: yes, the main part.
- One latent cause found while reading: the CAS digest is SHA-256 of the
  *sealed* bytes (`sealChunkWithNonce`, `filecas.go:261`), and the sealed
  bytes include zstd output. A `klauspost/compress` upgrade that changes
  the encoder's output would re-key every chunk once [?, whether the
  encoder's output is stable across versions is not checked]. One full
  upload, then normal.

### (b) Small fixed page groups keyed by content (g pages a chunk, g < 1,024)

Same format, same sealing, same CAS, same root check, same collector. Only
`pagesPerChunk` changes, for the state file.

- **Today's restore already reads it.** `restoreOne` accepts any
  `PagesPerChunk` with `PageSize x PagesPerChunk <= 2 x ChunkSize` and a
  chunk count equal to ceil(PageCount / PagesPerChunk)
  (`snapshot.go:963-976`). `assemble` checks every chunk against
  `PageSize x PagesPerChunk` and a whole number of pages
  (`snapshot.go:1059-1065`). So a binary rolled back to today's main
  restores the new snapshots: the rollback keeps every root restorable.
- Bytes: p_g x 22.4 GB / c_g, where c_g is zstd's ratio at group size g
  (lower than 6 for small groups [?]). If changes are spread out,
  p_g ≈ 1 - (1 - p)^g. At p = 0.2% and g = 64, about 12% of the file
  (about 0.45 GB sealed at c = 6) [E]. At p = 16%, nearly 100% (no gain).
- Chunk count per root: 5.46 M pages / g, plus 5.2k for the partitions.
  g = 128: about 48k. That fits today's host limits:
  `maxSnapshotRootChunks` = 65,536 and `maxSnapshotRootBody` = 8 MiB at
  about 67 bytes a digest, about 3.2 MB (`snapshot_roots.go:45-47`) [E].
  g = 64: about 90k. That needs both limits raised. g = 16: about 340k,
  with a 23 MB declaration and a 23 MB sealed record on the tape. Too big.
- Guest CPU and time: the same seal work. More HEAD probes (one for every
  chunk not repeated within the snapshot): about 48k at g = 128, roughly
  0.5 to 1 minute on 4 workers at about 2 ms a round trip [E, latency not
  measured].
- Host: root registration stats every chunk under the CAS reuse lock.
  10.5k chunks took 30 to 95 ms [M, root check problem], so 48k would take
  about 0.15 to 0.45 s [E, linear]. The live-root `LIKE` query takes
  2.3 ms against two 10.5k roots [M], so about 10 ms at 48k [E]; the
  generation shortcut makes it rare. There are more files on btrfs (the
  small-file metadata history is in the
  [FP10 note](fp10-snapshot-levers-and-host-slab-2026-10-22.md)): about
  48k files per paper root against 10.5k, and the hourly collector walks
  them.
- Tape: the sealed record carries the manifest, about 3 MB at g = 128
  against about 0.7 MB today [E]. No cap on private payload size was found
  in `computerevent` or `platform` [?, verify before S4.2].
- Restore: about 48k sequential GETs. At about 2 ms each that is about
  1.6 min plus transfer [E]. A prefetching `assemble` is a follow-up if
  needed.
- WAL pin: unchanged (the witness dominates).
- Code risk: low. One parameter in `streamFile`, per-file geometry already
  recorded, and the readers exist.
- Confidentiality delta: the host learns which 512 KiB regions changed,
  instead of which 4 MiB regions. The CAS already learns chunk equality
  (`filecas.go:196-198`).

### (c) Content-defined chunking (rolling-hash boundaries)

CDC finds shared content that has *shifted*. A SQLite file does not shift:
an insert rewrites pages in place and appends pages at the end. With an
average chunk size A, a changed page changes the one or two chunks around
it, which is (b) with g ≈ A / 4 KiB, except that boundaries are not page
aligned. The restore then loses its page checks, and the guest pays for a
rolling hash over 44 GB. After a compaction pages are renumbered, and a
B-tree page's bytes hold its children's page numbers, so the content
itself differs and CDC does not help there either. **Rejected:** no gain
over (b), and new code on the restore path.

### (d) Page-diff against the previous root

The guest sends only the changed pages, packed together, plus a page map.
"Changed" comes from a page-hash table the guest keeps, or from SQLite.
SQLite has no persistent per-page change tracking for the main database.
The session extension tracks rows, not pages. Reading WAL frame headers
before every checkpoint would give exact page numbers, but auto-checkpoints
and the `CheckpointAfterLongRead` path make that fragile. A full hash scan
is cheap because the stream already reads every page in 3 minutes.

- Bytes: at the bound (section 4). Packs compress better than single pages.
- Restore: either a chain (base plus every diff) or a full page map whose
  entries point into packs from many snapshots.
- Collection: a pack stays live while any live root names any page in it.
  "Keep two" then frees nothing until a pack's pages are all overwritten,
  so the host needs repacking or a periodic full base. That means a new
  record format, a new restore, and a log-structured store on the host,
  all red.
- Code risk: high.
- **Deferred, not rejected.** It is worth building only if the meter shows
  that (b)'s amplification p_g / p is above about 2 at every usable g
  *and* the absolute bytes still matter. With the root check, the collector
  and the backup all keyed on whole chunks, (b) gets most of the gain for
  almost none of the risk.

### (e) Not snapshotting the store, and bounding replay some other way

- Replay from genesis: the paper's tape is over 10.9 M events, about 10 h
  at 298 events/s. Breaks the 720k bound. Rejected.
- A host-side reflink copy of `data.img` (btrfs copy-on-write, so the cost
  is the changed 4 KiB extents, the physical bound without compression).
  It lives on the same disk as the image it protects, so it gives no
  protection against losing that disk. It names nothing on the tape, and
  it contradicts "the host keeps no projection bases: a guest recovers from
  its own sealed snapshots plus the tape tail" (ontology). The image is
  also not encrypted by the guest
  ([data at rest](../problems/data-at-rest-snapshots-images-escrow-key-plaintext-2026-10-21.md)),
  so the copy is plaintext at rest. Rejected: it is a doctrine change, not
  a lever.
- Faster replay, so the bound can grow and snapshots can be rarer: the
  bound is the owner's (720k is preserved here). The paper's own replay
  rate is unmeasured (`recovery_plan.go:18-21`: "Restate it when the
  paper's own rate is measured"). Kept as an open question (section 7).
- Lower the bound itself by changing the store's write locality: the
  append-clustered layout (og_id keys, about half the pages per insert) and
  a compaction cadence. In the synthetic store, 83% to 88% of 4 MiB ranges
  of a compacted file were unchanged after 24 h, against 0% for a grown
  file [M, append-clustered section 6.1]. That is the strongest lever on
  the bound, and it is outside S4 (it is the store's own mission).

### (f) Collect superseded roots at once, and the shorter unrooted window

- A superseded root's chunks are already collected within about an hour
  (section 2). "Immediate" would mean deleting inside the root-recording
  transaction. That saves under an hour of bytes and would put an unlink
  under the reuse lock on the registration path. Rejected; hourly is enough.
- The window that matters is `fileCASChunkUploadGrace` = 24 h for unrooted
  chunks (`artifact_gc.go:180`). At 3 h, a failed attempt's bytes leave
  in 3 to 4 h instead of 24 to 25 h, about 4 GB-days less for each failure
  after upload [E].
- What made 3 h unsafe was a root recorded for chunks already deleted
  (silent). afe50a926 closed that: registration stats every chunk under the
  lock and answers 409, and the collector keeps pending and live-root
  chunks. So a capture that spans a hibernation longer than the window now
  fails loudly. It takes the existing backoff and a retry re-uploads only
  what is missing. That cost is a retry, not a bad root. The planned-restart
  lane (`lane/restart-cancels-capture`, parked) is no longer a precondition
  for the window. It remains the fix for the 90-minute refusal.
- The window also serves file sync (a root at the end of each pass, every
  15 min; `HandleFileCASRoot` refuses a missing chunk with 409, per the
  root check problem record). 3 h covers a pass with 12x margin.
- "Keep two": large roots (over 2,048 chunks) already keep 2, small ones
  keep 4 (`snapshot_roots.go:34-43`). Under (b) every non-trivial store
  passes 2,048 chunks, so the count test stops meaning "large". Keep two
  for every computer, as the station says. Correct the comment on
  `snapshot.MaxCandidates` (`snapshot.go:896-899`), which says the
  platform keeps at least four.

### Summary

| Option | New bytes a snapshot (paper) | Guest time | WAL pin | Restore | Code risk |
|---|---|---|---|---|---|
| (a) today | 3.7 to 4.0 GB [M] | 26 min | 26 min, 3 to 11 GB WAL | 10.5k GETs + witness 22 to 35 min | none |
| (b) g = 128 | p_128 x 3.7 GB; about 0.85 GB (p = 0.2%) to 3.7 GB [E] | about 27 min | unchanged | 48k GETs + witness | low |
| (b) + S4.3 page root | same | about 5 min [E] | about 5 min, under 1 GB [E] | 48k GETs + page root about 1 min + quick_check [?] | medium (evidence change) |
| (c) CDC | as (b), worse alignment | + rolling hash | unchanged | new checks | medium, no gain |
| (d) page-diff | at the bound | about 5 min | about 5 min | chain or map | high |
| (e) no snapshot | 0 | 0 | 0 | breaks the 720k bound | rejected |
| (f) 3 h window | -about 4 GB-days per failed attempt | 0 | 0 | 0 | low (host constant) |

## 4. The physical bound

The least a snapshot can cost is the compressed size of the pages that
differ between it and the root before it:

    B(T) = sum over the distinct pages changed in (N, N+T] of |zstd(page)|
         ≈ p(T) x 22.4 GB / c_1

where T is the snapshot interval (540k events, about 22 h on the paper),
p(T) is the share of the state file's 5.46 M pages that differ, and c_1 is
the compression ratio of pages sent as a pack (about 6 at 4 MiB [M]; a
pack of changed pages should be similar [?]).

What we know about p(22 h): it is at least the share needed to make about
88% of 4 MiB ranges change in 30 minutes (p ≈ 0.2% [E]). It is at most 1.
One clone reading on 2026-10-31, at half today's event rate, saw 16% of
blocks rewritten in 50 minutes. If that is representative, p(22 h) is
probably above 50% [E: hot pages repeat, so it is not 26 x 16%]. **So B is
somewhere between about 0.1 GB and 3.7 GB a snapshot, and today's cost is
3.7 to 4.0 GB.** Today is at most about 40x the bound and possibly near
it. Existing data cannot narrow this, which is why S4.1 measures it.

How S4.1 measures it: the stream hashes every page it reads (SHA-256, in
page order, in the reader goroutine). It keeps the first 8 bytes of each
hash in a guest-local sidecar, `snapshot-pagehash` beside the store (5.46 M
x 8 B = 44 MB). It compares against the previous sidecar while streaming,
reading the old file in step, since pages arrive in order, so memory stays
O(1). It logs: pages changed (p), groups changed at g in {8, 16, 32, 64,
128, 256, 1024} (p_g), and the zstd ratio of a fixed sample of changed
pages packed together and in g-groups (c_1, c_g). That one line gives B
and the cost of every (b) variant against it.

How far the recommendation is from the bound: (b) costs
(p_g / p) x (c_1 / c_g) x B. If the paper's changes are as spread out as
the synthetic store's (about 2 pages per touched 4 MiB range per 1,000
inserts [M, append-clustered 6.1]), then p_g / p ≈ g at small p: 128x the
bound in pages, but with the absolute amount still small. If changes
cluster, the ratio falls toward 1. S4.2 picks g by the rule in section 5
so that (b) stays within 2x the bound wherever the bound is worth chasing.
Below about 0.5 GB a snapshot (about 0.5 GB a day, under 0.5% of Node B's
135 GiB free), chasing the bound further (option d) is past the point of
diminishing returns.

## 5. Recommended path

One path, four stages. Each ships on, with no setting. Each has its own
two-seat review and its own deployed proof. Acts on the paper are not
allowed: its proof is reading its log and the host's tables after it boots
a release in the normal way.

### S4.1 Page root and dirty-page meter (red: restore verification)

What: in `streamFile`, the reader hashes each page with SHA-256 as it is
read. The hash comes from the `sql.RawBytes` before the bytes go into the
chunk buffer, so it is independent of buffer reuse and chunk assembly. A
file's page root is SHA-256(`"choir/snapshot/page-root/v1"`, page size,
page count, the page hashes in order). `Record` gains `page_root`, and
`PartitionEntry` gains `page_root` (both omitempty; old readers ignore
them). `snapshot.Install` recomputes the root over the assembled file and
each partition before `Verify`, and refuses on a mismatch (the next
candidate is tried, as today). The meter and its sidecar as in section 4:
written atomically after a recorded snapshot only. A missing or
wrong-sized sidecar means "no reading this time". The sidecar is never on
the correctness path. Stale comments are corrected: the 25 GB figures, the
"no auto_vacuum" comment in `telemetry_retire.go:10`, and `MaxCandidates`.

Failure list (written before code):
1. the hash is taken after the bytes enter a reused buffer, so a buffer
   race goes unseen (take it from the row, before `append`);
2. the root covers pages but not geometry, so a truncated file with a
   matching prefix verifies (geometry is in the hash input);
3. a record with no `page_root` (every existing root) is refused (absent
   means not checked; the witness still runs);
4. the sidecar is trusted for anything but the log line;
5. the sidecar is written for a failed attempt, so the next reading
   compares against a state that was never recorded;
6. hashing in the reader slows the stream past the budget (SHA-256 at
   about 0.5 to 1.5 GB/s on one core is 30 to 90 s over 44 GB [E]; the
   stream's time is logged);
7. the sidecar's 44 MB lands on a full data volume (it goes under the same
   free-space floor as the WAL, `snapshotFreeFloor`);
8. partitions are skipped by the root, so a wrong partition installs.

Deployed proof: d@e takes two snapshots. Its log shows the page root and a
meter line. Then a rematerialize of d@e from its tape installs the newest
snapshot, and the log shows the page root verified. That is the
`rematerialize-from-tape` owner act
(`cmd/choir/commands.go:443`); who runs it on d@e is the lead's to
arrange [?]. A drill with a corrupted chunk is not needed: the AEAD already
refuses one. A unit test truncates the assembled file and must be refused.
The paper, read only: after it boots the release by a normal deploy
refresh, its next "snapshot taken" line carries the meter. That is the
first measurement of p(22 h).

Rollback: revert. Records with `page_root` stay readable by the old binary
(an unknown JSON field). The sidecar stays on disk, harmless.
Protected surfaces: snapshot record, restore path (red). Conjecture delta:
none for content, plus a byte-level check at restore. Evidence class: a
new byte-equality check added beside witness v3. Heresy delta: none.

### S4.2 Content-keyed page groups for the state file (red: snapshot format and the host's root limits)

Gate: built only if S4.1's paper reading shows some g >= 32 with
(p_g / p) x (c_1 / c_g) <= 2 and p_g <= 0.5. Otherwise the snapshot is
already within about 2x of what a smaller cut can reach. The byte lever
then moves to the store's layout (option e, last item), and S4 goes on to
S4.3 and S4.4. That is a decision by measurement, not a switch.

What: `streamFile` takes pages-per-chunk as an argument. The state file
uses g, the largest value that meets the gate. Partitions stay at 1,024.
If g < 128, the host's `maxSnapshotRootChunks` and `maxSnapshotRootBody`
become byte bounds (256 GiB of plaintext at the smallest allowed g), and
retention becomes "keep two" for every root (option f). No reader changes:
section 3 (b) shows the existing restore reads it.

Failure list:
1. the producer and an older restore disagree on geometry (covered by
   today's checks; a test restores a g = 64 record with the
   restore code at 77a2a8129);
2. root registration exceeds the host's limits and every snapshot fails
   (checked against the paper's real page count before landing, and the
   host change lands first);
3. the sealed record outgrows a payload limit on the tape (find any limit
   first: none was found by grep [?]);
4. registration holds the CAS reuse lock too long at 48k to 90k chunks
   (measure on the build VM, as the root-check lane did for 10.5k);
5. the first g-cut snapshot uploads the whole file once (expected:
   about 4 GB, once; say so in the report);
6. the collector's 50,000-file cap spreads a superseded root's deletions
   over two runs (fine; noted);
7. restore time grows past the boot deadline (measure the d@e restore;
   add a prefetching `assemble` only if it is over).

Deployed proof: d@e takes two snapshots, and the second shows held chunks
in proportion to its meter reading. A rematerialize of d@e verifies witness
and page root. The paper, read only: two consecutive snapshots, uploaded
bytes against the meter's prediction, and on the host
`computer_snapshot_roots` shows 2 roots with the new chunk counts.

Rollback: revert the guest. The old binary restores g-cut records. Revert
the host limits only after no live root exceeds them. Protected surfaces:
snapshot chunks and records, root registration (red). Evidence class:
unchanged. Heresy delta: none. Confidentiality delta: finer change
locality is visible to the host (owner question 3).

### S4.3 The page root replaces the in-transaction witness (red: evidence class)

What: `capture` stops computing witness v3 inside the read transaction
(`snapshot.go:563-573`). The read transaction becomes the stream alone:
about 3 to 5 minutes and under 1 GB of WAL [E, from 3.7 GB in 30 min].
`Record.WitnessRoot` is left empty in new records. `VerifySnapshotFile`
checks the page root, then `PRAGMA quick_check` [?, its time on 22 GB is
not measured]. It computes witness v3 only when the record carries one, so
old records verify as today. The budget `snapshotDefaultMaxDuration` falls
from 90 min to a value set from the measured stream time (a deadline, not
a switch). That also shortens the window in which a planned restart is
refused.

Why it is safe: the only consumer of `WitnessRoot` is
`VerifySnapshotFile` (grep: `restore_snapshot.go:183`). It proves that the
restored file has the logical content the capture saw. The page root
proves more for this purpose: the restored file is the captured pages,
byte for byte, hashed from the rows as read. What the witness adds is an
independent logical path that can catch a page stream that was internally
consistent but not the database's state (for example a broken
`sqlite_dbpage` read). `quick_check` covers the structural half of that.
The logical half is the owner's call (question 1).

Failure list: an old record is refused (no); a new record restores without
any content check (no: page root plus quick_check); the restore at boot
still spends 22 to 35 min on the witness (no: new records carry none);
`quick_check` is slower than the witness (measure on d@e and on a copy of
a large synthetic store before landing).

Deployed proof: d@e snapshot with read-transaction time logged; a
rematerialize of d@e verifies; the paper (read only) logs the read
transaction under 10 minutes and a WAL peak under 1.5 GB.

Rollback: revert. The old binary refuses a record with an empty
`WitnessRoot` (its `VerifySnapshotFile` compares unconditionally), so a
rollback must also keep, or take again, one snapshot with a witness. The
rollback is: revert, then the producer takes a fresh snapshot at its next
due point. Until then the newest restorable base is the previous one, and
the tail can approach the 720k bound. Protected surfaces: restore
verification, evidence class (red; two seats; owner ruling first).

### S4.4 The 3-hour window, keep two, and the replay guard (red: host collector)

What: `fileCASChunkUploadGrace` goes from 24 h to 3 h (option f).
Snapshot retention becomes 2 for every computer. The producer's trigger
stays at 540k events. A replay guard is added: the boot replay already
counts its work (`restoreReplayObserver`). Its measured events per second
are logged. If the estimated time to replay the tail at the computer's own
measured rate would exceed the time the 720k bound assumes (about 40 min),
the producer fires early. It only ever fires earlier, never later.

Failure list: a file sync's root is refused because its chunks were
collected (a sync pass is 15 min, so 3 h is 12x; 409 is loud); a capture
spans a hibernation over 3 h (it fails loudly and retries); a small
computer that kept 4 roots now keeps 2 (restore tries 4 and finds 2, which
is loud and has the same effect as today for large roots); the replay rate
is noisy and fires snapshots too often (fire only on a rate measured over
at least 10,000 events).

Deployed proof: on Node B (read only for the lead), the collector's
report shows unrooted chunks older than 3 h being taken. d@e: a snapshot
attempt that is made to fail after upload (a drill, d@e only) leaves
chunks that the collector removes about 3 to 4 h later.

Rollback: revert the constant. Protected surfaces: the collector (red; a
wrong window deletes chunks a root is about to name, which the root check
turns into a loud failure).

## 6. What must be preserved

- **Restore correctness.** Every new record verifies at restore: the page
  root from S4.1, witness v3 while the record carries one, and quick_check
  after S4.3. `Install` keeps trying older candidates on a failure. No
  stage writes a record that today's restore code cannot read, except
  S4.3's empty witness, which its rollback note covers.
- **The root-time chunk check (afe50a926).** Unchanged. Every root is a
  full list of the chunks it needs (no chains). Registration stats them
  all under the lock. The pending set and the generation counter protect
  them from the collector. S4.2 only makes the list longer.
- **Encryption at rest and key custody.** `SealChunk` and the computer's
  DEK as today. No stage exports a key, adds a key, or moves plaintext off
  the guest. The page root and the meter's sidecar stay on the guest. The
  page root goes to the host only inside the sealed record. The host
  learns finer change locality under S4.2 (owner question 3).
- **The 720,000-event recovery bound.** Unchanged. The trigger stays at
  540k, and S4.4's guard only fires earlier.
- **Old roots restorable until superseded.** Every stage keeps the v1
  record and its readers. Roots are self-contained, so superseding one
  never breaks another.
- **No switches.** No stage adds a setting. S4.2's gate is a build decision
  made from a measurement.

## 7. Owner questions, each with the conservative default

1. **May the page root plus `PRAGMA quick_check` replace witness v3 as a
   snapshot's restore check (S4.3)?** This ends the 22 to 35 min WAL pin
   and the same wait at restore. Default: no. S4.3 waits, the witness and
   the 90-minute budget stay, and S4.1 to S4.2 go ahead.
2. **The group size and the host's limits (S4.2).** Default: the largest g
   that meets the gate, and no smaller than 128, which fits today's
   `maxSnapshotRootChunks` and body limit for the paper. So there is no
   host limit change unless the owner wants the extra gain from g = 64.
3. **Finer change locality visible to the host (S4.2).** The CAS learns
   which 512 KiB regions changed between snapshots, instead of which 4 MiB
   ones. Default: accept, the same trade `filecas.go:196-198` already
   records.
4. **The 3-hour window before the planned-restart lane lands (S4.4).** A
   capture that spans a long hibernation becomes a loud failed attempt.
   Default: yes, after a read-only audit of the live roots against the
   chunk store on Node B (the open residual of the root-check record)
   shows no root naming a missing chunk.
5. **If the meter shows the snapshot is already within about 2x of its
   bound, stop S4 after S4.3 and S4.4, and put the byte lever in the
   store's layout (append-clustered S2 to S5 and a compaction cadence)?**
   Default: yes, with the reading recorded as evidence.
6. **The paper's replay rate.** The 720k bound was set from d@e's rate. The
   paper's own rate can only be measured on the paper (its payloads are
   sealed under its key, so no off-guest replay). Default: S4.4's guard
   logs it at the paper's next boot replay, and the bound is restated then
   by a separate record.

## 8. Precommitted claim for S4.1

To enter in `docs/calibration/claude-ledger.md` when S4.1's build starts
(the next free id at the time):

- **Claim (landing):** S4.1 (page root and dirty-page meter) is accepted
  by both seats within two review rounds, then lands and deploys with no
  fix commit. On d@e two snapshots log a page root and a meter line, and a
  rematerialize of d@e logs the page root verified. **Probability 0.40.**
  Recent red lanes on this path needed three rounds (afe50a926, d6b88cd63)
  or more (C64), and that sets the probability. Test: review outputs, CI
  run, deploy receipt, d@e log lines. Settle by 2026-11-18T12:00Z.
- **Claim (measurement, scored at the same time or at the paper's second
  snapshot after it boots S4.1, whichever is later, by 2026-11-21T12:00Z):**
  between two consecutive paper snapshots, the meter's p_128 is at most
  0.5. **Probability 0.30.** I lean to "spread out and high", from the
  16%-in-50-minutes clone reading and the daily incremental vacuum. Test:
  the paper's "snapshot taken" line. If it is false, S4.2 is not built
  (gate) and question 5 applies.

## 9. Residuals

- The split of the 5,858 uploaded chunks between the state file and the
  partitions is not logged. S4.1's meter line adds per-file counts.
- Whether zstd output is stable across `klauspost/compress` versions is
  not checked (option a). A dependency bump should be read against it.
- The 2026-10-31 "16% of blocks in 50 minutes" reading was of the file's
  blocks on a clone. Whether it counted WAL checkpoint writes of the same
  page more than once is not known [?].
- `prepareHibernate` does not refuse on an in-flight capture
  (`hibernate.go:354-366`, per the root-check record). Under S4.4 that
  costs a retry, not a bad root.
