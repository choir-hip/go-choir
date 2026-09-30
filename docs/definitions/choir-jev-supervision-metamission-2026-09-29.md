---
definition_version: 4
definition_id: choir-jev-supervision-metamission-2026-09-29
execution_mode: mission_orchestrator
readiness: executable

# review binds THIS file at its fixed commit (stamped at promotion).
# executable: now.status=working + named authority (owner-ratified spine
# 2026-09-29 + authoring-panel accept). Stations promote per their own
# readiness as dependencies clear.
review:
  reviewer: 'agentic-consensus authoring panel (codex, claude, devin,
    gpt6-sol, gemini38) — send_back round resolved'
  frozen_ref: 'main@63be04e3'
  verdict: accept
  evidence_ref: '.agentic-consensus/agentic-consensus-20260929-215420/'

metamission:
  stations:
    - id: m0-debug-stabilize
      path: docs/definitions/choir-signal-m0-debug-stabilize-2026-09-29.md
      readiness: drafted
      status: working
      depends_on: []
    - id: m-sub-signal-plane
      path: docs/definitions/choir-signal-async-signal-plane-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: [m0-debug-stabilize]
    - id: m0a-research-rlm-cutover
      path: docs/definitions/choir-signal-research-rlm-cutover-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: [m-sub-signal-plane]
    - id: m1-typed-commitments
      path: docs/definitions/choir-signal-typed-commitments-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: []
    - id: m2-model-policy-rlm-module
      path: docs/definitions/choir-signal-model-policy-rlm-module-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: [m0a-research-rlm-cutover, m1-typed-commitments]
    - id: m3-research-hillclimb
      path: docs/definitions/choir-signal-research-hillclimb-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: [m2-model-policy-rlm-module]
    - id: m4-jev-transport
      path: docs/definitions/choir-signal-jev-transport-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: []
    - id: m5-management-scorer
      path: docs/definitions/choir-signal-management-scorer-jev-2026-09-29.md
      readiness: drafted
      status: pending
      depends_on: [m1-typed-commitments, m4-jev-transport]
    # world-wire is the NEXT stack — not a station of this metamission.
    # Keeping it out of metamission.stations is load-bearing: a spine
    # settles complete only when every station is complete or superseded.

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
  predecessor:
    mission: choir-rectification-spine-2026-09-25 — settled 2026-09-29 on
      M11's landed receipt (probe run 9 satisfied, staging e886f576)
    disposition: satisfied — the substrate the prior spine built is the
      foundation this metamission extends (desks on the in-cell carrier,
      commitment ledger, typed acts, one-tool doctrine on 3/4 desks)
  observed_artifact:
    - 'QA repro on owner computer stalled: prompt -> v1 -> research leg
      123,490 input tokens ~470s -> "remains open", no v2 committed'
    - 'owner computer runs b85af274, one build behind wake-repair commits;
      desk_pending_mutations=7; runs b05f42a6/43ad448a/e98f8f3f pending
      since 9/28'
    - 'ReduceCellIntents(failed) persists nothing — a cell that fails
      silently leaves the reducer waiting (the stall terminator gap)'
    - 'rlm_inbox_cursor is run-memory scoped to runID — respawn replays
      the channel from zero'
    - 'injectUserTurns seam (toolloop.go:672) exists between model calls;
      boundary-drain injection lands there'

finish:
  deliver: 'The four desks run on the full-RLM doctrine with a durable
    async signal plane between them; commitments are typed and scoreable;
    management IS the commitment scorer (Jev default, RLM escalation);
    research iterates fast-and-deep inside Go cells instead of a serial
    tool-call loop; the owner-visible stall class is closed end-to-end.'
  artifact: 'All eight station files settled complete or superseded with
    deployed acceptance receipts on the tape; spine now.slice advanced
    through each station.'
  acceptance:
    - action: 'read each station''s goal file now.status and fetch + verify
        its deployed acceptance receipt and environment identity (not
        merely a reported status)'
      proves: 'every station delivered its promised artifact with verified
        staging evidence'
      evidence_class: deployed proof
    - action: 'owner prompt-bar QA repro runs on the final build:
        research evidence reaches texture mid-run, texture revises the
        doc, management scores the resulting commitments'
      proves: 'the supervision loop closes end-to-end on the product
        surface'
      evidence_class: deployed proof
  rollback: 'per-station rollback paths apply station-by-station; the
    spine itself is reverted by returning docs/ACTIVE.md +
    mission-graph.yaml to the prior entrypoint and marking the spine
    superseded'
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity,
      deployed_acceptance]

value:
  better_means: 'minimize the distance between the owner''s question and
    a supervised, evidence-backed answer — while preserving the epistemic
    boundary that keeps scores out of ActingPack and the tray atomicity
    that keeps writes all-or-nothing'
  goodharting_would_be: 'stations land structurally but the QA repro
    still stalls, or scores exist but nothing routes on them —
    supervision theater'

homotopy:
  realism_axis: 'delivery immediacy + scoring depth — from sequential
    blocking wake to durable async signal to scored commitments driving
    routing; each station is a continuous deepening, never a different
    object'

boundaries:
  mutation_class: red
  # red: this spine''s stations touch canonical event authority, kernel
  # broker surface, desk tool surfaces, model/provider routing, and
  # credential provisioning. Full ceremony applies per station.
  authority_sources:
    - 'owner corrections + direction 2026-09-29 (scorer=management,
      conductor=policy routing, model=RLM module, notice-injection,
      one-tool doctrine)'
    - 'orientation doc adjudicated design (two consensus panels,
      .agentic-consensus/agentic-consensus-20260928-* and 20260929-*)'
    - 'docs/choir-doctrine.md + AGENTS.md mutation classes'
  must_preserve:
    - 'tray atomicity — semantic acts commit all-or-nothing at cell end'
    - 'epistemic boundary — scores/distributions/disagreement never enter
      ActingPack'
    - 'channelID:seq wake dedup'
    - 'actors park; activations are not retained for delivery'
    - 'single state authority — commitments on the tape, not parallel
      stores'
  excluded:
    - 'World Wire fanout substrate (successor stack)'
    - 'conductor model routing (conductor is policy routing only)'
    - 'Box score / production wire (out of scope per owner 9/22)'
  protected_surfaces:
    - 'internal/yaegikernel broker + session loop'
    - 'internal/agentcore rlm_reduce + channel_store + run-memory'
    - 'internal/toolloop injectUserTurns seam'
    - 'gateway provider route surface + credential provisioning'
    - 'desk cell registries + prompt overlays'

now:
  status: working
  slice: 'M0 + M4 settled complete. M-SUB residuals landed (emit boundary-
    drain, cursor rekey, advisory piggyback); M0a phase-1 verbs landed +
    cell-proven (b83ea5db, a09fd215, 007b64df, 9e3d6948 — research choir.*
    egress verbs + handle auth + research messaging-authority repair). Live
    front: deployed acceptance on all three — M-SUB Emit proofs, M1
    typed-commitment tape, M0a controlled-comparison verify before the
    deletion commit.'
  source_ref: main@9e3d6948
  deploy_identity: 'staging deployed_commit=2404e7d2 (vmctl flapping;
    007b64df/9e3d6948 deploys propagating behind a wedged Node B job)'
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: signal-plane-closes-supervision
    claim: 'If the signal plane + typed commitments + management-scorer
      land in this order, the desk-to-desk supervision path the QA stall
      exposed closes end-to-end on staging'
    test: 'the finish acceptance: QA repro delivers evidence mid-run,
      texture revises, management scores'
    edge: frame_lock
    # frame_lock risk: closing THIS QA path could still leave the general
    # supervision claim unproven; scope_if_supported stays bounded to the
    # exercised path until further proof
    delta_o: 'post-M5 QA run on staging + commitment_score records'
    scope_if_supported: 'the desk-to-desk path exercised on staging —
      texture<-research emission + management scoring; wider claims need
      further stations'
    status: testing
    evidence_refs: []
  decision:
    what: 'ordered stations M0->M-SUB->M0a->M2->M3; M1+M4 parallel-safe;
      M5 after M1+M4; spine settles when all eight settle'
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
    main_uncertainty: 'whether the 7 pending mutations + 3 stale runs on
      the owner computer are the same wedge class or a second defect'
    next_observation: 'first M1 desk-cell commitment records + M4 live
      round-trip distribution on the staging tape; first M-SUB cell_fate
      record on a timed-out/killed cell'
  blocker_or_risk: 'staging env root-caused on Node B 2026-09-30 to a HOST
    CAPACITY fault, not code: platform-dolt (go-choir-platform-dolt) balloons
    to anon-rss 28GB (total-vm 48GB) on a 31GB box -> kernel OOM-killer kills
    it AND the retained-computer firecracker
    (vmmanager: "exited with error: signal: killed", 12:31:26). Each kill
    reboots the guest (~20-40min cycle) -> boot passivation mass-parks
    in-flight desk cells -> desk legs (incl. our QA research legs) die
    mid-leg -> v2 cannot form -> all deployed_proof items are unreachable.
    The stale-route/vmctl-unavailable flapping is a SYMPTOM of these re-mints;
    POST /internal/vmctl/refresh on the vmctl unix socket recovers the route
    non-destructively between reboots; build.commit=9e3d6948 is deployed and
    the guest itself is READY/working when up. Also REPAIRED
    this session: a second independent defect — texture audit events were
    hardcoded EventKind=lifecycle_observed (texture_audit.go), which lands
    them in the computer_lifecycle_receipts CAS join that a non-lifecycle
    action can never satisfy -> every texture audit append aborted
    "lifecycle receipt join unavailable"; the tape silently dropped the
    whole texture audit trail since ~Aug 20. Fix 0a03783b maps
    entry.Action->causal kind. THIRD fault surfaced 13:05 (the prompt-bar
    submit 500): "computer event projection repair required" — an OOM-kill
    mid-append committed the platform CAS but killed the embedded finalize,
    so platformHead != embeddedHead and the appender refuses all appends.
    UPDATE 13:20Z: a `choir computer refresh` rebooted the guest and the
    autoputer ran the replay phase — "projection recovery resume for
    computer-03335285 (local=174099 H=174100 tail=1)" then "computer event
    authority reconstructed". THE PROJECTION WEDGE IS REPAIRED. Remaining
    fault is the OOM itself + the stale vmctl route it re-mints (502
    "resolve user autoputer"); audit fix 0a03783b is merged but its deploy
    was concurrency-cancelled twice — build still 9e3d6948.'
  next_action: 'owner/ops: (a) cap/tune platform-dolt memory or scale Node B
    so the kernel OOM-killer stops killing the retained VM AND vmctl stays
    up; (b) get 0a03783b actually deployed (its deploy leg keeps being
    cancelled by newer doc commits — needs a code-bearing run or a manual
    re-dispatch). Then (stable guest, vmctl ok): M-SUB emit proofs -> M0a
    controlled-comparison verify -> deletion commit -> M1 typed-commitment
    tape -> M2/M3/M5. Station recon done in M5/M2/M3 files.'

receipts: []

weak_measures:
    kind: weak_signal
    baseline: '38e094fc (post-M0-repair; staging tip)'
    desired: 'tracks staging tip within one deploy cycle'
    decision_use: 'a computer more than one build behind can mask repaired
      substrate defects — treat its symptoms as suspect'
    cannot_prove: 'that the stall is a build artifact vs a real defect'
---
