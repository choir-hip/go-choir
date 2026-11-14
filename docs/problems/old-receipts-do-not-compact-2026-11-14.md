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

Rollback of step 1 to today's main: no format-2 row exists and both old
indexes are present, so main's boot DDL (`CREATE INDEX IF NOT EXISTS`) is a
no-op and its readers run as they do now. Nothing to undo.

## Remaining error

- The walk starts at sequence 0 for the paper: 11.6M rows at 500 a batch with a
  rest of at least a second is on the order of ten hours or more, not measured.
- Rewriting 5.13M rows makes about 5 GB of dead space in the table. VACUUM makes
  it reusable by later rows; PostgreSQL returns it to the operating system only
  when the freed pages are at the end of the file (these are the oldest rows, at
  the front), so the file does not shrink without a rewrite.
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
no secret. It needs step 2a deployed (corpusd logs the append timing line a
minute) and step 1 deployed (every reader works with either set of indexes).

**Before.** No deploy in flight and none planned for the night (a restart of
corpusd does not cancel the build, which is the database's, but a deploy adds
load and a second thing to watch). `scripts/receipts-index-swap status`: both old
indexes valid and ready, no build running, the free space of the database volume
(`df`) at least the partial index's size (about 16 MB) plus the WAL of the build
(a few hundred MB). `scripts/receipts-index-swap watch` in a second terminal.

**The pre-build reading.** From the timing line, the paper's p95 over the last
ten minutes, read three times a minute apart; the lowest is the pre-build
reading P. The cancel rule is: p95 of the last minute above the larger of 2 x P
and P + 250 ms on two consecutive readings a minute apart.

**Order, one statement at a time.**
1. `build`: `CREATE INDEX CONCURRENTLY computer_event_append_kind_partial_idx ...`
   (the exact definition step 1's readers expect). Expected 3 to 10 minutes: two
   scans of the 18 GB heap, a 16 MB index (the 0.8% of rows that are not
   projection batches), WAL of the order of the index's size plus the full-page
   images of the pages touched. Watch the timing line and `status` (the build's
   phase and blocks).
2. When the partial index reads valid and ready and the timing is back at P (two
   readings a minute apart), `drop-kind-seq` (1.9 GB, seconds to a minute; it
   waits for transactions that can see the index).
3. Again two readings, then `drop-receipt-id` (2.3 GB). Both drops refuse unless
   the partial index is valid and ready. Dropped index files go back to the
   operating system at once, 4.2 GB.

**Cancel.** If the rule fires during a statement, run `cancel`. It signals only a
backend running this script's create or drop on this table and says how many
signals it sent (zero is exit 1); a cancelled build leaves the partial index
INVALID, which every reader ignores and which the next `build` drops first.
**After a cancel**, wait until two readings a minute apart are back under the
rule, then run `build` again. At most three attempts in a night; after the
third, stop, leave the old indexes, and record the attempts and the readings
here. A restart of Postgres or a crash mid-build has the same result as a
cancel.

**Rollback.** Before the drops: nothing to undo (drop the invalid leftover with
`build`'s first step, or leave it). After a drop, if anything wants the old
index: `restore-old` recreates both concurrently with the definitions main's
Bootstrap gave them (the receipt id index first); no reader of step 1 or step 2a
needs them, so this is for a revert past step 1 only, and it takes a few
minutes each.

**What the script does not do.** It does not read or print the psql command, any
connection string or env file; it does not run two heavy statements at once; it
does not watch the timing for you. Its self-test,
`scripts/receipts-index-swap-test`, runs against a throwaway PostgreSQL and is
runner-only: both files are developer-side paths (docs-like), so no CI lane runs
them; run it on a runner before changing either.
