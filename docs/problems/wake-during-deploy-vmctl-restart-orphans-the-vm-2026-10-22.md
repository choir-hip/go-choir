# A wake that races a deploy's vmctl restart boots an orphan VM; the caller gets 502 and the computer ends stopped by wedged (2026-10-22)

Found 07:17 UTC by the self-development track monitor, during the freeze
close's deployed proof. Test computer only: computer-71afbf14… (vm-c5c801f8…,
class test, owner batching-e2e-1022). Mutation class of this record:
green. A fix would be red (vmctl lifecycle).

## Evidence (vmctl journal on Node B, read-only; the monitor's own act log)

- 07:09:42. Staging's `x-choir-build-commit` is 4fca1e01b (deploy G). The
  monitor treated that as "deployed" and woke the hibernated test computer.
- 07:10:15. `POST /api/computers/<id>/lifecycle/start` with
  `{"idempotency_key":"fp4-freeze-close-wake-20261022T0710"}`, test key.
  The answer was **502 `{"error":"lifecycle actuation failed"}`**. In the
  old vmctl (pid 4045150):
  - 07:10:15 `admission decision=admit … need_mib=1024 available_mib=10231`
  - 07:10:16 `snapshot resume … not used (disk_identity_mismatch)`; cold boot follows
  - 07:10:17 `gateway token request failed for vm-c5c801f8…: … 127.0.0.1:8084: connect: connection refused` (the gateway was restarting for the deploy)
  - 07:10:17 `cold boot of VM vm-c5c801f8… after snapshot fallback`
- 07:10:50 to 07:11:00. systemd stops and starts go-choir-vmctl (deploy G
  activation). It finds left-over firecracker 4099850 in the cgroup and
  ignores it.
- 07:11:01. The new vmctl (pid 4100215): `route-authorized reattach adopted
  4 managed VM(s)`. Those are the delegate's, the paper's, 7cc96dc6's and
  the owner's. **Not vm-c5c801f8.** The test computer's ownership row
  still read `state: hibernated, stopped_by: idle`.
- 07:11:04, 07:13:15, 07:15:15. `VM vm-c5c801f8… runs outside the registry
  for 0s / 2m11s / 4m11s; reattach pending`.
- 07:17:15. `killing orphaned Firecracker process … (pid=4099850)`,
  `destructive kill … cause=reap-unmanaged:orphan epoch=0`, `wedged VM …
  refused connections for 5m0s; stopped, it boots again on resume`. The
  row is now `state: stopped, stopped_by: wedged`.

## What it shows (hypotheses until read in code)

- H1. The boot claim was in the old vmctl's memory only. The row never
  left `hibernated`, and the process ran with `epoch=0`. So the restart
  had nothing durable to reattach. Standing question 8: what a restart
  erases.
- H2. The wake booted with no gateway token, because the gateway was down
  for the same deploy. That is the shape Argon raised for FP3's rescue
  ("RecoverVM with the old boot config and no gateway token"). Not checked
  here: the guest never answered.
- H3. The orphan held host memory for about 7 minutes (07:10:17 to
  07:17:15) while admission counted it nowhere.
- The caller's 502 is honest, but the computer then ends `stopped by
  wedged`, a fate the owner did not cause. Per FP3 group 1 that needs an
  owner act or a new deploy target to boot again.

## The monitor's own error

`x-choir-build-commit` turns over when the web tier restarts. That is
before vmctl and the gateway restart, and before CI's refresh round
(07:11:02 to 07:12:43 here). A wake between those points races the
deploy. From now on the monitor waits for CI's `deploy refresh
caller=deploy` round before any lifecycle act after a deploy.

## Next safe probe

Wake again with an owner act now that vmctl is stable. That is the
designed exit from `stopped_by: wedged`. Then read the boot and build
identity.

## Cause (read in code, 2026-10-22, lane fp3-wake-restart-orphan at 7e87dd3d3)

Read before any change. Mutation class of this section: green.

### The path the wake took

`POST …/lifecycle/start` reaches vmctl's resolve
(`internal/proxy/computer_lifecycle.go:144-147`). For an existing
hibernated row, `resolveDesktopContext` takes the existing-start branch
(`internal/vmctl/ownership.go:2084-2116`): it sets `r.pendingWaiters[key]`
and `r.lifecycle[key] = lifecycleVisit` (2109-2110), unlocks and calls
`startExistingVM` (2116). Admission runs there (1470-1482, the
`decision=admit` line). `startAdmittedVM` (1491-1521) tries the snapshot
(`resumeFromSnapshot` 1552-1574; the manager logs "not used
(disk_identity_mismatch)" at `internal/vmmanager/snapshot_lifecycle.go:346`
and vmctl "cold boot follows" at `ownership.go:1566`), then reserves an epoch
(`reserveFreshVMConfig` 1394-1427), asks the gateway for a token (1511, the
"gateway token request failed" line at 1784) and calls `mgr.BootVM` (1515).
`bootVM` allocates a **new** host subnet (`internal/vmmanager/manager.go:848`,
`allocateHostPortLocked` 563-578 always takes `nextPort++`), logs "cold boot
… after snapshot fallback" (862 → `snapshot_lifecycle.go:641`), spawns
Firecracker (1020; the PID file is written at 2254) and waits for the guest
(1042). The row is published only when that returns (`ownership.go:2143-2152`:
`ComputerURL`, `Epoch`, `State=active`).

The 502: vmctl's graceful shutdown waits 10 s for in-flight handlers
(`internal/server/server.go:104`), which matches the stop at 07:10:50 and the
start at 07:11:00. The proxy maps the dropped call to "lifecycle actuation
failed" (`computer_lifecycle.go:175-181`). Honest.

### Verdicts

**H1. Confirmed.** Nothing durable records a start of an existing computer
while it is in flight. The branch above writes no row before the boot; the only
file write during the boot is the global epoch counter
(`ownership.go:1407-1413`), not the row's state, epoch or URL. The same is
true of every other path that boots an existing computer: resume
(`ownership.go:3023`), recover (3125), refresh (3241), the deploy rescue
(3695) and a visit's readiness check (2020): each takes only `r.lifecycle`,
which the struct comment calls the one lifecycle claim (551-559). It lives in
process memory, and a restart erases it (standing question 8).

On restart, `loadLocked` turns only `booting`, `active`, `degraded` and
`stopping` rows into `stopped / vmctl-restart` (679-684); a hibernated row
stays hibernated. `ReattachManagedVMs` adopts only `stopped / vmctl-restart`
rows (777). So the new vmctl had nothing to reattach: the "adopted 4" line.

The `epoch=0` in the kill receipt is not the process's epoch:
`cleanupOrphanedFirecrackerLocked` writes the receipt without one
(`manager.go:2371`). The orphan ran at the reserved epoch, one past the row's.

*The deploy rescue's "claim before booting".* 05555afca, 211c56018 and
a80e37fd9 added a claim, but it is the in-memory one: `r.starting` in
a80e37fd9 (its rollback note says "r.starting … in-memory"), folded since into
`r.lifecycle` (`claimRescue` 3675-3697). It does not survive a restart, so it
cannot be reused as a durable claim. Three durable pre-boot records do exist:

1. the new-computer path persists `state: booting` before its boot
   (`ownership.go:2204-2229`), and so does the platform computer
   (`platform_computer.go:155-156`, 294-295);
2. the always-on retry is spent durably before the boot, "so a crash between
   the record and the boot loses the retry rather than repeating it"
   (`ownership.go:3561-3567`);
3. the manager's per-VM epoch file is reserved before the boot
   (`manager.go:763-785`).

None of them marks the ordinary wake. The first is a durable claim, but it is
read wrongly: `loadLocked` turns `booting` into `vmctl-restart`, and the
reattach then adopts the process at the row's `ComputerURL`
(`ownership.go:813`). For an unpublished boot that is never the process's
address. A new computer's row holds the placeholder
`VMCTL_AUTOPUTER_URL_BASE` (2210, 3982-3987). Any boot gets a fresh subnet
(`manager.go:848`), and `ReattachVMWithConfig` trusts the URL it is given and
adopts by PID identity even when the probe fails (`manager.go:1348-1354`,
1384-1398). This is the same defect class, reached from the new-computer path
and from a refresh of an active computer.

**H2. Confirmed, with a nuance; not causal.** `issueGatewayTokenAt` returns ""
on a transport error (`ownership.go:1782-1786`), and its caller treats that as
not fatal by design (1638-1642). The empty token goes to `BootVM` (1511-1515;
on the tracked path through `trackedBootConfig` 1531-1539). `bootVM` writes a
token only when it is non-empty (`manager.go:890-895`). Otherwise it falls back
to the previous boot's `persist/gateway-token` (`resolveGatewayToken` 691-707,
called at 896). So the wake got no fresh token. It booted with the previous
boot's token if that file existed. The orphan, the 502 and the wedged stop
happen the same way with or without a token. This is a separate residual:
during a deploy, a boot can start without a fresh credential. The rescue's
"freshly issued gateway token" (a80e37fd9 A3) does not fail closed either.

**H3. Refuted as stated.** Admission requires `MemAvailable >= floor +
headroom + reserved + need` (`internal/vmctl/host_admission.go:418`), with
`MemAvailable` sampled from the host (394). The orphan's resident memory was
inside that reading the whole time, so admission did count what the orphan
used. `sampleVMMemory` measured it too: `VMResidentBytes` covers every
Firecracker process (`internal/vmmanager/process_control.go:127-131`), and the
peak was recorded on the row (`host_admission.go:549-560`). What the restart
erased is the start's reservation: the ledger is in memory (131-143; the
reservation is created at 431-438). So the part of `need` the booting guest
had not yet touched (at most 1024 MiB here) was not reserved. No admission
change is needed if the orphan does not outlive the restart; the fix below
ends it at vmctl start.

**H4. Confirmed, and the evidence was about the wrong address.** For a
hibernated row with a live untracked process, `ReconcileVMProcesses` skips the
reattach branch (`internal/vmctl/host_capacity.go:156`, `vmctl-restart`
only). It probes `own.ComputerURL` (162). That is the hibernated realization's
address, not the cold boot's new subnet (`manager.go:848`, 994-995). Its log
line says "reattach pending" (179), but no path ever reattaches a hibernated
row. After 5 minutes of refusals, `stopWedgedUnmanaged` (`wedge.go:80-88`)
reaps the process (`ReapUnmanagedVM` → `cleanupOrphanedFirecrackerLocked`
with cause `reap-unmanaged`, `process_control.go:94`, `manager.go:2364-2380`).
Then `markWedgedStopped` writes `stopped / wedged` (`wedge.go:101-112`). So
vmctl recorded its own orphan, probed at a dead address, as the computer's
failure. And `wedged` counts as a failure stop (`ownership.go:3527-3529`): on
a protected computer it spends the always-on retry for 24 hours.

### Classification

Substrate: VM lifecycle restart durability. The lifecycle claim is the only
record of a start in flight, and it is process memory (standing question 8).
For the length of the boot, the single authority for lifecycle state, the
ownership row, disagrees with the host: the row says hibernated while a
realization runs (standing question 5). The wedge verdict, the stale URL, the
"reattach pending" line and the lost reservation are symptoms of that one
gap.

Existing replacement: the restart design's L1b boot ledger
(`docs/restart-subsystem-design-2026-10-13.md` §1, §3.1: "every Firecracker
spawn requires a grant recorded durably … before the spawn") is the designed
substrate and is not built. A fix here must be a compatible precursor (a
durable record before the spawn), not a competing authority.

### Fix options

- **A. A durable start claim on the ownership row** (recommended). It is
  written immediately before every manager spawn of an existing computer.
  The write that publishes the start clears it, and so does a failed spawn.
  At load, a claim, or a persisted `booting` row (the new-computer path's
  existing claim, read the same way), means a start the restart interrupted.
  That start is never reattached. The new vmctl powers off any process it
  left, before it serves a request, and the row reads
  `stopped / interrupted-by-vmctl-restart`. That fate is not a failure: the
  next owner act boots the computer fresh, and so does the always-on standing
  intent for protected computers. It is one field on the existing authority,
  a write before each spawn, and a read at load. It reuses the first-boot
  path's meaning without overloading `state`, and the boundaries of the
  existing lifecycle claim.
- **B. As A, but adopt the process when it is up.** Adoption needs the
  realization's real address and epoch. Only the old vmctl knew them, so the
  manager would need a durable launch record, or vmctl would have to parse
  `fc-config.json` boot args. It also finishes the old process's half-done
  wake. Rejected.
- **C. Persist `state: booting` before the wake's boot**, as the first-boot
  path does. This changes what every reader of `state` sees during a wake:
  resume returns early for `booting` (`ownership.go:2987`), refresh accepts it
  (3219), the rescue defers (3686), resolve treats a booting row as a stale
  boot to recover (2108), and lookups change. Every failure path would also
  need a restore. A has the same effect with a narrower diff.
- **D. Drain in-flight starts on SIGTERM.** The drain is bounded by the 10 s
  shutdown. A boot that replays can take minutes, and a crash cannot drain at
  all. Optional later; not a fix.
- **E. Symptom patches**: reattach hibernated rows that have a live process,
  probe the process's real address, or lengthen the wedge window. Each leaves
  the claim in memory. Rejected (Convergence Before Patching).
- **F. Build L1b now.** It is the right long-term answer and too large for
  this lane. A is shaped so that L1b can subsume it: a claim without a publish
  is a grant without a boot result.

### Why close, not adopt (the restart rule)

CLAUDE.md: "A crash restart never resumes work: at boot, every open obligation
(run, wake, …) is closed with the visible fate 'interrupted by a restart',
never re-dispatched." A start that has not been published is such an
obligation of the vmctl that died. vmctl's restart writes no planned-restart
marker for its own in-flight starts, so the new vmctl has crash semantics.

1. Adoption would complete the old process's wake. Closing settles it with a
   visible fate. The caller was already told 502, so the closed start matches
   what the caller saw.
2. The reattach cannot adopt correctly without new machinery: the row's URL
   is not the process's (H1, H4).
3. A wake that raced a deploy may have booted without a fresh credential
   (H2). The next start boots on a settled host.
4. The cost is one wasted cold boot of a computer that was not yet serving.
   Nothing committed is lost: a cold boot is a crash boot, and the guest's
   replay is crash-safe by design (`docs/computer-ontology.md`: a realization
   is replaceable).
5. A standing intent survives where the occurrence does not. An always-on
   computer's next sweep boots it again on the new vmctl (a fate that is not a
   failure spends no retry), and so does the platform computer's ensure at
   start.

Residual: the deploy that ships the fix restarts an old vmctl that wrote no
claims. A wake in flight at that one restart can still orphan once, as on
07:10.
