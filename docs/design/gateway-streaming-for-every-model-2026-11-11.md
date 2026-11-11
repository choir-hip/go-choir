# Streaming from the gateway for every model: audit and plan

Date: 2026-11-11. Status: design, read-only audit; no code changed, no computer
acted on. Mutation class of this document: green (rollback: revert).

Owner's ruling (2026-11-11), verbatim: "we should implement streaming from the
gateway for every model as a setting. In general, we don't really need it, but
I mean, maybe that could be a good change of pace for texture as well, because
it would give some faster output anyway."

Two owner rules from the same day bound the plan. The standing rule is no
off-by-default feature switches (CLAUDE.md, Delivery). The Autoradio ruling is
no deterministic timers around model calls on the turn path; speed is measured
and fixed, not enforced
(`docs/design/autoradio-desk-fast-turns-2026-11-11.md`, "Pool and timers").

Sources read: `CLAUDE.md`, `docs/choir-doctrine.md` (sections on Texture and
custody), `docs/reference/README.md` (there is no reference page for the
gateway or for any model provider; see section 9), the code named in each
claim, and the Autoradio desk branch through `git show
origin/lane/autoradio-desk:<path>`. Line numbers are those of `origin/main` at
the time of reading (the desk branch is cited as such).

## 1. Summary

1. **Nothing streams in production today.** The gateway journal on Node B holds
   57,196 `inference request` lines since 2026-11-10 06:30 UTC (45,909 in the
   last 24 hours); every one says `stream=false`. The streaming branch of the
   gateway has no production caller yet; the Autoradio desk will be the first.
2. **Eight of the nine routed providers already stream from the upstream
   incrementally.** Bedrock is the one that does not (it makes one whole call
   and fakes three chunks). Devin and ChatGPT stream upstream even on the
   non-streamed path, because their wires only stream.
3. **The streamed path through the gateway cannot yet carry an agent turn.**
   The guest reader `parseGatewaySSE` drops tool calls (it never accumulates
   `tool_call_delta`), zeroes input tokens on the second usage chunk of an
   Anthropic-style stream, drops cache-write counts, reads the gateway's
   untyped `{"error"}` line as a clean end, and accepts a body with no `[DONE]`.
   The stream handler also turns every non-quota error that happens before the
   first chunk into HTTP 200 plus an error line, where the non-streamed path
   answers 502 or 400. Every one of these is a defect to fix before any agent
   loop streams.
4. **Most upstream parsers accept a truncated stream as a complete answer.**
   Only Devin's parser refuses a stream with no end frame. The chat-completions,
   Anthropic and Responses parsers treat end of body as "stop".
5. **Where the choice lives (recommendation):** with the caller, expressed as
   "this call has a live reader". The tool loop streams when its caller hands it
   a delta sink; the desk manifest states which roles have a sink; the model
   policy file does not carry a stream flag. Nothing is off by default, and
   nothing is a hidden switch (section 6).
6. **Texture's faster output** is, honestly, earlier visible text, not faster
   completion. A Texture revision is written inside a `desk_go_eval` cell
   (`choir.Write`), and a measured revision call was 10 s for 1,642 output
   tokens with empty text. A streamed call lets the browser show the draft
   directive text as the model writes it, labelled a draft, on a new
   live-only route. A draft is not a revision and writes nothing canonical.
7. **Order:** (a) Bedrock real streaming (the Autoradio desk's fallback needs
   it first), (b) one reader and one error contract, (c) usage and metering
   parity, (d) the agent loop streams per role with a differential test, (e)
   Texture draft view. Slices (a) to (c) are red; (d) is red because it changes
   how provider calls are made; (e) is orange.

## 2. Part 1: the truth table

### 2.1 What the gateway does with a streamed request

`HandleInference` (`internal/gateway/handlers.go:432`) decodes the request,
resolves the provider (`:614`), logs the call with `stream=%v` (`:497`), counts
the admission in the call tally (`h.calls.Call`, `:497`) and, if `req.Stream`,
hands off to `handleStreamingInference` (`:549`). The provider seen there is the
wrapped one: paid-spend wrapper outside, circuit-breaking wrapper inside
(`SetPaidSpend`, `internal/gateway/paid_budget.go:408`; `WrapMultiProvider`,
`cmd/gateway/main.go:57`). Both wrappers implement `Stream` as well as `Call`
(`paid_budget.go:383`, `circuit_breaker.go:266`).

The streamed handler:

- sets `Content-Type: text/event-stream` headers up front (`handlers.go:552`)
  but writes no status until the first chunk; each chunk is one `data:` line of
  the provider package's `StreamChunk` JSON (`internal/provider/provider.go:248`),
  flushed at once (`:566`);
- on error with nothing written and a typed quota error, answers exactly as the
  non-streamed path does: HTTP 429, `Retry-After`, typed `provider_quota` body
  (`:585`, `writeProviderQuota` `:174`);
- on any other error (before or after the first chunk) writes
  `data: {"error":"<sanitized>"}` with status 200 and no `[DONE]` (`:594`);
- on success writes `data: [DONE]` and logs `streaming inference succeeded`
  (`:607`), then records `calls.Served` with the whole prompt size.

There is no final frame: the stream never carries the finished response (tool
calls as one object, full usage including cache writes, provider name).

### 2.2 Per provider and model

"True stream" means the provider's `Stream` reads the upstream body
incrementally and calls `onChunk` as events arrive. Pools are from
`modelcatalog.Pools()` (`internal/modelcatalog/catalog.go:514`): `default`
(devin swe-2-flash; opencode-go iceman and beam; openrouter flint and nemotron;
chatgpt gpt-6-luna; then the paid tail: bedrock haiku-5-5 about 80%, synthetic
DeepSeek 10%, river DeepSeek 10%), `synthetic-luna`, `luna-synthetic`,
`selfdev` (synthetic then river). ZAI and Fireworks are registered when their
keys are present (`provider.go:3829` onward) but are in no pool; Fireworks has
no catalog entry.

| Provider (models) | `Stream` exists; truly streams? | Upstream wire | Tool calls stream? | Reasoning separated? | Usage in stream |
|---|---|---|---|---|---|
| **bedrock** (haiku-4-5, haiku-5-5, sonnet-4-6, opus-4-6) | Exists, **fake**: calls `Call`, then emits `message_start`, one text chunk, `message_stop` (`provider.go:465`). `Call` posts to `/invoke` (`:495`). | Anthropic Messages JSON over HTTPS; streaming would be AWS EventStream (binary) on `/invoke-with-response-stream` | No. The whole `ToolCalls` list is on the returned response, none in chunks. | No. `parseBedrockResponse` (`:2637`) ignores thinking blocks in `Call` too. | The one `message_start` chunk carries input and output tokens only; cache read and write fields are dropped (`:471`). The returned `LLMResponse.Usage` is complete. |
| **zai** (glm-5.2, 5.1, 5-turbo; no pool) | True (`:809`, `parseSSEStream` `:3024`) | Anthropic Messages SSE | Yes: `content_block_start` with id and name, `input_json_delta` chunks (without id or name, only `Index`), accumulated and finished at `content_block_stop` (`:3139`-`:3182`) | **No**: `thinking_delta` and `signature_delta` are not handled (`:3139`-`:3166`), so `ReasoningContent` is empty where `Call` (`parseAnthropicResponse`, `:2680`) keeps signed thinking blocks | `message_start` carries input and cache read and write (`:3084`); `message_delta` carries output tokens only (`:3182`) |
| **fireworks** (no catalog entry; no pool) | True (`:966`) | OpenAI chat-completions SSE | Yes (see chat-completions row below) | Yes | as chat completions |
| **opencode-go** chat-shape models: deepseek-v4.1-flash, glm-5.3-flash, hy3, iceman, beam and the other free models (`:1737`) | True on `Stream`; `Call` is non-streaming for this shape (`:1580`, `:1659`) | OpenAI chat-completions SSE | Yes | Yes (`reasoning_content`) | Merged final usage chunk |
| **opencode-go / opencode-zen** Responses-shape: muse-spark-1.3 (`:1741`) | True; `Call` also streams (the shape has no non-stream branch, `:1666`) | OpenAI Responses SSE | Yes | **No**: reasoning events are ignored (`parseOpenAIStreamLogged` `:3252`) | `response.completed` usage (`:3358`) |
| **opencode-go** Messages-shape: qwen3.7-max (`:1743`) | True | Anthropic Messages SSE (`parseSSEStream`) | Yes | **No** (as zai) | as zai |
| **synthetic** (DeepSeek V4.1 Flash 1017) | True (`:1110`); the concurrency gate slot is held to the end of the stream (`:1117`) | chat-completions SSE | Yes | Yes (`reasoning` and `reasoning_content` spellings, `:2877`) | Trailing `choices:[]` chunk carries cache report; merged by `mergeStreamUsage` (`:1929`) |
| **river** (DeepSeek V4.1 Flash 1017) | True (`river.go:75`) | chat-completions SSE | Yes | Yes | merged final chunk |
| **openrouter** (flint, nemotron, inkling) | True (`openrouter.go:76`) | chat-completions SSE | Yes | Yes | usage only on the second finish chunk; handled |
| **chatgpt** (gpt-6-luna) | True. `Call` **also** streams upstream and discards chunks (`provider.go:1308`, `:1325`); `Stream` passes them on (`:1328`) | Responses SSE (Codex backend) | Yes | **No** | `response.completed` |
| **devin** (swe-2-flash, swe-2) | True. `Call` is `Stream(ctx, req, nil)` (`devin.go:147`) | Connect protocol, proto frames (`devin.go:1205`) | Yes: cumulative argument snapshots, emitted as suffix deltas (`:1384`); chunk types `toolcall_start`, `toolcall_delta`, `toolcall_end` differ from every other provider | **Yes** (`deltaThinking` becomes `ReasoningDelta`) | Input, output, cache read (`:1426`); cache writes not tracked. No usage chunk when output is 0 (`:1367`). |
| **jev** (OpenRouter Decisions) | Not a chat route. `/provider/v1/...` judgment transport, one request and one answer (`internal/gateway/jev.go`) | JSON | n/a | n/a | n/a |
| **anthropic relay** (`/provider/anthropic/*`) | Byte pass-through of the upstream stream with a usage tap (`internal/gateway/anthropic_relay.go:699`) | Anthropic Messages SSE | passes through | passes through | tapped for counts |
| OpenAI-compatible route (`/v1/chat/completions`) | Streams through `p.Stream` (`openai_compat.go:482`); never calls `calls.Served` (`handlers.go:532`, `:607` are the only callers) | OpenAI SSE out | Yes | Yes | drops cache-write fields |

Chat-completions requests set `stream: true` only; none sets
`stream_options.include_usage` (grep of `internal/provider` finds no
occurrence). Whether each chat-completions provider (River, Synthetic,
OpenRouter, OpenCode) sends usage on a stream without it is **not established
from code**; the desk's proof on River shows usage on its stream (the desk
doc's `done` example), but that is one provider. Treat the rest as unproven
until a staging probe says otherwise (section 8, open item 2).

### 2.3 Errors, refusals and pool walking

Before the first chunk, the upstream's HTTP status decides:

| Case | What the provider returns | Non-streamed gateway answer | Streamed gateway answer today |
|---|---|---|---|
| Quota (typed): devin (`devin.go:368`), openrouter (`openrouter.go:129`), river (`river.go:125`), synthetic (`provider.go:1162`), chatgpt (`quota.go:247`) | `*QuotaExhaustedError` | 429, `Retry-After`, typed body | **same** (`handlers.go:585`) |
| Quota by gateway state: breaker hold, paid day budget (`paid_budget.go:261`), hourly Devin pacing shed (`circuit_breaker.go:287`) | `*QuotaExhaustedError` before the upstream is called | same 429 | **same**, because nothing has been written |
| Bedrock, ZAI, OpenCode, Fireworks refusals | plain `status NNN (sanitized)` errors; no typed quota is built anywhere on these paths | 502 with the sanitized message | **200 with an error line** (`handlers.go:594`) |
| Refused image | media error | 400 (`handlers.go:517`) | 200 with an error line |
| Open circuit | plain "circuit open" error | 502 | 200 with an error line |

The guest classifies failures by **text** (`IsModelFailoverError`,
`internal/toolregistry/toolloop.go:1965`: "429", "quota", "circuit open",
"timeout"), and the sanitized message keeps the status text
(`sanitizeError`, `handlers.go:852`), so an error line still walks the pool. What
is lost is the HTTP status: `gatewayHTTPStatusError.Transient()`
(`internal/gatewayruntime/provider.go:395`) and the call path's retry and
rate-limit wait (`call` `:229`, three attempts, up to ten `Retry-After` waits)
read the status. `stream` (`:321`) has neither a retry nor a wait. A media 400
would also read as a retryable failure.

After the first chunk the gateway can only write an error line. Provider side:

| Parser | Mid-stream upstream failure | Truncated body (no end) |
|---|---|---|
| chat completions (`provider.go:2785`) | An in-stream `{"error":...}` payload has no `choices`; it is ignored. A transport error is returned. | Accepted: `finishSeen` false becomes reason "stop" (`:2954`), a clean `end_turn` |
| Anthropic SSE (`:3024`) | `event: error` is logged and swallowed (`:3205`), the stream is returned as a success | Accepted |
| Responses (`:3252`) | `response.failed` becomes an error (`:3382`); a bare `error` event is not handled | Accepted; stop reason may be empty |
| Devin (`devin.go:1205`) | Trailer error is classified, including typed quota (`:1463`) | **Refused** ("truncated"); also refuses a nameless or invalid-JSON tool call (`:1320`) and an empty stream |

The stall watchdog (`internal/provider/stall_reader.go:24`: 180 s to the first
byte, 75 s idle) wraps the three SSE parsers. It is a deterministic timer around
a model call. The owner's Autoradio ruling removes such timers from that turn
path; the watchdog still applies to the River stream the desk uses. See open
question 3.

The guest side, `parseGatewaySSE` (`internal/gatewayruntime/provider.go:436`):

- `chunk.Type == "error"` is an error; the gateway never sends that (it sends
  an untyped `{"error":...}`), so the real error line is skipped and the
  function returns the text so far as success. This is the problem record
  `docs/problems/gateway-stream-error-line-read-as-clean-end-2026-11-11.md`
  on the desk branch, confirmed here by reading both sides.
- `[DONE]` is skipped, never required: a dropped connection is a success.
- `ToolCallDelta`, `ToolCallID`, `ToolCallName` are never read, and
  `llmResponse.ToolCalls` is never filled. A streamed tool turn returns
  `StopReason: "tool_use"` with no calls, which the tool loop reports as an
  error (`toolloop.go:1423`, "tool_use without tool calls").
- Usage is **overwritten** on every usage chunk (`:470`-`:480`): an
  Anthropic-style stream sends input and cache on `message_start` and output on
  `message_delta` with input 0, so the input count ends at 0. The desk's
  `readTextStream` fixed this for itself; the shared reader still has it.
- Cache-write counts (`CacheWrite5mTokens`, `CacheWrite1hTokens`) are not in
  `StreamUsage` (`provider.go:286`) and so never cross the wire.

### 2.4 Metering, spend and the availability record on the streamed path

| Item | Non-streamed | Streamed | Difference |
|---|---|---|---|
| Call tally admission (`calls.Call`) | before the call (`handlers.go:497`) | same, before the mode branch | none |
| Call tally served (`calls.Served`, input tokens) | on success (`:532`) | on success (`:607`) | none for gateway-routed streams. The OpenAI-compatible route never calls it in either mode. |
| Paid spend (`paidSpendProvider.count`, `paid_budget.go:392`) | after a successful `Call`, from the full `resp.Usage` | after a successful `Stream`, from the full `resp.Usage` returned by the provider (not from chunks) | none on success. **On a failed or cancelled stream nothing is recorded** (`err != nil`), though tokens were generated and billed. A client disconnect mid-stream is far likelier than mid-call. |
| Devin hourly pacing record (`recordUsage`, `circuit_breaker.go:365`) | on success | on success | same gap on failure |
| Breaker outcome (`execute`, `circuit_breaker.go:281`) | whole call | whole stream: the breaker sees only the terminal result (`:262`) | none, but one slow-but-healthy stream holds a half-open probe slot for its duration |
| Guest availability record (`ledger.MarkSourced`, `Demote`, `DemoteHard`; `toolloop.go:1226`-`:1300`) | driven by the error `call` returns | would be driven by the error the stream returns | none if the error contract of section 4.2 holds; today the status-dependent parts are lost |
| Guest cost record (`provider_call_ms`, tokens, `served.Note`; `toolloop.go:1383`-`:1420`) | from `ToolLoopResponse` | same, if the reader returns full usage | lost today: input 0, no cache writes |
| Prompt-cache logging (`RecordCacheUsage`) | in `parse*` | in `parse*` | none |

### 2.5 The gateway journal

Command class: `journalctl -u go-choir-gateway --since "-24 hours"` and `"-7
days"` piped to `grep -c` and `uniq -c` on the `inference request from
autoputer` lines, run through `ssh node-b`. No secrets, environments or
command lines were read. The journal on Node B starts 2026-11-10 06:30 UTC.

| Window | Calls | `stream=false` | `stream=true` |
|---|---|---|---|
| last 24 h (to 2026-11-11 13:01 UTC) | 45,909 | 45,909 | 0 |
| since 2026-11-10 06:30 UTC | 57,196 | 57,196 | 0 |

By provider and model, last 24 h: devin swe-2-flash 33,756; opencode-go iceman
6,054; bedrock haiku-5-5 2,573; synthetic DeepSeek 1,381; openrouter flint
1,172; opencode-go beam 758; openrouter nemotron 185; chatgpt gpt-6-luna 30.
River does not appear (no production traffic yet). Consequence: the streamed
branch has had no production exercise at all, and every claim about it in this
document is from code, not from traces.

## 3. Part 2: the guest side

### 3.1 How an agent call is made today

`toolregistry.ToolLoop` (`internal/toolregistry/toolloop.go`, the loop around
`:1060`-`:1700`) builds a `ToolLoopRequest` per iteration (`:1088`) and calls
`callToolLoopProviderWithPoolRetries` (`:2175`), which calls
`provider.CallWithTools` (`:2181`). The gateway client implements it
(`gatewayruntime/provider.go:179`) with `Stream: false` hard-coded (`:188`), and
`call` (`:229`) forces `req.Stream = false` again (`:230`). Around it:

- retries: three attempts for transient failures and 5xx, rate-limit waits on
  `Retry-After` (up to ten, a minute each), `call` `:229`-`:318`;
- pool walk: a typed `ProviderQuotaError` switches model at once and marks the
  quota ledger (`toolloop.go:1226`-`:1260`); a failover-class error moves to the
  next member (`:1295`-`:1340`); `advanceFallback` (`:964`) strips provider
  replay state and changes the conversation id and cache key; both are
  guest-side decisions on the returned error;
- deadlines: `options.providerCallTimeout` and `requiredNextToolCallTimeout =
  45 s` (`:157`, `:1120`-`:1128`) bound a call. These are existing timers on the
  model-call path of the agent loop; the owner's ruling today concerns the
  Autoradio turn path, but the agent loop streaming work must not add a stall
  timer (open question 3);
- after the response: the progress line with tokens, cache and `provider_call_ms`
  (`:1383`-`:1420`), then the tool batch runs only on a **complete** response
  (`ExecuteToolBatch`, `:1463`), the assistant and tool-result messages are
  appended to the run memory, and the loop continues or ends.
- every desk, Texture included, has one tool, `desk_go_eval` with a `source`
  string (`internal/agentcore/tools_desk.go:585`); Texture authors with
  `choir.Write` inside that cell (`tool_profiles.go:335`).

Other model callers that use `CallWithTools`: the algorithm2 round
(`textureowner/algorithm2_owner.go:262`, a one-shot, no tools), the model-policy
verifier (`modelpolicy/tools_model_verify.go:203`), and the memory hooks'
thread call (`toolloop.go:1013`).

### 3.2 What a streamed agent call must preserve

1. **Complete tool-call JSON before acting.** The loop executes tools only after
   the whole response. A streamed call must accumulate and canonicalise each
   call (the providers already do: `canonicalJSON` at finish) and the stream
   must end with a frame that carries them. The tool loop must never act on a
   partial argument. This needs the final frame of section 4.2 because
   `parseGatewaySSE` has no tool accumulation today.
2. **Identical `ToolLoopResponse`.** ID, stop reason, text, reasoning content
   (needed for replay on deepseek-style providers), tool calls, usage with the
   cache read and both write counts, model. Section 5 (slice d) is the
   differential test for this.
3. **Usage accounting.** `served.Note`, the cost record and `llmcost` read
   `resp.Usage`; the stream must end with the final usage, not a last-chunk
   overwrite.
4. **Trace and ledger events.** `provider_call_started`, the `tool_loop`
   progress line and `EventModelServed` stay one per call, written once from the
   final response. Deltas are **not** tape events: `EventRunDelta` is a durable
   (batched) kind (`types/event_durability.go:41`), so one per token would be a
   tape cost the journal cuts removed on purpose (`types/event_live_only.go`).
   A new delta kind must be `LiveOnly`.
5. **Cancellation.** Context cancel closes the response body; the provider's
   `Stream` returns, the breaker sees a context error (not counted as a
   failure, `providerFailureCounts(ctx, err)`), spend is not recorded (gap in
   2.4).
6. **Retry and failover semantics.** An error before the first delta behaves
   exactly as `call` (the same retry, wait, typed-quota and failover rules). An
   error after the first delta ends that attempt; the live reader gets a reset
   marker, and the loop applies the same classification, because nothing has
   been acted on.
7. **No new timers.** A streamed call has no first-byte or idle limit of the
   guest's making. The existing 10 minute deadlines remain.

### 3.3 Where partial output could usefully surface

The loop already has the natural seam: the progress emitter (`emit`,
`runEventEmitter`, `agentcore/runtime.go:4000`), which publishes `LiveOnly`
lines on the in-process bus (`emitRunEvent`, `:4025`).

### 3.4 Texture: how a revision is produced and shown today

Production. A Texture run is a desk-cell run on `desk_go_eval`; the model's
output is a tool call whose `source` is Go code; inside it `choir.Write(...)`
takes the directive script (`@replace b3`, `@insert after b2`, `@all`, text in
a markdown subset: `internal/texturewrite/grammar.go:1`-`:30`). The reducer
folds the cell and `CommitCellTextureAuthor`
(`textureowner/texture_cell_author.go:44`) commits it through
`ApplyTextureTurn`: head-pinned, idempotent per cell identity. "Texture stays
the sole writer of canonical revisions: nothing else here mints one"
(`texture_cell_author.go:36`).

Timing. From the desk design's measurements on d@e
(`autoradio-desk-fast-turns-2026-11-11.md`, section 2): turn A, one Bedrock
Haiku 5.5 call of 9,002 input and 1,642 output tokens, **text empty** (the
output was the cell), took 10 s; ask to first revision 12.2 s. Turn B, three
sequential calls of 5, 1 and 1 s; 10.7 s. So the long pole of a revision is
generation of the cell, not first-token latency.

How the browser learns. Two carriers exist
(`textureowner/texture.go:1494` `HandleTextureDocumentStream`):

- documents with a `TrajectoryID` (lifecycle-authored, the current kind) get
  `handleLifecycleTextureDocumentStream` (`texture_observation.go:307`): a
  durable cursor over the store, polled every second, heartbeat every 15 s,
  `event: texture`, `replay_required` on cursor expiry. The web client opens an
  `EventSource` on `/api/texture/lifecycle/{trajectory}/stream?after=` and
  reconnects at the last durable cursor (`frontend/src/packages/texture/lifecycle.js:93`).
  It reads the store, so a bus-only event never reaches it.
- legacy documents get a bus subscription (`texture.go:1540`-`:1620`):
  `snapshot`, `synth_*`, `revision_created`, `head_changed`. The bus drops
  events for a full subscriber (`events/bus.go:186`).

Run progress is `texture.agent_revision.progress` (phase names only), emitted
`LiveOnly` (`types/event_live_only.go:26`; `agentcore/runtime.go:4010`).

### 3.5 What "faster output" can honestly mean for Texture

| Candidate | What the reader sees earlier | Custody | Verdict |
|---|---|---|---|
| A. Show the draft text as the model writes the `choir.Write` script, in a "draft" pane | The text starts appearing about 1 s after the call starts instead of at the commit, 10 s later in turn A | A draft is a live preview from a model stream; no revision, no write. The committed revision may differ (the cell can fail a Write refusal, `texturewrite` line errors, or the script can be rejected). Must be labelled and discarded on commit. | The smallest honest version. Fragile in one place: the text is inside a JSON string field `source` that holds Go that holds a string literal. The extractor must unescape the JSON string incrementally and then find the `choir.Write(` literal. A miss shows nothing, never wrong text. |
| B. Stream the "what I'm doing" narration | Only if the model writes narration text outside the cell. Turn A's text was empty. | None | Not available today; would need a prompt change that adds output tokens. Not recommended. |
| C. First-token latency for short replies | The model's first token sooner | None | Real for the Autoradio desk (short, tool-less). For Texture the call is one long cell. No gain to report. |
| D. Show each completed `@op` block as it completes | Whole blocks, not characters | Same as A | Variation of A: a calmer UI; same extractor. |

Where the carrier goes. Not the lifecycle document stream: it is a durable
cursor with replay, and a draft is neither durable nor replayable. A new
separate, live-only route (for example `GET /api/texture/documents/{id}/draft`)
that subscribes to a new `LiveOnly` event kind (`texture.agent_revision.draft`),
filtered by owner and document, and that sends `reset` when the run fails over
to another model or finishes. A draft event is lost with the process; the
client treats it as optional decoration. On `revision_created` the client
replaces the draft with the committed revision, which it already fetches.

## 4. Where the "setting" lives

Candidates:

| Place | For | Against |
|---|---|---|
| The model policy file (per role, `LLMSelection`, `provideriface/model_policy.go:13`) | The owner can see it | It is provider, model and effort for a role. A `stream` field there is a user-editable switch, per model, that can leave a role's behaviour untested. Violates "no switches". |
| The catalog (per model) | Per-provider capability is a fact | Bedrock is a capability gap (fake today), not a choice. After slice (a) every model streams, so the field has one value. A per-model field would be a switch with one value. |
| **The caller** (the request's `Stream` field, set by what the caller does with deltas) | The existing wire field; no new config; the stream is chosen by whether anything consumes it | The caller must exist; roles without a live reader do not stream |
| The desk manifest (`deskmanifest`): a role "has a live reader" | Declarative, tested, one place | Same as caller; just where the fact is written |

**Recommendation.** The caller chooses, by supplying a delta sink. In code:
`ToolLoopRequest` gains `OnDelta func(Delta)` (not serialised). `CallWithTools`
sends `Stream: true` exactly when `OnDelta != nil` and otherwise stays as it is.
The desk manifest records which roles have a sink (Autoradio: speech; Texture:
the draft view). The model policy file does not change. Every call that has a
live reader streams, with no flag to turn it off; every call without one does
not, because a stream with no reader costs a connection and buys nothing. This
reading of "as a setting" is the per-call, per-role option the brief allows. If
the owner wants a per-computer Settings entry anyway, it belongs in
`computer_settings.go` as an owner-visible fact about the Texture draft view
(show drafts or not), not as a gateway or provider switch.

## 5. Part 3: the plan

All slices land by the landing loop in CLAUDE.md (commit, push, CI, staging
deploy, verify, deployed proof). Local tests are not proof for the gateway.
Each commit that is not docs-only carries `Ring-Expedite: <defect>`; paths under
`internal/agentcore/` expedite on deploy without a trailer. Name every expedite
in the next report. Each slice enters its claim and probability in
`docs/calibration/claude-ledger.md` before landing and settles it after
(proposed claims at the end of each slice).

Review tiers (CLAUDE.md): red where a defect would be silent or irreversible
gets two seats (default seat Gemini 3.8 flash (high) through `agy`, second seat
Sonnet; the author may not sit); re-review after a blocking finding.

Deployed proof for every slice: a run on the test computer d@e against
`https://choir.news`, reading the gateway journal for the call line and the
accepted run ids. Browser proof is not needed except in slice (e).

Order, by value and by risk:

| # | Slice | Class | Why this position |
|---|---|---|---|
| 0 | Reference page `docs/reference/GATEWAY-STREAMING.md` (the Bedrock EventStream notes, the stream wire, the error contract) | green | CLAUDE.md requires a page before touching a dependency newer than training data. Lands with slice (a), not before. |
| a | Bedrock real streaming | red | The Autoradio desk's fallback member; unlocks the only provider that is fake |
| b | One reader, one error contract | red | Every later slice needs the reader to be correct; the desk's `StreamText` is a second reader |
| c | Usage and metering parity | red | Streams must be billed and counted before any agent loop uses them |
| d | The agent loop streams when it has a sink, with a differential test | red | Changes how provider calls are made for all loops with a sink; gated on (b) and (c) |
| e | Texture draft view | orange | New live-only route; touches no canonical write |

The Autoradio desk needs (a) first, and only (a). Its call path (`StreamText`)
already treats the untyped error line and a missing `[DONE]` as errors, takes
the typed quota as a pre-stream 429, and merges Anthropic-style usage. River
streams today. The desk can ship without (b) to (e).

### Slice (a): real streaming for Bedrock

What changes. `BedrockProvider.Stream` (`provider.go:465`) posts the same body
as `Call` (`buildRequestBody`, `:548`; same sanitisation, media handling,
bearer auth) to `/model/{id}/invoke-with-response-stream`
(`Accept: application/vnd.amazon.eventstream`), decodes the binary frames, and
feeds each event to the existing Anthropic SSE logic. `Call` stays as it is.

The wire, from the AWS API reference (fetched 2026-11-11, `InvokeModelWithResponseStream`):
the path above; the response content type is always
`application/vnd.amazon.eventstream`, with `X-Amzn-Bedrock-Content-Type` naming
the inner type; each event is a `chunk` carrying a `bytes` blob (the inner
Anthropic stream event JSON), or one of the exception events
`internalServerException` (500), `modelStreamErrorException` (424),
`modelTimeoutException` (408), `serviceUnavailableException` (503),
`throttlingException` (429), `validationException` (400). Pre-stream errors are
ordinary HTTP statuses, so the existing "status NNN (sanitized)" and the
quota/circuit paths keep working unchanged for them. Two points that are not on
that page and must be verified against a real call and the SDK's decoder tests
in the slice, not taken from memory: that the frame payload for `chunk` is JSON
`{"bytes":"<base64>"}`, and that this account's bearer-token auth is accepted on
the streaming path.

Decoder. **No EventStream decoder is in our module graph**: `go.mod` and
`go.sum` contain no `aws` or `smithy` module. The maintained one is
`github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream` (resolved `v1.7.27` on
2026-11-11; Apache-2.0; its own module, `go 1.24`, requiring only
`github.com/aws/smithy-go`; about 1,200 lines of which `Decoder`, `Message` and
header values are what we would use; stdlib-only imports, no cgo, with CRC and
test vectors in the package). Proposal: adopt it, because a hand-rolled decoder
of a binary protocol with CRCs is the kind of code that fails silently on the
one frame type nobody tested. Cost: two new modules in `go.mod`/`go.sum`, a new
`vendorHash` in `flake.nix:231`, and a nix eval of both systems
(CLAUDE.md, Review). The alternative, about 120 lines of stdlib
(`encoding/binary`, `hash/crc32`: 12-byte prelude with total length, headers
length, prelude CRC; headers; payload; message CRC), is acceptable if the owner
prefers no new modules; it would then need the SDK's test vectors copied as
fixtures. Recommendation: the SDK module.

The decoded events are the Anthropic stream events, so the slice reuses
`parseSSEStream`'s event handling by factoring its `switch` into a function
that takes one decoded event, rather than writing a fourth parser. The same
slice makes the Bedrock stream complete only on `message_stop` (a body that
ends first is an error), maps an exception event to a sanitized error whose
text carries the status words the guest classifies on ("429", "503",
"throttling"), and does not wrap the body in the stall watchdog (open question
3).

Protected surfaces: provider calls, gateway streamed path, usage fields
(`message_start` carries input and cache; `message_delta` carries output).
Rollback: revert the commit; `Call` is untouched and the gateway streams only
for callers that ask, so a revert restores the one-chunk fake. No data change.

Tests, with a fake upstream in the provider's wire format: a fake HTTP server
that writes EventStream frames built with the SDK's `Encoder` (or hand-built
bytes in the fixture): (1) text, split across frames and across TCP writes
mid-frame; (2) a tool call with `input_json_delta` pieces and canonical JSON at
the end; (3) thinking blocks (decided in slice (c)); (4) usage across
`message_start` and `message_delta`, with cache read and both cache writes;
(5) an exception event after two text frames: error, partial text visible to
`onChunk`, nothing recorded as success; (6) a body that ends before
`message_stop`: error; (7) a corrupted CRC: error; (8) a pre-stream 429 and 400:
the same errors as `Call`; (9) context cancel mid-stream: body closed, no
goroutine left; (10) differential, part of slice (d): `Call` and `Stream`
against the same fake produce equal `LLMResponse`.

Deployed proof: a Bedrock-pinned role on d@e (policy file pins
`us.anthropic.claude-haiku-5-5`), a `stream=true` request through the gateway,
journal line `provider: bedrock stream` with two or more chunks, first chunk
sooner than the whole-call time, and the run's `ToolLoopResponse` usage equal to
the `Call` run's for the same prompt within cache noise. Then the Autoradio
proof script with River refused (simulate by calling a pool whose first member
is Bedrock) shows hops and first-sentence time. Ledger claim: "Bedrock Haiku
5.5 streamed first token is under half the whole-call time on 10 to 35 K
token prompts", 70%.

Review: red, two seats. Estimated size: one day for the provider and tests, one
for the nix hash and the staging proof.

### Slice (b): one reader, one error contract

Defects to fix, each with its test (the failure list comes first, per
CLAUDE.md):

1. `parseGatewaySSE` reads an untyped `{"error"}` line as an error.
2. It requires `[DONE]`: a body that ends without it is an error.
3. It merges usage like `readTextStream` (never replace a non-zero count with
   zero).
4. It accumulates tool calls.

How, with the least new surface. Add a **terminal frame** to the gateway stream
(additive; old readers ignore unknown fields): before `[DONE]` the handler
writes `data: {"type":"message_complete", ...}` carrying the finished
`ProviderResponse` (the same struct the non-streamed path writes,
`handlers.go:534`): id, model, stop reason, text length, tool calls, the full
usage with cache writes, provider name. The reader takes tool calls, usage and
stop reason from that frame, and uses deltas only for the live view. This is
the cheapest way to get identical results in both modes: the streamed result
**is** the non-streamed result, and deltas are decoration. Also make the error
line typed: `{"type":"error","error":...,"kind":...,"status":...}` keeping the
old `error` key so existing readers still see it.

Pre-first-chunk parity: the handler should hold its status until the first
chunk is written, and on an error before that, answer as the non-streamed path
does (`status` 502/400, sanitized body), not 200 plus an error line. The
`wrote` flag is already there (`handlers.go:564`).

Converge: delete `readTextStream`'s private parsing by making it a thin consumer
of the shared reader that exposes the text deltas and the typed result; keep
`StreamText`'s interface (`provideriface.TextStreamProvider`), which is a
different, simpler contract and the desk's. One reader means one place that
knows the wire.

Also in this slice (provider side, same class): chat-completions and Anthropic
parsers refuse a truncated body and surface an in-stream `error` payload, as
Devin's already does. This changes behaviour on the non-streamed Responses and
OpenCode paths that share `parseOpenAIStreamLogged` (ChatGPT's `Call` streams
upstream), so it lands with its own tests and is the riskiest part of the slice;
if it must be split, split it as (b2) and land it after (b1), the reader.

Protected surfaces: gateway streamed wire, guest reader, provider parsers shared
with the non-streamed path (ChatGPT `Call`, Devin `Call`, OpenCode Responses).
Rollback: revert; the terminal frame is additive so a half-deployed state (new
gateway, old guest) keeps working, but the order is gateway first, then guest.
Because guests and gateway deploy separately, the reader accepts both with and
without the terminal frame for one release, then the fallback is deleted.

Tests: fake gateway bodies (typed error, untyped error, no `[DONE]`, terminal
frame with tool calls, Anthropic-style usage split); fake upstream per parser
for truncation and in-stream error. Deployed proof: the streamed `Execute` path
(plain prompt) on d@e with an upstream failure forced by a fake model name
mid-stream is not possible on production, so the proof is a staging run of the
desk's `scripts/prove-autoradio-turns` plus a one-off `choir` request with
`stream: true` against Devin (true stream, tool call) showing tool calls and
usage present in the reader's result. Ledger claim: "after (b), no streamed
tool turn returns tool_use without calls", 90%.

Review: red, two seats. Size: two days (b1) and one to two (b2).

### Slice (c): usage and metering parity

1. Add `CacheWrite5mTokens` and `CacheWrite1hTokens` to `StreamUsage`; the
   terminal frame of (b) already carries the full usage, so the chunks are
   informational. Priority is the final frame, not the chunks.
2. Probe every chat-completions provider for usage on a stream with and without
   `stream_options.include_usage`; add the option per provider where the probe
   shows usage is otherwise absent, and where the provider does not reject the
   field. A stream with no usage must be a logged anomaly in the gateway
   (`usage missing`), not silently zero.
3. Record spend on a failed or cancelled stream from the best available count:
   output tokens estimated from emitted characters, input from the request, both
   marked `estimated`, so day budgets do not lose billed tokens. Owner act: how
   the estimate is booked (open question 4).
4. `calls.Served` on the OpenAI-compatible route (both modes) so the tally
   matches.
5. Decide thinking: keep reasoning deltas from `thinking_delta` and signatures
   from `signature_delta` in `parseSSEStream` and encode them with
   `encodeAnthropicThinkingBlocks` as the non-streamed path does (`:2723`), so
   Anthropic-style providers replay the same history. Bedrock `Call` drops
   thinking today (`:2637`); the streamed path must equal it, so Bedrock keeps
   dropping until a separate slice changes both.

Protected surfaces: spend accounting (the day budgets, `paid_budget.go`), usage
records, provider requests (`stream_options`). Rollback: revert; no persisted
state beyond the spend ledger, whose estimated rows are marked.

Tests: usage across split chunks for each wire; failed-stream spend; the cancel
case; a fake provider that omits usage and the anomaly line. Deployed proof:
journal lines for a Synthetic and a River stream on d@e showing nonzero tokens
and the spend summary line (`SummaryLine`, `paid_budget.go:340`) moving by the
expected amount. Ledger claim: "River, Synthetic and OpenRouter send usage on a
stream without `include_usage`", 55%.

Review: red, two seats. Size: two days.

### Slice (d): the agent loop requests streaming per role, same results

What changes. `ToolLoopRequest.OnDelta` (section 4). `gatewayruntime.CallWithTools`
sends `Stream: true` when it is set and runs a streamed variant of `call` with
the same retry, rate-limit wait and typed-quota behaviour on errors before the
first delta, and a single attempt after it (the loop's failover decides what
next). The result is built from the terminal frame of (b): so the parsed turn is
identical by construction; the differential test proves it rather than hoping.
The tool loop passes a sink to roles that declare one (`deskmanifest`), emits
deltas as `LiveOnly` events, and writes the same single progress line per call.

Differential test. For each wire (chat completions, Anthropic SSE, Responses,
Bedrock EventStream, Devin Connect), one fake upstream scripted with the same
logical turn in streamed form and in non-streamed form where the provider has
one. Drive the real `gatewayruntime.Provider` through the real gateway handler
(an `httptest` server stacking the handler, the breaker and spend wrappers over
the provider with its base URL pointed at the fake). Run `CallWithTools` with and
without a sink and require `reflect.DeepEqual` on the whole `ToolLoopResponse`
for: plain text; text plus two tool calls; tool call only; thinking; usage with
cache read and both writes; `max_tokens` stop; a typed quota before the first
byte; a 5xx before the first byte (retried identically); an error after the
first byte; a truncated body; a cancelled context. Expected, documented
differences are written into the test as named exceptions (none intended); a
difference that cannot be removed is a finding, not an exception.

Protected surfaces: provider calls and the run loop. Rollback: revert; roles
without a sink never change. Because nothing is on by default and no role has a
sink until it declares one, the first landing changes no behaviour; each role
that declares one (Autoradio, Texture draft view) is its own small change.
That is how this satisfies "no switches": the first landing adds a capability
with no off state; a role either has a live reader or does not.

Deployed proof: an `engineering` or `management` run on d@e that declares a
sink in a test build is not an acceptable proof; the proof is the Texture draft
view of (e) and the Autoradio desk, both of which are real consumers. Before
them, the proof is the differential test run on the build VM plus one staging
run of the Autoradio proof script with the loop path enabled for that role.
Ledger claim: "streamed and non-streamed `CallWithTools` produce equal
`ToolLoopResponse` for all five wires on the fakes", 85%.

Review: red, two seats (one seat must read the differential test, not only the
code). Size: three days.

### Slice (e): Texture's faster output, the smallest honest version

Scope: candidate A of 3.5, narrowed to its smallest form.

1. Texture's role declares a delta sink; the sink passes `source` argument
   deltas (the gateway already sends `ToolCallDelta`) to a **draft extractor**
   (`internal/textureowner`), which unescapes the JSON string incrementally,
   finds the first `choir.Write(` string literal, and emits the directive text
   so far. Nothing else; no Go parsing. A failure to parse emits nothing.
2. A new `LiveOnly` event `texture.agent_revision.draft` (payload: doc id, run
   id, call counter, the text so far or a delta, `reset` flag) published on the
   bus; added to `LiveOnly`, `eventDurability`, `event_storage.go` and the
   kind-classification test that fails for an unclassified kind
   (`event_durability.go:28`).
3. A route `GET /api/texture/documents/{id}/draft` (SSE; owner-scoped like the
   other document routes) that sends a snapshot, then draft events, with a
   `reset` on model failover or when `revision_created` or the run end arrives.
4. The front end shows the draft in a separate, plainly labelled "draft" pane
   (never in the editor, never in history, never copyable as a revision), and
   removes it when the revision arrives.

Custody. The draft route reads the bus only; it creates no document, revision,
changelog, source graph row or tape event. `CommitCellTextureAuthor` is
unchanged; the committed revision comes only from the committed turn. The draft
text is untrusted model output and goes through the same output escaping as any
document text. Reading a document's draft is the owner's, as the document's
stream is (`authenticateUser`, `getTextureDocument`).

Protected surfaces: none of the red list is written. The route is orange; the
slice sits behind the review of the live-only event kinds and the owner-scoping
test. Rollback: revert; the extractor and route are additive.

Tests: extractor fixtures from real cells (the cell sources in
`internal/texturewrite` tests): escapes split across deltas, `\n`, `\"`, a raw
string with backticks, no `choir.Write` in the cell, a cell with two Writes;
a route test that a draft event for another owner never arrives; a test that no
tape row is written for draft events; a test that a failover emits `reset`.
Frontend: a Playwright spec against a fake draft stream (the existing
`frontend/tests/texture-*.spec.js` pattern); deployed proof is a browser run on
d@e at staging with a measured time to first draft text against the time to the
revision for the same prompt, both reported. Ledger claim: "draft text appears
at least 5 s before the revision commits on a prompt whose first call exceeds 8
s", 65%.

Review: orange, one seat. Size: two to three days, mostly the extractor and UI.

### What I would not do

- Stream every agent-loop call by default. A call with no live reader gains
  nothing and holds a longer connection, a breaker slot and a half-open probe.
- Put a `stream` flag in the model policy file or the catalog.
- Act on a partial tool call, or let a draft reach the tool batch.
- Write per-token tape events (`EventRunDelta` is durable, batched).
- Add a first-byte or stall timer anywhere on a streamed turn path. The existing
  watchdog stays out of the new Bedrock path.
- Fake streaming again for any provider: a provider that cannot stream returns
  an honest "no stream" and the caller uses `Call`.
- Stream Jev or the search plane. They are one-shot decisions or have their own
  stream (`searchplane`).
- Carry drafts on the durable lifecycle stream.
- Rewrite Devin, ChatGPT or the OpenCode parsers beyond the truncation and
  error contract of (b).

## 6. Risks across slices

- **Silent truncation** is the largest hazard: five of six parsers accept it
  (2.3). Slice (b) is the guard; slice (d) must not start before it lands.
- **Classification by text.** The guest reads error strings; a sanitized message
  that changes wording changes the pool walk. Tests must assert the walk (typed
  quota, 429 wait, 5xx retry, circuit open failover) in both modes.
- **Stalls without a stall timer.** Without the watchdog, a stream that stops
  sending holds to the 10 minute deadline. Measure the rate (journal line for
  streams over a minute with a gap) before deciding; the owner's ruling is to
  fix speed, not enforce it.
- **Gateway write deadline.** The gateway's `WriteTimeout` is 10m30s
  (`nix/node-b.nix:2556`); a stream shares it with the 10 minute inference
  context. The Texture stream clears its own deadline
  (`texture_observation.go:336`); the gateway stream does not need to.
- **Proxy chain buffering** between guest and gateway is untested for streams
  (section 8).
- **Live-view lies.** A draft that the committed revision contradicts erodes
  trust; the label and the removal on commit are part of the design, not
  polish.

## 7. Open questions that are truly the owner's

1. **Where does "as a setting" live?** Recommendation in section 4 is the
   caller's sink, declared per role in the desk manifest, with no flag in the
   policy file. If the owner wants a user-visible switch, it should be "show
   Texture drafts" in computer settings, which is a display preference, not a
   gateway option. Which?
2. **The EventStream decoder:** adopt `aws-sdk-go-v2/aws/protocol/eventstream`
   (two new modules, a `vendorHash` change) or accept about 120 lines of our own
   with the SDK's vectors as fixtures? Recommendation: the SDK module.
3. **The stall watchdog.** It is a timer around a model call on every SSE
   parser, including River's stream that the Autoradio desk uses. The Autoradio
   ruling is "no deterministic timers on the turn path". Remove the watchdog
   from streams the desk makes, from all streams, or keep it as the deadline the
   contract allows ("deadlines" stay)? And the agent loop's own
   `providerCallTimeout` and 45 s `requiredNextToolCallTimeout`: do they stay?
4. **Spend on a failed or cancelled stream.** Book an estimate (marked), book
   nothing as now, or book only what the provider reported? Money is the
   owner's.
5. **Texture drafts.** Is a draft of text the committed revision may contradict
   acceptable to show, labelled as a draft? It is the only honest "faster
   output" for Texture, and it is the one place a reader sees model output that
   is not yet a revision.

## 8. What I could not determine

1. **Any production behaviour of a stream.** The journal has no `stream=true`
   call. No latency, truncation or error-line rate of the streamed path is
   known, and none of the claims in 2.3 about the mid-stream paths has been seen
   in a trace.
2. **Which chat-completions providers send usage without `include_usage`.**
   Needs a staging probe per provider (slice c).
3. **Whether Bedrock bearer-token auth works on
   `invoke-with-response-stream`, and the exact `chunk` frame layout.** The API
   reference confirms the path, content type and exception set; the frame body
   and the auth must be confirmed with a real call in slice (a).
4. **Whether the guest to gateway chain (hostlink and the host proxy) passes
   chunks without buffering** at production message sizes. The desk's proof
   script measures first-sentence time at the phone; I did not run it.
5. **How much of a Texture revision's time is in the cell's `choir.Write` text
   against other cell code.** Only two turns on d@e are measured; a sample of
   cells from the tape would size candidate A.
6. **Whether any caller relies on a truncated stream being accepted.** None
   found by search; not exhaustive.
7. **No reference page exists** for the gateway, Bedrock, River or any provider
   (`docs/reference/README.md` lists four unrelated pages). I read the code and
   one AWS page. The plan makes the page part of slice (a).

## 9. Proposed ledger entries

To be entered in `docs/calibration/claude-ledger.md` before the first landing
(this document only proposes them):

| Claim | p | Test | Settle by |
|---|---|---|---|
| Bedrock streamed first token under half the whole-call time on 10 to 35 K token prompts | 0.70 | slice (a) proof, 10 calls | the day (a) lands |
| After (b) no streamed tool turn returns `tool_use` without calls | 0.90 | slice (b) tests plus one staging run | the day (b) lands |
| River, Synthetic, OpenRouter send usage on a stream without `include_usage` | 0.55 | slice (c) probe | the day (c) lands |
| Streamed and non-streamed `CallWithTools` give equal responses on five wires | 0.85 | slice (d) differential test | the day (d) lands |
| Texture draft text appears at least 5 s before commit on a prompt whose first call exceeds 8 s | 0.65 | slice (e) deployed measure | the day (e) lands |

## 10. Index of code cited

Gateway: `internal/gateway/handlers.go` (432, 497, 549, 585, 594, 607, 852),
`paid_budget.go` (261, 374-400), `circuit_breaker.go` (251-275, 281, 365),
`openai_compat.go` (482), `anthropic_relay.go` (699), `cmd/gateway/main.go`
(57), `internal/server/server.go` (111-113), `nix/node-b.nix` (2556).
Providers: `internal/provider/provider.go` (248, 286, 465, 495, 773, 809, 935,
966, 1106, 1308, 1328, 1534, 1737, 2307-2346, 2637, 2680, 2785, 3024, 3252,
3715), `devin.go` (147, 1119, 1205, 1384, 1426, 1463), `river.go` (71-135),
`openrouter.go` (72-157), `quota.go` (174, 243), `stall_reader.go` (24-28).
Catalog and policy: `internal/modelcatalog/catalog.go` (514),
`internal/provideriface/model_policy.go` (13). Guest:
`internal/gatewayruntime/provider.go` (116, 179, 229, 321, 406, 436),
`internal/toolregistry/toolloop.go` (157, 964, 1013, 1088, 1126, 1226, 1383,
1965, 2175), `internal/agentcore/runtime.go` (4000, 4025),
`internal/agentcore/tools_desk.go` (585), `tool_profiles.go` (335). Texture:
`internal/textureowner/texture.go` (1494, 1620-1665),
`texture_observation.go` (307), `texture_cell_author.go` (36, 44),
`internal/texturewrite/grammar.go` (1-30), `frontend/src/packages/texture/lifecycle.js`
(93). Events: `internal/types/event_live_only.go`, `event_durability.go` (28,
41, 50), `internal/events/bus.go` (186). Desk branch
(`origin/lane/autoradio-desk`): `internal/gatewayruntime/text_stream.go`,
`internal/provideriface/text_stream.go`, `internal/agentcore/api_autoradio.go`,
`docs/reference/AUTORADIO-TURNS.md`,
`docs/problems/gateway-stream-error-line-read-as-clean-end-2026-11-11.md`.
External: AWS `InvokeModelWithResponseStream` API reference (fetched
2026-11-11); `github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream` v1.7.27
(module metadata read from the module cache, 2026-11-11).
