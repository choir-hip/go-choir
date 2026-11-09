# Append-only and prompt-cache audit (read-only, 2026-11-09 evening)

Code read at /Users/wiz/Dev/go-choir/.claude/worktrees/followup-token (main = 5dc98bcf5, the deployed commit on the paper). The frame-windowing change was read in /Users/wiz/Dev/go-choir/.claude/worktrees/agent-a5a71f368574fa978 (4 commits on top of 5dc98bcf5). Raw data and scripts: `$SCR/cacheaudit/` (SCR = /private/tmp/claude-501/-Users-wiz-Dev-go-choir/f298736e-cabb-51ff-9cea-34588a4989cb/scratchpad). Labels: **M** = measured (command given), **I** = inferred from code or from M, **H** = hypothesis. Nothing was written to any computer; only GETs through the three helpers and a metadata-only gateway journal read.

Mission found: the 2026-10-14 cache mission, `docs/problems/prompt-cache-breaks-across-turns-2026-10-14.md` (+ `docs/archive/reports/prompt-cache-analysis-2026-10-14.md`, `chatgpt-cache-ratio-2026-10-15.md`). It moved all volatile text (time, ids, assignment) out of the system prompt into a run-context user turn (`internal/agentcore/run_context.go`), added `prompt_cache_key` for chatgpt, `x-opencode-session`, OpenRouter `session_id`. That fixed the system prefix and same-run replays. It did not cover the Texture carry path that came later (`texture.hold_trajectory`, 2026-10-25), and that path is where the largest defect now is.

## Headline

**The Texture wake path is not byte-for-byte append-only.** 88% of the paper's Texture wakes (276 of 315 runs, M) start a NEW run that carries the previous conversation through `carryActorConversation`. The carry deletes every run-context turn from the old transcript (including the first message of the transcript), then appends a new one. So the new request shares only system+tools with the previous request plus whatever the older runs contributed before the last run-context turn. On Synthetic (a real prefix cache) the first call of a carried wake hits **22%** (actual) against **87%** (what a pure append would give): median **25.4k fresh tokens per wake against 8.2k** (M, 44 wakes, d@e and b@c). The one-line cause is `internal/agentcore/run_memory.go:267`.

Second defect, equal in size on Bedrock: the cache TTL. `cache_control` is plain `{"type":"ephemeral"}` (5 minutes, `internal/provider/provider.go:524,2008`). Texture wake gaps on the paper are p50 628 s, and 82% exceed 300 s (M). Even a perfectly append-only Texture would find a cold cache on 4 wakes in 5. The 1-hour TTL covers 99% of wakes.

## 1. How a request is built, per desk

Common path (all desks that run as a run): `executeWithToolLoop` (`internal/agentcore/runtime.go:4018` area; lines cited below).

1. **System prompt**: `systemPromptForRun` (`internal/agentcore/tool_profiles.go:184-270`): core prompt + stored role prompt + overlay for the desk (management/engineering/research) + generated verb surface (`deskSurfacePrompt`, :476) + peer coordination. Built from desk and computer-setting inputs only; time, ids, requester, assignment moved to the run-context turn (`run_context.go:1-30`). `toolregistry.BuildSystemPrompt` appends a one-line tool catalog (`toolregistry.go:189`).
2. **Tools**: one tool on every desk, `desk_go_eval` (`capsule_go_eval` for engineering). Static text.
3. **Messages**: `memory.initialize` (`run_memory.go:90`): if the run has no memory yet, `carryTexturePriorConversation` (Texture only, :131) then `appendMessages(initialMessages)`, where `initialMessages = [runContext(time, facts), prompt]` (`runtime.go:4039-4046,4111`). If the run already has memory (reactivation of the same run), the stored messages replay and only a time-only `resumeTurn` is appended (`run_memory.go:100-107`, `runtime.go:4124`).
4. **Injected turns** after each tool batch (`toolloop.go:1458-1465`): Texture frame (`texture_frame.go:175`, whole document as blocks + unread records, only when revision or cursor changed), coagent update packets (`management_controller.go:2515`), calibration digest, shot images. They are appended with role `runtime_injection`.
5. **Every iteration rebuilds `messages` from the store** (`toolloop.go:1017-1021` calls `beforeProviderCall`, which returns `buildRunMemoryContext(entries)`, `run_memory.go:1101`). Stored messages are cloned verbatim, so an unchanged store gives identical bytes.
6. **Wire**: `gatewayruntime.convertRawMessages` (`internal/gatewayruntime/provider.go:485`) re-parses each message and re-encodes `tool_use.input` through `canonicalJSON` (sorted keys; deterministic; idempotent). The gateway then converts again per provider (`convertMessages` Bedrock, `convertOpenAIChatMessages` Synthetic/River/opencode-go, `provider.go:2055,1819`). Both are pure functions of the stored message, so they are byte-stable per message.

Per desk:

| desk | what persists between wakes | what a wake adds |
|---|---|---|
| Texture | a NEW run per committed wake (mutation complete) seeded by the carry; or the SAME run reactivated when it passivated without committing (`textureowner/texture_controller.go:1066`) | wake prompt (`textureowner/texture_agent_revision.go:853`: restates seed request and all owner directions every wake, about 5 k chars), run-context turn, frame, coagent packets |
| Texture, parked | the loop parks 120 s idle (`actor_park_idle_seconds`); an update arriving then is injected into the live loop (pure append) | injector turn |
| management (persistent) | the same run is reactivated (`management_controller.go:501`), replay + `resumeTurn` | update packets as injected turns; threshold compaction |
| management_task, research, engineering | nothing carried; "Every other desk starts from its system prompt and REPL only" (`run_memory.go:126-130`) | n/a |
| research children (parallel searchers) | none: plain in-memory loop with no memory hooks (`research_children.go:353`), so no compaction and no persistence; trivially append-only | n/a |

Texture wake from a research interim: research sends a record to Texture's inbox; the textureowner starts a run with `request_source=inbox_record`, `records_route`, `actor_rewarm_carry_conversation` (paper: 276 such runs; the 39 `update_coagent` ones are same-run reactivations) (M: `paper.sh '/api/runs?limit=500'`, counted by run metadata).

**Does a wake append or re-render?** The new content (prompt, frame, packets) is appended. But the carry rewrites the old transcript before the append: it drops run-context turns (`run_memory.go:262-273`), drops tool results whose call is not carried, and cuts a trailing unanswered tool_use. The first drop is the defect: the previous run's transcript began with its run-context turn, so after the carry message 0 differs from the previous request's message 0.

## 2. Everything that trims, truncates, compacts, drops or re-renders

"Pre-breakpoint mutation" = changes bytes that an earlier request already sent. "Append-time cut" = cuts a message once, before it is first sent; deterministic afterwards, so cache-safe but it violates "never trim a tool result".

| # | what | code | when it fires | mutates earlier bytes? | paper frequency | what append-only needs |
|---|---|---|---|---|---|---|
| T1 | **carry drops run-context turns** | `run_memory.go:267` (+ `carryableRunMemoryTail` :262) | every new-run Texture wake (88% of paper wakes) | **YES, message 0 of the transcript**; the cause of the 22% first-call hit | 276 of 315 paper texture runs (M) | delete the `isRunContextMessage` skip; carry verbatim; the new ctx is already appended after the carry. 1 line + test |
| T2 | **carried checkpoint is re-rendered**: the carry writes a new compaction entry with `TokensBefore = estimate(prior context)`; the summary text embeds that number (`compactionSummaryMessage`, :1332) | `run_memory.go:200,224,1328-1332` | only when the source run was compacted | **YES, message 0** (summary), changes at every hop | not observed on the paper (no compaction in the 73 sampled runs, M); latent | carry the source's compaction entry unchanged (same Summary, TokensBefore, FirstKeptEntryID), or copy the rendered context messages. about 10 lines |
| T3 | threshold compaction replaces history with a summary | `run_memory.go:344,467`, `provideriface/config.go:47` (70% of window) | when estimated prompt > 0.7 x window. DeepSeek/Synthetic: about 90k; crouton/paper wedge: 140k; **Haiku 5.5 is cataloged at 1M, so about 700k and it never fires** (`modelcatalog/catalog.go:136`) | YES (the whole prefix) | 0 events in 73 sampled paper runs (M: `grep compaction` over events); 4 in 8 minutes in the wedged management run (budget doc 1.3) | remove the threshold path from `beforeProviderCall`; keep only `onProviderError` context-overflow (:360) as an explicit boundary that starts a new thread. about -30 lines |
| T4 | cell stdout cut at 8000 runes: head 6000 + marker + tail 1500, whole text in the session for `choir.CellOut`, 8 kept | `yaegikernel/cell_output.go:21-91`, enabled at `agentcore/tools_desk.go:627` | any print > 8000 runes | no (append-time cut) | **research 127 of 324 tool results carry the marker (39%), texture 14 of 100, management 6 of 88** (M: sampled 73 paper runs, regex on `tool.result.output`); research's 14 `CellOut` calls in 240 cells (overhead doc) are the follow-up reads | delete the cut, or turn it into model-visible pagination chosen by the cell (the model slices before printing). `KeepCellOutput: false`; the 8-entry store goes |
| T5 | single cell value cut at 48000 bytes (`… [N bytes truncated]`) | `yaegikernel/session_loop.go:255,298` | a bare final expression over 48 kB | no (append-time) | 0 markers seen (M, same sample) | remove or raise past any transport bound; the real bound is the 4 MiB frame |
| T6 | Texture frame digests the document when it is over 12,000 chars | `texturewrite/frame.go:21,94-123` | doc > 12k chars | no (it is a new appended message) | paper documents are 0.2 to 3.9k chars (`texture.md` 1.4), so it rarely digests | not a defect for caching; a digest of a long doc is a choice about the appended message |
| T7 | `stripProviderReplay` removes `reasoning_content` from all history on a model failover | `toolloop.go:1987`, called at :907, :979 | when failing over to another provider | yes, but the provider changed anyway (cold regardless) | 0 switches in the 3 h log (overhead doc) | fine as is |
| T8 | `describeRunMemoryMessage` / deterministic emergency checkpoint: 350 chars per message, 250 per tool result | `run_memory.go` (`summarizeRunMemoryMessages`), now only the fallback when the LLM checkpoint fails | after T3 failure | yes | never | goes away with T3 |
| T9 | frame window (worktree a5a71f) | `yaegikernel/frame_window.go`, `agentcore/update_window.go` | desk-cell frame over budget (1 MiB records, 1 MiB updates, 256 KiB per item) | **no** (see below) | the paper's management desk (1000 pending updates) | n/a |
| T10 | `carryableRunMemoryTail` also removes orphan tool results and cuts at an unanswered tool_use | `run_memory.go:276-300` | interrupted runs only | only the tail of the previous transcript | rare | keep: a provider refuses an unmatched tool_use |

**Frame windowing (T9), answer**: it bounds the desk-cell **frame**, the host-to-worker transport frame that holds `choir.Records()` and `choir.Updates()` (MaxFramePayload 4 MiB), and the **wake turn** that names update ids (`wakeWindowIDs`, `coagentUpdateTurnInjectorWithDelivered`, `prependInitialCoagentUpdatePackets`). It never edits a stored message, never touches a summary, and `seenUpdates` already skips what an earlier turn delivered, so it only changes what the NEXT appended turn contains. It cannot change earlier transcript bytes (I) (diff read: `git diff 5dc98bcf5 HEAD` for `update_window.go`, `frame_window.go`, `management_controller.go`, `texture_turn_runtime.go`). Two notes against the owner's rule: (a) a record or update over 256 KiB is replaced by a stub (`stubRecord`) and a record over the frame is cut, which is trimming of a tool-visible item, but it is append-time and necessary because of the 4 MiB transport frame; (b) Texture's own `textureFrameInjector` (`texture_frame.go:175`) is unchanged and is not windowed.

## 3. Volatility inside the prefix

- System prompt: **stable**. M: one `system_sha256` (5aa70e80...) across all 96 Texture calls on d@e (`ev_*.json` loop.progress). Test: `internal/agentcore/prompt_cache_prefix_test.go`. Inputs that can vary: the stored role prompt per owner, `intake.labels` and event-wake settings for management, engineering's slot and four overlay variants (fixed per assignment). A package install or setting change changes it for that computer, once.
- Tools: one static tool; `ToolChoice` and filtered tool definitions are only applied on required-tool retries (`toolloop.go:1062-1080`) and `WithInitialToolChoice` is unused in production; Bedrock does not send tool_choice at all (`buildRequestBody` ignores it).
- Time and ids: in the run-context turn, which is **message 0 of a fresh run**. Fine for a single run, because it is never rewritten. It is only a problem because T1 removes it.
- Timestamps in the frame (`doc= rev= through=` header) and wake prompt ("Owner prompt reference time") are in appended messages. Fine.
- Map-ordered JSON: `canonicalJSON` sorts keys, deterministic; `buildAssistantContent` uses `json.RawMessage`; `assistantPayload` is a map marshaled once at append and stored. Stable.
- Reasoning: the assistant message stores `reasoning_content`; on the DeepSeek path it is sent back as `reasoning_content` (`provider.go:1893`), on Bedrock it is decoded to `thinking` blocks with signatures (`decodeAnthropicThinkingBlocks`, :2135) and kept. Kept, not dropped.
- **Wake prompt repeats**: `buildAgentRevisionRequest` re-states the owner's seed request, all owner directions and "reference time" on every wake (5.0 k chars per wake on d@e, M: run `prompt` length). Append-only means every earlier copy stays in context, so a document with n wakes carries n copies. It is growth, not a cache break.

## 4. Provider cache wiring

**Bedrock (Anthropic Messages)**: `internal/provider/provider.go:515-532`. `cache_control: {"type":"ephemeral"}` on the single system block and, via `markHistoryCacheBreakpoint` (:1998), on the last non-thinking block of the last message. Two breakpoints out of four.
- Tools sit before system in Anthropic's order, so the system breakpoint covers tools+system (I; Anthropic render order tools, system, messages). Correct.
- Per-call breakpoint on the last message is the right shape for an append-only transcript: next call finds the previous call's entry at most 2 to 4 blocks back. The 20-block lookback is not a problem inside a loop. A wake that appends many separate user messages (one per coagent packet) could pass 20 blocks; a third breakpoint on the last block of the carried prefix (the previous request's last block) removes the risk. 3 lines.
- **TTL is 5 minutes** (no `ttl` field anywhere, grep `ttl` in provider.go: none). Wake gaps (M, `gaps.py` over `/api/costs?detail=1&limit=10000`, last call of run k to first call of run k+1 for the same `texture:<doc>` agent):

| computer | n | p10 | p50 | p75 | p90 | >300 s | >3600 s |
|---|---|---|---|---|---|---|---|
| paper (18:19-20:00 UTC) | 71 | 212 s | 628 s | 1443 s | 1722 s | **82%** | 1% |
| d@e (22:43 yesterday-19:57) | 115 | 13 s | 119 s | 227 s | 1030 s | 22% | 0% |
| b@c (2 weeks) | 64 | 19 s | 1013 s | 2250 s | 3177 s | 62% | 6% |

  Intra-run call gaps are p50 7 s, p99 70 s (paper): the 5-minute TTL is right for everything inside a run (research, task, engineering) and wrong for Texture's wakes. Synthetic confirms the shape: consecutive calls more than 300 s apart missed 6 of 9 times (M, weak n) while 60-300 s gaps missed 9% (n=89).
  Should 1 h be used? **For Texture and persistent desks, yes**: a 1 h write costs 2x the base input price (Anthropic's published multiplier; I, not verified for Haiku 5.5 on Bedrock) and the read is 0.1x. At the paper's gaps 99% of wakes would then read the carried prefix. Section 6 prices it. Per-desk TTL: Texture/management 1 h; research/task/engineering 5 min (their gaps are seconds and the write is cheaper). Needs a `CacheTTL` on `LLMRequest` and the gateway body (about 30 lines); the cache_control map already takes `ttl`. Probe first: one tiny Bedrock call with `ttl:"1h"` on Haiku 5.5, then read `usage.cache_creation`.
- **Minimum cacheable prefix**: Anthropic's table says 4096 tokens for Haiku 4.5; Haiku 5.5's is not in the repo and I could not verify it (H: same as 4.5). Texture system+tools = 4.4k tokens (M: Synthetic caches exactly 4352 = 68 x 64 tokens of it on every first call, so system+tools is 4.35k to 4.4k), just above. management_task = 2.8k + 0.3k = 3.1k is below, so its system breakpoint caches nothing and its first call is uncached; the last-message breakpoint caches the whole request from the second call once the request passes 4096 (I). Not worth fixing.
- **Usage accounting**: `anthropicUsage.usage()` (:2041) adds cache reads and cache creation into `InputTokens`, and `llmcost` prices creation as base input (no write premium). The paper's Haiku dollars will be under-reported by the write premium (I). Cache-creation tokens are not reported separately in `/api/costs`, so a cold-cache TTL miss on Bedrock will look like a high-cost input, not like a miss.
- **No successful Bedrock call exists in the last 8 hours.** M: `ssh node-b journalctl -u go-choir-gateway --since -8h`: 9 `provider=bedrock` requests, all at 19:48:12-16 from one VM, all `status 400 Bad Request (sanitized)`; zero `inference succeeded ... provider=bedrock`. So the Haiku 5.5 cache behavior is **unmeasured**. H on the 400 (not tested): the request histories (5, 16 and 26 messages, i.e. mid-run) came from DeepSeek-style transcripts whose assistant messages carry plain-text `reasoning_content`, which `decodeAnthropicThinkingBlocks` drops, so they reach Haiku 5.5 (thinking on by default) as tool_use assistant turns with no thinking block, which Anthropic rejects when thinking is enabled. Owner point: before any caching question, Bedrock needs one clean successful call and the rejection reason logged (`logRejectedRequest` sanitizes it away).

**OpenAI-compatible providers** (`openai-compat` chat completions):
| provider | cache hint sent | M evidence |
|---|---|---|
| Synthetic | **none** (`SyntheticProvider.buildChatCompletionsRequestBody`, `provider.go:1000-1013`: no key field, no header); automatic prefix cache (64-token blocks) | 84-95% (table below) |
| River | **none** (`river.go:~96-110`: same body type, no key) | 0 River calls in the 8 h journal, unmeasured |
| opencode-go | `x-opencode-session` = `cacheRoutingKey` (`provider.go:1443,1368`) | 91-96% for engineering/management, 15% for texture (n=39; the same T1 break) |
| openrouter | body `session_id` (cache key, `openrouter.go:193-205`) | crouton 0% (free stealth endpoint, no cache pricing; `flint-v1.0:free` 98% on d@e management, M) |
| chatgpt | `prompt_cache_key` + session headers (`provider.go:1211-1244`) | b@c: management 83%, others 50% |
Synthetic's high hit rate without a key means a key is not what is missing. Whether Synthetic or River route better with one is H; a key costs one JSON field and the 8-model free pool already uses `x-opencode-session`. Not a defect.

## 5. Measured truth (gateway and /api/costs)

Commands: `$SCR/{paper,de,bc}.sh '/api/costs?detail=1&limit=10000'` (windows: paper 18:19-20:01 UTC today; d@e 2026-11-08 22:43 to 2026-11-09 19:57; b@c 2026-10-27 to 2026-11-09); `ssh node-b "sudo journalctl -u go-choir-gateway --since '-8h'"` (8 h: 28,847 openrouter calls, 2,881 synthetic, 817 chatgpt, 553 opencode-go, 9 bedrock). Scripts: `ratio.py`, `tex.py`, `show.py`, `gaps.py`, `ideal.py` in `$SCR/cacheaudit`.

### 5.1 Cached / input by desk and provider (M, calls with cache reported, > 0.2M input)

| computer | desk | provider / model | calls | input | cached |
|---|---|---|---|---|---|
| paper | research, management, texture | openrouter crouton-alpha | 831 / 658 / 496 | 18.6M / 13.6M / 7.1M | **0%** all three |
| d@e | engineering | synthetic DeepSeek-V4.1-Flash | 693 | 59.0M | 95.3% |
| d@e | engineering | opencode-go iceman | 136 | 11.1M | 95.4% |
| d@e | management | openrouter flint-v1.0:free | 174 | 8.0M | 98.0% |
| d@e | management | opencode-go beam | 78 | 3.2M | 91.0% |
| d@e | management | synthetic | 36 | 1.2M | 87.9% |
| d@e | **texture** | synthetic | 96 | 4.17M | **84.1%** |
| d@e | **texture** | opencode-go iceman / beam | 39 / 11 | 2.15M / 0.21M | **14.7% / 53.4%** |
| b@c | engineering | synthetic | 1006 | 102.4M | 94.8% |
| b@c | (no desk label: research children) | synthetic | 667 | 39.8M | 90.1% |
| b@c | management | synthetic / chatgpt | 272 / 150 | 5.7M / 1.6M | 89.9% / 83.1% |
| b@c | **texture** | synthetic | 15 | 0.46M | **48.3%** |

The paper itself is on crouton-alpha, so Texture wake caching on the paper cannot be read; the d@e and b@c Texture runs are the evidence.

### 5.2 First call of a run against later calls (d@e + b@c, caching providers, M: `ratio.py`)

| desk | first calls (n / hit / input) | later calls (n / hit / input) |
|---|---|---|
| engineering | 20 / 12.8% / 0.27M | 1855 / 95.1% / 173.7M |
| management | 62 / 41.8% / 0.55M | 683 / 93.5% / 19.6M |
| (research children, no desk) | 102 / 7.4% / 5.05M | 741 / 91.3% / 43.9M |
| **texture** | **64 / 20.4% / 3.15M** | 121 / 88.6% / 4.13M |

Only Texture loses a large share of its input on first calls (43% of its input at a 20% hit); every other desk's first call is 1 to 10% of input.

### 5.3 Texture wakes, carried vs ideal (M: `ideal.py`, 44 carried-run first calls with a prior run of the same agent on the same provider/model)

| | mean | median | total |
|---|---|---|---|
| input | 62.4k | 45.2k | 2.75M |
| cached | 13.7k | **4.35k** (the system prompt) | 0.60M |
| fresh actual | 48.7k | **25.4k** | 2.14M |
| fresh if pure append (input minus the previous call's input) | 8.1k | **8.2k** | 0.36M |

Hit 22.0% actual against 87.0% ideal. Intra-run Texture calls on Synthetic hit 92.4% on d@e (M), so the model, provider and prompt are fine; the break is at the wake boundary.

### 5.4 One Texture document, call by call (d@e, agent `texture:9ed099d2...`, Synthetic DeepSeek; M: `show.py de texture:9ed099d2-7d54-5edb-a9f9-18b02d4b8cb8`, run metadata from `/api/runs`)

| time | run (wake source) | input | cached | what was appended |
|---|---|---|---|---|
| 04:28:49 | R1 `de1f2615` (owner revision, new) | 6,733 | 4,352 | first run: ctx + prompt (5.3k chars) |
| 04:30:22 | R2 `9fa8f4a3` (**inbox_record: research record**, carry) | 12,074 | **4,352 (36%)** | +5,341 vs R1's call: ctx, prompt, frame. Ideal cached 6,733 |
| 04:30:25 ... 04:30:41 | R2 calls 2-7 | 14.6k to 27.0k | 12.0k to 24.7k (81-92%) | tool results, +1.2k to 3.3k per call |
| 04:32:03 | R2 call 8 | 29,231 | **4,352 (15%)** | messages 21 to 23, pure append of a Write call and its result; system_sha256 unchanged. A mid-run miss: 81 s call, 11,790 output tokens (H: provider-side, see 5.6) |
| 04:32:10 | R3 `e3eb8103` (new, carry) | 42,660 | **7,808 (18%)** | +13.4k vs R2's last call; 7,808 = system 4.35k + R1's contribution 3.4k, exactly the part before R2's run-context turn |
| 04:32:19 | R3 call 2 | 44,039 | 42,624 (97%) | |
| 04:32:43 | R3 second activation (same run reactivated, update_coagent) | 49,409 | **4,352 (9%)** | +5,370; replay of its own memory + resume turn + frame. This one is not explained by T1 (see 5.6) |
| 04:32:49 | R4 `6e2a0b8b` (inbox_record, carry) | 57,630 | **36,352 (63%)** | the part before R3's run-context turn |
| 04:36:28 | R5 `5595e9f1` (inbox_record, carry) | 107,137 | 52,992 (49%) | |

The pattern is exactly what T1 predicts: the cached amount at a carried wake is the size of everything before the previous run's run-context turn, and it never covers the previous run's own contribution (its prompt, frame, reads and writes). Each wake re-pays the previous wake in full, then the next wake re-pays this one. Another document (`texture:a6ccea97...`, same run) gives 6,235 (29%), 11,733 (37%, cached 4,352), 39,797 (21%, cached 8,192), 68,159 (48%, cached 32,768).

### 5.5 Same-run reactivation (T1 does not apply) mostly hits
Run `580a5e2c`, three activations: start #2 first call 35,458 / 33,152 cached (93%), start #3 38,878 / 35,456 (91%) (M, `ev_580a5e2c*.json`). The third sample (R3's second activation above) missed. So the replay path is fine in 2 of 3 samples; the miss is H.

### 5.6 Residual misses on a caching provider (not explained by our bytes)
Within a run, 5.2% of Synthetic calls cache under half of the previous call's input (M: 2,695 consecutive pairs d@e + b@c; engineering 4.1%, management 6.0%, texture 6.0%). Those few calls hold about 65 to 70% of all fresh tokens on Synthetic (engineering 5.19M of 7.93M fresh, M). They do not correlate with output size, gap under 5 minutes or appended size (miss rate 4.1% for gaps up to 10 s, 5.7% for 10-60 s, 9.0% for 60-300 s, 14% when more than 8k tokens were appended), except gaps beyond 300 s (6 of 9 missed, a TTL-like cliff). The 04:32:03 case above is a pure two-message append with an unchanged system hash and a 4,352 hit, so it is not our bytes (I). H candidates: Synthetic replica or eviction under load; DeepSeek chat template dropping earlier `reasoning_content` when a user turn is added (would explain the reactivation case, which appends a user turn, not the pure-append one). Probe: replay one pair of consecutive requests through Synthetic twice and diff `cached_in`; and log a per-request prefix hash like `cache_log.go` already does for chatgpt (`prefix_hash`, `prefix_hash_prev`), extended to the chat-completions path. That instrument exists only for the Responses path today (`internal/provider/cache_log.go:51`).

## 6. Ranked defects against the append-only rule, with fix shape and size

1. **T1: the Texture carry deletes run-context turns** (`run_memory.go:267`). Fix: carry every message verbatim; the new run's ctx + prompt + frame are already appended after the carry (`initialize`, :90-100). Size: delete 3 lines + one test (a two-wake transcript whose second request's first N messages are byte-equal to the first). Effect: Texture first call 22% to about 87% on Synthetic; fresh per carried wake from 25k median (48.7k mean) to about 8k. Largest single item.
2. **TTL: 5 minutes on a desk whose wake gap is 10 minutes** (`provider.go:524,2008`). Fix: `ttl:"1h"` for Texture and persistent management, per-desk `CacheTTL` on the request. Size: about 30 lines across `LLMRequest`, gateway handler body (`gateway/handlers.go:80`), `buildRequestBody`; one probe first. Without it item 1 is worth little on Bedrock (82% of wakes find the cache expired), see the table below.
3. **Bedrock does not work at all** (9 of 9 requests 400, no success in 8 h). Fix: get the rejection reason (`logRejectedRequest`, `rejected_request.go`) and fix the history conversion (thinking blocks required when thinking is on, or turn thinking off for resumed foreign transcripts). Size: unknown until the reason is read; probably a conversion fix of 10 to 30 lines. Blocks every other Bedrock caching claim.
4. **T3: threshold compaction rewrites the prefix** (`run_memory.go:344,467`). Zero on the sampled paper runs and irrelevant on Haiku 5.5 (700k trigger), but it is the one remaining place a live thread gets replaced on purpose. Fix: delete; keep only the context-overflow recovery as an explicit boundary. Size: remove about 30 lines of the proactive path; T2 and T8 die with it.
5. **T4 / T5: cell output cut at 8000 runes and 48 kB** (`cell_output.go:21`, `session_loop.go:255`), plus the `choir.CellOut` follow-up calls it forces. Not a cache break (append-time), but fires on 39% of research tool results, 14% of Texture's, 7% of management's (M). Fix: no cut; let the cell choose what to print (progressive disclosure belongs in the cell, not in the transport). Size: set `KeepCellOutput: false` and delete the store (about 150 lines removed), keep the 4 MiB frame bound.
6. **Wake prompt re-states the seed request and all owner directions each wake** (`texture_agent_revision.go:859-870`): with append-only these accumulate (5.0k chars, about 1.25k tokens per wake, copies stack up). Fix: on a carried run send only what is new since the last wake (the earlier text is already in the prefix). Size: about 20 lines. Pure growth control, no cache effect.
7. **Frame renders the whole document each wake** (`frame.go:54-87`): a full copy of up to 12k chars appended every time the revision changes. Append-only keeps all copies. Fix: after the first frame, a delta frame (blocks changed since `mark.revisionID` + new records). Size: about 40 lines. The doc is small on the paper today (0.2-3.9k chars); matters for long documents.
8. **Latent: carried compaction entry is re-rendered** (T2, `run_memory.go:200,224`), changes message 0 at every hop for any compacted source. Fix with item 4, or copy verbatim. 10 lines. No observed fires.
9. **No third breakpoint at the carried-prefix boundary** (20-block lookback), `provider.go:531`. 3 lines; only matters if a wake appends many separate messages.
10. **Observability gap**: Anthropic cache creation is folded into input and priced at base (`provider.go:2041`, `llmcost`); chat-completions has no `prefix_hash` log. Needed to see misses in production (see 5.6). 20 to 40 lines.
11. **Unexplained 5% intra-run misses on Synthetic** (5.6): provider-side or template-dependent; not actionable until the prefix-hash log exists.

### What an interim Texture wake should cost on Haiku 5.5 once correct

Fresh tokens on the first call = the appended wake prompt + frame + ctx. M: d@e median 8.2k incl. the previous output (frame 2.4k to 5k, `texture.md` 1.2; wake prompt 5.0k chars); paper median about 4.7k (paper documents are tiny; call 1 median 12.1k = system+tools 4.4k + history about 3k + appended 4.7k, `texture.md`). Then about 1.2k fresh per later call (3.5 calls per wake), all else read at $0.01/M.

Prices: $0.10/M input, $0.01/M cached read, $0.50/M output (`llmcost/pricing.go:98`); write premiums assumed from Anthropic's published multipliers, 1.25x for 5 min and 2x for 1 h (I; not verified on Bedrock for Haiku 5.5). 120 wakes/h (paper 77 runs/38 min, M), 3.5 calls, 0.8k output, share of wakes with a cold cache: 5-min TTL 82% (M: gaps), 1-h TTL 1%. Model, not measured:

| carried prefix P (incl. 4.4k system) | no cache | today's bug, 5 min | append-only, 5 min | append-only, **1 h** |
|---|---|---|---|---|
| 7.4k (paper median doc) | $0.0060 / wake, $17.2/day | $0.0027, $7.7/day | $0.0026, $7.5/day | $0.0026, $7.3/day |
| 20k | $0.0110, $31.7/day | $0.0046, $13.3/day | $0.0043, $12.4/day | $0.0031, **$8.9/day** |
| 45k (an old document) | $0.0210, $60.5/day | $0.0085, $24.5/day | $0.0077, $22.1/day | $0.0041, **$11.9/day** |

Reading: on a small prefix everything is cheap and the fix is worth cents; the cost grows with the document history (never trimmed), so the fix that matters is append-only + 1 h together: at 45k it halves the Texture bill, and at larger prefixes the gap widens. Append-only alone on a 5-minute TTL changes almost nothing (82% of wakes find the cache expired anyway). Fresh tokens per interim wake after both fixes: about 5k to 8k (the appended wake) plus about 3k of in-run increments; cache reads about 7k to 50k. That is the number to hold the implementation to: first-call `cache_read / (cache_read + fresh)` of at least 85% on a prefix over 10k.

## 7. Open questions with the probe that settles each

1. Is the 1 h TTL available on Bedrock for Haiku 5.5, and what is the write multiplier? One call each way (`ttl:"1h"`), read `usage.cache_creation_input_tokens` and the Bedrock invoice line.
2. What is Haiku 5.5's minimum cacheable prefix? Probe with a 3.5k, 4.5k prompt pair.
3. Why does Bedrock return 400 (item 3)? Log the sanitized `message` field of the response, not just the status.
4. Do thinking blocks survive a non-tool user turn in Haiku 5.5's cache (the resume turn / carried wake appends a user message after thinking assistant turns)? Two identical requests with a user message appended, compare cache reads.
5. Does Synthetic's DeepSeek template drop earlier `reasoning_content` when a user message is appended (5.6)? Same pair test on Synthetic: with and without the earlier assistant `reasoning_content`.
6. River: any caching at all, and does it want a session key? 2 identical requests; none exist in the 8 h journal.
