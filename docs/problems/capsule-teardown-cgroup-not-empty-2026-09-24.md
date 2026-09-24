# Capsule Teardown: ForceDestroy Strands on Non-Empty Cgroup

Date: 2026-09-24
Status: documented before fix, per the problem-documentation-first invariant.
Found by the R0 freeze-order acceptance probe (third document-channel cast,
`choir-sub-rlm-document-channel-2026-09-22` R0).
Mutation class of this document: green (analysis; no runtime change).

## Observed

On staging (`computer-03335285269bdba4f94377e56879f9e6`, deployed build
`9c607a3f`), a document-channel cast bound assignment
`assignment-0ccbbe2e-2003-588c-a313-3ebcfc733db2` (trajectory
`62e06ee1-83db-5316-b620-ca1ebd8e19ae`, capsule
`capsule-6f703dcb-17b8-5cd6-a75f-b75ece674ce2`). The cell ran a probe,
then `choir.Complete`. Canonical fate history:

- `freeze_requested` 03:32:30.505 → `frozen` 03:32:30.768 →
  `revoke_requested` 03:32:31.052 (events seq 10–12). The Complete saga
  committed its own freeze and revoke intents correctly.
- Final state: `disposition=bound`, `capsule_disposition=revoke_requested`.
  The saga never reached `revoked`.

The run's result text reports the reduce error:

```
reduce: persist tray-1: destroy assignment capsule after durable revoke
intent: cgroups: unable to remove path
"/sys/fs/cgroup/capsule/capsule-6f703dcb-…": still contains running tasks
```

## Root cause

`Executor.destroy` (`internal/capsule/executor.go:447-510`) signals only the
capsule's main process (`caps.Process.Signal(SIGKILL)`), waits for it via
`waitCapsuleProcess`, then calls `caps.Cgroup.Delete()`. It never kills the
cgroup itself. Child tasks spawned inside the capsule (the `go_eval` worker's
subprocesses) are still exiting — or orphaned — when `Delete` runs, so the
cgroup is non-empty and `Delete` returns EBUSY ("still contains running
tasks").

The orphan-cleanup path (`internal/capsule/namespace.go:187-193`) already does
the right thing: `manager.Kill()` (writes `cgroup.kill`, atomically SIGKILLs
every task) then `manager.Delete()`. The live `destroy` path omits the `Kill`
step and does not wait for the cgroup to drain.

`Pdeathsig: SIGKILL` (`executor.go:376`) reaps direct children on main-process
death, but does not guarantee the cgroup is empty at `Delete` time — reparented
or still-dying tasks race the removal.

## Consequence

The assignment fate is recorded (`revoke_requested` is durable), so no work is
lost and the wedge class stays closed. But the capsule cgroup, process tree,
and overlay mount leak until restart reconcile
(`cleanupOrphanedCapsuleCgroup`) runs. On a quiet document-parented computer
that is the next boot — a real resource leak on every assignment completion,
not a one-off.

## Fix shape

In `Executor.destroy`, after the main-process wait and before `Delete`:
`Cgroup.Kill()` (write `cgroup.kill`) then wait for `cgroup.events` to report
`populated 0` (bounded by ctx), then `Delete`. Mirrors the orphan-cleanup
contract; the freeze path already polls `cgroup.events` for `frozen` in
`setFrozen`, so the wait-for-empty pattern is established.

## Evidence

- Trajectory `62e06ee1-83db-5316-b620-ca1ebd8e19ae`: events 1–12; fate history
  `freeze_requested`/`frozen`/`revoke_requested`, no `revoked`.
- Assignment `assignment-0ccbbe2e-2003-588c-a313-3ebcfc733db2`:
  `disposition=bound`, `capsule_disposition=revoke_requested`.
- Run `run:assignment-0ccbbe2e-…`: `state=completed`, result text quotes the
  cgroup EBUSY teardown error.
