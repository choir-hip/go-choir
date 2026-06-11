# Durable Actors: The Rearchitecture Conjecture — 2026-06-11

## Status

Conjecture-program artifact for a **hard-cutover rearchitecture** of the agent
runtime. Compiles three evidence passes (the architecture review v2, the
continuation-system dissection, and the concurrency/messaging audit) plus
founder direction from 2026-06-11:

- coagents are not subagents — no parents, no completion; they own their own
  lifecycles, and old trajectories may be re-warmed later
- agent messaging should be Go-channel-based with durable actor semantics; the
  database persists, Go delivers
- agents should be event-driven, not waiting/polling
- continuation determinism is unspecced control policy in code; audit it,
  skeptical by default
- incidental complexity here is creating bugs; cut hard, not gradually

Companion evidence: `docs/choir-architecture-review-next-moves-2026-06-11.md`
(v2), `docs/choir-conjecture-refactor-program-2026-06-11.md`, the three
handoffs of 2026-06-10.

---

## 0. Compressed thesis

The runtime built three ad-hoc systems where one well-known model belongs:

| Built (ad hoc) | What it actually is | Replace with |
|---|---|---|
| parent/child run trees + notifyParent | causality bolted onto spawning | trajectories + provenance edges |
| "channels" as DB tables polled per turn + wait_agent | a message bus reinvented on SQLite | Go-channel mailboxes, durable log behind them |
| RunContinuation synthesis | control policy hardcoded in Go | event-driven actors + durable work assignments |

The replacement is one model, not three: **the durable actor**.

> An agent is a long-lived actor: a goroutine with a Go-channel mailbox while
> resident, an event log + compacted memory while passivated. It never
> "completes" — it goes idle and can be re-warmed. It has no parent — it has
> provenance and trajectories. It never waits — it passivates, or keeps
> working, and messages reach it either way. Sending to a cold actor activates it.
>
> **The database remembers. Go delivers.**

Prior art, deliberately: Erlang/OTP processes and mailboxes, Orleans virtual
actors (activation on message, passivation on quiescence). This is not novel
machinery; the novelty was reinventing it worse.

---

## 1. Evidence: what the current design costs

All file:line receipts are in the review v2 and audit; summarized here.

### 1.1 The bug classes are structural, not incidental

- **Lost wakes:** a send does persist-message → enqueue-inbox → wake-waiters as
  three non-atomic steps (`channels.go:352-407`); a middle failure leaves a
  message durably stored that nobody is ever woken for. An agent in
  `wait_agent` times out on a message that exists.
- **Restart amnesia:** every in-flight run is marked *failed* on process
  restart (`recoverInterruptedRuns`, runtime.go:977-1007) even if the LLM call
  finished. Nothing resumes. This is the direct cost of holding lifecycle in
  goroutine closures instead of durable state.
- **notifyParent is fire-and-forget** (runtime.go:1197, 2454-2498): a child can
  be "completed but nobody knows." This bug *only exists because parents
  exist* — in an actor model the worker sends a structured update to a peer's
  mailbox, durably, idempotently.
- **Message leakage across lifecycles:** inbox deliveries are addressed to
  `agent_id` with no run/incarnation scoping; stale messages replay into later
  activity; crash between inject and mark-delivered double-delivers
  (runtime.go:1231-1273).
- **Check-then-act races:** co-super slot reuse (runtime.go:534-545),
  skip-level cast enforcement (tools_coagent.go:889-932).
- **Coagent invisibility:** the Universal Wire leak — work continuing on a
  document channel by a peer is invisible to run-tree accounting; a week of
  compensation commits (436490f4, 362a0ded, 46a8ece6, e5ec5f74) could not
  converge because the model cannot represent the situation.

### 1.2 The messaging surface is four overlapping mechanisms

`cast_agent` (fire-and-forget text), `cast_agent_update` (multi-recipient,
non-atomic across recipients), `submit_coagent_update` (structured, atomic,
idempotent — the good one, tools_worker_update.go), plus automatic
`notifyParent` — and three ways to learn a peer's result (channel wait, run
poll, inbox), with no guarantee they agree.

### 1.3 The continuation audit: unspecced control policy in Go

`internal/runtime/continuation.go` encodes decisions that appear in no spec:

| Hardcoded policy | Location |
|---|---|
| priority rule: app-adoption work beats mission work | SynthesizeRunContinuation:43-49 |
| default mission: `docs/mission-choir-grand-deformation-v0.md` | line 48 |
| adoption-status → authority-profile mapping | continuationFromAppAdoption:291-317 |
| adoption "impacts source" matched by **substring of adoption ID** | appAdoptionImpactsSource:267 |
| lease defaults/clamps (8h default, 24h cap, 4h mission, 1-2h adoption) | lines 96-101, 54, 297-314 |
| objective text normalization with synonym folding ("world"→"computer") | normalizeObjectiveText:331-361 |

Self-development has not actually worked yet, so this machinery has produced
**no verified value** — it is an attempt, not an asset. Verdict: **delete the
synthesis layer**. Salvage four mechanics that map cleanly onto the actor
model: memory compaction before handoff (→ passivation snapshot), objective
fingerprint dedup (→ message idempotency), bounded authority profiles (→ spawn
policy). Lease clamps are NOT salvaged — see the lease deferral below.
Everything decision-shaped about it
dies; decisions belong to agents and owners, reacting to events.

---

## 2. The target model

### 2.1 Actor = agent

One actor per agent identity, resident as a goroutine in its computer's
runtime process.

```
resident actor:
  identity     agent_id (+ incarnation_id per activation)
  mailbox      Go channel (buffered), typed messages — live the entire activation
  loop         wake → work (steps/loops/compactions, mailbox live) → passivate
  state        in-memory context window + working state

passivated actor:
  compacted memory snapshot (existing CompactRunMemory machinery)
  + durable message log tail
  + agent record (profile, channel/trajectory refs, policy)
```

**Vocabulary, precisely** — "turn" is ambiguous over tool-calling agent loops
and is not used in this design:

| Unit | Definition | Bound |
|---|---|---|
| **step** | one model inference (text and/or tool calls) | token/latency budget |
| **loop** | steps + tool executions until the agent yields control | step budget |
| **activation** | wake → passivation; may span hours, many loops, many compactions | evictable at any time (safe by construction) |
| **passivation** | compact + release goroutine; memory durable, identity persists | idle condition |

An activation is *not* a run and not an LLM turn: a single activation can be
hours of work across many compactions, sending and receiving updates
throughout. Budgets bound steps and loops; activations are bounded only by
eviction (below) and per-owner activation caps.

**Leases: deferred, by the same discipline applied to continuation.** A
lease (residency window with durations, clamps, grants, renewal) is policy
with no proven requirement today — so v1 ships **no lease concept**. What v1
keeps is the free part: **eviction safety** — the runtime may force-passivate
any resident actor at any moment (memory pressure, shutdown, or a future
lease), and this is deliberately identical to a graceful crash: memory is the
last compaction, backlog stays in the log, obligations stay open, the sweep
re-wakes. Zero new protocol states; proven by the spec's Evict action. Cost
control in v1 is therefore two knobs and no planner: step/token budgets
inside a loop, and per-owner activation caps across the fleet — the caps are
load-bearing for the liveness proof (bounded evictions), not optional policy.

**The future requirement that will define lease semantics when it arrives:**
leases are a QoS/pricing primitive, not a timeout — service tiers at
guaranteed price levels with graceful rate limits ("fast tier: activate now,
premium rate" vs. "batch tier: guaranteed completion by T, cheaper, no manual
restart"). The protocol already supports it: batch tier is deferred wake
(priority ordering of the sweep *is* the tier), and graceful rate limiting is
eviction + guaranteed re-wake instead of a hard 429. Tailor lease semantics
to that requirement when it emerges; do not pre-build them.

**At-least-once visibility is accepted slack, not a defect:** models handle
the occasional duplicate message fine; ledger effects stay exactly-once
(committed transactionally at send). Buying exactly-once visibility would
mean transactional LLM steps — impossible — or coordination machinery whose
failure modes are worse than a duplicate.

**Activation on send** (the Orleans move): sending to a non-resident actor
activates it — registry lookup, rehydrate from snapshot + log tail, deliver.
"Recipient not running" stops being a failure mode; the lost-wake and
orphaned-inbox bug classes become unrepresentable.

**Passivation on quiescence:** idle actors compact and release their
goroutine. This *is* the founder requirement "agents own their own lifecycle /
old trajectories may be re-warmed": rewarming is just activation with old
memory.

**No completion.** Actors don't complete. An *activation* ends (one bounded
residency under a lease). A *work item* completes (an assigned objective).
A *trajectory* settles (its artifact reached its settlement rule). The agent
remains, passivated, re-warmable. `recoverInterruptedRuns` → on boot, no
actors are resident; they reactivate lazily on first message (or eagerly for
trajectories marked live), and an interrupted activation is the actor's own problem
to assess on rewarm — not a blanket "failed."

**No parents.** Spawning an actor records a provenance edge
(`spawned_by_agent_id`, frozen, no control reads) and a trajectory membership.
Coagents are peers on a trajectory. Supervision, where genuinely needed
(restart policy, budget), is an explicit named relationship — Erlang
supervisors are a *policy* choice, not an ambient tree.

### 2.2 Messaging: Go delivers, the database remembers

One send primitive with one semantics:

```
send(to_agent_id, message):
  1. append message to durable log (idempotent by message_id)   ← memory
  2. deliver into resident mailbox, activating if cold          ← delivery
```

The durable log exists so a passivated/crashed actor can replay its tail on
activation — it is **never polled as a delivery mechanism**. Per-turn inbox
polling (`injectPendingInboxTurns`) is deleted. Cross-computer sends (user VM ↔
platform VM) go through the existing HTTP boundary into the remote process's
local send — location-explicit, same semantics.

Tool surface collapses to **one primitive**:

- **`update_coagent`** (rename + promotion of `submit_coagent_update`): the
  structured, idempotent, typed update. It is already 100%, not 80%: free-text
  coordination rides in `summary`/`notes`/`questions`, and an audit of every
  prompted `cast_agent` use today (verifier failure reports in co-super.md:8,
  vsuper corrective messages in vsuper.md:17, VText worker instructions in
  vtext.md:141) shows they are all *typed* messages with prose bodies —
  verification, directive, blocker — never genuinely untyped text. The `kind`
  enum grows `directive` and `assignment`; certain kinds also write ledger
  records (`assignment` → work item, `verification` → acceptance evidence), so
  the message and the durable control record are one act.

Explicitly deferred until proven needed: pub/sub (publish to a topic with many
subscribers) and first-class multi-recipient. If a second primitive ever
exists, it is one of those — not a "lightweight note." Until then,
multi-recipient is a loop over sends of the same idempotent update
(per-recipient atomicity, honestly).

Deleted: `cast_agent`, `cast_agent_update`, `wait_agent` (see below),
`notifyParent` (a worker's last act before passivating is an update_coagent to its
requester — durable, idempotent, no special machinery).

**No waiting.** `wait_agent` exists because agents are run-shaped and must
harvest results before their run ends. Actors don't have that problem: an
agent that needs a result either passivates (the result wakes it cold) or
keeps working on something else (the result steers it warm, §2.4). No blocked
goroutines, no timeout heuristics, no
"target_terminal_without_matching_message" tri-state. The fit with LLM loops
is exact: messages in, messages out, sleep when idle.

### 2.3 Causality: trajectories (unchanged from review v2)

The trajectory record (kind, subject refs, status, explicit settlement rule)
and durable work assignments carry everything parent/child carried:

| was | becomes |
|---|---|
| liveness = terminal state + ActiveChildRuns | trajectory settlement |
| child budgets per parent | active-actor / active-activation caps per trajectory + owner |
| cancellation cascade over the tree | cancel-by-trajectory |
| co-super slots keyed (parent, slot) | slot registry keyed (trajectory, slot), atomic claim |
| notifyParent | update_coagent to the requesting actor |
| Wire candidate ledger (missing) | publication trajectories + their work assignments |

`sourcecycled` reconciles on settlement, not run state; `maxProc=1` retires
when a multi-story cycle shows clean settlement accounting.

### 2.4 update_coagent as the wake primitive: how control flow works

Since updates are the only messages, they are also the only wake source — the
entire control flow of the system is agents updating each other. Reviewed
explicitly:

```
update_coagent(to, kind, payload):
  1. append to durable log, idempotent by update_id     (memory)
  2. if kind has a ledger effect, write it in the same
     transaction (assignment → work item, verification
     → acceptance evidence)                              (control record)
  3. deliver into recipient mailbox; if the actor is
     cold, activate it with the update as wake input    (wake)

recipient actor:
  COLD   → activation begins; the update is the opening input
  WARM   → the update is injected at the next step boundary of the
           live loop, as a steering message — it does NOT end or
           restart the activation
```

Both delivery cases matter. A cold actor is woken into a fresh activation. A
**warm actor keeps working**: incoming updates surface at step boundaries as
steering input — a verifier's failure report redirects the worker mid-flight;
an owner's directive re-aims a researcher without losing its context. One
activation can therefore *send and receive many updates*; an hours-long
activation with steering traffic is the normal case for substantial work, not
an edge case. (This replaces today's per-turn inbox polling with push delivery
into the live loop.)

**Cross-VM send (v2, first pair: super ↔ vsuper).** vsuper lives in the
candidate computer's runtime, super in the active computer's — the first real
remote pair. Mechanics: transactional **outbox** — the sender appends the
update to a durable outbox in the same transaction as its ledger effects; a
forwarder retries delivery over the existing HTTP boundary until acked; the
receiving runtime's send() dedupes on update_id, so retries are free. Same
semantics across the wire: at-least-once visibility, exactly-once effects.
Spec v2 adds two processes with independent log/resident state, a
lossy-network action, and the outbox retry loop — a module on top of spec v1,
not a change to it.

Passivation has one sharp requirement: the idle check and the mailbox-empty
check must be **atomic** — an actor that decides to passivate while a message
is in flight is the lost-wake bug reborn at a new layer. This is the first
property the R6 model-checking spec must cover.

The system is a graph of actors exchanging typed updates; "control flow" is
just which updates wake or steer whom. A worker finishing is an update
(kind=verification or findings) waking its requester. A blocker is an update
waking the agent that can unblock. An assignment is an update waking the
assignee. A mid-activation directive is the same update, landing warm. There
is no second mechanism.

**Liveness without a loop.** The skeptical question: without deterministic
"continue unless goal complete" logic, what guarantees long-running work
doesn't silently stall? Answer in the model's own terms: a trajectory settles
when its conjecture is decided — proven, refuted, or abandoned. Until then it
carries **open obligations** (undischarged work items, unanswered blockers,
unverified claims). Three properties make indefinite running safe without a
driver loop:

1. **Every activation ends in updates or idleness** — work propagates as long as any
   agent has something to say; the chain terminates exactly when the
   conjecture is decided, which is the *correct* termination condition, not a
   proxy for it.
2. **Stalls are observable, not prevented by control.** A stalled trajectory
   is one with open obligations and no resident actor holding them — a pure
   query against durable state. The watchdog that surfaces this to the owner
   (or to a supervisor agent whose *prompted job* is triage) is
   observability, not a decision engine. It never synthesizes objectives.
3. **Re-warming is cheap**, so "nobody is working on this" is a recoverable
   state, not a failure — the owner or a triage agent sends one update and
   the trajectory resumes.

This is the upshot of deleting continuations: the system runs indefinitely
because agents update each other until the work is done — or rather, until
the conjecture is proven.

### 2.5 What replaces continuation synthesis

Event-driven reaction instead of Go-coded planning:

- **App adoption:** state transitions emit events; the owning vsuper/co-super
  actor is a registered listener and gets the event as a mailbox message; *it*
  decides the next move. The Go priority list dies.
- **Mission resumption:** an explicit wake source — owner action, schedule, or
  a supervisor actor whose *job* (prompted, not hardcoded) is deciding what
  deserves attention. The hardcoded mission-doc fallback dies.
- **Work assignment:** spawning or tasking an actor creates a durable work
  item on a trajectory (objective, bounded authority, lease, fingerprint).
  This is the continuation record's good half, re-keyed off the run tree.

Named risk (taken deliberately): until event wiring is complete the system
loses autonomous overnight progression. Given self-development has not yet
actually worked, what is being given up is unproven; what is gained is a
substrate it can actually work on.

---

## 3. Conjecture ledger

### R1 — Durable actor semantics eliminate the messaging bug classes structurally

- **Claim:** activation-on-send + single send path + idempotent durable log
  make lost wakes, orphaned inboxes, double delivery, and completed-but-unknown
  states unrepresentable rather than merely less likely.
- **Test:** kill -9 the runtime mid-activation under multi-agent load; on restart,
  sends to all involved agents reactivate them with correct memory; zero
  stranded messages; the Wire multi-story cycle (review v2 falsifier) runs
  clean with `maxProc > 1`.
- **Hyperthesis edge:** in-flight LLM steps at crash are genuinely lost work
  (acceptable — the actor reassesses on rewarm); single-process Go channels
  don't span computers, so cross-VM sends remain HTTP at an explicit boundary
  — if much coagent traffic turns out to be cross-VM, the win shrinks.
- **Scope:** within one computer's runtime; cross-computer messaging keeps
  current transport behind the same send().

### R2 — Agents never complete; passivation/rewarm replaces completion

- **Claim:** removing terminal states from *agents* (activations, work items,
  and trajectories still terminate) matches the real product semantics (VText
  agents revisit documents; supers persist; Wire stories get revised) and
  deletes the false-liveness class.
- **Test:** re-warm a week-old VText agent on its document and verify
  competent continuation from compacted memory; verify passivation reclaims
  goroutines/memory under load.
- **Hyperthesis edge:** cost discipline moves entirely to passivation policy
  and per-owner activation caps — a zombie actor that never passivates, or activation
  storms (one message fanning out into mass rewarming), are the new failure
  modes. Bound: activation budgets per owner; passivation deadline tested
  explicitly.
- **Scope:** all coagents. (Whether *runs* survive as a user-facing concept is
  presentation; internally they become activations.)

### R3 — Deleting continuation synthesis loses nothing proven

- **Claim:** every behavior the synthesis layer was meant to provide is either
  unproven (autonomous self-development) or better expressed as event-driven
  reaction + durable work items.
- **Test:** run one app-adoption flow end-to-end on event-driven wiring
  (propose → verify → promote/rollback) with no SynthesizeRunContinuation in
  the binary.
- **Hyperthesis edge:** there may be quiet dependencies on continuation events
  in acceptance evidence ("continuation-level", run_acceptance.go:1012-1014,
  AGENTS.md) and the Trace UI; these must be re-pointed at work items in the
  same cutover or verifier discipline silently weakens.
- **Scope:** the synthesis/decision layer. Compaction, fingerprints, bounded
  profiles, leases are retained under the actor model.

### R4 — One message primitive, doubling as the wake primitive

- **Claim:** `update_coagent` as the *sole* agent-to-agent message — and
  therefore the sole wake source — removes the four-mechanism ambiguity,
  makes results single-sourced, and makes control flow legible (the system's
  behavior is fully described by who updates whom with what kind).
- **Test:** grep-level: no remaining callers of cast_agent /
  cast_agent_update / notifyParent. Behavioral: a vsuper coordinating two
  co-supers sees every result exactly once, in its mailbox, across a process
  restart; a multi-day trajectory runs to settlement with no wake source other
  than updates.
- **Hyperthesis edge:** (1) prose pressure — if typed kinds don't fit real
  coordination, agents will stuff everything into notes (decorative
  structure); watch the kind distribution. (2) Silent stall — liveness now
  rests on the open-obligations query and stall surfacing (§2.4); if
  obligations are modeled too loosely, a stalled trajectory looks settled. The
  falsifier: a trajectory that stops progressing while showing zero open
  obligations.
- **Scope:** agent-to-agent. Human-to-agent and external ingress unchanged.
  Pub/sub and multi-recipient deferred until a real need is evidenced.

### R5 — Hard cutover beats incremental migration

- **Claim:** the dual-model window (old run-tree + new actors coexisting) costs
  more than a focused cutover, because every interim feature and every agent
  reading the code must reconcile two causality models — the exact tax this
  refactor exists to remove.
- **Test:** the cutover lands as one program (≈6 PRs, §4) on a branch; the
  full test suite + the Wire falsifier cycle + one adoption flow gate the
  route switch; rollback is the previous deploy.
- **Hyperthesis edge:** the runtime package is the blast radius (~everything
  in internal/runtime touches runs); 50+ tests assert on ParentRunID; the
  riskiest single semantic is co-super slot sequencing (runtime.go:549-703).
  Bound: slots get their own migration step and test; historical
  `parent_loop_id` data is frozen, never migrated.
- **Scope:** internal/runtime + internal/store + sourcecycled reconcile.
  platformd/corpusd, gateway, vmctl untouched.

---

## 4. The cutover program

One branch, ordered PRs, each gated on tests; route-switch at the end.
This program is also the §15 ConjectureRecord proof mission (review v2, N5).

0. **Spec first.** Write and model-check the TLA+ spec of the actor protocol
   — send / activate / deliver / steer / passivate — **before any Go**. The
   spec is the design dry-run: iterate the protocol where iteration costs
   minutes (a TLC run) instead of weeks (a refactor). Checked invariants, at
   minimum: no lost wake (every appended message is eventually delivered into
   some activation), idempotent delivery (no double-processing beyond
   update_id dedup), atomic passivation (no idle decision with a message in
   flight), settlement soundness (a settled trajectory has no open
   obligations). Lives in `specs/` in the repo; TLC runs in CI so the spec
   stays load-bearing instead of decorative. Every later PR that changes
   protocol behavior changes the spec first.

1. **Actor core.** Mailbox (Go chan) + registry + activation/passivation +
   durable message log with idempotent append. Compaction machinery rewired as
   the passivation snapshot. No behavior change yet — new package,
   `internal/actor` or similar.
2. **Trajectory model.** Trajectory record + settlement rules as data +
   `trajectory_id` on runs/activations + work items (port of continuation record
   mechanics: objective, bounded profile, lease, fingerprint).
3. **Messaging cutover.** send() as the single path; `update_coagent` rename +
   promotion; delete inbox polling, wait_agent, notifyParent,
   cast_agent_update; spawned-worker results become update_coagent calls.
   Slot registry keyed (trajectory, slot) with atomic claim.
4. **Lifecycle cutover.** executeRun closure → actor activation loop; delete
   recoverInterruptedRuns blanket-fail (boot = cold actors, lazy activation);
   run leases die with runs (no lease concept in v1 — eviction + activation
   caps instead); cancel-by-trajectory replaces
   CancelRunGraph.
5. **Continuation deletion.** Remove SynthesizeRunContinuation and friends;
   event-driven adoption wiring; re-point acceptance evidence and Trace UI at
   work items; `/api/continuations` returns 410 or shims to work items.
6. **Wire on settlement.** sourcecycled reconciles on trajectory settlement;
   raise `maxProc`; run the multi-story falsifier cycle. **This run is the
   evidence gate for the route switch.**

Side PRs (independent): `platformd → corpusd` rename; docs/glossary update
(actor, mailbox, activation, passivation, rewarm, trajectory, work item,
settlement; retire "continuation", disambiguate "channel" → *mailbox* for
delivery, *document/trajectory channel* for the product concept — the word
currently names a Go primitive, a DB table, and a product surface, which is
how "we built a shitty concurrency layer on top of a language that has
channels" went unnoticed).

Explicitly deferred (unchanged from review v2): capsules/Nucleus,
MutationTransaction coordinator, autoputer code rename — design track,
not this cutover.

---

## 5. What dies, what survives

**Dies:** parent/child control semantics; notifyParent; RunContinuation
synthesis and its hardcoded policy; wait_agent; per-turn inbox polling;
cast_agent and cast_agent_update; blanket fail-on-restart; "completion" as an
agent concept; the DB-as-message-bus.

**Survives, relocated:** memory compaction (→ passivation); objective
fingerprints (→ message/work-item idempotency); bounded authority profiles
(→ spawn/activation policy); channel_messages table
(→ durable log behind delivery, replay-only); submit_coagent_update's design
(→ the one primitive, renamed); VText tool-scope enforcement (untouched —
already correct); all of platformd, sources, vmctl, gateway.

**Becomes possible:** processor parallelism on evidence; honest liveness
("what is this trajectory waiting on?" is a query); rewarming month-old
stories; a substrate self-development can actually run on — because changing
the system can finally be expressed as events, work items, and settlements
the system itself can observe.

---

## 6. The proof-theoretic horizon (target: formally verified by June 2027)

Not yet metabolized in the docs until now: the conjecture system's real upshot
is that Choir should think and frame in **proof-theoretic terms**. Initially a
metaphor; the target is to make it literal — formal verification of the
system, end to end, by June 2027.

### 6.1 The dictionary (metaphor today, mechanics tomorrow)

| Choir object | Proof-theoretic reading |
|---|---|
| conjecture | proposition to be decided |
| work item / open obligation | proof obligation (open goal) |
| update_coagent | inference step / message in a proof search |
| verifier attestation | proof checker accepting a step |
| assertion with receipts | lemma admitted with its derivation |
| invariant | theorem in the ambient theory (or axiom, if assumed) |
| promotion gate | admission rule: nothing enters the theory unproven |
| trajectory settlement | QED / refutation / abandonment of the goal |
| heresy | inconsistency: a usable statement not derivable from the theory |
| hyperthesis edge | known incompleteness of the current proof system |
| bounded authority profiles | a capability lattice (who may apply which rules) |
| stalled trajectory | an open goal no prover currently holds |

Under this reading, §2.4's liveness story is just proof search: agents are
provers exchanging derivation steps; the system terminates on a decision, not
on a step count; the stall detector checks for open goals without provers.

### 6.2 What "formally verify everything" decomposes into

This rearchitecture is itself the enabling move — goroutine-closure soup is
unverifiable, while a message-passing actor system with a small state
machine per object is exactly what model checkers were built for.

| Layer | Property to verify | Plausible tooling |
|---|---|---|
| Actor runtime (mailboxes, activation, passivation, idempotent log) | no lost wakes, no double delivery, crash-recovery soundness | TLA+ / P model of send-activate-deliver; the spec is small once §2.2 is the only path |
| Business state machines (adoption, settlement, work items, slots) | only legal transitions; settlement implies obligations discharged; slot uniqueness | exhaustive state-machine checking; these become finite once extracted from closures |
| Self-improvement convergence | every self-change passes the same gates (the §0 fixed point); progress measure on open obligations; no gate bypass path exists | termination/progress arguments; gate completeness as an invariant over the promotion code paths |
| Threat model | capability confinement (the authority lattice is never escalated by message content); external signals are data, not instructions; non-interference between owners | information-flow typing over the update kinds; the one-primitive design makes the trust boundary auditable in one place |

### 6.3 How the verification practice works (method, staged)

Formal verification is two different activities, staged deliberately:

1. **Design-level model checking — now (cutover step 0).** A small abstract
   model of the protocol in TLA+; the TLC checker exhaustively explores every
   interleaving of a small instance (e.g., 3 actors, 4 messages) and reports
   any state violating an invariant, with the exact trace that reaches it.
   This is the AWS practice (S3/DynamoDB teams model-check designs in TLA+
   and have credited it with finding bugs requiring 35-step interleavings no
   test or review would catch). Cost: weeks, not years. Catches: the entire
   bug class in §1.1.
2. **Binding the code to the model — during/after cutover.** The model is not
   the code; the gap is closed by (a) keeping the implementation's protocol
   surface isomorphic to the spec (one send path makes this possible), and
   (b) **trace validation**: the Go runtime logs protocol events
   (send/deliver/activate/passivate) and a checker replays those traces
   against the TLA+ spec — production becomes a continuous conformance test
   (the MongoDB/CCF practice).
3. **Selective code-level proof — the 2027 stretch.** Machine-checked proofs
   (Dafny/Coq-class tools) only for the few hundred lines where it pays:
   the capability lattice (no message content can escalate authority) and
   the promotion gate (no path into canonical state bypasses verification).
   Full-codebase proof is a seL4-scale effort and is *not* the plan; verified
   protocol + verified gates + gated unverified cognition is.

Enterprise trust framing (true, not just sellable): "the concurrency model
that runs your agents is model-checked the way AWS model-checks S3, the
authority boundary is machine-proved, and production traces are continuously
validated against the spec." No agent vendor can currently say this.

### 6.4 Conjecture R6 — verification-readiness is a design constraint now

- **Claim:** designing the cutover so each component is a small,
  message-driven state machine makes June-2027 formal verification feasible;
  designed any other way, it is not.
- **Test:** cutover step 0 as specified — the TLA+ spec of
  send/activate/deliver/steer/passivate is written and model-checked *before*
  the Go implementation, lives in `specs/`, runs in CI, and the
  implementation conforms (eventually by trace validation). If that spec
  stays under a few hundred lines, the claim survives its first contact.
- **Hyperthesis edge:** the LLM steps themselves are not formally verifiable —
  verification covers the *harness*: what agents may do, what wakes whom, what
  enters canonical state. Scope creep toward "verify the model's reasoning"
  is the failure mode; the boundary (verified harness, gated unverified
  cognition) must be explicit in every claim. Second edge: proof-theoretic
  vocabulary can go decorative exactly like conjecture YAML — the §15-style
  gate applies to this framing too.
- **Scope:** harness, state machines, gates, and trust boundaries — not model
  cognition. June 2027 is the target for the harness story, asserted only
  when the layers in §6.2 have machine-checked artifacts.

Lfg.
