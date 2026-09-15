# Second Pass: The Minimal Version

**Date:** 2026-09-15
**Status:** analysis — candidate revision of the ontology design, under
adversarial review
**Mutation class:** green (analysis; no runtime change)
**Predecessors:** `choir-event-driven-rlm-ontology-2026-09-15.md` and its
consensus review `choir-event-driven-rlm-ontology-consensus-2026-09-15.md`.
Where the review report prescribes machinery, this memo is the
counter-proposal; the two should be reconciled after the second panel.

## What happened

The consensus panel found eight real problems with the design and
prescribed standard distributed-systems machinery for each: durable
inbox records with lease/epoch claims and acks, a durable timer
subsystem, a typed work-obligation protocol, a compare-and-swap
admission primitive, an event-sourced saga with a legal-transition
reducer, desk/sub-RLM lifecycle ontology, delegation-vs-exercise
authority, phased cutover.

The problems are load-bearing. The machinery is not forced. Each
finding has a resolution that removes code instead of adding it. This
memo works through all eight.

## The two moves that do most of the work

**Move 1 — "delivered" means "in the state head."** Every actor already
has a durable processed-position (its state head; today,
`processed_at` in the actor log). Define delivery as the event appearing
in the addressed actor's state head. Then pending deliveries = tape
events addressed to actors minus incorporated events — a projection,
not a ledger. The dispatcher is a small loop over that projection: fire
an activation (idempotent by event id), retry until incorporated. The
actor's state head IS the acknowledgement; the retry IS the lease. No
inbox records, no epoch protocol, no separate watermark table.

**Move 2 — one live activation per actor.** A dispatch rule, not a
lock: the dispatcher never runs two activations of the same actor;
events arriving during an activation are incorporated by the next one.
Every actor becomes a serial processor — the actor model, 1986. This
dissolves concurrent-activation races, admission check-then-spawn races
(policy evaluated inside a serial actor cannot race itself), per-actor
ordering (tape order), and most of what the drainer's mutex did.

## The eight findings, minimally resolved

### P1. Delivery/crash boundary → Moves 1 + 2

Panel's version: addressed activation records, lease/epoch claims,
acks, per-actor watermarks, poison-event quarantine.

Minimal: delivered = in state head; pending = projection; dispatcher
retries until incorporated. Head-of-line blocking dissolves because
pending deliveries are a *set* — the dispatcher retries event 5 while
event 3 is still pending; there is no contiguous scalar to stall.
Poison events: retry N times, then emit `delivery_failed` addressed to
the event's sender — a policy decision, not a subsystem.

### P2. Silence and timers → one field: `not_before`

Panel's version: durable timer source, executor heartbeats/leases,
deadline obligations.

Minimal: a scheduled wake is an event that is not dispatch-eligible
until a timestamp. One optional field covers every case:

- deadline obligation → event with `not_before = deadline`, addressed
  to the obligee;
- executor starting external work → schedules its own suspicion event
  ("if no report by T, wake the owner to inspect me") — the tape is the
  watchdog; completing the work makes the suspicion event a no-op;
- retry backoff → re-dispatch scheduled with `not_before = now + delay`.

No timer subsystem: same tape, same dispatcher, eligibility check is
`not_before <= now`. Executor death is covered because the suspicion
event is already durable before the work starts.

### P3. Desk vs sub-RLM → one bit: persistence

Panel's version: durable identity, addressability, standing
obligations, admission policy, budgets, lifecycle contracts.

Sub-RLMs are already addressable via the messaging system (owner
correction, 2026-09-15). The remaining difference: a desk is an actor
you do not expect to terminate; a sub-RLM exists to emit a result.
Same kernel, same addressing, same event processing. Standing
obligations, admission policy, and budgets are actor state plus code —
not ontology.

### P4. Work schema → two fields: `work_id`, `status`

Panel's version: typed obligation protocol with work_id,
activation_id, attempt_id, settlement rule, cancellation/supersession,
terminal uniqueness.

Minimal: spawn carries a spawner-chosen `work_id`; the terminal event
carries a `status` enum. Blocked vs runnable, cancelled vs superseded,
retry vs duplicate, partial vs terminal — all ordinary events in the
work's stream; the projection folds the stream into current state.
"Terminal uniqueness" = first terminal event wins; later ones are
flagged by the projection. Policy, not machinery.

### P5. Fate saga → the steps stay, the coordination dissolves

The saga is four physical executor operations: freeze the capsule →
revoke its capability → destroy it → record the outcome. Those stay —
they are syscalls, and were never the problem. What dissolves is the
~1,074 lines of coordination: intents, acks, pending proposals,
watchdogs, restart reconcilers. Each step becomes trigger-event →
executor action → result-event; crash recovery is the generic dispatch
retry, not bespoke machinery. "Legal-transition reducer" is the
projection's fold function validating order — arguably unnecessary,
since the executor acts on the next expected event in tape order.

### P6. Admission → inside the serial actor (Move 2)

Panel's version: conditional-append / CAS on the canonical appender, or
a reservation projection with atomic claim.

Minimal: "engineering admits ≤1 effects-capable sub-RLM" is evaluated
by the engineering desk inside its own serial activation against its
own projection. It cannot race. The slot table's real function — global
arbitration — is only needed if a *cross-desk* admission invariant
exists; none has been named. If one is named later, the admitting
actor's serial processing is still the serialization point.

### P7. Delegation vs exercise → one bit in the grant

Panel's version: distinct exercise and delegation authority.

Minimal: a capability grant carries one flag — "may create capability
X" vs "has capability X." Resolved by trusted spawn code. Correct and
small; not machinery.

### P8. Migration → a script, not a subsystem

Panel's version: phased cutover, seeded reconciliation.

Keep as plan, not ontology: for each pending row in the old tables,
mint its equivalent event. Phasing is a property of the mission plan.

## Net accounting

Added: `not_before` field, `work_id`/`status` fields, may-delegate bit,
one dispatch rule (serial per actor), one definition (delivered = in
state head).

Removed: the panel's inbox/lease/ack protocol, timer subsystem, typed
obligation protocol, CAS admission primitive, saga continuation
machinery — and behind them the existing drainer, watchdogs, sweeps,
slot table, assignment object, control bindings, `/tell`.

## Residual hard points (honest list)

- **Duplicate effects.** Actor dies after a provider call succeeds but
  before the result event commits → retry repeats the call. Resolution
  direction: effects go through trusted code (executor/gateway) which
  dedupes by request identity — consistent with capabilities being the
  only effect path. The idempotency key needs specifying.
- **Hot-actor starvation.** Serial-per-actor plus retry could starve a
  desk under event storm. Needs a coalescing/batching rule in the
  dispatcher.
- **First timer for pre-cutover rows.** Seeded reconciliation must mint
  `not_before` events for existing deadlines or they never fire.
- **`delivery_failed` ownership.** Who is woken when an event can't be
  delivered — the sender? a computer error context? Policy decision,
  currently unnamed.
- **Activation context durability.** The state head proves
  incorporation, not mid-activation progress. A long activation that
  dies restarts from its last committed state — acceptable if cells
  commit intermediate state as events, which they already must for
  rewarm (C6/I17).
