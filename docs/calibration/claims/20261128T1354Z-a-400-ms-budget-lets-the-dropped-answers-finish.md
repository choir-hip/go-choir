---
id: 20261128T1354Z-a-400-ms-budget-lets-the-dropped-answers-finish
recorded: 2026-11-28T13:54Z
who: lead
kind: experiment
p: 0.7
settle_by: 2026-11-28T22:00Z
evidence_source: the count of "took N ms of its own (CPU) time" lines, and the marker check of the sign-out drill, in a signed-in run of mobile/scripts/proof-real.sh on lane turn-budget 7c7f85593 merged with main
status: open
---
With Exact's own `turnBudgets` set to 400 ms for radioHold, radioWrite and accountSignOut, a signed-in run of the real-data proof drops none of those three answers in any drive and the sign-out drill leaves no marker or stamp. The largest overrun seen so far is 261.5 ms, so 400 should hold if the cost is work and not a stall; the experiment's own clock-check loop adds an unknown amount to the first three holds. A second reading from the same run, not scored: whether `performance.now` moves inside a turn, and how much slower than Bun (10.6 ms for the same loop on this Mac) the app's engine is.
