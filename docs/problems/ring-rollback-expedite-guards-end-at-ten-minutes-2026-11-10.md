# Under blanket Ring-Expedite the ring guards watch a release for ten minutes, then stop (2026-11-10)

Date: 2026-11-10. Found by: the X5 code read of the overnight Autopaper
mission. Status: open, problem record before any fix. Mutation class of a fix:
red (vmctl ring walk). Read only. The chain itself is in
[ring-rollback-host-deploy-has-none-2026-11-10.md](ring-rollback-host-deploy-has-none-2026-11-10.md).

## Evidence

- `stepExpedited` (`internal/vmctl/ring_walk.go:713-770`) closes a ring once
  `ringExpediteWatch` (10 minutes, `:707`) has passed since the expedite and
  every required member that is not pending was seen healthy on the release.
  When no ring is open the walk completes and the baseline moves to the new
  release (`:766-769`).
- Guards run only over unclosed rings of a `walking` walk
  (`sampleTargets`, `ring_walk.go:1208`; `observeRing` is called only for rings
  with `ClosedAt` zero, `:719-724`). After completion nothing samples the
  release, and the baseline (`s.Baseline`, the return target) is the new release, so a later breach has no return
  target and no pin.
- CLAUDE.md, 2026-11-09: until Autoradio is solid every landing commit carries
  `Ring-Expedite`. So the effective protection of every release today is the
  readiness delay (5 minutes) plus strikes inside a 10 minute watch, not the
  ring soaks (R2 4h, R3 4h, R4 2h, R1 15m: `ring_walk.go:90-95`).
- A defect that shows after ten minutes (a leak, a slow wedge, a crash at the
  first heavy run of the paper) is not rolled back by the walk. The guest
  release is then undone by `git revert` and a new walk, by a human.

## What this is and is not

Not a defect of the walk: it does what the expedite design says (an expedite
keeps the guards and the paper held until seen healthy, and drops the soak
floors). It is the cost of the owner's blanket expedite ruling, which makes the
10 minutes the whole watch. The readiness delay alone is 5 minutes on a fresh
boot, so a breach has roughly 5 minutes of real signal.

## Options, cheapest first (not chosen)

1. Do nothing; accept the ruling's cost until Autoradio is solid, then drop
   the blanket trailer (the owner's stated plan).
2. Raise `ringExpediteWatch` for R5 only (the paper) to the R5 soak (1h), since
   the paper is always busy and shows trouble late. One constant; the paper
   still refreshes last and automatically.
3. Keep guards on the completed walk's release for the R5 soak and roll back
   on a breach. Needs a `last_good` that does not move at completion.

Heresy delta: discovered 1, introduced 0, repaired 0.
