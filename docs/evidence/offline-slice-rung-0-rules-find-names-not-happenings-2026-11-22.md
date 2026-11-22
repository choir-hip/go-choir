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
