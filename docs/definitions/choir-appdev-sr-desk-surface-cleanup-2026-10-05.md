---
definition_version: 4
definition_id: choir-appdev-sr-desk-surface-cleanup-2026-10-05
execution_mode: mission_orchestrator
member_of: choir-supervised-app-development-metamission-2026-10-01
readiness: drafted
review:
  reviewer: none (owner approved the v5 outline 2026-10-05; file-level panel pending)
  frozen_ref: none
  verdict: none
  evidence_ref: docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md ("v5 plan" -> SR)

start:
  captured_at: '2026-10-05T00:00:00Z'
  source:
    canonical_ref: main@9e0fe4d
    deploy_identity: unknown (verify /health before the first deployed probe)
  worktrees:
    - path: repo root (orchestrator worktree)
      status: unknown
      class: unknown
      owner: orchestrator
      touch: read_only
      recovery: classify before any edit
  observed_baseline:
    - >-
      Research desk registry = desk_go_eval + researchtools.Register (9 typed
      world-access tools) + RegisterEvidenceTools + RegisterRunMemoryTools
      (internal/agentcore/tool_profiles.go:379-410, comment "R3r: the desk
      keeps its typed research surface beside desk_go_eval").
    - >-
      The 13 in-cell research verbs exist (internal/yaegikernel/choir.go,
      e.g. choir.WebSearch; internal/agentcore/tools_desk_egress_test.go).
      Jev M0a's deletion commit, deployed verify and prompt rewrite never
      happened; M0a's goal file is frozen at "phase 1 landed".
    - >-
      The research system prompt = promptstore role prompt
      internal/promptstore/defaults/research.yaml (teaches JSON-tool cadence
      and "use update_coagent for every non-canonical delivery") +
      RLMResearchOverlay (rlm_research_runtime.yaml, which advertises both the
      typed tools and the choir.* verbs). update_coagent is not in the research
      desk registry; S0m retired raw messaging.
    - >-
      Two research runtimes coexist: deskCarrierLive(Research) selects
      rlm_research_runtime.yaml vs legacy research_runtime.yaml
      (tool_profiles.go ~290-296).
    - >-
      processor/reconciler survive as model-policy roles, runtime overlays
      (processor_runtime.yaml, reconciler_runtime.yaml), queue tables
      (processor_requests: 2,582 dispatch_failed, 208k superseded) and the
      sourcecycled processing path. Owner 2026-10-05: delete them; the four
      desks carry this work.

finish:
  deliver: >-
    The research desk is a full RLM: exactly one tool, desk_go_eval, with all
    world access through in-cell choir.* verbs. Its prompt describes one
    record-native surface. processor and reconciler no longer exist anywhere
    in the product.
  artifact: >-
    Research registry {desk_go_eval}; legacy research runtime branch removed
    (or justified as live); research.yaml + rlm_research_runtime.yaml
    rewritten; processor/reconciler roles, overlays, dispatch path and
    queue-table writers deleted (tables left frozen as read-only residue with
    the WW corpus); capability-parity checklist; M1-vs-S0m disposition.
  acceptance:
    - action: >-
        A deployed research activation runs a multi-search loop inside one
        desk_go_eval cell (search -> emit -> search -> emit), with evidence
        reaching Texture mid-loop.
      proves: In-cell world access is sufficient for real research work.
      evidence_class: deployed proof
    - action: >-
        A capability-parity checklist: every deleted typed tool is reachable
        as a choir.* verb or explicitly dropped with a reason.
      proves: The deletion loses no needed capability.
      evidence_class: review receipt
    - action: >-
        A cell that exceeds the egress budget is refused with a budget error
        returned into the cell.
      proves: Metering survives the deletion.
      evidence_class: deployed proof
    - action: >-
        Fetch the deployed research tool schema and assert exactly
        {desk_go_eval}; grep the composed research prompt for update_coagent,
        typed tool names, processor and reconciler (none present).
      proves: The surface and the prompt agree.
      evidence_class: deployed proof
  rollback: git revert of the deletion commits + redeploy; the tool code is only unregistered until the final commit deletes it.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the number of ways a desk can do the same thing and the number
    of instructions that contradict its real surface, while keeping every
    needed capability reachable.
  goodharting_would_be: >-
    Deleting tool registrations while the prompt still teaches them, or
    shipping a prompt rewrite without a deployed cell actually using the
    verbs.

homotopy:
  realism_axis: >-
    One cell calling one verb -> a multi-search loop -> a full
    texture -> research -> report -> resolve chain on the deployed owner
    computer.

boundaries:
  mutation_class: orange
  authority_sources:
    - owner 2026-10-05 ("we get to do that deletion"; "just delete processor and reconciler")
    - docs/definitions/choir-signal-research-rlm-cutover-2026-09-29.md (M0a finish, absorbed)
  must_preserve:
    - Research keeps read-only world authority; egress stays metered by the shared ledger.
    - Frozen WW corpus tables are not dropped (data class frozen + retained).
  excluded:
    - New research verbs (that is SC).
    - Model-policy redesign (that is SM).
  protected_surfaces:
    - research desk prompt and registry
    - model policy roles (removal only)

now:
  status: closed
  slice: >-
    Station closed 2026-10-06 after 7 panel rounds (r7: 7-0 APPROVE, 4
    timeouts, no dissent; the r6 dissent's defect — prompts teaching
    ListContentItemSelectors/ReadContentItemSelector vs kernel's
    ListContentSelectors/ReadContentSelector — was found, fixed at
    12d3adc0, deployed, and re-verified).
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: one-surface-is-enough
    claim: >-
      The 13 in-cell choir.* verbs cover everything the typed research tools
      did, so deleting the tools and rewriting the prompt to one surface loses
      no capability and removes a source of contradictory instructions.
    test: The four acceptance actions above.
    edge: missing_oracle
    delta_o: Deployed cell traces + schema/prompt receipts — all present.
    scope_if_supported: The research desk on the in-cell carrier.
    status: supported
    evidence_refs:
      - docs/evidence/sr-deployed-acceptance-2026-10-06.json
      - docs/evidence/sr-deployed-prompt-research-4467a4fb.json
      - docs/evidence/sr-deployed-prompt-research-12d3adc0.json
  decision:
    what: Delete the typed research surface and processor/reconciler; rewrite the prompts.
    kind: architecture
    status: settled
    evidence_ref: owner 2026-10-05
    owner_ratification_ref: owner 2026-10-05 in the director session
  belief:
    believed_state: >-
      Research is sealed at {desk_go_eval} on deployed 12d3adc0; every
      taught verb resolves; management typed tools remain by design — that
      cutover is SMG (choir-appdev-smg-management-rlm-cutover-2026-10-06),
      owner-directed 2026-10-06.
    main_uncertainty: none for this station's scope.
    next_observation: SMG deployed schema leg.
  blocker_or_risk: none — station closed.
  next_action: >-
    SMG station (management RLM cutover) opens next; SM/SC station files
    remain queued behind it.
receipts:
  - id: sr-station-terminal-2026-10-06
    kind: station_terminal
    station: SR-desk-surface-cleanup
    status: closed
    terminal_receipt: docs/evidence/sr-deployed-acceptance-2026-10-06.json
    identity: 'https://choir.news deployed_commit=12d3adc0 (guest 10.200.4.2 build.deployed_commit matches)'
    landing:
      source_commit: 44a0e221 (docs; runtime HEAD 12d3adc0)
      ci_ref: run 37408595521 (success)
      deploy_ref: Node B deploy 2026-10-06T03:55Z
      environment_identity: 'https://choir.news build.deployed_commit=12d3adc0'
      deployed_acceptance: >-
        legs A (ordered single-cell loop, e80d11f0/call_b3872d82f,
        rlm:report:7/8), B (verbatim egress refusal a3a48168 -> Texture
        bcbe05b7), C (two_emit_loop 5c91129d), D (deployed schema
        {desk_go_eval} + 16-name clean scan on 4467a4fb + bidirectional
        parity 31/31 resolve on 12d3adc0) — all deployed-bound receipts.
      consensus_ref: .agentic-consensus/sr-close-r7-2026-10-06/ (7-0 APPROVE)
    heresy_delta: >-
      discovered: prompts taught nonexistent kernel verb names
      (ListContentItemSelectors/ReadContentItemSelector) — found by panel
      dissent, repaired at 12d3adc0; desk confabulated an egress-refusal
      claim (8a8d10d1) — corrected in evidence file.
      repaired: research sealed at {desk_go_eval}; processor/reconciler
      deleted end to end; prompt↔kernel parity test added (the defect
      class cannot recur silently).
      introduced: GET /api/prompts ungated for reads (owner-auth'd;
      composed prompt + policy fields — no secrets).
    rollback_ref: git revert of 8701668d..12d3adc0 + redeploy
    residuals:
      - 'management typed tools (report_to_texture, cancel_co_super_assignment) → SMG station'
      - 'role_policy.allow_coagent_tools=true on management — dead flag, SMG scope'
      - 'GET /api/prompts read surface is owner-visible; revisit if surface minimization matters'
---

# SR — Desk-Surface Cleanup

Completes the research RLM cutover that R3r started and Jev M0a chartered,
and removes the processor/reconciler pipeline the four desks replace. Small
by design. Its value is that every later station (SA load probes, SM evals,
SC verbs, World Wire extraction) measures and builds on one coherent research
surface instead of two overlapping ones.
