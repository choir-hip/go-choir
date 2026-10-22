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
