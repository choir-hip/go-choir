# River: hosted inference for DeepSeek V4.1 Flash 1017

Reference page (convention: [README.md](README.md)). Written 2026-11-14 from
River's four doc pages (section 8) and our code at `origin/main` 5af04baad.
Code lines drift.

Tags: **[V]** River's own page; **[R]** relayed, unverified; **M** measured by
us; **C** read from our code. The doc pages were fetched through a tool that
summarises with a small model, so every **[V]** figure is from a summary, not
raw text.

**State on 2026-11-14: River is not live on staging.** The key is in the
operator's `.env`, but the gateway's credential file on Node B has no River
entry (checked by variable name, no values read). Section 5 is the procedure
to register it. Until then every River pool member is skipped and the
Autoradio pool answers from its Bedrock fallback, which does not stream.

## 1. What it is

River is an OpenAI-compatible inference host. We use one model on it:

`deepseek-ai/DeepSeek-V4.1-Flash-1017`

Owner, 2026-11-14: "They have other models but I only want this one ...
bc it has 10x better cache pricing." The `-1017` suffix is a dated snapshot;
`deepseek-ai/DeepSeek-V4.1-Flash` is the moving alias. Use the snapshot.

| Fact | Value | Tag |
|---|---|---|
| Base URL | `https://api.river.ai/v1` | [V], and C (`internal/provider/river.go:30`) |
| Auth | `Authorization: Bearer $RIVER_API_KEY` | [V] |
| Context window | 1,048,576 tokens, input plus output | [V] |
| Max output | 393,216 tokens, less whatever the input used | [V] |
| Streaming, reasoning, function calling | all models support all three | [V] |

## 2. Endpoints, and the one we call

| Endpoint | Use | We call it |
|---|---|---|
| `POST /v1/chat/completions` | chat, images, tools, streaming | yes |
| `POST /v1/responses` | Responses API | no |
| `GET /v1/models` | list model ids | no (use it by hand to verify access) |

**We call Chat Completions only** (`internal/provider/river.go:101`). This is
deliberate, and the reasons to keep it:

- The request and stream parsers are the shared OpenAI Chat Completions ones
  (`openAIChatCompletionRequest`, `parseOpenAIChatCompletionsStream`), already
  used for other providers.
- River's Responses API does not support `store`, `background`,
  `previous_response_id` or `conversation` [V]. Full history goes in `input`
  on every call, so it offers no state we would use.
- River's quickstart documents streaming for Chat Completions. The Responses
  page says successful streams end with `response.completed`, so it streams
  too, but our Autoradio path depends on streaming and the Chat path is the
  one probed.

The two APIs name things differently. Do not mix them:

| Thing | Chat Completions | Responses |
|---|---|---|
| Reasoning | `reasoning_effort` | `reasoning.effort` |
| Output cap | `max_completion_tokens` or `max_tokens` | `max_output_tokens` |
| Cached input in `usage` | `prompt_tokens_details.cached_tokens` | `input_tokens_details.cached_tokens` |
| Reasoning tokens in `usage` | `completion_tokens_details.reasoning_tokens` | `output_tokens_details.reasoning_tokens` |
| Structured output | `response_format` | `text.format` |
| `top_p` | 0 to 1 | above 0, up to 1 |

All [V].

## 3. Reasoning

For DeepSeek on River, reasoning is **off by default**, `none` turns it off
explicitly, and enabling thinking with no level runs `high` [V].

What we send (`internal/provider/river.go:140-165`, C):

| Choir effort | Sent as `reasoning_effort` | Why |
|---|---|---|
| request has `ReasoningOff` | `none` | the Autoradio fast turn; the one exemption from "thinking is never off" (owner, 2026-11-11) |
| `low`, and every "off" spelling | `low` | thinking is never off elsewhere (owner, 2026-11-09) |
| `high`, `xhigh`, `max` | `high` | |
| `medium`, or anything else | field omitted | see the mistake below |

**Mistake already made (M, probed 2026-11-09):** `reasoning_effort: "medium"`
together with tools returns 400 `invalid_request`. Omitting the field works
with tools. River's page says DeepSeek accepts `none` through `max` [V]; the
probe disagrees for `medium` with tools. Trust the probe until re-probed.

Because the default is off, an omitted field means **no reasoning**, not
"medium". A role that needs thinking must ask for `low` or `high`.

## 4. Caching and price

- An unchanged prefix "can reuse the cache when available" [V]. There is no
  cache control parameter: no `prompt_cache_key`, no breakpoint [V]. Our
  `CacheBreak` blocks are ignored on River (`internal/provider/reasoning_off.go:19`, C).
- Cached input is billed at the cached rate; there is no cache-write fee [V].
- So the only lever is prefix stability. Keep the system prompt and the tool
  list byte-identical across calls of one role, and put what changes last.
- Send `stream_options: {"include_usage": true}` to get `usage` on a stream;
  it arrives in a final chunk with empty `choices` [V].
- **Possible defect, not yet probed.** Our parser reads
  `prompt_tokens_details.cached_tokens` when a `usage` object arrives
  (`internal/provider/provider.go:1979`, C), but nothing under
  `internal/provider` sends `stream_options` (grep for `include_usage`,
  2026-11-14, no hit). If River sends no `usage` on a stream without that
  option, every streamed River call is costed with no token counts, and the
  daily budget undercounts. The first Autoradio turn through River settles
  this: read its cost line.

Prices in our table (`internal/llmcost/pricing.go:125-130`, C, from
`console.river.ai/api/public/pricing`):

| Model | Input | Cached input | Output |
|---|---|---|---|
| `deepseek-v4.1-flash-1017` | $0.12 | $0.0024 | $0.48 |
| `deepseek-v4.1-flash` | $0.12 | $0.0024 | $0.48 |

Per million tokens. **Unresolved:** the owner says the `-1017` snapshot has
ten times better cache pricing than the alias; our table gives both the same
cached rate. River's pricing table did not load in the fetch on 2026-11-14, so
neither is verified. Re-check the public pricing endpoint before relying on
the alias row.

## 5. Configuring it on the gateway

Only the gateway holds the key. Guests never see it.

### 5.1 What the gateway reads

| Variable | Required | Value | Read at |
|---|---|---|---|
| `RIVER_API_KEY` | yes | the key | `internal/provider/provider.go:3928` |
| `RIVER_BASE_URL` | no | defaults to `https://api.river.ai/v1`; leave unset | `internal/provider/river.go:60` |
| `GATEWAY_RIVER_MODELS` | no | defaults to the 1017 snapshot only; leave unset | `cmd/gateway/main.go:188,215` |
| `GATEWAY_PAID_BUDGET_RIVER_USD` | no | daily dollar budget; default 10 | `internal/gateway/paid_budget.go:59,72` |

River registers only when the key is non-empty **and** the model list is
non-empty. With the defaults, setting the key is the whole configuration.
Leaving `GATEWAY_RIVER_MODELS` unset is what keeps us on the one model.

### 5.2 Registering the key on Node B

`nix/deploy-provider-creds.sh` reads the operator's `.env`, **rewrites the
whole of** `/var/lib/go-choir/gateway-provider.env` on Node B, and restarts
only the gateway. It has forwarded `RIVER_API_KEY` and `RIVER_BASE_URL` since
52d93ddb3 (2026-11-09).

The file on Node B was last written 2026-11-10 17:31Z and has no River line,
although the key was already in `.env` and the script already forwarded it.
The cause was not found. The likeliest is a run from a worktree older than
52d93ddb3, or with `CHOIR_PROVIDER_ENV_FILE` pointing elsewhere.

Procedure. This is an owner act: it spends the owner's money and restarts a
production service.

1. Run from a checkout at current `origin/main`, not an old worktree.
2. Confirm `.env` has a non-empty `RIVER_API_KEY`. Do not print it.
3. Confirm every other provider key the gateway needs is also in that `.env`.
   The script replaces the remote file; a key missing locally is removed
   remotely. The remote file's variable names on 2026-11-14 were:
   `AWS_BEARER_TOKEN_BEDROCK`, `BRAVE_API_KEY`, `CHATGPT_AUTH_PATH` and three
   `CHATGPT_SEARCH_*`, `DEVIN_API_KEY`, `EXA_API_KEY`,
   `GATEWAY_CHATGPT_MODELS`, `GATEWAY_CHATGPT_REASONING_EFFORT`,
   `GATEWAY_JEV_JUDGMENTS_ENABLED`, `GATEWAY_OPENROUTER_MODELS`,
   `GATEWAY_ZAI_MODELS`, `KEENABLE_API_KEY`, `OPENCODE_API_KEY`,
   `OPENROUTER_API_KEY`, `PARALLEL_API_KEY`, `SERPAPI_API_KEY`,
   `SERPER_API_KEY`, `SYNTHETIC_API_KEY`, `TAVILY_API_KEY`, `ZAI_API_KEY`,
   `ZAI_BASE_URL`.
4. `./nix/deploy-provider-creds.sh node-b`
5. Verify, in this order:
   - the remote file now has one `RIVER_API_KEY=` line (count it, do not
     print it);
   - the gateway journal shows `provider: resolved river (seed_model=deepseek-ai/DeepSeek-V4.1-Flash-1017)`;
   - an Autoradio turn against d@e logs
     `provider: river call model=deepseek-ai/DeepSeek-V4.1-Flash-1017 stream=true reasoning=none`
     and the first sentence arrives before the answer is complete.

Rollback: remove the key from `.env` and run the script again, or set
`GATEWAY_PAID_BUDGET_RIVER_USD=0`. Either way the pools fall back to their
next member.

### 5.3 Where River is used once registered

`internal/modelcatalog/catalog.go` (C):

| Pool | River's place |
|---|---|
| Autoradio (fast turn) | first; Bedrock Haiku 5.5 is the fallback |
| default | fallback only, one part in ten of the paid tail |
| self-development | fallback only |

The catalogue entry (`catalog.go:288-298`) declares a 524,288-token window
and 65,536 max output, copied from Synthetic's entry "until River's own is
probed". River documents 1,048,576 and 393,216 [V]. The entry is conservative,
not wrong; raise it only after a probe.

## 6. Limits and errors

| | | Tag |
|---|---|---|
| Rate and concurrency | no published per-key figure; "contact support for capacity commitments" | [V] |
| Timeouts | 10 minutes for prefill, 15 minutes for a non-streaming response | [V] |
| Request size | 50 MiB of JSON, images and history included | [V] |
| Our daily budget | $10 by default, platform-wide, reset at UTC midnight | C |

| Status | River says | We do (`river.go:118-130`) |
|---|---|---|
| 402 | "Check your Console balance." | quota error; the model cools until reset |
| 429 | reduce concurrency, back off with jitter, respect `Retry-After` | if the body names a quota or limit, a quota error; otherwise a short rate limit, and the guest retries the same model |
| 400, 422 | fix the request or the unsupported parameter | plain error |
| 404 | check the model id and endpoint | plain error |

River does not document the 429 body, so our quota-versus-rate split rests on
reading the reason text. "Incomplete streams cannot be resumed; retries are
new requests and may incur additional charges" [V].

## 7. Rules

1. One model: the `-1017` snapshot. Do not add the alias or another River
   model to a pool without an owner ruling.
2. Chat Completions only. A move to Responses is a provider change with its
   own review, not a config change.
3. Omitted `reasoning_effort` means reasoning off. Ask for `low` or `high`
   when thinking is wanted; never send `medium` with tools.
4. Cache by keeping the prefix stable. There is nothing else to set.
5. No limit on our own calls to River beyond the owner's dollar budget and
   backing off after a real refusal (CLAUDE.md, Delivery).
6. The key lives in the operator's `.env` and the gateway's credential file.
   Never in the repo, a log line or a guest.

## 8. Sources

Fetched 2026-11-14, through a summarising tool:

- <https://docs.river.ai/guides/inference/> (quickstart, endpoints, auth,
  streaming, usage on streams)
- <https://docs.river.ai/guides/inference-models/> (model ids, windows, usage
  field names, cached billing; the price table did not load)
- <https://docs.river.ai/guides/inference-controls/> (reasoning, structured
  output, Chat versus Responses)
- <https://docs.river.ai/guides/inference-limits/> (timeouts, size, status
  codes)

Not fetched: the public pricing endpoint our price table cites.
