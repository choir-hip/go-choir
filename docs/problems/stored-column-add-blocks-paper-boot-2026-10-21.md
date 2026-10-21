# Adding stored columns blocks the Autopaper computer's boot

Date: 2026-10-21. Status: **open**; rollback in flight.
Mutation class of the fix: orange (database schema and boot path).

## What happened

51667a3ce added four kind-scoped STORED generated columns to `og_objects`
(`og_meta_agent_id`, `og_meta_capture_id`, `og_meta_doc_id`,
`og_rec_addressee`) and their indexes, for the reads measured in
[research-more-slow-and-hidden-2026-10-21.md](research-more-slow-and-hidden-2026-10-21.md) E6.

The staging deploy (run 39632119465) refreshed the Autopaper computer
(vm-1669020a, critical_protected) at 16:20:16 UTC. The guest runtime
logged `actorruntime: boot follows a planned restart (platform update
51667a3ce...)` at 16:20:27 and then nothing. The deploy's ten-minute wait
for the served commit timed out at 16:30:15 (`curl_error_124`), and the
deploy failed. At 16:42 the paper still answered 502, and its health port
refused connections.

Evidence, read-only from Node B:

- The previous boot (c038df7c6, 15:45:56) logged `objectgraph dolt: record
  indexes ensured in 23ms` immediately after the same planned-restart line,
  then `computer event authority reconstructed (replay complete)` 57
  seconds later. The record-index step runs synchronously on the boot path,
  before the listener.
- At 16:42 the VM's process was alive: 16:38 CPU minutes in 22:31 elapsed
  (one of two vCPUs saturated), 68 GB read and 6.9 GB written, guest RSS
  8.3 GB.

## Belief

Each `ALTER TABLE og_objects ADD COLUMN ... STORED` rewrites every row of
the table, and each new index is another full pass. On the paper the table
is large (captures with bodies) and the guest has 2 vCPUs and 8.3 GB, so
the reads thrash the page cache. The 10-14 columns were added when the
table was much smaller, so the one-time cost measured then (about 50 µs a
row a column) did not predict this one. Hypothesis, not yet confirmed: the
`og_rec_addressee` expression reads `body` for every row, which costs
most. The rewrite's progress is not observable: nothing is logged between
the start and the end of the step.

## What it breaks

- Autopaper is down for as long as the rewrite runs (owner, 2026-10-14:
  "we need paper publishing 24/7, even after crashes").
- Any schema change that rewrites `og_objects` holds the boot for time
  proportional to the store, with no progress log and no bound.
- The rewritten chunks grow the guest store until garbage collection.

## Fix order

1. Roll back: revert 51667a3ce, so the paper boots the code that does not
   add the columns. Columns the rewrite already committed stay, unused:
   they are additive, and older binaries never name them. If the paper
   comes up before the revert deploys, keep it and cancel the revert's run.
2. Then a schema change to a large table never runs on the boot path: an
   online backfill in bounded batches that releases the engine between
   batches and logs its progress, with reads on the JSON path until the
   column is live. Alternatively, a plain column maintained by the four
   writers of `og_objects` (dolt_store.go PutObject and its batch path,
   store/project.go, vocab_migrate_og.go), backfilled the same way.
3. A boot-path step that can take longer than a minute logs its progress.
