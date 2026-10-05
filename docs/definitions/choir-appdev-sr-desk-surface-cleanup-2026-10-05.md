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
  status: working
  slice: >-
    Move 1 landed 2026-10-05 (8701668d): prompt rewrite + sealed
    {desk_go_eval} registry in one commit. Parity checklist below —
    every deleted typed tool maps to an existing choir.* egress verb.
    Processor/reconciler end-to-end deletion in flight (delegated cut).
    Deletion-citers grep complete: processor/reconciler citers are live
    surfaces (profiles, policies, prompts, overlays, dispatch, spawn
    list, handoff path) vs frozen-history recognition (vocabmigrate
    sets, computerevent/decode.go, lifecycle admission strings,
    wirepublish/eligibility kind checks, wire/processorkey, sourceapi
    request types) — the latter preserved.
    Parity checklist (typed tool -> choir.* verb):
    web_search -> WebSearch; source_search -> SourceSearch;
    fetch_url -> FetchURL; import_url_content -> ImportURL;
    import_document_content -> ImportDocument;
    search_wire_corpus -> SearchWireCorpus;
    read_content_item -> ReadContentItem;
    list_content_item_selectors -> ListContentItemSelectors;
    read_content_item_selector -> ReadContentItemSelector;
    save_evidence -> SaveEvidence; read_evidence -> ReadEvidence;
    list_evidence -> ListEvidence; get_run_memory_entry -> RunMemoryEntry.
    No tool exists without a verb; egress ledger shared (same budget).
    M1-vs-S0m disposition: M1 (rlm-cutover phase 2, the deferred
    deletion) is SR by absorption — this station is its continuation;
    S0m's channel-mail retirement is honored (prompt teaches Report,
    not mail; update_coagent gone).
  source_ref: main@9e0fe4d
  deploy_identity: unknown
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
    delta_o: A deployed cell trace showing the verbs used end to end.
    scope_if_supported: The research desk on the in-cell carrier.
    status: proposed
    evidence_refs:
      - internal/agentcore/tool_profiles.go:379-410
      - internal/promptstore/defaults/research.yaml
      - internal/runtimeprompts/overlays/rlm_research_runtime.yaml
  decision:
    what: Delete the typed research surface and processor/reconciler; rewrite the prompts.
    kind: architecture
    status: settled
    evidence_ref: owner 2026-10-05
    owner_ratification_ref: owner 2026-10-05 in the director session
  belief:
    believed_state: The cutover is half-done and the prompts contradict the registry.
    main_uncertainty: Whether any capability exists only as a typed tool (for example a selector or import path with no verb equivalent).
    next_observation: The parity checklist.
  blocker_or_risk: >-
    Deleting before the prompt rewrite strands the model on instructions it
    cannot follow; land the prompt rewrite and the registry change in the
    same deploy.
  next_action: >-
    Processor/reconciler deletion landing -> deployed acceptance (schema
    fetch {desk_go_eval}; composed-prompt grep; cell multi-search loop;
    egress-budget refusal). Done: citers grep, parity checklist,
    M1-vs-S0m disposition, prompt rewrite + registry seal (8701668d).

receipts: []
---

# SR — Desk-Surface Cleanup

Completes the research RLM cutover that R3r started and Jev M0a chartered,
and removes the processor/reconciler pipeline the four desks replace. Small
by design. Its value is that every later station (SA load probes, SM evals,
SC verbs, World Wire extraction) measures and builds on one coherent research
surface instead of two overlapping ones.
