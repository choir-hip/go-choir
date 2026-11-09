# Pool routing walks members already known to be dead (2026-11-09)

Status: documented, fix lands in the next commit. Class: red (provider routing).

## Owner report (2026-11-09, verbatim)

"the model fallback is an issue. It seems like it's cycling through all these
different models, and I didn't realize that it wasn't intelligently
auto-routing based on availability yet. Like, it shouldn't cycle through all
these models that we already know in advance are not going to work. It should
go straight to the one that does work, which currently is the Crouton Alpha.
So basically we should be using Crouton Alpha for almost everything right now,
and we can also use Synthetic DeepSeek right now, but I want to use that
seldomly, or basically save it for getting the timing of things, because we're
going to basically need to use it as like a benchmark for how long things take
to actually run."

## Evidence (owner screenshot, model trail on Texture documents)

- Document 1: flint -> iceman (known exhausted) -> beam (pool fallback) ->
  crouton (pool fallback); and flint -> nemotron (quota failover) -> gpt-6-luna
  (quota failover) -> synthetic DeepSeek (quota failover) -> iceman (quota
  failover) -> beam -> crouton.
- Management on another document: beam -> crouton -> flint -> luna (known
  exhausted).

Each turn starts on the member the thread hashed to and walks the pool,
through members that failed on the previous turn, and through members already
in the ledger.

## Mechanism

1. Thread to member. `modelpolicy.Assign` (internal/modelpolicy/model_policy.go:345)
   hashes the thread key (`ThreadKey`: the document's Texture agent id, else the
   trajectory) with FNV-32 and `weightedIndex` (:399) picks a member by
   `SlotWeight`. `rotate` (:373) makes the ordered list: assigned member, then
   the rest of the pool in order, wrapping. A document keeps its member; threads
   spread evenly (default pool: iceman, beam, crouton, flint, nemotron, luna at
   weight 1 each, internal/modelcatalog/catalog.go:405; Synthetic is
   `FallbackOnly`, slot weight 0). Assignment never consults availability, so
   5 of 6 threads start on a member that may be dead.
2. Availability state. Only one exists: `QuotaLedger`
   (internal/toolregistry/toolloop.go:431), model id to "out of quota until".
   It is written only at `ledger.Mark` (:1020), and only for a typed
   `ProviderQuotaError`. It lives on the runtime (`rt.quotaLedger`,
   internal/agentcore/runtime.go:321), shared by that computer's runs, in
   memory (a restart forgets it). Reset time is the provider's, else a 2 minute
   hold (`providerQuotaUnknownHold`, :486).
3. Known-exhausted skip. At the top of every iteration (:851) only the active
   model is checked against the ledger; if marked, `switchOnQuota` (:704)
   moves to the first later member of the order not in the ledger. That is the
   "(known exhausted)" hop in the trail. It lands on the next member in order,
   which is dead too if its death was not a typed quota.
4. Non-quota failure is not remembered. A 429 that is not typed quota, a 5xx,
   a timeout or an open circuit goes through `IsModelFailoverError` (:1700) to
   `advanceFallback` (:783), which marks the member only in the per-call
   `left` map (:786). The next call, the next document and the next computer
   turn start from the hash again and fail on the same member again. That is
   the "(pool fallback)" chain: iceman, beam, nemotron fail, every turn.
5. `advanceFallback` takes the next member in order without a ledger check
   (:1088), so a pool fallback can step onto a member already marked exhausted
   by another run, then a "known exhausted" hop moves it again.
6. Ordering. The pool leads with iceman and beam, which fail; crouton is third.

## Root cause

Availability is knowledge about a provider route, not about a call. The loop
keeps it in two places that do not meet: the ledger (typed quota only, checked
for the active model only) and the per-call `left` map (all other failures,
forgotten at the end of the call). The assignment hash ignores both. So every
turn re-learns, by failing, what the computer already knew.

## Design

One mechanism: the ledger is the computer's availability record.

- Quota marks stay (`Mark`, until the reset time).
- A failover-class failure (`IsModelFailoverError`, after the call's own
  retries, not on our cancellation) demotes the member for the whole computer:
  `Demote` holds it for 1 minute, doubling per consecutive failure, capped at
  15 minutes. A success on the member clears its strikes and any hold
  (`Succeed`). No setting; the durations are constants like
  `providerQuotaUnknownHold`.
- A pool failover picks the first later member the ledger does not hold; if the
  ledger holds all of them it takes the next in order, so a run is never
  refused for want of a candidate (the last-resort is still a call).
- The start of a call is already guarded at the loop top (:851); with failures
  recorded it now skips every demoted member, not only quota ones.
- Pool order (owner 2026-11-09): the default pool leads with openrouter
  stealth/crouton-alpha, the only member assigned threads. The other members
  (iceman, beam, flint, nemotron, luna) stay behind it as `FallbackOnly`, so
  they serve only when crouton is unavailable and are skipped while demoted.
  Synthetic DeepSeek V4.1 Flash 1017 stays `FallbackOnly`, last: the benchmark
  model, used seldom.

## Failure modes the tests cover

1. A member demoted by one run is called again by the next run on the same
   ledger.
2. A pool failover steps onto a member the ledger holds.
3. A demotion never expires, or never grows on repeated failure.
4. A success does not clear the hold, so a recovered member stays demoted.
5. Our own cancellation demotes a healthy model.
6. Every member held: the run is refused instead of trying the active one.
7. The default pool still hashes threads onto iceman, beam, flint, nemotron or
   luna, or Synthetic gets a thread.

## Not covered

`modelpolicy/tools_model_verify.go` walks its own failover list; it does not
use the loop's ledger and is unchanged.
