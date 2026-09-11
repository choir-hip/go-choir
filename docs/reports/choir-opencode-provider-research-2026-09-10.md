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
   additive integration path is specified in §10, and the recommended value is `RunID`, which
   survives both rewarm and the agent-to-agent callback arc.

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

The table maps the protocol surface, not the roster: which of these ids mission three actually
uses is fixed in §11. Auth headers are read at
`packages/console/app/src/routes/zen/v1/chat/completions.ts:9`,
`v1/messages.ts:9`, `v1/models/[model].ts:9`, `v1/responses.ts:9`, and
`go/v1/chat/completions.ts:9`.

**Consequence for mission three.** DeepSeek V4.1 Flash — the model the owner specifically
wants — is chat/completions and fits Choir's existing OpenAI-compatible adapter shape
unchanged. Muse Spark is Responses-shaped and does not.

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

Seed Phase A from the §11 roster's chat-shaped ids only: `deepseek-v4.1-flash`, `glm-5.3-flash`,
`mimo-v2.5`, `hy3` on Go, and `ling-3.0-flash-fin-free`, `nemotron-3-ultra-free`,
`nemotron-3.5-lightning-free` on Zen.
Send `User-Agent: choir-gateway/<version>` and `x-opencode-session: <session id>` on every
request; fail closed if the session id is empty rather than sending the request, so the
failure is ours and visible instead of a 400 from upstream.

**Phase B — Responses path.**
Muse Spark 1.3 Contributor (`opencode-go`) and its free Zen twin require `/responses`. Reuse
the ChatGPT Responses codec shape. Gate it on the owner's training-consent decision and on
the Go workspace's region configuration.

**Phase C — Messages path.** `qwen3.8-flash` uses the Anthropic shape with `x-api-key`. It is the
only Alibaba entry in the roster, so it is worth including once Phase A's plumbing is proven.
Gemini-shaped ids remain deferred; nothing in the roster needs them.

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

Note while mapping this path: `types.InboxDelivery` and its store schema and writer exist
(`internal/types/task.go:523-548`, `internal/store/store.go:239-257`,
`internal/store/graph_store.go:1859-1883`) but nothing calls `CreateInboxDelivery`. The RLM
"mailbox" in use is the channel log plus the per-run cursor, not an inbox-delivery row.

Nothing in `task.Metadata` reaches the gateway HTTP body today. The model-policy keys
(`internal/modelpolicy/model_policy.go:18-27`: `llm_provider`, `llm_model`,
`llm_reasoning_effort`, `llm_max_tokens`, `llm_policy_source`, `llm_policy_error`,
`llm_policy_overlay_id`) are consumed runtime-side by `provideriface` and never forwarded.

### 10.2 Candidate identities (verified against the actor path)

The earlier draft of this section assumed rewarm mints a new run. That is wrong, and the
correction is what makes `RunID` the right choice.

| Candidate | Stable across park/resume? | Stable across rewarm? | Stable across A→B→A? | Verdict |
| --- | --- | --- | --- | --- |
| `RunRecord.RunID` | yes | yes | yes, per agent | **recommended** |
| `TrajectoryID` | yes | yes | yes | rejected — deliberately spans agents and workers |
| `ChannelID` | yes | yes | yes | rejected — coordination surface, spans run families |
| `AgentID` / actor mailbox | yes | yes | yes | rejected — merges an agent's concurrent runs |
| `ComputerID` | yes | yes | yes | fallback only |
| RLM `activationID` | in-memory only | no | no | unusable as a cache key |

Evidence for the `RunID` row:

- **Park/resume keeps the run.** `resumeState` stores only `{RunID, Phase}` in the actor memory
  snapshot (`internal/actorruntime/handler.go:20-27`); on a coagent result the handler decodes it,
  loads that exact run, sets `actor_reactivate_existing_memory`, and calls
  `ExecuteActivationSync` on the same record (`handler.go:335-433`). A new run is minted only when
  there is no parked pointer *and* the run is terminally complete (`handler.go:340-350` →
  `reconcileCoagentWake`).
- **Channel mail never mints a run.** `handleChannelMessage` documents this outright — "unlike
  coagent_result, this kind never mints a new run: spawn/assign create runs, channel mail only
  wakes an existing activation" — and resumes `rs.RunID` in place (`handler.go:99-145`).
- **Children get their own id, by design.** `StartCoagentRun` mints a fresh uuid `RunID` for the
  child, sets `RequestedByRunID` to the requester, and inherits the trajectory
  (`internal/agentcore/runtime.go:947-1000`, `:1059-1133`). B's conversation is B's; A's remains A's.
- **Run memory is keyed by run.** `ListRunMemoryEntries(ctx, ownerID, runID)`
  (`internal/agentcore/run_memory.go:~52-58`) is exactly the provider-facing conversation history,
  so `RunID` is already the key of the thing being cached.

Where the id does change — a replacement run after a terminal reconcile, self-development
passivation, or a new child — the prompt is rebuilt from a compaction snapshot
(`seedActorMemorySnapshot`, `internal/agentcore/run_memory.go:66-131`, which writes an
`actor_rewarm` summary carrying `source_loop_id`). The prefix is therefore different by
construction, so a cache miss there is inherent to the prompt, not caused by the id choice.

Rejected alternatives matter because each is superficially attractive:

- `TrajectoryID` is the only durable identity that spans the whole A→B→A arc
  (`internal/types/trajectory.go:45-51`, carried on channel messages and worker updates), and it
  is inherited by children. That is precisely why it must not be the session id: it would merge
  agents and workflow branches onto one route.
- `AgentID` and the actor mailbox identity (`owner\x00computer\x00agent`) are stable across
  everything but are per-agent, not per-conversation, so two concurrent runs of one agent would
  share a session.
- `ChannelID` is a shared coordination surface across runs, and `activationID` lives only in the
  capsule worker's memory.

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

### 10.7 Resolved: the agent-to-agent arc does not break affinity

The concern that motivated this section — A messages B, waits, receives a callback, and comes
back as a *different* conversation — does not happen in the normal path:

```
A running (RunID=A)  ->  sends to B            ->  A parks: memory = {RunID: A, phase: parked}
B running (RunID=B)  ->  channel mail to A     ->  A resumes RunID=A on the same RunRecord
```

Both directions are the same-record path, verified above. Two caveats, both accepted:

- A **replacement** run (A's run went terminal before the reply, or self-development passivation
  replaced it) gets a new `RunID`, and its prompt is rebuilt from a compaction summary, so the
  prefix differs anyway.
- A **cold actor with no snapshot pointer** does not admit a run from channel mail at all
  (`handler.go:108-121`), so there is no conversation to keep warm in that case.

If a future requirement demands a durable conversation identity that survives replacement runs —
for example so the cache mission can reason about hit rate across a rewrite — the shape is a
lineage id created at conversation admission and inherited by replacement and child runs, linked
to the run-memory lineage rather than to the trajectory. That is a new durable vocabulary item
with real ceremony, and it belongs to the cache-optimization mission, not to mission three.

Superseded text: an earlier draft of this report claimed rewarm "seeds a new run from prior run
memory" and therefore loses affinity. The new-run path exists, but it is the replacement path,
not the ordinary coagent wait.

## 11. Model roster for mission three (diverse by design)

Owner pruning applied 2026-09-10: Muse Spark 1.2 is out, only DeepSeek V4.1 is kept from the V4
family, GPT-5 Nano is out, and every id that returned `429` or is listed-but-unserved is out. The
evidence for those exclusions remains in §9.2 — the roster below is what mission three should
define against. Every id in it was call-verified on 2026-09-10.

### Tier F — free, for testing (Zen)

| Family | Id | Route | Context | Image | Measured |
| --- | --- | --- | --- | --- | --- |
| Meta | `muse-spark-1.3-contributor-free` | responses | 1M | **yes** | 1.4s |
| Ling (Ant) | `ling-3.0-flash-fin-free` | chat | 262k | no | 1.0s |
| NVIDIA | `nemotron-3-ultra-free` | chat | 1M | no | 96.7s (pool latency) |
| NVIDIA | `nemotron-3.5-lightning-free` | chat | 262k | no | 102.8s (pool latency) |

Free chat coverage is Ling plus the two Nemotron ids, and both Nemotron ids run at roughly 100
seconds on the free pool. For interactive free testing the practical set is therefore Meta
(Responses) and Ling (chat).

### Tier C — cheap paid, under $0.65 per 1M output

| Family | Provider | Id | Route | Context | Image | In/Out $ | Go cap | Measured |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| DeepSeek | Go | `deepseek-v4.1-flash` | chat | 1M | **yes** | 0.15/0.60 | $15/mo | 1.3s |
| Zhipu | Go | `glm-5.3-flash` | chat | 1M | **yes** | 0.15/0.50 | $60/mo | 0.7s |
| Alibaba | Go | `qwen3.8-flash` | messages | 1M | **yes** | 0.15/0.47 | $30/mo | 1.1s |
| Xiaomi | Go | `mimo-v2.5` | chat | 1M | **yes** | 0.14/0.28 | $60/mo | 1.2s |
| Tencent | Go | `hy3` | chat | 256k | no (text only) | 0.14/0.58 | $60/mo | 1.6s |
| Meta | Go | `muse-spark-1.3-contributor` | responses | 1M | **yes** | 0.10/0.20 | $60/mo | 1.0s |

Eight vendor families across three wire shapes — chat completions, Responses, and Anthropic
Messages. DeepSeek V4.1 Flash prices off-peak $0.15/$0.60 and peak $0.30/$1.20; peak hours are
01:00-04:00 and 06:00-10:00 UTC Monday-Friday, all other hours and weekends off-peak.
`deepseek-v4.1-flash` and `muse-spark-1.3-contributor` both report 1M context, `reasoning: true`,
`tool_call: true`.

**Image input verified, not assumed.** On 2026-09-10 a 7-segment PNG rendering the number 427 was
sent to each id over its own route, asking for the digits. `deepseek-v4.1-flash`,
`glm-5.3-flash`, `mimo-v2.5` (chat), `qwen3.8-flash` (messages) and
`muse-spark-1.3-contributor` plus its free Zen twin (responses) all returned `427`. `hy3`
returned `400` with an upstream "No endpoint" error, matching its text-only metadata. So the
owner's expectation holds for five of six cheap paid ids, with `hy3` the exception.
Method note: a first attempt using colour identification was discarded — a no-image control
showed `mimo-v2.5` answering "Red" regardless, and `qwen3.8-flash` answering "White" with no
image at all. Only the non-guessable digit test is evidence.

### Excluded (owner decision 2026-09-10)

| Id | Reason |
| --- | --- |
| `muse-spark-1.2-contributor`, `muse-spark-1.2-contributor-free` | superseded by 1.3 |
| `deepseek-v4-flash` (Go and Zen) | only V4.1 is kept from this family |
| `deepseek-v4-flash-vision-exp` (Go and Zen) | same family rule; vision remains covered by five roster ids, so nothing capability-wise is lost |
| `deepseek-v4-flash-free` | listed, not serving: `400 Model is unavailable.` on three attempts |
| `gpt-5-nano` | owner: old |
| `mimo-v2.5-free` | `429` on three attempts |
| `big-pickle` | `429` on three attempts |
| `hy3-preview` | listed, not serving: `400` on chat, `500` on responses |

### Anti-overfit notes

- The reason to spread across families is not cost alone. A prompt or harness shaped around one
  model's output conventions (the earlier Go-code markdown episode) silently encodes that model's
  quirks. Per-model measurement, not a single shared prompt, is what the mission should record.
- Both contributor-consent ids served on this workspace, but they are training-consented models:
  the owner decision in §13 item 1 governs whether they carry anything but test traffic.
- Free-tier ids are testing capacity only. Two of eight were already rate-limited and one was
  listed-but-unserved before pruning, so no Choir route should depend on a free id.

### Cache-affinity requirements to record now (owned by the later cache mission)

1. **Session identity must be stable across a conversation.** Satisfied by `RunID` (§10.2) once
   the plumbing in §10.3 lands.
2. **Prompt prefix must be byte-stable turn to turn,** or affinity buys nothing. Partially
   audited: tool ordering is sorted (`internal/toolregistry/toolregistry.go:123`,
   `toolloop.go:1533`), and run memory is append-only, so the tool block and history prefix look
   deterministic. The unaudited surface is whether the system prompt and any injected metadata are
   byte-identical across turns — worth one measurement, not an assumption.
3. **Compaction and replacement runs change the prefix by construction** (§10.7). Any hit-rate
   target must exclude those boundaries or the measurement will look like a routing failure.
4. **Cache reads are the economics.** DeepSeek V4.1 Flash on Go is $0.30/1M fresh input at peak
   and $0.006/1M cached read — a ~50x delta — and the vendor's own usage estimates assume
   ~71,300 cached tokens per request. Hit rate is a first-class cost lever, not a tuning detail.
5. **Reasoning tokens count against the reply budget.** Observed: a 4-token cap was consumed
   entirely by reasoning, returning empty `content`. Budgets must be set for thinking models or
   the output is silently empty.

## 12. The one-prompt invariant (owner-stated, 2026-09-10)

Mission three's goal as stated by the owner: **one prompt that works for all models.** "Prompt"
means the explanation of the RLM environment's affordances plus the initial configuration of the
RLM REPL variables. It may vary by desk, never by model. Two consequences are binding:

1. **No code-level parsing workarounds.** Markdown stripping and similar output repair are not
   acceptable as per-model accommodations. This is an invariant, not a preference.
2. **One clean context initialization per desk.** Each desk gets a single affordance text and a
   single REPL initialization, shared by every model that serves that desk.

### 12.1 The seam already exists, and nothing in it is model-conditional

- Final system prompt assembly is `systemPromptForRun`
  (`internal/agentcore/tool_profiles.go:195-302`), in this order: core prompt
  (`promptstore/store.go:51-59`, shared context at `defaults/core.yaml:6-13`); temporal grounding
  (`tool_profiles.go:222-225`); role-specific instructions, owner override or `defaults/<role>.md`
  (`promptstore/store.go:81-109`); optional skill context, gated to Conductor/Texture/Management/
  Engineering (`agentcore/skill_context.go:48-57`); then the actuator overlay — for RLM the branch
  is `capsule.HostSelectsRLM` at `tool_profiles.go:262-266`; then the assignment tail
  (`:268-282`); then the run-context tail (`:294-302`). The tool catalog is appended afterwards by
  `RunToolLoop` → `BuildSystemPrompt`
  (`internal/toolregistry/toolloop.go:303-311`, catalog rendering at `toolregistry.go:172-200`),
  and the user's objective is not part of the system string at all — it is the initial user
  message (`agentcore/runtime.go:3278-3286`).
- The RLM affordance text is a single static body:
  `internal/runtimeprompts/prompts.go:57-62` loads
  `internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml` (authority and stateful-notebook
  framing at lines 8-13; examples, receipts, error handling, choir surface, and reporting at
  15-47).
- REPL/session initialization is `cmd/capsule-broker/session_worker.go:274-285` (allowlist,
  broker, scope, `NewSession`), backed by `internal/yaegikernel/session.go:53-72` (one persistent
  Yaegi interpreter, filtered symbol set loaded once), with the `choir` package prebound by
  `ChoirExports` (`internal/yaegikernel/choir.go:113-132`).
- Every variation above is role-, owner-, configuration- or run-driven. **Model id does not appear
  anywhere in that path.** A search of the prompt and RLM surfaces
  found no model-id-conditional prompt or initialization; model metadata is enriched at
  `internal/agentcore/runtime.go:778-780` and consumed as `llmConfig` at `:3332`, never read by
  `systemPromptForRun`. The invariant is therefore a preservation requirement, not a rewrite.

### 12.2 What violates or sits adjacent to it today

| Site | What it does | Class |
| --- | --- | --- |
| `internal/yaegikernel/eval.go:301-317` (`CleanGoSource`), called from `agentcore/tools_capsule.go:773`, `yaegikernel/eval.go:122,148`, `yaegikernel/session.go:86`, and `cmd/capsule-broker/session_worker.go:392` | strips leading ``` / ~~~ fences and the language line, and a trailing fence, from model-authored Go | **direct violation** on the RLM path |
| `agentcore/tools_capsule.go:753-757` | the tool description already forbids fences twice ("Pass raw Go source directly without markdown fences (never ```go)", "Do not wrap in markdown code fences"), and the schema offers both `source` and `code` for the same value | prompt/schema redundancy; the stripper is defense that masks whether the instruction works |
| `agentcore/tools_capsule.go:746-772` | `source` and `code` are both plain strings; `src := input.Source`, falling back to `input.Code` only when `source` is empty, with empty `required`, so a model that sends `code` works, a model that sends both is silently resolved to `source`, and a model that sends neither executes an empty cell. Both names arrived with the tool itself (`d01553f8`, 2026-08-26) with no recorded rationale; `796dcb64` (2026-09-05) only documented the alias during a description-enrichment pass. It is the only `Alias for` in Go code. Precedent for trimming model-facing surface exists on this same tool: `allowed_packages` was removed when the broker took authority over the allowlist | duplicate affordance on the invariant surface — two names for one action split model behavior by naming luck and confound per-model measurement |
| `internal/provider/provider.go:1144-1152`, `:707-719`, `:809-832`; `internal/toolregistry/toolloop.go:513-555,1248-1269` | DeepSeek rejects thinking with exact tool choice, so reasoning is forced off with tools, and a tool loop retries by pattern-matching a provider error string | provider-behavior workaround, adjacent class — not output parsing, but model-specific behavior the mission will meet again on DeepSeek V4.1 Flash |
| `agentcore/run_memory.go:683-693`; `agentcore/email_lifecycle.go:434-517` | JSON substring extraction from first `{` to last `}`; email body marker trimming | model-agnostic tolerance on other surfaces; not RLM prompt workarounds |
| `internal/provider/provider.go:2163-2171` | recognizes image-capable models by id (gpt-5.5/5.4/kimi-k2p6) | capability validation, not parsing — but it is a model-id list that should not grow into a prompt branch |
| `internal/toolregistry/toolloop.go:680-753`, `:1015-1020`, `:700-744` | re-prompts with a reminder when a model ends without a required tool call, and appends a continuation instruction after `max_tokens` | generic protocol recovery, model-agnostic — keep; these are prompt-level nudges, not format repair |

The single direct violation is `CleanGoSource`. The tool surface already takes code as a
structured `source` argument and already instructs against fences, so the stripping is redundant;
its real cost is that it hides whether the instruction generalizes across models. Silent tolerance
converts a prompt weakness into a passing test.

### 12.3 Falsifiable acceptance for the invariant

1. **Prompt identity.** For a fixed desk, the assembled system prompt plus tool catalog is
   byte-identical across every model in §11. Diff the rendered prompt per model.
2. **No model branches.** A guard test greps the prompt/RLM assembly path for model-id
   conditionals and fails on any new one. (There are none today, so this locks in zero.)
3. **No fence handling.** `CleanGoSource` is deleted, or retained only with a dated justification
   and a test proving no roster model emits fences. Deleting it makes a fenced cell fail loudly
   instead of silently passing.
4. **Roster task, one prompt.** Each roster model completes one fixed desk task through
   `capsule_go_eval` with no reformatting, no retry-with-repair, and no human intervention.
   Record per-model pass/fail plus tokens; a model that fails marks the roster, not the prompt,
   unless it fails for a reason shared by others.
5. **One initialization.** REPL variables and prebound modules are established once per
   activation, identical for all models, with no model-conditional setup.

### 12.4 Interactions the mission must set per desk, not per model

- **Thinking budget.** A small reply cap is consumed entirely by reasoning and returns empty
  `content` (§9.3). The budget belongs to the desk's configuration, shared by all models.
- **Vision is a capability filter, not a prompt variant.** Image input is verified for five of the
  six cheap paid ids and for the free Meta id (§11); `hy3` is text-only. A desk that invites image
  reading should filter its roster by verified modality rather than carry a different prompt for
  text-only models. Phrase affordances as what the environment provides, and let capability decide
  who serves the desk.
- **Provider behavior differences are allowed to differ; prompt text is not.** Forcing reasoning
  off for DeepSeek-with-tools is a transport accommodation. It must stay in the adapter and must
  not leak into prompt text or desk instructions.

## 13. Open questions for the owner

1. **Training consent.** Are Muse Spark *Contributor* models permitted for any Choir traffic?
   If not, Phase B serves only the free Zen twin (also contributor-consented) or nothing.
2. **Money.** Disable Zen auto-reload and set monthly limits before wiring? (Recommended.)
3. **Session identity.** Approve the `RunID` plumbing in §10.3, or accept the per-computer
   fallback for the first cut?
4. **Free-keyless Zen.** Should the gateway be allowed to use `Authorization: Bearer public`
   at all, or must every call carry a real key for attribution and quota accounting?
5. **Catalog authority.** Pin a curated id list in the Go catalog (deterministic, stale
   eventually) or discover the live catalog at build/startup (fresh, non-deterministic)?
   `[INFERENCE]` recommend the pinned list plus a periodic drift check, because routing is a
   red surface and should not change without a commit.
6. **Go region requirement.** The deployed handler rejects `deepseek-v4-flash` and
   `deepseek-v4-pro` on Go unless the workspace region includes `cn` (`handler.ts:158-166`).
   The pruned roster keeps `deepseek-v4.1-flash`, which was not in that check list and answered
   normally, so the constraint appears not to apply — worth confirming before Phase A ships.
7. **`CleanGoSource` disposition.** Does mission three delete the fence stripper outright (making
   a fenced cell fail loudly) or keep it with a dated justification? The invariant in §12.3 leans
   toward deletion.
8. **The `code` alias.** `capsule_go_eval` accepts both `source` and `code` for the same value.
   Should mission three collapse that to one name, so the affordance the prompt describes is the
   only affordance the schema offers?
9. **Conversation scope — answered in §10.2/§10.7.** Rewarm and the agent-to-agent arc both
   resume the same `RunID`; only replacement runs mint a new one, and they rebuild the prompt from
   a compaction summary, so the id choice costs nothing extra there. Remaining decision: accept a
   cache miss at those boundaries, or fund a durable lineage id in the cache mission.
