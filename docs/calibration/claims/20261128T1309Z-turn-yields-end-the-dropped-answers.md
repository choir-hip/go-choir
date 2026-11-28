---
id: 20261128T1309Z-turn-yields-end-the-dropped-answers
recorded: 2026-11-28T13:09Z
who: lead
kind: experiment
p: 0.4
settle_by: 2026-11-28T20:00Z
evidence_source: the count of "took N ms of its own (CPU) time" lines in the drive dumps of a signed-in run of mobile/scripts/proof-real.sh on lane turn-budget 17394bd58 merged with main
status: settled
settled: 2026-11-28T13:52Z
outcome: false
evidence: signed-in run of proof-real.sh on lane turn-budget 17394bd58 merged with main, 2026-11-28; seven answers were still dropped (radioHold in cards, read, spine and twice in drill, 101.6 to 192.0 ms; radioWrite 186.1; accountSignOut 116.3), the sign-out drill left its marker and stamp, and the lane's own log line for a long stretch of our code was written zero times
brier: 0.16
---
With the yields of lane `turn-budget` (17394bd58), a signed-in run of the real-data proof shows no answer dropped for running over Exact's 100 ms turn in any drive, where the run on 776291faa showed eleven (radioHold nine times, radioWrite once, accountSignOut once), and the sign-out drill leaves no marker or stamp. The builder measured our own longest stretches at 1 to 3 ms in Bun on this Mac and could not make a 25 ms budget fail, so the fix rests on a guess that the app's interpreter is about a hundred times slower; if the time is in the host's own work the yields change nothing. I put it below even.
