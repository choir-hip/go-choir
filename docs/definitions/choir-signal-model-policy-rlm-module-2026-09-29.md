---
definition_version: 4
definition_id: choir-signal-model-policy-rlm-module-2026-09-29
execution_mode: mission_orchestrator
readiness: reviewed
member_of: choir-jev-supervision-metamission-2026-09-29

# review binds THIS file at its fixed commit (stamped at promotion).
review:
  reviewer: 'agentic-consensus authoring panel (codex, claude, devin,
    gpt6-sol, gemini38) — send_back round resolved'
  frozen_ref: 'main@63be04e3'
  verdict: accept
  evidence_ref: '.agentic-consensus/agentic-consensus-20260929-215420/'

start:
  captured_at: '2026-09-29T22:40:00Z'
  source:
    canonical_ref: main@ac54317c
    deploy_identity: 'staging https://choir.news build.commit=b85af274
      (observed at capture; post-M0a/M1 identity recorded in now at promotion)'
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
    - path: docs/desk-rlm-rectification-plan-2026-09-23.md
      status: dirty
      class: other_agent_wip
      owner: other agent
      touch: forbidden
      recovery: 'leave in place; never include in station commits'
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
        next turn runs the new model; a restart/rewarm of the computer
        keeps the selected model effective'
      proves: 'persistent per-desk model policy is durable'
      evidence_class: deployed proof
    - action: 'a research cell casts a sub-RLM with a model chosen at
        call time; the sub-RLM runs that model; the documented default
        fallback path works when no override is given'
      proves: 'parametric model selection with safe fallback'
      evidence_class: deployed proof
    - action: 'on the deployed M2 build, run N model/effort configs
        through the eval-cast surface as parallel RLM casts without
        another deployment; the score-matrix artifact records per-cast
        model identities + timing/token/cost'
      proves: 'the eval surface exists as the instrument M3 needs'
      evidence_class: deployed proof
    - action: 'conductor routes prompt-bar intent to the right appagent
        without touching model selection'
      proves: 'conductor = policy routing, not model routing'
      evidence_class: deployed proof
  rollback: 'capture previous effective per-desk policies before revert;
    owner-reachable per-desk policy reset to toml default without
    redeploy; explicitly disable/restore durable overrides; drain or
    safely settle in-flight parametric casts; verify default model
    selection after rollback; module is additive — revert restores
    static model-policy.toml'
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
  mutation_class: red
  # red: model/provider selection is a protected routing surface.
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
  status: superseded
  director_note_2026_10_05: >-
    ABSORBED into the app-dev metamission station SM (model policy rewrite +
    evals; see its 'v5 plan' -> SM). M0a's dependency is now SR there; the M1
    dependency is checked against S0m in SR.
  slice: 'reconcile M0a/M1 receipts and inspect the existing model-policy
    manager, overlays, and cast selection path; then author choir
    model-policy module verbs: persistent per-desk policy storage and
    parametric cast override'
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
    test: 'on the deployed M2 build: change a desk''s persistent model,
      observe its next turn and restart behavior, run a parametric
      sub-cast and inspect the model actually charged/used, run the eval
      matrix with no redeploy'
    edge: missing_oracle
    # per-cast override may break billing/rate limits/gateway auth —
    # the test must surface that
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

  # Recon 2026-09-30 (M2 slice first move — reconcile before authoring).
  # modelpolicy substrate on current main is NOT absent — it is unwired:
  # - modelpolicy.Manager (internal/modelpolicy/model_policy.go) already
  #   gives persistent per-desk policy: Policy{Defaults, Roles
  #   map[string]LLMSelection}, per-owner Load, file overlays under
  #   System/model-policy-overlays, Resolve(ctx, ownerID, role, overlayID),
  #   and EnrichMetadata which stamps llm_model / llm_reasoning_effort /
  #   llm_max_tokens / llm_policy_source / llm_policy_overlay_id onto run
  #   metadata (read by runtime.go:854,1182 and
  #   engineering_assignment_runtime.go:759).
  # - One in-cell verb exists: verify_model_capability
  #   (modelpolicy/tools_model_verify.go) -> resolveToolModelSelection.
  # - THE M2 GAPS (no host tool, no choir verb, no reconciler):
  #   (a) no persistent per-desk policy *write* verb — Load/Resolve only;
  #       the "texture changes its own model in a go cell" owner statement
  #       needs a choir.SetModelPolicy-class verb writing Roles[desk].
  #   (b) no parametric cast override — cast/spawn resolve model via
  #       EnrichMetadata + overlayID; the owner wants model chosen at cast
  #       call time (research calls sub-RLMs and picks the model).
  #   (c) no parallel RLM eval-cast surface + per-turn timing/token/cost
  #       records — RunRecord.metadata already carries llm_model and
  #       input/output tokens (m0_qa_probe reads them); what's missing is
  #       the matrix runner + cost projection, not the fields.
  #   Conductor untouched per settled decision. Authoring gate remains
  #   M0a+M1 settle; recon only.

receipts: []
---
