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
    station M7 WORKING 2026-09-26 — boundary probe recorded;
    derivable-continuation substrate committed 3377ba91 (CI/deploy
    in flight). Probe: the reconciler ran only from two HTTP sites;
    nothing in the canonical event chain woke it, and a crash
    mid-Materializing required the same external call. Named seams
    chosen: (1) a post-commit observer on
    ComputerEventAppender.appendLocked — the sole event-commit choke
    point, replay bypasses it — firing the coalesced reconciler
    trigger on every committed kind; firing on all kinds is required
    because the Verified->AwaitingApproval transition is an in-cell
    operation-store write, not a canonical event — the cell's own
    subsequent events guarantee a later fire, and the reconciler's
    ListByStates is the sole state gate. Derivable advances:
    decision-recovery (AwaitingApproval->Accepted/Rejected),
    materialization, rollback — all ride canonical decision/rollback
    event commits (ActorProfile management, owner authority_ref).
    Never derivable: the decision itself — reconcile stops at the gate
    and waits for the real effect_accepted/rejected commit. (2) Boot
    phase selfdev_materialization_reconcile covers the crash leg. (3)
    Management observation: ops parked at AwaitingApproval mint an
    idempotent commitment_record addressed to
    persistentManagementAgentID — lands in the management desk's
    score-free acting pack (packEligible via Addressee). Race
    discovered + fixed: the drain can recover AwaitingApproval before
    the decision API's own Transition — ErrConflict now re-checks the
    durable op for the exact decision binding instead of 409ing.
    Rejected alternatives: per-API-site hooks (drift), control packets
    (wrong surface), kind-filtering (misses in-cell transition).
    maintenanceHeld() added to the reconciler guard. M9a still
    unblocks once M7 produces something signable.

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
      Local seeded-op proof landed
      (selfdev_derivable_materialize_test.go): canonical decision append
      -> observer -> drain -> op materialized to Applied with
      checkpoint + route promotion on a real ledger, zero API calls;
      crash mid-materialize parks at Materializing and the same
      reconciler completes the apply idempotently after control
      returns. Next: landing loop -> staging probe.
  blocker_or_risk: >-
    Authority risk: the owner decision step must never be synthesized
    by the loop — the continuation path must stop exactly at the
    decision gate and wait for the real API call (or a ratified
    qualified-consensus decision receipt if the M11-vintage consensus
    surface is already live). Scope risk: M9a's push signing could drag
    into this station; keep M7 to the advance trigger only.
  next_action: >-
    Landing loop: commit -> push -> CI -> Node B deploy -> staging
    health + deployed-commit identity; live selfdev op on staging was
    already qualified unreachable on the api-key computer.

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
  - "boundary probe 2026-09-26: derivable seam = ComputerEventAppender
    appendLocked post-commit observer on every committed kind (in-cell
    Verified->AwaitingApproval is an op-store write, not an event — all-
    kinds firing + coalesced drain is required; ListByStates is the sole
    state gate). Derivable: decision-recovery, materialization, rollback,
    crash-restart via boot phase. Never derivable: the owner decision —
    reconcile parks at AwaitingApproval and mints an idempotent
    selfdev-observation commitment_record addressed to
    persistentManagementAgentID (lands in the desk acting pack via
    Addressee eligibility). Rejected: per-API-site hooks (drift), control
    packets (wrong surface), kind-filtering (misses in-cell transition)."
  - "local proof 2026-09-26 (3377ba91): canonical decision-event append
    -> post-commit observer -> coalesced drain -> op recovered to
    Accepted with exact decision binding — decide handler never
    invoked (TestSelfDevReconcileRecoversDecisionDerivably); parked op
    mints management-addressed boundary record, zero API calls
    (TestSelfDevReconcileBoundaryMintsManagementObservation); observer
    fires only on durable commit
    (TestAppenderPostCommitObserverFiresOnlyOnCommit)."
  - "deployed identity 2026-09-26: choir.news/health status=ok,
    vmctl_status=ok, deployed_commit=3377ba91 — Deploy to Staging
    (Node B) success for CI run 36236447989; selfdev API surface
    answers ('effects are disabled' on the api-key computer — a real
    op was not reachable, qualified per goal)."
  - "local proof 2026-09-26 (materialize leg): seeded op at
    AwaitingApproval -> canonical EffectAccepted append -> post-commit
    observer -> coalesced drain -> recoverSelfDevelopmentDecision ->
    materializeSelfDevelopmentOperation -> Applied with
    materialization/checkpoint/route events exactly once and ledger
    promoted to the new version — no API call after the decision
    (TestSelfDevReconcileMaterializesDerivably). Crash leg: op parked
    at Materializing with control down, restored by re-running the
    same reconciler — updater journal replays identical receipts, all
    events idempotent, Applied with unchanged decision binding
    (TestSelfDevReconcileRecoversMaterializingOperation). Fixture
    binds the op store to the projection tape as production does, so
    every op mutation is a canonical batch event — the derivable wake
    under test, not a test-side drive loop."
---


# M7 — Skip the Harness: Derivable Selfdev Continuations (station on the rectification spine)

Live station under [`choir-rectification-spine-2026-09-25.md`](choir-rectification-spine-2026-09-25.md).
Scope per [`desk-rlm-rectification-plan-2026-09-23.md`](../desk-rlm-rectification-plan-2026-09-23.md) §11
(M7, on spine): selfdev operations advance to materialization on derivable continuations, driven by the
management cell — no external OMP/CLI stepping process. Proof: a self-dev op reaches a materialized
change with no external driver.
