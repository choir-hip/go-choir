---
definition_version: 4
definition_id: choir-signal-typed-commitments-2026-09-29
execution_mode: mission_orchestrator
readiness: drafted
member_of: choir-jev-supervision-metamission-2026-09-29

start:
  captured_at: '2026-09-29T22:40:00Z'
  source:
    canonical_ref: main@ac54317c
    deploy_identity: staging https://choir.news (post-M0a build, parallel-safe)
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: source
      owner: this session
      touch: read_write
      recovery: git
  predecessor:
    mission: none — parallel-safe (types/store change); M0 findings may
      reshape the schema
  observed_artifact:
    - 'choir.Precommit commits only a hypothesis string; choir.Resolve only
      a verdict word — not scoreable. Jev scoring needs typed questions +
      frozen probabilities at precommit, evidence refs at resolve, and a
      Disagreement split so scorer disagreement never reaches ActingPack.
      Owner 2026-09-29: strings→types confirmed.'

finish:
  deliver: 'Precommitments are typed records — typed question, frozen
    probability distribution, resolver — that a scorer (management via
    Jev by default) can evaluate against evidence-carrying resolutions;
    string-based legacy commitments grandfathered explicitly.'
  artifact: 'typed Precommit/Resolve/Disagreement schema in
    choir.commitment_record; string-commitment migration/grandfathering
    rule stated; ActingPack isolation preserved'
  acceptance:
    - action: 'a desk cell stages choir.Precommit with typed question +
        probabilities + resolver; the committed record carries the frozen
        distribution; Resolve carries evidence refs; a Disagreement act
        splits scorer verdict from resolver verdict'
      proves: 'commitments are machine-scoreable'
      evidence_class: deployed proof
    - action: 'a legacy string-based commitment resolves without schema
        violation under the grandfathering rule'
      proves: 'backward compatibility stated and honored'
      evidence_class: deployed proof
  rollback: 'schema is additive on the OG kind; revert + grandfathering
    keeps prior records valid'
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity,
      deployed_acceptance]

value:
  better_means: 'minimize the gap between "commitments are scoreable
    inputs to supervision" and "commitments are free-text strings a
    scorer cannot evaluate" while preserving ActingPack isolation'
  goodharting_would_be: 'typing the schema without the Disagreement split —
    scorer disagreement leaking into ActingPack, collapsing the
    epistemic boundary the whole mechanism exists to protect'

homotopy:
  realism_axis: 'type depth — string → typed question → typed question +
    frozen distribution → +evidence-ref resolution → +Disagreement split'

boundaries:
  mutation_class: red (canonical commitment schema — protected surface)
  authority_sources: [owner "yes, strings→types" 2026-09-29,
    both 9/27 panels'' flagged prerequisite, choir-doctrine.md epistemic
    boundary]
  must_preserve:
    - 'ActingPack never carries score fields'
    - 'Disagreement is a distinct act, not a Resolve verdict'
    - 'existing commitment_record kind and prior records remain valid'
  excluded:
    - 'Jev transport/scoring (M4/M5 scope)'
    - 'model-policy module (M2 scope)'
  protected_surfaces:
    - 'internal/types commitment_record kind'
    - 'rlm_reduce commit paths'
    - 'ActingPack assembly'

now:
  status: pending
  slice: 'not started — parallel-safe, may begin once M0 observations
    land'
  source_ref: main@ac54317c
  deploy_identity: unknown
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: typed-commitments-enable-scoring
    claim: 'If Precommit freezes typed questions + probabilities and
      Resolve carries evidence refs, then management can score
      commitments deterministically — the precondition for the Jev
      supervision loop'
    test: 'a scored commitment record exists on the tape with a resolved
      outcome'
    edge: 'missing_oracle — nothing reads the typed fields yet; the claim
      proves schema, not use'
    delta_o: 'first management-scored commitment in M5'
    scope_if_supported: 'all commitment-bearing desks'
    status: proposed
    evidence_refs: []
  decision:
    what: 'typed commitment schema per owner + panel prerequisite'
    kind: architecture
    status: settled
    evidence_ref: 'orientation doc §B ratified shape'
    owner_ratification_ref: 'owner: "precommitments strings→types"
      2026-09-29'
  belief:
    believed_state: 'schema change is mechanical; the risk is in the
      grandfathering rule and Disagreement split semantics'
    main_uncertainty: 'whether existing string commitments migrate,
      grandfather, or are excluded — needs a stated rule'
    next_observation: 'first typed commitment + typed resolution on the
      tape'
  blocker_or_risk: 'grandfathering rule must be stated, not assumed'
  next_action: 'author the schema: Precommit{question, distribution,
    resolver}, Resolve{verdict, evidence_refs}, Disagreement{...};
    grandfathering rule; ActingPack isolation check'

receipts: []
---
