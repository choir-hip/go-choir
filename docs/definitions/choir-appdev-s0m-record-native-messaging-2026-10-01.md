---
definition_version: 4
definition_id: choir-appdev-s0m-record-native-messaging-2026-10-01
execution_mode: mission_orchestrator
readiness: reviewed
member_of: choir-supervised-app-development-metamission-2026-10-01

# Owner direction 2026-10-01: raw messaging retires; every inter-agent act is
# a commitment-ledger record whose Addressee is the delivery instruction;
# ApplyTexture edits the document only. Design adjudicated by two convergent
# panels (record-native-20261001 7/13, messaging-normalize-20261001 5/13) and
# the arch-doc review panel (archdoc-review-20261001 7/13, commit-after-edits,
# all applied). Inserted ahead of S0b on owner instruction; S0 remains
# `working` and resumes its S0b slice after this station lands.

review:
  reviewer: >-
    agentic-consensus convergent panels — design adjudication
    (record-native-20261001, 7/13 quorum) + mechanism normalization
    (messaging-normalize-20261001, 5/13) + doc accuracy review
    (archdoc-review-20261001, 7/13, verdict commit-after-edits, applied)
  frozen_ref: main@3eb132c7
  verdict: accept
  evidence_ref: >-
    docs/reports/agent-messaging-system-state-2026-10-01.md §§9-12;
    docs/desk-system-harness-architecture-2026-10-01.md;
    .agentic-consensus/record-native-20261001/

start:
  captured_at: '2026-10-01T00:00:00Z'
  source:
    canonical_ref: main@3eb132c7
    deploy_identity: >-
      staging https://choir.news deployed_commit=fd8b2973 (deadlock fix
      live); owner guest computer-03335285269bdba4f94377e56879f9e6 epoch=1001
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean at capture
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed_baseline:
    - >-
      Lifecycle research runs carry `work_item_ids` +
      `lifecycle_control_bindings` but no `assignment_id` and no
      `requested_by_*` run metadata (staging run 20cd8749, /api/runs).
      isAssignedDesk() tests assignment_id only (rlm_reduce.go:1239-1243), so
      research Message/ReportPacket to texture falls to castStagedIntent:
      commitment record + channel row whose channel_message wake no-ops on
      texture:* (actorruntime/handler.go:306-308). Research findings never
      wake texture — the hollow-revision bug class.
    - >-
      The earlier ProducerUpdateID repair (rlm_reduce.go lifecycle branch)
      fixed queue internals the affected runs never reach. The dispatch
      predicate, not the queue, is the defect.
    - >-
      requested_by_* provenance lives on work.Details, not run metadata;
      inheritRequesterMetadataFromWorkItem (runtime.go:1382) already copies
      it but is called only from the trajectory-sweep spawn path
      (runtime.go:2827), not the lifecycle-control activation path
      (management_controller.go:~1740).
    - >-
      IntentReport never takes the queue path under any condition — only
      IntentMessage checks isAssignedDesk (rlm_reduce.go:942-953).
    - >-
      QueueLifecycleUpdate accepts producer_report direction only; the
      control direction exists solely inside ApplyTextureTurn
      (store/texture_turn.go). The agentcore producer-report validator
      requires texture target + texture requester
      (tools_worker_update.go:490-534); management:* targets have no
      record-native contract.
    - >-
      Vacuous operational records: ask/reply/note/escalate/cancel mint
      commitment records WITHOUT their body (rlm_reduce.go:1116-1198);
      operational records also pollute the materiality projection as open
      claims; cancel never closes its target.
    - >-
      Emit is an immediate side-effect outside the tray
      (yaegikernel/choir.go:383-390); ReduceCellIntents has zero non-test
      callers; stranded-bound repair (textureStrandedDeliveries) exists only
      for producer_report→texture; ListBoundPendingUpdatesForTarget filters
      to producer reports.
    - >-
      Full mechanism census: docs/reports/agent-messaging-system-state-
      2026-10-01.md. Architecture doc: docs/desk-system-harness-architecture-
      2026-10-01.md. Problem doc: docs/problems/texture-research-hollow-
      revisions-2026-10-01.md §residual defect.

finish:
  deliver: >-
    Inter-desk delivery is record-native on staging: every addressed desk act
    mints its commitment record and derives its lifecycle packet in one
    reducer commit, the channel log is audit-only, and research findings
    reach texture — the owner's desk sees research answers become revisions
    instead of silent hollow churn.
  artifact: >-
    The five-kind record surface ({precommit, report, resolve, disagreement,
    directive{note, escalate, cast, retract}}) on the cell verb layer;
    record+packet single-commit mint with packet IDs derived from record IDs;
    IssueLifecycleControl extracted from ApplyTextureTurn; requested_by_*
    provenance stamped at bind; stranded-bound repair covering control and
    research-bound packets; management:* producer-report authority contract;
    channel_message wake retired; desk prompts updated to the record verbs;
    docs/current-architecture.md messaging section updated.
  acceptance:
    - action: >-
        On the owner staging computer, drive a texture follow-up ask
        (precommit addressed to a research work item) and observe the
        research report land: derived control packet wakes the research desk,
        the report record + derived producer_report packet wakes texture, and
        the ask's precommit resolves mechanically on arrival.
      proves: >-
        The record→packet→wake→consume chain works end to end on deployed
        staging for the exact direction that was broken (research→texture)
        and the owner's motivating case (texture→research follow-up).
      evidence_class: deployed proof
    - action: >-
        Kill or let expire a bound-but-unconsumed packet's run on a
        disposable staging computer; observe stranded-bound rebind and
        eventual consumption or exhaustion, with the record left open and
        aging (never a fabricated resolve).
      proves: >-
        The delivery-failure-degrades-to-scored-failure invariant holds
        under run death, including control-direction packets.
      evidence_class: deployed proof
    - action: >-
        On staging, assert channel mail addressed to texture:* rejects at
        reduce time (not a durable dead letter) and that no desk prompt
        instructs a retired verb (Message/Outcome/Spawn envelope paths,
        EscalateActions, controls-inside-ApplyTexture).
      proves: The raw-messaging authored surface is gone, not just bypassed.
      evidence_class: deployed proof + prompt audit
    - action: >-
        Regression suite: per-kind cutover tests (record minted +
        deterministic packet derived + no envelope) for note, report,
        resolve, disagreement, precommit+control, escalate, cast, retract;
        dual-delivery absence test (envelope XOR packet per kind).
      proves: Each kind's transport is exclusive and the record is the
        authored artifact.
      evidence_class: test + deployed smoke
  rollback: >-
    Per-kind cutover is atomic: revert the kind's commit; the record mints
    stay additive/append-only (safe to keep). channel_message wake
    restoration is a git revert + redeploy. If mid-cutover rollback is
    needed, drain or explicitly quarantine in-flight packets per
    docs/reports/agent-messaging-system-state-2026-10-01.md §12-E; rollback
    must preserve new records and pending packets — do not roll back to a
    binary that cannot read them.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between "a desk stakes a consequential ask/report" and
    "the addressee's desk provably received, consumed, and answered it,"
    while preserving: no authority widening, every committed act reversible,
    texture sole agent writer, and every desk act a scorable ledger event.
  goodharting_would_be: >-
    A "record-native" cutover that dual-delivers (envelope AND packet) or
    silently drops one; ask-scores that reward answer rate over materiality
    (trivially-satisfiable asks); a rejected-mail implementation that blocks
    legitimate producer reports; claiming the surface cut over while the
    channel_message wake still silently consumes dead letters.

homotopy:
  realism_axis: >-
    Increasing coverage of the record-native invariant on live staging:
    single-kind cutover (note, cheapest proof) → report (fixes the live bug)
    → resolve/disagreement → precommit+control arm → escalate/cast/retract →
    full surface with channel wake retired and issuer tallies in Pack.

boundaries:
  mutation_class: red
  phase_mutation_classes:
    RN0-repair: orange (dispatch widening + provenance stamping)
    RN1-prune: green/yellow (dead-code deletion, prompt narrowing)
    RN2-substrate: orange (IssueLifecycleControl extraction, selectors,
      management:* contract, stranded-bound repair)
    RN3-cutover: orange→red per kind (delivery-path cutover)
    RN4-wake-retire: red (protected surface: actor dispatch)
    RN5-pack-tallies: orange (issuer feedback surface)
  authority_sources:
    - owner direction 2026-10-01 (record-native, ApplyTexture edits only,
      precommit wakes its receiver, issuer-visible ask tallies)
    - owner-ratified choir-supervised-app-development-metamission-2026-10-01
    - AGENTS.md
    - docs/choir-doctrine.md (desk ontology, I26 score-boundary relaxation
      is owner-directed and must promote doctrine wording at cutover)
  must_preserve:
    - Texture is the sole agent writer of canonical revisions; owner
      AuthorUser writes untouched.
    - A committed addressed record always carries its delivery obligation;
      dual-delivery (envelope + packet) for one act is a defect.
    - The commitment ledger stays append-only; records never re-derive
      packets (packets own delivery state; records own obligation).
    - Scorer independence: issuer never resolves its own stake; target never
      resolves the issuer's stake; management is scorer of record.
    - directives are excluded from claim accrual/materiality aging.
    - Every cutover commit keeps tests deterministic and the staging
      computer serviceable.
  protected_surfaces:
    - Texture canonical writes (ApplyTextureTurn)
    - Lifecycle delivery path (QueueLifecycleUpdate, outbox, actor tape)
    - Actor dispatch (handler.go update-kind table)
    - Commitment ledger (AppendCommitmentRecord)
    - Desk prompt overlays (all four desks)
  heresy_delta:
    discovered: >-
      documented in docs/problems/texture-research-hollow-revisions-
      2026-10-01.md §residual defect + map doc §12 defect list (vacuous
      operational records, projection pollution, addressee overload,
      Emit-tray escape, cancel-never-closes). Discovery recorded; not counted
      as repair.
    introduced: none intended — the design removes transports rather than
      adding authority.
    repaired: research→texture return path; texture→desk authenticated
      control path.

now:
  status: working
  slice: >-
    RN0 landed earlier (research→texture dispatch + requested_by_* at bind);
    RN1 landed 7a14f1c8 (dead machinery prune). RN2 landed 7bbe9ec6 —
    IssueLifecycleControl extracted from ApplyTextureTurn via the shared
    lifecycleControlAccumulator, stranded-bound unbind covering control and
    producer-report packets at both reconcile entry points, and the
    management:* producer-report authority contract. RN3b landed 758325af —
    IntentReport routes through CommitLifecycleAct (record+producer_report
    packet atomically, no envelope), overlay verb signatures documented, and
    desk-name ToDesk resolution (texture→currentTextureAgentID(caller
    trajectory doc), management→persistent desk) repaired after staging
    probes exposed it.
  source_ref: main@758325af
  deploy_identity: 'staging https://choir.news deployed_commit=758325af; owner guest computer-03335285269bdba4f94377e56879f9e6 serving autoputer 758325af (verified 2026-10-02T10:45Z)'
  next_action: >-
    RN3b landed (758325af, deployed report acceptance proven: update_queued
    → update_delivered bound_to_activation on trajectory 33032604). Next:
    extend CommitLifecycleAct kind-by-kind — resolve, disagreement,
    precommit+control, escalate, cast, retract — each behind its own
    commit + landing receipt; then RN4 wake retire and RN5 pack tallies.
  candidate: null
  conjecture:
    id: record-native-coupling
    status: active
    claim: >-
      Making the commitment record the sole authored act with delivery as
      its derived projection eliminates the silent-loss failure class on
      desk couplings and converts desk behavior into scoreable, improvable
      signal — while per-kind atomic cutover keeps the live computer
      serviceable throughout.
    test: >-
      The staging acceptance chain above: an addressed precommit wakes
      research, its report resolves the ask mechanically, a killed bound
      packet leaves an open aging record (not silent loss), and no verb
      dual-delivers.
    edge: >-
      frame_lock/missing_oracle: if desks cannot express a needed act in the
      five kinds, the vocabulary is too small — new acts must be argued as
      directive subtypes or record kinds, not reintroduced envelopes.
    delta_o: >-
      The record→packet→wake→consume deployed proof is the observer upgrade:
      it makes coupling state directly inspectable where today only
      envelope-vs-packet divergence is visible postmortem.
    scope: >-
      The four desks' authored surface; fate machinery (Complete/Freeze/
      Verify) and deadline wakes stay out of scope by design.
  decision: >-
    Owner rulings 2026-10-01 recorded in map §10-11: records are the acts;
    Addressee is delivery; ApplyTexture edits only; issuer tallies enter the
    acting pack (aggregate, doctrine wording promotes at cutover).
  belief: >-
    The dispatch-predicate gap (not the queue internals) is the live defect;
    the repair is small and uses existing machinery.
  blocker: null


receipts:

  - id: rn3b-landing-2026-10-02
    kind: slice_landing
    status: closed
    closed_at: '2026-10-02T10:50:00Z'
    boundary: >-
      RN3b record-native producer_report cutover landed on staging:
      IntentReport commits via CommitLifecycleAct (record + producer_report
      packet atomically, no envelope) with desk-name ToDesk resolution.
    pushed_commits: [d4d10e27, 29a8bd95, 267998a6, 758325af]
    ci: >-
      run 36981204683 success (d4d10e27); run 36986554521 success on rerun
      after a transient Nix builder crash (267998a6); run 36993877994
      success (758325af).
    deploy: >-
      Deploy to Staging (Node B) success on each push; owner computer
      refreshed owner-scoped to 758325af (fleet refresh preserves
      constructed-computer-version by design).
    environment_identity: >-
      https://choir.news/health deployed_commit=758325af; owner guest
      computer-03335285269bdba4f94377e56879f9e6 serving autoputer 758325af.
    deployed_acceptance: >-
      Prompt-bar research task on trajectory
      33032604-f085-57b4-ad7d-379095266226 minted record-native
      update_queued cell:…:report:tray-1:packet (no envelope), woke the
      bound texture desk, and update_delivered bound_to_activation.
      Evidence: docs/evidence/s0m-rn3b-report-acceptance-2026-10-02.json
      (probe scripts/s0m_report_acceptance_probe.mjs; probe's own 25-min
      window closed between queue and deliver — the trajectory event chain
      is the oracle).
    heresy_delta: >-
      discovered: desk-name ToDesk was unreachable for lifecycle reports
      (docs/problems/s0m-report-desk-target-resolution-2026-10-02.md,
      repaired 758325af); research overlay lacked record-native verb
      signatures (docs/problems/s0m-report-verb-signature-undocumented-
      2026-10-02.md, repaired 267998a6). Residual: engineering-desk
      directives mint but never bind (s0m-directive-engineering-desk).
    rollback: git revert 758325af 267998a6 d4d10e27 (problem docs stand).

  - id: rn3a-landing-2026-10-02
    kind: slice_landing
    status: closed
    closed_at: '2026-10-02T07:15:00Z'
    boundary: RN3a record-native directive mint + note cutover landed
    pushed_commits: [ed9fa123, 87549c17]
    ci: 'run 36973968895 — success (all shards, SBOM, Deploy to Staging)'
    deploy: 'Deploy to Staging (Node B) success; vmctl restarted, active VMs refreshed'
    environment_identity: >-
      https://choir.news/health deployed_commit=87549c17; owner guest
      computer-03335285269bdba4f94377e56879f9e6 active epoch 1003 serving
      autoputer commit 87549c17.
    deployed_acceptance: >-
      Owner-computer prompt-bar run staged choir.Note("management",
      marker); CommitLifecycleAct minted record + directive packet
      cell:572f074b-…:note:tray-1:packet (record-derived UpdateID, no
      envelope), directive wake minted persistent-Management run
      06781638-cda3-4858-9c6e-609f69bd0822 with the packet in
      metadata.worker_update_ids — record→packet→wake→injection proven
      end-to-end on staging. Evidence:
      docs/evidence/s0m-rn3a-note-acceptance-2026-10-02.json
      (probe scripts/s0m_note_acceptance_probe.mjs).
    heresy_delta: >-
      discovered: engineering-desk directives mint but never bind
      (docs/problems/s0m-directive-engineering-desk-2026-10-02.md) —
      residual for RN3b, not a RN3a regression.
    rollback: git revert 87549c17 (docs commit ed9fa123 stands).
  - id: rn2-landing-2026-10-02
    kind: slice_landing
    status: closed
    closed_at: '2026-10-02T05:20:00Z'
    boundary: RN2 substrate landed on staging
    pushed_commits: [1f7d0889, 3b3ce3e2, 7bbe9ec6]
    ci: 'run 36966746881 — success (all shards, vocab gates, heresy detector)'
    deploy: 'Deploy to Staging (Node B) success'
    environment_identity: >-
      https://choir.news/health deployed_commit=7f716ca1 (includes
      3b3ce3e2+7bbe9ec6); corpusd started 2026-10-02T05:13:00Z; zero
      post-deploy errors; 156-computer vmctl inventory healthy.
    deployed_acceptance: >-
      RN2 is internal substrate — no user-visible acceptance act in this
      slice; per-kind deployed acceptance lands with RN3. Local proof:
      internal/store lifecycle suite + internal/agentcore stranded-control /
      persistent-Management / scheduling-readiness tests green; the 7 tests
      that exposed the wake-drift defect now pass with 3b3ce3e2.
    heresy_delta: >-
      discovered+repaired: actor-wake-outbox created_at drift
      (docs/problems/actor-wake-outbox-createdat-drift-2026-10-02.md), fixed
      in 3b3ce3e2.
    rollback: git revert 3b3ce3e2 7bbe9ec6 (docs commit 1f7d0889 stands).
  - id: rn1-landing-2026-10-01
    kind: slice_landing
    status: closed
    boundary: RN0 (fc82e3be) + RN1 (7a14f1c8) landed; CI run 36963159655
      success; staging deployed fd8b2973→7a14f1c8 range on 2026-10-02.
