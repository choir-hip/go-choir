# Idle computers keep their old build after a deploy

Found 2026-10-11 while preparing the P2a deployed demo (staging a2bcf724,
deployed 18:37Z). Status: documented, not fixed. Cause unconfirmed.

## What happened

After the a2bcf724 deploy, every active QA test computer still ran an
older guest build, with no running runs:

| test computer | guest build | deployed at | running runs |
|---|---|---|---|
| demo-arxiv-2026-10-10c | 0f1c86c7 | 2026-10-11T17:06:28Z | 0 |
| demo-arxiv-2026-10-10 | 726aa392 | 2026-10-11T13:51:52Z | 0 |
| demo-ms-2026-10-10b | a114b46e | 2026-10-10T23:25:51Z | 0 |

vmctl restarted at 18:35:23Z ("idle timeout set to 30m0s", "route-gated
idle sweeper interval set to 2m0s"), but none of these computers stopped by
19:15Z. The reader computer had been idle since about 17:36Z. A hibernated
computer (demo-computer-2026-10-10) answered API requests with 502 for a
minute and stayed hibernated; its browser session had also expired.

CLAUDE.md says deploys "update idle computers and leave busy ones to their
next idle moment". On staging today, an idle computer that stays active
never takes the new build, so a deployed acceptance demo needs a fresh test
computer (created: demo-p2a-2026-10-11, booted a2bcf724 at 19:19:35Z).

## Hypotheses (unconfirmed)

1. The idle sweeper counts a computer as busy while it has live research
   desks (the reader computer's health shows researcher_count 3 with zero
   running runs), so it never stops.
2. The sweeper is route-gated and these computers keep a live route
   (an open desktop stream or a poll schedule), so they never idle.
3. The deploy step updates only computers that are stopped at deploy
   time; there is no later "next idle moment" update.

The next observation: the sweeper's decision for one of these computers
(vmctl logs or the ownership record's idle fields).

## Why it matters

A deployed fix does not reach a computer that stays up, so a staging
acceptance demo on an existing test computer tests the old code. P1's demo
reached the new build only because its computer happened to boot at 17:06.
The CI dispatch input refresh_active_computers restarts every running
computer, which interrupts work on the owner's computer too, so it is not
a substitute.
