# Restore bases that cost the change (2026-11-15)

Status: revised after review round 1. That round had one seat, DeepSeek v4.1
flash; the Gemini seat failed on quota. It returned REPAIR with four blocking
findings. Section 8 lists each finding, my verdict and what changed.

Class: green. This is a design record and changes nothing. It covers the
"going forward" half of the owner's ask of 2026-11-15: "we need to solve the
paper restore issue in a storage efficient way now and going forward." The
"now" half (restore as an accepted asynchronous act with a status poll) is
being built elsewhere and is not designed here. The owner clarified the same
day: "I'm more interested in the going forward. We don't need to restore
right now. Autopaper data isn't precious yet." So options are weighed as
long-term mechanisms, by bytes and capture time per base at the paper's
scale and at ten times that scale. Format changes and one-time migrations are
allowed. Where one would lose something, this record says what.

Labels:

- **[M]** measured, with its source;
- **[C]** read in code at origin/main f6c8176e0;
- **[W]** a primary web source;
- **[E]** an estimate, arithmetic shown;
- **[I]** inferred;
- **[?]** not known.

Only [M], [C] and [W] are facts.

## 0. The answer in brief

1. **A base is a cache of replay.** It adds no information. Its value is the
   restore time it saves; its cost is the bytes and capture time it takes.
2. **The first draft recommended page-delta bases. The corrected arithmetic
   does not support building them now.** A page delta beats today's
   fixed-chunk full capture on bytes only when the share of changed pages,
   p_1, is well under half the share of changed 4 MiB ranges, p_1024, at the
   same interval. Nobody has measured p_1. Two structural features of the
   paper's store push p_1 up:
   - 19 `og_objects` indexes, several keyed by values that scatter;
   - a daily `incremental_vacuum` that moves up to 250,000 pages
     (`telemetry_retire.go:333`, `compact.go:105`).

   At the four-minute replay bound the paper would take 13 deltas a day. At
   p(24 h) = 50% that is about 12 GB a day, worse than today (the reviewer's
   figure, confirmed in section 3).
3. **The capture-time problem and the byte problem are separate, and the
   capture-time problem has a cheap answer.** The paper's capture takes 37
   minutes because the whole-store logical witness runs inside the live read
   transaction. The page stream itself takes 3 minutes.

   If every install path ended at the restore gate's checkpoint witness,
   capture would need no witness: the page root, computed from the pages
   read, proves the bytes. Today the cold boot path has no such gate. That
   change, "one restore path", takes the paper's capture pin from about 37
   to about 3 minutes whatever the base format.
4. **"Doing less" now ranks first at the paper's scale.** The new ranking:
   - Make the replay budget an explicit asynchronous restore budget (60 to
     120 minutes, not 4) and take full bases on that cadence, without the
     live witness pin.
   - Delete the copying of `co_super_slots` and `agent_evidence` into the
     compact copy (`carryDirectTables`). Those tables are not on the tape,
     and they break restores today, not just in this design.

   Expected cost on today's 39.6 GB store: about 3 to 5 GB a day with a pin
   of about 3 minutes per capture [E]. After storage mission T3 shrinks the
   store toward its 10 GiB target, about 1 to 1.5 GB a day [E].
5. **At ten times the scale, no full-capture scheme survives**, whether
   today's, the compact copy or a logical export. An incremental base is
   necessary there. Whether it should be page deltas or logical row deltas
   is decided by measurement (landings 0 and 1), against an explicit rule in
   sealed bytes a day. The page-delta design is kept, with the reviewer's
   corrections, behind that gate.
6. **The largest lever is outside this design: the store's size.** About half
   of the store is copies of tape payloads (`og_bodies`,
   `run_memory_entries`). Every base, delta and capture shrinks with it.

## 1. The frame, and why it was chosen

The record applies four transforms
([cognitive-transform-portfolio](../../skills/cognitive-transform-portfolio/SKILL.md)).

- **Teleology.** A base bounds how long a rebuild takes. With restore
  asynchronous and the data not precious, the bound is a choice: R minutes
  of replay at the measured rate. Today's four-minute bound was set to fit a
  ten-minute request that is going away. The cadence follows from R:
  - interval I(R) = R × 77 × 60 / 10,100 hours = 0.457 × R hours [E];
  - R = 4 min gives 1.83 h; R = 60 min gives 27.4 h; R = 120 min gives
    54.9 h.
- **Limit case.** Take ten times the scale: 101,000 events an hour, and a
  store of 396 GB unbounded or about 100 GB if T3 bounds it. Any O(store)
  capture then fails its budget. This separates the mechanism that is right
  now from the one that is right at scale.
- **Physical bounds.** Replay costs about 13 ms an event on the paper [M].
  Writing pages costs microseconds. So a page or row image is the right
  encoding for a base, and the floor on bytes is what changed.

  Three things inflate what changed at page grain:
  - index leaves keyed by scattered values;
  - page moves from incremental vacuum;
  - renumbering by VACUUM INTO.

  Row grain is immune to all three but costs index maintenance at restore.
- **Deletion-first and one mechanism.** The same check, the restore gate's
  canonical checkpoint witness, can serve every install path. Then the
  separate capture-time witness, the install-time witness
  (`VerifySnapshotFile`) and the capture floor become unnecessary.

## 2. Facts, with sources, and corrections to the brief

### The paper's store, compaction and replay

- **The store is 39.6 GB.** Guest observability at 2026-11-15T05:49:12Z:
  `page_count` 9,669,301 at 4,096 bytes, `freelist_count` 1, `auto_vacuum` 2
  [M].
- **Compaction**, from the paper's console:
  - 00:42:04Z: "compact copy: due by freelist, 1356242 of 11628568 pages
    free, live ~42075447296 bytes";
  - 02:08:45Z: "staged 38811598848 bytes at sequence 11823018 in 45m13s
    (copy 17m17s)";
  - 03:43:13Z: "rebased onto the compact copy at sequence 11823018:
    46605361152 -> 38811598848 bytes, 13782 events to replay" [M].

  VACUUM INTO took 17 minutes for 42 GB live. The remaining 28 minutes were
  `quick_check` and fsync on the private copy (`compact.go:482`) [C].
- **The paper's replay rate, measured for the first time.** Console line at
  03:46:13Z: "replay applied 13782 events through seq=11836800 in 2m59.439s
  (77 events/s; fetch 45.834s in 14 pages, payload prefetch 17.167s, apply
  1m56.296s: reduce 1.213s, verify 2.351s, prepare 6.582s, finalize 1m46.139s
  of which payload fetch 3.03s; commit 126ms)" [M]. This was a cold cache
  after the rebase.
  - **What the producer uses.** `snapshotDue` takes the slower of 113 and
    the measured rate (`snapshot_producer.go:379-381`) [C]. But the
    measurement lives in process memory. The paper restarted at 05:32:51Z,
    so its block at 05:46:26Z reads `replay_rate_source: assumed`, 113 [M].
- **The event rate.** The producer block at 05:46:26Z reads newest 11,537,493
  (taken 2026-11-13T22:36:18Z) and head 11,853,375 [M]. That is 315,882
  events in 31.2 hours, about **10,100 an hour** [E]. Over the ten minutes
  from 05:36Z to 05:46Z the tail grew by 1,518 [M].
  - **Correction:** not 7,400. The current tail replays in about 68 minutes
    at 77/s, not 46 [E].
- **The last full captures.**
  - 2026-11-13T22:36Z: 46.7 GB in 37 min 40 s, 4.8 GB sealed added
    ([store doubles](../problems/the-papers-store-doubles-in-eleven-days-2026-11-13.md))
    [M].
  - 11-12: witness 22 min 37 s, stream 3 min 02 s, read transaction
    25 min 51 s; 5,858 of 10,524 chunks uploaded (24.6 GB of plaintext,
    4.04 GB sealed, about 6x on uploaded chunks); 64,433 events after the
    previous root
    ([snapshots-cost-the-change](snapshots-cost-the-change-2026-11-13.md)
    section 2) [M].

  So on a grown file, p_1024 over about 4.5 hours of paper events was about
  0.56 [E: 5,858 / 10,524].
- **Not verified:** the brief's 11,144 chunks and 11.5 GiB for the 22:36Z
  root.
- **What fills the store** (11-14, on a 47.5 GB file): `og_bodies` at least
  12.4 GB; `run_memory_entries` at least 11.2 GB; `og_objects` 5.4 GB plus
  its 19 indexes at least 7.7 GB; 2.17 M of 11.6 M pages overflow
  ([bounding the guest store](bounding-the-guest-store-2026-11-14.md)
  section 0) [M]. Tables are about 70% and indexes about 30% [E].
- **The compaction bench** (synthetic, 3.38 GB, the paper's full hourly write
  volume). 4 MiB ranges unchanged on a compacted file: 83.4% at 1 h, 80.6%
  at 12 h, 49.0% at 24 h. On a grown file, 0% at every reading
  ([paper-compaction](paper-compaction-2026-10-31.md) section 5) [M,
  synthetic]. So the bench's p_1024(24 h) is about 0.51 on a compacted file.

### Capture, restore and replay code

- **Capture** reads the head as the read transaction's first statement
  (`snapshot.go:547`), then streams `SELECT pgno, data FROM sqlite_dbpage`
  (`:696`) in fixed 4 MiB ranges (`:67`), and computes witness v3 in the
  same transaction (`:240`) [C].
- **The witness** streams every row of every table ordered by primary key
  (`sqlite_state_extractor.go:412-447`). `og_objects` is keyed
  `obj_id INTEGER PRIMARY KEY` (`objectgraph/sql_store.go:231`).
  `run_memory_entries` is keyed `entry_id TEXT` (`store/schema.go:94-95`)
  [C].
- **Restore** (`restore_snapshot.go`):
  - `planRestoreTarget` filters candidates by the snapshot event's sequence
    (`:108-136`, `snapshot.PlanCandidates`, `snapshot.go:945`);
  - the event is appended at head + 1 (`appender.go:984`), after the
    capture;
  - `Install` refuses a record whose version is not `Version`
    (`snapshot.go:1002`);
  - it probes only the candidate's own chunks (`:1035-1045`);
  - it refuses an existing destination (`:878`);
  - it empties the partition directory before installing (`:1210-1214`)
    [C].
- **The cold boot path** installs with `Verify: agentcore.VerifySnapshotFile`
  and no checkpoint gate after replay (`autoputer/projection_base.go:190-199`)
  [C]. The staged restore runs the canonical checkpoint witness before the
  flip (`rematerialize.go`, per the review; the gate's existence is
  confirmed in the [restore problem](../problems/restore-cannot-finish-inside-its-ten-minute-request-2026-11-14.md))
  [C].
- **The compact swap** copies `co_super_slots` and `agent_evidence` from the
  original into the copy (`compact.go:700-720`, `compactCarriedTables`). They
  are written by direct SQL, not by tape ops, and the witness excludes only
  `DerivedKeysTable` (`restore_snapshot.go:218-232`) [C]. On the paper:
  `agent_evidence` 150 rows, `co_super_slots` 0 rows [M, `store_tables` at
  05:49Z].
- **Host retention** (`platform/snapshot_roots.go`):
  - retention 2 for a store over 2,048 chunks (`:43-49`);
  - chain gap 360,000 events (`:63-64`);
  - a root may name at most 65,536 chunks (256 GiB at 4 MiB), in a body of
    at most 8 MiB (`:73-74`) [C].
- **Replay** opens one transaction per event (`computer_events.go:248`) at
  `synchronous=NORMAL` (`replay_durability.go:26`) [C].
- **Guest memory** is 8.3 GB (`memory` block) [M].

### Host and disk

- `df` on Node B at about 05:50Z: 195 GiB free [M].
- The paper's `data.img`: 128 GiB apparent, 42 GiB allocated [M].
  **Correction:** the 01:35Z problem record gave 88.5 GiB. The cause was not
  investigated.

### SQLite and chunking, from primary sources

- "The VACUUM INTO command is transactional in the sense that the generated
  output database is a consistent snapshot of the original database." The
  page says nothing about output layout
  (https://www.sqlite.org/lang_vacuum.html) [W].
- **Page pointers** (https://www.sqlite.org/fileformat.html) [W]:
  - interior pages hold child page numbers;
  - overflow pages begin with the next overflow page number;
  - ptrmap entries hold parent page numbers;
  - a WAL frame header holds the page number.
- **Incremental vacuum** moves pages and "The database file is truncated by
  the same amount" (https://www.sqlite.org/pragma.html#pragma_incremental_vacuum,
  quoted by the review) [W, not re-fetched by me].
- **The session extension** sees one database handle's changes, and only on
  tables with a declared primary key
  (https://www.sqlite.org/sessionintro.html) [W].
- **`sqlite3_snapshot_open`** fails once a checkpoint overwrote the snapshot,
  and needs `SQLITE_ENABLE_SNAPSHOT`
  (https://www.sqlite.org/c3ref/snapshot_open.html) [W].
- **Litestream** ships WAL pages into transaction files, takes daily full
  snapshots, restores from the newest snapshot at or below the target plus
  later files, and holds a long read transaction to own checkpointing
  (https://litestream.io/how-it-works/) [W].
- **FastCDC** (USENIX ATC 2016,
  https://www.usenix.org/conference/atc16/technical-sessions/presentation/xia)
  [W].

## 3. The options, with numbers

### The cost model

Inputs:

- paper: S = 39.6 GB; full sealed F ≈ S / 6 = 6.6 GB [E]; 10,100 events an
  hour; 77 events/s;
- ten times: 101,000 events an hour; S = 396 GB (F ≈ 66 GB) unbounded, or
  100 GB (F ≈ 17 GB) bounded.

n(R) = 24 / I(R) bases a day.

- A fixed-chunk full writes p_1024(I) × F per capture.
- A page-delta chain writes p_1(I) × F × (1 + 1/k) per delta. A full refresh
  happens whenever the packs reach k × F. Bytes kept are about (1 + k) × F
  plus the previous chain. I use k = 1 throughout.
- The seal ratio of packed changed pages is assumed equal to the whole
  store's [?].

**Sealed bytes a day on the paper (today's store), by restore budget R:**

| R (replay) | I | n/day | Fixed-chunk full, live (p_1024 0.56 to 0.9) | Fixed-chunk full, compacted (p_1024 0.5) | Page delta at p_1 = 1% / 5% / 20% / 50% |
| --- | --- | --- | --- | --- | --- |
| 4 min | 1.83 h | 13.1 | 48 to 78 GB [E]; 13 captures | about 43 GB [E] | 1.7 / 8.7 / 35 / 87 GB [E] |
| 60 min | 27.4 h | 0.88 | 3.2 to 5.2 GB [E] | about 2.9 GB [E] | 0.12 / 0.58 / 2.3 / 5.8 GB [E] |
| 120 min | 54.9 h | 0.44 | 1.6 to 2.6 GB [E] | about 1.5 GB [E] | 0.06 / 0.29 / 1.2 / 2.9 GB [E] |

At R = 4 min, p_1 is the per-delta share over 1.83 h. At R = 60 and 120 it
is the share over about a day or two, so the columns are not one variable.

**The decision rule this table yields.** At equal R, page deltas win on
bytes iff 2 × p_1(I) < p_1024(I). On a grown file p_1024 is near 1 within
hours. On a compacted file it is about 0.5 at a day (bench). So page deltas
need p_1(1 day) under about 0.25. The figure the reviewer computed, about
12 GB a day at p(24 h) = 50% at the four-minute cadence, is this table's
first row.

**Ten times, R = 60 min:** I = 2.7 h, 8.8 bases a day.

- Fixed-chunk full: 8.8 × 0.9 × 66 ≈ 520 GB a day unbounded, and about
  135 GB a day at 100 GB [E]. Capture reads the whole store each time.
  This fails.
- Page delta: about 1,160 × p_1 GB a day unbounded, and about 300 × p_1 GB
  a day bounded [E]. This needs p_1(2.7 h) of about 1% to stay near 3 GB a
  day.
- At ten times, nothing works unless the store is bounded and its page
  churn per change is low (the append-clustered layout), or the base is
  logical.

### Option 1: content-defined chunking of the whole-store capture

- On the live file, SQLite rewrites pages in place. CDC is then equivalent to
  small fixed page groups plus a rolling hash (rejected on 11-13) [C].
- On successive VACUUM INTO copies, the bytes shift and also change: every
  pointer-bearing page changes with its page number, and 19% of the paper's
  pages are overflow pages [W, M]. Capture time stays whole-store.
- **Rejected at every scale.**

### Option 2: page-delta bases

How the changed pages are found:

- (a) by comparing page hashes in one read transaction;
- (b) by reading WAL frame headers before each checkpoint (Litestream-like;
  needs a checkpoint takeover);
- (c) the session extension, rejected (one handle, keyed tables only).

Costs: the table above. Capture is about 3 minutes of pin on the paper for
(a), and about 27 minutes at 396 GB [E]. (b) reads only dirty pages.
Restore applies packs at disk speed.

Structural risks to its byte cost on this store:

- scattered index keys;
- incremental vacuum moving up to 250,000 pages a run (1 GB of raw pages,
  2.6% of the file, in one interval) [C, E];
- every compaction renumbering every page.

**Ranked third:** right at ten times only if measured p_1 is low. Kept behind
a gate (landing 2), with the review's corrections in its specification.

### Option 3: the compact copy as the base

- Capture: VACUUM INTO pins the live WAL for 17 minutes [M]. The rest runs
  on a static private file and pins nothing.
- Bytes: a full F (6.6 GB) per copy, with no dedupe between copies [I, W].
- It is not a state at its own sequence until the carry is removed (B3).
- At ten times, a 396 GB VACUUM INTO is about 2.8 hours [E], beyond
  `compactCopyBudget` = 90 minutes (`compact.go:60`). quick_check is about
  4.7 hours [E], beyond `compactCheckBudget` = 4 hours (`:67`). It does not
  fit today's budgets at ten times.
- **Ranked as an optional source of fulls at the paper's scale**, after
  landing 6 (landing 3).

### Option 4: tape-only restore made fast

Measured per-event cost: finalize 7.7 ms, fetch 3.3 ms, prefetch 1.2 ms,
prepare 0.48 ms, verify 0.17 ms, reduce 0.09 ms [M].

- Overlapping fetch with apply gives at most about 104 events/s [E].
- Deferred-index bulk apply might take finalize to 1 to 2 ms
  [E, unmeasured].
- 2,000 events/s is out of reach without replacing event-by-event apply with
  net-row application, which is a logical delta by another name.

**A side landing (5)**, not the base answer.

### Option 5: logical bases (rows, not pages)

- **Full.** The witness's own row stream, kept: tables only, about 28 GB of
  plaintext, 4.7 to 7 GB sealed [E]. It ships no index pages, and it is
  immune to page moves and renumbering. Restore imports and builds indexes:
  the split rebuilt 22.4 GB in 579 s, of which index build was 379 s [M],
  so about 18 minutes for the paper [E].
- **Dedupe across bases.** Successive full exports chunked by CDC in
  insertion order dedupe on appends; a delete or update costs about one
  chunk. Unmeasured [?].
  - `run_memory_entries`' text key means primary-key order is not insertion
    order for that table, so the export and the witness disagree on order
    there [C].
- **Logical deltas** need per-row change tracking on about 15 tables, or a
  coalesced net-row form of the tape tail. Applying net rows still pays
  index maintenance, so restore gains over replay are 2 to 3 times at most
  [E].
- **Ranked second for ten times.** It wins over page deltas if landing 0
  shows index pages dominate p_1, or if incremental vacuum makes p_1 spiky.

### Option 6: doing less

Full bases at an explicit asynchronous budget R (60 to 120 minutes), taken
without the live witness pin (landing 7), with fixed chunks as today.

- Bytes: about 1.6 to 5.2 GB a day on today's store, by R and file state [E,
  table]. Capture pin: about 3 minutes, not 37 [E].
- Restore: fetch, page-root check, at most R minutes of replay, then the
  checkpoint gate.
- At T3's bounded store (10 GiB, F about 1.7 GB), about 0.4 to 1.5 GB a day
  [E].
- At ten times it fails, as shown above.

**Ranked first at the paper's scale.** It is the interim and, if T3 lands,
possibly the steady state for every computer below a few tens of GB.

### Summary (re-ranked)

| Rank | Option | Paper, bytes a day | Paper, pin per capture | Ten times | Deletes | Gate |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | 6 do less: fulls at budget R, no live witness, one restore path | 1.6 to 5.2 GB [E] | about 3 min [E] | fails | live witness pass, install witness, capture floor, 4-min bound | landings 6 and 7 first |
| 2 | 5 logical bases | full 4.7 to 7 GB; deltas [?] | witness-pass length, or 0 from a static copy | possible with deltas | page format, VACUUM INTO compaction (import is a compaction) | landing 0 shows index-page dominance |
| 3 | 2 page deltas | 2 × p_1 × F × n [E] | about 3 min (a); dirty pages (b) | possible if p_1(2.7 h) ≈ 1% | full captures as routine | measured bytes a day at the R cadence < 0.5 × the fixed-chunk full's |
| — | 3 compact copy as full | 6.6 GB per copy | 17 min pin | fails budgets | separate live full | after landing 6 |
| — | 4 faster replay | 0 | 0 | constant factor | none | side landing |
| — | 1 CDC whole | as today | 37 min | fails | none | rejected |
| — | today | 2.2 GB a day at the ceiling [E: 4.8 GB per 53 h] | 37 min [M] | fails | — | — |

## 4. Why not "make replay fast"

Even at 2,000 events/s, a restore without a base replays 11.8 M events, 1.6
hours. At 77 events/s it is 43 hours [E]. Fast replay shortens the tail but
cannot replace the base. Cheap bases make replay speed nearly irrelevant.
The asymmetry still holds.

## 5. Recommendation: ordered landings

Every landing ships on, with no setting. Proofs run on d@e. Paper
observations are read-only.

**Order of execution:** 0 and 1 (measure) → 6 (carry) → 7 (one restore path,
fulls at budget R) → 5 at any time → 2 or a logical-base design only behind
the gate → 3 optional → 4 only at scale. The numbering is kept from the
first draft so that references stay stable.

### Landing 0 (yellow, offline): a first reading of p before any production change

Extend `TestCompactMeasure` (`compact_measure_test.go`). Hash every page at
0, 1.8, 6 and 24 hours of the paper's hourly volume, and report:

- p at 1, 32, 128 and 1,024 pages;
- the changed pages split by tree type (index or table, from `dbstat`
  owners);
- the zstd-sealed size of the packed changed pages;
- the new bytes when two full logical exports (insertion order, FastCDC at
  an average of 64 KiB) are compared.

Run on the build VM. No production surface, and nothing is deleted.

**What the bench can and cannot decide.** Each difference from the paper
biases it in a known direction:

- **About a sixth of the paper's size at the full hourly volume.** The share
  p is biased high, because the same writes land on fewer pages. Absolute
  changed bytes are about right, slightly low, because collisions are more
  frequent in a small file.
- **Two tables, with bodies inline.** The paper has `og_objects` with 19
  indexes, a separate `og_bodies`, and `run_memory_entries` with 5 indexes.
  Fewer scattered index leaves per write bias changed pages **low**.
- **Almost no in-place updates.** The paper rewrites `choir.run` about 19
  times per run, and each update moves `updated_at` index entries. Biased
  **low**.
- **No deletes.** The paper retires daily and then moves up to 250,000 pages
  with incremental vacuum. Biased **low**, and the bench cannot show the
  spike on retirement days at all.

Net: absolute bytes are biased low. The bench can **falsify** page deltas
(if even its bytes a day at R = 60 fail the rule, stop). It **cannot confirm**
them, and it says little about logical dedupe under deletes.

**Prediction** (claude ledger, settle by 2026-11-18): on the compacted
synthetic file, p_1(24 h) ≤ 0.25, and the share of 4 MiB ranges *unchanged*
at 24 h is at most 0.55 (p_1024 ≥ 0.45, anchored on the bench's 49.0%).
Probability 0.55.

### Landing 1 (orange; rollback: revert): the page-hash meter on every computer

**What it does.** Every 6 hours, when no capture is running:

- stream `sqlite_dbpage` in one read transaction, head first;
- keep a ring of four sidecars (6 h, 24 h, 72 h, 7 d), each holding
  16-byte truncated SHA-256 page hashes keyed by inode, page count and page
  size (154 MB each on the paper, 616 MB in all);
- compare against each slot, and log:
  - p_1 and p_1024;
  - the sealed size of a 1% sample of changed pages, extrapolated;
  - the **predicted sealed bytes a day** for page deltas and for
    fixed-chunk fulls at the R = 60 cadence, using the measured rate and
    events an hour, including the k = 1 refresh;
- report the same in the `snapshot_producer` block, as added fields (none
  removed);
- mark a slot that spans a file swap or a retirement run, so the spike is
  visible, not averaged away.

**Proof on d@e.** Readings at 6 h and 24 h. The page count equals the
`store_tables` header's. The bytes-a-day lines are present. (p_1 ≤ p_1024
holds by construction and is only a wiring check.)

**Rollback.** Revert and delete the sidecars.

**Deletes nothing.**

**Prediction for the paper** (settle 48 hours after the deploy, from its
log):

- each read takes at most 5 minutes and pins at most 1 GB of WAL;
- the 24-hour slot shows p_1024 ≥ 0.45 (the file was compacted at 03:43Z;
  bench 0.51) and p_1 ≤ 0.30.

Probability 0.45.

**Gate for landing 2.** Measured page-delta bytes a day at the R cadence,
including refresh, must be under half the fixed-chunk full's, on two
readings that each span a retirement run. If index trees hold more than two
thirds of changed pages, write option 5 up instead.

### Landing 2 (red, two seats; built only behind landing 1's gate): page-delta bases

The specification, as corrected by review:

- **B1 invariant.** The page root is computed only from pages read in the
  capture's own read transaction, whose first statement reads the head
  (`snapshot.go:547`). The sidecar only selects which pages ship and is
  never a source of hashes.

  The root covers every page: a delta's root is computed over the parent's
  hashes for unselected pages *as re-read and re-hashed in this
  transaction*. That means a full read, which is option (a); option (b)
  requires a periodic full re-hash to keep this invariant.

  **Test:** a sidecar that claims one changed page is unchanged makes the
  restore refuse on the page root.
- **The root's domain.** It binds page size, page count, hash size and
  arity, and is content-deterministic (not `filecas.BuildManifest`'s root,
  which hashes `CreatedAt`, `filecas.go:114`). Restore truncates to the
  record's page count, because incremental vacuum shrinks files.
- **Hashes.** 16 bytes of SHA-256.
- **B2.**
  - The record carries `BaseSequence`, the content head of the chain's full
    base, and `StoreHeadSequence`.
  - `PlanCandidates` and `PlanRecovery` use the chain base for the bound and
    the content head for dating.
  - `Install` probes every chunk of the whole chain before downloading.
  - Record fetches per candidate count against `CandidateBudget`.
- **The record carries no changed-page list.** Packs carry page numbers; a
  list would be 38 MB on the tape against a 64 MiB cap
  (`privacy.go:51`).
- **Partitions.** Deltas carry none. The full's partitions are installed
  once. Partition capture drops to the full cadence.
- **Witness.** The full's witness, if kept, is verified on the full before
  packs are applied. The chain is checked by its page root and then the
  checkpoint gate (landing 7).
- **Host.** The root is a tree of chunk lists, so that 396 GB stores fit
  inside `maxSnapshotRootChunks` and `maxSnapshotRootBody`
  (`snapshot_roots.go:73-74`). The host's chain rule
  (`snapshotRootChainGap`, `:63-64`) is kept. **Test:** a restore to a
  checkpoint between two fulls finds a base.
- **Versions.** The version gate widens to a supported set
  (`snapshot.go:1002`). A discarded compact copy deletes its sidecar.
- **Observability.** The fields stay for one release.
- **Rollback.** An old binary's producer reads the newest snapshot event,
  whatever its version, as current, so it takes no full until its bound
  fires. Its restores refuse v2 candidates until `CandidateBudget` runs out.
  Restores can therefore refuse for hours after a rollback. Landing 2
  publishes a v1-readable full before its first delta, so an old binary has
  a base.

**Prediction** (enter only after landing 1): the bytes a day it reports
within 1.5 times landing 1's prediction. No probability is set until then.

### Landing 3 (red, two seats; after landing 6): the compact copy may be a full base

**What it does.** After `checkCompactCopy`, the witness and the page stream
are computed on the static staged copy and published as a v2 full whose
`StoreHeadSequence` is the copy's sequence. The v2 version exists so that a
rolled-back binary refuses it.

**Requires landing 6.** Before it, the copy is not a state at its own
sequence once the carry writes later rows into it.

**Rebase bound.** Re-derived from the measured rate: 450,000 events is
97 minutes at 77/s, not 30 minutes. The bound becomes about 140,000 events
for 30 minutes [E].

**Not at ten times** unless the store is bounded: the budgets in option 3
fail.

**Prediction:** none until built.

### Landing 4 (red, two seats; built only at scale): WAL-frame dirty tracking

**Gate:** a landing 2 read takes more than 15 minutes, or deltas are needed
more often than every 2 hours.

**What it does.** The store takes over checkpoints. Any unclean boot, swap
or scan gap forces landing 2's full comparison. A periodic full re-hash
keeps the B1 invariant.

### Landing 5 (orange; rollback: revert): overlap replay fetch with apply

**Proof.** d@e's replay line shows fetch hidden under apply.

**Prediction:** the paper's next cold replay after a planned rebase reads
at least 95 events/s (77 measured). Probability 0.6.

### Landing 6 (red, two seats): the carried tables become tape-derived, and the carry is deleted

**The defect, which exists today.** `co_super_slots` and `agent_evidence`
are written by direct SQL with no tape op (`compact.go:700-716`), and they
are inside the witness (`restore_snapshot.go:218-232`) [C]. So:

- any restore whose base predates a write to them, and whose target
  checkpoint follows it, fails the witness;
- the compact swap fills the copy with rows from a later head.

That the first failure has happened is [I], not observed. `agent_evidence`
has 150 rows on the paper [M].

**What it does.** Route both tables' writers through tape ops. The graph
paths `evidence.graph` and `engineering.slots_graph` exist and are off by
default (`compact.go:708-711`), which is also a no-switches violation. Then
delete `carryDirectTables` and `compactCarriedTables`.

**What is lost:** the paper's 150 `agent_evidence` rows, unless the landing
migrates them with a one-time tape write. Data is not precious, so a
migration op is preferred but not required. Say which in the commit.

**Proof on d@e.** Write an evidence row after a snapshot, bind a checkpoint,
and restore: the answer is `witness_matched: true`.

**Rollback.** Revert. Rows written as ops stay readable through the graph
path only if the old binary has it on: name that in the rollback.

**Prediction:** the d@e restore above answers 200 with
`witness_matched: true` on the first deployed attempt. Probability 0.55.

### Landing 7 (red, two seats): one restore path, and fulls at an explicit asynchronous budget

**What it does.**

- **One restore path.** Cold boot recovery becomes the staged restore to the
  newest checkpoint at or above the base, gated by the canonical checkpoint
  witness, then the ordinary catch-up to head. A failed gate falls back to
  the next older base. With no base, it falls back to genesis replay, and
  says so by name.
- **Fulls.** They are captured with a page root computed from the pages
  read, head first (the B1 invariant), and no whole-store witness in the
  live read transaction.
- **The cadence.** Due when the tail's replay at the slowest measured rate
  exceeds R. R is a constant derived from the asynchronous restore budget
  (60 minutes proposed; the owner may set it) and replaces
  `snapshotReplayBound` (`snapshot_producer.go:101`). A replay rate measured
  at boot is persisted beside the store, so a restart does not fall back to
  113.

**Deletes:**

- the live witness pass in capture;
- `VerifySnapshotFile` at install;
- the capture floor, its persisted record and the WAL headroom rule (the
  observability fields stay one release, reading `floor_limited: false`);
- the separate cold-install verification design.

**Protected surfaces.** Snapshot records and capture, both restore paths,
the restore gate, recovery admission.

**Conjecture delta.** Before: each base is witnessed logically at capture
and at install. After: each base proves its bytes by page root, and each
restore proves its result by the checkpoint witness before anything serves.

**Heresy delta.** This overrules section 7 of
[snapshots-cost-the-change](snapshots-cost-the-change-2026-11-13.md) ("the
witness stays"). It is acceptable only because every path now ends at a
logical gate. It needs the owner's word, because it depends on checkpoints
being bound regularly on every computer. The paper's cadence binding is
[?].

**What is lost.** A rolled-back binary cannot restore from v2 fulls, and as
in landing 2 its producer may not take a v1 full for hours. Publish one v1
full at the switch.

**Proof on d@e.**

1. Destroy and cold-recover d@e (the N7 drill).
2. The log shows the checkpoint gate pass, then the catch-up.
3. A forced bad base (test-level) refuses by name and falls back.

**Prediction for the paper** (72 hours after the deploy, read-only):

- every capture's read transaction is at most 6 minutes;
- sealed bytes per capture are at most 5 GB;
- captures fire at most twice a day at R = 60;
- the block never reads `floor_limited: true`.

Probability 0.5.

## 6. Measure first

The deciding numbers:

- p_1 and p_1024 at the R-implied interval;
- the sealed bytes of the packed changed pages, with the index or table
  split;
- the logical-export dedupe.

In order of cost:

1. **Landing 0 on the build VM.** It can only falsify, for the reasons in
   landing 0.
2. **Landing 1 on every computer.** Its 24-hour and 72-hour slots must span
   a retirement run.

Two further cheap reads:

- the paper's next `snapshot_producer` block after a boot replay, to see
  whether it plans at 77 or 113;
- d@e checkpoint bind regularity, as a precondition for landing 7.

Restore install time for a 40 GB base remains [?]. Time `snapshot.Install`
against a synthetic 40 GB root on the build VM rather than restoring the
paper.

## 7. Residual risks

- **The checkpoint cadence becomes load-bearing.** If a computer binds no
  checkpoint above its base, landing 7's cold path falls back to genesis
  replay (43 hours on the paper at 77/s).
- **The checkpoint witness's blind spots** (derived tables, ignored columns)
  become the only logical check. Index and freelist corruption were
  invisible to the old witness as well. The page root covers bytes.
- **A failed cold install that left a non-empty state file** is not cleared
  by a retry (`snapshot.go:878`). Under landing 7, cold recovery stages,
  so the live path is protected. The operator step for a pre-landing-7
  wreck is to remove the file.
- **The incremental-vacuum spike** may make page deltas unattractive
  whatever the mean p. Landing 1 marks it.
- **The ten-times caps** fail as written:
  - the root caps at 396 GB (`snapshot_roots.go:73-74`);
  - the compact budgets (`compact.go:60`, `:67`).

  Landing 2's tree of lists addresses the first. A bounded store addresses
  the second.
- **Unverified facts:**
  - the brief's 11.5 GiB;
  - whether `og_objects` updates keep `obj_id`;
  - whether `modernc.org/sqlite` exposes sessions;
  - why `data.img` allocation fell;
  - the incremental vacuum quote, taken from the review.
- **The largest lever is not in this design:** storage mission T3. Every
  number above scales with S.

## 8. Review round 1 and what changed

Seat: DeepSeek v4.1 flash, REPAIR. I checked each finding against the code
before deciding.

| Finding | Verdict | Change |
| --- | --- | --- |
| B1: the page root must come from pages read, never the sidecar | Accepted. Head first is at `snapshot.go:547`. | The invariant and its test are in landing 2. The same invariant governs landing 7's fulls. |
| B2: the bound and the filter use the event sequence; probing covers only the candidate's own chunks | Accepted. `restore_snapshot.go:108-136`, `snapshot.go:945`, `:1035-1045`, `appender.go:984`. One nuance: mis-dating by the event sequence only loses candidates. `Install` refuses a record above `MaxSequence` (`snapshot.go:1002` area), so it is a coverage defect, not a safety one. The bound against a chain base is a real safety defect. | `BaseSequence`, content dating and a whole-chain probe in landing 2. |
| B3: the compact copy is not a state at its sequence after the carry | Accepted, and it is wider than the design: the tables are off the tape, so restores across a write to them fail today. `compact.go:700-720`, `restore_snapshot.go:218-232`. | New landing 6 (tape-derive the tables, delete the carry). Landing 3 depends on it and publishes v2. "Nothing is lost" is withdrawn; the 150 rows are named. |
| B4: landing 1 cannot measure p(24 h); the p_1024 prediction contradicts the bench; the gate is on the wrong variable | Accepted. The first draft read 49.0% unchanged as changed. | A sidecar ring. The gate is on predicted sealed bytes a day at the R cadence, including refresh. The predictions are corrected. Section 3 gives the cost model; page deltas drop to rank 3, and doing less with one restore path ranks first. |
| Ten-times caps | Accepted (`snapshot_roots.go:73-74`, `compact.go:60`, `:67`). | Section 3, option 3, landing 2 (tree of lists), section 7. |
| Keep the full's witness verified on the full | Accepted for landing 2. Landing 7 removes it, which only the one restore path justifies. | Landing 2 and landing 7. |
| Page-count shrink, the root's domain, `CreatedAt` in the manifest root | Accepted (`telemetry_retire.go:333`, `filecas.go:114`). | Landing 2. |
| Drop the changed-page list from the record | Accepted. | Landing 2. |
| The rollback claim is false | Accepted. | A v1 full is published at the switch (landings 2 and 7). |
| The host chain rule conflict | Accepted. The rule is the host's `snapshotRootChainGap`. | Kept, with a test (landing 2). |
| 16-byte cryptographic hash | Accepted. | Landings 1 and 2. |
| Partitions | Accepted (`snapshot.go:1210-1214`). | Landing 2. |
| Observability deletion | Accepted. | The fields stay one release (landings 2 and 7). |
| The rebase tail at the measured rate | Accepted. 97 minutes at 77/s. | Landing 3. |
| The producer already uses the measured rate | Partly accepted. It does (`snapshot_producer.go:379-381`), but the rate is process memory and the paper's block reads `assumed` after its 05:32Z restart [M]. | Section 2. Landing 7 persists the rate. |
| Recoverability after a failed cold install | Accepted. | Landing 7 stages cold recovery. Section 7 gives the operator step. |
| The witness question | Accepted as framed. The cold path lacks a logical gate. | Landing 7 makes the checkpoint gate cover the cold path. That is what allows the capture witness to go. |
| Nits: the p floor 0.11%; 6x is on uploaded chunks; the "49% changed" label; a tautological proof; 2.8 h against a 90-minute abort; the version gate; `MaxCandidates` cost; the discard-path sidecar | All accepted. | Corrected in sections 2 and 3 and landings 0 to 3. |

Nothing in the review was rejected. One part was narrowed: the mis-dating
half of B2 is a coverage defect, not a safety defect.
