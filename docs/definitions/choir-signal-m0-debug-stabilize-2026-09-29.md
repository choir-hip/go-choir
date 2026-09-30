---
definition_version: 4
definition_id: choir-signal-m0-debug-stabilize-2026-09-29
execution_mode: mission_orchestrator
readiness: drafted
member_of: choir-jev-supervision-metamission-2026-09-29

start:
  captured_at: '2026-09-29T22:40:00Z'
  source:
    canonical_ref: main@ac54317c
    deploy_identity: staging https://choir.news build.commit=b85af274
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: source
      owner: this session
      touch: read_write
      recovery: git
  observed_artifact:
    - 'computer-03335285269bdba4f94377e56879f9e6 runs b85af274 (9/28 build),
      one behind wake-repair commits; desk_pending_mutations=7; runs
      b05f42a6/43ad448a/e98f8f3f pending since 9/28; QA repro stalled at
      reducer seq 5'

finish:
  deliver: 'Owner computer runs the current staging build; desk wake debt is
    reconciled to zero pending mutations; the QA repro (prompt → v1 →
    research evidence → v2) runs end-to-end without silent stall; SSE stream
    survives a transient drop by auto-resubscribing; per-leg timing records
    exist for every desk turn.'
  artifact: 'redeployed computer on current build + pending_mutations=0
    health reading + passing QA repro + SSE reconnect fix +
    per-desk-turn timing records'
  acceptance:
    - action: 'owner prompt-bar QA on the redeployed computer: "What'\''s new
        in ai today" produces v1, research evidence, and a committed v2
        visible in the Texture doc'
      proves: 'the silent-stall repro either closes on the current build
        (wake-gap hypothesis confirmed) or produces named residual
        evidence for M-SUB'
      evidence_class: deployed proof
    - action: 'kill/reload the SSE connection mid-session; the frontend
        resubscribes via ?after= cursor and no terminal-looking error
        surfaces'
      proves: 'lifecycle stream reconnect is deployed'
      evidence_class: deployed proof
    - action: 'curl /health on the computer'
      proves: 'desk_pending_mutations=0 and no stale pending runs'
      evidence_class: deployed proof
  rollback: 'redeploy prior build; SSE revert is a git revert'
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
  mutation_class: orange (deploy + small frontend fix + diagnostics;
    not canonical-writes)
  authority_sources: [owner direction 2026-09-29 (redeploy first),
    orientation doc section A]
  must_preserve: [desk wake dedup on channelID:seq, trajectory reducer
    semantics, SSE heartbeat contract]
  excluded: [signal-plane kernel work (M-SUB), research tool deletion (M0a),
    commitment typing (M1)]
  protected_surfaces: [frontend lifecycle SSE client, vmctl/deploy path]

now:
  status: working
  slice: 'redeploy owner computer → reconcile wake debt → SSE fix → QA
    repro → baseline timings'
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
    claim: 'If the computer runs today''s build, the QA stall either
      resolves (wake-gap hypothesis) or produces named residual evidence
      proving the desk-consumption hole is separate'
    test: 'QA repro on redeployed build'
    edge: 'independence — a pass could hide the residual; a fail could be
      a different bug'
    delta_o: 'trajectory events + desk diagnostics diff before/after'
    scope_if_supported: 'M-SUB scope narrows to signal-plane-only vs
      signal-plane+desk-consumption'
    status: testing
    evidence_refs: []
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
    onto current staging build; run the QA repro; reconcile pending
    mutations; land the SSE reconnect fix'

receipts: []
---
