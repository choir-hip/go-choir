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

### What followed (all UTC, read-only from Node B)

- 16:39: the CI run of 90e567936 (a docs commit on top of 51667a3ce)
  deployed the same column code. It passed: the paper was deferred.
- 16:44:54: vmctl's wedge detector stopped the paper ("refused connections
  for 5m0s; stopped, it boots again"). The boot at 16:45:01 had no
  planned-restart marker, so it closed the open work as interrupted by a
  restart (CLAUDE.md, restarts end work), then started the same rewrite.
  Its listener did not come up either; by 16:51 it had read 21 GB.
- 16:53: the revert (f2c4a3ec3) deployed. It did not reach the paper: the
  new vmctl found it "outside the registry; reattach pending" (it was
  unhealthy when vmctl restarted), so the deploy neither refreshed nor
  stopped it.
- 16:59:09: the wedge detector stopped it again, "it boots again on resume".
  Its one automatic retry had been spent at 16:44:54 ("the next is allowed
  after 24h0m0s"), and API-key requests never boot a stopped computer
  ("api key computer realization unavailable"), so nothing would.
- 17:09:24: with the owner's go-ahead ("you can access node b as needed"),
  the session called `/internal/vmctl/resolve` on Node B for the paper's
  dedicated account, the path a visit takes (caller
  `operator.resume-paper`, the credential passed as a header file and
  never printed). vmctl admitted it (`admit_protected`) and booted it at
  17:10:07 on f2c4a3ec3. Boot closed the open runs as interrupted by a
  crash restart, as the restart rule requires; all 66 source schedules
  stayed active. The paper was down from 16:20 to 17:10.

The 24-hour retry window is the 2026-10-15 problem's item 2
([paper-computer-disk-full-outage-2026-10-15.md](paper-computer-disk-full-outage-2026-10-15.md)),
still unfixed: this is its second outage.

A second problem, then: **a rollback deploy cannot reach a protected
computer that hangs during boot.** Reattach needs a computer vmctl stopped
on its own restart (`internal/vmctl/host_capacity.go:155-157`); a
protected computer outside the registry stays as it is until the wedge
detector stops it. The rollback path that CLAUDE.md names (revert, then
deploy) therefore depended on the wedge detector's five-minute bound,
and each stop it made was unplanned, which closed the paper's open work.

Measured afterwards on 20,000 local rows: adding a plain nullable column
takes 8 ms and indexing it 94 ms; adding a stored generated column over
metadata takes 2.3 s and over the body 3.6 s; backfilling a plain column
costs about 0.4 ms a row.

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
4. A deploy refreshes a protected computer that is unhealthy outside the
   registry instead of leaving it: the rollback must reach the computer
   that the bad release broke.

## Fix design (superseded 2026-10-21: plain columns on og_objects)

**Superseded** by the rollback-safe side table in
[store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md](store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md)
(lane FP1, `docs/definitions/choir-launch-fp1-store-throughput-2026-10-21.md`).
The reasons the panel review gave against the plain-column design, as relayed
to the lane by the orchestrator (the panel's own text is not in this
repository, so treat the list as the lane's record of it, not a transcript):

1. **Silent staleness after rollback-then-forward.** An older binary's
   `INSERT ... ON DUPLICATE KEY UPDATE` never names a new column, so an
   updated row keeps its old derived value and nothing marks it; the
   `og_derived_v` marker would be left as it was. A side table that carries
   the `content_hash` it was derived from makes a stale row detectable.
2. **Indexes over a populated table are another full pass on the boot path**
   (item 5 below named the open question: one pass per index, holding the
   single connection, minutes on the paper). A side table and its indexes
   are created while empty, which is cheap at boot, and filled online.
3. **Completeness was unprovable.** A reader needs to know that every row of
   the kind is derived and still current; a per-row version marker on the
   table the older binary also writes cannot say that after a rollback.
4. **The columns change `og_objects`' schema and `SELECT *` content**, which
   the replay equivalence and the content witness hash. (A new table has its
   own cost there, handled in the superseding record's finding 3.)

The text below is kept as the record of what was proposed. Its count of
writers ("the two upserts, project.go, the two UPDATEs") was also short: there
are six SQL sites (see the superseding record's finding 1).


Measured: a plain nullable column adds in 8 ms; a BEFORE INSERT trigger
fills it, but a BEFORE UPDATE trigger does not fire on our
`INSERT ... ON DUPLICATE KEY UPDATE` upsert (the row kept its old value),
so the engine cannot maintain it for us. The columns are therefore filled
by the writers, from one shared expression.

1. **Columns.** Plain `VARCHAR(255) NULL` columns with the same meaning as
   the reverted generated ones: `og_meta_agent_id` (choir.run,
   metadata `$.agent_id`), `og_meta_capture_id` (choir.capture_label),
   `og_meta_doc_id` (choir.source_ref), `og_rec_addressee`
   (commitment_record, body `$.addressee`, guarded by `JSON_VALID`), and a
   marker `og_derived_v INT NULL`: the version of the derived-column set
   the row was written with.
2. **One expression, every writer.** A single Go helper renders each
   column's SQL expression over the statement's own parameters (kind,
   metadata, body) exactly as the generated column did, so Go never
   re-implements JSON semantics. Every `og_objects` writer uses it: the two
   upserts in `objectgraph/dolt_store.go`, `store/project.go`, and the two
   UPDATEs in `store/vocab_migrate_og.go`. The upsert's update arm copies
   `VALUES(col)`. A test fails if any `INSERT INTO og_objects` or
   `UPDATE og_objects` in the guest packages does not go through it.
3. **Online backfill.** After boot, behind the open gate, a background
   task fills rows whose marker is older than the current version, in
   canonical-id batches of a few hundred. It takes the engine mutex per
   batch only, logs progress (rows done, rows left, rate), and resumes
   from where it stopped after a restart because the marker is durable.
4. **Reads switch only when complete.** A column read is used when the
   column and its index exist and no row of that column's kind has an old
   marker; until then, the JSON path. The check is one indexed probe.
5. **Indexes.** Built after the backfill completes, one at a time, each
   timed and logged. Open question for the panel: an index build is one
   pass that holds the single connection; on the paper that may take
   minutes.
6. **Boot.** Boot only adds the plain columns (milliseconds). Nothing that
   scales with the table runs on the boot path, and any boot step that can
   pass a minute logs progress.
