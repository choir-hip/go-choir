# The paper's gpt-6-luna pin inherits the default pool's fallbacks

Status: open. This record documents the problem first; this commit has no
fix. Mutation class of a fix: orange, with full ceremony, because it
touches model routing.

Found 2026-10-22 by reading code, during the provider vault design
(`docs/provider-vault-and-model-policy-design-2026-10-22.md` §3.4). No
trace has yet shown a paper failover onto deepseek after the pin.

## What the owner asked for

At 21:4x on 2026-10-22 the owner asked to "make chatgpt (gpt-6-luna) the
sole model for the paper". FP0 wrote the paper's `System/model-policy.toml`
to name chatgpt/gpt-6-luna for the defaults and for every role, marked
`# owner-edited`
(`docs/definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md:875`).

Owner ruling R2, 2026-10-23: "I'd rather them fail than fallback. We are
still pre-launch."

## What the code does

- `Assign` (`internal/modelpolicy/model_policy.go:211-241`) gives a
  concrete selection that is a member of the default pool the rest of that
  pool as its fallbacks (`:225-231`). The test at
  `internal/modelpolicy/model_policy_test.go:248` asserts that an explicit
  gpt-6-luna gets deepseek-v4.1-flash as its first fallback.
- `EnrichMetadata` writes those fallbacks into `llm_fallbacks`
  (`model_policy.go:164-166`). The tool loop fails over through them
  (`internal/agentcore/runtime.go:3724`;
  `internal/provideriface/model_policy.go:73-96`).
- So on a failover-class error, such as a 429 after backoff, a 5xx, a
  timeout or an open circuit, the paper's threads move to
  opencode-go/deepseek-v4.1-flash. That breaks "sole model", R2 and R3
  ("Don't use Opencode as fallback tonight").
- A file that names a pool gets the pool's other members as fallbacks
  (`model_policy.go:234-240`). A two-member luna and Synthetic pool on the
  paper would therefore fail each member over to the other, which R2 also
  rules out.
- If Synthetic joins the default pool, a luna-pinned paper would fail over
  to Synthetic. A new route would then reach the paper first, through a
  failover.
- The management-task pin is the one existing exception: no fallbacks
  (`model_policy.go:156-160`).

## Evidence to collect

- The paper's run metadata, `llm_fallbacks`, on runs after 21:42:37
  2026-10-22. Expect `opencode-go/deepseek-v4.1-flash` on non-task desk
  runs.
- Gateway calls from the paper (vm-1669020a) to opencode-go after 21:49.
  The 21:53 ledger line saw none from 21:49 to 21:52, which fits a pin
  that had no failover to trigger yet.

## What this is not

It is not a claim that a fallback already happened. It is a claim about
what the code will do on the next failover-class error.
