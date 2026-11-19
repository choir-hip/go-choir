# Search volume outruns the Keenable credits (2026-11-18)

## Problem

The owner looked at Keenable's usage page on 2026-11-18 and found far more
use than expected: 82,797 requests in fourteen days, 5,914 a day on average,
10,572 on the busiest day (November 17). About 17,000 credits remain and the
plan's month ends on December 5. At the current rate they last two to three
days.

Nobody was watching the number. The hourly status prints the hour's search
count and flags only searches that got no answer, so a provider being spent
four times faster than its plan allows raised no flag.

## Evidence

- Keenable usage page, 14 days to 2026-11-18 (owner's screenshot): 82,641
  searches, 156 fetches. Daily bars rise from about 1,000 on November 5 to
  10,572 on November 17.
- `scripts/status-hourly`, 2026-11-18T12:38Z: 721 search requests in the
  last hour, 719 answered, 670 by Keenable. Brave, Parallel and Tavily are
  `quota_limited` (672, 285 and 672 skips in the hour), Serper is in error
  (525). SearXNG answers nothing. Keenable is the only provider answering.
- The search plane treats Keenable as free and uncapped
  (`internal/gateway/searchplane/config.go`, `DefaultProviderTiers`; owner,
  2026-11-08: Keenable is not paced against a monthly allowance). That was
  right when the allowance was not close.

## Belief

Two separate things are wrong.

1. **Supply.** One provider carries every search. The owner has added
   Ceramic (81,000 credits; [reference](../reference/CERAMIC.md)) and wants
   Keenable to last to December 5. Arithmetic: 17 days at today's volume
   (about 10,000 a day) is about 170,000 searches; Keenable plus Ceramic is
   about 98,000. A second provider alone does not reach December 5. It
   reaches about November 27.
2. **Demand.** Ten thousand searches a day for one newspaper is not
   explained. Volume rose tenfold in two weeks while the paper's output did
   not. Earlier work found most searches were repeats
   (`docs/archive/reports/search-efficiency-and-reuse-2026-10-14.md`) and
   added a cache; whether it still catches them at this volume is not known.

## Investigation, 2026-11-18 (read-only; gateway journal, 41 hours, 19,882 searches)

- **One caller.** 99.95% of searches come from one computer, the paper's.
- **Not repeats.** 94% of queries are distinct. Exact repeats are 6.5% and
  the cache already catches nearly all of them (1,251 hits of 1,298). The
  earlier diagnosis (repeats) is no longer the cause, and the cache is not
  the leak.
- **Many wordings of one thing.** 185 topics had twenty or more searches
  each and make 42% of the volume. One small local story drew 260 searches
  in eleven minutes. The research prompt
  (`internal/promptstore/defaults/research.yaml`) asks for "several searches
  per thread in one cell, with different wording", every round, and each
  interim packet starts another round.
- **Diminishing returns are visible and unused.** 53% of the URLs returned
  had already been returned that day. 29% of searches returned at least 80%
  already-seen URLs; 9% returned nothing new. Nothing tells the caller.
- **Why it rose.** The journal reaches back only 41 hours. Commit
  `279cba57d` (November 8) removed Keenable's monthly pace, which had been
  holding searches at zero results; the other providers then ran out of
  quota, so all demand landed on Keenable. Research changes of November 9
  (interim reports, wider intake) raised demand too. The split between the
  two is not known.
- **What Ceramic can take.** Of the paper's queries, about 78% are English
  or names, 18% are in another Latin-script language, 4% are in a non-Latin
  script. Queries are short and keyword-like (median seven words), which
  suits a lexical engine. Ceramic indexes English pages only, so about a
  fifth of searches (roughly 2,400 a day) still need Keenable, against a
  budget of about 880 a day to reach December 5.
- **Small defects seen:** four searches for the bare query `x` in one
  second; every search asks for 40 results.
- **Cannot be told from the logs:** which desk, run or story a search
  belongs to. The search log line needs the run id.

So demand has to fall whatever the providers are. The fix belongs in the
caller: tell the research desk what a search added, so it stops rewording a
question whose answers it already holds.

## Design, 2026-11-18 (replayed on the same 41 hours; owner: "halve search volume and keep or improve quality... duplicate less and reuse captured results more")

- **Searches are fired blind.** 87% share a second with another search:
  one cell fires many wordings at once, so nothing a search returns can stop
  the next. One story's burst ran 214 searches; its first three returned
  118 new pages and the next dozen returned almost none.
- **The desk is never told what it holds.** Each result set is kept for the
  session with titles and snippets (`internal/yaegikernel/search_store.go`),
  but nothing compares a new result against it; the only feedback counts
  provider arrivals, not new pages.
- **Reuse across runs is the smaller part.** 95% of mostly-held results fall
  inside the same half hour. A topic revisited later finds real news (21 new
  pages of 40 on average).
- Replayed savings: answering a reordered or re-quoted query from held
  results, 12% (23% with looser matching); one call that takes all the
  wordings of a question, sends them in turn and stops when they stop
  finding new pages, 23%; both with a stop between cells, 44% (about 5,000
  a day), at the cost of up to 30% of first-seen pages, most of them the
  tail of a 40-result list. Whether the desk would have read any of those
  pages cannot be told: the logs do not say which run a search belongs to
  or which pages were read.

## The owner's principle (2026-11-18): research on the corpus, search only the diff

"Are we correctly searching the corpus? More stories should be citing
existing stories and captured sources. Need to check for story duplication,
but not only duplication. Too much story overlap is wasteful."

"remember that we can add on to existing stories rather than make new
stories. So rather than making a new story that has 80% overlap with an
existing story, we should revise that existing story."

"The point is that research should substantially be upon our existing
corpus, because as the corpus grows, the likelihood that relevant sources in
the corpus exist for any new bit of information is very high. And then the
search should be all on the diff between the existing corpus and the world."

This changes what the fix is. The design below (mark results new or held,
reuse within half an hour, stop rewording) makes each web search cheaper to
waste. The principle says most of them should not be asked: a research run
starts from what the computer holds (the existing story on the event, the
related stories, the captured sources and their text), states what is
missing or stale, and searches the web only for that. What a search brings
back is captured, so the next run holds it. A story that would overlap an
existing one mostly becomes a revision of it.

Corrected the same day. The lead first wrote that the choice between a new
story and a revision is "decided before research starts". The owner: "it
doesn't really make sense to, before research starts, revise the story.
Like, how do you know you should revise it versus making a new story before
research starts?" The choice needs what research finds. Where in the
pipeline it belongs is open.

The owner then widened the question: "I'm realizing this is a bigger design
and architecture thing than I had initially thought. And we should audit how
our graph traversal and BM25 search is currently working. And now may be the
time where we should start looking into doing embeddings and vector search
over our corpus. I think we can use embedding Gemma 2 and run all that in
x86. But we should do a feasibility study for sure." Two read-only studies
were started on that: an audit of the corpus search as built (BM25, graph
traversal, whether research runs use them and what they return), and a
feasibility study of embeddings and vector search on the x86 hardware we
have. No design is settled until both report.

Not yet known, under investigation the same day: what the corpus search
indexes and whether captured source text is findable at all; whether research
runs call it and what it returns; how many published stories overlap an
earlier one at 80% or more; where the pipeline decides new story or
revision.

## Ceramic measured before landing: it cannot carry the paper (2026-11-18)

Live calls ([reference](../reference/CERAMIC.md), "Measured") show Ceramic's
index holds no recent news: for three events of the last three weeks the
newest page it returned was months to two years old, and it always returned
a full page. Routed first, it would have replaced fresh results with stale
ones and the short-answer escalation to Keenable would never have fired.
The provider is built and reviewed-ready in `lane/search-ceramic` and is
**not landed**; claim C128 is void. Supply is therefore unchanged: Keenable
alone, about 15,000 credits, about a day and a half at today's rate. What
moves the date now is demand (lane `search-reuse`) and whatever the owner
chooses to buy.

## What is being done

- (Withdrawn, see above.) Ceramic first, Keenable on a short answer under a
  daily cap. Lane `search-ceramic`, claim C128 void.
- Lane `search-reuse`, claim C129: the search log line carries the run and
  desk; results say which pages are new and which are held; a query that
  only reorders an earlier one is answered from held results; one call
  takes all the wordings of a question and stops when they stop yielding.

## Built, 2026-11-18 (lane `search-reuse`, three commits; not yet on main)

- Attribution: the search request line carries `run=`, `desk=`, `conv=`; the
  served line ends `urls=[...]`, hashes of the first ten result URLs; one
  `search run=...` line when a run ends (searches started, from held, not
  sent by reason, pages returned, new, read). Counts and ids only.
- The desk is told: pages marked new, held or read; "N of M pages new" in the
  cell result; a query with the same terms (`internal/searchquery.Key`) as a
  search of the same run in the last 30 minutes is answered from it with no
  call; the gateway cache keeps its own key. A query with no word of two
  letters or digits is refused. What sends the bare `x` is not found: the only
  host path to the gateway is the desk's `choir.SearchStart`, and nothing in
  the repo sends `x`, so it was a cell the model wrote.
- Log lines for the corpus audit
  (`research-does-not-start-from-the-corpus-2026-11-18`): the `corpus search`
  line ends `run= desk= conv= published= events= capped=`; a
  `corpus read` line per Read and ReadCapture (run, capture id, held, verb); a
  `management story choice` line per management record sent to a story (run,
  feed or open, the fed story's id, candidate stories shown). Ids and counts.
- Ask once for many: `choir.SearchAll`, and research prompt v15. After the
  two-seat review of 2026-11-18: the normaliser keeps years, dates, quoted
  phrases and capitalised acronyms as terms; `fresh` sends a held search; a
  running match is waited on and a failed or empty one is not reused; news
  queries reuse for 10 minutes; a failed wording is a failure, not an empty
  result. Residual: the search store holds every search of a session and is
  not pruned.
- Check after landing: `scripts/status-paper-search-quality` for the story
  count and mean cited sources against the days before, and the `search run=`
  lines for pages_new against pages_returned.

## Remaining error

- Ceramic is a lexical engine over English pages with no dates on results.
  Whether it finds today's news for the paper's queries is unmeasured. If it
  does not, the paper's research gets worse quietly while the counts look
  fine. The landing's proof must look at results, not only at counts.
- The Keenable cap must be lifted or resized on December 5 when the plan
  renews.
- Done 2026-11-18: the hourly status prints Keenable's calls today, the
  day's projection and how long the last credit reading lasts, and flags a
  day projected over the rate that reaches renewal
  (`scripts/status-paper-usage`, `KEENABLE_PLAN`). Keenable reports no
  balance, so the reading is a number a person copies from the usage page;
  the flag does not know what was spent since. First run, 16:20Z: 7,479
  calls so far today, 13,647 projected, against 1,032 a day.

## First 35 minutes after search reuse deployed (2026-11-18, 20:05Z to 20:40Z)

Deployed 20:03:32Z (`38d86121e`: the search-reuse commits and the floor
list fix). Read from the gateway journal and the paper's console; the new
lines make this readable for the first time.

- 332 searches, every one carrying its run and desk (all `desk=research`,
  29 runs: median 7 a run, most 50). 44 answered by the gateway cache; 291
  reached Keenable, about 470 an hour. The day's average before the deploy
  was about 460 an hour and the morning's peak 670. **No halving is
  visible.**
- The per-run summary line (24 runs): 296 searches started, **2 answered
  from held results, 4 not sent** (three "stopped yielding", one failing).
  The two mechanisms this change added fired on 2% of searches.
- Why: within a run, a new wording does find new pages. Pages returned
  11,480, new to the run 9,481 (83%). The replay that predicted a 44% saving
  counted pages already returned to anyone that day; inside one run the
  overlap is small, so "stop when wordings stop finding new pages" rarely
  stops.
- **What is wasted is not repeats, it is results nobody reads.** The same
  24 runs read 456 pages of the 9,481 new ones returned: under 5%. One run
  made 29 searches, was handed 1,036 new pages and read 13.
- Page reads by where the page came from (1,561 `corpus read` lines): 47%
  were captures the computer held before the run, 38% the run's own earlier
  fetches, 15% new fetches. So nearly half of what research reads is already
  held, while it still searches the web first.

Belief now: claim C129 (6,500 or fewer searches on the first full day) is
unlikely to settle true; it stays as entered and is scored on the day. The
lever is not novelty but need: a run should say what it lacks before it
searches, and a search's forty results should not be forty reasons for
another search. That is stations S1 and S3 of the
[design proposal](../design/corpus-first-retrieval-and-research-2026-11-18.md)
(research reads a corpus brief first and searches only for stated gaps),
now with a measured baseline: 12 searches a run, 19 pages read a run, 47%
of reads already held.

Remaining error: 35 minutes right after a restart, one sample; `from_held`
counts only reuse inside a run; the gateway cache's 44 hits are not split
by cause.

## Why a run searches again: the traces and the code (2026-11-18, read after 20:05Z)

Read only: the gateway journal of Node B and the paper's console, 20:05Z to
about 20:43Z, 32 research runs of the paper's computer, 349 gateway search
requests, 54 `corpus
search` lines, 2,004 `corpus read` lines and the 28 `search run=` lines of the
runs that had ended. Counts only; no query is quoted.

**Measured.**

- Shape. A run has a median of 2.5 groups of web searches (most 12) with
  reads between them; a group has a median of 2 searches and a mean of 3.3
  (largest 29, the 29 searches before any read of one run). 58% of all web
  searches (203 of 349) arrived in the same second as another search of the
  same run, in bursts of 2 to 8; 42% (146) arrived alone. The median run sent
  3 web searches before its first read (119 of 349 in all, 34%).
- Reads between. 779 of the 1,256 reads came before the run's last web
  search (the other 477 after it). So the desk does
  read between rounds; it does not read between the searches of a round
  (the round is a burst), and a round starts with new searches in 105 groups
  over 32 runs.
- Corpus first. 26 of 32 runs searched the corpus at all (54 corpus searches
  against 349 web searches); in 23 the corpus search came before the first
  web search. The corpus search is no gate: of those that returned captures
  by count, 18 of 54 returned the full 10, 9 returned none, the rest 1 to 8.
  The counts do not say whether the run then read what the corpus held (47%
  of reads were held captures, above).

**Read in code.**

- The prompt asks for both patterns in one list. `research.yaml` v15, first
  bullet: "To run searches at once, start each with choir.SearchStart(query)
  and read them with choir.Searches.Get(id) or choir.SearchWait(id, ms)";
  fifth bullet: "Cover every open thread in each round: give all the wordings
  of one question, with the named organizations' own sites, to one
  choir.SearchAll(question, wordings) call ... Also search for what the
  request asks and the document does not yet cover." The tool texts agree:
  `SearchStart`: "Starts a web search on every search provider at once";
  `Search`: "Use SearchStart when you want several searches running at once";
  `SearchAll`: "Give it ALL the wordings of ONE question ... It stops sending
  when a wording's first ten pages are at least 80% pages you already hold".
  The second-to-last sentence of the first bullet and "Cover every open
  thread in each round" are what produce a burst per round; "search the
  corpus first" is one clause and says nothing of writing down what the
  corpus held and what is missing.
- A round is a run. `researchInboxPrompt` (`inbox_records.go`) starts a run
  per Texture record: "Send an interim packet ... as soon as you have read
  one new source you can cite, and another each time you have read more".
  Nothing in it or in the prompt tells the desk to list what it holds before
  it searches; the next round starts from the objective, so it searches again
  (and the `from_held` reuse of a run does not carry over to the next).
- `SearchAll` stops on novelty only (`search_all.go`: 80% of a wording's
  first ten pages held, two empty, two failed, 120 seconds). With 83% of
  returned pages new, it sends every wording.

**Inferred.**

- Most searches are plain `SearchStart` bursts, not `SearchAll`: `SearchAll`
  waits for each wording (up to 15 seconds) so it cannot put two requests in
  one second, and 58% of the requests share a second. The logs cannot say
  how many `SearchAll` calls were made nor how many wordings each was offered:
  the run summary counts searches started and not sent, not the verbs. One
  field would tell: calls to `SearchAll`, wordings offered, wordings sent.
  That is added in the change below.
- Why searching again does not need more searching: a search returns 40
  pages, a run reads 19 in all; a second wording over a question that already
  holds 40 unread pages mostly adds unread pages. The need to ask for another
  wording is about what the question lacks, which only a desk that has read
  can say.

Remaining error: one 38-minute window right after a restart; "same second"
is the gateway's receive time, so a cached answer or a slow cell can move a
search a second; the corpus counts are per search, not per capture read.

## What was built: search for what a question lacks (2026-11-18, `lane/search-for-need`)

Belief from the section above: the desk does not need a limit, it needs to
be told what it holds unread when it is about to ask again. Built, orange,
rollback revert:

- `SearchAll` checks before each wording how many pages new to the run, from
  the searches sent for this question (the question's terms as
  `searchquery.Key` reads them, so a reworded question is the same one), the
  run has not read. At 15 or more (`searchEnoughUnread`: a run reads about 19
  pages in all and 1 to 2 of every 40 returned, and a search returns up to
  40; 15 is nearly a run's reading spent on one question) it sends no more
  wordings, names each as not sent with the count, and says: read the ones
  that matter, then search again for what is still missing, naming it. The
  count falls as the run reads, so a later call sends again. Counted in the
  run's counts as `holds_unread`, beside `stopped_yielding` and `failing`.
  `fresh` is unchanged. Nothing refuses a search.
- A plain `Search` or `SearchStart` of a run holding 15 or more such pages
  carries one note per cell with the count; the search is sent. Departure
  from the outline: the note counts the whole run, not one question. A plain
  call carries no question, and the only key in the repo is the exact term
  set, which two different wordings of one question do not share; grouping
  them would need a fuzzy matcher, which is not built.
- Research prompt v16 and the `Search`, `SearchStart` and `SearchAll` tool
  texts: corpus first; each thread written as held, stale or missing in the
  first packet; the web only for stale and missing threads, one question at a
  time, reading before searching again. The sentences that ask for searches
  at once and to cover every open thread with all its wordings are gone.
  Desk protocol 18.7.0 (a field `Next` on `SearchAllResult`).
- The run summary line gains `search_all_calls= wordings_offered=
  wordings_sent= unread_notes=`; a plain search count is `started` minus
  `wordings_sent`.

What would show it worked, from the `search run=` lines after deploy: median
searches a run from 7 toward 4 or fewer (12 a run on average now); the share
of offered wordings sent (`wordings_sent` over `wordings_offered`) well under
1 for runs that use `SearchAll`; `pages_read` over `pages_returned` up from
5%; `pages_read` a run not lower than the 19 of the baseline. If
`search_all_calls` is near zero the desk is using `SearchStart` bursts and
only the note and the prompt apply: then the burst, not the wording, is the
next thing to look at.

Remaining error: the constant 15 is read from one window; the note counts
the run, not the question; a model that ignores both is not stopped, by
design.

## The evening of 2026-11-18: volume fell, and not because of the change (21:20Z to 24:00Z)

The paper's web searches fell from about 470 an hour to 127 in the hour to
23:26Z. That is not search reuse working. Read from the gateway journal and
the paper's console:

- The whole computer did less from 21:20Z, an hour before the next deploy:
  its lines in the gateway journal fell from about 1,200 per ten minutes to
  50 at 21:20Z and ran at 300 to 1,000 after. The console shows the cause was
  ordinary: its open stories published at 21:20Z and the next captures
  arrived in smaller batches. No hold, quota line or wedge.
- Each run searched as before. Runs that searched, 20:00Z to 21:20Z: median 8
  started, 3.4% of returned pages read. After the 22:19Z deploy (search reuse
  and the full-page index, not yet searching for need): median 6 started,
  5.9% read. There were fewer runs, not fewer searches per run.
- In the hour 23:00Z to 24:00Z the volume was back: 34 runs that searched,
  276 searches started, 336 search requests at the gateway.

So an hourly count cannot settle a forecast about this change; searches per
run and the share of returned pages read can. `scripts/status-corpus-reuse`
(2026-11-19) prints both from the console and the gateway journal, and
`scripts/status-hourly` appends a row per whole hour to
`~/.cache/choir-status/corpus-reuse.tsv`, because the console keeps only
about 2.5 hours.

What the same reading shows about the corpus, 23:00Z to 24:00Z: the research
desk made 87 corpus searches (a median of 2 a run; 4 of 34 runs searched the
web with no corpus search), every one returned a capture with a page-text
hit, and of 1,754 pages the desk read, 25% were held before the run began,
57% were captured by the run itself and 17% were new fetches. Over
21:00Z to 23:40Z the held share was 7%. The desk finds held pages when it
asks and mostly does not start from them. The next change (the corpus
brief, lane `corpus-brief`) has the host search the corpus for the run's
objective and put the result in the run's first message.

## First hour on searching for need (deployed 2026-11-19T01:14Z at `f62811fec`)

The paper refreshed, planned, at 01:14:15Z. The forecast (C140) read the
first 24 research run summaries from 01:34:15Z, the 24th at 03:13Z, and
settled false: the median web searches a run is 4 (the forecast needed 4 or
fewer) and the run read 334 of 6,000 returned pages, 5.6% (it needed 10%).
Over the 24 runs 22 searches were held back because the question already
held unread pages, and 46 of 71 offered rewordings were sent. One run still
started 39 searches and another 17: the rule stops rewordings of one
question, not a run that asks many questions. The table is the first 13
runs, read at 02:29Z.

| | before the deploy, 00:00Z to 01:13Z (19 runs) | after, 01:34Z to 02:28Z (13 runs) |
| --- | --- | --- |
| web searches started a run, median | 7 | 4 |
| web searches started a run, mean | 9.1 | 4.3 |
| rewordings sent of those offered | not counted | 13 of 25 |
| searches not sent because the question held unread pages | 0 | 12 |
| returned pages the run read | 6.5% | 6.3% |

The gateway saw 69 searches from the paper in the hour to 02:21Z, against
about 500 an hour before the deploy. Fewer runs started in that hour (the
refresh interrupted work), so the whole fall is not the change's. The share
of returned pages a run reads did not move: a run that searches less still
reads about one page in sixteen of what a search returns.

Two things disturbed this reading.

- **A second refresh at 02:28Z.** A change to a status script and its test
  (`d2c8bfdd0`) was pushed at 02:13Z. `scripts/test-*` is not docs-like
  (`.github/scripts/ci-docs-like-paths`), so the push ran CI and a deploy,
  and the deploy refreshed the paper while the forecast's runs were being
  counted. A developer-side script should not restart the paper; until the
  classifier separates "CI must run" from "must deploy" for such a script,
  script changes wait for the next behaviour landing.
- **One run read one page 3,090 times.** Research run `c0351184` called
  `read_capture` on a single capture of its own in a loop inside a cell,
  between 02:13:01Z and 02:14:07Z. The per-cell bound on host calls refused
  it three times ("this cell made 1025 host calls; at most 1024 per cell"),
  once in each of three cells, and the run ended with two pages read. The
  bound worked. The reading did not: `scripts/status-corpus-reuse` counts
  read lines, so this one run made "pages captured by the run itself" 96%
  of the window's reads and "held before the run" 0.2%. The script should
  count each capture once a run. What the cell program was trying to do has
  not been read from the run's tape.


## Traces read, 2026-11-19 (station L1 of the night's goal file; read-only)

The owner, 2026-11-19: "we need to review the traces and investigate all the
problems eg the 3090 read loop ... if the problems are at the model level,
naively changing the harness or prompt may not be correct." The research desk
of the newspaper runs on `devin/swe-2-flash`. Read from Node B at about
05:20Z: the gateway journal (search requests carry the run id and the query),
the paper's guest console (rotated; it now starts at 02:14:45Z, so it holds
3 hours and 8 minutes), and the code. Counts only, plus a few of the paper's
public news queries named by kind.

**Where a run's own ledger stands.** The supported read path for a run's
cell programs and tool results is `choir trace export` (`cmd/choir/trace.go`:
it pages `GET /api/runs/{id}/events` and builds cells and egress from the
`desk.cell` and `desk.egress` events). It needs a Choir API key bound to the
newspaper computer with its run-event read scope; the bundle is an unredacted
owner trace. No such key was available to this reading, and the paper
owner's key, the platform operator key and the tape files are out of bounds,
so **no cell program of any run was read**. Everything below that says what
a model wrote is therefore an inference from logs and code, marked as such.
What is needed to finish: either a read-scoped key for the newspaper minted
by the owner, or the owner running `choir trace export` for run `c0351184`
and for runs `5f3ee2dd`, `ec43fb3f` and `d1cb7c40` and handing the bundles
over on the machine. The 02:12Z run's console lines are gone (the surviving
console starts at 02:14:45Z; only its last two model calls, at 02:14:58Z and
02:15:06Z, and its summary line, `started=0 ... pages_read=2`, remain).

### Item 1: the read loop (run `c0351184`)

What the logs and code establish.

- The run searched nothing (`started=0`) and ended with two pages read. Its
  three cells ran 02:13:01Z to 02:14:07Z; each stopped at the 1025th host
  call, so about 1,030 calls a cell and 3,090 in all (3 x 1,024 = 3,072, and
  18 other calls). The loop broke on the refusal; it did not run on past it.
- The refusal text, `internal/yaegikernel/desk_session_worker.go:618`, is
  "refused: this cell made N host calls; at most 1024 per cell (continue in
  the next cell)". It says the bound and tells the model to continue in the
  next cell. It does not say what the calls were, that they were all on one
  capture, or what a read of the whole capture would take. The bound's own
  comment says a cell is "meant" to make about a hundred.
- `choir.ReadCapture(captureID, fromBlock, maxChars)` returns "as many blocks
  as fit in maxChars (0 means 8000)" and `next_block` when more remain; the
  server returns at least one block a call and moves forward
  (`internal/platform/captures.go:366-376`), so a loop that follows
  `next_block` ends. 3,090 calls on one capture therefore means the program
  did not follow `next_block` to its end: it either asked for a small
  `maxChars`, stepped `fromBlock` by one over a very long page, or passed the
  same `fromBlock` again. A long filing or PDF with a few thousand blocks and a
  one-block-a-call walk fits the number (3 x 1,024). That is a reading of the
  arithmetic, not of the program.
- The same console shows the ordinary shape for comparison: in 3 hours, the
  most any run read one of its own captures was 18 to 21 times (a long page
  read in 8,000-character parts), 2,982 `corpus read` lines in all. The loop
  is 150 times the next largest case. No other `at most 1024 per cell`
  refusal exists on the paper's consoles, or on any other computer's console
  on Node B, in the hours they hold; there is no earlier case to compare.

What the model was shown at each step. In each cell: results of the calls it
made (blocks and `next_block`), then at the 1025th call an error with the
text above. Between cells: the error. Whether it was shown `block_count` and
used it is read from the cell programs, which are not read.

Cause (before the reproduction below; see its update): **not established**. The evidence leans toward a model-written
program with a stepping or limit mistake, which is model behaviour met by a
harness that offers a verb whose natural loop is thousands of calls and a
bound whose message says only "continue". Confidence in "model behaviour
produced the loop": 0.6; in "the surface made it easy": 0.7; both can hold.
The three cells are the evidence that the message did not change the
program: the model ran the same shape three times after being told.

What would settle it (read the three cell programs first, with the key):
whether `fromBlock` advanced between and within cells (a walk that continued
and nearly finished, against a reset to zero each cell); the `maxChars` it
passed; whether it printed or used what it read. Then a controlled
comparison: replay the run's first message and its capture against (a) the
same model, ten times, to see whether the loop recurs (variance in the
model); (b) two other models on the same surface (is it this model?); (c) the
same model with the surface changed one way at a time: `ReadCapture` result
carrying `block_count` and a line "you have read blocks a to b of n"; the
refusal text naming the verb that made the calls; and a page-read verb that
returns the whole page up to a cap, so nothing needs a loop. A loop in (a) and
(b) alone, and gone under (c), is a harness cause. A loop in (a) and not in
(b), whatever (c) does, is the model's.

### Item 2: runs that start many web searches

85 runs of the paper's research desk sent 641 search requests between 01:00Z
and 05:22Z (gateway journal; the run id is on every line). The median run
sent 4, the largest 44, 42, 39, 38, 27, 26, 18, 18, 17, 15, 14, 14. In the
console's 54 runs that searched (02:15Z to 05:22Z), the 10 runs that started
10 or more made 56% of all searches (392 started in all, 7.3 a run; median 4).

Every run of 12 or more searches was about one subject. Taking each run's two
commonest query terms, 67% to 100% of its queries contain one of them (the
four named runs: 0.90 to 0.95; the 22 runs of 8 or more searches: median 1.0), and exact repeats are 0 to 6 of a run. The
wordings read as rewordings, not as separate questions:

- `5f3ee2dd` (39 searches in 3.7 minutes, 02:48Z): one rail disruption on a
  Melbourne regional line; the queries vary the line, the operator, the date,
  "buses", "trains cancelled", and add `site:` operators for the operator and
  the state transport site, with two queries sent twice. Console: 29
  `SearchAll` calls with 37 wordings offered (29 sent, 8 held back), plus ten
  plain searches.
- `ec43fb3f` (42 searches in 6.6 minutes, 03:49Z): one illegal dump on two
  named streets of one Zagreb district; the queries cycle through the two
  street names, the district, a local news site and the city sanitation
  site, quoted and unquoted, in Croatian. Console: two `SearchAll` calls, 12
  `unread_notes`, no sign it slowed.
- `d1cb7c40` (44 searches in 6.9 minutes, 05:07Z): one news item about young
  founders in India. No `SearchAll` call; 44 plain searches; 8 notes that the
  run held unread pages; 11 pages read of 1,800 returned.
- `47806e6f` (15 searches, 05:17Z): one query sent seven times in 85
  seconds. Six of the seven were served from the gateway's cache; the in-run
  reuse that should answer a same-terms search from the run's own results did
  not (`from_held=0`). Why is not established (`fresh`, a failed first
  search, or a gap in the reuse); exact in-run repeats in all, 23 of 641
  requests in 14 of 85 runs.

What the run was asked (objective size, number of questions) is in the
run's first message, which the logs do not carry, so it is not read. What is
read is the shape: one subject, many wordings, results always 40, and 74%
(`ec43fb3f`: 1,240 of 1,680) to 94% (`d1cb7c40`) of the pages new to the run,
so the search novelty check never stops them (its design: stops at 80% held).
The notes of the 15-unread-pages rule were shown (95 notes in 34 of the 54
runs) and the search went on in the same cell batches. Only 16% of the
searches (63 of 392) went through `SearchAll`; the rest are plain
`SearchStart`, where the rule is a note and nothing more.

Cause (before the reproduction below; see its update): **model behaviour** under a harness that does not stop it:
rewording one question many times, in some runs after a note said it already
held unread pages. Confidence 0.65. It is not many questions (one subject per
run), and not rewording by the harness (the harness sends what the cell
asks). The part that is the harness's is that the prompt (v17) still
offers `SearchStart` for searches "at once" and that the stop rules are
attached to the verb the model uses least.

What would settle it: the cell programs of the four runs: did one cell
write a loop over a list of wordings (a program), or did the model write
several cells, each a new idea after reading results (a reasoning
trajectory)? The first is a program shape the harness can bound; the second
is what the model does when results disappoint. Then a controlled
comparison on a fixed set of 20 objectives of the hard kind (a local event
with thin coverage, a non-English locale) and 20 of the easy kind, each run
with: the same model on the current surface; another model on it; the same
model with `SearchStart` and `Search` removed so only `SearchAll` exists;
and the same model with the note replaced by the list of the held unread
titles. Count searches a run, subjects a run, and whether the answer was
found. If the count is high only on the hard kind for every model, the
searching is what thin results cause, and the lever is what a thin result
says. If it differs by model, it is the model.

### Item 3: a run reads about 6% of the pages its searches return

Over the 54 runs that searched (02:15Z to 05:22Z): 15,867 pages returned,
12,798 new to the run, 814 read: 5.1% of returned, 6.4% of new. Median run
6.5%. This is the same as before (5.1% to 6.5%) after two changes.

The strong fact is that **pages read do not follow pages returned**. By
searches started in a run: 1 to 3 searches, median 7 pages read of 80
returned; 4 to 6, 10 of 160; 7 to 9, 22.5 of 300; 10 to 20, 25 of 440; 21 or
more, 13 of 1,680. A run reads 7 to 25 pages whether it was handed 80 or
1,680. Runs that searched nothing read a median of 3.5 (the corpus). So the
share falls only because returns scale with searches and reads do not: the
read count looks set by what the run needs to write, and searching is not
tied to it.

What the model is shown. The cell result after a search carries a count
line (`s3: 3/4 answered, +2 in, 12 of 40 pages new`) and, where relevant, a
note; it does not carry titles or snippets. The titles are shown only if the
model calls `choir.Searches.Summary(id)` (results by host, the first two hits
of each host, title and URL, under 5,000 bytes) or reads `choir.Searches.Get(id).Results`
(each hit with its snippet). `SearchAll` returns the merged results with
`Held` marks. The prompt tells the desk to pick sources "from the host-deduped
`choir.Searches.Summary(id)`". Whether a run called `Summary` is not logged.

Cause (before the reproduction below; see its update): **not established**. The titles are available but not given; the
harness gives a count where a list is what a choice needs, and the logs
cannot say whether the model looked. Confidence that the share is not a
failure of attention at all: 0.4; that reading follows the story's needs and
searching is the misfit: 0.5.

What would settle it: from the cell programs, per run, whether a search was
followed by a `Summary` or `Get` call before the next search, and which ranks
were read; then a comparison that varies what the cell result after a search
shows (counts only, as now; counts and the top ten titles with hosts;
the same plus held marks) with the same model, and the same with another
model, measuring pages read, which rank, and whether the packet's sources
differ. If reads rise with titles shown, it was never looking; if they do
not, the 6% is what the stories need.

### Reproduction on the drill computer d@e, 2026-11-19 05:31Z to 05:51Z

The coordinator's ruling: no key for the paper is minted tonight, and none
of the forbidden ones may be used. The cell programs were therefore obtained
by **reproduction on d@e** (`computer-59c902c5d46caecc0a180a8557164e72`, the
drill computer, where paid and adversarial runs are allowed). **This is
evidence about the model and the surface, not proof of what the paper's runs
did.** The differences from the paper: another computer with a smaller corpus;
a request sent through the prompt bar (Texture then casts to research), where
the paper's research is woken by source captures; objectives I wrote, not the
paper's; nine runs. The research desk ran `devin/swe-2-flash` in every run
(the `model.served` event: selected provider `devin`, model `swe-2-flash`,
source `/mnt/persistent/files/System/model-policy.toml`); no setting was
changed.

**Commands used.** The `choir` binary already in the lane (an arm64 Mac
build whose `trace export` and `run start` exist; nothing was built on the
build VM). The key is passed only as a file:

    ./choir run start --api-key-file ~/.config/choir-qa/keys/d-e.key \
        --computer computer-59c902c5d46caecc0a180a8557164e72 \
        --idempotency-key l1-repro-<name>-20261119 "<objective>"
    ./choir trace export --api-key-file ~/.config/choir-qa/keys/d-e.key \
        --computer computer-59c902c5d46caecc0a180a8557164e72 \
        --out <scratch>/trace-<name>.json <trajectory id>

A bundle holds the research run's events. The `tool.result` event of a
`desk_go_eval` call carries `arguments.source` (the cell program, verbatim)
and `output` (the cell result as the model was shown it, with `searches`, the
notes the host added); the `desk.cell` event carries `egress_calls` (host
calls in the cell); `search.served` counts the provider's results; `reuse`
in the output says whether the session survived the cell. Bundles stay in
the session's scratch directory, outside the repo.

| name | kind | trajectory | research run |
| --- | --- | --- | --- |
| A1 | transit disruption, Wellington | `51ef6111-296f-53fc-848e-4e5c3446fbae` | `a09a618c` |
| A2 | municipal dispute, Lisbon | `725b94ab-dfa9-5762-947b-cfdbd012cec8` | `37f22797` |
| A3 | soft feature, Nigerian founders | `342c33d3-e2a5-5dfa-9d4a-74159adcdc7f` | `b1132328` |
| D1 | thin coverage, night buses in Kaunas | `588e67cd-10b1-5a36-861b-7e92897d1242` | `ee3695ca` |
| D2 | thin coverage, parking dispute in a Wroclaw district | `91ea5d00-eea1-5590-bd07-85f1692f39ba` | `6dfd6d4f` |
| D3 | thin coverage, tram closure in Porto | `808bf536-42c1-5b60-a257-f8f1f3de6896` | `ad478c97` |
| C1 | one long public page read in full (a novel's plain text) | `2b3ebc1e-e68d-5f7f-8013-10bcfb62d0aa` | `4e518c23`, `947cd501` |
| C2 | the same page, another question | `eb6bf795-e015-5b1c-8562-7a8070876785` | `4b5c85af` |
| C3 | the same page, another question | `1e1959c2-e5df-5c43-b063-7b31caa7eef2` | `0e30aec5` |

D1 to D3 were added after A1 to A3 came out near the median (6 to 12
searches, not 39): the paper's big runs were thin-coverage local events in
other languages, so D1 to D3 are that kind.

#### A and D: the many-search shape

| run | cells | searches served | search cells | verbs | pages returned | distinct URLs tried | share | largest "holds N unread" note shown |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| A1 | 50 | 12 | 8 | `SearchAll` 1, `SearchStart` 11 (10 with `fresh` true) | 480 | 38 | 7.9% | 148 |
| A2 | 65 | 11 | 8 | `SearchStart` 12 (2 with `fresh` true) | 440 | 42 | 9.5% | 71 |
| A3 | 91 | 6 | 3 | `SearchStart` 6 | 240 | 38 | 15.8% | 76 |
| D1 | 65 | **89** | 18 | `SearchStart` in loops, 0 `SearchAll` | 3,435 | 48 | 1.4% | 467 |
| D2 | 35 | 22 | 7 | `SearchStart` | 880 | 27 | 3.1% | 157 |
| D3 | 70 | 18 | 15 | `SearchStart` | 720 | 63 | 8.8% | 393 |

("Distinct URLs tried" counts the URLs in the source of cells that call
`Read`, `ReadAll` or `ReadCapture`, failed reads included, so it is an upper
bound on pages read.) In all, `SearchAll` was called once in 9 runs and
`SearchStart` about 82 times; in the paper's console, `SearchAll` was in 20 of
54 runs. The `SearchAll` stop was therefore not the thing meeting the model.

What the programs are.

- **A runs: one cell per idea, one to three searches a cell**, interleaved
  with reads. A1 cell 2, the only `SearchAll`: `q1, err1 :=
  choir.SearchAll("Wellington commuter rail signal failure delays", []string{"Wellington Metlink signal
  failure trains delayed November 2026", "Metro Trains Wellington signal fault commuter delays", "Metlink
  service alert signal failure Wellington"})`. The result: "1 of 3 wordings
  sent ... not sent (this question already holds 40 pages new to this run that you
  have not tried to read)". The model then used `SearchStart` for every search
  after it, and from cell 17 on passed `true` (fresh) as the second argument
  each time, for example `s4, _ := choir.SearchStart("Metlink signal fault Wellington trains delays", true)`.
  The prompt says `fresh` sends a search the run holds results for; the model
  used it as the default way to send. In A2 it came late (cells 46 and 60).
- **D1: a looped list of wordings, 18 search cells of 18.** Cell 8, verbatim:
  `qs := []string{"delfi Kaunas viesasis transportas skundai lapkritis 2026", "Kaunas transporto
  reorganizacija gyventoju pasisakymai", "Kauno viesasis transportas naktinis eismas naujoves 2026",
  "Kaunas bus routes changes November 2026 complaints", "Kaunas night service Kautra changes 2026"}`
  then `for _, q := range qs { s, e := choir.SearchStart(q) ... }` (diacritics
  dropped here; the program has them). Three to twelve searches a cell, in
  Lithuanian and English, then a `Summary` of each. Five of those cells
  failed to compile and the same list was resent in the next cell (nothing
  had started; the resend is not a repeat of a search).
- **D2: a list of 5 wordings in a loop at first, then one at a time**; **D3:
  one at a time, each with a comment to itself** ("Marathon was Nov 8 -- last
  weekend, not this week ... The main this-week finding: weekend tram
  closure ... Check its exact wording/date more"), the query written as
  string pieces joined with `+`. D3 is the thin-coverage run that behaves as
  the prompt asks: each search is for a named gap.

What the cell results showed the model. After a search: the count line
(`s3: 1/1 answered, +1 in, 31 of 40 pages new, done`), then (from the second
search on) the line "new pages in the latest searches: s1 40, s2 36, s3 31",
then the note "this run holds N pages new to this run that you have not
tried to read, returned by its earlier searches; the search is sent. Read the ones that
matter (choir.Searches.Summary(id) lists them), then search again for what
is still missing, naming it." N in D1 ran 95, 172, 289, 369, 467 over five
successive searching cells and the next cell searched again each time; in A1,
36, 75, 113, 148. The note is a note; the model kept searching. The
note "you have N captured sources and have sent no packet; send an interim
packet now" also appears and was acted on (D1 cell 4 sends an interim
packet).

`Summary` or `Get` between searches: yes in most gaps. A1: 5 of the 7 gaps
between search cells had a `Summary`/`Get` cell; A2: 5 of 7; D1: 23 `Summary`
calls (it prints a `Summary` of every search it starts, in the same cell).
So the model looked at titles; in A1 cells 13 and 18 it printed `h.Title`
and `h.URL` of all 40 results (`for _, h := range s.Results`), and D3 printed
`h.Snippet` of the pages it chose.

Class for item 2 after reproduction: **model behaviour** (writes a loop over
a literal list of 3 to 12 wordings of one question, defaults `fresh` to true,
and searches on while a note says it holds hundreds of unread pages), met
by a harness whose notes do not stop it and whose stop rule sits on a verb
the model hardly uses (1 `SearchAll` in 82 searches). Confidence 0.7 that the
paper's big runs have this shape; 0.5 that they have it for this cause, since
the paper's runs did use `SearchAll` (it stopped 8 of 37 wordings in
`5f3ee2dd`). What it does **not** show: why D3 searches by need and D1 does
not (the objectives differ in language and coverage; one run each).

#### B: the read share, from the same traces

Ranks read, taken from the lists the model had printed (the position of a
read URL in the earlier printed list of titles and URLs; reads of URLs in no
printed list are left out: A1 4, A2 17, A3 18, D1 30, D2 4, D3 42, most from
`Summary` output, which groups by host): in A1, 13 of 34 reads were in the first
ten ranks, in A2 8 of 25, A3 10 of 20, D1 9 of 18, D2 10 of 23, D3 11 of 21;
the rest run down to rank 42. So the model does not take the top of the list:
it chooses by title and host from a list it printed. The share of returned
pages tried moves from 0.8% (D1) to 15.8% (A3) with how many pages came
back, not with what the run needed: the same 25 to 63 URLs tried whether 240
or 3,435 pages were returned. That is the paper's pattern (reads do not follow
returns) reproduced.

What the model had seen before reading or skipping: titles and URLs of all 40
results, because it printed them; the snippet only when it printed
`h.Snippet` (D3). The host's own cell result carries counts and notes, not
titles. Class for item 3 after reproduction: **model behaviour**, with the
harness at most letting a count stand where a list is needed: the model looks
and chooses, and its reading is set by the task, so 5% to 6% is the arithmetic
of searching much and reading about 25 to 60 pages, not a failure to look.
Confidence 0.6. The paper's runs read fewer (a median of 11 pages), so the
reproduction reads more than the paper; not explained.

#### C: the read loop

The objective needed one long public page read in full (the plain text of a
novel, 2,567 blocks, 746,912 characters). Three runs.

- **No run stepped `fromBlock` by one.** Every paging loop follows
  `next_block`: `from = v.NextBlock` (C1, C2) or `from = rc.NextBlock` (C3),
  guarded by `if v.NextBlock == 0 || v.NextBlock == from { break }`. `maxChars`
  was `0` (the default 8,000) in C1 and C2 (96 calls a pass) and `60000` in C3
  (about 13 calls a pass); C2 once passed `80000`.
- **Each run read the whole page again and again.** Cells that begin a
  pass at `from := 0`: C1 8, C2 13, C3 14 (of 94, 30 and 20 cells). Host
  calls in a run: C1 622, C2 1,232 (two passes in a cell: 188 calls), C3 132.
  The largest number of host calls in any one cell was 188; the bound is
  1,024, so **no run met the bound and the 3,090-call loop did not
  reproduce.**
- **Why the model read it again.** The text did not survive to the next
  cell. (a) A slice out of range (`s[a:b]` past the end; `reflect.Value.Slice:
  string slice index out of bounds`) panics the cell and the session
  (`"reuse":"unsafe_to_reuse"`; `choir.CellOut refused: handle ... is from a
  session that ended`): C1 4 times, C2 3, C3 2. (b) A cell that fails to
  compile leaves its variables undefined for the next ("undefined:
  bookText", "undefined: fullSave", "undefined: pandpFull": the model
  assigned to a name it expected to exist): 5, 3 and 4 times; C3 cells 4 and
  5 each re-ran a full 13-call pass to try to set `fullSave` and then
  `pandpFull`. (c) Compile errors in all (18, 4 and 6 cells; the hints say "At
  cell top level, `x := v[i]` ... do not compile here"). The model's next
  cell then re-ran the paging loop with a one-line change. A run that kept
  the text in a variable that survived read the page once.
- **A silent model error.** C1 cell 9 declared the block struct with the tag
  `json:"id"; json:"text"`: the host accepted it, `Text` came back empty,
  and the cell printed `chars: 2566` for a 2,567-block page; cell 10 fixed it.
- **At a refusal**: not reached, so what the model does after "continue in the
  next cell" is not shown.

Class for item 1 after reproduction: **model behaviour inside a harness that
loses the state it relies on**. The mechanism that is shown to make a run
read one capture many times is re-paging the whole page in each cell after a
panic or a compile error; at 100 calls a pass it takes 10 or more passes in
one cell to reach 1,025. Whether the paper's run did that (a larger page, a
loop that restarts inside a loop, or a step of one block) is **not
established**: the 3 x 1,024 pattern is not reproduced, and the page it read
is unknown. Confidence 0.5 that the paper's loop is this mechanism at a larger
scale; the cell programs of `c0351184` are still the only thing that settles
it. The refusal text is not shown to have helped or hurt.
