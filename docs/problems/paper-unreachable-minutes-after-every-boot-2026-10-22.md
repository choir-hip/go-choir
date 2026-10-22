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
