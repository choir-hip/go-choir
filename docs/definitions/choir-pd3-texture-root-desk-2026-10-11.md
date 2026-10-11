---
definition_version: 4

# PD3 — one Texture root desk per computer, with a sub-RLM per document, and
# a master document. Station of
# definitions/choir-persistent-desks-metamission-2026-10-11.md. Owner
# 2026-10-11: "Should texture be a desk per document, or should there be one
# texture desk with a sub RLM per document? And therefore, there would be
# one master document as well." Answer accepted ("this all sounds great").
readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-11T01:35:00Z
  source:
    canonical_ref: main b3dbc686
    deploy_identity: staging choir.news b3dbc686
  worktrees:
    - path: /Users/wiz/go-choir
      status: see metamission start
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Texture agents are texture:<doc>, one per document
      (internal/textureowner/handler.go, store lifecycle_texture_target.go
      expectedTextureAgentID). There is no computer-level Texture.
    - >-
      Doctrine I26: four persistent root RLM desks per computer; engineering
      has root plus per-assignment sub-RLM cells. I26 also: "Later parallel
      Textures/trajectories ... require an admission-ledger release gate
      after sequential correctness."
    - >-
      CLAUDE.md: "Texture renders that ledger into the owner-facing
      supervision surface." No such document exists; the commitment ledger
      is read through per-document views.

finish:
  deliver: >-
    The owner has one master document, written by the Texture root desk,
    that says what every document is about, what is open, what each desk is
    doing, and what needs the owner. Each document is still revised by its
    own Texture sub-RLM, the only writer of that document.
  artifact: >-
    A Texture root agent per computer that receives every Texture wake and
    routes it to the document's sub-RLM; per-document sub-RLMs holding that
    document's write authority (existing ApplyTextureTurn CAS); a master
    document on the computer authored only by the root; isolation so a
    sub-RLM failure does not stop the root or other documents.
  acceptance:
    - action: >-
        Staging: two documents with research; both revise; the master
        document lists both with their open work; a forced failure in one
        document's sub-RLM leaves the other document and the master
        revising.
      proves: root routing, per-document authority, isolation, master document
      evidence_class: deployed proof (test computer)
  rollback: revert; per-document Texture agents return
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize what the owner must open to supervise the computer (target: one
    master document) and Texture desk identities per computer (target: one
    root), while preserving per-document sole-writer authority, isolation
    between documents, and Texture's turn latency.
  goodharting_would_be: >-
    A master document that is a static list regenerated on a timer; a root
    that serializes every document behind one run so documents slow each
    other down; per-document authority silently widened to the root.

homotopy:
  realism_axis: >-
    root that only routes wakes to today's per-document agents -> master
    document from the ledger -> sub-RLMs as children of the root with
    sessions (PD2) -> parallel sub-RLMs behind the I26 release gate.

boundaries:
  mutation_class: red (Texture canonical writes, wake routing)
  authority_sources: [owner statement 2026-10-11, docs/choir-doctrine.md I26]
  must_preserve:
    - per-document sole writer with base-revision CAS
    - research and management directed only by the Texture that owns the work
    - a sub-RLM failure is contained to its document
    - crash restart closes work
  excluded:
    - cross-computer documents (PD4)
  protected_surfaces: [internal/textureowner, internal/store lifecycle, Texture commit path]

now:
  status: working
  slice: not started (waits on PD2)
  source_ref: b3dbc686
  deploy_identity: staging b3dbc686
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: PD3-1
    claim: >-
      If one Texture root routes all documents and writes a master document,
      the owner supervises the computer from one place without losing
      per-document latency or isolation.
    test: staging acceptance above, plus turn latency per document before and after
    edge: frame_lock
    delta_o: >-
      name the master document's object in the ontology (what it renders,
      who writes it) before building, so "supervision surface" cannot be
      satisfied by any list
    scope_if_supported: one computer, sequential Texture turns
    status: proposed
    evidence_refs: []
  decision: {what: none, kind: architecture, status: proposal, evidence_ref: none, owner_ratification_ref: owner 2026-10-11 accepted the direction}
  belief:
    believed_state: per-document Texture agents with no root; no master document
    main_uncertainty: >-
      Whether the root is a run-level router over today's agents (smaller)
      or the sub-RLMs become children of the root's session (needs PD2).
    next_observation: map every place that derives texture:<doc> and every Texture wake path
  blocker_or_risk: none
  next_action: after PD2, write the design (wake routing, authority, master document object) and answer the standing questions

receipts: []
---

# PD3 — Texture root desk and master document

Texture becomes one desk per computer, like the other three. Each document
gets a sub-RLM that is still its only writer. The root routes wakes, keeps
the documents coherent with each other, and writes a master document: the
owner's one place to see every document, every open commitment and every
desk.
