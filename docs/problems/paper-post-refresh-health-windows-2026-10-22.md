# The paper's health has two failing windows after a planned refresh (2026-10-22)

Date: 2026-10-22, 18:50 UTC. Found by: FP0, after deploy V's planned
refresh of the paper. Status: open, problem record before any fix.
Mutation class of a fix: red (guest boot path and the paper's
availability).

## Evidence

Deploy V refreshed the paper (vm-1669020a…, critical_protected, busy) at
18:30:28. Its new address was 10.201.173.2. The guest started its server
at 18:30:39 after a 3.6 s store open. vmmanager checks /health every 15 s
with a 3 s timeout and logs only failures
(`internal/vmmanager/manager.go:443-444`, `1597-1607`, `2749-2756`):

- "booted VM" at 18:31:01 means a 200 was seen at about 18:30:40.
- Window A: failures at 18:31:00, 18:31:15, 18:31:30, 18:31:45, 18:32:00.
  It matches the boot phase `passivate_interrupted_activations`
  (18:30:42 to 18:32:04, 81.8 s); the console is silent from 18:30:46 to
  18:32:01 inside it.
- No failures from 18:32:15 to 18:33:30.
- Window B: one failure at 18:33:45, and an operator curl at 18:33:44
  that timed out at connect (10 s), not a refusal or a slow answer. 200 by
  about 18:34:25.

Read-only host checks at 18:49: no guest firewall rebuild between 18:33:20
and 18:34:10 in the vmctl journal, no other computer failed health at
18:33:4x, no conntrack messages in dmesg, host CPU pressure zero.
`boot-timeline.json` has no `first_healthy` field.

## What it is not

- Deploy E's fix holds: the wake-outbox list took 2.65 s this boot
  (3m45s before E).
- The host firewall rules for the new tap are installed before
  Firecracker starts (`manager.go:3616-3660`); the guest answered at the
  new address within a second of its listener.
- /health reads memory and three store reads under a 1 s budget and
  answers 503 `store_stalled` on a miss (`internal/agentcore/api.go`); it
  takes no engine lock and cannot cause a connect timeout.

## Open

1. Window A: the passivation phase grows deploy by deploy (48, 52, 81 s).
   During it one run step held about 75 s. Why passivation takes this
   long, and whether /health fails as 503 or as a timeout then (vmmanager
   discards the status).
2. Window B: a connect timeout on a listening guest. Hypotheses: the guest
   stalled (memory pressure: the daemon's limit plus the desk cgroup's
   memory.max add up to all of guest RAM; or vCPU starvation), or its
   accept queue overflowed. No route reads `ListenOverflows`.
3. The derived-keys validation pass re-reads every row of each family
   (46,381 commitment_record rows) after every process start, during the
   post-boot burst, because its validated state is memory-only. It is not a
   boot-path scan (a goroutine after `rt.Start`) but it competes with the
   burst.
4. Side finding: vmctl learns the new address only after the boot wait, so
   the guest's calls to vmctl are refused until then (a memory-lease
   release at 18:30:39 was refused; a report got through on retry at
   18:31:12).

Next observation: during the paper's next planned refresh (KF step 5),
probe /health every 2 s with `curl -w '%{http_code} %{time_connect}
%{time_total}'`, and read the guest's `/proc/net/netstat` ListenOverflows
and `/proc/pressure/memory` through an existing host-only route if one
exposes them.

## Steady-state 503s, not only after a refresh (FP0, 21:15)

FP0's 2 s probe of the paper's `/health` from Node B, 20:08 to 21:08
(`_runs/deployKF/paper-watch-kf.txt`, 3,416 probes): 3,160 answered 200,
218 answered 503, 19 failed to connect (the 20:05 host reboot), one was a
probe of the old address. The 503s are not confined to the boot and refresh
windows (20:54-20:58 holds 27 of them): they come in bursts of 1 to 16 per
minute every 3 to 10 minutes through the whole hour (for example 20:17,
20:29, 20:37, 20:44, 21:03-21:04). Each 503 took 1.000 s, exactly /health's
store-read budget, with a fast connect: the store reads missed the budget,
the listener did not stall. A five-minute capture of 503 bodies from
21:09 found none (bursts are intermittent).

At the 21:03-21:04 burst the paper's console shows it fully loaded: all
eight Texture activation slots busy (`dispatcher: activation wait ...
slots=8/8 due`, waits of 1m39s-1m47s), story publishes and verdicts several
per second.

Hypothesis (not yet confirmed from a trace): /health's three store reads
queue behind the store's writers under full Texture load, so the paper's
health signal turns 503 in bursts while it is working normally. Why it
matters: vmmanager polls /health every 15 s, and a run of 503s reads as an
unhealthy computer to anything that acts on it; the runbook's "paper
healthy" test would miss it, and an owner-facing status would flap.
Capture at 21:19:07, 21:19:09 and 21:19:12 (`_runs/paper-503-bodies.txt`):
all three reads unanswered (`desk_pending_mutations`,
`selfdev_active_operations`, `effective_release`), `running_runs` 8,
`engine_mutex_live.waiter_count` 0. So no caller was waiting on the engine
mutex; the reads were waiting for the store's only connection
(`internal/store/texture.go:353`, `SetMaxOpenConns(1)`), which some other
caller held for over a second, three times in five seconds. /health is
reporting that honestly; the question is what holds the one connection
that long under full load.

Holders found so far (FP0, 21:30; code read by a worker; the paper's
console and `/api/runtime/observability`):

- Online `DOLT_GC` (`internal/store/dolt_maintenance.go`, a 5-minute
  ticker, `CALL DOLT_GC()` on the write connection) ran ten times from
  19:40 to 21:24, every 10 to 20 minutes, for 10 to 25 s each, each time
  because the journal had grown past its 1,024 MiB threshold again (1,105 to
  1,717 MiB). Four 503 bursts sit inside a GC window: 20:17 (GC
  20:17:09-20), 20:37 (20:37:09-25), 20:52 (20:52:09-24), 21:04
  (21:04:20-36). So the paper's whole store stops for 10-25 s every 10-20
  minutes, by design, and the journal grows by over 1 GiB between GCs
  (about 1-2 MB/s of journal while working; see the disk-pressure record).
- Inline `DOLT_COMMIT`s are not it: 217 commits, the longest 32 ms
  (`guest_checkpoint.inline_max_ns`).
- Not attributed yet: the bursts at 20:29-20:30, 20:44-20:45 and 21:19.
  Next probe: during one, `GET /api/runtime/observability?goroutines=1`
  names the goroutine inside the Dolt engine (the health reads wait in
  `database/sql.(*DB).conn`).
- Discovered heresy (from the code read): `engineMu` exists because
  concurrent queries on the shared embedded engine race
  (`objectgraph/dolt_store.go:63-67`), yet raw `s.db` queries (the
  /health reads, `finalizeBatch`, `DOLT_COMMIT`, `DOLT_GC`, Texture SQL,
  selfdev) run beside read-pool object-graph queries without it.

Open: which store calls hold the connection over a second while eight runs
are live (a per-call hold-time log above a threshold would name them);
whether /health's three reads should come from memory instead of the
store, so the health signal stops turning on load.

### Findings: an instrument that names the holder at the stall (lane `lane/fp4-store-stall-holder`, 2026-10-22)

Why the existing probes miss it. The bursts at 21:30:27 and 21:38:45 UTC
are not inside a `DOLT_GC` window, and an external
`/api/runtime/observability?goroutines=1` taken 1 to 2 s after a 503 found
no goroutine inside the Dolt engine: by then the holder had let go.
`engine_mutex_live` sees only callers of the object graph's engine lock,
and the holders here are raw `s.db` callers, which never take it. Nothing
records how long callers wait for the write pool's one connection, so the
growth of that wait between stalls is invisible too.

What the lane builds (mutation class orange: observability on the guest
runtime; the store and /health's answers do not change):

1. In /health's `store_stalled` branch, after the 503 is written (so the
   answer is not delayed), at most once a minute, the runtime takes one
   goroutine snapshot and logs one line per goroutine that is inside the
   Dolt or go-mysql-server engine and not parked in
   `database/sql.(*DB).conn` (the health reads themselves and the other
   waiters): its first choir frame and its first two engine frames, then a
   header with the write and read pools' `sql.DBStats` (in use, wait count,
   wait duration). Bounded: at most 16 goroutines, each line and the total
   capped in bytes. The lines go to the standard log, so they reach the
   console and `recent_log`. This is taken at the moment the reads gave up,
   while the holder still holds the connection.
2. The same pool stats on `/api/runtime/observability` (`store_pools`), so
   wait-count and wait-duration growth shows without a stall. /health's 200
   body is left byte-for-byte as it is (its test pins the keys).

Not built: a per-call hold-time log on every store call (it would touch
every raw `s.db` caller); moving /health's reads off the store. Both stay
open above.
