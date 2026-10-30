# New-engine storage measurement (2026-10-30)

Status: measurement only, mutation class green (docs). No production data or
service was changed. Settles round 0f (R0f-02 to R0f-07) of
[the data-architecture precommit](../definitions/choir-data-architecture-selection-2026-10-28.precommit.md)
by the settler the records name: load the census-frozen keep set into each
engine on a scratch host directory, measure `pg_total_relation_size` and the
SQLite file after `VACUUM`. No record was edited. Inputs:
[keep-set accounting](keep-set-accounting-2026-10-29.md) and
[the storage inventory](node-b-storage-inventory-2026-10-29.md).

Labels: **[V]** verified by a command in this run (Node B, 2026-10-30
00:03 to 01:57 UTC); **[I]** inferred or extrapolated, basis stated.
GB means 10^9 bytes, MiB means 2^20 bytes; where a record says "GB" the
GiB figure is given beside it when it changes the score (it does not).

## Headline

| Record | Prediction (80% interval) | Measured | Verdict |
|---|---|---|---|
| R0f-02 PG control | 9 GB [3, 20] | **19.1 GB** (17.8 GiB), extrapolated from a 73% load [I] | inside, 0.9 GB under the 20 GB falsifier |
| R0f-03 PG corpus | 6 GB [2, 15] | **2.17 GB** (full load) [V] | inside, at the lower edge (1.57 GB if the three derived index tables are excluded, which would be outside) |
| R0f-04 paper SQLite | 3 GB [1, 10] | **27.9 GB** (4 KiB pages) / **20.8 GB** (16 KiB pages), extrapolated from a 1/16 sample [I] | **outside, 2 to 3 times the upper bound** |
| R0f-04 d@e computer | 150 MiB [30, 800] | **64 MiB** (67.0 MB) small computer [V]; 1.9 GiB for the always-on owner computer [V] | inside (small); the always-on owner computer is outside |
| R0f-05 object store | 12 GB [9.5, 30] | **9.98 GB** [V] | inside, 0.5 GB above the lower bound |
| R0f-06 control growth | 0.5 GB/day [0.15, 1.5] | **0.61 GB/day** (30-day mean), 1.07 GB/day last 7 days [I] | inside |
| R0f-06 corpus growth | 0.15 GB/day [0.05, 0.6] | **0.13 GB/day** [I] | inside |
| R0f-06 blob growth | 0.65 GB/day [0.4, 1.2] | **0.58 GB/day** (17-day mean), 0.52 last 7 days [V] | inside |
| R0f-07 total | 35 GB [15, 80] | **59.2 GB** (52.0 with 16 KiB pages) [I] | inside |

Three things the predictions did not see, in order of weight: (1) the
guest's `computer_event_index` carries a second ~1.5 KB JSON copy of every
event (`event_head_receipt_json`), so removing `event_json` alone removes
almost nothing from the SQLite file; (2) SQLite at its default 4 KiB page
holds one 2.4 to 3.8 KB event row per page, wasting up to 40% of the file;
(3) the control receipts are 3.1 KB each in PostgreSQL, not the 1 to 1.5 KB
assumed, so control is within 5% of the falsifier on today's data and will
cross it within a day at the current 1.07 GB/day.

## Method

### Source data without stressing Dolt

First attempt, live reads of Store A over SQL, was abandoned (see
Surprises: Store A sits at its own `MemoryHigh`, a 5,000-row range read took
38 s and the next timed out at 100 s). Every row used below was read instead
from **btrfs reflink copies** (`cp --reflink=always`, 0.4 s for the 55 GB
Store A directory, 3 s for a 34 GB guest image) of:

- the Store A data directory (`platform`, 55 GB), snapshot at 00:11 UTC,
  receipts count 6,065,689 [V];
- the Store B data directory (`corpus`, 15 GB), same minute;
- the Autopaper's guest `data.img` (never the live file; the copy was mounted
  read-write, journal replayed, on the copy only);
- the guest images of the owner's `computer-03335285` (always-on) and
  `computer-59c902c5` (small).

Rows were read with offline `dolt sql -r csv` processes (own memory, niced,
`ionice -c3`, `MemoryMax` scope) in keyset-paginated batches and loaded
through `COPY` (PostgreSQL) or `executemany` (SQLite). The few live
queries against the running servers are listed under Surprises. The password
file was read into an environment variable on the host only; it appears in no
output or file.

### Engines

PostgreSQL 17.11 from nix, `initdb --locale=C --encoding=UTF8`, one instance
with two databases (`control`, `corpus`), unix socket only, run as a transient
unit with `MemoryMax=4G`, `Nice=10`, idle IO class, `shared_buffers=256MB`,
`fsync=off`, `autovacuum=off`, `wal_level=minimal`. Schemas translated from
`SHOW CREATE TABLE` (the live DDL, which is what `internal/platform/store.go`
and `internal/store` produce): `varchar(n)`/`char(n)` to `varchar(n)`,
`bigint` to `bigint`, `tinyint(1)` to `boolean`, `datetime`/`datetime(6)` to
`timestamp(0)`/`timestamp(6)`, `json` to **`jsonb`**, `text`/`longtext` to
`text`, `longblob` to `bytea`. Primary keys and **every secondary and unique
index Dolt carries** were created (the code's indexes; none dropped). Indexes
were built after `COPY` (compact b-trees); then `VACUUM (ANALYZE)`.
Measurement: `pg_total_relation_size`, `pg_table_size`, `pg_indexes_size` per
table and `pg_database_size`.

SQLite 3.53.3 from the host: tables as `INTEGER`/`TEXT`/`BLOB` affinity
rowid tables, the primary key as a unique index (a mechanical port; no
`WITHOUT ROWID`), all Dolt secondary indexes (including the nine
generated-column indexes of `og_objects`, stored as plain columns), indexes
built after load, `VACUUM`, file size read with `stat`. Page size 4096 (the
SQLite default; the guest sets none) and 16384 (repacked with
`PRAGMA page_size=16384; VACUUM`) are both reported.

### What "event_json leaves the index and content-hash kinds move" means here

R0f-04 names two removals. They were applied to the loaded file as steps:

- **A**: `computer_event_index.event_json` emptied (R0f-04, first half);
- **B**: A, plus, for the VM-store `IdentityContentHash` kinds
  (`choir.event`, `channel_message`, `inbox_delivery`, `run_memory_entry`,
  `texture_revision`, `content_item`, `source_entity`, `source_ref`;
  `internal/objectgraph/registry.go:89-101`), `body` set to NULL and
  `metadata` emptied (the row stub and its indexes stay);
- **C**: B, plus those kinds' `og_objects` rows, their `og_key_index` rows,
  and all of `run_memory_entries` deleted (the object store holds them). C is
  the closest reading of the record and is the **scored** figure; A and B
  bracket it.
- **D** (not in the record, added because of the surprise below): C, plus
  `event_head_receipt_json` dropped, since the same receipt is a column of the
  PG control receipts.

Each step was run on a reflink copy and followed by `VACUUM`.

### Full load versus sample

| Dataset | Source rows | Loaded | Basis |
|---|---|---|---|
| Control `computer_event_append_receipts` | 6,065,689 | **4,435,572 (73.1%)** | [V] counts; remainder [I] extrapolated at the measured bytes per row |
| Control, other 34 tables | all | all | [V] |
| Corpus, 5 tables | all | **all** (663,991 / 500,528 / 156,492 / 65,966 / 823,332; exact counts match Dolt) | [V] |
| Paper guest store | 5,023,253 events, 3,843,666 objects, 1,672,478 run-memory rows, 545,234 key-index rows, 112,641 edges | **1/16 sample**: rows with `CRC32(first PK column) MOD 16 = 0` (314,361 / 240,344 / 104,885 / 34,143 / 6,983 = 6.26% to 6.27% of each table) | [I] file bytes times 16 |
| Small d@e computer | all (12,709 events) | all | [V] |
| Always-on owner computer | all (596,003 events) | all | [V] |

Why the receipts load stopped at 73%: with up to six parallel loaders the
host load average ran 8 to 17 and Autopaper health began to answer
`store_stalled` 503s and 3 to 5 s latencies; the loaders' guards slowed and
then aborted twice. I killed all loaders at 01:24 rather than push the host.
The loaded rows are the leading 60 to 100% of three of four equal sequence
windows, so they cover the whole period; the per-row width is flat
(2,512 bytes average row width in every 500K-sequence bucket, 1,564 bytes
for `event_head_receipt_json` in every bucket) [V], so the extrapolation
error should be a few percent [I].

The texture_agent_mutations table (42,925 rows) drew zero rows in the paper
sample (its first PK column is the owner, CRC-mod 16 missed it); 0.03 GB
(about 660 B per row from the always-on computer's 546 rows) was added
[I].

## R0f-02: PostgreSQL control, 19.1 GB

Per-table sizes, `pg_total_relation_size` after `VACUUM (ANALYZE)`.
`computer_event_append_receipts` is measured on 4,435,572 rows.

| Table | Rows | Total | Table (heap+TOAST) | Indexes |
|---|---|---|---|---|
| computer_event_append_receipts (loaded) | 4,435,572 | 13,922,500,608 | 11,928,182,784 | 1,994,317,824 |
| computer_event_append_receipts (extrapolated, 6,065,689) | | **19.04 GB** [I] | | |
| computer_lifecycle_receipts | 28,808 | 53,837,824 | 47,333,376 | 6,504,448 |
| computer_key_escrow_transparency | 3,527 | 2,203,648 | 2,105,344 | 98,304 |
| computer_checkpoints | 213 | 1,662,976 | 1,564,672 | 98,304 |
| computer_version_route_authorization_evidence | 576 | 1,548,288 | 1,400,832 | 147,456 |
| computer_file_roots | 2,050 | 1,179,648 | 696,320 | 483,328 |
| verifier_attestations | 640 | 417,792 | 352,256 | 65,536 |
| the other 27 tables | under 600 each | 1.9 MB together | | |

`pg_database_size(control)` is 13,996,447,411 bytes with the loaded rows
[V]; the 34 non-receipt tables plus catalogs are 73.9 MB. Extrapolated
database: 19,039,158,600 + 73,946,803 = **19.11 GB (17.8 GiB)** [I].

Receipts per row in PostgreSQL: 3,139 bytes = 1,049 heap + 1,641 TOAST +
450 index. The four indexes are 554 MB (primary key), 630 MB (digest unique),
467 MB (kind/sequence), 344 MB (sequence unique) for 4.44M rows [V]. The
precommit assumed 1 to 1.5 KB per row. The two JSON columns
(`event_head_receipt_json` 1,564 bytes, `pin_receipt_digests_json` 73 bytes
average stored size) and 14 hash columns are the bulk; most of it is hex
digests, which compress only about 2:1.

Tuning check, a 300,000-row copy [V]: default 805.8 MB (2.69 KB/row) versus
`SET COMPRESSION lz4` plus `toast_tuple_target=256`, 1,076 MB, which is
larger, not smaller. Compression tuning does not help this table.

**Verdict.** Inside the interval, and 0.9 GB (4.5%) under the 20 GB
falsifier "reopens receipt retention and granularity". That is a thin
margin on an extrapolation of ±3% [I], and moot within a day: the receipts
were 340,000 per day over 2026-10-23..29 (5.90 M since 09-25), so +1.07
GB/day (R0f-06) pushes control over 20 GB at the next cutover rehearsal. The
falsifier should be treated as live, not as having passed.

## R0f-03: PostgreSQL corpus, 2.17 GB

Full load of the Store B keep-set tables, no sampling [V].

| Table | Rows | Total | Heap+TOAST | Indexes |
|---|---|---|---|---|
| og_objects | 663,991 | 949,444,608 | 779,526,144 | 169,918,464 |
| og_edges | 500,528 | 508,305,408 | 262,094,848 | 246,210,560 |
| proj_capture_terms (derived) | 823,332 | 479,854,592 | 225,705,984 | 254,148,608 |
| og_lookup_keys (derived) | 156,492 | 111,837,184 | 39,600,128 | 72,237,056 |
| proj_capture_index (derived) | 65,966 | 111,493,120 | 48,529,408 | 62,963,712 |
| **`pg_database_size(corpus)`** | | **2,169,083,571** | | |

The 26 other Store B tables are empty (keep-set accounting §2) and were not
created. Store B is 15 GB on disk in Dolt (16 GB with `.dolt`), so the
engine move is a 7.4 times reduction on the same rows [V]. In `og_objects`,
57 MB of `choir.publication_transclusion` bodies and 26 MB of
`choir.publication_check` bodies are the largest inline bodies; bodies of
1 KiB or more are already outside (object store, R0f-05).

**Verdict.** 2.17 GB is inside [2, 15], at its lower edge. The record counts
"objects, edges, captures' metadata, publications, capture index"; if the
capture index means only `proj_capture_index` and lookup keys and terms are
excluded as derived, the figure is 1.57 GB (objects + edges + capture
index), which is below the interval. I score the five-table load (the tables
the code needs to run) as the measurement and report the other reading. The
estimate of 6 GB was 2.8 times too high either way: Store B's logical bytes
were 1.2 GB (accounting §2) and PostgreSQL adds only about 0.9 GB of index
and overhead.

## R0f-04: guest SQLite

### Paper (1/16 sample, ×16) [I]

Sample sizes and extrapolation (×16, plus 0.03 GB for the missing table):

| Variant | 4 KiB pages: sample file | ×16 | 16 KiB pages: sample file | ×16 |
|---|---|---|---|---|
| Full mechanical port (event_json kept, nothing moved) | 2,793,828,352 | 44.7 GB | 2,733,047,808 | 43.8 GB |
| A: event_json emptied | 2,793,828,352 | 44.7 GB | 2,303,574,016 | 36.9 GB |
| B: A + content-hash kinds' bodies and metadata moved, stubs stay | 2,317,676,544 | 37.1 GB | 1,873,379,328 | 30.0 GB |
| **C: B + those rows, their key-index rows and run_memory_entries gone (scored)** | **1,744,871,424** | **27.9 GB** | **1,296,744,448** | **20.8 GB** |
| D: C + `event_head_receipt_json` dropped | not run | | 740,933,632 | 11.9 GB |

Per-table breakdown of the sample (4 KiB, full port) [V]:
`computer_event_index` table 1,290,850,304 + indexes 94,068,736;
`og_objects` table 787,623,936 + indexes 330,035,200;
`run_memory_entries` table 222,076,928 + indexes 28,835,840;
`og_key_index` 11,919,360 + 22,409,216; `og_edges` 3,096,576 + 2,846,720.
Times 16: events 22.2 GB, objects 17.9 GB, run memory 4.0 GB, key index
0.5 GB. In state C at 4 KiB the remaining objects are 5.3 GB, events 22.2 GB
(unchanged by A and C), key index 0.35 GB.

**Verdict: outside, high.** 27.9 GB at the SQLite default page size, 20.8 GB
repacked to 16 KiB, against 3 GB [1, 10]. Even the generous D variant, which
removes a second per-event duplicate the record did not name, is 11.9 GB,
still above 10. Reasons, all [V] on the always-on computer and confirmed on
the paper sample:

1. `event_json` (1,511 bytes average) is **not** the only large column of
   `computer_event_index`. `event_head_receipt_json` is 1,459 bytes and holds
   a copy of the same receipt that already lives in the PG control receipts
   (above, 1,564 bytes there). The design note's "about 12 GiB of 26.5 GB is
   that copy" accounted for one of two.
2. Row width 2.4 to 3.8 KB against a 4,096-byte page gives **one row per
   page**: `dbstat` shows 595,989 leaf pages for 596,003 rows. Emptying
   `event_json` therefore freed 11 MB of 3,889 MB (0.3%) at 4 KiB; at 16 KiB
   it freed 814 MB (21%). The file shrinks only if the page size is raised or
   the second JSON column goes too.
3. SQLite stores the Dolt content uncompressed; Dolt's chunks compress. The
   paper store is 29 GB in Dolt (`state.texture`, [V] `du`) and about 44.7 GB
   as an uncompressed SQLite port with event_json.
4. The run-memory table (4.0 GB) and the object kinds the record calls
   content-hash move only in B/C; the precommit does not say whether the
   `run_memory_entries` table moves with them. Kept (A), it is 4.0 GB.

Also unmeasured and not included: the guest already carries a separate
SQLite, `state-actor.db`, 625 MB on the paper [V `du`], which is not part of
the Dolt store and not in these figures.

### d@e-class computers [V], full loads

The record says "a d@e-class user computer". Two owner computers were
measured because they differ by 50 times in event volume.

| Computer | Events | Dolt `.dolt` | Full port 4 KiB | A | B | C (scored) 4 KiB | C 16 KiB | D 16 KiB |
|---|---|---|---|---|---|---|---|---|
| Small: `computer-59c902c5` | 12,709 | 600 MB | 100.5 MB | 100.2 MB | 89.4 MB | **67.0 MB (63.9 MiB)** | 50.2 MB (47.9 MiB) | 27.7 MB (26.4 MiB) |
| Always-on: `computer-03335285` | 596,003 | 2.3 GB | 3.889 GB | 3.878 GB | 3.402 GB | **2.849 GB (2.65 GiB)** | 2.025 GB | 0.971 GB |

(16 KiB full/A/B for the small computer: 99.2 / 81.8 / 71.8 MB; for the
always-on one: 3.840 / 3.026 / 2.568 GB.)

**Verdict.** The small computer is inside [30, 800] MiB in every variant but
D (A 96 MiB at 4 KiB, B 85 MiB, C 64 MiB, C at 16 KiB 48 MiB, D 26 MiB which
is just under 30). Scored on C at 4 KiB: 64 MiB, inside. The always-on owner
computer is 2.65 GiB, outside the same interval (3.4 times the upper bound)
and dominated by 596,003 `projection_batch_recorded` events (592,313 of
them) [V]. "A d@e-class computer" is not one size; the proof ring contains
both.

## R0f-05: object store, 9.98 GB

The bytes the keep set references, deduplicated by hash [V]:

| Part | Distinct objects | Bytes |
|---|---|---|
| Capture bodies referenced by `choir.web_capture` metadata (`content_blob_id`, `blocks_blob_id`, `extracted_text_blob_id`), all present in `capture-bodies/` (0 missing) | 155,203 | 9,380,260,904 (allocated: 9,816,215,552) |
| Published blocks, manifests, transclusions: og blobs referenced by `og_objects.body_ref`, all present | 21,896 | 475,662,075 |
| **Subtotal, what the object-storage design imports** | 177,099 | **9,855,922,979** (matches the design's 9.85 GB) |
| File contents of the Autopaper (file-CAS chunks, `computer-1355b83b`) | | 118,592,088 |
| Referenced image blobs | 0: `state.image-blobs` does not exist on the paper or the always-on guest image | 0 |
| **Total scored** | | **9.975 GB** |

Not in the scored figure: the owner's three computers' file-CAS chunks
(144 MB, 207 MB, 219 MB; nice-to-keep per the owner's note), which would
bring it to 10.54 GB; the 36 orphan capture bodies (5.7 MB) and 14 orphan
og blobs. Per-owner dedup is not applied separately because the store is
content-addressed by hash and the 604 shared bodies are already one file.
Directory totals today: `capture-bodies/` 155,394 files, 9,388,551,964 bytes
(9.82 GB allocated); `sha256/og/` 21,928 files, 476,627,561 bytes [V].

**Verdict.** Inside [9.5, 30], 0.5 GB above the lower bound and 17% below
the 12 GB point. The estimate's headroom was for image blobs and file
contents, which are small or absent.

## R0f-06: growth

The records ask for "growth from a replayed day of the paper's write mix".
No day was replayed: the writers cannot be driven against scratch engines
here. Rates below are measured row counts per day (`created_at`, or file
mtime for blobs) times the measured bytes per row, which is the same arithmetic
a replay would give if the per-row sizes stay constant [I]. The corpus and the
capture directory exist only since the 10-09 reset (17 full days of data,
10-13 to 10-29), so "30-day mean" is a 17-day mean for those two.

| Stream | Window | Mean per day | Rows or files per day | Bytes per row |
|---|---|---|---|---|
| Control (receipts) | 30 days 09-30..10-29 | **0.61 GB** | 193,916 | 3,139 (total incl. indexes) |
| | 14 days | 1.00 GB | 319,554 | |
| | last 7 days | 1.07 GB | 340,071 | |
| Corpus (five tables) | 17 days 10-13..10-29 | **0.127 GB** | 39,000 objects, 29,400 edges | 2.169 GB / 664K objects |
| | last 7 days | 0.134 GB | 41,100 objects | |
| Blobs (captures + og) | 17 days | **0.580 GB** (0.552 + 0.028) | 10,300 files | 9.86 GB / 177K |
| | last 7 days | 0.523 GB | | |

Control per-day counts (true, from the Dolt snapshot, `created_at`, [V]):
10-22 762,436; 10-23 437,949; 10-24 321,234; 10-25 321,019; 10-26 370,357;
10-27 361,607; 10-28 225,927; 10-29 342,407. The precommit's "about 400 K
events a day" held; its "1 to 1.5 KB per receipt" did not.

**Verdicts.** All three inside their intervals: control 0.61 GB/day
(est 0.5), corpus 0.127 (est 0.15), blobs 0.58 (est 0.65). The control rate
on the last seven days is 1.07 GB/day, 2.1 times the estimate and 71% of the
interval's upper bound. The note "falls by an order of magnitude if the paper
leaves the computer tape" stands: the paper's computer is 5.02 M of the
6.07 M receipts (83%) [V].

## R0f-07: total

Both PostgreSQL instances, the paper's SQLite, the object store; excluding
VM images, Nix, swap, backups, projection bases, the owner computers and
`state-actor.db`:

| Component | Scored | With 16 KiB pages | With variant D at 16 KiB |
|---|---|---|---|
| PG control | 19.11 GB | 19.11 | 19.11 |
| PG corpus | 2.17 | 2.17 | 2.17 |
| Paper SQLite (C) | 27.95 | 20.78 | 11.88 |
| Object store | 9.98 | 9.98 | 9.98 |
| **Total** | **59.2 GB** | **52.0 GB** | **43.1 GB** |

**Verdict.** Inside [15, 80] under every reading, 24 GB above the 35 GB
point estimate. The estimate's error cancels: corpus and object store came
in low (by 4 and 2 GB), control close to the interval ceiling, and the paper
SQLite 25 GB high. About 47% of the total is the paper's store, which the
owner has said does not need its tape.

## Surprises, residuals, and what went wrong

1. **Store A (control Dolt) is throttled at its own `MemoryHigh`.**
   `systemctl show go-choir-platform-dolt`: `MemoryCurrent=4,294,828,032`,
   `MemoryHigh=4,294,967,296` [V] (a runtime drop-in
   `50-MemoryHigh.conf`). A live 5,000-row range read of the receipts took
   38 s and the next timed out at 100 s; loading 6 M rows over SQL would have
   taken more than 13 hours. This is why the load used a reflink copy. Offline
   reads of the copy ran at about 3,000 rows/s [V].
2. **Store B (corpus Dolt) was OOM-killed twice during the window**, by its
   own cgroup (`MemoryMax=7,516,192,768`): 00:06:02 and 00:39:50 UTC,
   `anon-rss` 7.3 GB each, `NRestarts=2` [V from `journalctl -k`]. systemd
   restarted it both times. My live queries against :13307 were five
   `SHOW CREATE TABLE` reads at about 00:03 and none after; at 00:39 I
   had no live query against it and the offline loads read the reflink copy,
   so I attribute both kills to Store B's own readers [I]. This is the
   "more Dolt problems" the owner wanted to avoid and is independent of this
   measurement; it recurred at a 34-minute interval and deserves its own
   problem doc.
3. **Live queries I did issue against the production Dolt servers**, all
   `SELECT`/`SHOW` with a client timeout, before switching to the snapshot:
   table lists and row counts on both; `SHOW CREATE TABLE` for 35 + 5
   tables; on Store A, a `LIMIT 1 OFFSET 2999` ordered read, a
   `GROUP BY computer_id` count over 6 M rows (28 s), two 5,000-row range
   reads (38 s, then timeout at 100 s), one 3,000-row batch of the first
   loader (a second batch was killed), and the whole-table reads of the 34
   small control tables (about 40,000 rows in 2,000-row batches, the largest
   being `computer_lifecycle_receipts`, 28,808 rows in 15 s). After that, nothing touched :13306 or :13307.
4. **Autopaper health under load.** The health endpoint answered
   `{"status":"store_stalled"}` 503 on roughly 1 in 8 to 1 in 15 probes during
   loads, and once on an earlier idle probe, and 3 to 5 s latencies at load
   above 9; I cannot separate this from the Paper's own baseline (its
   firecracker ran at 93% CPU and host load was 4.4 to 7.9 before I
   started). Host load average peaked at **17.2 at 00:39** when I ran a
   `GROUP BY` over the 3.8 M-row paper `og_objects` on the offline copy while
   five loaders ran; I killed it at once, and the loaders' guards aborted on
   health. Afterward the loaders ran with a throttle (pause while load above
   9, health not ready, or latency over 3 s) and load stayed 4 to 11. The
   guideline was "under about 6"; it was exceeded on and off. Nothing
   restarted and `/health` ended `ready`.
5. **Pre-existing, not mine, noticed:** vmctl's
   `deploy-refresh-pending.json` shows 4,193 attempts for
   `vm-70070cf7...`, and `go-choir-platform-dolt-squash` and
   `go-choir-platform-dolt-history-audit` units were already `failed` at
   00:04.
6. **Dolt `information_schema.table_rows`** was right for every table here
   (matched `COUNT(*)` for og_objects, og_edges, receipts and the six paper
   tables), unlike `data_length` (accounting §2).
7. **Decisions that move the numbers** [I]: indexes were built after the
   load (compact); a steady-state tree with random inserts is typically
   10 to 30% larger for the `idempotency_key` and digest indexes. `jsonb`
   rather than `json` (normalises whitespace and key order; the byte count
   of `pg_column_size` for the 1,564-byte receipt is the stored form).
   SQLite used rowid tables, so the 64-character-hash primary keys are stored
   twice (row and index); `WITHOUT ROWID` for the narrow tables
   (`og_key_index`, `og_edges`, lookup tables) would cut the guest index
   bytes, but those are 0.5 GB and 0.1 GB at the paper's scale. The SQLite
   `crc32` sample is uniform by key but not stratified by size; the
   re-measured row counts per table (6.26 to 6.27%) match the sample
   fraction, so I assume the byte share holds [I].
8. **Not measured:** the paper's published Textures and revision counts in
   the guest (the accounting §8.1 residual); I did not read table contents.
   `choir.texture_revision` rows are in the CH set and are removed in C, so
   their bytes are inside the C-to-B delta. Growth by a replayed day (above).
   The owner computer `computer-6c50d6e5` (hibernated).
9. **Disk and cleanup.** Peak scratch use was roughly 16 GB of PostgreSQL data plus
   10 to 20 GB of SQLite files (reflink copies share extents; I did not
   `du` the peak), under the 40 GB cap by my estimate [I]; free space was
   checked before each big step; free space never fell under 87 GB on `/`. Cleanup done at 01:57:
   unit `engine-measure-pg` stopped, three loop mounts unmounted, the whole
   `/var/lib/go-choir/scratch-engine-measure` tree (including the reflink
   copies) deleted, `systemctl is-active engine-measure-pg` is `inactive`,
   no PostgreSQL, Dolt-offline, sqlite3 or loader process remains. The
   reflink copies were removed before any Dolt compaction could pin their
   extents. The tooling (loaders, variant scripts, size files) is kept
   outside the repo only; it is described by this section, not committed.

## Heresy delta

`discovered`: (a) the guest event index stores each event's receipt twice
(`event_json` and `event_head_receipt_json`, about 1.5 KB each) and the
precommit's design note named only one; (b) Store B OOM-killed twice in one
hour by its own cgroup limit, and Store A runs pinned at its `MemoryHigh`;
(c) receipts cost 3.1 KB per row in PG, not 1 to 1.5 KB.
`introduced`: none (read-only; scratch removed). `repaired`: none.

Sources and artifacts: the scripts and per-table size files are in the run's
scratch (deleted); the figures above were copied from them before deletion.
