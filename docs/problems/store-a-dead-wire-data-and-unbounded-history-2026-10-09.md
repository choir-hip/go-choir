# Store A carries dead wire data and unbounded Dolt history

Date: 2026-10-09. Owner direction: "We should investigate store a (it may
have some storage leaks as well)." Read-only investigation; operational
invariant O10 (declared bound per growth surface). Mutation class of any
fix: red (canonical event/control store).

## Evidence (staging Node B, 2026-10-09 ~01:15Z)

- `/var/lib/go-choir/platform-dolt/platform` is 22 GB, all of it in
  `.dolt/noms/oldgen` (GC'd chunks). Host root filesystem 80% used
  (96 GB free of 476 GB).
- Store A still holds the pre-split world-wire tables, last written
  2026-09-19 (the Store A/B split): `og_objects` 6.09M, `og_edges` 3.05M,
  `ingestion_events` 3.00M, `fetches` 2.38M, `items` 1.72M,
  `cycle_events` 170k, `processor_requests` 152k, `cycles` 25k, plus the
  publication/provenance tables. No code reads them from Store A since the
  split (`Store.corpus()` routes og/corpus tables to Store B).
- Live control tables are small by comparison: `computer_event_append_receipts`
  677k, `computer_lifecycle_receipts` 26k, `computer_file_roots` 5k.
- `dolt_log` has **86,021 commits**: every platform snapshot commits
  ("platform snapshot: append computer event …", "record file root …"),
  several per minute.

## Problem

1. **Dead data in the control store.** ~17M rows of abandoned wire data
   share a process and a disk with the canonical event heads.
2. **Dropping them frees nothing.** Dolt history retains every chunk ever
   committed; with 86k commits referencing the pre-split tables, `DROP
   TABLE` + `DOLT_GC` cannot reclaim them. Reclaiming needs a history
   rebuild (a fresh repo from current state).
3. **History grows without a declared bound.** One commit per snapshot
   batch, forever. The tape (event heads + CAS + receipts) is the
   authority; Dolt commit history in Store A is not the tape and has no
   stated retention purpose — but nobody has decided it is disposable.
4. `computer_event_append_receipts` grows per appended event with no
   declared retention (likely needed as idempotency/audit; bound unstated).

## Open decisions (owner/director)

- Whether Store A Dolt history has any authority or audit role. If not:
  declare it disposable, rebuild Store A from current state (drops dead
  tables and history), and bound future history (periodic squash or no
  per-snapshot commits).
- Retention class for `computer_event_append_receipts`.

Not done here: no tables dropped, no history rewritten. Store B (corpus)
teardown is tracked separately and was owner-approved 2026-10-09.

## Decision (owner, 2026-10-09)

"The dolt history currently doesn't have audit value, but it definitely will
once the system is in production." Decided:

- **Now:** rebuild Store A from current state (drops the dead wire tables and
  the 86k-commit history), after a full copy of the current directory kept
  for one week of clean running. History before 2026-10-09 is discarded on
  purpose. Runbook step after the Store B reset: copy → dump live tables →
  fresh repo → import → verify event heads and receipts → restart corpusd.
- **For production:** the audit authority is the tape (signed event heads,
  append receipts, content-addressed artifacts) plus the key-escrow
  transparency log; Store A Dolt history is a derived secondary record.
  Before production, push Store A history to a cold-storage Dolt remote on a
  schedule (bounded hot history, full cold history), and replicate the tape,
  content store and escrow off-host — see O22 in the register.
