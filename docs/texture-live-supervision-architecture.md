# Texture Live Supervision & Revision Surface Architecture
**Date:** 2026-08-20 (revised 2026-08-21)  
**Classification:** Green Architecture Doctrine  
**Authority:** `docs/choir-doctrine.md`, `docs/agent-product-doctrine.md`

---

## 1. The Core Principle: Document-Driven Long-Horizon AI

Texture is not a chat interface, conversation history, or message broker. It is a live, collaborative, updating supervision and revision surface. Document-based, soft-realtime supervision — not chat — is load-bearing as agent count and task horizon grow.

```
        Owner-facing Texture surface (owner edit or prompt)
                         │
                         ▼
                     Conductor
               (classify / open / route;
                cannot author agent revisions)
                         │
                         ▼
                      Texture
        (Living Document & Supervision Surface)
      versions are monotonic self-contained snapshots
                         │
                  execution_request
                   Control Packet
                         │
                         ▼
                       Super
     (one per computer; coherence and error correction,
            not a concurrency limiter)
                         │
                  assign_co_super
                         │
                         ▼
                      CoSuper
   (task-level actuator; assignment may hold N capability-
    bound capsules — candidate A uses 1:1, transitional)
```

In Choir's product architecture:
1. **The unit of long-horizon work is a living document**, not an ephemeral conversational turn.
2. **Canonical document versions have two writer classes.** `AuthorUser` is the owner and may edit the current head through an immediate canonical CAS. `AuthorAppAgent` is the Texture agent and the sole *agent* writer. No other agent writes the document directly.
3. **A Texture-agent authoring turn that changes semantic state commits exactly one new monotonic version.** A wait, block, control, rejected, pending, or otherwise no-change turn commits no revision. Every committed revision is a self-contained snapshot; prior versions are history for inspection and debugging, never required context for acting on the current head.
4. **Citations open into transcluded content** (live source diffs, test receipts, candidate manifests, consensus verdicts) directly within the document viewer on `choir.news`.
5. **Supervision is observable in soft-realtime**: opening an active Texture document on `choir.news` shows current document state — open work items, verified evidence, latest semantic snapshot — without reading raw log streams.

Humans interface only with Texture, and the document is optimized for human-language readability. Owner edits may occur at any time; Texture propagates downstream whatever is clear from the diff, verbatim only when precision requires it.

---

## 2. The Living Document Protocol

The protocol is about snapshots of semantic state, not lifecycle stages:

- Versions are **monotonic and self-contained**. Acting on the current head never requires reading prior versions.
- A Texture-agent turn whose outcome changes semantic state produces **exactly one new version**. No-change turns produce none.
- `texture_turn_committed` is a lifecycle event emitted for every turn outcome — including wait, block, and no-change outcomes. It is **not itself a revision** and must never be counted as one.
- Owner `AuthorUser` edits are immediate canonical-head CAS transitions. Downstream inputs (Super reports, CoSuper evidence, worker updates) become canonical only when Texture incorporates them through its own authoring turn.
- Dozens to hundreds of versions per tens-of-minutes-to-hours session are expected and healthy.
- There is no prescribed stage sequence. The same lifecycle can create many revisions, few, or none at a given stage.

### Non-normative example: one candidate lifecycle

Illustrative stages only — not required semantics, ordering, or version numbering:

| Illustrative stage | Content & Supervisory Evidence | Transcluded Citations |
| :--- | :--- | :--- |
| User intake | Initial user prompt and objective as received by Conductor. | `[prompt:seed]` |
| Scope & trajectory binding | Formulated trajectory plan, target deliverables, open work items. | `[trajectory:id]`, `[work:items]` |
| Delegation & capsule provisioning | Super acknowledges execution authority and opens CoSuper assignment. | `[capsule:id]`, `[binding:handle]` |
| Authoring & execution evidence | CoSuper reports work progress, modified file paths, test receipts. | `[source:diff]`, `[test:receipts]` |
| Bundle freeze | Immutable candidate bundle frozen with 5 required artifacts. | `[bundle:manifest]`, `[patch:content]` |
| Consensus + promotion/acceptance | Qualified consensus under frozen policy; event appender commits promotion. One illustrative acceptance transition, not two prescribed revision stages. | `[consensus:votes]`, `[event:head]`, `[epoch:receipt]` |
| Verification / falsification | Post-promotion API/DB verification and test results. | `[verifier:logs]`, `[proof:receipt]` |
| Restore / supersession | Candidate B supersession or acceptance-fenced restore back to baseline. | `[checkpoint:witness]`, `[restore:event]` |

---

## 3. Diagnosing Apparent Low Revision Counts

Do not infer a revision defect from `texture_turn_committed` counts: that lifecycle event records every turn outcome, while only `TextureTurnRevision` outcomes advance the canonical head. Before any cadence repair, record a turn-outcome census and a revision-outcome census separately.

The actual self-development join defect (identified 2026-08-21): the join path projects a synthetic deterministic Texture run and commits `TextureTurnWait` directly, bypassing a genuine Texture-agent authoring turn. The remedy is to route the join through genuine Texture authoring when semantic state must change — **not** to require every execution milestone or worker state transition to call `ApplyTextureTurn` or to produce a revision. This supersedes the earlier "mailbox chat regression" and "missing revision on every CoSuper state change" diagnoses recorded here previously.

---

## 4. Actor, Scheduling, and Resource Boundaries

- **Texture** has exactly two jobs: revise the human-readable document and message other agents. No capsule, host filesystem, provider-routing, event-chain, or promotion authority.
- **Style-guide Textures** are planned, not implemented. Any document may later be designated to shape Texture's writing style; this affects prose/register, not authority or routing. The existing wire-publish style catalog is only a precursor, not the generalized feature.
- **Super** remains one per computer to see every document's state, arbitrate resources, and maintain coherence/error correction — explicitly not to limit concurrency.
- **CoSupers** are task-level actuators. N capsules per assignment is the doctrinal general shape; 1:1 holds only through candidate A.
- **Mission A scheduling:** one live CoSuper assignment per computer. Requests carry a durable computer-scoped arrival ordinal; selection is FIFO among non-expired requests; unselected requests remain pending untouched. A request expires when its operation reaches terminal state, a superseding owner correction arrives, or its deadline passes. Assignment deadlines fail rather than hang. Admission refusal is retryable — work stays pending and the computer never deadlocks.
- **Mission-A memory containment:** `memory.high` = requested, `memory.max` = 2× requested, `memory.events` OOM counters as feedback. No PSI pause/resume tier and no zram dependency in Mission A.
- **Mission B (parallel release gate):** parallel Textures/trajectories with N concurrent assignments under an admission-ledger overcommit factor is a release-gate requirement, proven after sequential correctness.

---

## 5. The Self-Development Flow (Choir in Choir)

The long-term goal of Choir is the **automatic computer**: moving from changing Choir code in an external coding harness to using the `choir` CLI to have Choir VMs modify their own code autonomously.

```text
User / Developer                  Choir VM (Staging)
      │                                  │
      ├── choir run start ──────────────►│ Conductor Intake
      │   "implement feature X"          │       │
      │                                  │       ▼
      │                                  │ Texture Document Created
      │                                  │       │
      │                                  │       ▼
      │                                  │ Super Orchestrates
      │                                  │       │
      │                                  │       ▼
      │                                  │ CoSuper Capsule Authors Code
      │                                  │       │
      │                                  │       ▼
      │                                  │ CoSuper Freezes Bundle
      │                                  │       │
      │                                  │       ▼
      │                                  │ Consensus Evaluates & Accepts
      │                                  │       │
      │                                  │       ▼
      │◄── Watch live on choir.news ─────┤ VM Promotes & Restarts Runtime
```

These lifecycle events do **not** determine revision numbers or cadence; the flow above is an example, not protocol. The user monitors live progress by opening the active Texture document in the browser while the VM safely executes, tests, and promotes changes to its own environment.
