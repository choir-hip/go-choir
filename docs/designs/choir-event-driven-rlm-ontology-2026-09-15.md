# Design: Event-Driven RLM Ontology

**Date:** 2026-09-15
**Status:** design document — not a Definition, not executable authority.
Amended after a 12-agent consensus review
(`docs/reports/choir-event-driven-rlm-ontology-consensus-2026-09-15.md`):
direction endorsed, several "only" claims revised. Amendments are marked
*[panel]* inline; original text retained where the panel confirmed it.
**Mutation class:** green (analysis; no runtime change)
**Origin:** owner-directed orientation review after the engineering-carrier
mission stalled at the P5 roster gate and the owner reported that the
roles→desks refactor had not actually happened in the code.

## Why this document exists

The week of 2026-09-08..15 produced ~271 commits, most of them inside one
mission (the RLM engineering carrier), and most of *those* spent repairing a
family of defects that share one cause: durable state transitions whose
continuations lived in process-local machinery. The mission's own escalation
(`docs/memo-activation-wake-authority-substrate-2026-09-13.md`) named the
family and asked for a substrate decision. This document is the substrate
answer, restated in the system's own ontology after an orientation pass over
the code, the docs, and the week's evidence.

## What the current system actually is

Orientation findings (verified in source, 2026-09-15):

- **The rename is a value veneer.** `agentprofile.Super = "management"`,
  `CoSuper = "engineering"`. The symbols, file names, object kinds, digest
  domains, command IDs, and the `co_super_slots` SQL table still speak the old
  names. `internal/vocabmigrate` maps values, not identifiers. Reading any
  file requires mentally running the V1→V2 map.
- **Three parallel work ledgers.** Work items, `CoSuperAssignment` objects,
  and `co_super_slots` rows all record "who owes what work." The slot table
  still gates budgets, verifier sequencing, and `update_coagent` authority on
  the older coagent path while the assignment path runs beside it.
- **Two parallel delivery systems.** Owner instructions
  (`QueueLifecycleOwnerInstruction`, the `/tell` API) and lifecycle control
  bindings (`lifecycle_control_bindings` merged into run metadata) are two
  implementations of "durable request for attention."
- **Five partial wake mechanisms.** Boot reconcile, Super selection sweep,
  fate watchdog, stranded-proposal resume sweeps, and the drainer's packet
  consumption each rescue one signature of pending state. The six-strand
  defect family is what happens when a transition commits durable state but
  mints its continuation in a process-local actor.
- **A bespoke settlement saga.** `cosuper_assignment_fate.go` (~1,074 lines)
  implements freeze→revoke→commit as a multi-step saga with intent refs,
  ack refs, pending proposals, watchdogs, and restart reconcilers — and still
  strands at `revoked → terminal commit`.
- **Model policy is host-side.** `model_policy.toml` maps role→provider+model
  at spawn time; per-run selection required overlay files on the host store,
  which is what made the roster harness necessary and what poisoned three of
  its arms (base-policy serving, silent and loud by turns).
- **A second control channel into docs.** The `/tell` endpoint lets
  instructions reach a document's agent without a revision — a side channel
  that bypasses the artifact and produced the desk-prose-vs-reducer-state
  divergence class.

## The design

Five nouns. All load-bearing. None invented for the occasion.

### 1. The event tape is the only durable record and the only channel

The canonical per-computer event chain already exists. In this design it is
also the work queue: every request for attention, every delegation, every
state transition, every settlement is an event. There is no separate
instruction queue, no control-binding merge, no assignment ledger, no slot
table.

### 2. Durable dispatch is the only wake authority

*[panel: the original text claimed one scalar cursor was sufficient. The
panel showed that is a restatement, not a substrate — see F1/F2 in the
review report. Revised claim below.]*

Per computer, a durable **dispatch position** on the tape plus a
transactional handoff: an event addressed to an actor produces a durable
addressed-activation record, claimed under a lease/epoch, acknowledged on
completion or made retryable. Crash → resume from the position. Event
identities make re-dispatch recognizable; the handoff makes it
idempotent. Per-actor ordering and progress are a mailbox projection —
per-actor watermarks — so one stalled actor cannot head-of-line-block
unrelated ones, and a permanently failing event is quarantined rather
than silently skipped or allowed to stall the position.

There is no scanner, no watchdog, no sweep, no drainer: **pending work is
not discovered, it is delivered** — the event is the wake, and the
durable handoff makes the wake survive the process.

Two event producers are first-class substrate, not emergent behavior:

- **Executor-owned observations.** The harness that owns the process
  table emits `capsule_lost`, `execution_failed`, `provider_unavailable`,
  and executor lease/heartbeat expiry. Silence is not an event; a wedged
  or dead executor must still produce one.
- **Durable timers.** Deadline obligations (owner-visible expiry,
  supersession windows, retry delays) are legitimate semantic state; a
  durable timer source mints the wake event when the deadline elapses.
  Executor timeouts govern execution liveness; they do not define
  semantic truth — but semantic deadlines exist and are events.

This replaces the memo's "transition-minted recovery occurrences" with
the smaller truth: the tape already contains the occurrences; what was
missing is the durable dispatch position and the producers that keep it
fed.

### 3. An activation is an RLM spawned by an event carrying a context

One actor type, recursive. A desk is a persistent RLM with a role and a
capability set. A sub-RLM is the same thing spawned from inside a cell. The
spawn payload is a **context** — an initial REPL state: task prose, doc refs,
evidence refs, capability grants, prior state. A spawn with a task is a
worker; a spawn without one is a stateful sub-computation. The system does
not distinguish them.

The parent's context construction is code. That is what makes sub-RLMs
transparent to the spawner and opaque to everyone else: upstream sees the
desk's own events — spawn context in, settlement event out — never the
internal fan-out. A research desk 1000 agents wide is 1000 ordinary
activations whose events are scoped to the research context. Management and
engineering never learn it happened.

### 4. Desks and sub-RLMs share one activation kernel; their differences are bindings

*[panel: "capabilities are the only difference" was too strong — see F3,
F7. Revised claim below.]*

One actor type, one activation mechanism. A desk additionally carries
durable bindings: stable identity, addressability, standing obligations,
admission policy, budgets, reactivation semantics. A sub-RLM may be
anonymous, caller-scoped, and disposable. The differences are declarative
bindings on the same kernel — not separate role-specific loops, and not
capabilities alone.

Capabilities attenuate at spawn, with one correction: **exercise
authority and delegation authority are distinct.** Management may grant
an effects capability it is authorized to delegate but not to wield;
otherwise it must ambiently hold every capability it might delegate.

Spawn context carries grant *references* and data — never serialized
capability handles or physical bindings. Trusted activation setup
resolves current authority at spawn; handles do not survive revocation
or rewarm (C6/I17). Capsule quotas and executor bindings are resolved by
trusted spawn code from the context's claims, not embedded in it.

"Assignment" as a separate object disappears: spawning an RLM with a
capability binding resolved against a context *is* the assignment. The
binding is recorded by the spawn event.

Admission control is per-desk policy at the spawn boundary — engineering
may admit one effects-capable sub-RLM at a time while research fans out a
thousand — but policy evaluation needs an atomic primitive: a
conditional-append / compare-and-swap on the canonical appender, or a
reservation projection with atomic claim. Model-level check-then-spawn
races.

### 5. Work tracking is a typed event protocol plus projection

*[panel: "spawn minus terminal" is a liveness view, not a work schema —
see F4. Revised claim below.]*

Work obligations are a typed event protocol on the tape: stable
`work_id`, `activation_id`, `attempt_id`, settlement rule,
cancellation/supersession, terminal uniqueness. The current state of a
piece of work is *derived* — a canonical, rebuildable projection — never
stored twice and never independently writable.

"Open work" = spawn events without matching terminal events is the base
liveness view; the projection additionally distinguishes blocked vs
runnable, failed vs abandoned, cancelled vs superseded, retry vs
duplicate, partial vs terminal, compensation-pending vs ordinary
failure. "What is engineering doing" = filter by role and scope.

This kills the run-terminal-vs-fate-terminal divergence by construction:
there is one stream, and a run finishing is just an event in it. A worker
reading state reads the projection; the projection is a function of the
tape. (Deployed projections can lag or carry reducer bugs — they are
rebuildable indexes, not a second authority.)

## The document is the control surface

Owner input enters as revisions: the prompt bar produces V0, texture's first
response is V1, direct edits are revisions. There is no `/tell` API, no owner
instruction queue, no second channel. Supervision is reading diffs; every
state change an owner can cause is a document version. This is deliberately
off the chat-paradigm distribution: it makes owner intent auditable by
construction, because intent is an artifact.

Actor input is events. Delegation between desks is an event addressed to a
context, not a packet merged into run metadata.

## The gateway is a model registry, not a policy engine

The gateway publishes a catalog — registered models, wire shapes, limits —
and enforces entitlement: a call for an unregistered model fails. Selection
moves into the cell:

```go
models := choir.Models()                       // catalog read
resp  := choir.Call("deepseek-v4.1-flash", req) // per-call selection
```

Consequences:

- Model policy shrinks to the registered set plus spend caps. Registration is
  the authority; selection within the set is free.
- Multimodel evals are ordinary code: a cell fans one context across N models
  and compares settlement events. No roster harness, no overlay files, no
  per-run policy swap — the eval *is* the product path.
- Prompts stay model-free by construction: the prompt never names a model
  because the model is chosen in the cell.
- Sub-RLM calls and model calls are the same mechanism at different grain —
  which is what "transparent subagents" meant all along.

## What gets deleted

*[panel: each row now names the replacement invariant that must exist
before the deletion is safe — see F5/F6 in the review report.]*

| Current machinery | Replaced by |
|---|---|
| `co_super_slots` table + budget/sequencing enforcers | spawn-boundary admission policy evaluated through a conditional-append/CAS primitive on the canonical appender *[panel F6]* |
| `CoSuperAssignment` object + fate saga + watchdogs + resume sweeps | event-sourced saga: spawn event + capability binding + typed legal-transition reducer + executor receipts *[panel F5 — the saga survives as events; only its bespoke continuation machinery is deleted]* |
| `lifecycle_control_bindings` delivery + drainer machinery | events addressed to management's context, delivered by the durable dispatch handoff |
| `/tell`, `/correct`, `QueueLifecycleOwnerInstruction` | revisions (prompt bar / doc edits); a revision commit atomically mints the addressed wake event — transport, not a second channel *[panel: gemini dissent resolved this way]* |
| `roster.go` + overlay-id plumbing | `choir.Models()` + `choir.Call(model, …)` in a cell |
| `actuator=tools` fallback (R8) | nothing — the carrier is the only path |
| `super`/`cosuper` symbols, `super_controller.go`, `persistentSuperAgentID` | `management`/`engineering` as role strings; one actor type |
| Wire `reconciler` role | the RLM-based newspaper system (separate work) |
| assignment deadline sweep | durable timer events mint deadline wakes; executor timeouts emit `execution_failed`; semantic deadlines remain semantic *[panel F2]* |
| `model_policy.toml` role→model routing | gateway catalog + entitlement |

## What is kept (the genuinely hard parts)

- Digest-bound commands with replay/conflict semantics.
- Proposition digests binding a settlement claim to its pinned inputs.
- Capsule freeze/revoke with executor receipts — as *event steps* driven by
  the cursor, not a saga with its own continuation machinery.
- The vocab migration machinery — it is how the durable protocol names
  (`co_super_*` object kinds, `co-super-*` command IDs, digest domains) get
  renamed safely. That is a V3 vocabulary pass, not a find-and-replace.
- Admission control, restated as spawn-boundary policy.

## Failure semantics

External-world failures become events emitted by the harness that owns
the process table: `capsule_lost`, `execution_failed`,
`provider_unavailable`. Detection lives in the executor — a first-class
resident observer, not an emergent property. A wedged capsule is an
executor observation that must still reach the tape; executor liveness
timeouts are executor events, while semantic deadline obligations are
durable state driven by timer events. If a sub-RLM dies mid-flight, its
spawn event has no terminal partner; the parent context observes that
through the projection and decides — retry, compensate, or fail — in
code, not in a sweeps table.

## Panel review (2026-09-15)

A 12-agent convergent consensus panel reviewed this document
(`docs/reports/choir-event-driven-rlm-ontology-consensus-2026-09-15.md`).
Verdict: unanimous endorsement of the diagnosis and direction; unanimous
rejection of the document as build-ready. The five nouns survive —
events, activations, contexts, capabilities, projections — joined by two
the panel forced back in: the **durable dispatch handoff** (§2) and
**durable event producers** (§2, Failure semantics). The deletion table
above now names each row's replacement invariant; the original
"only"-claims it revises are marked inline.

The panel's phasing recommendation (ling): (1) substrate — consolidate
dispatch onto the tape, build the handoff and the executor/timer event
producers; (2) reduction — distribute saga logic into event handlers,
replace the slot table with the admission primitive, replace the
assignment object with spawn event + durable binding record; (3)
ontology — collapse role symbols, V3 vocabulary pass. Cutover also
requires a seeded reconciliation pass: existing pending rows with no
corresponding event must be minted their events or audited out, or the
new substrate inherits stranded state while claiming the old classes
deleted.

## What this resolves

- The six-strand wake family: every strand was "event committed, no cursor
  reached it." One cursor ends the class.
- The pending owner fork (repair substrate / amend roster / settle blocked):
  the substrate repair *is* this consolidation. The roster's 1-of-4 stays
  blocked until spawn-with-model-selection exists — which is the same work.
- The dual ontology: roles become strings in a column; the old names survive
  only in the V1/V2 decode seam and migration provenance.
- "Assignment is incidental complexity": confirmed and dissolved — it was
  spawn-and-settle built as a ledger because the recursive primitive didn't
  exist.

## Open questions for review

1. ~~Does event-tape-plus-cursor fully replace the wake-authority
   substrate?~~ **Answered by panel:** not as stated — the durable
   handoff and event producers are required additions (§2).
2. ~~Is "spawn carries context, task is just a field" the right
   primitive?~~ **Answered:** yes for cognition; accountable work
   additionally needs the typed obligation protocol (§5).
3. ~~Is parent-subset attenuation sufficient?~~ **Answered:** no —
   exercise vs delegation authority are distinct (§4).
4. The prompt bar as revision-shaped intake: resolved as revision commit
   atomically minting the addressed wake event. Remaining sub-question:
   does any owner input exist that is not revision-shaped (e.g. urgent
   interrupt) and does it deserve an event-only path?
5. What breaks when management stops being a resident drainer — the
   panel named the list: cold-start latency, activation storms,
   serialization of the management mailbox, loss of implicit FIFO,
   events arriving during passivation. Each is a dispatch-handoff
   property to specify, not a reason to keep residency.
6. Migration path: the panel endorsed a phased cutover with seeded
   reconciliation (Panel review). The V3 vocabulary pass stays an open
   cost question within phase 3.
