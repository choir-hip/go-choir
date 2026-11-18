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
requests (the 343 of the journal plus the last few minutes), 54 `corpus
search` lines, 2,004 `corpus read` lines and the 28 `search run=` lines of the
runs that had ended. Counts only; no query is quoted.

**Measured.**

- Shape. A run has a median of 2.5 groups of web searches (most 12) with
  reads between them; a group has a median of 2 searches and a mean of 3.3
  (largest 29, the 29 searches before any read of one run). 58% of all web
  searches (203 of 349) arrived in the same second as another search of the
  same run, in bursts of 2 to 8; 42% (146) arrived alone. The median run sent
  3 web searches before its first read (119 of 349 in all, 34%).
- Reads between. In 779 of the 1,256 reads that follow a first web search, a
  later web search of the run still came after the read. So the desk does
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
