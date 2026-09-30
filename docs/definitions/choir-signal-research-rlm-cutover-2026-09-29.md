---
definition_version: 4
definition_id: choir-signal-research-rlm-cutover-2026-09-29
execution_mode: mission_orchestrator
readiness: reviewed
member_of: choir-jev-supervision-metamission-2026-09-29

# review binds THIS file at its fixed commit (stamped at promotion).
review:
  reviewer: 'agentic-consensus authoring panel (codex, claude, devin,
    gpt6-sol, gemini38) — send_back round resolved'
  frozen_ref: 'pending-stamp'
  verdict: accept
  evidence_ref: '.agentic-consensus/agentic-consensus-20260929-215420/'

start:
  captured_at: '2026-09-29T22:40:00Z'
  source:
    canonical_ref: main@ac54317c
    deploy_identity: 'staging https://choir.news build.commit=b85af274
      (observed at capture; post-M-SUB identity recorded in now at promotion)'
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
    mission: M-SUB (signal plane) — emits + notices must exist before the
      research loop can use them
  observed_artifact:
    - 'research desk registry = desk_go_eval + 14 model-facing tools
      (web_search, fetch_url, source_search, import_document_content,
      read_content_item, list/read_content_item_selector,
      import_url_content, search_wire_corpus, save/read/list_evidence,
      get_run_memory_entry, run-memory tools); engineering is
      capsule_go_eval-only; texture is desk_go_eval-only; management has
      3 typed tools. EgressBudgetLedger (64 calls/32MiB per activation)
      meters the typed tools — deleting without rehoming deletes
      governance. rlm_research_runtime.yaml + research.yaml prompts
      instruct the deleted cadence.'

finish:
  deliver: 'The research desk is a full RLM — exactly one tool
    (desk_go_eval) — with all capability carried as in-cell choir.* verbs;
    the egress budget meters the verbs; prompt overlays describe only the
    Go-module surface.'
  artifact: 'research registry = {desk_go_eval} only; choir.WebSearch /
    FetchURL / evidence-ops verbs; egress budget charging at the broker/
    host layer; prompt overlays rewritten; capability-parity checklist'
  acceptance:
    - action: 'a research activation performs a multi-search loop in one
        desk_go_eval cell — search→emit→search→emit — with evidence
        reaching texture mid-loop and no sequential model round-trips'
      proves: 'the iterative-research pattern the QA failure exposed'
      evidence_class: deployed proof
    - action: 'capability-parity checklist: every deleted tool''s
        capability either reachable via choir.* or explicitly dropped
        with reason, recorded in the goal receipts'
      proves: 'no silent capability regression'
      evidence_class: static analysis + deployed proof
    - action: 'run a cell that exceeds the rehomed egress budget; the
        broker refuses with a budget error returned into the cell'
      proves: 'governance moved, not deleted'
      evidence_class: deployed proof
    - action: 'fetch the deployed research model-request tool schema after
        the deletion commit; assert exactly {desk_go_eval} and verify the
        active prompt overlays reference only retained in-cell capabilities'
      proves: 'one-tool doctrine landed, overlays consistent'
      evidence_class: deployed proof + static analysis
  rollback: 'two-commit phasing (verbs+emission first while the typed
    surface remains, deletion second) gives a rollback point; rollback of
    the first commit names the broker+budget changes and keeps the typed
    surface compatible until it can also revert safely; git revert the
    deletion commit restores the typed surface'
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity,
      deployed_acceptance]

value:
  better_means: 'minimize research''s context-compounding tool-call loop
    while preserving every capability it actually needs — the divergence
    between "RLM authors Go" and "RLM makes 9 sequential tool calls with
    growing context"'
  goodharting_would_be: 'deleting tools without parity (capability
    regression) or leaving the egress ledger unwired (governance
    regression)'

homotopy:
  realism_axis: 'loop depth — from one-cell-one-search up to
    search→emit→deepen cycles inside a single Go cell; research running
    for days/weeks continuously is the far end'

boundaries:
  mutation_class: red
  # red: desk tool surface + broker actions + prompts + protected
  # research surface.
  authority_sources: [owner direction (all four desks = one tool,
    desk_go_eval), orientation doc M0a rows, panel adjudication]
  must_preserve:
    - 'egress budget accounting (rehomed, not deleted)'
    - 'capability parity — every needed capability reachable via choir.*'
    - 'desk_go_eval worker contract (session worker, poisoning, respawn)'
  excluded:
    - 'management typed-tool deletion (M5''s scope)'
    - 'World Wire fanout'
    - 'model-policy module (M2''s scope)'
  protected_surfaces:
    - 'researchtools register + egress ledger'
    - 'desk cell registry + runtime prompt overlays'
    - 'rlm_research_runtime.yaml + promptstore/defaults/research.yaml'

now:
  status: blocked_incomplete
  slice: 'reconcile M-SUB acceptance; enumerate every current research
    tool + its budget hook; freeze the parity checklist; then author
    choir.WebSearch/FetchURL/evidence-ops verbs on the typed surface
    (two-commit phase 1)'
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
    id: go-cell-loop-closes-123k
    claim: 'If research authors a Go cell looping search→emit→deepen, the
      123k-input-token serial tool-call pattern disappears and the
      research leg drops from ~470s to a small multiple of per-call
      latency, normalized per committed evidence item'
    test: 'QA repro timing on the cutover build: research leg duration +
      input-token count vs the M0 baseline
      (docs/evidence/m0-qa-baseline-timings-*.json)'
    edge: frame_lock
    # "turn" semantics change; the metric must compare work done, not
    # calls made — normalize per committed evidence item
    delta_o: 'per-leg timing records from M0 baseline vs cutover build'
    scope_if_supported: 'research desk performance + iterative-research
      shape'
    status: proposed
    evidence_refs: []
  decision:
    what: 'two-commit phasing (verbs+emission land while typed surface
      still exists → controlled-comparison verify → deletion commit)'
    kind: operational
    status: settled
    evidence_ref: 'owner answer 2026-09-29 + panel phasing
      recommendation'
    owner_ratification_ref: 'owner confirmed sequential-commit phasing'
  belief:
    believed_state: 'research is the furthest-behind desk; the QA stall
      is the direct symptom of the serial tool loop + no report-back'
    main_uncertainty: 'exact verb surface beyond WebSearch/FetchURL —
      evidence/memory ops parity needs the checklist'
    next_observation: 'first desk_go_eval-only research turn completing
      a multi-search loop on staging'
  blocker_or_risk: 'depends on M-SUB; prompt-overlay rewrites must ship
    in the deletion commit or regression is instant'
  next_action: 'after M-SUB: land verbs+emission on the existing typed
    surface, verify stall fix, then the deletion commit'

receipts: []
---
