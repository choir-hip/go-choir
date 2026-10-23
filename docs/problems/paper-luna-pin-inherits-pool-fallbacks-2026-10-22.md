# The paper's gpt-6-luna pin inherits the default pool's fallbacks

Status: fixed in lane/fp0-pin-hardening (see the 2026-10-23 update at the end;
not yet on main). This record documented the problem first; the first commit had no
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

## Update 2026-10-23 (fix round fp0-pin-hardening): findings from review bunch 147

Status of the original claim: the default pool is gpt-6-luna alone since the
D8 landing (`modelcatalog.Pools`), so the luna pin has no fallback to inherit
today (`rotate(members, i)[1:]` is empty). The claim above described the
pool as it was at 21:4x. The structural defect is still in the code: the
rule "a concrete model that is a member of the default pool gets the rest of
the pool" is still in `Assign`, so adding any second member to the default
pool again gives every explicit pin a fallback it did not ask for. Review
bunch 147 (omp-deepseek-v41-flash, devin) raised three findings against the
D8 lane; this round answers them. Mutation class: orange (model routing).

Finding 1 (Medium). No-failover is a property of the pool spec only
(`NoFailover(pool)`). `Assign` still gives an explicit provider/model pin the
default pool's other members as fallbacks (`model_policy.go`, the
`pool == ""` branch). R2 ("I'd rather them fail than fallback") applies to a
pin as much as to a pool. Fix: a concrete pin gets no fallbacks. The policy
file has no real fallback field today: `fallback_provider` and
`fallback_model` are only aliases of `provider` and `model` in `applyValue`
(a `fallback_model` line replaces the primary model; this is its own trap
and is recorded below as a residual). So there is no "explicit fallback" to
honour, and none is added here.

Finding 2 (High, deploy gate). An unknown pool name resolves silently as
`pool:default`: `Assign` falls back when `Pools()[pool]` is missing, and
`validSelection` accepts any `pool:<non-empty>`. A typo, or a pool removed
from the catalog, runs on the default pool with no signal. Fix: the policy
parser rejects a pool the catalog does not name (the file is then handled as
every invalid file is: last valid policy, else the platform policy, with
`llm_policy_error` naming the pool on every run and on the resolve API; the
Texture prompt-eval path and engineering assignment already refuse a run
whose policy has an error), and `Assign` stops remapping: an unknown pool
yields no fallbacks and an `unknown_pool` assignment, and `EnrichMetadata`
records a policy error naming the pool.

Finding 3 (Low). `syntheticReasoningEffort` passes `minimal` and `medium`,
which the live model accepts (probe, Q2 in
`synthetic-provider-onboarding-2026-10-22.md`) but which are outside the
advertised enum none/low/high/xhigh/max. Fix: map by thinking mode, to the
nearest advertised value in the same mode: `minimal` to `none` (both select
the non-thinking template; 0 reasoning tokens in the probe), `medium` to
`high` (both think). The advertised enum is the safer contract: a server
that tightens its validator to the documented set would 400 the thread
otherwise.

Evidence: code reading only. No trace shows any of the three firing.

### Disposition (fix round fp0-pin-hardening, orange, rollback: `git revert` the fix commit)

1. Pin fallbacks: fixed. `Assign` returns no fallbacks for a concrete
   provider/model, in any pool; the default-pool-member rule is deleted. A
   pool spec still fails over inside its pool (except `synthetic-luna`), and
   a retired model remapped to the default pool keeps that pool's
   fallbacks because it asked for the pool. Test:
   `TestAssignExplicitPinGetsNoDefaultPoolFallbacks` (a two-member default
   pool through the `catalogPools` seam; fails on the prior tip).
2. Unknown pool: fixed by failing closed, not by keeping the remap. The
   policy and overlay parsers reject a pool the catalog does not name, with
   the pool in the error. A rejected file is handled like every invalid
   file: `Load` returns the last valid policy, else the platform policy,
   plus the error; every run's metadata carries `llm_policy_error` naming the
   pool, and the resolve API reports `policy_error`. The Texture
   prompt-eval and engineering-assignment paths already refuse a run whose
   policy has an error, so they now fail for such a file. Other runs
   (conductor, research, management, co-agent spawn) ignore the error and
   run on the platform policy as they do for any invalid file; this is the
   existing handling, not a new loop or crash. Defense in depth: `Assign`
   no longer remaps (assignment `unknown_pool`, unresolved selection),
   `EnrichMetadata` adds the error, and a research child refuses the
   selection. Existing files: the platform default text names only
   `pool:default`, and `pool:synthetic-luna` is the one other pool; no
   test or default names another pool. A computer whose own file names a
   pool the catalog lacks (I could not look at any live file, no SSH) would
   now see the error where it previously ran on the default pool; for the
   old pools `default` the platform policy gives the same model. Test:
   `TestUnknownPoolFailsClosedNamingThePool`.
3. Synthetic effort: fixed, and I chose the advertised enum as the
   contract: `minimal` maps to `none`, `medium` maps to `high` (same
   thinking mode as the probe in Q2 showed), so the provider only ever
   sends none, low, high, xhigh or max, or omits the field. Test:
   `TestSyntheticReasoningEffortNeverSendsAnInvalidValue`.

Declined: an explicit fallback in the policy file. No such field exists.
`fallback_provider` and `fallback_model` are aliases of `provider` and
`model` in `applyValue`, kept because old platform-default files used them
as the primary (a test still parses such a file), so a `fallback_model` line
sets the primary, not a failover. Adding a real fallback field would touch
`LLMSelection`, overlays, fill and the runtime; R2 says fail rather than fall
back, so it is not built here.

Residual (named, not fixed): the `fallback_*` key aliases are misleading and
could be renamed or rejected in a later policy-version bump. Evidence
collection for the original claim (paper run metadata after 21:42) is
unchanged and still wanted before this record closes.
