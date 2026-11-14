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
