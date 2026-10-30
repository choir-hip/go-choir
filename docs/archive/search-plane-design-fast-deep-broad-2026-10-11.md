# Search that is fast, broad, deep, iterative, recursive and comprehensive

> **Revised 2026-10-12** ([re-engineering decisions](../re-engineering-decisions-2026-10-12.md)): Search becomes asynchronous: the call returns a stub, each provider's results land in REPL state as they arrive, and the RLM reads them when it chooses. The deadline-ladder station is deleted. Say "M of N search providers active", not "degraded".
>
> **Built 2026-10-13** (Cut 2 unit 2a): the gateway streams NDJSON, one line per provider as it answers; the waves, merge target and wave grace below are deleted. choir.SearchStart, choir.Searches and choir.SearchWait replace choir.WebSearch.

Research and design for the owner, 2026-10-11. Read-only: no code, config or
Node B state was changed. Owner's words: "Search deadline I don't quite
understand. What I know is search needs to be fast, iterative, deep, broad,
recursive, comprehensive."

Inputs: [problems/guest-search-timeout-equals-gateway-provider-timeout-2026-10-11.md](problems/guest-search-timeout-equals-gateway-provider-timeout-2026-10-11.md),
the search code path from the cell verb to the providers, 24 hours of Node B
gateway journal (2026-10-10 21:09Z to 2026-10-11 21:09Z, 509 searches), a
read-only copy of the gateway's search health store taken at 21:09Z, and the
guest traces saved in this session's scratchpad (demo16, demo18).

## Summary

**What I found.** About one search in ten in the last 24 hours (51 of 509)
ran to exactly 30.0 seconds at the gateway. In every case the gateway logged
"search succeeded", usually with 10 results, and the guest had already given
up. The guest and the gateway each allow a search 30 seconds. The guest's
clock starts first, so whenever the gateway uses its full 30 seconds the
guest gives up a few milliseconds before the answer arrives. This started
when ChatGPT web search joined the search pool at 08:52Z. Before that, 1% of
searches hit the cap. Since then it has been 15 to 27%. ChatGPT search takes
about 23 seconds when it answers at all. The router then has to wait for it
whenever its partner in the same group (a "wave") returns fewer than 40
results, which Tavily and Parallel always do because they return 10 each.
The cell receives `context deadline exceeded`, and research reports that
"the search gateway timed out".

**Corrections to the problem record.** Its key facts are right, but three
details are wrong. (1) A search cannot take two minutes at the gateway. The
HTTP handler wraps every search in its own 30-second deadline
(`internal/gateway/handlers.go:914`), so the four-wave setting cannot stretch
past 30 seconds. The real defect is that the two deadlines are equal, not
that the gateway's is longer. (2) The 19:47:26 search returned 10 results
from Tavily alone, not 40. (3) The gateway already returns whatever partial
results it has when its deadline arrives. They just arrive after the guest
has hung up.

**How sure.** High on the mechanism and the rates. Each of the 8 guest-side
search timeouts in the saved traces matches a gateway "success" at exactly
30.0 seconds (table below). The claim that ChatGPT was the provider that
stalled is an inference, held with medium-high confidence. The gateway does
not log per-provider attempts, and the guest loses the attempt list along
with the timed-out response. The inference rests on four things: the
results and providers logged at the cap, the round-robin order of the four
eligible providers, ChatGPT's measured latency, and the jump in capped
searches when ChatGPT was added.

**What I recommend.** First, cheapest, biggest effect: order the deadlines
so each layer gives up before the layer above it. The ladder is per-provider
call 10 s < gateway total 25 s < guest 35 s. That is three constants in
three files. Prediction: no search ever outlives the guest, and the
"Tavily alone, 10 results, 30 s" searches become roughly 12-second searches
with about 40 results. ChatGPT's timeouts also start counting against its
health, because its 10-second limit fires while the caller is still
waiting.

After that comes a search plane in two halves. The gateway owns time and
providers: caller deadlines, early return with typed partial results, batch
queries, latency-aware waves, and an async lane for slow "deep" providers.
The research desk owns the questions: a coverage plan, query variants,
reading pages, following leads, recursion into children, and a stopping
rule. Fast and broad belong to the gateway. Deep, iterative, recursive and
comprehensive belong to research.

## 1. The deadline problem in plain words

A search makes three hand-offs. The research cell asks the guest (the user's
computer). The guest asks the gateway on Node B. The gateway asks several
search companies (Tavily, Parallel, Keenable, ChatGPT) and merges what they
send back. Each hand-off has a timer. Today the guest's timer and the
gateway's timer are both 30 seconds, and each provider call also gets
30 seconds. The guest starts its timer the moment it sends the request. The
gateway starts its own a few milliseconds later, after it has checked who
is asking. When a search takes the whole 30 seconds, the gateway gives up
waiting on the slow provider and sends back what it has, but the guest
stopped listening a few milliseconds earlier. The work is done and paid for,
and then thrown away.

Searches take the whole 30 seconds because of how the router fills its
quota. Every search asks for at least 40 results
(`internal/researchtools/researchtools.go:656`). The router asks providers
two at a time and keeps asking until the merged list reaches 40
(`internal/gateway/searchplane/router.go:108`). It stops waiting on a slow
provider only once the list is already full (`router.go:300`). Tavily and
Parallel return 10 results each, Keenable returns 40 in a fifth of a second,
and ChatGPT returns about 18 in about 23 seconds. When the rotation pairs
ChatGPT with Tavily, which happens to one search in four, the router has 10
results after two seconds and must wait for ChatGPT. If ChatGPT answers at
around 23 seconds, a second wave runs and the search barely makes it in
26 to 29 seconds. If ChatGPT does not answer by 30 seconds, the gateway
returns Tavily's 10 results at 30.0 seconds to a guest that has already left.

What the desk sees makes this worse. The cell gets an error string,
`gateway search: http call: Post "http://10.200.x.1:8084/provider/v1/search": context deadline exceeded`.
It carries no attempts and no provider health, so nothing says which
provider was slow or that the gateway was about to answer. The model reads
it as "the search gateway timed out" or "search providers are cooling down"
and puts that in its report to Texture. The gateway, meanwhile, files the
search as a success. ChatGPT's stalled call is classified as `caller_gone`,
"the caller left", rather than as a provider timeout
(`router.go:253-254`). The reason is that the provider's 30-second limit and
the handler's 30-second limit fire together. So ChatGPT is never benched for
being slow, and it keeps being picked. Its health record shows 37 successes
in 38 counted attempts.

## 2. Evidence

### 2.1 Gateway journal, last 24 hours

Source: `journalctl -u go-choir-gateway`, lines `gateway: search request` and
`search succeeded|outage|failed`. There were 1,018 lines: 509 requests and
509 completions, with no `search failed` lines and 8 outages. The traffic
came from three computers, and one of them sent 454 of the requests.
Requests were paired with completions first-in-first-out per computer and
process. Pairing is exact for the 284 requests that had no other search in
flight on that computer, and the distribution is the same for those as for
all 509. Up to 13 searches were in flight at once on one computer, from
research children.

| Period (UTC) | Searches | p50 | p75 | p90 | At the 30.0 s cap |
|---|---|---|---|---|---|
| Oct 10 21:09 to Oct 11 08:52 (before ChatGPT search) | 230 | 2.7 s | 5.7 s | 10.4 s | 2 (1%) |
| 08:52 to 11:03 (ChatGPT in pool, wave waits for all) | 67 | 24.7 s | 30.0 s | 30.0 s | 18 (27%) |
| 11:03 to 21:09 (ChatGPT in pool, wave grace 9c1c96c7 deployed) | 212 | 3.8 s | 17.6 s | 30.0 s | 31 (15%) |
| All | 509 | 3.6 s | 12.9 s | 30.0 s | 51 (10%) |

ChatGPT search arrived with 78c9173e (gateway restart 08:52:53Z). The wave
grace fix 9c1c96c7 was committed at 10:51Z and was running by the 11:03Z
restart.

The 31 capped searches since the grace fix break down by what the gateway
returned:

| Returned at 30.0 s | Count | Reading |
|---|---|---|
| Tavily alone, 7 to 14 results | 22 | ChatGPT and Tavily in wave 1; ChatGPT never answered |
| Keenable alone, 39 results | 4 | Keenable's 39 after dedup fell one short of 40, so grace never started |
| ChatGPT + Tavily + Keenable, 40 | 3 | ChatGPT answered late; wave 2 ran past 30 s |
| Parallel alone, 10 | 2 | same shape with Parallel |

**Results per successful search** (501). 298 had 40 results, 52 had 10,
39 had 20, and the rest fell between 7 and 39. In 149 searches one provider
contributed, in 187 two, in 128 three and in 37 four. Providers
contributing: Tavily 323, Parallel 318, Keenable 299, SerpAPI 54, ChatGPT 37,
Brave 24.

**Waves per search.** The gateway does not log waves. In the trace sample
below, searches used 1 or 2 waves. Only four providers are eligible today,
and with two per wave that allows at most two waves, so the
`MaxWaves = 4` default never comes into play.

**Outages** (8). Seven of the eight were narrow `site:` queries, for example
`site:sec.gov/news/press-release AI acquisition October 2026`.

### 2.2 The guest saw these as timeouts

The saved demo16 and demo18 traces contain 8 web-search tool results with
`gateway search: http call: Post ...: context deadline exceeded`. Each lines
up with a gateway "success" at 30.0 s:

| Gateway request | Gateway "succeeded" (results, providers) | Guest tool result recorded |
|---|---|---|
| 10:21:17 | 10:21:47 (40, keenable+chatgpt) | 10:21:47.76 |
| 10:21:51 | 10:22:21 (40, chatgpt+tavily+keenable) | 10:22:21.99 |
| 10:24:54 | 10:25:24 outage | 10:25:24.59 |
| 11:43:42 | 11:44:12 (39, keenable) | 11:44:12.55 |
| 12:01:26 | 12:01:56 (10, tavily) | 12:02:00.34 |
| 14:05:41 | 14:06:11 (10, parallel) | 14:06:13.26 |
| 14:07:01 | 14:07:31 (10, parallel) | 14:07:31.42 |
| 15:16:36 | 15:17:06 (10, tavily) | 15:17:06.61 |

The guest calls the gateway directly at `10.200.x.1:8084`, with no proxy in
between. Its 30-second limit covers one attempt including reading the body
(`internal/search/search.go:85`, `internal/hostlink/hostlink.go:38-40`).

### 2.3 Per-provider latency (trace sample)

There are no per-attempt gateway logs, so this comes from tool results in
the saved traces: 42 unique searches, mostly demo18, about 14:00 to 15:40Z.
Capped searches are missing from this sample by construction. The guest
discards the response, and its attempts with it.

| Provider | Successful calls | p50 | p90 | max | Results p50 |
|---|---|---|---|---|---|
| keenable | 35 | 0.20 s | 0.39 s | 0.48 s | 40 |
| brave | 3 | 0.49 s | | 0.51 s | 20 |
| serpapi | 4 | 0.47 s | | 0.72 s | 9 |
| parallel | 35 | 1.6 s | 2.5 s | 2.9 s | 10 |
| tavily | 26 | 2.0 s | 3.5 s | 4.1 s | 10 |
| chatgpt | 5 | 22.9 s | | 23.8 s | 18 |

ChatGPT was called in 20 of the 42 searches. It answered in 5 (8.0 to
23.8 s), and the wave grace cancelled it in 15, at about 1.7 s, after
Keenable had filled the list. The S1f record has the same shape: ChatGPT
median 22.9 s over 12 calls.

### 2.4 Provider health at 21:09Z (read-only copy of search-health.db)

| Provider | State | Until | Window successes / attempts | Last failure |
|---|---|---|---|---|
| parallel | active | | 4908 / 4929 | |
| tavily | active | | 2517 / 2603 | |
| keenable | active | | 299 / 302 | |
| chatgpt | active | | 37 / 38 | |
| brave | cooling_down | Oct 12 00:38 | 31 / 37 | rate_limited (plan limit) |
| serpapi | cooling_down | Oct 12 00:39 | 109 / 114 | "run out of searches", classed rate_limited before b7668dc3 |
| exa | cooling_down | Oct 13 09:55 | 2 / 63 | parse error |
| searxng | cooling_down | Oct 15 20:12 | 16 / 86 | success_empty |
| serper | cooling_down | Oct 15 20:12 | 0 / 16 | quota ("Not enough credits") |

The plane runs on four providers today. ChatGPT's 38 counted attempts leave
out every `caller_gone` call, so its record does not show how often it was
called or cut off.

## 3. The path, end to end

1. **Cell verb.** `choir.WebSearch(query, maxResults)` makes a synchronous
   broker call with `context.Background()`
   (`internal/yaegikernel/choir.go:456`, `choir.go:216`). The cell waits.
   There is no batch and no async form.
2. **Broker.** `handleEgress` passes the payload to the host's egress
   handler (`internal/yaegikernel/broker.go:439`). The desk wiring sends web
   search to `researchDeps.HostEgress` (`internal/agentcore/tools_desk.go:308`).
3. **Host tool.** `newWebSearchTool` charges one call against the
   activation's egress budget: 64 calls per activation, shared with
   children (`internal/researchtools/egress_budget.go:46`). It raises
   `max_results` to at least 40 (`researchtools.go:656, 688`) and calls the
   guest gateway client. `compactWebSearchProjection` shows the model up to
   40 results with 400-character snippets, the attempt list, and a
   `gateway_status` line when degraded (`researchtools.go:854-860`). A
   transport error skips all of this and comes back as a bare error.
4. **Guest client.** One POST to `/provider/v1/search`, with a 30 s
   per-attempt limit (`internal/search/search.go:85`). No deadline is sent
   with the request.
5. **Gateway handler.** Authenticates, rate-limits (separate per-computer
   search bucket), then calls the plane under a fixed 30 s context
   (`internal/gateway/handlers.go:914`). It logs one line per request and
   one per outcome, without timing or attempts (`handlers.go:910, 930`).
6. **Plane.** Clamps `max_results` to 40..50 (`internal/gateway/search_plane.go`).
   The router (`searchplane/router.go`) takes a health snapshot, rotates the
   starting provider round-robin across all callers (`router.go:79`), and
   runs waves of 2 providers (`config.go:34`). Each call gets
   `RequestTimeout` = 30 s (`config.go:37`, `router.go:244`). A wave waits
   for all its calls unless the merged list reaches the target (40), and
   then it gives the rest a 1.5 s grace (`router.go:300`). Waves continue
   while the merged list is under target, up to 4 waves.
7. **Health.** Outcomes move the shared, durable cooldown state
   (`searchplane/policy.go`, `health.go`). A call cut off because the
   caller's context ended is `caller_gone` and records nothing
   (`router.go:253-262`).
8. **Providers.** Each HTTP client has its own 30 s timeout
   (`internal/gateway/search.go`). ChatGPT's is 60 s
   (`search_chatgpt.go:93`) and runs a reasoning model with hosted web
   search, on the same Codex OAuth credential as ChatGPT inference
   (`search_chatgpt.go:66-100`).
9. **Above search.** Research can call `choir.Research(children)`: at most
   4 children at a time, a 10-minute window, and children cannot fan out
   further (`internal/agentcore/research_children.go:23, 27, 44`). Children
   share the parent's 64-call egress budget. For deep reading, research
   has `choir.FetchURL` (256 KiB, 12,000 characters by default),
   `ImportURL` and `ReadContentSelector`. Each is one URL per call.

## 4. What goes wrong today

**The guest gives up while the gateway is still working.** Three timers that
should nest are all set to 30 seconds. A provider call can use the whole
search's budget, and the search can use the whole guest's budget, so the
only layer guaranteed to be on time is the outermost one, the guest. The
gateway's early-return logic (returning the merged partial when its
deadline fires) already exists, but it fires a few milliseconds too late to
help. About 10% of all searches in the last day, and 15% since the
wave-grace fix, were lost this way.

**Provider calls are wasted.** Each capped search used a Tavily or Parallel
call whose results were discarded, and a ChatGPT call that ran for
30 seconds and was then cancelled. Because the router rotates over the four
eligible providers, ChatGPT lands in three of every four searches: wave 1
for two of the four starting positions and wave 2 for a third. It is
useful in about one: it contributed to 37 of 501 searches. In the trace
sample it was called 20 times and answered 5. One hypothesis, not
established: ChatGPT search draws on the same Codex account as research
and management inference. If so, wasted search calls compete with the
model calls behind the usage limit recorded in
[problems/chatgpt-usage-limit-stops-research-and-management-2026-10-11.md](../problems/chatgpt-usage-limit-stops-research-and-management-2026-10-11.md).

**The slow provider is never charged.** Since the provider and handler
limits are equal, ChatGPT's stalls end with the caller's context, and the
router records them as `caller_gone` (no strike). The health store keeps
calling it "active, 37 of 38". This is the third defect in the same family
as the cancel and empty-result bench records: provider health moving on the
wrong evidence. Here it fails to move on evidence it should count.

**Desks misreport.** The cell gets a transport error with no attempts. It
cannot tell "slow search, results existed" from "gateway down" or
"providers cooling". Research writes the wrong cause into its reports, for
example "timed out at the search gateway" in the P2a demo, and narrows its
own coverage on that false belief. Texture then rejects the source-less
report, as it should, and a research round is spent.

**Breadth is bought in the wrong place.** The 40-result floor asks one query
for 40 hits. Two of the four providers cap at 10, so the floor is what
makes the router wait for a slow third provider. Breadth that helps
research mostly comes from asking differently (entities, dates, primary
source sites, other phrasings), not from the 40th result for one phrasing.
Today each phrasing costs a separate cell call, up to 30 seconds, and one
of 64 egress calls.

**Depth, recursion and coverage are left to the model.** Nothing reads pages
in bulk, follows links, or checks what a plan said should be covered.
Children are one level deep and share one 64-call budget. The research
record [problems/research-leaves-leads-unresolved-and-unexplained-2026-10-11.md](problems/research-leaves-leads-unresolved-and-unexplained-2026-10-11.md)
shows the result: leads end as "unverified" with no plan to resolve them.

### Clustering (CLAUDE.md, Convergence Before Patching)

This is the fifth search-plane record in three days:
- cooldown on caller cancel (10-09);
- empty result benches healthy providers;
- rate limit benches for a day;
- wave waits for its slowest provider;
- guest timeout equals gateway timeout.

All five share two substrate causes:
1. **No single time authority.** Each layer has its own fixed timeout, and
   none knows the caller's.
2. **No honest attribution.** Outcomes are charged to provider health by
   string matching and context state, not by who actually ran out of time.

The design below targets those two causes. The next fix should be
substrate-level, not a sixth patch.

## 5. Design

### 5.1 Principles

- **One time authority per search.** The caller's remaining budget travels
  with the request, and every layer below takes its limit from what is
  left, minus a margin to answer. A layer's limit is always below its
  caller's. Each layer's answer at its deadline is a typed partial result,
  never a transport error.
- **Fast means first useful results early, not everything at once.** Return
  when there is enough, or when the soft deadline comes, with what has
  arrived. Slow providers either fit the budget or move to an async lane.
- **Broad means many queries, not many hits per query.** Fan out across
  query variants and providers together, under a per-provider concurrency
  cap.
- **Deep, iterative, recursive and comprehensive are research's job.**
  These are questions about what to ask next and when coverage is enough.
  The gateway should not guess them. It supplies fast, honest, budgeted
  primitives.
- **Honest attribution.** A provider is charged a timeout only when its own
  limit fired while its caller was still waiting.

### 5.2 Gateway: the search plane (fast, broad)

**G1. Deadline ladder and propagation.** The guest sends its budget, as
`budget_ms` in the body or an `X-Choir-Deadline` header. The gateway's
total is the smaller of (budget − 1 s) and a server cap (25 s). Provider
calls get min(lane limit, time left). The guest's transport limit is
budget + 5 s, so it always outlasts the gateway. Default budget: 20 s.
Code: `handlers.go:914`, `search_plane.go`, `router.go`, `search/search.go`.

**G2. Early return with a typed partial.** Each search has a soft deadline,
min(6 s, total/3). At the soft deadline, return if the merged list has at
least `min_results` (default 10). Otherwise keep waiting until the target
or the total deadline. The target (40) becomes something to fill when time
allows, not a reason to wait. The response gains:
- `status: complete | partial | outage`;
- `elapsed_ms`;
- `deadline_ms`;
- `pending` (providers still running when it answered);
- `reason` (`target_met`, `soft_deadline`, `deadline`, `providers_exhausted`).

The existing wave-grace logic is the special case "target met".

**G3. Latency-aware waves and lanes.** Keep a per-provider latency average
alongside health (same store). Wave 1 takes the fastest eligible providers.
Providers whose typical latency is above the soft deadline go in a slow
lane. They are called only when the caller's budget can absorb their
typical latency, and they never sit alone beside a 10-result partner. In
today's numbers ChatGPT is in the slow lane and the other four are fast.

**G4. Batch queries.** `POST /provider/v1/search` accepts `queries[]`, up to
about 8. All variants run concurrently under one deadline, with a
per-provider concurrency cap to avoid the rate-limit bursts in
[problems/search-rate-limit-benches-provider-for-a-day-2026-10-11.md](problems/search-rate-limit-benches-provider-for-a-day-2026-10-11.md).
The response holds a merged list per query plus one fused list across
queries (reciprocal-rank fusion, deduplicated by normalized URL). Breadth
then costs one cell call and one deadline instead of N of each.

**G5. Async deep lane (later).** Slow, high-value providers go behind a job
API: ChatGPT web search, and in future Parallel's task API or Exa research.
`POST /provider/v1/search/jobs` returns a job id, and the result is
collected later by poll. In the guest the result arrives as a runtime
observation the desk sees in `choir.Updates()`. The precommitment design
already treats timers and deadlines as runtime machinery rather than agent
speech (design §1, §6). The desk keeps working while a job runs.

**G6. Honest attribution.** Once provider limits sit below the caller's
deadline (G1), a provider's own timeout is a strike again. `caller_gone`
applies only when the caller's context ended before the provider's own
limit. This is `router.go:253-254`, with the ordering guaranteed by G1.

**G7. Observability.** Log one line per finished search with elapsed time,
status, reason, waves, and each attempt as provider:status:latency:results.
Flag whether the caller's context was already done when the response was
written. That one flag would have found this problem in a day, and it is
what the verifier below counts.

**Not in the gateway:** query rewriting, choosing what to read, recursion,
and coverage. The gateway knows nothing about the research question.

### 5.3 Guest host: reading (deep)

**H1. Typed results for every outcome.** The guest client turns its own
transport timeout into a typed result rather than an error:
`status: guest_deadline`, with "the guest stopped waiting after N s; the
gateway may have finished; this is not evidence of a provider outage". The
projection shows `status`, `reason` and `pending` from G2. Outages remain
the only case that shows provider health.

**H2. `choir.ReadMany(urls, budget)`.** Fetches a batch of URLs
concurrently (about 6 at a time) under one deadline. For each it returns:
- final URL, title and published date;
- the main text excerpt;
- the top outlinks, with anchor text;
- a `content_id` when the page was imported for selector reads.

This is the "read pages, follow links" primitive. It reuses the existing
fetch and import paths and charges the egress budget per URL. It must land
after the address guard in
[problems/research-fetch-url-has-no-address-guard-2026-10-10.md](problems/research-fetch-url-has-no-address-guard-2026-10-10.md),
because batch reading multiplies that exposure.

**H3. `choir.SearchMany(queries, budget)`.** The cell-side form of G4. It
returns per-query and fused lists as a value in the cell, so the model
filters with code instead of reading 40 snippets per query. This is the RLM
pattern: results are a variable, not context.

**H4. Budget by query, readable by the cell.** The egress ledger charges per
query and per URL. Its cap rises to match (64 calls is a few rounds once
batches exist). `choir.Budget()` reports what is left, so the cell can plan
a round.

### 5.4 Research desk (iterative, recursive, comprehensive)

**R1. Plan before searching.** Research writes a coverage plan for the
question: subtopics × facets (who, what, when, primary record,
independent confirmation), with the time window and the source types
expected. Under the precommitment protocol, this plan is the content of the
Cast's prediction. For example: "this round will find a primary record for
items 1-6; items 7-8 likely remain open". Each round's Report resolves
items, which makes coverage scorable rather than asserted.

**R2. Broad round.** For each open item, generate 3 to 5 variants: entity
names, date-anchored phrasing, `site:` restricted to the likely primary
source, a different phrasing, and another language where relevant. Send
them as one `SearchMany`.

**R3. Deep round.** Rank the fused results per item, preferring unseen
domains and primary sources. `ReadMany` the top k. Extract claims with
dates and quotes. Follow one hop of outlinks toward the primary record (a
press release, filing, docket or paper).

**R4. Iterate.** Names, numbers and new leads found while reading seed the
next round's queries. A lead becomes a plan item with a status:
- confirmed (with source);
- refuted;
- likely or unlikely (with the reason);
- open (with the next probe).

This answers the "unverified and unexplained" record. Report to Texture
after every round.

**R5. Recurse.** Plan items that stay open after a round go to
`choir.Research` children, one item each, with a carve-out of the budget.
Allow children one more level (depth 2). That is within the settled
D7b/M6.5 depth cap of 2 to 3 recorded in the fan-out definition. The
model-call gate from the fan-out record bounds the tree. Children return
items with statuses, not prose.

**R6. Stop on coverage, not on habit.** An item closes when it is resolved,
or when two rounds in a row add no new domain or claim. The activation
ends when every item is closed or the budget is spent. The final Report
carries the coverage table, items by status, and says which items are open
and why.

**R7. Read search status honestly.** Prompt line: a `partial` or
`guest_deadline` search is not an outage. Report provider trouble only when
`status` is `outage`, quoting `provider_health`.

### 5.5 Responsibilities

| Concern | Gateway | Guest host | Research desk |
|---|---|---|---|
| Fast | deadline ladder, soft deadline, latency-aware waves, partial results | typed timeouts | asks for short budgets per round |
| Broad | batch queries × providers, fusion, dedup, rate caps | `SearchMany` | writes query variants |
| Deep | async deep lane for slow providers | `ReadMany`, import, selectors | chooses what to read, follows links |
| Iterative | — | `Budget()` | results seed the next queries |
| Recursive | — | children runtime, budget carve-outs | splits open items into children |
| Comprehensive | — | — | coverage plan, item statuses, stopping rule |
| Honest | attribution, per-search log | typed outcomes in the projection | reports status, not guesses |

## 6. Changes and mutation classes

| # | Change | Where | Class |
|---|---|---|---|
| 1 | Guest search limit above the gateway's (30 → 35 s) | `internal/search/search.go:85` | orange |
| 2 | Gateway total below the guest's (30 → 25 s) | `internal/gateway/handlers.go:914` | red (gateway provider calls) |
| 3 | Per-provider limit default 30 → 10 s | `internal/gateway/searchplane/config.go:37` | red |
| 4 | Honest timeout attribution (follows from 2+3; test that a provider's own timeout strikes) | `searchplane/router.go:253` | red |
| 5 | Per-search log line with attempts and a caller-gone-at-write flag | `handlers.go:930`, `search_plane.go` | orange (log only); red if reviewers count any gateway edit as provider surface |
| 6 | Caller budget propagation, soft deadline, typed partial status | gateway + guest client | red + orange |
| 7 | Latency-aware waves, slow lane | `searchplane` | red (provider routing) |
| 8 | Batch queries + `choir.SearchMany` | gateway + yaegikernel + researchtools | red + orange |
| 9 | Async deep lane | gateway + guest Updates | red |
| 10 | `choir.ReadMany` (after the fetch address guard) | researchtools, yaegikernel | orange |
| 11 | Typed `guest_deadline` / `partial` in the projection | `researchtools.go:854` | orange |
| 12 | Coverage plan, variants, iteration, stopping rule, status reading | research prompt overlay | yellow |
| 13 | Research children depth 2 | `agentcore/research_children.go` | orange |
| 14 | Egress charged per query/URL, higher cap, `choir.Budget()` | `researchtools/egress_budget.go` | orange |

Changes 2, 3, 4, 6, 7, 8 and 9 are red and need the full ceremony. Rollback
for each is a git revert. The health store keeps any strikes the new
attribution records; those expire on their own, under the owner's rule of
no health-store edits.

## 7. The first change

**The deadline ladder: rows 1, 2 and 3 together.** Set per-provider 10 s,
gateway total 25 s, guest 35 s. That is three constants in three files,
with no new code paths. Expected effect, as predictions to check on staging:

- **No search outlives the guest.** The 51 capped searches per day
  (10% of all) go to zero. Every search returns something typed, and the
  "timed out at the search gateway" class of report disappears.
- **The tail gets much shorter.** With today's four providers: wave 1 runs
  at most 10 s, and wave 2 (Keenable and Parallel) about 2 s. So p90 should
  fall from 30 s to about 12 s. The 22 "Tavily alone, 10 results" searches
  should come back with about 40 results at about 12 s, because wave 2 now
  runs.
- **ChatGPT gets honest health.** Its calls will mostly hit 10 s while the
  caller waits. That strikes it into cooldown (2 min, doubling). It will
  then mostly sit out the synchronous plane, which cuts its wasted calls.
  This amounts to a routing decision, so it is stated here, not hidden. If
  the owner wants ChatGPT search kept, G3/G5 (slow lane or async lane) is
  how to keep it.

Why this beats the alternatives:
- **Guest-only change (row 1, orange, one line).** Recovers the discarded
  results, but searches still take 30 s, and ChatGPT stays invisible. Use
  this if the owner wants a non-red step first.
- **Gateway-only total (row 2).** Fixes the race but leaves the 25 s tail.
- **Lowering the 40-result floor.** Removes the wait but cuts breadth per
  query before batch variants exist to replace it.

How to verify on staging after landing:
1. From the gateway journal, count searches at or above 24.9 s with the
   caller gone at write. Target 0.
2. p90 search duration. Target 15 s or less.
3. Guest traces: no `context deadline exceeded` on web search.
4. Health store: ChatGPT's state and strikes now move.
5. Run one research-only demo and compare the source count against the
   P2a demo.

## 8. Open questions for the owner

1. **Should ChatGPT web search stay in the synchronous search path?** It
   takes about 23 s, answers in roughly a quarter of the calls it gets,
   contributed to 37 of 501 searches, and may share the Codex usage limit
   with research and management. The options are to drop it, move it to an
   async deep lane only, or keep it in a slow lane used only when the
   budget allows.
2. **How fast is fast?** Proposal: first useful results within about 5 s
   typical, and a hard 25 s per search call. Longer work goes through
   rounds, children and the async lane.
3. **What does one research question get to spend?** Wall time, searches,
   page reads, and model calls. The current 64 network calls per
   activation, shared with children, is a few rounds once batches exist.
4. **May research children recurse one more level** (depth 2) under a
   shared budget?
5. **Breadth: 40 hits per query, or several phrasings with 10 to 20 each?**
   This design assumes the second once batch queries exist, with 40
   remaining a target rather than a reason to wait.
6. **Should the paid providers be restored?** Serper is out of credits,
   SerpAPI out of searches, and Brave is on a rate-limited free plan. The
   plane runs on four providers until Oct 12 00:38Z, and on fewer after any
   further strike.
