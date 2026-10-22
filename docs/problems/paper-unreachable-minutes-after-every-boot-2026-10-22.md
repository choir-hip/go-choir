# The paper is unreachable for 4 to 6 minutes after every boot (2026-10-22)

Found 02:56 UTC, reading the paper (vm-1669020a) after deploy B
(3c46d5196). The trace is host-side only: vmctl's journal and the guest's
console log on Node B. No guest shell.

## Evidence

| Boot (refresh) | Booted | Runtime "started" | Health failures after boot | Healthy again |
|---|---|---|---|---|
| 7c61ce078, about 01:12 | — | — | 17 (15 s apart) | about 4 min |
| 4dbdee107 | 02:18:5x | — | 17 | about 02:23 |
| 3c46d5196 | 02:52:59 | 02:54:27 (124 s uptime) | about 25 | 02:59:12 |

The 3c46d5196 boot:
- The runtime finished its boot phases at 02:54:27 ("runtime: started").
  The dispatcher then showed 67 due agents with every slot full (research
  8/8, sources 4/4).
- The console went silent from 02:54:31 to 02:59:12, more than 4.5 minutes.
  `curl http://<guest>:8085/health` from the host timed out (000) for the
  whole window. Firecracker used about 60% CPU, with RSS 8.1 of 8 GiB.
- The first lines after the silence:
  `runtime: actor wake outbox list wakes=0 dur=3m45.517s`, then
  `objectgraph dolt: derived keys run.agent_id: validation pass starts over
  25202 rows (page 200)`.
- There are no kernel OOM, hung-task or soft-lockup lines. Host memory
  pressure was low (PSI some avg60 0.19).
- The 4dbdee107 boot's console was silent 02:21 to 02:22 and resumed at
  02:23, the same shape.
- CI's deploy step printed "Verified vm-1669 serves autoputer commit
  3c46d5196" at 02:54:25. That check passes before the window starts, so
  a deploy reports success while the paper is about to be unreachable.

## Is it a defect?

Yes. Every deploy, and every crash restart, makes the paper unreachable
for several minutes: the public front page's live reads, its desks and
the owner's view of it. FP1's acceptance says "the paper boots in under two
minutes", and the derived-key validation's first pass is "seconds of
engine time" (docs/definitions/choir-launch-fp1-store-throughput-2026-10-21.md).
Boot does finish in about 2 minutes, but the computer is not reachable
until about 6.

## Hypotheses (not confirmed by trace)

- H1, store contention after boot. A 3m45s read of the wake outbox means
  the store was held for minutes. The post-boot burst could hold the engine
  mutex: 67 due activations, the derived-key validation, and FP1's
  remaining scans. If `/health` touches the store, health fails with it.
- H2, the runtime HTTP server is starved, not blocked. 4 vCPUs at
  about 2.5 busy, with every slot full of confined workers (FP2 batch 1
  spawns a worker process per desk cell).
- H3, guest memory pressure. RSS has touched all 8 GiB. With no OOM line,
  reclaim may stall user space without killing anything. FP2's worker
  processes add memory per slot.

## Next observation

Read the paper's engine-mutex counters and `/proc/pressure` (the guest's
procmem endpoint) during the next boot window: the read path FP1 used at
00:52 (ledger), started at boot + 2 min. Find what `/health` touches in
code. Read the boot-window counts after the earlier deploys (01:12, 02:18)
to see whether the window is growing.

The fix belongs to FP1's store throughput work, or to the post-boot
admission ramp. Deploy B did not cause it.

## Code reading (FP1 follow-up, 2026-10-22, lane/fp1-boot-window)

Read from the code at 8afe1bb49. No trace was read in this pass; every
causal line below is a hypothesis until the measurements at the end
confirm it.

### What `/health` touches

- The route is `agentcore.(*APIHandler).HandleHealth`
  (`internal/agentcore/api.go:636`), installed by `apihandler.RegisterRoutes`
  (`internal/apihandler/routes.go:18`) and wrapped by the replay gate
  (`internal/autoputer/run.go:665-671`), which after replay only delegates.
- It makes **three store reads, all on the store's write connection
  `s.db`, with no time budget**:
  1. `CountPendingAgentMutations` (`api.go:665` →
     `internal/store/texture.go:1907`; `textureHandle()` is `s.db`);
  2. `selfdev.(*Store).ListByStates` (`api.go:666` →
     `internal/selfdev/operations.go:308`; the selfdev store holds
     `store.DB()`, which is `s.db`, `internal/store/store.go:203`);
  3. `Store.Head` inside `EffectiveRelease` (`api.go:694` →
     `internal/agentcore/effective_release.go:136` →
     `internal/store/computer_events.go:15`, `s.db`).
  The rest is in memory (`HealthState`, `RunningCount`, the startup
  manifest, statfs, the GC disposition file, `EngineMutexStats`).
- `s.db` is a pool of **one connection** (`configureEmbeddedDoltDB`,
  `internal/store/texture.go:352`). These reads do not take the engine
  mutex, but they queue for that connection behind every `ogStore`
  statement, the projector's `finalizeBatch` transactions, raw store
  writes, Dolt checkpoint commits and the online `CALL DOLT_GC()`.
- `HandleHealth` calls `WriteHeader` before the reads (`api.go:664`), so
  nothing reaches the caller until the reads return. A held `s.db` is a
  silent hang to curl (000), and vmctl's busy probe reads it as busy after
  its 4 s timeout (`internal/vmctl/ownership.go:3452`).
- By contrast `/api/runtime/observability` (`api.go:816`) bounds every read
  that can wait on a lock to 3 s (`observabilityReadBudget`, `api.go:886`),
  reads the engine mutex's live holder and waiters without the store, and
  dumps goroutines with `?goroutines=1`. It is the route that can see this
  window.

### The wake outbox list

- The log line is `sweepActorWakes` (`internal/agentcore/runtime.go:2721`).
  `dur` covers the whole list call, waits included.
- The list is `ListUnprojectedActorWakes` (`internal/store/lifecycle.go:861`)
  → `ogListAllByMetadata(choir.actor_wake_outbox, "projected", "false")` on
  the **read store** (`internal/store/graph_store.go:138`) →
  `ListObjectsByMetadataPage` (`internal/objectgraph/dolt_store.go:1258`),
  which takes the **engine mutex** for the statement. `$.projected` has no
  generated column and no derived-key family, so it is the JSON scan
  (`dolt_store.go:1193`) over every outbox row ever written: 2 to 4 s on
  the paper in steady state (E6 of research-more-slow-and-hidden).
- With `wakes=0` the first page is empty, so the 3m45s is **one engine
  acquisition plus one statement**. Either the wait for the engine mutex
  or the statement itself took minutes.
- 02:59:12 − 3m45.5s = 02:55:27: this list began a minute **into** the
  silence (health already failing at 02:54:31), and ended the moment
  everything resumed. It is a waiter, not the cause.

### Who else held the store

- **The derived-key validator is ruled out as the holder.** It starts after
  `rt.Start` (`run.go:689`). Its first act is the kind's `COUNT(*)` under the
  engine mutex, and it logs "validation pass starts over N rows" only
  after that COUNT returns (`internal/objectgraph/derived_keys.go:627-633`).
  The line came at 02:59:12, so it too waited the whole window for the
  engine and had held nothing yet. Its pages hold one page each (188 ms
  max on the paper at 00:52, ledger) at a 25% duty cycle.
- **The online Dolt GC is the one actor in the code that holds the store's
  only write connection for minutes, and it is started at exactly this
  moment.** `startPeriodicDoltGC` is called just before `rt.Start`
  (`run.go:683`; `run.go:716` on the no-replay path), and its loop runs one
  pass **immediately**, before its 5-minute ticker
  (`internal/store/dolt_maintenance.go:433`). That pass runs `CALL DOLT_GC()`
  on `textureHandle()` = `s.db` while holding `doltCommitMu`
  (`dolt_maintenance.go:327-335`). For its whole duration:
  - every `s.db` reader waits for the connection: `/health`'s three reads;
  - every `ogStore` call takes the engine mutex first and then waits for the
    connection, so the engine mutex is held by a victim for the GC's whole
    duration, and every read-store call (the wake list) queues behind it;
  - every writer that marks Dolt history dirty waits on `doltCommitMu`
    (`internal/store/store.go:240`; ten call sites);
  - Dolt's session-aware GC also waits for in-flight statements on other
    sessions to reach a safepoint (docs/problems/dolt-clustering-assessment-2026-10-15.md),
    so a long read-store statement can stretch the GC.
  The boot GC (`run.go:206`, before the store opens) leaves a journal over
  4 GiB to this online pass by design ("a larger journal is left to the
  online GC, whose first tick runs as soon as the computer serves",
  `dolt_maintenance.go:42-49`), and the replay writes journal between the
  boot GC and this pass. A 13 GiB journal took 148 s at boot with nothing
  else running.
  **One fact argues against it:** `rt.Start`'s boot phases write through
  `s.db` after the GC goroutine is started, yet "runtime: started" came at
  02:54:27, before the silence. Either the GC's pass was a no-op, or it got
  the connection only after `Start`'s last synchronous write. The console
  answers this (below).
- **The post-boot burst** (not ruled out): after "runtime: started" `Start`
  launches the projector's first full outbox sweep, the marker-gated
  outbox migration (one `GetObject` once migrated), the corpus search warm
  (every Texture document listed, then every head read,
  `runtime.go:916`, `internal/store/search_index.go:279`), Qdrant
  ensure, and the dispatcher's 67 due activations at 8/8 research and 4/4
  sources. Each is many short engine holds. They would make the engine
  slow, not stop it, unless one statement is minutes long.

### Ranking (code evidence only)

1. **H1, store held, and specifically H4 (new): the online Dolt GC's
   first pass at runtime start holds `s.db`.** `/health` cannot answer
   while `s.db` is held (three unbudgeted reads); the engine mutex convoys
   behind any `ogStore` call waiting for `s.db`; the two waiters in the log
   (wake list, validator COUNT) released in the same second; the console
   was silent because every actor waits on the store; 60% CPU fits a GC
   copying chunks. H4 has one counter-fact (above). If the console shows no
   online GC in the window, the holder is another `s.db` or engine-held
   statement, which the goroutine dump will name.
2. **H3, memory pressure**, as an aggravator: a slow holder is slower if
   the page cache holding the Dolt store is squeezed (8 GiB guest, daemon
   soft limit 4 GiB, `internal/gomemlimit/gomemlimit.go`, plus a
   worker per busy slot). Alone it does not explain a total log silence
   with no OOM or hung-task line.
3. **H2, an HTTP server starved of CPU**, last: at about 2.5 of 4 vCPUs busy
   Go's scheduler still runs the accept loop, and `/health`'s own work is
   microseconds. H2 predicts that every route hangs, including the
   observability route; H1/H4 predict that observability answers (with
   `unanswered` entries) while `/health` hangs. That is the discriminator.

## Measurements for FP0 (read-only, no guest shell)

**Now, before the next deploy (the evidence may already be there):**

- M0a. In the paper's console log on Node B, for each of the three boots
  (about 01:12, 02:18, 02:52:59): every `store: dolt gc (boot)` and
  `store: dolt gc (online)` line (starting, deferred_online, complete in
  T, journal X MiB → Y MiB). H4 is confirmed if an online pass starts at
  about runtime start and its "complete in" lands at the end of the
  silence (02:59:12 for the last boot).
- M0b. If the paper has not restarted since 02:52:59, GET
  `/api/runtime/observability` (the 00:52 read path) and read
  `engine_mutex`: the op with `max_hold_ns` near 4 minutes is the engine
  holder of the window. A cheap op (a `GetObject`, the validator page) with
  a minutes-long hold was itself waiting on `s.db`, so the holder was
  outside the engine mutex (the GC, the projector, a raw query). The
  `max_wait_ns` of the wake list and validator ops should be about 3m45s.
- M0c. `/health`'s `dolt_gc` block (the last GC decision). A later
  5-minute tick may have overwritten it with `noop`.

**During the next boot window** (from runtime start, 1 to 2 minutes
after the VM boots, every 10 s until `/health` has been 200 for a minute),
with `scripts/paper_boot_window_probe.py`:

- M1. `/health` from Node B, 5 s timeout: status or timeout per sample.
- M2. `/api/runtime/observability?goroutines=1` with the 00:52 headers, a
  20 s timeout: `engine_mutex_live` (holder op, held ms, waiters),
  `unanswered`, `memory`, and in the goroutine dump the frames
  `runOnlineDoltGC`, `DOLT_GC`, `database/sql.(*DB).conn`,
  `engineMutex).lockAs`, `HandleHealth`, `finalizeBatch`.
- M3. `/internal/debug/pprof/procmem` (host-sourced; the host credential
  when the guest enforces it): RSS by process, which says whether desk
  workers fill the guest.
- Reading: observability answers and the dump shows `runOnlineDoltGC`
  while `/health` hangs → H4. It answers and names another holder → H1
  with that holder. It does not answer either → H2 or H3; read M3 and
  `memory`.

## Fix candidates (not built; smallest first; the choice waits on M0/M1)

- **F1. `/health` never waits unbounded on the store** (orange: the guest
  health contract, read by vmctl's busy probe and the deploy wait). Bound
  the three reads to about 1 s each, as observability does, and on a miss
  answer at once with `status` "store_stalled", the engine's live holder
  and its age, and HTTP 503. That fails closed for vmctl: a non-200 is
  already "busy" (`ownership.go:3469`), the same as today's timeout, so no
  lifecycle decision changes. It turns a blind 000 into a diagnosis. It
  must not answer 200 while the store is stalled: the paper's readers are
  stalled too, and a green `/health` would hide that (the Goodhart). Test
  first: a store whose `s.db` is held answers `/health` within the budget
  with 503 and the holder. Rollback: revert. FP0 checks before landing
  that the deploy wait and the wedge detector treat a fast 503 as they
  treat a timeout.
- **F2, if H4 is confirmed. The online GC does not hold the serving store
  for minutes after boot** (red: store, boot path, Dolt maintenance).
  Smallest shape: the first online pass runs only when its journal is
  under a size it can collect in seconds; a larger journal waits for the
  next planned restart, where the boot GC runs before the listener and the
  host already expects the computer to be away (the 4 GiB boot cap
  stays). An emergency (avail ≤ 512 MiB) still runs at once: a full disk
  is worse than an outage. Test first: a large journal at runtime start
  does not call `DOLT_GC` while serving, a small one does, and an
  emergency does. Rollback: revert; `RUNTIME_DOLT_GC_DISABLED` is not the
  rollback (it stops collection and the disk fills). Residual: a journal
  that keeps growing past the cap between planned restarts needs a
  scheduled maintenance window, a decision for the owner.
- **F3, if the holder is the post-boot burst. A post-boot admission ramp**
  (orange: the dispatcher). Admit due activations at a rising rate over
  the first minutes (for example 2, then 4, then the full slots) and
  start the corpus search warm after the ramp. Rollback: revert, or a ramp
  of zero. It does not help if one statement holds the engine for minutes.
- **Not proposed: making the validator yield more.** It held nothing in
  this window (above).
