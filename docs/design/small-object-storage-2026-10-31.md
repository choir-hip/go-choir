# Small objects: one rule for every tiny thing Choir stores (2026-10-31)

Status: design, revision 1. Nothing is built or deployed by this note.
Mutation class of the work it proposes: **red** (tape/evidence storage,
store schema, snapshot layout, btrfs profile of the host). This note
integrates with, and does not replace, the
[object storage design](object-storage-2026-10-30.md) (packs, blobd, holds),
the [one-tape design](one-tape-durability-2026-10-31.md) (bundle, telemetry
partitions, compact snapshots) and the
[append receipt trim](append-receipt-trim-2026-10-31.md).

Labels: **[M]** measured on Node B 2026-10-31 by a metadata-only command
(receipts in section 1 and the appendix), **[C]** read from code at the cited
line, **[E]** estimated from [M] and [C] with the arithmetic shown.

Owner, 2026-10-31: "the paper is the real problem. Some of it may be
historical and unrelated to how we're doing things now, but the general
problem is we have all these little bits of tiny data and far too many tiny
files. We need to generally solve that problem so we're much more efficient
in the ratio of data we need to store to storage space it takes — and that
efficiency must not come at the cost of CPU and memory."

## 0. The answer in one paragraph

Choir pays for small data five times: (0) **append-and-fsync on btrfs**: each append copies the segment's partly filled tail block, so the tape segment store holds 28 GB in 47 GB [M]; (1) **one file per object** on btrfs,
where every file under 2 KiB is inlined into metadata that is DUP on top of an
md RAID1, so a 900-byte receipt occupies about 2.4 KB of filesystem and about
4.8 KB of NVMe; (2) **compression one record at a time** (btrfs per file,
tapestore per record), which gets 1.5-2x where the same bytes compressed as a
block get 5.4x; (3) **long random keys copied into many indexes** (the
136-byte `obj:<kind>:<owner b64>:sha256-<hex>` canonical id in 9-17 indexes
of `og_objects`, a random UUIDv4 for `run_memory_entries`), so the guest
store spends 1.65 KB of index per object and every insert dirties random
pages across the whole 21 GB file; (4) **random page churn defeats chunk
dedup**, so every snapshot (4 MiB page-range chunks) and every nightly
restic run re-sends most of the file. One rule fixes all five: **nothing
small is stored alone, and nothing is written where it was not already
going to be appended.** Immutable content-addressed objects go into
append-only packs in the tapestore segment format (one pack format for the
host, compressed per 64 KiB block); mutable rows go into SQLite tables
clustered by time with integer surrogate keys (the F-c `choir_events`
table and the telemetry partitions, both built, plus an id intern table);
tiny records are batched at the source (the telemetry sealer, built). Each
piece states its CPU and memory price below; none needs more than a few MiB
of index memory per million objects or a compaction peak larger than one
pack.

## 1. Anatomy of the paper's disk image (Job 1)

Paper: computer-1355b83bced1613eb048ee28d66688a8, VM
vm-1669020adcece51c2f5ff9b6df399ef5, `data.img` 128 GiB apparent, 23 GB
allocated on the host [M, `du` at 17:01 UTC]. Method: `cp --reflink=always`
into `/var/lib/go-choir/offhost-backup/staging-anatomy/` (5 s), then the
clone attached read-only (`losetup -r`) under a device-mapper snapshot with a
scratch COW file, so the ext4 journal replays into the COW and never into the
clone; mounted `ro`. A first `ro,noload` mount (no journal replay) showed a
torn `state.db` (header page count past the file end), so all SQLite numbers
below are from the replayed view. Everything ran under `nice -n 19 ionice
-c3`. The clone, the COW file and the loop devices were removed afterwards.

**Filesystem.** ext4, 4 KiB blocks, 8,435 inodes in use of 8.4 M [M]. Used
22.77 GB of 134.95 GB [M, `df`]. Host allocation (23 GB) equals guest use:
the hourly `choir-data-fstrim` timer (`nix/autoputer-vm.nix:863`) with
virtio-blk discard (`nix/node-b.nix:1549`) returns freed blocks, so **free
space inside the guest costs nothing on the host** [M]. Lifetime writes
recorded by the superblock: **1,582 GB since 2026-10-13** (about 88 GB a
day, most of it the Dolt era and the cutover migration and compactions) [M].

**The image is not a tiny-file problem.** 7,537 regular files, 22.75 GB;
2,894 under 4 KB, 4,334 of 4-64 KB, 5 over 64 MB [M]. Three files are 97.6%
of the bytes:

| Path (by role) | Bytes | Share |
|---|---:|---:|
| `state.db` (main guest store, SQLite) | 20,961,644,544 | 92.1% |
| `state-actor.db` (actor runtime, SQLite) | 727,465,984 | 3.2% |
| `state.db-wal` | 583,857,592 | 2.6% |
| `capsule-artifacts/` (one capsule subject workspace from 10-22, 4,846 files; a git pack is 97 MB of it) | 188,466,372 | 0.8% |
| `choir-updater/releases/` (one staged release; the runtime binary is 157 MB) | 161,141,625 | 0.7% |
| `files/` (the synced Files tree: a `Source/platform` checkout, mail; 2,610 files) | 118,685,318 | 0.5% |
| `state-actor.db-wal`, prompts, signers, credentials, markers | ~12 M | 0.05% |

No Dolt directory, legacy projection base or migration copy is left in the
image; the residue is two marker files and `state.db.migration.json`
(18 KB) [M].

**state.db internals** [M, `PRAGMA` and `dbstat` on the replayed clone]:
page_size 4096, page_count 5,117,589, freelist_count 0 (it was compacted
today), journal_mode wal, auto_vacuum 2 (incremental).

| Table family | Pages (GB) | Cell payload (GB) | Unused in pages (GB) | Rows |
|---|---:|---:|---:|---:|
| `og_objects` table | 10.54 | 8.53 | 1.95 | 3,336,242 |
| `og_objects` 17 indexes | 5.51 | 4.43 | 0.60 | (same rows) |
| `run_memory_entries` + 3 indexes | 4.46 | 3.96 | 0.45 | ~1.82 M |
| `og_key_index` + 2 indexes | 0.55 | 0.48 | 0.06 | 611,398 |
| `og_edges` + 3 indexes | 0.12 | 0.10 | 0.01 | 140,521 |
| `computer_event_index` + 4 indexes (windowed) | 0.09 | 0.07 | 0.02 | 68,383 |
| everything else | 0.04 | | | |
| **total** | **20.94** | | | |

`events`, `choir_events`, `texture_revisions` and the other runtime tables
are empty (one page each): the event index trim works, and nothing on the
paper writes the F-c `choir_events` table yet.

What the rows are, by object kind [M, `count`/`length` aggregates; no row
contents read]: `choir.event` 1,750,527 rows (2.39 GB body, 0.65 GB metadata),
of which loop.progress 593 k, desk.cell 378 k, tool.result 210 k (0.81 GB
body), desk.egress 168 k; then lifecycle_command 0.76 GB, texture_revision
0.53 GB, run 0.47 GB, source_ref 0.44 GB, commitment_record 0.36 GB and 30
smaller kinds. Logical bytes held (body + metadata + id) are about 7.9 GB for
`og_objects` and 3.0 GB for `run_memory_entries` (message_json 2.54 GB in
1,138,814 message rows), so **the store holds about 11 GB of uncompressed
JSON in 21 GB: 1.9x before compression, and roughly 7-10x against the same
JSON zstd-compressed in blocks** [E: 5.4x block ratio from the tape window
sizing evidence §4, on the same paper JSON].

Where the 1.9x comes from:

- **The canonical id.** `obj:` + kind + `:` + base64 of the owner id + `:` +
  `sha256-<64 hex>` or `key-<hex>`, 131-154 bytes, averaging 136-145 per kind
  (`objectgraph/object.go:76`) [M, C]. It is the table's TEXT primary key
  (a separate `sqlite_autoindex`, 0.60 GB) and the last column of 13 of the
  17 indexes. About 70 of its bytes are the same prefix on every row of a
  kind.
- **Index count.** 17 indexes on `og_objects` (`objectgraph/sql_store.go:165`
  and following): 1.65 KB of index per row against 2.2 KB of logical row [M].
- **Page slack.** 2.55 GB unused inside `og_objects` pages right after a
  compaction, from overflow chains (600 k of the 2.57 M table pages are not
  leaves: bodies over ~4 KB overflow) [M].

**Growth** [M, `created_at` histogram]: `og_objects` gains 170-280 k rows a
day (430 MB a day of body on a typical day, 860 MB on 10-22), and
`run_memory_entries` about 125 k message rows (about 280 MB) a day. Of the
choir.event bytes created in the last 24 h (about 290 MB), tool.result is
137 MB and loop.progress 60 MB; these retire after 7 and 14 days
(`store/telemetry_retire.go:88-94`), so they are the window, not the growth.
The semantic kinds (lifecycle commands, revisions, runs, source refs,
commitment records) are about 140 MB a day of body that nothing retires
[E: 430 total minus 290 telemetry].

### 1.1 Churn: what is rewritten and why

Measured: see section 1.2 (extent-map comparison an hour apart).

From the code, every write path into this image is a random-page writer:

1. **Inserts with random keys.** Each `og_objects` insert puts its
   canonical id (random sha256 or keyed hex tail) into the PK autoindex and
   into `idx_og_objects_kind_id`, `_owner_computer_kind` and
   `_kind_owner_computer`, each 0.6-0.9 GB: four leaf pages at uniformly
   random places per insert. The per-run, per-trajectory and per-channel
   indexes add pages at the tail of each open run's key range (as many hot
   spots as live runs). Only `idx_og_objects_updated` appends.
   `run_memory_entries.entry_id` is a UUIDv4 [M: version nibble 4], so its
   autoindex is random too. The suspected `"projection-batch:"+random id`
   key is **not** random: event ids are UUIDv7
   (`computerevent/event.go:116`), time-ordered; and that index is windowed
   to about 68 k rows. It contributes little.
2. **Updates move rows.** Run, agent, conversation-head, inbox-cursor and
   wake-outbox objects are rewritten in place as runs advance, and each
   update moves the row's `updated_at` entries in four `*_recent` indexes.
3. **The WAL.** `synchronous(FULL)` on every commit (`dbx/dbx.go:77`), no
   `journal_size_limit`, the default 1,000-page autocheckpoint. A long read
   transaction (a snapshot capture reads all 21 GB in one) stops
   checkpoints, so the WAL grew to 584 MB and stays that size; it is then
   rewritten from its start, cyclically, and each checkpoint copies its
   frames to their random home pages in the main file.
4. **Retirement and incremental vacuum.** The daily retire deletes about
   150-300 k telemetry rows spread over two weeks of pages, and
   `reclaimFreePages` (`store/compact.go:62-66`, 250,000 pages a run)
   moves pages from the file's end into the holes, rewriting each moved
   page and its pointer-map page.
5. **ext4's journal** (ordered mode, metadata only) and the hourly fstrim
   are small next to these.

Consequences, not just bytes: the guest store's 4 MiB snapshot chunks
(`snapshot/snapshot.go:66`) each cover 1,024 pages, and a chunk is re-sent if
any one of its pages changed. With four random index pages per insert and
about 250 k inserts a day, more than a million page writes land uniformly
over 5.1 M pages a day, so essentially **every chunk changes between two
snapshots** [E]. Snapshots come every 540,000 tape events, about 1.6 days on
the paper (ledger 2026-10-31 14:40), and superseded chunks wait out the
168 h artifact GC grace (`nix/node-b.nix:1082`). The paper's
`file-cas-chunks` directory shows it: 18,736 chunks, 54.3 GB, of which the
one retained snapshot root names 7,465 (31.3 GB, a pre-compaction file) and
the live Files tree about 5,200 small ones (119 MB); about 6,000 4 MiB
chunks (25 GB) written on 10-30 are a superseded snapshot waiting for GC
[M: `computer_snapshot_roots`, the on-disk root manifests, `find`]. In steady
state that is roughly (7 days / 1.6 days + 2 retained) x 21 GB, **about
130 GB of snapshot chunks on the host for one 21 GB store** [E], unless chunks
are reused. The nightly restic run sees the same image churn (the 10-31 run
added 108.7 GB [M, `status.json`]; the ledger names the paper's snapshots as
most of it).

### 1.2 Churn measurement

(Filled in from the extent-map comparison when it lands: rewritten 4 KiB
blocks and distinct 1 MiB regions per hour.)

### 1.3 Historical residue vs ongoing writes

| Item | Bytes | Historical or ongoing | Fate |
|---|---:|---|---|
| Image: Dolt stores, projection bases, migration copies | 0 (gone) | historical | done |
| Image: lifetime writes 1,582 GB | n/a | mostly historical (Dolt rewrote ~5 GB/h, `nix/autoputer-vm.nix:863`) | done |
| Image: `capsule-artifacts/` subject workspace of 10-22 | 0.19 GB | historical | capsule teardown residue; delete with its receipt |
| Host: paper's superseded snapshot chunks of 10-30 | ~25 GB | historical (one-off), GC-eligible ~11-06 | GC grace |
| Host: retained pre-compaction snapshot (31 GB file) | 31.3 GB | historical; replaced by a ~21 GB snapshot at the next tape distance | snapshot retention |
| Host: legacy one-file tape pins (`sha256/pin-receipts`, `computer-event`, `computer-event-payload`) | 24.2 GB apparent, ~10.8 M files | **both.** Events stopped (2,849 new event files in 24 h), but **388,399 pin receipts and 385,623 payload files were written in the last 24 h** (14,152 and 13,456 in the last hour), 0.35 GB and 2.30 GB apparent [M, `find -newermt`; an mtime count, so re-pins are included]: sealed (private) projection-batch payloads and their receipts still go loose until 529ae0d0c deploys | 529ae0d0c (in flight) stops most of it; taperepack census then delete; packs for all kinds (rule A) |
| Host: PostgreSQL `computer_event_append_receipts` | 18.7 GB, 6.17 M rows | **ongoing**, ~290-350 k rows/day | compact codec (in flight), then B3 |
| Guest: semantic og kinds + run memory | ~420 MB/day logical | **ongoing user/machine state** | B2 layout; compression |
| Guest: telemetry window | ~290 MB/day in, retired at 7/14 d | ongoing, bounded | partitions + F-c (built) |
| Guest: snapshot chunk re-send | ~13 GB/day of new chunks [E] | **ongoing, the largest** | B2 (append-clustered layout) |

## 2. Inventory of small objects and their storage-to-data ratio

Host filesystem: btrfs on md RAID1 (`/proc/mdstat`: md127 raid1 of two NVMe
partitions), `compress=zstd:3`, `discard=async`, `space_cache=v2` [M].
`btrfs filesystem usage -b /`: Data single 278.7 GB used; **Metadata DUP
18.8 GB used, 37.6 GB on the device** (54.0 GB allocated); so every metadata
byte is four physical copies on the NVMe pair [M]. Files of 2,048 bytes or
less are stored inline in metadata (btrfs `max_inline` default).

| Store | Objects | Logical bytes | Stored | Ratio | Notes |
|---|---:|---:|---:|---:|---|
| `sha256/pin-receipts` | 5,421,404 files, all inline, mean 896 B | 4.86 GB | compsize: 2.9 GB zstd (65%) inline [M]; plus ~0.6 KB of inode, ref, dir item and dir index per file [E], all in DUP metadata: **~12 GB fs, ~24 GB NVMe** [E] | 2.5x fs | st_blocks reports 22.2 GB [M] |
| `sha256/computer-event` | 2,468,665 files, all inline, mean 1,513 B | 3.73 GB | compsize: 1.5 GB zstd (45%) inline [M] + items, DUP: **~6 GB fs** [E] | 1.6x | |
| `sha256/computer-event-payload` | 2,953,043 files; 2.07 M inline, 0.90 M extents | 15.61 GB | compsize: 8.3 GB (51%; 1.1 GB incompressible) [M] + 1.8 GB items, inline part DUP: **~11 GB fs** [E] | 0.7x | the older plaintext payloads compress; sealed private ones (2.3 GB/day now) do not |
| `capture-bodies` | 168,711 files; 104,534 inline | 9.59 GB | compsize: 3.1 GB (34%) [M] | 0.35x | text compresses 4x even per file; per block more |
| `sha256/og` | 23,119 files | 0.51 GB | compsize: 134 MB (26%) [M] | 0.26x | |
| `sha256/file-cas-chunks` | 512,176 files; 79,644 inline; 17,993 full 4 MiB chunks | 86.5 GB | 81 GB, 100% uncompressed (encrypted) [M compsize] | 1.0x | the big files are snapshot chunks; small ones are Files-tree files, one chunk each |
| `sha256/file-cas-roots` | 1,957 manifests, 0.46-0.64 MB each | 1.02 GB | | | full manifest per sync, every 15 min when anything changed (`autoputer/file_sync.go:27`); the paper writes ~35 MB/day of them |
| `tape-seg` (all computers) | 1,676 files (segments + idx); paper 1,093 | 28 GB referenced | **compsize: 47 GB on disk for 28 GB referenced, 99% uncompressed, 5,156,807 extents** [M] | **1.68x** | see below: one wasted 4 KiB block per fsynced append; files carry the btrfs no-compress flag (`lsattr` m); 5.4x available by block compression [M, tape window sizing]; segments seal at every boot |
| PostgreSQL `computer_event_append_receipts` | 6,173,492 rows | ~1.5 KB receipt JSON + 9 columns | heap 13.64 GB, indexes 5.03 GB (5 indexes, 815 B/row), total 18.67 GB = **3.0 KB per row** [M] | ~2x | rows under the TOAST threshold are not compressed |
| PostgreSQL graph `og_objects`, `proj_capture_terms` | 0.67 M, 0.83 M rows | | 1.0 GB, 0.67 GB (pkey 0.41 GB for 0.24 GB heap) | | |
| Guest `og_objects` (paper) | 3.34 M rows, 2.2 KB logical | 7.9 GB | 16.05 GB pages | 2.0x (7-10x vs block zstd) | section 1 |
| Guest `run_memory_entries` (paper) | 1.82 M rows | 3.0 GB | 4.46 GB | 1.5x | |
| Guest `computer_event_index` | 68 k rows (windowed) | | 0.09 GB | | already fixed |

**The segment store wastes 40% of its disk** [M]. A sealed 64 MiB paper
segment occupies 105-108 MB (seg 27: 10,624 extents; seg 463: 11,413); the
open segment 547 holds 19 MB in 39 MB. Cause [C + M]: the appender writes
each group of records and fdatasyncs (`tapestore/appender.go:349`); the next
append rewrites the partly filled last 4 KiB block, btrfs copies it on
write, and the old extent stays whole while any part is referenced
(extent bookending). 5.16 M extents x one 4 KiB block is about 21 GB, the
measured 19 GB gap. The first write of a segment is incompressible (a sealed
payload), so btrfs marks the inode no-compress and never compresses it.
The paper's `data.img` shows the same effect smaller: 27 GB on disk for
23 GB referenced [M, compsize], from guest 4 KiB writes copied on write.

The two numbers that matter: **about 10.8 M loose files hold 24 GB of tape
data in about 30-35 GB of filesystem (60-70 GB of NVMe)**, and **the guest
store holds 11 GB of JSON in 21 GB that is rewritten everywhere every day**.

## 3. Failure modes first

A design that saves bytes is wrong if any of these happen. Each build in
section 5 names which it must test.

- **F1 Evidence loss.** A record dropped or replaced by repack, compaction
  or a key rewrite: a digest the tape, a receipt or a run acceptance names
  no longer resolves. Check: every pack rewrite reads back every live digest
  from the new pack and verifies its SHA-256 before the old pack is
  unlinked; a census lists every digest named by the tape and receipts and
  resolves each one (taperepack's keep-digests rule).
- **F2 Witness/replay divergence.** A layout change (surrogate keys, new
  table, partition) changes what the content witness hashes, so replay of
  the same tape gives a different witness. Check: the witness covers rows,
  not layout (`one-tape §6b`); surrogate integers must be a pure function of
  the tape (assigned in apply order), or excluded from the witness.
- **F3 Restore failure.** A snapshot taken of the new layout cannot be read
  by the binary a rollback lands on. Check: schema floor epoch raised before
  the first write, as the trim and partitions do (`storeschema`).
- **F4 Crash loss / torn pack.** A pack tail torn by power loss loses
  acknowledged records. Check: group commit fsyncs the pack before the ack;
  recovery truncates to the last record whose CRC and digest verify
  (tapestore `sealUnowned`); power-loss harness (object-storage §5).
- **F5 Read amplification.** Block compression makes a 900-byte read
  decompress a 64 KiB block; a newest-first segment probe makes misses scan
  every segment. Check: p99 read latency and CPU per read on the paper's
  replay, against today's.
- **F6 Memory spike.** An index held in RAM per million objects, a repack
  that buffers a pack, a zstd dictionary trainer, or a VACUUM that holds the
  whole file. Check: each job runs in a cgroup with a stated MemoryMax and
  the growth meter records its peak.
- **F7 CPU spike.** Compression at a level that costs more than the IO it
  saves; a repack that runs at full speed during the paper's day. Check:
  `nice`/`ionice` idle, paced loops (the receipt backfill's 500-row,
  rest >= 4x pattern), per-job CPU seconds in the growth meter.
- **F8 Dedup illusion.** A change that shrinks a file but still rewrites
  every 4 MiB range, so snapshots and backups do not shrink. Check: the
  snapshot's `ChunksHeld / chunks` ratio (`snapshot/snapshot.go:697`) logged
  per snapshot and recorded by the growth meter; the target is >= 80% held.
- **F9 Two formats.** A new pack format beside tapestore and beside
  object-storage's packs: a second reader, a second recovery, a second
  backup rule. Check: one format, versioned; this note adds namespaces and a
  block record type to tapestore, nothing else.

## 4. The design: four rules

### Rule A. Immutable small objects go into packs, never into files

Every content-addressed object under 1 MiB (tape events, payloads, pin
receipts, capture bodies, og host blobs, small file-cas chunks, head
receipts) is a record in an append-only pack. **The pack format is the
tapestore segment format** (`internal/tapestore/format.go`): it already has
a header, per-record CRC, SHA-256 verification on every read, a fanout index
with 45-byte entries, 64 MiB / 50,000-record seals, crash recovery and
zstd records; object-storage §5 already adopts its seal and write
discipline. Changes:

1. **Namespaces.** `NS` grows past 1-3 (`format.go:33-38`): capture body,
   og blob, file chunk, head receipt. `valid()` and `LegacyRel` follow.
   A pack belongs to one scope (computer, owner, platform class), as
   object-storage §5 requires.
2. **Block records.** A new record version (`recVersion` 3): a block of up
   to 64 KiB of consecutive small records compressed as one zstd frame, the
   index entry pointing at (block offset, offset inside the block). This is
   where the 5.4x lives; record-at-a-time v2 measured 1.54x synthetic
   (ledger 2026-10-31 14:20). Private payloads are sealed (encrypted) and
   stay raw; their compression is the guest's, before sealing (in flight,
   `compressPrivateWrites`).
3. **Seal rule.** Seal at 64 MiB / 50,000 records and at nothing else; a
   restart reopens the open segment instead of sealing it (today the paper
   has 546 segments for 28 GB because each boot seals; [M] segment sizes
   5.9-17.7 MB at boot times). Fewer, fuller packs keep the per-segment probe
   short (F5).
4. **Deletion by time.** Records are packed in arrival order per scope, so
   retention (the paper's 7-day hot window, test-computer deletion) is
   dropping or archiving whole sealed packs, with no rewrite. Only
   content-addressed classes with per-object deletion (captures, og blobs,
   file chunks) need repack: object-storage §5's rule (rewrite a sealed
   pack whose dead fraction exceeds 40%; write amplification <= 1.7x of what
   is deleted).

Cost [E]. CPU: zstd level 3 compresses at 300-500 MB/s per core; the paper
writes about 2.7 GB a day of segment records (tape window sizing), 31 KB/s,
under 0.01% of a core. Reads decompress one 64 KiB block, 50-100 us. Memory:
the reader keeps only the 1 KiB fanout of each sealed index
(`reader.go:362`), so 1 M objects in 20 packs cost 20 KiB resident; the open
segment's entry map costs about 100 B per record up to 50,000 records (5 MB
per open scope); one zstd encoder per writer (level 3, 64 KiB window: under
1 MB); no dictionary needed at block level. Repack: streams one pack (64 MiB
read, 64 MiB written), holds one index (50 k x 45 B = 2.3 MB), paced and
idle-IO; peak memory under 10 MB. Bytes [E]: the 10.8 M loose files (30-35 GB
of fs, 60-70 GB of NVMe) become about 24 GB / 5 = 5 GB of packs; captures
and og blobs likewise.

### Rule B. Mutable rows are appended where time already puts them

SQLite B-trees are cheap when every insert lands on the right edge and
expensive when it lands at a random leaf. Make the keys time-ordered and
short:

1. **Telemetry in the F-c table and day partitions** (built:
   `store/event_table.go`, `store/telemetry_partition.go`). `choir_events`
   is keyed `(tape_seq, op_index)`, three secondary indexes instead of 17,
   no canonical id in any index. Today the paper writes none of it (the
   table is one empty page) and partitions are on for d@e only. Turning both
   on for the paper moves 52% of `og_objects` rows (1.75 M choir.event) and
   their 17-index fan-out to append-only structures, and makes retirement a
   file delete. No new code; a ring widening.
2. **An id intern table for the object graph.** `og_ids(id INTEGER PRIMARY
   KEY, canonical_id TEXT UNIQUE)`; `og_objects` becomes `WITHOUT ROWID`
   keyed by the integer, and every secondary index ends in the 8-byte
   integer instead of the 136-byte text. The integer is assigned in apply
   order (so a replay assigns the same one, F2) and never leaves the
   guest; the witness hashes canonical ids, not integers. Inserts then
   append to the PK and to every index whose leading columns are
   (kind, ...) at the kind's tail; the text-to-integer lookup is one random
   leaf (the UNIQUE index), instead of four. Saves [E] about 128 B x 13
   index entries + 128 B in the table = ~1.8 KB of payload per row before
   slack, roughly 4-5 GB of the paper's 16 GB `og_objects` family. Memory:
   none extra (page cache as today); CPU: one extra index probe per write.
3. **Fewer indexes.** 17 indexes on one table is a design smell: the
   `_recent` twins (four, 0.69 GB) and the two owner/kind orderings (1.76 GB)
   should be measured against their readers and merged where one serves both
   (a reader census with `EXPLAIN QUERY PLAN` over the store's query set).
4. **run_memory_entries** keyed `(loop_id, seq)` (already UNIQUE) as a
   `WITHOUT ROWID` primary key, dropping the random `entry_id` autoindex;
   `entry_id` becomes a column found through `(loop_id, seq)`.
5. **WAL bound.** `journal_size_limit` (for example 64 MiB) and a
   checkpoint after each snapshot capture so the 584 MB WAL shrinks; the
   capture already reads in one transaction and cannot be shortened.
6. **Compression of large values** before they enter SQLite (tool results,
   message_json): built for chunks and private payloads, held on writes;
   the same zstd envelope for bodies over 1 KiB in `og_objects.body` and
   `run_memory_entries.message_json` would take the 2.5 + 2.4 GB of the two
   largest columns down by 3-5x [E]. Readers must expand transparently;
   indexes over JSON (the generated columns) read metadata, not body, so they
   are unaffected; `og_rec_claim` reads the commitment-record body and must
   be kept uncompressed for that kind.

Effect on churn (F8): with telemetry and the intern table, an hour's inserts
dirty the right edge of each structure plus a few hundred update-hot pages,
so 4 MiB chunks of last week's region stop changing, the snapshot holds most
chunks, and restic stops re-sending the image. This is the only proposal that
cuts the ~13 GB a day of snapshot chunks [E]; shrinking the file without it
does not.

### Rule C. Tiny records are never written alone

Batching at the source is the multiplier on A and B:

- the telemetry sealer (built, on for the paper since 14:48): up to 64 ops
  per tape event; the paper's receipt rate fell only 35% (18,400 to
  11,900 an hour) because several streams each seal about once a second.
  **Group commit across streams** (one sealer per computer, the in-flight
  `lane/tape-group-commit`) is the next factor: a window of 1-2 s across all
  streams gives one tape event, one payload record, two pin receipts and one
  receipt row per window.
- pin receipts: one receipt per pin call per artifact; a batch's event and
  payload pins can share one receipt record covering both digests.
- file sync: a manifest per change rewrites the whole 0.46 MB manifest every
  15 minutes; a delta manifest (changed entries plus the parent root) cuts
  the paper's ~35 MB a day of roots to kilobytes, and any small file in the
  Files tree goes into a pack (rule A) instead of its own chunk file.

Cost: memory is the queue (sealer capacity 256 records,
`store/telemetry_sealer.go`, about 256 KB); CPU none; the risk is crash loss
of the newest window, already bounded by the append barrier.

### Rule D. The host's receipt table stops being a per-event row store

`computer_event_append_receipts` costs 3.0 KB per row [M] and is the one
host table that grows with every tape event:

1. compact receipt body (in flight, d68fb3d87): saves ~68% of the JSON,
   about 1 KB a row;
2. digests as `bytea` (32 B) instead of 64-char hex `text` in the columns
   and the three digest-bearing indexes: halves those index entries [E:
   ~1.2 GB of the 5.0 GB of indexes];
3. phase 2 of the trim design: sealed ranges of receipts become records in
   the computer's tape segment (namespace head receipt, rule A, block
   compressed at 2-6.6x), and the table keeps the head, the open range and a
   sparse sequence index (one row per sealed segment). Rows per day go from
   ~300 k to ~10 [E]. The reader inventory and the conditions are already in
   the trim design; this note only adds that the segment is the place.

### Rule E. The host filesystem: stop paying four copies for metadata

Metadata DUP on md RAID1 stores every inode, directory entry and inline file
four times. Converting metadata to the single profile
(`btrfs balance start -mconvert=single /`) returns 18.8 GB now [M: 37.6 GB
on device for 18.8 GB used], and its value falls as rule A empties the
loose-file directories. Risk: btrfs loses its own second copy for repairing
a metadata checksum error that md cannot detect (md mirrors without
checksums and may serve the bad copy); the balance takes hours of IO. This
is a black-class change to the host's filesystem; rank it after A, and
decide it with the owner once the loose files are gone (when the saving is a
few GB, keep DUP).

### Rule F. Append-only files never rewrite their tail on btrfs

The segment store loses 19 GB of 47 GB to copy-on-write of partial tail
blocks (section 2). Three fixes, cheapest first:

1. **Now, for sealed segments: rewrite once.** `btrfs filesystem defragment`
   of each sealed segment file (or a copy to a fresh file, fsync, rename,
   as the seal already does for the index) writes it as a few large
   extents and frees the bookended blocks: about 19 GB back [E from M],
   bytes unchanged, verifiable by the segment's own SHA-256 and the index.
   Cost: one sequential read and write of 28 GB, idle IO, paced; memory
   negligible. Risk: low (content identical; skip the open segment, skip
   any file a backup run has reflinked, recheck its digest after).
2. **At seal, from now on:** the appender's seal step copies the segment to
   a fresh file before the rename (64 MiB sequential, about 0.1 s of IO per
   seal at the paper's ~40 seals a day). With rule A's block compression
   the copy is also where small plaintext records are compressed.
3. **Or NOCOW segment directories** (`chattr +C` on `tape-seg/<computer>`
   before the first segment): appends are written in place, no bookends,
   no btrfs checksums (tapestore checks a CRC and SHA-256 on every read, so
   integrity does not depend on them), no btrfs compression (rule A does
   its own). Simpler at run time, but it changes the integrity story of
   the host's copy of the tape: decide with the review of build 1.

The same mechanism costs the paper's `data.img` 4 GB (27 GB on disk for 23
GB); that is the VM image path's to decide (vmctl, red), noted not proposed.

## 5. Ranking: bytes saved per unit of risk

| # | Proposal | Bytes saved | Bytes/day avoided | Risk | CPU / memory | Status |
|---|---|---|---|---|---|---|
| 0 | Rewrite sealed tape segments once (F1); copy at seal (F2) | **~19 GB now** [E from M] | ~40% of new segment bytes | low | sequential IO, no memory | new, small |
| 1 | Telemetry partitions + F-c table on the paper (B1) | 3-5 GB of the store [E] | the telemetry share of churn; retirement becomes O(1) | low (built, on d@e, red class) | none extra | built, widen ring |
| 2 | Deploy 529ae0d0c (sealed payloads to segments), then delete legacy loose tape covered by segments (taperepack) | 24 GB apparent, ~30-35 GB fs, ~60 GB NVMe [E] | ~772 k files/day, 2.65 GB/day apparent [M], ~3.2 GB/day of fs [E: payloads sealed, incompressible; receipts ~2.4 KB each] | medium (F1), gated by census + keep-digests | idle IO, MBs | built; census running |
| 3 | Intern table + WITHOUT ROWID + index diet (B2-B4) | 5-7 GB of the store [E] | most of ~13 GB/day snapshot chunk churn [E]; restic re-send | medium-high (F2, F3; store schema) | one probe per write | new (build 2) |
| 4 | Tapestore as the one pack format: namespaces, block records, no boot seal (A) | 5x on all new tape bytes; captures and og blobs to packs | ~2.2 of 2.7 GB/day of segments [E from M] | medium (F4, F5, F9) | < 0.01 core, < 10 MB | new (build 1) |
| 5 | Receipts: compact codec, bytea, fold sealed ranges into segments (D) | 6 GB now (codec), ~15 GB at phase 2 [E] | ~0.9 GB/day [E: 300 k rows x 3 KB] | medium (reader inventory) | paced backfill | codec in flight; phase 2 new |
| 6 | Group commit across streams (C) | | receipts and pins per day x 1/streams [E: 2-4x] | low | 256 KB queue | in flight |
| 7 | Compress bodies over 1 KiB in og/run memory (B6) | 3-4 GB [E] | 60-70% of new body bytes | medium (every reader) | zstd level 1-3 on write, under 1% core | new |
| 8 | Delta file-root manifests; small Files in packs (C) | 1 GB of roots fleet-wide | ~35 MB/day on the paper | low | none | new |
| 9 | btrfs metadata single profile (E) | 18.8 GB now, less later | 0 | high (black: redundancy, hours of balance) | IO only | owner decision |
| 10 | WAL bound (B5) | 0.5 GB | WAL rewrite | low | none | new, small |

Measured vs estimated: the "bytes saved now" for 2 and 9 and the 3.0 KB
receipt row are [M]; everything per day and every post-change ratio is [E]
and becomes [M] only through the growth meter (section 7).

## 6. In flight: integrate, do not redesign

- `lane/tape-group-commit` (sealer per computer): rule C's main lever.
- `lane/private-payload-segments` (529ae0d0c): sealed payloads to the
  segment; it is rule A for one namespace.
- `lane/receipt-trim` (d68fb3d87): rule D step 1; phase 2 lands receipts in
  segments.
- `lane/growth-meter` (8f867aaeb, `scripts/node-b-growth-meter`): the proof
  instrument; section 7 adds fields.
- `lane/backup-memory` (086e1669a, 2f1b882bb): restic shards and `pg_dump -Z
  0`; rule A makes the shards unnecessary (a few hundred packs instead of
  10.8 M files).
- taperepack (`internal/taperepack`, `lane/fp10-tape-repack`): the legacy to
  segment migration and the keep-digests delete; it is also the repack tool
  rule A needs, so it should gain the block record type rather than a new
  tool.
- object-storage design (blobd, holds, packs): rule A is its pack format;
  holds and GC stay as that design states.

## 7. Measuring it (growth meter additions)

Hourly, metadata-only, into the growth meter's ledger:

- per namespace: file count and apparent bytes of each loose directory
  (from the directory's own counters, not a walk, once packs exist);
  `btrfs fi usage` data and metadata used;
- per computer: segment count, sealed bytes, open bytes;
- guest store (through the guest's observability endpoint, not SSH): page
  count, freelist count, WAL size, rows per family; the snapshot's
  `ChunksHeld` and uploaded bytes per snapshot (F8's ratio);
- the paper's `data.img` changed 1 MiB regions per hour (the extent-map
  comparison of section 1.2, run daily);
- receipts rows per computer per day.

"Flat day over day" is then: loose-file count non-increasing, pack bytes
growing at the compressed tape rate, guest store growing at the semantic
rate (about 140 MB a day of body before rule B6), snapshot held ratio >= 80%.

## 8. First three builds

1. **Tapestore block records and namespaces** (rule A).
   `internal/tapestore/format.go` (NS, record version 3, index entry for
   block offset), `compress.go` (block frame), `appender.go` (buffer up to
   64 KiB of small records per open segment, flush at group commit; reopen
   instead of seal at boot, `begin`/`sealUnowned` at 375-504), `reader.go`
   (block decode, newest-first probe unchanged), and
   `internal/platform/tape_segments_write.go` (all pin kinds, not only
   projection batches). Tests first: F1 (every digest re-read), F4 (torn
   block tail), F5 (read latency), F9 (old readers refuse v3 cleanly; readers
   ship before writers, as v2 did). Rollout: readers live, writes held, then
   d@e, then the paper.
2. **Append-clustered object graph** (rule B2-B4).
   `internal/objectgraph/sql_store.go:165` (DDL: `og_ids`, `WITHOUT ROWID`,
   index set), `internal/objectgraph/object.go:76` (ids unchanged on the
   wire), `internal/store/schema.go` (`run_memory_entries` key),
   `internal/storeschema` (floor epoch), the replay witness extractor
   (`agentcore/restore_snapshot.go`), and the compact snapshot path
   (`store/compact.go`) which is where the layout is rebuilt: the new layout
   arrives through a compact snapshot and planned rebase (one-tape §6b
   option 3), never an in-place migration. Tests first: F2 (replay witness
   equal across layouts), F3 (rollback refuses by floor), F8 (two snapshots
   an hour apart hold >= 80% of chunks on a d@e store under load).
   Before it: turn on `store.event_table` and `store.telemetry_partitions`
   for the paper (ranked 1), which needs no build.
3. **Receipts into segments** (rule D phase 2).
   `internal/platform/receipt_compaction.go` (the compact form becomes the
   segment record body), `internal/platform/computer_events.go`,
   `event_replay.go`, `checkpoints.go` (readers walk segment then table, as
   the reader inventory in the trim design lists), a sparse
   `computer_event_receipt_ranges` table. Tests first: every reader in the
   trim design's inventory against a computer whose receipts are half in
   segments; F1 census of every sequence 1..head.

## Appendix: receipts

Commands (all on Node B, all read-only on live data, metadata only):

- `cp --reflink=always .../data.img .../staging-anatomy/data.img`;
  `losetup -r`; `dmsetup create anat-snap` (snapshot target, COW in
  scratch); `mount -o ro /dev/mapper/anat-snap`; `df -B1`, `df -i`,
  `du -x -B1 -d1`, `find -printf %s` histograms; `dumpe2fs -h` (lifetime
  writes); `sqlite3 'file:...state.db?mode=ro'` with `PRAGMA page_size,
  page_count, freelist_count, journal_mode, auto_vacuum`, `SELECT name,
  sum(pgsize), count(*), sum(payload), sum(unused), sum(ncell) FROM dbstat
  GROUP BY name`, and `count`/`sum(length())` aggregates by `object_kind`,
  by `json_extract(metadata,'$.kind')` and by `created_at` day.
- `btrfs filesystem usage -b /`; `/proc/mdstat`; `findmnt -T /var/lib/go-choir`.
- `find <ns> -xdev -type f -printf '%s %b'` per namespace; `compsize` on
  20,000-file samples and on whole namespaces; `find -newermt`.
- `psql -d control`: `pg_relation_size`/`pg_total_relation_size` for the
  public relations; `computer_snapshot_roots` chunk counts; `jq` over the
  paper's root manifests for chunk references and total sizes (no names
  read out).
