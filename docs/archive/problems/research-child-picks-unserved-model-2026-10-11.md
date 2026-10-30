# A research child picks a model this computer cannot serve (2026-10-11)

Mission: [research fan-out](../definitions/choir-research-fanout-2026-10-10.md),
slice S1.

## Evidence

S1 demo on staging 0b7250ef, test computer demo-arxiv-2026-10-10c, document
ee7e8c2b, prompt "Research the news on and around AI for the last week."
submitted 00:01:54 UTC.

Research used `choir.Research` without being told to. Root run 7f1242cf ran
three children on its own model, gpt-5.6-luna: 61 s, 79 s and 125 s, each
returning 2,400 to 5,000 characters of findings. Root run 0abcac12 named
`gpt-5.4-mini` for its children. Guest log, 00:03:53 to 00:04:02:

```
research child: root=0abcac12-... model=gpt-5.4-mini duration_ms=80220 input_tokens=0 output_tokens=0 findings_chars=0 error="tool loop iteration 3: gateway call failed: gateway client: chatgpt: status 400 Bad Request (sanitized)"
```

(three children, all the same). The host gateway log shows every
gpt-5.4-mini request rejected on its first message (`messages=1`), and
over the last 14 days the gateway has served only two models: gpt-5.6-luna
(chatgpt) and deepseek-v4.1-flash (opencode-go). The 36 gpt-5.4-mini
requests in that window all failed.

## Cause

`researchChildSelection` accepts any model in `modelcatalog.SupportedModels()`.
The catalog lists 19 models Choir knows how to route through some provider;
it is not the set this computer is configured and credentialed to call.
That set is the computer's model policy (`modelpolicy`: defaults plus one
selection per role), which the owner edits. Nothing told the model which
models exist, so it guessed a plausible ID, the catalog check passed, and
each child spent about 80 seconds on rejected calls before returning an
empty result.

## Effect

Three of the six children in the first minute returned nothing. Their
objectives (half of one research run's plan) went uncovered unless the
parent retried. The 80 seconds also counted against the parent cell's
deadline.

## Fix direction

A child may name any model the computer's model policy names (its
defaults and every role), and runs with the provider and reasoning effort
the policy gives that model. Any other model is refused at once, before
any model call, with an error that lists the configured models, so the
parent can retry in the same cell. The owner widens the set by editing the
policy; the agent cannot.
