# The day's top story was published and ranked near the bottom

2026-11-23. Found by the owner, who opened the paper and did not see that
Jensen Huang was back at Nvidia.

## Problem

The paper's sense of what matters has no memory. A story's importance is one
model answer about its title and first paragraph, asked again from nothing at
every revision, and a turn in a running story is written into the old story.
The day's largest story can be complete, well sourced and invisible.

## Evidence

Read 2026-11-23 19:16Z to 19:25Z.

- The story existed: "Jensen Huang reinstated as Nvidia CEO after
  petition-backed campaign", publication ending `429dfbf2d`, 63 sources, 1,846
  words, revision 68, updated 19:13Z. It was tenth of sixty in
  `/api/platform/publications/front`, which orders by update time.
- Its importance was 3 of 10, and the score was fresh: the row in
  `/var/lib/go-choir/front-importance/scores.json` on Node B reads revision
  68, scored 19:17:10Z. A second story on the same event, two sources and 569
  words, had 6. Stories that hour carried 8 and 9.
- The scorer (`internal/frontimportance/scorer.go`) gives the model the title
  (300 characters) and the dek (the first paragraph, 240 runes). It does not
  see the sources, the count of them, the story's history or the stories
  around it. Its prompt says a company's own announcement scores 1 to 3.
- The story's address still names the petition: it was opened two weeks ago
  as "Petition ... demands Jensen Huang return as Nvidia CEO" and fed since.
  The management rule (`rlm_management_task_runtime.yaml`) says a story stops
  growing when its main beat changes, and also "when unclear, feed". The main
  point is written once at open and never changed.
- Captures were not backed up: 196, 227 and 86 judged in 17Z, 18Z and the
  first quarter of 19Z (`scripts/status-story-choice`).
- The web page and the phone multiply importance by depth and a 24-hour decay
  (`frontend/src/lib/autopaper.ts`), so a 3 sinks under a day-old 8. The voice
  ask weights by it too.

## Belief

Two mechanisms, both by design, at about 0.85:

1. Importance is not state. It is a one-question model judgment of the
   current wording. Nothing accumulates: not distinct sources, not arrivals
   over time, not other stories on the subject, not what readers did.
2. A changed beat was fed instead of opening a successor, so the largest
   development sat under a two-week-old address with a main point that no
   longer described it.

The lead first told the owner the score was stale, carried from the petition
story. That was wrong and said before reading the row.

## Remaining error

- Why the judge gave 3 is inferred from the prompt's company-announcement
  rule; the answer was not replayed.
- When the reinstatement first reached the paper is not read. The second
  story puts it at 18:19Z or before.
- Whether throughput is down against last week, as the owner believes, is
  not measured: the console lines the count needs reach back only hours.
- No document says how the front should be ordered, and nothing measures
  whether the paper is current against the day's real news. Both are missing
  designs, not missing code.

## Not a fix

Rewording the scoring prompt would move this story and leave the mechanism.
The design work is gathered in
[the letter of 2026-11-23](../reports/five-days-and-where-we-stand-2026-11-23.md);
the owner's rulings come first: lean to revise or open a successor on a
changed beat; which of three definitions of importance; whether the first
design continues after its failed trial.
