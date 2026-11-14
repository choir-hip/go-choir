# Append receipt trim: compact the body, keep every row (2026-10-31)

Mission: docs/definitions/choir-lean-system-2026-10-30.md (stopping condition:
Node B bytes and PostgreSQL rows flat day over day). Class red: it rewrites
stored evidence (`computer_event_append_receipts.event_head_receipt_json`),
losslessly. Tier 2 review.

## Measured (Node B, read-only metadata, 2026-10-31 ~16:40 UTC)

- 6,653,235 rows; heap 13 GB, five indexes 4.79 GB, total 17 GB (2.6 KB a
  row). TOAST table 32 kB: nothing is compressed.
- `pg_stats.avg_width`: event_head_receipt_json 1,511 B; every other column
  together about 370 B.
- Indexes: digest_uq 1,193 MB, receipt_id_idx 1,173 MB, sequence_uq 869 MB,
  pkey 841 MB, kind_seq_idx 716 MB.
- Paper 5,598,508 rows (5,559,111 `projection_batch_recorded`); owner
  computer 597,600; d@e 21,196. projection_batch_recorded rows per day:
  224k to 367k this week (758k on 10-22).
- Each of the three computers has exactly one `snapshot_recorded` event
  (paper at sequence 5,234,768, 10-30).
- PostgreSQL 17.11, `default_toast_compression=pglz`, `plan_cache_mode=auto`,
  pg_repack not available. 164 GB free on `/`.

## Failure modes first: what a trimmed or slimmed row would break

The table is not a cache of idempotency keys. It is the host's only
sequence index of every computer's tape (sequence to event digest, pin
digest, the signed head receipt), and four readers walk it gapless from
sequence 1.

| # | Reader | What it needs | A deleted old row | A digest-only body |
|---|---|---|---|---|
| R1 | `existingComputerEventReceipt` (computer_events.go), the append's idempotent retry | the row by (computer, idempotency key), the full signed receipt to return | a late retry appends a duplicate (in practice retries are seconds old) | the retry cannot return the receipt |
| R2 | `EventsPage` (event_replay.go), the guest's boot replay, `snapshot.Scan`, `planRestoreTarget`, rematerialize, the replay-completeness probe, `finalizedEventBody` | every row after+1..min(head, after+n), gapless, with the receipt; the guest verifies every receipt's signature (`VerifyEventHeadReceipt`, appender.go:963) | **restore breaks**: `snapshot.Scan` and `planRestoreTarget` page the tape from sequence 0 to find snapshots and the target head, and EventsPage refuses a gap ("chain gap at sequence"); genesis replay (the fallback when no snapshot is usable, the probe's strongest evidence) is gone | replay refuses (no signature to verify) |
| R3 | `walkTapeChain` + `tapeReceiptsMatchSnapshot` (tape_sweep.go), the orphan sweep (`PLATFORM_TAPE_SWEEP_MODE=active`) | every row from 1 to head; payload pin digests out of the receipt to keep pin files live; row counts equal to the walked chain | the sweep refuses (gap), or, if relaxed, lists the event and pin files of the deleted rows as orphans and deletes them: **tape loss** | payload pins lost, same tape loss |
| R4 | bundle `readReceipts` (export from 0, import overlap check), `headReceipt` | every row with the receipt bytes "kept byte for byte" | whole-tape bundles have gaps; import refuses | bundles cannot carry the signed receipt (B2 signature checks) |
| R5 | checkpoints.go:63 (`event_head_receipt_id` lookup), :84, :92, :121, :138, :225, :253, :355, checkpoint_list.go:98 | the row of a checkpoint's accepted head (often a projection batch on the paper), its receipt, its kind, the genesis row | checkpoints of an old head fail authority | checkpoint binding fails |
| R6 | `EventsPage` credential epoch (`COUNT(*) ... event_kind='key_revoked'`) | every key_revoked row below the page | wrong epoch if key_revoked rows went (they are not trim candidates) | none |
| R7 | tape_purge.go (destroyed computers), `off-host-restore-drill.sh` (Dolt-era count) | whole computer, count | none | none |

No `scripts/` or `nix/` reader touches the table's contents; the Node B
backup copies whatever is stored.

So no row of the paper or the owner computer is proven unread: R2, R3 and R4
read every one of them. Deleting rows needs those readers changed first
(phase 2 below). The reduction the readers allow today is the body.

## What ships: the lossless compact body

Two thirds of the stored JSON respell the row's own columns (computer,
sequence, kind, event digest, request commitment, the event pin digest, the
receipt id) or constants (version, kind, issuer), as hex under JSON keys.
`computerevent.CompactHeadReceipt` stores only what no column holds, in
binary (`c1:` + base64): previous head, the next heads and commitments, the
pending ref, the payload pin digests, issued_at, the signer refs and raw
signatures, and a 16-byte check of the original bytes.
`ExpandHeadReceipt` rebuilds the exact stored bytes from it and the row: the
canonical spelling for format 1, PostgreSQL's jsonb spelling for format 2 (rows
written while the column was jsonb; see
`docs/problems/old-receipts-do-not-compact-2026-11-14.md`).

Why it is safe:

1. Lossless by construction: Compact returns a compact form only if
   Expand of it gives back the original bytes exactly; otherwise the row
   stays full JSON. The signature still verifies (tested).
2. Every reader expands through one helper (`scanExpandedAppendReceipt`,
   `appendReceiptRowColumns`; bundle `readReceipts`), and
   `TestAppendReceiptJSONReadersExpand` fails on any new raw reader of the
   column outside four allowed files.
3. Damage or a wrong row is an error, never other bytes (the check).
4. The backfill's UPDATE is a compare-and-set on the old body, and each
   batch re-reads and expands what it wrote against the original before it
   commits.
5. Rows, sequences, digests and counts are unchanged, so R2..R7 see the same
   chain; R1 returns the same signed bytes.

Modes (`PLATFORM_RECEIPT_COMPACT_COMPUTERS`, `id=mode,...`, a bare id is
active, switch `platform_receipt_compact`):

- `active`: new appends are stored compact; a paced backfill rewrites the
  older rows.
- `dry-run`: nothing is written; the backfill reads every row once, runs the
  round trip and records counts (`rows_rewritten` would-rewrite,
  `rows_kept` cannot-compact, `bytes_before`, `bytes_after`) in
  `computer_event_receipt_compaction` and the log. This is the differential
  gate: rebuilt bytes == stored bytes over every row of the computer.
- `expand`: rewrites compact rows back to full JSON and stops compact
  writes. The rollback.

Shipped: d@e `active`; the paper and the owner computer `dry-run`
(read-only proof). Widening the paper is a config edit (`=dry-run` to
`=active`) after its dry-run reports `rows_kept` about 0 and d@e has replayed
and restored on compact rows.

Backfill pacing: one goroutine in corpusd, two minutes after start, a pass
every 10 minutes; a batch is 500 rows in one short READ COMMITTED
transaction on the (computer_id, sequence) unique index (EXPLAIN on Node B:
Index Scan using computer_event_append_sequence_uq, LIMIT cost 752), then a
rest of at least 1 s and 4 times the batch's time. About 1 MB in memory a
batch. Progress is durable per computer and mode, so a restart does not
rescan; a mode change starts the walk over.

## Expected bytes

Codec on CAS-shaped receipts: 31 to 32 percent of the full bytes (tests:
12,796 to 4,053 over 9 rows; 1,592 to 557 with three payload pins). At the
paper's 1,511 B average, about 1,000 B a row saved.

- Per day at about 300k rows: today about 0.78 GB a day (0.6 heap, 0.22
  indexes). Compact: about 0.48 GB a day, -0.30 GB a day (-38 percent). The
  indexes (720 B a row) are not touched and become the larger share.
- One time: about 6.6 GB of heap (6.65M rows times about 1 KB).

How the space comes back. The backfill's UPDATEs leave the old versions
dead; autovacuum (or a manual `VACUUM computer_event_append_receipts`) turns
them into free space inside the file. Plain VACUUM does not shrink the file;
the freed 6.6 GB is reused by new rows, so the heap file stops growing for
about three weeks at the compact rate. To return it to the disk:

- `VACUUM FULL computer_event_append_receipts`: ACCESS EXCLUSIVE lock (every
  append on the host waits), rewrites about 6.5 GB of heap and rebuilds the
  4.8 GB of indexes (sorts spill to disk at maintenance_work_mem 64 MB;
  memory stays low), needs about 11 GB free (164 GB is). Estimated 10 to 20
  minutes; run it in a planned stop of corpusd (downtime is acceptable,
  Operating Tempo 4) after the paper's backfill is done.
- pg_repack (nixpkgs postgresql17Packages.pg_repack) does the same with a
  brief lock but is not installed and needs about twice the table free.
- Time partitioning does not help a table nothing deletes from; it matters
  only once phase 2 deletes ranges.

Costs of the backfill: each rewritten row is a non-HOT update (pages are
full), so it writes a new tuple and five index entries; PostgreSQL 14+
bottom-up index deletion keeps the unchanged-key versions from splitting
pages, but some index bloat is possible until VACUUM FULL. WAL for the paper
is on the order of the heap (about 13 GB over hours, recycled; archive is
off). Measure on d@e first.

## Considered and not taken now

- PostgreSQL compression (`toast_tuple_target=128`, lz4): measured on a
  local PostgreSQL 17.11 with 20k rows shaped like these: 1,534 B per value
  in all three tables (default, pglz+128, lz4+128). Rows under the 2,032 B
  TOAST threshold are never compressed, whatever the target. And random hex
  barely compresses under LZ without entropy coding.
- Partial `kind_seq_idx` (`WHERE event_kind <> 'projection_batch_recorded'`,
  about 700 MB, and its share of the daily index growth): its two readers
  (event_replay.go:39 key_revoked, checkpoints.go:138 genesis_imported) use
  it today (EXPLAIN: Index Only Scan). They bind the kind as a parameter;
  pgx caches named statements and `plan_cache_mode=auto` may switch to a
  generic plan that cannot prove a partial predicate, after which the count
  falls back to the sequence index over the whole chain on every replay
  page. Safe form: put the kind in the SQL text as a literal, then build the
  index CONCURRENTLY and drop the old one. A follow-up.
- `receipt_id_idx` (1,173 MB, 42 scans since stats reset): its readers
  (checkpoints.go:63, :84, :355) could look up by the accepted head digest
  (digest_uq) and compare the receipt id. A follow-up.
- Moving the receipt to the tape segment as a record: also lossless, but
  adds a tape write per append and a second read path; the compact body gets
  most of the bytes with no new storage.

## Phase 2: deleting rows (not built)

Row deletion of old finalized projection_batch_recorded rows (the guest
already windows its own index of them, store/event_index_trim.go) needs, in
order:

1. A per-computer floor F at or below the oldest kept snapshot root's
   `head_sequence` (`computer_snapshot_roots`, the newest
   `snapshot.MaxCandidates`), and no trim for a computer without a snapshot.
2. `snapshot.Scan` and `planRestoreTarget` stop paging from genesis: find
   snapshots by kind and the target by digest on the host (both indexed),
   then replay only from a snapshot at or above F.
3. `EventsPage` answers a page below F with an explicit "trimmed" refusal,
   never a silent gap.
4. The tape sweep keeps the files of trimmed rows: a per-range keep record
   (segment ids or a digest list) written before the delete, so nothing
   becomes an orphan. Without it the delete is tape loss.
5. Bundle export starts at F (a snapshot bundle, which it already supports).
6. Keep: every non-projection row, any row a checkpoint, route certificate
   or keep-set record names, the newest row per computer.

With group commit of acknowledged writes expected to cut the event rate
about five times, the compact body plus that may already bring the daily
growth near the target; phase 2 is worth its risk only if it does not.

## Rollback

Set the computer to `=expand`, wait for its log line `done=true` with
`rewritten` equal to the earlier compaction, then revert the commit. Reverting
the binary while compact rows exist breaks replay, retry and the sweep for
that computer (a JSON decode error on `c1:`), loudly. Only d@e is active.
