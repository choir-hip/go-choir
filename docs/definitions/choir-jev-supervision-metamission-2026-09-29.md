---
definition_version: 4
definition_id: choir-jev-supervision-metamission-2026-09-29
execution_mode: mission_orchestrator
readiness: reviewed

review:
  reviewer: agentic-consensus (5-agent convergent ordering panel + 7-agent
    divergent/convergent architecture panels)
  frozen_ref: docs/orientation-jev-supervision-metamission-2026-09-29.md@af234a02+
  verdict: accept
  evidence_ref: .agentic-consensus/agentic-consensus-20260929-205259/

start:
  captured_at: '2026-09-29T22:40:00Z'
  source:
    canonical_ref: main@ac54317c
    deploy_identity: staging https://choir.news build.commit=b85af274
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty (docs/desk-rlm-rectification-plan-2026-09-23.md — other
        WIP, do not touch)
      class: source
      owner: this session
      touch: read_write
      recovery: git
  predecessor:
    mission: choir-selfdev-gate-2026-09-27 (M11) — settled 2026-09-29,
      staging e886f576. Spine complete; this metamission resumes the stack.
    evidence: docs/evidence/m11-probe-run9-satisfied-2026-09-29.json
  observed_artifact:
    - claim: 'Owner QA session 2026-09-29 ~22:16-22:27Z: prompt → conductor
        (+14s) → texture v1 (+64s) → control_delivered to research (+16s) →
        research ran 7.7min (123k input tokens, serial non-streaming calls) →
        completed "remains open" → NO v2 ever rendered; reducer frozen at
        seq 5; desk_pending_mutations=7; VM one build behind wake repairs.'
      evidence: trajectory 5b5d8cb3-68d3-51e5-9808-74f921b671a7 on
        computer-03335285269bdba4f94377e56879f9e6

metamission:
  stations:
    - id: M0
      path: docs/definitions/choir-signal-m0-debug-stabilize-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: []
    - id: M-SUB
      path: docs/definitions/choir-signal-async-signal-plane-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: [M0]
    - id: M0a
      path: docs/definitions/choir-signal-research-rlm-cutover-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: [M-SUB]
    - id: M1
      path: docs/definitions/choir-signal-typed-commitments-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: []
    - id: M2
      path: docs/definitions/choir-signal-model-policy-rlm-module-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: [M0a, M1]
    - id: M3
      path: docs/definitions/choir-signal-research-hillclimb-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: [M2]
    - id: M4
      path: docs/definitions/choir-signal-jev-transport-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: []
    - id: M5
      path: docs/definitions/choir-signal-management-scorer-jev-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: [M1, M4]
    - id: world-wire
      path: '(next stack — not yet chartered)'
      readiness: intent
      status: pending
      depends_on: [M0, M-SUB, M0a, M1, M2, M3, M4, M5]

finish:
  deliver: 'Choir desks are full RLMs (one tool, desk_go_eval) communicating
    over an async signal plane; research streams evidence to texture mid-loop;
    commitments are typed/scoreable; model policy is an RLM module; management
    scores commitments (Jev default, RLM escalation on low confidence); the
    silent-stall class is closed.'
  artifact: 'all stations complete with deployed acceptance evidence per
    station; orientation doc updated to post-landing state'
  acceptance:
    - action: 'each station goal file reports now.status=complete with its
        own deployed acceptance receipt'
      proves: 'the metamission delivered every named station'
      evidence_class: deployed proof
    - action: 'owner QA repro: prompt → texture v1 → research evidence →
        texture v2+ visible, no silent stall, SSE survives transient drop'
      proves: 'the observed failure class is closed end-to-end'
      evidence_class: deployed proof on https://choir.news
  rollback: 'git revert per-station commits; each station carries its own
    rollback path; metamission-level rollback is station-order reversal'
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity,
      deployed_acceptance]

value:
  better_means: 'minimize the gap between "desks are RLMs that communicate
    asynchronously" (the doctrine) and "research runs a serial tool loop that
    never reports back" (the observed reality) while preserving tray
    atomicity, single-state-authority, and the commitment epistemic boundary'
  goodharting_would_be: 'stations closing on local tests or narrow
    definitions while the owner-visible QA failure pattern (silent stall,
    no evidence stream, serial-tool-loop latency) persists on staging'

homotopy:
  realism_axis: 'depth of the async signal substrate — from boundary-only
    delivery (parked wake + model-boundary notice) through advisory mid-cell
    piggyback, up to watcher/demux if measured need justifies it'

boundaries:
  mutation_class: red (kernel + canonical surfaces across stations; each
    station's own class is on its goal file)
  authority_sources:
    - 'owner direction 2026-09-29 (scorer=management; conductor=policy
      routing; model policy=RLM module; notice-injection; continuous
      authority under consensus)'
    - 'docs/choir-doctrine.md'
    - 'AGENTS.md'
    - 'docs/orientation-jev-supervision-metamission-2026-09-29.md
      (evidence + adjudicated design)'
  must_preserve:
    - 'tray atomicity — semantic acts commit all-or-nothing at cell end'
    - 'single state authority — channel log is the durable substrate;
      injected notices are views, never a second record'
    - 'epistemic boundary — commitment scores never reach ActingPack'
    - 'actors park rather than retain activations'
  excluded:
    - 'World Wire fanout substrate (deferred past this metamission)'
    - 'native demux / true mid-eval push (deferred; gated on mux proof)'
    - 'conductor model routing (rejected: conductor does policy routing only)'
  protected_surfaces:
    - 'yaegikernel session/broker protocol'
    - 'rlm_reduce tray commit path'
    - 'channel store + dispatchActor wake path'
    - 'texture canonical revision writes'

now:
  status: working
  slice: 'chartering: spine + station goal files authored; M0 is the live
    station to promote'
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
    id: signal-plane-closes-stall
    claim: 'If the async signal plane lands (emit + boundary notice + repl
      variable), then research evidence reaches texture mid-loop and the
      silent-stall class closes'
    test: 'owner QA repro produces v2+ on staging without manual repair'
    edge: 'frame_lock — the delivery contract is new vocabulary; the claim
      could survive falsely if "available" is measured at the wrong boundary'
    delta_o: 'deployed evidence: texture v2 committed citing a research
      emission seq, observed on the live doc'
    scope_if_supported: 'desk-to-desk async communication across all four
      desks plus future World Wire consumers'
    status: proposed
    evidence_refs: []
  decision:
    what: 'metamission stations ordered M0→M-SUB→M0a→M2→M3 with M1/M4
      parallel; signal plane = ActionEmit + boundary notice + repl
      variable (panel-adjudicated)'
    kind: architecture
    status: settled
    evidence_ref: '.agentic-consensus/agentic-consensus-20260929-212240/
      (convergent verdicts)'
    owner_ratification_ref: 'conversation 2026-09-29: owner corrections
      ratified (scorer=management; conductor=policy; model=RLM module;
      notice-injection)'
  belief:
    believed_state: 'the QA stall is the desk-consumption hole
      (ReduceCellIntents(failed) persists nothing) plus research running a
      serial tool loop; both are substrate problems, not tuning problems'
    main_uncertainty: 'whether the 7 pending mutations + 3 stale runs on the
      owner computer are the same wedge class or a second defect'
    next_observation: 'M0 redeploy: does the QA repro pass on today's build
      before any substrate change?'
  blocker_or_risk: 'station files are drafted, not reviewed — each needs a
    consensus pass before promotion to executable'
  next_action: 'promote M0 (choir-signal-m0-debug-stabilize) to executable:
    run its authoring consensus review, then execute the station'

receipts: []

weak_measures:
  - name: owner-computer-build-freshness
    kind: weak_signal
    baseline: 'b85af274 (one build behind wake repairs)'
    desired: 'current main'
    decision_use: 'whether M0s redeploy retest is even measuring the right
      substrate'
    cannot_prove: 'that the stall is fixed — only that stale code is ruled
      out'
---

# Jev/Supervision Metamission — spine

The metamission's evidence basis, adjudicated signal-plane design, and
station rationale live in
[`docs/orientation-jev-supervision-metamission-2026-09-29.md`](../orientation-jev-supervision-metamission-2026-09-29.md).
Each station carries its own `/goal` file under `docs/definitions/`. This
spine's `now.slice` advances as stations land; it settles `complete` when
every station is `complete` or `superseded`.

Execute with: `/goal docs/definitions/choir-jev-supervision-metamission-2026-09-29.md`
