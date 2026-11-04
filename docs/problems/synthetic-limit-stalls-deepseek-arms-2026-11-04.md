# Synthetic's subscription limit stalls every DeepSeek arm (2026-11-04)

Status: open; fix in the next commit.

## Evidence

- The gateway journal on Node B, after the 03:07Z reset: Synthetic answered
  `status=429 reason="You've exceeded your subscription rate limits"` again
  at 03:36:10, 03:48:00, 03:53:56, 03:59:01, 04:05:27, 04:10:32 and
  04:20:37Z. The provider-quota circuit (7efaaeae2) parks the callers until
  each estimated reset, and every half-open probe is refused again.
- The live8 trace (04:40Z): claude Library r2 (assignment-55ba1b07) and the
  pool repair round (assignment-89b768ab) lost their engineers at 03:31 and
  03:32Z, iterations 29 and 35, on "circuit open"; deepseek Calendar r2's
  Texture failed at 03:36Z; the management nudges sit in 20-minute retry
  backoff. No round on those arms has frozen since.
- Every desk on the deepseek, claude and pool arms, and every desk except
  engineering on luna, pins `synthetic/hf:deepseek-ai/DeepSeek-V4.1-Flash-1017`.
  One external limit stops four of the five arms at once.

## Why not another model

The arms compare models: their calibration and delivery records are sliced
by pinned model. Moving them to a different model would end that
comparison. OpenRouter serves the same snapshot,
`deepseek/deepseek-v4.1-flash-1017` (public models listing 2026-11-04,
0.15/0.60 per million, 1048576-token window), and the gateway already has an
OpenRouter provider (5138fe884) whose allowlist names only
`stealth/crouton-alpha`.

## Fix

Allow the OpenRouter snapshot and give it a catalog entry. The arms' own
`System/model-policy.toml` (owner-edited, test arms only) then names
`openrouter` for the same model. Rollback: restore each arm's policy file;
revert the commit.

Cost note: the price lookup matches by model name and so prices the
OpenRouter calls at Synthetic's 0.60/1.20 entry, which overstates
OpenRouter's price. That is the safe direction for a cost report.

## Route facts, read before landing (Devin review 1)

OpenRouter's public models listing for `deepseek/deepseek-v4.1-flash-1017`
on 2026-11-04: input modalities text and image; supported parameters
include tools, tool_choice, parallel_tool_calls, reasoning and
reasoning_effort; reasoning optional (default on, default effort high) with
efforts max, high, low and none only. The OpenRouter effort map was
crouton's (it sent minimal, medium and xhigh, which this model refuses), so
the map is now per model: for DeepSeek it follows Synthetic's meaning for
the same snapshot (minimal and the disable spellings to none, medium to
high) and sends xhigh, which this listing lacks, as max. Vision stays on,
on the listing's evidence; a refused image would show as a 400 on the
verifier_multimodal desk.
