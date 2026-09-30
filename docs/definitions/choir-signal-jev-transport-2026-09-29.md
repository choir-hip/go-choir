---
definition_version: 4
definition_id: choir-signal-jev-transport-2026-09-29
execution_mode: mission_orchestrator
readiness: executable
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
      (observed at capture; parallel-safe, no runtime dep)'
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
    mission: none — pure transport; parallel-safe throughout
  observed_artifact:
    - 'ratified constraints (9/27 panels + owner §12.2): pinned
      typesafe/jev-1.13 (never jev-latest); transport = gateway
      POST /provider/v1/judgments → OpenRouter /api/alpha/decisions;
      per-VM bearer + own rate bucket; scores land as
      choir.commitment_score OG kind; full distributions recorded;
      confidence never gates; replay = deterministic score ID bound
      to the resolution'

finish:
  deliver: 'The Jev transport exists and is proven: a VM can call
    POST /provider/v1/judgments, get a Jev decision back from OpenRouter''s
    pinned typesafe/jev-1.13 endpoint, with per-VM bearer auth and its own
    rate bucket.'
  artifact: 'gateway /provider/v1/judgments route + OpenRouter
    /api/alpha/decisions client + pinned model + per-VM bearer +
    rate bucket + alpha-endpoint credential provisioning +
    transport diagnostic receipt (request/response identity +
    returned full distribution)'
  acceptance:
    - action: 'a live VM issues a typed choice question to the gateway;
        the gateway calls OpenRouter''s pinned jev-1.13 alpha endpoint;
        the full distribution returns and a transport diagnostic receipt
        records request/response identity — canonical commitment_score
        writes are deferred to M5'
      proves: 'transport works end-to-end with real auth'
      evidence_class: deployed proof
    - action: 'deployed refusal probes: a call with missing/invalid
        bearer is rejected; a cross-computer bearer misuse is rejected;
        two VMs hit their own rate buckets independently with recorded
        responses (already-authorized stable computers for the
        two-computer probe)'
      proves: 'per-VM isolation is enforced, not just configured'
      evidence_class: deployed proof
  rollback: 'disable the route; settle in-flight requests; retain
    request/distribution evidence; remove only station-owned
    credential/config additions (the alpha-endpoint credential +
    per-VM bearer + bucket config) without revoking shared credentials;
    git revert the station commits'
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity,
      deployed_acceptance]

value:
  better_means: 'minimize the distance between "management can call a
    decision model" and "no transport exists" — a narrow, testable
    connectivity claim'
  goodharting_would_be: 'a transport that compiles and deploys but was
    never called from a real VM with real auth'

homotopy:
  realism_axis: 'connectivity depth — a test client on the workstation →
    a VM on staging with real bearer → under load/rate-limiting'

boundaries:
  mutation_class: red
  # red: gateway provider route surface + external provider client
  # credentials — protected routing surface per AGENTS.md.
  authority_sources: [owner §12.2 (OpenRouter alpha endpoint),
    9/27 consensus panels (pinned model, per-VM bearer, full
    distributions, no confidence gate), orientation doc]
  must_preserve:
    - 'pinned typesafe/jev-1.13, never jev-latest'
    - 'per-VM bearer + own rate bucket'
    - 'full distributions recorded, not flattened'
    - 'confidence never gates'
  excluded:
    - 'commitment scoring (M5 scope) — canonical commitment_score writes
      are NOT this station''s deliverable'
    - 'conductor routing (not this mission — conductor is policy routing)'
    - 'jev.decide cell verb (M5 scope)'
  protected_surfaces:
    - 'gateway provider route surface'
    - 'deploy-provider-creds credential provisioning'

now:
  status: working
  slice: 'verify OpenRouter alpha-endpoint access for typesafe/jev-1.13
    first — if absent, record as a named residual and stop; then author
    gateway POST /provider/v1/judgments route handler + OpenRouter client'
  source_ref: main@ac54317c
  deploy_identity: 'staging build.commit=b85af274'
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: jev-transport-unblocks-scorer
    claim: 'If the gateway→OpenRouter pinned transport exists, then M5''s
      management-scorer work is unblocked — the missing piece is
      transport, not scoring logic'
    test: 'a VM→gateway→OpenRouter round-trip with a returned
      distribution and a diagnostic receipt'
    edge: independence
    # transport existing does not prove scoring is right; it proves
    # connectivity
    delta_o: 'M5 uses it'
    scope_if_supported: 'all judgment transport for the computer'
    status: proposed
    evidence_refs: []
  decision:
    what: 'gateway POST /provider/v1/judgments → OpenRouter
      typesafe/jev-1.13, per owner §12.2'
    kind: architecture
    status: settled
    evidence_ref: '9/27 consensus panels + orientation doc'
    owner_ratification_ref: 'owner 2026-09-27 §12.2 + 2026-09-29'
  belief:
    believed_state: 'transport is narrow and parallel-safe; alpha
      endpoint provisioning is the named external dependency'
    main_uncertainty: 'OpenRouter alpha endpoint access + per-VM bearer
      provisioning'
    next_observation: 'first live VM→judgment round-trip'
  blocker_or_risk: 'alpha-endpoint credential provisioning is external'
  next_action: 'verify OpenRouter alpha-endpoint access first (named
    residual if absent); then author the gateway route + OpenRouter
    client + cred provisioning; live round-trip on a staging VM'

receipts: []
---
