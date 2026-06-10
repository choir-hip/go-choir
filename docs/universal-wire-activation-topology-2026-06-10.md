# Universal Wire — Activation Topology Checkpoint

Date: 2026-06-10

Status: **architecture checkpoint before Slice 3 code** (problem-documentation-first).

Requirements contract:
[choir-wire-source-to-vtext-spec-2026-06-09.md](choir-wire-source-to-vtext-spec-2026-06-09.md)
(amendments in this document supersede conflicting activation prose until spec
is patched).

Mission:
[mission-wire-community-news-v1.md](mission-wire-community-news-v1.md)

Belief ledger:
[mission-report-wire-community-news-2026-06-09.md](mission-report-wire-community-news-2026-06-09.md)

---

## Product name

**Universal Wire** is the product name (formerly Global Wire / Community Wire in
user-facing copy). This is a product decision, not a compatibility shim.

| Surface | Rule |
|---------|------|
| User-visible copy, app name, mission/docs | Rename to Universal Wire **now** (Workstream naming commit) |
| Canonical edition alias | `universal-wire/Wire.vtext` replaces `global-wire/Wire.vtext` |
| Old edition alias | **Deleted** after one-time migration (migration run id + zero-references proof); no redirect |
| API routes | Renamed in the same commit that deletes old routes and their tests; **no route aliases** |
| Infra ids (e.g. `global-wire-platform` VM) | May remain immutable infra identifiers; document the split in migration runbook |
| SourceMaxx / source-maxx symbols | **Not renamed** — deleted per Workstream 1 (Deletion Ledger) |

---

## Core invariant

**Only VText agents and humans write VText revisions** — and humans write only
versions they own.

Every architecture error is an agent or role being handed a pen it must not
hold. Processors and reconcilers emit **requests** and **evidence**; VText runs
the single-writer autoregressive loop.

---

## Four-layer topology (approved)

```mermaid
flowchart TB
  subgraph L0["Layer 0 — Ingestion (acyclic)"]
    SC[sourcecycled]
    IE[ingestion events]
    SC --> IE
  end

  subgraph L1["Layer 1 — Understanding (acyclic)"]
    P[processor]
    IE --> P
    P -->|"spawn vtext only"| VT0[VText doc opened / wake scheduled]
  end

  subgraph L2["Layer 2 — Authorship (autoregressive, single writer)"]
    VT[VText revision loop]
    R[researcher on doc channel]
    S[super tree via request_super_execution]
    VT0 --> VT
    VT -->|spawn| R
    VT -->|request| S
    R -->|findings wake| VT
    S -->|delivery wake| VT
    VT -->|edit_vtext| REV[canonical revision n]
    REV --> VT
  end

  subgraph L3["Layer 3 — Corpus (acyclic triggers)"]
    PUB[autonomous publish]
    REC[reconciler]
    UF[user edit on published platform doc]
    SCHED[scheduled sweep]
    REV --> PUB
    PUB -->|debounced batch| REC
    UF -->|corpus-change| REC
    SCHED --> REC
    REC -->|"vtext wake request on doc_id"| VT
  end
```

**Edition** `universal-wire/Wire.vtext` is an ordinary VText document. Reconciler
schedules a **VText wake request** on the edition `doc_id`; there is no
edition-writer role and reconciler never calls `edit_vtext`.

**VText loop:** autoregressive, not recurrent — each revision conditions on
explicit `base_revision_id` plus worker deliveries. Workers are inputs to the
next step, not parallel authors.

---

## Activation matrix

| Role | Lawful inbound | Lawful outbound | Loop? |
|------|----------------|-----------------|-------|
| **sourcecycled** | adapter fetch cycles | ingestion events, processor dispatch queue | No |
| **processor** | ingestion events **only** (via sourcecycled dispatch) | `spawn_agent role=vtext` only; watch-lists/checkpoints for low-signal items | No |
| **VText agent** | processor/reconciler wake requests; worker deliveries; human edits **on owned docs** | `spawn_agent role=researcher` on doc channel; `request_super_execution`; `edit_vtext` | **Yes** (Layer 2) |
| **researcher** | VText spawn on document channel | evidence packets → wake VText | No |
| **super tree** | VText `request_super_execution` | deliveries → wake VText | No |
| **reconciler** | debounced post-publish batch; scheduled sweep; corpus-change (user fork/edit on **published platform** docs) | VText **wake requests** (correction/synthesis/edition doc); durable checkpoints — **never** `edit_vtext` | No |
| **human** | product UI on owned or platform docs | `edit_vtext` on **owned** docs only; platform published edit → user-owned version + corpus-change | Owned docs only |
| **prompt bar / conductor** | owner prompts | editorial supervision; **not** ingestion, processor, or story creation | No |

### Forbidden edges (negative proofs required)

- prompt bar → ingestion event or processor run
- processor → researcher or super
- reconciler → `edit_vtext` or direct edition mutation
- per-cycle reconciler queued on ingestion handoff (current code violation)
- user edit on published **platform** canon → in-place canonical mutation
- `global-wire` routes, aliases, or user-visible strings after Universal Wire migration

---

## Event catalog (dispatch owners)

| Event | Emitter | Dispatches | Notes |
|-------|---------|------------|-------|
| `ingestion_event` | sourcecycled after `SaveItems` | processor run(s) | Only lawful story-creation entry |
| `vtext_wake_request` | processor, reconciler | VText agent revision run | Carries `doc_id`, brief, source handles |
| `vtext_revision` | VText `edit_vtext` | (internal) next loop or publish eligibility | Provenance recorded at write time |
| `publish` | autonomous publish path (Community Cloud policy) | platformd projection; debounced reconciler | No operator approval gate on Community Cloud |
| `corpus_change` | publish; user fork/edit on published platform doc; promotion | reconciler (debounced) | Idempotent reconciler key TBD in implementation |
| `reconciler_sweep` | scheduler | reconciler | Periodic corpus review |

**Reconciler debounce:** configurable `N` publishes or `T` seconds. Never on
processor submit, never on in-flight drafts, never per ingestion cycle.

**Processor low-signal path:** watch-lists and `submit_coagent_update`
checkpoints only; do not open a VText per noise item.

---

## Publication policy

| Deployment | Publish path |
|------------|--------------|
| **Universal Wire (Community Cloud)** | Autonomous — no operator approval gate |
| **Private Wire instances** | Per-deployment policy may enable human gating |

Load-bearing guards (must be verified in acceptance, not assumed):

- no edition inclusion without eligible article VText (programmatic guard — not anthropomorphic “approval”)
- fidelity checks on publish projection
- update-awareness rendering for corrections (publish-then-correct is intended behavior)

---

## Human fork/claim loop (platform published docs)

A human edit to a **published platform** VText never mutates platform canon.

```text
user edit on published platform doc
  -> user-owned version (canonical for that user)
  -> corpus_change signal
  -> reconciler assembles evidence packet from published corpus
  -> reconciler emits vtext_wake_request on platform canonical doc_id
  -> VText agent revision (Layer 2 loop: optional researcher/super)
  -> new platform canonical version confirms / disputes / acknowledges
  -> MUST cite or transclude the user version responded to
```

Authorship provenance (`human` | `vtext-agent`, plus run id) is recorded at
**write time** on every revision, never inferred afterward.

On docs the user owns (their computer's VTexts, forks, editions), human edits
are simply canonical revisions — no fork/claim loop.

**Dependency:** provenance-on-`edit_vtext` may need implementation before
fork/claim staging proof (deliverable e).

---

## Workstream execution order

Execute **sequentially**. Do not start later workstreams until the prior
deliverable evidence is recorded in the mission report.

```text
(a) This architecture checkpoint + mission doc amendments (docs only)     <- NOW
(b) Workstream 1 — Deletion Ledger
(c) Workstream naming — Universal Wire rename/migration
(d) Workstream 2 — Activation graph (Slice 3 dispatch + negative proofs)
(e) Staging acceptance (two proofs)
```

### (b) Workstream 1 — Deletion Ledger (Slice 0 debt)

**Problem:** `BuildSourceMaxxHandoff` and related symbols survive on the active
ingestion path while invariant 23 treats SourceMaxx as deleted. Slices 1–2
landed on this spine.

**Order inside Workstream 1:**

1. **Replace** ingestion handoff + dispatch with neutral vocabulary (same
   behavior, non-legacy names).
2. **Delete** legacy symbols per mission invariant 23 list.
3. Grep-clean proof per symbol (recorded command, empty output).
4. Types/routes deleted with tests in the same commits.
5. Staging data purge (run id + zero-row queries).
6. Remove read-compat shims after purge.

Renaming, wrapping, or view-hiding is not deletion. If a symbol cannot be
deleted, stop and document evidence — do not shim.

**Named symbols (non-exhaustive; see mission v1 invariant 15):**

- `GlobalWireStory` types and stored seed rows
- StoryGraph authority
- all `SourceMaxx` / `source_maxx` / `source-maxx` forms including
  `BuildSourceMaxxHandoff`, `sourceMaxxRuntimeDispatcher`, `source-network` shims
- seeding helpers; legacy style-source / publication / newsletter / autoradio routes

### (c) Universal Wire rename/migration

After (b). User-visible copy, app name, docs; API route cutover; edition alias
migration; delete `global-wire/Wire.vtext` alias with zero-references proof.

### (d) Workstream 2 — Activation graph

- Remove per-cycle reconciler from ingestion handoff dispatch
- Processor inbound ingestion-only; outbound vtext-only; harden generic run API
- Reconciler on publish debounce + schedule + corpus-change only
- Reconciler emits `vtext_wake_request`, never holds pen
- Negative proofs (item 7 in mission checkpoint)

### (e) Staging acceptance

**Proof 1 — ingestion chain:**

```text
ingestion cycle
  -> processor (activation_origin=ingestion_event)
  -> VText
  -> autonomous publish (through publication guard)
  -> debounced reconciler (activation_origin=publish event)
  -> reconciler correction-request
  -> VText-agent revision on existing doc
```

**Proof 2 — fork/claim loop:**

```text
user edit on published platform doc
  -> corpus-change signal
  -> reconciler evidence packet
  -> VText-agent response version citing/transcluding user version
```

Plus Phase A negative proofs (prompt bar cannot create Wire stories) per Slice 4.

---

## Spec / mission contradictions resolved here

| Prior text | Resolution |
|------------|------------|
| Slice 3: processor → researcher/VText | processor → **VText only**; VText owns researchers |
| Slice 3b separate from autonomous publish | Community Cloud autonomous publish is in Workstream 2 acceptance; platform-internal platformd projection remains implementation detail |
| Reconciler per-cycle on handoff | **Removed** — violates feed-forward |
| Spec: processor requests researchers | **Request via VText wake brief**, not processor spawn |
| Slice 0 marked done while SourceMaxx active | **False** — Workstream 1 reopens Slice 0 until grep-clean |
| `global-wire/Wire.vtext` | **`universal-wire/Wire.vtext`** after migration (c) |

---

## Related documents to update when implementing

- [mission-wire-community-news-v1.md](mission-wire-community-news-v1.md) — phased route, checkpoint state
- [choir-wire-source-to-vtext-spec-2026-06-09.md](choir-wire-source-to-vtext-spec-2026-06-09.md) — Activation section
- [mission-report-wire-community-news-2026-06-09.md](mission-report-wire-community-news-2026-06-09.md) — evidence per deliverable (a)–(e)
