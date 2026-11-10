# The gateway's per-computer inference and search bucket is a self-imposed limit (2026-11-10)

Mutation class: red (gateway/provider calls); a deletion. Rollback: revert the
commit.

## Problem

`internal/gateway/ratelimit.go` (`PerAutoputerRateLimiter`, default 60
requests a minute per computer per route scope; nix sets no
`GATEWAY_RATE_LIMIT_*`) refused our own computers' model and search calls. No
provider limit or owner ruling backs the number. Owner rule (2026-11-10): "Too
many model calls is a code problem to fix, not a limit to impose." What stays:
backing off after a provider's real refusal, owner-set spending caps, bounds
on what strangers can cause, deadlines. Volume must be logged so a spike is
seen.

## Evidence

In the last 24 hours the bucket refused 584 calls, all from the paper's
computer (peak 218 in one hour). It was the only limiter in the gateway that
refused our own callers outright. Provider refusals are already handled by
`circuit_breaker.go`, quota holds, the Devin pacer and the paid budgets
(`paid_budget.go`), which stay.

## Decision per use of the limiter

| Use | Where (pre-change) | Decision | Why |
|---|---|---|---|
| Model inference | `handlers.go` HandleInference "Step 2" | REMOVE | throttles our own computers' model calls |
| Search | `handlers.go` HandleSearch "Step 2" (scope `search`) | REMOVE | throttles our own computers' search calls |
| OpenAI-compatible chat/models | `openai_compat.go` `authorizeOpenAICompatibleRequest` | REMOVE | model calls of our own computers |
| Source API | `source_api.go` HandleSourceAPI (scope `source_api`) | REMOVE | search/source calls of our own computers; authenticated, allow-listed upstream |
| `/health` fields `rate_limit_max_requests`, `rate_limit_window_ms` | `handlers.go` | REMOVE | reported the removed inference bucket; `git grep` finds no reader in frontend, scripts or nix (only docs) |
| Anthropic relay | `anthropic_relay.go` `rateLimiter.Record(rateLimitBucketKey(..., "anthropic_relay"))` | KEEP, behaviour untouched | the relay and its caps are an owner grant |
| Host fetch | `host_fetch.go` own `NewPerAutoputerRateLimiter(600/min)` | KEEP | not a model or search call; bounds an allow-listed fetch hop with its own in-flight cap |
| `cmd/gateway/main.go`, `config.go` `LoadRateLimiterConfig` | construction of the one handler limiter | KEEP | the relay still takes its counter from it |

`ratelimit.go` therefore keeps two users (the relay, host fetch) and is not
deleted; the handler's `rateLimiter` field now documents that only the relay
draws on it. `auth`'s limiter (`internal/auth/rate_limit.go`) guards
unauthenticated sign-in and is a different package, untouched.

No route reached by strangers lost its bound: every removed use sat behind
`authenticateAutoputer` (bearer bound to a computer), so the callers were
our own computers.

## Replacement: visibility

`internal/gateway/call_tally.go`: one log line per computer and provider each
hour, in the style of `jev hour summary` and `devin pacing hour=`:

`gateway: calls hour=2026-11-10T14:00Z computer=<id> provider=<name> calls=N input_tokens=N`

Calls are counted at admission (inference, OpenAI-compatible chat, `search`,
`source_api`); input tokens are added on non-OpenAI-compatible inference
success (the OpenAI-compatible route reports calls only). The window closes on
the first call after the hour. It refuses nothing and delays nothing.

## Failure list for the removal

1. A runaway caller now reaches the provider and may trip the provider's
   quota for every computer. Responding mechanisms, all kept: the circuit
   breaker (`circuit_breaker.go`) opens on real provider failures and quota
   holds back off after a real refusal; the Devin pacer sheds by token budget;
   `paid_budget.go` stops Bedrock and River at the owner's daily dollar
   budgets. The hourly line names the computer and provider so the spike is
   seen in the journal, and the fix is in the calling code.
2. A stranger-reachable route losing its only bound: none found (see above).
3. Health or readers breaking: no reader of the two health fields found;
   the fields are omitempty and removed.
4. The relay depending on the limiter's counter: it still gets the same
   limiter, constructed the same way, with the same key; unchanged.
5. Guest code that string-matches a gateway 429 / "rate limit" (for example
   `subCallRateLimited`) keeps working for provider-originated 429s; the
   bucket's own text ("rate limit exceeded for autoputer ...") simply no
   longer occurs on these routes.

## Follow-up: the tally's own failure cases (review round 1)

Review found the first tally counted a call under the provider it was
admitted to but its tokens under the provider that served it, and left a
closed hour unlogged until the next call. Fixes and their failure cases:

1. One label per call. `Served` attributes the call and its input tokens to
   the provider that served it; if a fallback served it, the call moves from
   the admitted provider to the serving one. A call that fails stays with the
   provider it was admitted to and carries no tokens.
2. A closed hour is logged by a single timer owned by the tally (one goroutine
   per process, started with the handler, none per call). Failure cases:
   - Timer firing during a call: both take the closed hour out of the map
     under the same mutex; whichever is first logs it, the other finds
     nothing. Lines are written after the lock is released, so logging never
     holds it.
   - Double flush of one hour (a call and the timer, or two ticks): the
     counts are removed when taken, so one hour is logged once.
   - Shutdown mid-hour: `stop` ends the timer and drops the partial hour,
     which is not logged. A restart therefore loses at most the last partial
     hour of counts. This is accepted: the tally is visibility, not a ledger.
   - An idle gateway: the timer logs the hour that closed with no further
     call needed. An hour with no calls has no lines.

## Tests

The tests that asserted the bucket's refusals on inference and search were
removed or rewritten; new tests prove 150 inference and 150 search calls from
one computer are not refused with the relay limiter at 1 a minute, health no
longer reports a limit, and the hourly line is emitted once per computer and
provider.
