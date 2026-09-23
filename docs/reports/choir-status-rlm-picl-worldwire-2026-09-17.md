# Choir Status Report — RLM, PICL, World Wire



> **Stale as of 2026-09-22.** Dated evidence — not current authority. The carrier (`choir-rlm-engineering-carrier-2026-09-11`) is the sole working entrypoint, blocked on owner decision; the revised roadmap is `docs/world-wire-mission-stack-2026-09-22.md`; precommitment records (renamed from PICL) is the mechanism; throughline is the `/goal` format.


**Date:** 2026-09-17
**Audience:** owner + incoming collaborators
**Scope:** where the program actually is, what must ship for beta, and the
honest residue/security ledger. Sources cited throughout; nothing here is
aspiration dressed as fact — "designed" and "built" are kept separate.

---

## 1. Executive summary

Choir is the automatic computer: a persistent, per-user computer that works
continuously and develops itself under owner supervision
(`docs/choir-vision.md`). The vision orders two mountains: supervised
self-development first, the World Wire downstream. We are on the first
mountain, most of the way up the substrate, partway up the runtime.

**The one-line status:** the durable substrate is proven on staging; the RLM
runtime is mid-cutover with its mechanism proven and its settlement gate open
on one owner decision; PICL is design-complete at schema level as of today;
the World Wire is vision plus a codesign memo, correctly waiting its turn.

**The one blocker that matters:** the Engineering Carrier mission's P5 gate
is open on a six-strand defect family in the activation-wake substrate —
lifecycle transitions that lose their continuation. The escalation memo
(`docs/memo-activation-wake-authority-substrate-2026-09-13.md`) requests
exactly one owner decision: charter the wake-authority substrate repair,
amend the roster scope, or settle the mission blocked. Everything downstream
of the roster waits on it.

## 2. Where we are

### 2.1 Proven substrate (settled, deployed evidence)

| Mission | Completed | What it proved |
|---|---|---|
| Coherent computer convergence | 2026-07-21 | Durable-work kernel: canonical event tape, reducers, restart-surviving trajectories |
| Tape recovery | 2026-08-15 | Whole-computer restore substrate; six receipts incl. `serving_join`, `capability_renewal_pass` (staging `4ac90583`) |
| Durable substrate overhauls | 2026-08-26 | Track K key escrow (2-of-N quorum + WebAuthn PRF); Track F encrypted 4MiB file-CAS + Merkle manifests; Track M host MTA spool + guest Maildir; recovery capsules + restore drills |
| Substrate cleanup & cutover | 2026-08-26 | Guest MicroVM runs one immutable Nix binary; Dolt GC restored; boot-proven on month-old disk |
| Private Go actor kernel | 2026-08-27 | Yaegi-interpreted Go activations in disposable guest capsules; unified Bash/Go broker; durable continuity across forced activation death |
| Substrate & scheduling readiness | 2026-09-03 | Live-trigger-only Super wakes; FIFO under arrival ordinals; tombstone settlement; exact-run resume isolation |

All deployed on staging `https://choir.news`, retained computer
`computer-03335285269bdba4f94377e56879f9e6`, **effects OFF**, pre-A
checkpoint `99949fe2`. Source: `docs/ACTIVE.md`.

### 2.2 RLM mission sequence (three complete, one in flight)

| Mission | Status | Outcome |
|---|---|---|
| RLM restore-zero | complete 2026-09-09 | Retained-store boot at `local=W=H=148431`, zero prefix fetches |
| RLM settlement gate | complete 2026-09-09 | Yaegi compile gate; v1 terminal identity + proposition digest; supersede tuple; resumable fate saga; deployed `6b758878` |
| RLM versioned rename | complete 2026-09-11 | V1→V2 canonical desks (`management`, `engineering`, `research`); frozen mapping; carrier coverage over OG objects/edges; deployed `e3396329` |
| **RLM Engineering Carrier** | **working — P5 gate open** | see below |

The carrier (`docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md`,
chartered 2026-09-11) moves the engineering desk fully onto the in-cell
carrier: `capsule_go_eval` as the only JSON envelope, typed `choir.*`
in-cell functions staging intents for one reducer, the reducer authoring
assignment fate, run acceptance keyed on canonical evidence not tool names,
OpenCode Go/Zen wired as phase 1, one model-independent prompt.

**Done inside the carrier (P0–P4):** simplified envelope, reducer-owned
settlement, in-cell freeze/verify/inspect surface, closed assigned registry,
five overlay JSON tool deletions each earned by replay proof, replay harness
with golden receipts — deployed replay proof `d6b4fab1` (CI 34695293124,
all five operations in 1.64s, canonical equality, zero effects).

**Open at P5:** the roster proving ground. Mechanism proven — arm A9 was the
first ever served end-to-end by its funded overlay (opencode-go /
deepseek-v4.1-flash); A14 executed the task's three cells and staged
`choir.Complete`. But the terminal roster receipt is **quarantined**: the
reducer's terminal saga stranded at `revoke_requested`, run state reports
`completed` while reducer fate stays `bound` — run-terminal ≠
fate-terminal. Roster tally: 1 of 4 expected model ids.

**The defect family:** six strands in ~24h sharing one cause — a lifecycle
transition commits but no durable wake/retry authority is minted for the
continuation, so work strands until an unrelated sweep notices. Three
strands repaired (`a2676517`, `67823ae2`, `caa171b6`); the terminal-saga
revocation strand is open. Escalated per convergence-before-patching
doctrine; owner decision pending (three options in the memo).

**P6 (landing) not started.**

### 2.3 The minimal ontology (design-complete, not built)

`docs/designs/choir-event-driven-rlm-ontology-minimal-2026-09-15.md` —
survived two adversarial consensus rounds. The design collapses today's
sprawl — three ledgers, two delivery systems, five wake mechanisms, a
bespoke saga, host-side model policy, the `/tell` side channel — into:

- **Move 1:** delivered = in the actor's state head, committed atomically.
  Pending deliveries are a projection, not a ledger. Fenced atomic commit:
  {emitted events + new head} in one conditional append.
- **Move 2:** one live activation per actor, fenced. Every actor a serial
  processor — the actor model, 1986.
- Eight panel findings resolved minimally: `not_before` timer field fired
  by the dispatcher's due-index; `work_id`/`attempt_id`/`status` with
  latest-attempt-settles; executor-as-fate-continuer; admission as a
  capability on one serial actor; `may_delegate` bit; migration as a script
  inside a write fence.
- Calling convention is **cast-only** (owner correction): `rlm.spawn` =
  admission event returning a handle; results arrive as `agent_message`
  events; no synchronous sub-RLM calls across actors.

Residual hard points named honestly in the doc: duplicate effects
(idempotency keys at the gateway), hot-actor starvation, first-timer
seeding, `delivery_failed` ownership, activation-context durability,
dispatcher topology.

### 2.4 PICL — learning records (design-complete today, nothing built)

Owner direction 2026-09-17: the tape's second job is the learning record.
The PICL paper review + nine-agent consensus
(`docs/reviews/picl-paper-review-2026-09-17.md`,
`docs/reviews/picl-consensus-synthesis-2026-09-17.md`) returned a unanimous
verdict: **adopt at schema level now, defer consumption and the
experimental program.** Eight decisions:

1. `expected` field — optional, on `rlm.spawn` and red/black actions.
2. `resolves`/`prediction_ref` linkage on result events.
3. Epistemic boundary invariant — prediction valid only if committed before
   its resolution reaches the predicting actor's stream.
4. Pairing projection — deterministic fold materializes unresolved
   (prediction, resolution) candidate pairs onto the object graph.
5. Curator desk — async, off the dispatch path, mints
   `learning_record_minted` events.
6. Result-wake policy — context construction reattaches or withholds
   `expected` deliberately, not accidentally.
7. Retrieval injection off by default — records queryable via REPL, never
   auto-injected.
8. Actor/model scoping on every record; cross-actor retrieval is an
   experiment, not a default.

Cost ≈ zero; worst case is a provenance-linked expectation ledger useful
for audit. The panel's key correction: the tape supplies ordering and
provenance, not epistemic privilege — a preserved prefix can be replayed
outcome-blind, so the decisive experiment (sealed pre-state replay) must
run on real beta data before any learning claim.

### 2.5 World Wire — vision and codesign, correctly downstream

The automatic newspaper is the payoff the automatic computer enables — not
a separate product, not a separate ontology
(`docs/choir-vision.md`: "there is no wire before the computer"). The
codesign memo (`docs/memo-autopaper-world-wire-generalization-codesign-2026-08-09.md`)
frames it as the generalization test: if the wire needs a second scheduler,
news-specific loop, or bespoke state ontology, the harness hasn't
generalized.

What exists today: `cmd/sourcecycled` (source polling daemon — RSS/Telegram/
GDELT via `internal/cycle`, dedup, ingestion handoff), `cmd/corpusd`
(platform publication/object-graph service), `internal/agentcore/
wire_publication.go` + `wire_platform_publish.go` (eligibility, publish
work items, Texture revision sync), `internal/platform/universal_wire.go`
+ `internal/proxy/universal_wire.go` (authenticated
`/api/universal-wire/stories`), and `frontend/src/lib/
UniversalWireApp.svelte` (transitional edition-card UI with honest empty
state). What does not: durable per-source scheduling, edition-Texture
Wire app, subscription/newsletter proof, any Wire PICL implementation.
Autopaper is tabled with no active Definition. Correct state: waiting for
the self-development proof.
The business framing now lives in `docs/memo-diagonal-media-strategy-2026-09-17.md`: autoradio is the wire's second surface, the box score its judgment layer, and the platform endgame enables organizations to build automatic newspapers on private and public data.

## 3. What ships for beta — dependency-ordered

**B0 — Unblock the carrier (owner decision → substrate repair → roster →
landing).** The wake-authority repair: transition-minted recovery
occurrences, one consumer, run-terminal ≠ fate-terminal, idempotent
revocation resume. Then the four-model roster tally and P6 landing. Closes
R7. *This is the critical path; everything else queues behind it.*

**B1 — Ontology cutover.** Implement the minimal design: collapse the three
ledgers / two delivery systems / five wake mechanisms into tape + state
heads + dispatcher; fenced atomic commit; serial-per-actor; `not_before`;
work schema; capability admission; migration script inside a write fence.
Management and research desks cross → R8 (`actuator=tools` deletion) and
R10 (Super substrate retirement, ~2,809-line `super_controller.go` + 1,331
references) close.

**B2 — The self-development proof.** Unpause
`choir-supervised-self-development-on-rlm-2026-09-02`: candidate A
solitaire authored via RLM cells, qualified consensus under
`reversible-selfdev-v1`, promotion, live play verification, falsification
with B, restore to `99949fe2`. **This is the vision's proof target** — the
computer makes one real change to itself, legibly, durably.

**B3 — PICL instrumentation.** The eight schema decisions above. Additive,
ignorable, never load-bearing. The beta generates the naturalistic records
the experimental program needs.

**B4 — Beta hardening.** R2 watermark cadence; R1 scoped fault-injection
control; actor-log SQLite pool refactor (`docs/refactors/`); problem-doc
burn-down (28 open, ~22 non-closed); heresy-detector CI wiring.

**Then World Wire** — post-proof: source observation modules, publication
artifact program, editorial supervision policy, public projection
transaction — all on the proven substrate, per the codesign formula.

## 4. Invariants and security posture

**Runtime invariants** (`docs/runtime-invariants.md`): 13 sections —
development tooling boundary, deployment source of truth, computer
lifecycle/reclaim, agent roles, scheduling & memory containment, computer
model, Super-tier execution policy, state placement, acceptance &
promotion, messaging, Trace, run acceptance.

**Authority model** (`docs/supervision-protocol.md`,
`docs/agent-product-doctrine.md`): owner is constitutional root, not
per-effect approver. Effect-specific consensus policies; irreversible
effects need stronger evidence + durable receipts. Capabilities
mechanically restricted; CoSuper capability-bounded. Texture sole agent doc
writer; owner CAS second writer. **Effects OFF** pending deployed proof.

**Mutation classes** (AGENTS.md): green→black ceremony; red surfaces =
Texture canonical writes, Trace/evidence, checkpoint/route projection,
auth/session renewal, vmctl, gateway/provider calls, run acceptance,
deployment routing. The carrier is a red-class mission.

**Heresy detectors** (`docs/heresy-detectors.md`): manifest H001–H037 + I4.
Current: zero active hits across parent/child API, continuation, tool
forcing, role-keyword routing, lease, polling, model-ID, and route-identity
detectors; structural detectors (H013–H021, H033–H037) at review level.

**Key management:** Track K escrow live — 2-of-N quorum gate, WebAuthn PRF
wrapping, deployed and proven on staging.

**Standing questions** (`docs/standing-questions.md`): the nine pre-flight
checks every mission answers — decision provenance, settled-decision
conformance, deletion citers, single state authority, artifact-verified
success, fate-sharing, restart durability, no-SSH operability, registry
hygiene.

## 5. Residues and refactors

**Open residues** (`docs/mission-residues.md`):

| ID | What remains | Closes when |
|---|---|---|
| R1 | Live fault-injection drill — no scoped product control exists | a mission charters the control |
| R2 | Watermark within 10k of head | before any near-bound restore |
| R3 | Proxy boot-screen during resolve lock (cosmetic) | next proxy/UX pass |
| R6 | Cutover final disposition | a successor is named |
| R7 | 5 overlay JSON tools + 4 legacy capsule ops retired | carrier completes (in flight) |
| R8 | `actuator=tools` branch deleted | all desks cross to RLM |
| R9 | deepseek/xiaomi provider heresy deleted | carrier deletion lands |
| R10 | Super substrate fully retired | mission 6 / post-11 residue pass |

Closed: R4, R5 (settlement-gate repairs, 2026-09-09).

**Refactors:** `docs/refactors/actor-log-sqlite-pool-cap.md` — documented,
not implemented (central `OpenLogDB` + `MaxOpenConns(1)`).

**Dissolution inventory** (`docs/runtime-dissolution-inventory.yaml`): 2,331
listed records — 127 Go files (70 prod / 57 test), 812 exports, 2 routes,
50 tools, 1,330 citers — the collapse surface the ontology cutover
consumes.

**Problem ledger:** 28 docs in `docs/problems/`; ~22 non-closed. The
activation-wake family is the only cluster blocking the critical path.

## 6. Honest risks

1. **Single owner-decision dependency.** The carrier cannot move until the
   wake-authority question is answered. Mitigation: the memo scopes exactly
   three options; any of them unblocks.
2. **The wake family may have more strands.** Six in 24h suggests the class
   isn't exhausted; the substrate repair (mint-at-transition) is designed
   to kill the class, not the instances.
3. **PICL is a hypothesis.** H1–H6 falsifiable; records are additive and
   never load-bearing. The sealed-replay control decides whether temporal
   commitment or mere record structure carries the gain.
4. **Designed ≠ built.** The minimal ontology, PICL schema, and World Wire
   are design-complete documents. The only runtime truth is deployed
   staging evidence; this report keeps the categories separate.
5. **Effects stay OFF** until the self-development proof lands — by design,
   not by delay.

## 7. Sources

`docs/ACTIVE.md` · `docs/choir-vision.md` ·
`docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md` ·
`docs/memo-activation-wake-authority-substrate-2026-09-13.md` ·
`docs/designs/choir-event-driven-rlm-ontology-minimal-2026-09-15.md` ·
`docs/reviews/picl-consensus-synthesis-2026-09-17.md` ·
`docs/mission-residues.md` · `docs/runtime-invariants.md` ·
`docs/heresy-detectors.md` · `docs/standing-questions.md` ·
`docs/supervision-protocol.md` ·
`docs/memo-autopaper-world-wire-generalization-codesign-2026-08-09.md` ·
`docs/memo-diagonal-media-strategy-2026-09-17.md` ·
`docs/reports/choir-rlm-engineering-carrier-narrative-report-2026-09-13.md`
