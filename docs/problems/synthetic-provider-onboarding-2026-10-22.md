# Synthetic provider onboarding: DeepSeek V4.1 Flash 1017 (2026-10-22)

Status: probed; code lands in the commits after this record. Mutation class of
this document: green. Owner words that start the lane: "each new provider and
model needs to be tested. Sure synthetic has their own inference stack, so
it's not 1:1 identical. We need to review their docs to find out what to do.
And do some tests. Like, I see they have 512k max tokens? Is that true? Other
DeepSeek is 1M." And: "the setup of a provider and model needs agentic care,
not human manual config of some config strings." This record is the first
instance of that: the probe receipts are the deliverable, the code follows from
them.

Scope change, owner 2026-10-22: "for now, I actually want to switch the paper.
So the paper is half synthetic DeepSeek 1017, half ChatGPT. I want to run it
like that overnight and just see what the results are. Basically to see how
much of my synthetic usage I end up using." This lane adds a named pool
`synthetic-luna` (1:1) next to the untouched default pool. Pointing the paper
at it is a separate step by the orchestrator after deploy.

Second scope change, owner 2026-10-23 (UTC): "Just ChatGPT luna and synthetic
DeepSeek 1017 tonight. I'd rather them fail than fallback. We are still
pre-launch" and "Don't use Opencode as fallback tonight." So `synthetic-luna`
has no failover (a thread on Synthetic that fails, fails; the same for luna),
and the default pool is gpt-6-luna alone tonight. Everything in this record
about failing over to luna describes the existing mechanism and why the
provider must not hide a 429; tonight it is switched off for this pool, so a
Synthetic 429 or 404 ends the call for that thread.

Raw probe requests and responses (no key material; the key never leaves the
wrapper) are outside the repo in `/Users/wiz/Dev/lanes/_runs/synthetic-probes/`
(`*.req.json`, `*.out.json`, `*.out.json.hdr` response headers). Wrapper:
`/Users/wiz/Dev/lanes/_runs/synprobe.sh PATH REQ OUT`. Total probe spend is
about 2.2M prompt tokens (almost all the five long-context calls) and about
0.15M output tokens.

## Sources read

- https://dev.synthetic.new/docs: API overview, getting started, models,
  `/chat/completions`, `/models`, `/messages`, `/quotas`.
- `GET /openai/v1/models`, entry `hf:deepseek-ai/DeepSeek-V4.1-Flash-1017`:
  `context_length` 524288, `max_output_length` 65536, `quantization` fp8,
  `supported_features` tools/json_mode/structured_outputs/reasoning,
  `reasoning_parameters.efforts` none/low/high/xhigh/max, text+image input,
  price $0.60/M in, $1.20/M out, $0.03/M cache read, US datacenter. The docs
  mark the model "Beta".
- Docs versus the wire: the docs table says "512k tokens" (true: 524288, see
  Q1). The chat-completions page says `reasoning_effort` is `low`, `medium` or
  `high`; the server's validator accepts a longer set (Q2). The docs warn that
  pinned `hf:` names 404 when a model is rotated out and recommend `syn:`
  aliases.

## Ways the provider could fail (written before any code)

1. A reasoning parameter that is silently ignored, so management and Texture
   "high" effort never reasons on Synthetic.
2. Reasoning text returned under a different field than our parsers read, so
   it is dropped (or, on a tool turn, not echoed back and the next turn loses
   its thinking or is rejected).
3. Tool calls: malformed arguments, parallel calls collapsed to one, a
   finish_reason our loop does not read as tool use, tool_choice forms
   rejected, a large schema refused.
4. Streaming: a different SSE shape, usage missing, tool-call deltas
   interleaved so that arguments mix.
5. Prompt caching absent or not reported, so every turn of a long desk thread
   is paid in full and our cache accounting reads zero.
6. The advertised 512k window served only partly (a gateway limit below the
   model), a different error shape past it, or max output not honoured.
7. Concurrency or rate limits that return something our failover does not read
   as a failover trigger, so a Synthetic limit stalls the paper instead of
   moving the thread to luna.
8. The pinned model id rotating out (404), which would fail every call at once.
9. Image input not served although the catalog defaults every model to vision.
10. Cost accounting priced at another provider's rate for the same model name.

## Findings

### Q1. Is the 512k context real? Yes: 524288 tokens; 65536 output is accepted but unproven

Needle in a haystack: a unique code planted at 40% depth of filler text, asked
back with `reasoning_effort: none`.

| request | prompt_tokens | result | ttfb |
|---|---|---|---|
| ctx_300k | 301,096 | needle found | 6.9 s |
| ctx_500k | 499,893 | needle found | 10.4 s |
| ctx_522k | 522,880 | needle found | 13.5 s |
| ctx_over (about 526k) | not reported | HTTP 400 `{"error": "Context limit exceeded"}` | 4.6 s |
| ctx_edge_gen2 | 522,882 + 1,406 out | `total_tokens` exactly 524288, `finish_reason: length` | 15.4 s |

The window is 524,288 tokens (512 Ki), prompt plus completion, and generation
stops at the window with `length`. A request whose `max_tokens` would overrun
the window with the prompt is not rejected up front (prompt 522,880 with
`max_tokens` 20000 returned 200). It is half of the 1M the owner remembers for
the DeepSeek family. The catalog must say 524288 for this model, not 1,000,000
and not the 200,000 default: Choir's compaction pressure reads
`ContextWindowTokensForModel`, and an entry that overstated the window would
send a request past 524288 and get the 400 above.

Output: the validator accepts `max_tokens` 65536, 65537 and 70000 (and
`max_completion_tokens` 70000) without error; there is no clamp error. One
generation (count to 40000, `max_tokens` 70000, streaming) ended itself at
31,589 completion tokens after 287 s (about 110 tokens/s), so the 65,536 cap
was neither reached nor shown to be enforced. Treat 65,536 as the advertised
figure, unverified at the cap. Agent loops omit the output budget anyway (see
Decisions).

The over-limit error is `{"error": "Context limit exceeded"}`, a bare string
under `error` rather than an OpenAI error object, with no token count. Our
provider layer logs status only and sanitizes the body, so a context overrun
surfaces as `status 400 Bad Request (sanitized)` and is not a failover trigger
(correct: a smaller request is the fix, not another model).

### Q2. Reasoning parameter shape

The model reasons by default when no parameter is sent. One reasoning prompt
(bat and ball plus sheep), `usage.completion_tokens_details.reasoning_tokens`:

| request parameter | reasoning_tokens | reasoning text |
|---|---|---|
| none | 168 | yes |
| `reasoning_effort: "high"` | 221 / 170 | yes |
| `reasoning_effort: "max"` | 364 | yes |
| `reasoning_effort: "xhigh"` | 172 | yes |
| `reasoning_effort: "medium"` | 133 | yes |
| `reasoning_effort: "low"` | 0 | no |
| `reasoning_effort: "minimal"` | 0 | no |
| `reasoning_effort: "none"` | 0 | no |
| `reasoning: {"effort": "high"}` | 401 | yes |
| `thinking: {"type": "enabled"}` | 166 | yes |
| `chat_template_kwargs: {"thinking": true}` | 278 | yes |
| `reasoning_effort: "off"` or `"turbo"` | HTTP 400 validation error | n/a |

Reading: `reasoning_effort` (top level, OpenAI shape) is the supported control.
It is a mode switch plus a budget: `none`, `minimal` and `low` select the
non-thinking template (the prompt is 26 tokens shorter, 55 versus 81 on the
probe); `medium`, `high`, `xhigh` and `max` select the thinking template, and
the budget tends to grow with the level (single samples, noisy). The other
shapes (`reasoning: {effort}`, `thinking`, `chat_template_kwargs`) were
accepted but equal the default, so they are not controls. An unknown string is
a hard 400 (`reasoning_effort did not match any option in the schema`), so a
value outside the set must not be sent: Choir code treats `off` as a disable
value in places, and sending it here would 400. The earlier one-probe finding
"reasoning_tokens 0 with high" was a tool turn: with tools attached the model
often calls tools without thinking even at high (`t_par_high`: 0 reasoning
tokens; the same prompt with `parallel_tool_calls: false` produced 7), so
tool-turn reasoning is the model's choice, not a lost parameter.

What we send: `reasoning_effort` passes through when it is one of
none/minimal/low/medium/high/xhigh/max; `off`, `disabled`, `false` map to
`none`; anything else is omitted (model default, thinking on) rather than risk
a 400 that fails the thread over. Management and Texture "high" therefore
reasons on Synthetic. The opencode-go route sends no reasoning parameter
("opencode-go ignores the setting", modelpolicy comment), so this differs
between the two DeepSeek routes: there the setting is inert, here it changes
the cost and latency of a turn.

Cache interaction: the thinking and non-thinking templates differ at the start
of the prompt, so they do not share a cache entry (the same 27,548-token
prefix sent with `none`, then `high`, missed once on `high`, then hit on
repeat; both entries coexist). Switching a thread between low/none and
medium/high costs one cold prefill. Per-role effort is stable in the model
policy, so this does not occur in normal use.

### Q3. Where the reasoning text comes back (a parser bug waiting for us)

- Non-streaming: `choices[0].message.reasoning_content`, the field our parser
  and the DeepSeek convention use.
- Streaming: `choices[0].delta.reasoning`, not `reasoning_content`. A delta can
  carry both `content` and `reasoning` in one chunk
  (`{"content":"27 sheep","reasoning":"."}` seen).
- `parseOpenAIChatCompletionsStream` reads only `delta.reasoning_content`, so
  on Synthetic every streamed reasoning token would be dropped,
  `ReasoningContent` would be empty, and the next tool turn would be replayed
  without its thinking. Repair: read `delta.reasoning` as an alias in the
  shared stream parser.

### Q4. Tool calls and multi-turn reasoning echo

- Single and parallel calls work. "Weather in Paris, Tokyo and Lima" returned
  three `tool_calls` in one message with OpenAI-shape ids (`chatcmpl-tool-...`)
  and valid JSON arguments. With `parallel_tool_calls: false` it returned one.
- A full loop (call, three tool results fed back, final answer) works.
- Echoing `reasoning_content` on the assistant tool-call message is accepted
  and used (the prompt token count rises by the reasoning length: 655 versus
  648). Omitting it is also accepted, with no error and no degradation seen.
  Echoing under the key `reasoning` is accepted but ignored (648). Some
  DeepSeek-serving APIs reject a tool turn that drops `reasoning_content`;
  Synthetic does not, and honours it when sent as `reasoning_content`. Our
  message converter already sends `reasoning_content` on assistant turns,
  which is the right shape. On failover the tool loop strips provider replay,
  also safe here.
- `tool_choice`: `"required"` forced a call; a named function object forced
  that function but the response has `finish_reason: "stop"` with `tool_calls`
  present (`tc_named`). A loop reading only `finish_reason == tool_calls` would
  end the turn without running the tool, so the code treats "stop with tool
  calls" as tool use. `"none"` suppressed calls.
- Large schema: 34 tools (about 9,200 prompt tokens, nested objects, enums,
  oneOf) accepted and called correctly (`module_op_17` with the right nested
  arguments).
- Streaming tool calls use the standard OpenAI delta shape (the first delta
  carries `id`, `type`, `index`, name and the start of the arguments; later
  deltas carry `index` and argument fragments; three parallel calls had indices
  0, 1, 2 with no interleaving).
- Vision: a 64x64 red PNG as a data URL answered "Red" (`multimodal_tokens`
  reported). The catalog's vision default is right.

### Q5. Streaming and usage

- Every SSE chunk carries a cumulative `usage` object (even without
  `stream_options.include_usage`), and a last chunk with `choices: []` carries
  the full usage including `prompt_tokens_details.cached_tokens`, followed by
  `data: [DONE]`. Chunks also carry vllm extras (`token_ids`, `prompt_token_ids`,
  `system_fingerprint: vllm-0.32.0-tp2-...`). Our stream parser takes usage from
  the finish chunk (no cache detail), then replaces it with the final
  `choices: []` chunk (with cache detail), so cache accounting ends right.
- Stream time to first byte is 0.4 to 0.7 s on small prompts.

### Q6. Prompt caching: works, automatic, 512-token blocks, replica-local

- Reported as `usage.prompt_tokens_details.cached_tokens` (plus
  `created_cache_tokens`, which our parser ignores). No request parameter is
  needed. Block size is 512 tokens: a 27,548-token prefix cached 27,520.
- A repeated 27.5k prefix: first call cold (`cached_tokens` 0, 1.33 s), repeats
  hit 27,520 (99.9%).
- A 9.4k-token system+34-tool prefix over 8 sequential calls hit 9,344 on 6 or
  7 of 8, with sporadic misses (a different replica). Three variants (plain,
  `user`, `prompt_cache_key`) behaved alike, so no affinity key was found;
  expect an 80 to 90% hit rate on a stable prefix, not 100%. A cached token is
  billed at $0.03/M against $0.60/M, so a miss costs 20x on that call's prefix.
- An entry survived at least several minutes. TTL was not measured further.

### Q7. Errors, rate limits and failover shape

- Concurrency limit: a burst of 24 simultaneous streaming requests returned 10
  x HTTP 200 and 14 x HTTP 429 `{"error":"Too many concurrent requests"}`. The
  account allows about 10 concurrent requests. The 429 returns in 0.4 to 0.7 s
  with no `Retry-After` and no rate-limit headers.
- The gateway's model scheduler admits 24 in flight globally and 6 per computer
  (`defaultModelInflight`, `defaultModelPerComputer`), so the paper alone is
  below 10 but all computers together can exceed it. A Synthetic 429 reaches the
  gateway as `synthetic: status 429 Too Many Requests (sanitized)`; the circuit
  breaker's `: status (\d{3})` pattern counts it against the model breaker (the
  short per-model backoff), and the tool loop's `IsModelFailoverError` treats
  429 as a failover trigger, so a thread in a pool that fails over moves to
  the next member (luna). In `synthetic-luna` (no failover tonight) the thread
  has no next member: the run errors, and the model breaker's short backoff
  makes the following Synthetic calls from every computer fail fast for its
  open window. A 429 must therefore NOT be absorbed by a retry or key parking in the
  provider: the OpenCode key pool parks a key after any 429, which for a
  transient concurrency limit would park the only Synthetic key. The Synthetic
  provider has one key and no parking.
- Unknown or rotated model: HTTP 404 `{"error": "<id> is no longer supported.
  Try using a different model, like hf:moonshotai/Kimi-K3"}`. Our error text
  carries no body, so a rotation reads `status 404 Not Found (sanitized)`;
  `IsModelFailoverError` does not match it, so a 404 does NOT fail over by
  itself (R-a).
- Malformed request: HTTP 400 `{"error": "Request failed validation: ..."}`
  (string, not an object).
- Every successful response carries an `x-synthetic-quotas` header, the same
  JSON as `GET /v2/quotas`. The provider logs it.

### Q8. Latency

Ten sequential streaming calls at agent size (about 9.5k prompt tokens: 34
tools plus a long system, `reasoning_effort: high`, 40 to 290 completion
tokens): ttfb 0.63 to 1.10 s (median 0.73 s), total 0.96 to 2.30 s (median
1.2 s). Non-streaming tiny prompts: 1.2 s typical, 2.2 to 4.2 s when it
reasons. A cold 500k prefill costs about 10 s. Decode is about 110 to 130
tokens/s. The statistics are one night's sample from one client, no comparison
to the opencode-go route was run in this lane.

### Q9. Usage and quota: how to read it

- Docs: `GET https://api.synthetic.new/v2/quotas` ("requests do not count
  against your subscription limits"). It is not under `/openai/v1`, so the
  probe wrapper's path allowlist was extended to this one GET path and the
  wrapper now also saves response headers next to the output (`OUT.hdr`).
- Baseline reading, 2026-10-23T02:59:17Z, after about 2.2M probe tokens and
  before the burst and the cache probes:

  ```json
  {"subscription":{"limit":500,"requests":0,"renewsAt":"2026-10-23T07:59:17.038Z"},
   "search":{"hourly":{"limit":250,"requests":0,"renewsAt":"2026-10-23T03:59:17.039Z"}},
   "planWeeklyLimit":{"nextRegenAt":"2026-10-23T05:56:13.000Z","percentRemaining":98.5612528,"nextRegenPercent":2}}
  ```

  After the latency run and the 24-call burst the header on `burst_0` read
  `percentRemaining` 98.5284. The figure the overnight run consumes is
  `planWeeklyLimit.percentRemaining`. `subscription.requests` read 0
  throughout although well over 100 requests had been made, so the 500-request
  counter is not what is metered for this model; usage-based billing appears to
  meter tokens against the weekly percentage.
- Morning reading: `/Users/wiz/Dev/lanes/_runs/synprobe.sh /v2/quotas - OUT.json`
  (wrapper), or the gateway log lines `provider: synthetic quota ...` (one per
  call: percentRemaining, renewsAt), or the `x-synthetic-quotas` header of any
  call. Usage over the night is the drop in `percentRemaining` from the
  baseline (watch `nextRegenAt`: the plan regenerates a few percent at a time,
  so read the minimum, not only the last value).

## Decisions that follow

1. A dedicated `SyntheticProvider` (chat completions, single key, no parking)
   rather than another OpenCode wire-shape entry: the OpenCode provider is
   built around session headers (`x-opencode-session`) and a key pool that
   parks on 429, both wrong here. It reuses the existing OpenAI-compatible
   request converter and parsers.
2. Model id pinned to `hf:deepseek-ai/DeepSeek-V4.1-Flash-1017`, not the
   `syn:large:text` alias: the alias moves to "the latest recommended model"
   without notice, which would change the model under a running overnight
   comparison. The pin rotates out with a loud 404 instead (R-a).
3. Catalog: context 524288, max output 65536, provider `synthetic`,
   `RecommendedFor` empty (no role recommendation before the overnight
   evidence). A new pool `synthetic-luna` (1:1, `NoFailover`). Per the second
   scope change `Pools()["default"]` becomes gpt-6-luna alone (the opencode-go
   deepseek entry stays in the catalog) so nothing falls back to OpenCode.
   `Assign` honours `modelcatalog.NoFailover(pool)` by returning no
   fallbacks; the guest builds fallbacks only there, so that is the one
   switch.
4. `MaxInteractiveOutputTokensForSelection` returns 0 for `synthetic` like the
   other OpenAI-compatible chat-completions routes (omit the output budget):
   probes ran without `max_tokens` and the window check is on the total.
5. Pricing: an exact entry at Synthetic's list rate. Without it the prefix
   matcher prices the model at DeepSeek's direct rate ($0.30/$1.20, cache
   $0.006), understating input by half and the cache read by 5x.

## Staging: concurrency 429s and the circuit breaker (2026-10-23)

Status: discovered on staging; fix in the commits after this section. Mutation
class of the fix: red (gateway provider calls and the circuit breaker). This
section is the problem record and is committed before any fix.

Evidence (FP0, gateway journal on Node B, 03:55 to 04:11 UTC). The paper
(`vm-1669020a...`) runs `pool:synthetic-luna` (Synthetic
`hf:deepseek-ai/DeepSeek-V4.1-Flash-1017` and chatgpt `gpt-6-luna`, 1:1, no
failover by owner ruling: "I'd rather them fail than fallback"). In the 15
minutes to 04:11 UTC the gateway made 93 Synthetic calls and 20 of them failed
with `synthetic: status 429 Too Many Requests`. Synthetic's reason for those is
"Too many concurrent requests"; the probe measured about 10 concurrent and then
429, with no `Retry-After` (finding in the probe section above). That is
Residual R-b arriving as predicted: the gateway admits 24 calls in flight (6
per computer) against an account ceiling near 10.

Second effect, worse than the 20 rejections: the 429s open the gateway's
per-model circuit breaker. After the threshold the journal shows `provider
synthetic model ...: circuit open (model unhealthy)`, and the following
Synthetic calls fail without being sent at all, for the breaker's open window,
although the model is healthy. `providerFailureCounts`
(`internal/gateway/circuit_breaker.go`) counts a 429 on purpose: the design
(free-model-429-opens-provider-circuit-stub-stories-2026-10-14) makes an open
model breaker the short per-model backoff that moves an opencode-go run to its
next pool member. That reasoning needs a failover target. `synthetic-luna` has
none, and a concurrency 429 says nothing about the model's health: it says
"this account has too many calls in flight right now", which the gateway
itself caused.

Classification. Substrate: the gateway has no per-provider concurrency bound
for a provider with an account-wide ceiling. The RES-11 model scheduler
(`internal/gateway/model_scheduler.go`) is the existing seam but is off
(`GATEWAY_MODEL_SCHEDULER`), is fair-share per computer rather than per
provider, and bounds all providers together (24 global), so it cannot express
"Synthetic: 8, everything else: unbounded" without a per-provider dimension;
turning it on would also change every other route. The closer seam is the
provider package's own bounded-wait gates (the image decode slots and output
budget in `internal/provider/media_budget.go`, reported on `/health` as
`image_media`), which sit inside the provider call, beneath the breaker, and
count waited and refused. Symptom: the breaker counting a concurrency 429.

Chosen change (smallest correct):

1. A concurrency gate inside `SyntheticProvider` (semaphore, default 8, env
   `SYNTHETIC_MAX_CONCURRENT`; wait bound default 120 s, env
   `SYNTHETIC_MAX_WAIT`, and the request context). A call waits for a slot
   instead of being sent and rejected, and holds the slot to the end of the
   response or stream. Waiting is not failover, so the owner's ruling holds.
   In flight, peak, waited, wait-timeouts and the limit are reported on
   gateway `/health` as `synthetic_gate`, next to `image_media`.
2. The breaker does not count (a) a Synthetic 429, typed
   `provider.ErrUpstreamConcurrencyLimit` at the provider where the status is
   known, nor (b) a gate wait that ended without a slot
   (`provider.ErrConcurrencyGateWait`): neither says the model is unhealthy,
   and (b) never reached the upstream. 5xx, timeouts and transport failures
   still count. The error text of both keeps the sanitized form the run reads
   (`synthetic: status 429 Too Many Requests (sanitized)`).

Declined for now: not counting 429 for every provider. The opencode-go and
cat-jam-free 429s are per-key or per-model rate limits where the open breaker
is the designed failover trigger, and `circuit_breaker_client_error_test.go`
pins that. Removing it needs a clustering assessment of the opencode pools, not
a patch riding on this one. The 429 is scoped to the provider whose 429 is
known to be a concurrency limit.

Ways it could fail (written before the tests): the gate leaks a slot on a
context cancel or an upstream error and starves later calls; a stream releases
its slot at response headers instead of at the end; a wait timeout is counted
as a model failure; the gate is bypassed on the non-stream path; a 429 still
reaches the breaker through the stream path; `/health` reports no gate until
the first call.

Disposition (fix landed on `lane/fp0-synthetic-concurrency`, not yet deployed):

- Gate: `internal/provider/synthetic_gate.go`, held in `SyntheticProvider.call`
  before media preparation, released at the end of the response or stream.
  Defaults 8 slots and a 120 s wait; `SYNTHETIC_MAX_CONCURRENT` and
  `SYNTHETIC_MAX_WAIT` override. The unit needs no change for the defaults. A
  caller that goes away while waiting leaves at once and holds nothing.
- Breaker: `providerFailureCounts` skips `ErrUpstreamConcurrencyLimit` (a
  Synthetic 429, typed at the provider where the status is known) and
  `ErrConcurrencyGateWait`. The error text the run reads is unchanged
  (`synthetic: status 429 Too Many Requests (sanitized)`); a gate wait reads
  `synthetic: call ended while waiting for a synthetic concurrency slot ...`
  and carries no status, so the run does not read it as a rate limit.
- Health: `/health` carries `synthetic_gate` (limit, max_wait_ms, in_flight,
  peak, granted, waited, wait_timeouts, cancelled). To verify on staging after
  deploy: `waited` rises under load while `wait_timeouts` stays near zero, the
  journal shows no `synthetic: status 429` and no `circuit open` for the
  Synthetic model, and `peak` never exceeds `limit`.
- Tests: provider `synthetic_gate_test.go` (a fake upstream that 429s above N
  concurrent: 12 callers all succeed in both call shapes, a timed-out waiter is
  never sent and leaks no slot, a waiter honours its context, the 429 is typed,
  counters) and gateway `synthetic_gate_breaker_test.go` (a 20-caller burst
  through the real breaker succeeds, six straight 429s leave the breaker
  closed, 5xx still opens it, an opencode-go 429 still counts, `/health`).
- Declined: not counting 429 for every provider (reason above). Also declined:
  a gateway-wide scheduler dimension per provider; RES-11 stays off and
  untouched.
- Residual R-b below is superseded: a burst above the ceiling now waits (up to
  120 s) instead of failing. The gate is per gateway process and the account
  limit is per key, so a second consumer of the same key (the probe, a local
  run) still uses the headroom; the default 8 of about 10 leaves two. Waiting
  adds latency to the Synthetic half of the paper under load, visible as
  `waited` and the run's step time, and a wait of 120 s then fails the call
  (no failover, by ruling).

## Residuals and risks (named, not fixed here)

- R-a: a pinned-name rotation returns 404 with a body our sanitized error
  drops, and a bare 404 is not a failover trigger. Tonight that is the intended
  behaviour (owner: fail rather than fall back): the Synthetic share errors
  until someone looks, loudly, not silently. Candidate follow-up when failover
  returns to this pool: treat a 404 from a pool member as a failover trigger.
- R-b (superseded 2026-10-23 by the gate; kept for the record): concurrency 10 account-wide against 24 in flight at the gateway and 6
  per computer. With no failover, a burst above 10 in flight turns the excess
  Synthetic calls into 429 run errors (and opens the model breaker for its
  short window) instead of luna calls: expect error events on the Synthetic
  half under load, and read them as the concurrency ceiling, not as a model
  fault. The paper alone is capped at 6 in flight.
- R-g: a gateway without `SYNTHETIC_API_KEY` answers `unsupported provider:
  synthetic`, which is not a failover marker; with no failover it is moot, but
  the key must be provisioned before the paper is pointed at the pool
  (`CHOIR_PROVIDER_ONLY_KEYS=SYNTHETIC_API_KEY ./nix/deploy-provider-creds.sh
  node-b` upserts it with no other change, then restarts the gateway). No nix
  change: the unit's `EnvironmentFile` loads `gateway-provider.env`, and the
  base URL defaults in code. The pool and the catalog also live in the guest
  binary (the guest builds assignments), so a computer's guest must carry this
  commit before a `pool:synthetic-luna` policy resolves; an older guest treats
  an unknown pool as `pool:default`.
- R-c: the output cap of 65,536 is advertised, unverified at the cap.
- R-d: `flushTools` in the shared stream parser ranges over a map, so the order
  of parallel tool calls in `LLMResponse.ToolCalls` is not the model's order.
  Pre-existing and shared by every chat-completions route; discovered here.
- R-e: cache hits are 80 to 90%, not 100%, so cost per turn varies; compare by
  `percentRemaining`, not token counts alone.
- R-f: Synthetic flags the model "Beta".

## Heresy delta

- discovered: shared stream parser drops `delta.reasoning` (Q3); bare-404 model
  rotation is not a failover trigger (R-a); stream tool-call order (R-d).
- introduced: the default pool no longer carries deepseek-v4.1-flash (owner
  ruling, tonight only); `synthetic-luna` has no failover (owner ruling).
- repaired: the shared stream parser reads `delta.reasoning` (Q3), keeps
  parallel tool calls in the model's order (R-d), and reads finish_reason
  "stop" with tool calls as tool use.
