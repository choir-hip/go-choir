# Consensus Review: Event-Driven RLM Ontology

**Date:** 2026-09-15
**Subject:** `docs/designs/choir-event-driven-rlm-ontology-2026-09-15.md`
(commit `4ae146be`)
**Method:** agentic-consensus runner, convergent mode, 13-agent default
panel; 12 returned usable verdicts (omp-nemotron-3-ultra failed to start).
**Mutation class:** green (analysis; no runtime change)
**Artifacts:** `.agentic-consensus/rlm-ontology-review/run/*.out`

## Verdict

Unanimous in structure across all 12 respondents: **the diagnosis and the
convergence direction are correct; the document is not sufficient as
written and must not authorize its deletion list verbatim.** No panelist
defended the current architecture or proposed a different substrate. The
disagreement is over which "only" claims survive scrutiny.

Representative verdicts:

- codex: "Revise before adopting… directionally strong, but its strongest
  'only' claims are not yet theoretically sound."
- claude: "Take the direction, but the design isn't sufficient as written…
  'tape plus one durable dispatch cursor' is a restatement, not a
  substrate."
- gpt-5.6-sol: "Directionally sound, but not yet theoretically complete
  enough to authorize the literal deletion plan."
- gpt-5.6-luna: "Conditional accept as a substrate direction; reject as a
  complete cutover design."
- ling: "Correctly identifies the disease and the right general remedy,
  but its specific prescriptions under-specify what survives the
  deletion."
- gemini-3.8: "Conditional endorsement — revise architecture before
  implementation."
- devin: "Approve the direction; do not approve as written."
- opencode: "Conditionally approve… with three mandatory amendments."
- muse-spark: "Endorse direction, do not treat as complete substrate."
- cursor: "Adopt the diagnosis and the wake substrate… do not start from
  the deletion list."
- glm-5.3-flash: "Endorse the direction; do not execute as written."

## Convergent findings (independent, ≥4 panelists each)

### F1. One scalar cursor is not a delivery substrate

Raised independently by codex, claude, sol, luna, gemini, devin, opencode,
muse-spark, cursor, glm-5.3-flash, ling — effectively the whole panel.

The crash boundary the doc does not name:

- advance cursor before activation is durably accepted → lost wake;
- activate before advancing cursor → duplicate activation after crash;
- hold cursor until activation settles → head-of-line blocking across
  unrelated actors.

Required restatement: the cursor is the dispatcher's durable consumer
position. Delivery needs a transactional handoff — durable addressed
activation/inbox record → claimed under lease/epoch → completed or
retryable — with per-actor ordering, poison-event quarantine, and
backpressure. Per-actor watermarks (a mailbox projection) replace the
single scalar where fan-out or independent progress matters.

### F2. Silence is not an event — durable producers are missing

Raised by claude, sol, luna, gemini, devin, opencode, muse-spark, ling.

The cursor can only dispatch events that exist. Timers, executor
heartbeats/leases, process death, provider state changes, and wedged
capsules all require a durable producer that mints the event. Gemini
invokes FLP: "semantic state never times out" leaves permanently
unterminated spawns after silent host failure. The doc's claim must be
narrowed: executor liveness timeouts are executor observations, but
durable deadline obligations (owner-visible expiry, supersession) are
legitimate semantic state driven by timer events.

### F3. Desks and sub-RLMs differ by more than capabilities

Raised by codex, claude, sol, luna, glm-5.3-flash, ling.

Same activation kernel: yes. Only capabilities: no. A desk additionally
has durable identity, addressability, standing obligations, admission
policy, budgets, and reactivation semantics; a sub-RLM may be anonymous,
caller-scoped, disposable. Correct claim: one activation mechanism;
differences are declarative bindings (capabilities among them), not
separate role-specific loops. Codex's three-tier split: bare model call /
nested RLM activation / durable actor spawn — one harness, three
lifecycle contracts.

### F4. "Spawn minus terminal" is a liveness view, not a work schema

Raised by codex, claude, sol, luna, opencode, muse-spark, cursor.

Unmatched spawn/terminal pairs cannot distinguish blocked vs runnable,
failed vs abandoned, cancelled vs superseded, retry vs duplicate, partial
vs terminal, compensation-pending vs ordinary failure. Required: a typed
work-obligation event protocol (stable work_id, activation_id,
attempt_id, settlement rule, cancellation/supersession) with the current
state derived by projection. Delete duplicate ledgers; keep obligation
identity and settlement invariants. This matches doctrine C7/I7.

### F5. The fate saga is the most underestimated deletion

Named by claude, sol, luna, codex; ling analyzed it deepest.

Freeze→revoke→receipt→commit crosses external effects and remains a saga
when encoded as events — a good outcome (event-sourced saga) only if the
legal-transition reducer, pinned intent, idempotent re-entry, and
executor receipts are named first. Ling's sharper point: the assignment
binding is a *physical infrastructure specification* (capsule quotas,
executor access, capability handles), not a "context." Spawn context must
carry grant *references* and data; trusted activation setup resolves
current authority. Serializing capability handles into context survives
revocation and rewarm — explicitly forbidden by C6/I17.

### F6. Admission control needs an atomic primitive

Raised by sol, luna, opencode, muse-spark, cursor, claude
(occupancy-as-CAS).

"Per-desk policy at the spawn boundary" races without a transactional
check: model-level check-then-spawn is not admission. Required: a
conditional-append / compare-and-swap on the canonical appender, or a
reservation projection with atomic claim. Deleting `co_super_slots`
without this replacement deletes the enforcement, not just the table.

### F7. Strict parent-subset attenuation is too strong

Raised by codex, luna, sol.

Management must be able to *delegate* an effects capability it is
authorized to grant but not to exercise. Distinguish exercise authority
from delegation authority, or management either ambiently holds every
capability or cannot spawn engineering.

### F8. Cutover needs phasing and a migration audit

Raised by ling (explicit 3-phase), luna, sol, cursor, devin.

Ling's phasing: (1) substrate — consolidate cursors onto the tape, extend
it to carry actor dispatch, build the executor event-producer substrate;
(2) reduction — distribute saga logic into event handlers + executor
substrate, replace slot table with admission primitive, replace
assignment object with spawn event + durable binding record; (3)
ontology — collapse role symbols, V3 vocabulary pass. Plus: existing
pending rows with no corresponding event need a seeded reconciliation
event or one-time migration audit, or the new substrate inherits stranded
state while claiming the old classes deleted.

## Divergent findings (single-panelist, worth keeping)

- **gemini:** owner instructions should be events on the tape, with the
  document kept strictly as product artifact — forcing supervision
  through revisions pollutes artifacts with operational meta-instructions.
  Partially conflicts with the doc's "docs are the only control surface."
  Resolution path: revision commit atomically mints the addressed wake
  event; the event is transport, not a second channel.
- **claude:** the harness observer (process table owner) should be named
  a first-class resident, not left implicit.
- **devin:** the cursor guarantees event re-delivery, not computation
  resumption — activation context durability is a separate spec.
- **codex:** "the projection cannot disagree with the tape" is true of
  the mathematical definition, not deployed projection implementations —
  projections can lag or carry reducer bugs.

## Required amendments to the design document

1. §2: restate cursor as dispatcher consumer position + durable handoff
   protocol (addressed activation record, lease/epoch claim, ack,
   per-actor ordering, poison-event path).
2. New substrate clause: durable event producers — executor-owned
   timer/lease/heartbeat source, process table, external observers.
   Narrow "no semantic deadlines" to "no executor-defined semantics";
   durable deadline obligations remain, driven by timer events.
3. §3/§4: desks and sub-RLMs share one activation kernel; differences are
   durable bindings (identity, addressability, lifecycle, admission,
   budgets, settlement contract), capabilities among them.
4. §4: exercise vs delegation authority; spawn context carries grant
   references, never capability handles or physical bindings.
5. §5: typed work-obligation event protocol + canonical projection;
   "spawn minus terminal" demoted to base liveness view.
6. Deletion table: each row gains its named replacement invariant;
   `CoSuperAssignment`+saga row explicitly becomes "event-sourced saga:
   typed legal-transition reducer + executor receipts."
7. Owner input: revision commit atomically mints the addressed wake
   event; no second channel, but the wake is an event.
8. Migration: seeded reconciliation for existing pending rows; phased
   cutover (substrate → reduction → ontology).

## Disposition

The design doc has been amended in place (same file, new "Panel review"
section recording this verdict and revising the overclaimed sections).
The ontology survives review with its five nouns intact but demoted from
"sufficient" to "direction": events, activations, contexts, capabilities,
projections — plus the two the panel forced back in: **durable dispatch
handoff** and **durable event producers**.

Next step if pursued: a Definition scoped to ling's Phase 1 (substrate)
only — cursor consolidation + dispatch handoff + executor event
producers — with the deletion list explicitly out of scope until the
named replacement invariants have live proof.
