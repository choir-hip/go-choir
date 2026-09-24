# Texture Live Supervision & Revision Surface Architecture
**Date:** 2026-09-23 (rewritten for the desk-RLM rectification)
**Classification:** Green Architecture Doctrine
**Authority:** `docs/choir-doctrine.md`, `docs/agent-product-doctrine.md`,
`docs/desk-rlm-rectification-plan-2026-09-23.md`

---

## 1. The Core Principle: Document-Driven Long-Horizon AI

Texture is not a chat interface, conversation history, or message broker. It is
a live, collaborative, updating supervision and revision surface. Document-based,
soft-realtime supervision — not chat — is load-bearing as agent count and task
horizon grow.

The product point (owner direction 2026-09-23): the owner opens choir.news and
sees a live-updating Texture document showing ongoing work state. Idea-level
supervision, not action-level.

```
        Owner-facing Texture surface (owner edit or prompt)
                         │
                         ▼  owner revision = the input event (no tell channel)
                     Texture desk
        (root RLM; sole agent writer of the document)
      versions are monotonic self-contained snapshots
                         │
                  choir.Cast / choir.Report
                  (semantic acts, in-cell yaegi)
                         │
                         ▼
                   Management desk
     (one per computer; coherence, error correction,
      resource arbitration — not a concurrency limiter)
                         │
                  delegated choir.Cast
                  (new admission authority)
                         │
                         ▼
                   Engineering desk
   (root RLM; mutation via capsule-bound sub-RLM cells;
    effect verbs Complete / Freeze / Verify)
                         │
                         ▼
              commitment ledger on the tape
   (OG objects; reports resolve commitments; scores
    accrue per desk and stay off the acting context)
```

In Choir's product architecture:

1. **The unit of long-horizon work is a living document**, not an ephemeral
   conversational turn.
2. **Canonical document versions have two writer classes.** `AuthorUser` is the
   owner and may edit the current head through an immediate canonical CAS.
   `AuthorAppAgent` is the Texture desk and the sole *agent* writer. No other
   agent writes the document directly.
3. **A Texture-agent authoring turn that changes semantic state commits exactly
   one new monotonic version.** A wait, block, control, rejected, pending, or
   otherwise no-change turn commits no revision. Every committed revision is a
   self-contained snapshot; prior versions are history for inspection and
   debugging, never required context for acting on the current head.
4. **Citations open into transcluded content** (live source diffs, test
   receipts, candidate manifests, consensus verdicts) directly within the
   document viewer on `choir.news`.
5. **Supervision is observable in soft-realtime**: opening an active Texture
   document on `choir.news` shows current document state — open commitments,
   resolved claims, falsified predictions, latest semantic snapshot — without
   reading raw log streams.

Humans interface only with Texture, and the document is optimized for
human-language readability. Owner edits may occur at any time; Texture
propagates downstream whatever is clear from the diff, verbatim only when
precision requires it.

---

## 2. The Desk-RLM Actor Model

Four desks are **persistent root RLMs**, not per-task spawns. Each runs yaegi
Go cells in a killable subprocess and exposes a desk-specific `choir` module
surface. Agent-to-agent communication happens through those in-cell functions —
semantic acts — not a separate tool-call channel.

| Desk | Cardinality | Authority | Module surface (target) |
| :--- | :--- | :--- | :--- |
| **texture** | per bound document | sole agent writer of canonical revisions; editorial discretion over what the ledger renders | doc revision, inbox, report, precommit, note |
| **management** | exactly one per computer | whole-computer coherence, error correction, resource arbitration; delegated `Cast` admission into engineering; `Escalate` to owner | inbox, cast, report, precommit, escalate, spawn research |
| **engineering** | per assignment | mutation only through capsule-bound sub-RLM cells; effect verbs `Complete`/`Freeze`/`Verify` | file/exec, freeze/complete/verify, inbox, report, precommit, note |
| **research** | per assignment | read-only world + message authority; evidence assertion via `Report` | read/search, inbox, report, ask, precommit, note |

Sub-RLMs are per-assignment runs a desk casts; the desk's own cell loop is the
actor. The assignment/fate saga (freeze/revoke/destroy/record) remains host
machinery — desk actors cannot replace cgroup operations; what changes is who
opens assignments and who notices wedges.

### Semantic acts

One transport/provenance envelope, two record semantics:

- **Operational acts** (fate-bearing, tracked, not scored): `Cast`
  (delegation — opens an assignment plus the caster's implicit commitment),
  `Complete`/`Freeze`/`Verify` (engineering effect verbs), `Ask` (query —
  resolves on answer, liveness not truth), `Note` (raw transport — unscored
  escape hatch, deliberately weak), `Cancel`/`Withdraw`, `Escalate`,
  `Reply`/`Answer`.
- **Epistemic acts** (claims that resolve and score): `Precommit` (frozen
  prediction *before* resolving evidence) and `Report` (evidence assertion —
  resolves on recipient acceptance or independent verification; the act names
  its resolver, and the resolver is never the claimant for engineering claims).
  `Resolve` is the resolver's act.

`choir.Outcome` folds into `Report`; `choir.Message` demotes to `choir.Note`;
`choir.Assign` (the synchronous broker path) folds into `Cast`. The retired
`update_coagent` tool and `assign_co_super` opener are deleted for the four
desks; `Report` generalizes the evidence write.

### The commitment ledger

All acts write through one commitment ledger on the tape — OG objects with
provenance edges, never a third store. A commitment is a typed claim with a
resolver and a deadline; it resolves to an outcome and accrues to the desk's
score. Scores stay off the acting agent's context (the epistemic boundary —
feeding scores back recreates reward hacking).

**Foliation:** the ledger is the foliation for provenance; the document renders
a *materiality projection* — top-level claims, falsified claims, overdue
commitments — not the org chart. Texture keeps editorial discretion: a deep
delegation can hold one owner-relevant decision, a shallow one many.

---

## 3. The Living Document Protocol

The protocol is about snapshots of semantic state, not lifecycle stages:

- Versions are **monotonic and self-contained**. Acting on the current head
  never requires reading prior versions.
- A Texture-agent turn whose outcome changes semantic state produces **exactly
  one new version**. No-change turns produce none.
- `texture_turn_committed` is a lifecycle event emitted for every turn outcome —
  including wait, block, and no-change outcomes. It is **not itself a revision**
  and must never be counted as one.
- Owner `AuthorUser` edits are immediate canonical-head CAS transitions.
  Downstream inputs (desk reports, resolved commitments, evidence assertions)
  become canonical only when Texture incorporates them through its own
  authoring turn.
- Dozens to hundreds of versions per tens-of-minutes-to-hours session are
  expected and healthy.
- There is no prescribed stage sequence. The same lifecycle can create many
  revisions, few, or none at a given stage.

### Non-normative example: one candidate lifecycle

Illustrative stages only — not required semantics, ordering, or version
numbering:

| Illustrative stage | Content & Supervisory Evidence | Transcluded Citations |
| :--- | :--- | :--- |
| User intake | Initial owner prompt/objective as received. | `[prompt:seed]` |
| Scope & trajectory binding | Formulated trajectory plan, target deliverables, open commitments. | `[trajectory:id]`, `[commitment:ids]` |
| Delegation | Management acknowledges and casts engineering; assignment opens. | `[cast:id]`, `[assignment:id]` |
| Authoring & execution evidence | Engineering sub-RLM reports progress, modified paths, test receipts. | `[source:diff]`, `[test:receipts]` |
| Bundle freeze | Immutable candidate bundle frozen with required artifacts. | `[bundle:manifest]`, `[patch:content]` |
| Resolution & acceptance | Commitments resolve; verifier and owner-scoped acceptance land. | `[resolve:receipt]`, `[event:head]` |
| Verification / falsification | Post-acceptance verification; falsified claims surface. | `[verifier:logs]`, `[proof:receipt]` |
| Restore / supersession | Candidate supersession or acceptance-fenced restore to baseline. | `[checkpoint:witness]`, `[restore:event]` |

---

## 4. Diagnosing Apparent Low Revision Counts

Do not infer a revision defect from `texture_turn_committed` counts: that
lifecycle event records every turn outcome, while only `TextureTurnRevision`
outcomes advance the canonical head. Before any cadence repair, record a
turn-outcome census and a revision-outcome census separately.

The known live gap (2026-09-23): the engineering-bound document has **no live
writer** — owner revisions trigger host reconcile and open the assignment
directly, but nothing writes an `AuthorAppAgent` revision back, so the owner
sees only their own last edit. The desk-RLM rectification closes this: the
texture desk metabolizes the commitment ledger into revisions under editorial
discretion.

---

## 5. Actor, Scheduling, and Resource Boundaries

- **Texture** has exactly two jobs: revise the human-readable document and
  communicate with other desks through semantic acts. No capsule, host
  filesystem, provider-routing, event-chain, or promotion authority.
- **Style-guide Textures** are planned, not implemented. Any document may later
  be designated to shape Texture's writing style; this affects prose/register,
  not authority or routing.
- **Management** remains one per computer to see every document's state,
  arbitrate resources, and maintain coherence/error correction — explicitly
  not to limit concurrency.
- **Engineering** mutation is capsule-bound. An assignment may hold N
  capability-bound capsules; 1:1 holds only through the current candidate.
- **Admission:** one live engineering assignment per computer. Owner-cast
  (owner revision admits work) is kept; delegated cast (a desk's staged
  `choir.Cast` admits work) is new admission authority and must implement the
  one-live-assignment admission ledger. Requests carry a durable
  computer-scoped arrival ordinal; selection is FIFO among non-expired
  requests; unselected requests remain pending untouched. A request expires
  when its operation reaches terminal state, a superseding owner correction
  arrives, or its deadline passes. Assignment deadlines fail rather than hang.
  Admission refusal is retryable — work stays pending and the computer never
  deadlocks.
- **Memory containment:** `memory.high` = requested, `memory.max` = 2×
  requested, `memory.events` OOM counters as feedback.
- **Parallel release gate:** parallel Textures/trajectories with N concurrent
  assignments under an admission-ledger overcommit factor is a release-gate
  requirement, proven after sequential correctness.
- **Isolation:** desk cells are model-authored Go and run in a killable
  subprocess with restricted stdlib exports; yaegi already fail-closes without
  a worker binary. Research needs a network/memory-capped boundary — untrusted
  web-derived code in-process can OOM the daemon.

---

## 6. The Self-Development Flow (Choir in Choir)

The long-term goal of Choir is the **automatic computer**: moving from changing
Choir code in an external coding harness to using the `choir` CLI to have Choir
computers modify their own code autonomously.

```text
User / Developer                  Choir computer (staging)
      │                                  │
      ├── choir run start ──────────────►│ owner revision on a bound doc
      │   "implement feature X"          │       │
      │                                  │       ▼
      │                                  │ Texture desk (sole writer)
      │                                  │       │
      │                                  │       ▼
      │                                  │ Management desk casts engineering
      │                                  │       │
      │                                  │       ▼
      │                                  │ Engineering sub-RLM authors code
      │                                  │ in a capsule-bound cell
      │                                  │       │
      │                                  │       ▼
      │                                  │ Freeze → verify → resolve
      │                                  │ commitments on the ledger
      │                                  │       │
      │                                  │       ▼
      │◄── Watch live on choir.news ─────┤ Texture metabolizes the ledger
      │                                  │ into AuthorAppAgent revisions
```

These lifecycle events do **not** determine revision numbers or cadence; the
flow above is an example, not protocol. The user monitors live progress by
opening the active Texture document in the browser while the computer safely
executes, tests, and promotes changes to its own environment.

---

## 7. Status

| Component | State |
| :--- | :--- |
| Document channel as owner input (M1) | **Landed** 2026-09-23 (deployed `3b780ed2`) |
| Engineering desk on the yaegi in-cell carrier | **Landed** (M2 partial) |
| `assign_co_super`, `tell`/`correct`/`roster` side channel | **Deleted** |
| Four persistent root desks, semantic-act surface, delegated cast | **Proposed** — R2/R3, under owner review |
| Commitment ledger, scores, materiality projection | **Proposed** — R2/R4 |
| `super_controller` replacement, kernel timer for fate sweeps | **Proposed** — R3/K |
| `actuator=tools` desk path | **Retired target** — one carrier: yaegi cells |
| `processor`/`reconciler`/`conductor` | **Deferred** — world-wire/system-one decision, out of scope |
