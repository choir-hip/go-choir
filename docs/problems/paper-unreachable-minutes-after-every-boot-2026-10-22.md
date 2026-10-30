# The paper is unreachable for 4 to 6 minutes after every boot (2026-10-22)

Status 2026-10-30: boot unreachability is a live concern, but the Dolt store-open and `gc` causes (about 30 references below) are historical; the guest store is SQLite now. Design source for the new data system: [the data system frame](../design/choir-data-system-frame-2026-10-30.md).

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
engine time" (docs/archive/definitions/choir-launch-fp1-store-throughput-2026-10-21.md).
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

## Holder found by trace (2026-10-22 03:45 UTC, deploy C boot)

The 3c16f7c33 refresh booted the paper at 03:39:46, and the runtime started
at 03:41:01. The console went silent and `/health` timed out from 03:40. The
host-side sampler (`_runs/bootwin/poll.sh`, read-only) shows the guest
observability route answering throughout. So the HTTP server is not starved
(H2 refuted). `/health` blocks because `HandleHealth`
(`internal/agentcore/api.go:636`) reads the store per request
(`pendingDeskMutations`, `activeSelfdevOperations`, and `Store.Head` under
`EffectiveRelease`), after `WriteHeader` and with no deadline.

A goroutine dump taken through the host-only pprof route at 03:44:56, mid-stall
(46 goroutines), shows:
- 12 goroutines in `objectgraph.(*engineMutex).lockAs` for 3 minutes:
  `DoltStore.GetObject` from 8 dispatcher activations.
- One goroutine, created by `Runtime.Start` (`runtime.go:882`), in a Dolt row
  scan (`prollyIndexIter.Next` → zstd decompress) inside
  `Store.LatestComputerEventByIdempotencyPrefix`
  (`internal/store/computer_events.go:360`). Its call chain is
  `resumePendingPlatformUpdateAfterReady` → `settleOrResumePlatformUpdate`
  → `resumePendingPlatformUpdate` → `resumeStrandedPlatformUpdateTail`
  (`internal/agentcore/platform_update.go:882`).

The query:
`SELECT event_json FROM computer_event_index WHERE computer_id=? AND
status='finalized' AND idempotency_key LIKE 'platform-update-applied-%'
ORDER BY sequence DESC LIMIT 1`.
The table has `UNIQUE(computer_id, idempotency_key)`. But `ORDER BY sequence
DESC LIMIT 1` steers the plan onto `idx_computer_event_index_status
(computer_id, status, sequence)`. That walks every finalized event
newest-first and fetches each row (with `event_json`) to test the key. When
no applied platform update exists, it reads the whole event index. That is
the paper's case, if it has never applied one. The paper takes a planned
refresh on every deploy, so this runs on every boot, after ready.

M0a result (console GC lines): no online Dolt GC ran in the 02:54:31 to
02:59:12 window. The first came at 03:02:35. So H4 (GC) is not this
window's holder.

### A second, separate finding: the online GC every 10 minutes

The paper's journal grows past 1 GiB every 10 minutes:
- 03:02:35: 1034 MiB, 19 s;
- 03:17:35: 1772 MiB, 31 s;
- 03:27:35: 1613 MiB, 28 s;
- 03:37:35: 1634 MiB, 30 s.

Each pass stalls the store for 20 to 30 s. `/health` failed at 03:37:53,
inside the 03:37:35 to 03:38:05 pass. About 1.6 GiB of journal per 10
minutes is its own question: write amplification on the paper.

### Repair (orange: one query, same result)

`LatestComputerEventByIdempotencyPrefix` range-scans the unique
`(computer_id, idempotency_key)` key for the prefix. It selects only
`sequence` and `event_digest`, keeps the highest finalized sequence in Go,
then reads that one event by digest. The prefix family has one row per
transition, so the scan is bounded by the family, not the table. Tests
first:
- latest-of-several;
- absence;
- a prefix that is a LIKE metacharacter;
- a non-finalized newer row ignored;
- a store with thousands of unrelated events, which must not walk them
  (a call-count or EXPLAIN check, whichever the store test kit supports).

Separately, F1 (health reads bounded, with a fast 503) stays a candidate,
so that any future slow holder can't make `/health` hang.

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
    sessions to reach a safepoint (docs/archive/problems/dolt-clustering-assessment-2026-10-15.md),
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
- M3. `/internal/debug/pprof/procmem` (host-sourced: the guest gates it on
  the host source address, and guests run hostauth off, so it takes the
  marker header only; never send a guest the host-internal token,
  docs/problems/host-internal-token-sent-to-guests-by-fp0-probes-2026-10-22.md):
  RSS by process, which says whether desk workers fill the guest.
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

## Repair built (FP1, 2026-10-22, lane/fp1-boot-window)

Written against FP0's "Holder found by trace" section (origin/main
53c8bb181, which this worktree could not fetch; the lane is rebased on
664ce049a and FP0 rebases it). The holder: `resumeStrandedPlatformUpdateTail`
(`internal/agentcore/platform_update.go:882`) calls
`Store.LatestComputerEventByIdempotencyPrefix` after ready, and its
`LIKE ? ORDER BY sequence DESC LIMIT 1` walks
`idx_computer_event_index_status (computer_id, status, sequence)`
newest-first and reads `event_json` for every row to test the LIKE. A
family whose newest member is old (one `platform-update-applied-*` per
deploy) costs the whole finalized index, with the one `s.db` connection
held.

- **Tests first** (`internal/store/computer_event_prefix_test.go`, b6523c170,
  37cab03d4): the newest finalized member of several, with a newer prepared
  member ignored; absence (empty, prepared only, another family, another
  computer); `_`, `%` and `\` literal; neighbouring keys (`.` after `-`, the
  prefix minus a byte, a multi-byte member); the right answer among 5,000
  newer unrelated events; and the plan. Against the old query the five
  behaviour tests passed (it was correct, only slow) and the plan test failed
  with `index: [computer_id, status, sequence]`.
- **Why EXPLAIN and not a time bound:** a time bound on the embedded engine
  under `-race` is noise, and it does not say which rows were read. The plan
  says it: a range on the unique `(computer_id, idempotency_key)` key, no
  Sort or TopN, no `event_json` in the family read. The cost is the
  family's size, not the computer's event count.
- **Fix** (40cdc19fc, orange): one range read `idempotency_key >= prefix AND
  idempotency_key < upper` on the unique key, selecting `sequence,
  event_digest, status, idempotency_key`; the newest finalized member
  picked in Go; then `EventByDigest` for that one event. `upper` is the
  prefix with its last rune incremented (code-point order, which the
  column's `utf8mb4_0900_bin` keeps; surrogates skipped; a non-UTF-8 or
  all-U+10FFFF prefix is unbounded above), and `strings.HasPrefix` keeps the
  result literal under any collation. `escapeLikePrefix` is deleted (no
  other caller). No schema change.
- **Siblings:** no other caller of the function and no other
  `LIKE`-prefix read of `computer_event_index` (git grep).
  `FinalizedDecisionForOperation` (`internal/store/computer_event_recovery.go:12`)
  has a related shape: it walks finalized `effect_accepted`/`effect_rejected`
  events newest-first through the same status index and decodes
  `event_json` per row until one matches the operation. It is bounded by the
  number of decisions, not all events, and it is not on the boot path
  (self-development API, materializer, standing consensus, release witness).
  Residual: give it an indexed key (the operation id) if its counters ever
  show it.
- **Rollback:** `git revert 40cdc19fc`.

**Next candidate, not built: F1, bounded `/health` reads** (orange; above).
This repair removes the boot-window holder. It does not stop any other long
`s.db` holder from silencing `/health` the same way.

**FP0's deployed proof:** in the next paper boot window, `/health` answers
200 within seconds of "runtime: started" (no window of failures after it),
and the console shows no `actor wake outbox list` line with a minutes-long
`dur`. If it fails, run `scripts/paper_boot_window_probe.py` from runtime
start and keep its goroutine dump: the holder's frame is in it.

## Deployed result (FP0, 2026-10-22 05:13 UTC, deploy E d991907e6)

The repair is on main as 8388838ee, with DeepSeek's edit 18e02d303: a prefix
that is not UTF-8 is absence. It was deployed in CI 39725541974.

| Boot | Kill | Boot | /health 200 | Boot to healthy |
|---|---|---|---|---|
| deploy C (before) | 03:39 | 03:39:46 | 03:46:11 | about 6.5 min |
| deploy D, first | 04:18:54 | 04:19:27 | 04:21:36 | about 2 min |
| deploy D, second | 04:29:35 | 04:30:08 | 04:32:30 | about 2.4 min |
| deploy E (repair) | 05:12:26 | 05:12:59 | 05:13:29 | about 30 s |

vmctl logged three unhealthy probes in the deploy E window (05:12:58,
05:13:13 and 05:13:28), all within the first 30 s after boot. Then /health
stayed 200 through the derived-key validation passes, which ran from 05:13:34
on. The slowest boot phase is still passivate_interrupted_activations, at 52 s
(48 s on deploy D). The pass condition holds: /health answers 200 within
seconds of "runtime: started", which the guest clock logged at 05:13:32.

Still open: F1 (bounded `/health` reads, lane/fp1-health-bounded, in review
edits for its callers' attempt budgets). F2 is the online GC's 11 to 30 s
store stall every 10 to 17 minutes; telemetry batching's staging gate is
the first lever against it.

## F1: `/health` never waits unbounded on the store (lane/fp1-health-bounded)

Orange: the guest health contract. Read from the code at a55660c0d.
`HandleHealth` (`internal/agentcore/api.go:636`) makes three store reads
with no deadline: `CountPendingAgentMutations`, `selfdev ListByStates` and
`Store.Head` inside `EffectiveRelease`. All three use the store's write pool
`s.db`, which has one connection (`configureEmbeddedDoltDB`). They wait for
it behind any holder: an `ogStore` call holding the engine mutex, a
projector batch, a Dolt commit, or the online GC. It also calls
`WriteHeader` before the reads. So a held `s.db` leaves the caller with no
status line until the holder lets go: a blind 000.

### Shape

The three reads run concurrently under one 1 s deadline, before
`WriteHeader`. If all three answer in time, the response is the same 200
body as today, byte for byte. If one misses, `/health` answers at once with
503 and `status` "store_stalled". The body names the reads that missed, the
budget, and the engine mutex's live holder, its age and its waiters
(`Store.EngineMutexLive`, the same block the observability route reads).
`desk_pending_mutations` and `selfdev_active_operations` are left out of
the 503 body: they are unknown, and a 0 would read as "idle". The layering
boot guard's status still wins (it is 503 already). The budget is shared,
not 1 s per read in sequence, because the proxy's runtime probe gives up at
2 s (`internal/proxy/compute_status.go:666`).

### Ways it could fail (written before the code)

1. **A late result races the response.** The read helpers swallow errors
   and return 0. A read cut off by the deadline returns 0 quickly, and a
   `select` that takes that value would answer 200 with
   `desk_pending_mutations: 0` while the store is stalled. A value is kept
   only if the read finished before its context ended; anything else counts
   as a miss. A read that finishes right at the deadline is a miss too, so
   the error is toward 503.
2. **The bounded read leaks a goroutine or a connection.** The abandoned
   read's context is cancelled when the handler returns, so a read still
   waiting for the `s.db` connection gives up at once (database/sql honours
   the context while it waits). A read already inside a Dolt statement runs
   until the statement ends, then writes to its own buffered channel and
   exits. It never touches the response. Each poll spawns at most three
   such goroutines, and they end at the deadline, so a minutes-long stall
   polled every few seconds does not pile them up. No connection is held
   past the statement: `QueryRowContext(...).Scan` and `rows.Close` release
   it.
3. **WriteHeader already sent.** Today 200 goes out before the reads, so a
   503 is impossible without moving the reads first. The fix moves them.
4. **A slow but healthy store flaps to 503.** A read that legitimately takes
   over 1 s (CI notes a migration step that holds the store about 3 s,
   `.github/workflows/ci.yml:1157`) now gives a 503 where today it gave a
   late 200. Every caller below retries, and a 503 is treated as busy or
   not-yet-healthy, so this delays a deploy wait by one poll; it does not
   change a lifecycle decision. Flapping between 200 and 503 under load is
   visible, by design: the paper's readers wait on the same connection.
5. **A body a caller parses as a different 503.** `vmmanager` reads 503
   `status` "refused" (boot refusal) and "replaying" (replay progress). The
   new status is "store_stalled", which is neither, so it never fails a boot
   or extends a replay window.
6. **A faster failure uses up an attempt-counted retry budget sooner.**
   Callers that count attempts, not wall time, burn through them faster on a
   fast 503 than on a 5 s timeout (see the table). This matters for the
   updater's post-apply probe and the CI deploy waits.
7. **The engine holder is not the stall.** `s.db` can be held by something
   outside the engine mutex (the online GC, a raw query), and then
   `engine_mutex_live` names no holder, or a victim. The body says which
   reads missed. It does not claim to know the holder.

### Callers: a fast 503 compared with a timeout

| Caller | Fast 503 "store_stalled" | Timeout (today) |
|---|---|---|
| vmctl busy probe `httpGuestBusy` (`internal/vmctl/ownership.go:3452`, 4 s) | non-200 → busy | error → busy. Same decision, answered in about 1 s instead of 4 s. |
| vmctl deploy-refresh sweep `httpGuestServedCommit` (`internal/vmctl/deploy_refresh_retry.go:319`, 10 s) | non-200 → "" (never matches a target) | error → "". Same. |
| vmmanager `waitForGuestReady` (`internal/vmmanager/manager.go:2706`, 3 s per probe) | not healthy, not refused, not replaying → keeps polling until `BootReadyTimeout`. Marks `first_http_response` earlier (timeline detail only). | error → keeps polling until `BootReadyTimeout`. Same deadline. |
| vmmanager `CheckHealth` and adopt (`manager.go:1335`, `:1505`) | unhealthy (the adopt path adopts it unready) | unhealthy. Same. |
| vmctl wedge detector (`internal/vmctl/wedge.go`) | TCP connect only, never `/health` → not wedged | not wedged. Same. |
| proxy `/api/compute` runtime status `probeRuntimeHealthForTarget` (`internal/proxy/compute_status.go:654`, 2 s) | `reachable: false`, `status: "store_stalled"` | `reachable: false`, "runtime health unavailable". Same reachability, now with a cause. |
| proxy `/health` upstream `probeUpstreamHealth` (`internal/proxy/handlers.go:1647`, 2 s; static routing only) | unhealthy → "degraded", and the upstream build is shown | unhealthy → "degraded", no build. |
| choir-updater post-apply probe `HTTPHealthProber` (`internal/updater/runtime.go:124`, 5 s client, 30 attempts 1 s apart, 15 min cap) | non-200 → uses one attempt, about 2 s each | error → uses one attempt, about 6 s each. **Same verdict, budget about 60 s instead of about 180 s of stall.** |
| CI deploy identity wait (`.github/workflows/ci.yml:1156`, `curl -f --max-time 5`, 120 tries 1 s apart) | `curl -f` fails → empty commit → retry, about 2 s per try | timeout → retry, about 6 s per try. **Same verdict, budget about 4 min instead of about 12 min.** |
| CI refresh shortcut (`ci.yml:1625`) and rescue loop (`ci.yml:1711`) | empty → posts to vmctl, or retries | the same. The rescue loop is attempt-counted as above. |
| CI host-service probe (`ci.yml:1825`) | host services only, not the guest | — |
| Frontend Settings (`frontend/src/lib/SettingsApp.svelte:231`) | reads the proxy's own `/health`, not the guest's | — |
| `scripts/paper_boot_window_probe.py` (FP0) | records the status and the holder | records a timeout |

No caller turns a fast 503 into a different lifecycle decision than a
timeout. Two attempt-counted waits (the updater's post-apply probe and CI's
deploy waits) had about a third of their wall-time budget during a stall:
about 60 s instead of about 180 s, and about 4 min instead of about 12 min.
That would fail the documented 4 to 6 minute boot window in CI, where today
it passes (bunch 18 review, DeepSeek, High).

**Repaired in the lane (bunch 18):**
- The updater charges a run of `store_stalled` answers by wall time
  against `attempts x (client timeout + interval)`, the time the same
  attempts took as timeouts. It does not count them one attempt each. A
  deadline, not a sleep, so the probe still polls every second and passes
  at the first healthy answer. Other failures count attempts as before.
- CI's `wait_for_autoputer_commit` and the rescue loop call one helper,
  `guest_health`, which sleeps 4 s after a `store_stalled` 503, so the try
  costs what the timeout did. A sleep, not a deadline: a deadline would
  also stretch the non-stall fast failures (refused connection, other
  503s) from about 2 min to 12 min.


## Merged from docs/problems/paper-post-refresh-health-windows-2026-10-22.md (2026-10-22)

**The paper's health has two failing windows after a planned refresh (2026-10-22).**
Date: 2026-10-22, 18:50 UTC. Found by: FP0, after deploy V's planned refresh of the paper. Status: open, problem record before any fix. Mutation class of a fix: red (guest boot path and the paper's availability).
- receipt yet):

(Full text in git history at the path above.)
