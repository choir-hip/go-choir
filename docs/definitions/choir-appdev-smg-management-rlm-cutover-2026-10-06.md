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
    Slices 1–4 landed locally 2026-10-06: report_to_texture and
    cancel_co_super_assignment deleted from the management registry —
    {desk_go_eval} only; report packets ride choir.ReportPacket ->
    commitLifecycleReportActIntent -> persistentManagementBoundReport
    (DisallowUnknownFields at the cell boundary, schema_version defaulted);
    cancellation rides choir.CancelAssignment -> IntentCancelAssignment ->
    cancelAssignedEngineeringForRun; reducer isSemanticActKind covers both
    kinds; management prompts (rlm overlay + promptstore default) teach the
    in-cell surface only; legacy management_runtime overlay deleted (dead
    path behind deskCarrierLive); TestPromptTaughtVerbsMatchExports
    extended to management + core.yaml and caught the overlay's stale
    choir.Assign teaching — fixed to Cast-only.
  candidate: >-
    Slices: (1) verb carriers landed; (2) prompts rewritten; (3) typed
    tools deleted; (4) parity test extended. Remaining: (5) deployed
    acceptance per finish.acceptance (bound-report leg, cancellation leg,
    no-binding refusal leg, schema exactly {desk_go_eval}).
  conjecture:
    statement: >-
      Management's lifecycle tools are thin wrappers; every capability
      maps to an existing or cheaply-added choir.* verb, so the desk
      seals at {desk_go_eval} with no capability loss.
    verdict: >-
      supported (local) — both tools delete with validations moved
      host-side; unit coverage converted to carrier-level tests. Deployed
      binding proof is the remaining leg.
  believed_state: >-
    Local: agentcore + yaegikernel green, management registry exactly
    {desk_go_eval}, prompts contain zero typed-tool names. Deployed
    management schema unverified until the landing-loop probe activation
    or /api/prompts fetch.
  blocker_or_risk: >-
    report_to_texture's binding validation moved to
    persistentManagementBoundReport unchanged; the no-binding refusal leg
    on staging is the load-bearing negative proof.
  next_action: >-
    Commit + push the cutover, run the landing loop (CI -> Node B deploy
    -> deployed identity), then the four deployed acceptance legs per
    finish.acceptance.

receipts: []
---
