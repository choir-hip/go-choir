---
definition_version: 4
definition_id: choir-signal-management-scorer-jev-2026-09-29
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
      (observed at capture; post-M1+M4 identity recorded in now at promotion)'
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
    mission: 'M1 (typed commitments — scoreable input) + M4 (Jev
      transport — the channel to the scorer)'
  observed_artifact:
    - 'owner 2026-09-29: the scorer for commitment records IS management.
      Management can score with Jev, other decision models, LLMs, or any
      combination; Jev is the default, escalating to an RLM sub-cast on
      low confidence. Management''s eng↔texture routing (passthrough if
      Jev agrees / translator otherwise) is coupled to precommitment
      scoring — engineering is downstream of the precommitments.
      Management still carries typed tools (report_to_texture,
      cancel_co_super_assignment, assignment family) — the last desk to
      reach the one-tool doctrine.'

finish:
  deliver: 'Management IS the commitment scorer — Jev by default,
    pluggable later — with `jev.decide` as an in-cell verb; low confidence
    ROUTES scoring to an RLM sub-cast (the distribution is a context
    variable) but NEVER blocks commitment recording, resolution, or work
    admission; eng↔texture routing rides the same scoring loop;
    management reaches the one-tool (desk_go_eval) doctrine.'
  artifact: 'jev.decide in management''s cell; commitment-score reconciler
    + choir.commitment_score OG kind; management typed tools → choir.*
    verbs; passthrough-or-translate routing through the scoring loop;
    low-confidence → RLM sub-cast with distribution as context var;
    deterministic score replay (same resolution → same score ID)'
  acceptance:
    - action: 'a typed commitment resolves; management''s cell calls
        jev.decide, gets a distribution, and a choir.commitment_score
        record lands on the tape with the full distribution'
      proves: 'management is the scorer end-to-end'
      evidence_class: deployed proof
    - action: 'duplicate-delivery/restart replay of the same resolution
        produces ONE deterministic score identity bound to that
        resolution — no double-scoring'
      proves: 'ratified deterministic replay rule holds'
      evidence_class: deployed proof
    - action: 'after a scored commitment + a Disagreement, fetch the
        ActingPack assembled for the acting cell: no score,
        distribution, or disagreement fields present'
      proves: 'epistemic boundary holds under scoring'
      evidence_class: deployed proof + static test
    - action: 'a low-confidence distribution triggers an RLM sub-cast
        with the distribution available as a context variable; the
        commitment still records and resolves — low confidence never
        gates recording, resolution, or admission'
      proves: 'escalation works AND confidence never gates'
      evidence_class: deployed proof
    - action: 'an engineering-bound work item routes through management''s
        scoring loop in BOTH modes: passthrough when the score agrees,
        translated when it disagrees'
      proves: 'routing is coupled to scoring, not a separate path'
      evidence_class: deployed proof
    - action: 'management''s tool surface shows exactly desk_go_eval'
      proves: 'four-desk one-tool doctrine complete'
      evidence_class: deployed proof + static analysis
  rollback: 'prove the selected fallback (LLM-judge or passthrough)
    BEFORE cutover; safely pause/drain scoring work; preserve score
    identities and records across restart/revert; restore compatible
    management tools and prompts together (typed surface + its prompt
    overlays as one unit); git revert station commits'
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity,
      deployed_acceptance]

value:
  better_means: 'minimize the gap between "management supervises via
    scored commitments" and "management exists but scores nothing" — the
    divergence that turns precommitment records from documentation into
    supervision'
  goodharting_would_be: 'jev.decide exists but management never escalates
    on low confidence, or scoring works but eng/texture routing bypasses
    it — supervision theater'

homotopy:
  realism_axis: 'scoring depth — Jev default only → +RLM escalation on
    low confidence → +pluggable scorer methods → +routing coupled to
    scores'

boundaries:
  mutation_class: red
  # red: management desk surface + commitment_score OG kind +
  # jev.decide verb + routing semantics.
  authority_sources: [owner direction 2026-09-29 (scorer=management;
    pluggable methods; routing coupled to scoring; management last desk
    to one-tool), 9/27 ratified Jev constraints, orientation doc]
  must_preserve:
    - 'ActingPack never carries commitment scores'
    - 'Disagreement split (from M1) — scorer disagreement never reaches
      ActingPack'
    - 'full Jev distributions recorded; not flattened'
    - 'low confidence routes scoring to an RLM sub-cast; it NEVER gates
      commitment recording, resolution, or work admission (owner
      2026-09-29)'
  excluded:
    - 'conductor model routing (rejected — conductor is policy routing)'
    - 'research/texture/engineering desk internals (routing changes
      confined to management''s side of the channel — the routing code
      path, e.g. internal/agentcore/management_controller.go)'
    - 'World Wire'
  protected_surfaces:
    - 'management desk registry + typed tools'
    - 'choir.commitment_score kind (new, canonical)'
    - 'jev.decide verb + sub-cast path'
    - 'eng↔texture routing path (management side)'

now:
  status: blocked_incomplete
  slice: 'reconcile M1+M4 receipts; enumerate management''s current
    typed capabilities; trace resolution→score→routing ownership before
    migration; then author jev.decide verb + scorer reconciler +
    management verb migration + routing-through-scoring'
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
    id: management-scoring-is-supervision
    claim: 'If management scores commitments (Jev default, escalating to
      RLM on low confidence) and eng/texture routing rides that loop,
      then precommitment records become the computer''s supervision
      surface — the product''s alignment mechanism, not just a log'
    test: 'a scored commitment influences a routing decision and produces
      a deterministic commitment_score record; ActingPack isolation
      verified on the same tape'
    edge: frame_lock
    # "supervision" must mean more than "a score exists" — the edge is
    # supervision-theater (scores exist, nothing consumes them); the
    # routing-coupling acceptance is the discriminator
    delta_o: 'a routing decision demonstrably driven by a score'
    scope_if_supported: 'management''s whole responsibility surface'
    status: proposed
    evidence_refs: []
  decision:
    what: 'management IS the scorer; Jev default + RLM escalation;
      routing coupled; management RLM-ified in same station'
    kind: authority
    status: settled
    evidence_ref: 'owner corrections 2026-09-29'
    owner_ratification_ref: 'owner: "the scorer for commitment records
      *is* management... the routing work that management does, as a
      passthrough (if jev agrees) or translator between eng and texture,
      is really coupled with precommitment scoring"'
  belief:
    believed_state: 'this is the heaviest station — scoring + routing +
      RLM-ification land together on management''s surface'
    main_uncertainty: 'whether the scorer-async-vs-cell-actuated split
      needs staging inside this station'
    next_observation: 'first commitment_score record + first low-confidence
      RLM escalation on staging'
  blocker_or_risk: 'depends on M1+M4; largest single station; the
    routing-scoring coupling is the load-bearing claim'
  next_action: 'after M1+M4: author jev.decide verb + scorer reconciler +
    management verb migration + routing-through-scoring'

receipts: []
---
