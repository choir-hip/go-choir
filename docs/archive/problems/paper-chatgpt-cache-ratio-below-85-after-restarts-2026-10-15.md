# Paper ChatGPT prompt-cache ratio sits at 72-76%, below the 85% watch line (2026-10-15)

Date: 2026-10-15, observed 15:55 UTC. Status: documented; hypothesis only.

## Observed (paper VM `vm-1669020adcece51c2f5ff9b6df399ef5`, gateway journal, cached / input per UTC hour)

| hour | chatgpt calls | chatgpt cached% | opencode-go cached% |
|---|---|---|---|
| 08 | 728 | 74.7 | 84.5 |
| 09 | 345 | 87.0 | 87.6 |
| 10 | 110 | 69.1 | 82.1 |
| 12 | 404 | 75.6 | 82.9 |
| 13 | 316+ | 71.8-85.9 | 77.9-79.2 |
| 14 | 662-997 | 76.2-76.8 | 82.3-83.0 |
| 15 (to 15:55) | 929 | 72.0 | 81.9 |

Gateway `/health` `prompt_cache` since the last gateway restart (15:43): chatgpt
74.5% over 334 calls, opencode-go 89% over 117 calls.

## Why this was checked

`fix/engineering-release-copy` (merged `d43dd6301`, deployed 15:17Z) changed the
Texture frame hint (`frame.go`, prompt text). The coordinator asked that the
paper's chatgpt ratio stay at 85% or more (it was about 88% at 09h).

## Reading

The ratio was already 76.2-76.8% in the 14h hour, before the frame.go deploy, and
is 72-75% after it; it is not a sharp drop attributable to that change. The only
hour at 87% (09h) was the long steady period after the 08:00 cache-routing deploy;
since then the paper's VM has been restarted for the host reboot (10:51) and by
deploys at 11:20, 11:37, 12:22, 13:14, 13:54, 15:17 and 15:43. A restart empties
the guest-side conversation state the provider's prefix cache keys on, and the
Texture/management desks start new conversations each time. Hypothesis: the
steady-state ratio is high only when the paper is left running; frequent deploys
keep it at 70-76%. Not tested: a quiet 60-minute window after the last deploy
(15:43), and whether the chatgpt session/conversation header values survive a
guest restart.

## Next

Re-read the hourly ratio for 16:00 and 17:00 UTC with no deploys; if it returns
to >= 85% the cause is restart frequency, otherwise compare the Texture prompt
prefix bytes before and after `d43dd6301`.
