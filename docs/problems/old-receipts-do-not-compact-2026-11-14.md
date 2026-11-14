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

**Step 2a (`lane/receipts-step2a`, on step 1): the writer and the walk.**
After three review rounds of an automatic index-swap controller inside corpusd
(docs/problems/receipts-step2-three-rounds-structural-assessment-2026-11-14.md)
the swap is no longer in the binary; it is a supervised act (step 2b).

- The backfill writes format 2. Its progress is its own table,
  `computer_event_receipt_compaction_v2`, which no earlier release names; the
  shared row `computer_event_receipt_compaction` is no longer written, and the
  tape purge deletes the new table too. A release that does not know the walk
  (step 1) advances only the shared row, so it cannot move this walk past rows it
  kept. The restart rule is conservative: any row of the shared table for the
  computer with `updated_at` newer than the walk's own row, in any mode, starts
  the walk over from zero. It costs one pass over rows that are already compact
  (they are kept) and needs no reading of what the older release did. Tested for
  expand, expand then active, expand then dry run, and active alone.
  A row whose `through_sequence` is past the computer's head is walked from
  zero: a defensive invariant with no known trigger (heads are never deleted and
  a tombstoned computer refuses appends), kept because progress past the head
  cannot describe the tape. A restart writes the row back to zero once,
  on purpose; every save of a pass
  after that is held never to move `through_sequence` backward in the same mode,
  so a second process or a pass that began earlier cannot undo another's
  progress.
- The walk does not depend on the state of any index (every reader works with
  either set of kind indexes) and the swap does not wait for it. It steps aside,
  at the start of a pass and after every batch, while another session holds
  SHARE UPDATE EXCLUSIVE on the receipt table (an index build or drop, a manual
  VACUUM or ANALYZE; an autovacuum worker is not counted), so that there is one
  heavy job at a time on the table. The view that names a build,
  `pg_stat_progress_create_index`, shows every column as NULL for another role's
  backend unless the reader has `pg_read_all_stats`, which the deployed role
  lacks while the swap runs as `postgres`; `pg_locks` is visible to every role,
  and so is `usename` (NULL for a background process), so those are read
  (tested from an unprivileged role). Progress is saved before it steps aside,
  the reason is logged when it changes, and "receipt compaction resumes" when it
  clears. It also defers for free space: under 12 GiB on the measured volume,
  read at the start of a pass and again after the 100th, 200th, 300th batch of a
  pass; the load line is written after the 1st, 101st, 201st batch. The deferral
  line is repeated every time the walk is held (a pass is ten minutes apart), so
  it is in any ten-minute journal window; it is the signal of the walk's state,
  because a load line is written only by a walk that is running and carries no
  state. If the
  build check itself cannot be read (the table does not resolve, the lock view
  fails) the walk runs and says so once, `build check unavailable: ...`, and again
  when it is readable. The estimate is 2.4 GB
  of new heap, 0.8 GB of index churn and 7.7 GB of dead tuples that stay until a
  vacuum, 10.9 GB, with the WAL on top. The measured volume is the smaller of
  PostgreSQL's data directory and corpusd's artifacts root; the deployed database
  role has no `pg_read_all_settings`, so `data_directory` is refused there and
  the volume measured is the artifacts root alone, which may not be the volume
  of the database. The log says so once, naming the directory. An unknown free
  space does not hold the walk back. Every 100 batches the load line gives live
  and dead tuples, autovacuum count and last run, table size, WAL since the walk
  began and the free bytes.
- Bootstrap creates `computer_event_append_kind_partial_idx` only when the
  receipt table has no rows (a fresh database), in one transaction: a transaction
  advisory lock (a second booting process waits for the first and finds the
  index; SHARE mode alone does not do this, it is compatible with itself and the
  second CREATE INDEX then fails on pg_class), a first look at the table that
  takes no table lock (a live table never queues), the table in SHARE mode with a
  ten-second lock timeout, a second look (a row appended between the first look
  and the lock is not missed) and the build. A table with rows is never touched,
  and nothing here can fail Bootstrap: the index is not needed for correctness,
  so a lock timeout or any error is logged and the index is left to the swap
  script. The live table's index swap is not done by any binary.
- corpusd logs one line a minute: the busiest computer's append count, p50 and
  p95 over the last minute and over the last ten
  (`corpusd append timing: 1m busiest=... n=... p50=... p95=...; 10m busiest=... ...`,
  and, when the busiest computer of the last minute is not the busiest of the last
  ten, `; 1m of <the ten-minute computer> ...`, so a reading can always be taken
  for the same computer). The
  signal is the wall time of the serializable append transaction, kept per
  computer in an in-memory ring; nothing in corpusd measured the host append
  before. It is an observation: no code gates on it, waits for it or cancels
  anything because of it. A restart empties it.

Rollback of step 2a to step 1: step 1 reads every format-2 row, its backfill
keeps a compact row it may not write, its Bootstrap creates no index, and its
readers need neither old index. The partial index and the v2 table stay,
harmless. Re-landing step 2a restarts the walk if step 1 wrote the shared row
meanwhile.

Rollback past step 1 (to the main before it, after step 2a ran): not a revert to
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
- The ring holds less than ten minutes above about 13 appends a second; the
  count in the timing line says how many appends it read.
- The 12 GiB deferral threshold is the record's estimate rounded up, not a
  measured need, and it applies to the artifacts root's volume on the live host
  (see above), which may not be the database's.
- Other computers' old rows were not sampled; the same walk covers them if they
  are named in `PLATFORM_RECEIPT_COMPACT_COMPUTERS`.
- Only receipts of kind EventHeadReceipt with this issuer are handled; a row of
  another spelling still stays full, as before.

## Procedure: the supervised swap of the live table's indexes

Owner's ruling N3: the indexes may be rebuilt or dropped on the live table
overnight, after review, one at a time, concurrently, stopping if the paper's
append times rise. That is a permission for an act an operator performs, once,
on one table, on one host, watching one number (see
`receipts-step2-three-rounds-structural-assessment-2026-11-14.md`). The act is
`scripts/receipts-index-swap`, which runs on Node B through local psql and reads
no secret. It needs step 1 and step 2a deployed (every reader works with either
set of indexes; corpusd logs the append timing line a minute).

**Before.**
1. The script is on Node B at the running deploy's commit: a docs-like push does
   not deploy, and the script is not in a closure; it is in the checkout the
   deploy resets to the deployed SHA (`/opt/go-choir`,
   `.github/scripts/node-b-ci-deploy`). Check that `git -C /opt/go-choir
   rev-parse HEAD` is the staging SHA and that `scripts/receipts-index-swap
   status` runs and reads sanely (both old indexes valid and ready, the table's
   sizes, no build running). The script lands in the same push as step 2a so it
   arrives with the deploy that has the timing line.
2. No deploy in flight and none planned for the night; no restart of corpusd
   since the pre-build reading (the append ring is in memory: after any restart,
   take the reading again).
3. The disk gate is the operator's `df` on the volume of PostgreSQL's data
   directory (the walk's own check measures the artifacts root, because the
   deployed role cannot read `data_directory`). Compute the floor from `status`:
   the build writes a new index of about the size of the rows it covers (the
   partial index, a few tens of MB on the paper) plus WAL of the order of that
   index and of the pages it reads (take twice the table's `heap_bytes` as a
   ceiling for a first look at WAL volume, not a prediction); with the walk
   running, add its estimate (about 11 GB) as the record above does. Proceed
   only if free space is above the floor by a wide margin; `status` prints
   `table_total_bytes`, `heap_bytes` and `indexes_bytes`, the numbers to use,
   not a constant from this record.
4. `scripts/receipts-index-swap watch` in a second terminal.

**The walk.** Step 2a's walk steps aside by itself while another session holds
the table for an index build or drop (it watches `pg_locks`). While it is held it
logs, at every pass (ten minutes apart) and whenever a pass steps aside,
`receipt compaction deferred: an index build or other maintenance holds the
receipt table...`, and `receipt compaction resumes` when it clears. After the build starts, wait for a
`deferred` line newer than the build's start (the journal window of `watch` is
ten minutes, and the line repeats at that cadence; a `receipt compaction load`
line newer than the build's start means the walk is still running) before relying on a reading: until then the walk may still be
writing and its I/O is in the append times. Name the walk's state when taking the
pre-build reading (running mid-pass, deferred, or finished: its last `receipt
compaction load` line). To take the swap with the walk out of the picture
entirely, run it before the first pass starts (two minutes after a boot) or after
the walk is done (`receipt compaction ... done=true` for the paper).

**The pre-build reading.** From the timing line
`corpusd append timing: 1m busiest=<id> ...; 10m busiest=<id> ...[; 1m of <id> ...]`:
the paper's computer id is the one the rule is about. Read the 10-minute figure
three times a minute apart; if the busiest computer is not the same id in all
three, take them again; the lowest p95 of one id is the pre-build reading P, and
that id is written down. Every later reading must be for the same computer
id: take it from `1m busiest=<id>` when that is the id, and otherwise from
`1m of <id>` (the line gives the ten-minute computer's own last minute); if
neither names it, take another reading a minute later. The cancel rule: that
computer's p95 over the last minute above the larger of 2 x P and P + 250 ms, on
two consecutive readings a minute apart.

**Order, one statement at a time.**
1. `build`: `CREATE INDEX CONCURRENTLY computer_event_append_kind_partial_idx ...`
   (the exact definition step 1's readers expect; a valid index of that name with
   another definition is refused with exit 6, nothing is changed). Expected 3 to
   10 minutes: two scans of the table's heap and a small index (the 0.8% of rows
   that are not projection batches). Watch the timing line and `status` (the
   build's phase and blocks).
2. When the partial index reads valid and ready and the paper's p95 is back at P
   (two readings a minute apart), `drop-kind-seq` (about 1.9 GB, seconds to a
   minute; it waits for transactions that can see the index).
3. Again two readings, then `drop-receipt-id` (about 2.3 GB). Both drops refuse
   (exit 3) unless the partial index is valid, ready and defined as expected.
   Dropped index files go back to the operating system at once.

**Cancel.** If the rule fires during a statement, run `cancel`. It signals only
an active backend running this script's create or drop on this table and says how
many signals it sent (zero is exit 1); a cancelled build leaves the partial index
INVALID, which every reader ignores and which the next `build` drops first.
**After a cancel**, wait until two readings a minute apart, for the same computer
id as P, are back under the rule, then run `build` again. At most three attempts
in a night; after the third, stop, leave the old indexes, and record the attempts
and the readings here. After both drops, a partial index that later turns INVALID
leaves no kind index at all (readers stay correct through the sequence unique
index, but slow): rebuild it promptly with `build`, not at the next night's
leisure. A restart of Postgres or a crash mid-build has the same
result as a cancel; a restart of corpusd does not touch the build (it is the
database's) but empties the timing ring, so take the pre-build reading again.

**Rollback.** Before the drops: nothing to undo (drop the invalid leftover with
`build`'s first step, or leave it). After a drop, if anything wants the old
index: `restore-old` recreates both concurrently with the definitions main's
Bootstrap gave them (the receipt id index first); no reader of step 1 or step 2a
needs them, so this is for a revert past step 1 only, and it takes a few minutes
each.

`build` and `restore-old` refuse (exit 3) while another session builds, drops or
vacuums the table, so a second terminal cannot wait out a minute to be told to
retry, or drop the first build's finished index as a leftover. The index the
script builds is owned by `postgres`; a later change of role would need a
superuser to drop it.

**What the script does not do.** It does not read or print the psql command, any
connection string or env file; it does not run two heavy statements at once; it
does not watch the timing for you. psql runs with `ON_ERROR_STOP` and every read
is checked, so a failing statement is exit 4 or 5, never a quiet success. Its
self-test, `scripts/receipts-index-swap-test`, runs against a throwaway
PostgreSQL and runs in CI (the `Receipts Index Swap Script` job, selected when
the script, its test, the workflow or the index definitions change); the script
and its test are not docs-like and not a deploy by themselves.

## The swap as run, 2026-11-14

Run by the lead from 22:01Z to 22:14Z under N3, claim C88 entered first.

- Before: staging and `/opt/go-choir` on c1ea7a418; corpusd up since 21:31:20Z;
  no CI or deploy in flight; 170 GiB free; the walk mid-pass on the paper,
  rewriting nothing (rows already in the new format). P = 3.33 ms (the paper's
  ten-minute p95, lowest of four readings), so the cancel line was 253 ms.
- `build` 22:04:31Z: valid and ready in under a minute, 9.4 MB. The walk logged
  `deferred` at 22:04:33Z. Paper one-minute p95 during it: 5.9 ms.
- `drop-kind-seq` 22:07:54Z and `drop-receipt-id` 22:11:01Z, each after two
  readings under 10 ms a minute apart, each exit 0.
- After: `indexes_bytes` 12,429,336,576 to 7,403,438,080; `table_total_bytes`
  33.78 GB to 28.75 GB; paper ten-minute p95 3.56 ms at 22:14Z; no error line
  in corpusd's journal since 22:04Z.
- The build estimate of 3 to 10 minutes was far too high: the partial index
  covers under 1% of rows.

Rollback is `restore-old`. What remains of the table is the heap (21.3 GB,
about 970,000 dead tuples awaiting vacuum) and three unique indexes (7.4 GB).

