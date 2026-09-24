---
definition_version: 3
definition_id: choir-commitment-scores-surfacing-draft-2026-09-23
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
    mission: choir-live-desks-supervision-loop-draft-2026-09-23
    disposition: >-
      hard dependency — scores and surfacing render the ledger R2 built
      through the live desks R3 landed. Subsumes the old M6 (precommit
      yaegi surface — landed in R2's verbs) and M8 (context packs +
      learning-claims gate).
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md
  observed_artifact:
    - claim: >-
        The commitment ledger (R2) records claims with resolver, deadline,
        outcome; scores accrue per desk. What does not yet exist: score
        views, the materiality projection texture renders from, context
        packs for desk cells, and the learning-claims gate.
      claim_scope: current
      evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md

finish:
  deliver: >-
    The owner sees per-desk track records and the commitment tree's
    material state in the live doc; desk cells receive context packs
    assembled from the ledger; a learning-claims gate keeps scored
    outcomes out of the acting agent's context.
  artifact: >-
    (a) Score accrual: per-desk scores derived from resolved commitments
    with the memo's full record (typed outcome, scorer identity,
    discrepancy class, specificity — scalar views derived, never stored
    bare). (b) The materiality projection: which commitments render in the
    doc body (top-level claims, falsified claims, overdue commitments)
    vs. transclude vs. stay ledger-only — texture's editorial rule made
    explicit. (c) Context packs: desk cells open with a ledger-derived
    context pack (open commitments, relevant resolved claims, desk score
    summary) replacing ad-hoc prompt assembly. (d) The learning-claims
    gate: scores and outcomes never enter the acting agent's context —
    the epistemic-boundary invariant enforced in pack assembly.
  acceptance:
    - action: >-
        On staging, after a task completes: the doc shows the resolved
        commitment tree at idea level (claims, outcomes, scores per desk);
        a falsified commitment renders as falsified, not silently dropped.
      proves: scores and the materiality projection are live
      evidence_class: deployed proof + human inspection
    - action: >-
        Inspect a desk cell's context pack: it contains open commitments
        and relevant history but zero score fields for the acting desk's
        own unresolved work.
      proves: the epistemic boundary holds in pack assembly
      evidence_class: local test
    - action: >-
        A desk that precommits at wrong granularity (per-cell trivia)
        produces ledger noise but the doc does not flood — the materiality
        projection filters it.
      proves: editorial discretion is real, not aspirational
      evidence_class: local test + human inspection
  rollback: git revert + redeploy; scores and packs are derived views —
    the ledger is untouched.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between the ledger's full record and what the owner
    can absorb — idea-level state, not action-level noise.
  goodharting_would_be: >-
    A score view that flatters (hiding falsified commitments, averaging
    away calibration) or a materiality rule that renders everything
    (a dashboard, not supervision).

homotopy:
  realism_axis: >-
    Surfacing fidelity: from ledger-only (R2/R3) through scored +
    materiality-projected docs (this mission) to the full learning loop
    (context packs informing future desk behavior — the world-model
    compounding path).

boundaries:
  mutation_class: orange
  authority_sources:
    - docs/desk-rlm-rectification-plan-2026-09-23.md
    - docs/Precommitment Records — Engineering Memo.md
    - docs/why-texture-2026-06-15.md
    - AGENTS.md
  must_preserve:
    - the epistemic boundary: scores never in the acting agent's context
    - texture's sole-writer authority and editorial discretion
    - the ledger's full record (scores are views, never lossy storage)
  excluded:
    - the ledger schema itself (R2)
    - live desk mechanics (R3)
    - durable vocabulary (R5)
    - self-dev push/publish/proof (M7/M9–M11)
  protected_surfaces:
    - texture canonical writes
    - the commitment ledger's record integrity

now:
  status: blocked_incomplete
  slice: scores + surfacing + context packs
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
    id: r4-idea-level-surfacing
    claim: >-
      A materiality projection over the commitment ledger produces
      idea-level supervision state in the doc — the owner reads beliefs,
      commitments, and track records, not action logs.
    test: >-
      Human inspection of a live doc mid-task: does it answer "what is
      believed, what changed, what is uncertain, what decision is
      required" (why-texture's list) without reading raw events.
    edge: frame_lock — the materiality rule may not capture what a human
      finds supervisable; the first live doc may reveal the rule is wrong.
    delta_o: >-
      Owner review of the first live doc is the oracle; iterate the
      projection rule on that feedback, not on internal metrics.
    scope_if_supported: >-
      The supervision surface is genuinely idea-level; the learning loop
      has its context-pack substrate.
    status: active
    evidence_refs:
      - docs/why-texture-2026-06-15.md
      - docs/Precommitment Records — Engineering Memo.md
  decision:
    what: >-
      Scores are derived views over the full ledger record (D8); the
      materiality projection is texture's editorial rule made explicit;
      context packs are ledger-derived with the epistemic boundary
      enforced at assembly.
    kind: architecture
    status: proposal
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md
    owner_ratification_ref: pending — plan under owner review
  belief:
    believed_state: >-
      The ledger (R2) and live desks (R3) provide the substrate; this
      mission is the rendering and learning-loop layer.
    main_uncertainty: >-
      The materiality projection rule — what makes a commitment
      doc-worthy vs. ledger-only — is a product judgment that needs the
      first live doc to calibrate.
    next_observation: >-
      The owner's read of the first scored, projected doc.
  blocker_or_risk: hard dependency on R3
  next_action: promote after R3 lands

receipts: []
---

## Carried from the consensus review

- Score records keep the memo's full schema — typed outcome, scorer
  identity, discrepancy class, specificity. Scalar views are derived;
  a bare scalar breaks calibration (D8, gemini's finding).
- The materiality projection is a *rule texture applies*, not a mechanical
  render — the ledger is the foliation for provenance; the doc is
  editorial.
- Context packs replace ad-hoc prompt assembly; the learning-claims gate
  (scores out of acting-agent context) is enforced at pack assembly, not
  by prompt discipline.
- Commitment quotas: a per-assignment open-commitment bound + deadline,
  separate from the 16-intent cell tray cap (D9).
