# Prompt Cache Analysis, 2026-10-14

Owner question (2026-10-14): "Re prompt caching, it should be automatic…
no? As long as we are keeping the message list append-only."

## Answer

Partly. The caching is automatic on the providers we use: OpenAI caches
prompts of 1024 tokens or more, and DeepSeek caches every prompt on disk. Neither needs
any request flag. A hit needs the request to start with the same bytes as
an earlier one: the system prompt, then the tools, then the messages, in
order.

- **Within one activation of a desk, yes.** The tool loop is append-only.
  The system prompt and tool list are built once per activation and never
  change between iterations, and earlier messages are never edited. Each
  call's prompt therefore starts with the previous call's whole prompt.
- **Across activations (turns) of the same thread, no.** Every desk's
  system prompt has the current time, to the second, at about token 275:
  "Current UTC date/time: 2026-10-14T07:58:00Z". It is rebuilt on every
  activation, including resumes of the same run. Everything after it
  misses: the remaining 2,000 to 4,600 tokens of system prompt, the tools,
  and the whole replayed history. OpenAI's minimum cacheable prefix is
  1024 tokens, so a gpt-6-luna activation reuses nothing from the
  previous one.
- **We were not measuring it.** No parser read a cached-token field, so
  every cost figure, including
  `docs/evidence/model-prices-2026-10-14.md`, assumed 0% hits. The current
  hit rate is unknown from recorded data. The commit after this report
  records it; read it after deploy (see Measurement).

## Measurement (landed with this report)

Each provider reports cached input in its own field. The provider layer now
reads all four shapes into `Usage.CachedInputTokens`, always a subset of
`InputTokens`, and `Usage.CacheReported`, so that a reported miss (0) can
be told apart from a provider that reports nothing.

| Wire shape | Field | Routes |
|---|---|---|
| Chat Completions (OpenAI style) | `usage.prompt_tokens_details.cached_tokens` | opencode-go (deepseek-v4.1-flash, cat-jam-free, longcat, glm, hy3, step), fireworks |
| Chat Completions (DeepSeek native) | `usage.prompt_cache_hit_tokens` / `prompt_cache_miss_tokens` | opencode-go deepseek, if OpenCode passes DeepSeek's usage through |
| Responses API | `usage.input_tokens_details.cached_tokens` | chatgpt (gpt-6-luna), opencode-go muse-spark |
| Anthropic Messages | `usage.cache_read_input_tokens` (+ `cache_creation_input_tokens`) | opencode-go qwen3.7-max, zai, bedrock |

Which of these OpenCode Go actually returns is **not verified**. The repo
has no recorded OpenCode responses: every test fixture carries only
`prompt_tokens`/`completion_tokens`. The parser accepts both chat shapes,
and `cache_reported` on each event will show which routes report. The
Responses API always sends `input_tokens_details`, so chatgpt should
report. Jev (OpenRouter Decisions) is unchanged: its prompts are short
one-shots and it has no cached rate.

The count is carried along this path:

provider `Usage`
→ the gateway's native response (JSON passthrough), and
`prompt_tokens_details.cached_tokens` on its OpenAI-compatible endpoint
→ the `gatewayruntime` client
→ `provideriface.TokenUsage`
→ each tool-loop `loop.progress` event (`cached_input_tokens`,
`cache_reported`)
→ run metadata and the completed/passivated payloads
→ `llmcost`.

`llmcost` handles it this way:
- New `ModelPricing.CachedInputPerMillion`: deepseek-v4.1-flash 0.006,
  deepseek-v4-pro 0.044, gpt-6-luna 0.01, kimi-k2p6 0.16. A model with no
  cached rate prices cached tokens at the input rate, never at $0.
- Cached tokens price at the cached rate and the rest of the prompt at the
  input rate.
- `GET /api/costs` now shows, in the summary and in every breakdown
  (`by_model`, `by_provider`, `by_run`, `by_trajectory`, `by_day`):
  `total_cached_input_tokens`, `cache_reported_call_count`,
  `cache_reported_input_tokens`, `cache_hit_rate` and `cache_savings_usd`.
- `cache_hit_rate` is token-weighted and counts only calls that reported.
  Events from before this change, and routes that never report, do not
  pull it down.

After deploy, read it with
`GET /api/costs?from=<deploy time>` and look at `summary.by_model`.

## Cache-breaking patterns

These come from reading the code, at origin/main f4650420b. The system
prompt sizes come from a throwaway probe of `systemPromptForRun` with the
default prompt store, and do not include the tool catalog appended later:
Texture 9.4K chars (about 2.3K tokens), Engineering 13.9K (3.5K),
Management 19.1K (4.8K), Research 19.5K (4.9K). On every desk the
timestamp sits at char 1103.

| # | Pattern | Where | Desks | Effect |
|---|---|---|---|---|
| B1 | **Timestamp in the system prompt**, built once per activation with `time.Now().UTC().Format(time.RFC3339)` | `internal/agentcore/tool_profiles.go:200-205`; template `internal/runtimeprompts/overlays/temporal_context.yaml`; built per activation at `runtime.go:3400` | all four, plus research children | Breaks at about token 275 on **every activation**. Stable within one. |
| B2 | **New run seeds a summary as message 0.** A run with no memory gets a summary of the agent's previous run, not its replayed history | `internal/agentcore/run_memory.go:67-83` (`seedActorMemorySnapshot`) | any desk that mints a new RunID (Texture user turns, engineering assignments, fresh management/research activations) | Breaks from message 0 on every new run. Expected: only the system prompt and tools could be reused. |
| B3 | **Research child time budget in the system prompt**: `researchChildTime(share)` renders `time.Until(deadline)` to the second | `internal/agentcore/research_children.go:217, 245-247` | research children | Sibling children share only the roughly 100-token preamble. Each child's own loop is stable. |
| B4 | **Per-run text in the system prompt**: engineering `assignment_id`/`subject_digest`/`candidate_id`; Texture self-dev operation id; management intake-labels overlay; research `RequesterAgentID` | `tool_profiles.go:229-231, 235-239, 257-268, 283-299` | engineering, Texture, management, research | Hidden behind B1 today. These become the next breakers once B1 moves. |
| B5 | **Pool assignment hashes the trajectory, not the desk**: `ThreadKey` uses the `texture:<doc>` agent id for Texture, otherwise `trajectory_id`, otherwise `random:<nanotime>` | `internal/modelpolicy/model_policy.go:177-188`; pool `internal/modelcatalog/catalog.go:211-218` | management, research, engineering | The same desk lands on deepseek, cat-jam or luna per trajectory. Caches are per model, so a desk's cache goes cold whenever its trajectory hashes to a different model. Texture is sticky per document. |
| B6 | **Session affinity is per run**: `x-opencode-session` = `ConversationID` = `rec.RunID` | `runtime.go:3495`; header at `internal/provider/provider.go:1090`; children use `"research-child:"+uuid` (`research_children.go:223`) | all on opencode-go | Any stickiness OpenCode does by session is lost on every new run of a persistent desk. |
| B7 | **No `prompt_cache_key` on the Responses API**: the request carries model, instructions, input, tools, tool_choice, `store:false`, stream and reasoning only | `provider.go:920-938` (chatgpt), `provider.go:1149-1162` (opencode responses) | runs on gpt-6-luna | OpenAI routes cache lookups by a prefix hash plus this key. Without it, hits depend on landing on the same machine. Codex CLI sends the conversation id here. |
| B8 | **Model failover** changes provider/model and conversation id (`<run>#failover`) mid-activation | `internal/toolregistry/toolloop.go:478-503` | all | Full cache loss for the rest of the activation. Rare: only after retries are exhausted on 429/5xx/quota. |
| B9 | **Compaction** at 70% of the context window rewrites history as `[summary] + tail` | `run_memory.go:172-177`, ratio at `internal/provideriface/config.go:39` | all | Breaks from message 0. The system prompt and tools survive. Rare at today's 10K to 48K-token median prompts. |
| B10 | **DeepSeek may drop earlier turns' `reasoning_content` once a newer user message exists** (hypothesis, provider-side, unverified). We replay reasoning on every assistant message (`gatewayruntime/provider.go:389, 408, 430`; `provider.go:1406-1413`) and inject frames and packets as **user** turns | `texture_frame.go:162-187`; packet injectors | all on DeepSeek | If true, every injected user turn re-renders earlier assistant turns and breaks from the first one carrying reasoning. Test it with per-iteration hit rates after deploy. |
| B11 | **Gaps longer than the cache TTL.** OpenAI keeps a cache about 5 to 10 minutes idle (up to an hour off-peak); DeepSeek's disk cache lasts hours. Gaps: Texture passivates after each committed write; Texture park idle is 2 min (`provideriface/config.go:30`); management waits on minutes-long engineering casts; Autopaper polls every 15 min (`source_schedule.go:36`); a management cell runs up to 300 s (`tools_desk.go:51`); `choir.Research` up to 10 min (`research_children.go:46`) | as listed | all on gpt-6-luna | Even after B1 is fixed, luna turn-to-turn hits are rare, and a long cell can expire the cache mid-activation. DeepSeek covers these gaps. |

Side finding, not about caching: after a failover, `stripProviderReplay`
(`toolloop.go:488`) has no effect on desk runs. The next iteration's
`BeforeProviderCall` (`toolloop.go:337-343` → `run_memory.go:172-177`)
rebuilds the messages from the store, which still holds the old
provider's `reasoning_content`. Worth its own problem note.

Confirmed cache-friendly:
- The tool loop appends through `appendMessage` only, and the run-memory
  store never updates or deletes, so it replays the exact stored bytes.
- The system prompt and tools are fixed for an activation
  (`toolloop.go:238-239`).
- Tools are sorted (`toolregistry.go:69-73`) and desks expose one tool with
  a static description.
- Lever 6 (`choir.CellOut`, `yaegikernel/cell_output.go:18-89`) cuts only
  a new cell's output before it is written; it never edits earlier
  messages.
- The Texture frame and coagent packets are appended at the end.
- Resumed runs replay their full history.

One cost note that is not about caching: each Texture frame appends the
whole document again when the revision changes. That is cache-safe, but
the prompt grows by one document per revision within an activation.

## Expected hit rate and savings

Inputs:
- `docs/archive/evidence/model-latency-paper-2026-10-14.md`: 760 calls over 100
  runs (about 7.6 calls per run); median prompt 10.8K (luna), 11.9K
  (cat-jam) and 15.2K (deepseek) tokens.
- `docs/evidence/model-prices-2026-10-14.md`, last window: deepseek
  1.38M in / 0.20M out, luna 2.01M in / 0.02M out.

**The ceiling today, within an activation only.** With k calls per
activation and each prompt about 10% longer than the last, the cached share
is about 84% of input tokens for k = 8 and about 70% for k = 4. The misses
are each activation's first call. This holds only if the provider actually
caches on that route and the calls fall within the TTL.

| Model, last window | Uncached (today's figure) | At 80% hit | Saved |
|---|---|---|---|
| deepseek-v4.1-flash | $0.654 (input $0.414) | $0.089 input + $0.24 output = $0.33 | **$0.33 (50%)** |
| gpt-6-luna (list) | $0.211 | $0.056 input + $0.01 output = $0.066 | $0.145 (69%), but $0 marginal on the subscription |

On deepseek the saving is mostly allowance: the Go plan meters at these
rates, so an 80% hit rate stretches the input part of the $80 monthly
allowance (and its 5-hour cap, 20% of it) about 4.5 times. Fewer cap hits
also mean fewer quota failovers (B8).

## Recommended fixes, ranked by expected savings

1. **Deploy the measurement and read it before restructuring anything**
   (this branch). Everything below is a hypothesis until `cache_hit_rate`
   by model is known. In particular: does OpenCode report and pass
   caching through for deepseek and cat-jam, and is the within-activation
   rate near the 70 to 84% ceiling? If deepseek reads near 0% within
   activations, the cause is the route (OpenCode affinity, B6) or B10,
   not our prefix, and fix 3 moves up.
2. **Make the system prompt byte-static per desk** (fixes B1, B3, B4).
   - Move the timestamp, the per-run text and the research child's time
     budget out of the system prompt and into the run's first user
     message, or into the appended frame or packet.
   - Do not just move them to the end of the system prompt: the messages
     come after the system prompt, so anything dynamic there still breaks
     the whole history.
   - Expected effect: every activation's first call reuses the static
     system prompt and tools (2.3K to 4.9K tokens plus the catalog).
     Resumed runs reuse their whole replayed history while the cache is
     warm, which on DeepSeek is hours.
   - Estimate: on deepseek, the first-call miss share (about 16%) falls to
     about 5 to 10%, worth roughly another 6 to 10 points of hit rate. On
     a resumed management or Texture activation, the first call goes from
     0% to near 100% on DeepSeek.
   - The model loses no information: the time is still in the context.
3. **Cache-affinity keys per desk, not per run** (fixes B6, B7).
   - Send `prompt_cache_key` = the desk's thread key (agent id or Texture
     doc agent) on the Responses requests.
   - Set `x-opencode-session` to the same key instead of the RunID.
   - The effect depends on the providers' routing and cannot be estimated
     before measurement. OpenAI documents a higher hit rate with the key
     for prompts that share prefixes.
4. **Sticky model per persistent desk** (fixes B5). Hash the persistent
   management and research agent id the way Texture's doc agent is
   hashed, so a desk keeps one model and therefore one cache. The cost is
   coarser load spreading across the pool.
5. **Test B10 once data exists.** Compare hit rates on iterations that
   follow an injected user turn with those that follow a tool result. If
   the turns after injections miss, deliver frames and packets in a form
   DeepSeek does not re-render.
6. **Accept B8 and B9** (failover and compaction). They are rare, and both
   trade cache for correctness.

Mutation class of the measurement commit: orange (cost reporting and the
usage wire fields). All fields are additive and `omitempty` on the wire,
and payloads without them decode as before. Rollback: revert the commit;
old and new events price the same way when no cached count is present.
