# S2: post-swap guest restart loop can kill Firecracker and cold-boot the disposable

**Date observed**: 2026-10-05 (S2 station acceptance, rollback legs).
**Computer**: `computer-6450a253b8b6ebc0866471973694f5be` (staging disposable).
**Mutation class**: orange (runtime behavior) with a red edge (protected VM lifecycle surface).
**Status**: cause split 2026-10-05 ~13:20 — two distinct kill mechanisms, one
fixed by environment, one still open. See Addendum.


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

## Addendum 2026-10-05 ~13:20: two distinct kill mechanisms

The 10:37 rollback kill and the 12:59-13:07 rollback kill have **different
causes**, established by the vmctl journal:

1. **10:37 — guest restart loop (~9 min of `served SPA is underivable`),
   then `signal: killed`, hibernate, marked failed.** Guest-side loop is
   the plausible killer (host killed an unhealthy VM). Still open —
   whether the updater's recovery-restart races the boot guard or the new
   runtime crashes before the health probe reports is unknown; the console
   sink dies ~16s post-boot so the deciding lines are missing.
2. **12:59–13:07 — host pressure-reclaim hibernated the VM mid-restart**
   (`decision=reclaim`, epoch 12895/12896), then vmctl itself went down
   during the fae12950 deploy and the resume came up under a new FC
   process. Not a guest defect at all — an environment kill during the
   leg. The 13:07 re-run then applied **cleanly**: tape seq 109–111
   accepted/started/applied, serving 6fcb05e5 healthy within ~90s, no
   restart loop. The only reason criterion 6 is still open is the changed
   boot id across the environment kill, plus a harness bug found in the
   same leg (`wait_served` compared full-SHA served output against a
   short-SHA want, polled 450s, and the outer timeout killed the leg
   before it wrote its note — fixed, prefix-match).

**Consequence for the gap list:** the rollback path itself is now proven
twice (release outcome correct, restore healthy, no guest loop on the
re-run). What remains is one *quiet-environment* re-run that records fc
pid + boot id + served commit before and after with no host kill in the
window — not a code fix.

## Closure 2026-10-05 ~13:24: clean rollback leg green — criterion 6 satisfied

Re-ran the rollback leg in a quiet window (no deploy, no pressure-reclaim,
vmctl healthy): `rb-push` 13:23:23 re-applying retained release `2811c779`
under a fresh update id, then serving `2811c779` healthy with **same fc pid
3123628 and same boot id 7f725ae1**. Tape seq 112–114
accepted/started/applied (`s2acc-rb-20261005T132300Z`), no restart loop, no
host kill. Record: `rollback-clean` row in
`s2-acceptance-20261005T132300Z.jsonl` (the leg's own note raced the
restart, so pins were reconstructed from health + boot timeline + tape).

The 10:37 guest-loop kill (cause 1 above) remains an open hazard for
post-swap health-failing releases in general, but it did not recur here —
the rollback release was healthy, so no loop started. The remaining
question is bounded: what kills the VM when the *new* runtime fails health
repeatedly, not whether healthy rollbacks preserve the boot.
