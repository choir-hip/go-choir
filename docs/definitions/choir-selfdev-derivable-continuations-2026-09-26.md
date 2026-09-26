---
definition_version: 3
definition_id: choir-selfdev-derivable-continuations-2026-09-26
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-26T10:15:00Z'
  source:
    canonical_ref: main@68a2e023
    deploy_identity: staging https://choir.news build.commit=68a2e023
    worktrees:
      - path: /Users/wiz/go-choir
        status: clean
        class: source
        owner: this session
        touch: read_write
        recovery: git
  predecessor:
    mission: choir-commitment-scores-packs-2026-09-26
    disposition: >-
      R4 landed+deployed 2026-09-26 (68a2e023, ci 36233806473). The
      commitment ledger has its read surface: derived accrual views, a
      falsification-visible materiality projection feeding the texture
      doc's evidence surface, score-free acting-desk packs on the cell
      frame (choir.Pack), and the learning-claims gate (flag posture) on
      the selfdev verification event payload.
    evidence_ref: docs/definitions/choir-commitment-scores-packs-2026-09-26.md
  spine_meta_goal: docs/definitions/choir-rectification-spine-2026-09-25.md
  station: M7
  owner_directive: >-
    Plan §11 M7 (on spine, after R3c): skip the harness — selfdev
    operations advance to materialization on derivable continuations,
    driven by the management cell, no external OMP/CLI stepping process.
    Proof: a self-dev op reaches a materialized change with no external
    driver.

finish:
  outcome: >-
    Self-development operations advance through the full lifecycle —
    freeze -> verify -> owner-decision recovery -> materialize —
    driven by derivable continuations on the computer's own event/wake
    substrate (the management cell observing operation state and
    casting the next step), not by an external harness process calling
    the HTTP API. The operator API stays as authority ingress for the
    human decision itself, but no stepping process is required for the
    op to advance.
  artifact: >-
    A derivable-continuation path from selfdev operation state changes
    to the next required action: the materializer reconciler invoked on
    canonical event heads (not only on API calls), and the management
    cell able to observe operation state and act on it through the
    carrier (inbox act / cast), plus a staging proof that an op reaches
    a materialized change with no external driver.
  acceptance:
    - action: >-
        Boundary probe first: name the exact seam where operation state
        changes become derivable continuations (event heads the
        materializer can reconcile on; the management cell's wake
        surface), and which lifecycle stages can advance without an
        external driver vs which genuinely require owner input.
      proves: R2-boundary discipline — acceptance names only reachable
        evidence.
      evidence_class: boundary probe + local test
    - action: >-
        A selfdev operation transitioning to a materializable state
        triggers materialization through the canonical event/wake path —
        observed with the API caller absent (no HTTP call after the
        owner's recorded decision).
      proves: derivable continuation drives the advance, not the
        harness.
      evidence_class: local test
    - action: >-
        The management cell observes operation state through the
        carrier (its inbox acts and ledger/materiality surfaces carry
        the op's fate) and can stage the next-step act — the supervision
        loop closes on the desk, not on the operator console.
      proves: the management cell drives the loop.
      evidence_class: local test
    - action: >-
        Failure/replay: a mid-advance crash leaves the operation
        recoverable — the reconciler re-derives the continuation from
        durable state, matching the strand-2 saga discipline.
      proves: no external driver is needed for recovery either.
      evidence_class: local test
  rollback: >-
    git revert + redeploy. The API-triggered reconciler path remains
    valid underneath — reverting restores API-driven advance; the
    derivable wake is additive, so rollback is fully reversible.
    Protected surfaces: canonical event commit path, selfdev operation
    store transitions, run acceptance — unchanged semantics, only the
    trigger gains a second (in-product) caller.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Self-development stops needing a human (or an out-of-band script)
    to click "next". The computer advances its own selfdev operations
    on evidence it already computes — the first honest step toward
    M11's supervised episode: supervision reads the doc, the desk does
    the driving, and the owner's decision remains the only authority
    the product actually requires.
  goodharting_would_be: >-
    Re-adding the harness under a new name: a polling loop that calls
    the same API on a timer, a drive-side service that mimics owner
    calls, or "derivable continuations" that silently replicate the
    human decision step. Acceptance insists the advance is driven by
    durable event heads the management cell can see — and that no step
    mints an owner decision it wasn't given.
  falsifiers:
    - 'Falsified: materialization still requires an API call — the
      harness moved, not left.'
    - 'Falsified: the loop mints an owner decision — the desk is
      impersonating authority.'
    - 'Falsified: a crash mid-advance needs external intervention —
      the continuation is not derivable.'

now:
  status: working
  slice: >-
    station M7 CHARTERED 2026-09-26. Substrate (re-verified at
    charter): reconcileSelfDevelopmentMaterialization runs only from
    two HTTP API sites (api_self_development.go:1013 decision path,
    :1297 rollback path) — today every lifecycle advance rides an
    external caller; the reconciler itself already re-derives per-op
    continuation from durable state (recoverSelfDevelopmentDecision +
    materializeSelfDevelopmentOperation), so the missing piece is a
    canonical trigger, not new reconciliation logic. Management is on
    the carrier (R3c) and can observe + act; R4 packs/materiality give
    it scored supervision. M9a unblocks once M7 produces something
    signable.
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: derivable-selfdev-advance
    claim: >-
      Binding the selfdev reconciler to canonical event heads (and the
      management cell's carrier surface to operation state) makes every
      post-decision lifecycle advance derivable — the op materializes
      with no external driver while the owner's decision stays the sole
      authority gate.
    test: >-
      Local: a seeded op transitions to a materializable state and
      reaches materialized without an API call after the recorded
      decision; a crash mid-advance recovers via the same reconciler.
      Deployed: platform healthy at M7 head; staging op advances on the
      derivable path where a real op is reachable.
    delta_o: >-
      With M7 landed, M9a can sign/push a real materialized change and
      the M11 episode has the management-driven loop it was ratified
      to require.
    scope_if_supported: selfdev lifecycle is product-internal; M11 prep unblocked
    status: active
    evidence_refs:
      - docs/desk-rlm-rectification-plan-2026-09-23.md §11 (M7)
      - internal/agentcore/self_development_materializer.go
  belief:
    believed_state: >-
      CHARTERED. All desks are on the carrier, the ledger read surface
      is live, but selfdev advancement still requires an external HTTP
      caller — the product's own event substrate never wakes the
      reconciler. The gap is a trigger, not reconciliation logic.
    next_observation: >-
      Boundary probe: which event heads (selfdev transitions, capsule
      freezes, verification records) should wake the reconciler, and
      how the management cell's pack/inbox should carry operation state.
  blocker_or_risk: >-
    Authority risk: the owner decision step must never be synthesized
    by the loop — the continuation path must stop exactly at the
    decision gate and wait for the real API call (or a ratified
    qualified-consensus decision receipt if the M11-vintage consensus
    surface is already live). Scope risk: M9a's push signing could drag
    into this station; keep M7 to the advance trigger only.
  next_action: >-
    Boundary probe (reconciler trigger seam + management observation
    surface), then: (1) event-head-triggered reconcile; (2) management
    carrier observation of op state; (3) crash/recovery proof; (4)
    tests; (5) landing loop.

receipts:
  - "charter: M7 = skip the harness per plan §11 (on spine, after R3c).
    Substrate re-verified: reconcileSelfDevelopmentMaterialization is
    invoked only from api_self_development.go decision (:1013) and
    rollback (:1297) handlers — no canonical wake calls it; the
    reconciler already re-derives per-op continuation from durable
    state. Boundaries: the owner decision is never synthesized by the
    loop; the derivable trigger is additive (API path stays); rollback
    is revert+redeploy. Boundary probe on the trigger seam and the
    management observation surface runs before implementation."
---

# M7 — Skip the Harness: Derivable Selfdev Continuations (station on the rectification spine)

Live station under [`choir-rectification-spine-2026-09-25.md`](choir-rectification-spine-2026-09-25.md).
Scope per [`desk-rlm-rectification-plan-2026-09-23.md`](../desk-rlm-rectification-plan-2026-09-23.md) §11
(M7, on spine): selfdev operations advance to materialization on derivable continuations, driven by the
management cell — no external OMP/CLI stepping process. Proof: a self-dev op reaches a materialized
change with no external driver.
