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

## What is being done

- Ceramic becomes the provider every search starts with; Keenable joins only
  when Ceramic's answer is short, under a daily cap sized so the remaining
  credits last to December 5 (an owner-set spending cap). Lane
  `search-ceramic`, claim C128.
- The investigation above. Next: the search log line carries the run id,
  and a design for the caller-side fix, measured on the journal sample
  before any prompt is changed.

## Remaining error

- Ceramic is a lexical engine over English pages with no dates on results.
  Whether it finds today's news for the paper's queries is unmeasured. If it
  does not, the paper's research gets worse quietly while the counts look
  fine. The landing's proof must look at results, not only at counts.
- The Keenable cap must be lifted or resized on December 5 when the plan
  renews.
- The hourly status needs a line for credits spent against the plan, so
  this is seen in a day and not in a fortnight.
