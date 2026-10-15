# Autopaper compute halving (2026-10-15)

Status: measurement done (§3), levers ranked (§3b), three levers
implemented behind flags (§5), staging measurement plan (§6). This file is
green (docs only); the code commits it describes carry their own classes.

Owner request (2026-10-15 ~05:20 UTC): halve the compute and token use of
Autopaper, keep ingesting every source, keep the most important stories,
and if possible raise quality at half the compute.

## 1. Baseline (paper computer, Node B gateway journal)

Paper guest `vm-1669020adcece51c2f5ff9b6df399ef5`, computer
`computer-1355b83bced1613eb048ee28d66688a8`.

Prompt caching came on at about 2026-10-14 13:00 UTC (coordinator check:
0% cached on both providers before then). Use the window from 13:00 on.
opencode-go was down 19:00–23:59 on 10-14 (402, out of funds), so the paper
ran on chatgpt only in those hours.

Window 2026-10-14T13:00 to 2026-10-15T05:20 (about 16 h), paper only:

| Provider / model | Calls | Input (M) | Output (M) | Cached | Avg input/call |
|---|---|---|---|---|---|
| opencode-go / deepseek-v4.1-flash | 7,485 | 217.0 | 10.92 | 89% | 28,987 |
| chatgpt / gpt-6-luna | 7,742 | 109.7 | 2.37 | 29% | 14,166 |
| opencode-go / cat-jam-free | 1,912 | 41.1 | 1.94 | 89% | 21,470 |
| total | 17,139 | 367.8 | 15.2 | 71% | 21,460 |

The chatgpt model is not in the gateway's provider line; the guest's
`/api/costs` names it `gpt-6-luna`. chatgpt's cached share rose from
16–31% on 10-14 to 36–47% in 10-15 00:00–05:00; opencode-go holds 84–91%.

Input per call, opencode-go: p50 18k, p90 64k, p99 117k, max 155k. Calls
above 32k input are 29% of calls and **63% of input tokens**. chatgpt: p50
10k, p90 26k; calls above 32k are 7% of calls and 26% of input.

Cache-weighted cost needs a cached-token price ratio. **Assumption** (not
verified on the billing pages): cached input costs about 0.1x on both
providers. At 0.1x the window's effective input is about
217×0.2 + 41×0.2 + 110×0.74 ≈ 133M uncached-equivalent tokens, of which
chatgpt is ~61% while carrying 30% of raw input.

## 2. Breakdown by desk (guest `/api/runs`, last 500 runs, 02:14–05:19 UTC)

The run list caps at 500 runs (about 3 h). Token counts are run metadata
(`input_tokens`, `cached_input_tokens`); 447 of 500 runs carry them.

| Desk / run kind | Runs | Input (M) | Share | Input per run p50 / p90 |
|---|---|---|---|---|
| research ("inbox has new records from Texture") | 66 | 33.9 | 53% | 180k / 1.1M |
| texture (`integrate_execution_findings`) | 273 | 20.1 | 32% | 31k / 163k |
| management (inbox records) | 89 | 6.8 | 11% | 46k / 198k |
| management_task ("delegated captures") | 72 | 2.6 | 4% | 36k |

Findings so far:

1. **Research is half the input, in a few runs.** The top 5 of 66 research
   runs carry 53% of research input; the two largest took 6.4M and 5.7M
   input tokens each (deepseek, reasoning effort low, ~12 min). At ~29k
   average per call that is ~200 calls in one run, with context growing
   turn by turn: the long-context bucket (above 32k per call) is the
   history replay of these runs.
2. **Texture is a third**, mostly short runs, with a heavy tail (one run
   2.0M input).
3. Search: ~4–5k Keenable calls/day, ~9 searches per research run,
   `max_results=40` on every search (gateway journal). How much of each
   research turn's context is search-result text is **not yet measured**.

## 3. Measurements that fix the ranking (2026-10-15, staging reads only)

Sources: guest `/api/costs?detail=1` (per-call cached counts, 05:11 to 05:31
UTC, 273 calls), `/api/runs/{id}/events` of the two largest research runs
(`loop.progress` input per iteration, `tool.result` output size), the public
front (`/api/platform/publications/front`). No story bodies were read.

**A. Where a research run's input goes.** Two runs, 98 and 87 iterations,
6.35M and 5.74M input tokens. Per iteration: the static prefix (system plus
tools) is 5.1k tokens, 8% of the run's input. Replay of earlier tool
results is **75%**: results are capped at 8,000 characters (about 2k
tokens), median 0.8–1.0k, and each is resent on every later call. A result
that prints search hits (a quarter of all cells) is at the cap, median
2.0k, and is **29%** of the run's input; other cell output is 47%; the
model's own earlier turns are about 17%. A run is about 90 iterations, so
the average result is replayed about 45 times.

**B. Stories/day.** About 410 front stories were published or revised in
24 h (median 3 revisions, 192 words, 2 sources). Of the 100 that carry an
importance score, 36 score 1–3 and 62 score 5 or more. The score arrives
after publication (timer every 10 minutes), so nothing triages before
research. About 1.3M input tokens per published story per day.

**C. chatgpt cache, per call (gpt-6-luna, 200 calls).** Cached tokens on a
**later call of a run equal the desk's static prefix and stay there**: a
research run shows 3,584 cached on 12 consecutive calls while its prompt
grows from 5.2k to 19.4k; texture 2,560 (system plus tools); management
5,632. Calls are 2–10 s apart and the request bytes are an append-only
extension of the previous request (checked: the new test
`TestChatGPTConsecutiveRequestsShareEveryByteUpToTheNewTurn`). Occasional
large hits (15,872 of 20,062; 13,824 of 15,076) appear on about one call in
six. That is the signature of calls landing on different cache machines: the
static prefix is replicated everywhere, the history lives on one. It does
not depend on load (hit share of the previous prompt is 0.54 at 0 other
calls per minute and 0.36–0.40 at 30+). `prompt_cache_key` is sent and
stable, so it is not enough on this backend. The Codex CLI, the reference
client, also sends `session_id` and `conversation_id` headers equal to the
thread id; this provider sends neither (`doStreamRequest` sets only
Authorization and Accept). **Hypothesis**, to be confirmed by the staging
after-measurement in §5: the backend routes by those headers.
First calls of a run (7% of luna calls) cache the static prefix only,
by design: the run-context turn carries run-unique facts, so coarsening its
time would not help and is not done.

**D. Elision arithmetic (simulation on the two runs' real per-iteration
sizes).** Keep the last N tool results verbatim and replace older ones with
an 80-token stub, in batches of B so that the stable prefix is rewritten
once per B calls instead of every call (a sliding window rewrites the whole
history every call and **costs more** cache-weighted: 165% at N=8, B=4).

| N / B | raw input | cache-weighted, cached at 0.1x | cache-weighted, 0.2x |
|---|---|---|---|
| 6 / 16 | 61–64% | 89–99% | 74–81% |
| 6 / 24 | 64–66% | 83–89% | 73–77% |
| 4 / 30 | 64–70% | 84% | 74–77% |

Capping search-hit output at 700 tokens at write time (no rewrite, so no
cache cost) gives 87–88% of the run's input, raw and cache-weighted.

## 3b. Ranked levers

Shares are of the 16 h window (368M raw; about 133M cache-weighted at the
**assumed** cached price of 0.1x on both providers, not verified on the
billing pages). Research is 53% of raw input.

| # | Lever | Raw input | Cache-weighted | Risk | Status |
|---|---|---|---|---|---|
| 1 | chatgpt cache routing (session headers, hit rate 29% to about 85–90%) | 0 | **−44%** (chatgpt 81M to about 23M of 133M) | none to quality; provider request shape (red) | implemented |
| 2 | Rolling elision of old tool results, batched (N=6, B=24) | −18 to −19% of all (−34 to −36% of research) | −11 to −17% of research at 0.1x, −23 to −27% at 0.2x | medium: the model may need an elided result | implemented, default on at a high threshold |
| 3 | Search hits in context: 15 results, 500-character snippets, each hit printed once | −6% of all (−12% of research) | −6% of all | low | implemented |
| 4 | Bound research runs: about 90 iterations per run, top 5 runs are half of research input; turn or token budget per run | up to −13% of all at the p90 | similar | quality: cut-off research | not implemented, needs a measured cap |
| 5 | Importance triage before research (about 35% of scored stories are 1–3 yet each costs the same 1.3M) | −10 to −15% if the lowest third skip research | same | quality: needs a pre-research score; the 10-minute scorer runs after publication | not implemented |
| 6 | Texture wake coalescing (273 runs in 3 h, 32% of input) | −5 to −10% | same | medium | not implemented |

Reading: **cost** (cache-weighted) can be roughly halved by #1 plus #2 and
#3 (about −50%); **raw tokens** need #4 and #5 as well, because #2 and #3
together give about −22% raw. The stories are all still ingested (no change
to polling, capture or dedupe), and the most important stories get the same
research as before: #1 to #3 change cost per call, not what a story is given.

## 3c. Code map (read-only sweep, not yet re-verified line by line)

- **Poll → management.** Polls make no model calls (20 new captures per
  poll by default, 15 min interval; `researchtools/poll_source.go`). Each
  poll sends one captures record to management (`capture_arrival.go`).
  Management, or one management task per poll when `intake.labels` is on,
  opens, feeds or declines each capture in a multi-turn loop.
- **No triage gate and no cap on stories opened.** Dedupe is exact only
  (canonical URL plus block hash). A Qdrant dedupe threshold exists in
  config but nothing reads it (`provideriface/config.go:71`).
- **Story fan-out.** Every story gets its own Texture desk and its own
  research desk. `choir.Research(children)` has no cap on the number of
  children: 4 run at a time, each a full multi-turn loop
  (`research_children.go`).
- **No turn cap.** The tool loop runs `for i := 0; ; i++`
  (`toolregistry/toolloop.go:446`); runs are bounded only by time (60 min
  activation, 10 min research window).
- **Full history replay.** Every call replays every earlier cell output
  (`run_memory.go`). Compaction triggers only at 70% of the window, about
  140k tokens, which explains the 64–155k calls. On OpenCode the replay
  also includes `reasoning_content`.
- **Context sizes.** Cell output is cut to 8,000 runes, but a
  bare-expression value can show up to 48,000 bytes. Search hits keep
  untruncated snippets (Exa returns full page text). A printed `Search`
  holds each hit twice (`Arrivals[].Results` and `Results`).
  `max_results=40` is set in `agentcore/desk_search.go:34`.
- **Prompt caching.** The cache key is per agent scope (one per document
  or desk) and covers system plus tools. chatgpt gets `prompt_cache_key`
  but no `previous_response_id`, so each call resends the full history.
  The second message carries a timestamp to the second, plus the rewarm
  summary with the earlier run id, so a new run caches only the system
  prompt and tools.
- **Model per desk.** Persistent management has no trajectory, so its
  model is drawn again on every run (`runtime.go:4464`).
- **Levers this map points to:**
  1. Compact earlier than 70%, or summarize cell outputs older than N turns.
  2. Cap per-snippet size and the results shown per search.
  3. Add a soft cap on turns or calls per run, and on research children.
  4. Move the run-context timestamp after the stable prefix, or make it
     coarser.
  5. Gate story opens on importance (the intake labels path already makes
     a Jev call per capture).

## 4. Next steps (for the successor)

1. Per-turn growth of a big research run: `GET /api/runs/{id}/events` on
   the guest (from Node B: `curl -H "X-Authenticated-User: <owner>"
   http://<guest-ip>:8085/...`; guest IP from `journalctl -u
   go-choir-vmctl | grep 'booted VM vm-1669020a'`). Runs to read:
   `e7955dc9-f42a-492a-a5de-49bd0f665d96`, `64a3a0c6-6657-46ea-9f8f-b49f739e6724`.
   Split each call's input into system prompt, history replay and cell/tool
   results (search text).
2. Map stages in code: `internal/agentcore/source_schedule.go`,
   `capture_arrival.go`, `research_children.go`, `story_publish.go`,
   `internal/frontimportance`; prompt assembly and history replay in the
   desk loop; search result shaping (`desk_search.go`, `internal/search`,
   yaegikernel search store).
3. Count stories/day published, started-but-dropped, merged, duplicates.
4. Confirm the lever ranking with that arithmetic, then implement the top
   1–2 behind flags (orange), failure modes listed before tests.

Rollback: docs only; `git revert`.
