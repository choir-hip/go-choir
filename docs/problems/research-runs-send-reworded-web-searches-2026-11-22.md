# Research runs send reworded web searches because their prompts ask for sources the frame does not need

Recorded 2026-11-22 from the traces of 20:04Z to 23:04Z, three hours after
graph-first (`3fc71ee89`) went live. Read-only investigation; the run ledger
was not read, so a run's own reason for a search is not known (see "Remaining
error").

## Problem

Research runs that search the web send too many searches, and most of the
excess is the same question reworded. Graph-first made the graph the first
step; it did not change how many web searches follow.

## Evidence

Window 20:04Z to 23:04Z: 55 research runs, 157 web searches (gateway journal),
2.85 per run over all runs, 4.24 over the 37 runs that searched.

- Hand-labelled searches of seven runs (77 searches): 50 were reformulations of
  an earlier query of the same run with no new information, 22 were for a thing
  the corpus lacked, 1 for a thing it held, 1 verification, 3 other.
- By timing alone, over all 157: 48 (31%) were sent with no read since the
  previous search and with overlapping words or within 15 seconds; 33 (21%)
  shared at least 60% of their words with an earlier query of the run.
- The five heaviest runs sent 72 of 157 searches (46%). One run (7dd12fab) sent
  23 in 14 minutes, cycling `site:` and quoted variants of one French wording
  over six outlets. One (d56a491a) sent 21 in the window, 19 of them naming a
  venue ("Ultimo's") that appears in the Cast's third thread and in no source of
  the digest.
- Ten runs whose Cast objective said corroborate, independent reporting or
  confirm, and named no page to fetch, sent 74 searches (47% of all, mean 7.4).
  Runs whose Cast named a page to fetch and did not ask to corroborate: 9 runs,
  13 searches (mean 1.4). The objective's wording predicts volume better than
  the graph-first text does.
- The prompts carried three source counts at once: "read at least 8
  independent sources across distinct hosts" for a survey question
  (`research.yaml`, 2026-11-09), "a second independent outlet ... do not search
  again" (role prompt and overlay), and "one report of an event is enough"
  (Texture). The platform's real gate is `MinPublishOutlets = 2`
  (`story_publish.go`); nothing told the run that the second outlet may come
  from the graph's event record.
- The worked cell sent a web search whenever the top five graph hits gave fewer
  than two outlets (`if len(outlets) < 2 { SearchAll(...) }`), so one capture of
  one outlet, the normal case for a new story, sent the web.
- `SearchWireCorpus` returned captures in 224 of 224 searches: it falls back to
  the nearest by meaning. The run could not tell "the graph lacks this" from
  "the graph holds ten pages".
- The gap measure was a template echo: the worked cell wrote the literal
  `missing: what the graph lacked` and the log counted any line starting with
  `missing:`, so 55 of 56 Reports (8 of 8 corpus-only ones) "named a gap".
- `held` in web results means an earlier search of this run returned the page;
  the prompts put it beside graph-first text, and `SearchHit.CaptureID`, said
  to mark a page the corpus holds, was set nowhere. 23 of 343 first-time page
  fetches (6.7%) were pages the corpus already held.
- The first user turn did not mention the digest, and told the run to send an
  interim packet "as soon as you have read one new source you can cite, and
  another each time you have read more".

## Belief

The volume is the caller's understanding, not a missing limit. A run told to
corroborate, to read eight sources, and that a detail unconfirmed is still a
task, keeps searching until one of those is met; the web often lacks the detail
(a venue named only in the Cast), so the loop does not end. Texture's Cast
supplies the corroborate wording and, once, a detail from memory. The fixes
are text and measurement: the Cast asks for the one thing the frame lacks, one
consistent count, a stop rule that makes an unconfirmed detail a finding, a
worked cell that goes to the web only for a lack the run names, a corpus
search that says whether any page matched the words, and a gap figure that
means something. The aim is not fewer web searches as such (owner, 2026-11-23: "the point isnt to
cut search to zero ... we need to scale corpus utility"): a run still goes to
the web whenever the corpus does not hold the answer. What the change removes is
rewording the same query against pages the run already holds, and what it adds
is a corpus the run can read honestly (the two counts, the held mark's real
meaning, one source rule, a gap figure that means something). It is judged by
`scripts/status-search-reads` on a window of 50 or more runs with the
no-objective runs segmented: the share of searches sent with no read since the
previous one and with overlapping words, the heaviest-five share, the digest's
and the graph's pages read by capture id, and the real gaps tabulated; the mean
over all runs is reported, not targeted.

## Remaining error

- The run ledger was not read: why a run sent each search is inferred from the
  order of searches, reads and the Cast's wording, not from the run's own
  narration. The 77 hand labels are one reader's.
- The text of the Reports' `missing:` lines was not logged until this change;
  the gap table of the investigation rests on objectives and queries.
- The domains of the pages read are not logged.
- 21 of the 55 runs (22 searches) were woken by a record with no objective
  (the management-replies path, `research-desks-woken-by-management-replies-loop-2026-11-20.md`);
  they are in the denominator and are not addressed by this change.
- The 14 of 34 Cast objectives that ask to fetch a page a lead links to are a
  corpus gap (the page is not captured at intake); not addressed here.

## Not in this change (named residuals)

1. Waking a research run on a record with no objective: needs its own
   investigation of the management-replies path.
2. Capturing the page a lead links to at intake.
3. Logging the domains of the pages read.
