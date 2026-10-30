# What still stalls the paper's store after the run-memory index (2026-10-23)

Date: 2026-10-23, 13:30 UTC. Found by: FP0 worker, lane `fp0-stall-holders`
(investigation only; nothing is fixed here). Status: open. Mutation class of
this record: green (docs) plus one yellow test (`TestStallHoldersCensus`,
env-gated). Mutation class of the fixes named below: orange, and red where it
says so. Claims are marked **[V]** verified (a plan, a measurement, a log line
or the code, named beside it) or **[H]** hypothesis.

Parent record, whose method this follows:
[run-memory-entries-hold-the-engine-2026-10-23.md](run-memory-entries-hold-the-engine-2026-10-23.md)
(holder ranking from the console's `store_stalled holders:` snapshots, joined
to the `/health` probe files; an env-gated census test on a seeded local
Dolt store). Records cited, not repeated, for the GC:
[paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md](paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md)
(GC holds the one write connection; time about 16.6 s per GiB),
[paper-unreachable-minutes-after-every-boot-2026-10-22.md](../../problems/paper-unreachable-minutes-after-every-boot-2026-10-22.md)
(F2, H4: the first pass after a boot),
[paper-post-refresh-health-windows-2026-10-22.md](../../problems/paper-unreachable-minutes-after-every-boot-2026-10-22.md)
(the four 503 bursts inside GC windows, the holder log's origin),
[guest-dolt-storage-growth-2026-10-15.md](guest-dolt-storage-growth-2026-10-15.md)
(FM5, online GC and sessions).

## The question

After the run-memory kind index was built on the paper (12:30 UTC), the steady
`/health` 503 rate fell from 3.0% to 0.17%. Four things were left on the list:
`selfdev.(*Store).ListByStates`, `objectgraph.scanSideRows`, `runOnlineDoltGC`,
and `GET /api/runs` (timed out at 90 s). What is each one's query and plan, how
often is it called, what does it hold as data grows, and how much of what is
left does it explain?

## Answer

**One holder explains the remaining 503s: the online Dolt GC. [V]** In the 30
minutes after the index (12:30:20 to 13:00:25 UTC, 1,103 probe samples) 16
samples were not 200. 13 sit inside a GC pass (the console's own
`dolt gc (online) starting` and `complete in` lines). One more is a 503 in the
minute whose snapshot names `scanSideRows`. Two are 3 s timeouts 5 and 7
minutes after a boot. None belongs to `ListByStates` or `GET /api/runs`.

| rank | holder | share of the 16 non-200 samples after the index | share of its own evidence | what it is |
|---|---|---|---|---|
| 1 | `runOnlineDoltGC` | **13 of 16 (81%)**; 3 of 3 once the boot and the index build are past (12:36:06 to 13:00:25, 797 samples) | 18 passes, 17 to 24 s each, 1.56% of wall time | the journal reaches 1 GiB about every 20 minutes; `CALL DOLT_GC()` holds the write connection for the pass |
| 2 | `objectgraph.scanSideRows` | 1 of 16 (6%); up to 3 of 16 (19%) if the two boot-window timeouts are the validation pass **[H]** | 2 snapshots (09:48:03, 12:34:57), both 9 to 12 minutes after a boot | one page of the post-boot derived-key validation, the first cold read of a family |
| 3 | `selfdev.(*Store).ListByStates` | **0** | 2 snapshots (10:42:15, 12:51:04) | `/health`'s own read; the snapshot caught it as the first mover after the real holder let go |
| 4 | `GET /api/runs` | **0** observed; not on any periodic path | 3 cancelled `list runs` queries at 09:44 to 09:47 | two full scans of the run kind; 22 s warm locally at the paper's 34,000 runs, quadratic in the legacy half |

The brief's picture of rank 3 needs correcting. The 12:50:59 503 is the
GC pass of 12:50:45 to 12:51:04, not `ListByStates` (section 3). **[V]**

The "0.17%" is a window with one GC pass and 5 s probe timeouts. By time the
GC alone is **1.56%** of the paper's wall time (353 s in 22,557 s,
06:35:07 to 12:51:04), about nine times that. Sample-count rates are not
comparable across probe files; time-share is. **[V]** arithmetic (section 1).

**Cheapest next fix. [H]** Issue the GC on the read pool's connection instead
of `s.db` (`runOnlineDoltGC`, one line) and print `online_gc=active` in the
stall header. Locally, at the paper's journal size, the pass then blocks the
write connection for 30 to 38% of its length instead of 100% (28 to 58% over
all sizes), with no failed writer and no lost row (section 1). On the paper that would turn 19 s of stall into about 7 s [H].
It is a red-class review (a Dolt maintenance procedure on another session,
FM5 in the storage-growth record). **Superseded by section 1a: not safe.
A pass on `s.readDB` fails an open write transaction ("chunk ... required but not
found"), because the embedded driver never registers sessions with Dolt's GC
safepoint controller. Do not build it.** If the aim is zero 503s from the GC rather
than a shorter stall, the cheapest is `/health`'s three store reads on
`s.readDB`, which a GC on `s.db` does not block (0.04 to 0.07 s locally); that
trades the GC for object-graph reads as the thing that can stall `/health`.
Both are laid out in section 1.

## The evidence base

- Paper console, read-only: `ssh node-b 'sudo grep -a ...
  /var/lib/go-choir/vm-state/vm-1669020adcece51c2f5ff9b6df399ef5/console.log{,.1,.2,.3,.4}'`.
  The logs cover 06:28 to 13:02 UTC and seven boots (06:52:53, 07:28:43,
  08:05:28, 09:36:28, 10:06:02, 10:41:13, 12:25:39). 76 `store_stalled
  holders:` snapshots (06:35:29 to 12:51:04) and 40 `dolt gc` lines. No SQL
  against production Dolt; `GET /api/runs` was not repeated on the paper.
- Probe files: `/Users/wiz/Dev/lanes/_runs/deployD15/paper-health-d15.txt`
  (10:21:47 to 11:21:45, 1,674 samples, before the index),
  `/Users/wiz/Dev/lanes/_runs/paper-index/build.txt` (12:29:48 to 12:40:06) and
  `health-after.txt` (12:40:27 to 13:00:25, 574 samples).
- Join rule: a non-200 sample is a GC sample if it starts within 1 s before
  to 3 s after a GC pass; a boot sample if it is within 60 s before to 420 s
  after a boot; an index-build sample from 12:29:50 to 12:30:30; otherwise it
  takes the holder of a snapshot that follows it by up to 62 s (the log keeps
  one snapshot a minute and takes it after the answer). **[V]** method.

| window | samples | non-200 | GC | `ListRunMemoryEntriesForAgent` (snapshot) | boot | index build | `scanSideRows` | unattributed |
|---|---|---|---|---|---|---|---|---|
| D15, before the index (10:21:47 to 11:21:45) | 1,674 | 72 | 21 | 28 | 15 | 0 | 0 | 8 |
| after the index (12:30:20 to 13:00:25) | 1,103 | 16 | 13 | 0 | 2 | 0 | 1 | 0 |
| after, boot and build past (12:36:06 to 13:00:25) | 797 | 3 | 3 | 0 | 0 | 0 | 0 | 0 |

The 8 unattributed D15 samples all sit within 90 s of a run-memory snapshot
and behind the one-a-minute limit (10:32:22, 10:53:48, 10:54:08, 10:54:15,
10:56:11, 10:56:16, 11:13:06, 11:13:18) **[H]** the same holder. The index
took the run-memory share from 28 of 72 (39%) to 0 of 16, and the GC went
from 29% to 81% of what is left. **[V]**

## 1. `runOnlineDoltGC`

**Trigger and code. [V]** `internal/autoputer/run.go:727` starts
`StartPeriodicDoltGC` with a 5-minute ticker. Each tick `maybeRunDoltGC`
(`internal/store/dolt_maintenance.go:340`) plans: run when the noms journal is
at least `min(1 GiB, avail/4)`, floor 64 MiB (`RUNTIME_DOLT_GC_JOURNAL_GIB`,
default 1; with 84 GiB free the 1 GiB limit binds). The pass is
`CALL DOLT_GC()` on `s.textureHandle()`, which is `s.db` (the one write
connection, `MaxOpenConns(1)`) while it holds `doltCommitMu`
(`dolt_maintenance.go:327`). `/health`'s three reads use `s.db`
(`readHealthStore`, 1 s budget), so a pass longer than 1 s is a 503 by
construction.

**Passes today. [V]** (console, 06:35:07 to 12:51:04): 18 online passes and 2
boot passes (the boot ones run before the listener, 22 s and 20 s).

| | value |
|---|---|
| online passes | 06:35, 07:23, 07:43, 08:10, 08:30, 08:45, 09:05, 09:25, 09:46, 10:21, 10:36, 10:51, 11:11, 11:26, 11:51, 12:16, 12:35, 12:50 |
| journal at start | 1,025 to 1,342 MiB (mean 1,148) |
| duration | 17 to 24 s (mean 19.6 s); fit 18 s per GiB on this range |
| gap between passes (no boot between) | 14.6 to 26.5 min, median 19.7 (the 5-minute tick quantises it) |
| journal growth between passes | 0.66 to 1.36 MiB/s, mean 1.0 (the 03:xx record had 2.0 to 2.7) |
| stalled share of wall time | 353 s of 22,557 s = **1.56%** |

The 03:xx record fitted 16.6 s per GiB plus about 2 s from four passes of 1.0
to 1.8 GiB. Today's range is too narrow to separate an intercept
(1,025 MiB took 17 s, 1,342 MiB took 21 s). **[V]**

**What the snapshots show. [V]** 15 of the 76 snapshots name it; 7 stand in
`nbs.journalChunkSource.getManyCompressed` (reading journal chunks), 7 in
`nbs.archiveReader.has/search` (membership against the archives), 1 in
`markAndSweeper.SaveHashes`. All 15 headers show `write_pool in_use=1`. The
pass is chunk reading, not CPU. **[H]** that this is why the paper is about
6.6 times slower than the local pass of the same journal (19 s against 2.9 s
at 1.1 GiB).

**503 match. [V]** Two complete windows in the after files:

- 12:35:45 to 12:36:05: 10 consecutive 503 samples at 1.0 s each
  (`build.txt`, 12:35:45 to 12:36:03); the console shows the starting and
  complete lines and the 12:35:58 snapshot names `runOnlineDoltGC`.
- 12:50:45 to 12:51:04: 3 samples, two 5 s timeouts (000) and a 503 at
  12:50:59 that took 4.589 s. The `complete in 19s` line is at 12:51:04. The
  probe timeout there was 5 s, so a 19 s pass yields 3 samples; with a 1 s
  failure it yields about 7. This is why the sample rate understates.

In D15 the three complete windows (10:36:26, 10:51:18, 11:11:18) hold 21
samples, 7 a pass. **[V]**

**Two observations that bear on the fix. [V] observed, [H] cause.**
(a) `/health` is not always answered within its own 1 s budget during a pass:
the 12:50:59 request took 4.589 s to return 503 and two others timed out at
5 s; 10:53:48 in D15 took 1.53 s. The console is silent from 12:50:45 to
12:51:04 (no line in 19 s, against one every second or two otherwise), so the
process, not only the connection, was blocked. A request-scoped timing in
`HandleHealth` would say where. (b) The 12:51:04 snapshot names
`ListByStates` because it was taken 134 ms after the pass returned (section
3).

**Local measurement (`TestStallHoldersCensus/gc`). [V]** Embedded Dolt,
`run_memory_entries` filled to a journal size, then one pass; a probe goroutine
issues `SELECT 1` every 5 ms and keeps the longest wait.

| journal | pass on `s.db` (the code today) | longest `SELECT 1` on `s.db` | reads on `s.readDB` meanwhile | the same pass issued on `s.readDB` | longest wait on `s.db` while it runs | writer (1-row INSERT every 20 ms) |
|---|---|---|---|---|---|---|
| 131 MiB | 0.49 to 0.69 s | the whole pass | 0.03 to 0.04 s | 0.52 to 0.68 s | 0.20 to 0.31 s (36 to 58%) | 8 inserts, 0 failed, rows intact |
| 513 MiB | 1.15 to 1.51 s | the whole pass | 0.04 to 0.05 s | 1.50 to 1.76 s | 0.44 to 0.72 s (28 to 41%) | 21 inserts, 0 failed, rows intact |
| 1,103 MiB | 2.88 to 2.96 s | the whole pass | 0.07 s | 3.04 to 3.62 s | 1.09 to 1.20 s (30 to 38%) | 50 inserts, 0 failed, rows intact |

- Issued on `s.db`, the pass blocks the write connection for its whole length
  (the probe wait equals the pass), as the code says. Local pass time is about
  2.5 s per GiB with an intercept near 0.2 s; the paper's is about 18 s per
  GiB.
- A read on `s.readDB` (a trivial one and one on a table) is not blocked by a
  pass on `s.db`: at most 0.07 s.
- Issued on `s.readDB`, the pass is slightly longer but `s.db` waits only
  28 to 58% of it (30 to 38% at 1.1 GiB), and every concurrent insert
  succeeded with every row present afterwards (the writer checks its own row
  count). An intermediate run of this section reported failed inserts; they
  were duplicate keys from the test's own writer reusing a sequence, fixed
  before the second test commit.
- The local store is warm and small; the paper's pass is IO-bound. The ratios
  are the portable part. **[H]** that they hold on the paper.

**Fix shapes. [H] each, ordered by cost; none built.**

1. **Name the pass (instrument, orange). Built on `lane/fp0-online-gc-session`:** `Store.OnlineGCStatus` (set by `runOnlineDoltGC` under `doltCommitMu`, cleared by a defer on every exit), the stall header field `online_gc=active since=<RFC 3339>` or `online_gc=idle`, and `store_pools.online_gc` `{"active":bool,"since":...}` on `/api/runtime/observability`. `/health`'s answer is unchanged. An in-process flag set by
   `runOnlineDoltGC`; the stall header prints `online_gc=active
   since=<t>`. Then the 12:51:04 snapshot would have said GC. It removes no
   503. Smallest, and part of every other shape.
2. **Run the pass on the read pool's connection (orange, review as red).**
   `s.readDB.ExecContext(ctx, "CALL DOLT_GC()")`, still under
   `doltCommitMu`. Writers and `/health` wait about a third of the pass
   instead of all of it: 19 s becomes about 7 s [H], which is still over the
   1 s budget, so `/health` still answers 503 for about 7 s a pass (about 2
   samples against 7). Needs the FM5 questions answered again on the
   paper: sessions, the safepoint controller, a writer that parks in the
   keeper phase (`store/nbs/store.go` `waitForGC`, cited in the journal
   record). The local run is not a safety proof.
3. **`/health`'s three reads on `s.readDB` (orange).** Three call sites:
   `CountPendingAgentMutations`, `selfdev.Store.ListByStates` (a second
   provider) and `Store.Head`. A pass on `s.db` does not block them (0.04 to
   0.07 s locally), so the GC 503s go to zero. Costs: the read connection is
   itself held by object-graph reads (6 of 76 snapshots show `read_pool
   in_use=1`; `GET /api/runs` held it 2.9 s warm), so some 503s remain; and
   `/health` no longer signals a stuck write connection, the F1 intent (an
   answer that is not a 200 while the writer is stuck). The updater already
   treats a run of `store_stalled` answers as wait-and-see within a
   wall-time budget (`internal/updater/runtime.go:152`), so these 503s do not
   fail a deploy today.
4. **Report maintenance, not a stall (orange).** When the flag is set and the
   reads miss their budget, answer 200 with `store.maintenance=dolt_gc`. A
   one-branch change; it removes the 503s and changes the meaning of
   `/health`. A relabel, not a repair; keep the stall share (GC seconds over
   wall seconds) as the metric if it is taken.
5. **Collect less journal per pass (the substrate).** Pass time is
   proportional to the journal (18 s per GiB on the paper); the journal
   grows about 1.0 MiB/s from write amplification (about 90 KiB of journal
   per run-memory append, and the duplicate index `idx_run_memory_entries_loop_seq`,
   R5 of the journal record). Less growth is a shorter and rarer pass.
6. **Smaller, more frequent passes: probably not. [H]** If the paper's fixed
   cost is the 2 s of the 03:xx fit, no pass gets under the 1 s budget and the
   count of 503s rises. It needs a MiB-granular threshold and a one-minute tick
   (not available today) and a paper measurement of the intercept. Locally the
   intercept is 0.2 to 0.3 s.

## 1a. Is a pass on the read pool's connection safe? No (2026-10-23, FP0 worker, lane `fp0-online-gc-session`)

Mutation class of this section: green (docs) plus one yellow test
(`TestOnlineGCSessionOpenTransaction`). Dolt module
`github.com/dolthub/dolt/go v0.40.5-0.20260716231223-6bea93f2f665`, driver
`github.com/dolthub/driver/v2 v2.2.0` (go.mod). Paths below are under the module
cache (`go env GOMODCACHE`) in `dolt/go@v0.40.5-0.20260716231223-6bea93f2f665/`
unless they name the driver or go-mysql-server. Shape 2 above is **not built**:
the write connection is not killed, but a write transaction open on it when the
pass starts is destroyed. **[V]**

**Q1. Is `DOLT_GC` safe from a session other than the writer's? Not with this
driver. [V]** The procedure is written for it. `doDoltGC`
(`libraries/doltcore/sqle/dprocedures/dolt_gc.go:208-276`) takes whatever session
calls it, and the default safepoint controller is session-aware (`:57`
`UseSessionAwareSafepointController = true`; the other mode needs
`DOLT_GC_SAFEPOINT_CONTROLLER_CHOICE=kill_connections`, `:47-52`; nothing in this
repo sets it). The session-aware controller (`:169-206`) keeps other sessions'
state in two steps: `BeginGC` visits the calling session's roots, then
`gcctx.GCSafepointController.Waiter` (`libraries/doltcore/doltdb/gcctx/gc_safepoint_controller.go:141-185`)
waits for every *other registered* session to end its current command and has
`DoltSession.VisitGCRoots` (`libraries/doltcore/sqle/dsess/session.go:803-882`)
keep that session's branch heads, working-set roots, open transaction's initial
root, savepoints and its write session's pending-edit roots
(`store/prolly/tuple_mutable_map.go:67-80`, which flushes pending edits and their
out-of-line values so the visited root reaches them). A session is registered
only by `SessionCommandBegin` (`gc_safepoint_controller.go:256-286`), which
go-mysql-server's wire handler calls (`go-mysql-server@v0.20.1-0.20260715210958-36c4fed9afa6/server/handler.go:140,193,228,449`,
`server/context.go:188`). **The embedded driver never calls it**:
`grep -rn 'SessionCommandBegin\|CommandBegin\|SessionEnd\|gcctx\|Safepoint'` over
`driver/v2@v2.2.0` (and over the cached `v2.4.1`) finds nothing; its
`newLocalContext` (`connector.go:50-68`) and `beginQuery`/`endQuery` (`conn.go:106-140`)
only touch the process list. So under this driver the controller's session map
is empty: the pass waits for no session, and visits no session's roots except
the caller's. Today (`s.db`) the caller is the writer, whose roots are visited,
and the writer cannot be mid-transaction because the pass holds its one
connection. On `s.readDB` the caller is the read session, and the writer is
invisible.

**Q2. What does it lock; does it kill sessions? It locks the whole chunk store
for a short window, and kills nothing. [V]**

- Default controller: `EstablishPostFinalizeSafepoint` is a no-op (`:198-200`);
  the kill-and-poison behaviour (`:120-164`: `KillConnection` on every other
  process, `SetValidateErr(ErrServerPerformedGC)` on the caller, `:71`) belongs to
  `kill_connections` only. No connection is killed or invalidated.
- Mark phase: `nbs.BeginGC` sets `gcInProgress` and `keeperFunc`, disables conjoin
  and waits out a running conjoin (`store/nbs/store.go:2183-2230`). Chunk reads
  and writes by any session then pass through the keeper and are retained.
- Finalize: `ValueStore.transitionToFinalizingGC` (`store/types/value_store.go:478-490`)
  sets `gcState_Finalizing` and waits for in-flight critical sections. While it
  holds, `ValueStore.Put` and `Commit` (`:357`, `:541-545`, via
  `waitForNotFinalizingGC` `:414-440`) and the NBS `addChunk` keeper
  (`store/nbs/store.go:1054-1070`, `waitForGC` `:960-973`) park for **every**
  session, not only the caller's. This is the 28 to 58% of the pass the writer
  still waits on locally in the table above, whichever connection the pass is
  issued on. `EndGC` waits for outstanding reads (`:2232-2250`).
- The pass takes no engine mutex of the store's own (`objectgraph.DoltStore`
  engineMu); it is a raw `ExecContext`. On `s.readDB` it holds that pool's only
  connection (`configureEmbeddedDoltDB`, `texture.go:354`), so every object-graph
  read (`ogReadStore`) queues behind the whole pass; a cost the table above did not
  measure (it measured `SELECT 1` on `s.db` and table reads only on `s.readDB`
  *while the pass ran on `s.db`*).

**Q3. Can a read-pool session run it? Yes; it is not read-only. [V]**
`readDB := sql.OpenDB(poolConnector{connector})` (`internal/store/store.go:893`)
is a second pool over the same connector and engine as `s.db`, one session per
connection, `root@%` (`connector.go:64`). Nothing opens it read-only.
`branch_control.CheckAccess(ctx, Permissions_Write)` (`dolt_gc.go:214`) passes
with no branch-control tables, and the census ran the pass on it
(`TestStallHoldersCensus/gc`).

**Q4. What happens to the write connection's open transaction? It fails at its
next statement. [V] by test.** The connection survives (no kill, no
`ErrServerPerformedGC`), and the committed state is intact. A transaction open
across the pass loses the out-of-line chunks it wrote before the pass, because
no root reaches them and the session is not visited. Test
`TestOnlineGCSessionOpenTransaction` (`internal/store/online_gc_session_test.go`):
the writer opens a transaction on `s.db`, inserts 40 rows of 300 KiB text, a
`CALL DOLT_GC()` is issued on `s.readDB` (it returns nil in about 50 ms), and the
writer's next statement fails:

```text
insert 40: Error 1105: malformed database: chunk 4de477q3706g4vhhj0j98iq6lnkbf4sa required but not found
```

7 of 7 runs, two with `-race`; the same hash each time. After `Rollback` the 200
baseline rows are intact on both connections, a write succeeds and a second pass
succeeds, so this is a failed write, not store corruption. The control arm (the
pass on `s.db` between two committed halves, today's shape) commits every row.
A single autocommit statement racing the pass (40 rows of 200 KiB in one
`INSERT`, six delays from 0 to 200 ms) never failed; the exposed shape is an
explicit multi-statement transaction. This repo has 47 `Begin`/`BeginTx` sites on
Dolt handles (7 in `objectgraph/dolt_store.go`, `PutBatch` among them, 3 in the
derived-key validator, `store/run_memory.go`, `computer_events.go`,
`vocab_migrate_og.go`, platform and maild stores), none of which holds
`doltCommitMu` or can know a pass has begun. Whether the paper's passes would hit
one is a matter of write duty cycle; any hit is a failed write that the caller
sees, with no retry.

**Verdict.** Shape 2 (and any variant that puts the pass on another connection of
this engine, including a dedicated third session) fails FM5. It is unsafe for an
unregistered write session, which is the driver's property, not the pool's. The
GC stays on `s.db`. A safe way to keep writers running during the mark phase
needs the driver (or this store) to register sessions with the controller, or a
write gate that waits out open transactions before `CALL DOLT_GC()` and holds new
ones until `BeginGC` has returned, which is most of the 100% hold again.
**[H]** whether that gate is worth building; the remaining shapes (1, 3, 4, 5)
stand, and shape 1 (the `online_gc=active` flag) is independent of this.

Two consequences for the code as it stands. **[V] source.** The comment on
`MaybeRunOnlineDoltGC` (`dolt_maintenance.go`) says the safepoint controller
"keeps the store's read session valid across a collection". Under this driver the
read session is not registered, so that comes from the chunk store keeper
(reads retained as they happen), not from the controller; an autocommit read
cannot hold a root across the pass, so it holds. And nothing on `s.db` may start
a transaction while the pass runs, which `MaxOpenConns(1)` guarantees only
because the pass uses that connection.

## 2. `objectgraph.scanSideRows` (the validation page)

**What it is. [V]** One page of `ValidateDerivedKeys`, the background pass that
compares the derived-key side table `og_key_index` with `og_objects` before a
family is trusted for reads. `StartDerivedKeyValidation`
(`internal/store/derived_keys.go:66`) starts it after the listener is up, on
every boot, and again if a family is withdrawn or a repair is pending. It runs
the four families in order (`actor_wake_outbox.projected`, `run.agent_id`,
`commitment_record.addressee`, `source_ref.doc_id`), 200 rows a page, on `s.db`
in one transaction a page, under the engine mutex
(`objectgraph/derived_keys.go:565`, `validatePage`). The goroutine caught in
`scanSideRows` is the loop over the side-table rows of one page.

**Queries and plans. [V]** (`TestStallHoldersCensus/runs`, EXPLAIN on the
production text):

```text
head page   IndexedTableAccess(og_objects) index [canonical_id], range (obj:choir.run:, obj:choir.run;), Limit(200)
side page   Sort(canonical_id) <- IndexedTableAccess(og_key_index) index [key_name,key_value,canonical_id],
            filters {[run.agent_id], [NULL,inf), (lo, hi)}
```

The head page is a primary-key walk that stops at 200 rows. The side page
reads the family index with `canonical_id` as its third column, behind
`key_value`, and sorts.

**Cost as data grows. [V] warm local.** One mid-range side page costs 1.7 to
4.7 ms at 8,500 and 34,000 keys; a whole pass with no pause is 0.1 to 0.2 s
for 8,500 rows and 0.9 to 1.1 s for 34,000 (longest page 10 to 12 ms, longest
`SELECT 1` on `s.db` 12 to 41 ms). Page cost is nearly flat, so warm there is
nothing to see. The paper's hold is the cold first read of a family's pages.

**On the paper. [V]** At the 12:25:39 boot the pass ran 12:26:06 to 12:38:29
(12.4 minutes) over 72,033 + 34,261 + 55,112 + 110,332 rows, about 1,360
pages. The 12:34:57 snapshot lands on the first pages of `source_ref.doc_id`
(`validation pass starts` at 12:34:56; 400 rows by 12:35:05, 47 rows/s, then
480 rows/s once warm). The other snapshot (09:48:03) is 11.6 minutes after the
09:36:28 boot. Both stand in `nbs` archive reads. The 503 at 12:34:56 is
matched to it; it is one sample. **[V]**

**A pacing artifact. [V] code, [H] cause of the gap.** `passState.page` times
a page from before `engineMu.LockAs`, so a page that waited 20 s for the
mutex looks like a 20 s hold and the pass then sleeps three times that
(`derivedKeyDutyCycle = 0.25`). The pass logged 19,200 rows at 12:36:06 and
19,400 at 12:37:09: one page across the GC pass, then a long pause. Harmless
to `/health`; it slows the pass and so keeps its families on the JSON path
longer. **[H]**

**Fix shapes. [H], none built.** (a) Time the hold after the lock is taken.
(b) Cap a cold page: when one holds the connection past 250 ms, halve the
page for the rest of the family. (c) Start the pass after the boot's other
reads settle (the `ListObjects` seed and the first GC are inside the same
window, see
[boot-reads-with-the-seed-shape-2026-10-23.md](../../problems/boot-reads-with-the-seed-shape-2026-10-23.md)).
One sample in thirty minutes and once a boot: not the next fix.

## 3. `selfdev.(*Store).ListByStates` is `/health`'s own read

**Query and plan. [V]** `operationSelect WHERE computer_id=? AND state IN (...)
ORDER BY created_at` on `self_development_operations`
(`internal/selfdev/operations.go:308`):

```text
Sort(created_at) <- IndexedTableAccess(self_development_operations)
  index [computer_id, state, updated_at], one range per state
```

**Call sites. [V]** `activeSelfdevOperations` (`runtime.go:2521`,
busy states: executing, accepted, materializing, rollback_pending), which is
one of `/health`'s three reads (`api.go:783`), so one call per `/health`
request; the rest are on demand (`obligations_surface.go:190`,
`api_self_development.go:268`, the materializer reconcile, planned restart,
`appliedOperationForRelease`, which the `/health` release path runs only
through a cached background resolve).

**Cost. [V]** warm local, 20,000 rows of which 10,000 applied and 2 open:

| call | first / best | rows returned | allocated |
|---|---|---|---|
| busy states (`/health`) | 0.86 / 0.76 ms | 0 | 0.6 MiB |
| open states | 0.86 / 0.86 ms | 2 | 0.6 MiB |
| applied (`appliedOperationForRelease`) | 41.7 / 28.7 ms | 9,999 | 65 MiB |

The `/health` call touches only the busy ranges and costs a millisecond at any
table size (200 rows: 0.79 ms against 0.87 ms at 20,000). Only the applied
read scales with history (1.1 ms at 200 rows, 29 to 33 ms at 20,000),
and it is not per request.

**Why it is named. [V]** The snapshot is taken after the 503 answer and lists
goroutines inside the engine that are not parked in `database/sql.(*DB).conn`.
The goroutine that gets the connection when the holder releases it is not
parked there, so it appears. At 12:51:04 the console reads, 134 ms apart:

```text
[1531.626] ... store: dolt gc (online) complete in 19s: journal 1096 MiB -> 0 MiB
[1531.760] ... store_stalled holders: ... write_pool{in_use=1 ...}
           holder g=997821 [runnable] choir=selfdev.(*Store).ListByStates
           engine=vitess/go/vt/sqlparser.(*Tokenizer).Scan
```

It is in the SQL tokenizer, the first step of a query that has just got the
connection; the pass began 12:50:45. The 12:50:59 503 is the GC's. **[V]** The
10:42:15 snapshot (engine frame `analyzer.pushFilters`, so also at the start
of its query) is 62 s into the 10:41:13 boot, with the wake-outbox validation
running and no GC within five minutes; the real holder is not in the log
**[H]**. In both, the holder log names the first mover after a release, not
the holder. A reader of the log would have chased a one-millisecond query.

**Fix shape. [H]** Instrument only: have the stall header carry the GC flag
(shape 1 above) and drop goroutines whose stack passes through
`readHealthStore` from `shown`. No query change.

## 4. `GET /api/runs`

**Path and queries. [V]** `HandleRunList` (`internal/agentcore/api.go:524`) ->
`Runtime.ListRunsByOwner` (`runtime.go:2320`), two scans of the run kind
(`choir.run`, 34,261 rows at 12:33, from the `run.agent_id` validation line):

1. `Store.ListRunsByOwner` -> `ListRunsByOwnerOG` (`graph_store.go:763`) ->
   `ogListAllObjectsByKind` -> `DoltStore.ListObjectsPage`, 256 rows a page,
   one engine-mutex hold per page, on the read pool:

   ```text
   SELECT <12 columns incl. body, metadata> FROM og_objects
    WHERE object_kind = ? AND canonical_id > ? ORDER BY canonical_id ASC LIMIT 256

   Limit(256) <- TopN(canonical_id) <- IndexedTableAccess(og_objects)
     index [object_kind, og_rec_conversation_id, canonical_id], filters {[choir.run],[NULL,inf),(cursor,inf)}
   ```

   The plan cannot seek on the cursor, so every page reads every remaining row
   of the kind with its body and keeps the top 256: **cost per page is the
   rows left, total is quadratic in the kind.** It then decodes each body
   and skips it if `computer_id` is set (`lifecycleRunProjection`), which every
   run on the paper has if its lifecycle projection is the norm **[H]**; the
   scan returns nothing.
2. `Store.ListLifecycleRunsByOwner` -> `ReadObjectSnapshotFiltered`
   (`dolt_store.go:832`): one read transaction under one engine-mutex hold,
   refs for `(kind, owner_id, computer_id)` through
   `[object_kind, owner_id, computer_id]` with a sort, then the bodies in
   primary-key batches. This is the scan that returns the rows.

**Call sites. [V]** not periodic: `GET /api/runs` (the debugging and `choir
trace` surfaces), `ListRunsByTrajectory` (`api_run_events.go:109`, with limit
`1<<30` for the legacy half), `SynthesizeRunAcceptance` (limit 1,000), the
Texture workflow verifier (1,000) and the Texture diagnosis API (two calls). The
console has no per-request line; the only trace is the error line when the
client gives up.

**Cost as the kind grows. [V]** warm local (`TestStallHoldersCensus/runs`,
`choir.run` objects with `computer_id` set, about 3 KiB of body each):

| runs | legacy scan | alloc | lifecycle scan | alloc | longest engine-mutex hold | a point `GetObject` issued 20 ms into the lifecycle scan | ids-only newest 100 (fix shape) |
|---|---|---|---|---|---|---|---|
| 8,500 | 1.33 to 1.38 s (35 pages) | 388 to 393 MiB | 0.87 to 1.02 s | 268 to 275 MiB | 0.72 to 0.87 s | 0.69 to 0.78 s | 17 ms |
| 34,000 | **18.3 s** (134 pages) | **3,691 MiB** | 3.46 s | 1,075 MiB | 2.91 s | **2.96 s** | 62 ms |

- Four times the rows: the legacy scan takes 13.8 times as long (quadratic),
  the lifecycle scan 4.0 times (linear). At the paper's 34,000 the legacy half
  is 84% of the 21.8 s and returns zero rows. **[V]**
- A page holds the engine mutex 0.26 s at most at 34,000; the lifecycle scan
  holds it for its whole 2.9 s, so every object-graph read and write waits that
  long (a point read waited 2.96 s). **[V]**
- It does **not** hold the write connection: a `SELECT 1` on `s.db` waited at
  most 19 ms during both scans; on `s.readDB` (the scans' own connection) 2.9 s.
  So `GET /api/runs` cannot cause a `/health` 503 by `s.db` contention, and
  none is matched. It can delay anything that takes the engine mutex. **[V]**
  local.
- The paper timed out at 90 s: warm local is 22 s, so the paper is at least
  4 times slower, the cold-chunk factor the run-memory record found (30 to
  500 times for its read). **[H]**

**On the paper. [V]** `console.log.2` holds three `runtime api: list runs`
errors, all `context canceled`: 09:44:47 (`Error 1105`), 09:45:17 (`error
processing column 6`) and 09:46:53 (`objectgraph dolt: list objects page`),
the last inside the legacy scan after the client gave up. They match the
90 s timeouts of the earlier investigation **[H]** (its samples were taken
09:48 to 09:51). None is near a 503.

**Existing replacement. [V] in the repo.** `ogHeadPageSQL`
(`objectgraph/derived_keys.go:544`) is the primary-key range walk for the same
problem, already used by the validator: `WHERE canonical_id > ? AND
canonical_id < ? ORDER BY canonical_id LIMIT n` over the kind's id prefix
`obj:<kind>:` (its plan above stops after 200 rows). `ListObjectsPage` is not
connected to it, and 12 `ogListAllObjectsByKind` call sites and four direct
ones inherit the quadratic shape.

**Fix shapes. [H], none built.**

1. **Drop the legacy scan where it can only discard (orange).** Replace the
   full-kind pass in `ListRunsByOwnerOG` by the owner-and-empty-computer range
   `WHERE object_kind = ? AND owner_id = ? AND computer_id = ''` (the
   `[object_kind, owner_id, computer_id]` index seeks it). 84% of the
   local cost, one query.
2. **Page `ListObjectsPage` by the id prefix range (orange, substrate).**
   Connect it to the `ogHeadPageSQL` form so any full-kind listing is linear.
3. **A bounded newest-N list (orange).** `ORDER BY created_at DESC LIMIT n`
   on `[object_kind, owner_id, computer_id]` returns the ids in 62 ms at
   34,000 (12 MiB allocated, against 1,075 MiB for the lifecycle scan),
   then n primary-key reads. `GET /api/runs` has no filter, so this is
   exact if `og_objects.created_at` equals the run's `CreatedAt` **[H]**
   (`CreateRunOG` passes it as `now`).

### Fix shape 1 built (orange; lane `lane/fp0-runs-list-scan`)

`ListRunsByOwnerOG` now pages `og_objects WHERE object_kind = ? AND owner_id = ?
AND computer_id = '' AND canonical_id > ? ORDER BY canonical_id LIMIT ?`
(`DoltStore.ListObjectsByKindOwnerComputerPage`, `ObjectsByKindOwnerComputerPageSQL`)
instead of `ogListAllObjectsByKind` plus a Go filter. **Callers [V]:** the only
production caller of `Store.ListRunsByOwner` / `ListRunsByOwnerOG` is
`Runtime.ListRunsByOwner` (`runtime.go:2320`), reached by `HandleRunList`
(`api.go:551`), `ListRunsByTrajectory` (`api_run_events.go:109`),
`SynthesizeRunAcceptance` (`run_acceptance.go:78`), the Texture workflow
verifier (`texture_workflow_verifier.go:107`) and the Texture diagnosis API
(`texture.go:1949`, `:1961`); the rest are tests. `frontend/src` has no
`/api/runs` caller (the callers are `choir runs`, `choir trace`, and scripts).
`ogListAllObjectsByKind` itself is unchanged (ten other callers).

- **Same answer [V].** `TestListRunsByOwnerEquivalentToTheFullKindScan`: 2,400
  runs over two owners, three computer values and five states with created_at
  ties; the old function (kept in the test as the oracle) and the new one
  return the same ids in the same order at limits 0, 1, 7, 100, 500, 5,000 and
  for an unknown owner; the keyset walk at page sizes 1, 3, 50, 256 and 4,096
  equals the old filtered full-kind list. `GET /api/runs` has no cursor of its
  own: the cursor is the internal canonical-ID walk, and the handler's
  limit/sort/merge is untouched. One accepted divergence: a run whose
  computer_id is whitespace-only (never written: lifecycle scope is trimmed
  before write) was legacy to the old Go filter and is outside `computer_id = ''`.
- **Index [V].** `idx_og_objects_kind_owner_computer` is a base index: a fresh
  store builds it at open (empty table); a populated store builds it only in
  `MigrateAccessPaths` (the operator route). The SQL is correct without it:
  the plan then ranges `(object_kind, owner_id)` (the owner's runs of the
  kind, one page when the legacy half is under 256), which every store has
  (`SchemaDDL`). `TestScopePage*` pins the plan with and without the index and
  after migration. With the index the engine still plans a TopN for the cursor
  order, but over the scope's range only, so per page cost is the owner's
  legacy rows, not the kind.
- **Cost [V], local, `TestStallHoldersCensus/runs`, `limit=100`:**

  | runs | legacy half before | after | lifecycle half (unchanged) |
  |---|---|---|---|
  | 8,500 | 1,312 ms, 390 MiB | 0.9 ms, 1.8 MiB | 0.93 s (untouched code; 0.91 s after) |
  | 34,000 | 18,626 ms, 3,697 MiB | 1.1 to 2.1 ms, 1.8 to 2.1 MiB | 3.6 s before; 5.0 to 7.5 s in two later runs (run-to-run noise on this machine) |

  The legacy half no longer grows with the lifecycle runs
  (`TestListRunsByOwnerCostDoesNotGrowWithLifecycleRuns`: zero `ListObjectsPage`
  calls and flat allocation at 500 and 2,000 lifecycle runs; the old read made
  4 and 9 calls).
- **Not done: the second scan's engine-mutex hold (2.9 s at 34,000).** Bounding
  it needs fix shape 3 (newest-N by `created_at`), exact only if
  `og_objects.created_at` equals the run record's `CreatedAt`, which is still
  **[H]**, and `ListRunsByTrajectory` and others want the full set. Left as the
  next step; it is a different mechanism from this deletion.

## What the holder log gets wrong

- **It names the first mover after the release.** The 12:51:04 snapshot is
  134 ms after the pass returned. A snapshot taken when the budget expires
  (inside `readHealthStore`, before the answer) would still hold the pass in
  its stack. **[V]** evidence, **[H]** fix.
- **It keeps one name a minute.** The 8 unattributed D15 samples are the cost.
- **Sample rates are probe-shaped.** A 5 s probe timeout during a 19 s pass
  yields 3 samples, a 1 s one yields 7. Use time-share of the stalled
  pass (1.56%) to compare windows.

## Order of work

1. The GC pass (shape 2 of section 1, plus the flag). Largest share, one
   line, one review.
2. `ListRunsByOwnerOG`'s discarded scan (section 4, shape 1). Not a 503
   holder; the largest latency hazard left (22 s and an engine-mutex hold of
   3 s at the paper's size) and one query.
3. The validation pass's page pacing (section 2) after the boot reads are
   settled.
4. `ListByStates`: nothing; fix the log (section 3).

## How this was measured

- Worktree `lane/fp0-stall-holders` off `origin/main` (`b1abdfb0b`). Local
  embedded Dolt only. No SQL against production Dolt; no mutation of any
  computer; the host-internal token was not used; `GET /api/runs` was not
  repeated on the paper. The paper's console was read with `ssh node-b 'sudo
  grep -a ...'`; the parsing and the join (76 snapshots, 40 GC lines, the two
  probe sets) were scratch scripts, deleted.
- `TestStallHoldersCensus` (`internal/store/stall_holders_census_test.go`,
  env-gated, repeatable):

  ```text
  CHOIR_STALL_HOLDERS=selfdev go test ./internal/store -run TestStallHoldersCensus -v -count=1
  CHOIR_STALL_HOLDERS=runs CHOIR_STALL_RUNS=34000 go test ./internal/store -run TestStallHoldersCensus -v -count=1 -timeout 50m
  CHOIR_STALL_HOLDERS=gc CHOIR_STALL_GC_MIB=128,512,1100 go test ./internal/store -run TestStallHoldersCensus -v -count=1 -timeout 55m
  ```

  It seeds directly (run objects through `PutBatch` with `computer_id` set;
  journal through multi-row inserts), prints each plan with
  `EXPLAIN PLAN`, and reports time, allocation, engine-mutex stats and the
  longest wait of a concurrent `SELECT 1`. The only assertion is that the
  writer in the GC section lost no row.
- Conditions: a warm local store on a shared machine; repeat runs differ by up
  to 25% (the 8,500-run rows give both). The ratios and shapes are the
  evidence, not the absolute times; the paper is IO-bound and 4 to 7 times
  slower on the same work.
- Gates: `check-test-hang-guards`, `check-decode-roots` and
  `check-v1-writer-purity` pass.

## What this does not settle

1. Why `/health` answered in 4.6 s and timed out twice at 5 s in the 12:50
   pass when its budget is 1 s; whether the process is blocked or one call in
   the handler is (section 1).
2. Whether the paper's GC has a fixed cost per pass (the 03:xx fit says about
   2 s) or none; it decides whether smaller passes can ever get under the
   budget.
3. What held the connection at 10:42:15 before `ListByStates` took it.
4. Whether the paper's runs all have `computer_id` set (so the legacy scan
   discards every row) and what a cold `GET /api/runs` costs; a request-scoped
   timing on the handler, as the run-memory record did for its function,
   would answer both on the next refresh.
5. ~~Whether a GC issued on the read pool's connection is safe on the paper
   (FM5), not only on a local store.~~ Settled by section 1a: it is not safe on
   any store, because it is the driver's session registration, not the paper's
   state.

## Heresy delta

- Discovered: the stall snapshot can name a bystander as the holder (the
  first mover after a release) and is taken after the answer; `/health` can
  exceed its own 1 s budget during a GC pass; the sample-count 503 rate
  understates the GC's time-share by about nine times; `GET /api/runs` reads
  and discards every lifecycle run before it reads them again, and
  `ListObjectsPage` is quadratic over a kind (12 callers); the validator's
  pacing counts the engine-mutex wait as hold.
- Introduced: none (a yellow test, a green record).
- Repaired: none.

## Rollback

Revert the commits that add `internal/store/stall_holders_census_test.go` and
this record. No runtime, schema, data or index change.
