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

## Fix (lane fp3-wake-restart-orphan, 2026-10-22)

Mutation class of this section: green. The fix it records is red (vmctl
lifecycle). Commits, on top of 4da173c13 (the Cause section):

- 20163083e and fe9b2db5d (yellow): the failing tests.
- 0469522b4 (red): option A.

Nothing has shipped yet. FP0 lands, deploys and proves.

### What changed

All the code is in `internal/vmctl`. vmmanager, cmd/vmctl, routes, the
gateway and auth are unchanged.

- **The claim** (`boot_claim.go`, new). `VMOwnership.BootClaim`
  (`boot_claim` in ownerships.json) holds the kind (`boot`, `resume`, or
  `first boot`), the realization epoch and the time.
  - `spawnClaimed` (boot_claim.go:91) writes the claim to the ownership file
    before the manager is called. If the write fails, the start is refused
    before anything spawns. A failed spawn drops the claim, compared by
    pointer.
  - It wraps every spawn of an existing computer: BootVM, RecoverVM and
    RefreshVM in `startAdmittedVM`, `recoverOrRestartActiveVM`, recover,
    refresh and `rescueLaunch`; `ResumeVMFromSnapshot` in
    `resumeFromSnapshot`; and the platform computer's `ResumeVM`.
  - Every publish clears the claim in its own write. There are 11 sites:
    six in ownership.go and five in platform_computer.go.
  - A new computer's persisted `booting` row is its claim, read the same way.
- **Load** (`ownership.go:693`, `interruptStartAtLoadLocked`
  boot_claim.go:71). A row with an open claim, or a `booting` row, becomes
  `stopped` with `stopped_by: interrupted-by-vmctl-restart`. It joins the
  in-memory set of open closes; a booting row is first given a `first boot`
  claim.
  - A claim that outlived its publish is dropped, so it can never end a
    running computer. That is a claim on an active or degraded row at the
    claimed epoch or later, or on any row past it (`publishedIn`,
    boot_claim.go:56).
  - Everything else loads as before: running rows become `vmctl-restart`
    and are reattached.
- **Close** (`closeInterruptedStarts`, boot_claim.go:134).
  - `ReattachManagedVMs` runs it first (ownership.go:789), at startup and
    before vmctl serves. Every `ReconcileVMProcesses` pass retries it
    (host_capacity.go:97).
  - It powers off the surviving process with `ReapUnmanagedVM`. The
    manager's per-VM operation lock serializes that kill with any boot of
    the same VM, and the manager refuses to kill a VM it runs.
  - It then drops the claim, discards a memory snapshot the start may have
    run from, and saves.
  - A kill that fails keeps the close open and durable. The fate is saved,
    and the claim stays in the file for the next restart.
- **Until the close finishes**, the process is never reattached
  (`reattachOwnershipInfo`, ownership.go:836). The `startExistingVM` path
  therefore answers `guest_reattach_pending` with Retry-After. The process
  is never probed for a wedge, and reconcile counts it as pending
  (host_capacity.go:152). It still shows in health's `unmanaged`.
- **The fate is not a failure.**
  - The always-on sweep boots the computer without spending the failure
    retry.
  - A deploy rescue treats it like `vmctl-restart`: it is eligible, its
    retry is charged per target, and a hard failure makes it
    `recovery_failed` (`stoppedByRestart`).
  - The next owner act boots a fresh realization (a new epoch, a cold boot).
- **H3 is refuted, so admission is unchanged.**

### Tests (`internal/vmctl/interrupted_start_test.go`)

Each test runs two registries over one ownership file. The first blocks
mid-spawn. The second loads the file over a fake host whose process
answers only at the address its boot was given. It then runs reattach, one
reconcile, and eight one-minute reconcile passes (the incident's seven
minutes). A third registry reloads the file.

The table covers six start paths:

- a visit wake of a hibernated computer, as a cold boot (the incident);
- the same wake from its memory snapshot;
- a resume of a stopped computer;
- a deploy refresh of an active computer;
- a recovery of a failed computer;
- a new computer's first boot.

Each must be adopted at the address its boot was given, or closed with an
honest fate. It must never be wedged, and the fate must survive the reload.
The other tests are:

- an owner act after the close boots fresh and never adopts the orphan;
- a close whose kill fails stays honest for eight passes, and the next
  restart closes it;
- guards: a published start (visit, resume, refresh) is reattached by the
  next restart and not reaped; a computer hibernated after a published wake
  stays hibernated; a failed wake keeps its fate.

Before the fix, every table case failed:

- the hibernated, stopped, failed and snapshot rows were never touched;
- the refresh was adopted at the stale URL;
- the first boot was adopted at the placeholder `http://127.0.0.1:8085`.

The owner-act test failed because the later wake adopted the orphan at the
stale URL, and the failed-kill test failed at minute 0. The guards passed.
After the fix, all of them pass.

Runs on darwin, at 0469522b4:

- `go test -race ./internal/vmctl -count=1`: ok, with no skips.
- `go test -race ./internal/vmmanager -count=1`: ok. Four Linux-only tests
  skipped because they need `/proc`: `TestReattachVMAdoptsByIdentity`,
  `TestBootAndDestroyRefuseOverLiveUntrackedFirecracker`,
  `TestReattachDrainsALegacyConsolePipe` and
  `TestReattachLeavesAFileConsoleAlone`.
- `go test -race ./cmd/vmctl ./internal/proxy ./internal/platform`: ok.
- `go build ./...`: ok.

### Rollback

`git revert 0469522b4`. The old binary decodes `boot_claim` as an unknown
field and ignores it. A row the fix already closed reads
`stopped / interrupted-by-vmctl-restart`, and the old binary boots it fresh
on its next start: an owner act, or the always-on sweep. The old binary's
deploy rescue does not reach that fate; its sweep does.

### Staging proof plan (FP0, on a test computer)

1. Land and deploy. The deploy that ships the fix restarts an old vmctl,
   which wrote no claims (see the residual), so do not race a wake against
   that restart. Verify the build identity first.
2. Pick a test-class computer that is not always-on, hibernated or stopped
   by idle. Record its ownership row: state, stopped_by, epoch,
   computer_url.
3. Fire an owner wake (lifecycle start) and, while its boot is in flight,
   restart vmctl. Use the next deploy's restart, a redeploy of the same SHA,
   or whatever no-SSH restart FP0 has. The 07:10 window was the cold boot
   after a snapshot fallback, about 35 s. A wake that publishes before the
   restart exercises the guard instead; retry.
4. The caller sees 502 or a connection reset. That is expected: the start
   was interrupted.
5. Acceptance evidence:
   - the vmctl journal shows `vmctl: VM <vm> (user <owner>): its boot of
     epoch N was in flight at the vmctl restart; closing it`, then
     `vmctl: closed VM <vm> ... (powered off pids [...]);
     stopped_by=interrupted-by-vmctl-restart`;
   - for that VM, there is no `runs outside the registry ... reattach
     pending` line, no `wedged VM` line, and no destruction receipt with
     cause `wedged`; the receipt cause is `reap-unmanaged:orphan`;
   - the ownership row reads `stopped`, `stopped_by:
     interrupted-by-vmctl-restart`, with no `boot_claim`;
   - health's process reconcile lists no unmanaged entry for the VM.
6. A second owner wake returns 200 at a new epoch, above N, with a fresh
   address. The computer serves.
7. Negative control: a test computer that was active at the same restart is
   reattached as before (`vmctl-restart`, then reattached), not closed.
8. Repeated interruption (round 2, F1). Never the paper or any platform
   computer: use a test computer made always-on for the proof
   (critical_protected or premium_always_on primary), or a test computer
   driven by owner wakes. Interrupt its start once (steps 3 to 5): the row
   reads `stopped_by: interrupted-by-vmctl-restart`,
   `start_interruptions: 1`, no `failure_warmed_at`, and the next sweep (or
   wake) boots it without a `spent the automatic retry` line. Interrupt
   that re-boot too, before it publishes: the journal shows `closed VM <vm>
   ... 2 starts in a row; stopped_by=start-interrupted-repeatedly-by-vmctl-restart,
   the automatic retry is spent`, the row carries `failure_warmed_at`, and
   for the next ten minutes no sweep warms it (no `started VM <vm>` line).
   An owner wake then boots it, and the row's `start_interruptions` is gone
   (reset by the publish). If the always-on class cannot be set on a test
   computer without touching a real one, record step 8 as not run and say
   so; the unit test is the evidence.
9. Shield (round 2, F2), only if a reap failure can be produced on a test
   computer without SSH: while the close is open, an owner recover answers
   409 with `retry shortly` and the row keeps its fate. Otherwise not run.

### Heresy delta

- Discovered: none beyond the Cause section. That section already records
  the false "reattach pending" log for hibernated rows, and the new-computer
  reattach at its placeholder URL.
- Introduced: none.
- Repaired:
  - the lifecycle claim held only in memory (standing questions Q5 and Q8);
  - the `wedged` fate, and its charged retry, for an orphan vmctl itself
    made;
  - reattach at a stale or placeholder URL after a restart mid-start.

### Residuals

- The shipping deploy's own vmctl restart: a wake of an existing computer in
  flight at that one restart still orphans once, because the old binary
  wrote no claim. A new computer's first boot is already covered, since its
  persisted `booting` row now reads as a claim.
- H2 is unchanged: a boot without a fresh gateway token falls back to the
  previous boot's token file.
- ~~The platform computer's first-ever boot has no row before `BootVM`.~~
  Fixed in review round 2 (F3): it persists its booting row first.
- A refresh whose publish is refused because the row changed under it keeps
  its claim. A later restart closes that realization. This is deliberate:
  the start was never published.
- Each spawn adds one ownership-file write.
- The deploy rescue's deferral text for a live, still-closing process still
  names the wedge detector. This is cosmetic.
- The L1b boot ledger (`docs/restart-subsystem-design-2026-10-13.md`) is
  still the substrate this claim anticipates: a grant without a boot result.

## Review round 2 (bunch 37, 2026-10-22)

Reviewers: Gemini flash accepted; DeepSeek accepted with edits that gate the
deploy (~/runs/wakeorph-r1-deepseek.md on FP0's VM); Devin's findings were
pending at this round. Tests first in 14c53fa58 (yellow: F1, F2, F3 and F4
fail on 611223b30; the publishedIn guard and claim-write tests pass there,
as coverage), fix in 8a9ad074b (red, ceremony in its message).

### Dispositions

- **F1 (gate), repeated interrupted starts unbounded: fixed.** The row
  counts consecutive interrupted starts (`start_interruptions`, reset by
  every publish). The first close is free. The second is
  `start-interrupted-repeatedly-by-vmctl-restart`, a failure, and the close
  charges `failure_warmed_at`; the always-on sweep, the platform warm and
  the platform ensure leave it for 24 h or the next deploy's re-arm, and an
  owner act boots it. Test: `TestRepeatedInterruptedStartsAreBoundedLikeAFailure`.
- **F2 (gate), shield dropped before the replacement is proven: fixed.**
  `spawnClaimed` keeps the shield and refuses (a transient
  `guest_reattach_pending` refusal) while the close is open. The platform
  warm, the platform ensure and the direct recover (owner and maintenance)
  refuse before touching the row while the close is open or an untracked
  process lives, and a refusal restores the row. Also found and fixed: the
  warm `ResumeVM`'d a VM the manager no longer tracks (any VM after a vmctl
  restart) into `recovery_failed`; it now boots it as the ensure does.
  Test: DeepSeek's repro, `TestPlatformWarmAndRecoverRefuseWhileAnInterruptedStartIsOpen`.
- **F3 (medium), platform first boot unclaimed: fixed.** The first boot
  persists its booting row before `BootVM` (a failed first boot removes it,
  as before). The startup ensure retries transient refusals every 15 s
  inside its 10-minute timeout. Test:
  `TestPlatformFirstBootInterruptedByARestartIsClosedAndEnsuredAgain`.
- **F4 (medium), claims not start-scoped: fixed.** The platform warm takes
  the lifecycle claim and the ensure's pending slot and refuses while
  either is held; the ensure refuses while a lifecycle claim is held. A
  publish drops only a claim the published row records (epoch compare,
  `publishStartLocked`); vmmanager publishes the config's epoch, so the
  compare is exact. Test: `TestPlatformWarmAndOwnerStartNeverBothSpawn`.
- **Low, ResumeVM lock comment: fixed (comment), lock not taken.**
  `ResumeVM` acts only on a VM the manager tracks and the close's kill
  refuses those, so the kill and a `ResumeVM` never meet; changing
  vmmanager's lock order to take the per-VM lock under `m.mu` is a
  deadlock risk out of this lane's scope.
- **Low, unclaimed spawn without a persistence path: fixed (loud log),
  refusal declined.** Every in-memory registry (most tests, host-process
  mode) has no path; refusing would break them. Production sets
  `VMCTL_OWNERSHIP_PATH` or `VM_STATE_DIR`.
- **Low, missing tests: fixed.** `TestStaleClaimOnAPublishedRowIsDroppedNotClosed`
  (publishedIn on an active row), `TestStartIsRefusedWhenItsClaimCannotBeWritten`.
- **Low, `closeInterruptedStarts` reads UserID after unlock: declined.**
  UserID is immutable on a row; DeepSeek rates it style.
- **Pre-existing, `saveLocked` only logs write errors: residual, unchanged
  this round.** A publish whose write fails leaves the pre-publish claim on
  disk; the next crash then closes a computer that was serving.

### Residuals added this round

- The count is written by the close's save, not at load: a vmctl that
  dies again between load and close does not count that load. The claim is
  still on disk, so the next load closes it; the bound can lag by one.
- An old binary (rollback) reads `start-interrupted-repeatedly-by-vmctl-restart`
  as an unknown, non-failure stop, so its sweep would re-boot such a row.
- `saveLocked` (above).
