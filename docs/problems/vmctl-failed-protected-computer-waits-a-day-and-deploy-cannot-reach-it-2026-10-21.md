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

- `internal/vmctl/ownership.go`: `VMOwnership.DeployGrantTarget`;
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
