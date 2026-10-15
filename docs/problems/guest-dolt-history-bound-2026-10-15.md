# Guest Dolt history still grows several GB a day after the journal fix

Date: 2026-10-15 (~03:00–04:30 UTC). Residual of
[`guest-dolt-storage-growth-2026-10-15`](guest-dolt-storage-growth-2026-10-15.md)
("bounding history is the next station"). Prior records of the same class:
[`guest-store-history-bloat-and-memory-shape-2026-10-09`](guest-store-history-bloat-and-memory-shape-2026-10-09.md)
(history was 81% of the owner store; offline compaction 9.6 → 1.7 GiB) and
[`paper-computer-disk-full-outage-2026-10-15`](paper-computer-disk-full-outage-2026-10-15.md).
Mutation class of the fix: **red** (guest persistent state: the store's
commit graph and GC policy on the always-on paper computer).

## Evidence

Data rules: the owner's computer, so counts, sizes, states and timings only.
Sources: the running paper guest's `/health` (`persistent_disk`, `dolt_gc`)
and its console `store: dolt gc` lines, sampled read-only every 5 minutes
from Node B (host IO pressure `some avg10` stayed at or under 0.92); a
paper-shaped load on a real embedded store, run locally (below). No image was
mounted for this record; the earlier record's row counts are reused.

### The journal fix works; history is now the growth

The fix (4bf25d82a, deployed 133de45fa at 02:40) runs an online GC every
5 minutes:

| GC (UTC) | Journal | Avail after | Store (non-journal) |
|---|---|---|---|
| 02:41 (first online tick) | 6,590 → 0 MiB in 54 s | 34,001 MiB | 9 → 11 GiB |
| 03:00:20 | 1,624 → 0 MiB in 10 s | 33,726 MiB | 11 GiB |
| 03:20:20 | 1,626 → 0 MiB in 10 s | 33,455 MiB | 11 GiB |
| 03:35:17 | 1,044 → 0 MiB in 7 s | 33,279 MiB | 11 GiB |

- The journal now refills at about 70–110 MiB/min and is collected at
  1 GiB. Before the outage the rate was about 43 MiB/min (2.6 GB/h).
- Free space after each GC falls by about 13 MiB/min (271 MiB in 20 min,
  176 MiB in 15 min). That is **about 18 GB/day at today's load**; the
  10-13/14 old-generation archives gave about 6 GB/day at the lower load.
- About 17% of each journal survives GC (271 of 1,626 MiB; 176 of
  1,044 MiB): the chunks reachable from a commit, promoted to the old
  generation, which a default GC never collects.
- The store is 11 GiB; the earlier record put live logical data at about
  2 GB raw. With 34 GB free, the 48 GB disk fills in about **2–6 days**,
  depending on load.

### Why 3 tape events per second

- 358,503 of 359,911 `computer_event_index` rows (99.6%) are
  `projection_batch_recorded`: one tape event per projection write
  (`internal/store/projection_tape.go`, `appendOps`).
- The writes are run trace, not publishing state:
  - `AppendEvent` stores each run trace event as a `choir.event` object,
    one tape event each: 207,658 objects, **58% of the tape**;
  - `AppendRunMemoryEntry` writes one tape event per memory entry: 111,636,
    **31%**.
- The paper's console shows 643 tool-loop iterations in about 28 minutes
  (0.38/s) with `running_runs` 0 at most samples. Autopaper's story
  pipelines run short tool loops back to back; each iteration emits several
  trace events and memory entries.
- Each tape event is a prepare transaction and a finalize transaction. The
  event and its receipt add about 3 KB raw to the index. The finalize
  rewrites prolly-tree paths in `computer_event_index` (hash primary key,
  so inserts land at random), `og_objects` and their indexes, and the heads
  row. One Dolt checkpoint per 100 finalized events
  (`defaultFinalizeCheckpointEvery`) pins every node those 100 events
  changed.

### Paper-shaped load on a real store (local, `go test`, scratch harness)

The harness writes 10,000 events on an embedded store the way finalize
does. Per event:
- one prepare insert into a hash-keyed index with two secondary indexes;
- one transaction that updates the heads row, finalizes the index row with
  a receipt, and inserts a `choir.event`-like object;
- 3.8 KB of raw payload, JSON with hex digests;
- a default GC every 2,000 events.

Three checkpoint cadences were measured:

| Checkpoint cadence | Commits | Store after default GC | Live after squash + full GC | History share | Store per event | Squash + full GC |
|---|---|---|---|---|---|---|
| every 100 events (today) | 102 | 61.9 MB | 10.3 MB | **83.3%** | 6.2 KB | 6.4 s |
| every 1,000 events | 12 | 24.1 MB | 10.4 MB | 57.1% | 2.4 KB | 1.7 s |
| none until the end | 3 | 10.1 MB | 10.1 MB | 0% | 1.0 KB | 0.8 s |

With no checkpoints the default GCs took 53 s in all, against 29 s at a
1,000-event cadence, and the writes took 17 minutes against 10.6. Everything
stays in the new generation, so every GC copies the live set again.

- The 83% history share at today's cadence matches the 81% measured on the
  owner's store on 10-09. History, not live data, is what grows.
- Live data compresses to about 27% of raw (1.03 KB per event for 3.8 KB
  raw).
- A sparser cadence helps, but it does not bound anything. In a large table
  a commit still pins about one leaf per changed row plus the upper levels,
  so history keeps growing with the event count.
- A squash plus full GC leaves exactly the live set. The rows are unchanged:
  the squash commits the same working set.

### What an online compaction costs (local, same harness)

The scale points used the implementation's own `compactDoltHistoryOnline`
on bulk-loaded index rows: 3.8 KB raw each, one commit per 100 rows, a
default GC every 10,000 rows. The rerun had the Mac to itself:

| Rows | Commits | Store before | Live after | Witness (3 reads) | Squash | Full GC |
|---|---|---|---|---|---|---|
| 50,000 | 502 | 342 MB | 41 MB | 16 ms | 49 ms | 0.91 s |
| 200,000 | 1,502 | 1,602 MB | 190 MB | 11 ms | 45 ms | **4.05 s** |

- History was **88%** of the store at both points.
- The full GC dominates. Its cost follows the live set, about 21 ms per
  MB of live data on a Mac SSD. The squash and the witness reads are
  negligible.
- An earlier run of the same 200k point took 84 s on a CPU-pegged machine.
  This is a cost to measure on the guest, not to infer.
- Extrapolated to the paper's 1–3 GiB live set: about 20–65 s on the Mac.
  The guest's default GCs read 1.6–6.6 GiB of journal in 10–54 s, so
  under about 2 minutes is the planning number. The paper-scale run
  (about 360k rows plus objects) is left for Node B or the first deployed
  compaction's record (`gc_ms`).

## Constraint: what may be compacted

- The tape is the authority. Event heads, the canonical chain and the
  `computer_event_index` rows that index it are never rewritten or deleted.
  Doctrine C8 and I23 name one appender as the owner of semantic order.
  O21 says a realization holds no unique state, so the projection store can
  be rebuilt from the tape.
- The **Dolt commit graph** of the projection store is not state. Nothing
  reads it:
  - Texture history follows the revision parent chain
    (`internal/store/texture.go`, `getHistory`), not `AS OF` or
    `dolt_history_*`; no product query uses either.
  - The checkpoint and rematerialize witness is the **content root**
    (`computerversion.DoltStateExtractor`, compared by
    `WitnessContentMatches`). `HASHOF('HEAD')` is recorded for audit only
    and is not part of equivalence.
  - No code creates a branch or a tag on the guest store.
- Projected rows (trace objects, run memory) can be rebuilt from the tape,
  but a projection must stay equal to a replay of its tape. Deleting rows
  is therefore not compaction: it would need a retention event on the tape
  that the reducer applies.

## Options evaluated

1. **Fewer `projection_batch_recorded` events.** Most of the tape is
   per-iteration trace (89%). Batching a tool-loop iteration's trace events
   and memory entries into one projection batch would cut the event count
   roughly 5–8x. It would cut live growth, and the index rows (about 3 KB
   raw per event) most of all.
   - It changes the tape's write path and `AppendEvent`'s durability
     semantics (orange/red, elsewhere).
   - It does not bound history: fewer, larger events still pin what they
     change.
   - **Deferred to its own station**, the lever for live growth below.
2. **Online squash plus full GC on a budget.** This bounds history to the
   budget whatever the load or cadence. It changes no row, and it is the
   proven offline remedy (`projection-compact`, 10-09: 9.6 → 1.7 GiB with
   heads, count and content witness intact), run in place.
   - Cost: a full GC rewrites the live set while it holds the write
     connection.
   - **Chosen.**
3. **Retention for regenerable rows** (old trace objects, run memory of
   closed runs). This saves live bytes. It needs a tape-declared retention
   event, or replay diverges from the projection. **Deferred**: product and
   doctrine work.
4. **Commit cadence.** Measured above: 10x sparser checkpoints cut history
   per event about 2.6x on a 10k-row table, and less on large tables.
   - It does not bound anything, and the squash makes it a frequency knob
     only.
   - Dropping commits entirely would leave everything in the new
     generation, so every 5-minute default GC would copy the whole live set.
   - **Not changed.**

## Design

Each tick of the periodic loop runs the journal GC first. It then considers
an **online history compaction** (`internal/store/dolt_compaction.go`):

- **Gate (added at review, 2026-10-15).**
  - **Off for the fleet.** Compaction runs only on computers listed in
    `CHOIR_DOLT_COMPACT_COMPUTERS` (comma or space separated ComputerIDs).
    The list is set in the guest image's runtime environment
    (`nix/autoputer-vm.nix`), and each guest compares it with its own
    `CHOIR_COMPUTER_ID`. It lists only the paper:
    `computer-1355b83bced1613eb048ee28d66688a8` (VM slot
    `vm-1669020adcece51c2f5ff9b6df399ef5`). The journal GC stays
    fleet-wide. Every other computer records `not_enabled`.
    - Why not `nix/node-b.nix`: that file is vmctl's environment, and no
      existing host-to-guest path carries a generic per-computer switch.
      The Claude delegate grant rides vmctl's credential disk, which would
      mean a vmctl change (red, VM lifecycle) for a list the guest can
      check itself.
    - Changing the list takes a deploy, then the computer's next boot.
  - **Never on the first tick.** The first tick fires before `rt.Start`
    (`internal/autoputer/run.go`), so boot writes would wait out a full GC.
    It records `boot_wait`.
  - **Boot jitter.** Compaction is not eligible before a random 0–30
    minutes after boot, so computers refreshed by one deploy do not
    compact together. Until then the record says `boot_wait` with the
    eligible time.
  - **In-progress record.** Before anything changes, the record is
    rewritten durably (fsync of file and directory) as `in_progress`, with
    `last_failure_at` set to the start. Success clears it.
    - A guest killed mid-run (a planned refresh of the always-busy paper,
      an OOM, the wedge detector) boots into the 6 h backoff instead of
      compacting again.
    - The first tick after such a boot notes the interrupted run in the
      record and in the log.

- **Trigger.**
  - History is estimated as the non-journal store's growth since the last
    compaction (`baseline_bytes`, kept in the record). Live growth counts
    too, so the estimate errs early.
  - The budget is `RUNTIME_DOLT_HISTORY_BUDGET_MIB` (default 2,048, about
    the paper's live size), lowered to a quarter of free space, with a
    512 MiB floor.
  - There is **no store-size gate.** A full GC walks only what is
    reachable, so its cost follows the live set. A size gate is how the
    journal GC switched itself off for good (FM2 of the prior record).
- **Space.** A full GC writes everything reachable before it deletes
  anything, so it runs only with free space of at least store + journal +
  512 MiB. Otherwise the outcome is `deferred_space`, and the offline
  runbook applies.
- **Exclusivity.** The whole compaction holds `doltCommitMu` and the
  store's single write connection (`db.Conn`), so no write interleaves.
  Reads on the read pool continue. Dolt's session-aware safepoint
  controller keeps them valid across the GC.
- **Steps.**
  1. Refuse unless there is exactly one branch.
  2. Take both witnesses (below).
  3. Find the root: the only parentless commit in `dolt_commit_ancestors`
     (`parent_hash IS NULL`). It is not chosen by date, so clock skew
     cannot pick the wrong commit. More than one root refuses.
  4. `DOLT_RESET('--soft', root)` then
     `DOLT_COMMIT('-A', '--allow-empty', …)`.
  5. Take the light witness again. If it changed, `DOLT_RESET('--soft',
     old head)` and fail.
  6. Run `DOLT_GC('--full')`.
  7. Take both witnesses again, still before releasing the write
     connection. A mismatch **fails closed**:
     - the outcome is `degraded`, with `degraded` naming the changed
       table(s), shown on `/health` until a compaction succeeds;
     - the 6 h backoff starts;
     - a loud log line (`DOLT COMPACTION DEGRADED`) says to rebuild the
       projection from the tape.
- **Witnesses.**
  - The **light witness**, on the write connection, holds the runbook's
    checks:
    - a digest of every `computer_event_projection_heads` row (sequence,
      canonical, desired and effective heads);
    - the `computer_event_index` count;
    - every base table's row count.
  - The **full content witness**, on the read pool (a second session),
    digests every column of every row of every base table, in
    primary-key order. Rows are streamed into the digest, so memory stays
    flat. The record carries its root as `content_root`.
    - It has the strength of the offline tool's `WitnessContentMatches`.
      It does not reuse `computerversion.DoltStateExtractor`, which opens
      a second engine on the workspace and sorts every row in memory.
  - Both are taken while writes are still held off. The review asked for
    the content witness after releasing the write connection. Once writes
    resume, the content changes legitimately and no before/after
    comparison means anything, so the second session is used but the
    write connection is released last.
- **Crash safety.**
  - The only non-atomic window is between the soft reset and the squash
    commit. There, HEAD is the root and every row is in the persisted
    working set; the next checkpoint commits it.
  - Dolt's GC swaps its manifest atomically.
- **Backoff and record.**
  - A refused or failed compaction is retried after 6 h, because each
    attempt holds writes for a full GC.
  - The decision is written on every tick to
    `.choir-dolt-compaction.json` and served on guest `/health` as
    `dolt_compaction`. It carries:
    - outcome, store, baseline, history, budget and free space;
    - commits before and after;
    - duration with `witness_ms`, `squash_ms` and `gc_ms`;
    - `peak_rss_mib` (the runtime's VmHWM after the compaction);
    - heads digest, event index count and `content_root`;
    - `witness_match`, `degraded`, `last_success_at` and
      `last_failure_at`.
- **Kill switches.** `RUNTIME_DOLT_COMPACT_DISABLED`, or the existing
  `RUNTIME_DOLT_GC_DISABLED`.
- **Boot.** Boot is unchanged: no compaction before the listener (the
  wedge detector's 5-minute window). The offline `projection-compact` stays
  the break-glass path.

## Failure modes (written before the tests)

- **FH1** History grows without bound between offline compactions.
- **FH2** Compaction changes projected state: the projection heads, the
  event index count, or a table's row count.
- **FH3** Compaction loses the working set written since the last
  checkpoint.
- **FH4** Concurrent writes interleave with the squash and the checks.
- **FH5** Online full GC leaves the write or read session unusable, or the
  tape cannot append afterwards.
- **FH6** Compaction starts without free space for its output.
- **FH7** Compaction re-runs every tick: no reclaim, or after a failure.
- **FH8** A second branch keeps history reachable; squashing it would
  free nothing.
- **FH9** A crash between the soft reset and the squash commit loses rows
  or leaves the store unopenable.
- **FH10** A compaction decision is invisible off the guest.
- **FH11** The kill switches are ignored.
- **FH12** A store-size gate turns compaction off for a large store.
- **FH14** A compaction killed mid-run compacts again on the next boot
  instead of backing off.
- **FH15** Boot storm: compaction runs on the first tick (before
  `rt.Start`), or every enabled computer compacts at once after a deploy.
- **FH16** Fleet exposure: compaction runs on a computer the host did not
  enable.
- **FH17** A content change the light witness cannot see (rows, not
  counts) passes, or a mismatch is only an error string.
- **FH18** The squash target is picked by date and the wrong commit is
  chosen.
- **FH13** (residual) A full GC holds the write connection while it
  rewrites the live set. Measured locally: 4 s for 190 MB live. On the
  paper the estimate is under about 2 minutes for 1–3 GiB live; the record
  carries `gc_ms` to confirm it. Run trace appends wait for it. Today's
  default GC already stalls writes for 7–54 s every 15–25 minutes; a
  compaction adds one longer stall every few hours.
  - If the guest's `gc_ms` is too long, the follow-up is to run the full
    GC on a separate session over the same connector, so writes continue.
    The Dolt pin includes the upstream race fix 6bea93f2.
  - The post-GC check then becomes a prefix check: the pre-squash head
    and the event count up to its sequence are unchanged. It cannot be an
    equality check.

Tests (`internal/store/dolt_compaction_test.go`):
- The policy:
  - run over budget, no-op under it;
  - no size gate;
  - the headroom-scaled budget;
  - `deferred_space`;
  - backoff;
  - the baseline reset.
- The tape-fed projection test: real prepare and `FinalizeBatch` per event,
  frequent checkpoints, default GC, then compaction. It asserts that the
  runbook's head check, the index count, the run memory count, the
  uncommitted working set and the read session are unchanged; that the
  store shrinks; and that the tape appends and checkpoints afterwards.
- A second branch is refused.
- The crash window: soft reset, close, reopen; rows and heads are intact
  and compaction then succeeds.
- The record is readable for `/health`; the baseline makes the next tick a
  no-op; both kill switches work.
- The periodic loop compacts.
- Gate tests (FH14–FH18):
  - A snapshot of the record taken mid-run is `in_progress` with
    `last_failure_at`; restored as if the process died, the next call backs
    off and names the interrupted run.
  - The first tick never compacts (`boot_wait`, commits unchanged).
  - Jitter stays in [0, 30 min), and a tick before the eligible time waits.
  - Off by default: an empty list, another computer's id, or an empty
    computer id records `not_enabled`; a listed id runs.
  - A row changed after the GC with every count unchanged is caught by
    the content witness, the record is `degraded` naming the table, and
    the next tick backs off.
  - The root is parentless.

## Expected outcome and what is achievable

- **Disk after this fix** = live + history since the last compaction
  (≤ 2 GiB) + journal (≤ 1 GiB). The first eligible tick after deploy
  (5–35 minutes after boot) compacts the
  paper's 11 GiB store. The estimate is 1–3 GiB live: about 2 GB raw at
  10-15 00:30, at the measured 27–40% ratio, plus growth since.
- **Steady-state growth** is the live growth rate:
  - about 270,000 tape events/day on 10-13/14 at about 5.3 KB raw per
    event (index row about 3 KB, object about 1.7 KB, memory about 0.7 KB)
    and 27–40% compression gives **about 0.4–0.6 GB/day**;
  - today's roughly 2x load gives about 0.8–1.1 GB/day.
  - That is 10–20x less than now: the 48 GB disk lasts weeks instead of
    days.
- **The 0.5 GB/day target** is met at the 10-13/14 load, but not at
  today's. Going further needs option 1 (batch trace per tool-loop
  iteration, about 5–8x fewer index rows) or option 3 (tape-declared
  retention of closed runs' trace). Both are product changes for the next
  station.
- **Compactions on the paper**: one every 2–8 h at 6–18 GB/day of history.
  Each rewrites the live set.
- **Staging acceptance after deploy** (read-only, paper guest `/health`
  on port 8085 from Node B; counts and sizes only):
  - before the first compaction: `dolt_compaction.outcome` is
    `boot_wait`, then `noop` or `ran` within about 35 minutes of boot;
  - on another computer: `not_enabled`;
  - after: `outcome = ran` and `witness_match = true`, with
    `commits_after ≤ 2`;
  - `gc_ms` and `witness_ms`, the measured write stall, replace the FH13
    estimate;
  - `peak_rss_mib` stays well under the guest's memory;
  - `heads_digest` and `event_index_count` are consistent with the tape
    head, and `content_root` is recorded;
  - `after_store_mib` is the live size, replacing the estimate above;
  - free space after GC is flat to within the live growth over 24 h;
  - no `degraded`.

## Belief and remaining error

- **Belief**: history is bounded by the budget online, with the same
  checks and the same proven mechanism as the offline tool.
- **Remaining**:
  - the live growth rate (above);
  - the write stall during each full GC (FH13): 4 s for 190 MB live
    locally, not yet measured on the guest;
  - one-time cost: the first eligible tick on an enabled computer (only
    the paper) compacts once, because no baseline exists yet;
  - a `deferred_space` disk still needs the offline runbook;
  - pre-existing: replay completeness opens a second embedded engine on
    the live workspace. `computerversion.DoltStateExtractor.Extract` is
    called from `internal/agentcore/replay_completeness.go:239`, under
    `HoldAppends`, which does not stop a compaction. A full GC rewrites
    the table files and swaps the manifest under that engine, so a check
    running during a compaction may read a moving store. The compaction
    widens this window; it did not create it. The fix is for the
    extractor to read through the store's own read pool, as the
    compaction's content witness does;
  - the witness reads add to the write stall: two full content scans
    while writes are held. That is `witness_ms` in the record, not yet
    measured on the guest;
  - after compaction, projection bases copy a compact store. That is an
    upside: bases were 8.7 GB each on 10-09.
