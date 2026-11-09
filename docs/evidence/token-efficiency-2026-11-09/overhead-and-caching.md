# Slice: overhead-and-caching (token efficiency audit, 2026-11-09)

Read-only. Code read from `origin/main` (a5103d5a0). Paper API read through `paper.sh`; gateway journal `ssh node-b "sudo journalctl -u go-choir-gateway --since '-3h'"` saved as `eff/gw-oc.log` (16:17 to about 19:30 UTC). Event samples: `eff/research-events.json` (14 runs), `eff/texture-events.json` (30 runs), `eff/management-events.json` (24 intake-task runs), from `GET /api/runs?limit=500` then `GET /api/runs/{id}/events?limit=500`. No model call was made. Every number is tagged (M) measured, (I) inferred, (H) hypothesis.

## 0. Corrections to the token-budget doc (important)

The doc `paper-token-budget-2026-11-09.md` section 1.4 maps gateway `system_chars` to roles wrongly. Evidence that the right map is:

| system_chars (gateway request line) | desk | how I know |
|---|---|---|
| 11986 | management_task (intake) | doc, and my build of the prompt: 11,864 chars (M) |
| 17637 | texture | my build: 15,579 chars; reasoning=high on those lines (I) |
| 37943 | research | my build: 37,821 chars; research run events say `llm_reasoning_effort: low`, first call input_tokens 9186, `system_sha256 4fbead15...` on all 253 calls (M) |
| 49908 | management (conductor) | my build: 42,677 chars with intake labels off; the intake/event-wake overlays add the rest (I) |

How the prompts were built (M): extracted origin/main to a scratch dir (not the repo), added a throwaway test calling `rt.systemPromptForRun` per profile, and wrote the text to `eff/prompt-<role>.txt`. Command: `go test ./internal/agentcore -run TestZZSizes -v` in the scratch copy. So the doc's "texture is 78% fixed system prompt of 38k to 50k chars" is wrong: texture's prompt is about 17.6k chars (about 4.1k tokens). Research carries the 38k-char (about 8.5k-token) prompt, and the conductor carries the 50k-char one. Call counts support the new map: over 3 h the gateway saw 2213 requests at 37943 (research ~1336 per 2 h in the doc), 1425 at 17637 (texture 908 per 2 h), 1245 at 11986 (task 793 per 2 h).

## 1. What this slice spends

### 1.1 Tool schemas: one tool per desk, not a lever

Every inference request on the paper has `tools=1` (M: all request lines in `gw-oc.log`; the one tool is `desk_go_eval`, `capsule_go_eval` for capsules, per `internal/agentcore/tools_desk.go:591` and `tool_profiles.go` "desk_go_eval is the sole tool on every desk registry"). The tool definition is two literal strings plus a small schema: about 1.3k chars, about 300 tokens (I, count of the literals). `BuildSystemPrompt` also appends a one-line "Available tools:" catalog (about 150 chars). So "per-desk tool subsets" has nothing left to remove at the schema layer. The equivalent cost sits in the system prompt: the generated "REPL interaction surface" listing every `choir.*` verb with its documentation. That is where the bytes are.

### 1.2 System prompt per role (M for chars, I for tokens at about 4.3 chars/token, calibrated so that research call 1 = system + tool + one message = 9.2k tokens)

| desk | prompt chars (prod, from request lines) | tokens (I) | of which verb surface (my build) | verbs listed | share of an average call |
|---|---|---|---|---|---|
| management_task | 11,986 | 2.8k | 6.1k chars | 13 | 36% (call 1 = 3.0k, M) |
| texture | 17,637 | 4.1k | 7.6k chars | 17 | 31% of 14.4k avg |
| research | 37,943 | 8.5k | 18.7k chars | 36 | 40% of 22.3k avg |
| management (conductor) | 49,908 | 11.6k | 11.5k chars (intake overlay adds more) | 31+ | 35% of 33k avg |

The average input per call per desk is from `/api/costs?detail=1&limit=10000` (M, 17:47 to 19:18 UTC, crouton only): management 685 calls avg 33.3k, research 869 calls avg 22.3k, texture 602 calls avg 14.4k.

Fixed-prompt volume per hour on the paper (M for request counts over about 3 h, I for tokens; request counts include some failed/retried requests, so upper bound by 10 to 20%): research about 710 requests/h x 8.5k = 6.0M; texture 460 x 4.1k = 1.9M; task 400 x 2.8k = 1.1M; conductor 240 x 11.6k = 2.8M. Total about 11.8M tokens/h of about 41M/h the paper's crouton path used in this log (123.4M input over about 3 h, M, `inference succeeded` lines). So the fixed prompt is about 29% of input on this window; on a non-wedged paper (about 24M/h) it would be about 40%.

### 1.3 Skills and manifests

No skills are injected on these desks (`skillContextForProfile` returned nothing in my build of research, texture, management, task; I did not find skill text in the prompt files). The "manifests" are the verb surface above plus the per-desk overlays (`rlm_research_runtime.yaml` 13.1k chars, `rlm_management_runtime.yaml` 27.6k, `rlm_management_task_runtime.yaml` 5.2k, texture.yaml 8.8k). The unused part of the research surface: in 14 sampled research runs (240 cells, M), 16 distinct `choir.*` verbs were called (web_capture 79 cells, Searches 52, ReadCapture 47, Read 45, SearchStart 40, SearchWait 36, StoryAnalysis 34, ReportPacket 31, Captures 19, Records 15, SearchWireCorpus 15, CellOut 14, Relate 13, Reads 6, Context 5, ListEvidence 3). About 10.6k chars (57%) of the research surface documents verbs that were never called in the sample (Ask, Cast, Escalate, Pack, Precommit, RelateStories, Research, SourceAPI, System, SearchTexture, PollSource, and 10 smaller ones). Sample is small, 14 runs; some of these matter in rarer runs.

### 1.4 Per-call decomposition of a run (M sums from event samples; fixed prefix at the I token counts above)

| desk | calls/run | fixed prefix (system+tool) | first-turn context beyond prefix | carried tool results | cell source and other growth |
|---|---|---|---|---|---|
| research (14 runs, 253 calls, 5.58M) | 18.1 | 40% | 2% of run input is call 1 in total | 54% | about 6% |
| management_task (19 runs, 186 calls, 1.40M) | 9.8 | 36% | small | 55% | about 5% |
| texture (22 runs, 81 calls, 1.46M) | 3.7 | 25% | about 50%: call 1 median 15.4k with median 20 messages already in the thread | 24% | |

How the carried-results share was computed (M): for each run, result k (from `tool.result` `output` length / 4) is resent in every later call; carried = sum over results of tokens x later calls. Research result-carry is 3.02M of 5.58M.

Texture is different in kind: it starts each wake with a replay of its previous conversation (`runMemoryManager.carryTexturePriorConversation`, `actor_rewarm_carry_conversation`, median 20 messages at call 1, up to 36) plus the document frame. Call 1 is 21% of a run's input, and the 2.3 wakes per document are spaced tens of minutes apart (doc, section 1.4), so a provider cache with a short TTL will not hold the replayed thread between wakes (H).

### 1.5 Tool loop history and compaction policy (code read)

- History is strictly append-only. There is no observation masking, no result trimming beyond the per-cell output cut at about 8.6k chars (`KeepCellOutput`, the `choir.CellOut` handle), no truncation of assistant cell source (avg 1.25k chars in research, M).
- Compaction (`run_memory.go` `compactIfNeeded`): fires only when estimated prompt (messages + system estimate + 12k reserve) exceeds 70% of the model's context window; `modelcatalog.DefaultContextWindowTokens` is 200,000, so threshold 140k (`provideriface/config.go` ratio 0.70, keep-recent 20k). Research runs peak near 60k and intake near 15k (M), so compaction never runs for them; it only ran for the wedged conductor (doc 1.3). The compaction is a full-context call by the same model ("ask the thread"), so each compaction costs one call of about 115k to 118k input plus about 2k output, and the next calls restart cold.
- Marginal cost is linear in carried context: a 60k-token call costs 6x a 10k call. A threshold derived from the window (140k) rather than from cost means long runs pay 14x per call before anything is shed.
- Reasoning content: `reasoning_content` is stored and resent (`toolloop.go:743,1429`), but growth per research iteration (1.4k) is fully explained by result (about 1.15k tokens) plus cell source (about 0.3k), so reasoning is not a measurable carry cost today (I).
- Result envelope boilerplate: each `desk_go_eval` result carries about 206 chars of JSON fields and, on first results, a 200-char session sentence, plus a 330-char untrusted-data wrapper on data results (M, 240 results). About 130 tokens per result, about 0.6% of research input. Not a lever on its own.

## 2. Prompt caching end to end

### 2.1 Is the prefix byte-stable? Yes, by design, tested

- The system prompt is built once per desk and is free of volatile fields: time, ids, requester, assignment, seed prompt, capture count are all moved to the run-context user turn (`internal/agentcore/run_context.go`, header comment; test `prompt_cache_prefix_test.go`, cases texture, management, research, engineering). Measured: one `system_sha256` across all 253 research calls and all 81 texture calls in the samples (M). Tool list is sorted (`ToolRegistry.order` sorted) and schemas are marshalled from maps (sorted keys), so tools are byte-stable.
- The cache key (`toolregistry.PromptCacheKey`) is `choir-<desk>-<scope hash>-<hash(system+tools)>`, derived from what is sent, so a new prompt version gets a new key. Per-model failover tags the key (`failoverCacheKey`).
- Remaining cache breakers: (1) the first user message of every run carries the time to the second and run facts, so cross-run sharing stops after system+tools (for research that is the first 8.5k of a 9.2k-token first call, so little is lost); (2) compaction rewrites history (cold again); (3) model failover to another provider re-sends everything uncached (0 switches in my 3 samples, so rare); (4) texture's carried conversation is only warm if the next wake falls inside the provider's cache TTL (H); (5) the run-context turn is appended at resume and persisted, so replay is byte-identical (tested).
- Ceiling on in-run cache hit rate if the provider caches perfectly (M, computed from per-call input in event samples: cacheable = previous call's whole prompt): research 92% (95% with a cross-run system prefix), management_task 86%, texture 73% (many 2 to 4 call runs with a large fresh frame).

### 2.2 What the provider layer sends

| provider | cache hint in request | evidence | observed hit |
|---|---|---|---|
| chatgpt (luna) | `prompt_cache_key` body + `session_id`, `conversation_id` headers (`provider.go:1211-1244`) | code | doc 2026-10-14: 4 to 10% |
| opencode / opencode-go | `x-opencode-session` header (`provider.go:1443`) | code | 85% (doc) |
| synthetic | none; automatic prefix cache | no key set in `SyntheticProvider` | 79% of 6.1M input in my 3 h log (M: 77 calls) |
| openrouter | body `session_id` = cache key, capped 256 (`openrouter.go:193-205`, added 2026-11-04 in 44a8bae07, body-only after c2c0590c5) | code | crouton-alpha 0% (M) |
| anthropic/bedrock | `cache_control` ephemeral on system block and a breakpoint on the last history block (`markHistoryCacheBreakpoint`) | code | not used on the paper |

### 2.3 Why crouton-alpha on OpenRouter reports cached_in=0 (M for the metadata, H for the conclusion)

1. Across 5334 successful crouton requests in the 3 h log, `cached_in=0` and `cache_reported=true` on all of them, 134.2M input tokens (M: `grep 'chat completion' gw-oc.log`). `cache_reported=true` means the upstream usage block did include `prompt_tokens_details.cached_tokens`, with value 0. It is a real "no hit", not a missing field.
2. OpenRouter's public endpoint metadata for the model (M, `curl -s https://openrouter.ai/api/v1/models/stealth/crouton-alpha/endpoints`, no key needed): a single endpoint "Stealth", `pricing.prompt` 0, `pricing.completion` 0, no `input_cache_read` price, and `"supports_implicit_caching": false`. The same call for `deepseek/deepseek-v4.1-flash-1017` lists five endpoints, each with `input_cache_read` pricing and `supports_implicit_caching: true` (prompt $0.006 to $0.038 per M, cache read $0.0015 to $0.01 per M); `recursive/flint-v1.0:free` and `nvidia/nemotron-3.5-super-vl:free` are `false` too.
3. OpenRouter's prompt-caching guide (fetched): implicit caching is automatic only for the listed upstream families (OpenAI, Grok, Moonshot, Groq Kimi, DeepSeek, Z.AI, Gemini 2.5+); `session_id` only makes follow-ups sticky to one endpoint "when the provider's cached reads are cheaper than regular prompt pricing". A free stealth endpoint with no cache pricing gets no stickiness benefit, and there is no cache to be sticky to.
4. Conclusion: Choir is doing everything a client can do (stable prefix, session id, key hygiene). The `session_id` change of 2026-11-04 cannot help; the deployed gateway already sends it (the cache lines in the log use it). The lever is the endpoint, not the request. Caveat (H): the metadata says "no implicit caching"; it does not prove the stealth upstream does no hidden prefix reuse that it fails to report. One free same-prompt-twice probe would settle that (section 5).
5. Side facts: Synthetic's 79% is the real DeepSeek v4.1 flash prefix cache (the 2026-10-14 problem doc measured 87 to 92% in-run). The paper's cached share fell from 50 to 80% to near 0 when routing moved to crouton at about 13:00 UTC (doc 1.1). Fresh input tokens per hour are therefore the metric that went up.

## 3. Waste found

W1. The prefix is cacheable and 29% to 40% of all input is fixed prompt, and on crouton every byte of it is re-processed on every call (M/I above). With a caching endpoint this would cost about a tenth; without, it is about 12M tokens/h.

W2. Carried tool results are 54% (research), 55% (task) and 24% (texture) of input (M, event samples) and the biggest single items are results at the cut cap. 36% of research results, 37% of texture results and 10% of task results are over 8,000 chars (87/240, 29/79, 19/186; M). Results above 8k chars carry 65% (research), 91% (texture) and 54% (task) of the result carry.

W3. Texture page-reads long captures through the output cut: run `57942a...` (events in `texture-events.json`) reads capture `r2.c1..c5` in one cell, then spends nine more cells paging the same cut output with `choir.CellOut(...)[a:b]` slices of 8k to 11k chars (each result 8.5k to 8.9k chars, about 2.2k tokens), re-reads captures it already read (`ReadCapture("r2.c4",0,0)` twice, the second time grepping), and only in the last two cells greps in-cell (`strings.Index`). Input per call went 7.7k to 30k over 12 calls (241k total). Output per paging call is 15 to 40 tokens; the model is spending whole round trips moving text it then discards. If the first cell had searched in-cell and printed only matches, the run would plausibly have cost about 50k (H, derived by dropping the eight paging cells; the same shape appears in research run results with `CellOut`: 14 cells in the 14-run sample, and in 4 of 14 research, 3 of 30 texture and 4 of 19 task runs there are two or more CellOut cells).

W4. Verb surface is the documentation for everything the desk could do, resent each call: research 18.7k chars (about 4.3k tokens), of which about 57% documented verbs were not called in the 14-run sample (section 1.3). `choir.Help(name)` already exists and returns signature, fields and example, and `cellErrorHint` already names near-miss verbs, so the model already has an on-demand path.

W5. Long-prompt narrative: research role text plus overlay is about 19k chars before the surface (the same 4.4k tokens on every one of about 710 calls an hour). I did not find literal duplication (dup-sentence check: 51 chars in research, 293 in management); the cost is length, not repetition.

W6. Compaction as designed is only a safety valve: threshold 140k (window-based, not cost-based) means a 140k-token context is resent per call for a long loop. In normal runs it is moot (research tops out near 60k); in the wedge it cost about 2.4M tokens per cycle (doc 1.3). Not a normal-path waste, but a bound that should depend on spend.

W7. Texture re-sends a replayed conversation (median 20 messages, about 10k tokens) at the start of each wake, uncacheable if wakes are minutes apart (H), and 2.3 wakes per document (doc).

## 4. Ranked levers (savings as a fraction of the slice, and of the paper's total; quality risk; size)

Baselines: "paper total" = the doc's 49.5M input tokens/h on crouton including the wedge; "non-wedge" = research + texture + task = about 24.5M/h. I give both because the wedge fix will change the denominator.

L1. Serve the paper from an endpoint that caches (DeepSeek v4.1 flash through OpenRouter, Synthetic, or opencode-go), keep the append-only prefix, and report fresh tokens. Raw tokens unchanged; fresh (uncached) tokens fall to about 8% (research), 14% (task), 27% (texture) in the ideal case (M ceilings in 2.1); measured Synthetic reality is 79% (M). Savings: 70% to 85% of fresh input across the non-wedge slice; 60%+ of paper total. Quality risk: the model change, not the caching (a quality slice question; the paper ran on deepseek-v4.1-flash before 13:00). Size: routing policy only. Dollar view (I): 41M/h x $0.028/M = $1.15/h uncached on the OpenRouter DeepSeek endpoints, about $0.30/h at 90% cached ($0.005/M read), so it is cheap either way; the binding constraints on the free paths are 429 concurrency and quota (1233 failures in 2 h, doc 1.1), and whether a provider counts cached tokens against its quota is unknown (open question).

L2. Result carry control for non-caching endpoints: mask old tool results (keep the last K=2 results, replace older ones with a 40-token stub plus the CellOut handle, which already exists) and lower the cell-output cut from about 8.6k to about 3k chars with an in-cell extract rule. Counterfactual on my samples (M, same method as the doc's 47%): K=1: research 47%, task 43%, texture 18%; K=2: 43/34/14; K=3: 38/26/10 (percent of that desk's input). Weighted by non-wedge shares (research 61%, texture 26%, task 13%): K=3 about 29% of non-wedge, about 14% of paper total; K=1 about 36% / 18%. A 3k-char cap alone (upper bound, ignores the extra paging calls it will cause): research 29%, texture 15%, task 28%. Quality risk: medium; a masked result the model needs again costs a CellOut call. Interaction with L1 (important): on a caching endpoint, masking breaks the cached prefix at the first masked result, so the K most recent results and everything after are billed fresh on each call. With cached reads at about 0.18x to 0.1x price this loses for contexts under about 60k tokens (my arithmetic on a 30k context: append-only 1.5k fresh + 3k cached-equivalent = 4.5k; K=2 masking 4.5k fresh + 1.5k = 6k). So choose by endpoint: masking only on non-caching endpoints, or mask in blocks (every 8 to 10 calls) on caching ones. Size: toolloop change in `internal/toolregistry/toolloop.go` plus a result-stub contract; about 150 lines.

L3. Lazy verb surface: list each verb as `Name(args) result` (one line, about 100 chars) and rely on `choir.Help(name)`. Research surface 18.7k chars to about 4.5k (36 verbs x 120 chars), task 6.1k to 2.5k, texture 7.6k to 3.5k (keep Write's full doc), conductor 11.5k to 5k. Saves about 3.3k tokens per research call, 0.85k per task call, 1.0k per texture call: about 710 x 3.3k + 400 x 0.85k + 460 x 1.0k = 3.1M tokens/h (I), 6% of paper total, 13% of non-wedge. Quality risk: low to medium (the model must call Help for rare verbs; cost is one cheap call when it does, and the near-miss hint exists). On a caching endpoint the dollar value is about 10x less, the latency gain stays. Size: the generator `DeskSurface` and `renderSurface` in `internal/yaegikernel/surface.go`; about 100 lines plus a prompt-length gate.

L4. Cut the narrative prompts: research role + overlay about 19k chars to about 10k (rule of thumb: each rule that never changed behavior in a trace is deleted), conductor overlay 31k chars to about 15k. Saves about 2.1k tokens x 710 = 1.5M/h for research and about 3.7k x 240 = 0.9M/h for the conductor: about 5% of paper total. Quality risk: medium; it needs per-rule trace evidence. Size: edits to yaml.

L5. Page-reading rule for cells: give the desk a verb that returns a keyword-focused extract of a capture (`ReadCapture(id, query)` returning matching paragraphs plus context) and a prompt line "grep in the cell, print less than 2k chars". Targets W3: results over 8k chars carry 65% (research) and 91% (texture) of result carry. Saving overlaps L2: 15% to 30% of those desks' input at upper bound; I would book 10% of research and 15% of texture as the independent part (H). Quality risk: low if the extract is a superset window. Size: one host verb, one prompt line.

L6. Cost-aware compaction: compact when carried tokens x expected remaining calls exceeds one summary call. Break-even is about 2 further calls at 140k and about 2.4 at 60k (my arithmetic: compaction call = C, saves C-35k per later call). On a non-caching endpoint a 60k threshold would cut the long tail of the 38-call research runs (the 12 to 15 runs above 30 calls). Impact is small on average (runs above 40k at call 20 are a minority), so book 3% to 5% of research (H). On caching endpoints compaction should stay rare. Size: threshold function, 20 lines.

L7. Texture wakes: store the carried conversation only when it is likely to still be warm, otherwise replace it with the document frame plus a short digest (call 1 median 15.4k, 21% of run input). Saving 10% to 15% of texture input (H), about 1% to 2% of paper total. Quality risk: medium (the design "texture holds the trajectory" says carry is the point). Size: medium.

Structural redesign, "10x to 50x leaner" shape for this slice (design hypothesis, not tested):
- Layer the context in three cacheable tiers: desk-static prefix (about 1.5k tokens of rules + one-line verb index, identical across every run and computer), thread prefix (digest of the story or document, rewritten only on a boundary), and a short tail. The goal state is a first call of about 3k tokens for research (today 9.2k) and a result channel that returns about 1.5k tokens instead of 2.2k on the average large result.
- Move reading and filtering into one host cell: "search, read the top N, extract passages that answer question Q, return a digest of at most 2k chars with capture ids". Research goes from about 18 calls (410k tokens per run) to about 4 to 5 calls (about 6k each) = 30k per run, which is 14x. This is the single biggest structural change available in this slice and it is also where the quality risk is concentrated (selection of passages). Jev-class scoring can do the filtering inside the cell (section 4 of the brief; see below).
- Choose per endpoint: on a caching endpoint keep append-only (L1); on a non-caching endpoint use masking and a 3k result cap (L2). One flag-free code path can derive this from the endpoint's reported `cache_reported`/hit rate over the last N calls (a measured fact, not a switch).

Combined for this slice on the non-wedge baseline (they overlap; my estimate): L1 alone about 70% to 80% fresh; L2+L3+L4+L5 on a non-caching endpoint about 45% to 55% of raw tokens; both together a 10x reduction of fresh tokens is within reach with no change to what the desks do. A 50x reduction needs the host-side digest cell above.

## 5. Jev and classifier opportunities in this slice

The slice's own LLM calls that are really decisions:
1. Compaction summary ("ask the thread" for a checkpoint): only runs in long loops; today a full-model, full-context call. A deterministic summary exists (`deterministicEmergencyRunMemoryCompaction`). Not Jev-shaped; rare. Skip.
2. Tool-result relevance ("is this page chunk worth keeping in context?"): a per-chunk label. 36% of research results are over 8k chars (about 87 per 14 runs, about 6 per run, about 4,000 per day at the paper's rate, H). A Jev batch of one question per 2.2k-token chunk is about 2.3k tokens each, so even 100 chunks per research run is 230k Jev tokens at $0.042/M = $0.01 and is parallel (one batch). Parallel shape: after a `Read`/`ReadCapture`, send all paragraphs of the capture as one judgments batch with the story question; keep the top K paragraphs in the cell result. This is the Jev version of L5 and the real candidate for the 14x research redesign. Quality risk medium: the question wording decides recall, so shadow-run it first against the model's own later citations (the later `ReportPacket` captures).
3. Search-result triage: a `Searches` call returns up to 40 results; the model reads 8+ of them in the next cells (doc, research overlay). One Jev batch of 40 snippets ("is this a primary source; does it add a new outlet?") picks the read list in one call instead of the model choosing across cells. Parallel shape: one batch of up to 40 per search. Hypothesis; needs a replay of past searches.
4. "Stop researching" check (two distinct outlets found): this is a count over the cited captures, not a model judgment; compute from `Captures()` and host-side outlet attribution. Not Jev; deterministic.

Already Jev: system1/support_judgement/cast/inert judgments, about 250 calls per 73 min at 1.4k to 2.3k tokens (doc 1.2). In my research sample there were 28 `system1.judgment` events for 14 runs (2 per run, 11 questions each).

## 6. Open questions and the probes that settle them

1. Does crouton-alpha cache but fail to report? Probe: send the same 4k-token request twice through the gateway (or directly to OpenRouter with the existing key, by the owner) and compare `provider_call_ms` and `cached_tokens`; free model, tiny. Also check `usage.cost` and the OpenRouter activity log's `cache_discount` field.
2. Do cached tokens count against the free-model and Synthetic quotas and the 429 concurrency limit? Probe: compare quota burn per token on Synthetic (cache 79%) against the quota header (`x-synthetic-quotas`, `syntheticQuotaSummary`) over one hour.
3. Texture carry cache TTL: hit rate of a texture wake's first call on Synthetic as a function of the gap since the document's last wake (gateway `cached=` for the 17637-char requests joined to wake timestamps). Predict: near 0 when the gap exceeds a few minutes.
4. System-chars map (section 0): confirm with one run's `loop.progress.system_sha256` versus the gateway request line for that call. I rely on call counts, reasoning effort and my own prompt builds; one joined line would close it. (The conductor's 49,908 vs my 42,677 gap, I attribute to the intake labels and event-wake overlays; not verified.)
5. Verb usage beyond a 14-run research sample: count `choir.X` per verb across the 79 research runs the list returns (read only) before cutting any verb from the surface in L3.
6. Marginal value of reasoning effort: research is `low` and task is `high` per the gateway line; I did not measure hidden reasoning tokens in `tokens_out` (output is 1.0% of input), so it is outside this slice.

## Commands

    # gateway, per provider / model cache share
    ssh node-b "sudo journalctl -u go-choir-gateway --since '-3h' --no-pager" > eff/gw-oc.log
    grep 'chat completion' eff/gw-oc.log | sed -E 's/.*provider: ([a-z0-9-]+) chat completion model=([^ ]+).*tokens_in=([0-9]+) tokens_out=([0-9]+) cached_in=([0-9]+) cache_reported=([a-z]+).*/\1 \2 \3 \4 \5 \6/' | awk ...
    # tools / system chars per request
    grep -o 'inference request from autoputer[^)]*' eff/gw-oc.log   # messages=, tools=1, system_chars=
    # OpenRouter endpoint metadata (no key)
    curl -s https://openrouter.ai/api/v1/models/stealth/crouton-alpha/endpoints
    # per-desk averages
    paper.sh '/api/costs?detail=1&limit=10000'
    # run events (role samples)
    paper.sh '/api/runs?limit=500' ; paper.sh '/api/runs/<id>/events?limit=500'
    # system prompt sizes (scratch copy of origin/main, throwaway test, not in the repo)
    go test ./internal/agentcore -run TestZZSizes -v
