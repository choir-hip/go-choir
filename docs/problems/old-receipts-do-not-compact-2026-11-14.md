# Old append receipts never compacted: they are stored as jsonb text (2026-11-14)

Status: cause found and fixed in code (lane `lane/receipts-first-steps`, not
deployed). Mutation class of the fix: **red** (receipt bytes are evidence).

## Problem

About 5.13M rows of the paper (computer
`computer-1355b83bced1613eb048ee28d66688a8`, sequences 1 to about 5.13M,
written before about 2026-10-30T08:36Z) still hold the full 1,5xx-byte receipt
JSON. The compaction table says the paper is `active` through its head, so
the 10-minute backfill never revisits them. The rows hold about 5 GB of heap
that compaction should have turned into about 470 B each.

## Evidence

Read-only, Node B, structure only (no signature, key or token value read):

- Rows 1,000, 100,000, 3,000,000, 5,000,000 and 5,100,000 are 1,504 to 1,508
  bytes and start with `{`; rows 5,130,000 and 5,200,000 are 467 to 469 bytes
  and start with `c1:`.
- Every column the compact form takes from the row (computer id, sequence,
  kind, event digest, request commitment, receipt id, first pin) equals the
  JSON's field in the old rows too. Top-level keys, their types and lengths
  are the same twenty as in a new receipt; one signer; two pins.
- The text is not canonical JSON. Masked of long values, row 3,000,000 reads
  `{"issuer": "corpusd", "sequence": N, "issued_at": "...", "event_kind": ..., "receipt_id": ..., "computer_id": ..., "event_digest": ..., "receipt_kind": ..., "previous_head": ..., "signature_set": [{"key_id": ..., "signature": ..., "signer_domain": ...}], "receipt_version": 1, "required_signers": [...], ...}`:
  a space after every `:` and `,`, and keys ordered by length and then bytes
  (issuer, sequence, issued_at, event_kind, ..., effective_state_commitment).
  That is PostgreSQL's `jsonb` text output. The canonical form
  (`computerevent.CanonicalJSON`) has no spaces and sorts keys bytewise. The
  rows were written when the column was `jsonb` and kept their spelling when it
  became `TEXT`.
- `CompactHeadReceipt` (`internal/computerevent/receipt_compact.go`) proves its
  output by expanding it and comparing with the input bytes. Expand rebuilds
  the canonical spelling, so for these rows the comparison fails and the row is
  returned as not compactable; the backfill counts it as `kept` and moves on
  (`receipt_compaction.go` batch). That is the large `rows_kept` the census
  noticed (10,421,477 against 518,320 rewritten).
- Fix checked against the real rows: 337 rows spread over sequence 1 to
  5,130,000 were read from Node B into a local throwaway test (no value
  printed, file deleted). Before the change CompactHeadReceipt refused every
  full one; after it, 335 of 335 full rows compact and expand back to the
  stored bytes exactly (the other two were already compact).

## Belief

The cause is the stored spelling, not a field of another shape and not a
non-exact signature round trip. The signature stays valid because every reader
parses the text and recomputes the canonical payload; none verifies the stored
bytes. High confidence (0.95): the structure and the code agree, and the real
rows round-trip.

## Fix, in two landings

A second compact format (format byte 2 inside the same `c1:` prefix) carries
the same fields and expands to the jsonb spelling. `CompactStoredHeadReceipt`
tries the canonical format first and returns the jsonb one only when its
expansion is the stored bytes exactly; anything else stays full. Nothing is
respelled, so the tape sweep's reference chunks stay valid (C9) and no writer
is added.

Two reviewers found that landing it in one step breaks a revert two ways: a
binary from before it fails closed on a format-2 row, and a binary from before
it re-creates the two indexes the same work drops, without `CONCURRENTLY`, in
a 60 s boot that blocks appends. So:

**Step 1 (`lane/receipts-step1`), the safe revert target.**

- Every reader expands format 2. Nothing writes it: the backfill's write
  allowance (`receiptCompactionAllowJSONB`, from `receiptCompactionCodec`, still
  1) is false, `codec_version` is not added and the walk is not restarted. A
  compact row the codec may not write is kept by an active pass, not expanded.
- Bootstrap no longer creates `computer_event_append_kind_seq_idx` or
  `computer_event_append_receipt_id_idx`. This release creates and drops no
  index: both stay on the live table.
- The kind readers spell the partial index's predicate out and the
  latest-applied reader is three one-kind seeks; the head receipt lookups read
  by `(computer_id, sequence)`. All are tested index-backed with the old full
  kind index alone and with the partial one alone.

Rollback of step 1 to the main before it: no format-2 row exists and both old
indexes are present, so main's boot DDL (`CREATE INDEX IF NOT EXISTS`) is a
no-op and its readers run as they do now. Nothing to undo.

**Step 2 (`lane/receipts-step2`, on step 1).**

- The backfill writes format 2 (`receiptCompactionCodec` 2). The progress row
  gains `codec_version`; a pass saved by codec 1 starts over, so the 5.13M old
  rows are visited. Every 100 batches the pass logs live and dead tuples,
  autovacuum count and last run, table size and the WAL written since the walk
  began.
- `StartReceiptIndexMigration` builds `computer_event_append_kind_partial_idx`
  concurrently, reads it back valid, then drops `computer_event_append_kind_seq_idx`
  and `computer_event_append_receipt_id_idx` concurrently, one at a time. Each
  statement carries a three-hour `statement_timeout` and a one-minute
  `lock_timeout`; an invalid leftover is dropped and rebuilt; a process that
  does not get the advisory lock retries with backoff; the table's own schema
  names every index; each phase is logged (waiting, building, valid, dropping,
  dropped).
- Owner's ruling N3 for the live table (one index at a time, stop if the busiest
  computer's append times rise) is a wait before the build, after the partial
  index is valid and before each drop: the migration reads the busiest
  computer's append p95 over two minutes and compares it with the p95 it read
  over ten minutes before it began; it waits while the p95 is both twice the
  baseline and 250 ms above it. Nothing in corpusd measured the host append
  before (retrieval metrics time Store B reads, the guest appender times its own
  request, `pg_stat_statements` is not loaded), so the signal is the wall time of
  the serializable append transaction, recorded per computer in an in-memory
  ring (`receipt_index_health.go`). It is a read, not a gate on any append;
  it is lost at a restart, and then there is no baseline and no wait. The wait
  ends by itself when the appends recover; no setting turns it off.

Rollback of step 2 to step 1: step 1 reads every format-2 row, its backfill keeps
a compact row it may not write, its Bootstrap creates no index, and its readers
need neither dropped index (the head lookups use the sequence unique key, the
kind readers work with the partial index). The partial index stays and is
harmless. Nothing is re-created and nothing to expand.

Rollback of step 2 or step 1 to a binary from before step 1 (not planned): run
mode `expand` on the named computers to completion first, which writes the exact
stored bytes back (about as long as the walk), and create the two old indexes
concurrently by hand before it boots; its boot DDL would otherwise build them
without `CONCURRENTLY` inside a 60 s open.

## Remaining error

- The walk starts at sequence 0 for the paper: 11.6M rows at 500 a batch with a
  rest of at least a second is on the order of ten hours or more, not measured.
  The load lines in the log are how the first hour is read.
- Dead space (a reviewer's arithmetic, not measured): each of the 5.13M updates
  rewrites a full page's worth of tuple, so almost none can be HOT. Expect about
  7.7 GB of dead heap tuples (5.13M x 1.5 KB), about 2.4 GB of new heap (5.13M x
  470 B) and about 0.8 GB of index churn (a new entry in each of the primary key,
  the sequence and the digest indexes for each updated row). VACUUM makes the
  dead space reusable by later rows; PostgreSQL returns it to the operating
  system only when the free pages are at the end of the file, and these are the
  oldest rows at the front, so the file does not shrink without a rewrite.
- What triggers vacuum: the default `autovacuum_vacuum_scale_factor` of 0.2 on
  12.7M rows means one run per 2.5M dead tuples, so about twice during the walk;
  until it runs the dead space is not reused and the table grows by the new heap.
  Proposed, not applied: `ALTER TABLE computer_event_append_receipts SET
  (autovacuum_vacuum_scale_factor = 0.02)` for the duration of the walk, which
  vacuums about every 250k rewrites. It is a host tuning with its own I/O cost
  on an 18 GB table, so it is the owner's call after the first hour's load lines
  (dead tuples against autovacuums) show whether the default keeps up; the
  migration does not apply it.
- The index build time (about 3 to 10 minutes) and its effect on appends on the
  live table are estimates; the N3 wait is how the build is held to the owner's
  ruling.
- Other computers' old rows were not sampled; the same walk covers them if they
  are named in `PLATFORM_RECEIPT_COMPACT_COMPUTERS`.
- Only receipts of kind EventHeadReceipt with this issuer are handled; a row of
  another spelling still stays full, as before.
