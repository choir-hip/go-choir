---
definition_version: 4
definition_id: choir-signal-m0-debug-stabilize-2026-09-29
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
      (node B health, 2026-09-29)'
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
  observed_artifact:
    - 'computer-03335285269bdba4f94377e56879f9e6 runs b85af274 (9/28 build),
      one behind wake-repair commits; desk_pending_mutations=7; runs
      b05f42a6/43ad448a/e98f8f3f pending since 9/28; QA repro stalled at
      reducer seq 5'

finish:
  deliver: 'Owner computer runs the current staging build; desk wake debt is
    reconciled to zero pending mutations; the QA repro (prompt → v1 →
    research evidence → v2) either completes end-to-end with a committed
    visible v2, or its failure is attributed by a committed residual-evidence
    doc (trajectory + desk-diagnostics diff) handed to M-SUB — a Problem
    Documentation First commit; SSE stream survives a transient drop by
    auto-resubscribing; per-leg timing records exist for every desk turn.'
  artifact: 'redeployed computer on current build + pending_mutations=0
    health reading + passing QA repro OR committed residual-evidence doc +
    SSE reconnect fix + per-desk-turn timing records'
  acceptance:
    - action: 'owner prompt-bar QA on the redeployed computer: "What''s new
        in ai today" produces v1, research evidence, and a committed v2
        visible in the Texture doc'
      proves: 'the silent-stall repro closes on the current build
        (wake-gap hypothesis confirmed)'
      evidence_class: deployed proof
    - action: 'if the QA repro still fails: commit a residual-evidence doc
        (trajectory + desk-diagnostics diff, named defect class) before
        closing M0 — the failure is M-SUB''s entry evidence, not silent'
      proves: 'a persistent stall is documented and attributed, never
        silently absorbed'
      evidence_class: deployed proof + problem documentation
    - action: 'QA repro trajectory carries a timing record (wall ms,
        model-call ms, input/output tokens) for every desk turn; baseline
        saved to docs/evidence/m0-qa-baseline-timings-2026-09-29.json'
      proves: 'per-leg baseline exists for M0a/M3 comparison'
      evidence_class: deployed proof
    - action: 'fetch /health + desk diagnostics: desk_pending_mutations=0;
        runs b05f42a6/43ad448a/e98f8f3f each reach terminal or reconciled
        state (an omitted zero-valued field reads as 0)'
      proves: 'wake debt reconciled, no stale pending runs'
      evidence_class: deployed proof
    - action: 'kill/reload the SSE connection mid-session; the frontend
        resubscribes via ?after= cursor and no terminal-looking error
        surfaces'
      proves: 'lifecycle stream reconnect is deployed'
      evidence_class: deployed proof
  rollback: 'record pre-change source/effective identity before redeploy;
    redeploy prior build b85af274; git revert frontend SSE commit;
    pending-mutation reconcile is forward-only — snapshot /health + desk
    diagnostics first; never force-delete debt to satisfy the counter'
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity,
      deployed_acceptance]

value:
  better_means: 'minimize owner-visible stall class while preserving desk
    wake/cursor semantics — every hour of stall we can attribute to stale
    build vs real defect is precision M-SUB needs'
  goodharting_would_be: 'redeploying and declaring victory without the
    QA repro, or clearing pending mutations by force-delete without the
    counter returning to 0 organically'

homotopy:
  realism_axis: 'diagnostic depth — from "redeploy and retest" up to
    full per-leg instrumentation with timing records on the tape'

boundaries:
  mutation_class: red
  # red: operates the vmctl/deploy path (protected surface) + frontend
  # SSE client mutation. Full ceremony applies.
  authority_sources: [owner direction 2026-09-29 (redeploy first),
    orientation doc section A]
  must_preserve: [desk wake dedup on channelID:seq, trajectory reducer
    semantics, SSE heartbeat contract,
    'vmctl/deploy path operated, not modified']
  excluded: [signal-plane kernel work (M-SUB), research tool deletion (M0a),
    commitment typing (M1)]
  protected_surfaces: [frontend lifecycle SSE client, vmctl/deploy path]

now:
  status: complete
  slice: 'settled — redeployed 4c279162; wake debt reconciled; QA repro
    residual (texture runtime_restarted no-rewake) root-caused and FIXED
    d1d875a0: reconcileAgentWakeLocked suppressed initialWorkWake on any
    texture run regardless of state, so the passivated run stood down its
    own reactivation. Gate now requires State.Active(). Regression:
    TestTextureOwnerStartReactivatesPassivatedRunOnOpenWork (fails pre-fix).'
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
    id: wake-gap-or-residual
    claim: 'the current wake-repair build closes the observed stall — the
      QA repro either passes end-to-end or fails with a named defect class'
    test: 'QA repro on the redeployed build; failure = committed
      residual-evidence doc, not silent absorption'
    edge: independence
    # a pass could hide the residual; a fail could be a different bug —
    # the test discriminates by named-defect attribution
    delta_o: 'trajectory events + desk diagnostics diff before/after'
    scope_if_supported: 'M-SUB scope narrows to signal-plane-only vs
      signal-plane+desk-consumption'
    status: resolved
    evidence_refs:
    - 'docs/evidence/m0-residual-texture-runtime-restart-2026-09-30.md (repair section; fix d1d875a0)'
  decision:
    what: 'redeploy-first per owner; SSE fix bundled (small, same mission)'
    kind: operational
    status: settled
    evidence_ref: 'owner conversation 2026-09-29'
    owner_ratification_ref: 'owner: "redeploy computer first"'
  belief:
    believed_state: 'the stall is probably the known wake-gap (VM is a
      build behind the reconcile-kick repairs); SSE disconnect is a
      frontend onerror handling bug'
    main_uncertainty: 'whether 7 pending mutations reconcile on the new
      build or reveal a second wedge class'
    next_observation: 'post-redeploy /health pending_mutations and the
      QA repro trajectory'
  blocker_or_risk: none
  next_action: 'restart/redeploy computer-03335285269bdba4f94377e56879f9e6
    onto current staging build; snapshot /health + desk diagnostics first;
    run the QA repro; reconcile pending mutations; land the SSE reconnect fix'

receipts:
  - id: m0-redeploy-and-residual
    kind: outcome
    status: settled
    summary: 'Owner computer refreshed to 4c279162 (epoch 958); desk
      pending_mutations 0; 3 stale pending runs reconciled to passivated;
      SSE reconnect fix deployed (stream?after=cursor in TextureEditor
      chunk); QA repro timed out but failure attributed to named residual
      — texture desk passivated runtime_restarted with no re-wake.
      Baseline legs recorded for the executed legs.'
    evidence_ref: 'docs/evidence/m0-residual-texture-runtime-restart-2026-09-30.md
      + docs/evidence/m0-qa-baseline-timings-2026-09-30.json'
  - id: m0-runtime-restart-rewake-fix
    kind: outcome
    status: settled
    summary: 'runtime_restarted texture run suppressed its own re-wake:
      reconcileAgentWakeLocked zeroed initialWorkWake on any texture run
      regardless of state, so the passivated activation stood down its own
      reactivation. Fixed d1d875a0 (State.Active() gate) + two regression
      tests. Residual narrows to: consumed-head + no-open-work still relies
      on armedUpdates/re-minted outbox (M-SUB scope).'
---
