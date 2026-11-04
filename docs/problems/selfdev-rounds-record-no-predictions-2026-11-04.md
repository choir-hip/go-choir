# Self-development rounds record no predictions about their outcome (2026-11-04)

Status: open. Mutation class of the fix: red (run acceptance evidence and
tape records: new rule-settled precommitment kinds, a Cast refusal, the
engineering calibration digest). This note is green.

Spec: [`docs/design/calibration-seed-2026-11-03.md`](../design/calibration-seed-2026-11-03.md),
"The bridge, specified (lane F)" and "The sequence". Owner rulings there are
binding: engineering first; predictions mandatory per round, stated before
the capsule starts; a within-desk A/B split by a hash with a per-computer
secret salt; `calibration.feedback` stops being a switch.

## Evidence

- The arm rounds of 2026-11-03 to 11-04 staged no prediction about the judge
  or the owner's decision. They could not: the closed rule vocabulary
  (`internal/types/commitment_rules.go`, `PrecommitRuleKinds`) is
  `tests_pass`, `freeze_within`, `paths_confined`, `verify_pass`,
  `completes`; nothing settles on the judge's verdict, the approve or reject
  act, rounds to approval or adoption elsewhere. Every existing kind is
  optional, staged by engineering itself after its run starts, and
  management's Cast to engineering carries no prediction at all
  (`planEngineeringCast`, `internal/agentcore/engineering_assignment_runtime.go`).
- Engineering is absent from the calibration digest:
  `calibrationDeskOfThread` (`internal/agentcore/calibration_feedback.go`)
  maps management, Texture and research only, and `collectCalibration`
  reads no rule-settled record.
- The held-out set and the arm split hash a fixed public prefix
  (`calibration.HeldOut`: `"choir-icrl-heldout-v1:" + key`;
  `calibration.Arm`: `"choir-icrl-v1:" + thread`), so a desk that reads the
  source can compute which of its records are held out and which arm it is
  in.
- `calibration.feedback` is a computer setting, default off
  (`store.SettingCalibrationFeedback`), which the 2026-11-03 "no switches"
  ruling retires.

## Owner ruling, 2026-11-04: clock time is calibrated

"clock time is something that agents should be precommitting to and getting
scored on. Thats a factor to calibrate".

So every round also states the wall-clock minutes it expects, and the runtime
scores it:
- what: minutes from management's Cast to the Freeze, and from the Cast to the
  approved decision;
- how stated: a point estimate with an interval and the stated chance the
  actual falls inside it;
- how settled: from the recorded timestamps, the same ones the per-round phase
  timing receipt uses (the Cast record's `committed_at`, the operation's
  `created_at`, the decision act), so the two agree;
- how scored: interval coverage against the stated chance (calibration), the
  log ratio of estimated to actual minutes, and interval width (sharpness)
  beside coverage, so a desk cannot buy coverage with a huge window;
- when an engineer is reused after a rejection, each re-Freeze attempt states
  a fresh "minutes to re-Freeze".

The 2026-11-01 calibration-seed intent already named duration as the first
facet ("you estimated three weeks, you took three hours").

## Why it is a problem

Without a stated, event-settled prediction per round, the self-development
arms produce no calibration record: the model arms can be compared only by
what they deliver, never by whether they know what they will deliver, and the
engineering digest the experiment needs has no data. The public hash lets the
treated desk game the held-out set.

## Heresy delta

discovered: rounds unscored past Freeze; a public held-out hash; a switch the
owner retired. introduced: none.
