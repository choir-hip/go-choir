# Research runs rose from 13 to 62 an hour right after the research-frame prompt change

Recorded 2026-11-23. Read-only investigation of the hourly counts and 15-minute
buckets; no run ledger and no Cast objective text was read (see "Remaining
error"). The revert that carries this record is the conservative act, not a
diagnosis.

## Problem

Commits `3dcddbe86`, `2a0c5745f`, `20e0de1b8`, `af15c2355`, `33fc8ff10`
("research-frame": research role prompt v21, overlay, Texture prompt v4, the
corpus search's `words_note`, the report log's gap count, the status script)
deployed to staging at 2026-11-23 07:58Z. Research runs then rose by a factor
of about five, the provider began refusing calls for quota, and search credits
project to run out before renewal.

## Evidence

- Research runs an hour: 13.2 from 2026-11-22 17Z to 2026-11-23 07Z; 62.3 from
  08Z to 10Z. In 15-minute buckets run starts went 4, 6, then 21, and 17 to 27
  from 08:15Z.
- Captures, management calls and Texture runs stayed flat over the same hours,
  so the input did not rise.
- Web searches rose from 40 to 132 an hour; searches per run stayed at 3.3 to
  3.8. The rise is in the number of runs, not in searches per run.
- After the deploy 41 of 48 newly opened stories got a first-round research
  Cast.
- Model calls doubled to 1,792 an hour; the provider refused 82 calls for quota
  in one hour; search credits project to run out before renewal.
- 19 research rounds across 14 stories in 1.6 hours "ended without a final
  Report", with Casts following.

## Belief

About 55%: the Texture prompt (`internal/textureprompts/texture.yaml` v3 to v4)
changed the Cast rule from "cast research only for what they leave open" to
"cast research for whatever it leaves open, the web included", states the
two-outlet hold, and asks for "the one thing the frame lacks" on every Cast.
Texture then casts research on almost every new story, and each Cast is a run.

The other candidate: the stop rule and the removed "read at least 8 independent
sources" target (role prompt v21) leave rounds ending without a final Report
(19 runs, 14 stories), and an unfinished round is followed by another Cast.
The two are not exclusive.

## Remaining error

- No console from before the deploy survives, so the before figures come from
  hourly counts, not from the runs themselves.
- No Cast objective text was read; the Texture mechanism is inferred from the
  prompt diff and the 41-of-48 count.
- The 55% is a hypothesis. The 15-minute step coincides with the deploy but a
  second change of the same release (the platform edges-index deadline,
  `6229cd73a`) was not ruled out as a cause; it is not on the research path.
- The run ledger was not read for any of the 19 runs that ended without a
  Report.

## Action taken

Revert of the five commits' effect (prompts, code, tests, status script). The
prompt files are numbered role v22 and Texture v5 with the old text, since a
version label only rises. Rationale for the earlier change stays in
`research-runs-send-reworded-web-searches-2026-11-22.md`, whose evidence of
reworded searches still holds.

## What a rework must show before it lands again

1. First-round research Casts per opened story, measured on staging traces
   before and after, from the Cast records themselves: the rate after must not
   exceed the rate before by more than noise (before the 2026-11-23 deploy:
   well under 41 of 48).
2. Research runs an hour against the 13.2 baseline over at least 6 hours, with
   captures, management calls and Texture runs flat.
3. Rounds ended without a final Report, per story, no higher than before.
4. A read of the Cast objective texts and of the run ledgers of at least the 19
   unfinished runs, so the mechanism is known rather than guessed.
5. Texture's prompt and the research prompt changed in separate landings, so a
   step in the hourly counts is attributable.

## After the revert (read 2026-11-23 18:05Z)

The revert (`771fcc088`) deployed at 15:30Z; from that minute every research
call carries the old system prompt (30,391 characters against 33,278 under the
change, which ran from 08:02Z to 15:30Z).

| UTC hour | runs active | runs started | calls | median calls a run |
| --- | --- | --- | --- | --- |
| 06Z, 07Z (before) | 16, 13 | 15, 12 | 394, 284 | 22, 19 |
| 08Z to 14Z (changed prompts) | 39 to 82 | 36 to 77 | 668 to 1138 | 9 to 11 |
| 16Z | 44 | 39 | 626 | 10 |
| 17Z | 12 | 11 | 578 | 24 |

The prediction (claim C201: 25 or fewer in each of the first two full hours)
was **false as written**: 16Z had 44. From 17Z the count and the shape of a
run are back where they were. Belief: the change was the cause, at about 0.8;
the hour of lag is not explained (stories opened under the changed prompts
still working through their rounds is a guess, not read from a run ledger).
Remaining error: two hours is thin, and before the change two hours of
twelve passed 25 on their own (32 and 34 runs active). Claim C202 covers 18Z to 21Z.
Two runs in 17Z made 160 and 142 model calls each; runs of 72 and 108 were
seen before and during the change too, and none has been read step by step.

## Four more hours (read 2026-11-23 21:10Z)

Research runs that made a model call: 29 in 18Z, 24 in 19Z, 34 in 20Z. Claim
C202 (none of 18Z to 21Z over 25) is false. The line ignored the time of
day: 20Z on 11-22, before the change, had 32. Against the same daytime hours
under the changed prompts (39 to 82), the revert about halved runs. Since the
revert the hours read 44, 12, 29, 24, 34.

Belief now: the change raised runs, at about 0.8, by something like a factor
of two in daytime hours, not four. The "13 to 62" in this record's title
compared overnight hours before with daytime hours after, so it overstated
the effect. What is still not known: the mechanism (no run ledger read), and
a like-for-like baseline, which needs the same hours of a weekday without the
change. Tuesday 11-24, 08Z to 14Z, is the first such window.

