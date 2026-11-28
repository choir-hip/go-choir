---
id: 20261128T1120Z-activity-storm-ends
recorded: 2026-11-28T11:20Z
who: lead
kind: landing
p: 0.8
settle_by: 2026-11-28T20:00Z
evidence_source: the count of `GET https://choir.news/api/changelog` lines in the plain-launch console (bg-js-console.txt) of a signed-in run of mobile/scripts/proof-real.sh on the landed commit
status: settled
settled: 2026-11-28T12:35Z
outcome: true
evidence: signed-in run of proof-real.sh on 776291faa (main 954102fc7 plus the proof lane), 2026-11-28; the plain-launch console holds 0 changelog requests (was 6,643) and 3 `api: session found=false` lines (was 313)
brier: 0.04
---
After lane `activity-storm` lands, a signed-in plain launch of the app in the real-data proof sends 3 or fewer changelog requests in the background drive's window, where the run of 2026-11-28 on bb10f2d21 sent 6,643 in 159 seconds; and `api: session` flips between found and not found 5 times or fewer in the same log, where it flipped 313 times. Test: grep the two counts in the proof's console file. The fix was measured only in the web host, signed out; another per-frame asker I have not seen would make this false.
