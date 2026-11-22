# The offline slice, rung 0: the rules find the names and not what happened (2026-11-22)

Station F1/F2 of [the first design](../definitions/choir-first-design-2026-11-22.md);
the plan is [first-design-offline-slice-plan-2026-11-22.md](../design/first-design-offline-slice-plan-2026-11-22.md);
ledger C192 (still open: this rung measures one of its four lines). No model
call, no Jev call, no spend; nothing written on Node B.

## What was run

The harness on `lane/offline-slice-r0` (`1afb66e7f`, not landed) replayed the
paper's captures of 2026-11-15 to 2026-11-22 in arrival order and listed, for
each, the candidate subjects the rules alone propose: (a) known labels and
aliases in the text, from entities created before that day; (b) names the
deployed extractor proposes that are not yet known; (c) the subjects of the
ten nearest earlier captures by vector. It calls the deployed candidate code
(`capture_ready.go`, `entitytext`) as a library. Read path: corpusd's `graph`
database read-only, and capture text from files on Node B by a privileged
read that writes nothing (as `scripts/status-storage` does).

## Numbers

- 72,993 captures in the week; 5,402 repeats of one URL; 67,591 replayed.
  **65% are pages research read during its own runs** (`web:read`), 26%
  Mastodon tag feeds, 5% RSS.
- Candidates a capture: median 28, mean 31, 90th percentile 55. No candidate
  at all: 1.1%.
- Names first seen, per 1,000 captures: 3,720 on day 1, 1,707 on day 7. Days 4
  to 7 over days 1 and 2 is about 0.57.
- The most-proposed new names are page furniture (Getty Images, Updated,
  Share, Advertisement, Read More); the most-found known entities are junk or
  outlets (News in 16,147 captures; Arab News, Antara, ITV in 12,000 to
  13,000 each).

## The hand read

A Sonnet agent that did not build the harness and did not see the sources of
the candidates read 100 captures drawn uniformly from those with a mention,
named each one's main subjects before opening its candidate list, and marked
each as in the list or absent.

| | all with a subject (94) | news only (53) |
|---|---|---|
| every main subject in the list | 16 | 12 |
| at least one in the list | 86 | 52 |
| none in the list | 8 | 1 |

- Six of the 100 had no subject (a 404 page, a bot check, menus, indexes).
- **92 captures have a main subject that is not a proper name** (a patrol, a
  port closure, a recall). For 19 of them something in the list corresponds;
  the reader calls that count soft.
- By the reader's estimate over ten lists, half to three quarters of a list
  is junk or unrelated to the capture.

## Reading (the lead's)

- **Kill line (c) was written loosely** ("the right subject was among the
  candidates"). Read as at least one main subject, it passes (86 of 94; 52 of
  53 for news). Read as every main subject, it fails (16 of 94). The line is
  not settled by choosing the reading afterwards; it is rewritten below
  before the next rung.
- What the numbers say without a threshold: **the rules find the actors and
  not the happening.** Nearly every capture is about something that happened,
  and a name-based rule proposes it for about one in five. The panel and the
  read of story pairs earlier the same day pointed here.
- The candidate lists are too long and too dirty to put to System 1 as they
  are: a median of 28, most of it page furniture and outlet names.
- Two thirds of "captures" are not arrivals. The hot path is for what sources
  bring, not for pages a desk fetched for itself.

## What changes

1. The hot path runs on source arrivals, not on research's own reads.
2. Candidate text is the article, not the page: strip navigation and
   furniture before the rules run; outlet names and a capture's own byline
   are not subjects.
3. **Subjects that are happenings do not come from rules over capture text.**
   They come from stories: a story's Texture names what it is about, and a
   capture is offered the subjects of the nearest stories by vector. The next
   no-spend rung tests exactly that: for these same 100 captures, is the
   happening among the main points of the ten nearest stories that existed
   before the capture arrived?
4. Kill line (c), rewritten before that rung: for 80 or more of 100 news
   arrivals, the candidates include at least one actor and the happening (or
   the reader marks the happening as new, with no earlier story about it),
   in a list of 12 or fewer.

## Limits

One reader, one pass; the sample is not stratified by source; neighbours used
final vectors; alias times are approximated by day; wire copies under
different URLs count separately.

## Rung 0b, the same day: stories as the source of happenings

Harness `lane/offline-slice-r0` at `495b4fe38`; no model call, no spend.

- **Source arrivals only:** 23,851 in the week (about 3,400 a day) of 67,591
  captures: 19,278 social, 3,876 RSS, 500 arXiv, 197 official. The rest were
  research's own page reads.
- **Cleaned text:** the candidate list from rules fell from a median of 12 to
  4; the most-proposed are real actors (us, trump, ai, ukraine, russia, iran)
  with some feed furniture left (computer science, comments url, abstract).
- **The oracle, no reader:** of 2,282 arrivals later cited by a story that
  existed before they arrived, that story was among the K nearest earlier
  stories by vector for 62.4% at K=5, 72.6% at K=10, 82.1% at K=20; the
  control (the K most recently revised stories) gives 9.2%, 19.5%, 34.9%. By
  class at K=10: RSS 78.3%, social 70.2%. It falls on the last two days (65%,
  56%), when citations are still arriving.
- 20,367 arrivals are cited by no story; 1,202 only by a story first published
  after they arrived. "Cited" stands in for the paper's feed decision, which
  is in the paper computer's own store and was not read.
- **The hand read** (a Sonnet agent, 100 news-like arrivals from days 4 to 7,
  actors and happening written before opening the list; lists capped at 12,
  five nearest stories first):

| | all 100 | the 75 that report a happening |
|---|---|---|
| at least one actor in the list | 88 | 71 |
| the happening in the list (a listed story is about this affair) | 30 | 29 |
| the happening is new (no listed story covers it) | 46 | 46 |
| neither actor nor happening | 11 | 4 |

  Median 5 of a list's 10 or 11 entries unrelated. 25 of the 100 report no
  happening (chatter, opinion, promotion).

### Reading (the lead's)

- The rewritten kill line (c) passes at 71 of 75, and it passes too easily:
  46 of those 71 pass because the reader marked the happening as new. What it
  shows is that an actor is nearly always offered; the reader cannot tell a
  story the list missed from a story that does not exist.
- The oracle is the better evidence: when the paper did have the story, the
  ten nearest stories by vector contained it about three times in four, at no
  model cost, against one in five for the control.
- **Most of what arrives is about something the paper has no story for:** 46
  of 75 by the read, and 20,367 of 23,851 arrivals cited by nothing. So "none
  of these" is the common answer, not the exception. If every such capture
  escalated to a language model, kill line (a) (escalation at or under 30%)
  would fail by design.
- A quarter of news-like arrivals report nothing; they need no subject.

### What changes

1. A capture that matches no story does **not** escalate. It is held with its
   actors and its vector. Unmatched captures are grouped by vector; a group
   reported by several distinct origins is what the root is woken by ("a
   subject that matters and has no story"), and a model names it once, for
   the group. Escalation is per group that crosses the line, not per capture.
2. The question to System 1 gets a cheap first answer: does this report
   something that happened at all.
3. The feed decision should be written where the host may read it, so the
   oracle is the real decision and not a citation proxy (F0's logging is the
   start).
4. Next rung (no spend): group the week's unmatched arrivals by vector and
   count groups by distinct origin, to see how many a day would reach the
   root at thresholds of 2, 3 and 5 origins, and hand-read 50 groups for
   whether each is one happening.

## Rung 0c, the same day: unmatched arrivals grouped by vector

Harness `lane/offline-slice-r0` at `b327f61ef`; no model call, no spend.

- **Unmatched** = nearest earlier story below cosine 0.795 (chosen on days 1
  to 3; it wrongly calls 20.2% of arrivals a story later used unmatched).
  53 to 59% of arrivals a day are unmatched; 7,519 on days 4 to 7.
- **Grouping** in arrival order, centroid cosine at least 0.84 (the value with
  the most multi-member groups on days 1 to 3; the knee criterion gave one
  group of 1,652 and was rejected), closing after 48 h quiet. Outlets by
  `internal/outlet.Of`, wire copies by the event layer's `wireOrigin`.
- **Wakes a day** (the day a group's Nth distinct outlet arrived): 2 outlets
  51 to 92; 3 outlets 13 to 35; 5 outlets 1 to 10. 58.2% of unmatched arrivals
  end in groups that never reach 2 outlets.
- **Cross-check, 70 groups reaching 3 outlets on days 4 to 7:** 26 later got a
  new story citing a member (median 9.3 h after the group opened); 2 only an
  existing story; 42 none (citations lag, so an overcount).
- **The hand read** (a Sonnet agent, 50 of those 70): ONE happening 37, MOSTLY
  one 4, THEME 5, NOISE 4. News a general paper would cover: yes 32, marginal
  14, no 4. Of the 41 ONE or MOSTLY, 38 are reports from different newsrooms;
  3 are one text re-posted. Bad groups: an aggregator feed (1,635 members
  behind the 12 shown), donation appeals, contract news sharing one word.

### Reading (the lead's)

Four in five groups that reach three outlets are one happening, and about two
in three are plainly news. At about 20 such groups a day, naming them costs
about 20 model calls a day, against one per unmatched arrival (about 1,900 a
day). The paper opened a story on at least 26 of 70, a median of nine hours
after the group formed. The defects are chaining (a group that keeps
absorbing) and aggregator accounts counted as outlets; both are bounded by a
cap on group age and size and by collapsing re-post accounts to the outlet
they relay. This supports the design change above. It does not test System 1
or the model that names a group: that needs spend (decision Q5).

## Rung 1: System 1 scores each arrival against its nearest stories (2026-11-22)

Harness `lane/offline-slice-r0`; the owner granted spend the same day. Inputs: the week's 23,851 source
arrivals with their five nearest earlier stories (rung 0b). Estimate $3 for both rungs; actual $0.94.

**Method.** One System 1 question an arrival, type `choice`: which of its five stories reports the same
affair, or `other` (an affair none covers), or `idk`. Model `~typesafe/jev-latest` through OpenRouter's
decisions endpoint, as the gateway calls it. One call carries as many arrivals as the vendor's limits allow
(`internal/jevbudget`, 64k and 32k tokens): 432 calls of about 55 arrivals, with the stories shared inside a
call. A second run of 1,200 arrivals of days 4 to 7 at 12 an arrival a call (100 calls) is the same-captures
comparison. Keys: `scripts/offline-slice/jev-run.py` and `river-run.py` copy the loader of
`scripts/event-blind-sample.py` (the environment, else the one named line of the repo `.env`); no key is
printed or stored. Disclosure: the first 526 of the 534 calls of this rung were sent before the owner's
constraint on `.env` arrived and used that loader; the lead then ruled the loader allowed.

**Numbers.**
- 432 calls, 23,851 arrivals answered, none missing, 19.6M input tokens, 1.9M output, **$0.8233** (about
  $0.035 per 1,000 arrivals); 0.53 s a call, about 10 ms an arrival. The 12-a-call run: $0.0442 for 1,200
  arrivals ($0.037 per 1,000), 19 ms an arrival.
- Answers: story 50.3%, other 47.7%, idk 2.0%. **Escalated (other or idk): 49.7% over the week, 50.1% on days 4
  to 7** (per day 47.5% to 52.7%). 32% of the story picks have confidence under 0.5.
- Against the oracle, days 4 to 7: of 1,222 arrivals later cited by a story that existed, the cited story was
  among the five offered for 57.6%; System 1 picked it for 421 (34.5% of those, 59.8% of the reachable), picked
  another story for 639, answered other or idk for 162. Precision of picks among the cited: 39.7%. Of 635
  arrivals cited only by a story first published after them, 60.6% picked an existing story. Of the 12,091 that
  no story cites, 45.7% picked a story.
- The oracle undercounts. "Picked another story" is often a second story about the same affair (the hand read
  below: the US measles status has four stories, the Trump-Putin-Xi summit about eight); the cited story is
  one of several.
- Batching: on the same 1,200 arrivals the verdict agrees in 1,089 (90.8%); both pick a story for 538 and the
  same one for 447 (83%); of 104 cited arrivals, 39 right at 55 a call and 43 at 12 a call. Batch size changes
  little.

**Hand read of 30 answers** (uniform from days 4 to 7, seed 20261126; read by the author of the harness):
27 agree, 1 disagrees, 2 borderline. The disagreement: a mass-drone-alert post matched with confidence 0.12 to
one night's attack story that is probably another night. Borderline: two picks of a summit story for a
different report of the summit (both defensible). Eighteen of the 30 were "other", all correctly: a different
earthquake, a different company's earnings, a different ransomware victim, a different sentencing.

## Rung 2: naming groups of unmatched arrivals, and kill line (d)

**Grouping.** The rung 0c rule (T 0.795 from days 1 to 3; centroid; G 0.84) with chaining bounded (a group
takes no member once 48 hours old or at 25 members) and relay accounts collapsed (a post whose first hashtag
names an outlet counts as that outlet; 961 arrivals changed origin). Capped, the largest group is 25 (105 hit
the cap); uncapped it was 1,647. 7,687 groups; 165 reach three distinct origins in the week, 80 of them on days
4 to 7. Groups opened per 1,000 arrivals: 336.8 on days 1 and 2, 316.2 on days 4 to 7. Groups reaching 2, 3, 5
origins within 24 hours of opening, days 4 to 7: 217, 47, 11 (within 6 hours: 133, 21, 5; within 48: 254, 63,
19). 80.3% of days 4 to 7 unmatched arrivals sit in groups that never reach two origins. The groups that reach
three on days 4 to 7 hold 705 member arrivals (5% of the arrivals).

**Naming.** River, `deepseek-ai/DeepSeek-V4.1-Flash-1017`, reasoning high, temperature 0, 25 groups to a call
(`scripts/offline-slice/river-run.py`): kind (ONE, MOSTLY_ONE, THEME, NOISE), a one-line main point, and whether
a general newspaper would cover it. 9 calls for 215 groups; about $0.05 at uncached prices (95k tokens out,
mostly reasoning).
- The same 50 groups the reader hand-read (ONE 37, MOSTLY 4, THEME 5, NOISE 4): the model gives ONE 37,
  MOSTLY 4, THEME 7, NOISE 2. Totals only; per-group agreement is not known.
- The 165 groups: ONE 93, MOSTLY 21, THEME 28, NOISE 23; news yes 126, marginal 14, no 25. Of the 80 reaching
  three on days 4 to 7: ONE 47, MOSTLY 8, THEME 12, NOISE 13.
- The paper: of those 80, 35 had a story cite a member (32 first published after the group opened, median 11.6
  hours later, p25 3.2, p75 25.9; 3 existing) and 45 had none. Of the model's 55 ONE or MOSTLY groups, the paper
  made a story for 19 after the group opened; 35 of the 55 had none. Of the 32 groups the paper opened a story
  for, the model calls 19 ONE or MOSTLY and 13 THEME or NOISE (feeds of one account, live-blog rolls).
- **Hand read of 30 group names** (uniform from the 80, seed 20261128; by the harness's author): 27 agree. Two
  disagreements: a Le Monde live blog of several different happenings of 17 November named ONE (I read THEME);
  a mixed lot of video links with a job advert named THEME (I read NOISE). One borderline: the Berlin coalition
  meeting and the Shenzhen summit as THEME.

**Kill line (d), offline.** 50 stories drawn at random from those last revised on days 2 and 3 (seed
20261127), as they stood at the end of day 3; River wrote 3 predictions each (150, mean stated probability
0.131); the prediction was asked as a System 1 noul question of each days 4 to 7 arrival that was offered the
story among its five nearest (1,848 pairs, 16 calls, $0.008), and settled true if any answer was at least 0.5.
4 of 150 settled true (base rate 0.027). Brier 0.0307 against 0.0260 for the base rate: **skill -0.18, 95%
bootstrap interval over stories [-2.19, 0.22]**. The interval includes zero and the point is below it.

**Kill lines of ledger claim C192.**

| Line | Result | Met |
|---|---|---|
| (a) escalation at or under 30% over days 4 to 7 | System 1 answers other or idk for 50.1% of arrivals of days 4 to 7 (per arrival). Per group, the model is asked about 80 groups holding 5% of the arrivals | **not met** as written (per arrival); the redesigned escalation (per group) is 5% of arrivals |
| (b) new subjects per 1,000 under 70% of days 1 and 2 | groups opened per 1,000 arrivals 316.2 on days 4 to 7 against 336.8: 0.94 (the 0b name proxy was 0.57, but proposals are not subjects) | **not met** |
| (c) the right subject among the candidates for 80 of 100 hand-read | the 0b reader: an actor in the list for 71 of the 100 (71 of the 75 that report a happening); the happening for 29, new for 46 | **not met** (71 under 80; 95% of the 75) |
| (d) prediction skill above zero with its interval | -0.18, interval [-2.19, 0.22], 4 of 150 settled true | **not met** (interval includes zero; underpowered) |

C192 is false on all four lines by these measures. What the slice does show: for an arrival whose story exists
the nearest five hold it (72.6% at K=10 against 19.5% for recency) and System 1 picks it for 60% of those
reachable; about half of arrivals match no story; a handful of groups a day (about 20 on days 4 to 7 reach three
origins) are what a root would be woken by; the model names them correctly in the cases read (27 of 30); the
paper made a story for about a third of them, a median 11.6 hours after the group formed.

**Cost.** Jev $0.8765 (550 calls); River about $0.06 (12 calls, 116k tokens in, 95k out). Total about $0.94
against the $3 estimate.

**Unverified.** The oracle is citation, not the paper's feed decision. The hand reads are one reader who built
the harness, 30 each. The five stories offered are the top five by vector, in that order (Jev leans to the
first option). The group names are totals against the hand read, not per group. The settlement of predictions
reaches only arrivals that were offered the story; the base rate is the pooled outcome rate (hindsight, so
conservative); 4 settled-true cases cannot carry a skill estimate. Day 1 has no document-frequency statistics.
