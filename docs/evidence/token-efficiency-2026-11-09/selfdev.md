# Token efficiency audit, slice "selfdev" (2026-11-09)

Scope: self-development on d@e (computer-59c9...) and b@c (computer-6c50...), read-only. Engineering desk runs, capsule_go_eval cells, the Claude delegate, verifier calls, theme rounds, candidate/apply loops, model policy roles. Budget under test: River about $1/day for self-dev (internal/gateway/paid_budget.go, `defaultRiverDailyUSD = 1.0`).

Tags: **[M]** measured, **[I]** inferred from measured numbers, **[H]** hypothesis, no trace yet.
Raw data: `eff/de_costs.json`, `eff/bc_costs.json` (GET `/api/costs?detail=1&limit=10000`), `eff/sd/` (d@e engineering run events), `eff/sdb/` (b@c theme-round run events), `eff/sd/gw14.log` (Node B gateway journal, 14 h). Event dumps are paginated with `eff/fetchall.sh` (the run-events route returns 200 events by default; use `?limit=1000&after=<next_after>`).

## 0. Headline

- A b@c theme round (a data-only desktop theme: one JSON file of 37 colour tokens plus a manifest) costs **100-143 DeepSeek calls, 10.4-14.8M input tokens (95% cache-read), 0.10-0.17M output tokens, $0.64-$1.32 at DeepSeek list rate, mean $0.94 over 6 rounds [M]**. At the River budget of $1/day that is one round a day. Engineering is 98-99% of a round's tokens; Texture (5-20 calls) and management (1-4 calls) are about 1% [M].
- The round is two thirds waste by construction. 51% of its input tokens are spent before the Claude delegate is briefed (the DeepSeek engineer explores the repo, then pastes the finished content of theme.json into the delegate brief), and 44% after the delegate returns (the engineer re-runs the delegate's builds, gates and shots, because "only your own receipts are evidence"). The delegate itself, which does the actual work, cost $0.10-$0.28 list per run on the owner's Claude plan [M].
- Three more measured leaks: isolated cold cache misses are 28% of dollars; Poll-only turns are 8% of turns and 11% of tokens; hidden reasoning is about 85% of output tokens.
- A lean self-dev round (delegate-first, no duplicate exploration, one combined gate job) is about 14 DeepSeek calls and **$0.04-0.06 on River, about 16-23x cheaper** [I]. A deterministic theme-pack path is about $0.01-0.02, about 50-90x cheaper [I].

## 1. What the slice spends

### 1.1 Totals

| scope | window | engineering | all desks |
|---|---|---|---|
| d@e | 2026-11-08 22:42 to 11-09 19:07 UTC (`de_costs.json`, 3120 calls) | 1455 calls, 104.5M input, 0.92M output; 18 assignments | 147M input over the day, DeepSeek-equivalent $5.05 [M] |
| b@c | 2026-11-09 (`bc_costs.json`, 2143 calls) | 1830 calls over the day (all desks 236.7M input), $7.76 DeepSeek-equivalent | 8 theme trajectories |

[M] Commands: `$SCR/de.sh '/api/costs?detail=1&limit=10000'`, `$SCR/bc.sh ...`, grouped in Python by Desk, Model, RunID, TrajectoryID.

Who actually pays. The `selfdev` pool (internal/modelcatalog/catalog.go `SelfDevPool`) is crouton-alpha (free, cached=0), then the free opencode-go and OpenRouter models, then Synthetic DeepSeek, then River DeepSeek, all fallback-only after crouton. River has spent $0.00 so far (gateway line `spend today 2026-11-09 bedrock $0.00/20 river $0.00/1` [M]). The dollar columns above are the DeepSeek list rate ($0.60/M uncached input, $0.03/M cache read, $1.20/M output, fitted from `Cost.USD` against token counts [M]; same table prices River as an estimate, internal/llmcost/pricing.go). They are the River bill the same work would run up once crouton's free window closes (pricing.go says about 2026-11-10) and Synthetic is spent. Today b@c ran 122M input tokens on crouton at $0 and 102M on Synthetic DeepSeek; d@e ran 34M on crouton, 59M on Synthetic, 11M on opencode-go free [M].

### 1.2 Anatomy of one theme round (b@c, 6 rounds on DeepSeek, calls matched by trajectory)

| round | calls | input | cached | output | $ list | cold-miss $ | trajectory status |
|---|---|---|---|---|---|---|---|
| c0b5687d OLED Black | 101 | 10.38M | 95% | 101k | 0.71 | 0.16 | settled |
| 79bd0043 Vector Heart | 125 | 12.27M | 96% | 158k | 0.82 | 0.13 | settled |
| 4b9398fc Neo Vector Heart | 111 | 10.71M | 97% | 118k | 0.64 | 0.11 | live (never settled) |
| 5342f74b Metal Heart | 110 | 11.59M | 93% | 151k | 0.97 | 0.31 | live |
| f8778238 Utopian Scholastic | 143 | 14.48M | 93% | 139k | 1.18 | 0.39 | live |
| 23e98367 Aero Heart | 133 | 14.83M | 92% | 172k | 1.32 | 0.48 | settled |

[M] `bc_costs.json` entries joined by TrajectoryID; status from `GET /api/trajectories/<id>` (`bc.sh`). Settlement failures follow learnings #15/#16 (package apply minted no owner-act record; fixed per the coordinator).

Per-call shape (c0b5687d event dump, `eff/sdb/ev_c0b5687d.json`) [M]:
- Iteration 1 input is 13.1k tokens: system overlay (`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml`, 30 KB, about 7.5k tokens), the one tool schema (`capsule_go_eval`), the assignment packet. First-call size is 10.6-15.7k across all 24 engineering runs I dumped.
- Input grows from 13k to about 150k over about 100 iterations; mean about 100k. New content per call is about 1.0-1.5k tokens (median delta 981). Final transcript is roughly 65% tool output (350k chars of cell stdout per round), 20% model-authored cell source, the rest prompt.
- Cost split over the 6 rounds ($5.64 total): cache reads $2.10 (37%), uncached input $2.53 (45%), output $1.01 (18%). Of the uncached $2.53, $1.59 is **cold misses**: 23 calls out of 723 where cached/input < 20% on a prefix over 20k tokens. Each re-bills the whole 70-165k prefix at 20x the cache price.
- Output: 100-172k tokens per round, but visible text plus cell source is only 17-25k tokens, so **about 85% of output tokens are hidden reasoning** even though the event says `llm_reasoning_effort: low` [M for event field; the hidden share is inferred: 3.5 chars/token]. Roughly $0.12-0.17 per round.

### 1.3 Phase split (iteration of first `choir-delegate start`, last delegate touch; input tokens summed) [M]

| round | pre-delegate | delegate window | post-delegate | iterations |
|---|---|---|---|---|
| c0b5687d | 5.28M (51%) | 0.51M | 4.58M (44%) | 101 (delegate brief at 66) |
| 79bd0043 | 7.03M (57%) | 0.69M | 4.55M | 125 (brief at 91) |
| 4b9398fc | 5.99M (56%) | 1.02M | 3.69M | 111 (brief at 78) |
| 5342f74b | 6.21M (54%) | 0.68M | 4.69M | 110 (brief at 75) |
| 23e98367 | 7.07M (48%) | 1.66M | 6.10M | 133 (brief at 87) |
| f8778238 | 6.88M | delegate re-briefed twice at 93/94, no post phase | | 143 |

What the delegate costs. The OLED delegate run: 26 turns, 25 tool uses, 400 s, `total_cost_usd 0.279`, usage 44 input, 43.7k cache-creation, 564.7k cache-read, 11.3k output (thinking 2.2k). The Aero Heart delegate run: 8 turns, 67 s, $0.103, 20.7k cache-creation, 97k cache-read, 4.1k output [M, `choir-delegate wait` output inside the engineer's cell results]. Gateway relay lines for b@c show 6-32 relay calls an hour, 2 uncached input tokens each, 0.05-1.3M cache-read an hour [M: `grep 'anthropic relay computer-' sd/gw14.log`]. The delegate is the cheap, effective part: `daily_tokens_used` is 429k on b@c and 41k on d@e against a 50M cap [M: `/api/runtime/observability`].

The brief the engineer writes is the finished work: the OLED brief enumerates all 37 colour keys and values; the Aero brief carries the full palette "validated: every gate pair below passes WCAG". So the DeepSeek engineer already did the design, then paid ~45% of the round again to re-prove what the delegate had run.

### 1.4 d@e engineering (18 assignments in 21 h, mostly acceptance drills)

[M] `eff/sd/`. Tokens by assignment 0.3M-24.7M; no delegate use except one run (14 relay calls at 07:00). Same shape: iterations 17-173, input 11-15k at start, up to 160k at end. Cells: 1397 tool results, 53 with an interpreter error (yaegi: `reflect.Value.Slice: string slice index` 14, `import` refused 8, undefined methods), 129 Poll-only turns.

Repeat work on the same outcome:
- One FRED report: 5 assignments (a713 failed 1.3M, 9d8e failed 7.8M, 892a failed 3.0M, 246a done 6.7M, 70dc "clean re-freeze after rejection" 11.5M) = 30.3M tokens, $2.09 list.
- textstats: 3 assignments (cancelled 0.5M, retry 5.2M, "rebuild fresh, do not resubmit rejected candidate" 8.1M).
- prompt bar: two assignments for the same owner words. 03:09 on crouton: 49 calls, 2.0M. 07:00 on DeepSeek: 173 calls, 24.7M, $1.31, 12x the tokens.
- menu-default: 3 assignments (1.5M, 2.9M, 13.6M); mailnote footer: 2 assignments, the second "package-only round" because the first candidate's frozen diff included `scripts/choir-shot.mjs` (a platform path).
- Retry-shaped assignments (prompt says retry, rebuild, re-freeze, fresh attempt, round 2, package-only round) total about 31.7M of 104.5M (30%); failed or cancelled runs total 12.6M (12%) [M by prompt text and run state; classification mine].

### 1.5 b@c failure round

9a29063b (the first OLED attempt, 03:17-04:58): **734 calls, 116M input tokens** (528 on crouton with no cache, 146 DeepSeek, 40 beam-free), two attempts refused by the same platform defect (data-only pack refused by `check-packages`; scratch `themes/learnings.md` #1). That is 11x the tokens of the successful retry (10.4M) and nothing stopped it: there is no per-assignment token budget [M for tokens; cause from learnings.md]. $0.95 of DeepSeek-equivalent landed on top.

### 1.6 Verifier and verify_multimodal

No cost entry on either computer carries a `verifier` desk; `by_desk` shows conductor, engineering, management, research, texture, unattributed only [M]. Shot judging is management's (`choir.ShotStart/ShotWait/ShotVerdict`, desk_shots.go) and per round management has 1-4 calls (about 40-150k tokens, crouton); shot images do not enter the engineer transcript [M for call counts; the no-image claim is from absence of any image_ref in the engineering cells I read, so [I]]. `verify_model_capability` is never called in these 24 engineering runs [M]. So verifier spend in this slice is below 1%; there is nothing to cut here.

## 2. Waste found

| # | waste | evidence | share of a round |
|---|---|---|---|
| W1 | Engineer explores the repo for 50-90 iterations before briefing the delegate, which then explores the repo itself | c0b5687d iterations 3-65: greps and `sed -n` of theme.ts, App.svelte, check-packages.mjs, docs, 12-21k char outputs; delegate brief at 66; 5.28M of 10.38M tokens before it | 48-57% of tokens |
| W2 | Engineer re-runs what the delegate already ran (gates, tests, shots), by contract ("only your own receipts count") | c0b5687d 82-96: `go test ./internal/apppackage` x3, check-packages x4, playwright spec, gate.mjs; post-delegate 4.58M; the delegate's own result already says "every check passes" | 36-48% of tokens |
| W3 | Cold cache misses | 23 of 723 calls cached<20% of >20k prefix, each a lone miss followed by 0.98+ on the next call (23e98367 call sequence `0.99 0.0 0.98 0.13 0.97` and `...0.99 0.0 0.99 0.85 ...`); same provider, gap 5-70 s | $1.59 of $5.64 (28% of dollars) |
| W4 | Poll-only turns | 56 of 723 turns on b@c (7.7%) carry 7.94M of 74.3M input (10.7%); d@e 129 of 1432, 11.3M of 104M. The overlay says "end your turn, you are woken", yet models call `choir.Poll` in a loop (23e98367 iterations 82-84, 103, 106, 113, 117, 122, 124) | 8-11% of tokens |
| W5 | Hidden reasoning | 82-88% of output tokens in all 6 rounds; output is 18% of dollars | about 14-16% of dollars |
| W6 | Hand-written contrast calculators | `math.Pow` cells in every round: 2-8 per round, 217k-1.27M input tokens each round; the gate math is deterministic | 2-9% of tokens |
| W7 | Interpreter-error turns | b@c 36 error cells (5%) at 3.74M input; d@e 53 at 4.08M. Top causes: `reflect.Value.Slice: string slice index` (26 total), `import "x" refused` / `expected statement, found 'import'` (10), undefined `fmt.Println` as method (cell syntax). Each costs a full turn at 100k+ context | 5% of tokens |
| W8 | Same file read many times | theme.ts read 3-8 times per round, check-packages.mjs 2-6, App.svelte 2-4 (35 KB single read in 5342f74b). 5-22 repeat reads per round | within W1 |
| W9 | Whole outcomes done twice | section 1.4: FRED x5, textstats x3, prompt bar x2, menu-default x3; 30% of d@e engineering tokens | 30% of d@e |
| W10 | No budget on a failing assignment | section 1.5: 116M tokens | outlier |
| W11 | Platform paths in frozen diff cause rejection and a whole new assignment | mailnote 96b5 prompt: "refused because its frozen diff also included scripts/choir-shot.mjs"; FRED 70dc: remove `__pycache__` from the patch | one retry each |
| W12 | 7.5k-token system overlay plus 13k first call resent every iteration | cached on DeepSeek ($0.0004/call), but on crouton (cached=0) 13k x 100 = 1.3M tokens per round of identical bytes | 10% of tokens on uncached providers |

Four of 8 b@c trajectories today never settled (live after install). That is correctness, not tokens, but it means "tokens per accepted change" is worse than per round: 3 settled rounds plus the failed OLED = $3.80 for 3 settled, **$1.27 per settled change** [M].

## 3. Ranked levers

Numbers: S = share of the slice's DeepSeek cost per round (mean $0.94), P = share of the paper's total tokens (paper is about 50M input/h, self-dev is 5-25M/h only while rounds run, so against the paper's total the slice is under 10-30% on average; I give P only as a rough range).

1. **Delegate-first, literally: shrink the engineer to brief, wait, verify once, freeze.** Delete W1 and most of W2. The engineer reads the Cast, writes a short brief with the owner's words and the checks, parks one wait, reads the result once, runs one combined gate job (the same `go test` plus `check-packages` plus choir-shot full set, in one `choir.Start` script, one Poll-free wake), freezes, completes. That is about 12-16 iterations with context 13k to about 40k. Estimate: 0.35M tokens and **$0.04-0.06 per round [I]**, 16-23x cheaper, **S about 90-95%**; P 5-20% of the paper while rounds run, 0 otherwise. Quality risk: low to medium. The delegate is Sonnet at medium effort and finished OLED in 400 s and Aero in 67 s; the risk is the delegate misreading the Cast without the engineer's pre-reading, which the engineer can offset by pasting the Cast verbatim (already required by "owner_words"). Size: prompt edit in `rlm_engineering_runtime.yaml` (a 30 KB overlay that should also shrink to the delegate-first path: about 20 KB gone) plus one helper script for the combined gate; days, not weeks. Proof: rerun 3 rounds, compare gates and Freeze acceptance.
2. **Fix the repo exploration at the source: ship a theme-pack recipe and a `choir.CheckPack` verb.** Learnings #2: "copy DEFAULT_THEME, change values only" is the entire task. A one-page recipe in the prompt (manifest template, theme.json keys, the gate commands) removes W1 for the data-only class, and a deterministic `choir.CheckPack(path)` (key sets, contrast pairs, check-packages, the apppackage tests; no model) removes W6, W7 for contrast code, and most of the post-delegate phase. S about 25-40% on top of lever 1 if lever 1 is not taken; with lever 1, S for the remaining 5%. Risk: very low (deterministic checks the platform already owns). Size: small.
3. **Per-assignment token or iteration budget.** W10 shows 734 calls and 116M tokens for two refused attempts. A fail-closed budget (for example 25M input or 120 iterations, then Complete(failed) with the last error) bounds the tail. S 0% on a healthy round, but removes 10x outliers; today's b@c outlier alone is 116M of 237M (49% of that computer's tokens). Risk: low (a failure that would have burned tokens is reported sooner). Size: small. Evidence: section 1.5, d@e dbe4 (173 calls, 24.7M) vs the same task done in 49 calls on crouton.
4. **Kill the cold misses (W3).** 28% of DeepSeek dollars. [H] Synthetic requests carry no session or cache key (SyntheticProvider.call sets only Authorization, Content-Type, User-Agent, Accept; no `prompt_cache_key`, `session_id` or equivalent), while the ChatGPT provider documents that without a session header "later calls cache only the static prefix, as if each landed on another cache machine" (provider.go ~line 1211). The miss pattern here (isolated 0.0 followed by 0.98) matches routing to a machine without the prefix, not a TTL expiry (gaps 5-70 s). Fix candidates, cheapest first: send a stable per-run cache key if Synthetic honours one (probe needed, see section 5); keep a run's calls serialised so no two calls of a run are in flight (I did not check concurrency); if still lossy, route a miss to a retry only when input exceeds 20k and cached is 0 (rare, and the price of a retry is the same as the miss). S about 28% of dollars, P about 0 (River/Synthetic only). Risk: none to quality. Size: small to medium, mostly the probe.
5. **Stop the Poll loop (W4).** The runtime already wakes the engineer when a job ends; the transcripts show it polling anyway. Options: make `choir.Poll` on a running job return an instruction instead of state ("end your turn; you are woken when the job ends") so the model does not get a status to chew on, or let `choir.Wait(id)` block the cell up to the 5-minute cell cap and return the tail (one turn instead of 3-8). Estimated 8-11% of round tokens. Risk: low. Size: small.
6. **Reasoning off or minimal for engineering.** 85% of output tokens are hidden reasoning at stated effort `low`; the default policy gives engineering `model = "pool:default"` with no reasoning line (model_policy.go `defaultPolicyText`, `[roles.engineering]`), and fallbackPolicy's performer is `pool("")`. Roughly 14% of dollars per round. Risk: medium (engineering plans through reasoning; test on three rounds). Size: one line in policy; needs a trace comparison, not a guess.
7. **Trim cell stdout and compaction.** Tool output is about 65% of the final transcript, and exploration outputs average 3.5k chars with a tail of 15-36k. Cap `choir.ReadFile`/Exec stdout echo at 4k chars with a "more" cursor, and compact at 60-80k instead of letting it reach 150k+. Saves roughly a third of cache-read dollars (about 12% of round dollars) and more on crouton (cached=0, where every token of history is billed each call). Risk: medium (the model may need to re-read what was trimmed). Size: small.
8. **Retry hygiene: Freeze strips known junk, rejections name a class.** W11: auto-exclude `__pycache__`, `*.pyc` and `scripts/choir-shot.mjs` from the frozen diff (the platform owns that file); if a rejection still occurs, reopen the same capsule instead of a new assignment (a new assignment pays 1M+ tokens of re-exploration; handoff paragraphs exist but the run still starts cold). Removes a slice of the 30% retry tokens on d@e. Risk: low. Size: small to medium.
9. **Inline the contrast math (W6) and fix the yaegi pitfalls (W7).** Contrast: part of lever 2. Pitfalls: the 26 `reflect.Value.Slice` errors suggest one common idiom (`s[a:b]` on a string that is shorter) failing; a one-line overlay example or an interpreter fix. 2-5% of tokens.
10. **Fewer assignments per outcome (W9).** FRED x5, textstats x3: the management Cast decides to re-run instead of resuming. That is the management desk's slice; the self-dev side only sees 30% of its engineering tokens in retry assignments.

What a 10x and a 50x version look like:

- **10x (lever 1 + 3 + 4):** delegate-first engineer, hard budget, cache fix. Round about $0.05-0.06 list. A 12-16 call DeepSeek loop plus a $0.10-0.28 Sonnet run on the owner's Claude plan. River's $1 a day buys 15-20 rounds.
- **50x:** the data-only theme pack is a typed object, not an engineering project. One DeepSeek call (13k prompt: recipe plus the 37-key schema, about 2.5k output) returns the token JSON; the platform runs the deterministic gates, shots and Freeze with no model turn; a repair call happens only if the gate names a failing pair. Expected 2-3 calls, about 40k tokens, **about $0.01-0.02 per theme [I]**, and the owner-facing judgement stays management's shots. The same shape applies to any "add a pure-data package" change; code changes keep the delegate path.

## 4. Jev and classifier opportunities in this slice

Honest answer: small. Jev is a text classifier ($0.042/M input, output free) and the expensive things here are not label decisions, they are a transcript resent 100 times.

- Existing Jev traffic from self-dev: 89 calls in 21 h on d@e (52 unattributed, 24 management, 13 conductor), 20 on b@c; 0.2M tokens, $0.01 total [M]. Nothing to cut.
- Candidate Jev calls (all tiny, run in parallel as one batch per event):
  - Route label at assignment start: `data_only_pack` / `in_place_package` / `platform_patch` / `new_package` / `research_report`, so the engineer picks the lean recipe (lever 2) and the right capsule mode. About 1 call per assignment, 18 a day on d@e; one batch; replaces the engineer's first 30-60 exploration iterations that discover the task class.
  - Rejection-cause label (patch hygiene, defect, contract miss, timeout) to decide between reopening the capsule and a fresh assignment (lever 8). One call per rejection.
  - "Did this cell error out for a known idiom" label on the 26+ yaegi `reflect.Value.Slice` failures: better done by a regex in the cell wrapper than by Jev.
- Things that look like model work but are deterministic and need neither Jev nor an LLM: "is the job done" (Poll), contrast pass/fail (WCAG arithmetic), "does theme.json have the 37 keys" (a set diff), "does the diff include a platform path" (a path check), "do the screenshots exist and are not blank" (Freeze already reports `blank`).
- Not Jev: shot judgement needs vision (verifier_multimodal / management); Jev is text only.

## 5. Open questions and the probe for each

1. **Why do isolated calls miss the Synthetic cache?** Probe (free, no new spend beyond one existing round): take the gateway journal lines for `provider: synthetic` in one round window and test whether misses coincide with another in-flight Synthetic call (concurrency gate `SYNTHETIC_MAX_CONCURRENT`, default 8) from the paper VM; then check whether Synthetic's `/openai/v1` accepts and honours a `prompt_cache_key` or session header (its docs, not a call). If the miss correlates with concurrency, the fix is queueing; if with absence of a key, add one.
2. **Is the hidden reasoning really 85%, and does it re-enter the transcript?** Probe: the provider response `usage.completion_tokens_details.reasoning_tokens` for one Synthetic call (a single logged gateway line, not new spend), and a diff of `input_tokens` growth against visible content. I inferred it from chars/token (3.5) and never saw the reasoning field.
3. **Does the engineer re-run the delegate's checks only because the prompt says so?** Probe: read the Freeze/verifier acceptance contract that requires the engineer's own receipt (the overlay says `Freeze cites` own receipts); if the host can cite the delegate's receipts instead (they are brokered inside the same capsule), lever 1 drops to 5-8 iterations.
4. **What did the delegate cost in tokens per round across the day, and how much is cache-creation?** `daily_tokens_used` 429k on b@c does not match the relay sums (cache-read 3.0M, output 57k over the day), so it counts something narrower (perhaps uncached input plus output [H]). Probe: sum `modelUsage` from each delegate `wait` output in the dumped events.
5. **Why did the 03:17 OLED attempt run 734 calls?** I know the cause class (learnings #1) but did not read its events; a read of `run:assignment-*` for trajectory 9a29063b would show whether the model repeated the same refused build.
6. **Who started 4 live-never-settled trajectories?** Not a token question, but it inflates tokens per accepted change; the scratch learnings say a fix is in review. Verify on the next round.
7. **Crouton after its free window.** From about 2026-11-10 the first selfdev pool member may stop being free or disappear; all the self-dev traffic then lands on Synthetic and River, and cached=0 on crouton makes the current numbers a floor for what the work costs without caching. I did not find a live statement of the date; read pricing.go and the OpenRouter listing before relying on it.

## 6. Commands used

```
$SCR/de.sh '/api/costs?detail=1&limit=10000'   ->  eff/de_costs.json
$SCR/bc.sh '/api/costs?detail=1&limit=10000'   ->  eff/bc_costs.json
eff/fetchall.sh <de.sh|bc.sh> run:assignment-<id> <out.json>    # paginated /api/runs/<id>/events
$SCR/bc.sh /api/trajectories/<id>               # status, objective
$SCR/{de,bc}.sh /api/runtime/observability      # claude_delegate daily tokens
ssh node-b "sudo journalctl -u go-choir-gateway --since '-14h' --no-pager"   ->  eff/sd/gw14.log
grep 'anthropic relay computer-' eff/sd/gw14.log   # delegate relay tokens per hour per computer
git show origin/main:internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml   # engineer prompt, delegate-first and verify-never-trust paragraphs
git show origin/main:internal/modelcatalog/catalog.go   # SelfDevPool
git show origin/main:internal/gateway/paid_budget.go    # River $1/day
```

Nothing was posted to any computer; no model was called.
