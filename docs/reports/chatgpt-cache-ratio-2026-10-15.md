# ChatGPT prompt-cache ratio on the paper computer (2026-10-15)

Status: green (docs only). Owner priority: the paper's chatgpt cache ratio
above 90%, prompts append-only, no economizing on search. Companion to
[autopaper-compute-halving-2026-10-15.md](autopaper-compute-halving-2026-10-15.md)
and [the 72-76% problem doc](../problems/paper-chatgpt-cache-ratio-below-85-after-restarts-2026-10-15.md).

Tags: **[E]** evidence read from the Node B gateway journal, **[C]** read from
code, **[H]** hypothesis.

## 0. What the journal can and cannot show

[E] The gateway logs, per call, `vm`, `provider`, `model`, `messages=`,
`tools=`, `system_chars=`, `tokens_in+out`, `cached_in`. It does **not** log
the cache key, the `PromptCacheKey` label, the desk, or a run/thread id. So
the label/key breakdown the task asked for cannot be read from the journal.
The only desk-like discriminator is `system_chars`. For the paper's 5,303
chatgpt calls since 07:00 UTC (all `gpt-6-luna`, `tools=1`) it takes exactly
three values for the whole window: 11,922, 19,638 and 25,449. Below they are
called **system A / B / C**. The label names the code can emit
(`agentProfileForRun`, `management_task`, `research_child`, `run`) are in
`promptCacheScope` / `researchChildCacheScope` [C]; mapping A/B/C onto them is
not possible from the journal. [H] From prompt-file sizes plus the generated
verb list, A is probably Texture, B research, C management, and B's share of
input (49%) matches research's 53% share in the compute-halving report, but
this is unconfirmed. Adding `cache_key` (a hash is enough) and the label to the
`provider: ... stream complete` log line is the first lever (section 4).

Method: 22,908 paper-VM gateway lines (07:00 to 16:xx UTC) parsed; request
and success lines paired FIFO per (vm, provider); calls chained into threads
by (same `system_chars`, `messages` +1..8, input not shrinking, under 1 h).
Scratch scripts were in the session scratchpad, not committed. **Caveat:**
the chaining is a reconstruction. 1,244 of 3,851 "later" calls report a cached
count above their reconstructed parent's input, so some links are wrong
(interleaved threads, or the backend caching more than the parent's input).
Per-thread split numbers below are therefore +/- a few points; the aggregate
hourly numbers are exact, and the isolated-call check (section 3b) is clean.

## 1. Per hour (paper, chatgpt)

[E] Hour = UTC. The paper was silent on chatgpt from about 10:56 to 12:40
(host reboot, see `paper-silent-since-10-56-after-host-reboot-2026-10-15.md`),
so there is no 11:xx row.

| hour | calls | input tokens | cached | cached % | notes |
|---|---|---|---|---|---|
| 07 | 544 | 5.73M | 2.07M | 36.1 | cache-routing deploy 07:52:27 (gateway); ramp |
| 08 | 728 | 9.70M | 7.25M | 74.7 | gateway restarts 08:05, 08:23 |
| 09 | 343 | 7.61M | 6.63M | 87.1 | gateway restart 09:50 |
| 10 | 110 | 1.37M | 0.94M | 68.7 | restarts 10:40, 10:49/10:50 (reboot) |
| 12 | 407 | 4.89M | 3.71M | 75.8 | silent until ~12:40 |
| 13 | 1,201 | 27.54M | 24.24M | 88.0 | restarts 13:14, 13:54 |
| 14 | 997 | 12.16M | 9.27M | 76.2 | restart 14:56 |
| 15 | 973 | 10.97M | 7.95M | 72.5 | restarts 15:16, 15:39 |

Window total 07:00-16:00: 5,303 calls, 79.97M input, 62.04M cached = 77.6%.
Steady state is about 81%: 08:30-10:59 = 81.2%, 12:40-16:00 = 81.3%.

The 88% hour (13h) and the 87% hour (09h) are **composition**, not a better
cache: 13h carries 27.5M tokens in 1,201 calls, with a few threads at
100k-250k-token contexts that cache at 96-99%. The ratio is token-weighted, so
a handful of huge, well-cached calls decides the hour.

[E] Cached % by call size (all hours):

| input tokens per call | calls | input | uncached | cached % |
|---|---|---|---|---|
| <5k | 309 | 1.3M | 0.82M | 38.5 |
| 5-8k | 1,543 | 10.2M | 4.81M | 52.7 |
| 8-12k | 1,533 | 14.8M | 5.39M | 63.6 |
| 12-20k | 1,047 | 16.1M | 3.94M | 75.4 |
| 20-40k | 594 | 15.8M | 2.31M | 85.3 |
| 40-80k | 185 | 10.5M | 0.50M | 95.3 |
| 80k+ | 92 | 11.3M | 0.16M | 98.6 |

Calls under 12k tokens are 64% of calls, 33% of input and **61% of uncached
tokens (11.0M of 17.9M)**. Thread length: 1,571 of the reconstructed threads'
calls are in 2-3-call threads, 2,172 in 4-6-call threads; the median thread is
about 3 calls, so most threads are short.

### Restart effect

[E] Cached % by minutes since the most recent deploy/boot (after 08:24, so the
07:52 ramp is excluded): 0-5 min 70.3% (300 calls, 3.4M); 5-10 min 79.5%;
10-20 min 87.2%; 20-40 min 76.1%; 40-90 min 81.6%. So the first ~5 minutes run
about 10 points below steady state, and it is gone by minute 5-10. Those 5
minutes hold about 4% of tokens, so restarts cost roughly **0.4 points** of the
ratio, not 10. Per-event 20-min windows are mixed (13:14 +19 points because a
huge cached thread ran; 13:54 -15; 14:56 -4; 15:16 -4; 15:39 +6). The problem
doc's hypothesis "restarts keep it at 70-76%" is **not supported**: the same
ratio (81%) holds in the long quiet window 08:30-10:59 and after 12:40. The
ratio is low because of what happens inside threads, not because of restarts.

## 2. Where the uncached tokens are

[E] Total uncached 17.93M of 79.97M (22.4%).

| system | calls | input | uncached | share of uncached | cached % |
|---|---|---|---|---|---|
| A (11,922 chars) | 1,870 | 22.1M | 6.74M | 37.6% | 69.5 |
| B (19,638 chars) | 2,024 | 39.1M | 6.37M | 35.5% | 83.7 |
| C (25,449 chars) | 1,409 | 18.7M | 4.82M | 26.9% | 74.2 |

By call position (reconstructed): first call of a thread 1,452 calls, 4.84M
uncached (27%); calls 2+ in a thread 3,851 calls, 13.1M uncached (73%).

Decomposition of the 17.9M uncached, approximate (+/- a few points, see the
chaining caveat):

| part | tokens | % of all input | explanation |
|---|---|---|---|
| later calls, prefix not recovered ("mid-thread miss") | ~6.5M | 8.1 | cached count is below what the previous call of the thread should have left |
| first calls (fresh run 1.4M + resumed activation 3.4M) | ~4.8M | 6.0 | unique content (run context, inbox, frame) after a static prefix; resumed activations replay history |
| later calls, the appended delta and the 1,024 grid tail | ~6.6M | 8.3 | previous output plus tool result, rounded to the cache block; append-only makes this irreducible |

Cached counts are always `512 + 1024k` (4,700 calls) or 0 (603 calls)
[E]: the backend caches in 1,024-token blocks offset by 512, so each call loses
0-1,023 tokens at the tail on top of the real delta. That matters for the
short calls, which are most of the uncached volume.

## 3. Why they are not cached

### 3a. Candidates ruled out

- **Prefix below the 1,024 minimum.** [E] Smallest first call is 3,469
  tokens; the static prefix (system plus tools) alone is about 2.5k-5.6k.
  Not the cause.
- **Volatile content early in the prefix.** [C] `systemPromptForRun`
  (`internal/agentcore/tool_profiles.go`) carries only stable text; the time
  and ids are in the `Run context:` user turn (`run_context.go`). [E]
  `system_chars` is constant per desk for 8+ hours and across 12 restarts. The
  run-context turn holds a time to the second, but it is message 0 of a thread
  and replayed byte-identically (the persisted turn is replayed, not
  re-rendered); within one thread it is not a per-call change. Across threads it
  only affects the unique tail, not the static prefix before it.
- **Tool definitions reordered.** [E] `tools=1` on every call. [C]
  `convertOpenAITools` keeps order and `InputSchema` is a map, which Go
  marshals with sorted keys. `chatgpt_cache_stability_test.go` asserts that
  consecutive requests share every byte up to the new turn. Not the cause on
  our side, with the caveat that the test uses synthetic messages, not the live
  run memory replay (`contextMessages` rebuilds the history from the store
  every call).
- **Cache TTL lapsing between calls.** [E] For later calls with a reliable
  parent, full-hit share by gap since the parent finished: 0-2 s 62%, 3-10 s
  64%, 11-30 s 65%, 31-60 s 64%, 1-2 min 67%, 2-5 min 62%, 5-10 min 54%,
  10+ min 58%; prefix recovered 81-86% at every gap. The miss rate is flat in
  time, so a 5-10 minute TTL does not explain the misses.
- **Concurrency / too many concurrent calls on one prefix.** [E] Miss rate does
  not rise with the number of in-flight same-system calls (42-52% at 1..6).
- **Compaction.** [C] `compactIfNeeded` rewrites the prefix when a threshold is
  crossed; the 40k+ calls cache at 95-99%, so it is not a visible share.
- **Restarts.** [E] Section 1: about 0.4 points.

### 3b. What is left: the backend does not find the previous call's cache

[E] Cleanest evidence: 110 later calls with no other chatgpt call overlapping
them or their parent (so the parent link is certain, gap about 1 s): 51 full
hits, 44 partial, 15 zero, i.e. **54% miss**, even one second after the
previous call finished. Example thread from the journal (12:10, system B,
`messages` 2,4,6,8,10): input 4,713/cached 0; 5,266/4,608; 5,772/4,608;
6,142/**0**; 6,667/5,632. A call that cached 4,608 is followed by a call that
caches 0, then 5,632.

[E] The miss values cluster on a few numbers (0, 2,560, 3,584, 4,608, 5,632
...). The existing test file notes 2,560/3,584/5,632 are the three desks'
static prefix sizes: a miss typically still hits the shared static prefix
(system plus tools, identical across threads of a desk) but not the
conversation after it. That is the signature of the request landing on a cache
machine that has the static prefix but not this thread's earlier calls.

[E] Before the 07:52 headers deploy the mid-thread miss rate was 87% (07h);
afterwards 29-53% per hour. The headers helped a lot but did not make the
routing sticky. The remaining ~45-50% miss rate does not depend on gap, time
of day, size, desk or concurrency, and survived 12 restarts.

[H] Root-cause verdict: the dominant remaining loss (about 8 points of the
22) is **imperfect affinity on the ChatGPT backend** for a stable
`prompt_cache_key` plus `session_id`/`conversation_id`, not anything in our
prompt bytes (which the code and tests say are append-only). It is a
hypothesis because the journal has no per-call prefix hash; confirming whether
our bytes are identical is the first lever. Secondary [H] factors: the account
is shared by 995 non-paper chatgpt calls in the same window (other computers
and the search plane use the same Codex auth), so the backend's per-account
load balancing may also fragment affinity; and a very small prefix is cheap
for the backend to evict.

### 3c. First calls (27% of uncached)

[E] Fresh-run first calls cache 73% (5.28M in, 1.42M uncached); 77 of 452
had cached = 0 although the static prefix is 3-6k tokens (the same
mis-routing as 3b). Resumed activations (`messages` over 3 at the first call)
cache 68% (10.6M in, 3.4M uncached). Resumed first calls by size: under 10k
tokens 49.5% (817 calls), 10-30k 70.5%, 30-70k 89%, 70k+ 98%. By the time since
the nearest earlier call of the same system with a smaller history (matching
is heuristic): under 1 min 83%, 1-60 min 55-61%, over 60 min 63%. Most of this
is the unique content (inbox records, the Texture frame with the document
head, assignment text) that is new by construction, and it is a bigger share
of a small call. Not reducible without dropping content.

[C] Texture's frame is a separate new user turn appended after replayed
history (`FrameHeader`, `internal/texturewrite/frame.go`), not a rewrite of an
earlier turn, so it is already append-only.

## 4. Levers, ranked

Gains are on the 77.6% window average (81% steady); they are estimates from
the decomposition above, with the chaining caveat.

| rank | lever | expected gain | risk | notes |
|---|---|---|---|---|
| 1 | **Add `cache_key` (hash), label, and a prefix hash (instructions, tools, each input item boundary) to the gateway per-call log**, then re-run this analysis per key | 0 directly; decides whether levers 2-3 can work | low (green/yellow: log field) | the journal today cannot separate "our bytes drifted" from "the backend routed away". Without it the 8-point loss cannot be assigned |
| 2 | **Fix mid-thread misses**. If the prefix hash shows our bytes are stable: A/B the routing key (a coarser key per desk role instead of per desk thread; or a key per run; headers on/off), and try `prompt_cache_retention` if the Codex backend accepts it | up to +8 points if all misses go (77.6 -> 85.7); +4 if half (81.6) | orange/red (provider requests); the data does not say a coarser key helps (miss rate is independent of concurrency and gap); a coarser key risks overflow past about 15 requests/min per key on the API | the only lever with enough size to reach the 85-90% band; unproven |
| 3 | **If the prefix hash shows drift**, fix it (the live run-memory replay, tool-output re-serialisation) | same +8 | low once located | would be the cheapest case: our bug, no provider dependence |
| 4 | Lengthen the **stable, shared prefix** (desk-stable instructions, skills text placed in the system prompt, ahead of anything unique) | +2.5 points per extra 2k stable tokens on every thread (calculated: (62.0+10.6)/(80.0+10.6) = 80.1%) | red-adjacent (prompt text), costs real tokens on every call at the cached price | **raises the metric, not the savings**: it adds cached tokens, and a miss makes them expensive. Do only if the content is wanted anyway |
| 5 | Fewer, longer threads (keep a desk's thread instead of new runs): fewer first calls and resumed-activation uncached content | maybe +1 to +3 | design change in desk activation | first calls are 6 points of uncached; the unique content does not shrink, but the static prefix is amortised |
| 6 | Keep-warm pings | about 0 | wasted calls | gap-flat miss rate says TTL is not binding; do not do |
| 7 | Restart avoidance | about +0.4 | operational | restart effect is 5 minutes, about 10 points on 4% of tokens |

Not recommended: elision or search trimming (owner ruling, and it rewrites
cached bytes).

Reachability: with zero mid-thread misses the window is about 85.7%. The rest
(first-call unique content 6.0 points, appended delta plus 1,024-grid tail 8.3
points) is irreducible under append-only without padding. **90% is not
reachable by fixing the miss rate alone**; it needs lever 2 fully plus about
4 points from lever 4 or 5. A realistic target without padding is 84-86% on
the current mix; the 13h-style hours above 88% come from long, huge-context
runs, not from a better cache.

## 5. Next measurement

1. Ship the log fields in lever 1 (gateway, `internal/provider/provider.go`
   log lines near 2551/2721, plus the label in the request). Class: orange (a
   log line; nothing sensitive: hash only, no key or token).
2. After 2 hours with no deploy, rebuild the table by (label, key): calls,
   first vs later, cached %, and whether a missed call's prefix hash equals the
   previous call's.
3. Only then change the key scope or the retention parameter.

Mutation class of this document: green. Heresy delta: discovered (the
problem doc's restart hypothesis is not supported by the journal; the journal
lacks the cache key), introduced none, repaired none.

## 6. Lever 1 implementation (2026-10-15)

Mutation class: orange (a log line in the provider stream parser; no request
or routing change). Rollback: revert the commit; the log line returns to its
old shape. Branch: `feat/gateway-cache-key-logging`, not pushed.

The per-call usage line is the `provider: chatgpt stream complete` line in
`internal/provider/provider.go` (`parseOpenAIStream`). On the chatgpt route it
gains these fields, after the existing ones:

| field | meaning |
|---|---|
| `cache_key` | the `prompt_cache_key` sent. `toolregistry.PromptCacheKey` builds it as `choir-<label>-<sha256(scope)[:n]>-<sha256(system+tools)[:n]>`: the label is a sanitised desk label, both other parts are hashes, no owner, computer or agent id appears. When the caller names no key the sent key falls back to the run's conversation id; a key that does not start with `choir-` is logged as `h:<short sha256>` instead, so an id never reaches the log |
| `cache_label` | the label part of a `choir-` key (`-` otherwise) |
| `input_messages` | number of Responses input items sent |
| `prefix_hash` | short SHA-256 (16 hex chars) of the full request input: instructions, tool definitions, then every input item as JSON, in order. This is what the next call of the thread must extend |
| `prefix_hash_prev` | the same hash over the input with the last appended turn removed (`-` when there is no earlier model output, i.e. the first call). The last appended turn is the trailing run of non-model items (tool outputs, user messages) plus the model-output block before it (function calls, assistant messages) |

Check for analysis: for a thread, `prefix_hash_prev` of call N must equal
`prefix_hash` of call N-1. Equal and still a cache miss: the backend routed
away (lever 2). Unequal: our bytes drifted (lever 3). Hashes only; no prompt
text, tool output, or argument value is logged. Cost: one SHA-256 pass over the
request bytes per call (items are marshalled once more for hashing), a few
thousand calls an hour.

Heresy delta: discovered none, introduced none, repaired none (measurement
only). Next step is section 5 item 2 after a deploy and two quiet hours.

## 7. First data after lever 1 deployed (8f2f0dc5d, about 16:40 UTC)

In 15 minutes of `provider: chatgpt stream complete` lines on Node B,
**48 of 75 paper calls (64%) carried a fallback key** (`cache_key=h:<hash>`,
`cache_label=-`). Only 27 used desk keys: `choir-management` 18,
`choir-research` 4, `choir-texture` 5. So most calls never set
`PromptCacheKey` and fall back to the per-conversation id, which means
cache affinity is per conversation, not per desk thread. This is the
leading candidate for the mid-thread misses in §3 (evidence: the label
counts; hypothesis: that it drives the misses). Next: find which callers
reach the chatgpt route without a cache key (grep for the provider call
sites that leave `CacheKey` empty), and measure the hit rate of `h:` calls
against `choir-` calls once a day of data exists, by grouping on
`cache_label` and checking `prefix_hash_prev` against the previous
`prefix_hash`.

### 7.1 Which calls carry the fallback key (coordinator, about 17:00 UTC)

- **Evidence:** the `cache_label=-` calls are multi-turn tool loops
  (gpt-6-luna, about 11–12k input tokens, `stop=tool_use` then `end_turn`,
  4–19 input messages). Their prefix chain is intact: each call's
  `prefix_hash_prev` equals the previous call's `prefix_hash` (for example
  ce2a779e… → next call's prev ce2a779e…), and that pair cached 10,752 of
  11,957 tokens. So these loops are append-only and cache well within
  their own conversation. They just don't share a desk thread key.
- **Evidence:** only two tool-loop entry points exist,
  `runtime.go:3689` (desks, with `promptCacheScope(rec)`, which never
  returns an empty scope because `agentIDForRun` falls back to the RunID)
  and `research_children.go:220` (research children, with
  `researchChildCacheScope(parent)`, label `research_child`).
  `PromptCacheKey` returns "" for an empty scope, and the provider then
  uses the conversation id `research-child:<childKey>`, logged as `h:`.
- **Hypothesis (leading):** the fallback calls are research children whose
  `parent.AgentID` is empty, so `researchChildCacheScope` returns an empty
  scope. No `cache_label=research_child` line appeared in 30 minutes,
  which supports it. If so, each child is its own cache conversation and
  siblings never share the system prompt and tools prefix. The first call
  of every child is cold (§3: about 4.8M first-call tokens).
- **Next check:** log or derive where the research child's `parent`
  ExecutionContext is built (`runResearchChild`, `childExec := parent`)
  and whether it carries AgentID. If it doesn't, scope children by the
  parent's computer and desk agent, and fall back to the parent run id
  rather than to nothing. Expected gain: sibling children would share
  their first call's prefix (system prompt plus tools, about 3–6k tokens
  each).

### 7.2 Research-child hypothesis refuted from code; failover keys are the likelier source (green)

- **Refuted [C]: `parent.AgentID` is not empty.** The parent passed to
  `runResearchChildren` (`tools_desk.go:1048`) is `cellExec`, taken from
  `ExecutionContextFrom(egrCtx)` or, if that carries no run, from the
  desk tool's `execCtx` (`tools_desk.go:419-421`). That context is installed
  by `toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(rec))`
  (`runtime.go:3521`), which sets `AgentID: agentIDForRun(rec)`, `OwnerID`
  and `ComputerID` from the run (`tool_profiles.go:39-49`).
  `agentIDForRun` falls back to `rec.RunID` (`tool_profiles.go:117-130`),
  so AgentID is non-empty for every run with a record. `runResearchChild`
  copies the parent (`childExec := parent`, `research_children.go:190`) and
  clears only RunRecord, ChannelID and ToolCallID, so nested children keep
  it too. `researchChildCacheScope` (`research_children.go:271`) therefore
  returns a non-empty scope and children get a `research_child` key. No code
  path found that builds a parent with an empty AgentID. Not hypothesis-
  tested against a live ledger; the absence of `cache_label=research_child`
  in 30 minutes is explained below without it.
- **Finding [C, reproduced]: a failed-over call is logged as `h:` / `-`.**
  On model failover `RunToolLoop` sets `cacheKey = baseCacheKey + "-failover"`
  (plus a counter from the second fallback), `toolloop.go:583-588`.
  `loggableCacheKey` (`internal/provider/cache_log.go:85-97`) recognises a
  desk key only when `strings.Split(rest, "-")` has exactly 3 parts. A
  failover key has 4, so it is logged as `h:<hash of key>` with
  `cache_label=-`. A throwaway test printed, for
  `choir-management-<scope>-<prompt>`: key as sent, label `management`;
  with `-failover`: `h:bf446ea518086703`, label `-`; with `-failover2`:
  `h:ba0750e7e3d3793b`, label `-`.
- **Reading:** the `cache_label=-` calls are plausibly calls that failed
  over to gpt-6-luna on the chatgpt route (docs/problems/free-model-429-
  opens-provider-circuit-stub-stories-2026-10-14.md describes the free
  models 429ing), from any desk or child. That fits every observation in
  §7.1: multi-turn tool loops, an intact `prefix_hash_prev` chain (the
  failover key is stable for the rest of the loop, so the loop caches
  within itself), and no `research_child` label. It is a hypothesis until a
  ledger shows `model_failover` run.retry events on the same runs
  (`toolloop.go:595-603`), which is the next check. Mechanism of harm, if
  true: the failover key is deliberately distinct from the desk key, so a
  failed-over call does not share the desk's cache thread. That is by design
  (the key doubles as OpenCode's session), so the lever is fewer failovers,
  not a different key. The compactor (`run_memory.go:680`) sets no CacheKey
  and uses its own provider path; it is a single-shot call with 1 input
  message and is not a tool loop.
- **Decision:** no code change. The logging blind spot (a 4-part key shown
  as `h:`) is worth a one-line fix later (label the failover suffix) so
  the next measurement separates failover from keyless calls; not made here.

### 7.3 Root cause found: OpenCode Go out of funds, failover keys shard the cache (coordinator, about 17:45 UTC)

- **Evidence (after b5c2bcaa5 labelled failover keys):** in 8 minutes of
  paper chatgpt calls there were **no keyless calls**. Failover share:
  - research: 35 primary-key, 28 `+failover`, 41 `+failover2`;
  - texture: 57 primary-key, 13 `+failover`, 16 `+failover2`;
  - management: 11 primary-key, 8 `+failover`, 15 `+failover2`.

  About 55% of chatgpt calls are failovers.
- **Evidence (gateway, 30 min):**
  - 99 calls failed with `provider opencode-go model deepseek-v4.1-flash:
    circuit open`;
  - 88 with `opencode-go: every OpenCode key is parked (out of funds or
    rate limited)`;
  - 54 with `cat-jam-free: circuit open`;
  - 14 with upstream 429, and 2 with
    `402 … Insufficient account funds`.

  The paper's primary and secondary models are on OpenCode Go, whose
  account is out of funds.
- **Mechanism (hypothesis, strong):** `toolloop.go:583-588` appends an
  ordinal suffix (`-failover`, `-failover2`) to the cache key. Which
  ordinal chatgpt gets depends on which earlier models are open at that
  moment, so one desk thread's chatgpt calls are spread over 2–3 keys and
  later calls often land on a cold key. That is the "mid-thread misses"
  of §3, so this is our side, not backend routing.
- **Repairs:**
  1. Key failover by the serving provider and model, not by ordinal, so
     every chatgpt call of a thread shares one key.
  2. While a model's circuit is open, skip it without a call.
  3. **Owner decision: top up OpenCode Go** (402 insufficient funds), or
     change the paper's model policy to put chatgpt first while OpenCode
     is unfunded.

- **Fix (branch fix/failover-cache-key-by-model, orange):** the failover
  suffix now names the serving selection instead of the ordinal. A call
  served by the primary selection keeps the base key; a failed-over call
  uses `<base>-m<6 hex of sha256(provider/model)>`, so every call of a
  thread served by chatgpt/gpt-6-luna shares one key whether chatgpt is
  failover1 or failover2, while different models still get different keys.
  The tag adds 8 characters, keeping a key at most 64 characters and free of
  owner or computer ids. `loggableCacheKey` labels these `<desk>+m<tag>` and
  still labels legacy `-failover<N>` keys. Expected gateway effect: the
  `+failover` and `+failover2` chatgpt labels collapse into one
  `+m<tag>` label per desk and the mid-thread cold-key misses stop. Rollback:
  git revert the code commit.
