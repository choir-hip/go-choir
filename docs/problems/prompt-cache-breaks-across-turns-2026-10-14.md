# Prompt cache breaks across turns, and gpt-6-luna does not cache at all (2026-10-14)

Owner, 2026-10-14: fix Choir's prompt caching across turns to cut
inference cost. Analysis: `docs/archive/reports/prompt-cache-analysis-2026-10-14.md`
(findings B1-B11). Measurement landed in 34a3e524f and is deployed
(staging build b349cbbef, deployed 17:13:44Z). This record is the first
measured read of it. Problem first; the fixes come after this commit and
reference it.

Mutation class of the fixes: **orange** for the prompt-assembly move
(runtime behaviour: where per-run text sits in the model's context) and
**red** for the provider request shapes (`prompt_cache_key`, the
`x-opencode-session` value: gateway/provider calls). No model routing
changes: B5 (pool assignment by trajectory) is left as it is.

## Evidence: the baseline

`GET /api/costs?detail=1&limit=10000&from=2026-10-14T17:13:44Z` on the
Autopaper paper computer (QA credential `autopaper-paper.json`, read inside
python; the key was never printed). Window 17:13:44Z to 17:55:56Z, 42
minutes, 800 calls, 768 of them reported a cache count (all but Jev).

| Desk | Model | Calls | Input tokens | Hit rate |
|---|---|---|---|---|
| research | deepseek-v4.1-flash | 135 | 5,003,097 | **91.6%** |
| management | deepseek-v4.1-flash | 230 | 4,238,685 | **86.9%** |
| texture | deepseek-v4.1-flash | 141 | 2,029,393 | **68.9%** |
| research | gpt-6-luna | 109 | 1,285,689 | **10.0%** |
| texture | gpt-6-luna | 135 | 971,743 | **4.7%** |
| management | gpt-6-luna | 18 | 240,167 | **4.1%** |
| all reported | | 768 | 13,768,774 | **71.5%** (summary `cache_hit_rate` 0.715) |

Split by the call's position in its run (calls sorted by time per RunID):

| Model | Desk | First call of run | Later calls |
|---|---|---|---|
| deepseek | management | 6.1% (20 of 21 zero) | 88.1% |
| deepseek | research | 48.2% | 92.0% |
| deepseek | texture | 0.0% (21 of 21 zero) | 74.2% (12 zero) |
| luna | management | 0.0% | 4.8% (13 of 14 zero) |
| luna | research | 0.0% | 10.6% (83 of 98 zero) |
| luna | texture | 2.5% | 5.4% (87 of 94 zero) |

Typical runs, (input, cached) per call:

- deepseek management: `(2483,0) (2847,2560) (5080,2944) (7911,5248)
  (8566,8192) …` — each call caches the previous call's whole prompt.
- deepseek texture: `(9747,0) (15004,9728) (18048,14976) …` — the same.
- luna: `(7762,0) (8278,0) (8742,0) (10888,0) (12624,0) …` — nothing,
  even when the prompt is the previous prompt plus one turn.

## What the evidence says

1. **OpenCode passes DeepSeek's cache count through, and DeepSeek caches
   the append-only loop almost perfectly.** The analysis's open question 1
   is answered: yes, and the within-activation rate (88-92%) is above the
   70-84% ceiling estimated there.
2. **B1 confirmed on the wire.** Management's first calls cache about 190
   tokens on average (3,968 over 21 calls) and Texture's cache nothing. That
   is the ~275-token preamble before the timestamp, cut to DeepSeek's
   64-token blocks. Texture's 12 zero-hit "later" calls are first calls of
   re-activations of the same run (Texture passivates after each committed
   write and resumes the run): the new timestamp at token ~275 throws away
   the whole replayed history.
3. **New: gpt-6-luna on the chatgpt route does not cache, even within one
   activation.** 220 of 245 luna calls report 0 cached tokens, including
   calls whose prompt is the previous prompt plus one turn, seconds apart.
   The request prefix is byte-stable (the Responses input conversion is
   deterministic, tools are sorted, the loop is append-only), so the cause
   is routing, not our bytes: the request carries no `prompt_cache_key`
   (B7). OpenAI routes cache lookups by a prefix hash plus that key; without
   it a request lands on a machine at random and hits only when it happens
   to land where the prefix was cached (the 5-10% seen). Codex CLI, the
   reference client for this endpoint, sends the conversation id as
   `prompt_cache_key`. **Hypothesis until the after-measurement confirms it.**
4. **Luna is 18% of input tokens and nearly all of it is billed uncached**
   ($0.233 of $0.774 input dollars in the window at list prices).

## Belief state

- B1, B3, B4 (volatile text in the system prompt) are the cross-turn
  breakers on DeepSeek. They cost little today on deepseek (first calls are
  2.3% of deepseek input) but every Texture re-activation loses its whole
  history.
- B6 (session per run) and B7 (no `prompt_cache_key`) decide luna. B7 is
  the largest single loss in the window.
- B9 (compaction) does not thrash: it fires at 70% of the context window
  (~90K tokens on a 128K model) and keeps 20K recent tokens, so it rewrites
  history once per threshold crossing, not once per turn.
- B2, B5, B8, B10, B11 are unchanged by this work.

## Expected effect (simulated on the baseline)

`simulate.py` (scratchpad; method in the report): a later call caches the
previous call's prompt if within the TTL (deepseek 1 h, luna 5 min), a
first call caches the desk's static prefix (its smallest first-call prompt)
if the desk called within the TTL.

| Desk | Model | Hit now | Hit expected |
|---|---|---|---|
| research | deepseek | 91.6% | 94.8% |
| management | deepseek | 86.9% | 90.9% |
| texture | deepseek | 68.9% | 82.8% |
| research | luna | 10.0% | 89.0% |
| texture | luna | 4.7% | 78.5% |
| management | luna | 4.1% | 78.7% |
| all | | **71.5%** | **~90%** |

Input dollars in the window at list prices: $0.774 → ~$0.42 (−46%).

## Failure modes, written before the code

- F1. A volatile field stays in the system prompt or the tool catalog
  (time, run id, assignment id/kind/subject digest/candidate id, requester
  and delivery agent ids, channel id, Texture self-development operation id,
  conductor seed prompt, management task id and capture count, research
  child time budget). Guard: a test renders every desk's prompt for two
  different runs at two different times and requires byte equality.
- F2. The model loses something it needs: the time, its assignment, its
  requester. Guard: the facts move to a run-context user message at the
  start of the run, and tests assert they are there.
- F3. The time refreshed on a resumed activation is sent but not
  persisted: the next activation's replay then differs from what was sent
  and breaks mid-history. Guard: the refresh is appended to run memory like
  any other turn.
- F4. The refresh lands where a user turn is invalid (after an unanswered
  tool call). It goes in at the same point as the Texture frame injection,
  which already appends user turns there.
- F5. Compaction summarises the run-context message away. Guard: after a
  compaction the run context is appended again (a compaction breaks the
  cache anyway).
- F6. Key too fine (per run: no reuse across runs) or too coarse (one key
  for every computer: OpenAI spreads a key past ~15 requests a minute, and
  one OpenCode "conversation" across many users). Key = computer + desk
  thread + prompt version.
- F7. The key leaks owner/computer identifiers to a provider. It is a hash.
- F8. A provider rejects an unknown request field. `prompt_cache_key` is
  sent only on the chatgpt route (Codex sends it there); other routes get
  only what they document.
- F9. Failover keeps a session pinned to a failing upstream. The key takes
  the same `#failover` suffix the conversation id takes.
- F10. Version skew between guest and gateway. The gateway decodes
  non-strictly and the field is `omitempty`: an old gateway ignores it, a
  new gateway without it falls back to the conversation id.

## Rollback

Revert the fix commits. No stored data changes shape: run-context messages
are ordinary user turns in run memory and replay as such after a revert.

## Repair (branch, not deployed)

### 1. The system prompt is byte-stable per desk (B1, B3, B4)

`systemPromptForRun` now renders only what every run of a desk shares: the
core, the role prompt, skills, the role overlays, the generated surface and
the peer-coordination sentence (`peer_coordination.yaml`). Everything
volatile moved to a **run-context turn** (`internal/agentcore/run_context.go`):

| Moved out of the system prompt | Now in |
|---|---|
| Current UTC date/time (all desks, management tasks) | run-context turn, every activation |
| Current agent id, requester agent id, Texture delivery scope, channel id | run-context turn, first activation |
| Engineering exact assignment (assignment_id, kind, subject_digest, candidate_id) | same |
| Texture self-development supervision note (operation id) | same |
| Conductor requested app and seed prompt | same |
| Management task id, capture count, inbox line | same |
| Research child time budget ("You have about …") | the child's first message |

How the turn is placed, so the history stays append-only:

- a new run's first message is `Run context:` + the time + the run's facts;
- a resumed activation (Texture after each passivation, management and
  research rewakes) appends `Run context:` + the time after its replayed
  history, **persisted** in run memory, so the next activation replays the
  same bytes (test: three activations, each a byte prefix of the next);
- a compaction appends the time and the facts again after the summary,
  and the "never compact twice in a row" guard skips that turn.

What stays and is stable by construction: the management intake-labels
overlay (a computer setting) and engineering's four overlay variants
(two flags). The settings preview (`EffectiveSystemPrompt`) no longer shows
the time or ids; it shows what the provider caches.

### 2. Cache keys per desk thread, per provider (B6, B7)

The tool loop derives `CacheKey = choir-<role>-<hash(owner, computer,
agent)>-<hash(system prompt, tool definitions)>` once per activation
(`toolregistry.PromptCacheKey`); the same desk on the same computer at the
same prompt version gets the same key across runs; a new prompt version
gets a new one; ids are hashed. It travels `ToolLoopRequest.CacheKey` →
gatewayruntime `cache_key` → gateway `ProviderRequest.CacheKey` →
`provider.LLMRequest.CacheKey`. A failover suffixes it (`-failover`), as the
conversation id is. Research children share one key per parent desk.

| Route | Provider caching | What Choir sends now |
|---|---|---|
| chatgpt (gpt-6-luna), Responses | automatic ≥1024 tokens, looked up by prefix hash + `prompt_cache_key` | `prompt_cache_key` = desk key, else the run's conversation id (so a gateway-only deploy already fixes within-run luna) |
| opencode-go chat completions (deepseek-v4.1-flash, cat-jam-free, glm, hy3, longcat, step) | upstream automatic prefix caching; OpenCode routes a session to one upstream "for routing and prompt-cache affinity" | `x-opencode-session` = desk key (was the run id); no body field |
| opencode-go Responses (muse-spark) | session affinity only is documented | `x-opencode-session` = desk key; no `prompt_cache_key` (pass-through unverified) |
| opencode-go Messages (qwen3.7-max) | unverified | `x-opencode-session` = desk key; no `cache_control` (not in the pool) |
| bedrock (Anthropic Messages) | explicit breakpoints only | system breakpoint (existing) + a breakpoint on the last message's last non-thinking block (new) |
| zai (GLM, Anthropic-shaped) | automatic context caching | nothing |
| fireworks | automatic prefix caching | nothing (not in the pool) |
| gateway OpenAI-compatible endpoint | — | accepts the caller's `prompt_cache_key` and passes it as `CacheKey` |
| Jev (OpenRouter) | short one-shots | unchanged |

### 3. Message history order (B9, unchanged by design)

The loop appends only; run memory replays stored bytes; compaction fires at
70% of the window with 20K recent tokens kept, so it rewrites the prefix
once per threshold crossing, not per turn. B2 (a new run seeds a summary as
message 0), B5 (pool member by trajectory, a routing change), B8, B10 and
B11 are left as residuals.

### Measurement plan (after deploy)

```
python3 -I costs.py   <deploy time>   ""   after.json   # summary + per desk/model
python3 -I percall.py after.json                         # first vs later calls
```

(the scripts read the QA credential inside python and never print it;
copies are in the session scratchpad.) Compare with the baseline above,
same computer, same window length. Pass criteria:

- luna later calls ≥ 70% (from 5-10%); if they stay near 0, B7 was not
  the cause and the next suspect is the chatgpt backend ignoring the key;
- deepseek first calls > 50% (from 0-6%): the static system prompt and
  tools now hit;
- deepseek Texture ≥ 80% (from 69%): re-activations keep their history;
- summary `cache_hit_rate` ≈ 0.9 (from 0.715).

The run-context turn is visible on any run: `provider_call_started`
`system_preview` no longer contains "Current UTC date/time"; the first
message does.
