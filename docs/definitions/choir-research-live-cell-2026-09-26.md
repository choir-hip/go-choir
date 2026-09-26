---
definition_version: 3
definition_id: choir-research-live-cell-2026-09-26
execution_mode: mission_orchestrator

start:
  captured_at: '2026-09-26T06:45:00Z'
  source:
    canonical_ref: main@8f228efb
    deploy_identity: staging https://choir.news build.commit=119e0edd
    worktrees:
      - path: /Users/wiz/go-choir
        status: clean
        class: source
        owner: this session
        touch: read_write
        recovery: git
  predecessor:
    mission: choir-texture-live-authoring-2026-09-26
    disposition: >-
      R3d landed+deployed 2026-09-26 (119e0edd, ci 36222906276). Texture is
      a full-RLM desk on the cell carrier: desk_go_eval only; cell-authored
      choir.ApplyTexture intents commit through the atomic ApplyTextureTurn
      as AuthorAppAgent revisions; the typed texture tools and the
      desk-originated worker_updates consumer path are deleted.
    evidence_ref: docs/definitions/choir-texture-live-authoring-2026-09-26.md
  spine_meta_goal: docs/definitions/choir-rectification-spine-2026-09-25.md
  station: R3r
  owner_directive: >-
    Plan §11 R3r (off spine): research on cells with the network/memory cap
    policy resolved — the D2 residual, contained here. Proof: a research
    cell Report mints a record under its cap boundary. D2 (corrected):
    desk cells run in a killable subprocess with restricted stdlib;
    research needs network/memory caps — untrusted web-derived code
    in-process can OOM the daemon. No a/b split: the D2-capable promotion
    is this mission, not a staged flag.

finish:
  outcome: >-
    The research desk runs on cells under the host desk-cell carrier with
    its cap policy resolved: its registry is desk_go_eval plus the typed
    research domain surface (web_search, fetch_url, source_search,
    import_url_content, import_document_content, search_wire_corpus,
    read_content_item, list_content_item_selectors,
    read_content_item_selector) and the evidence/memory domain tools
    (save_evidence, read_evidence, list_evidence, get_run_memory_entry) —
    every host-mediated network call accountable to a per-activation
    egress budget — and the generic host tools (read_file, glob, grep,
    verify_model_capability, spawn_agent, cancel_agent) removed from the
    research desk registry. Desk session workers carry an address-space
    cap so a model-authored cell cannot OOM the daemon.
  artifact: >-
    deskCarrierLive(research) promoted unconditional; a research-scoped
    cell-registry builder sealing desk_go_eval + the capped research
    surface; an activation-scoped egress budget (calls + fetched bytes)
    enforced across the host-mediated research tools; a worker
    address-space cap (RLIMIT_AS) plumbed through DeskSessionWorkerConfig
    and applied in the desk-session entrypoint; prompt/overlay text
    updated for the cell-carrier research desk.
  acceptance:
    - action: >-
        A research-profiled activation runs on the cell carrier with the
        sealed registry — desk_go_eval + the typed research/evidence/memory
        surface — and no generic host tools.
      proves: promotion + seal.
      evidence_class: local test
    - action: >-
        A research cell stages choir.Report / choir.ReportPacket; the
        reducer appends the commitment_record under the activation's cap
        accounting — the record mints while the egress budget and worker
        memory cap hold.
      proves: research cell Report mints a record under its cap boundary.
      evidence_class: local test
    - action: >-
        The per-activation egress budget refuses beyond-cap network calls
        honestly (counted call budget and byte budget both trip, distinct
        activation keys are independent); the worker entrypoint applies
        the address-space cap before serving cells.
      proves: the D2 cap policy is real, not a flag.
      evidence_class: local test
    - action: >-
        A mid-episode worker death mid-cell leaves no half-commit: tray
        intents die with the worker; a respawned worker resumes from the
        tape.
      proves: restart-durable research cells.
      evidence_class: local test
  rollback: >-
    git revert + redeploy. Reverting re-gates deskCarrierLive(research) to
    actuator=rlm, restores the full 18-tool research registry under
    actuator=tools, and removes the egress budget + worker address cap.
    The carrier substrate (R3b), management (R3c), and texture (R3d) are
    unaffected.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Research — the desk that gathers web-derived evidence for the
    supervision doc — becomes a cell desk like the others: its claims mint
    as commitment_records from cells, and the D2 residual (untrusted
    web-derived code in-process can OOM the daemon) closes with a
    declared, enforced boundary instead of an open flag.
  goodharting_would_be: >-
    Promoting research onto cells with no cap and declaring D2 resolved,
    or keeping the flag and declaring the mission done. Acceptance
    insists the promotion is unconditional AND the egress budget + worker
    memory cap both exist and trip.
  falsifiers:
    - 'Falsified: research stays behind actuator=rlm or keeps generic host
      tools on the carrier — the promotion is incomplete.'
    - 'Falsified: any host-mediated network call bypasses the activation
      egress budget — the cap is decorative.'
    - 'Falsified: a research worker runs without the address-space cap —
      the OOM boundary is unenforced.'
    - 'Falsified: a cell-staged Report does not mint a commitment_record —
      the proof is a projection.'

now:
  status: working
  slice: >-
    station R3r CHARTERED 2026-09-26. Research already carries the full
    semantic cell-verb set (Cast/Ask/Note/Reply/CancelAct/Escalate/
    Precommit/Report/ReportPacket/ResolveAct) — cells never reach the
    network themselves (allowlist bans net/net-http); all world access is
    the host-mediated typed research surface, whose calls this mission
    binds to a per-activation egress budget. Worker memory cap is
    RLIMIT_AS applied in the desk-session entrypoint. Generic host tools
    (read_file/glob/grep/verify_model_capability/spawn_agent/
    cancel_agent) retire from the research desk registry; the typed
    research/evidence/memory surface stays (R3c precedent: typed
    authority surfaces persist beside desk_go_eval).
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: research-cell-capped-egress
    claim: >-
      Binding the research desk to the cell carrier with desk_go_eval +
      the typed research surface under a per-activation egress budget and
      a worker address-space cap lets a research cell stage
      choir.Report/ReportPacket that mints a commitment_record — under
      the cap boundary, not beside it.
    test: >-
      A research activation runs on cells with the sealed registry; a
      cell Report/ReportPacket appends a commitment_record; the egress
      budget trips beyond-cap calls and bytes; the worker entrypoint
      applies RLIMIT_AS before serving.
    delta_o: >-
      If research is cell-bound and capped, every non-wire desk is on the
      carrier (engineering/management/texture/research); D2 is fully
      discharged; only wire desks (processor/reconciler, D4-deferred)
      remain on the tool loop.
    scope_if_supported: research desk on cells + D2 cap boundary closed
    status: active
    evidence_refs:
      - docs/desk-rlm-rectification-plan-2026-09-23.md §11 (R3r), §7 D2
  belief:
    believed_state: >-
      CHARTERED. Research verbs already stage and reduce (R2 substrate);
      the promotion gate is a one-line deskCarrierLive flip; the real work
      is the cap policy: an activation-scoped egress budget threaded
      through researchtools.Dependencies and an RLIMIT_AS plumbed through
      SessionWorkerConfig into the desk-session entrypoint.
    next_observation: >-
      CI + staging deploy for the R3r head; deployed acceptance = platform
      healthy at the R3r head with research activations binding the cell
      carrier and the capped typed surface.
  blocker_or_risk: >-
    Egress budget keyed per activation (RunRecord-derived), not per call:
    a persistent desk accumulates its budget across cells of one
    activation — deliberately, since budget state lives in run memory
    semantics (per-activation lifecycle), not per cell. Risk: budget keys
    must survive worker respawn — they are host-side in the Runtime, not
    worker memory.
  next_action: >-
    Landing loop: push, monitor CI + Node B deploy, verify staging
    environment identity at the pushed SHA, record deployed acceptance,
    settle; then charter the next spine station (R4, spine after R3d).

receipts:
  - "charter: R3r = research on cells with the D2 network/memory cap
    policy resolved (plan §11, off spine). Design: desk_go_eval + typed
    research/evidence/memory surface (R3c precedent for typed authority
    tools); generic host tools removed; per-activation egress budget on
    every host-mediated network tool; RLIMIT_AS on desk workers."
  - "local proofs (pre-push): build clean for agentcore/researchtools/
    yaegikernel/runtimeprompts/autoputer; focused shard 38 tests green —
    TestResearchCellRegistryIsSealed (desk_go_eval + the 13-tool typed
    surface, no generic host tools), TestR3rResearchCellReportMintsCommitmentUnderCap
    (Report + ReportPacket mint commitment_records),
    TestR3rResearchEgressChargedOnNetworkTools (fetch_url charges calls +
    bytes; second call past cap refuses honestly), TestEgressBudget*
    (call cap, byte cap, per-activation key independence, refusal does not
    charge), TestDeskSessionWorkerEnvRoundTrip (cap crosses the spawn env
    contract), TestResearchOverlaySwitchesToCellCarrier,
    TestDefaultProfileRegistriesExactAuthorityContract/research (exact
    14-tool surface)."
  - "cap boundary landed: deskCarrierLive(research) unconditional; egress
    ledger installed before registry builds and charged by web_search,
    source_search, fetch_url (call + response bytes), import_url_content,
    import_document_content on both the cell and host research paths;
    SessionWorkerConfig.MemoryLimitBytes plumbed through the spawn env to
    ApplyWorkerMemoryLimit (RLIMIT_AS 8GiB Linux — the autoputer worker
    binary's linked baseline exceeds 1GiB at spawn, so the cap binds
    growth over baseline; unsupported-kernel platforms degrade to
    uncapped, containment stays the subprocess+package boundary).
    Note: ingress reliability for Cast sub-agents is unchanged — the
    budget meters the desk's own host-mediated network calls."
---

# R3r — Research Live Cell (station on the rectification spine)

Live station under [`choir-rectification-spine-2026-09-25.md`](choir-rectification-spine-2026-09-25.md).
Scope per [`desk-rlm-rectification-plan-2026-09-23.md`](../desk-rlm-rectification-plan-2026-09-23.md) §11
(R3r, off spine): research on cells with the network/memory cap policy resolved — the D2 residual,
contained here. Proof: a research cell Report mints a record under its cap boundary.
