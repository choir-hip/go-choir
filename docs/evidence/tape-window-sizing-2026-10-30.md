# Tape retention window sizing (2026-10-30)

Status: measurement only, mutation class green (docs). No production data or
service was changed; every row was read from btrfs reflink copies or from
sealed segment files read-only. Serves the "Owner direction 2026-10-30"
section of
[the data-architecture selection](../definitions/choir-data-architecture-selection-2026-10-28.md)
(item 3: "to measure per computer before choosing: events per day and bytes
per event under the new schema"). Method inherits
[last night's storage measurement](new-engine-storage-measurement-2026-10-30.md).

Labels: **[V]** verified by a command in this run (Node B, 2026-10-30 02:36 to
03:25 UTC); **[I]** inferred or extrapolated, basis stated. GB means 10^9
bytes. Days are UTC dates of the host receipt `created_at`; 2026-10-29 is the
last complete day (10-30 holds 40,190 receipts up to 02:36 and is left out of
every rate).

## Headline

1. **The paper is the whole fleet.** Autopaper computer (computer-1355b83b):
   5,057,779 of 6,102,549 host receipts [V], 96% of the last 14 days' events
   (307,887 per day mean, 326,662 last 7 days, one 741,952 day on 10-22) [V].
   The other 248 computers (178 with any event in 30 days) together produce about 10,500 events per day [V].
2. **A guest tape row costs 4.4 KB, and only the page size or dropping both
   JSON columns changes that.** Measured by building the real table in SQLite
   from the small computer's 12,709 rows: 4,409 bytes per event at 4 KiB pages
   and 4,406 at 16 KiB (table + four indexes) [V]. Dropping the duplicate
   receipt (layout b) saves nothing at 4 KiB (4,403: still one row per page)
   and 31% at 16 KiB (3,035) [V].
3. **There is no large event body to move.** `event_json` is a 1.5 KB envelope
   for every kind (max 1,880 bytes over 164,000 rows read) [V]; the payload
   that is large (mean 4.5 KB, p95 12.9 KB, max 637 KB) is the
   `projection_batch_recorded` payload artifact, which already lives on the
   host in `tape-seg`, not in the guest. So layout (c) is measured as
   "`event_json` replaced by the `event_artifact_digest` already in the row"
   (1,325 bytes per event at 4 KiB, 1,212 at 16 KiB) [V].
4. **Window bytes for the paper** (mean rate 307,887 events per day): 7 days
   is 2.16 M events, 9.5 GB as today, 6.5 GB under (b) at 16 KiB, 2.6 GB under
   (c) at 16 KiB; 14 days is 4.3 M events and 19.0 / 13.1 / 5.2 GB; 30 days
   is 9.2 M events and 40.7 / 28.0 / 11.2 GB [I from V rate and V row size].
   Every other computer is under 0.4 GB for 30 days under any layout (below).
5. **Sealed segments the paper sends to object storage: 2.6 GB raw per day,
   about 0.49 GB per day zstd-3 (5.4x), 0.43 GB zstd-9 (6.2x)**, plus the host
   receipt rows if they are sealed with them: 0.77 GB raw, about 0.12 GB per
   day compressed [V rates, V ratios].
6. **Precommitment records are small next to the tape.** The paper holds
   127,901 `choir.commitment_record` objects, about 10,800 per day over the last
   7 days, 3.2 KB each (412 MB total) [V]. Kept forever that is 35 MB per day,
   about 13 GB per year before indexes [I]. Receipts behind them are at most
   3.3% of the paper's receipts (one per record, at most) [V bound].

## Method

- **Host receipts.** `go-choir-platform-dolt` data directory reflink-copied
  (`cp --reflink=always`, one second), read by offline `dolt sql -r csv`
  under `systemd-run` (MemoryMax=3G, Nice=19, idle IO). Query 1: count by
  computer, UTC day, kind over all 6,102,549 receipts (66 s). Query 2: every
  64th receipt by sequence (95,256 rows, about 17 minutes) with the byte
  length of each variable column. Fixed columns (nine 64-character digests,
  sequence, created_at) are counted at their stored width.
- **Guest tape.** The three guest `data.img` files were reflink-copied
  (paper 128 GB sparse, owner computer, small computer), loop-mounted on the
  copies, and read with offline Dolt. The live paper image was never opened
  directly. Owner computer: every 4th event plus all non-batch events
  (151,759 rows). Small computer: all 12,709 rows. Paper guest `computer_event_index`
  was **not** scanned (first attempt, a `count(*), min, max` over 5 M rows of
  4.1 KB, pushed the host load to 23 and health to 503; I stopped it and used
  the host receipts, the owner and small computers, and the sealed segments
  instead; see Surprises).
- **Layout bytes.** The small computer's rows were loaded into SQLite 3.x with
  the DDL of `internal/store/store.go` (rowid table, the four indexes built after
  load, `VACUUM`) at 4096 and 16384 page size, and the file read with
  `dbstat`. Variants: (a) as today; (b) `event_head_receipt_json` and
  `event_head_receipt_digest` dropped; (c) (b) plus `event_json` emptied (the
  event artifact digest stays in the row, so a ref costs 0 extra bytes). The
  paper's rows have the same shape (measured last night at 4,106 bytes per row
  for the table alone at 4 KiB, here 4,111) [V], so the small computer's
  per-row figures are used for all computers. Window bytes = events x bytes per
  event; this assumes steady-state packing equals a bulk load (a window with
  deletes leaves free pages that `VACUUM` or reuse recovers) [I].
- **Sealed segments.** The host already writes `tape-seg/<computer>/seg-N.seg`
  (format: `internal/tapestore/format.go`, 32-byte header, 46-byte record
  header, namespaces event / payload / receipt) for `projection_batch_recorded`
  only. Bytes per day come from the file sizes and mtimes of all 493 paper
  segments (26.9 GB, 10-21 to 10-30); record composition and compression from
  parsing five paper segments (about 335 MB, 164,000 records, one per
  day-ish spread) plus every segment of the small (89.6 MB) and owner (7.9 MB)
  computers, and `zstd -3` / `zstd -9` of each whole segment.
- **Records.** `choir.commitment_record` rows of `og_objects` in each guest
  Dolt (precommitment-records.md section 2.2: "code: commitment record, kind
  `choir.commitment_record`", "live on the computer's object graph and tape"),
  counted and sized by day from `created_at`. Every act (precommit, report,
  resolve, directive) is one such object, so this is an upper bound on
  "precommitment records" proper; the share of acts that are precommits
  was not split out.

## 1. Events per day

Host receipts per day and kind [V], 2026-09-30 to 2026-10-29. PBR is
`projection_batch_recorded`. Zeros before 10-13 for the paper are real: its
computer's first receipt is 10-13 (genesis), after the 10-09 reset. The small
computer exists from 10-26. "Other 178" is every computer except the three.

| Date (UTC) | Paper total | PBR | artifact_produced | file_root | key_revoked | Owner 0333 | Small 59c9 | Other 178 | Fleet |
|---|---|---|---|---|---|---|---|---|---|
| 2026-09-30 | 0 | 0 | 0 | 0 | 0 | 25,194 | 0 | 834 | 26,028 |
| 2026-10-01 | 0 | 0 | 0 | 0 | 0 | 60,336 | 0 | 184 | 60,520 |
| 2026-10-02 | 0 | 0 | 0 | 0 | 0 | 90,994 | 0 | 231 | 91,225 |
| 2026-10-03 | 0 | 0 | 0 | 0 | 0 | 68,156 | 0 | 148 | 68,304 |
| 2026-10-04 | 0 | 0 | 0 | 0 | 0 | 2,234 | 0 | 7,204 | 9,438 |
| 2026-10-05 | 0 | 0 | 0 | 0 | 0 | 69,633 | 0 | 233 | 69,866 |
| 2026-10-06 | 0 | 0 | 0 | 0 | 0 | 61,506 | 0 | 4,027 | 65,533 |
| 2026-10-07 | 0 | 0 | 0 | 0 | 0 | 27,014 | 0 | 6,348 | 33,362 |
| 2026-10-08 | 0 | 0 | 0 | 0 | 0 | 2,916 | 0 | 3,526 | 6,442 |
| 2026-10-09 | 0 | 0 | 0 | 0 | 0 | 5,222 | 0 | 19,947 | 25,169 |
| 2026-10-10 | 0 | 0 | 0 | 0 | 0 | 298 | 0 | 33,375 | 33,673 |
| 2026-10-11 | 0 | 0 | 0 | 0 | 0 | 162 | 0 | 15,288 | 15,450 |
| 2026-10-12 | 0 | 0 | 0 | 0 | 0 | 95 | 0 | 9,127 | 9,222 |
| 2026-10-13 | 60,803 | 60,523 | 254 | 21 | 4 | 776 | 0 | 65,302 | 126,881 |
| 2026-10-14 | 299,108 | 297,980 | 1,035 | 85 | 8 | 9,897 | 0 | 28,660 | 337,665 |
| 2026-10-15 | 347,261 | 346,217 | 939 | 75 | 30 | 319 | 0 | 17,356 | 364,936 |
| 2026-10-16 | 331,766 | 330,129 | 1,532 | 82 | 23 | 182 | 0 | 19,706 | 351,654 |
| 2026-10-17 | 101,716 | 100,901 | 718 | 94 | 3 | 209 | 0 | 0 | 101,925 |
| 2026-10-18 | 89,743 | 89,018 | 629 | 96 | 0 | 116 | 0 | 0 | 89,859 |
| 2026-10-19 | 143,129 | 142,133 | 898 | 85 | 13 | 309 | 0 | 0 | 143,438 |
| 2026-10-20 | 180,792 | 179,532 | 1,148 | 77 | 35 | 437 | 0 | 16,122 | 197,351 |
| 2026-10-21 | 434,682 | 431,838 | 2,741 | 70 | 33 | 169 | 0 | 11,742 | 446,593 |
| 2026-10-22 | 741,952 | 738,015 | 3,829 | 75 | 33 | 353 | 0 | 20,131 | 762,436 |
| 2026-10-23 | 435,894 | 432,205 | 3,586 | 83 | 20 | 255 | 0 | 1,800 | 437,949 |
| 2026-10-24 | 300,338 | 297,369 | 2,870 | 87 | 12 | 178 | 0 | 20,718 | 321,234 |
| 2026-10-25 | 298,595 | 295,623 | 2,872 | 83 | 17 | 271 | 0 | 22,153 | 321,019 |
| 2026-10-26 | 337,188 | 334,323 | 2,757 | 77 | 31 | 485 | 12,179 | 20,505 | 370,357 |
| 2026-10-27 | 347,413 | 344,776 | 2,542 | 84 | 11 | 444 | 311 | 13,439 | 361,607 |
| 2026-10-28 | 225,140 | 223,078 | 1,996 | 59 | 7 | 102 | 131 | 554 | 225,927 |
| 2026-10-29 | 342,069 | 339,120 | 2,865 | 83 | 1 | 240 | 88 | 10 | 342,407 |

Summary rates [V arithmetic on the table]:

| Series | Days | Mean / day | Last 14 d | Last 7 d | p50 day | p95 day | Peak day |
|---|---|---|---|---|---|---|---|
| Paper (10-13 on) | 17 | 295,152 | 307,887 | 326,662 | 300,338 | 435,894 | 741,952 (10-22) |
| Owner 0333 (09-30 on) | 30 | 14,283 | 268 | 282 | 395 | 68,156 | 90,994 (10-02) |
| Owner 0333 (since 10-09 reset) | 21 | 977 | 268 | 282 | 271 | 776 | 9,897 (10-14) |
| Small 59c9 (10-26 on) | 4 | 3,177 | 908 | 1,816 | 221 | 311 | 12,179 (10-26 import) |
| Other 178 computers | 30 | 11,956 | 10,491 | 11,311 | n/a | n/a | 65,302 (10-13) |
| Fleet | 30 | 193,916 | 319,554 | 340,071 | 114,403 | 437,949 | 762,436 (10-22) |

**Bursts.** (1) The paper's 10-21/10-22 step: 435 k then 742 k receipts, then
back to 300 k to 350 k; the rate is not stationary (90 k per day on 10-17/18, 300 k on
10-24+). Of the 14-day mean 307,887, no day other than 10-22 is above 440 k.
(2) The owner computer 0333 was a 60 k to 91 k per day test load on
09-30 to 10-07 (428,502 events in 30 days) and has idled at 100 to 485 per
day since the reset except 9,897 on 10-14. The "always-on" label of last
night's note does not hold for its rate now: 596,003 events are history, 282
per day are present. (3) The small computer's 12,179 events on 10-26 are its
genesis import and first projections; 88 to 311 per day since. (4) Other
computers burst individually: 65 k on 10-13 (fleet-other), 33 k on 10-10.

**Kinds** (09-30 to 10-29, share of events) [V]:

| Computer | projection_batch_recorded | artifact_produced | file_root_committed | key_revoked | other kinds |
|---|---|---|---|---|---|
| Paper (5,017,589) | 99.31% | 0.66% | 0.03% | 0.01% | genesis_imported 1 |
| Owner 0333 (428,502) | 99.25% | 0.05% | 0.57% | 0.11% | trajectory_started 94, effect_accepted 15, materialization_* 30 |
| Small 59c9 (12,709) | 96.00% | 0.58% | 2.63% | 0.28% | 15 other kinds, about 0.5% together |
| Fleet (5,817,470) | 99.25% | 0.60% | 0.11% | 0.02% | trajectory_started 331, genesis_imported 178, effect_* and materialization_* under 0.01% each |

Per-computer distribution [V]: 251 computers have receipts ever, 181 had
events in the last 30 days, 27 in the last 14 days. Among those 27 the median
is 62 events per day (last 14 d); the largest non-paper computer
(computer-6c50d6e5) did 5,432 per day (76,044 events in its whole life); no
non-paper computer has more than 22,500 events in a day.

## 2. Bytes per event

### 2.1 Guest `computer_event_index` row (the on-computer cost)

Measured by building the table from the small computer's 12,709 rows [V]:

| Layout | Page | File B/event | Table B/event | Index B/event | Leaf pages | Overflow pages |
|---|---|---|---|---|---|---|
| (a) as today | 4 KiB | 4,409 | 4,111 | 297 | 12,638 | 90 |
| (a) as today | 16 KiB | 4,406 | 4,107 | 298 | 3,183 | 0 |
| (b) receipt dropped | 4 KiB | 4,403 | 4,105 | 297 | 12,709 | 0 |
| (b) receipt dropped | 16 KiB | 3,035 | 2,736 | 298 | 2,119 | 0 |
| (c) b + event_json to ref | 4 KiB | 1,325 | 1,027 | 297 | 3,178 | 0 |
| (c) b + event_json to ref | 16 KiB | 1,212 | 913 | 298 | 707 | 0 |

Page packing, stated: the logical row in (a) is about 3.87 KB (event_json
1,511 + event_head_receipt_json 1,457 + nine 64-character digests and the
receipt digest 640 + payload pin JSON 68 + idempotency key 53 + computer id 41 +
kind 25 + two timestamps 52 + small fields), so at 4 KiB a page holds one row
(a second would need 7.7 KB) and the other 3% to 6% of the page is waste (12,638
leaf pages for 12,709 rows); (b) is 2.4 KB, still one per page at 4 KiB, which
is why dropping 1.5 KB frees nothing; at 16 KiB the usable 16,376 bytes hold
4 rows of (a) (97% full) and 6 of (b) (90%). (c) is 0.95 KB: 3 per 4 KiB page,
15 per 16 KiB page. Index cost is flat at 297 bytes (event_digest PK,
(computer,sequence), (computer,idempotency), (computer,status,sequence)).
Last night's separately measured paper sample, 4,106 bytes per row at 4 KiB, matches.

By component, mean bytes per event [V]:

| Component | Bytes | Share of (a) |
|---|---|---|
| `event_json` | 1,511 | 34% |
| `event_head_receipt_json` | 1,457 to 1,459 | 33% |
| Nine hex columns + receipt digest (10 x 64) | 640 | 15% |
| `payload_pin_receipt_digests_json` | 68 | 1.5% |
| `idempotency_key` | 53 | 1% |
| other columns (computer_id, kind, status, times, ints) | about 140 | 3% |
| Index overhead | 297 | 7% |
| Page waste at 4 KiB | about 240 | 5% |

By event kind (owner computer 151,759 rows, small computer 12,709 rows) [V].
Bytes are `length()` of the text; all the kinds are within 25% of each other
and the page layout rounds them to the same cost, so the per-kind byte share of
a window equals its event share (99.3% projection batches on the paper):

| Kind | n (owner + small) | event_json mean / p50 / p95 / max | event_head_receipt_json mean / max |
|---|---|---|---|
| projection_batch_recorded | 160,270 | 1,511 / 1,511 / 1,511 / 1,575 | 1,459 / 1,523 |
| file_root_committed | 2,933 | 1,520 / 1,520 / 1,520 / 1,584 | 1,453 / 1,517 |
| key_revoked | 779 | 1,505 / 1,507 / 1,507 / 1,571 | 1,445 / 1,509 |
| artifact_produced | 270 | 1,611 / 1,611 / 1,611 / 1,611 | 1,451 / 1,451 |
| trajectory_started | 107 | 1,618 / 1,612 / 1,665 / 1,676 | 1,454 / 1,516 |
| effect_accepted / proposed / rejected | 15 + 6 + 10 + 4 | 1,638 to 1,879 (max 1,880) | 1,447 to 1,515 |
| materialization_* , rollback_*, checkpoint_published, others | under 60 | 1,495 to 1,816 | 1,380 to 1,515 |

Overall: event_json mean 1,511, p50 1,511, p95 1,511, max 1,880; receipt
column mean 1,459, max 1,523. There is no heavy tail in the guest row.

### 2.2 Host receipt row (`computer_event_append_receipts`)

95,256-row sample, every 64th sequence, all 251 computers, 99.3% paper
batches [V]. Logical bytes per row, columns at stored width (variable
columns measured, nine 64-character digests, 64-character head and state
commitments, sequence and created_at at fixed width):

| Component | Bytes (mean = p50 = p95) |
|---|---|
| `event_head_receipt_json` | 1,507 (max 1,571) |
| `pin_receipt_digests_json` | 68 to 73 |
| `event_artifact_ref` + `idempotency_key` + `event_head_receipt_id` + `computer_id` | 64 + 74 + 36 + 41 |
| nine hex digest and head columns | 576 |
| sequence, created_at, other | 16 |
| **Row total, logical** | **2,362 (p50 2,362, p95 2,362, max 2,425)** |

By kind: projection_batch_recorded 2,361 (94,553 sampled), artifact_produced
2,379 (558), file_root_committed 2,375 (107); all other kinds under 2,425. The
receipts are fixed-size records, so the byte share of a kind is its event share.
Measured last night in PostgreSQL with indexes: 3,139 bytes per row [V, previous
run]. Index overhead on this table is therefore +33% on top of the logical row.

### 2.3 Host `tape-seg` bytes per event (what gets sealed to object storage)

Parsed from five paper segments (seg 27, 98, 234, 381, 463; 164,000 records,
67.1 MB each) [V]:

| Namespace | Records per event | Body bytes mean | p50 | p95 | max |
|---|---|---|---|---|---|
| event | 1 | 1,512 (exactly) | 1,512 | 1,512 | 1,512 |
| payload | 1 | 4,657 (4,457 in the three segments parsed for kind) | 2,150 | 13,168 | 766,505 |
| pin receipt | 2 | 895 | 890 | 901 | 903 |
| Total with 46-byte headers | | **8,139** | | | |

The payload is the only tail: p95 is 6 times the p50 and the max is 767 KB, so
a handful of batches per segment decide the day's bytes. Only
`projection_batch_recorded` is segmented today
(`internal/platform/tape_segments_write.go`); `artifact_produced` and the
other kinds (0.7% of events) are still legacy one-file-per-digest pins, small
(1.5 KB event, 0.9 KB receipt) and not counted here. Small computer, all 33
segments: 12,202 events, event 1,509, payload mean 3,862 (p50 1,970, max
144,257), receipts 895 [V]. Owner computer: 1,549 events, payload mean 1,630,
max 7,689 [V].

## 3. Precommitment records

Where they live [V from code and tables]: `choir.commitment_record` objects
in the computer's `og_objects` (registry `internal/objectgraph/registry.go:146`,
owner VM store, writer runtime; nine generated indexed columns `og_rec_*`),
written into the guest Dolt and carried on the tape inside projection
batches. No record sits in a host table of its own today.

| Computer | Records | Dates | Body + metadata, total | Per record (body / metadata) | Rate / day |
|---|---|---|---|---|---|
| Paper | 127,901 | 10-13 to 10-30 (02:36) | 412.6 MB | 3,226 B (2,526 / 700) | last 7 d 10,823; 10-29 13,075; peak 15,366 (10-27) |
| Owner 0333 | 1,725 | 09-30 to 10-14 | 2.4 MB | 1,397 B | none since 10-14 |
| Small 59c9 | 284 | 10-26 only | 0.9 MB | 3,287 B | none since 10-26 |

A one-in-ten sample of the paper's records (5,939) gives per record
body+metadata mean 2,963, p50 2,404, p95 6,028, max 113,210 bytes [V]; the
population mean above is the better figure. 80% are claim acts, 20% directives
(`og_rec_claim` 1 versus 0). Not counted: the nine generated-column indexes
(about +30% of the table, from last night's `og_objects` split) [I].

Receipts behind records: a record is created inside exactly one projection
batch event, so receipts that must outlive the window are at most
min(records, events) per day: up to 10,823 per day on the paper, **at most
3.3% of its 326,662 receipts**, at 2,362 logical bytes (3,139 in PostgreSQL)
each: at most 26 MB per day logical, 34 MB per day in PostgreSQL, about 12 GB per
year of PostgreSQL if no two records share a batch [I bound; the true number needs
the record-to-event map, which this run did not read]. The records themselves
at 3.2 KB are 35 MB per day, 12.7 GB per year before indexes [I].

## 4. Host receipts per day, fleet-wide

[V counts, V row size, previous run for PostgreSQL size]

| Window | Receipts per day | Logical (2,362 B) | PostgreSQL with indexes (3,139 B) |
|---|---|---|---|
| Last 7 d mean | 340,071 | 0.80 GB per day | 1.07 GB per day |
| Last 14 d mean | 319,554 | 0.75 GB per day | 1.00 GB per day |
| 30 d mean (09-30 to 10-29) | 193,916 | 0.46 GB per day | 0.61 GB per day |
| Since 10-09 reset (21 d) | 256,512 | 0.61 GB per day | 0.81 GB per day |
| Peak day (10-22) | 762,436 | 1.80 GB | 2.39 GB |

The paper is 96% of this (last 14 d). Receipts kept beyond the window are only
those behind records (section 3), so the host table can follow the window
and not grow at these rates if receipts are windowed too; if they are not, they
grow at the PostgreSQL column above.

## 5. Window size under the three layouts

Events per window use the last-14-day mean for the paper (307,887 per day;
the 17-day lifetime means a 30-day window is a projection [I]); "burst" is the
worst observed rolling 7-day span (10-21 to 10-27, 2,896,061 events, 413,723
per day) for the 7-day column, and that week plus the mean rate for the rest of
the 14-day and 30-day windows. Bytes
are events x section 2.1 bytes per event, in GB; "/" separates mean from burst.
Computation: window events in the second column; layouts from the table in
2.1. Includes the four indexes. Excludes `og_objects`, `run_memory_entries` and
the rest of the store (not tape; last night's 11.9 to 44.7 GB paper store
figures cover them).

### Paper (computer-1355b83b)

| Window | Events (mean / burst) | (a) 4K | (a) 16K | (b) 4K | (b) 16K | (c) 4K | (c) 16K |
|---|---|---|---|---|---|---|---|
| 7 d | 2,155,209 / 2,896,061 | 9.50 / 12.77 | 9.50 / 12.76 | 9.49 / 12.75 | 6.54 / 8.79 | 2.86 / 3.84 | 2.61 / 3.51 |
| 14 d | 4,310,418 / 5,051,270 | 19.00 / 22.27 | 18.99 / 22.26 | 18.98 / 22.24 | 13.08 / 15.33 | 5.71 / 6.69 | 5.22 / 6.12 |
| 30 d | 9,236,610 / 9,977,462 | 40.72 / 43.99 | 40.70 / 43.96 | 40.67 / 43.93 | 28.03 / 30.28 | 12.24 / 13.22 | 11.19 / 12.09 |

### Owner computer 0333 (rate since reset 268 per day; burst = the observed 09-30 to 10-29 test load)

| Window | Events (steady / observed burst) | (a) 4K | (b) 16K | (c) 4K | (c) 16K |
|---|---|---|---|---|---|
| 7 d | 1,876 / 379,873 | 0.01 / 1.67 | 0.01 / 1.15 | 0.00 / 0.50 | 0.00 / 0.46 |
| 14 d | 3,752 / 414,536 | 0.02 / 1.83 | 0.01 / 1.26 | 0.00 / 0.55 | 0.00 / 0.50 |
| 30 d | 8,040 / 428,502 | 0.04 / 1.89 | 0.02 / 1.30 | 0.01 / 0.57 | 0.01 / 0.52 |

(a) 16K and (b) 4K equal (a) 4K to within 0.01 GB for this and the next table.

### Small d@e computer 59c9 (steady about 200 per day; burst = the 12,179-event import day)

| Window | Events (steady / with import) | (a) 4K | (b) 16K | (c) 4K | (c) 16K |
|---|---|---|---|---|---|
| 7 d | 1,400 / 13,379 | 0.01 / 0.06 | 0.00 / 0.04 | 0.00 / 0.02 | 0.00 / 0.02 |
| 14 d | 2,800 / 14,779 | 0.01 / 0.07 | 0.01 / 0.04 | 0.00 / 0.02 | 0.00 / 0.02 |
| 30 d | 6,000 / 17,979 | 0.03 / 0.08 | 0.02 / 0.05 | 0.01 / 0.02 | 0.01 / 0.02 |

Measured directly: the small computer's whole life (12,709 events) is 56.0 MB
as (a) 4K, 38.6 MB as (b) 16K, 16.8 MB as (c) 4K, 15.4 MB as (c) 16K [V].

### Other computers (178 with receipts; 27 active in 14 days)

Per computer, the median active one (62 events per day) has 1,860 events in a
30-day window: 8 MB as (a). The largest ever (computer-6c50d6e5) has 76,044
events in its whole life, which caps any window at 0.34 GB (a), 0.23 GB (b) 16K,
0.10 GB (c). All 178 together, at 10,491 per day: 7 d 0.32 GB, 14 d 0.65 GB,
30 d 1.39 GB as (a); (c) 16K 0.09 / 0.18 / 0.38 GB [I from V].

### Reading the layouts

- (b) alone is worth nothing at 4 KiB pages (4,403 versus 4,409) and 31% at
  16 KiB. The first half of the previous night's finding holds for windowing
  too: a smaller row only pays after the page size rises.
- (c) as measured is a 70% cut at either page size, because it also removes
  event_json. It is a cut of the guest's copy of data that stays whole in the
  host's `tape-seg` event record and in the artifact store, so the guest holds
  only the ref. "Event bodies of large kinds" has no referent in the event
  table: the largest body is 1,880 bytes. The body that is large is the payload
  (mean 4.7 KB, tail to 767 KB), and it is already off the computer.
- What is left in (c) at 16 KiB, 913 bytes of table, is 12 hex digests of 64
  bytes (768) plus a few small fields; the digests are the remaining cost and
  would halve if stored as 32 raw bytes [I, not measured].

## 6. Sealed segment bytes per day to object storage

Raw bytes per day, from segment file sizes by mtime, all computers in
`tape-seg` (seg + idx files) [V]:

| Date | Paper seg (GB) | Paper idx (MB) | Fleet seg+idx (GB) | Paper events | Bytes per paper event |
|---|---|---|---|---|---|
| 10-21 (partial, from start) | 2.22 | 52 | 2.29 | 434,682 (day) | n/a (segments began mid-day) |
| 10-22 | 5.80 | 132 | 6.08 | 741,952 | 7,818 |
| 10-23 | 3.57 | 79 | 3.66 | 435,894 | 8,190 |
| 10-24 | 2.45 | 53 | 2.66 | 300,338 | 8,157 |
| 10-25 | 2.42 | 53 | 2.63 | 298,595 | 8,105 |
| 10-26 | 2.65 | 61 | 2.97 | 337,188 | 7,859 |
| 10-27 | 2.65 | 62 | 2.81 | 347,413 | 7,628 |
| 10-28 | 1.98 | 43 | 2.03 | 225,140 | 8,794 |
| 10-29 | 2.77 | 61 | 2.84 | 342,069 | 8,098 |
| 10-30 (to 03:15) | 0.41 | 9 | 0.42 | 40,190 | n/a |

Total 26.9 GB of paper segments + 0.605 GB of idx since 10-21; the fleet
total is 28.4 GB. Segments start 10-21; events before then are legacy one-file
pins and were not measured. The average over 10-23 to 10-29 is 2.64 GB per day of
segments plus 0.059 GB idx (2.2%) [V]. Idx files are rebuildable, so what
must be sealed is the segment.

Compression, whole 64 MiB segments, `zstd` single-thread [V]:

| Segment | Bytes | zstd -3 | ratio | zstd -9 | ratio |
|---|---|---|---|---|---|
| paper seg 27 (10-21) | 67,109,618 | 11,923,964 | 5.63x | 10,266,080 | 6.54x |
| paper seg 98 (10-22) | 67,109,690 | 12,846,374 | 5.22x | 11,260,681 | 5.96x |
| paper seg 234 (10-24) | 67,110,794 | 12,039,887 | 5.57x | 10,409,665 | 6.45x |
| paper seg 381 (10-27) | 67,109,197 | 12,655,744 | 5.30x | 11,040,122 | 6.08x |
| paper seg 463 (10-29) | 67,109,131 | 12,461,607 | 5.39x | 10,706,539 | 6.27x |
| small seg 33 (144 records) | 180,409 | 34,428 | 5.24x | 33,597 | 5.37x |
| owner seg 88 (612 records) | 771,095 | 141,533 | 5.45x | 137,921 | 5.59x |

Receipt rows (host `computer_event_append_receipts` equivalent), small
computer's 12,709 rows as NDJSON with the receipt and the nine chain digests,
2,755 bytes per row: zstd -3 6.64x (415 bytes per row), zstd -9 6.52x, zstd -19
7.24x [V]. This overstates the ratio against a mixed fleet table
(consecutive receipts repeat the previous head); last night's PostgreSQL
TOAST saw about 2:1 on single rows [V previous run], so the honest range for
sealed receipt rows is 2x (row at a time) to 6.6x (in a chained block) [I].

Sealed bytes per day for the paper (mean 326,662 events per day over the last
7 days; 8,139 raw bytes per event from section 2.3; 2,362 per receipt row)
[I from V]:

| What is sealed | Raw per day | zstd -3 per day | zstd -9 per day |
|---|---|---|---|
| Segment records only (event, payload, 2 pin receipts) | 2.66 GB | 0.49 GB (5.4x) | 0.43 GB (6.2x) |
| plus receipt rows, in the same sealed unit | +0.77 GB | +0.12 GB (6.6x block) to +0.39 GB (2x) | same range |
| Total | 3.43 GB | 0.61 to 0.88 GB | 0.55 to 0.82 GB |
| Peak day (741,952 events) | 6.04 GB seg + 1.75 GB receipts | 1.1 to 1.6 GB | 1.0 to 1.5 GB |

At 14 days a window of 4.31 M events also costs the host 35 GB of `tape-seg`
that has not yet aged out of the window if segments are only sealed at the
window edge: the sealing rate times the window is the on-host backlog before
offload, 14 d x 2.66 GB = 37 GB raw (7 d: 19 GB; 30 d: 80 GB) [I], on top of
the guest figures in section 5. Per year, the object store accumulates 179 GB
(zstd -3, segments only) to 320 GB (receipts at 2x) for the paper [I, mean rate
held, no growth]. Other computers together: 10,491 events per day x 8.1 KB =
85 MB raw per day, 16 MB compressed [I].

## Recommendation

The window is set by operations (crash-close, traces, the 7-day GC hold;
direction item 3), so the minimum is 7 days for every computer. Beyond that
the bytes decide, and they decide only for the paper.

| Class | Window | Why | Guest bytes, as today (a) 4K / (b) 16K / (c) 16K |
|---|---|---|---|
| Paper / always-busy (about 300 k to 330 k events a day, bursts to 742 k) | **7 days**; 14 days only after (c) at 16 KiB pages | 7 d is 2.2 M events: 9.5 GB today, 2.6 GB at (c) 16K (3.5 GB at the burst rate). 14 d is 4.3 M events and 19 GB as today (5.2 GB at (c) 16K). The 7-day GC hold is met by 7. | 9.5 / 6.5 / 2.6 GB (7 d); 19.0 / 13.1 / 5.2 GB (14 d) |
| Owner dev computers (0333 class: idle at 270 per day, test bursts to 90 k per day) | **30 days** | Observed worst 30 days is 428,502 events: 1.9 GB as today, 0.5 GB at (c). The steady state is 8 MB. | 1.9 / 1.3 / 0.5 GB (burst); 0.04 / 0.02 / 0.01 GB (steady) |
| Small d@e / ordinary user (62 events a day median, up to 5.4 k a day) | **30 days** (it holds all of a computer's life to date for most) | Whole life of the small computer is 56 MB as today. Largest ordinary computer ever is 76 k events: 0.34 GB as today. | 0.03 to 0.08 / 0.02 to 0.05 / 0.01 to 0.02 GB (30 d) |

How to read this:

- The window should be given in **events or bytes with a time floor**, not
  days alone: "at least 7 days, at most 2.5 M events" bounds the paper and
  leaves every other computer on its 30 days. The paper's 14-day figure moves
  by 0.4 M events between mean and burst; its day-to-day rate moves by a factor
  of eight (90 k to 742 k).
- Without (c) and 16 KiB pages the paper's 7-day window already costs 9.5 GB of
  guest SQLite, 3.2 times the (c) 16K figure; that is a larger share of the
  paper's 134.95 GB persistent disk (41.3 GB used at 02:36 [V, `/health`])
  than any other tape choice. (b) is not worth shipping alone at 4 KiB.
- Offload needs a segment path that keeps pace: 2.66 GB raw per day of
  `projection_batch_recorded` segments is already written on the host (not the
  guest). Sealing and uploading it compressed is 0.5 GB per day (0.6 to 0.9
  with receipt rows); a week is 3.4 GB to 6.2 GB of object storage, a year
  179 to 320 GB.
- Records are not what makes the paper big: 10,823 records a day at 3.2 KB is
  35 MB per day, and receipts retained for them are at most 3.3% of the receipts.
  Records can be kept forever at about 13 GB per year before indexes on the
  paper's present rate [I].

Open items this run could not close: the paper's `computer_event_index` was not
scanned (so its per-kind byte shares come from the owner and small
computers and from 164,000 sealed records, not from its own guest rows); the
record-to-event map (to replace the 3.3% bound by a count); whether the 10-22
step (742 k) is a recurring batch pattern or a one-off; and legacy (pre-10-21)
pins, which are outside `tape-seg`.

## Surprises, residuals, what went wrong

1. **A health dip and a load spike from my own query.** At about 03:08 an
   offline `dolt sql` on the loop-mounted copy of the paper's guest store
   (`select count(*), min, max` over `computer_event_index`) ran with host
   load 15 to 23 and the paper's `/health` returned 503 `store_stalled` and
   once 000 (timeout) and 5 to 7 second replies. The host had been at load
   8 to 10 before (firecracker for the paper at 250% CPU and live Store A at
   its memory limit, as last night). The offline process ended on its own within
   a few minutes; I issued the first `pkill` that missed. I did not repeat any
   scan of that table and cut all later reads to the owner and small computers
   and bounded record queries. After cleanup `/health` was `ready` at 03:13, 03:18, 03:22 and 03:27, 503 at
   03:24, and a burst of five probes at 03:25 saw four 503 then one 200 (the
   intermittent `store_stalled` last night saw at 1 in 8 to 1 in 15 at baseline).
   I cannot separate my load from baseline; the rule "stop if it degrades" was
   met once, and I stopped.
2. **The host Store A copy: first attempt wrote nothing.** The first
   aggregate (with the JSON length sums) read 61.8 GB from disk in 6.5 minutes
   and wrote an empty file (systemd `StandardOutput=file:`); the second, with
   counts only, took 66 s. Both used the reflink copy, not the live server.
3. **Disk.** Free space on `/` was 105 GB at 02:36 and 48 GB at 03:27 (used
   363 GB to 420 GB) and did not return after I deleted the 55 GB Store A copy, the
   three guest image copies and the scratch SQLite files at 03:24. The live
   Store A changed generation at 03:06 (its `noms/manifest`, `oldgen/` and a
   fresh archive file) and `btrfs filesystem du` reports
   109 GiB exclusive in `platform-dolt` against 55 GB apparent. I attribute the
   rise to Dolt garbage collection and btrfs copy-on-write while the reflink
   copy existed, but I could not verify it. **48 GB free of 475 GB is a risk the
   owner should look at; it is not caused by data I left behind.**
4. **Store A and the reset.** `dolt-reset-20261029T220557Z` exists; the live
   receipts count (6,102,549) is above the 6,065,689 of last night's snapshot
   by 36,860 in about 3 hours (about 300 k per day), consistent.
5. **Segments are not the whole tape.** Only `projection_batch_recorded` is in
   `tape-seg`; the other 0.7% of events remain legacy files, and everything
   before 10-21 is legacy. The sealing rates above are for batch events.
6. **Owner computer count mismatch with last night.** The owner computer 0333
   was called always-on last night because of 596,003 events; 428,502 of them
   fall in 09-30 to 10-29, almost all in 09-30 to 10-07, and its present rate is
   282 per day.
7. **Queries against the running servers:** `/health` of the paper (HTTP GET,
   about 25 probes) and none against the live Dolt servers.
8. **Cleanup done at 03:24:** three loop mounts unmounted, the whole
   `/var/lib/go-choir/scratch-tape-window` tree deleted (confirmed absent), my
   `/tmp` scripts on the host removed, no `systemd-run` unit, loop device,
   dolt, python or zstd process of mine remains. The scripts were kept off the
   repo.

## Heresy delta

`discovered`: (a) the guest tape's `event_json` is an envelope with no heavy
tail, so "move large-kind bodies" has nothing to move in the event table (the
tail is the payload, already host-side); (b) a read of the paper guest's event
table is enough to degrade its health, so any future tape scan must be sampled
by key range; (c) free space fell 57 GB during the run without an owner I could
find (item 3 above). `introduced`: none intended; item 1 is a transient health
dip. `repaired`: none.

Sources: the host receipt aggregates, parsed segment sizes and SQLite builds
were run from scratch files that are deleted; the figures above were copied
from them first.
