# Guest Dolt storage growth filled the paper's disk: GC ran only at boot, then not at all

Date: 2026-10-15 (~00:30–01:00 UTC). Follows
[`paper-computer-disk-full-outage-2026-10-15`](paper-computer-disk-full-outage-2026-10-15.md)
problem 1. Owner direction: "research and fix the dolt storage leak for the
future." Mutation class of the fix: red (guest persistent state, the store's
GC policy on the always-on paper computer).

Prior records of the same class:
[`guest-dolt-journal-and-host-image-leak-2026-10-03`](guest-dolt-journal-and-host-image-leak-2026-10-03.md)
(27 GiB state, the 5 GiB guard named as the mechanism; the durable fix never
landed) and
[`guest-store-history-bloat-and-memory-shape-2026-10-09`](guest-store-history-bloat-and-memory-shape-2026-10-09.md)
(history is 81% of the store; offline compaction to 1.7 GiB). This is the
third disk-full or near-full event on a guest Dolt store in 12 days.

## Evidence

Data rules: the owner's computer, so counts, sizes, states and timings only.
Source: a stopped copy of the disk,
`vm-1669020a…/data.img.pre-grow-20261015002827` on Node B, loop-mounted
`ro,noload`. Dolt queries and the GC experiment ran on a btrfs reflink copy of
that file, under `/var/tmp/paper-dolt-scratch`, never the running image.
Growth samples come from the running guest's `/health`.

### Disk breakdown (32 GiB `data.img`, 33.48 GB used, 0 free)

| Path | Size |
|---|---|
| `state.texture/texture/.dolt/noms/vvvv…` (chunk journal) | **27,227,583,860 B** (mtime 10-15 00:20) |
| `…/noms/journal.idx` | 471,144,342 B |
| `…/noms/oldgen/` (7 `.darc` archives) | **5,453,103,104 B** |
| `…/noms/2kn6….darc` (new gen) | 1,683,890 B |
| `files/` | 120 MiB |
| `choir-updater/` | 154 MiB |
| `state-actor.db` (+wal) | 45 MB |

The old-generation archives were written at 10-13 16:24, 18:28, 19:10,
23:51 and 10-14 05:48, 10:24, 13:00. Their sizes run from 186 MB to 2,542 MB.
Each archive is one GC's promotion of everything reachable from commits.
None was written after 10-14 13:00.

The data image was created 2026-10-13 15:13: this computer (genesis
imported once) went from empty to full in **32 hours**.

### What the store holds

- 1 database, 1 branch, 24 tables. **3,595 commits** since 10-13 15:13. The
  rate is 10–342 per hour (about 110 on average): one checkpoint per 100
  finalized events (`defaultFinalizeCheckpointEvery`), and
  3,595 ≈ 359,911 / 100.
- Row counts:
  - `computer_event_index`: 359,911, of which 358,503 (99.6%) are
    `projection_batch_recorded`. That is about **3 tape events per second**.
  - `og_objects`: 275,332, of which `choir.event` is 207,658;
    `source_ref` 10,874; `actor_wake_outbox` 8,933;
    `commitment_record` 6,579.
  - `run_memory_entries`: 111,636.
  - `og_edges`: 5,242.
  - `texture_agent_mutations`: 2,063.
  - Every other table: 0–2.
- Logical payload sums (`SUM(LENGTH(col))`, no content read):
  - event index `event_json` 542 MB plus receipt JSON 523 MB;
  - `og_objects.body` about 0.6 GB;
  - run memory `message_json` 242 MB.
  - About 2 GB raw in all. The live set compresses to roughly 1–2 GB, in line
    with the 1.7 GiB measured on 10-09.

So **about 82% of the disk was journal garbage, about 16% was history plus
live data, and about 2% was everything else.**

### GC history (guest console logs, `store: dolt gc` lines only)

| Boot (UTC) | Decision |
|---|---|
| 10-14 10:22 | `journal reached 1 GiB (now ~13 GiB)` → ran 148 s, used 17 → 6 GiB |
| 10-14 13:00 | `used crossed 4 GiB milestone` → ran 50 s, used 11 → 6 GiB |
| 10-14 13:43, 14:27, 17:14 | **`skipped: live=5 GiB exceeds safe bounded-guest GC size`** |
| 10-14 23:22 (runtime restart, disk full) | `emergency: avail=0 MiB` → never completed |
| 10-15 00:06, 00:20 | `emergency: avail=0 MiB` → never completed |
| 10-15 00:28, 00:41 (after the grow to 48 GB) | `skipped: live=5 GiB …` |

- The disposition file still read the 17:14 boot decision
  (`skipped_size, live_gib 5, journal_gib 14`), although the runtime ran until
  23:22.
- The periodic GC writes a disposition on every 5-minute tick, so **no
  periodic tick ran in six hours**. No log has ever contained
  `store: periodic dolt gc`.

### Growth rates

- **Journal:** 0 → 27.2 GB from the 13:00 GC to 23:22 (10.4 h), about
  **2.6 GB/h**. The earlier window was 13 GiB from 05:48 to 10:22, about
  2.9 GB/h. That is about **240 KB of journal per tape event**. Each event is a
  prepare and a finalize transaction. Each transaction rewrites prolly-tree
  paths in `computer_event_index` (hash primary key, so inserts land at
  random), `og_objects` and their indexes, and the projection heads row.
- **Old generation (history plus live):** 5.45 GB in the 21.8 h between the
  first and last archive, about **6 GB/day**. Live data grows about 1.4 GB/day
  raw (append-only event index, events, run memory). The rest is history
  pinned by the 3,595 commits.
- **Running guest after the grow** (`/health persistent_disk.used_bytes`):
  - 00:34: 36.59 GB;
  - 00:52: 37.15 GB;
  - 00:57: 37.93 GB.
  - That is about **160 MB/min (~9.6 GB/h)** after the 00:41 deploy reboot.
    The boot GC skipped again (`live=5 GiB`) and no periodic GC runs.
  - At that rate the 12.5 GB still free runs out at about **02:15 UTC on
    10-15**, unless the fix ships or the store is collected by hand.

### GC experiment (reflink copy, throttled cgroup)

`dolt gc` (CLI 2.1.9, default mode) ran on the full-disk copy, which the copy
had grown to 64 GiB. The run was capped at 25 MB/s IO and 1.5 CPU, so its
duration is an upper bound.
- Reads: over 35 GB. Default GC reads the reachable new-generation chunks out
  of the 27 GB journal.
- Memory peak: 6.3 GB in the cgroup (process plus page cache).
- Disk: free space fell by **1.7 GB+** during the run, before any journal
  bytes were released. GC writes its output first and deletes the journal
  only at the end.
- An unthrottled first attempt raised Node B IO pressure (`some avg10` 74)
  and was stopped. The other cgroups (vmctl and the VMs) stayed near 0
  throughout.
- Final numbers are under "Experiment results" below.

## Root cause (ranked)

1. **The periodic GC was dead code.** `autoputer.startPeriodicDoltGC`
   created a context, called `store.StartPeriodicDoltGC` (which returns at
   once and loops in a goroutine), then cancelled the context on return. The
   loop exited before its first tick, so GC ran only at boot. A paper
   computer that is "always on" reboots only on deploys. The evidence is the
   disposition file frozen at the 17:14 boot and the absence of any periodic
   log line.
   - Even if it had ticked, it would have opened a **second embedded engine**
     on the workspace the store holds (`runDoltGCWorkspace`). It would hit
     journal-lock contention, not a GC.
2. **The 5 GiB "live store" guard turned boot GC off for good.**
   - The guard measured non-journal noms bytes, which is the old generation.
     A default GC never collects the old generation: it promotes everything
     reachable from commits into it (`dolt gc --help`; the pinned
     `store/types/value_store.go` uses `OldGenGCFilter` in `GCMode_Default`).
     So every successful GC grows the guarded quantity.
   - At 1 commit per 100 events, history passed 5 GiB about 22 h after
     genesis, at 10-14 13:43. From then on every boot skipped GC. This is the
     same mechanism recorded on 10-03; 9858ca1db fixed what the guard
     measured, not the guard.
   - The guard's premise does not hold for default GC. Its cost follows the
     new generation, not the store, because the walk does not descend into
     chunks already in the old generation. Dolt's own auto-GC fires on a
     128 MB journal with no store-size cap (`sqle/auto_gc.go`,
     `shouldRequestGC`).
3. **The write rate amplifies the defect.** About 3 tape events per second
   on the paper, at about 240 KB of journal each, is 2.6–9.6 GB/h. A 32 GB
   disk tolerates about 10 h without GC.
4. **The emergency line sat at 512 MiB free.** That is too late. Default GC
   writes the reachable new generation into a new archive before it swaps
   the manifest and deletes the journal: peak disk is old plus new (measured:
   1.7 GB+ before release). At 0 MiB free the emergency GC could not finish.
   Every boot retried it while health stayed down (outage doc, problem 4).
5. **History grows without bound (slower class).** One commit per 100 events
   pins about 1.5 MB of superseded chunks per commit. Only a history squash
   plus `dolt gc --full` reclaims it: the offline `projection-compact` tool,
   `docs/runbooks/offline-guest-gc.md`. Nothing on the guest reads Dolt
   history. Texture history follows the revision chain, and the checkpoint
   witness is the content root; `HASHOF('HEAD')` is audit-only
   (`computerversion/dolt_state_extractor.go`).

Classification: substrate (data persistence: the store's reclaim
contract), not symptom. Root-cause clustering applies (3 events in 12 days).
The existing replacement was half-present: a periodic GC loop existed but was
never live.

## Dolt storage semantics (cited)

- **Chunk journal** (`noms/vvvv…`, 32 v's): an append-only table file that
  every SQL transaction commit appends its new chunks to. It is the new
  generation's write path. "Dolt transactions that do not have a
  corresponding Dolt commit create on-disk garbage"
  (dolthub.com/docs/sql-reference/server/garbage-collection).
- **`dolt gc`** (default): generational. "Everything reachable from any
  commit on any branch is put into the old generation … By default, Dolt GC
  will only visit data in the new generation, and so will never collect data
  … which has previously made its way to the old generation" (`dolt gc
  --help`, 2.1.9).
- **`--full`**: also collects the old generation, which costs O(live set).
  It frees nothing that history still reaches, so it has to be paired with a
  squash.
- **`--shallow`**: a fast, incomplete pass; it does not shrink history.
- **Archives** (`.darc`, zstd with dictionaries): the GC output format since
  Dolt 2.0 (archive level 1 by default; `docs/evidence/dolt-v2-research-2026-08-24.md`).
- **Auto-GC**: on by default in `dolt sql-server` and `dolt sql` since 1.75
  (docs above). The embedded driver (`driver/v2` 2.2.0,
  `openEngineWithRetry`) installs no `AutoGCController`, so the guest gets
  none. That makes our own scheduler load-bearing.
- **Online GC and sessions**: the docs page still says `call dolt_gc()`
  breaks open connections. That describes the `kill_connections` safepoint
  controller. The pinned Dolt defaults to the **session-aware** controller
  (`dprocedures/dolt_gc.go`, `UseSessionAwareSafepointController = true`,
  overridable by `DOLT_GC_SAFEPOINT_CONTROLLER_CHOICE`; not set anywhere in
  this repo), which every `SqlEngine`, embedded included, installs.
  `TestOnlineDoltGCKeepsStoreUsable` proves both of the store's sessions
  stay valid.
- **Free space**: GC needs room for its output (the reachable new
  generation) before it frees the journal. An emergency GC at 0 free cannot
  run.

## Failure modes the fix must remove (written before the tests)

- **FM1** GC never runs between boots (periodic loop dead).
- **FM2** A store-size gate suppresses journal GC for good once history
  passes 5 GiB.
- **FM3** GC waits until free space is too small for its output.
- **FM4** A nearly full disk collects a near-empty journal on every tick
  (thrash).
- **FM5** Online GC invalidates the store's sessions or loses committed or
  working-set rows.
- **FM6** Online GC opens a second engine on the held workspace (lock
  contention or read-only).
- **FM7** A failed or deferred GC is invisible off the guest (the
  disposition file was guest-local only).
- **FM8** Growth GC cannot reclaim (history, live data) goes unreported until
  the disk is full.
- **FM9** The kill switch `RUNTIME_DOLT_GC_DISABLED` is ignored.
- **FM10** (residual) A huge journal left by an older binary makes the first
  boot GC slow (minutes), which holds health. Bounded journals remove it
  going forward.

## Belief and remaining error

- **Belief**: a live, online, journal-bounded GC with a headroom-scaled
  threshold removes the journal class (about 82% of this disk). The journal
  stays at or below min(1 GiB, free/4).
- **Remaining**: the history and live class grows about 6 GB/day on the
  paper. That fills the 48 GB disk in about a week after the fix, unless
  history is bounded (squash plus full GC, today offline only) or the commit
  cadence drops. The fix makes this visible (`compaction_needed` on
  `/health`) instead of silent. Bounding it is the next station.
- **Immediate operational risk**: the running paper computer is on the old
  binary and fills its disk again around 02:15 UTC on 10-15.

## Experiment results

The throttled CLI `dolt gc` on the 27 GB-journal copy was stopped at the
coordinator's request after 20 minutes, so that the real offline-GC runbook
had Node B's IO to itself. It had not finished. Measured up to the stop:
- Reads: over 50 GB. With the cgroup memory capped, chunks are re-read
  through the page cache.
- Disk: grew by **1.93 GB** before any journal byte was released. That is
  the free space a GC needs for its output.
- Memory: cgroup peak 8 GB, page cache included (1.5 GB swapped).
- The scratch copies were deleted.

Conclusion for the policy: a GC over a journal this large is slow,
memory-heavy and needs real free space. Hence:
- bound the journal at min(1 GiB, free/4);
- cap boot GC at a 4 GiB journal;
- collect an oversize journal left by an older binary offline (the runbook),
  or with the online GC, which runs while the computer serves.

## Fix (commit after this one)

- **The periodic GC is live and online.**
  - `StartPeriodicDoltGC(ctx, store, …)` uses the process-lifetime ctx.
  - It runs once at start and then every 5 minutes.
  - It runs `CALL DOLT_GC()` on the open store's own write connection,
    serialized with checkpoint commits. It never opens a second engine.
- **One policy for boot and online.**
  - GC runs when the journal ≥ min(`RUNTIME_DOLT_GC_JOURNAL_GIB` (1 GiB),
    free/4), floored at 64 MiB, or when free space ≤ 512 MiB (emergency).
  - The 5 GiB store-size guard, milestones and `skipped_size` are deleted.
- **Boot GC is capped at a 4 GiB journal.** A larger journal is recorded as
  `deferred_online`, and the online GC collects it once the computer serves.
  This keeps boot GC inside the wedge detector's 5-minute window.
- **The record is visible.** The disposition is written on every decision
  and served on guest `/health` as `dolt_gc`, with `mode`, `outcome`,
  `journal_mib`, `threshold_mib`, `avail_mib`, `after_*` and `duration_ms`.
  `compaction_needed` is set when free space stays under 25% after a GC,
  meaning history or live data, not the journal, fills the disk.
- Expected steady state on the paper:
  - the journal stays at or below about 1–2 GiB;
  - one GC of about 10–20 s every 6–25 minutes at 2.6–9.6 GB/h of journal
    churn;
  - the disk grows at the history-plus-live rate, about 6 GB/day, instead of
    2.6–9.6 GB/h.
- Remaining work (the next station): bound history. Options are an automated
  squash plus full GC (projection-compact, today offline only) or a sparser
  checkpoint cadence.
