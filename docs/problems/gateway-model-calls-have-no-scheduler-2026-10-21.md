# The Gateway Rejects Model Calls Instead of Scheduling Them (RES-11)

Date: 2026-10-21. Source: the 2026-10-11 fan-out incident
([research-fanout-overruns-computer-model-budget-2026-10-11.md](../archive/problems/research-fanout-overruns-computer-model-budget-2026-10-11.md)),
the checklist item RES-11 and owner decision DEC-07 ("track spending, never
cap"), and a re-read of the code at origin/main 26c4e1ed0. Station:
[FP7](../definitions/choir-launch-fp7-gateway-identity-2026-10-21.md).
Mutation class of any fix: **red** (gateway and provider calls). Problem
first; no fix in the commit that wrote this record. Status: scheduler built,
off by default (station FP7); to be flipped after the staging proof.

## The problem in one sentence

The gateway meters model calls with a fixed one-minute window per computer
and answers the 61st call with a 429, so it has no notion of fairness
between computers, of the provider's total capacity, or of waiting; the
guest compensates with its own retry loops and pacing, which is where the
2026-10-11 storm came from.

## Evidence (code, read 2026-10-21)

- `internal/gateway/ratelimit.go:9-16, 88-116`: `PerAutoputerRateLimiter`
  is a fixed-window counter, 60 requests per one-minute window per bucket
  key, keyed by computer id (inference), `computer:search`, and the Jev
  scopes. The 61st call in a window is refused with 429 and
  `Retry-After` set to the seconds left in the window
  (`handlers.go:358-373`). Nothing queues.
- A fixed window admits 60 calls in the first second of a window and 60
  more in the first second of the next, and every refused client is told to
  return at the same instant (the window's end), so waiting callers wake
  together.
- The gateway has no concept of the provider's capacity. N computers may
  each send 60 calls a minute to one provider account; the sum is
  unbounded by anything but the upstream's own refusals, which open the
  per-provider and per-model circuit breakers for every computer
  (`internal/gateway/circuit_breaker.go`; the 2026-10 incidents in
  `docs/learn/03-gateway-and-providers.md` section 4).
- The guest paces itself because the gateway does not: a 1.5 second spacer
  between research child calls (`internal/agentcore/research_children.go:30-37`), a wait of
  up to ten `Retry-After` windows per call
  (`internal/gatewayruntime/provider.go:218-262`), then tool-loop retries.
  Two clocks (the guest's pacing and the gateway's window) decide when a
  call runs, and neither knows the other.
- No priority exists between a desk's call and a research child's call from
  the same computer; the 2026-10-11 record, item 4, shows Texture's one turn
  waiting behind hundreds of child calls.
- Search and Jev judgments draw on their own buckets of the same limiter
  and are not model calls in this sense; this record is about inference
  (`/provider/v1/inference` and the OpenAI-compatible chat adapter, which
  draws on its own scope bucket of the same limiter,
  `openai_compat.go:233-252`).

## Direction already ruled

Fairness, not spending caps (owner, DEC-07 and "track spending, don't
cap"). A computer may use as much as it needs; what the gateway owes the
others is that a heavy computer cannot starve a light one, and what it owes
the provider is a bound on concurrent calls.

## Ways the repair can fail (written before the code)

1. The scheduler stalls: a slot leaks (a panic, a cancelled caller, a
   provider that never returns), the pool drains, and every computer's
   model calls hang. This is the worst case because it takes everything
   down at once.
2. A heavy computer still starves a light one (the property the scheduler
   exists to give).
3. A light computer waits behind a heavy one's queue (head-of-line
   blocking across computers).
4. A queued call outlives its caller: the guest times out or disconnects
   and the call still runs, spending a provider call nobody reads.
5. A queue grows without bound under a runaway loop (memory), or a call
   waits forever instead of being told to come back.
6. Streaming calls hold a slot for their whole stream, and long streams
   crowd out short calls.
7. The scheduler replaces the 60-per-minute backstop and a runaway caller
   now reaches the provider at full speed.
8. Rollout is all-or-nothing: no way to turn it off without a deploy of old
   code.
9. Search and Jev calls are accidentally queued behind model calls.
10. The wait is invisible: nothing says how long calls queued, so a
    regression is only found when the paper slows.

## Review finding, batch 1 (2026-10-21, candidate b80a9bcac)

Found by both panel reviewers (non-blocking), confirmed by reading the code.

11. **The queue wait is outside the call's deadline.** With the scheduler
    on, a call may wait up to 120 s for a slot (`defaultModelQueueWait`),
    and only then does the handler start its ten-minute `inferenceTimeout`.
    The gateway's `SERVER_WRITE_TIMEOUT` and the guest's
    `gatewayClientTimeout` are both 10m30s. A call that queued more than
    30 s and then ran long is cut by the write timeout (the guest sees EOF)
    before the gateway's own deadline fires, so the provider is still billed
    for an answer nobody receives. Repair: the deadline starts when the call
    asks for a slot, so wait plus call stay inside `inferenceTimeout`, which
    is inside both 10m30s bounds. The scheduler is off by default, so this
    is a precondition of turning it on, not a live defect.

## Heresy delta

- Discovered: two clocks (guest pacing and gateway window) for one decision.
- Introduced: none yet.
- Repaired: none yet.
