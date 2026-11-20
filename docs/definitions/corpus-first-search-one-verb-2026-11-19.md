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

## Where "only search for what is new" goes (design note, 2026-11-20)

Written from [the measurement](../evidence/what-starts-research-runs-and-what-is-new-2026-11-20.md).
Every number is a hypothesis with the uncertainty shown; the base is about
3,900 web searches a day, 544 research runs, 402 stories.

### What the numbers say about the three places

| Place | What it can reach | Measured size | Verdict |
| --- | --- | --- | --- |
| (a) the Cast: Texture casts no research when the story's event has no new capture | repeat runs only | repeats are 21% of searches (upper bound, about 800 a day); web-side floor 1.8% (about 70 a day) | not first. 79% of searches are the story's first run, which has no earlier run to be "nothing new since". The event layer joined 27 of 750 captures (3.6%), so "newest capture for the event" cannot be read; and the research desk's own reads were 537 of those 750, so a test that counts any capture passes by construction. A refusal at the Cast also costs Texture a model turn, because a turn must end in Cast, Wait or Done (`texture.yaml`). |
| (b) research run start: the brief says what is held and what is new since T | every run, first and repeat | the brief already runs for free before every run (`research_brief.go:739`, 40 of 40 delivered, no model turn); the corpus held 6 to 33 captures on the objective every time, newest median 26 min old | right place for "the corpus answers this" and for repeats. Desk rules hold: the host reads corpusd on the research desk's behalf (as the brief does), and Texture learns the result through the research desk's Report record, never by reading research's data. Precedent: the entities request sends a research run with "search for no new sources" and an empty Report (`store/story_research_request.go`). |
| (c) the one verb: held, sent, sent with since | every search, first and repeat | 65% of sends were a run's 4th or later (54% since the gate); 43% (pre-gate) and 38% (post-gate) are near-repeats or follow two mostly-seen searches; 15 of 16 exact repeats are in the heavy runs | the largest sure removal, and it is the only place that sees the loop. |

Why the gate lets loops through (section 3 of the measurement): reading a
page, even a dead link, takes it off the unread count; a repeated page is not
new to the run so the count never climbs; the yield stop exists only in
`SearchAll`; a refusal is per wording, never per goal; nothing asks whether the
objective is met. Rows 0 and 4 of the table above, applied to every verb with
leads that survive a restart, close the first four. Row 5's `since` helps only
the sends that remain.

The missing piece is the objective test. A story publishes only when its
citations span two distinct outlets, and 34 of 41 objectives (213 of 236
searches) ask for a second source of one captured claim. So the need is
countable: **distinct outlets, other than the claim's origin, that the run
holds read or the corpus holds unread on the objective**. At two, the
objective is met and a further send for it is held with those pages. At zero
after three distinct wordings returned nothing that bears on the claim, the
run reports "not found" instead of a fourth (the Las Vegas run reworded one
query 18 times, 85% of results already seen by the fourth). This needs one
field the logs lack: the outlet of each capture (corpusd has it; the guest
line does not).

### Risks, and how the paper still learns what is new

- **A story that stops updating.** A story updates on intake: source polls
  queue captures (`source_schedule.go`), management feeds them to the story,
  Texture revises. None of that is a web search. Research's search adds a
  second outlet and fills gaps. A hold never touches the feed path, so a held
  run cannot freeze a story that has new intake; it can leave a story at one
  outlet, which is the real risk (97 of 160 revision attempts in the window
  already waited for a second outlet).
- **Missing a development the corpus has not captured.** Row 5 and `since`
  still send when the corpus holds strong captures that are all read; row 6
  sends when nothing is held. A live event is covered by polls and by sends
  that carry a `since` after the newest intake capture, not by repeating
  yesterday's question with a new date word.
- **Self-capture.** "New since T" must count intake captures only: the event
  layer's `queued` minus `research_reads`, and a story's `feed` choices. A
  research read of a page is not news to the paper.
- **A repeat run held when a reader asked.** A reader's "research more" and the
  entities request are owner acts and are never held (0 of 41 in the window).
- **Stories a day.** Baseline from the console: 42 distinct stories published in
  1 h 36 min (63 publications), 97 revisions held for 0 or 1 outlet. The
  guard is stories first published a day, and the share of stories published
  within 3 h of their first research run, against the same weekdays the week
  before; no landing below ships if either falls 10%. The journals I read do
  not hold the week before; the first step records it.

### One ordered plan

| # | Landing | Class | Removes (searches a day, of about 3,900) | Falsified by |
| --- | --- | --- | --- | --- |
| 0 | Measure first: on the brief line and the cast, add `best_cosine`, `newest_strong`, distinct outlets held, the Cast's record kind, newest intake capture of the story's event (intake only); on the verb, log why `reuseSearch` declined (key matched but unusable, out of window, unknown search); persist the run's search total (gateway `run=` count) | yellow, with log lines in agentcore the class stays yellow only because no behavior changes | 0 | the fields are absent from the first day's lines |
| 1 | The verb refuses a send that re-asks held pages: rows 0 and 4, for every verb (plain Search and SearchStart included), with the lead memory so a restart does not empty it; two sends in a row with 70% or more of results already returned hold the third with the pages | orange (guest search verb; no gateway change); red if it changes run acceptance | 600 to 1,300, central 900 (replay: 38% to 43% of sends are the upper bound; the in-run held mark sees less than the gateway's computer-wide `seen`) | replay of the 33 h journal: more than 5% of the held sends returned a page a later turn read and Texture cited; or a day after deploy where searches fall under 20% |
| 2 | Research run start: the brief carries "held: N captures from M outlets, newest T; new intake since the last run: K". When M is 2 or more outlets other than the origin and nothing is unread-new, the run's first message says the objective is answered from held pages and sends are held for it; the run reads and reports | orange; the held-run path is red (run acceptance) | 400 to 900 after overlap with 1 (first runs are 79% of searches, about 3,080; if 20% to 35% have the second outlet held) | outlets held is 2 or more in under 20% of briefs after landing 0: then this removes under 200 |
| 3 | The objective is met or absent: a run holding two distinct outlets on the objective is sent no more for it; three distinct wordings with nothing bearing on the claim end in a "not found" Report | orange | 200 to 500 (the 8 heavy runs, 59% of the window, all ran past two sources or a source that does not exist) | stories first published a day falls more than 10%, or "not found" Reports followed within a day by a run that found it |
| 4 | Repeat-run hold: a repeat run on a story with no intake capture since the last run starts held (a Report "nothing new since T; corpus holds X"), never at the Cast | red (run acceptance) | 100 to 400 (bound 800) | the held story takes an intake capture within an hour with the question open |

Sum of the central estimates, with overlap taken off: about 1,500 removed of
3,900, leaving **about 2,400**; the range 1,100 to 2,800 left. **No landing
here, and not all of them, is shown to reach 1,000 a day.** 1,000 is 1.8
searches a run. The only mechanism in this note that can reach it is the
second-outlet test (landings 2 and 3), because it takes the first run, 79% of
the searches, from "search until the model is tired" to "search until the
need is met", and the outlet count is the one number I could not measure. It
is landing 0's first job. A per-run count (stop at 3, or 2) would size to about
1,050 to 1,400 a day and is not recommended: it is a limit, not a need test,
and it would stop the one run in forty that needs ten.

Not recommended: a Cast-level hold first (a); any rate or quota on search
calls; another switch.

What I could not determine, repeated for the reader of this note: the
capture-side share of repeats with nothing new; the outlets held per
objective; stories a day against the week before; why 256 exact repeats passed
`reuseSearch`; the run ledgers (not reachable without a production database
read).
