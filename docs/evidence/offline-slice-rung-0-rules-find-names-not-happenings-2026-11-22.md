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
