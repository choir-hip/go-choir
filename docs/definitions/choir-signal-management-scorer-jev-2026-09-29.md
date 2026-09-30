---
definition_version: 4
definition_id: choir-signal-management-scorer-jev-2026-09-29
execution_mode: mission_orchestrator
readiness: drafted
member_of: choir-jev-supervision-metamission-2026-09-29

start:
  captured_at: '2026-09-29T22:40:00Z'
  source:
    canonical_ref: main@ac54317c
    deploy_identity: staging https://choir.news (post-M1+M4)
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: source
      owner: this session
      touch: read_write
      recovery: git
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
    escalates to an RLM sub-cast carrying the distribution as a context
    variable; eng↔texture routing rides the same scoring loop;
    management reaches the one-tool (desk_go_eval) doctrine.'
  artifact: 'jev.decide in management''s cell; commitment-score reconciler
    + choir.commitment_score OG kind; management typed tools → choir.*
    verbs; passthrough-or-translate routing through the scoring loop;
    low-confidence → RLM sub-cast with distribution as context var'
  acceptance:
    - action: 'a typed commitment resolves; management''s cell calls
        jev.decide, gets a distribution, and a choir.commitment_score
        record lands on the tape'
      proves: 'management is the scorer end-to-end'
      evidence_class: deployed proof
    - action: 'a low-confidence distribution triggers an RLM sub-cast
        with the distribution available as a context variable'
      proves: 'escalation path works'
      evidence_class: deployed proof
    - action: 'management''s tool surface shows exactly desk_go_eval'
      proves: 'four-desk one-tool doctrine complete'
      evidence_class: deployed proof + static analysis
    - action: 'an engineering-bound work item routes through management''s
        scoring loop (passthrough when Jev agrees / translated when not)'
      proves: 'routing is coupled to scoring, not a separate path'
      evidence_class: deployed proof
  rollback: 'Jev default can fall back to LLM-judge or passthrough;
    management RLM-ification revert restores typed tools; git revert
    station commits'
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
  mutation_class: red (management desk surface + commitment-score OG
    kind + jev.decide verb + routing semantics)
  authority_sources: [owner direction 2026-09-29 (scorer=management;
    pluggable methods; routing coupled to scoring; management last desk
    to one-tool), 9/27 ratified Jev constraints, orientation doc]
  must_preserve:
    - 'ActingPack never carries commitment scores'
    - 'Disagreement split (from M1) — scorer disagreement never reaches
      ActingPack'
    - 'full Jev distributions recorded; confidence never gates'
    - 'one-tool doctrine on management (last desk)'
  excluded:
    - 'conductor model routing (rejected — conductor is policy routing)'
    - 'research/texture/engineering desk changes'
    - 'World Wire'
  protected_surfaces:
    - 'management desk registry + typed tools'
    - 'choir.commitment_score kind (new, canonical)'
    - 'jev.decide verb + sub-cast path'
    - 'eng↔texture routing'

now:
  status: pending
  slice: 'not started — promote after M1+M4 settle'
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
      a commitment_score record'
    edge: 'frame_lock — "supervision" must mean more than "a score exists";
      the edge is supervision-theater (scores exist, nothing consumes
      them)'
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
