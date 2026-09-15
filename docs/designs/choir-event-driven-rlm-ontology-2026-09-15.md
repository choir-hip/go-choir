# Design: Event-Driven RLM Ontology

**Date:** 2026-09-15
**Status:** design document — not a Definition, not executable authority
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

### 2. A durable dispatch cursor is the only wake authority

Per computer, one cursor: "events through N have been dispatched." Dispatching
an event means activating the actor the event addresses. Crash → resume from
the cursor. Event identities make re-dispatch idempotent. There is no
scanner, no watchdog, no sweep, no drainer: **pending work is not discovered,
it is delivered** — the event is the wake, and the cursor makes the wake
durable.

This replaces the memo's "transition-minted recovery occurrences" with the
smaller truth: the tape already contains the occurrences; what was missing is
the cursor.

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

### 4. Capabilities are the only difference between a desk and a sub-RLM

Engineering's capsule verbs, research's search, texture's canonical write —
all capability sets on the same actor type, attenuated at spawn. A sub-RLM
receives a subset of its parent's authority, never more. "Assignment" as a
separate object disappears: spawning an RLM with a capsule capability bound
to a context *is* the assignment. The binding is the spawn.

Admission control becomes per-desk policy at the spawn boundary — engineering
may admit one effects-capable sub-RLM at a time while research fans out a
thousand — not a global invariant and not a slot ledger.

### 5. Everything I called a ledger becomes a projection

"Open work" = spawn events without matching terminal events. "What is
engineering doing" = filter by role and scope. Settlement state = the event
sequence `spawned → … → settled|failed|cancelled`, derived, never stored
twice. Work items, assignments, slots, obligations — all projections over
spawn/settle event pairs.

This kills the run-terminal-vs-fate-terminal divergence by construction:
there is one stream, and a run finishing is just an event in it. A worker
reading state reads the projection; the projection cannot disagree with the
tape because it *is* the tape.

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

| Current machinery | Replaced by |
|---|---|
| `co_super_slots` table + budget/sequencing enforcers | spawn-boundary admission policy |
| `CoSuperAssignment` object + fate saga + watchdogs + resume sweeps | spawn event + capability binding + settlement events |
| `lifecycle_control_bindings` delivery + drainer machinery | events addressed to management's context |
| `/tell`, `/correct`, `QueueLifecycleOwnerInstruction` | revisions (prompt bar / doc edits) |
| `roster.go` + overlay-id plumbing | `choir.Models()` + `choir.Call(model, …)` in a cell |
| `actuator=tools` fallback (R8) | nothing — the carrier is the only path |
| `super`/`cosuper` symbols, `super_controller.go`, `persistentSuperAgentID` | `management`/`engineering` as role strings; one actor type |
| Wire `reconciler` role | the RLM-based newspaper system (separate work) |
| assignment deadline sweep | executor timeouts emit `execution_failed` events; semantic state never times out |
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

External-world failures become events emitted by the harness that owns the
process table: `capsule_lost`, `execution_failed`, `provider_unavailable`.
Detection lives in the executor; semantics stay pure. A wedged capsule is an
executor observation, not a semantic deadline — there are no semantic
deadlines. If a sub-RLM dies mid-flight, its spawn event has no terminal
partner; the parent context observes that through the projection and decides
— retry, compensate, or fail — in code, not in a sweeps table.

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

1. Does event-tape-plus-cursor fully replace the wake-authority substrate, or
   is there a class of continuation it cannot express?
2. Is "spawn carries context, task is just a field" the right primitive, or
   does something real get lost when work-tracking is pure projection?
3. Capability attenuation at spawn: is parent-subset sufficient, or are there
   legitimate grants a sub-RLM needs that the parent never held?
4. The prompt bar as revision-shaped intake: does V0-as-prompt satisfy
   "docs are the control" or is a thin intake event still needed?
5. What breaks when management stops being a resident drainer and becomes an
   event-activated actor — latency, ordering, anything else?
6. Migration path: is a V3 vocabulary pass over durable protocol names the
   right cost, or should old names be accepted as permanent protocol literals?
