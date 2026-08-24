# Post-replay CoSuper fate reconcile panics on typed-nil capsule executor — 2026-08-24

## Evidence

During the recovery replay-resume experiment (see
`recovery-replay-resume-experiment-2026-08-24.md`), run 2 completed the
reconstruct (replay reached the end of the fetched tape) and the runtime
started; the process then crashed with SIGSEGV:

```
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x10 pc=0x975e83]

goroutine 167612 [running]:
sync/atomic.(*Int32).Add(...)
        sync/atomic/type.go:94
sync.(*RWMutex).RLock(...)
        sync/rwmutex.go:72
github.com/yusefmosiah/go-choir/internal/capsule.(*Executor).AssignmentHandle(0x0, ...)
        github.com/yusefmosiah/go-choir/internal/capsule/executor.go:533
github.com/yusefmosiah/go-choir/internal/agentcore.(*Runtime).assignedCoSuperCapsuleUsable(...)
        github.com/yusefmosiah/go-choir/internal/agentcore/cosuper_assignment_fate.go:88
github.com/yusefmosiah/go-choir/internal/agentcore.(*Runtime).ReconcileCoSuperAssignmentsForTrajectory(...)
        github.com/yusefmosiah/go-choir/internal/agentcore/cosuper_assignment_fate.go:414
created by github.com/yusefmosiah/go-choir/internal/autoputer.Run in goroutine 1
```

(Full trace: `/tmp/exp-run2.log` on node-b, line ~25 onward.)

## Root cause

`(*Runtime).assignedCapsule()` (internal/agentcore/cosuper_assignment_fate.go:68-75):

```go
func (rt *Runtime) assignedCapsule() assignmentCapsuleRuntime {
	if rt == nil {
		return nil
	}
	if rt.assignmentRuntime != nil {
		return rt.assignmentRuntime
	}
	return rt.capsuleExecutor
}
```

`rt.capsuleExecutor` is `*capsule.Executor` (concrete). When it is nil (capsule
executor not configured — the harness run lacked `CHOIR_CAPSULE_*`), the
`return rt.capsuleExecutor` boxes the typed nil pointer into the
`assignmentCapsuleRuntime` INTERFACE, which is then non-nil. The guard in the
only caller, `assignedCoSuperCapsuleUsable` (`exec == nil` → false) lets the
call through, and `exec.AssignmentHandle(...)` dispatches on the typed-nil
`*capsule.Executor` → nil-receiver dereference → SIGSEGV. Same latent hazard at
`revokeAssignedCapsule` (fate.go:97 `exec == nil`).

## Impact classification

- Red path: post-replay runtime start / CoSuper assignment reconcile on the
  computer event authority boundary — the exact boundary the recovery crosses
  when the tape replay completes (B9 finish).
- Today's production guest configures the executor (unit env
  `CHOIR_CAPSULE_BROKER_PATH`/`STATE`/`SOURCE`/`LOWER`), so the crash is not
  observed on the live guest; `configuredCapsuleExecutor` fatals on partial
  config but returns **(nil, false, nil) when ALL vars are absent** — exactly
  the harness condition, and any deployment that drops the capsule vars while
  a CoSuper assignment is bound reproduces this hard crash at guest start.
- Consequence in the experiment: run 2 (replay to the end of tape) crashed at
  the runtime-start reconcile; the final head/witness check for that run was
  lost (the guest-side `/health` died). The platform side was unharmed.

## Belief state

- `assignedCapsule()` must return a NIL interface (not a boxed typed nil)
  when the underlying executor is nil, so the `exec == nil` guards at both
  call sites work as written.
- Fix shape: `if rt.capsuleExecutor == nil { return nil }` before the
  interface return (root fix, no caller changes; no behavior delta when an
  executor IS configured).
- Follow-up: re-run the experiment with the capsule env set (guest-faithful)
  to observe the full-chain finish to head 132,436.

## Not a regression

- The pre-replay reset that the Phase-0/Phase-1 work fixed is independent and
  unaffected; this crash is a NEW discovery at the post-replay boundary.
- Not counted as a repair until a nil-guard + regression test lands.

## Resolution (2026-08-24, commit `85fa83b4`)

`assignedCapsule()` now returns an untyped nil when `rt.capsuleExecutor ==
nil`, so the existing `exec == nil` guards at `assignedCoSuperCapsuleUsable`
and `revokeAssignedCapsule` short-circuit correctly instead of dispatching on
a boxed typed nil. Regression tests
(`cosuper_assignment_nil_executor_test.go`):

- `TestAssignedCapsuleWithoutExecutorIsNilInterface` — the interface itself
  must be nil, not a typed-nil box.
- `TestAssignedCoSuperCapsuleUsableWithoutExecutorDoesNotPanic` — a bound
  assignment with no executor reports unusable instead of SIGSEGV.

Both fail on the pre-fix code (non-nil interface / nil-receiver panic) and
pass on `85fa83b4`. Verified live: run 3 of the experiment (capsule executor
configured) crossed the boundary without a crash (see the experiment receipt).

heresy: discovered (post-replay nil-executor SIGSEGV at the replay-complete
boundary), repaired (nil-interface return; guards now effective), introduced
(none).
