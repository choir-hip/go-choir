# Second Pass: The Minimal Version

> **Unratified design — not a Definition, not executable authority.** Candidate
> revision under owner review; the ontology cutover decision is open (see
> `docs/current-situation-2026-09-22.md`, open question 2). PICL below is the
> pre-rename name for precommitment records. *Stale as of 2026-09-22.*


**Date:** 2026-09-15
**Status:** analysis — candidate revision of the ontology design.
Amended to v2 after a second adversarial consensus round
(`docs/reports/choir-rlm-ontology-minimal-consensus-2026-09-15.md`):
the compression survived; several mechanisms gained panel-forced
clauses, marked *[r2]* inline.
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

**Move 1 — "delivered" means "in the state head," committed atomically.**
Every actor already has a durable processed-position (its state head;
today, `processed_at` in the actor log). Define delivery as the event
appearing in the addressed actor's state head. Then pending deliveries =
tape events addressed to actors minus incorporated events — a
projection, not a ledger. The dispatcher is a small loop over that
projection: fire an activation, retry until incorporated. The actor's
state head IS the acknowledgement; the retry IS the lease. No inbox
records, no watermark table.

*[r2: two clauses the panel forced. (a) **Fenced atomic commit** — an
activation commits {emitted events + new state head} in ONE append,
conditional on the head/epoch being what the activation started from.
The per-actor conditional append is the lease/epoch, relocated into the
head append — and it fixes today's real MarkProcessed-before-
SaveSnapshot hazard as a side effect. Without it, a dispatcher restart
double-activates and a mid-commit crash marks delivered what never
persisted. (b) **Ordering** — set-parallelism across actors only;
within an actor, tape order among eligible events. Retry accounting is
itself tape events (`dispatch_attempted`/`delivery_failed`) so poison
detection survives restart.]*

**Move 2 — one live activation per actor, fenced.** A dispatch rule,
not a lock: the dispatcher never runs two activations of the same
actor; events arriving during an activation are incorporated by the
next one. Every actor becomes a serial processor — the actor model,
1986. This dissolves concurrent-activation races, admission
check-then-spawn races (policy evaluated inside a serial actor cannot
race itself), per-actor ordering (tape order), and most of what the
drainer's mutex did.

*[r2: the fence is the conditional head-append from Move 1 — a killed
or stale activation's commit fails its epoch check, so preemption is
safe. Interrupts (cancel/revoke) are addressed to the executor actor,
which kills the activation — they do not queue behind the busy actor's
stream.]*

*[owner correction, 2026-09-15: there are no synchronous sub-RLM calls.
Sub-RLM calls are async casts only — spawn is admission, the result is
a later message event, and the parent either continues other work or
ends its activation and is re-woken by the reply. This matches the
reference RLM harness: prime-agent's `rlm.spawn` "returns immediately
after task admission... it never waits for or returns the child's
answer"; results arrive only via `agent_message` events; the child
registry survives kernel restart; and events arriving during a live
activation are delivered as steering at turn boundaries, not queued
for the next activation and not interrupting a running tool. The
round-2 "self-deadlock" finding dissolves: no parent ever blocks
awaiting a child. Model calls inside a cell remain synchronous —
sync effects in-cell, async casts across actors.]*

## The eight findings, minimally resolved
### P1. Delivery/crash boundary → Moves 1 + 2

Panel's version: addressed activation records, lease/epoch claims,
acks, per-actor watermarks, poison-event quarantine.

Minimal: delivered = in state head; pending = projection; dispatcher
retries until incorporated. Head-of-line blocking dissolves because
pending deliveries are a set *across actors* — the dispatcher retries
event 5 for actor B while event 3 for actor A is still pending. Poison
events: retry N times (attempts recorded as tape events), then emit
`delivery_failed` addressed to a named error-sink actor — a policy
decision, not a subsystem.

*[r2: UNSOUND as originally written — see the fenced-commit and
ordering clauses in Moves 1–2. With them, the panel's guarantee is
preserved without an inbox table.]*

### P2. Silence and timers → one field: `not_before`, fired by the dispatcher

Panel's version: durable timer source, executor heartbeats/leases,
deadline obligations.

Minimal: a scheduled wake is an **unaddressed** event carrying
`not_before` and its intended addressee. The dispatcher keeps a
due-index (min-`not_before`), self-wakes at the next due time, and when
one comes due **mints the addressed wake event at the then-head**. Due
order = tape order; per-actor FIFO survives; there is no separate
timer store — the timer wheel relocates into the dispatcher's
next-wake computation.

- deadline obligation → scheduled event; when due, the dispatcher mints
  `deadline_fired` addressed to the **arbiter/executor actor** — never
  to the obligee, which may be the wedged party;
- executor starting external work → appends its own suspicion event
  *before* starting (append-before-effect); completing the work makes
  the fired suspicion a no-op;
- retry backoff → the dispatcher schedules the re-dispatch with
  `not_before = now + delay`.

*[r2: the original text put `not_before` on addressed events and
assumed the tape itself was the producer. Panel breaks: a field
doesn't mint wakes on a quiet tape; a future event at a per-actor head
blocks later eligible ones; a deadline addressed to the wedged obligee
can never fire. The three clauses above are the fix — still no timer
subsystem, but the dispatcher's due-index is admitted to be the
relocated wheel.]*

### P3. Desk vs sub-RLM → one bit: persistence

Panel's version: durable identity, addressability, standing
obligations, admission policy, budgets, lifecycle contracts.

Sub-RLMs are already addressable via the messaging system (owner
correction, 2026-09-15). The remaining difference: a desk is an actor
you do not expect to terminate; a sub-RLM exists to emit a result.
Same kernel, same addressing, same event processing. Standing
obligations, admission policy, and budgets are actor state plus code —
not ontology.

*[r2: mostly SOUND. One rule added: events addressed to a terminated
actor fold as no-ops — otherwise a late cancel retries forever into a
false `delivery_failed`. Obligation-liveness (a crashed sub-RLM with
open work gets re-driven) is the work projection's job under P4, not
the actor's.]*

*[owner correction: the calling convention is cast-only, matching
prime-agent — `rlm.spawn` = admission event returning a handle,
`agent_message` = result event, `list_subagents`/poll = projection
read, turn ends while children run. A sub-RLM's "result" is a message
event addressed to the spawner's context, which is also what makes it
addressable: the messaging system is the return channel.]*

### P4. Work schema → `work_id`, `attempt_id`, `status`

Panel's version: typed obligation protocol with work_id,
activation_id, attempt_id, settlement rule, cancellation/supersession,
terminal uniqueness.

Minimal: spawn carries a spawner-chosen `work_id` and an `attempt_id`
(or its spawn-event ref); the terminal event carries a `status` enum.
Blocked vs runnable, cancelled vs superseded, retry vs duplicate,
partial vs terminal — all ordinary events in the work's stream; the
projection folds the stream into current state.

*[r2: UNSOUND as originally written — "first terminal wins" lets a
stale attempt's late failure beat a live attempt's success. Forced
clauses: `attempt_id` on work events; **only the latest attempt
settles**; earlier terminals fold as late evidence; cancel intent
takes precedence over in-flight success. Still no typed protocol —
three fields and a fold rule.]*

### P5. Fate saga → the steps stay, the continuer moves

The saga is four physical executor operations: freeze the capsule →
revoke its capability → destroy it → record the outcome. Those stay —
they are syscalls, and were never the problem.

*[r2: UNSOUND as originally written — the most code-grounded finding
of the round. A frozen capsule refuses every operation, so the
continuer cannot be the worker; `resumeStrandedFrozenAssignmentCommit`
exists precisely for that. Revised minimal version:]*

- Fate steps are events; the **continuer is the executor actor**,
  driven from the work stream — never the frozen worker.
- The terminal **proposal is staged before freeze** (an event), so a
  post-freeze crash loses nothing.
- The executor mints the terminal event only after the revoke ack;
  cancel intent takes precedence over a late success report.
- The "legal-transition reducer" is the work stream's **fold
  function** — small, named, not optional. Pending work =
  unincorporated events ∪ fold-declared continuations.
- `CapsuleAbsent` receipts make destroy retryable.

What still dies: watchdogs, resume sweeps, pending-proposal side
tables, restart reconcilers — the continuation obligation is derivable
from the stream, and the continuer is an ordinary actor.

### P6. Admission → a capability on one serial actor

Panel's version: conditional-append / CAS on the canonical appender, or
a reservation projection with atomic claim.

*[r2: UNSOUND as originally written — parents X and Y can both check
"0 live" and both mint spawn events; no serial point sees both. The
computer-wide invariant is real (one live effects capsule). Revised
minimal version:]*

Admission is a **capability**: the exclusive grant "may mint
effects-capable spawn" lives on exactly one actor (the engineering
desk). Spawns requiring that capability are *requested from* the
owner, whose serial fold is the arbitration point — the CAS relocated
into the fold. Cross-desk invariants get one named arbiter actor each.
Model-level check-then-spawn cannot race because the check and the
mint are inside one serial incorporation.

### P7. Delegation vs exercise → one bit in the grant

Panel's version: distinct exercise and delegation authority.

Minimal: a capability grant carries one flag — "may create capability
X" vs "has capability X." Resolved by trusted spawn code. Correct and
small; not machinery. *[r2: SOUND, unanimously — the one finding no
panelist broke.]*

### P8. Migration → a script inside a write fence

Panel's version: phased cutover, seeded reconciliation.

Keep as plan, not ontology: for each pending row in the old tables,
mint its equivalent event — with deterministic mint identity (re-run
safe), `not_before` seeding for live deadlines, and a final audit.
*[r2: the script is only correct inside a write fence — quiesce or
dual-read the old writers, or rows committed between scan and cutover
are stranded. Mission-plan phasing, not ontology — but a named
correctness requirement.]*

Added: `not_before`, `work_id`, `attempt_id`, `status`,
`may_delegate`, `activation_epoch` fields; serial-per-actor dispatch
rule; delivered = in-head definition; fenced atomic commit; the rules
listed below.

*[r2 — forced rules: latest-attempt-settles with cancel precedence;
kill-not-queue interrupts; capability-bearing spawns go through the
capability owner; sync sub-RLM calls return values; late events to
terminated actors are no-ops; append-suspicion-before-effect;
scheduled events are unaddressed and fired by the dispatcher's
due-index.]*

Relocated, not deleted (small, named): dispatcher due-index (the timer
wheel), work-stream fold (the legal-transition reducer),
executor-as-fate-continuer (the watchdog), per-actor conditional
head-append (the lease/CAS).

Removed: the panel's inbox/lease/ack *tables*, timer *subsystem*, typed
obligation *protocol*, CAS admission *primitive*, saga *coordinator* —
and behind them the existing drainer, watchdogs, sweeps, slot table,
assignment object, control bindings, `/tell`, roster, model_policy
routing.

## Residual hard points (honest list)

- **Duplicate effects.** Actor dies after a provider call succeeds but

  before the result event commits → retry repeats the call. Resolution
  direction: effects go through trusted code (executor/gateway) which
  dedupes by request identity — consistent with capabilities being the
  only effect path. The idempotency key needs specifying. *[r2: the
  fenced commit shrinks this window to effects-in-flight but cannot
  eliminate it — the external world doesn't participate in our
  commits.]*
- **Hot-actor starvation.** Serial-per-actor plus retry could starve a
  desk under event storm. Needs a coalescing/batching rule in the
  dispatcher.
- **First timer for pre-cutover rows.** Seeded reconciliation must mint
  `not_before` events for existing deadlines or they never fire.
- **`delivery_failed` ownership.** Panel leaned to a named error-sink
  actor rather than the sender (which may be a dead sub-RLM).
- **Activation context durability.** The state head proves
  incorporation, not mid-activation progress. A long activation that
  dies restarts from its last committed state — acceptable if cells
  commit intermediate state as events, which they already must for
  rewarm (C6/I17).
- **Dispatcher topology.** Single dispatcher per computer assumed;
  multi-dispatcher failover needs the epoch durable and fenced — same
  mechanism, stronger requirement. Whether the dispatcher is itself an
  actor on the tape (its head = the dispatch position) or a thin
  process loop is unresolved.

## Learning records (PICL)

*[owner direction, 2026-09-17: the tape's second job is the learning
substrate — Predictive In-Context Learning. A frozen model commits a
prospective prediction before acquiring decision-relevant evidence,
compares it with the observation, records discrepancy and revision,
and retrieves the record later. Prospective commitment preserves the
before-state that retrospective reflection destroys.]*

The tape is the PICL substrate by construction — prediction-before-
observation is two events in order, and the append-only record is the
frozen commitment chat-based systems have to engineer. Two
qualifications the consensus panel surfaced (2026-09-17): tape order
proves P preceded O but not that the predictor hadn't already seen O
through another actor or unrecorded context — the epistemic boundary
lives in context construction, so a prediction is PICL-valid only if
committed before its resolution is delivered to the predicting actor's
stream; and a preserved prefix can be replayed outcome-blind to recover
an equivalent prediction, so what online commitment uniquely adds is
the sampled, staked record — not information-theoretic privilege. What
PICL adds is discipline and measure, not machinery:

- **Retrieval runs over the object graph, not the tape.** Learning
  records materialize as OG objects with provenance edges back to
  their source events — queryable from the RLM REPL like any other
  object. The tape stays the authority; the graph is the query
  surface. (Owner correction, 2026-09-17.)
- **Prediction events.** A spawn or action may carry `expected` —
  anticipated result/consequences + confidence. Every async cast is
  then a PICL episode *frame* for free — the result event resolves it —
  but the prediction itself is inference the design mandates nowhere;
  a task description is not a stated expectation. A desk fanning out
  1000 sub-RLMs produces 1000 prediction-resolution pairs — delegation-
  calibration data (the parent's model of the child's competence, not
  world modeling; the resolver is a different learner). Model calls
  likewise: `choir.Call` with an expected shape yields per-model
  calibration — what model selection should learn from.
- **The learning projection is two-stage.** A deterministic fold pairs
  `expected`-carrying events with their resolutions into unresolved
  candidate pairs on the object graph — projections stay pure, no model
  calls. An asynchronous Curator desk consumes candidate pairs off the
  critical dispatch path, computes discrepancy/revision, and appends
  `learning_record_minted` events. On result delivery, context
  construction either reattaches `expected` or deliberately withholds
  it — a stated policy, not an accident. Retrieval ranks by expected
  corrective value; records are queryable via the REPL but never
  auto-injected into cell context until the experimental program
  justifies consumption. Records carry `actor_id` + `model_id`;
  cross-actor retrieval is an experiment, not a default.
- **Material-consequence predictions** formalize the mutation-class
  ceremony: red/black actions require a committed `expected`
  consequences record; the discrepancy is the auditable surface.
  Green/yellow don't — PICL says strategically placed commitments,
  not universal ones.
- **A desk's persistence is its calibration history** — a better
  answer to "why desks persist" than standing obligations.

Risks the paper names that the design must not hide: anchoring
(commitments defended instead of revised — the hidden-commitment
variant is a context-construction choice, not a tape problem); trivial
hedging (predictions scored on specificity, not presence); retrieval
noise at scale. The mechanism is a hypothesis (H1–H6, falsifiable), so
PICL records are additive events, never load-bearing for correctness.
The tape doesn't need PICL; PICL needs the tape.
