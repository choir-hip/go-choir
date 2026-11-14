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

## Remaining error

Not repaired. The repair is in the tests, not in retries: replace a
wall-clock deadline with an injected clock or an explicit event to wait
on. `scripts/land` rerunning a failed test once would hide real races and
is not proposed. Until then a landing that fails on a test outside its
diff is rerun, and the failure is added here.
