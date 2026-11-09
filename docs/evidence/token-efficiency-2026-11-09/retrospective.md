# Retrospective survey: paper and d@e traces, 2026-11-08 00:20 to 2026-11-09 19:20 UTC

Slice: cross-cutting. Read-only. Every number is marked (M) measured or (I) inferred, with the command or script. Scratch data and scripts: `$SCR/eff/rt/` (`SCR=$SCR`). Scripts `p1.py`..`p3.py` (gateway), `a1.py`, `a2.py` (runs), `fetch.py` (run events).

## 0. Data reach, and what could not be reached

- Gateway journal on Node B starts 2026-11-08 00:19:54 UTC (M: `ssh node-b "sudo journalctl -u go-choir-gateway --no-pager -o short-iso | head -1"`). So the longest window is 43 h. `gw96.log` holds 139,337 matching lines. The journal has no run id, so gateway-only analysis cannot split by desk.
- `/api/costs` reaches only the last 10,000 events: 2,347 entries, 17:53 to 19:18 UTC (85 minutes) (M: `paper.sh '/api/costs?detail=1&limit=10000'`). It carries desk and run id, including the persistent management activations.
- `/api/runs` is capped at 500 runs: 17:00 to 19:18 UTC (2.3 h) (M). I pulled the events of all 500 (`fetch.py`) and summed `loop.progress`. The 8 persistent management runs in the list carry no model events (their calls are in the costs route under other run ids).
- Two joins are not possible with the routes I have: management-task trajectory to texture trajectory (different ids), and gateway call to desk older than 85 minutes. Where I extrapolate from the 85-minute or 2.3 h window to 43 h I say so and use a signature (section 2.1).
- Front page: `GET https://choir.news/api/platform/publications/front?limit=100` paged by cursor, 1,100 stories back to 11-07 18:46. It shows only each story's latest `updated_at`, so older hours undercount stories (a story re-updated later moves out of its earlier hour). The tokens-per-story figures for old hours are therefore upper bounds.

## 1. What the paper spent (M)

43 h, paper VM only (`vm-1669020a...`), successful calls (`p2.py`): 82,603 calls, 2,747.0M input tokens, 27.7M output (1.0% of input), 799M cached (29%). Average 64M input tokens an hour. Providers: openrouter (crouton-alpha) 1,852M, 2% cached; opencode-go 377M, 84% cached; synthetic 518M, 87% cached.

Other VMs in the same journal: `vm-70070cf7` 499.5M (7,942 calls, mean 63k input a call), `candidate-fleet-bf04...` 236.7M. These are self-development computers, outside the paper. d@e is covered in section 7.

Per-hour paper table (input M, fresh = input minus cached M) is in `p1.py` output. Highlights:

| span (UTC) | input M/h | fresh M/h | note |
|---|---|---|---|
| 11-08 01-02 | 89-104 | 21 | opencode-go, 76-80% cached |
| 11-08 04-13 | 39-94 | 39-94 | all openrouter crouton, 0-5% cached |
| 11-08 14-18 | 18-67 | 18-62 | publishing outage, see 2.3 |
| 11-09 01-12 | 61-86 | 16-57 | synthetic and opencode-go, 44-81% cached |
| 11-09 13-19 | 18-51 | 18-51 | back on crouton, 0-13% cached |

Fresh (uncached) input in total: 1,948M of 2,747M (71%). Fresh per hour is lowest when the router sends the paper to cache-reporting providers (16-35M/h) and highest on crouton (40-94M/h), even in hours where raw input is lower. The routing change to crouton (11-09 13:00) raised fresh tokens per hour.

### 1.1 Per-call size distribution (M, `p2.py`)

| input per call | calls | tokens M | share |
|---|---|---|---|
| under 5k | 6,261 | 25 | 0.9% |
| 5k-20k | 37,054 | 447 | 16.3% |
| 20k-50k | 24,170 | 719 | 26.2% |
| 50k-100k | 8,135 | 587 | 21.4% |
| 100k and above | 6,983 | 968 | 35.2% |

Percentiles of input per call: p10 5.5k, p50 19.0k, p90 89.9k, p99 183.9k, p99.9 242k. The top 1% of calls carry 6.5% of tokens; the top 10% carry 39.7%. The tail is thick, not spiky: 1,674 calls over 150k (past the 140k compaction threshold) carry 319M tokens (11.6%).

Output per call: 72% of calls output 250 tokens or fewer and carry 70% of input; 12% of calls (10,105) output 30 tokens or fewer and carry 15% of input (M).

### 1.2 Per-run distribution, 2.3 h window, 500 runs (M, `a1.py`)

Total 52.5M input, 0.88M output across the 500 runs. Persistent management activations are not in this total (they are in 2.1).

| role | runs | input M | share | p50 per run | p90 | max | calls per run |
|---|---|---|---|---|---|---|---|
| research | 79 | 32.4 | 62% | 403k | 714k | 1.25M | 18.8 |
| texture | 323 | 13.7 | 26% | 24k | 99k | 500k | 3.0 |
| management_task | 90 | 6.4 | 12% | 60k | 138k | 294k | 9.8 |

Gateway rate over the same hours is higher (about 40M/h at 17:xx) because of the management wedge (2.1). With the wedge excluded the paper runs at roughly 22M input tokens an hour (M: 85-minute costs window, 48.2M total minus 17.0M in four wedge activations, over 1.42 h = 22M/h).

### 1.3 Where a call's tokens come from (M, `a1.py`, `a2.py`)

For each run, "fresh" = first call plus every positive increase in input between calls; the rest is history resent.

| role | total input | fresh | resent | prefix (first call) resent N times | rest (carried tool output and history) |
|---|---|---|---|---|---|
| research | 32.2M | 2.4M | 93% | 41% (9.2k a call, 18.8 calls) | 52% |
| texture | 13.1M | 4.8M | 63% | 83% (first-call frame 13.7k a call, 3 calls) | 17% |
| management_task | 6.4M | 0.9M | 86% | 38% (3.0k a call, 9.8 calls) | 62% |

Overall for the 500 runs: about 84% of input tokens are a re-send of text the model has already been given in the same run. Output is 1.7% of input. Latency per call is 5.2 s (research), 6.7 s (texture), 4.9 s (management_task), independent of input size, so wall time is turn count, not token count.

Research tool-result carry (tokens of each tool result times the number of later calls that re-send it; chars/3.6 estimate, so an upper bound): Read and ReadCapture 28% of research input, SearchWait 21%, Records/Context/StoryAnalysis 8%, SearchStart 4%, ReportPacket 4%. Unique tool-result tokens are only 2.3M of 32.4M; the carry multiplies them by about 10.

## 2. Waste classes

### 2.1 Class W1: the management wedge is not new, it is 41% of 43 h (M for the signature, I for the attribution)

Signature: a paper call with input of 60k or more and output of 300 tokens or fewer. In the 85-minute costs window, every call of 50k or more is a management-desk call (M: 100% of 15.3M tokens in 50k+ calls are desk=management), and 629 of 667 management calls output under 300 tokens (M). Over 43 h (`gw96.log`): 10,120 calls, 1,119M tokens, 40.7% of all paper input match the signature. Research calls cannot reach 60k (max observed context 58.6k) and texture has about 2 calls above 60k per wake, so the signature is almost all management (I).

Trace evidence in the current window (M, `costs.json`, run `a475f115`: 62 calls, 5.14M tokens, 18:41 to 19:14, output 11 tokens on most calls; run `50fd0c3c`: input 11k, 17k, 23k, 29k, 34k, 40k, 45k ... +5.7k each call, output 7 to 11 tokens, started 19:15 and still running at 19:18). That is the same final-checkpoint loop the 11-09 evidence doc diagnosed (+5.7k tokens a call with a 7 to 11 token reply). Fix `7b4bbebd8` is on origin/main; it was not live at 19:18.

Cost of the class in the 85-minute window: wedge activations `a11122b6` 7.64M, `a475f115` 5.14M, `96a1e79a` 2.66M, `50fd0c3c` 1.57M = 17.0M of 48.2M (35%). Each activation produced a few thousand output tokens and no acts.

The full-day shape: signature share by hour ranges 15% to 65%; it peaks at 79-85% in 11-08 14:00 to 18:00 (section 2.3). So the wedge, or its earlier version, has been running for at least 43 h, not only today.

### 2.2 Class W2: resend of a growing transcript, with no cache (M)

The 84% resent figure (section 1.3) is paid in full on crouton because it reports 0 cached tokens on every call: 1,852M input, 0 cached (ratio 2%, M). On the same hours' other providers the same resend costs 15% of that (synthetic 87%, opencode-go 84% cached). The paper's fresh-per-hour moves with the router, not with the work (section 1 table). Same effect on d@e: engineering on DeepSeek is 95% cached, engineering on crouton 0% (M, `de-costs.json`).

### 2.3 Class W3: a publishing outage that kept spending (M for tokens and stories, I for cause)

11-08 14:00 to 19:00 UTC the front page shows no story updated in hours 14 to 19 (`front.json`; hours 14-19 absent from the updated-per-hour counts). The gateway shows the paper still spending: 45.7, 67.4, 51.5, 18.0, 32.5M input tokens in 14, 15, 16, 17, 18 (215M, 7.8% of the 43 h total) on 574, 774, 587, 201, 328 successful calls, mean input per call 80k to 90k, output 65k to 125k tokens an hour (100 to 170 per call). 79 to 85% of the tokens in hours 14 to 17 are the wedge signature. No stories published. This is the largest single "produced nothing" block in the data: roughly 215M tokens for zero stories. The cause of the stoppage is not in my data; the burn is.

### 2.4 Class W4: quota storms (M)

Gateway failures for the paper VM over 43 h (`p3.py`, `provider call failed` lines): 14,850 + 1,989 = 16.8k failed requests against 82.6k successes (about 17%). Classes: opencode-go circuit open (beam-free 3,860, iceman-contributor-free 3,151), openrouter 429 concurrency 2,727, chatgpt luna quota 1,500, recursive/flint free quota 1,307, nemotron 1,224, synthetic DeepSeek quota 1,144, all opencode keys parked 505+309, crouton circuit open 285. Failed requests cost no input tokens; their cost is latency and retry delay. Storm hours: 11-08 03-13 (circuit 277-651 an hour), 11-09 00 (566 concurrency 429s), 11-09 13-17 (circuit open 521-875 an hour).

Run-level effect, 2.3 h window (M, `a1.py`): 25 of 500 runs failed on "every model is out of quota after 2-8 retry rounds over 5 to 18 min": 17 texture, 6 research, 2 management_task. A further 6 research runs sit in `passivated` waiting. Tokens in failed or passivated research runs: 1.65M + 1.16M of 32.4M (8.7% of research). The largest: run `5ebe9731` spent 1.04M input tokens in 30 calls and died at iteration 52 with no completion (it did send interim packets). 17 failed texture runs spent about 0.04M tokens: the loss is not tokens, it is the wake: those documents did not get a revision. Retry events in the window: 1,059 `loop.retry` (reasons provider_quota 365, provider_quota_incident 248, provider_quota_retry 225, model_failover 156), 707 of them in the 17:xx hour. The tool loop holds a run for 5 to 18 minutes retrying before it fails.

Planned restart: 8 management_task runs cancelled with "interrupted by a planned restart; management tasks are not resumed" (195k tokens spent, their captures need re-leasing) (M).

### 2.5 Class W5: calls that end with a no-op (M)

Every research run ends with one `end_turn` call; there is no other `end_turn` in research, texture or management_task (stop_reason counts: research 1,361 tool_use / 64 end_turn; texture 914 / 0; management_task 811 / 0). So outside the management desk there is no ack-loop. The waste in these desks is carried context, not no-op calls.

### 2.6 Class W6: search results that have been seen before (M)

Gateway search lines for the paper over 43 h: 15,192 searches, 460,439 result URLs, 187,665 already delivered to this computer earlier in the day (40.8%), `served=miss` on 15,187 of 15,192, so the search cache did not hit (`p3.py`, `grep 'gateway: search for'`). About 353 searches an hour. Each search returns 40 URLs; the model reads and carries the summaries. In the 2.3 h run window, 680 SearchStart in 79 research runs, 136 repeated queries inside a run, and 14% of Reads (63 of 454) repeat a URL already read in the same run (M). Search providers were rate limited through most of 11-08 (`skipped=[brave=quota_limited parallel=... exa ... serper ...]`, one answering provider).

### 2.7 Class W7: Texture wakes that produce no revision, and frames that grow (M)

323 Texture runs in 2.3 h: 264 produced a revision; 59 (18%) did not, spending 1.32M tokens (9.7% of texture tokens) (`a1.py`). 30 of them completed normally with no revision. Per document: 144 documents, 2.2 wakes each. The first-call frame grows with each wake of the same document: 4.6k, 8.0k, 14.9k, 18.2k, 21.2k tokens; the largest chain went 34.7k, 41.2k, 47.2k, 54.2k, 60.9k, 69.6k tokens (M, run chain on doc `7b2ab9fa`; 8 wakes, 1.0M tokens, 6 revisions). Each wake re-reads the whole document plus every earlier record (cumulative, O(wakes squared)). Texture frames are 83% of Texture's tokens (section 1.3). The 15 Texture runs whose first call is above 40k carry 2.1M of 13.7M (15%).

### 2.8 Class W8: research continues after the first packet (M)

64 completed research runs: median 7 tool calls before the first `ReportPacket`, 11 after it, 19 total. Tokens in calls issued after the first packet: 20.6M of 29.4M (70%) (`a2`-style computation; first packet detected by `ReportPacket` in cell source). 163 packets across 79 runs (1 to 4 each, 44 runs have exactly 2), 199k input tokens per packet. Interim packets are by design (the cell says "send an interim packet now"), so this is not a defect; it is the cost of the design and no packet-value measure exists (open question 3).

### 2.9 Class W9: intake that re-derives a label Jev already gave (M for counts, I for avoidability)

`management_task`: 90 runs, 6.4M tokens (12% of the 2.3 h tokens, about 2.8M an hour), 9.8 calls each. A sample trace (run `...` index 5 in `a1.py` order): `Records()` → `Stories(20,"")` → `Stories(100,"")` (48,453 characters, 13k tokens, then re-sent on every later call) → `ReadCapture` → `Help("Feed")` → `Feed(...)` → `Done("Fed 1 capture ... as Jev labeled")`. 39 of 78 completed runs name Jev in their `Done` text. Outcomes in the 78 completed runs: Cast + Done 30, Feed + Done 20, Cast + Feed + Done 19, Decline + Done 4, mixed 5. Calls: Stories 182, Records 151, Cast 126, ReadCapture 122, Done 78, Help 69, Feed 62, Decline 10. 69 `Help` calls: the model reads documentation for a function it has used before (1.7k characters each, in a fresh session each run: "new session: nothing from this desk's earlier runs is defined").

### 2.10 Class W10: reasoning effort differs by desk (M, cause unknown)

`llm_reasoning_effort` on the calls: research "low" 1,425 calls; texture "high" 914; management_task "high" 811. Output per call is small in all three (texture 264, management_task 118, research 330), so the high setting does not show up as output tokens; whether the provider bills hidden reasoning as input latency is unknown (hypothesis, no trace).

## 3. Tokens per published story over time (M with caveat in section 0)

Gateway tokens per hour divided by stories whose latest version was updated in that hour (front page, 1,100 stories). Published stories per hour are flat at about 30 (range 3 to 43, median about 31). Spend is not.

| block (UTC) | paper input M | stories updated | M per story |
|---|---|---|---|
| 11-08 00-05 | 467 | 120 | 3.9 |
| 11-08 06-11 | 477 | 95 | 5.0 |
| 11-08 12-23 | 305 | 203 | 1.5 |
| 11-09 00-05 | 444 | 197 | 2.3 |
| 11-09 06-11 | 452 | 179 | 2.5 |
| 11-09 12-17 | 288 | 185 | 1.6 |
| 11-09 18-19 | 41 | 43 | 1.0 |

Stories per hour barely respond to a 4x swing in tokens per hour: output is bounded by intake rate and the two-outlet rule, not by tokens. Everything above the need is waste. Version counts: 397 stories at v1, 433 at v2, 147 at v3, 55 at v4 and up; source counts 2 to 9 (362 at 3). At the current 22M an hour excluding the wedge, one story-update costs about 0.7M input tokens; with the wedge it is 1.0M to 1.5M; the 11-08 morning was 5M.

## 4. Top of the tail, with traces

Top run-level consumers (2.3 h window): all ten largest runs are research, 0.67M to 1.25M each, 25 to 39 calls. Example `52fb37c2` (research, completed): 39 model calls, context 9k to 58k, 1.24M input, 8.6k output, `cache_reported` true with 0 cached. Sequence (M, events 3 to 231): 6 search rounds (4 to 8 parallel searches per cell, 40 results each), 11 reads, 3 packets; the context rises about 1.5k to 2.2k tokens a call because each cell returns 6k to 9k characters of output that stays in the transcript. Nothing pathological: this is the normal shape of a research run, which is why research is 62% of the non-wedge spend.

The pathological tail is outside the run list: management activations `a11122b6` (94 calls, 7.64M), `a475f115` (62 calls, 5.14M), `96a1e79a` (29 calls, 2.66M), `50fd0c3c` (22 calls, 1.57M, running). Each contains the +5.7k-per-call climb, compaction at about 115k to 140k, and a drop to about 31k to 37k, then the climb again (M, `costs.json`).

## 5. Ranked levers

Percentages are of the slice's raw input and of the paper's total at the current 43 h mix. "Post-wedge" means measured 22M an hour.

| # | lever | saving | paper share | quality risk | size | evidence |
|---|---|---|---|---|---|---|
| 1 | Stop the management final-checkpoint loop (landed as `7b4bbebd8`, verify live) and add an independent guard: a management activation that makes N consecutive tool-less calls with an identical last-user hash is closed with a visible fate | wedge signature 40.7% of 43 h (35% in the latest 85 minutes, 51% in the doc's 2 h window) | 35% to 41% of raw tokens, 0% of output | none: it produces no acts | done / small | section 2.1; verify with the 10-minute bucket query in section 8 |
| 2 | Use a caching provider for every long loop, or make crouton cache. 1,852M tokens ran at 2% cached; the same traffic at 85% leaves about 15% fresh. Even only for research and management_task, which are 74% of post-wedge | fresh tokens −50% to −75% of what remains; raw input unchanged | about 40% of fresh tokens, more if all desks | none to model quality; routing and quota change | config / router rule | section 1 table; d@e: DeepSeek 95% vs crouton 0% |
| 3 | Mask consumed tool output: after the model has used a SearchWait or Read result, replace it in the transcript with a one-line stub plus a handle (`CellOut`/`ReadCapture` can re-fetch it). Research carry is 50% of research tokens | research −35% to −45% | 20% to 27% of post-wedge | medium: the model may need the text again (re-fetch exists) | medium: toolloop transcript policy | section 1.3, `a2.py`; unique tool-result tokens 2.3M vs 32.4M total |
| 4 | Intake without a model when Jev already labeled: if the Jev label is feed-to-story-X or decline, do it in code; call the model only for open-new-story casts. `Stories(100)` plus `Help` plus `Records` are 3 of 7 calls | management_task −60% to −85% | 8% to 10% of post-wedge | low if the Jev label is trusted (it is already followed in 39 of 78 runs) | medium | section 2.9 |
| 5 | Texture: send the diff, not the full frame. A wake gets the new records plus the revision head; the document is a handle. Gate wakes that cannot change the document with a Jev "does this packet add a supported claim" label | texture −40% to −60% | 10% to 15% of post-wedge | medium: needs the document read path | medium | section 2.7: 83% of texture is the frame; 18% of wakes make no revision |
| 6 | Search: serve a cached/deduped result list (41% of returned URLs already delivered; `served=miss` on 99.97%); show only unseen URLs plus a count of seen ones; Jev-score results and pass the top 8 not 40 | research −8% to −12% | 5% to 7% of post-wedge | low | small to medium | section 2.6 |
| 7 | When every model is out of quota: stop dispatching new wakes rather than start runs that fail after 5 to 18 minutes; keep one probe | tokens −0 to −3%; fewer dead runs (25 of 500, 5%) | 0% to 1% tokens, up to 5% of throughput | none | small | section 2.4 |
| 8 | Run-end rule: a research run stops after its second packet unless the second packet's novelty label says new evidence | research −20% to −35% if 70% of tokens after packet 1 shrink by half | 12% to 20% of post-wedge | high until the packet-value measure exists | small | section 2.8; open question 3 |
| 9 | Prompt prefix: research 9.2k, texture first-call 13.7k, management_task 3.0k are resent every turn. Moving the prefix to a cached head (lever 2) or shrinking it | prefix is 41% of research, 83% of texture, 38% of management_task | folded into lever 2; shrinking by 50% saves another 15% of post-wedge | low | medium | section 1.3 |

Cumulative effect, rough and multiplicative on what each leaves: lever 1 takes 41% off raw; lever 2 reduces fresh by about 60% of the rest; levers 3, 4, 5, 6 together cut the remaining raw post-wedge input by 45% to 60%. A paper at 20% to 25% of today's raw tokens and 8% to 12% of today's fresh tokens is reachable with these alone.

### 5.1 What a 10x and a 50x version looks like

10x (fresh tokens, 20 to 25M an hour of post-wedge fresh load → 2 to 5M): levers 1, 2, 3, 4, 5 together. The wedge is gone, cached prefix hits 85%, consumed tool output is stubbed, intake is code where Jev decides, Texture reads the diff. Nothing requires a new model.

50x (the paper at 1 to 2M fresh tokens an hour): the information floor is the fresh content. Measured fresh content (first calls plus new tokens per run) is 8.1M for 500 runs in 2.3 h, or 3.5M an hour, with output 0.4M an hour. A 50x cut means the model is called once per story-update to write the draft, on a packet built by code. That structure: (a) intake and routing are Jev and code (no model per capture); (b) a research cell is a code pipeline: search (cached), fetch, dedupe, Jev passage-select; the model sees one packet of 3k to 6k tokens of selected passages, not 19 turns; (c) Texture is one call per story-update with the diff and the packet, writing a section; (d) wakes are gated by a label. That is a redesign in which "agent loop" is replaced by "pipeline with one model call at the end", and quality risk is real (the exploratory fetch-and-react turns are what finds the second outlet). The tier between 10x and 50x is 20 to 25x with a loop capped at 4 turns per research run and a cached prefix.

## 6. Jev / classifier opportunities

Rates are from the 2.3 h window (M) unless marked. Jev is already called about 172 times (`system1.judgment` events) in that window; the rate is small (1.4k to 2.3k tokens each).

| decision currently made by a model loop | model cost now | rate | Jev shape |
|---|---|---|---|
| Intake disposition: feed to story X / open new story / decline | management_task 6.4M per 2.3 h, 90 runs | 38 runs an hour; 3.4 captures an hour | one batch per sweep: question `disposition` over (capture, top-5 candidate story heads) pairs; run all pairs in parallel; code does the Feed or Decline |
| Does this record/packet change the document (Texture wake gate) | 59 wakes, 1.3M, no revision (10% of texture) | about 26 an hour | question `adds-supported-claim` over (packet claims, current head), before waking Texture |
| Is more research needed after packet n | 70% of research tokens come after the first packet (20.6M per 2.3 h) | about 71 packets an hour | question `new-evidence?` over (packet n, packet n-1) in a batch; stop or continue in code |
| Search result relevance and novelty | 40 results a search, 41% already seen, 353 searches an hour on the 43 h gateway | 353 an hour | one batch per search: 40 (query, snippet) pairs scored in parallel; pass top 8 unseen |
| Read result: which passages answer the question | Read/ReadCapture is 28% of research tokens carried | about 200 reads an hour | batch per read: each paragraph vs question, keep the labeled ones; the model sees the selection plus a handle |
| Publish-rule precheck (two outlets, disclosure) | held revisions: 13 of 18 attempts in the other evidence doc | not re-measured | label before spending the Texture call: distinct outlets count is code; disclosure is Jev |
| Compaction | summary call at 115k to 140k input in the wedge; 1.5k to 2k output | about 10 an hour in wedge hours | deterministic stub of old tool output (lever 3) instead of a model summary |

A batch of 40 pairs at 1.4k to 2.3k tokens is about 80k input tokens at $0.042 per million, about $0.003, in parallel.

## 7. d@e (computer-59c902c5...)

Window: 11-08 22:42 to 11-09 19:07, 20 h (M, `de-costs.json`, 3,120 calls). Input 147.2M: engineering 104.4M (71%), management 22.8M, research 7.1M, texture 10.7M. Eighteen engineering assignment runs; the largest: 24.7M (173 calls), 13.6M (124 calls, 14.6k output tokens total, 0.1% of input), 11.5M, 9.9M, 8.1M. Median input per engineering call 57k, maximum 240k; context rises to 120k with no stable compaction. Cached: DeepSeek on Synthetic 95%, iceman 95%, flint 98%, crouton 0% (34.3M engineering tokens and 11.6M management tokens on crouton at 0% cache). Failed runs: 5 engineering and 2 management failed, 9 texture cancelled. d@e is a drill and build computer; the levers 2 and 3 apply directly: an engineering assignment is a single long transcript with a 5k to 24k token carry per call, and the router decides whether it is billed fresh.

## 8. Commands (all read-only)

- Gateway: `ssh node-b "sudo journalctl -u go-choir-gateway --since '-96h' --no-pager -o short-iso | grep -E 'inference succeeded|provider.*(429|quota|circuit|failed)|inference failed'" > gw96.log`; parsed by `p1.py`, `p2.py`, `p3.py`.
- Runs: `paper.sh '/api/runs?limit=500'`; events `paper.sh '/api/runs/{id}/events?limit=1000&after=N'` for all 500 (`fetch.py`, 4 parallel); `a1.py`, `a2.py`.
- Costs: `paper.sh '/api/costs?detail=1&limit=10000'`, `de.sh` same; bucket by 10 minutes and desk for the wedge check:
  `python3 -c "import json,collections;e=json.load(open('costs.json'))['entries'];b=collections.defaultdict(collections.Counter);[b[x['Timestamp'][:15]+'0'].update({x['Desk']:x['InputTokens']}) for x in e];[print(k,dict(b[k])) for k in sorted(b)]"`.
- Front: anonymous GET `https://choir.news/api/platform/publications/front?limit=100&cursor=...`.

## 9. Open questions and the probe that would settle each

1. Is the 43 h wedge signature all management? Probe: read `/api/costs` after the fix deploys and compare the 100k-plus share per hour; the signature should fall under 5%. A pre-fix confirmation would need run-event retention longer than 10,000 events; not available.
2. Is the 11-08 14:00 to 19:00 outage one cause? The tokens are measured; the cause (providers, deploy, wedge only) needs the deploy and journal for that window (`journalctl -u go-choir-vmctl`, deploy log). I did not read them.
3. Value of later packets. Probe: for the 64 completed research runs, diff the final Texture revision against the one made after packet 1 (which claims, sources and the publish decision changed). No such join exists in the routes I can reach.
4. Does the crouton upstream cache but not report it? Probe: compare `provider_call_ms` for a repeated 60k-token prefix on crouton across calls; latency collapse would indicate caching. Free to measure from existing events: mean 5 to 7 s regardless of input size, which weakly supports caching being active but unreported (hypothesis).
5. Why do texture and management_task run at "high" reasoning while research runs at "low"? Probe: run config in `model-policy.toml` and a per-call reasoning-token field in the provider response, if the gateway logs one.
6. Management_task trajectory vs texture trajectory join (to price one story end to end). Probe: add the originating capture id to texture and research run metadata; in the data I have, 90 management_task runs share no trajectory id with any texture run.
