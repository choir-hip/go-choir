# Model pools autoroute on failures (2026-11-02)

Owner, 2026-11-01: "Fix the pool to autoroute on successive failures" and
"Not just synthetic. The pools should always autoroute on failures. See the
search pool as an example of that". Earlier: "The paper should be set to
auto switch 100% to ChatGPT if synthetic is down."

Mutation class: red (gateway/provider routing; guest tool loop).

## Problem

A model pool (default: 50/50 Synthetic `hf:deepseek-ai/DeepSeek-V4.1-Flash-1017`
and ChatGPT `gpt-6-luna`, `internal/modelcatalog/catalog.go`) assigns each
thread one member and fails over to the next. Failover existed but was slow
and never short-circuited a member that was down:

- The guest (`internal/toolregistry/toolloop.go`) retried any 429 on the
  same model at 5 s / 20 s / 60 s (about 85 s) before failing over, even
  with a healthy member left.
- The gateway (`internal/gateway/circuit_breaker.go`) excluded every
  Synthetic 429 from its per-model breaker, because the provider typed every
  Synthetic 429 as a concurrency limit (`upstreamConcurrencyError`). A
  quota-exhausted Synthetic therefore never opened, and every thread
  assigned to it paid the 85 s first.
- The breaker's open duration was a fixed 30 s, so a dead member was probed
  every 30 s forever and a recovered one had no notion of a streak.
- ChatGPT's 429 reason was discarded unlogged.

## Evidence (gateway journal, Node B, 2026-11-01 20:00 to 2026-11-02 03:57 UTC)

- ChatGPT 429 from 20:01 to 03:43; Synthetic 429 from 01:16.
- 11,917 Synthetic 429s. Every body read "You've exceeded your subscription
  rate limits. Upgrade, or try again later" (quota, not concurrency), and
  `x-synthetic-quotas` read `weekly_percent_remaining=0`.
- After the owner reset ChatGPT at about 03:50, runs moved to ChatGPT:
  failover worked, but late and at full retry cost.

## Mechanism

**One health authority: the gateway's per-model breaker.** It is already
the member's health (per model, per provider), it sees every call from
every computer, and the guest already reads it: an open breaker answers
`provider P model M: circuit open (model unhealthy)` and the guest fails
over at once on "circuit open". No second breaker in the guest, no new
channel. Mirroring the search plane (`internal/gateway/searchplane`):

| Search plane | Model pool |
| --- | --- |
| health row per provider | `health.CircuitBreaker` per model (`CircuitBreakingProvider.ModelBreaker`) |
| `BackoffPolicy` exponential per class | `BreakerConfig.Backoff`: OpenTimeout on the first trip, doubling per failed probe, capped at 10 min; quota: 5 min doubling to 1 h (`CHOIR_MODEL_QUOTA_COOLDOWN_BASE` / `_MAX`) |
| success resets strikes | a successful half-open probe closes the breaker and resets `trips` |
| `EmptyStreakBeforeCooldown = 10` | `ConcurrencyStreakBeforeCooldown = 10` Synthetic concurrency 429s in a row, no success between |
| quota_limited benches at once | `provider.ErrUpstreamQuotaExhausted` trips the model on the first one (`CircuitBreaker.Trip`) |
| router skips cooling providers | the guest walks the pool; a cooling member answers "circuit open" unsent |
| probe when the cooldown ends | half-open admits one probe |

Classification at the provider, where the status, body and headers are known:

- Synthetic 429: quota when `x-synthetic-quotas` has a zero weekly
  percentage, or the body names a subscription/quota/upgrade and not
  "concurrent"; otherwise a concurrency limit (the onboarding default).
  Texts: `synthetic: quota exhausted: status 429 ... (sanitized)` and
  `synthetic: status 429 ... (concurrency limit, sanitized)`.
- ChatGPT non-2xx now goes through `logRejectedRequest` (redacted reason in
  the journal, like Synthetic's); a 429 whose reason names a usage limit or
  quota is typed quota, any other keeps `chatgpt: status 429 ... (sanitized)`.
- Other classes (5xx, timeouts, untyped 429s) count toward the breaker as
  before (5 in a row); the open duration now grows.

The guest, `callToolLoopProviderWithPoolRetries`:

- With a member left, any failover-class error returns at once and the run
  moves, except a Synthetic concurrency 429 and the gateway's own
  per-computer limits ("for autoputer"), which refuse every member alike
  and keep the same-model retry.
- On the last member the rate-limit backoff runs once more for every member
  that skipped its own (`skippedRetryRounds`), and with skipped rounds an
  open circuit is waited on too (the cooldown may end and admit a probe).
  A pool down everywhere waits at least as long as before; the run's
  existing transient-error loop (3 x 15 s) still follows.

## Failure modes and their tests

- Synthetic quota 429 never opens the model:
  `TestSyntheticQuota429CoolsTheModelOnTheFirstOne` (gateway),
  `TestSynthetic429Classification` (provider).
- Quota 429 waits the same-model backoff with a member left:
  `TestSyntheticQuotaRoutesToChatGPTWithNoRetryWait` (real delays, under 2 s).
- A concurrency burst under 10 demotes:
  `TestConcurrency429BurstUnderTheStreakDoesNotDemote`; the guest still
  retries the same model: `TestConcurrency429RetriesTheSameModelInAPool`.
- Cooldown never grows or never resets:
  `TestSuccessiveFailuresGrowTheCooldownAndASuccessRestores`.
- In-flight errors stack trips: `TestInFlightQuotaErrorsDoNotStackTrips`.
- All members down fails faster than before: `TestAllMembersDownKeepsTheBackoff`.
- ChatGPT quota does not route to Synthetic, or its reason is not logged:
  `TestChatGPTQuotaRoutesToSyntheticWithNoRetryWait`,
  `TestChatGPTQuotaCoolsOnlyThatModel`, `TestChatGPT429ReasonIsLoggedAndQuotaTyped`.
- The computer's own gateway limit moves the run:
  `TestGatewayComputerLimitRetriesTheSameModel`.

## Residual risks

- Breaker state is in memory: a gateway restart forgets a cooldown and
  re-learns it with one call per cooling member (the search plane persists
  to SQLite; not needed for the cutover).
- Each new run still starts on its hash-assigned member; a cooling member
  costs it one fast refusal. The guest does not reorder the pool.
- A run never goes back to an earlier member once it failed over; the
  last-member backoff waits on that member, not on a recovered earlier one.
- Quota markers are text: a provider that changes its 429 wording falls
  back to the concurrency (Synthetic) or plain-429 (ChatGPT) class, which is
  today's behavior.
- An old guest reading a new gateway retries a quota 429 once (5 s) before
  the open circuit moves it; a new guest reading an old gateway fails a
  Synthetic concurrency 429 over (its text lacks "concurrency limit").

## Rollback

`git revert` of the branch commit. Operational: `CHOIR_MODEL_QUOTA_COOLDOWN_BASE`
and `_MAX` shorten the quota cooldown without a deploy; the ops breaker
reset (`BreakerRegistry.Reset`, "<provider>/<model>") closes a model at once.
The guest security epoch rose 33 -> 34 with this change and never falls.
