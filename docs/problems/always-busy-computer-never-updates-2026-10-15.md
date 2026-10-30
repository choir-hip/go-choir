# An Always-Busy Computer Never Takes an Update

Date: 2026-10-15 (owner ruling); evidence from the night of 2026-10-14.
Computer: the live paper, `vm-1669020a…` (warmness class
`critical_protected`, owner `e5962310…`). Mutation class of the fix:
**red** (VM lifecycle, restart semantics, deploy routing). Status:
documented; fix on branch `feat/always-on-planned-refresh`, not pushed.

## Owner ruling (2026-10-15)

> "we need to update the Autopaper computer even when it is busy, because
> it's always going to be busy."

This amends one sentence of CLAUDE.md "Restarts End Work (Crash) Or
Resume It (Planned Update)": "Deploys never restart a busy computer". The
rest of that rule stands: a crash never resumes; a planned restart for an
update resumes once, keyed by a durable marker consumed once at boot.

## What happened

- Every deploy asks each active computer's guest to write its
  planned-restart marker (`POST /internal/runtime/planned-restart`,
  `internal/vmctl/deploy_refresh.go`). The guest refuses with 409 while any
  run is running or a self-development operation is mid-transaction, for
  example `{"outcome":"busy","running_runs":2,"selfdev_active_operations":0}`,
  and vmctl defers the computer **to the next deploy**.
- The paper polls 14 sources every 15 minutes with intake labels on, so at
  deploy time it always has running runs. It was deferred by the 04:21
  (e9d727a7, 2 running runs) and 05:26 (d5139fd6, 1 running run) deploys on
  10-14 and was still on 9ace75e5 at 08:28
  (`planned-refresh-closes-paper-schedules-2026-10-14.md`, "After the fix
  deployed").
- Tonight it stayed on a build without the Dolt GC fix and refilled its
  disk. The fix existed on the platform; the paper could not take it.
- Nothing retried it between deploys. CLAUDE.md says deploys "leave busy
  ones to their next idle moment"; the code leaves them to the next
  **deploy**. The patch-pipeline report found the idle-moment retry
  unimplemented; this confirms it (`deploy_refresh.go` has no retry, and
  CI prints "Deferred (busy) computers are retried at the next deploy").

## Why this is wrong

- A 24/7 paper is by definition never idle at a deploy, so "deploys never
  restart a busy computer" means it never updates. Security, storage and
  correctness fixes all miss it. The restart design (§9, "Turn boundary:
  wait, then defer") already anticipated this: "A recorded forced-refresh
  kind is added only if a never-idle computer blocks a needed fix." That
  has now happened.
- The busy refusal conflates two kinds of busy. Running runs are
  resumable on a planned boot (below). A self-development apply or a live
  capsule is not: interrupting those loses or corrupts work. Only the
  second kind needs to block a refresh.

## What a planned boot does today, per obligation kind (code-verified)

Boot path: `internal/autoputer/run.go` consumes the marker
(`ConsumePlannedRestartMarker`) before any actor runs; `Runtime.Start`
(`internal/agentcore/runtime.go`) branches on `BootWasPlannedRestart()`;
the actor handler (`internal/actorruntime/handler.go`, `plannedBoot`) and
the wake projector (`preBootWorkWake`) let pre-boot occurrences through.

| Obligation | Planned boot | Crash boot |
| --- | --- | --- |
| Run pending/running (desk run mid-turn) | **Resumes.** `passivateInterruptedActivations` parks it (`passivated_reason=runtime_restarted`); its unconsumed actor occurrence (initial_dispatch, coagent_result, channel_message, inbox_record) is redelivered and `handleInitialDispatch`/the park-resume path reactivates it from the persisted conversation. The cut model call is redone. | closed `interrupted_by_restart` |
| Management task run | **Closed visibly**: fate `planned_restart_closed`, its round closes `run_passivated` with residuals and wakes the parent (R2-12: tasks are not resumed). | closed |
| Run passivated/blocked before boot | Unchanged (parked/blocked as before). | closed |
| Texture revision run | Resumes as a run (above); its agent mutation is marked stale so the revision re-reads its base. | closed; mutation stale |
| Actor wakes (outbox) | **Resume**: delivered. | pre-boot work kinds marked interrupted |
| Controls, directives, owner revisions, inbox records (actor occurrences) | **Resume**: redelivered (`plannedBoot` skips the pre-boot gate). | consumed as `interrupted_by_restart` |
| Rounds (records) | Stay open; closed when their run ends (`closeRoundsOfEndedRun`). | system closure `interrupted_by_restart` |
| Work items | Stay open; a passivated spawned run gets its work item (`ensureSpawnedCoagentWorkItem`). | may stay open (problem 2026-10-12) |
| Engineering assignment, capsule **active** | **Closed visibly**: capsules live in the executor's memory (`capsule.Executor.capsules`), so after any restart the capsule is absent; reconcile revokes it and cancels the assignment "restart revoked absent assignment capsule", fails its bound selfdev op, and the final Report wakes management. The capsule's in-progress work is lost. | cancelled silently `interrupted_by_restart` |
| Engineering assignment, capsule **freeze_requested/frozen** (not decided) | With a staged proposal: resumed (`resumeStrandedFrozenAssignmentCommit`). Without one: revoked and cancelled as above, which destroys the frozen candidate. | cancelled silently |
| Self-development op pre-decision (requested, executing, frozen, verified) | Not closed; but executing ops lose their capsule (row above). | failed `interrupted_by_restart` |
| Self-development op decided (accepted, materializing, rollback_pending) | Re-driven by reconcile (bounded by `guestApplyAttemptLimit`). A host kill mid-apply leaves the updater journal mid-flight. | settled from the journal, never re-applied |
| Platform update pending | Resumed once (`resumePendingPlatformUpdate`, attempt-bounded). | settled from the journal |
| Autopaper source schedule | **Continues**: a cut poll is cleared (not counted as a crash) and the saved tick re-armed; the poll's wake is redelivered. | schedule kept, poll closed, next tick after one interval (L1) |

Loop safety: the marker is removed before work resumes, so a resume that
crashes the guest leads to a markerless boot that closes the work
(`TestPlannedRestartEndpointMarkerResumesOnceThenCrashBoot`). The actor
dispatcher bounds redelivery attempts durably.

So running runs are safe to interrupt on a planned boot. Self-development
applies (decided ops) and live capsules (bound, not terminal) are not.

## Marker TTL against a slow boot

`ConsumePlannedRestartMarker(storePath, now)` treats a marker older than
15 minutes as a crash, and `now` is the instant of consumption
(`run.go`, after the boot Dolt GC, store open, projection base and
credential exchange). The boot Dolt GC runs on journals up to 4 GiB; the
paper took 2 to 4 minutes to boot tonight, and its disk was full of Dolt
journal. A slow pre-consumption phase can therefore expire a valid marker
and turn a planned refresh into a crash boot. (Replay and the record-index
build, about 2 minutes on first boots, run after consumption and do not
count.)

## Failure modes the fix must exclude (written before the tests)

1. A busy always-on computer is still deferred, so the paper never updates.
2. A busy ordinary computer is refreshed (the ruling covers always-on
   computers, not every user's running turn).
3. A refresh kills a self-development apply in flight (accepted,
   materializing, rollback_pending), or a live capsule (active, freezing,
   frozen and undecided), even on an always-on computer.
4. A planned resume loops: the marker survives the resume, or a crash
   during the resume is read as planned.
5. A deferred computer waits for the next deploy instead of being retried
   until it takes the update.
6. The retry and a CI deploy refresh the same computer at once, and the
   loser's failure path clears the winner's marker (vmctl clears the
   marker when `RefreshVMForDesktop` fails, and that fails with "refresh
   is already in progress" for a concurrent call).
7. The retry refreshes a computer repeatedly (a layered computer serves its
   release commit, never the pointer commit, so "still not on the target"
   is not a reason to refresh again), or keeps killing a computer whose
   refresh fails.
8. A slow boot expires the marker, so the planned refresh boots as a crash.
9. A guest built before the change rejects the new request shape and the
   paper can never be reached by the deploy that ships it (the handler
   decodes with `DisallowUnknownFields`).

## Design

1. **vmctl policy, not a CI flag.** vmctl decides from the computer's
   warmness class: `critical_protected` and `public_platform` (the
   computer-level, platform classes; the paper is `critical_protected`)
   may be refreshed while busy. `premium_always_on` (a paid user's
   computer) and everyone else still defer: the conservative reading of
   the ruling, and the retry (3) reaches them at their next idle moment.
   One authority (vmctl's warmness policy), and the retry sweep, which has
   no CI, applies the same rule. CI needs no flag.
2. **Guest: two kinds of busy.** vmctl asks with `?allow_busy=1` (a query
   parameter, so a guest built before the change ignores it and answers as
   before, instead of rejecting an unknown body field). With it, running
   runs no longer refuse; the guest still refuses (409, outcome `busy`,
   naming the reason) while a self-development operation is executing,
   accepted, materializing or rolling back, or an engineering assignment
   holds a live capsule (bound, not terminal, capsule
   active/freeze_requested/frozen/revoke_requested). It writes the marker
   and answers with the running-run count it is interrupting.
3. **Retry until it takes the update.** Every deferral is recorded in a
   durable pending set (`VM_STATE_DIR/deploy-refresh-pending.json`, keyed
   by computer, carrying the newest target). A vmctl sweep (every minute)
   retries each pending computer through the same deploy-refresh path until
   it is refreshed, already serves the target, or is skipped (held, not
   active, gone); a failed refresh is dropped (logged) rather than retried
   into a kill loop, and the next deploy re-records it. A per-computer
   in-flight guard makes a concurrent deploy refresh answer "deferred" before
   touching the marker. `GET /internal/vmctl/deploy-refresh` lists the
   pending set.
4. **TTL bound to the refresh.** The marker's age is taken at process
   start (the first line of `autoputer.Run`), not at consumption, so boot
   Dolt GC and store open do not count against the 15 minutes. The window
   is then the kill and the VM boot (well under a minute). Consumption
   stays where it is: before any actor runs.

## Ceremony (red)

- **Conjecture delta.** Before: busy computers are safe only if never
  restarted. After: a planned restart is safe for every obligation that is
  durable on the tape (runs, occurrences, wakes, rounds, schedules) and
  unsafe only for in-memory or mid-apply state (capsules, decided
  self-development ops), so only those block a refresh.
- **Protected surfaces.** VM lifecycle (deploy refresh, new retry sweep),
  restart semantics (planned-restart endpoint, marker age), deploy routing
  (CI's deferred message). Not touched: Texture canonical writes,
  checkpoint/route projection, auth, gateway, run acceptance.
- **Admissible evidence.** Unit tests in `internal/vmctl` and
  `internal/agentcore` for failure modes 1 to 8; the staging proof is the
  paper's `/health` build commit matching the deploy commit after a deploy
  that finds it busy, with `/api/runtime/obligations` reporting
  `restart.kind=planned` and its schedules still active.
- **Rollback.** `git revert` of the fix commits; the guest change is
  backward compatible (old vmctl sends no `allow_busy`), and the pending
  file is advisory (deleting it only loses retries).
- **Heresy delta.** Discovered: the "next idle moment" rule was never
  implemented (deferral waited for the next deploy); the deploy-refresh
  failure path could clear a concurrent refresh's marker; marker age was
  measured after slow boot phases. Introduced: busy critical computers
  have their in-flight model calls cut and redone on resume (at-least-once
  tool effects for a turn cut mid-tool-call). Repaired: the three
  discovered ones, once landed.

## Remaining error field

- The admission race (restart design §2, "a run can be admitted between the busy check (200) and the kill") remains: between the guest's 200 and
  the kill, a self-development op can be approved or a capsule opened; it
  then meets the planned-boot behaviour in the table (bounded, visible).
- A turn cut mid-tool-call is redone from the last persisted conversation;
  a non-idempotent tool effect inside it can repeat.
- vmctl still cannot see whether the guest read the boot as planned (the
  D7 epoch check can overrule it); `/api/runtime/obligations` shows it.
- A pending in-guest platform update does not block a host refresh (as
  today on the idle path); its own attempt bound covers it.
- The bootstrap deploy: the paper runs a guest without `allow_busy`, so
  the deploy that ships this defers it once more; the retry sweep then
  refreshes it at its first idle minute between polls. If that never comes,
  `refresh_active_computers=true` (a crash boot; schedules survive crashes
  since L1) is the one-time operator path.

## Fix (branch `feat/always-on-planned-refresh`, not pushed)

- 9bc79854f (guest): `?allow_busy=1` on the planned-restart endpoint;
  self-development ops (executing, accepted, materializing,
  rollback_pending) and live capsules always refuse; marker age taken at
  process start.
- f4260e116 (vmctl, CI text): warmness policy (`critical_protected`,
  `public_platform`), durable pending set and one-minute retry sweep,
  per-computer in-flight guard; CI logs `busy_allowed` and says deferred
  computers are retried by vmctl. No new CI flag.
- Tests: `internal/agentcore` `TestPlannedRestartEndpointAllowBusy*`,
  `TestPlannedRestartAllowBusyResumesOnceThenCrashBootCloses`,
  `TestPlannedRestartMarkerAgeIsTakenAtProcessStart`; `internal/vmctl`
  `TestDeployRefreshAllowsBusyForCriticalProtectedComputer`,
  `TestDeployRefreshBusyOrdinaryComputerStillDefersAndIsRecorded`,
  `TestDeployRefreshSelfDevelopmentApplyDefersEvenWhenBusyIsAllowed`,
  `TestDeployRefreshSweep*`, `TestDeployRefreshConcurrentRefreshDoesNotTouchTheMarker`,
  `TestDeployRefreshPendingSurvivesVMCTLRestartAndKeepsNewestTarget`.
- Not yet done: the Landing Loop (push, CI, staging deploy, the paper's
  `/health` commit and `restart.kind=planned` after a busy deploy). CLAUDE.md
  still says "Deploys never restart a busy computer"; the owner's ruling
  amends it for platform computers, and that sentence is the owner's to
  change.


## Merged from docs/problems/deploy-restarts-busy-computers-2026-10-09.md (2026-10-09)

**Deploys restart busy computers (2026-10-09).**
Mutation class of the fix: red (deployment routing, VM lifecycle).

(Full text in git history at the path above.)
