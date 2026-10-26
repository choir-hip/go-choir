# Idle hibernation cannot see a live engineering capsule (2026-10-26)

Status: documented; fix not built. Found by the density D3 worker's first check
(lane/density-d34, 36f4d671f). Class of the fix: red (guest health contract,
vmctl idle decision), review tier 2.

## Problem

vmctl decides a computer is idle from the guest's `/health` busy fields, read by
`httpGuestBusy`: `running_runs`, `desk_pending_mutations` and
`selfdev_active_operations`. Three states hold a live engineering capsule and
read as idle on all three fields:

- a parked (passivated) engineering run keeps its capsule
  (`assignedEngineeringCapsuleUsable` treats it as usable) but is not in
  `rt.running`, so `running_runs` is 0;
- before the first Freeze, no self-development operation exists yet (it opens at
  `freezeCapsuleEffectBundle`);
- a frozen, verified or awaiting-approval candidate is excluded from
  `selfdev_active_operations` on purpose.

`plannedRestartLiveCapsule` and `live_capsules` (`planned_restart.go`) already
count these capsules, but only for the planned-restart refusal, not in
`/health`.

The hibernate fence skips capsules deliberately, because a memory snapshot keeps
them. But every deploy voids every snapshot today (density D2), so the next wake
is a cold boot, which revokes the capsule and loses its work. The gap is live at
today's 30-minute idle timer; D3's shorter timer would widen it.

## Consequences

- A test computer's round can lose its capsule between rounds or while awaiting
  approval, if it idles 30 minutes and a deploy lands before its next wake.
- D3 (minutes-scale idle hibernation of primary computers) is blocked on this.

## Fix shape (FP0, 12:5x)

1. Guest: `/health` reports `live_capsules` (the count `planned_restart.go`
   already computes).
2. vmctl: `httpGuestBusy` reads it; a live capsule is busy for the idle timer.
3. A guest that does not report the field is not eligible for D3's shorter
   timer (it keeps today's 30-minute rule), so the timer drop cannot run ahead
   of the guest deploy; no old guest is treated as busy forever.
4. D2 (snapshots survive deploys) shrinks the loss further but is not a
   substitute: a capsule must never depend on a snapshot surviving.
