---
definition_version: 4

# One protocol between agents: precommitment records. Owner 2026-10-11:
# "lock down the system so that all interagent comms are done through the
# precommitment records protocol ... Gotta bring the future forward";
# "Yes, this is a great plan", with the cross-computer scoring frame.
# Design: docs/precommitment-protocol-design-2026-10-11.md. Theory:
# docs/prospective-self-alignment-2026-10-09.md. Unblocks Autopaper W2.
readiness: executable

review:
  reviewer: owner (design approved 2026-10-11)
  frozen_ref: fd7426df
  verdict: accept
  evidence_ref: owner 2026-10-11 "Yes, this is a great plan"

metamission:
  stations:
    - id: P1
      path: docs/definitions/choir-protocol-p1-record-first-controls-2026-10-11.md
      readiness: reviewed
      status: working
      depends_on: []
    - id: P2
      path: none (one surface — Cast and Ask replace Texture's controls; Message, Emit and Texture-write ProductAPI routes deleted)
      readiness: intent
      status: pending
      depends_on: [P1]
    - id: P3
      path: none (predictions required on Cast, Ask and Escalate; count resolver)
      readiness: intent
      status: pending
      depends_on: [P2]
    - id: P4
      path: none (scores as separate records; calibration visible in desk packs)
      readiness: intent
      status: pending
      depends_on: [P3]
    - id: P5
      path: docs/definitions/choir-autopaper-w2-living-textures-2026-10-11.md (re-authored on the protocol)
      readiness: intent
      status: pending
      depends_on: [P4]

start:
  captured_at: 2026-10-11T16:20:00Z
  source:
    canonical_ref: main f1be9e2f
    deploy_identity: staging choir.news 25c8b5b0
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      problems/inter-agent-communication-bypasses-precommitment-records-2026-10-11.md:
      Texture's 283 dispatches in two days of staging traces were lifecycle
      controls outside the ledger; reports (216) came back as records; no
      desk called Precommit or Resolve.
    - >-
      S0m (complete 2026-10-01) made the semantic acts record-native through
      CommitLifecycleAct (record plus derived packet in one commit).

finish:
  deliver: >-
    Every inter-agent hand-off is a commitment record addressed to its
    recipient; every request for work carries a scored prediction; scores
    are separate records by named scorers; Autopaper W2 runs on it.
  artifact: P1-P5 landed on staging with deployed acceptance.
  acceptance:
    - action: trace audit over an AI news demo and an M11 rerun
      proves: zero off-ledger dispatches; every request for work carries a prediction that resolves
      evidence_class: deployed proof
  rollback: revert each station; records already written stay (append-once)
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    More of the system's hand-offs are measured predictions with scored
    outcomes, at the same or lower spend.
  goodharting_would_be: >-
    Records written beside an unchanged side channel; predictions that say
    nothing; scores stored inside the claim they judge.

homotopy:
  realism_axis: >-
    records under the existing controls -> one surface -> required
    predictions -> separate scores -> Autopaper on it.

boundaries:
  mutation_class: red (lifecycle reducer, evidence ledger, desk surface)
  authority_sources:
    - owner 2026-10-11 (direction and plan approval)
    - docs/precommitment-protocol-design-2026-10-11.md
  must_preserve:
    - Texture is the sole writer of a document's revisions
    - a crash restart never resumes work
    - records are append-once; a replayed command mints nothing new
  excluded: [signing and commit-reveal (need per-computer keys), cross-computer publishing (W3)]
  protected_surfaces: [internal/store lifecycle commands, internal/agentcore reducer, internal/yaegikernel surface, internal/types/commitment.go]

now:
  status: working
  slice: P1 — record-first controls
  source_ref: f1be9e2f
  deploy_identity: staging 25c8b5b0
  candidate: none
  conjecture:
    id: PR-1
    claim: >-
      Texture's controls can mint their cast records inside the same
      lifecycle commit as their packets without changing any desk's
      behavior, which makes every Texture dispatch visible on the ledger.
    test: trace audit after an AI news demo on P1
    edge: missing_oracle
    delta_o: count of cast records whose packet came from a control, against controls issued
    scope_if_supported: Texture's controls on one computer
    status: proposed
    evidence_refs: []
  decision:
    what: design §9 decisions take their recommended defaults until the owner answers
    kind: architecture
    status: provisional
    evidence_ref: docs/precommitment-protocol-design-2026-10-11.md §9
  belief:
    believed_state: the ledger carries reports only; dispatch is off-ledger
    main_uncertainty: whether the control accumulator can bind each packet to a record without a second commit
    next_observation: P1's file review, then the code
  blocker_or_risk: none
  next_action: build P1 (store tests first); land it; trace audit on staging

receipts: []
---

# One protocol between agents

The design is
[docs/precommitment-protocol-design-2026-10-11.md](../precommitment-protocol-design-2026-10-11.md).
This metamission lands it in five stations, each through the landing loop.
P1 changes no desk surface; it makes every Texture dispatch a record. Each
later station removes a side channel or adds a measurement.
