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
