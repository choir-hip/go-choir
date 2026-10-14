# Model Prices, 2026-10-14 (LR Lever 7)

Researched 2026-10-14 from provider pages, cross-checked against
OpenRouter's endpoints API. Owner: "token pricing per model and provider
is easily researched." The prices go into `internal/llmcost/pricing.go`,
at each model's origin list rate. What a route actually charges is below.

## Routes (from the repo)

- **opencode-go**: `https://opencode.ai/zen/go/v1`
  (`internal/provider/provider.go`). OpenCode's docs now give
  `https://opencode.ai/inference/go/v1`; ours still serves and may be a
  legacy alias.
- **chatgpt**: `https://chatgpt.com/backend-api/codex/responses`, a
  ChatGPT login, so flat-rate. The plan is not recorded in the repo.
- **typesafe/jev-1.13**: OpenRouter's Decisions alpha endpoint, with
  TypeSafe as the only host.
- **fireworks**: `https://api.fireworks.ai/inference/v1`; no Fireworks
  model is in the catalog now.

## OpenCode Go

A flat subscription: Go is $10 a month, Go Plus $40
([opencode.ai/docs/go](https://opencode.ai/docs/go/)).
- Each model has a monthly dollar allowance, metered at per-token prices.
  The 5-hour cap is 20% of it and the weekly cap 50%.
- Monthly allowances, Go / Go Plus: DeepSeek V4.1 Flash $80 / $160,
  DeepSeek V4 Pro $15 / $60, GPT 6 Luna $15 / $60. The Go Plus column came
  from a page summary.
- With "Use balance" on, usage past the limits bills pay-as-you-go at the
  same prices.
- Effective rate: the fee divided by the usage consumed. Go fully used on
  V4.1 Flash is $10 for $80 of usage, about 0.125 of list.
- The free models (LongCat 2.5 Preview Free, Cat Jam Free, Step 5 Preview
  Free) are "free for a limited time", with no end date.

## Prices (USD per 1M tokens: input / cached input / output)

| Pair | Billing on our route | Origin list price (in the table) | Source | Confidence |
|---|---|---|---|---|
| opencode-go / longcat-2.5-preview-free | free, limited time | table: 0 / 0. Elsewhere about 0.30 / 0.006 / 1.20 (Vercel AI Gateway, 2026-09-26) | OpenCode docs; Vercel listing | route high; list unverified |
| opencode-go / deepseek-v4.1-flash | Go allowance | **0.30 / 0.006 / 1.20** peak (01-04 and 06-10 UTC weekdays); off-peak 0.15 / 0.003 / 0.60 | [DeepSeek pricing](https://api-docs.deepseek.com/quick_start/pricing) | high |
| opencode-go / cat-jam-free | free, limited time | table: 0 / 0. Origin unknown | OpenCode docs | route high |
| chatgpt / gpt-6-luna | ChatGPT subscription (Plus $20; Pro $100 to $500) | **0.10 / 0.01 / 0.50** up to 272K context; 0.20 / 0.02 / 0.75 above | [OpenAI API pricing](https://developers.openai.com/api/docs/pricing) | high |
| typesafe / jev-1.13 | per token, OpenRouter credits | **0.042 / – / 0** (prompt up to 32K) | OpenRouter endpoints API (`jev-1.13-20260917`) | high |
| deepseek-v4-pro | not used now | **1.32 / 0.044 / 3.96** peak; off-peak half | DeepSeek | high |
| kimi-k2p6 | not used now | **0.95 / 0.16 / 4.00** | OpenRouter (Moonshot) | medium |

## The last window on the paper computer (recent window, 10-14)

265 calls: 10.5 M input and 0.69 M output tokens, all uncached (upper
bound).

| Model | Tokens in / out | What we paid | At list |
|---|---|---|---|
| longcat-2.5-preview-free | 6.29 M / 0.40 M | $0 | $2.37 (unverified rate) |
| deepseek-v4.1-flash | 1.38 M / 0.20 M | $0.33 to $0.65 of the Go allowance | $0.33 to $0.65 |
| cat-jam-free | 0.84 M / 0.07 M | $0 | unknown |
| gpt-6-luna | 2.01 M / 0.02 M | $0 marginal (subscription) | $0.21 |
| **Total** | | **$0.33 to $0.65** plus flat fees | **$2.91 to $3.23** plus Cat Jam |

About 81% of the list-rate cost is LongCat, which is free only for now.

## Gaps

- The table has one rate per model. DeepSeek's off-peak discount and
  GPT-6 Luna's above-272K tier are not modelled, so DeepSeek reads high
  off-peak.
- Cached input is not split out in the cost record.
- **Jev's calls are not in `/api/costs` at all.** Intake labels and
  support judgements go through the gateway's Decisions path, which
  records no cost event. They are cheap ($0.042 per 1M input), but they
  should still be counted.
- Unverified: LongCat's list price, Cat Jam's origin, which ChatGPT plan
  the gateway uses, and Fireworks' V4 Pro and K2.6 prices.
