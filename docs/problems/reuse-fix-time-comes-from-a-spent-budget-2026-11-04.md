# Engineer reuse: fix time comes from a spent build budget (2026-11-04)

Evidence. deepseek Calendar r6 (assignment-7ebac72e, release 4cf05e4d0):
bind 08:47; candidate 1 frozen 09:40:58 (minute 54) and rejected; the
engineer resumed in the same run and froze candidate 2 at 09:48:15 (minute
61), which failed on a cited failed test job, and the activation ended at
its 60-minute progress deadline. luna files-next r1 froze at 60.1 minutes
and its build ended 15 s later. claude and pool Calendar r1 converged over
three candidates; their fixes took 1.5 to 9 minutes each.

Cause, from the code (internal/agentcore/engineer_reuse.go,
activation_deadline.go): the review park, where the engineer waits with no
model call for management's verdict, does not suspend the activation's
progress deadline (only the provider quota park does), and a rejection
grants no time. A candidate frozen late in the budget leaves its engineer
minutes, or none, to fix what management names, so reuse ends at the
budget instead of at a verdict.

Fix: the review park suspends the deadline as the quota park does
(waiting on management is not the engineer's stall), and a rejection
leaves at least 20 minutes, bounded by the activation's hard wall.

Correction, same day: the 09:47Z check-in recorded a "retained candidate
arms no verify-owed re-check" gap on luna files-next r1. That was wrong.
The platform's cancel report woke management at 09:39:23 and its Verify
ran in that run (5162f2c8), filed under management's task trajectory, not
the document trajectory the lane exported. The nudge at 09:44:47 came
after. No fix was landed for it.
