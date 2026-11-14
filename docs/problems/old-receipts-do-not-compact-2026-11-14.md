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

- Every reader expands format 2. Nothing writes it: in step 1 the backfill's
  write allowance is a constant false, no progress column is added and the walk
  is not restarted. A compact row the backfill may not write is kept by an
  active pass, not expanded.
- Bootstrap no longer creates `computer_event_append_kind_seq_idx` or
  `computer_event_append_receipt_id_idx`. This release creates and drops no
  index: both stay on the live table.
- The kind readers spell the partial index's predicate out and the
  latest-applied reader is three one-kind seeks; the head receipt lookups read
  by `(computer_id, sequence)`. All are tested index-backed with the old full
  kind index alone and with the partial one alone.

Rollback of step 1 to the main before it, on the live host: no format-2 row
exists until step 2 lands and both old indexes are present, so main's boot DDL
(`CREATE INDEX IF NOT EXISTS`) is a no-op and its readers run as they do now.
That holds only on the live host and only until step 2 has run: on a database
that step 1 bootstrapped and a restore then filled, main would build both
indexes without `CONCURRENTLY`; after step 2 has written format-2 rows, main
cannot read them.

**Step 2 (`lane/receipts-step2-r3`, on step 1).**

- The backfill writes format 2. Its progress is its own table,
  `computer_event_receipt_compaction_v2`, which no earlier release names. The
  shared row `computer_event_receipt_compaction` is no longer written. A release
  that does not know the walk (step 1, main) advances only the shared row, so it
  cannot move this walk past rows it kept; re-landing step 2 after a revert
  resumes where step 2 stopped and compacts the rows in the gap. The one thing
  an older release does that the walk must see is an expand pass (it writes full
  text over compacted rows): an expand row in the shared table newer than the
  walk's own progress restarts the walk.
- The walk does not start a pass while the index migration is running (one
  heavy job at a time on that table), and defers while the smaller of the free
  space of PostgreSQL's data directory and of the artifacts root is under 12 GiB:
  the estimate below is 2.4 GB of new heap, 0.8 GB of index churn and 7.7 GB of
  dead tuples that stay until a vacuum, 10.9 GB, with the WAL on top. An unknown
  free space does not hold it back. Every 100 batches it logs live and dead
  tuples, autovacuum count and last run, table size, WAL since the walk began and
  the free bytes.
- `StartReceiptIndexMigration`, started as corpusd's listener starts, builds
  `computer_event_append_kind_partial_idx` concurrently, reads it back valid,
  then drops `computer_event_append_kind_seq_idx` and
  `computer_event_append_receipt_id_idx` concurrently, one at a time. Each
  statement carries a three-hour `statement_timeout` and a one-minute
  `lock_timeout`; an invalid leftover is dropped and rebuilt; a process that does
  not get the advisory lock retries with backoff; the table's own schema names
  every index; each phase is logged.
- Owner's ruling N3 for the live table (one index at a time, stop if the busiest
  computer's append times rise). Nothing in corpusd measured the host append
  before (retrieval metrics time Store B reads, the guest appender times its own
  request, `pg_stat_statements` is not loaded), so the signal is the wall time of
  the serializable append transaction, recorded per computer in an in-memory
  ring (`receipt_index_health.go`); it is a read, not a gate on any append. There
  is no build until a baseline exists: the busiest computer's append p95 and
  rate over the last ten minutes with at least 20 appends, or a whole ten-minute
  window since the listener started with fewer (a quiet host has nothing to
  slow). Before the build, after the partial index is valid and before each
  drop, the baseline computer's own p95 over two minutes is compared with its
  baseline, and the migration waits while it is both twice the baseline and
  250 ms above it, or while its append count has collapsed to a tenth of what its
  baseline rate expects. The build itself is watched on a second connection:
  when the same reading holds for two polls its backend is cancelled, which
  leaves an invalid index that the next attempt drops and rebuilds after the
  wait. Waits log on change and every five minutes; they end by themselves when
  the appends recover; no setting turns them off.
- What a restart during the build costs: the connection closes and the build is
  cancelled; the next boot drops the invalid index and starts the two heap scans
  over. A deploy loop during the build repeats the whole cost each time. The
  samples live in memory, so each boot waits for its own baseline.

Rollback of step 2 to step 1: step 1 reads every format-2 row, its backfill keeps
a compact row it may not write, its Bootstrap creates no index, and its readers
need neither dropped index (the head lookups use the sequence unique key, the
kind readers work with the partial index). The partial index stays and is
harmless; nothing is re-created and nothing needs expanding. Re-landing step 2
resumes the walk (see the progress table above).

Rollback past step 1 (to the main before it, after step 2 ran): not a revert to
run blind. Main cannot read format-2 rows (run mode `expand` on the named
computers to completion first, which writes the exact stored bytes back) and its
boot DDL would build both old indexes without `CONCURRENTLY` inside a 60 s open
(create them concurrently by hand first).

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
  live table are estimates. The watch cancels a build on a sustained rise of the
  baseline computer's append p95 or a collapse of its rate; a rise shorter than
  two polls (one minute) or that the two-minute window dilutes is not seen, and
  a ring that holds fewer than ten minutes (above about 13 appends a second)
  gives a baseline from what fits.
- The 12 GiB deferral threshold is the record's estimate rounded up, not a
  measured need; the data directory's free space is read only where corpusd may
  stat it, and the artifacts root otherwise.
- Other computers' old rows were not sampled; the same walk covers them if they
  are named in `PLATFORM_RECEIPT_COMPACT_COMPUTERS`.
- Only receipts of kind EventHeadReceipt with this issuer are handled; a row of
  another spelling still stays full, as before.
