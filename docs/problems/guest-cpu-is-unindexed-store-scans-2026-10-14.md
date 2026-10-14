# The Paper's CPU Is Unindexed Store Scans, Not Agent Work

Date: 2026-10-14. Found by a 60-second guest CPU profile of the paper
computer's main `autoputer` process at 16:46 UTC
(`docs/evidence/computer-cpu-guest-profile-2026-10-14T1646Z.txt`, taken
by `scripts/cpu_efficiency_probe.py --profile`), with a host sample over
the same window. Status: documented; the fixes are levers C1 to C4 in
[computer CPU efficiency](../reports/computer-cpu-efficiency-2026-10-14.md).
Fix commits cite this record.

## Evidence

- The paper's Firecracker process used 1.29 cores in the window: vCPU
  guest mode 1.19, KVM halt polling 0.07.
- The main daemon used **1.17 cores**, 98% of guest time. All other guest
  processes, including the 12 Yaegi workers, used 0.03.
- 34 model calls in the window (31 a minute over the 20 minutes to
  16:50): **2.27 CPU-seconds per model call**.
- **73%** of the daemon's CPU is under `database/sql.(*Rows).Next`, inside
  the embedded Dolt engine. **40%** is under `encoding/json.Unmarshal`.
- **52%**: `store.listInboxRecordsBy` → `ogListAllByMetadata` →
  `objectgraph.(*DoltStore).ListObjectsByMetadataPage`. It filters
  `JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?)) = ?` over every
  `commitment_record` (`internal/objectgraph/dolt_store.go:765-770`).
  - **44%** of it is `recheckRecordsInbox` → `closeRoundsOfEndedRun` →
    `OpenRounds` → `ConversationState`, at the end of every desk run.
    That is 2 scans for the inbox's conversations, then one scan per
    conversation (`internal/store/inbox.go:850-902`).
  - **8%** is `loadCellRecords` → `ListInboxRecords` per cell.
- **11%**: `actingCommitmentPackForDesk` → `ListCommitmentClaimRecords` →
  `ListObjectsByOwnerAndBody`. It runs per cell, as a `JSON_EXTRACT` on
  the body of every commitment record of the owner
  (`internal/store/commitment.go:122-150`,
  `internal/objectgraph/dolt_store.go:845-880`).
- Model request build and reply parse, transcript reads and canonical JSON
  are each under 0.5% or about 1%.

## Reading

- The cost is set by the number of stored commitment records, not by the
  work done. Captures add about 270 an hour at 28 sources.
- Hypothesis, unmeasured: the paper's CPU per model call rises over the
  computer's life.
- An indexed replacement half exists. `metadataIndexColumns` (generated
  columns for `run_id`, `trajectory_id`, `channel_id`,
  `dolt_store.go:150-200`) is used by `ListObjectsByMetadata`
  (`:686-699`).
  - It is not used by the paged variant that `ogListAllByMetadata` calls
    (`internal/store/graph_store.go:138-165`).
  - It has no column for `inbox`, `sender_inbox`, `conversation_id` or
    the body's `directive`.

Same substrate as:
- the Texture list latency after idle (`internal/server/debug_pprof.go`
  header);
- the heap under `AppendNewPayload`
  (`docs/problems/autopaper-intake-defects-2026-10-14.md`).

All three are the object graph used as a JSON document store with
scan-and-filter reads under one engine mutex. The next fixes are
substrate-level: indexed access paths, not per-caller patches.

## Options

1. Wire the generated columns into the paged read and add the missing
   fields (C1). Then fold rounds only for the conversations an ended run
   touched (C2). Then claims by index (C3) and the cell inbox as a range
   read (C4). Orange (queries, guest schema at boot); S each.
2. A typed projection table for commitment records (inbox, conversation,
   seq, directive), maintained on the tape's projection path. Larger (R),
   and it subsumes 1.

## Chosen repair and its failure modes (written before the code)

Option 1, in one slice per lever. Lever names follow the fix mission:
C1 indexes, C2 run-end check, C3 claims and cell inbox, C4 host halt
polling, C5 event sequence. (The report's own table numbers them C1, C2,
C3+C4, C5 and C6.) Mutation class: orange (database queries, guest schema
at open); C5 is red (Trace/evidence: the event sequence); C4 is a host
kernel parameter (orange/red, Node B only).

Probed on the embedded Dolt engine this repo links
(go-mysql-server 2026-07-15) before writing the code:

- A `VARCHAR(255)` generated column **refuses the insert** of a row whose
  value is longer (`incompatible type for default value`). An unbounded
  JSON value in a new generated column would turn into a write failure.
- A generated column over `CAST(body AS JSON)` refuses the insert of a row
  whose body is not JSON. The existing three columns read `metadata`, which
  every write already parses, so they are safe.
- `ORDER BY canonical_id LIMIT n` over an index `(object_kind, col,
  canonical_id)` is still a TopN over the whole remaining range. Ordering by
  every index column (`ORDER BY object_kind, col, canonical_id`) removes the
  sort and the page reads only its rows. The same rule gives a reverse index
  walk for `ORDER BY ... DESC` on every index column.
- `CAST(<json> AS SIGNED)` never errors (a string gives 0, a boolean 1).

Failure modes the code and tests must close:

1. **A new column breaks writes.** A long value or a non-JSON body makes
   inserts fail. Repair: the new columns are restricted to
   `choir.commitment_record` rows (`CASE WHEN object_kind = ...`), the text
   columns are `LEFT(..., 255)`, and the body column is guarded by
   `JSON_VALID`. A lookup value of 255 characters or more keeps the old
   JSON path, so `LEFT` never changes a match.
2. **The indexed read returns different rows or order.** The column must
   be the exact expression the old query evaluated, collation included
   (trailing spaces, case, JSON `null`, numbers, missing fields). Repair:
   equivalence tests run the old query and the new one over randomized
   fixtures and compare rows and order.
3. **The planner ignores the index or sorts the whole range** (retrieval
   research §4 G). Repair: a plan-shape test per new query asserts the
   index name, no full-table access, and no TopN or Sort where the index
   order is meant to serve it, so a planner change fails CI.
4. **The migration bricks or stalls a booting computer.** Repair:
   additive `VIRTUAL` columns (no table rewrite), `CREATE INDEX` only when
   `information_schema` says absent (idempotent, the existing mechanism),
   any DDL failure leaves that field on the old JSON path. No tape event is
   written or rewritten: `og_objects` is a projection. The store schema
   epoch (`storeschema.Version`) stays 1: older binaries ignore the extra
   columns, so a rollback needs no migration.
5. **The run-end check misses a round it used to close (C2).** The new
   check reads the conversation heads, the cache that `planInboxRecords`
   writes in the same compare-and-set batch as every record, and that
   already decides whether a closure commits (`stepConversation` reads the
   head). Repair: a randomized test commits casts, reports, finals and
   closures and compares the fold-based `OpenRounds` filtered by recipient
   with the head-based read; `VerifyInboxHeads` stays the drift check.
6. **The cell inbox range skips a record (C3).** The range is read on the
   metadata `inbox_seq`, which `planInboxRecords` writes from the same
   value as the body's. Repair: the same Go filter and sort run on the
   fetched rows, and an equivalence test compares every (after, through]
   window with the old read.
7. **The claim pack changes (C3).** The newest-256 window moves from a
   TopN to a reverse index walk. Ties on `updated_at` (second precision)
   were ordered arbitrarily before; the index orders them by canonical ID.
   Repair: equivalence test on distinct timestamps; the tie order is noted
   as a refinement, not a change of the set outside ties.
8. **The in-memory event sequence duplicates or skips (C5).** Repair: one
   counter per (run, owner) under `eventMu`, seeded once per process from
   the store (the same bounded read as today), so it is correct across
   restarts; concurrent appends to one run, and reopen, are tested. Every
   `choir.event` with a non-zero `Seq` is written through `AppendEvent`
   (the lifecycle batch path writes `Seq` 0), so the process is the single
   writer of the sequence, the same assumption `StreamSeq` already makes.

## Found while building the repair (2026-10-14, before the fix commits)

Two findings change failure mode 4. Evidence: local probes on the linked
embedded Dolt and the store benchmark
(`internal/store/store_scan_cost_test.go`, 40,269 `og_objects`, Apple M1
under a host load average of 50 to 120, so CPU time, not wall time, is the
measure).

1. **VIRTUAL generated columns are unusable here.** With an indexed
   VIRTUAL column, `INSERT ... ON DUPLICATE KEY UPDATE` (every `og_objects`
   write) panics in the engine: `index out of range [12] with length 12`
   in `prollySecondaryIndexWriter.keyFromRow`. Without an index the column
   reads back empty after an upsert. So the record columns are STORED, as
   the three legacy columns are (C1 commit `bc3b08b5e`).
2. **A STORED column costs a table rewrite on a store with rows, once per
   column.** Measured: about 50 µs CPU per row per column (adding one
   column to 40,000 rows: 2.1 s CPU; four in one `ALTER`: 8.1 s, so the
   engine rewrites once per column, not once per statement; a nullable
   column with no default is instant, 4 ms). The five record columns took
   16.2 s CPU and 51 s wall at 40,269 objects. The owner's computer had
   205,966 `og_objects` on 10-09
   ([guest store history bloat](guest-store-history-bloat-and-memory-shape-2026-10-09.md));
   at that size the migration is estimated at 50 to 60 s CPU, and the
   paper may hold more.
   - `store.Open` runs before anything answers `/health`
     (`internal/autoputer/run.go`: the server starts after the store and
     runtime are built). The in-place updater's health probe gives a guest
     that refuses connections 30 one-second attempts
     (`internal/updater/runtime.go`), so a migration inside `Open` on a
     large store **would roll the release back**, on every attempt.
   - Repair: `EnsureSchema` adds the record columns only to an empty
     table (a fresh store or a replay from scratch: free). A store with
     rows gets them from `Store.EnsureObjectGraphIndexes`, which the boot
     calls after the tape replay, behind the replay health gate, with the
     gate's `tick` as a two-second heartbeat (the vocabulary migration's
     pattern; vmctl and the updater treat a changing `progress` as
     liveness, bounded by the updater's 15-minute cap). Until it finishes,
     every read keeps its old JSON path, so correctness never depends on
     the migration. A failure is logged and the boot continues.
