# Provider usage, spend and limits data: design input

Date: 2026-11-06. Research only; no code or config changed. Repo read at
/Users/wiz/Dev/go-choir-docs1 (origin/main), read-only. No secrets were read.
"Unverified" marks anything I could not confirm from a primary source.

## 0. What Choir actually uses (from the repo)

Sources: cmd/gateway/main.go (default model lists, ~L157-205), internal/provider/*.go,
internal/provider/quota.go, internal/provider/opencode_keys.go,
internal/gateway/anthropic_relay.go, internal/modelpolicy/model_policy.go.

| Provider id | Auth in the repo | Notes |
|---|---|---|
| chatgpt (gpt-6-luna) | Codex OAuth (ChatGPT subscription), read from CHATGPT_AUTH_PATH or ~/.codex/auth.json, auto-refreshed; calls https://chatgpt.com/backend-api/codex/responses | Not an API key. Quota 429 is detected by body text ("usage_limit_reached") and Retry-After. |
| opencode-go | OPENCODE_API_KEY (+ OPENCODE_API_KEY_ALT_n), a key pool with parking on 401/402/429; base https://opencode.ai/zen/go/v1 | Subscription key. Models: deepseek-v4.1-flash, glm-5.3-flash, qwen3.7-max, free previews, etc. |
| opencode-zen | same key pool; base https://opencode.ai/zen/v1 | Pay-as-you-go credits. |
| synthetic | API key, base https://api.synthetic.new/openai/v1 | Gateway already parses the x-synthetic-quotas response header (weekly percentRemaining, nextRegenAt, subscription renewsAt). |
| openrouter | API key; https://openrouter.ai/api/v1; models stealth/crouton-alpha, recursive/flint-v1.0:free, nvidia/nemotron-3.5-super-vl:free; separate key for the Decisions route (~typesafe/jev-latest, /api/alpha/decisions) | Free-tier models in use. |
| bedrock | AWS credentials (inferred; models us.anthropic.claude-*) | Unverified auth detail; not read. |
| zai | API key, https://api.z.ai/api/anthropic | GLM models. |
| fireworks | API key, https://api.fireworks.ai/inference/v1 | |
| anthropic relay (Claude delegate) | Owner's Claude OAuth token OR Console API key (ANTHROPIC_API_KEY) | Gateway relay already meters per-computer tokens (relayUsage, spend file). Code comment (SEC-08) records the terms risk of the OAuth token path. |

Existing gateway facts relevant to design: provider.Usage already carries
InputTokens, OutputTokens and CachedInputTokens; quota.go already types quota
exhaustion and reset time; the circuit breaker already reacts to 429s.

Discrepancy to check: OpenCode's current docs list base URLs under
`https://opencode.ai/inference/...` (see below), while the repo default is
`https://opencode.ai/zen/go/v1` and `/zen/v1`. The repo URLs may be legacy
aliases; unverified. Worth a live check before relying on `opencode.allowance`.

Naming discrepancy: the repo parses `planWeeklyLimit` from x-synthetic-quotas,
but a third-party doc describes `/v2/quotas` as `weeklyTokenLimit` and
`rollingFiveHourLimit`. The header and the endpoint are said by the repo comment
to be the same JSON; the schema may have changed. Verify with a live call.

## 1. Summary table

| Provider | Auth Choir uses | Usage API (tokens/requests/cost) | Limits / quota remaining visible | Recommended source |
|---|---|---|---|---|
| ChatGPT / Codex (subscription) | Codex OAuth | None for the subscription (docs name only dashboard and /status). OpenAI's Usage/Costs API covers API-key/org usage, not ChatGPT plan usage | Undocumented GET chatgpt.com/backend-api/wham/usage (unofficial, third-party-observed); dashboard chatgpt.com/codex/settings/usage; response 429 | Gateway metering for tokens; wham/usage poll as best-effort, flagged unofficial |
| OpenAI API (if ever used with a key) | API key | Yes: /v1/organization/usage/completions and /costs, Admin key, 1m/1h/1d buckets | x-ratelimit-* headers on responses; no limits endpoint | Admin API for reconciliation; headers live |
| OpenCode Go | API key (subscription) | No usage API documented | Console only; per-response `opencode.allowance` field (documented) | Gateway meters from `opencode.allowance` + usage fields |
| OpenCode Zen | API key (credits) | No usage/balance API documented; budgets in console | Not documented | Gateway meters usage; balance unknowable by API (unverified beyond docs) |
| OpenRouter | API key | Yes: GET /api/v1/activity (management key; grouped by date/endpoint/model, last 30 completed UTC days); per-response `usage.cost`; GET /api/v1/generation per id | GET /api/v1/key (normal key): usage, daily/weekly/monthly, limit_remaining, free_model_daily_requests; GET /api/v1/credits (management key) | /key for limits, response `usage.cost` for per-call spend, /activity for reconciliation |
| Synthetic | API key | No token/spend history endpoint found; /v2/quotas is a snapshot | GET /v2/quotas and x-synthetic-quotas header: rolling 5h, weekly % remaining, nextRegenAt, subscription | Header on every response (already parsed) + /v2/quotas poll |
| Anthropic (relay) | OAuth token or Console key | Console key: Admin Usage/Cost API (admin key, org accounts only). OAuth token: no | anthropic-ratelimit-* headers; Rate Limits API | Gateway relay metering (exists); Admin API for key path |
| Bedrock | AWS creds | CloudWatch AWS/Bedrock metrics (unverified, general knowledge) | Service Quotas (unverified) | Gateway metering + CloudWatch/Cost Explorer |
| Z.ai | API key | Undocumented monitor routes (third-party reverse-engineered) | same | Gateway metering; monitor API best-effort |
| Fireworks | API key | Not found in docs fetched (unverified) | Quota endpoints exist for account quotas | Gateway metering |

## 2. Per-provider findings

### 2.1 OpenAI / ChatGPT (Codex subscription path)

How Choir calls it: Codex OAuth tokens from auth.json against
chatgpt.com/backend-api/codex/responses. This is the subscription path, not the
OpenAI API.

Usage API (API-key/org accounts only):
- Endpoints GET /organization/usage/completions (and audio, code interpreter
  variants) and GET /organization/costs; Admin key as bearer; start_time
  required; bucket_width 1m/1h/1d (costs: 1d only); group_by project_id,
  user_id, api_key_id, model; paginated. Source:
  https://developers.openai.com/api/reference/resources/organization/subresources/usage
  and the cookbook https://developers.openai.com/cookbook/examples/completions_usage_api
  (result fields: input_tokens, output_tokens, num_model_requests,
  input_cached_tokens, ...). Freshness not stated in the pages I read (unverified).
- These cover traffic made with OpenAI API keys. They do not report ChatGPT
  subscription (Codex sign-in) consumption. The Codex docs say API-key use
  "follows API pricing instead of plan limits" and name no API reporting plan
  usage: https://learn.chatgpt.com/docs/pricing (redirect target of
  https://developers.openai.com/codex/pricing).

Limits / quota visibility on the subscription:
- Official: usage dashboard chatgpt.com/codex/settings/usage and `/status` in the
  Codex CLI; plans have a 5-hour window on Plus/Business (Pro currently none)
  plus weekly limits that may apply; local and cloud share an allowance; credits
  buy extra use. Same pricing page. Whether the 5-hour limit is active has been
  changing (conflicting third-party reports); treat as unverified, and the
  official page text is the only authority.
- Unofficial: GET https://chatgpt.com/backend-api/wham/usage with the OAuth
  bearer and a ChatGPT-Account-Id header returns rate_limit.primary_window /
  secondary_window with used_percent, limit_window_seconds (18000 / 604800
  seen), reset_at, reset_after_seconds. Seen only in third-party tools, no
  official docs; may change without notice. Sources:
  https://docs.rs/crate/codexusage/0.4.0/source/src/app/codex_limits.rs ,
  https://pypi.org/project/codex-backend-sdk/0.3.0/ . I did not call it.
- API-key rate-limit headers (x-ratelimit-limit/remaining/reset for requests and
  tokens, plus project-token variants) exist for the API, and there is no
  general rate-limits endpoint:
  https://developers.openai.com/api/docs/guides/rate-limits . Whether the Codex
  backend returns them on the subscription path: unverified.

Terms: Codex docs allow ChatGPT sign-in for Codex use, caching tokens in
auth.json, and say to treat it like a password; API keys are the default for
CI/automation; "Codex access tokens" exist for trusted enterprise automation
with admin permission: https://learn.chatgpt.com/docs/auth . The page does not
address using those credentials in a third-party service. I could not fetch
OpenAI's Terms of Use (HTTP 403), so the restriction on automated access
outside the API is unverified. Flag as a standing risk: Choir's gateway acts
as a non-Codex client of the subscription backend, and the wham/usage poll
would deepen that.

### 2.2 OpenCode Go and Zen

Go (https://opencode.ai/docs/go/):
- Three allowances per model: 5-hour (20% of monthly), weekly (50%), monthly
  (100%), in dollar terms; usage tracked in the Console; "Each response reports
  how much the request counted toward your allowance in a field named
  `opencode.allowance`." No dedicated usage API is described.
- Docs list base URLs under https://opencode.ai/inference/go/... and a model
  list at /inference/go/v1/models; the repo uses /zen/go/v1 (see section 0).
- Terms: Go/Go Plus are for single-member workspaces; traffic is monitored for
  abuse; clients should send their own user agent and a stable
  `x-opencode-session` header. Choir pools several keys (OPENCODE_API_KEY_ALT_n);
  whether multi-account pooling fits the single-member terms is unverified and
  worth an owner decision.

Zen/Console (https://opencode.ai/docs/zen/ ; the fetched page is titled Console):
- Pay-as-you-go with credits, auto-recharge, workspace and per-member monthly
  budgets. "The page doesn't describe a way to check your balance"; only
  documented API is the model list. No rate limits documented. Free-model data
  policies are listed there (some free models train on data or retain 30 days),
  relevant to which pools may carry owner content.

Conclusion: no pull API. The gateway must meter. `opencode.allowance` is the
only machine-readable remaining-quota signal, and I could not see its exact
shape (unverified; capture a live sample).

### 2.3 OpenRouter

Best-documented provider.
- Per-key status, normal key: GET https://openrouter.ai/api/v1/key returns
  limit, limit_remaining, limit_reset, usage, usage_daily/weekly/monthly (UTC),
  byok_usage, is_free_tier, free_model_daily_requests {used, limit, remaining}.
  Per-IP burst limit on this endpoint. https://openrouter.ai/docs/api-reference/limits
- Credits: GET /api/v1/credits requires a management key; returns total_credits
  and total_usage. https://openrouter.ai/docs/api-reference/get-credits
  (the fetch worked, but note the doc path differed from the search index; fine).
- Activity: GET /api/v1/activity, management key, last 30 completed UTC days,
  optional date and api_key_hash filters, rows by date/endpoint/model with
  USD cost; workspace aggregation. Source (search result only, per-row field
  names beyond date/endpoint/model/cost unverified):
  https://openrouter.ai/docs/api/api-reference/analytics/get-user-activity
- Per call: every response includes a usage object with tokens, cached_tokens
  and `cost`; streaming puts it in the final event; GET /generation by id for
  later lookup. https://openrouter.ai/docs/use-cases/usage-accounting
- Rate-limit headers: successful responses carry no X-RateLimit-* headers; 429s
  for free models and key management do. Free models: 20 requests/minute, and
  50/day (under 10 credits purchased ever) or 1000/day (10 or more). Negative
  balance gives 402 even for free models; in-flight budget 402 has
  limit_source openrouter_in_flight_budget. Same limits page.
- Management keys are separate credentials; reading keys/usage/credits does not
  count toward their 10,000/day management limit.
- Choir uses free models, so the real limit is free_model_daily_requests, which
  /key exposes directly. Good fit for a quota tile.
- Terms: nothing restrictive found in what I read; the Decisions route is
  alpha (`/api/alpha/decisions`), so its accounting is unverified.

### 2.4 Synthetic

- Quota snapshot: GET https://api.synthetic.new/v2/quotas, Bearer key, and
  quota checks do not count against limits (third-party docs). Fields per the
  OpenTokenUsage doc: rollingFiveHourLimit {remaining, max, limited, tickPercent,
  nextTickAt}, weeklyTokenLimit {percentRemaining, nextRegenAt}, subscription
  {limit, requests, renewsAt} (legacy), search.hourly. Sources:
  https://raw.githubusercontent.com/PowerUserZ/OpenTokenUsage/main/docs/providers/synthetic.md ,
  https://skills.sh/sirn/dotfiles/synthetic-quota . The official docs index
  confirms a `/quotas` endpoint exists but did not show the schema:
  https://dev.synthetic.new/docs/api/overview . Field names are therefore
  unverified against an official page, and the repo's `planWeeklyLimit` differs.
- Official limits text: weekly credit limit, regenerating 2% about every 202
  minutes, full regen from zero in a week; cheaper models draw less.
  https://synthetic.new/rate-limits (an earlier search summary also mentioned a
  5-hour request limit and concurrency throttle; the page fetch said weekly
  only. Conflict, unverified.) Repo evidence: Synthetic answers a concurrency
  429 and a quota 429 differently.
- Header: the repo reads x-synthetic-quotas on responses. I did not find this
  header in any public doc (unverified outside the repo's own code); treat it
  as observed behavior.
- No token or spend history endpoint found. Per-call usage comes from the
  OpenAI-compatible usage object (standard, unverified for Synthetic).

### 2.5 Anthropic (delegate relay) and other providers

- Console API key path: Usage and Cost Admin API (/v1/organizations/usage_report/messages,
  /cost_report), Admin API credentials, 1m/1h/1d buckets, data "typically
  appears within 5 minutes", poll about once per minute; unavailable to
  individual accounts; a newer Usage and Cost Analytics API is recommended for
  new work. https://platform.claude.com/docs/en/manage-claude/usage-cost-api
- Response headers anthropic-ratelimit-requests/tokens/input-tokens/output-tokens
  {limit, remaining, reset} (RFC 3339); a Rate Limits API reads configured
  limits. https://platform.claude.com/docs/en/api/rate-limits . Spend-cap 429
  has error_code enforced_spend_limit_reached and no retry-after.
- OAuth token path: Anthropic's policy says OAuth is for ordinary use of Claude
  Code and native apps; developers must not route requests through plan
  credentials on behalf of users or "collect, store, or intermediate Claude.ai
  credentials or session tokens"; Anthropic may enforce without notice.
  https://code.claude.com/docs/en/legal-and-compliance . The relay already
  records this residual risk. A usage dashboard that makes the OAuth path more
  prominent should keep the Console-key path as the supported route. Subscription
  usage windows via OAuth: no API found (unverified).
- Bedrock: CloudWatch AWS/Bedrock metrics (Invocations, InputTokenCount,
  OutputTokenCount by ModelId), invocation logging, Service Quotas. My fetch
  returned a stub page and the tool answered from general knowledge, so this is
  unverified. Verify at
  https://docs.aws.amazon.com/bedrock/latest/userguide/monitoring-cw.html .
- Z.ai: undocumented monitor routes (GET api.z.ai/api/monitor/usage/quota/limit,
  .../model-usage) found only via third-party tools; unverified, may be
  unavailable while inference works.
  https://pypi.org/project/zai-limits/ , https://docs.rs/zai-rs/latest/zai_rs/usage/index.html
- Fireworks: the API intro lists quota endpoints (List/Get/Update Quota) but I
  found no usage/billing API or rate-limit headers in what I fetched (unverified);
  check https://docs.fireworks.ai/llms.txt .

## 3. Can the gateway meter itself?

Yes for the primary record, and it should. Every call passes the gateway, which
already normalizes Usage (input, output, cached input) per call.

What it can capture reliably per call: timestamp, computer/VM id, desk/role,
pool and chosen member, provider, model, latency, outcome and typed failure
(quota, 429 concurrency, 5xx, circuit-open), input/output/cached/reasoning
tokens from the response usage object (streams: final event; OpenRouter
documents this explicitly), OpenRouter `usage.cost`, OpenCode
`opencode.allowance`, selected rate-limit headers (Synthetic x-synthetic-quotas;
OpenAI/Anthropic x-ratelimit-* / anthropic-ratelimit-*), the key identity
(by variable name, never by value), and Retry-After.

What self-metering misses:
- Calls not through the gateway (owner's other Codex/Claude Code use sharing the
  same ChatGPT subscription or OpenCode account; other keys; the dashboard UI).
  Subscription allowances are shared, so remaining-quota derived from Choir's own
  count is a lower bound on use.
- Dollar cost where no price is in the response: must be computed from a price
  table (drifts; caching/long-context tiers; subscriptions are dollar-equivalent
  allowances, not cash). Real billed spend needs provider reconciliation.
- Aborted or dropped streams where the final usage event never arrives (tokens
  spent but unseen); the relay already estimates input as len(body)/4 for that.
- Provider-side adjustments, free-tier accounting, and retries billed upstream
  that the client never saw.
- Remaining quota itself, except where headers/fields report it.

## 4. Recommendation

Record in the gateway (append-only usage ledger, one row per upstream attempt,
including failed ones; no prompt content): the fields listed above. This is
the source of truth for who/which desk/which pool, tokens, latency, failure
modes and cache hit rate, and it works identically for every provider, including
ones with no API (OpenCode, Z.ai, Fireworks, ChatGPT subscription).

Pull from provider APIs, on a slow poll (about 1/min at most), as a separate
"provider status" record, not mixed into the ledger:
- OpenRouter: /key (limits, free-model daily counter) with a normal key; add a
  management key only if org-level reconciliation via /activity and /credits is
  wanted (a second credential to hold; owner decision).
- Synthetic: /v2/quotas (free to call) in addition to the response header.
- ChatGPT: wham/usage only as an optional, explicitly unofficial probe; default
  to headers and 429 signals. Needs an owner decision on terms risk.
- Anthropic: Admin Usage/Cost API only if the Console-key path is used.
- OpenAI API/Bedrock/Fireworks: only if those become active spend paths.

Reconcile ledger totals against OpenRouter /activity and the Anthropic or OpenAI
admin reports daily; show drift in the analytics app rather than hiding it.

Unknowable or weakly knowable:
- ChatGPT subscription remaining quota except via the unofficial endpoint,
  the dashboard, or 429s; the plan's actual window rules also shift.
- OpenCode Go/Zen remaining allowance or balance outside the per-response
  `opencode.allowance` field and the Console; Zen balance not documented at all.
- Shared-account consumption by non-Choir clients.
- True cash spend on subscription plans (flat fees; per-token cost is an
  equivalent, not a charge).
- Exact weekly Synthetic credits in dollars (percent only).

Design implications for the model-pool/routing app: routing on "remaining
quota" is practical for Synthetic (header), OpenRouter (/key), and OpenCode
(allowance field once its shape is confirmed); for ChatGPT, route on observed
429s with Retry-After, as the gateway already does. Keep per-provider status
as best-effort, with a freshness timestamp and a source label (header, poll,
estimate) in the UI so stale or unofficial numbers are visible as such.

## 5. Items to verify live before building

1. Capture a real `opencode.allowance` payload and confirm whether /zen/go/v1
   still serves or redirects to /inference/go/....
2. GET /v2/quotas on Synthetic: confirm schema vs the repo's `planWeeklyLimit`.
3. Whether the Codex backend returns any rate-limit headers on responses.
4. OpenAI Terms of Use text on automated access (fetch was blocked).
5. OpenRouter /activity row schema (only search-snippet evidence) and the
   Decisions route's accounting.
6. Bedrock metrics and Service Quotas against AWS docs.
