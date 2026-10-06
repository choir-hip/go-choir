---
definition_version: 4
definition_id: choir-appdev-smg-management-rlm-cutover-2026-10-06
execution_mode: mission_orchestrator
member_of: choir-supervised-app-development-metamission-2026-10-01
readiness: drafted
review:
  reviewer: pending (owner directive 2026-10-06 supersedes authoring panel on scope)
  frozen_ref: none
  verdict: none
  evidence_ref: docs/problems/management-typed-json-tools-remain-2026-10-06.md

start:
  captured_at: '2026-10-06T03:45:00Z'
  source:
    canonical_ref: main@44a0e221
    deploy_identity: 'https://choir.news deployed_commit=12d3adc0'
  observed_baseline:
    - >-
      buildDeskCellRegistry (internal/agentcore/tool_profiles.go:370-392)
      registers desk_go_eval for every desk, then adds for management:
      report_to_texture (RegisterPersistentManagementReportTools,
      tools_engineering_assignment.go:132) and cancel_co_super_assignment
      (RegisterAssignedEngineeringTools, :46). Management desk schema =
      {desk_go_eval, report_to_texture, cancel_co_super_assignment};
      research and texture are sealed at {desk_go_eval}.
    - >-
      report_to_texture carries one real capability the bare verbs lack:
      bound lifecycle-control validation — requires persistent-management
      execution context, an exact delivered control binding (one
      trajectory/work/Texture scope), and work_disposition open|completed.
      cancel_co_super_assignment durably revokes an assignment capsule with
      executor acknowledgement. These validations must move behind choir.*
      verbs (host-side checks), not be deleted.
    - >-
      choir verbs already export for management: Message, Emit, Cast, Ask,
      Note, Reply, CancelAct, Escalate, Precommit, Report, ReportPacket,
      Resolve, Disagreement (deskModuleSets["management"], choir.go:177).
      CancelAct is a candidate carrier for assignment cancellation;
      Report/ReportPacket for the typed progress packet.
    - >-
      role_policy.allow_coagent_tools=true still advertised on management
      (R6 panel residual). Management/engineering prompts may still teach
      typed-tool cadence — audit all desk prompts for non-RLM surface.
    - >-
      Owner directive 2026-10-06: "get rid of all of these adhoc json tool
      calls everywhere" + "make sure the prompts are all updated for full
      rlm only".

finish:
  deliver: >-
    The management desk is a full RLM: exactly one tool, desk_go_eval.
    report_to_texture and cancel_co_super_assignment are deleted end to
    end; their validations live as host-side checks behind choir.* verbs
    (bound lifecycle control on Report/ReportPacket for persistent
    management runs; executor-acknowledged cancellation on a management
    verb). Every desk prompt teaches the cell + choir.* surface only.
  artifact: >-
    Management registry {desk_go_eval}; typed tools deleted with their
    registration tests migrated to verb-level tests; lifecycle binding
    validation reachable from a cell; all desk prompts audited RLM-only;
    deployed schema leg for management.
  acceptance:
    - action: >-
        A deployed persistent-management activation emits a typed progress
        report through a choir.* verb inside a desk_go_eval cell; the
        packet binds the delivered lifecycle control (trajectory/work/
        Texture scope) and reaches Texture; work_disposition=completed
        settles the work item.
      proves: Lifecycle reporting survives the cutover with binding intact.
      evidence_class: deployed proof
    - action: >-
        A deployed persistent-management activation cancels an assignment
        through a choir.* verb; the capsule is durably revoked with
        executor acknowledgement.
      proves: Cancellation capability survives the cutover.
      evidence_class: deployed proof
    - action: >-
        A cell calling report-style verbs without a delivered control
        binding is refused with the binding error returned into the cell.
      proves: The binding validation moved to the verb, not away.
      evidence_class: deployed proof
    - action: >-
        Fetch the deployed management tool schema and assert exactly
        {desk_go_eval}; grep every desk's composed prompt for typed-tool
        names (none present); every taught choir.<Name> resolves to a
        kernel export (TestPromptTaughtVerbsMatchExports extended to
        management).
      proves: All desk surfaces and prompts agree — full RLM everywhere.
      evidence_class: deployed proof
  rollback: >-
    git revert of the deletion commits + redeploy; verbs land before tool
    deletion so the intermediate state is dual-listed, never capability-less.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    One way to do each semantic act (a choir.* verb committing a
    precommitment record), not a typed JSON tool beside the cell. Zero
    desk-carries-typed-tools exceptions.
  goodharting_would_be: >-
    Deleting the JSON tools while losing the binding validation, or
    leaving management prompts teaching a surface that no longer exists.

homotopy:
  realism_axis: >-
    One in-cell Report -> bound lifecycle report -> assignment
    cancellation -> deployed schema exactly {desk_go_eval}.

boundaries:
  mutation_class: orange
  authority_sources:
    - owner 2026-10-06 (kill all ad-hoc JSON tool calls; RLM-only prompts)
    - docs/problems/management-typed-json-tools-remain-2026-10-06.md
  must_preserve:
    - Lifecycle control binding validation (report_to_texture semantics).
    - Executor-acknowledged assignment cancellation.
    - Engineering desk verifier-slot conditional exports (Verify/InspectBundle).
  excluded:
    - SC desk capability surface expansion (new read verbs).
    - SM model-policy rewrite.
  protected_surfaces:
    - management desk registry and prompt
    - persistent-management lifecycle binding path
    - assignment capsule lifecycle

now:
  status: working
  slice: >-
    Slices 1-4 landed and DEPLOYED 2026-10-06 (commits 1b1d9b7e,
    4bedf999; CI run 37420355583 green incl. Node B deploy):
    report_to_texture, cancel_co_super_assignment AND the third ad-hoc
    tool found on the deployed registry — product_api_request — are all
    deleted. Reports ride choir.ReportPacket ->
    commitLifecycleReportActIntent -> persistentManagementBoundReport;
    cancellation rides choir.CancelAssignment -> IntentCancelAssignment;
    product API calls ride choir.ProductAPI -> broker ActionProductAPI ->
    agentcore.productAPIRequest (same allowlist + owner-bound serving).
    Latent defect repaired in passing: evidence broker actions
    (save/read/list/run-memory) validated but never dispatched — desk
    evidence verbs returned "unsupported action".
  candidate: >-
    Deployed schema leg PASSED twice: GET /api/prompts/management on
    computer-03335285 reports tools exactly [desk_go_eval] (zero
    typed-tool names in the effective prompt), and a fresh disposable
    guest's boot log prints super=1 — exactly one management tool at
    runtime. Behavioral legs attempted: smg_rlm_acceptance_probe.mjs
    submitted two trajectories on the owner computer — both starved
    (one texture_turn_committed, then stalled) by the known Management
    live-occurrence storm (owner guest cycles terminal/received pairs
    every ~10-15s; a research actor delivery-poisoned after 65
    deferrals). A disposable computer cannot substitute — it hits the
    canonical-head bootstrap residual (genesis absent -> submit 500).
    Post-deploy guest log shows organic cell ReportPacket intents +
    cancel_report obligations minted on the new path.
  conjecture:
    statement: >-
      Management's lifecycle tools are thin wrappers; every capability
      maps to an existing or cheaply-added choir.* verb, so the desk
      seals at {desk_go_eval} with no capability loss.
    verdict: >-
      supported — all three typed tools deleted, validations moved
      host-side, deployed schema exactly {desk_go_eval}. Behavioral legs
      blocked by substrate: the owner-guest occurrence storm (a known
      heresy) starves fresh activations.
  believed_state: >-
    Deployed https://choir.news build=4bedf999; management schema exactly
    {desk_go_eval} (deployed proof); organic post-deploy traffic mints
    ReportPacket/CancelAssignment intents with no typed-tool fallback.
  blocker_or_risk: >-
    BLOCKED on the Management live-occurrence storm starving probe
    activations on the only reachable computer (the API key owns
    computer-03335285; the other active computer belongs to a different
    owner). The storm's substrate fix is SA-station scope. Next attempt:
    drain the storm (park/repair the replaying trajectories) or run the
    probe on a computer with genesis already committed.
  next_action: >-
    Storm observed to SURVIVE guest restart (reboot drained pending, then
    the replayed backlog re-fed the storm: running_runs 0->4, pending 2->72
    in ~15min; third probe starved). Retry path: park/drain the replayed
    storm trajectories or a pre-genesis disposable, then re-run
    scripts/smg_rlm_acceptance_probe.mjs for legs 1-3. Schema leg proven.

receipts: []
---
