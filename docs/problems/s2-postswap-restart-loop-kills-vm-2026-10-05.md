# S2: post-swap guest restart loop can kill Firecracker and cold-boot the disposable

**Date observed**: 2026-10-05 (S2 station acceptance, rollback leg).
**Computer**: `computer-6450a253b8b6ebc0866471973694f5be` (staging disposable).
**Mutation class**: orange (runtime behavior) with a red edge (protected VM lifecycle surface).
**Status**: documented; no fix attempted in this run. The rollback contract path needs one clean leg before S2 close.

## Symptom

The rollback leg (`rb-push` 10:37:55, re-apply of retained release `2811c779`
under a fresh update id) cut the caller — the expected swap+restart signal.
The guest then restart-looped ~9 minutes (`self-development checkpoint: served
SPA is underivable` in console), Firecracker was killed (`signal: killed`),
the VM hibernated and was marked failed, and the resume came up under a **new**
Firecracker process (2686228 -> 2718287) with a **new** guest boot id
(d541d292 -> e89d4557). The rolled-back release entrypoint (`jvbszni0`)
stayed swapped in and came up healthy, so the *release* outcome was correct —
but the acceptance leg that must record "guest boot id remains unchanged"
could not be satisfied: the boot id changed.

## Mechanism (reported, not root-caused)

The `served SPA is underivable` checkpoint message recurs on every layered
swap whose staged release lacks the serving frontend join, and each failed
health check re-triggers the update's own restart sequence. Whether the loop
is the updater's recovery-restart racing the boot guard, or the new runtime
crashing before the health probe reports, is unknown — the console sink dies
~16s post-boot, so the deciding lines are missing. The Firecracker kill is a
host vmmanager decision on an unhealthy VM, not a guest action.

## Why this matters for S2

Criterion 6 explicitly requires the prior release restored **while the guest
boot id remains unchanged**. A rollback that survives the release swap but
kills the VM proves the recovery path, not the no-reboot invariant. The leg
must be re-run cleanly: record fc pid + boot id before and after, and the run
is only green if both are identical.

## Residual vs the other wedge fixes

- `6fcb05e5` fixed pre-mutation refusals discharging the pending transition.
- The stranded-tail fix discharges accepted events bound to a retired
  realization on boot sweep.
- Neither covers a post-swap runtime that fails health repeatedly: the guest
  keeps restarting inside the same boot until the host kills the VM. A
  fail-closed bound on that loop (or a host-readable guard trip before the
  kill) is the missing piece.

## Next probe

One clean `rollback` leg against the coherent `63865ede` deploy after the
`served SPA is underivable` checkpoint path is understood: build the
predecessor release with the serving frontend joined, apply under a fresh
update id, and compare fc pid + boot id + served commit before/after.
