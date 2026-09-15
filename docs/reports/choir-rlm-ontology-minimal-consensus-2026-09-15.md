# Consensus Review Round 2: The Minimal Counter-Proposal

**Date:** 2026-09-15
**Subject:** `docs/designs/choir-event-driven-rlm-ontology-minimal-2026-09-15.md`
(commit `75f179fd`), reviewed adversarially against the round-1 report
**Method:** agentic-consensus runner, convergent mode with adversarial
prompt (concrete interleaving/crash-point required for any UNSOUND
verdict); 12 usable verdicts (opencode failed).
**Mutation class:** green (analysis; no runtime change)
**Artifacts:** `.agentic-consensus/rlm-ontology-review/run2/*.out`

## Verdict

The minimal memo's *direction* survived; several of its *mechanisms* did
not. The panel's verdict pattern: **adopt the compression, restore the
guarantees it dropped — as fields, rules, and folds, not subsystems.**

Per-finding tally (12 panelists):

| Finding | SOUND | SOUND-BUT-INCOMPLETE | UNSOUND |
|---|---|---|---|
| P1 delivered = state head + retry | 0 | 1 | 11 |
| P2 `not_before` timers | 2 | 4 | 6 |
| P3 desk/sub-RLM = persistence bit | 7 | 4 | 1 |
| P4 `work_id` + `status` | 2 | 2 | 8 |
| P5 fate saga dissolves | 0 | 1 | 11 |
| P6 admission inside serial actor | 2 | 3 | 7 |
| P7 may-delegate bit | 12 | 0 | 0 |
| P8 migration as script | 3 | 5 | 4 |

## What broke, and the convergent minimal fix for each

### P1 — UNSOUND: retry is not a lease, and today's ack isn't atomic

Three independent, code-grounded breaks:

1. **No fencing across dispatcher restart.** D1 fires activation A1;
   D1 dies or partitions; D2 sees the event un-incorporated and fires
   A2. Two live activations, duplicate effects. "One live activation
   per actor" is a live-process rule, not a crash fence. (11 panelists)
2. **The ack isn't atomic with the state.** Today's `processed_at` is
   set by `MarkProcessed` *before* `SaveSnapshot` — a crash between
   them marks the event delivered while its effects were never
   persisted. The codebase already knows this cut and emits special
   recovery occurrences for it (ling, sol, codex; grounded in
   `internal/actor/log_sqlite.go`, `actor.go`, and the documented
   hazard in `docs/definitions/choir-continuous-texture-supervision-2026-08-07.md`).
3. **Set-retry contradicts per-actor tape order.** "Retry event 5 while
   event 3 is pending" breaks same-actor ordering (E3=grant, E5=revoke:
   revoke lands as no-op, grant lands after — revocation lost). And a
   poison event never leaves the pending set; retry counts in
   dispatcher memory don't survive restart.

**Convergent fix — two clauses, no inbox table:**

- **Fenced atomic commit.** An activation commits {emitted events + new
  state head} in ONE append, conditional on the head/epoch being what
  the activation started from. The per-actor conditional append IS the
  lease/epoch — relocated into the actor's head append, not a separate
  ledger (claude, cursor, sol, codex). This also fixes the real
  MarkProcessed-before-SaveSnapshot bug as a side effect.
- **Ordering rule:** set-parallelism across actors; tape order among
  eligible events within an actor (devin). Retry accounting as tape
  events (`dispatch_attempted`/`delivery_failed`) so poison detection
  survives restart (devin, nemotron).

### P2 — split: the field is right, the producer was assumed away

Breaks: (a) a field doesn't mint wakes — on a quiet tape nothing
re-evaluates eligibility at T (sol, cursor, nemotron, codex); (b) a
future event at a per-actor head blocks later eligible events, or
skipping it breaks tape order (nearly everyone); (c) a deadline
addressed to the wedged obligee can never fire (cursor, claude).

**Convergent fix — the timer wheel relocates into the dispatcher:**

- Scheduled events are **not addressed**. The dispatcher keeps a
  due-index (min-`not_before`), self-wakes at the next due time, and
  when one comes due **mints the addressed wake event at the
  then-head**. Due order = tape order; per-actor FIFO survives; no
  separate timer store (claude, grok46, devin).
- Deadline/suspicion events are addressed to a **different actor than
  the obligee** (executor/arbiter) — a wedged actor cannot block its
  own deadline (cursor, claude).
- Suspicion events are appended **before** the external work starts —
  append-before-effect is the rule that makes executor death
  observable (luna, gemini).

### P3 — mostly SOUND: persistence bit holds, plus one rule

Late events to a terminated actor must fold as no-ops (else a cancel
arriving after a sub-RLM's terminal event retries forever → false
`delivery_failed`). Obligation-liveness ("crashed sub-RLM with open
work gets re-driven") is the work projection's job (P4), not the
actor's (luna, sol, claude).

### P4 — UNSOUND: stale attempt wins

Attempt A times out; attempt B starts and succeeds; A's late `failed`
lands first; "first terminal wins" fails the work. (9 panelists, same
interleaving.)

**Convergent fix — one more field + one rule:** `attempt_id` (or
spawn-event ref) on work events; **only the latest attempt settles**;
earlier terminals fold as late evidence. Cancel intent takes precedence
over in-flight success. Still no typed protocol — two fields and a
fold rule.

### P5 — UNSOUND: the saga's continuer can't be the frozen worker

The most code-grounded finding of the round. Generic dispatch retry
fails because:

- A **frozen capsule refuses every operation** — re-firing an
  activation into the worker livelocks. The continuer must be a
  *different* actor (executor/fate owner) driven from the staged
  proposal — which is exactly why `resumeStrandedFrozenAssignmentCommit`
  exists (all 11).
- `PendingProposal` must be staged **before** freeze, or a post-freeze
  crash loses the report payload.
- Revoke-ack must precede terminal visibility; cancel-vs-complete is
  two causal chains no single actor's tape order serializes;
  `CapsuleAbsent` receipts make destroy retryable.

**Convergent fix — the saga shrinks to a fold + an addressee:**

- Fate steps stay events; the **continuer is the executor actor**,
  never the frozen worker.
- The "legal-transition reducer" is the work stream's **fold function**
  — small, named, not optional. Pending work = unincorporated events
  ∪ fold-declared continuations (devin).
- Proposal staged before freeze; executor mints terminal only after
  revoke ack; cancel intent wins over late success.
- What still dies: watchdogs, resume sweeps, pending-proposal side
  tables, restart reconcilers — the continuation obligation is
  derivable from the stream.

### P6 — UNSOUND as stated, SOUND with one capability rule

Cross-spawner race: parents X and Y both check "0 live" and both mint
spawn events — no serial point sees both. A computer-wide invariant
does exist (one live effects capsule; `co_super_slots`,
`reclaimSupersededAssignmentCapsules`).

**Convergent fix — admission is a capability:** the exclusive grant
"may mint effects-capable spawn" lives on exactly one actor (the
engineering desk). Spawns requiring that capability are *requested
from* the owner, whose serial fold is the arbitration point — the CAS
relocated into the fold (cursor, devin, codex). Cross-desk invariants
get one named arbiter actor each.

### P7 — SOUND, unanimously. Keep the bit.

### P8 — script yes, but inside a write fence

Migration needs: quiesce or dual-read the old writers, deterministic
mint identity (re-run safe), `not_before` seeding for live deadlines,
and a final audit. Mission-plan phasing, not ontology — but a named
correctness requirement, not a hand-wave.

## New findings not in P1–P8

- **Self-deadlock on synchronous sub-RLM calls** (claude, codex): if a
  cell's sub-call result arrives as a mailbox event, the parent can't
  incorporate it while blocked waiting — deadlock. Fix: synchronous
  sub-RLM results are **return values inside the activation**, not
  mailbox deliveries; or activations yield/passivate when awaiting.
  Required for "sub-RLM calls = model calls" to hold.
- **Interrupts kill, not queue** (claude, cursor, sol, devin, grok46,
  nemotron): cancel/revoke addressed to the executor actor, which kills
  the activation; the fenced commit makes the killed activation's
  output harmless. Today's cancel path already works this way
  (`CancelRun` outside the worker's mailbox).
- **The atomic commit fixes a real current bug**: the
  MarkProcessed-before-SaveSnapshot cut is a documented hazard in
  today's code, not just a design gap.

## Net accounting, revised

The minimal memo's claim survives in amended form: **no separate inbox
table, no separate timer wheel, no separate slot table, no saga
coordinator.** What is forced:

Fields: `not_before`, `work_id`, `attempt_id`, `status`,
`may_delegate`, `activation_epoch`.

Rules: serial-per-actor; delivered = in-head; atomic fenced commit;
latest-attempt-settles with cancel precedence; kill-not-queue
interrupts; capability-bearing spawns go through the capability owner;
sync sub-RLM calls return values; late events to terminated actors are
no-ops; append-suspicion-before-effect.

Relocated mechanisms (small, named, not deletable): dispatcher
due-index (the timer wheel), work-stream fold (the legal-transition
reducer), executor-as-fate-continuer (the watchdog), per-actor
conditional head-append (the lease/CAS).

Still deleted: drainer, watchdogs, sweeps, slot table, assignment
object, control bindings, `/tell`, roster, model_policy routing, inbox
ledgers, timer stores, saga coordinators.

## Open questions the panel did not resolve

1. Single dispatcher per computer is assumed throughout; multi-
   dispatcher failover needs the epoch to be durable and fenced — same
   mechanism, stronger requirement.
2. `delivery_failed` ownership: sender vs a computer error context.
   Panel leaned to a named error sink actor.
3. Hot-actor coalescing policy under event storms — named, undesigned.
4. Whether the dispatcher is itself an actor on the tape (elegant:
   its state head is the dispatch position) or a thin process loop
   (simpler: no self-dispatch paradox). Both appeared; unresolved.

## Disposition

The minimal memo is amended in place (v2): each P-finding now carries
its panel-forced clause. The design that emerges is still a deletion
project — but the deletions are now gated on named replacement
invariants that are fields, rules, and folds rather than the first
panel's subsystems or the memo's original hand-waves.
