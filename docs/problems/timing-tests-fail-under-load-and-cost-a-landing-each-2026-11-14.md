# Timing tests fail under load and each failure costs a landing

2026-11-14. Recorded 07:20Z.

## Problem

Tests that assert on short wall-clock windows fail when the machine is
busy. Each failure throws away a landing run of 45 to 90 minutes or blocks
a deploy until a rerun. Three happened between 06:40Z and 07:06Z tonight,
none in code the change under test touched.

## Evidence

- `internal/gateway/searchplane`
  `TestKeenableTimeoutOutageIsBenchedFromCompletion/memory`: CI run
  43567350353 on a21558f9b (per-op counters; touches no gateway code),
  race shard 7, `transient_repair_test.go:90: probe arrivals = [], want an
  answer`. Passed on rerun of the same commit. The deploy waited for the
  rerun.
- `internal/agentcore`
  `TestEngineeringBoundDeadlineRunsFromBindNotArrival`: landing of
  receipts step 1 on the runner VM, 06:52Z,
  `engineering_admission_test.go:520`, a 300 ms assignment deadline expired
  while two builder suites shared the VM. The landing (33 minutes in) was
  lost.
- `internal/agentcore` `TestShotReuseForeignJobDoesNotUnlockTheSwap`
  (`selfdev_shot_reuse_test.go:282`, `:311`): a landing died at 10:51Z with
  `panic: Fail in goroutine after TestShotReuseForeignJobDoesNotUnlockTheSwap
  has completed`, which kills the whole package run, not one test. The log
  just before: "selfdev owner act: fail ...: read the operation back: sql:
  database is closed".
- `TestRevertConcurrentTapsAreOneAct`: failed once in a landing earlier
  tonight and passed on rerun.
- Three more one-off failures earlier in this session are in the session's
  landing logs and are not named here; I did not record them when they
  happened, which is the error this record corrects.
- An older record of the same class:
  `ci-flakes-actorruntime-recovery-and-vmctl-admission-2026-10-26.md`.

## Belief

These are not three bugs; they are one. A test that waits a few hundred
milliseconds for an asynchronous effect is a load detector. The build VMs
are now shared by landings and builder suites, and Node A is short of
memory (see `node-a-oom-killed-the-agents-build-vm-2026-11-14.md`), so
load is the normal case. The searchplane test already has a fake clock;
its failure is a goroutine ordering wait, not the clock.

## Cause and repair (2026-11-14, lane `flake-fix`, tests only)

1. `TestShotReuseForeignJobDoesNotUnlockTheSwap`. The appender starts each
   post-commit observer call as an unjoined goroutine
   (`computerevent/appender.go:1114`, `go (*observer)(kind)`). The shared
   fixture's observer (`selfdev_derivable_materialize_test.go`, in
   `newDerivableSelfDevFixtureBooted`) runs the whole reconcile drain in
   that goroutine, and the drain calls the test's `shotsAskedHook`, which
   calls `t.Errorf`. The test returns when it has seen the state it wants;
   a drain started by the last appends is still running, finds the store
   closed by cleanup, and the hook reports through a finished `t`. Every
   test on the fixture has the same leak; this one hits it because its hook
   calls `t` and its last append races the return. Repair, once, in the
   fixture: the observer registers with a wait group (refusing calls that
   start after cleanup began), and a cleanup registered after the store's
   closes the fixture to new drains and waits for the running ones, before
   the store closes. No product change.
2. `TestEngineeringBoundDeadlineRunsFromBindNotArrival`. The test set a
   300 ms deadline and had to arm, encode and run the open-time wake within
   300 ms of bind (`engineering_admission_test.go`, the call at the old
   line 520); the handler compares with `time.Now()`
   (`continuation_schedule.go`, `HandleAssignedEngineeringFateDeadline`).
   Repair: the deadline is read per call, so the test sets an hour while
   arming, a value halfway between the stored arrival and bind times for
   the open-time wake, and 1 ns once bound, with no second sleep. The
   open-time step still has a window (half the queue wait, 200 ms or more,
   on each side, and only the one handler call inside it); it cannot be
   removed without a clock seam: `HandleAssignedEngineeringFateDeadline`
   uses `time.Now()` where admission uses `rt.admissionNow`. That seam is a
   product change and is not made here.
3. `TestKeenableTimeoutOutageIsBenchedFromCompletion/memory`. A probe call
   delivers its arrival and only then runs its deferred `releaseProbe`
   (`router.go:570-572`), so the stream can close and the next search start
   while the probe claim is held; `claimProbe` then fails, the search skips
   the provider as "probing", and it has no arrivals. The fake clock is not
   involved. Repair: `startAndDrain` waits for the probe claims to release
   (`transient_cooldown_test.go`). The product could release before it
   sends; that is a one-line reorder not made here.

## Remaining error

The three cases above are repaired on `lane/flake-fix` (not yet landed).
`TestRevertConcurrentTapsAreOneAct` and the three unnamed ones are not
investigated. The repair is in the tests, not in retries: replace a
wall-clock deadline with an injected clock or an explicit event to wait
on. `scripts/land` rerunning a failed test once would hide real races and
is not proposed. Until then a landing that fails on a test outside its
diff is rerun, and the failure is added here.
