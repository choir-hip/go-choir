# One search verb that asks the corpus first (design, 2026-11-19)

Design for fixes 2 and 3 of
[the search-volume problem](../problems/web-searches-are-sent-without-asking-what-is-held-2026-11-19.md).
Produced by an Opus design pass from the code at `lane/search-gate`; not
built. Each threshold and each landing's number is a hypothesis.

## Object

A **search lead** is one row per (owner, computer, URL key) in a new corpusd
table `search_leads`: title, snippet, the provider's `published_at`, first
and last time returned, times returned, the latest query keys and
conversation ids that returned it, `tried_at`, `capture_id`, and a 256-dim
vector of title and snippet. corpusd is its custodian. Only the host
writes it: `agentcore/desk_search.go` sends each provider arrival, one
write per arrival; the Read path sets `tried_at`, and `capture_id` when the
read captures the page. The embed worker fills leads first, newest first.
Visibility is the captures' owner and computer rule. A lead is never
evidence: it never enters `Entries`, stories, the event layer or the
brief, and `ReadCapture` refuses it. Nothing is deleted. A per-conversation
ledger is not needed: it is a filter on the conversation id.

## The verb: `choir.Search(query, why ...string)`

The host decides. `startSearch` makes one ranked corpus call before the
gateway; the query is embedded once and the same vector scans captures and
leads. The guest sends the URL keys and capture ids its run has tried.
`capture_rank.go` adds a cosine and a time to each entry and a
`Sufficiency` block: best cosine; strong (captures with cosine at least τs
and no older than horizon H); newest strong; unread strong; unread leads
(untried leads returned to this conversation in any run, or with cosine at
least τl from any conversation on this computer); same terms searched on
this computer within the reuse window.

The first matching row decides:

| # | Condition | Decision | Returned |
|---|---|---|---|
| 0 | same terms on this computer within 30 min (10 for news words) | held | that search's leads |
| 1 | the corpus did not answer, or no vector half | sent | "corpus did not answer: why" |
| 2 | `why` given | sent | the reason is logged as an override |
| 3 | at least one unread strong capture | held | "read these" (at most ten captures with passages) |
| 4 | unread leads at least K | held | the leads |
| 5 | strong exists and all are tried | sent with since = newest strong minus lag | "sent: new since T" |
| 6 | no strong capture | sent | "nothing held for this" |

A run is never starved: row 3 costs at most ten free reads; rows 1, 2, 5
and 6 always send. No cap, no switch, no extra model call.

Thresholds come from data held: τs is the 25th percentile of the target
page's cosine on the 200-question recall set, checked by replaying a
logged day's queries with `until=` each search's time (accept if under 5%
of searches that found a genuinely new, later-cited page would have been
held); τl is τs less the measured snippet-to-capture cosine shift; H is
the 90th percentile of story version time minus cited capture
`published_at`; lag is the 90th percentile of `fetched_at` minus
`published_at` for captures read from search results; K stays 15 until
lead read rates by rank exist.

## Deleted

`refuseForUnread`, `unreadPages`, `unreadRefusal`, `tagQuestion`,
`questionKey` (`sendMu` and `settle` stay); `search_all.go` whole;
`SearchStart` from the prompt, then its path after a day of zero calls; the
per-run `reuseSearch` and held marking; the `fresh` boolean (replaced by
`why`); `gateway/search_seen.go`; the run line's `search_all_calls`,
`wordings_*`, `holds_unread`, `fresh_over_unread`, `untagged` (replaced by
`decision=` counts); the prompt's search paragraphs, for one sentence.

## Landings

| Landing | Change | Number to watch | Falsifier |
|---|---|---|---|
| L1 (orange) | leads written; Read marks them tried or captured; the worker embeds leads first | share of a day's returned URLs already a lead (expect at least 48%) | under 20%: carry-over is worth little |
| L2 (orange and yellow) | `Sufficiency` and `Leads` in the ranked answer; a replay script sets τs, τl, H, lag | recall at ten stays at least 0.94; the replay's predicted held share | the replay predicts under 60% held: 1,000 a day is out of reach on gating alone |
| L3 (red) | the one verb; the guest gate deleted; prompt v19 | searches a day under 1,000; per searching run 7.9 to at most 2; share from later runs 21% to at most 8%; pages read of returned 4.7% to at least 15%; override share | override share over 30% with low yield: the model is escaping the gate and τs is wrong |
| L4 (orange) | since passed to the provider; per-run reuse and `search_seen` deleted | share of since-results already captured, toward 0 | since never narrows the provider's results: keep row 5 as marking only |

## Risks and unknowns

Staleness: row 5 and H. Starvation: ten free reads, then a send; `why`
always sends. Pollution: leads are a separate list, never evidence.
Privacy: owner and computer scope in SQL, with the captures' cross-computer
test copied for leads; lead query lists are never projected. Cost: one
query embed and one ranked search per attempted web search.

Unsure, with the cheapest check: whether the provider accepts a date filter
(read its API reference); whether τs separates answers from near misses
(one recall run printing cosine by target and non-target); whether the
worker keeps up with about 20,000 new leads a day (time 1,000 snippets on
the host); whether row 3 inflates model turns (turns per run before and
after L3); whether reads by earlier runs should count as tried.
