# OpenCode Go and OpenCode Zen: Provider Integration Research

**Date**: 2026-09-10
**Status**: Research complete. No configuration, credential, or code change has been made.
**Scope**: Prerequisite for mission three (adding the two OpenCode providers to the Choir
gateway). Everything below is either observed live on 2026-09-10, read out of a pinned
third-party source tree, or marked `[INFERENCE]`.
**Mutation class**: green (documentation only).

Sources used:

- Live OpenCode gateway endpoints (`https://opencode.ai/zen/...`), probed 2026-09-10.
- Vendor docs: `https://opencode.ai/docs/zen/`, `https://opencode.ai/docs/go/`.
- OpenCode source: `github.com/anomalyco/opencode` at `193de13a` (dev, 2026-09-10 15:33 -0500).
- OMP source: `github.com/can1357/oh-my-pi` at `3b3a6dc` (dev, v18.1.17).
- `models.dev/api.json`, fetched 2026-09-10.
- Choir source as mapped by two read-only scout passes (citations inline).

---

## 1. Verdict

Six findings decide the shape of this work.

1. **`x-opencode-session` is now hard-required on OpenCode Go and on Zen free-tier models.**
   A request without it returns `HTTP 400 {"type":"error","error":{"type":"MissingSessionID", ...}}`
   — on every Go model, and on every free Zen model ("OpenCode's free tier can only be used in
   OpenCode"). Paid Zen models still answer without it. This is not advisory: the email warning
   was real, enforcement is live, and the adapter must send the header unconditionally (§9.1).
2. **The free tier is real, and it needs no API key at all.** `Authorization: Bearer public`
   plus a free model id reaches Zen anonymously. `muse-spark-1.3-contributor-free` returned
   `HTTP 200` on `/zen/v1/responses` with no credential.
3. **There is no single wire shape.** Chat-completions, Responses, Anthropic Messages, and a
   Gemini-shaped path are all in use across model ids, and each has its own auth header.
   A chat-completions-only adapter can serve a useful subset — but not Muse Spark.
4. **Muse Spark is Responses-only.** `muse-spark-1.3-contributor-free` on
   `/zen/v1/chat/completions` returns `HTTP 500`; the same id on `/zen/v1/responses` returns
   `HTTP 200`. The docs' endpoint table is accurate.
5. **The "Bun fetch" email is about this workstation's OMP harness, not about Choir.**
   Choir's gateway has no OpenCode provider today; the only OpenCode traffic attributable to
   this machine comes from `omp`, which is what the panel skill invokes.
6. **Choir has no place to put a per-conversation session id today.** `LLMRequest`
   (`internal/provider/provider.go:70-105`) carries no session field and `HandleInference`
   (`internal/gateway/handlers.go:386-400`) has only `computerID`. Because Go and free Zen
   cannot be called without one, this is a hard prerequisite rather than a refinement. The
   additive integration path is specified in §10.

---

## 2. What the two providers are

Both are OpenCode-operated gateways over largely the same model families. They differ in
billing, model set, and catalog size, not in protocol.

| | OpenCode Zen (`opencode`) | OpenCode Go (`opencode-go`) |
| --- | --- | --- |
| Billing | Pay-as-you-go credits, charged per request | $10/month subscription with per-model monthly dollar caps |
| Auth | API key from `opencode.ai/auth` | API key from the same console, after subscribing to Go |
| Base URL | `https://opencode.ai/zen/v1` | `https://opencode.ai/zen/go/v1` |
| Live catalog size | 70 ids (observed) | 37 ids (observed) |
| models.dev provider id | `opencode` | `opencode-go` |
| OpenCode config id | `opencode/<model-id>` | `opencode-go/<model-id>` |
| Credential variable (models.dev) | `OPENCODE_API_KEY` | `OPENCODE_API_KEY` |
| Docs | `opencode.ai/docs/zen` | `opencode.ai/docs/go` |

Catalog endpoints are public and unauthenticated (verified `HTTP 200`, no key):

```
GET https://opencode.ai/zen/v1/models        -> 70 ids
GET https://opencode.ai/zen/go/v1/models     -> 37 ids
```

### Billing mechanics that constrain policy

- **Zen auto-reload is a live money risk.** The docs state that if the balance drops below
  $5, Zen automatically reloads $20. With roughly $10 of credits, the first heavy run can
  cross that line and trigger a charge. Workspace and per-member monthly limits exist and
  should be set before wiring Zen.
- **Go usage is capped per model, not per account.** Each model carries a monthly dollar
  cap; the included usage is partitioned 5-hour = 20%, weekly = 50%, monthly = 100%. For
  example DeepSeek V4.1 Flash caps at $15/month and Muse Spark 1.3 Contributor at $60/month
  with token prices $0.10/$0.20 per 1M.
- **Go has a "Use balance" fallback** that spends Zen credits after Go limits are reached.
  Leave it off unless spending credits is intended.
- **Contributor models are a data-policy decision, not a technical one.** Muse Spark
  *Contributor* ids are heavily discounted in exchange for permission to train on prompts
  and completions, are restricted to regions allowed by Meta's geographic policy, and are
  enforced server-side: the Go handler rejects them when the workspace has not accepted
  training (`handler.ts:146-151`). Production or user content must not be routed to them
  without an explicit owner decision.

### Catalog drift is real; treat the live endpoint as authoritative

Observed on 2026-09-10:

- models.dev lists 102 Zen ids; the live endpoint serves 70.
- `hy3-free` is present in models.dev and absent from the live Zen catalog.
- `deepseek-v4-flash-free` is served live but does not appear in the published pricing table.
- `deepseek-v4.1-flash`, `hy4-preview`, `hy3-preview`, `mimo-v2-omni`, `omen-alpha`, and
  `deepseek-flash` are served on Go only.
- Free ids are explicitly "limited time" feedback models and can disappear.

**Side finding (not this mission's scope):** `skills/agentic-consensus/SKILL.md:31-34,235-238`
pins a panel anchor at `opencode-zen/hy3-free`. No `hy3*` id exists in the live Zen catalog,
so that anchor is very likely broken. Worth one confirmation run and a follow-up fix.

---

## 3. Wire protocol map

Route by id, and auth by route. All four shapes exist under both base URLs.

| Shape | Path (Zen / Go) | Auth header | Representative ids |
| --- | --- | --- | --- |
| OpenAI chat completions | `/zen/v1/chat/completions`, `/zen/go/v1/chat/completions` | `Authorization: Bearer <key>` | `deepseek-v4.1-flash`, `deepseek-v4-flash`, `deepseek-v4-pro`, `glm-5.3*`, `glm-5*`, `kimi-k3`, `kimi-k2.7-code`, `kimi-k2.6`, `mimo-v2.5*`, `minimax-m2.x`, `longcat-2.0`, `hy3`, `hy4-preview`, `big-pickle`, most `*-free` ids |
| OpenAI Responses | `/zen/v1/responses`, `/zen/go/v1/responses` | `Authorization: Bearer <key>` | `muse-spark-1.3-contributor`, `muse-spark-1.2-contributor`, `muse-spark-1.3`, `muse-spark-1.3-contributor-free`, `grok-4.6`, `grok-4.5`, `gpt-5.6-luna`, `gpt-5*` |
| Anthropic Messages | `/zen/v1/messages`, `/zen/go/v1/messages` | `x-api-key: <key>` | `claude-*`, `mini*max` (MiniMax), `qwen3.8-max`, `qwen3.7-max`, `qwen3.7-plus`, `qwen3.6-plus` |
| Gemini-shaped | `/zen/v1/models/<model-id>` | `x-goog-api-key: <key>` | `gemini-*` |

Auth headers are read at `packages/console/app/src/routes/zen/v1/chat/completions.ts:9`,
`v1/messages.ts:9`, `v1/models/[model].ts:9`, `v1/responses.ts:9`, and
`go/v1/chat/completions.ts:9`.

**Consequence for mission three.** DeepSeek V4.1 Flash — the model the owner specifically
wants — is chat/completions and fits Choir's existing OpenAI-compatible adapter shape
unchanged. Muse Spark is Responses-shaped and does not.

### Models relevant to mission three

| Model | Provider | Id | Route | Price per 1M (in/out) | Cap |
| --- | --- | --- | --- | --- | --- |
| DeepSeek V4.1 Flash | Go | `deepseek-v4.1-flash` | chat | $0.15/$0.60 off-peak, $0.30/$1.20 peak | $15/mo |
| DeepSeek V4 Flash | Go | `deepseek-v4-flash` | chat | same peak split | $30/mo |
| DeepSeek V4 Flash | Zen | `deepseek-v4-flash` | chat | $0.14/$0.28 | PAYG |
| DeepSeek V4 Flash (free) | Zen | `deepseek-v4-flash-free` | chat | free | — |
| Muse Spark 1.3 Contributor | Go | `muse-spark-1.3-contributor` | **responses** | $0.10/$0.20, cache read $0.002 | $60/mo |
| Muse Spark 1.3 Contributor (free) | Zen | `muse-spark-1.3-contributor-free` | **responses** | free | — |
| GLM 5.3 Flash | Go/Zen | `glm-5.3-flash` | chat | $0.15/$0.50 | $60/mo |
| MiMo V2.5 (free on Zen) | Zen | `mimo-v2.5-free` | chat | free (observed rate-limited) | — |
| Nemotron 3 Ultra (free) | Zen | `nemotron-3-ultra-free` | chat | free | — |

Verified metadata: `deepseek-v4.1-flash` reports 1M context, `reasoning: true`,
`tool_call: true`; `muse-spark-1.3-contributor` reports 1M context, `reasoning: true`,
`tool_call: true`.

DeepSeek peak hours are 01:00-04:00 and 06:00-10:00 UTC Monday-Friday; all other hours and
weekends are off-peak. Off-peak is roughly half price on Go.

---

## 4. Identity headers: what the email meant

### What the vendor requires

From `https://opencode.ai/docs/go/#where-can-i-use-it`, the client must:

1. send typical coding-agent traffic;
2. identify itself with its own user agent (`my-coding-agent/1.0`), not a generic SDK or
   HTTP-library name;
3. send a stable session id in `x-opencode-session` **per conversation**, used for routing
   and prompt-cache affinity.

### Observed enforcement (live, 2026-09-10)

```
POST https://opencode.ai/zen/go/v1/chat/completions
  Authorization: Bearer <go key>, User-Agent: Bun/1.3, no session header
-> HTTP 400
   {"type":"error","error":{"type":"MissingSessionID",
    "message":"Error from provider (Console Go): Request is missing x-opencode-session and
     cannot be routed efficiently. Please see https://opencode.ai/docs/go/#where-can-i-use-it"}}
```

The same request with `User-Agent: choir-research/0.1` and
`x-opencode-session: choir-research-1` returned `HTTP 200` with a normal chat completion.

**The public source lags the deployed service.** At `193de13a` the console handler only logs
the header and falls back to workspace/ip stickiness (`handler.ts:124-127`, `:168`); the
`MissingSessionID` string does not exist in the public tree. Treat the header as mandatory
regardless of what the source says, and never treat a passing local test as evidence that a
later deploy has not tightened this.

### What OpenCode itself sends

`packages/opencode/src/session/llm/request.ts:185-200` attaches, for providers whose id
starts with `opencode`:

```
x-opencode-project: <project id>
x-opencode-session: <session id>
x-opencode-request: <user id>
x-opencode-client:  <client flag>
User-Agent:         opencode/<version>
```

Non-OpenCode providers instead get `x-session-affinity` and `X-Session-Id`.

### What OMP does, and where it deviates

OMP (`github.com/can1357/oh-my-pi`) is the third-party reference the owner pointed at. Its
provider ids and base URLs match models.dev exactly
(`packages/catalog/src/provider-models/openai-compat.ts:7119-7137`):

```ts
openAiCompletionsDescriptor("opencode",    "opencode-zen", "https://opencode.ai/zen/v1", {...})
openAiCompletionsDescriptor("opencode-go", "opencode-go",  "https://opencode.ai/zen/go/v1", {...})
```

Its header logic is one function, `packages/ai/src/providers/inference-headers.ts:44-55`:

```ts
const isOpenCode = options.provider === "opencode-go" || options.provider === "opencode-zen";
const sessionId = options.sessionId;
if (!sessionId) return;                       // no session -> sends nothing
...
if (isOpenCode) {
  setHeaderIfAbsent(headers, "User-Agent", USER_AGENT);
  setHeader(headers, "x-opencode-session", sessionId);
}
```

Two deviations matter for us:

1. **Discovery and usage calls use the installation id, not a conversation id.**
   `openai-compat.ts:3092-3094` and `packages/ai/src/usage/opencode-go.ts:114-118` send
   `x-opencode-session: getInstallId()`; the source comment reads "(x-opencode-session
   required from 09/06) and omp's UA instead of Bun's default". That is a deliberate
   attribution choice for calls that have no conversation context. It satisfies the header
   requirement but defeats per-conversation pinning if used for inference.
2. **When no session id is available, OMP sends no session header at all** (early `return`).
   That is exactly the shape that now returns `MissingSessionID` on Go.

Regression tests pin both behaviors:
`packages/catalog/test/opencode-provider.test.ts:705-720`,
`packages/ai/test/opencode-session-header.test.ts:250-321`,
`packages/ai/test/opencode-go-usage.test.ts:98-99`.

**Lesson for Choir.** Adopt the header, not the identity. The correct value is Choir's own
conversation-scoped identity (see §6). An install-wide constant is an acceptable fallback; a
per-request random value is not — it would defeat sticky routing and read as abuse.

### The "Bun fetch" user agent in the email

The email's listed user agents ("Bun fetch") are generic Bun-runtime fetch calls. Evidence
that this describes the OMP harness on this workstation and not Choir:

- `OPENCODE_GO_API_KEY` is present in this workstation's environment.
- `omp` is installed at `~/.local/bin/omp` (v18.1.17) with state under `~/.omp`.
- `skills/agentic-consensus/SKILL.md` invokes `omp` against `opencode-go/...` and
  `opencode-zen/...` model ids in the default panel.
- `omp models` reports exactly the live catalog sizes (37 Go, 70 Zen), i.e. OMP discovers
  live and is the OpenCode client on this machine.
- OMP's own source comments confirm it was fixed for discovery after the 09/06 requirement,
  which implies a window where it sent Bun's default UA.

`[INFERENCE]` Choir has never called these gateways: no OpenCode provider exists in
`internal/provider`, `internal/modelpolicy`, `internal/gateway`, `internal/llm`, `cmd`, or
`internal/modelcatalog` (verified by exhaustive search). The email is therefore a diagnostic
about the client harness, not about the Choir gateway.

---

## 5. What Choir has to build against

Mapped by two read-only scout passes; all citations verified in-tree.

### Provider registry (Go, not config or database)

| Concern | Location |
| --- | --- |
| `Provider` interface | `internal/provider/provider.go:259-277` — `Call`, `Stream`, `Name`, `IsReal` |
| Runtime-facing interface | `internal/provideriface/provider.go:38-49`; `ToolLoopProvider` from `:51` |
| Registry | `internal/provider/provider.go:3249-3353` (`MultiProvider`), `:3355-3452` (`ResolveAll`) |
| Config struct | `internal/provider/provider.go:279-347` (`ProviderConfig`, model lists per provider) |
| Startup wiring | `cmd/gateway/main.go:28-37`, default model tables `:128-205`, `GATEWAY_*_MODELS` overrides |
| Model-to-provider routing | `internal/modelcatalog/catalog.go:42-190` (`SupportedModels`) |
| Routing tests that enumerate every catalog row | `internal/gateway/gateway_test.go:2635-2735`, fixtures `:1369-1578` |
| Role-to-provider/model selection | `internal/modelpolicy/model_policy.go:87-123` (TOML, re-read per `Resolve`), `:320-396` defaults |
| Policy API | `internal/agentcore/api.go:375-419` (`GET /api/model-policy/resolve`) |

Templates to copy:

- **Chat completions egress**: Fireworks adapter, `internal/provider/provider.go:849-1000`
  (`Call`/`Stream` `:911-975`) — `newJSONRequest`, `Authorization: Bearer`, `Content-Type`,
  `Accept`.
- **Responses egress**: ChatGPT provider, `internal/provider/provider.go:1363+`, request
  `:1467-1492`; `ReasoningEffort` already maps to `reasoning.effort`
  (`internal/provider/provider.go:102-104`). This is the template for Muse Spark.

### Credentials and deployment

- Provider secrets are **not** tracked, not Nix-store, not database. They live in a
  gitignored `.env` / `.envrc.local` locally, and on Node B in
  `/var/lib/go-choir/gateway-provider.env`, loaded via `EnvironmentFile`
  (`nix/node-b.nix:699-721`).
- `nix/deploy-provider-creds.sh` reads the local `.env` plus optional
  `~/.config/go-choir/provider-settings.json`, maps `customModels` entries
  (apiKey/baseUrl/provider/model) to provider env vars (`:92-147`, `:159-200`), writes the
  remote env file atomically over SSH, and restarts the gateway (`:218-232`).
- **Provider env applies at gateway restart only.** No watcher, no hot reload
  (`cmd/gateway/main.go:28-37`, `ResolveAll` runs at startup and registers a provider only
  when its key is present and its model list is non-empty).
- **There is no no-SSH credential mutation path today.** `docs/standing-questions.md:87-95`
  requires diagnosis/lifecycle/acceptance through the product API or a scoped CLI key with
  SSH as break-glass; the joined-runtime review
  (`docs/evidence/continuous-texture-supervision-joined-runtime-review-2026-08-08.md:190-222`)
  confirms provider-secret renewal is still SSH-shaped. This prerequisite therefore needs
  the SSH the owner offered, and that is a known, documented gap rather than a surprise.
- **No provider sets a User-Agent today.** Verified: every egress site sets only
  `Authorization`, `Content-Type`, `Accept`. OpenCode requires a real UA, so this adapter is
  the first egress in the gateway that must identify itself.
- **No spend, cost, or free-tier accounting exists.** Token usage is counted and returned
  (`internal/provider/provider.go:190-194`) but never priced. Rate limiting is per-autoputer
  fixed quota (`internal/gateway/ratelimit.go:9-16`) with a circuit breaker
  (`cmd/gateway/main.go:39-46`). A $10 Go subscription plus free Zen models are new failure
  modes for this gateway: nothing currently stops a runaway loop from exhausting a monthly
  cap.

### The unresolved design point: session identity

`LLMRequest` (`internal/provider/provider.go:70-105`) has `Provider`, `Model`, `System`,
`Messages`, `Tools`, `ToolChoice`, `MaxTokens`, `Stream`, `ReasoningEffort` — and no session
or conversation identity. `HandleInference` (`internal/gateway/handlers.go:386-400`) has
`computerID` but no session id, and the gateway has no inbound session header today.

The candidate identities, the exact additive path, and the compatibility argument are
specified in §10. One rule is fixed here because it is easy to get wrong later: the value must
be stable across the consecutive calls of one conversation, and a per-request random value must
be rejected outright — it would defeat sticky routing and read as abuse.

---

## 6. Recommended shape for mission three

**Phase A — chat-completions providers (low risk, proves the plumbing).**
Register two provider entries over one shared OpenAI-compatible adapter body, differing only
in base URL, key env var, and model list:

- `opencode-zen` → `https://opencode.ai/zen/v1`
- `opencode-go` → `https://opencode.ai/zen/go/v1`

Seed with ids that exercise free, cheap, and paid paths without the Responses dependency:
`deepseek-v4.1-flash`, `deepseek-v4-flash`, `glm-5.3-flash`, `deepseek-v4-flash-free`,
`mimo-v2.5-free`, `ling-3.0-flash-fin-free`, `nemotron-3-ultra-free`, `big-pickle`.
Send `User-Agent: choir-gateway/<version>` and `x-opencode-session: <session id>` on every
request; fail closed if the session id is empty rather than sending the request, so the
failure is ours and visible instead of a 400 from upstream.

**Phase B — Responses path.**
Muse Spark 1.3 Contributor (`opencode-go`) and its free Zen twin require `/responses`. Reuse
the ChatGPT Responses codec shape. Gate it on the owner's training-consent decision and on
the Go workspace's region configuration.

**Deferred.** Anthropic Messages ids (`x-api-key`, different body shape) and Gemini-shaped
ids. Neither is needed for mission three.

**Cost guardrails to land with Phase A** (Choir has none today):

- Set Zen workspace monthly limits and **disable Zen auto-reload** before the first real call.
- Decide and record whether Go's "Use balance" fallback stays off.
- Add a per-provider spend or request ceiling for OpenCode routes, or explicitly record that
  the gateway has none and the monthly caps are the only backstop.

---

## 7. Verification plan (to run after mission two closes)

1. **Unit.** Extend the catalog table and the exhaustive routing test
   (`internal/gateway/gateway_test.go:2635-2735`) with the new model ids; add adapter tests
   for header presence, error shape, and credential non-leakage (VAL-GATEWAY-007).
2. **Wire-format rehearsal (local, not proof).** One call per provider per route shape
   against the live endpoints, asserting the session header and UA are present on the wire.
3. **Deployed acceptance (staging, the real proof).** Populate the local gitignored secret,
   run `nix/deploy-provider-creds.sh`, confirm `/health` lists the providers, run one gateway
   inference per provider through the product path, confirm
   `GET /api/model-policy/resolve` returns the new provider/model selection, and publish a
   fetched artifact under `docs/evidence/`.
4. **Negative probes.** A request with the session header stripped must be refused by our own
   adapter (not by upstream); an unknown model id must return a clean structured error.

---

## 8. Initial probe log (round 1; the full matrix is §9)

```
GET  https://opencode.ai/zen/v1/models                 -> 200, 70 ids, no auth
GET  https://opencode.ai/zen/go/v1/models              -> 200, 37 ids, no auth

POST /zen/v1/chat/completions  model=deepseek-v4-flash-free  Bearer public, no session
                                                       -> 400 "Model is unavailable."
POST /zen/v1/chat/completions  model=mimo-v2.5-free    Bearer public, session+UA
                                                       -> 429 FreeUsageLimitError
POST /zen/v1/responses         model=muse-spark-1.3-contributor-free  Bearer public
                                                       -> 200 (status incomplete, max_output_tokens=16)
POST /zen/v1/chat/completions  model=muse-spark-1.3-contributor-free  Bearer public
                                                       -> 500 "Internal server error"
POST /zen/go/v1/chat/completions model=deepseek-v4.1-flash  Bearer <go key>, session+UA
                                                       -> 200 chat.completion, model echoed
POST /zen/go/v1/chat/completions model=deepseek-v4.1-flash  Bearer <go key>, Bun UA, no session
                                                       -> 400 MissingSessionID
```

Choir source checks:

```
grep -r "opencode" internal/provider internal/modelpolicy internal/gateway internal/llm cmd
  -> no provider or model-id hits
grep -r "User-Agent" internal/provider
  -> no matches
```

---

## 9. Live conformance matrix (2026-09-10)

Every probe below used `OPENCODE_API_KEY` from the repo's gitignored `.env`,
`x-opencode-client: choir`, `User-Agent: choir-research/0.1`,
`x-opencode-session: choir-research-probe-2026-09-10`, and a 4-token reply budget
(`max_tokens: 4` for chat/messages, `max_output_tokens: 24-32` for responses). Thirty-three
probes total. No key value was printed or written to any file.

### 9.1 Session-header and credential requirement — the decisive rule

| Surface | Free models | Paid models |
| --- | --- | --- |
| Zen `/zen/v1` | **header required**: without it `400 MissingSessionID` — "OpenCode's free tier can only be used in OpenCode" | header optional: paid chat completed with no session header |
| Go `/zen/go/v1` | no free models are served | **header required**: without it `400 MissingSessionID` |

| Credential used | Zen free | Zen paid | Go paid |
| --- | --- | --- | --- |
| `OPENCODE_API_KEY` (workspace key) | `200` | `200` | `200` |
| `Bearer public` (anonymous) | `200` | `401 AuthError: Missing API key.` | `401 AuthError: Missing API key.` |

Three consequences, all verified:

1. **One secret covers both providers.** The same `.env` value authenticated all nine Go probes
   and every Zen probe. models.dev lists `OPENCODE_API_KEY` as the env var for both provider ids,
   which the probes confirm. No second key is needed.
2. **The header is unconditional.** It is required on every Go request and every free Zen
   request, so the adapter sends it always rather than branching per provider. Sending it on paid
   Zen calls costs nothing.
3. **The free-tier gate keys off header presence, not a client-name allowlist.** Requests
   carrying `x-opencode-client: choir` were accepted, so a third-party client with a session id
   is permitted — anything else would have broken the OMP and Hermes integrations.

### 9.2 Model matrix

Free models — the complete live Zen free set (8 ids):

| Provider | Model | Route | Result | Latency | Observation |
| --- | --- | --- | --- | --- | --- |
| Zen | `ling-3.0-flash-fin-free` | chat | `200` | 1.0s | clean |
| Zen | `nemotron-3-ultra-free` | chat | `200` | **96.7s** | unusable latency (§9.3) |
| Zen | `nemotron-3.5-lightning-free` | chat | `200` | **102.8s** | unusable latency (§9.3) |
| Zen | `muse-spark-1.3-contributor-free` | responses | `200` | 1.4s | clean, 1M context |
| Zen | `muse-spark-1.2-contributor-free` | responses | `200` | 1.2s | clean |
| Zen | `big-pickle` | chat | `429` | 0.2s | `FreeUsageLimitError`, 3 attempts |
| Zen | `mimo-v2.5-free` | chat | `429` | 0.2s | `FreeUsageLimitError`, 3 attempts |
| Zen | `deepseek-v4-flash-free` | chat | `400` | 0.2s | "Model is unavailable." 3 attempts |

Paid models under $0.65 per 1M output, latest of each family:

| Provider | Model | Route | Out $/1M | Result | Latency | Observation |
| --- | --- | --- | --- | --- | --- | --- |
| Zen | `deepseek-v4-flash` | chat | 0.28 | `200` | 3.5s | content empty, all 4 tokens went to reasoning |
| Zen | `deepseek-v4-flash-vision-exp` | chat | 0.28 | `200` | 0.9s | vision variant |
| Zen | `glm-5.3-flash` | chat | 0.50 | `200` | 1.1s | |
| Zen | `gpt-5-nano` | responses | 0.40 | `200` | 1.1s | `output_tokens: 0` under a 32-token cap |
| Go | `deepseek-v4.1-flash` | chat | 0.60 | `200` | 1.3s | released 2026-09-10; the newest DeepSeek flash |
| Go | `deepseek-v4-flash` | chat | 0.60 | `200` | 1.7s | |
| Go | `deepseek-v4-flash-vision-exp` | chat | 0.60 | `200` | 1.6s | |
| Go | `glm-5.3-flash` | chat | 0.50 | `200` | 0.7s | fastest measured |
| Go | `mimo-v2.5` | chat | 0.28 | `200` | 1.2s | billed 254 prompt tokens for 19 (see §9.3) |
| Go | `hy3` | chat | 0.58 | `200` | 1.6s | |
| Go | `qwen3.8-flash` | messages | 0.47 | `200` | 1.1s | Anthropic shape, `x-api-key` |
| Go | `muse-spark-1.3-contributor` | responses | 0.20 | `200` | 1.0s | training consent already enabled on this workspace |
| Go | `muse-spark-1.2-contributor` | responses | 0.20 | `200` | 1.0s | same |
| Go | `deepseek-flash` | chat / responses | n/a | `200` / `200` | 1.2s / 1.4s | live-only id, absent from models.dev |
| Go | `hy3-preview` | chat / responses | n/a | `400` / `500` | — | listed but not serving |

### 9.3 Operational findings

- **Free-tier latency is not interactive.** The two Nemotron free ids took 96.7s and 102.8s for
  a 4-token reply while reporting only 5 reasoning tokens each — this is upstream queueing on the
  free pool, not thinking time. Every other model answered in 0.7-5.2s.
- **Two free ids are already rate-limited from this environment.** `big-pickle` and
  `mimo-v2.5-free` returned `429 FreeUsageLimitError` on three attempts spread over roughly ten
  minutes. Free access is per-IP/per-workspace limited and cannot be treated as dependable
  capacity.
- **One live free id does not serve at all.** `deepseek-v4-flash-free` returned
  `400 "Model is unavailable."` on three attempts while its paid twin answered normally. It is
  listed and unserved — a vendor-side condition, not a credential problem.
- **Reasoning-first models return empty content under a small cap.** `zen/deepseek-v4-flash` with
  `max_tokens: 4` spent all four on `reasoning_tokens` and returned `content: ""`.
  `zen/gpt-5-nano` returned `output_tokens: 0`. `[INFERENCE]` Any Choir probe that asserts
  non-empty content must budget for reasoning tokens, and the adapter's reasoning-effort mapping
  decides whether `content` is ever populated.
- **Metering differs by surface for identical text.** Zen billed `deepseek-v4-flash` 11 prompt
  tokens where Go billed 37 for the same request; Go's `mimo-v2.5` billed 254. Cost accounting
  cannot assume one tokenizer, and the model policy's max-token budgets should be generous enough
  for the worst observed expansion.
- **Both Muse Spark contributor ids served on Go**, which means the training-consent acceptance
  for this workspace is already in place. The open question is therefore only whether Choir
  traffic may use them, not whether it can.
- **The Anthropic-shaped surface works with the same key** (`qwen3.8-flash` via `/messages` with
  `x-api-key`), so a future phase can add it without new credentials.

## 10. Conversation identity: where to integrate (prep, no code)

Design research only; no code, config, or credential change. Two read-only passes mapped the
runtime→gateway path and the provider-metadata precedents.

### 10.1 What crosses the wire today

| Hop | Struct | Identity present | Where it is dropped |
| --- | --- | --- | --- |
| Run admission | `types.RunRecord` (`internal/types/task.go:89-157`) | `RunID`, `AgentID`, `ChannelID`, `TrajectoryID`, `OwnerID`, `ComputerID`, open `Metadata map[string]any` | — |
| Tool loop | `ToolLoopRequest` (`internal/provideriface/provider.go:70-94`) | none | built each iteration at `internal/toolregistry/toolloop.go:399-433` |
| Runtime provider | `gatewayruntime` `llmRequest` (`internal/gatewayruntime/provider.go:359-376`) | none; reads only policy metadata keys | `provider.go:59-138` |
| HTTP call | `POST /provider/v1/inference` | only `Content-Type`, `Authorization`, `Accept` (`provider.go:190-217`) | identity never marshalled |
| Gateway handler | `ProviderRequest` (`internal/gateway/handlers.go:32-83`) | none; `computerID` exists only in the auth path and logs (`handlers.go:343-358`) | `handlers.go:386-400` |
| Provider adapter | `provider.LLMRequest` (`internal/provider/provider.go:69-105`) | none | adapters set only auth/content headers |

Nothing in `task.Metadata` reaches the gateway HTTP body today. The model-policy keys
(`internal/modelpolicy/model_policy.go:18-27`: `llm_provider`, `llm_model`,
`llm_reasoning_effort`, `llm_max_tokens`, `llm_policy_source`, `llm_policy_error`,
`llm_policy_overlay_id`) are consumed runtime-side by `provideriface` and never forwarded.

### 10.2 Candidate identities

| Candidate | Durable? | Unique per conversation? | Verdict |
| --- | --- | --- | --- |
| `RunRecord.RunID` (`task.go:89-97`) | yes, persisted for the record's lifetime | per run; a rewarm starts a new run | **recommended**, with the rewarm caveat (§10.7) |
| `TrajectoryID` (`task.go:103-115`) | yes | spans related runs and agents; may be absent | too broad — would merge distinct conversations |
| `ChannelID` (`task.go:98-101`) | yes | shared coordination surface across runs/agents | wrong axis — coordination, not conversation |
| `ComputerID` (`task.go:132-137`) | yes | one per computer | fallback only when no run id exists |
| RLM `activationID` (`internal/yaegikernel/choir.go:44-62`) | no — in-memory worker scope | per activation | unusable as a cache key; not visible at the gateway |
| Browser `SessionID` (`internal/types/browser.go:19-33`) | yes | unrelated substrate | out of scope |

`RunID` is also the key of durable provider-facing run memory
(`internal/store/run_memory.go:229-256`), which makes it the closest thing Choir has to a
provider conversation key.

### 10.3 Recommended additive path

One provider-neutral optional field, copied explicitly at each hop. Name: `conversation_id`.

| Step | File | Change |
| --- | --- | --- |
| 1 | `internal/provideriface/provider.go:70-94` | add `ConversationID string` to `ToolLoopRequest` |
| 2 | `internal/toolregistry/toolloop.go:399-433` | populate it from the run identity available in the execution context (`internal/agentcore/runtime.go:3278-3307`) |
| 3 | `internal/provider/provider.go:69-105` | add `ConversationID string \`json:"conversation_id,omitempty"\`` to `LLMRequest` |
| 4 | `internal/provider/bridge.go:200-211,509-522` | carry the field through the tool-loop→provider bridge |
| 5 | `internal/gatewayruntime/provider.go:59-138,359-376` | set it from `task.RunID` on the Execution path and from the tool-loop request on the tool path |
| 6 | `internal/gateway/handlers.go:32-83,386-400` | add `conversation_id` to `ProviderRequest`, copy into `LLMRequest` |
| 7 | `internal/gateway/client.go:63-84,177-203` | marshal it in both `Call` and `Stream` |
| 8 | `internal/gateway/openai_compat.go:141-156` | copy it if the inbound compatibility route should carry it |
| 9 | OpenCode adapter (new) | translate to `x-opencode-session`, set the client User-Agent |

Provider-specific parts stay in the adapter: header name, header value normalization
(trim, length cap, character safety — `encoding/json` will happily carry a newline), and the
User-Agent constant (`choir-gateway/<version>`). Nothing provider-named goes into shared
structs, and no routing decision reads the field.

### 10.4 Compatibility

- The gateway inference decoder is `json.NewDecoder(r.Body).Decode` with no
  `DisallowUnknownFields` (`internal/gateway/handlers.go:364-368`), so **version skew is safe in
  both directions**: a new autoputer sending `conversation_id` to an old gateway is ignored, and
  an old autoputer sending nothing to a new gateway yields the empty string, which the adapter
  treats as absent.
- The strict decoders in the repo are on different surfaces — the internal run API envelope
  (`internal/agentcore/api.go:534-538`) and the channel cast (`:735-743`) — and are untouched by
  this field.
- Existing tests observe the request through capture stubs rather than exact-body equality
  (`internal/gateway/gateway_test.go:284-307`, `internal/gatewayruntime/provider_test.go:15-74`),
  so an extra optional field does not break them. They should nonetheless be extended to assert
  the value is copied, since a silently dropped id is exactly the failure this adapter cannot
  detect at runtime: the call still succeeds, just without cache affinity.

### 10.5 Constraints from doctrine

- `docs/agent-product-doctrine.md:212-224` — keep provider call semantics uniform; prefer policy
  and product state over role branches. A single provider-neutral field with adapter-local
  translation is the conforming shape; a per-provider field on `LLMRequest` is not.
- `docs/agent-product-doctrine.md:279-309` — provider secrets and catalogs are platform-owned,
  while model selection policy is computer-owned. The OpenCode header/UA constants are
  platform-owned adapter details and must not become policy.
- `docs/choir-doctrine.md:782-787` — the harness owns provider choice and the gateway holds
  credentials host-side. The conversation id is request metadata, not a credential, and must not
  become a trust input.
- `AGENTS.md:85-99` — provider/model routing is `orange` and gateway/provider calls are `red`;
  threading a request field through the gateway is red-ceremony work regardless of how small the
  diff is.
- There is no existing per-provider header mechanism in `internal/provider`; every adapter sets
  headers directly (`provider.go:463-471`, `:579-583`, `:762-765`, `:927-929`, `:1081-1083`,
  `:1253-1255`, `:1488-1489`) and none sets a User-Agent. Introducing a generic header-modifier
  map would be a new abstraction for one consumer — do not.

### 10.6 What must not change

- Routing and policy resolution: `modelcatalog.SupportedModels`, `resolveProvider`, and the
  model-policy TOML semantics are untouched; the conversation id is opaque to all of them.
- The credential boundary: the id never carries secrets, and the gateway continues to inject
  credentials host-side.
- Core vocabulary: no new event kinds, trajectory fields, or durable trace keys. If the id must
  ever be *recorded* (rather than sent), that is a separate, reviewed decision.

### 10.7 Open question: rewarm

Actor rewarm seeds a new run from prior run memory (`internal/agentcore/run_memory.go:66-131`), so
a conversation that survives a rewarm changes `RunID` and therefore loses cache affinity at that
boundary. `[INFERENCE]` For OpenCode's prompt caching this is a single cache miss per rewarm,
which is acceptable for a first cut. If it is not, the correct fix is a conversation identity
owned by the run-memory lineage, not a wider id such as `TrajectoryID` — and that is a larger
change than mission three should carry. The alternative fallback is `computer-<ComputerID>`,
which is stable but pins every conversation of a computer to one route.

## 11. Open questions for the owner

1. **Training consent.** Are Muse Spark *Contributor* models permitted for any Choir traffic?
   If not, Phase B serves only the free Zen twin (also contributor-consented) or nothing.
2. **Money.** Disable Zen auto-reload and set monthly limits before wiring? (Recommended.)
3. **Session identity.** Approve plumbing a real session id through `LLMRequest` (option 1 in
   §5), or accept the per-computer fallback for the first cut?
4. **Free-keyless Zen.** Should the gateway be allowed to use `Authorization: Bearer public`
   at all, or must every call carry a real key for attribution and quota accounting?
5. **Catalog authority.** Pin a curated id list in the Go catalog (deterministic, stale
   eventually) or discover the live catalog at build/startup (fresh, non-deterministic)?
   `[INFERENCE]` recommend the pinned list plus a periodic drift check, because routing is a
   red surface and should not change without a commit.
6. **Go region requirement.** The deployed handler currently rejects `deepseek-v4-flash` and
   `deepseek-v4-pro` on Go unless the workspace region includes `cn`
   (`handler.ts:158-166`). Verify this against the owner's workspace before depending on
   DeepSeek through Go rather than Zen.
7. **Dependence on free tiers.** `big-pickle` and `mimo-v2.5-free` are already rate-limited from
   this environment and `deepseek-v4-flash-free` is listed but not serving. Should any Choir
   route depend on a free model, or are free models research- and panel-only?
8. **Conversation scope.** Is one `conversation_id` per run enough (§10.7), or must it survive
   actor rewarm, which starts a new run and would therefore change the id?
