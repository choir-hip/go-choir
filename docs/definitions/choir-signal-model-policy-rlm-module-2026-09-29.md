---
definition_version: 4
definition_id: choir-signal-model-policy-rlm-module-2026-09-29
execution_mode: mission_orchestrator
readiness: drafted
member_of: choir-jev-supervision-metamission-2026-09-29

start:
  captured_at: '2026-09-29T22:40:00Z'
  source:
    canonical_ref: main@ac54317c
    deploy_identity: staging https://choir.news (post-M0a/M1)
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: source
      owner: this session
      touch: read_write
      recovery: git
  predecessor:
    mission: 'M0a (research RLM — evals must measure the post-deletion
      surface) + M1 (scored evals need typed commitments; raw
      instrumentation does not)'
  observed_artifact:
    - 'model/provider/effort is static via model-policy.toml; conductor
      does NOT pick models (it classifies prompt-bar intent → appagent).
      Owner: model policy is an RLM module — persistent per-desk change
      inside a Go cell + parametric selection at cast time.'

finish:
  deliver: 'Desks own their model config: persistent per-desk policy a
    cell can change (texture changes its own model in a Go cell and it
    sticks); parametric selection at cast time (research picks the
    sub-RLM''s model at call time); evals run as parallel RLM casts;
    per-turn timing/token records feed every downstream station.'
  artifact: 'choir model-policy module verbs (persistent + parametric);
    parallel RLM eval-cast surface; per-turn timing/token/cost records;
    QA fixture/golden set; conductor stays policy-routing only'
  acceptance:
    - action: 'a texture cell changes its own model persistently; the
        next turn runs the new model'
      proves: 'persistent per-desk model policy'
      evidence_class: deployed proof
    - action: 'a research cell casts a sub-RLM with a model chosen at
        call time; the sub-RLM runs that model'
      proves: 'parametric model selection'
      evidence_class: deployed proof
    - action: 'an eval matrix runs N model/effort configs as parallel
        RLM casts and emits a score-matrix artifact'
      proves: 'the eval surface exists as the instrument M3 needs'
      evidence_class: deployed proof
    - action: 'conductor routes prompt-bar intent to the right appagent
        without touching model selection'
      proves: 'conductor = policy routing, not model routing'
      evidence_class: deployed proof
  rollback: 'module is additive; revert restores static model-policy.toml;
    parametric callers fall back to default'
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity,
      deployed_acceptance]

value:
  better_means: 'minimize the gap between "desks can choose their model"
    and "model is static config" while keeping conductor on policy
    routing only — the divergence between RLM-owned config and
    harness-owned config'
  goodharting_would_be: 'adding model switching to conductor instead of a
    desk-owned module — the wrong authority inheriting the decision'

homotopy:
  realism_axis: 'config autonomy — static → persistent per-desk →
    parametric at cast time → full eval-driven selection'

boundaries:
  mutation_class: red (model/provider selection is a protected routing
    surface)
  authority_sources: [owner direction 2026-09-29 (model policy = RLM
    module; conductor = intent classifier only), orientation doc]
  must_preserve:
    - 'conductor never selects a model'
    - 'per-desk policy is durable across restarts'
    - 'parametric selection has a default fallback'
    - 'eval casts are parallel RLM casts, not a separate harness'
  excluded:
    - 'Jev transport/scoring (M4/M5)'
    - 'conductor intent-classification adaptivity (a later prompt-bar
      mission)'
    - 'World Wire'
  protected_surfaces:
    - 'model-policy.toml read path'
    - 'conductor routing code'
    - 'per-desk run profile assembly'

now:
  status: pending
  slice: 'not started — promote after M0a (research-surface validity)
    and M1 (scored evals) settle'
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
    id: rlm-module-enables-hillclimbing
    claim: 'If desks own model policy (persistent + parametric), then
      research hill-climbing becomes a desk-level operation, not a
      platform deploy — evals run as casts, results feed commitments'
    test: 'M3 runs a multi-model eval matrix without any redeploy'
    edge: 'missing_oracle — whether per-cast model override breaks
      anything (billing, rate limits, gateway auth)'
    delta_o: 'first parametric cast observed with correct model'
    scope_if_supported: 'all desks + sub-RLM casts'
    status: proposed
    evidence_refs: []
  decision:
    what: 'RLM module (persistent + parametric), conductor untouched'
    kind: architecture
    status: settled
    evidence_ref: 'owner corrections 2026-09-29'
    owner_ratification_ref: 'owner: "rlm module... texture changing its
      own model in a go cell... research can call subrlms and choose the
      model at call time. conductor shouldnt be responsible for deciding
      which model"'
  belief:
    believed_state: 'the module is additive atop existing per-desk
      profiles; conductor''s role was previously misread by the panel'
    main_uncertainty: 'how per-cast override interacts with
      gateway/provider auth and rate buckets'
    next_observation: 'first desk-changed model observed in a live turn'
  blocker_or_risk: 'depends on M0a + M1 settling; parametric cast
    override touches gateway auth'
  next_action: 'after M0a+M1: author the module verbs + persistence
    layer + parametric cast plumbing + eval-cast surface + QA fixtures'

receipts: []
---
