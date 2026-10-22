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

## Window A findings (FP4 lane boot-passivation, 2026-10-22, from code and the 18:30 console)

Read from the code at 420aeae30 and `~/runs/reviews/paper-console-1830.txt`.
No guest trace beyond the console was read, so the step attribution in A3
is an inference from the log order, confirmed in shape by a local probe,
not by a paper trace.

**A1. What the phase does per activation.** `Runtime.Start`
(`internal/agentcore/runtime.go:885-886`) runs
`passivateInterruptedActivations` (`runtime.go:2636-2687`) only on a
marker boot. It lists every run header of the owner
(`forEachPreBootRun`, `runtime.go:2561-2582` ->
`ListRunsByOwnerStates`, `internal/store/graph_store.go:1020` ->
`ListAllObjectRefsByKindOwner`, `internal/objectgraph/dolt_store.go:1636`,
no LIMIT, under the engine lock) and keeps pending and running ones.
Then per run: a management task run is cancelled and its task closed
(`runtime.go:2640-2658`); any other run is set passivated with reason
`runtime_restarted`, and before its `UpdateRun` the phase calls
`ensureSpawnedCoagentWorkItem` (`runtime.go:2666`). For a research,
management or engineering run with a requester, that reaches
`CreateWorkItem` (`internal/store/trajectory.go:146`), which holds
`trajectoryMu` and looks the work item up by fingerprint
(`trajectory.go:169-170` -> `findWorkItemByFingerprint`, `:198` ->
`ogListByMetadata(choir.work_item, objective_fingerprint)`). No generated
column or derived key covers `objective_fingerprint`
(`dolt_store.go:219-226`, `derived_keys.go:76-80`), so the query is
`JSON_EXTRACT(CAST(metadata AS JSON), '$.objective_fingerprint')` over
every `choir.work_item` row (`dolt_store.go:1250-1252`), run while
holding the shared engine lock (`dolt_store.go:1283`; the read store
shares it, `internal/store/store.go:809-813`). Then `UpdateRun`,
`MarkAgentMutationStale` for Texture runs, and one event.

**A2. The work item it ensures already exists, and nothing reads the
boot copy.** The spawn path creates the same work item with the same
fingerprint (`runtime.go:1445`, through the same function) and applies
the requester's Texture metadata (`runtime.go:1350`). The boot call's
metadata key `passivated_spawned_work_item_id` has no reader anywhere in
the tree. The planned resume does not use the work item:
`handleInitialDispatch` resumes on `State == passivated` and
`passivated_reason == runtime_restarted` alone
(`internal/actorruntime/handler.go:551-575`). So the boot call is a
second, idempotent ensure whose only effect is the lookup (and, for a run
whose spawn-time item was lost, a work item minted at boot, the shape
Cut 0 invariant I3 rejected for crash boots,
`crash_boot_close_test.go:14-19`).

**A3. The 75 s step.** Console order: the two task closes end at
18:30:46.311; the next line is run d7b839a3 passivated at 18:32:01.222;
the 17 runs after it take 120 to 300 ms each. Every other research run in
the log prints "spawned work item skipped … requested_by=" (empty
requester, so `createSpawnedCoagentWorkItem` returns at
`runtime.go:1512-1514` before any lookup). d7b839a3 prints neither that
line nor an error, so it is the one run that went past the requester
check, into the work-item lookup. Inference: the 75 s is one
`objective_fingerprint` JSON scan over all of the paper's work items.
A local probe (planned boot, 300 work items, one spawned run with a
requester, one Texture run, one orphan research run; engine-lock stats
before and after the phase) shows exactly one
`ListObjectsByMetadata choir.work_item $.objective_fingerprint json`
call, for the spawned run only. Every other call in the phase is a
point read or an indexed read.
If the run's trajectory is a lifecycle trajectory, the same boot call
goes to `OpenLifecycleWork` instead (`runtime.go:1552-1566`). That
returns early from the spawn-time replay receipt (`lifecycle.go:2911`).
If the receipt is missing, it reads a whole snapshot of the computer:
every object of nine kinds (`scanLifecycleSnapshotObjects`,
`lifecycle.go:2319` -> `ReadObjectSnapshotFiltered`,
`dolt_store.go:815`). That second scan is behind the same call, so one
deletion removes both.

**A4. Why it grows deploy by deploy.** Every spawned coagent run creates
a work item and none is deleted, so the scanned set grows with the
paper's lifetime of spawns (the paper's run count went from 25,657 at
05:13 to 30,268 at 18:32 today). 48 s, 52 s, 81.8 s follow it. The
phase's other cost that grows is the owner-wide run header list: 2.7 s
at 18:30:42-45 for 30,268 run headers (A1). It is a scan of all runs
of the owner, not of all og_objects, and it is not the window.

**A5. Why /health fails during it.** /health's three store reads
(`internal/agentcore/api.go:663`, `readHealthStore` `:762`) need the same
engine and its single write connection. A 75 s statement holding the
engine lock makes every probe miss the 1 s budget, so /health answers
503 `store_stalled` (F1). The console's 18:30:46 to 18:32:01 silence and
vmmanager's failures at 18:31:00 to 18:32:00 coincide with the one step,
not with the phase: the phase's first 4 s and last 3 s do not hold the
store long. vmmanager discards the status, so 503 against a timeout is
inferred from the code, not observed.

**A6. Smaller items seen.** 3 s pass between the last startup line
(18:30:39) and "boot phase begin" (18:30:42): `configureTelemetryBatching`
and `configureGuestCheckpoint` run first in `Start`
(`runtime.go:869-870`); not measured further. The same JSON lookup runs
on every live spawn (`runtime.go:1445`), so each spawn on the paper
holds the engine lock for a scan of all work items; that is off the boot
path and is a residual (R-A2 below).

### Fix shape (red, guest boot path)

Delete the boot call at `runtime.go:2666`. The run is still passivated
with reason `runtime_restarted` on a marker boot and closed on a crash
boot; the one resume is untouched. The phase then makes only point and
indexed reads per run. Add a per-run log when one run's passivation
takes 2 s or more, naming its profile, so the next slow step names
itself.

### Residuals

- R-A1: the owner-wide run header list (2.7 s, grows with runs). Needs a
  state access path; adding a generated column is a schema change over
  all og_objects and is out of the boot path's rules.
- R-A2: `findWorkItemByFingerprint` is a JSON scan of every work item on
  every live spawn. Fix with a deterministic work-item id (as the
  lifecycle branch already does, `runtime.go:1553`) or a derived key.
- R-A3: the 3 s before the first boot phase (A6).

### Window A fix and proof steps (lane/fp4-boot-passivation)

Fix: c85b7f394 (red), test d8bea520e (yellow, fails before the fix with
`ListObjectsByMetadata choir.work_item $.objective_fingerprint json ran 1
times`). Local: the test and the planned/crash boot, management-task,
source-schedule, selfdev crash-close and planned-restart suites pass;
actorruntime passes.

Proof (FP0, at the paper's next planned refresh after the fix deploys):

1. Run `ops.sh watch` on the paper across the refresh. Pass: no vmmanager
   health failure between "runtime: boot phase begin
   passivate_interrupted_activations" and "boot phase end". Window B (a
   connect timeout about 3 minutes later) is a separate open item; record
   it, do not count it against A.
2. From the guest console or journal, read the boot timeline:
   `grep -E 'boot phase (begin|end)|passivated run|closed management task run|owner-scoped'`.
   Pass: `passivate_interrupted_activations dur=` well under 10 s (the
   expected order is the 2.7 s header list plus about 0.2 s per run). Every
   "passivated run … dur=" line is under 2 s. No line reads "spawned work
   item skipped" or "create spawned work item" (both are deleted).
3. The restart rule: the console still reads "boot follows a planned
   restart … pre-boot work resumes", and the parked runs resume. The
   dispatcher's "activation wait" lines for them follow "runtime: started".
   A crash boot (none planned for this proof) is unchanged.
4. If a run's dur= is still slow, its profile in the same line names the
   path. Record it here before any further fix.

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

Built (`7a7fdee1e`, tests in `df538f65a`; not yet deployed, so no staging
receipt yet):

- `internal/agentcore/api_health_stall_holder.go`: the 503 branch starts
  the capture in a goroutine after `writeAPIJSON`, so the 503 still answers
  at the 1 s budget (the test checks under 2 s). One log write per capture:
  a header `store_stalled holders: computer=... unanswered=...
  engine_goroutines=N shown=M parked_in_conn=K stack_truncated=...
  write_pool{max_open open in_use idle wait_count wait_ms} read_pool{...}`,
  then one `store_stalled holder g=<id> [<state>] choir=<first choir frame>
  engine=<frame> < <frame>` line per holder. Rate limit: one capture per
  runtime per minute.
- Only engine goroutines with a choir frame or a choir creator are listed.
  The engine's own background goroutines (events sender, binlog streamer,
  branch activity tracker, async replication: about eight per open store,
  parked for its lifetime) are counted in `engine_goroutines` but not
  listed; in the first test run they filled the 16-line cap. Only a choir
  caller takes a connection from the pool, so this drops no possible holder.
- `store.(*Store).PoolStats` and `store_pools` on
  `/api/runtime/observability`: `wait_count` and `wait_duration_ms` since
  boot for both pools.

How to read it: `in_use=1` on the write pool with `shown=0` means the
connection is held by a caller that is not inside the engine at that moment
(a `*sql.Conn` or `*sql.Tx` held between statements, or rows not yet
closed); the existing stall test, whose holder is an idle `Conn`, logs
exactly that. A listed holder with its choir frame names the call.
Holders that hold the connection between statements are the instrument's
known blind spot; if the deployed log shows `shown=0` during the
unattributed bursts, the next instrument is a hold-time log on the store's
`Conn`/`BeginTx` callers.

Declined in this lane: the pool stats on /health's 200 body. That body's
test pins it to today's keys, and the lane's contract is that /health's
answers do not change; observability carries them instead.
