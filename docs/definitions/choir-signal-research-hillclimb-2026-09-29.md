---
definition_version: 4
definition_id: choir-signal-research-hillclimb-2026-09-29
execution_mode: mission_orchestrator
readiness: reviewed
member_of: choir-jev-supervision-metamission-2026-09-29

# review binds THIS file at its fixed commit (stamped at promotion).
review:
  reviewer: 'agentic-consensus authoring panel (codex, claude, devin,
    gpt6-sol, gemini38) — send_back round resolved'
  frozen_ref: 'pending-stamp'
  verdict: accept
  evidence_ref: '.agentic-consensus/agentic-consensus-20260929-215420/'

start:
  captured_at: '2026-09-29T22:40:00Z'
  source:
    canonical_ref: main@ac54317c
    deploy_identity: 'staging https://choir.news build.commit=b85af274
      (observed at capture; post-M2 identity recorded in now at promotion)'
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
    mission: M2 (model-policy module + eval surface + QA fixtures)
  observed_artifact:
    - 'research ran a serial tool-call loop on 9/29: 123,490 input
      tokens, ~470s, "remains open", no v2. Owner: research tuning needs
      multi-model evals, not a single luna-xhigh config. Success = a
      research pattern that is fast AND deep — iterative research that
      can run days/weeks and eventually feed more than one texture
      (the World-Wire seed).'

finish:
  deliver: 'Research tuning is evidence-driven: a frozen QA fixture +
    groundedness/attribution rubric committed BEFORE any matrix run;
    a multi-model/effort matrix over the fixture produces measured
    prompt, repl-state, and search-API-ergonomics decisions; the
    iterative-research pattern (search→emit→deepen, texture revising
    per update) is the tuned operating shape.'
  artifact: 'frozen QA fixture + named quality rubric; eval matrix over
    ≥3 model/effort configs with per-config quality + latency + cost;
    tuned research prompt + repl-state + search-API ergonomics;
    measured iterative-research pattern with evidence streaming'
  acceptance:
    - action: 'the score-matrix artifact names the frozen rubric and
        records quality + latency + token cost per config; an
        independently assessed before/after comparison under that rule
        states what result would refute improvement'
      proves: 'research tuning is measured against a frozen rubric, not
        post-hoc'
      evidence_class: deployed proof
    - action: 'a research activation sustains the search→emit→deepen
        loop across multiple evidence deliveries to texture, with
        texture revising per update; each texture revision cites a
        distinct research emission seq (no cosmetic-churn versions)'
      proves: 'the iterative-research pattern works under tuning without
        version-spam'
      evidence_class: deployed proof
  rollback: 'tuning is config + prompt; revert restores prior prompt/
    config; restore captured effective per-desk runtime policy via the
    supported module path (M2) and observe a turn using it; the eval
    surface stays'
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
  mutation_class: orange
  # orange: prompt + config changes; no kernel or canonical-authority
  # mutation. choir.* verb signature changes would reclassify red.
  authority_sources: [owner direction (research tuning needs multi-model
    evals), orientation doc M3 row]
  must_preserve:
    - 'evals measure the post-M0a surface (desk_go_eval only)'
    - 'iterative-research shape — not a one-shot benchmark'
    - 'quality is measured alongside speed (groundedness, attribution)
      under a frozen rubric'
  excluded:
    - 'substrate changes (M-SUB/M0a already landed)'
    - 'World Wire fanout'
    - 'choir.* verb signature changes (reclassify red if needed)'
  protected_surfaces: []

now:
  status: blocked_incomplete
  slice: 'reconcile M2 receipts; freeze the QA fixture, the
    groundedness/attribution rubric, the baseline, and the comparison
    rule BEFORE running any tuning candidate; then run the eval matrix'
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
    id: tuned-config-beats-default
    claim: 'a tuned model/effort/prompt config beats the M0a default on
      quality-per-time on the frozen fixture — the claim is about tuning,
      not the iterative-vs-serial shape (M0a''s claim)'
    test: 'frozen-rubric before/after on the QA fixture + a sustained
      iterative-research run; matrix existence alone does NOT support
      the claim'
    edge: missing_oracle
    # the fixture may be unrepresentative — a tuned prompt can still be
    # slow on real prompts
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
    main_uncertainty: 'the groundedness rubric — must be frozen in the
      first slice, not discovered after the matrix'
    next_observation: 'first eval-matrix score artifact under the frozen
      rubric'
  blocker_or_risk: 'depends on M2; quality rubric freeze is the first
    move, not a deferred detail'
  next_action: 'after M2: commit the QA fixture + frozen rubric, run the
    eval matrix, tune prompt+repl+search ergonomics, prove the
    iterative pattern'

receipts: []
---
