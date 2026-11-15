# Restore bases that cost the change (2026-11-15)

Class: green. This is a design record and changes nothing. It covers the "going
forward" half of the owner's ask of 2026-11-15: "we need to solve the paper
restore issue in a storage efficient way now and going forward." The "now" half
is being built elsewhere: restore becomes an accepted asynchronous act with a
status poll, and this record does not design it.

The owner clarified on 2026-11-15: "I'm more interested in the going forward.
We don't need to restore right now. Autopaper data isn't precious yet." So each
option is weighed by whether it is the right long-term mechanism, and by its
cost in bytes and capture time per base, at the paper's scale and at ten times
that scale. Format changes and one-time migrations are allowed. Where one would
lose something, this record says what.

Status: proposed by an Opus designer, 2026-11-15T06:10Z. Not yet reviewed. Landing
0 (an offline bench) may run before review; landings 1 and later wait for a
two-seat review of this record.

Labels used below:

- **[M]** measured, with its source.
- **[C]** read in code at origin/main f6c8176e0.
- **[W]** a primary web source.
- **[E]** an estimate, with the arithmetic shown.
- **[I]** inferred.
- **[?]** not known.

Only [M], [C] and [W] are facts.

## 0. The answer in brief

1. **A base is a cache of replay**
   ([snapshots-cost-the-change](snapshots-cost-the-change-2026-11-13.md)
   section 1). The tape already records every change. A base is worth its
   bytes only because applying stored pages is far faster than replaying
   events. The real question is therefore: what is the cheapest encoding of
   "the store at head M, given the base at head N"?
2. **The paper's capture is expensive because of a whole-store logical
   witness, not because of the bytes.** The page stream takes 3 minutes. The
   witness takes 22 to 35 minutes and keeps the WAL pinned the whole time.
   Every whole-store mechanism (today's capture, VACUUM INTO, a logical
   export) costs time proportional to the size of the store. None of them
   survives ten times the paper's scale: at about 400 GB, the witness alone
   would take about 4 hours against a 90-minute budget.
3. **Recommendation: page-delta bases.** Each base after the first ships only
   the pages whose content changed since the base before it. Changed pages are
   found by comparing page hashes, not by trusting a log of writes. The deltas
   are packed densely, and a chain ends at a full base. A full base comes from
   the compact copy whenever one is staged, so the planned-restart rebase and
   the full snapshot are one artifact, written once. A delta carries a page
   root, not a whole-store witness. The logical end-to-end check stays where it
   already is: the restore gate's checkpoint witness.
4. **Before anything is built, one number decides the size of the win.** That
   number is p(T): the share of the paper's pages whose content changes in an
   interval T. It has never been measured. Landing 1 measures it, and an
   offline bench can give a first reading before that.
5. **Faster replay does not make the base question go away.** The paper's
   replay, measured for the first time tonight, runs at 77 events a second, not
   113. Its cost is the projection writes (59%) and the host fetch (26%), not
   signature checks (1.3%). 2,000 events a second is out of reach without
   changing what replay is. The relationship runs the other way: once bases
   are cheap, replay speed matters little, because the tail stays short.

## 1. The frame, and why it was chosen

The record applies four transforms
([cognitive-transform-portfolio](../../skills/cognitive-transform-portfolio/SKILL.md)).

- **Teleology (what is a snapshot for?)** A snapshot bounds how long a
  rebuild takes. It adds no information. Its cost should therefore follow
  what changed, not how large the store is. Today it follows size: each
  capture reads and witnesses all 39.6 GB.
- **Limit case (ten times the scale).** Take about 101,000 events an hour and
  a store of about 400 GB if nothing bounds it. Any mechanism with
  O(store) capture time then fails its own budget. This rules out options on
  structural grounds before their constants matter.
- **Physical bounds.** Two rates set the trade:
  - Replaying events costs about 13 ms an event on the paper [M, section 2].
  - Writing pages costs about 4 µs a page at disk speed [E: 4 KiB at about
    1 GB/s].

  So a page image of the change is the right encoding whenever a base is
  needed at all. The floor on bytes is the distinct changed pages,
  compressed. The ceiling is the whole store.
- **One mechanism, several jobs.** A per-page hash table beside the store does
  four jobs:
  - it is the meter for p(T);
  - it chooses which pages a delta must carry;
  - it is the page root a restore verifies;
  - it lets the compact copy serve as the full base, because after the swap
    the live file is page-for-page the copy.

  The compact copy does two jobs as well: it compacts the store, and it is
  the full base. Today it does the first and is often thrown away by a
  restart before it does anything else
  ([problem](../problems/compact-copy-grows-the-host-image-and-restarts-discard-it-2026-11-15.md)).

Two frames were rejected:

- **"Make replay fast enough that bases do not matter."** Section 4 explains
  why this fails.
- **"Store only rows, never pages."** Section 3, option 5, explains why this
  is deferred rather than chosen.

## 2. Facts, with sources, and corrections to the brief

### The paper's store and capture

- **The store is 39.6 GB.** Guest observability at 2026-11-15T05:49:12Z:
  `page_count` 9,669,301 at 4,096 bytes, `freelist_count` 1, `auto_vacuum` 2
  [M, `store_tables` header]. The compaction staged 38,811,598,848 bytes, and
  the store has grown since.
- **The compaction and its timings**, from the paper's console:
  - 00:42:04Z: "compact copy: due by freelist, 1356242 of 11628568 pages free,
    live ~42075447296 bytes";
  - 02:08:45Z: "staged 38811598848 bytes at sequence 11823018 in 45m13s (copy
    17m17s)";
  - 03:43:13Z: "rebased onto the compact copy at sequence 11823018:
    46605361152 -> 38811598848 bytes, 13782 events to replay".

  So VACUUM INTO took 17 minutes for 42 GB live [M]. The rest of the
  45 minutes was `PRAGMA quick_check` and fsync on the private copy
  (`compact.go:482`) [C].
- **The paper's replay rate, measured for the first time.** Console line at
  03:46:13Z: "replay applied 13782 events through seq=11836800 in 2m59.439s
  (77 events/s; fetch 45.834s in 14 pages, payload prefetch 17.167s, apply
  1m56.296s: reduce 1.213s, verify 2.351s, prepare 6.582s, finalize 1m46.139s
  of which payload fetch 3.03s; commit 126ms)" [M]. This replay ran on a cold
  page cache straight after the rebase.
  - **Correction:** the brief's 113 events a second is d@e's rate, which the
    producer assumes (`snapshot_producer.go:108`). The paper's tail of
    315,882 events therefore replays in about 68 minutes, not 46 [E:
    315,882 / 77].
- **The event rate.** The producer block at 05:46:26Z reads newest sequence
  11,537,493 (taken 2026-11-13T22:36:18Z) and head 11,853,375 [M]. That is
  315,882 events in 31.2 hours, about **10,100 events an hour** on average
  [E]. Over the ten minutes from 05:36Z to 05:46Z the tail grew by 1,518,
  about 9,100 an hour [M, two readings].
  - **Correction:** the brief's 7,400 an hour is low for this window.
- **The last full capture.** At 2026-11-13T22:36Z: 46.7 GB in 37 min 40 s,
  4.8 GB of sealed chunks added
  ([store doubles](../problems/the-papers-store-doubles-in-eleven-days-2026-11-13.md))
  [M]. The capture before it (11-12): witness 22 min 37 s, stream 3 min
  02 s, read transaction 25 min 51 s, 5,858 of 10,524 chunks uploaded,
  4.04 GB sealed, about 6x compression
  ([snapshots-cost-the-change](snapshots-cost-the-change-2026-11-13.md)
  section 2) [M].
  - **Not verified:** the brief's 11,144 chunks and 11.5 GiB for the 22:36Z
    root. 11,144 chunks of 4 MiB is 46.7 GB of plaintext, which matches. I
    found no record of 11.5 GiB and did not read the host tables.

### What fills the store

- On 11-14 the store was 47.5 GB. Of that, `og_bodies` was at least
  12.4 GB, `run_memory_entries` at least 11.2 GB, `og_objects` 5.4 GB, and
  its 19 indexes at least 7.7 GB
  ([bounding the guest store](bounding-the-guest-store-2026-11-14.md)
  section 0) [M].
- Tables are about 70% of the file and indexes about 30% [E from those
  floors].
- 2.17 M of 11.6 M pages were overflow pages [M, the same readings].

### How capture and replay work

- **Capture** (`snapshot.go`):
  - one read transaction streams `SELECT pgno, data FROM sqlite_dbpage`
    (`snapshot.go:696`);
  - the stream is cut into fixed 4 MiB page ranges (`ChunkSize`,
    `snapshot.go:67`);
  - the witness v3 is computed in the same transaction (`snapshot.go:12`,
    `:240`) [C].
- **The witness** streams every row of every table, ordered by primary key
  (`sqlite_state_extractor.go:412-447`). For `og_objects` that key is
  `obj_id INTEGER PRIMARY KEY` (`objectgraph/sql_store.go:231`), which is
  the rowid, so this witness is already a logical export in insertion order
  that is thrown away after hashing [C].
- **Replay finalize** opens one transaction per event (`finalizeBatch`,
  `computer_events.go:248`) and runs at `synchronous=NORMAL` during replay
  (`replay_durability.go:26`) [C]. Commit cost is negligible (126 ms in
  total). The finalize time is projection writes into a store about five
  times the guest's 8.3 GB of RAM, with a cold cache (`memory` block:
  `mem_total_bytes` 8,325,382,144) [M]. That cause is [I].

### The producer and the compact copy

- **The producer** (`snapshot_producer.go`):
  - the ceiling fires at 540,000 events (`:71`) and the interval at 7 days
    (`:66`);
  - the replay bound is 4 minutes (`:101`);
  - between replay-time captures, a floor is computed from the last capture's
    cost (`:117-118`, `snapshotDue` at `:376-413`);
  - there is WAL headroom of 2 MiB/s (`:124`);
  - the paper is `floor_limited` because the cost of its last capture is not
    recorded [M, block at 05:46:26Z].
- **The compact copy** (`compact.go`):
  - copy budget 90 minutes (`:60`), check budget 4 hours (`:67`);
  - rebase tail bound 450,000 events (`:80-82`);
  - the copy is renamed into place at a planned boot
    (`RebaseOntoCompactCopy`, `:590`), which gives the file a new inode and
    renumbers every page [C].

### Host and disk

- `df` on Node B at about 05:50Z: 195 GiB free of 476 GiB [M].
- The paper's `data.img`: 128 GiB apparent, **42 GiB allocated** [M, `du`].
  - **Correction:** the problem record of 01:35Z gave 88.5 GiB allocated.
    The image shrank since, so some discard path reached the host. How was
    not investigated.

### SQLite and chunking, from primary sources

- "The VACUUM INTO command is transactional in the sense that the generated
  output database is a consistent snapshot of the original database." The
  page says nothing about page layout or determinism of the output
  (https://www.sqlite.org/lang_vacuum.html) [W].
- **Page pointers** (https://www.sqlite.org/fileformat.html) [W]:
  - interior b-tree pages hold child page numbers;
  - the first four bytes of an overflow page name the next overflow page;
  - each ptrmap entry is "a 4-byte big-endian page number" of the parent;
  - a WAL frame header carries the page number at offset 0.

  So a file whose pages are renumbered changes the bytes of every interior,
  overflow and ptrmap page, whether or not their rows changed.
- **The session extension** monitors changes "via a single sqlite3* database
  handle", works only on tables with a declared PRIMARY KEY, and ignores rows
  with a NULL in the key (https://www.sqlite.org/sessionintro.html) [W].
- `sqlite3_snapshot_open` fails once "the specified snapshot has been
  overwritten by a checkpoint". It needs `SQLITE_ENABLE_SNAPSHOT`
  (https://www.sqlite.org/c3ref/snapshot_open.html) [W].
- **Litestream**, the production design closest to option 2:
  - it "continuously copies write-ahead log pages" into transaction files;
  - it takes a full snapshot every 24 hours by default;
  - it restores from "the most recent snapshot that does not overshoot the
    requested restore point" plus later files in order;
  - it takes over checkpointing with "a long-running read transaction".

  Source: https://litestream.io/how-it-works/ [W].
- **FastCDC** (Xia et al., USENIX ATC 2016,
  https://www.usenix.org/conference/atc16/technical-sessions/presentation/xia)
  is Gear-based CDC, reported about 10x faster than Rabin CDC with nearly the
  same dedupe ratio [W, abstract].

## 3. The options, with numbers

Paper inputs:

- S = 39.6 GB [M];
- c = 4 to 6 (seal ratio) [M, 6 on 11-12; about 4 if the brief's 11.5 GiB
  for 46.7 GB holds];
- events = 10,100 an hour [M];
- replay = 77 events/s cold [M];
- stream = 245 MB/s (44.1 GB in 3 min 02 s) [M].

Ten-times inputs:

- 101,000 events an hour;
- S = 396 GB if the store stays unbounded, or about 100 GB if storage
  mission T3 bounds it (its acceptance target is under 10 GiB at today's
  scale).

**p(T)** is the share of pages whose content changed in an interval T. It is
**[?]**, and everything that follows is written with it as a variable. Three
cases are used: p(24 h) = 5%, 20% and 50%.

Two bounds on p are known:

- p is at least about 0.2%: the 11-12 measurement that 88% of 4 MiB ranges
  change in 30 minutes needs only about 11,000 scattered pages
  ([snapshots-cost-the-change](snapshots-cost-the-change-2026-11-13.md)
  section 3a) [E].
- p is at most 1.

### Option 1: content-defined chunking of the whole-store capture

- **Live file.** SQLite rewrites pages in place and never shifts bytes. CDC
  therefore matches small fixed page groups and adds a rolling hash. This was
  rejected on 11-13 ([snapshots-cost-the-change](snapshots-cost-the-change-2026-11-13.md)
  section 3c) [C].
- **Successive VACUUM INTO copies.** These do shift: each b-tree starts after
  the ones before it in the file, so a page added early moves everything
  after it [I]. Shifting is the case CDC handles, but the bytes change as
  well as move: every interior page, every overflow page (19% of the
  paper's pages) and every ptrmap page carries page numbers that move with
  the shift [W, file format]. Copy-to-copy dedupe is **[?]**, probably low
  [I].
- **Capture time** is unchanged at about 37 minutes: the witness is still
  whole-store [M].
- **Ten times:** about 4 hours of witness [E: 25 min × 10]. This fails.
- **Deletes nothing.** Rejected.

### Option 2: page-delta bases

A delta carries the pages whose content changed since the parent base,
packed densely and sorted by page number. Three ways to know which pages
changed:

- **(a) Compare page hashes.** Keep an 8-byte hash per page beside the store
  (9.67 M × 8 B = 77 MB on the paper). At capture, read every page in one
  read transaction, hash it, and ship the pages whose hash differs.
  - Capture is about 2.7 minutes of read [E: 39.6 GB / 245 MB/s] plus
    hashing, and the WAL is pinned for about 3 minutes.
  - This needs no change to the write path. It cannot miss a change, because
    it compares contents rather than trusting a log.
- **(b) Read WAL frame headers before each checkpoint** into a dirty-page
  bitmap (1.2 MB). The store must take over checkpointing (autocheckpoint
  is SQLite's default today: `dbx.go` sets only `journal_size_limit` [C]),
  as Litestream does [W].
  - Capture reads only the dirty pages.
  - It can miss a page if any path checkpoints without the scan, which would
    be a silent defect. A crash or an unknown state must force a full base.
- **(c) The session extension.** It sees only one connection's changes,
  only tables with a declared key, and it is not exposed by
  `modernc.org/sqlite` as far as I found [W; driver exposure [?]].
  Rejected.

Costs for (a):

- **Bytes per delta:** p(T) × S / c. At p(24 h) = 5%, 20%, 50% that is
  0.33 to 0.5, 1.3 to 2.0, and 3.3 to 5.0 GB sealed [E]. The last full
  capture added 4.8 GB [M], so the win is large only if p(24 h) is well
  under 50%.
- **Bytes kept:** one full base (6.6 to 9.9 GB sealed [E: S / c]) plus its
  deltas, plus the previous chain until the new full is on the tape.
- **Restore:**
  1. fetch the full base and the deltas;
  2. write the full image, then overwrite with each delta in order;
  3. hash every page against the record's page root (about 3 minutes [E]);
  4. replay the tail, which is at most one delta interval: one hour of
     paper events is 131 s at 77/s [E];
  5. pass the restore gate (unchanged, and not this design's).
- **Ten times, (a):** reading 396 GB per delta is about 27 minutes of WAL
  pin [E], which is acceptable a few times a day. For deltas every 15
  minutes, (b) is needed: at 101,000 events an hour, one hour of tail is
  22 minutes of replay [E].
- **Complexity:**
  - a new record version naming its parent and page root;
  - a pack format;
  - a restore that assembles a chain;
  - one sidecar.
- **Deletes:**
  - the whole-store live witness pass from routine capture;
  - the capture floor and its persisted record;
  - the WAL headroom rule;
  - the `floor_limited` state;
  - the 360,000-event chain retention, replaced by "the newest chain and the
    one before it".

### Option 3: the compact copy as the base

- **Capture.** VACUUM INTO pins the live WAL for 17 minutes [M]. Everything
  after that reads a private, static file and pins nothing:
  - quick_check, about 28 minutes [M: 45m13s − 17m17s];
  - the logical witness, about 25 minutes [E, as on the live store];
  - the page stream, about 3 minutes [E].
- **Bytes:** a full S / c = 6.6 to 9.9 GB sealed for every copy. Successive
  copies renumber pages, so dedupe against the previous copy is near zero
  [I, W].
- **Cadence:** only when compaction is due (freelist over 10% or 2 GiB, or a
  layout age of 7 days; `compact.go:47-51`, `:92`). On its own it does not
  bound the tail: at 10,100 events an hour, 7 days is 1.7 M events, past
  the 720,000-event recovery bound [E].
- **Combined with option 2:** after the swap the live file is the copy, byte
  for byte. The copy's page hashes therefore seed the sidecar, and the first
  delta costs only the pages the rebase replay wrote. The synthetic bench
  found 83% of 4 MiB ranges of a compacted file unchanged after 1 hour and
  49% after 24 hours at the paper's hourly write volume
  ([paper-compaction](paper-compaction-2026-10-31.md) section 5) [M,
  synthetic]. Page-level survival is higher than range survival [I].
- **Ten times:** VACUUM INTO of 396 GB is about 2.8 hours of pin, and
  quick_check about 4.7 hours [E]. Fulls must become rare, which they are
  in a delta chain.
- **Deletes:** the separate live full capture, and the waste of a copy that a
  restart discards locally, since it is already uploaded.

### Option 4: tape-only restore made fast

Where the measured time goes [M, the 03:46:13Z line], per event:

| Phase | Per event |
| --- | --- |
| finalize | 7.7 ms |
| fetch | 3.3 ms, serial pages of 1,024 events |
| payload prefetch | 1.2 ms |
| prepare | 0.48 ms |
| verify | 0.17 ms |
| reduce | 0.09 ms |
| commit | about 0 |

Signature and receipt verification is not the bound. The bounds are the
projection writes into a 39.6 GB store larger than memory, and the serial
fetch.

Reachable improvements:

- **Overlap fetch with apply:** at most the 26% fetch share, giving about
  104 events/s [E].
- **Apply many events per transaction, with secondary indexes dropped and
  rebuilt afterwards:** the header/body split rebuilt the indexes of a
  22.4 GB store in 379 s (`store.split.summary` `build-indexes` 378,750 ms)
  [M]. This could take finalize from 7.7 ms to perhaps 1 to 2 ms
  [E, unmeasured].

2,000 events/s means 0.5 ms an event in total, which is below today's fetch
cost alone. **Not reachable** without replacing event-by-event apply with
something that is no longer replay: applying net row images, which is a
base by another name. Tail replay also grows with event rate, so at ten
times scale replay speed is a constant factor against a tenfold tail.

Deletes nothing. A fetch-overlap landing is worth doing for its own sake
(landing 5), but it is not the base answer.

### Option 5: logical bases (rows, not pages)

- **A full base** would be the witness's own row stream, kept rather than
  hashed and dropped. Tables only, about 28 GB of plaintext [E: 70% of S],
  4.7 to 7 GB sealed [E]. No index bytes are shipped, which is about 30%
  less than a page base. The format survives schema and index changes.
- **Capture:** the witness pass, 22 to 35 minutes with the WAL pinned on a
  grown file [M], or unpinned if read from the staged compact copy.
- **Restore:** import plus index build. The split rebuilt 22.4 GB in 579 s
  [M], so about 18 minutes for the paper [E].
- **Deltas** need per-row change tracking on every table: a change sequence
  column and tombstones, a schema change on about 15 tables with deletes in
  `run_memory_retire.go` and the object retire paths. Rowid order gives good
  age locality for appends, but whether `og_objects` updates keep their
  `obj_id` was not read [?]. CDC would handle the shifts caused by deletes.
- **Ten times:** a full logical export takes hours, as option 1 does, and
  deltas carry high complexity.
- **Deferred.** It becomes the better choice only if landing 1 shows that
  index pages dominate p(T), so that page deltas are mostly index churn.
  It is also what storage mission T3 does to the store itself: payloads
  named rather than copied shrink every page base by the same factor.

### Option 6: doing less

- An asynchronous restore of 46 to 80 minutes is acceptable while paper data
  is not precious.
- A base only at each compaction is not acceptable. At a 7-day layout
  cadence the tail reaches 1.7 M events: 6.1 hours of replay [E], and past
  the recovery bound, so the restore refuses.
- At ten times scale it fails outright.

Acceptable as the interim the other lane is building, not as the mechanism.

### Summary

| Option | Bytes per base, paper | Capture and WAL pin, paper | Restore, paper | Ten times | Complexity | Deletes | Verdict |
| --- | --- | --- | --- | --- | --- | --- | --- |
| today (fixed 4 MiB, whole) | 4.0 to 4.8 GB [M] | 26 to 38 min, all pinned [M] | fetch + witness 22 to 35 min [M] + tail at 77/s [M] | witness about 4 h, over budget [E] | none | none | replace |
| 1 CDC whole | [?], low on vacuumed copies [I] | 37 min [M] | as today | fails [E] | medium | none | reject |
| 2a page delta, hash compare | p·S/c: 0.3 to 5 GB [E] | about 3 min pinned [E] | full + deltas + page-root check about 3 min [E] + at most 1 h tail, 131 s [E] | 27 min pin per delta [E] | medium | floor, live witness pass, chain rule | **choose** |
| 2b page delta, WAL bitmap | as 2a | dirty pages only [E] | as 2a | needed below 1 h cadence [E] | high (checkpoint takeover) | as 2a | later, gated |
| 3 compact copy as full | 6.6 to 9.9 GB [E] | 17 min pinned [M] + 56 min unpinned [E] | as today, minus the witness | 2.8 h pin [E] | low (reuses the copy) | separate full capture | **choose, as the full of 2** |
| 4 faster replay | 0 | 0 | about 1.35x with fetch overlap [E] | constant factor against a 10x tail | low to medium | none | small side landing |
| 5 logical rows | 4.7 to 7 GB full [E] | 22 to 35 min [M] | about 18 min import [E] | hours [E] | high for deltas | page format | defer, gated on landing 1 |
| 6 do less | 0 | 0 | 6 h, past the bound [E] | fails | none | none | interim only |

## 4. Why not "make replay fast"

The paper replays one hour of its own events in about 131 seconds. A base
written every hour at a capture cost of about 3 minutes keeps a restore's
tail at one hour or less. If replay were 25 times faster (2,000/s), the
restore would still have to install a full image of the store: 39.6 GB, or
396 GB at ten times. Without a base, it would replay 11.8 M events from
genesis: 1.6 hours at 2,000/s, and 43 hours at the measured 77 [E]. Fast
replay shortens the tail but cannot replace the base, while cheap bases make
replay speed nearly irrelevant. That asymmetry is why the base is where the
design effort goes.

## 5. Recommendation: ordered landings

Every landing ships on, with no setting. The paper takes each one through an
ordinary deploy. Proofs run on d@e. Paper observations are read-only.

### Landing 0 (yellow, offline): a first reading of p before any production change

Extend the synthetic churn bench used for the compaction design
(`TestCompactMeasure`, `compact_measure_test.go`; 3.38 GB at the paper's
hourly write volume). Hash every page before and after 1, 6 and 24 hours of
writes, and report:

- p at 1, 32, 128 and 1,024 pages;
- the split between index trees and table trees, using `dbstat` page owners;
- zstd's ratio on the packed changed pages.

Run it on the build VM. There is no production surface, and nothing is
deleted. Rollback: none needed.

**Prediction (for the claude ledger, settle by 2026-11-18):** on the
compacted synthetic file, p_1(24 h) ≤ 0.25 and p_1024(24 h) ≥ 0.45.
Probability 0.55. The second half is anchored on the measured 49%
range-change at 24 hours.

### Landing 1 (orange; rollback: revert): the page-hash meter on every computer

**What it does.** On the producer's tick, when no capture is due, and at most
every 6 hours:

- stream `sqlite_dbpage` in one read transaction;
- write the 8-byte page hashes to a sidecar beside the store, keyed by inode,
  page count and page size;
- compare with the previous sidecar of the same inode;
- log p at 1, 32, 128 and 1,024 pages, the read time, and the WAL bytes
  pinned;
- report the same in the `snapshot_producer` block.

It reads nothing else and changes no restore path.

**Proof on d@e.** Two readings, six hours apart, show p_1 ≤ p_1024 and a
page count equal to the `store_tables` header's.

**Rollback.** Revert. The sidecar is harmless and is removed by the next
landing or by hand.

**Deletes nothing.** This is the measurement.

**Prediction for the paper.** Settle by 48 hours after the deploy, from its
log:

- the read takes at most 5 minutes and pins at most 1 GB of WAL;
- the first 24-hour reading shows p_1 ≤ 0.30 and p_1024 ≥ 0.80.

Probability 0.5. The p_1024 half is anchored on C59's 88% at 30 minutes on a
grown file. The paper's file was compacted at 03:43Z, so this half could
fail low.

**Gate.** If p_1(24 h) > 0.5, stop. Page deltas then save little, and the
lever moves to the store's own size and layout (T3, the append-clustered
note) or to option 5. If index trees hold more than two thirds of the
changed pages, write option 5 up properly before building landing 2.

### Landing 2 (red, two seats): page-delta bases

**What it does.**

- **The record.** A new snapshot record version names:
  - its parent root;
  - the page count;
  - the sorted list of changed pages;
  - a page root, the Merkle root over all 8-byte page hashes.

  Packs are dense 4 MiB runs of (page number, page), sealed by the existing
  `filecas.SealChunk`.
- **The root's chunk list is cumulative.** It holds the full base's chunks
  plus every pack of the chain so far. The host's collector and its
  live-root rule then need no change.
- **The producer** takes a delta when:
  - the sidecar's inode matches the live file;
  - its page root equals the parent record's;
  - the chain's packs sum to under half of the full base.

  Otherwise it takes a full base: the existing page stream plus a page root.
- **The trigger is the replay bound alone.** The floor, the capture record
  file, the WAL headroom rule and `floor_limited` are deleted. The ceiling
  and the interval stay only as backstops.
- **Restore:**
  1. install the full base;
  2. apply the packs in order;
  3. check the page root;
  4. then run the unchanged path: replay, then the checkpoint witness gate.
- **Fulls keep the logical witness. Deltas carry none.** A delta read
  happens inside one read transaction, so mixed state cannot occur. A stale
  sidecar is caught by the page-root equality. The logical end-to-end check
  is the restore gate's canonical checkpoint witness, which already
  recomputes over the result.

**Protected surfaces.** The snapshot record and chunks, restore base
selection, root retention.

**Conjecture delta.** Before: a base is a whole store, witnessed logically at
capture. After: a base is a whole store or a chain to one, verified by its
page root, and checked logically at the restore gate.

**Evidence class.** d@e restores and the paper's capture logs.

**Heresy delta.** A delta trusts the page hash of unchanged pages. The
residual is a hash collision on 8 bytes, about 2^-64 per page. Use 16 bytes
if a seat asks (154 MB sidecar).

**What is lost.** Nothing on the paper. Old v1 records stay readable, because
the reader for them is kept. An old binary after a rollback cannot read v2
deltas. Its producer would see the newest v1 root as old and take a full at
once: one full capture, about 37 minutes on the paper.

**Proof on d@e.**

1. Take a full base, then two deltas an hour apart.
2. Run `choir computer restore` to a checkpoint above the second delta.
3. It answers with `witness_matched: true`, and the log shows "applied 2
   delta packs, page root ok".
4. Corrupt one pack on a test host and see a refusal by name. This step is
   test-level, not a drill.

**Rollback.** Revert, as described under "what is lost".

**Prediction for the paper** (claude ledger; settle 72 hours after the
deploy, from the paper's log and the host's root rows, read-only):

- every delta captured in the window takes at most 6 minutes with at most
  6 minutes of read transaction;
- the deltas' sealed bytes sum to at most 2.5 GB a day;
- the producer block never reads `floor_limited`.

Probability 0.5, to be revised from landing 1's number before entry.

### Landing 3 (red, two seats): the compact copy is the full base

**What it does.** After `checkCompactCopy` passes:

- compute the logical witness on the staged copy (a private, static file:
  no live pin);
- stream its pages as a full base with its page root;
- register the root and append `snapshot_recorded` at the copy's head;
- write the copy's page hashes as the sidecar for the file the swap will
  create.

At the planned rebase the live file becomes that file, and the sidecar
follows it. The next delta then carries only the pages the rebase replay and
later writes changed. A full base from the live store remains only for a
computer that has never compacted, or whose chain grew past half a full.

**Deletes:**

- the live-store full capture as the routine way to start a chain;
- the second whole-store read every compaction cycle;
- the "copy discarded by a restart is pure waste" problem: the copy is a
  base on the host whether or not it is swapped in.

**Protected surfaces.** Snapshot records, the compact staging, the
planned-boot swap.

**Proof on d@e.** Force a compaction on d@e only (`store.compact_force` was
used this way on 11-01, and `compactReason` still reads it). Then:

- a full root is registered at the copy's sequence;
- after the planned refresh, the first delta is small;
- a restore to a checkpoint above that delta answers
  `witness_matched: true`.

**Rollback.** Revert. Copies are still made and swapped as today.

**Prediction for the paper.** At its next compaction (not forced), the first
delta after the rebase carries at most 25% of the full base's sealed bytes,
and the copy-to-registered-root time is at most 70 minutes after the copy
finishes. Probability 0.45.

### Landing 4 (red, two seats; built only behind its gate): WAL-frame dirty tracking

**Gate:** landing 2's read time on any computer exceeds 15 minutes, or a
computer needs deltas more often than hourly.

**What it does.** The store takes over checkpoints, and reads frame headers
since the last scan before each checkpoint into a persisted bitmap. Any
unclean boot, file swap or scan gap marks the bitmap unknown, which forces a
full read (landing 2's comparison). The comparison remains the fallback and
the verifier.

**Deletes:** the full read per delta. Not built at the paper's present
scale.

### Landing 5 (orange; rollback: revert): overlap replay fetch with apply

**What it does.** Fetch page k+1 while page k applies.

**Proof.** d@e's replay line shows fetch time hidden under apply.

**Prediction.** The paper's next cold replay after a planned rebase reads at
least 95 events/s (77 measured). Probability 0.6.

**Deletes nothing.** It is independent of the base work and may be dropped
if review time is short.

## 6. Measure first

The deciding number is p(T) for the paper, together with the split of
changed pages between index and table trees. In order of cost:

1. **Landing 0, on the build VM:** an hour of work and no production surface.
2. **Landing 1, a read-only meter on the paper:** about 3 minutes of read
   transaction every 6 hours.

No read-only experiment on the host can give p: the host holds only sealed
4 MiB chunks, and reading them would need the computer's key, which this
design does not touch. The second number, the restore install time for a
40 GB base on the paper, is unmeasured [?]. d@e's 3 minutes 44 seconds of
scan and install for a 1 GB store suggests the scan, not the bytes,
dominated there. Measure it in the "now" lane's first paper restore, if one
is ever run.

## 7. Residual risks

- **Page churn.** If p(24 h) is high, page deltas cost nearly a full base,
  and the lever is store size and layout, not snapshot format. Landing 1's
  gate exists for this.
- **The check at cold recovery.** A delta carries no logical witness. Cold
  boot recovery (lost disk, no checkpoint target) today installs a base and
  replays with no witness gate
  ([restore problem](../problems/restore-cannot-finish-inside-its-ten-minute-request-2026-11-14.md),
  "Cold recovery"). With deltas, that path rests on the page root alone.
  Running the replay-completeness probe after a cold recovery would close
  this; it is not in these landings.
- **Root size.** A cumulative chunk list grows with the chain: at most about
  1.5 × the full's chunk count, about 17,000 entries on the paper, inside
  `maxSnapshotRootChunks` = 65,536 [C, per the 11-13 design].
- **Pinned WAL.** The 3-minute pin per delta on the paper becomes about 27
  minutes at 396 GB, which is landing 4's gate.
- **VACUUM INTO's own pin.** It stays at 17 minutes on the paper and becomes
  hours at ten times scale. Compaction frequency is then the risk, not the
  base.
- **Unverified facts.** The brief's 11.5 GiB of host chunks for the 22:36Z
  root. Whether `og_objects` updates keep `obj_id`. Whether
  `modernc.org/sqlite` exposes sessions. Why the paper's `data.img`
  allocation fell from 88.5 to 42 GiB.
- **The largest lever is not in this design.** Storage mission T3 (payloads
  named rather than copied into `og_bodies` and `run_memory_entries`, about
  half the store) shrinks every base, delta and capture by the same factor.
