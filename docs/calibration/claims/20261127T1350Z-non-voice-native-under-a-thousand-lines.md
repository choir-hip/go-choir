---
id: 20261127T1350Z-non-voice-native-under-a-thousand-lines
recorded: 2026-11-27T13:50Z
who: lead
kind: landing
p: 0.35
settle_by: 2026-11-30T14:00Z
evidence_source: the count printed by scripts/check-autoradio-native-boundary on origin/main at the settle-by time
status: open
---
Non-voice native code on main, by the gate's own count, falls from 3,375 lines (34 files, 2026-11-27) to 1,000 or fewer by 2026-11-30T14:00Z, without a file being added to the voice list to get there. Test: run the gate on origin/main and read `git log` of the voice list over the window. The owner's estimate is one day; the earlier cutover took nine days to reach this number, every native build is the lead's and one at a time, and the pin must move first, so I put it below even.
