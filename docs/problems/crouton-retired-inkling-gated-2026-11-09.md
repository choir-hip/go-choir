# crouton-alpha is retired; the owner's intended default, inkling, is gated (2026-11-09)

Status: open. Mutation class of the fix: orange (provider/model routing).

## Evidence (orchestrator, about 23:40 UTC, 2026-11-09)

- OpenRouter answers `stealth/crouton-alpha` with HTTP 404: "Crouton Alpha is
  no longer available. This stealth model was retired on November 9, 2026."
  The gateway journal shows `provider call failed ... openrouter: status 404
  Not Found` repeating for d@e.
- crouton-alpha leads the default pool and the selfdev pool, is the sole
  primary of `crouton-synthetic-luna` (`internal/modelcatalog/catalog.go`), is
  first in the gateway's OpenRouter list (`cmd/gateway/main.go`) and is the
  default of `GATEWAY_OPENROUTER_MODELS` (`nix/deploy-provider-creds.sh`).
  Every pooled call on every computer therefore starts with a dead model.
- The owner wants `thinkingmachines/inkling-2-small:free` as the default.
  Listing: 1,048,576 context, 131,072 max completion tokens, text and image
  input; supported parameters include_reasoning, max_tokens, reasoning,
  reasoning_effort, seed, stop, temperature, tools, top_p. No `tool_choice`.
- Today OpenRouter answers inkling with HTTP 403 for our key: "only available
  on agentic harnesses. Try plugging it into a coding agent or productivity
  app listed on https://openrouter.ai/apps" (failed_routing_step "Gate Free
  Endpoints by Agentic Harness"), also with `HTTP-Referer: https://choir.news`,
  `X-Title: Choir` and `X-OpenRouter-Categories: cli-agent`. Community
  reports say the gate keys on apps OpenRouter already lists, and an app page
  appears only once attributed traffic arrives. The gateway sends no
  attribution headers today (`internal/provider/openrouter.go`, only
  `User-Agent: choir-gateway/0.1`).
- The other free OpenRouter members (flint, nemotron) answer 429
  "free-models-per-day-high-balance", limit 1000 a day.

Attribution header reference (read 2026-11-09,
https://openrouter.ai/docs/app-attribution): `HTTP-Referer` is the primary
identifier and is required for an app page; `X-OpenRouter-Title` sets the
display name (`X-Title` is still supported for backwards compatibility);
`X-OpenRouter-Categories` takes up to 2 comma-separated lowercase values from
a fixed list, `cli-agent` among them; unknown values are dropped silently.

## Belief state

- Certain: crouton is gone; all current pool leads are a dead call. Pool
  failover covers it (see below), at the cost of one failed call per computer
  per hold window.
- Believed: inkling passes the gate only once OpenRouter lists Choir as an
  app, which needs attributed traffic. Honest attribution headers are the one
  thing we can do, and are correct to send regardless.
- Not known: whether attribution headers alone will ever open the gate; how
  long an app page takes to appear; inkling's rate limits and its behaviour
  without `tool_choice` (unmeasured: every call is refused).

## Failover finding (read, not run)

A 404 from OpenRouter becomes `openrouter: status 404 Not Found (sanitized)`
in the gateway, which answers the guest 502 (`internal/gateway/handlers.go`,
`status := http.StatusBadGateway`). The guest's `gatewayHTTPStatusError`
treats 5xx as transient (`internal/gatewayruntime/provider.go`
`Transient()`), so `IsModelFailoverError` is true
(`internal/toolregistry/toolloop.go` `isTransientProviderError`) and, with a
fallback left, `failsOverWithoutRetry` returns at once with no backoff on the
dead member. The loop then `ledger.Demote`s it (non-hard: 1 minute doubling to
a 15 minute cap, `quota_ledger_state.go` / `toolloop.go` `failureHoldBase`),
so a dead member costs one failed call per hold window. `IsHardProviderFailure`
does not match a bare 404 or 403 by design (comment at
`quota_ledger_state.go:33-36`), so the hold is the short one, not the long
one. Acceptable; no code change. A second problem, left documented: a
retired-model 404 whose body says so could take the hard hold; the gateway
sanitizes the body away, so the guest cannot tell it from a request-shaped 404.

## Plan and failure list (tests written to this list)

1. Pools still lead with a dead model: catalog tests that pin crouton as
   primary (`TestOpenRouterCroutonAlphaCatalogued`, `TestCroutonSyntheticLunaPool`,
   the default and selfdev pool tests) must now pin the next member.
2. A stored policy naming `openrouter/stealth/crouton-alpha` must not crash or
   strand a role: it falls to the default pool (`Assign`, "retired_remapped")
   and must say so visibly in `llm_policy_error`.
3. A stored policy naming `pool:crouton-synthetic-luna` would be rejected by
   `parsePolicy` as an unknown pool; nothing in the repo names it, so the pool
   is deleted and this is recorded as a residual.
4. Inkling is catalogued, routable by the gateway, and in no pool.
5. OpenRouter requests carry `HTTP-Referer`, `X-OpenRouter-Title`, `X-Title`
   and `X-OpenRouter-Categories`, and nothing naming another app.
6. Inkling is called without `tool_choice`.
7. Pricing: the zero-price crouton entry goes.
