# The derived-key validation re-reads og_objects bodies at every boot, for hours

Date: 2026-11-01. Status: fix built (derived-key marks bound to the projection
head, replay maintains the table, the three metadata families validate without
reading body overflow): [docs/design/derived-key-marks-2026-11-01.md](../design/derived-key-marks-2026-11-01.md).
Deployed proof pending.

## Problem

At every open the og_key_index families are unvalidated, by design: a valid
flag is never persisted, because a binary that does not maintain the side table
could leave it lying (internal/objectgraph/derived_keys.go, the header comment).
An online pass then compares the table with og_objects page by page. Its page
query reads updated_at and content_hash, which sit after body in the row, so
SQLite walks each row's body overflow chain. Every row of the four kinds is
read in full.

On the paper, that pass takes hours and never finishes between deploys, so the
guest keeps streaming state.db through its page cache. That is the cache that
busy_cold freed and saw refilled at once
(docs/problems/paper-busy-cold-freed-cache-refills-2026-11-01.md).

## Evidence (the paper's console log on Node B, metadata lines only)

- Pass durations: commitment_record.addressee validated 156,432 rows in
  1h09m (done 05:05Z); actor_wake_outbox.projected 174,762 rows in 59m
  (05:34 to 06:34Z, 531 re-derived); run.agent_id 79,463 rows at about 92
  rows/s; source_ref.doc_id 216,051 rows at about 40 rows/s.
- Boots tonight (startup phase=compact-rebase): 05:30, 06:41, 07:09, 07:24Z.
  Each boot started the outbox pass over. Three of the four passes never
  reached the second family.
- No "runtime: snapshot" line in the same logs: the snapshot producer did not
  fire, so it is not the reader.
- The 531 re-derived rows followed the 05:30 compact rebase, whose replay
  skips the side table (DerivedKeysTable comment: "The replay and witness
  extractors skip it"). Validation after a rebase or replay does real work.

## Constraints on a fix

- A persisted valid mark must be impossible to leave stale: a rollback
  binary, a replay, a rebase, or a restore must invalidate it.
- No new reads that scale with og_objects at boot.
- The JSON fallback stays the answer for anything unvalidated.

## Directions

- Persist the mark with a writer generation and the store's epoch; writers of
  this generation and later maintain the table in the same transaction, and
  the open trusts the mark only when the last og_objects writer was of a
  maintaining generation and no replay or rebase has run since.
- Or maintain og_key_index with SQLite triggers in the database itself, so no
  binary can bypass it, and validate only after a replay or rebase.
- Either way, make the validator read from an index or a column ordered
  before body, not the body overflow chain.
