# Web searches are sent without asking what is held (2026-11-19)

Owner, 2026-11-19: "we should be able to get our searches underneath 1,000
per day. All the work that we're doing with BM25 and vectors should mean
that we only need to search for things which are new information for us."

## Problem

The paper sent 4,748 web searches in the 24 hours to 19:20Z, each one
Keenable call (4,615 of 4,740 gateway lines say "answered 1 of 1
providers"; the gateway's cache served 5.4%). The credits last to the
renewal at about 1,000 a day. The corpus holds 224,000 captures with a full
vector index, and it answers: before 90% of web searches the run's last
corpus search had returned captures. The run searched the web anyway, and
read 4.7% of the pages the searches returned.

## Evidence (read-only, the paper's console 17:41Z to 19:18Z and the gateway's journal for 24 hours)

- **No code turns "the corpus answered" into "do not search".**
  `SearchStart` has no need check: it adds a one-time note when the run
  holds 15 or more unread pages and sends the search all the same
  (`internal/yaegikernel/search_need.go:80-94`). The only need-based stop is
  in `SearchAll` (`search_all.go:175`, `:259`), and it counts pages held by
  this run, never the corpus. The prompt's sentence "Search the web only for
  the stale and missing threads" is prose no code checks.
- **`SearchAll` carries one search in ten** (32 of 305 in the window; 12 of
  54 runs called it). Where it ran it refused 63% of the wordings it was
  offered. The prompt's first search instruction is `choir.SearchStart`
  (`internal/promptstore/defaults/research.yaml:58`).
- **Searches come in bursts and blind.** 34% of the day's requests were in
  same-second groups of three or more from one run; 54% of searches in the
  window followed another search with no read between. One run sent 21 in a
  row and read 0 of the 1,000 pages returned.
- **Later runs of the same story search again.** 21% of the day's searches
  came from a story conversation's second or later searching run. The
  unread count and same-terms reuse are per run; pages a run was handed and
  did not read are not captured, so the corpus never learns them. A second
  run on the Zagreb dump story sent 18 more searches after the first had
  read none of its 1,000.
- **A few runs make most of it.** Runs with 10 or more searches are 22% of
  runs and 60% of searches; 26 runs with 25 or more make 28.5%.
- **The corpus brief does not hold searching down.** 66% of briefs were
  skipped ("no objective in the unread records"), 9 of 22 delivered ones
  were cut by the 8 s deadline, and runs with a delivered brief searched
  twice as much as runs without.
- **The corpus answer has no "enough" in it.** It is a ranked top-N
  (`internal/platform/capture_rank.go:91-135`) with no signal a caller could
  gate on: how strong the best capture is, how recent, how many are newer
  than the question's window.

## Belief

The count of web searches is tied to nothing. The corpus is asked and then
ignored, because neither the verb nor the answer has a place where "we hold
this" can stop a search. Three causes, overlapping: the ungated verb (about
60% of volume), later runs repeating a story's searches (20 to 30%), and no
sufficiency signal (the rest and the shape of both).

## Fixes, in order (caller-side; no cap, no rate limit)

1. One web-search verb, with the need test `SearchAll` already has, for
   every search a run sends. Shows as: bursts of three or more fall from 34%
   of requests to under 10%; searches per searching run from 7.9 toward 3.
2. What an earlier run of the story was handed and did not read is held by
   the next run, and search results' titles and snippets are indexed so the
   corpus can find them. Shows as: searches from a conversation's second or
   later run fall from 21% to half that; pages read of pages returned rises
   from 4.7%.
3. The corpus answer says how strong and how recent its best captures are,
   and a thread's first web search waits until the run has read one strong,
   recent capture. This is the owner's rule in code: search for what is new.
4. A run whose unread records carry no objective is briefed from the
   conversation's last objective, and the brief fits its deadline.

Each is a hypothesis until a deployed day shows its number.

## Remaining error

What started each run is not in the console (no origin line), so searches
per day by origin are not counted, and what wakes the runs whose brief is
skipped is not known. Whether the corpus would in fact have answered a given
web search was not tested (48% of the day's returned URLs had been seen that
day, which is only suggestive). The "10 of 67 runs searched the web with no
corpus search" figure from `scripts/status-corpus-reuse` mostly measures
console rotation: one real case in eight examined.

## Update 2026-11-20: one provider carries every search

The hourly status now counts each provider (`scripts/status-hourly`, search
usage block). In the hour to 02:12Z, 169 of 171 sent searches went to
Keenable. The others did not answer: Brave quota_limited on every request;
ChatGPT search auth_error on every request (4,633 since the first one the
journal holds, 2026-11-18T16:50Z; the gateway resolves the provider at
start and logs no reason for the refusal); SearXNG success_empty on every
request; Tavily out of quota; Parallel and Serper one call each, then
quota_limited and error. So the 1,000 a day target is in practice a
target on Keenable, and a Keenable outage would leave the paper with no
web search. Not read: why ChatGPT search fails authentication (a
credential question, the owner's) and why SearXNG returns nothing.
