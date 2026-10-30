# A failure-stopped always-on computer waits 24 hours, and a rollback deploy cannot reach it

Date: 2026-10-21. Status: **fix built** on lane/host-vmctl-b4-b5 (FP3 group
1), not yet landed or proved on staging. Class: red (vmctl, VM lifecycle, deploy routing). Owner rule:
"we need paper publishing 24/7, even after crashes."

This is the vmctl half of the 50-minute Autopaper outage in
[stored-column-add-blocks-paper-boot-2026-10-21.md](../archive/problems/stored-column-add-blocks-paper-boot-2026-10-21.md)
("What followed", and its "Fix order" item 4), and the second occurrence of
item 2 of
[paper-computer-disk-full-outage-2026-10-15.md](../archive/problems/paper-computer-disk-full-outage-2026-10-15.md).
Read those for the timeline. This record states the two defects, pins the code
that causes each, and writes the failure modes before any test.

## Defect A: one automatic retry per 24 hours

`OwnershipRegistry.alwaysOnWarmTargets` and `WarmAlwaysOnDesktops`
(`internal/vmctl/ownership.go`, `failureWarmWindow = 24 * time.Hour`) let the
always-on sweep boot a computer stopped with `stopped_by` `wedged` or
`recovery_failed` once per `failureWarmWindow`, counted from
`VMOwnership.FailureWarmedAt`. The comment calls it an "interim bound until
the boot ledger (restart station L1b)". It has now held the paper down twice:

- 2026-10-15 23:29: wedge stop, retry spent, next allowed 24 h later; the
  lead resumed it by hand at 00:06.
- 2026-10-21 16:44:54: wedge stop on the bad release, retry spent at the
  boot of 16:45:01 into the same bad release; the wedge stop of 16:59:09 had no
  retry left, so the paper stayed down until the session called
  `/internal/vmctl/resolve` at 17:09.

The bound does what it was built for (no crash loop), but its only inputs are
the clock and the owner. The restart design says the allowance is **per
external event**: "an external event (owner act ... a deploy refresh carrying a
new build ...) opens a new episode with a fresh allowance"
([restart-subsystem-design-2026-10-13.md](../restart-subsystem-design-2026-10-13.md) §1).
A deploy of a different build is such an event. Nothing in the retry decision
reads it.

## Defect B: a deploy cannot reach an unhealthy protected computer

`selectDeployRefresh` (`internal/vmctl/deploy_refresh.go`) lists only
`State == active` computers, and `deployRefreshWith` answers `skipped: not
active` for any other state ("it boots the current image when it next
starts"). CI then compares each candidate's `/health` commit with the target;
an unreachable `/health` reads as empty, the script substitutes the target, and
prints "already serves ...; nothing to refresh" (`.github/workflows/ci.yml`,
the `refresh_expected_commit` shortcut). So the computers a rollback exists for
are the ones the deploy skips:

1. **Failure-stopped** (`stopped_by` `wedged` or `recovery_failed`): not
   active, so not listed; its image is replaced but nothing boots it (defect A).
2. **Outside the registry, reattach pending**
   (`internal/vmctl/host_capacity.go`, `ReconcileVMProcesses`): a live process
   behind a stopped ownership. Reattach needs `State == stopped` and
   `StoppedBy == "vmctl-restart"` and a route-authorized caller; any other
   shape leaves the process running and logs `protected VM ... runs outside the
   registry; reattach pending`. The wedge check (`guestWedged`) runs before the
   protection `continue`, so the **only** thing that ends such a guest is the
   wedge detector: 5 minutes of refused connections, counted from the first
   probe of the *current* vmctl process (`networkDeadSince` is in memory). Every
   deploy that restarts vmctl restarts that clock; the rollback at 16:53 waited
   until 16:59:09.
3. **Tracked, active, but refusing connections**: the existing deploy path asks
   the guest for a planned-restart marker, gets no answer, defers, and the
   retry sweep asks again every minute until the wedge detector stops it, and
   then it is case 1.

## What the fix must exclude (failure modes, written before the tests)

Defect A:

- A1. A deploy with a build the retry has not been spent for does not re-arm a
  failure-stopped computer, so the rollback still waits for the clock.
- A2. The same deploy target posted again (a CI re-run, the retry sweep, a
  duplicate request) re-arms the retry each time: a crash loop under one build.
- A3. A deploy re-arms an ordinary (non-protected) computer, or a held one.
- A4. The time bound is removed or loosened: the always-on sweep with no
  deploy still gets exactly one retry per 24 hours.
- A5. A transient refusal (host memory pressure, reattach pending) that spawned
  nothing burns the one retry.
- A6. Many distinct deploy targets in a row are unbounded spawns: it is one
  automatic boot per distinct target per computer, and each needs a fresh
  failure stop in between (a deploy never boots a healthy computer into a
  second boot).

Defect B:

- B1. A deploy kills a live guest that is working. A guest that accepts TCP
  connections is never stopped by this path; only the wedge detector's evidence
  (refused connections for the whole window) ends a live process.
- B2. A deploy boots over a live untracked process (two guests on one data
  disk). The rescue refuses while a process lives, tries the adoption a vmctl
  restart would have made, and otherwise waits for the detector.
- B3. A deploy of an unrelated computer class (a free user's stopped computer)
  boots a computer nobody keeps on.
- B4. The retry sweep drops the rescue after one pass (the guest still lives),
  so the computer is stopped by the detector minutes later and nobody boots it.
- B5. CI treats an unreachable `/health` as "already current" and never posts.
- B6. A rescue that fails to boot is retried every minute (a kill loop); it is
  one spawn per target.
- B7. A rescue boot is read as a planned restart: it has no marker, so the
  guest closes open work as `interrupted by a restart` (CLAUDE.md, restarts end
  work). It must not write one.

## Fix

1. **Re-arm per deploy target.** `VMOwnership.DeployGrantTarget` records the
   last deploy target that re-armed the computer. A POST to
   `/internal/vmctl/deploy-refresh` for a protected, unheld computer calls
   `GrantDeployRetry(vmID, target)`: if the target differs it records it and
   clears `FailureWarmedAt`. The retry is still spent by `recordFailureWarm`.
   Result: at most one automatic boot per (computer, deploy target) after a
   failure stop, plus the unchanged 24-hour backstop. A transient admission
   refusal refunds the retry (nothing spawned).
2. **Reach protected computers that are not active.** The deploy selection
   lists protected, unheld computers stopped by `wedged`, `recovery_failed` or
   `vmctl-restart` as candidates with `rescue: true`. For them `deployRefreshWith`
   grants the retry, then: a live untracked process is adopted if it will
   reattach, otherwise the answer is `deferred` (the wedge detector remains the
   only thing that ends a live guest, and the pending sweep keeps asking);
   with no live process it spends the retry and resumes the computer (a crash
   boot, `planned_restart=false`). The retry sweep applies the same path to
   pending rescues.
3. **CI.** The candidate list carries the `rescue` flag; the script skips the
   "already serves" shortcut for a rescue candidate and waits for the target
   after a rescue that returns `refreshed`.

## As built (2026-10-21)

- `internal/vmctl/ownership.go`: `VMOwnership.DeployGrantTargets` (the last 8 consumed targets, see "Review round 1");
  `GrantDeployRetry`; `spendFailureWarm`/`refundFailureWarm`/`transientStartRefusal`
  (the always-on sweep also refunds a retry on a refusal that spawned nothing);
  `alwaysOnBootClass`.
- `internal/vmctl/deploy_refresh.go`: `Rescue` on candidates;
  `deployRescueStopped`/`deployRescueEligible`; `deployRescue`; the grant at the
  top of `deployRefreshWith`.
- `internal/vmctl/deploy_refresh_retry.go`: the sweep keeps a pending rescue.
- `.github/workflows/ci.yml`: the `rescue` field leads each row (always
  non-empty, so `read` cannot shift it), no "already serves" shortcut for a
  rescue, no forced operator refresh of one, and a rescue-specific wait (the
  target commit, or any answer from `/health`, which a retained app-layer
  release would give).
- Tests (`internal/vmctl/deploy_rescue_test.go`): grant once per target and for
  protected unheld computers only (A1-A3); the sweep alone stays at one per
  window (A4); a refusal that spawned nothing refunds (A5); one spawn per
  target under many posts, sweeps and warms, with the 24-hour backstop
  intact (A6, B6); the rescue never boots over or stops a live guest, keeps the
  pending entry, boots once the detector ends the guest (B1, B2, B4); an
  ordinary, idle-stopped or held computer is left alone (B3); no planned
  marker is requested (B7). Mutation checks: dropping the same-target guard,
  the protected-class guard, or the live-guest guard each fails a named test.
  The live-guest case is also guarded one layer down by `startExistingVM`.

## Review round 1 (2026-10-21, independent panel): four blocking findings, fixed

- **B1.** `DeployGrantTarget` remembered only the last target, so A, B, A
  re-armed A; and the store compaction (`compactStore` calls
  `deployRefreshWith` with Target `store-compaction`) granted and rescued like a
  deploy. Fix: `DeployGrantTargets` remembers the last `deployGrantTargetsKept`
  (8) consumed targets, and the grant and the rescue run only for a real deploy
  request (`maintenance == nil`). Tests: `TestDeployGrantRemembersEveryConsumedTargetNotOnlyTheLast`,
  `TestStoreCompactionNeverGrantsOrRescues`.
- **B2.** A refund restored the value it replaced even after a newer grant and
  spend. Fix: spends return a receipt (previous value, a per-computer
  generation); a refund is a compare-and-restore on the generation. Test:
  `TestStaleRefundCannotEraseANewerGrantOrSpend` (the interleaving is driven
  explicitly: sweep spend, deploy grant and spend, late refund).
- **B3.** A rescue of a computer stopped by `vmctl-restart` spent nothing. Fix:
  `spendDeployRetry` charges every rescue that spawns a process, whatever the
  stop reason; adoption of a live guest spawns nothing and stays free; a hard
  boot failure of a `vmctl-restart` stop becomes `recovery_failed`, so the
  always-on sweep's bound applies to it instead of re-trying every pass. Tests:
  `TestRescueFromVMCTLRestartIsChargedAgainstTheTarget`,
  `TestAdoptionWithoutASpawnSpendsNothing`.
- **B4.** In `ci.yml` an unreachable `/health` of an active computer still read
  as "already serves; nothing to refresh". Now the shortcut needs `/health` to
  have answered with the target commit; on no answer the script posts to vmctl.
  Checked with a stubbed `curl` over the four cases (no answer, current, old
  commit, rescue).

Mutation checks: reverting each of the four fixes fails the test named for it.

## Review round 2 (2026-10-21, second reviewer): findings A1-A7

- **A1 (fixed).** A spend succeeded for a computer that was no longer stopped
  by a failure, so a caller on a stale snapshot could write a spend that its
  refund then erased. `spendRetry` now judges the registry's entry under the
  lock: `State == stopped` and a failure stop (the sweep) or a failure or
  vmctl-restart stop (the rescue); anything else refuses and writes nothing.
  Test: `TestSpendRefusesUnlessTheRegistryEntryIsStoppedByAFailureNow`
  (stale wedged label on an active, booting, hibernated and failed entry).
- **A2 (fixed).** Two races. (a) The warm sweep's `ResumeVMForDesktop` runs
  `startExistingVM` outside `r.mu` for as long as the manager takes, with the
  registry still saying stopped; a rescue then went to `RecoverVM`, which
  waits for the first boot, kills the healthy guest without a marker and boots
  it again. Resume and recovery now count themselves in `r.starting[key]`
  under the lock before they unlock. (b) A rescue acting on the deploy's
  snapshot of a computer that had reattached resumed the already-active VM
  unchanged and answered "refreshed" for the old image. `deployRescue` now
  calls `claimRescue`, which re-reads the registry under the lock: active
  returns not-done (the ordinary planned refresh), booting, a refresh in
  flight or a start in flight returns deferred, and a rescue takes the
  `refreshing[key]` claim before any boot, so the sweep, a visit's recovery
  and a deploy refresh see "in progress" (that refusal is transient and
  refunds a retry the sweep had spent). Tests:
  `TestRescueDefersWhileAWarmSweepBootIsInFlight` (the sweep is held inside
  `BootVM`; the rescue must answer deferred and spawn nothing),
  `TestRescueOnAStaleSnapshotOfAReattachedComputerTakesThePlannedRefresh`.
- **A3 (fixed).** After the wedge detector's `StopVM` the manager still tracks
  the stopped instance, and the rescue went `startAdmittedVM` to `RecoverVM`,
  which keeps `inst.Config` (the old kernel, initrd, store disk and kernel
  parameters, the image that hung) and passed no gateway token. The rescue
  now boots through `rescueBoot`: a tracked instance is refreshed with
  `RefreshVM` (`refreshBootConfig`: the current deploy's artifacts), an
  untracked one is booted with `BootVM`, both with a freshly issued gateway
  token and the same admission gates as any start. The test fixture's
  `failStop` now keeps the stopped instance as `StopVM` does; `failStopUntracked`
  models the stop after a vmctl restart. Test:
  `TestRescueOfATrackedStoppedInstanceBootsTheCurrentDeployWithAFreshToken`.
- **A4 (fixed, CI).** The rescue verification overwrote the observed commit on
  every probe, so a non-layered computer still on the old commit passed after
  120 probes, and a layered one slept the full 120 seconds. Each probe now
  reads `.build.commit` and `.effective_release.layered` and breaks on the
  target commit, or on any commit with `layered == true`; a loop that ends
  without a break fails the deploy. Exercised with a stubbed `curl` over six
  cases (target, layered other commit, non-layered old commit, no layered
  field, no answer, layered without a commit). This retires the earlier
  residual "the rescue's final /health accepts any commit".
- **A5 (residual; fixed in round 3, R3-P1c).** A protected computer in `VMStateFailed` (a failed deploy
  refresh sets `State=failed, StoppedBy=""`; the platform warmer sets
  `recovery_failed` with `State=failed`; `loadLocked` keeps it) is skipped by
  `selectDeployRefresh`, `deployRescueStopped`, `alwaysOnWarmTargets` and
  `ResumeVMForDesktop`. The paper was not in that state in the 2026-10-21
  outage as the timeline records it: active (the 16:20 refresh answered, the
  guest hung), stopped by wedge (16:44:54), active (16:45:01 warm), stopped by
  the vmctl restart or outside the registry (16:53), stopped by wedge
  (16:59:09), active by resolve (17:09). I did not read the journal; the
  timeline is the orchestrator's. Covering `failed` is a named residual: a
  failed boot after a deploy refresh leaves an always-on computer down until a
  visit or an operator refresh.
- **A6 (fixed, CI).** Every field of the candidate and skipped rows now has a
  non-empty placeholder (`-`), restored to empty after the `read`, so an empty
  `snapshot_kind` or `computer_url` cannot shift the later columns.
- **A7 (recorded, benign for the bound).** The grant clears `FailureWarmedAt`
  before the rescue knows whether it can proceed (a live unmanaged guest
  defers it). The next warm sweep can then spend that re-armed retry, if the
  wedge detector has ended the guest by then. That is still one spawn for the
  target: the spend is charged to the same allowance, a start the manager
  refuses refunds it, and the rescue for the same target then finds it spent.

## Review round 3 (2026-10-21, GPT-6.1-sol on bcdb74659): sent back; one cause

Four findings, each a source-level interleaving (not reproduced on Firecracker):

- **R3-P1a.** A visit's resolve of a stopped computer marks only
  `pendingWaiters[key]` and boots outside the lock; `claimRescue` reads
  `refreshing` and `starting`, not `pendingWaiters`, so a rescue can
  `RefreshVM` the visit's first boot (a markerless kill and reboot). And
  `ResumeVMForDesktop` calls `endStart` before it publishes `State=active`,
  so the same window reopens between the boot's return and the publish.
- **R3-P1b.** Spending and starting are separate decisions. (1) The warm
  sweep snapshots a `vmctl-restart` stop as `afterFailure=false`, waits in
  its route guard; a rescue spends, fails hard and relabels the stop
  `recovery_failed`; the sweep resumes on its stale flag and boots
  uncharged past an exhausted allowance. (2) While a rescue holds
  `refreshing`, the sweep can still spend (`spendRetry` ignores claims); the
  rescue then answers "skipped: spent", the pending rescue and its identity
  check are lost, and the sweep later boots the old config through
  `RecoverVM` or refunds.
- **R3-P1c (A5 rejected as a residual).** A deploy refresh that fails to
  boot sets `State=failed` itself; a corrective deploy neither lists nor
  rescues a failed protected computer and the warmer skips it, so the paper
  can stay down until a visit or an operator act. The deploy path produces
  this state, so the outage timeline not passing through it is no excuse.
- **R3-P2.** `DeployGrantTargets` keeps the last 8 targets. Inside one retry
  window, consume A..I with failure stops between, post A again: A was
  evicted and re-arms. Eight strings are not a once-per-consumed-target
  ledger.

**Common cause (Convergence Before Patching; this is the third round on
this group, and rounds 1 and 2 each closed one interleaving and opened the
next).** One computer's lifecycle is guarded by three separate in-registry
mechanisms, each added by a different change: `refreshing` (refresh,
hibernate, rescue), `starting` (resume, recover; added in round 2),
`pendingWaiters` (a visit's resolve, the platform warmer). No mechanism is
read by every start path, and the release points differ (`endStart` before
the publish, `defer` after it). The retry allowance is spent in a fourth
locked step that reads none of them. So every pair of paths that do not
share a mechanism is an interleaving, and patching pairs does not converge.
The bound on consumed targets has the same shape: a cap chosen for memory,
not for the window the safety argument needs.

**Substrate fix (this round).** One lifecycle claim per computer
(`r.lifecycle[key]`), taken under `r.mu` by every path that boots an
existing computer — a visit's resolve (stopped and readiness paths), resume,
recover, the always-on warm sweep, the retry sweep and the deploy refresh
(through refresh or rescue) — and by hibernate, and released in the same
critical section that publishes the resulting state (or on the failure
path). The always-on sweep re-reads eligibility, charges the allowance and
takes the claim in one locked step; the rescue takes the claim with its
eligibility re-read and charges only while holding it, and a spend that
does not hold the claim refuses. `refreshing` and `starting` are deleted.
Failed protected computers are rescue candidates with the same gates
(holds, typed recovery admission, live-process protection, per-target
charging). The consumed-target memory keeps every target granted within the
retry window and refuses a new grant beyond a cap rather than evicting one.
`pendingWaiters` stays as the waiter list for collapsed resolves; it is no
longer an exclusion mechanism on its own. The handler's deploy-refresh
`inFlight` set stays: it guards the guest marker conversation (a
request-level dedup before any claim), not a boot.

### As built (round 3)

- `internal/vmctl/ownership.go`: `OwnershipRegistry.lifecycle`
  (`map[key]lifecycleHolder`) replaces `refreshing` and `starting`;
  `lifecycleConflictLocked`, `releaseLifecycle[Locked]`,
  `lifecycleRebootingLocked` (lookups report booting under any claim but a
  visit's). A visit's resolve takes the `visit start` claim next to
  `pendingWaiters[key]` on both existing-computer branches (stopped and
  readiness) and releases it at every exit, in the section that publishes;
  another visit still joins through `pendingWaiters`, any other holder
  refuses. `resumeVMForDesktop(…, alwaysOn)`: the sweep's eligibility
  (`alwaysOnWarmEligibleLocked`), its charge of a failure stop
  (`spendRetryLocked`) and the claim are one locked step; the refund of a
  transient refusal and the release happen under the lock after the boot;
  the release is in the publish section (the round-2 `endStart` before the
  publish is gone). Recover and refresh take the claim (released after the
  publish by `defer`), hibernate takes it. `spendDeployRetry` charges only
  while the caller holds the `deploy rescue` claim; a standalone
  `spendFailureWarm` refuses while anyone holds it. `DeployGrants`
  (target, granted_at) replaces `DeployGrantTargets`: entries are kept for
  `failureWarmWindow`, and a 65th target inside one window is refused
  (`deployGrantsCap`), never by evicting an older one.
- `internal/vmctl/deploy_refresh.go`: `deployRescueStopped` includes
  `VMStateFailed` (any stop label) for an always-on class; `rescueCause`;
  a rescue defers when the manager still tracks the instance in
  `running`, `pending` or an unreported state (`managedGuestMayLive`, the
  same liveness test vmmanager uses); a deploy refresh refused by another
  path's claim answers `deferred` (kept pending) instead of `failed`.
  Holds, typed recovery admission (`admitRecoveryStart` in `rescueLaunch`)
  and the per-target charge apply to failed computers unchanged.
- `.github/workflows/ci.yml`: unchanged. CI keys on the `rescue` flag, not
  the state, so a failed candidate takes the same path. Its comment at the
  rescue step still says a candidate "is stopped"; it should read "is
  stopped (…) or failed" (the lane's push token cannot update workflow
  files; FP0 may fold it in).
- Tests (`internal/vmctl/deploy_rescue_test.go`), each failing on
  bcdb74659 for its stated reason before the fix:
  `TestRescueDefersWhileAVisitBootsTheStoppedComputer` (R3-P1a),
  `TestDeployRefreshDefersWhileAVisitBootsAnActiveComputer` (R3-P1a,
  ordinary refresh vs a visit's readiness boot),
  `TestSweepOnAStaleSnapshotCannotBootPastAnAllowanceARescueSpent`
  (R3-P1b.1, the sweep held in its route guard),
  `TestNoSpendWhileARescueHoldsTheComputer` (R3-P1b.2, the rescue held
  between its claim and its charge),
  `TestDeployRescuesAProtectedComputerItsOwnFailedRefreshLeftFailed` and
  `TestDeployNeverBootsOverALiveTrackedGuestOfAFailedComputer` (R3-P1c),
  `TestConsumedTargetIsRememberedForTheWholeRetryWindow` (R3-P2). Changed
  on purpose: `TestSpendRefusesUnless…` (a rescue spend needs the claim; a
  failed computer is now the rescue's to charge), `TestDeployGrantRemembers…`
  (cap and window instead of eviction). The mock's `BootVM` now locks its
  record (two boots can race in these tests).
- Mutation checks: removing each of the nine pieces (visit claim on the
  stopped branch, on the readiness branch, the sweep's locked re-read and
  charge, the claim check of a standalone spend, the claim requirement of a
  rescue spend, failed as a rescue, the live tracked guard, the window
  memory, refresh-busy as deferred) fails the test named for it. **Not
  covered by a deterministic test:** the resume releasing its claim before
  the publish (R3-P1a's second half). Nothing runs in that window, so no
  hook can stop a goroutine there; the fix is structural (release and
  publish share one critical section) and the mutation passes the suite.
- Runs: `go test ./internal/vmctl` whole (258 tests) and `-race -count=5` on
  the lifecycle, rescue, sweep, resume, recover, refresh and hibernate
  tests, all passing; `go build` of cmd/vmctl, cmd/choir, cmd/corpusd and
  internal/platform (the exported field rename has no other user).

### Behavior changes a reviewer should weigh (introduced)

- A claim now refuses more callers: `StopVMForDesktop`, `LogoutVMForDesktop`,
  `RemoveOwnershipForDesktop`, `MarkUnhealthyForDesktop` and hibernate were
  refused only during a refresh or hibernate; they are now also refused
  during a visit's start or readiness check, a resume, a recovery or a
  rescue (`… is already in progress`, transient). A visit's resolve is
  refused during a resume, recovery or rescue boot, where it used to start
  a second boot of the same computer.
- A visit's readiness check of an active computer holds the claim for its
  `CheckHealth` probe; a deploy refresh posted in that window now defers to
  the retry sweep (one minute) instead of racing a possible readiness boot.

### Discovered, not fixed (heresy delta: discovered)

- The held branch of `resolveDesktopContext` sets `pendingWaiters[key] = nil`
  and returns without deleting it, so a later resolve of that computer
  waits on a channel nobody signals until its context ends (also after the
  hold is cleared). It takes no lifecycle claim, so it cannot block a
  deploy or the sweep; it predates this lane.
- The public platform computer's warmer (`ensureUniversalWirePlatformOwnership`)
  still serializes by `State=booting` and `pendingWaiters`, without the
  claim. A deploy rescue never reaches that class (`alwaysOnBootClass`
  excludes `public_platform`), and the deploy refresh needs `active`, but
  the operator refresh accepts `booting` and can race it.
- Reattach (`ReattachManagedVMs`, reconcile) adopts a live process and
  publishes `active` without the claim. It never boots; a rescue that holds
  the claim makes its own reattach attempt first.
- The always-on sweep still skips a failed computer; only a deploy, a visit
  or an operator act brings it back.
- (Reviewer, non-blocking.) "A freshly issued gateway token" in A3 is
  conditional: `issueGatewayToken` answers "" when issuance fails and the
  boot continues on the guest's existing token. The rescue requests a fresh
  token; it does not require one.

## Review round 3, second reviewer (Gemini 4 Argon on bcdb74659)

Each finding read against 122eb9623:

- **Argon 1 (not fixed by 122eb9623).** The deploy's grant clears the
  allowance before the deploy defers (an unanswering guest; A7). When the
  wedge detector stops the guest, `StartIdleSweeper` runs the always-on warm
  right after reconcile, before the retry sweep; the warm spends the re-armed
  retry and `startExistingVM` → `startAdmittedVM` recovers the stopped
  instance the manager still tracks with `RecoverVM`: the stored config of
  the hung build (`mergeVMConfigOverrides(inst.Config, …)`) and an empty
  gateway token. The rollback target is then consumed by a boot of the old
  image. 122eb9623 serializes that boot against the rescue but does not
  change what it boots. This is the round-2 residual "a sweep boot of a
  tracked instance can still run the stored (older) image", now shown to
  lose the rollback. Fix: a start of a tracked instance whose guest is not
  live (manager state not running or pending) boots through `RefreshVM`
  with a freshly issued token, from every path (sweep, visit, resume,
  `recoverOrRestartActiveVM`), so whichever path spends the re-armed retry
  boots the current deploy. `RefreshVM` and `RecoverVM` both kill then
  boot; they differ only in the config (`refreshBootConfig` vs the stored
  one). A live tracked guest keeps `RecoverVM` (crash recovery, unchanged).
  The grant still precedes the deferral: with the boot fixed, a sweep that
  spends it boots the rollback, which is what the grant is for, charged
  once.
- **Argon 2a, 2b, 2c (fixed by 122eb9623).** (a) the visit's resolve takes
  the `visit start` claim with `pendingWaiters` and `claimRescue` refuses on
  any claim (`TestRescueDefersWhileAVisitBootsTheStoppedComputer`); (b) the
  sweep charges inside its claim step and `spendDeployRetry` needs the rescue
  claim (`TestNoSpendWhileARescueHoldsTheComputer`); (c) resume, recover and
  refresh refuse on any claim, so overlapping sweep ticks or a resolve and
  a resume cannot both boot.
- **Argon 2d (recorded, not changed).** After a rescue defers to another
  path's boot, the pending entry still asks once: if `/health` does not yet
  serve the target (or the computer is layered) the computer takes the
  ordinary planned refresh. That is a marker-backed planned restart of an
  active computer (work resumes), at most one per deferral, the existing
  rule for pending refreshes; it is not a failure retry and does not touch
  the allowance. Making it exact needs a record of which artifacts a
  realization booted, which the registry does not keep.
- **Argon 3 = R3-P1c (fixed by 122eb9623):** `deployRescueStopped` takes
  `VMStateFailed`, `spendRetryLocked` charges it for a rescue, the selection
  lists it (`TestDeployRescuesAProtectedComputerItsOwnFailedRefreshLeftFailed`).
  The always-on sweep still skips failed (named residual).
- **Argon non-blocking (not fixed by 122eb9623; fixed next).** `rescueBoot`
  and `startExistingVM` admit host memory only for an untracked instance.
  A stopped tracked instance holds no memory, so its boot adds a VM; a
  protected computer is always admitted, but without a reservation other
  wakes do not count it.

As built: `trackedBootConfig` and `bootTracked` (`internal/vmctl/ownership.go`)
start a tracked instance whose manager state is not `running`, `pending` or
unreported through `RefreshVM` with a freshly requested gateway token, from
`startAdmittedVM` and `recoverOrRestartActiveVM`; a live one keeps
`RecoverVM`. `startExistingVM` admits host memory for any start that adds a
VM (untracked, or tracked and not live), and `rescueBoot` admits always (it
never starts over a live guest). Tests, each failing on 122eb9623:
`TestSweepAfterADeferredDeployBootsTheCurrentDeployNotTheHungConfig` (the
deferred deploy, the wedge stop, the always-on warm; and a visit's start),
`TestStartsOfAStoppedTrackedInstanceAreAdmittedForHostMemory` (rescue and
sweep). Mutation checks: refresh, the fresh token, sweep admission and
rescue admission each fail their test when removed. Four existing tests
that asserted `RecoverVM` for a dead tracked instance (failed, stopped,
hibernated after a snapshot fallback; the platform warmer's stale-booting
recovery) now assert the same fresh realization through `RefreshVM`; none
asserted the stored config. Behavior change (introduced): a hibernated
computer's snapshot resume of a tracked instance now takes a host memory
admission; protected classes are still always admitted.

## Residuals recorded by the review (not fixed here)

- The rescue's final `/health` check is identity for a non-layered computer and
  liveness for a layered one (it keeps its retained release); see A4.
- A planned-restart marker written by an earlier refresh could survive into a
  rescue boot (the inherited marker weakness of the restart design, section
  3.3; the host does not attest planned-ness yet).
- `saveLocked` does not fsync and only logs errors (restart ledger L1b); a
  crash between a spend and its write can lose the spend or the grant.
- (Repaired after Argon 1.) The always-on sweep and a visit's start used to
  recover a tracked stopped instance with `RecoverVM` and its stored config;
  they now refresh it onto the current deploy. Still on the stored config:
  the recovery of a live tracked guest (crash recovery) and the operator /
  maintenance `recoverVMForDesktop`.
- The always-on sweep still boots a `vmctl-restart` stop or a hibernated
  computer without an allowance (a host act, not a failure); only the rescue
  charges and converts a hard failure.

## Not fixed here (residuals, named)

- A guest that accepts TCP connections but never answers `/health` is not
  reached by a deploy: the wedge definition stays "refuses connections". The
  operator path remains `refresh_active_computers=true` (a forced crash boot).
- The wedge clock is still per vmctl process, so a vmctl restart still adds up
  to one wedge window to the rollback. The standing owner ruling is that
  evidence, not elapsed time, bounds protection; persisting the clock safely
  needs the restart ledger (L1b).
- Defect A's time backstop (24 h) is kept: a fault that recurs on one build
  holds the computer until a deploy or an owner act, which is the design in
  restart-subsystem-design §1 and §4.
