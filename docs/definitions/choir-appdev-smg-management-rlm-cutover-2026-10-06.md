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
    deferrals). Disposable attempt 2026-10-06 (computer-423134e8,
    bootstrap-chain genesis OK, marker smg-dispo-*): THREE texture desk
    activations consumed the owner revision with `desk cell completed
    with no authoring act` then exhausted the 1.2M-token loop budget
    (~22 desk_go_eval iterations, no ApplyTexture staged). Second
    attempt used a corrected prompt (non-empty execution_request
    actions, worked ApplyTexture call, correct 3-arg ReportPacket
    arity) — same outcome in ~7 min. This is the documented desk-agency
    edge (texture desk does not emit open_persistent_super on demand)
    recorded under S0m; not a transport defect. Then host-pressure
    reclaim HIBERNATED the probe VM mid-run (vmctl journal 12:44Z,
    epoch=12916): gateway now 502s `failed to resolve user autoputer`
    for the computer — reclaim killed a VM with a live owner-scoped
    probe in flight. Recorded for SA slice 1+ and the S2 hibernate
    reclaim killed a VM with a live owner-scoped
    probe in flight. Recorded for SA slice 1+ and the S2 hibernate
    contract.

    2026-10-06 second disposable (computer-03335285, marker
    smg-rlm-clean): prompt_bar_submit OK; escalate leg minted persistent
    Management run 462d30ea at 17:24Z that stayed `pending` forever —
    `initial_dispatch` lost AND the fresh-mint watchdog's
    redriveStrandedFreshMintManagement returned false on zero bound
    packets (escalate binds none), slot deadlocked. Probe timed out 45min
    (`docs/evidence/smg-rlm-acceptance-owner-clean-2026-10-06.json`). NEW
    SUBSTRATE DEFECT — problem doc
    `docs/problems/sa-management-mint-no-start-slot-deadlock-2026-10-06.md`,
    fix 01199fb1 fail-releases the slot at the watchdog deadline.

    2026-10-06 post-fix probe (owner computer refreshed to build
    01199fb1, guest /health verified): 40min window, ZERO persistent
    Management mints (`bound run` absent from console). Console streamer
    (`SmgConsoleTail` job) confirms no `slot_occupied` deferrals in the
    fixed-boot segment — zombie-slot defect absent. Probe legs therefore
    never exercised: texture desk never authored open_persistent_super.
    Blocker is unchanged: texture-desk agency, not substrate.

 2026-10-06 midcourse consensus review (10/11 agents,
 `.agentic-consensus/agentic-consensus-20261006-midcourse/`): the
 probe's 40-min zero-mints stays attributed to texture-desk agency as a
 **hypothesis** (`controls[]` emission is not console-observable; only the
 turn-commit receipt distinguishes never-emitted from
 emitted-and-discarded). Panel reorientation: build a deterministic
 owner-side control surface that mints persistent Management without
 texture agency — the desk's `open_persistent_super` authoring is a
 model-behavior dependency with no convergence date and every behavioral
 leg sits behind it. Landed en route: `e6327506` — vmctl
 degraded→active re-promotion on lookup (bearer-routing wedge) +
 dispatcher emission-discard instrumentation. Probe-hardening residuals
 recorded: `/api/trajectories` polling is status-blind, `work_disposition`
 over-matches bound reports, and "report before bound report" is not a
 genuine unbound test (controls bind pre-activate).
  conjecture:
    statement: >-
      Management's lifecycle tools are thin wrappers; every capability
      maps to an existing or cheaply-added choir.* verb, so the desk
      seals at {desk_go_eval} with no capability loss.
    verdict: >-
      supported — all three typed tools deleted, validations moved
      host-side, deployed schema exactly {desk_go_eval}. Behavioral legs
      still unproven: owner-guest storm (substrate heresy, SA slice 1)
      starves activations there; disposable path now reaches the texture
      desk but the desk does not author open_persistent_super on demand
      (agency edge, documented since S0m).
  believed_state: >-
    Deployed https://choir.news build=4bedf999; management schema exactly
    {desk_go_eval} (deployed proof); organic post-deploy traffic mints
    ReportPacket/CancelAssignment intents with no typed-tool fallback.
    Behavioral legs remain unproven on both reachable surfaces.
  blocker_or_risk: >-
    BLOCKED on two independent substrate edges for legs 1-3: (a) the
    Management live-occurrence storm on the owner computer (director
    forbade draining it — SA slice 1 owns the fix); (b) texture-desk
    agency: three activations on a clean disposable failed to emit
    open_persistent_super and burned the loop budget (S0m-documented
    edge: prompting a specific verb is unreliable on this model;
    transport is proven by the schema leg + unit suite). No
    owner-reachable path mints a persistent-management activation other
    than the texture controls[] opener — fabricating one via internals
    would not exercise the product surface.
  next_action: >-
    SA slice 0 pushed (9f6f369c): computers provisioned after this
    deploy mint genesis_imported in-guest before the replay gate opens,
    and pre-genesis writes map to a clean 503 — the bootstrap-chain
    preamble becomes a repair path only. Then SA slice 1 (storm
    convergence) unblocks legs on the owner computer. For the
    disposable path the remaining blocker is texture desk agency, not
    substrate: retry probe after SA lands, OR accept the agency edge
    and close legs via a future deterministic owner surface. DIRECTOR
    2026-10-06 guidance (do not drain the owner storm; run legs on a
    fresh disposable) executed — disposable reached the desk; the desk
    did not author the opener control. New residual recorded:
    host-pressure reclaim hibernates a VM with a live probe run.

    DIRECTOR 2026-10-06: do not drain or park the owner guest's storm to
    pass legs 1-3; that proves nothing about the substrate and hides it.
    Run the legs on a fresh disposable with a registration -> API key ->
    POST /api/computers/{id}/lifecycle/bootstrap-chain preamble (the
    documented manual genesis route; s0b-registration-computer-missing-
    genesis-2026-10-04). Have the probe create any history a leg needs
    (an assignment to cancel). If the legs pass there, close SMG with a
    named edge: re-run on the owner computer after SA slice 1.
receipts: []
---
