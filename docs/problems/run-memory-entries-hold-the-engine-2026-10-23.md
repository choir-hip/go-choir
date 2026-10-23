# ListRunMemoryEntriesForAgent holds the paper's one write connection (2026-10-23)

Date: 2026-10-23, 09:00 UTC. Found by: FP0 worker, lane `fp0-run-memory-hold`
(investigation only; nothing is fixed here). Status: open; the first fix (shape 2) and an observation log are built
(section "What was built"); the rest stay open. Mutation class of this record: green (docs) plus one yellow
test (`TestRunMemoryHoldCensus`). Mutation class of the fixes named below:
orange (a read shape, an online index) and red (anything that changes where
the durable inbox cursor lives).

Parent record (the census that named this holder, 49 of 97):
[boot-reads-with-the-seed-shape-2026-10-23.md](boot-reads-with-the-seed-shape-2026-10-23.md)
(on `origin/lane/fp0-boot-read-census`, section "Not asked, seen"). Claims
are marked **[V]** verified (a plan, a measurement, a log line or the code,
named beside it) or **[H]** hypothesis.

## The question

Between refreshes the paper answers `/health` with 503 `store_stalled` for
about 1.3% of requests, in bursts of 1 to 4 a minute. Which function holds
the store when it does, what is its query, how often does the paper call it,
what does it cost as the data grows, and do its stalls line up with the 503
bursts?

## Answer

`Store.ListRunMemoryEntriesForAgent` is the top named holder (52 of 101
holder lines in 98 stall snapshots, 53% of snapshots) and the 503 bursts in
the two D9 probe files line up with it: 7 of the 8 steady-state 503s of the
after file, and 17 to 19 of the 26 503s of the before file, fall in a minute
whose snapshot names it. Its query reads every run-memory row of the desk
agent through an index that does not carry `kind`, materialises each row's
text columns, filters `kind` above the index and sorts in memory. It runs
twice per desk cell on the single write connection (`s.db`, one connection),
so `/health`'s three reads queue behind it. At the sizes a local store can be
seeded to it holds the connection for 12 ms (1,000 cursor rows of 5,000 desk
rows) to 850 ms (60,000 of 300,000 desk rows), growing linearly with the
desk's rows and not with the cursor rows it returns. The paper's 1 to 4 s
episodes are 20 to 100 times the warm local number at any size the table can
plausibly have; the cause of that gap is not settled here (cold chunk reads
and zstd decompression are what the stacks show). **[V]** (code, plan,
measurements, console) **[H]** (the cold-read cause).

| claim | status |
|---|---|
| It is the most frequent named holder: 52 of 101 holder lines, 53% of snapshots, 5 console logs, 01:05 to 08:42 UTC | **[V]** console |
| The 503 bursts of both D9 probe files line up with its snapshots | **[V]** join below |
| The holder is the write pool's one connection, not the objectgraph engine mutex | **[V]** code + pool stats |
| The plan walks the whole desk (owner, agent) index range and filters `kind` above it | **[V]** EXPLAIN |
| Hold grows linearly with the desk's row count | **[V]** local, warm |
| A kind-leading index makes it 4.5 to 7 times cheaper, a narrow projection on top of it 11 to 15 times | **[V]** local, warm |
| One desk cell calls it twice (start, commit); about 28 tool iterations a minute on the paper | **[V]** code, **[H]** rate |
| The paper's long holds are cold chunk reads, not CPU on a warm cache | **[H]** |
| The hot desk on the paper has hundreds of thousands of rows | **[H]** |

## The query and its call sites

**[V]** `internal/store/run_memory.go:289`:

```sql
SELECT entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq,
       kind, role, message_json, summary, first_kept_entry_id,
       tokens_before, reason, model, details_json, created_at
  FROM run_memory_entries
 WHERE agent_id = ? [AND owner_id = ?] [AND kind IN (?)]
 ORDER BY created_at ASC, seq ASC
```

through `s.db` with the rows scanned and each `details_json` decoded inside
the same call (`scanRunMemoryEntry`), so the connection is held for query,
iteration and decode together. Its only production caller is
`LoadInboxCursorForDesk` (`internal/agentcore/rlm_reduce.go:516`, passing
`kinds = [rlm_inbox_cursor]`), which takes the maximum cursor over the desk's
cursor rows for one channel. The three call sites of that wrapper:

| site | when | per |
|---|---|---|
| `rlm_reduce.go:665` `rlmReductionForCall` | start of a capsule (engineering) cell | cell |
| `rlm_reduce.go:732` `rlmReductionForDeskCall` | start of a desk cell (management, texture, research), from `desk_go_eval` (`tools_desk.go:549`) | cell |
| `rlm_reduce.go:891` `recoverPartialActCommit` | first line of every `commit` (`rlm_reduce.go:817`, from `tools_desk.go:667` and `tools_capsule.go:664`), before it knows whether any semantic act has a prior commitment record | successful cell |

**Trigger. [V]** per desk cell, twice: not per turn, not per wake. The third
site loads the cursor and only uses it after the loop finds an existing
commitment record (`exists` true, `rlm_reduce.go:903` onward); in the common
case (a fresh act) the read is unused. Other callers: the test
`rlm_reduce_test.go:516` only.

**Rate on the paper. [H]** The console's `tool loop: iteration N, executed 1
tools` line is logged per tool execution of any agent; over `console.log`
08:08 to 08:43 UTC it appears 1,027 times in 36 minutes (28.5 a minute), with
`dispatcher: ... slots=8/8 due_agents=52..57` (eight concurrent activations,
57 agents due). If every one of those were a desk cell the paper would call
the function about 57 times a minute (once a second); the true share of desk
cells in those lines is not logged. A request-scoped log on the function
(rows scanned, elapsed, caller) would settle it in one refresh.

**Data it reads. [V]** the table is `run_memory_entries` (outside the object
graph, same Dolt engine). Index `idx_run_memory_entries_owner_agent_created
(owner_id, agent_id, created_at)` serves the equality and the created_at
order (`store.go:598`); `kind` is not in it. Row count on the paper:
424,992 rows (0.76 GiB) at the 10-19 probe
([paper-disk-full-again-history-growth-2026-10-19.md](paper-disk-full-again-history-growth-2026-10-19.md),
line 344), written at about 3.7 entries a second on 10-22
([paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md](paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md)),
and a cursor entry is written after a cell whenever the cursor moved
(`rlm_reduce.go:1047`). The per-desk split is not known; no SQL was run on
production.

## The plan

**[V]** `EXPLAIN PLAN` on a local Dolt store (the test prints it), owner given,
`kinds = [rlm_inbox_cursor]`:

```text
Project
 └─ Sort(created_at ASC, seq ASC)
     └─ Filter (kind HASH IN ('rlm_inbox_cursor'))
         └─ IndexedTableAccess(run_memory_entries)
             ├─ index: [owner_id, agent_id, created_at]
             ├─ filters: {[owner, owner], [agent, agent], [NULL, inf)}
             └─ columns: all sixteen, including message_json, summary, reason, details_json
```

- It uses the index, to find the desk's whole range, not its cursor rows.
  The `kind` filter sits above the access and runs after every row of the
  range was fetched with all columns. **[V]**
- It sorts in memory on `(created_at, seq)`; `seq` is not in the index. **[V]**
- Allocation shows the full-row fetch: at 5,000 desk rows the cursor-kind
  call allocates 17.6 MiB for 1,000 returned rows against 22.3 MiB for all
  5,000 rows; the narrow projection on the same index allocates 2.1 MiB. So
  the cost is rows read, not rows returned. **[V]**
- With `ownerID == ""` (no production caller passes it, **[H]**) the plan is a
  full table scan: `Filter(agent_id = ... AND kind IN ...)` over `Table`. A
  latent trap, not today's stall. **[V]** plan.
- With a `(owner_id, agent_id, kind, created_at)` index (made on the scratch
  store only) the plan reads exactly the cursor rows and drops the filter. **[V]**

## Hold and allocation as the desk grows

`TestRunMemoryHoldCensus` (`internal/store/run_memory_hold_census_test.go`,
env-gated). Local embedded Dolt, warm, 425,000-row table for the last two
rows, the desk's rows interleaved with 40 other agents' rows. Kinds: 55%
message (1.5 KB), 20% cell_fate, 20% `rlm_inbox_cursor`, 5% compaction (3 KB).
That mix is a guess. "Wait" is a trivial `SELECT 1` on `s.db` issued 2 ms
into a list of the hot desk: it waits for the connection, which is the
`/health` situation (`readHealthStore`, three reads, 1 s budget).

| desk rows / table rows | cursor rows returned | whole call, first / best | allocated per call | `SELECT 1` waits | narrow projection (A) | + kind index (B: all columns, C: narrow) |
|---|---|---|---|---|---|---|
| 500 / 5,000 | 100 | 2.1 / 1.8 ms | 1.4 MiB | 0.3 ms | 1.3 ms | 1.1 / 1.0 ms |
| 5,000 / 50,000 | 1,000 | 18 / 12 ms | 17.6 MiB | 10.4 to 10.8 ms | 8.9 ms | 4.6 / 2.6 ms |
| 20,000 / 200,000 | 4,000 | 102 / 47 ms | 137 MiB | 46 to 48 ms | 33 ms | 15 / 7.6 ms |
| 20,000 / 100,000 | 4,000 | 83 / 45 ms | 108 MiB | 38 to 45 ms | 30 ms | 14 / 6.9 ms |
| 100,000 / 425,000 | 20,000 | 548 / 524 ms | 644 MiB | 488 to 517 ms | 427 ms | 74 / 34 ms |
| 300,000 / 425,000 | 60,000 | 849 / 849 ms | 1,000 MiB | 836 to 907 ms | 672 ms | 189 / 80 ms |

- **[V]** The hold is the whole call: a concurrent `SELECT 1` on `s.db`
  waits within 5% of the call's duration. At 100,000 desk rows that is half a
  second per call; at 300,000 it is 0.85 s. A cold desk of the same table (2,124
  to 624 cursor rows) holds 6 to 30 ms.
- **[V]** It scales with the desk's rows, about 2.3 to 5 microseconds a row
  warm, not with the cursor rows returned (the all-kinds call, 100,000 rows,
  costs 576 ms, close to the cursor-only 548 ms).
- **[V]** Allocation per call is 6.6 KiB per desk row at 100,000 rows: 644 MiB
  per call. Two calls per cell and about one cell a second would allocate
  hundreds of MiB a second on a guest [H, rate].
- Sorted by what dominates: query and row iteration are 85% of the call
  (`rows.Next` only: 458 of 548 ms at 100,000); decode and scan are the rest. **[V]**
- To reach one second warm, the desk needs about 350,000 rows, which is the
  whole table of 10-19 (425,000). **[V]** extrapolation of a linear fit, **[H]** that the
  fit holds there.
- After `DOLT_GC` and after a close and reopen of the scratch store (the
  `CHOIR_RUN_MEMORY_HOLD_GC=1` pass, 20,000 desk rows of 100,000) the hot call
  is 42 to 43 ms, unchanged: an empty engine chunk cache with a warm OS page
  cache does not reproduce the paper. **[V]** The paper's reads go to a
  137 GB sparse image through a guest block device; that is the untested part.
- Measurement conditions: the second run (the last two rows) shared the
  machine with another lane's `go test -race` run; its first-call numbers are
  noisy by up to 2 times. Timings of `best` are repeatable within 10%.

## The paper's console

Read-only: `ssh node-b 'sudo grep -a store_stalled
/var/lib/go-choir/vm-state/vm-1669020adcece51c2f5ff9b6df399ef5/console.log{,.1,.2,.3,.4}'`,
copied to the worker's scratch and parsed there. 98 `store_stalled holders:`
headers, 101 `store_stalled holder g=` lines, 01:05:40 to 08:40:35 UTC across
eleven guest lifetimes (ten boots inside the logs). (The parent census counted 49 of 97 at 08:17; the same logs
through 08:17 now give 31 of 65 holder lines from 03:24, and 52 of 101 over
the whole of the logs, which start at 01:05.)

### Holders ranked

| rank | holder (first choir frame) | lines | what it is |
|---|---|---|---|
| 1 | `store.(*Store).ListRunMemoryEntriesForAgent` | 52 | this record; 51 of 52 goroutines `[runnable]`, engine frames in chunk fetch: `nbs.tableSet.get` 17, `gozstd ZSTD_decompress_usingDDict` 8, `nbs.fileReaderAt.ReadAtWithStats` 5 |
| 2 | `objectgraph.(*DoltStore).ListObjects` | 23 | the stream_seq seed at boot (`choir.event`, limit 500); 22 of 23 within 417 s of a boot |
| 3 | `store.(*Store).runOnlineDoltGC` | 15 | the online GC, every 10 to 15 minutes; goroutine `[chan send]` or `[runnable]` in `nbs.journalChunkSource.getManyCompressed` / `tableReader.hasMany` |
| 4 | `objectgraph.scanDoltObject` | 3 | a single object read, in `nbs.tableReader.get` / `archiveReader.readByteSpan` (caller beyond the frame is not named) |
| 5 | `computerversion.queryTableContentHash` | 3 | the table content witness, in archive reads, at 262 to 597 s after a boot |
| 6 | `objectgraph.(*DoltStore).derivedStrayIDs` | 2 | derived-key validation of the outbox (the parent record's 8-minute pass) |
| 7 | `store.(*Store).ListRunMemoryEntries` | 1 | one run's memory (`CellFate`, `LoadInboxCursor`), in `HasDoltDataDir` |
| 8 | `objectgraph.(*DoltStore).ListObjectRefsByKindOwner` | 1 | the limit-8192 scan of `latestLifecycleRunByAgent` |
| 9 | `store.(*Store).Head` | 1 | a commit-head read, in the SQL parser |

Excluding the boot-window work (ranks 2, 5 and 6, 28 lines), rank 1 is 52 of the
remaining 73 lines (71%) and the GC is 15 (21%). **[V]**

### What the console does not give

The stall line is one goroutine snapshot taken after the 503 was written and
at most once a minute (`storeStallHolderEvery`), so it records who held the
connection, not for how long. **[V]** There is no per-call duration in the
log. What it gives instead:

- Every sample implies the connection was already held, or queued for, longer
  than `/health`'s 1 s budget when the snapshot was taken. **[V]**
- `write_pool` counters in the 97 headers, summed over the 86 intervals
  between snapshots of one boot (21,482 s): 153,526 waited connection
  acquisitions, 7.1 a second, 5,622 s of total wait: 36.6 ms mean wait, 0.26
  goroutines waiting on average. The sample is stall-biased (every header is
  a stall). **[V]** arithmetic, biased sample.
- A busy episode on the connection can be read from the probes: a 503
  needs the connection unavailable for over 1 s, and the probe runs every
  3 s while stalled. In the windows whose snapshot names rank 1, 14 of the
  19 to 20 503 runs are one sample long (the episode ended within about 4 s)
  and 5 to 6 are two samples long (4 to 7 s); none is longer. The GC
  window (05:34:37 to 05:34:55) is seven samples in a row, at least 18 s. **[V]**
  An episode can be several calls queued, not one call. **[H]**

## Do the stalls line up with the 503 bursts?

The probe files are `/Users/wiz/Dev/lanes/_runs/deployD9/paper-health-{before,after}.txt`:
one `/health` probe each 2 s (3 s while failing), timestamps `HH:MM:SS` (read
as UTC, the same clock as `date -u` on node-b; the console log timestamps are
guest-clock UTC and agree to within 1 to 3 s). Join: a 503 cluster is
matched to a snapshot within 5 s before and 5 s after.

| file | window | samples | 503 | rate | 503 clusters (gap over 20 s) |
|---|---|---|---|---|---|
| before | 05:22:59 to 05:52:57 | 858 | 26 | 3.0% | 5 |
| after, whole | 06:33:39 to 07:03:38 | 758 + 25 failed | 42 | 5.5% | 7 (one of 31 during a boot) |
| after, guest uptime over 420 s | | 619 | 8 | **1.3%** | 5 |

The 1.3% in the brief is the after file past the boot seed window. **[V]**

Before (one guest, booted 04:04:21, no boot in the window):

| 503 cluster | 503s | snapshot in the same minute | holder |
|---|---|---|---|
| 05:31:09 to 05:32:18 | 7 | 05:31:10, 05:32:19 | `ListRunMemoryEntriesForAgent`, both |
| 05:32:42 to 05:32:45 | 2 | none (1 a minute limit, 23 s after the 05:32:19 snapshot) | unknown |
| 05:34:37 to 05:34:55 | 7 | 05:34:38 | `runOnlineDoltGC` |
| 05:44:01 to 05:44:50 | 7 | 05:44:02 | `ListRunMemoryEntriesForAgent` |
| 05:45:15 to 05:45:28 | 3 | 05:45:16 | `ListRunMemoryEntriesForAgent` |

17 of 26 samples sit in a minute that a rank-1 snapshot names, 7 in the
GC's, 2 in the shadow of the rate limit. Every snapshot in the window has a
503 cluster, and every cluster but one has a snapshot. **[V]**

After (past the boot window, guest 06:24:56 to 06:52:47, then 06:52:47):

| 503s | snapshot | holder |
|---|---|---|
| 06:37:05 (1) | 06:37:06 | `ListRunMemoryEntriesForAgent` |
| 06:43:41 to 06:43:59 (4) | 06:43:42 | `ListRunMemoryEntriesForAgent` |
| 06:52:38, 06:52:43 (2) | 06:52:35 (3 s earlier, the guest was being closed) | `ListRunMemoryEntriesForAgent` |
| 07:02:43 (1) | 07:02:44 | `derivedStrayIDs`, `queryTableContentHash` (the parent record's 8-minute pass) |

7 of 8 are rank 1. The 31-sample cluster 06:53:43 to 06:55:14 and the 06:57:09
to 06:59:41 ones are the new guest's boot windows (`ListObjects` seed,
`queryTableContentHash`, `derivedStrayIDs`), not this holder. **[V]**

So the answer is yes, they line up, with the limit that the log keeps one
holder a minute: a minute with several 503s has one name. **[V]**

## Fix shapes (the record's first pass; shape 2 and the narrow projection are dealt with below)

Ordered by how much of the problem class each removes. None is a patch of
the 503 symptom.

1. **Do not read the table for a number. [H]** `LoadInboxCursorForDesk` wants
   the maximum cursor per `(desk, channel)`. A keyed state (one row, or one
   object per `(desk, channel)`, replaced when the cursor moves) makes the
   read O(1) at any size and stops adding a `run_memory_entries` row (about
   90 KiB of journal each, parent
   [paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md](paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md))
   to the table the next reader scans. Red: it moves the durable inbox
   cursor and needs a backfill for existing desks. The substrate answer.
2. **Drop the commit-time read in the common case. [H]** In
   `recoverPartialActCommit` (`rlm_reduce.go:891`) the cursor is only used
   after a commitment record is found to exist; load it there. Halves the
   calls in a cell with no prior record. Orange, one function.
3. **Read only the cursor rows, narrowly. [V as a measurement]** An online
   `(owner_id, agent_id, kind, created_at)` index on `run_memory_entries`,
   built by the existing access-path migration and never a boot step, plus a
   `SELECT details_json` projection, measured 15 times (100,000 desk rows,
   524 to 34 ms) and 11 times (300,000, 849 to 80 ms) cheaper and 75 times less
   allocation. Still linear in the desk's cursor rows. Orange (schema).
   A newest-64 form (variant D) is not faster at these sizes, 38 to 101 ms: the
   plan is `TopN`, not a reverse index scan.
4. **Keep the cursor in memory, written through. [H]** The cursor advances
   only in `CommitInboxCursor` in this process; read once per desk at first
   use after a boot and keep it. Orange. The restart rule (a crash closes
   work, a planned restart resumes) means the single cold read at boot is the
   only one left.
5. **Move the read off the write connection. [H]** The read pool
   (`readDB`) shares the engine, serialised on the objectgraph mutex (parent
   record); it moves the hold from the connection to the engine mutex, where
   `ListObjects` and the others also queue. Not a cure.
6. **Bound the table. [H]** A retention cut for cursor rows other than the
   newest per `(desk, channel)`, or run-memory compaction. Red (it deletes
   run memory the replay diagnostics read).

## What was built (orange, same lane)

The coordinator asked for the cheap part: shape 2, the narrow projection if
it earned 2x, and an observation. Shapes 1, 3, 4 and 6 (keyed cursor state,
the `kind` index, the in-memory cursor, retention) are **next**, not built.
Rollback of all of it: revert `d95af3714`; nothing else changed at runtime.

**(a) Shape 2: `recoverPartialActCommit` reads the cursor lazily. [V]**
`rlm_reduce.go` loaded the agent cursor on its first line for every commit.
It now loads it once, on the first intent whose commitment record exists, and
reuses it for the rest of the tray. Tests written first
(`rlm_reduce_test.go`, commit `cf6c4027c`, run on the unfixed code):

| test | before the fix | after |
|---|---|---|
| `TestDeskCellReadsTheAgentCursorOnceInTheCommonCase` (cell start + commit of a fresh act) | FAIL: 1 call at start, **2** per cell | 1 call at start, **1** per cell |
| `TestCommitWithNoPriorCommitmentRecordDoesNotReadTheAgentCursor` (commit of two fresh acts) | FAIL: **1** call | **0** calls |
| `TestPartialCommitRecoveryStillReadsTheAgentCursor` (two staged commitment records, then a replay) | pass: 1 call, 1 on replay, one envelope | pass: the same |

The calls are counted by `Store.RunMemoryAgentListCalls()`, a counter added
to the store for this and for the log below. The cursor answer is unchanged:
the first test reads the durable cursor after the cell (1), the third checks
the recovered receipt and the single envelope. Effect on the paper [H]: one
`ListRunMemoryEntriesForAgent` per desk cell instead of two, so the
function's share of the connection falls by about half in the common cell;
a cell whose acts already have records (a crash-recovered tray) still reads
it. The rest of the tests that touch `rlm_reduce` pass under `-race`
(`-run 'Reduc|Cursor|Cell|Commit|Partial|Recover'`, 126 s).

**(b) The narrow projection alone: not built. [V]** Variant E of
`TestRunMemoryHoldCensus`, at 100,000 desk rows of 425,000, with no new index
and no schema change: `SELECT details_json ... WHERE agent_id = ? AND
owner_id = ? AND kind IN (?)` with no `ORDER BY` (the caller takes a maximum).
Plan: the same `(owner_id, agent_id, created_at)` index access with `kind`
filtered above it, columns `owner_id agent_id kind details_json`.

| shape | hold, first / best | allocated |
|---|---|---|
| today | 565 / 510 ms | 644 MiB |
| E: narrow, no `ORDER BY`, same index | 424 / 419 ms | 481 MiB |

1.2 times on the hold and 1.3 times on allocation, under the 2 times bar, so
it is left out (the sort was not the cost; the fetch of every row of the
desk's range through a non-covering index is, 458 ms of `rows.Next` alone
earlier). It only pays together with the `kind` index (variant C, 34 ms), which
is shape 3 and is next.

**(c) The observation log. [V] in test, [H] on the paper.** Every call of
`ListRunMemoryEntriesForAgent` now measures itself (`run_memory_observe.go`).
A call over 250 ms logs one line, at most once a minute (the stall snapshots'
limit), with the number of slow calls it stood for:

```text
store: run_memory_agent_list slow site=desk_cell_start agent=<desk> kinds=1
  rows_returned=4000 conn_wait_ms=0 first_row_ms=410 iterate_ms=30
  elapsed_ms=440 calls_since_open=1234 slow_suppressed=3
```

`site` is `capsule_cell_start`, `desk_cell_start` or `commit_recovery`, put on
the context by the three call sites in `rlm_reduce.go`; any other caller is
`unlabelled`. `conn_wait_ms` is the wait for the write pool's one connection
(the call now takes it with `db.Conn`, same pool, same semantics),
`first_row_ms` the plan, fetch and in-memory sort, `iterate_ms` the scan and
decode. `rows_returned` is the rows the call returned, not the rows the engine
read (the filter runs inside the engine and the log cannot see them). On the
next refresh this settles three things: which call site is the long one; how
much of a slow call is waiting for the connection and how much is holding it;
and, from `first_row_ms` against the 20 to 100 times gap, whether the paper's
cost is the fetch (cold chunks) or the scan. `TestRunMemoryAgentListSlowLine`
covers the fast call (silent), the line's fields, the rate limit with its
suppressed count, and the unlabelled site. The log is read with the same
read-only console grep as the stall lines:
`sudo grep -a run_memory_agent_list .../console.log*`.

Heresy delta of this pass: discovered, none new; introduced, a write-pool
`db.Conn` in place of `db.QueryContext` for this one function (the call holds
the connection exactly as before; the log is the only new output); repaired,
the unconditional cursor read in `recoverPartialActCommit`.

## The first cold read: 16.7 s (D15 observation, 10:41 UTC)

D15 (`ff5cb4cbb`, deployed 10:41:36) carries the slow-read log. Its first
line, from the paper:

```text
run_memory_agent_list slow site=desk_cell_start agent=management:e5962310-8767-4624-87e5-a9b93a4e1db1 kinds=1 rows_returned=14965 conn_wait_ms=0 first_row_ms=16648 iterate_ms=49 elapsed_ms=16699 calls_since_open=1
```

**[V]** (the console line, copied by FP0). It answers three of the open
questions of "What this does not settle":

- the long call site is `desk_cell_start`, the first call after a boot
  (`calls_since_open=1`);
- the call does not wait for the connection (`conn_wait_ms=0`) and does not
  spend its time scanning or decoding (`iterate_ms=49` for 14,965 rows, 3
  microseconds a row); it holds the write connection for 16.6 s before the
  first row, in the plan, fetch and in-memory sort of the whole desk range:
  `first_row_ms` is the cold read of the desk's rows through
  `idx_run_memory_entries_owner_agent_created` and the `kind` filter above it;
- the management desk holds 14,965 cursor rows, about 15,000. Locally, warm,
  that read is 34 ms with the kind index and 510 ms without at 20,000 cursor
  rows (100,000 desk rows); 16.6 s is 30 to 500 times the warm number, which
  is the cold-chunk gap the stacks suggested. The warm local figures cannot
  be used to size it; only the shape (rows read versus rows wanted) is
  portable. **[H]** that the kind index cuts the cold read by the same ratio
  as the warm one (the rows fetched fall from the whole desk range to the
  cursor rows, which are 20% of a row mix we guessed).

Every `/health` read that arrives in those 16.6 s answers 503
`store_stalled`, and every writer waits.

## Shape 3: how the index gets onto a live store

Questions asked before writing code, answered from the repo.

**What mechanism does this repo have for an index on a populated table?**
Two exist and one is a trap.

- *The trap, `schemaDDL`.* `store.go:599-602` creates the existing
  `run_memory_entries` indexes with `CREATE INDEX IF NOT EXISTS` in
  `schemaDDL`, applied by `bootstrap` at every `Open` (`store.go:1001`). A
  fourth index line there would build on the paper at its first boot after
  the deploy, on the boot path, before the listener, holding the engine for as
  long as the build takes. That is the shape the repo has already refused
  twice: [boot-alters-og-objects-when-older-columns-missing-2026-10-21.md](boot-alters-og-objects-when-older-columns-missing-2026-10-21.md)
  and [stored-column-add-blocks-paper-boot-2026-10-21.md](stored-column-add-blocks-paper-boot-2026-10-21.md),
  and R5 of the journal record ("run as an explicit migration, never at
  boot"). So the index does **not** go in `schemaDDL`.
- *The mechanism, the explicit access-path operation.* Derived columns and
  indexes are added at open only to an empty table; on a table with rows the
  boot is discovery only and the operator adds them with `POST
  /internal/store/derived-columns?action=migrate` (host peer only, one step at
  a time, cancellable, progress on `GET`; `internal/store/derived_columns.go`,
  `internal/autoputer/derived_columns.go`). Its DDL steps run under the shared
  engine mutex through `DoltStore.WithEngine`, the hook the F-c event table
  uses for a table outside the object graph (`event_table.go:258`). That is
  the mechanism used here: the run-memory kind index becomes one more step of
  that operation. It is a per-computer act at a time the operator chooses.

**Does it need a store schema epoch bump? No.** `storeschema.Version`'s
contract asks for a bump when a change "is not freely backward compatible
for older binaries". An additive secondary index is. An older binary's
`schemaDDL` is `CREATE INDEX IF NOT EXISTS` for its own three indexes and
leaves a fourth alone; its queries are the same SQL and the planner may even
use the new index. **The content witness is not rows only, and the first
pass of this record said it was (wrong, bunch 160 round 1, DeepSeek HIGH).**
`computerversion/dolt_state_extractor.go:194-203` also hashes a per-table
schema observation, `queryTableSchemaHash` (`SHOW COLUMNS`, `:338-382`), and
its `Key` field reads `MUL` for the first column of a non-unique index.
`replay_eligibility.go:235-243` records `SchemaDrift` when a live and a replay
schema differ, the report goes ineligible, and
`checkpoint_restore_bindings.go:53-60` refuses a self-development checkpoint.
The first pass's order, `(agent_id, kind, owner_id, created_at)`, turned
`agent_id` from no key to `MUL`; a live store that had built the index would
then have differed from a fresh store (which makes it at open, on the empty
table). **[V]** (`TestRunMemoryKindIndexKeepsTheWitnessSchemaObservation`: with
an agent-leading index the observation changes; its control keeps that
visible). The index is therefore **owner-leading**, `(owner_id, agent_id, kind,
created_at)`: `owner_id` is already `MUL` through the existing
`(owner_id, agent_id, created_at)` index, so `SHOW COLUMNS` is byte-identical
with and without the index, and the test asserts every table's schema
observation is equal on a store with the index and one without. The row
content hash (`queryTableContentHash`, `SELECT *`) never saw indexes. Nothing
else keys on the epoch for an index, so there is no floor to raise, and the
same reasoning is why the F-c table needed a bump (an old binary cannot know
the table) and this does not. The CI store-floor guard named in the ledger is
not in `main`, so there is nothing here for it to fence. **[V]** (code and
tests); **[H]** that no off-repo tool reads the index set or `SHOW COLUMNS`
keys.

**Index order.** A plan test on 100,000 rows built three orders on a scratch
store and planned them against the query's four shapes (owner and one kind,
owner and two kinds, no owner and one kind, owner and all kinds). All three turn
the owner-and-kind shapes from a range of the whole desk with `kind` filtered
above it (80 ms) into an index range on `kind` (1.1 ms, no filter above). The
`(owner_id, ...)` order leaves the no-owner shape on a full table scan (93 ms);
the two agent-leading orders serve it. The first pass chose an agent-leading
order for that reason; that choice changes the witness (above), so it is
reversed. Chosen: `(owner_id, agent_id, kind, created_at)`. The only production
caller, `LoadInboxCursorForDesk`, always sends an owner and one kind; the
no-owner shape is **not served** (no production caller; it stays on the table
scan it had, and the earlier "latent trap" note stands as a note, not a fix).
With all kinds the planner keeps the old index. The sort on `(created_at, seq)`
remains above the access (it now sorts the cursor rows only).

## Shape 3 built (red, store schema, tier 2; lane `fp0-run-memory-kind-index`)

Mutation class: **red** (store schema, a boot-path function, and an
operation on a live computer's store). Protected surfaces touched: the store
schema (an additive index), `Store.Open` (one discovery-or-create call),
`/internal/store/derived-columns` (one more step). Not touched: the store
schema epoch, `schemaDDL`, the event tape. The content witness is affected
only if the index changes `SHOW COLUMNS`; the first pass did, the second does
not (below). Commits, in
order: record `f616a3b14` (green), tests `4205d0a78` (yellow), code
`80e81254b` (red), then this update (green) with the census variant that
measures the build.

**What it is. [V]** `internal/store/run_memory_kind_index.go`. The index is
`idx_run_memory_entries_owner_agent_kind_created ON run_memory_entries(owner_id,
agent_id, kind, created_at)`.

- *Open.* On an empty `run_memory_entries` (a fresh store) the index is
  created at open, where it costs nothing. On a table with rows Open only
  reads `information_schema` and one row, and logs `run-memory kind index ...
  missing on a table with rows, not added at open`. It is not in `schemaDDL`,
  which every boot applies, and a test pins that (`schemaDDL` does not name
  it, a populated store reopened twice does not gain it).
- *The operation.* `POST /internal/store/derived-columns?action=migrate` (host
  peer only, 202, 409 while one runs, cancellable) runs the og_objects steps,
  then this one: a single `CREATE INDEX` through `DoltStore.WithEngine`, so
  under the engine mutex and on the write connection, logged as
  `run-memory kind index ... building` and `built in <d>`. `GET` reports
  `run_memory_kind_index {name, present, has_rows}` and the step
  (`kind: index`, `duration_ms`) in `steps`. It is idempotent (a second run
  adds no step), skipped by a cancel, and a failure leaves state `failed` with
  the error and the read on the slower plan.
- *The query.* Unchanged SQL (now built by `runMemoryAgentListQuery`). The
  planner reads the index when it exists; no code path depends on it.
- *Mechanism chosen, and why not the other.* See "How the index gets onto a
  live store" above: the repo refuses a build on a populated table at boot, so
  the F-c-style boot DDL was not used; the explicit operation is the repo's
  own. This means the paper does **not** build it at its first boot after the
  deploy: FP0 (or the operator) runs the POST at a chosen time. That departs
  from the brief's sentence "the build runs once at the first boot"; the
  mechanism, not the brief, is the repo rule, and the cost below says why.
- *Epoch.* No bump, no floor to raise (reasoning above). **[V]** code,
  **[H]** nothing outside the repo reads the index set.

**Plan before and after. [V]** Printed by `TestRunMemoryKindIndexPlanHasNoFullRangeWalk`
from the production statement:

```text
before:  Sort(created_at, seq)                      after:  Sort(created_at, seq)
          Filter (kind HASH IN (...))                        IndexedTableAccess(run_memory_entries)
           IndexedTableAccess                                  index: [owner_id, agent_id, kind, created_at]
            index: [owner_id, agent_id, created_at]            filters: {[owner], [agent], [kind], [NULL, inf)}
            filters: {[owner], [agent], [NULL, inf)}
```

Owner and one kind, and owner and two kinds (two ranges), read the new index
with no `Filter` above it; with all kinds the planner keeps the old index
(both asserted). No owner and one kind is not served and not pinned.

**Hold at 100,000 and 300,000 desk rows of 425,000, warm local. [V]**
`TestRunMemoryHoldCensus` (`CHOIR_RUN_MEMORY_HOLD=425000:100000,425000:300000`),
the production index built by the production step:

| | desk rows 100,000 (20,000 cursor rows) | desk rows 300,000 (60,000 cursor rows) |
|---|---|---|
| `ListRunMemoryEntriesForAgent`, before | 465 / 454 ms, 644 MiB | 867 / 815 ms, 1,000 MiB |
| after (the production query, new index) | 107 / 67 ms, 133 MiB | 252 / 184 ms, 320 MiB |
| ratio of the best | 6.8 times | 4.4 times |
| narrow projection on top (not built) | 31 ms, 8.6 MiB | 80 ms, 25.8 MiB |

The build, with a `SELECT 1` probe on the write connection every 10 ms:

| | 100,000 desk rows | 300,000 desk rows |
|---|---|---|
| build time | 1.13 s (1.19 s whole) | 1.02 s (1.07 s whole) |
| longest `SELECT 1` wait on `s.db` during it | **1.16 s** | **1.04 s** |
| probes over 100 ms | 1 (the build is one statement) | 1 |

So the build holds the write connection, and the engine mutex, for its whole
duration, and the duration is the table's size, not the desk's (the same 425,000
rows both times). **/health during the build: [V] it blocks.** `/health`'s
three store reads wait on `s.db` with a 1 s budget (`readHealthStore`); a
1.04 to 1.16 s hold is just over the budget, so locally it costs one or two
`store_stalled` 503s. **[H] on the paper:** the build reads the whole table once,
and the paper's reads of it are cold: the first-row read of its 14,965 cursor
rows took 16.6 s, 30 to 500 times the warm local number for a read that
touches a fraction of the table. A full pass over the paper's table (the
10-19 probe counted 424,992 rows, 0.76 GiB, and it has grown) may take tens of
seconds to minutes with `/health` 503 for that long, and every writer waiting.
It is bounded by the table, runs once, and needs no restart. Run it at a
chosen time, with the paper's reads of the cursor held up for that window
either way (they are already 16 s each).

**Write cost of the index. [V] local, [H] paper.** Each `AppendRunMemoryEntry`
now maintains one more tree: the store directory grew 41.0 and 45.1 KiB per
append before, 53.9 and 58.4 KiB after (+12.9 and +13.3 KiB, +29%) in the two
runs (time per append unchanged, 9.1 to 9.7 ms). At 3.7 entries a second
([paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md](paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md))
that is about 48 KiB a second, 2.7% of the paper's modelled 1.77 MiB/s journal
growth. That journal record's R5 (drop `idx_run_memory_entries_loop_seq`, an
exact duplicate of `UNIQUE(loop_id, seq)`) removes about as much and is the
natural pairing; it is not done here.

**Rollback. [V]** (`TestRunMemoryKindIndexKeepsTheAnswerAndRollsBackSafely`)

- *Revert the commit, leave the index.* The index stays in the store. An older
  binary's boot applies `schemaDDL` (its own three `CREATE INDEX IF NOT
  EXISTS` lines) without error, appends and reads work on the indexed table, a
  close and reopen neither drops nor rebuilds it, and its queries are the same
  SQL, so the old binary probably also gets the faster plan **[H]**. The only residue is the
  extra 12.5 KiB per append.
- *Full rollback.* `DROP INDEX idx_run_memory_entries_owner_agent_kind_created
  ON run_memory_entries` (a metadata operation, fast). After it the table's row
  hash (`dolt_hashof_table('run_memory_entries')`) is exactly what it was before
  the index existed, and the cursor answer is unchanged. Both facts are logged
  by the test. **On a dieted store, run `restore_run_memory_indexes` first:
  `owner_agent_created` must exist before the kind index is dropped**, because
  after the index diet the kind index is the only index that leads with
  `owner_id`, and dropping it alone moves `owner_id`'s `SHOW COLUMNS` Key from
  `MUL` to empty and with it the content witness (see "The kind index is now
  load-bearing for the witness" in the index diet section below). *Forward note
  (2026-10-23): the rollback lines in
  `docs/definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md`
  that call `DROP INDEX` the whole kind-index rollback predate the diet; the
  ledger is append-only history and is not edited, this paragraph supersedes
  them.*
- *Boot invariant: the kind index is `owner_id`'s only Key on a fresh store.*
  On a fresh store `owner_id` has a Key (`MUL`) only because `Open`'s hook
  (`ensureRunMemoryKindIndexAtOpen`) creates the kind index on the empty table;
  `schemaDDL` no longer creates any other `owner_id`-leading index. That hook's
  error is logged and recorded as a failed boot phase, not fatal, so a failure
  there boots a store with `owner_id`'s Key empty, and every witness comparison
  against a store that has the index fails closed (a visible drift, never a
  silent match).
- *The answer.* The cursor, the two-kind and the no-owner reads return the same
  entries in the same order with and without the index.
- A reopen after the operation does nothing to a present index, and a revert
  to a binary without this code leaves a store whose Open never touches it.

**Checks. [V]** focused `-race` on `internal/store`
(`-run 'RunMemory|DerivedColumn|BootSteps|TestListRunMemory'`), the
`internal/autoputer` derived-columns endpoint tests, `internal/computerversion`
and `internal/projectionbase` whole; the three gate scripts and
`scripts/selftest-guest-token` (196 passed). The whole `internal/store`
package does not finish in 10 minutes locally (it never did; CI shards it).

**Heresy delta.** Discovered: the brief's "build at first boot" and the repo's
rule disagree (resolved by the rule); the build is a whole-table pass whose
duration on the paper is unknown and blocks `/health` for it; each run-memory
append already cost 41 KiB of journal locally. Introduced: one more index to
maintain (+13 KiB per append). Repaired: the 16.6 s first read, once the
operation runs (not yet observed on the paper).

**Next, after the operation runs on the paper:** read the
`run_memory_agent_list slow` line again (it should not appear, or show a
`first_row_ms` of a small multiple of the cursor-row count), then decide on
shape 1 (keyed cursor state), 4 (in-memory cursor) and 6 (retention), and on
R5.

## Shape 3, round 2: the witness finding (bunch 160)

Bunch 160: Devin APPROVE (nits only); DeepSeek v4.1 flash REVISE with one HIGH
and four lows. Answers, tests first (commit `04d733474`, then the code):

- **HIGH, the witness hashes a schema observation, so the agent-leading index
  would have been a replay drift and a checkpoint refusal. Accepted, and the
  first pass's claim "the witness hashes rows, not indexes" is corrected above.**
  The index is now `(owner_id, agent_id, kind, created_at)`. The schema
  observation test result: **passes with the owner-leading index and fails
  with the agent-leading one.** `TestRunMemoryKindIndexKeepsTheWitnessSchemaObservation`
  opens a fresh store (index made at open) and one with the index dropped,
  closes both, runs the production extractor
  (`computerversion.DoltStateExtractor`, the configuration of `eventTableWitness`)
  and asserts every table's `:schema:` observation is equal; a control store with
  an extra agent-leading index must differ from the fresh one (agent_id becomes
  `MUL`), so the test can see a drift. Before the reversal the same comparison
  failed on the agent-leading production index. The comparison does not stand in
  for a run of the replay probe on the paper, which was not done. **[V]** local.
- **Low 2, the rollback row hash was logged, not asserted.** It is asserted now:
  after `DROP INDEX` the `dolt_hashof_table('run_memory_entries')` equals its
  value before the index existed.
- **Low 3, a cancel between the og_objects steps and the index step reported
  `done` with the index absent.** Fixed: a cancelled context at the end of the
  run is state `cancelled`, and the index is reported absent. Test: the
  step hook holds the run, the cancel lands, the og part (nothing missing)
  returns without looking at the context.
- **Low 4, a `GET` during a run omitted `run_memory_kind_index`.** Fixed: the
  start probes it (bounded 2 s, best effort) and the status carries it while
  the run holds the engine; the end of the run refreshes it.
- **Nit 6, the boot hook's two queries had no deadline and swallowed errors.**
  Fixed: the hook takes a context with a 5 s deadline and returns its error;
  `Open` records it as a failed boot phase (`runtime-schema/run-memory-kind-index
  status=failed`) and goes on, so the open never fails on it.
- **The no-owner shape** is not served by an owner-leading index and no
  production caller sends it. Recorded as unserved; the plan test does not pin
  it.

**Rollout order. [H]** Run `POST /internal/store/derived-columns?action=migrate`
on the paper before any base rebuild or replay-gated work (a self-development
checkpoint binding, a replay-completeness probe). With the owner-leading order
the schema observation is identical with and without the index, so the order is
not needed for the witness to agree; it is needed for two other reasons. A
rebuilt or fresh store gets the index at open and a store that has not been
migrated does not, so until the operation runs the two differ in structure
(not in any hashed observation), and the faster read exists only on stores that
ran it. And the build holds the write connection and the engine for its whole
duration (1.0 to 1.2 s warm, unknown cold), during which a replay probe reading
live state would wait on it; done first, nothing replay-gated overlaps it. No
computer ever built the agent-leading index: the first-pass branch was not
merged.

**Numbers redone with the owner-leading order.** The build-cost and hold tables
above are from the owner-leading index (the census was rerun): the order changes
them by under 10%, build 1.02 to 1.13 s, hold of the write connection 1.04 to
1.16 s, read after 67 ms and 184 ms.

## R5: the duplicate `idx_run_memory_entries_loop_seq` (red, store schema; lane `fp0-run-memory-r5`)

Mutation class: **red** (store schema, `schemaDDL`, `Open`, and one more step of
`/internal/store/derived-columns`). Protected surfaces: the store schema (a
dropped secondary index), the boot path (one discovery-or-drop call on an empty
table), the content witness only if `SHOW COLUMNS` moves (it does not, below).
Not touched: the epoch, row data, the event tape. Review tier: FP0 judges; the
drop is additive in effect (an index is not state), behind an operator act with
a one-statement rollback, but it edits the boot path, so it is recorded here as
tier 1 or 2 by FP0's reading of "silent or irreversible".

**The problem. [V] local.** The kind index (shape 3) cost 12.5 KiB more journal
per append (+29%). The journal record
([paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md](paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md),
R5) named the pairing: `run_memory_entries` carries
`idx_run_memory_entries_loop_seq ON (loop_id, seq)` as well as
`UNIQUE(loop_id, seq)`. Both are in `schemaDDL` and the `CREATE TABLE`
(`store.go:504-521`, `:599`), introduced in the same commit (`e759ce3f1`), so
every store, the paper included, has both. Each append writes every secondary
tree; the duplicate is one tree that nothing reads.

**Duplicate proof. [V]** (`TestRunMemoryLoopSeqIndexIsADuplicateOfTheUniqueKey`).

- *Prefix-superset.* `information_schema.STATISTICS` on a fresh store: the
  unique key is named `loop_id` by Dolt and has columns `(loop_id, seq)`, the
  same columns in the same order, and is `NON_UNIQUE=0`. The other indexes:
  `PRIMARY(entry_id)`, `(owner_id, loop_id, seq)`, `(owner_id, agent_id,
  created_at)`, `(owner_id, agent_id, kind, created_at)`, `(parent_entry_id)`.
  `(loop_id, seq)` is not a prefix of any of the others, and the unique key is
  the identical key.
- *Plans.* `EXPLAIN PLAN` of every production statement over the table, on a
  3,000-row store, with the index and after `DROP INDEX`: the texts are
  **equal** for all of them. With the index present the planner already reads
  `[run_memory_entries.loop_id,run_memory_entries.seq]`, which is the unique
  key's column list; the plan does not name the duplicate. The statements
  (all other statements over the table are `INSERT`s or by `entry_id`):

  | statement | where | access |
  |---|---|---|
  | `SELECT COALESCE(MAX(seq),0)+1 ... WHERE loop_id=?` | `run_memory.go:55` (tape path), `:108` | `(loop_id, seq)` range, GroupBy MAX |
  | `SELECT entry_id ... WHERE loop_id=? ORDER BY seq DESC LIMIT 1` | `run_memory.go:64`, `:119` | `(loop_id, seq)` range, TopN |
  | `ListRunMemoryEntries`, no owner | `run_memory.go:247` | `(loop_id, seq)` range, Sort |
  | `ListRunMemoryEntries`, owner | `run_memory.go:257` | `(loop_id, seq)` range, Filter owner (the `(owner_id, loop_id, seq)` index is not chosen for the `AND` form) |
  | `ListRunMemoryEntriesForAgent` | `run_memory.go:290` (`runMemoryAgentListQuery`) | the kind index / `(owner_id, agent_id, created_at)`; `loop_id` not used |
  | `ListRunMemoryEntryFingerprints` | `run_memory.go:390` | full scan |
  | `LatestRunMemoryEntry` | `run_memory.go:461` | `(loop_id, seq)` range, TopN |
  | `GetRunMemoryEntry` | `run_memory.go:481`, `:491` | `PRIMARY(entry_id)` |
  | projection read `... WHERE entry_id=? FOR UPDATE` | `project.go:313` | `PRIMARY(entry_id)` |
  | residue run memory | `residue_import.go:409` | `(owner_id, loop_id, seq)` ranges |

  The `INSERT`s are `run_memory.go:134` and `project.go:330`. The test keeps
  a copy of the statement list, with these file:line references, as the thing a
  new statement must be added to.

**Witness. [V]** (`TestRunMemoryLoopSeqIndexDropKeepsTheWitnessSchemaObservation`).
A store with the index and a store without it (both closed, the production
extractor, `computerversion.DoltStateExtractor`) have **equal schema
observations for every table**. `loop_id` is `MUL` through the composite unique
key `UNIQUE(loop_id, seq)` with or without the duplicate, so, unlike the first
kind-index order, the drop moves no `Key`. The control (an index on the keyless
`model` column) does move the observation, so the test can see a drift. The
row hash `dolt_hashof_table` never saw indexes. **[H]** that no off-repo tool
reads the index set.

Because the observation is unchanged, a live store that dropped it and a fresh
store agree in every hashed observation. They still must agree in structure,
and the repo's rule for a table with rows applies regardless of the witness
(no boot-time DDL against a populated table,
[boot-alters-og-objects-when-older-columns-missing-2026-10-21.md](boot-alters-og-objects-when-older-columns-missing-2026-10-21.md)).

**Design.** The kind index in reverse, in `internal/store/run_memory_duplicate_index.go`.

- `schemaDDL` no longer creates it (a fresh store is made without it). A test
  pins that `schemaDDL` does not name it, so no boot recreates it.
- *Open, empty table:* a store that has it and no rows drops it at open (free;
  it makes an old empty store agree with a fresh one). *Open, populated:*
  discovery only, a log line, no DDL.
- *The operation:* one more step of `POST /internal/store/derived-columns?action=migrate`
  after the kind index, `DROP INDEX` under the engine mutex through
  `WithEngine`, step kind `drop_index`, reported in the status as
  `run_memory_loop_seq_index`. A second run finds it absent and does nothing. A
  cancel before the step is `cancelled`, not `done`.
- *Guard:* the drop is refused (the migration fails naming the index, the index
  stays) unless a unique key on exactly `(loop_id, seq)` exists, so the
  statements above can never end up on a table scan.
- No epoch bump: an index is dropped, an older binary only has one fewer.

**Rollback. [V]** (`TestRunMemoryLoopSeqIndexRollbackRecreatesIt`)

- *Revert the commit, store already dropped.* The index stays dropped until the
  reverted binary boots: its `schemaDDL` line is `CREATE INDEX IF NOT EXISTS
  idx_run_memory_entries_loop_seq`, which **rebuilds it at that boot, on the
  populated table, before the listener** (the shape this record refuses for new
  work). The cost is one build of a two-column index; measured at 425,000 rows
  below. The residue of a revert is therefore a one-time boot-time build, then
  the index and its 12-ish KiB of journal per append are back. A revert should
  be a planned update, not a crash restore.
- *Recreate by hand:* `CREATE INDEX idx_run_memory_entries_loop_seq ON
  run_memory_entries(loop_id, seq)`; the test recreates it, the answer is
  unchanged, and the operation drops it again.
- *Revert the commit, store not yet dropped:* nothing changes.

**Built (code `ba04ae68c`) and measured. [V] local**, `TestRunMemoryHoldCensus`
(the R5 section: with the index, dropped by the production operation, without,
recreated, with, dropped, without; a `SELECT 1` probe on `s.db` every 10 ms
through each DDL; 200 appends per phase, the store directory's growth per
append). The kind index is present throughout.

| rows | journal per append with / without loop_seq (two alternations) | saved | drop (operation step) hold of the write connection | CREATE (the rollback's rebuild) hold |
|---|---|---|---|---|
| 50,000 | 46.0, 47.0 / 44.4, 46.0 KiB | about 1.3 KiB (3%) | 58 and 71 ms whole, longest probe wait 2 ms | 80 and 88 ms |
| 100,000 | 48.9, 50.4 / 47.9, 49.2 KiB | about 1.1 KiB (2%) | 60 and 60 ms, longest wait 3 ms | 163 and 137 ms |
| 425,000 | 55.7, 56.1 / 53.1, 53.2 KiB | about 2.75 KiB (5%) | 111 and 73 ms, longest wait 29 ms and 10 ms | 1.12 s and 1.01 s (longest wait 1.10 s) |

**The saving is much smaller than the pairing note above said.** The kind index
cost 12.5 KiB per append; this one (two narrow columns, short keys) is worth
**1.1 to 2.8 KiB per append**, the measure noisy by about 1 KiB between phases.
At 3.7 appends a second that is about 4 to 10 KiB/s, **0.2 to 0.6%** of the
paper's modelled 1.77 MiB/s journal growth, not "about as much" as the kind
index (the journal record's R5 figure of about 30 KiB/s, 2%, is not
reproduced locally). Time per append is unchanged within noise (6.7 to 10.5 ms,
no ordering). It is the right cleanup (a tree nothing reads, a fresh store
made without it) and a small one; the chosen cost is a few hundred lines
of test and a two-statement schema change for 0.2 to 0.6% of the growth. If FP0
weighs it against a restart-bearing deploy, the weight is small. **[H]** the
paper's saving per append is the local one: the paper's tree depth at 425,000
rows is the 425,000-row row above.

**The drop's hold. [V]** 73 to 111 ms at 425,000 rows (a metadata operation on
Dolt: the index tree is released, not rewritten), the longest `SELECT 1` wait
10 to 29 ms, well under `/health`'s 1 s budget; nothing in the paper's
`/health` 503 rate comes from it. The rollback's rebuild is the expensive
direction: 1.0 to 1.1 s of the write connection at 425,000 rows, at the boot of
a reverted binary (before its listener) on a store that had dropped the index.
The paper's own kind-index build took 19.3 s against 1.0 to 1.2 s locally, so a
paper rebuild could be on the order of tens of seconds **[H]**.

**Checks. [V]** focused `-race` on `internal/store` (`-run
'RunMemory|DerivedColumn|BootSteps|TestListRunMemory'`) and on
`internal/autoputer` (`-run DerivedColumn`); the store's
`Open|Schema|Boot|Witness|Index|Epoch|Residue|EventTable` tests; the three gate
scripts and `scripts/selftest-guest-token` (196 passed).

**Rollout. [H]** Deploy; then, with the kind index already built, run
`POST /internal/store/derived-columns?action=migrate` on the paper again: it
finds the kind index present and drops `idx_run_memory_entries_loop_seq` (one
step, `drop_index`), and `GET` reports `run_memory_loop_seq_index.present=false`.
A computer that is not migrated keeps the index (and the journal cost) and
works. No order dependency with the kind index; the operation does kind first.

**Heresy delta.** Discovered: the journal record's R5 estimate (2% of the
growth) and this record's "about as much as the kind index" were not measured
and are off by a factor of 4 to 20 locally (0.2 to 0.6%); a revert of any
index drop is a boot-time build because `schemaDDL` is the old binary's
authority. Introduced: one more `Open` hook (a discovery read and, on an empty
table only, a drop). Repaired: one duplicate tree per append; the fresh-store
and migrated-store structures agree after the operation.

## The index diet: owner_loop_seq and owner_agent_created (red, store schema; lane `fp0-run-memory-index-diet`)

Mutation class: **red** (store schema, `schemaDDL`, `Open`, two more steps of
`POST /internal/store/derived-columns?action=migrate`) plus one **orange** read edit
(`residue_import.go`). Protected surfaces: the store schema (two dropped secondary
indexes), the boot path (one more discovery-or-drop hook, on an empty table only),
the content witness (unchanged for the two dropped, and the reason a third is left
out). Not touched: the epoch, row data, the event tape. Review tier: FP0 judges; my
reading is tier 1 (Devin): additive in effect, behind an operator act, a one-statement
rollback per index. The two things a reviewer should look at are the kind-index
coupling and the reverted-binary boot rebuild below, not the drops.

**The question.** Option 4 of
[store-journal-write-amplification-2026-10-23.md](store-journal-write-amplification-2026-10-23.md):
every secondary index on `run_memory_entries` costs 9 to 13 KiB of journal per append
(one prolly-tree path, rewritten on every statement). Which indexes does no production
statement need?

### 1. The reader audit **[V]**

Every statement over the table, found by a source census
(`TestRunMemoryProductionStatementCensus` counts `FROM|INTO|UPDATE|JOIN run_memory_entries`
in every non-test `.go` file under `internal/` and `cmd/` and fails on a statement the
audit does not know; no other file, script or non-Go source names the table), planned
with `EXPLAIN PLAN` on a 20,000-row store (three owners, 40 agents and a hot desk,
loops of 50 entries chained by `parent_entry_id`), with the full set, with each
candidate dropped, with each pair and with all three
(`TestRunMemoryIndexAuditPlansWithAndWithoutEachCandidate` logs every plan change;
`CHOIR_RUN_MEMORY_AUDIT_PLANS=1` prints the plan texts; `CHOIR_RUN_MEMORY_AUDIT_ROWS`
sets the size).

| index (Dolt name) | columns | tree cost | production readers | decision |
|---|---|---|---|---|
| `PRIMARY` | `entry_id` | pk | `GetRunMemoryEntry` x2 (`run_memory.go:481, 491`), the projection read `FOR UPDATE` (`project.go:313`), the fingerprints `ORDER BY entry_id` (`:390`) | keep |
| `loop_id` (the `UNIQUE`) | `loop_id, seq` | unique | `:55, 64, 108, 119` (seq allocation, parent), `:247, 257` (`ListRunMemoryEntries`), `:461` (`LatestRunMemoryEntry`), the uniqueness itself | keep |
| `idx_..._loop_seq` | `loop_id, seq` | 0.5 to 1.2 KiB | none (its twin is the unique key) | dropped by R5 |
| `idx_..._owner_loop_seq` | `owner_id, loop_id, seq` | 9 KiB | one: the residue import (`residue_import.go`), as written | **drop**, with the residue statement rewritten |
| `idx_..._owner_agent_created` | `owner_id, agent_id, created_at` | 10 KiB | one: `ListRunMemoryEntriesForAgent` with no `kind` filter (`ORDER BY created_at, seq`), which has no production caller | **drop** |
| `idx_..._owner_agent_kind_created` | `owner_id, agent_id, kind, created_at` | the kind index | `ListRunMemoryEntriesForAgent` (`:290`), the only production caller `LoadInboxCursorForDesk` (`rlm_reduce.go:520`) sends an owner and one kind | keep |
| `idx_..._parent` | `parent_entry_id` | 13 KiB | none | **keep: it moves the witness** (section 3) |

Plans, with each candidate. Every statement not named below has a **text-equal** plan
with and without each candidate, for all five subsets (each alone, both owner indexes,
all three), and I pinned the access each uses on the diet's set
(`TestRunMemoryIndexDietEveryStatementKeepsItsAccess`): the loop-keyed statements
(`:55, 64, 108, 119, 247, 257, 461`) read `[loop_id, seq]` (the unique key; R5 already
showed `:257`, the owner form, does not pick `owner_loop_seq`), the by-id ones read
`PRIMARY`, the agent read with one or two kinds reads the kind index with no `Filter`.

- **Residue import, as written, needs `owner_loop_seq`.** `WHERE (e.owner_id=? AND
  e.loop_id=?) OR (...) ORDER BY e.owner_id, e.loop_id, e.seq`:

  ```text
  with:    Sort <- IndexedTableAccess  index [owner_id,loop_id,seq]  filters: two ranges
  without: Sort <- Filter((owner=.. AND loop=..) OR (..)) <- Table   (full scan)
  ```

  A full scan of the table (425,000 rows on the paper) in a rare owner-initiated import.
  That is a worse plan, so by the brief's rule the index stays **unless the statement
  changes**. It now filters `e.loop_id IN (...)` first, in batches of 100 scopes (3
  placeholders a scope; the old statement had 2 a scope with no batch, which also
  capped it at about 32,000 scopes):

  ```text
  with and without owner_loop_seq: Sort <- IndexedTableAccess  index [loop_id,seq]  (the unique key)
  ```

  The scopes are already sorted by (owner, run) and each batch orders by (owner, run,
  seq), so batches in order keep the whole in order. Rows are identical to the old
  statement's, including a loop id under the wrong owner and a loop that does not exist
  (`TestRunMemoryResidueRewriteReturnsTheSameRows`); the production function returns
  every entry once in table order over 230 runs (3 batches) with and without the index
  (`TestSnapshotResidueRuntimeKeepsRunMemoryOrderAcrossManyRuns`); the as-written plan
  is pinned so the premise of the rewrite is visible
  (`TestRunMemoryResidueStatementAsWrittenNeedsOwnerLoopSeq`).

- **The one `ORDER BY created_at` read** is `ListRunMemoryEntriesForAgent` (`:290`); the
  other `ORDER BY`s are on `seq`, `entry_id` or `(owner_id, loop_id, seq)`. With one or
  two kinds it uses the kind index and sorts on `(created_at, seq)` in memory above it,
  with or without `owner_agent_created` (`seq` is in neither index, so the sort is
  never index-ordered). With **no** kind filter (no production caller) it moves from
  `owner_agent_created` to the kind index:

  ```text
  with:    Sort(created_at ASC, seq ASC) <- IndexedTableAccess index [owner_id,agent_id,created_at]
           filters {[owner,owner],[agent,agent],[NULL,inf)}
  without: Sort(created_at ASC, seq ASC) <- IndexedTableAccess index [owner_id,agent_id,kind,created_at]
           filters {[owner,owner],[agent,agent],[NULL,inf),[NULL,inf)}
  ```

  The same (owner, agent) range, the same rows fetched, the same sort; no ordering is
  lost (`TestRunMemoryIndexDietEveryStatementKeepsItsAccess` pins the `Sort`, and
  `TestRunMemoryIndexDietKeepsEveryAnswer` the rows and their order for one kind, two
  kinds and all kinds). The ownerless form (no production caller) is a full scan with
  or without it. A future caller of the no-kinds form pays the kind index's wider key
  (one more column per entry), nothing else.

- **`parent`:** its only reader is a hypothetical `WHERE parent_entry_id = ?`; nothing
  in the repo sends it (`parent_entry_id` is only written and read back in a row).

### 2. Decision per index

| index | decision | evidence |
|---|---|---|
| `owner_loop_seq` | drop, with the residue rewrite | section 1; 9 KiB an append (section 4) |
| `owner_agent_created` | drop | section 1; 10 KiB an append |
| `parent` | **left out** | section 3 |
| `loop_seq` | already R5 | |

### 3. The witness, and why `parent` is out **[V]**

`SHOW COLUMNS` reads `MUL` for the first column of a non-unique index and of a
composite unique one (`computerversion/dolt_state_extractor.go:338`). Observed with the
production extractor on closed stores (`TestRunMemoryIndexDietKeepsTheWitnessSchemaObservation`,
`...WithoutTheKindIndexMovesTheWitness`, `TestRunMemoryParentIndexDropMovesTheWitnessSchemaObservation`):

| drop | Key moves? |
|---|---|
| `owner_loop_seq` and `owner_agent_created`, kind index present | **no**: `owner_id` stays `MUL` through the kind index (it leads with `owner_id`), `loop_id` through `UNIQUE(loop_id, seq)`; the observations of every table are equal to the full set's |
| the same two, **no kind index** (the control) | **yes**: `owner_id` `MUL` -> empty. So the drop is **refused** unless the kind index exists with exactly `(owner_id, agent_id, kind, created_at)` (the first-pass `(agent_id, kind, owner_id, created_at)` order under the same name does not count; tested) and a unique key on exactly `(loop_id, seq)` exists |
| `parent` | **yes**: `parent_entry_id` `MUL` -> empty; nothing else moves |

The `parent` drop can be made to agree between a fresh store and a migrated store (out
of `schemaDDL`, dropped by the operation, as R5 does). What it cannot do is agree by
**vintage**, and the witness is compared across vintages in four places **[V]** (code):

- `replayEligibility` (`agentcore/replay_eligibility.go:235`) compares the live store's
  schema observation with the replay store's; the replay store is `OpenFresh` (current
  schema) or a **staged base** (`replay_completeness.go:444-471`) with whatever index
  set the base was made with. A live store that dropped `parent` against an older base
  (or the reverse, an unmigrated live store against a fresh replay) is `SchemaDrift`,
  and `rebindCheckpointWitness` (`checkpoint_restore_bindings.go:151`) refuses a
  self-development checkpoint on it.
- `projectionbase/seed.go:218` and `verify.go:95` recompute a base's witness and require
  it to equal the descriptor's recorded one.
- `agentcore/rematerialize.go:196` re-extracts and requires equality with the
  checkpoint's recorded witness: a checkpoint recorded with `MUL` cannot be matched by
  a store that has dropped `parent` if the rematerialization starts from a fresh store.
- A store-schema epoch bump, the repo's tool for a witness-moving schema change, is
  refused until LU2 (`hostbuild/types.go:210`).

So `parent` is not an index drop: it is a witness change that needs a coordinated cutover
(bases republished after the drop, pre-drop checkpoints retired or rematerialized from a
base of the same vintage), or an extractor change that makes `Key` insensitive to
non-unique secondary indexes (every recorded witness changes at once, so a witness
epoch). Neither belongs in this lane. The operation never touches it
(`TestRunMemoryIndexDietOperationDropsEachOnceAfterTheKindIndex` asserts it survives) and
`schemaDDL` still creates it (`...FreshStoreLacksTheCandidatesAndKeepsParent`). Its value
is the largest left: 13 KiB an append, a further 25% off a four-tree append (section 4,
measured as a what-if only).

### 4. Built (code) and measured **[V]** local

**Mechanism: R5's, extended** (`internal/store/run_memory_index_diet.go`).

- `schemaDDL` no longer creates the two (a fresh store is made without them; a test
  pins that `schemaDDL` does not name them).
- `Open` drops them from an **empty** table only, when the guard holds; on a table
  with rows it only logs. No boot DDL on a populated table
  (`...IsNotDroppedOrBuiltAtBoot`, including a reopen after the operation).
- The operation: two more steps after the kind index and R5's drop, in order
  `owner_loop_seq`, `owner_agent_created`, each `drop_index` under the engine mutex
  through `WithEngine`. A step whose index is absent does nothing (a second run
  touches none); a refused guard fails the migration naming the index and stops the
  rest (`...IsRefusedWithoutTheKindIndexColumns`, `...WithoutTheUniqueKey`); a cancel
  before a step is `cancelled`, not `done`. The status gains `run_memory_diet_indexes`
  (`name`, `present`, `has_rows`, `guard_ok`, `guard_reason`), in the start probe, the
  final status and the idle read.
- The guard is one function (`dietGuard`): a unique key on `(loop_id, seq)` and the
  kind index with its four columns. R5's discovery now reads the same inventory.
- The migrated store's index set equals a fresh store's
  (`...OperationDropsEachOnceAfterTheKindIndex` compares columns and uniqueness).

**Journal per append** (`TestRunMemoryIndexDietCensus`, the journal census's meter: it
walks the chunk records written in the window, so these are journal bytes; UUID ids and
word-salad text, 200 appends per window, each schema measured twice, alternating; direct
statements, one append each; the table is seeded at the row count):

| rows | today (7 trees) | R5 (6) | + owner_loop_seq dropped | + owner_agent_created dropped = **the diet (4 trees: pk, unique, parent, kind)** | what-if: parent too (3) |
|---|---|---|---|---|---|
| 100,000 | 67.0, 65.6 KiB | 65.8 | 56.6 | **47.1, 47.6** (47.4) | 34.1 |
| 425,000 | 72.3, 73.3 KiB | 72.4 | 63.5 | **53.7, 53.2** (53.5) | 40.3 |

- **The diet saves 18.9 KiB an append at 100,000 rows (28%) and 19.3 KiB at 425,000
  (26%)** against today's 7-tree table (66.3 and 72.8 KiB mean of two windows). Per drop
  (first descent): `owner_loop_seq` 9.2 and 8.9 KiB, `owner_agent_created` 9.0 and 9.9
  KiB, `loop_seq` 1.2 and about 0 (R5, content-addressed twin). The sum matches the
  amplification record's per-tree table (9.6 + 10.1). Noise between two windows of one
  schema is about 1 to 1.5 KiB here, 4 KiB in the amplification record's five repeats,
  so each per-drop figure is good to about 1 KiB and the totals are not in doubt.
- A **tape** append is the same plus a fixed 36 KiB per event, so through the tape at
  100,000 rows the saving is 19 of 103 KiB (18%). On the paper's 3.4 to 3.7 run-memory
  appends a second that is **about 65 to 70 KiB/s, 4 to 7% of the modelled 1.0 to 1.77
  MiB/s journal growth [H]** (the model's rate is the paper's, the per-append number is a
  warm local floor: the paper's trees are deeper). By the amplification record's
  arithmetic (stall share = 18 s/GiB x journal rate) that moves the online GC's share of
  the write connection from 1.76% to about 1.65% and the passes from 3.1 to 2.9 an hour;
  it does not shorten a pass. It is the cheapest honest byte saving left that changes no
  tape op, and it is a fifth of a run-memory append, not a cure for the 503s.
- Time per append is unchanged within noise (5.9 to 9.9 ms, no ordering).

**The holds** (a `SELECT 1` on the write connection every 10 ms through the DDL; "wait"
is the longest one, which is what `/health`'s reads would see):

| | 100,000 rows | 425,000 rows |
|---|---|---|
| `DROP INDEX` (direct, each of the three, 2 runs) | 11 to 13 ms, wait 0 to 1 ms | 12 to 13 ms, wait 0 to 1 ms |
| the operation's three drops (steps `DurationMS`) | 4 to 5 ms each; whole 70 ms, longest wait 6 ms | 6 ms each; whole 132 ms, longest wait 36 ms |
| `CREATE INDEX` (the rollback, see section 5): `owner_loop_seq` | 182 ms, 171 ms | 1.91 s, 1.71 s |
| `CREATE INDEX`: `owner_agent_created` | 193 ms, 182 ms | 1.71 s, 1.60 s |
| `CREATE INDEX`: `loop_seq` (R5's, for the revert of both) | 298, 153 ms | 3.16 s (cold), 1.59 s |
| `CREATE INDEX`: `parent` (what-if) | 137 ms | 1.63 s |

The drops are metadata operations (the tree is released), far under `/health`'s 1 s
budget, and the whole operation is 132 ms of write-connection time at 425,000 rows. A
`CREATE` holds the connection for the whole build (the longest probe wait equals the
build).

### 5. Rollback, and what a reverted binary does at boot **[V]**

- *Recreate by hand, exactly what the old `schemaDDL` ran:*

  ```sql
  CREATE INDEX idx_run_memory_entries_owner_loop_seq ON run_memory_entries(owner_id, loop_id, seq);
  CREATE INDEX idx_run_memory_entries_owner_agent_created ON run_memory_entries(owner_id, agent_id, created_at);
  ```

  Local timing above: 0.17 to 0.19 s each at 100,000 rows, 1.6 to 1.9 s each at 425,000.
  The answers are unchanged across the recreate and the operation drops them again
  (`TestRunMemoryIndexDietRollbackRecreatesThem`).
- *A reverted binary, store already dropped:* the same as R5, for both. The old
  `schemaDDL` has `CREATE INDEX IF NOT EXISTS` lines for them, applied by `bootstrap`
  at every `Open` **on the populated table, before the listener**; the test runs those
  exact statements on the dropped store and they rebuild the indexes. That is the shape
  this record refuses for new work, and it holds here. **The honest cost of a revert:**
  two builds at boot, 3.3 to 3.6 s locally at 425,000 rows (about 5 s if R5 is reverted
  too, which adds `loop_seq`). The paper's kind-index build took 19.3 s against 1.0 to
  1.2 s locally (17 times); scaled by that one ratio the paper's boot would wait
  **about 60 s for the two, about 90 s with R5 [H]**, with the listener down (the ratio
  is from one index; the true figure could be half or double). No data risk (`IF NOT EXISTS`, same columns), and the
  indexes and their 19 KiB of journal an append come back.
- *The fix, built (red, same lane): `restore_run_memory_indexes`.* The old binary's DDL
  cannot be changed, so the build moves off the boot path. `POST
  /internal/store/derived-columns?action=restore_run_memory_indexes` (host peer only like
  `migrate`; loopback gets 403; a `GET` never runs it) recreates, online with the listener
  up, every index R5 and the diet dropped, **in this order: `owner_agent_created`,
  `owner_loop_seq`, `loop_seq`** (`owner_agent_created` first, for the witness coupling
  below: it is the second owner-leading index, so a kind-index rollback that follows is
  witness-neutral again). Optional `&indexes=a,b` restores a subset (a revert of the diet
  that keeps R5 does not need `loop_seq`); an unknown name is a 400 before anything runs.
  It is **idempotent**: an index that exists is reported `present` and costs one catalog
  read, so a second call, or a call on a computer that never dieted, builds nothing
  (200, all three `present`, `total_hold_ms: 0`). Each build takes the engine mutex for
  its length like a drop does, so the write connection is released between builds. It
  refuses (409) while a migration runs and a migration refuses while it runs. The 200
  body is the report:

  ```json
  {"steps":[{"name":"idx_run_memory_entries_owner_agent_created","action":"created","hold_ms":541},
            {"name":"idx_run_memory_entries_owner_loop_seq","action":"created","hold_ms":317},
            {"name":"idx_run_memory_entries_loop_seq","action":"created","hold_ms":269}],
   "total_hold_ms":1127}
  ```

  It is synchronous (the holds are the request's length); if the request is cut, the
  builds that finished stay and a retry resumes (idempotent). Proven by tests: a restore
  after the diet gives a store whose **index set, schema observation, every audited plan
  and every answer equal the pre-diet store's** (`TestRunMemoryIndexRestoreAfterTheDietEqualsThePreDietStore`);
  a second restore and a restore on a store that never dieted are no-ops (the table hash
  does not move); **after a restore the old binary's `CREATE INDEX IF NOT EXISTS` lines
  run and leave the table untouched**, i.e. build nothing
  (`...LeavesAnOldBootDDLANoOp`); non-host callers get 403
  (`TestDerivedColumnsRestoreEndpoint`).

  **Measured holds of a restore** (the census, after the operation, three builds; the
  longest `SELECT 1` wait equals the longest single build):

  | rows | `owner_agent_created` | `owner_loop_seq` | `loop_seq` | total | second restore |
  |---|---|---|---|---|---|
  | 100,000 | 540 ms | 316 ms | 268 ms | 1.12 s (longest wait 542 ms) | 0 ms, 24 ms request |
  | 425,000 | 5.28 s | 1.64 s | 1.82 s | 8.74 s (longest wait 5.27 s) | 0 ms, 54 ms request |

  The first build after the drops is the slow one (5.3 s at 425,000 rows, against 1.6 to
  1.9 s for a warm build in the earlier census; cold chunk reads). Scaled by the kind
  index's paper/local ratio (17 times) a build on the paper is **about 30 s, the first
  perhaps 90 s, about 2.5 minutes in all, in three separate `/health` 503 windows [H]**.
  That is the honest cost of a revert, paid at a time the operator picks with the
  listener up, instead of the same 2.5 minutes (60 s for the two, 90 s with R5) at the
  reverted binary's boot with the listener down. Prefer an idle moment (the deploy's
  deferred-refresh sweep); the always-busy computers take the same builds as a planned,
  marker-backed refresh either way.

**Rollback procedure for a revert of R5 and/or the diet (ordered).**

1. On each computer that ran the operation (`GET /internal/store/derived-columns` shows
   `run_memory_diet_indexes[*].present=false`, or `run_memory_loop_seq_index.present=false`):
   check no migration is running (`state != "running"`), then `POST
   /internal/store/derived-columns?action=restore_run_memory_indexes` (for a revert of the
   diet only, add `&indexes=idx_run_memory_entries_owner_agent_created,idx_run_memory_entries_owner_loop_seq`).
   Expect 200 and `created` for each missing index; read `total_hold_ms`. A failure (500)
   or a cut request leaves finished builds in place: call it again.
2. `GET` again and confirm all three are present (`run_memory_diet_indexes[*].present`
   and `run_memory_loop_seq_index.present` are re-read from the inventory when a restore
   ends, success or failure; `restoring` is true while one runs). (Equivalently the
   second `POST` returns all `present`.)
3. **Then** deploy the revert. Its boot DDL finds the indexes and builds nothing. A
   computer that never ran the operation needs no step 1.
4. If the kind index is also being rolled back (`DROP INDEX`), do it only after step 1:
   `owner_agent_created` must exist first or `owner_id`'s Key moves (below).
5. Do not deploy the revert first and restore after: the reverted boot builds the
   indexes itself, on the boot path.

- *Revert the commit, store not yet migrated:* nothing changes.
- **The kind index is now load-bearing for the witness.** After the diet it is the only
  index that leads with `owner_id`. The kind-index record's rollback ("DROP INDEX is the
  whole rollback") was witness-neutral while the two owner indexes existed; it no longer
  is: dropping the kind index on a dieted store moves `owner_id` `MUL` -> empty
  (`...WithoutTheKindIndexMovesTheWitness` is exactly that store). So a rollback of the
  kind index must recreate `owner_agent_created` first (or accept the drift). The guard
  stops the operation from creating the situation; nothing stops a manual drop. So
  `Open` now says so: when neither the kind index (with its four columns) nor
  `owner_agent_created` exists, the diet hook logs `run-memory: no (owner_id, agent_id)
  index exists ... restore with ...action=restore_run_memory_indexes or build the kind
  index with action=migrate` and returns the error, so the boot phase is logged as
  failed (never a failed `Open`). It is quiet in every state that has one of them: a
  fresh store, a pre-diet store, a dieted store
  (`TestRunMemoryOpenWarnsWhenNoOwnerAgentIndexExists`).

### 6. Rollout **[H]**

Deploy, then run `POST /internal/store/derived-columns?action=migrate` on the paper:
the kind index is already built, so the steps are `loop_seq` (R5), `owner_loop_seq`,
`owner_agent_created` (three `drop_index`, about 0.13 s in all at 425,000 rows locally),
and `GET` reports `run_memory_diet_indexes[*].present=false`. A computer that is not
migrated keeps all three indexes and works. A fresh computer is made without them. In a
mixed fleet a fresh store and an unmigrated live store agree on `owner_id` (`MUL` through
the kind index and through the legacy indexes), so no `SchemaDrift` appears during the
rollout; that is exactly the property `parent` lacks.

Checks: focused `-race` on `internal/store -run 'RunMemory|DerivedColumn|Witness|Index'`, `internal/autoputer -run DerivedColumn`; the
four gates; gofmt. Two existing kind-index tests encoded the old index set and are
updated, not weakened: the all-kinds plan now expects the kind index with its sort
(`TestRunMemoryKindIndexPlanHasNoFullRangeWalk`), and the kind-index witness test adds
the legacy owner indexes to both stores it compares so it still tests "the kind index
adds no Key".

**Heresy delta.** Discovered: the residue import's statement needed an index only it
used (a full scan without it) and had a placeholder cap; the kind index becomes
load-bearing for the witness once the owner-leading indexes go (its documented rollback
is no longer witness-neutral); a revert of any index drop is a boot-time build on the
populated table (`schemaDDL` is the old binary's authority); `parent` is the largest
remaining unread tree and cannot be dropped without a witness cutover. Introduced: one
more `Open` hook (a discovery read, a warning, and on an empty table only two drops);
two more operation steps; a batched residue read; the `restore_run_memory_indexes`
action. Repaired: two unread trees per append (19 KiB of
journal, 26 to 28%), and a latent full scan in the residue import.

## Next (not in this pass)

Shape 1 (keyed cursor state, red), shape 3 (the `(owner_id, agent_id, kind,
created_at)` index with the `details_json` projection, orange, built by the
online access-path migration, never at boot), shape 4 (the in-memory cursor)
and shape 6 (retention). Read the new log line after the next refresh before
choosing between them.

Index diet follow-up: `idx_run_memory_entries_parent` (13 KiB an append, no reader)
needs a witness cutover (section "The index diet", 3).

## Not asked, seen

- `ListRunMemoryEntryFingerprints` (`run_memory.go`) reads every run-memory
  row with every text column (`ORDER BY entry_id`) for owner-authorized replay
  diagnostics; it is not in any stall line, and it is the same shape at table
  size (425,000 rows). **[H]** worth a look when a diagnostic is run on the
  paper.
- `readHealthStore` uses `s.db`; `/health` shares the write connection with
  every writer, so any holder over 1 s is a 503 by construction, whichever
  function it is. The 1.3% is a measure of connection busyness, not of one
  query. **[V]** (`api.go:767`; header `max_open=1`)

## How this was measured

- Worktree `lane/fp0-run-memory-hold` off `origin/main` (`7ca537e01`). Local
  embedded Dolt only; no SQL against production Dolt; no API call to any
  computer. The console was read with the `ssh node-b 'sudo grep -a
  store_stalled ...'` above; the two probe files were already on disk. Tool
  repo gates (`check-test-hang-guards`, `check-decode-roots`,
  `check-v1-writer-purity`) pass.
- `TestRunMemoryHoldCensus` (env-gated, repeatable):

  ```text
  CHOIR_RUN_MEMORY_HOLD=50000:5000,200000:20000,425000:100000,425000:300000 \
    go test ./internal/store -run TestRunMemoryHoldCensus -v -count=1 -timeout 60m
  CHOIR_RUN_MEMORY_HOLD_GC=1 CHOIR_RUN_MEMORY_HOLD=100000:20000 \
    go test ./internal/store -run TestRunMemoryHoldCensus -v -count=1
  ```

  Each size is TOTAL:HOT. Rows are inserted directly (not through
  `AppendRunMemoryEntry`, which only adds the tape projection). It asserts
  the plan reads the `(owner_id, agent_id, created_at)` index and nothing
  about time. The table above is its output. Allocation is `TotalAlloc` over
  the call, not peak heap. The 5,000, 50,000 and 100,000 rows used the first
  pass; the 200,000 row used the second.
- Parsing of the console (a join of 98 headers and 101 holder lines to the
  probe timestamps) was a scratch script, deleted.

## What this does not settle

1. The desk-agent row counts on the paper and the kind mix (no production
   SQL was run). The size at which the function reaches 1 s warm (about
   350,000 desk rows) is an extrapolation of a local fit.
2. Why the paper's episodes are 20 to 100 times the local warm hold. The
   stacks (`nbs.tableSet.get`, `ZSTD_decompress_usingDDict`,
   `fileReaderAt.ReadAtWithStats`) point at chunk fetch and decompression; a
   local GC and engine reopen did not reproduce it. A request-scoped log on
   `ListRunMemoryEntriesForAgent` (rows, elapsed, caller site) on the next
   refresh would give the real per-call distribution and the call rate in one
   step; so would the `engine_mutex` per-op stats if they covered raw `s.db`
   callers, which they do not.
3. Which of the three call sites is the long one.

## Heresy delta

- Discovered: the top steady-state stall holder reads a whole desk's rows to
  take a maximum, twice per desk cell, on the connection `/health` shares;
  `recoverPartialActCommit` loads the cursor before it knows it needs it;
  the `ownerID == ""` form of the function is a full table scan; the stall
  log keeps one holder a minute and records no duration; the 1.3% steady-state
  `/health` 503 rate is a measure of write-connection busyness, not of one
  query.
- Introduced: none (a yellow test, a green record).
- Repaired: none.

## Rollback

Revert the commit that adds `internal/store/run_memory_hold_census_test.go`
and this record. No runtime, schema, data or index change.
