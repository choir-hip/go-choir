---
definition_version: 4
definition_id: choir-signal-research-hillclimb-2026-09-29
execution_mode: mission_orchestrator
readiness: drafted
member_of: choir-jev-supervision-metamission-2026-09-29

start:
  captured_at: '2026-09-29T22:40:00Z'
  source:
    canonical_ref: main@ac54317c
    deploy_identity: staging https://choir.news (post-M2)
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: source
      owner: this session
      touch: read_write
      recovery: git
  predecessor:
    mission: M2 (model-policy module + eval surface + QA fixtures)
  observed_artifact:
    - 'research ran a serial tool-call loop on 9/29: 123,490 input
      tokens, ~470s, "remains open", no v2. Owner: research tuning needs
      multi-model evals, not a single luna-xhigh config. Success = a
      research pattern that is fast AND deep — iterative research that
      can run days/weeks and eventually feed more than one texture
      (the World-Wire seed).'

finish:
  deliver: 'Research tuning is evidence-driven: a multi-model/effort
    matrix over the QA prompt family produces measured prompt, repl-state,
    and search-API-ergonomics decisions; the iterative-research pattern
    (search→emit→deepen, texture revising per update) is the tuned
    operating shape.'
  artifact: 'eval matrix over ≥3 model/effort configs on the QA fixture;
    tuned research prompt + repl-state + search-API ergonomics;
    measured iterative-research pattern with evidence streaming'
  acceptance:
    - action: 'eval matrix runs the QA fixture across model/effort
        configs; the score-matrix artifact records quality + latency +
        token cost per config'
      proves: 'research tuning is measured, not guessed'
      evidence_class: deployed proof
    - action: 'a research activation sustains the search→emit→deepen
        loop across multiple evidence deliveries to texture, with
        texture revising per update'
      proves: 'the iterative-research pattern works under tuning'
      evidence_class: deployed proof
  rollback: 'tuning is config + prompt; revert restores prior prompt/
    config; the eval surface stays'
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity,
      deployed_acceptance]

value:
  better_means: 'minimize research latency + maximize evidence depth
    measured on the QA family — the divergence between the observed
    serial-loop turn and the iterative pattern the owner wants'
  goodharting_would_be: 'optimizing for the "10 versions in 5 min" bar
    literally — a version-spam loop with cosmetic churn instead of
    evidence-attributable revisions; or a matrix that tunes the model
    but never changes the loop''s serial shape'

homotopy:
  realism_axis: 'tuning coverage — one model/effort config → multi-model
    matrix → matrix + continuous-research-day-scale evals'

boundaries:
  mutation_class: orange (prompt + config changes; not kernel)
  authority_sources: [owner direction (research tuning needs multi-model
    evals), orientation doc M3 row]
  must_preserve:
    - 'evals measure the post-M0a surface (desk_go_eval only)'
    - 'iterative-research shape — not a one-shot benchmark'
    - 'quality is measured alongside speed (groundedness, attribution)'
  excluded:
    - 'substrate changes (M-SUB/M0a already landed)'
    - 'World Wire fanout'
  protected_surfaces: []

now:
  status: pending
  slice: 'not started — promote after M2 settles'
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
    id: iterative-beats-serial
    claim: 'If research runs the search→emit→deepen Go-cell loop instead
      of the serial tool-call loop, then quality-per-time rises and the
      QA repro''s 7.7-minute dead leg collapses'
    test: 'eval matrix on the QA fixture + a sustained iterative-research
      run'
    edge: 'resource — a tuned prompt may still be slow if the fixture is
      unrepresentative'
    delta_o: 'expand the QA fixture; measure on more prompt families'
    scope_if_supported: 'research desk operating shape; seeds World Wire
      continuous research'
    status: proposed
    evidence_refs: []
  decision:
    what: 'eval-driven tuning on the QA fixture family; 10-in-5 is a
      directional smell-test, not a hard guarantee'
    kind: operational
    status: settled
    evidence_ref: 'owner corrections + panel consensus'
    owner_ratification_ref: 'owner: "10 versions in 5 minutes isnt a hard
      guarantee... research pattern which is both fast and deep...
      iterative research"'
  belief:
    believed_state: 'the serial tool loop was the dominant cost; the
      iterative pattern plus model choice should collapse it'
    main_uncertainty: 'what the quality bar is — groundedness rubric for
      the eval matrix is unstated'
    next_observation: 'first eval-matrix score artifact'
  blocker_or_risk: 'depends on M2; quality rubric for evals needs
    definition at station time'
  next_action: 'after M2: author the QA fixture, run the eval matrix,
    tune prompt+repl+search ergonomics, prove the iterative pattern'

receipts: []
---
