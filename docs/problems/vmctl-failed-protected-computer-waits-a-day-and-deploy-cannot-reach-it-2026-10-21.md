# A failure-stopped always-on computer waits 24 hours, and a rollback deploy cannot reach it

Date: 2026-10-21. Status: **fix built** on lane/host-vmctl-b4-b5 (FP3 group
1), not yet landed or proved on staging. Class: red (vmctl, VM lifecycle, deploy routing). Owner rule:
"we need paper publishing 24/7, even after crashes."

This is the vmctl half of the 50-minute Autopaper outage in
[stored-column-add-blocks-paper-boot-2026-10-21.md](stored-column-add-blocks-paper-boot-2026-10-21.md)
("What followed", and its "Fix order" item 4), and the second occurrence of
item 2 of
[paper-computer-disk-full-outage-2026-10-15.md](paper-computer-disk-full-outage-2026-10-15.md).
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
- **A5 (residual).** A protected computer in `VMStateFailed` (a failed deploy
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

## Residuals recorded by the review (not fixed here)

- The rescue's final `/health` check is identity for a non-layered computer and
  liveness for a layered one (it keeps its retained release); see A4.
- A planned-restart marker written by an earlier refresh could survive into a
  rescue boot (the inherited marker weakness of the restart design, section
  3.3; the host does not attest planned-ness yet).
- `saveLocked` does not fsync and only logs errors (restart ledger L1b); a
  crash between a spend and its write can lose the spend or the grant.
- The always-on sweep (`ResumeVMForDesktop`) and a visit's recovery still
  recover a tracked stopped instance with `RecoverVM`, which keeps the stored
  config. Only the deploy rescue refreshes onto the current deploy's artifacts
  (A3). After a deploy that deferred the paper's refresh, a sweep boot of a
  tracked instance can still run the stored (older) image.
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
