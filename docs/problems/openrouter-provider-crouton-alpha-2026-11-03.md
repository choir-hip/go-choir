# OpenRouter chat provider for stealth/crouton-alpha (2026-11-03)

Owner, 2026-11-03 ~21:30Z: "lets add one more model. we have free usage for
it for a week. provider is openrouter. model string "stealth/crouton-alpha"
... configure this model on node-b, then run another vm with only it for
every desk."

## Problem

OpenRouter reaches the gateway only as the Jev Decisions route
(`internal/gateway/jev.go`). `OPENROUTER_API_KEY` is already in the gateway
env (`nix/deploy-provider-creds.sh`), but there is no OpenRouter chat
provider, no catalog entry and no price, so a model policy that names
`provider = "openrouter"` fails at the gateway ("unsupported provider").

## Probe (2026-11-03, before wiring)

Public listing, `GET https://openrouter.ai/api/v1/models` (no key):

- id `stealth/crouton-alpha`, name "Crouton Alpha"; context_length 1048576,
  max_completion_tokens 943718;
- input modalities text, image, video, file, audio; output text;
- pricing prompt 0, completion 0;
- supported_parameters: include_reasoning, max_tokens, reasoning,
  reasoning_effort, response_format, structured_outputs, temperature,
  tool_choice, tools, top_p;
- reasoning mandatory; efforts max, xhigh, high, medium, low, minimal;
  default max.

Keyed calls to `/api/v1/chat/completions` (the key's presence was checked,
never printed; the model is free):

- a tool call answers `finish_reason: tool_calls` with one call, about 1.3 s;
  the message carries `reasoning: null` and `reasoning_details` of type
  `reasoning.encrypted`;
- the second turn with the tool result answers with or without the
  `reasoning_details` echoed back (200 both ways), so round-tripping them is
  not needed to run;
- `reasoning_effort: "none"` is a 400 ("Reasoning is mandatory for this
  endpoint and cannot be disabled"); an unknown value is a 400; minimal and
  max are 200;
- streaming: reasoning_details deltas, tool-call deltas, then **two**
  `finish_reason: tool_calls` chunks, usage only on the second;
- a solid-colour 64x64 PNG sent as an `image_url` data URI was named
  correctly twice (blue, red);
- usage reports `cost: 0`; prompt_tokens read 12 to 13 for a prompt with a
  system message and a tool, which looks under-counted upstream.

## Design

Follow Synthetic's path for a provider named `openrouter`:

- `OpenRouterProvider` in `internal/provider`: OpenAI-compatible chat
  completions at `https://openrouter.ai/api/v1/chat/completions`
  (`OPENROUTER_BASE_URL` overrides), Bearer `OPENROUTER_API_KEY`, held only
  by the gateway. Tools, tool_choice and streaming as the other
  chat-completions providers. No HTTP-Referer/X-Title: the code has no
  convention for them (jev.go sends neither).
- Reasoning effort maps to the advertised set; none/off/disabled/false map
  to minimal (reasoning cannot be disabled); an unknown value is omitted.
- A 402 (credits) and a 429 whose reason names a quota or limit are typed
  quota exhaustion; other statuses are sanitized status errors that the
  breaker counts as before. No concurrency gate: no limit is known.
- The shared chat-completions stream parser emits the closing
  `message_delta` at the end of the stream with the last usage seen, so a
  usage that arrives on a later chunk (OpenRouter's second finish chunk,
  Synthetic's trailing cache report) reaches the guest.
- Models from `GATEWAY_OPENROUTER_MODELS`, default `stealth/crouton-alpha`;
  `deploy-provider-creds.sh` sets it when the key is present.
- Catalog entry `openrouter` / `stealth/crouton-alpha`, "Crouton Alpha
  (OpenRouter stealth)", window 1048576, max output 65536 (the catalog's
  ceiling, not the advertised 943718); vision on (probed). In no pool.
- Priced at zero in llmcost, dated: free for one week from 2026-11-03.
- Chat-completions providers omit the interactive output budget;
  openrouter joins that list.

## Residuals

- `reasoning_details` are not round-tripped. The probe shows they are not
  required; whether they improve multi-turn quality is open.
- The free window closes about 2026-11-10; the zero price and the arm must
  be revisited then.
- Rate limits for the stealth model are unknown; a 429 storm would trip the
  breaker per model as for other providers.
- Run `scripts/vision_probe.py --model openrouter/stealth/crouton-alpha`
  on staging to confirm vision through the product route.
