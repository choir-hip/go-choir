---
definition_version: 3
definition_id: choir-live-desks-supervision-loop-draft-2026-09-23
execution_mode: mission_orchestrator
draft: true

start:
  captured_at: '2026-09-23T21:00:00Z'
  source:
    canonical_ref: main@b0adf6f7
    deploy_identity: staging https://choir.news build.commit=4de7fdf9
  worktrees:
    - path: /Users/wiz/go-choir
      status: unknown
      class: unknown
      owner: unknown
      touch: read_only
      recovery: reconcile at charter
  predecessor:
    mission: choir-commitment-ledger-desk-carrier-draft-2026-09-23
    disposition: >-
      hard dependency — R3 builds live desk actors on R2's carrier + verbs
      and K's derivable wakes. Both must land first; building live desks on
      process-local wakes recreates the wrong-path cluster under clean
      names (consensus finding).
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md
  observed_artifact:
    - claim: >-
        The engineering-bound document has no live writer: the desk agent
        never runs (engineering_desk.go:18); owner revisions trigger host
        reconcile; completion reports land as trajectory events; nothing
        writes AuthorAppAgent revisions. The owner sees their own last
        edit.
      claim_scope: current
      evidence_ref: internal/agentcore/engineering_desk.go
    - claim: >-
        The fate sweeps (deadline cancel, stranded-freeze resume) run only
        inside persistent-Super selection (super_controller.go:333) with no
        periodic ticker — strand-2's 5h wedge mechanism.
      claim_scope: current
      evidence_ref: internal/agentcore/super_controller.go
    - claim: >-
        Management cannot address engineering: Super may message only
        texture and research (agentprofile.go:115-116); assign_co_super is
        deleted. Restoring the leg is a policy change, not wiring.
      claim_scope: current
      evidence_ref: internal/agentprofile/agentprofile.go

finish:
  deliver: >-
    The owner opens choir.news mid-task and sees a live-updating Texture
    document showing ongoing work state — open commitments, resolved
    claims, desk scores — written by a live texture desk, not a host
    projection.
  artifact: >-
    Deployed staging state where: (a) desk actors are live root RLMs —
    revision occurrences wake the bound desk via the kernel's derivable
    wakes (not process-local scheduling); (b) the management desk (one per
    computer, D1 ratified) casts engineering assignments via choir.Cast;
    (c) reports return as acts that resolve commitments on the ledger;
    (d) the texture desk (per-doc, D3) writes AuthorAppAgent revisions
    metabolizing ledger traffic under editorial discretion — materiality
    projection, not org-chart dump; (e) super_controller.go is replaced,
    not beside; the fate sweeps moved to the kernel's periodic timer (D11);
    (f) verification assignments stay host-opened (D10).
  acceptance:
    - action: >-
        On staging: owner creates an engineering-bound doc and revises it
        with a real task; observe the texture desk wake, the management
        desk cast, the engineering assignment bind and execute, reports
        resolve commitments, and the doc head advance with AuthorAppAgent
        revisions — all on canonical evidence.
      proves: the supervision loop is live end-to-end
      evidence_class: deployed proof
    - action: >-
        Kill the runtime process mid-task; restart; observe the pending
        cast/commitment resume from the tape with no sweep or process-local
        timer — the desk's open commitments re-derive from the ledger.
      proves: desk liveness is restart-durable (K's contract exercised)
      evidence_class: deployed proof
    - action: >-
        Reproduce the strand-2 shape (a wedged assignment): observe the
        overdue commitment surface in the doc and the kernel timer fire
        the reconcile — not the Super-selection path.
      proves: the wedge class is closed by live actors + kernel timer
      evidence_class: deployed proof
    - action: >-
        The doc shows idea-level state: top-level claims, falsified claims,
        overdue commitments, per-desk scores — not a per-cell action log.
        Texture's revisions are genuine authoring turns, not per-milestone
        projections (doctrine: no revision per execution milestone).
      proves: supervision is idea-level, not action-level
      evidence_class: human inspection
  rollback: git revert + redeploy; the desk actors are additive actors —
    the host reconcile path can be restored as fallback during cutover.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between the document's semantic head and the actual
    work state — the owner should see beliefs, commitments, and resolutions
    as they happen, not their own last edit.
  goodharting_would_be: >-
    A doc that advances by host projection or per-milestone revision —
    motion without authorship. Or a live desk that wakes but whose cells
    strand on process-local state — liveness without durability.

homotopy:
  realism_axis: >-
    Desk liveness: from phantom desk agents + host reconcile (current)
    through live desks on derivable wakes with the supervision loop (this
    mission) to the full multi-actor RLM fabric (world-wire phase). The
    middle rung is the real actors on the real substrate — not a simulation.

boundaries:
  mutation_class: red
  authority_sources:
    - docs/desk-rlm-rectification-plan-2026-09-23.md
    - docs/why-texture-2026-06-15.md
    - docs/texture-live-supervision-architecture.md
    - docs/archive/choir-event-driven-rlm-ontology-minimal-2026-09-15.md
    - AGENTS.md
  must_preserve:
    - texture as sole agent writer (AuthorAppAgent) per document
    - the kernel's derivable-wake contract — no process-local wakes
    - host-opened verification assignments (D10)
    - the one-live-engineering-assignment admission rule
    - the epistemic boundary (scores never in the acting agent's context)
  excluded:
    - score accrual tuning and context packs (R4)
    - durable vocabulary migration (R5)
    - processor/reconciler/email/conductor changes
    - the commitment record schema itself (R2)
  protected_surfaces:
    - canonical event appends and texture canonical writes
    - the assignment fate saga
    - the kernel's dispatch/delivery contract
    - the object graph

now:
  status: blocked_incomplete
  slice: live desks + supervision loop
  source_ref: main@b0adf6f7
  deploy_identity: staging https://choir.news build.commit=4de7fdf9
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: r3-live-supervision-loop
    claim: >-
      Live desk actors on derivable wakes, with management casting
      engineering and texture writing revisions from the commitment
      ledger, produce a live-updating supervision document — the product
      point — without recreating the wrong-path cluster.
    test: >-
      The staging acceptance: doc advances mid-task via AuthorAppAgent
      revisions; restart resumes pending casts; the wedge reproduction
      surfaces via the ledger + kernel timer.
    edge: frame_lock — texture's editorial discretion may not be
      expressible as a cell policy; a desk that renders every ledger node
      produces a status dashboard, not idea-level supervision.
    delta_o: >-
      The materiality projection rule (top-level claims, falsified claims,
      overdue commitments) is the first approximation; the acceptance
      includes human inspection of doc quality, not just motion.
    scope_if_supported: >-
      The four desks are live root RLMs; the supervision loop closes;
      the product point is demonstrable on staging.
    status: active
    evidence_refs:
      - docs/desk-rlm-rectification-plan-2026-09-23.md
      - docs/texture-live-supervision-architecture.md
  decision:
    what: >-
      Desks are root RLMs on the kernel's derivable wakes; management is
      one-per-computer (D1 ratified); texture is per-doc (D3); verification
      stays host-opened (D10); fate sweeps move to the kernel timer (D11);
      super_controller.go is replaced, not preserved beside.
    kind: architecture
    status: proposal
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md
    owner_ratification_ref: pending — plan under owner review
  belief:
    believed_state: >-
      All pieces are designed; the hard part is texture's editorial
      discretion as a cell policy and the super_controller replacement's
      blast radius.
    main_uncertainty: >-
      Whether the desk wake/cell model can express texture's editorial
      judgment well enough to produce idea-level revisions rather than
      a ledger dump.
    next_observation: >-
      The first live doc on staging: does it read as supervision or as
      a log.
  blocker_or_risk: >-
    Hard dependency on K (derivable wakes) and R2 (carrier + verbs +
    ledger). The super_controller replacement is the largest single
    deletion.
  next_action: promote after K and R2 land

receipts: []
---

## The supervision loop, concretely

```text
owner revision → texture doc head moves
  → revision occurrence wakes texture desk (kernel wake, not reconcile)
  → texture desk cell: reads diff, decides semantic change
    → choir.Cast(management, objective) — delegated cast
      → management desk wakes; cell: decomposes, precommits
        → choir.Cast(engineering, spec) — opens assignment
          → engineering sub-RLM cells execute (capsule-bound)
          → choir.Report(management, claim, evidence) — resolves cast
        → management cell: choir.Report(texture, synthesis, refs)
      → texture desk wakes on report; cell: choir.Precommit? revise doc
        → AuthorAppAgent revision — the doc head moves
owner sees: the doc advancing with beliefs, commitments, resolutions
```

## Hard requirements carried from the consensus review

- Desk wakes are kernel derivable wakes — never process-local scheduling,
  never a sweep. R3 on process-local wakes = the wrong-path cluster under
  clean names.
- Texture revisions are genuine authoring turns — doctrine forbids
  revision-per-milestone and synthetic projections
  (`texture-live-supervision-architecture.md:82-84`).
- The materiality projection is texture's editorial choice, not a
  mechanical render of the commitment tree.
- `super_controller.go` is replaced — its wake loops, pending-packet
  injection, and fate sweeps move to the kernel or die; nothing sits
  beside it.
- The stranded-frozen reconcile and deadline sweep move to the kernel's
  periodic timer (D11) — they are physical-state reconciles the tape
  cannot derive, and they must not be gated on desk selection.
