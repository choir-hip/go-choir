---
definition_version: 4

# Cut 1, authority. Station C1 of
# definitions/choir-reeng-metamission-2026-10-12.md; depends on Cut 0
# (definitions/choir-reeng-cut0-eyes-2026-10-12.md). Owner-agreed design:
# docs/re-engineering-decisions-2026-10-12.md §2 (conversations, rounds,
# hypotheses, cursors), §9 (routing), §10-§12 (radical cutover). Those
# decisions win over everything below. Code-cited findings carried from the
# superseded drafts R1-R4 (03499ef9) and studies B, C, G; every cite below
# was re-read at 03499ef9 (no Go file changed since a401c878). Their gradual
# slicing (shadow fold, dual-write, interim closer, standing anchor) is
# rejected (decisions §12).
readiness: reviewed

review:
  reviewer: convergent agentic-consensus panel (codex, claude opus, gpt-6.1-sol, gemini-3.8-flash, deepseek-v4.1-flash, glm-5.3-flash; gpt-6-luna and muse-spark timed out)
  frozen_ref: 2d2b864e
  verdict: send_back (codex, gpt-6.1-sol, gemini, deepseek), accept with amendments (claude, glm); repaired in the following commit, re-review pending
  evidence_ref: docs/re-engineering-decisions-2026-10-12.md §13 (synthesis)
  rereview: claude opus and gemini-3.8-flash at deb6f760 (codex and gpt-6.1-sol hit a usage limit); accept (claude, gemini); baseline wording and D1-D9 amendments applied

start:
  captured_at: 2026-10-12T19:30:00Z
  source:
    canonical_ref: main 03499ef9
    deploy_identity: staging choir.news b172895b
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty (this session's doc edits in progress; decisions doc and metamission)
      correction_2026-10-12: those edits landed at 2d2b864e and the tree was clean at review; Go source is unchanged between 03499ef9 and 2d2b864e
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Two mutable layers sit beside the append-only records. Work items
      (objectgraph choir.work_item; 62 non-test Go files, 1,090 lines match
      WorkItemRecord|WorkItemID|work_item|WorkItem) do six jobs (study B §1).
      Packets (choir.worker_update, types.CoagentSourcePacket at
      internal/types/evidence.go:178-240; 40 non-test files, 343 lines,
      corrected 2026-10-12 from a miscount) carry
      delivery status, claims (DeliveredToRunID, DeliveryAttempts), work
      bindings and report content.
    - >-
      Records cannot carry work yet. Kinds precommit, report, resolve,
      disagreement, directive (internal/types/commitment.go:41-47); directive
      subtypes note, escalate, cast, retract (:55-60); emitter kinds agent,
      owner, edition, source, no system (:262-265). No conversation id,
      round, final flag, hypothesis id or inbox sequence. Desk-authored
      records set no EmitterKind or SubjectRefs
      (internal/agentcore/rlm_reduce.go:1080-1205); a Report's content is the
      raw packet JSON stuffed into Prediction.Hypothesis (:1157).
      CommitmentPrediction and TypedQuestion exist with no id, stakes or
      crux (commitment.go:74-122). Only run_id, trajectory_id and
      channel_id are indexed columns (internal/objectgraph/dolt_store.go:155-162);
      every Report commit lists up to 10,000 records
      (internal/store/lifecycle_commit_act.go:144).
    - >-
      Delivery today: CommitLifecycleAct mints record + packet
      (UpdateID = recordID+":packet") numbered from the caller's trajectory
      ReducerSeq (lifecycle_commit_act.go:298, :365-377); the actor wake is
      derived from the pending packet (internal/store/lifecycle.go:548);
      Texture consumes packets with seq <= the run's scheduled_message_seq
      (internal/textureowner/texture_turn_runtime.go:85-152;
      WithinTextureTurnBoundary, evidence.go:166-176). A Texture revision run
      binds the first open Texture work item and refuses without one
      (internal/textureowner/texture_agent_revision.go:502-509);
      ResolveLifecyclePacketWorkBindings errors on directive packets
      (internal/store/lifecycle_texture_target.go:244-275); this is P2d2's
      blocker. RLM cells commit one act per transaction and keep a legacy
      envelope-mailbox cursor in run memory written after the acts
      (rlm_reduce.go:364-494, :766-830).
    - >-
      Boot violates the restart rule both ways
      (docs/problems/crash-restart-leaves-work-open-and-recasts-engineering-2026-10-12.md).
      A markerless boot refuses only pre-boot actor wakes
      (internal/actorruntime/handler.go:105-126, :168-172). Boot phases
      (internal/agentcore/runtime.go:748-761): passivate runs as
      runtime_restarted and ensure spawned work items (:2382-2425, :2405);
      engineering capsule sweep cancels and wakes the parent
      (engineering_assignment_fate.go:530-573); wire-publication claim
      recovery cancels trajectories every boot (wire_publication.go:21-43);
      terminal settlement repair (runtime.go:2478-2551); then
      platform_update_resume, selfdev_close_after_crash,
      source_schedules_close_after_crash and
      selfdev_materialization_reconcile (runtime.go:750-761), which this cut
      keeps (prose, "Restart"). The Texture boot
      calls ReconcileEngineeringDesk (texture_controller.go:110-125), whose
      recast re-opens a restart-cancelled assignment up to 3 times with no
      marker check (engineering_desk.go:44-50, :197-279). The Texture wake
      re-arms stranded packets and open work and reactivates passivated
      runs (texture_controller.go:615, :638-660, :694).
    - >-
      Routing. Path B: a document cast's engineering report is retargeted to
      texture:<doc> and a supervision work item minted
      (internal/store/engineering_assignments.go:1747-1757, :1770-1823, call
      sites :2258, :2393, :2722), with Texture-side exclusions
      (texture_controller.go:71, :593-607, :642; texture_agent_revision.go:238-241,
      :283; store/lifecycle.go:858-866) and entries (selfdev_texture_join.go,
      123 lines; texture_lifecycle_create.go:84-120; handler.go:602-629).
      Engineering's module set exports Assign, Message, Emit, Cast and Ask
      (internal/yaegikernel/choir.go:249-251); Assign is an in-memory stub
      (broker.go:356-377). Management's report target is "the trajectory of
      the most recent delivered control" (tools_engineering_assignment.go:123-169).
      A runtime Jev judge runs on the path-B reconcile (engineering_desk.go:405;
      selfdev_candidate_judgment.go, 218 lines). Texture may send one
      management Cast per turn (texture_dispatch_fold.go:53-57).
    - >-
      Research leftovers: round cap 4 (texture_turn_runtime.go:161, :187-206,
      :295-305, :405); egress budget 64 calls / 32 MiB
      (researchtools/egress_budget.go:46-47, installed tool_profiles.go:476-481);
      Texture and engineering tool-loop budgets (runtime.go:3503-3506,
      :4105-4152); forced status report in the wake text (runtime.go:2953-2999);
      update_coagent next_instruction on every search and fetch result
      (researchtools.go:717-738, :787, :922-924, :995, :1023; update_coagent
      named in 31 non-test Go files); research reports hard-set open
      (rlm_reduce.go:2099). choir.Research blocks on all children
      (research_children.go:151-167), a 10-minute window and 0.75 share
      (:44-46, :238), children get the objective plus a child preamble and
      role prompt (:201, :213) and emit no events (:210-215).
    - >-
      Autonomous wire publication is a second work-item consumer: every
      eligible Texture revision commit (tools_texture.go:445) calls
      MaybeAutonomousPublishWireArticle (wire_publication.go:45), which opens
      a wire_publication work item and publishes to Universal Wire
      (internal/wirepublish, internal/proxy/wire_platform_publish.go);
      wire-processor decision work items are minted beside it
      (wire_publication.go:149-260).
    - >-
      Self-development binds to operation state: Freeze looks up the
      operation by the run's trajectory and requires StateExecuting, opening
      one lazily through OpenFreezeCandidate when none exists
      (tools_capsule.go:251-270; rlm_reduce.go:1335-1344); Verify binds
      through the same lookup and checks the bundle digest
      (rlm_reduce.go:1388-1405); the materializer acts on operation states
      (self_development_materializer.go:128-165) and already tells
      management a verified operation awaits decision with a record of
      deterministic id (:858-883). The marker is
      PlannedRestart{Reason, Target, RequestedAt} (planned_restart.go:25-29),
      with no epoch.
    - >-
      Prompts: hand-written role and overlay files
      (internal/promptstore/defaults/{management,engineering,research}.yaml,
      internal/runtimeprompts/overlays/rlm_*_runtime.yaml,
      internal/textureprompts/) duplicate the generated surface
      (deskSurfacePrompt, tool_profiles.go:569; DeskSurface, surface.go:211);
      the only parity test checks one direction
      (choir_parity_test.go:310). Study G lists 40+ mismatches.
    - >-
      Run acceptance reads runs and trace events, not work items
      (internal/agentcore/run_acceptance.go:61-211); it has a path-B branch
      (:171-188) and selfdev-operation checkpoints (:650-686).
    - >-
      Settlement: documents settle only with no open work and no pending
      packet (store/lifecycle.go:2730; RequireNoOpenWorkItems at
      texture_lifecycle_create.go:97, texture_handoff.go:254,
      coagent_route.go:218, selfdev_texture_join.go:73,
      engineering_assignment_seed.go:36).

finish:
  deliver: >-
    Work is records. Each desk talks to another in one conversation per
    (sender, recipient, subject); the sender adds hypotheses and steering,
    the recipient closes each round with a final Report that says what it
    settled, dropped and why it stopped. Desks read their inbox by cursor,
    so a Texture takes every management and research record waiting for it
    in one turn. A crash closes open rounds and starts nothing; a planned
    update restart resumes once. Engineering reaches Texture only through
    management. All four desks are told exactly what they can do.
  artifact: >-
    One coherent cutover, landed commit by commit, each through deploy
    and a fast tracer run (owner pace ruling 2026-10-13, metamission
    boundaries.pace 5; inventory first, then store, then desks, then
    prompts). A commit may leave a test computer worse at a step the cut
    has not reached yet; the tracer says which, and nothing is preserved.
    (0) Baseline: the run-acceptance level today's system reaches on the
    tracer story, recorded by Cut 0 (its artifact 7), and an absolute
    target for this cut (the engineering round trip reaches capsule
    acceptance with freeze, independent verification, owner decision,
    apply and restore).
    (1) Records: conversation_id and round stamped by the store; Cast
    hypotheses with store-assigned ids (H1, H2, ...), probability, stakes
    and crux, open uncertainties with stakes, general hypotheses (seed name,
    probability, operational meaning of the key word), revise and withdraw
    by id; a typed Report body (claims with sources, findings tagged
    bears_on <id> or outside, settled[{id, outcome}], dropped[{id, reason}],
    stop_reason, final); a system emitter kind and a closure record with a
    deterministic id; subject, EmitterKind and ComputerID on desk-authored
    records; owner revisions recorded as owner-emitted records that point
    at the revision, committed with it; per-inbox sequence numbers;
    indexed columns for inbox,
    inbox_seq, conversation_id, round and record kind. The store validates
    well-formedness only and scores nothing.
    (2) Delivery by cursor (decisions below): inbox heads, reader cursors
    (a map from inbox to sequence) on agent records, wakes derived from
    record commits, Texture's turn commit and each RLM cell commit
    advancing the cursor in the same transaction as the work; activation
    separated from eligibility (prose, "Cursor and wakes").
    (3) Deleted with every reader (inventory in the prose): work items,
    packets, delivery claims and dispositions, stranded repair, Texture
    breaker, stale-packet expiry, the envelope mailbox and its cursor,
    owner-revise reopen and the 409 class, document settlement, the one
    management Cast per turn rule, and the autonomous wire-publication path
    (MaybeAutonomousPublishWireArticle and its call site, the
    wire_publication and wire-processor work-item kinds, their boot
    recovery; Cut 2's per-version publication replaces it). Pre-cut
    publications need not keep resolving (decisions §13.1).
    (4) Boot: a markerless boot terminalizes pre-boot runs and engineering
    assignments with fate interrupted_by_restart, appends one closure record
    per open round, and dispatches nothing. Deleted: boot passivation and
    passivated-run resume, spawned-work replay, terminal settlement repair,
    Texture wake re-arm, wire-publication claim recovery, the engineering
    restart recast. Kept: platform_update_resume,
    selfdev_close_after_crash, source_schedules_close_after_crash (a
    markerless boot closes active schedules and mints no wake; a planned
    boot keeps the pending wake) and selfdev_materialization_reconcile.
    The marker gains a work_state_epoch field; a marker of the booting
    binary's epoch resumes once.
    (5) Routing: path B, Assign, Message, Emit, Emits deleted; engineering's
    module set loses Cast and keeps Ask and Reply, its addressee forced to
    its conversation's sender (management), the reducer refusing any
    engineering act addressed to texture:*; management relays an
    engineering Report to Texture by forward (pass-through) or its own
    Report; the runtime Jev judge and self-development work-state
    scaffolding removed (public start-operation API, one-open lock,
    auto-advance and auto-fail from engineering fates, prompt
    special-casing). The self-development operation stays as the effect
    journal of one candidate bundle (prose, "Self-development"): Freeze,
    Verify, owner decision, updater, materializer, marker, platform
    update, route projection and restore keep working, re-bound to
    records where they bound to work items or packets.
    (6) Research: round cap, egress budget, Texture and engineering
    tool-loop budgets, forced status report, next_instruction and every
    update_coagent name deleted; research reports carry final. Sub-researchers
    are runs with events, their own short prompt and read-only surface, and
    report to the parent research inbox through records; choir.Research
    returns at once; the all-children barrier, window, share and noEmit go.
    (6b) One child primitive. spawn_agent and cancel_agent are deleted
    (internal/coagentowner/spawn_tool.go; registered only for the
    conductor, autoputer/run.go: RegisterSpawnTool returns nil for
    engineering, whose policy has no spawn targets; dead-code census
    desk-config.md 1.5). With them go AllowedSpawnTargets, CanSpawn, the
    allow_coagent_tools flag, the cancel_agent registration and the
    conductor spawn_agent dedupe in toolregistry/batch_executor.go.
    A desk's children are sub-RLMs on records. The conductor's one live
    use, opening a Texture from the prompt bar (conductor.yaml:14-16,
    conductor_run.yaml:9-13), becomes an owner-emitted Cast to a new
    texture:<doc>.
    (6c) Deletions the dead-code census found for this cut
    (docs/reports/dead-code-census-2026-10-12.md Part C, B14): the 17
    legacy relational tables in internal/store with no production writer
    (agents, runs, channel_messages, inbox_deliveries, run_acceptances,
    run_continuations, trajectories, work_items, browser_sessions,
    worker_updates, coagent_mailboxes, desktop_state,
    computer_effective_state, texture_documents,
    texture_controller_checkpoints, content_items, podcast_subscriptions),
    their legacy readers, and their replay_eligibility.go entries (retire
    them as absent, the b85af274 path); POST /api/texture/management-open
    and GET /internal/runtime/control-records, which serve the control
    accumulator this cut replaces with Casts; the processor and reconciler
    metadata keys in durableMetadataKeys (runtime.go), api.go and
    texture_task_types.go, and the handoff metadata copy in
    textureowner/coagent_route.go. Census defect D3 (research can Cast)
    is fixed here by gating Cast on the casting desk.
    (7) Prompts: four desk prompts written once for this system (role and
    judgment by hand; verb list generated from the module set; general
    hypotheses offered as a space to sample from); wake turns carry
    pointers only; a reverse parity test (every export taught or listed,
    every taught verb and addressing rule accepted by the reducer).
    (8) Run acceptance re-expressed on the fold (direction opened = first
    Cast from the trajectory's Texture; capsule checkpoints from Freeze and
    Verify records; path-B and operation branches deleted).
    (9) docs/problems/reeng-cut1-breakage-<date>.md, one per cut window.
  acceptance:
    - action: >-
        Cut 0 tracer AI news run on a freshly minted staging test computer,
        then the invariant checker extended with Cut 1's invariants (prose)
      proves: the cutover runs end to end with gapless inbox sequences, at most one open round per conversation, cursors never past head, no engineering record addressed to texture:*
      evidence_class: deployed proof (exported trace + checker JSON)
    - action: >-
        Two or more management records and two or more research Reports
        committed past Texture's cursor before its turn; one turn
      proves: Texture reads every record in (cursor, head] in one turn, its prompt lists each, and the revision commit advances the cursor to head atomically
      evidence_class: deployed proof (turn receipt range joined to records and revision)
    - action: >-
        Texture -> research conversation over two rounds: a Cast with H1-H3
        and one general hypothesis; research Reports tagged by id; final
        Report settles H1 and drops H2 with a reason; Texture's next Cast
        revises H3 and adds H4 in the same conversation; a second final Report
      proves: conversation identity, rounds, hypothesis ids and states are a fold over records only
      evidence_class: deployed proof (fold query through the product API)
    - action: >-
        Owner asks a Texture for a code change: Texture -> management ->
        engineering (capsule) -> management -> Texture, then a Texture revision
      proves: engineering reaches Texture only through management; zero engineering records addressed to texture:*
      evidence_class: deployed proof
    - action: >-
        Crash drill: open rounds in every direction plus a running Texture
        turn, then the owner restart route (markerless); 10-minute quiet
        window; an owner revise; a second markerless boot
      proves: every open round gets exactly one closure record, every pre-boot run is terminal with the fate, no run starts before the owner acts, the revise starts exactly one Texture turn that sees the closures, the second boot writes nothing
      evidence_class: deployed proof (Cut 0 crash drill receipt, before/after counts with ids)
    - action: planned restart (platform update with marker, same epoch) on a test computer with open rounds, then a markerless boot
      proves: no closure records, each desk with an open round wakes once, the following crash boot closes them
      evidence_class: deployed proof
    - action: research spawns two or more sub-researchers in the tracer run
      proves: child runs leave events, report only to the parent inbox, and the parent cell does not block on them
      evidence_class: deployed proof
    - action: reverse parity test (fails on a planted stale verb, shown once) and a grep gate on WorkItem|work_item, CoagentSourcePacket, update_coagent, Assign, choir.Message, choir.Emit, textureSupervision, MaybeAutonomousPublishWireArticle, wire_publication, round cap and budget names
      proves: the deleted names have no non-test citer; old objects stay in og_objects as untyped history and no code decodes them
      evidence_class: CI
    - action: run acceptance synthesized for the engineering round-trip trajectory
      proves: acceptance derives from the fold; the tracer story reaches at least Cut 0's recorded level, and the engineering round trip reaches the absolute target in artifact 0
      evidence_class: deployed proof
    - action: >-
        Self-development on a fresh test computer: management Casts
        engineering; engineering Freezes; management Verifies against the
        bundle digest named in engineering's final Report; the owner
        approves; the materializer applies (planned restart with marker);
        then a restore to the pre-apply head
      proves: Freeze, Verify, decision, apply and restore work with work items and packets deleted; the apply restart resumes once and the restore lands on the prior head
      evidence_class: deployed proof (operation states, route receipt and event heads through the product API)
    - action: >-
        Late arrival: while a Texture turn runs, management commits a
        record to it; then, separately, a turn that fails with a record
        arriving after it started
      proves: the record that arrived mid-turn starts exactly one more turn after the commit; the failed turn's own records stay unread with no new turn; the record that arrived after the failed turn began still starts a turn
      evidence_class: deployed proof
    - action: planned restart with an open round whose records are all read (head equals cursor)
      proves: the planned-resume wake activates the recipient once, and the following markerless boot closes the round
      evidence_class: deployed proof
  rollback: >-
    Forward-fix (decisions §12.3). Before the cutover push, tag the Cut 0
    landed commit reeng-cut1-last-good. Staging test computers are
    disposable and are re-minted, never reverted: old code cannot read
    new-format records. The owner's computer takes the cut at its next
    boot like any other (decisions §13.1); no state is snapshotted, since
    none is valuable before Autopaper is live. Break-glass: redeploy the
    tag and mint fresh test computers.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance, consensus_review]

value:
  better_means: >-
    Minimize work-state facts held anywhere but the append-only record tape
    (status columns, packet dispositions, delivery claims, boot-time
    re-dispatchers, prompt text that contradicts the surface) toward zero,
    while preserving Texture as sole writer, the restart rule, engineering
    never addressing Texture, and one authority per fact.
  goodharting_would_be: >-
    Work items and packets renamed while their jobs move to a head row,
    cursor table or run metadata that becomes the new authority; a tracer
    that passes the happy path while rounds stay open; crash closure proven
    by a simulated boot; prompts that pass parity by deleting what desks
    need; hypothesis ids filled with boilerplate nobody reads.

homotopy:
  realism_axis: >-
    store fold and cursor tests (shape only) -> one fresh staging test
    computer on the tracer path -> crash drill and planned restart on fresh
    test computers -> two-round research conversation and the engineering
    round trip -> repeated tracer runs with the checker clean -> Cut 2
    building Autopaper on these records

boundaries:
  mutation_class: red
  authority_sources:
    - docs/re-engineering-decisions-2026-10-12.md §2, §9, §10, §12 (owner 2026-10-12, including the delegation in §12)
    - CLAUDE.md "Restarts End Work (Crash) Or Resume It (Planned Update)"
    - docs/definitions/choir-reeng-metamission-2026-10-12.md (C1 depends on C0)
  must_preserve:
    - only the Texture desk writes a Texture
    - a crash restart never resumes work; a planned restart resumes once
    - engineering never addresses Texture; capsule state stays on EngineeringAssignment
    - research has read-only world authority; one fan-out level
    - no budgets, round caps, reminders, runtime scoring or circuit breakers are added
    - substrate stays (decisions §1.2) - child concurrency 4 and the 1.5 s spacer, tray quotas, poison-free wakes, the 60-minute run limit as a labelled prototyping constraint
  excluded:
    - async search, the research desk's new verbs, Texture's write verb, captures, permalinks, Autopaper (Cut 2)
    - trace export, tracer, invariant checker, fresh computers, request-id fix, one REPL owner per desk, broker-handle fix, baselines (Cut 0)
    - shadow folds, dual-writes, interim closers, standing anchors, one-release keeps, feature flags
    - migrating old execution state (archived as history, never read)
    - Jev as a support judge (Cut 2, decisions §12.7); signing keys
  protected_surfaces:
    - evidence ledger - internal/types/commitment.go, internal/store/commitment.go, internal/store/lifecycle_commit_act.go, internal/objectgraph/dolt_store.go
    - lifecycle reducer and replay - internal/store/lifecycle*.go, internal/types/lifecycle.go, internal/types/evidence.go
    - Texture canonical writes and activation - internal/store/texture_turn.go, internal/textureowner/texture_controller.go, texture_turn_runtime.go, texture_agent_revision.go
    - boot and restart semantics - internal/agentcore/runtime.go Start, planned_restart.go, internal/actorruntime/handler.go, internal/autoputer/run.go
    - engineering capsule authority - engineering_assignment_runtime.go, engineering_assignment_fate.go, engineering_desk.go, internal/store/engineering_assignments.go
    - run acceptance - internal/agentcore/run_acceptance.go, internal/store/run_acceptance.go
    - desk act path, module sets and prompts - rlm_reduce.go, internal/yaegikernel, research_children.go, prompt files
  admissible_evidence: >-
    Deployed proof on freshly minted choir.news test computers through the
    product API and Cut 0's trace export, tracer, invariant checker and
    crash drill. Store and reducer tests written before the code shape the
    work and settle nothing. No local claim about wakes, boots or live desks.
  conjecture_delta_note: the self-development operation stays as effect journal, not work state (prose)
  conjecture_delta: >-
    Architectural mode change: the authority for open work, activation,
    delivery and completion moves from work items and packets to a fold
    over immutable records plus one cursor per reader. Supersedes R3's RC-1
    (staged fold) and R2's R2-1 (interim closer); P2d's shape D is not built.
  heresy_delta:
    discovered: >-
      (already documented, not counted as repair) crash leaves work open and
      recasts engineering; work items never crash-close; wire recovery
      cancels trajectories at boot; path B lets engineering reach Texture;
      management's report target is a heuristic. New in this read: report
      content lives as raw packet JSON in Prediction.Hypothesis
      (rlm_reduce.go:1157); the envelope-mailbox cursor is written after the
      acts, not with them (rlm_reduce.go:481-494, :766-809)
    introduced: none intended; two cache objects (inbox head, conversation head) written only by the record commit and rebuilt by the checker (glm suggests deriving the next sequence by max(seq) in the commit instead; measure on the tracer and drop the head cache if the query is fast enough)
    repaired: none yet; the documented list is the target

now:
  status: core landed; C1e pending
  slice: >-
    C1-core landed and accepted on staging 2026-10-13 at 76ab40fd. L1
    crash closure (c1-l1-crash-2; again c1-core-crash-2: the in-flight run
    closed interrupted_by_restart, I3 pass on a 150 s quiet window). L2
    deletions. L3a Texture-research and L3b Texture-management on records
    (c1-l3a-story-1; c1-core-mgmt-1: Cast, management's final Report, a
    committed Texture turn reading it, about 3 minutes, I6 and I7 pass).
    Texture turns on records need no work item; the planned-restart
    marker's work_state_epoch (D7); late arrival; the routing refusal of
    engineering acts to Texture (reducer test); I6 and I7 in the checker;
    L6 surfaces and prompts teach only callable verbs, with the parity test
    over Texture, research and management prompts. Breakages B1-B4 fixed
    (docs/problems/reeng-cut1-breakage-2026-10-13.md); B4, a records wake
    reactivating a parked Texture run that nobody ran, was found by the
    field run c1-l3b-mgmt-1. Evidence:
    docs/evidence/c1-core-mgmt-acceptance-2026-10-13.json,
    docs/evidence/c1-core-crash-acceptance-2026-10-13.json,
    docs/evidence/c1-l3b-mgmt-field-2026-10-13.json. C1e items remain
    (scope_split below).
  source_ref: 76ab40fd
  deploy_identity: staging 76ab40fd
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: C1-1
    claim: >-
      If work state is only records (conversations, rounds, hypotheses,
      inbox cursors) and boot does nothing but append closure records, then
      the work-closure bug class and crash re-dispatch cannot recur,
      management can feed any Texture, and Cut 2 builds Autopaper without
      touching work state.
    test: the acceptance list, a clean tracer run after the last landing, and no new work-state entry in the breakage log after its last fix (owner pace ruling 2026-10-13: one fast run per landing, not a series)
    edge: frame_lock
    delta_o: >-
      the checker states failures only in rounds and cursors; a desk that
      reads the right records and answers in the wrong conversation passes.
      Add the tracer's per-conversation transcript, sender view beside
      recipient view, read by someone other than the builder
    scope_if_supported: the four root desks and sub-researchers on staging test computers
    status: proposed
    evidence_refs: [docs/reports/re-engineering-studies-2026-10-12/B-work-items-as-records.md, docs/problems/clustering-texture-obligation-closure-2026-10-09.md]
  decision:
    what: >-
      Delivery and identity, chosen for review (prose "Decisions"). D1 an
      inbox is (desk, subject); its sequence comes from an inbox head
      advanced in the record's commit; an owner revision commits an
      owner-emitted record pointing at it, in the same transaction, which
      takes the next number in Texture's inbox; the text stays on the
      revision chain. D2 conversation id
      is derived from (sender inbox, recipient inbox); round from a
      conversation head in the same commit. D3 a reader's cursor lives on
      its agent record; Texture's turn commit and each RLM cell commit (one
      transaction for all its acts) advance it from the value the work saw.
      D4 every addressed commit mints one wake row for the recipient
      inbox; a turn spends the wake rows inside its range; rows past its
      range fire when the desk is idle; after a committed turn the
      controller re-checks head > cursor; a failed turn re-arms nothing;
      boot mints wakes only under a marker. D5
      the closure record is a system Report, final, reason
      interrupted_by_restart, addressed to the sender, id
      <conversation_id>:r<round>:restart, never waking anyone. D6 the store
      assigns hypothesis ids. D7 a marker resumes only within the same
      work-state schema epoch; Cut 1 bumps the epoch, so the cutover boot is
      a crash boot. D8 a planned resume mints one wake per desk with an
      open round as recipient or unread records, which activates it even
      when head equals cursor; runs are never resumed. D9 Freeze binds to
      the engineering run's capsule as today; Verify binds to the
      operation id and bundle digest named in the engineering Report it
      answers.
    kind: architecture
    status: proposal
    evidence_ref: docs/re-engineering-decisions-2026-10-12.md §2, §12
    owner_ratification_ref: decisions §2 and §12 ratify the direction and delegate the mechanism (§12, "I trust your judgement"); D1-D9 are this file's proposal
  belief:
    believed_state: the code claims above hold except where L1 changed boot (runtime.go Start, closePreBootRunsAfterCrash); Cut 0 landed (docs/evidence/c0-eyes-acceptance-2026-10-13.json)
    main_uncertainty: whether one cutover this size converges inside the redesign tripwire
    next_observation: Cut 0's measurements (push-to-healthy-computer time, baselines), then the deletion inventory re-grepped on Cut 0's landed commit
  blocker_or_risk: >-
    Depends on Cut 0 (tracer, checker, crash drill, fresh computers,
    request-id fix, one REPL owner per desk, broker-handle fix). Red
    throughout.
  scope_split: >-
    Orchestrator decision 2026-10-13 under No Blocking Asks, reviewed by a
    convergent panel (claude opus and deepseek-v4.1-flash answered; codex
    and the two GPT anchors hit usage limits;
    .agentic-consensus/c1-scope-2026-10-13, receipt in
    docs/reports/reeng-plan-review-2026-10-13.md). C1 closes on its core,
    the conversations Autopaper uses, and the engineering and
    self-development items move to station C1e, run after C2's
    autopaper_live and before the completion review. Reason: engineering
    assignments are bound to work items throughout
    store/engineering_assignments.go, Autopaper never casts engineering,
    and moving them first would hold C2 most of a night. C1-core
    acceptance: records as the only work state on Texture-research and
    Texture-management (deployed, L3a, L3b); a Texture turn that starts and
    commits from records with zero open work items; cursors, wakes and the
    post-turn re-check; crash closure of runs and rounds with a no-op second
    boot; the planned-restart marker's work_state_epoch (D7); the routing
    refusal of engineering acts to texture:* (reducer test); the checker on
    records (I6 as cursor never past head and heads equal their rebuild,
    at most one open round per conversation, only a final Report or system
    closure closes, and no packet or work item minted on a records route);
    late arrival; prompts and generated verb lists with the reverse parity
    test for Texture, research and management. C1e: engineering on records
    (capsule Records, records-authorized assignments, engineering prompt),
    path B deleted, management Verify, the self-development round trip on
    records, sub-researchers as runs (L5; the all-children barrier stays on
    C2's research path meanwhile, a named risk), run acceptance on the fold
    (L7), and the wholesale deletion of packets and work items with its
    grep gate. Cost until C1e: management refuses engineering casts on
    records turns with an explanation, so execution requests come back
    blocked on staging and on the owner's computer.
  next_action: >-
    Landing order, revised by the plan review of 2026-10-13
    (docs/reports/reeng-plan-review-2026-10-13.md). Each push that changes
    behavior goes through deploy and one claim-directed tracer scenario
    under the identity fence (scripts/reeng_tracer.mjs --scenario,
    --expect-sha). Pushes are batched by dependency, as separate commits.
    L1 restart (landed 6d676094, B1; B2 fixed in 877689e1): a markerless
    boot cancels every pre-boot pending, running, parked or blocked run with
    fate interrupted_by_restart, opens no work and wakes no one;
    Engineering cancel wakes and the boot recast are suppressed on a crash
    boot; the terminal-outcome repair and wire-publication claim recovery
    phases are deleted. Planned boots keep park-and-resume (the one resume
    a planned restart allows) until L3 re-derives it from records and
    wakes (D8); passivation itself stays for that path.
    Accepted 2026-10-13 on 50ed3337 by --scenario crash
    (docs/evidence/c1-l1-crash-acceptance-2026-10-13.json): the run open at
    the crash closed with the fate, I3 pass on a 150 s quiet window, 4m54s
    end to end. Field run c1-l1-1
    (6d676094): the in-flight run closed with the fate, nothing moved until
    the owner asked again, I3 had no violation (two pre-boot packets
    redelivered after owner input, the residual L3 removes).
    L2 deletions (6b, 6c, research caps and budgets) ride L3's push as
    separate commits.
    L3 is sliced by conversation (orchestrator decision 2026-10-13 under
    No Blocking Asks, after the inventory put L3 at about 15k lines
    deleted and 8-9k rewritten across about 80 test files): each slice
    moves one route wholly onto records and deletes that route's packets
    and work items, so every route has one authority at every commit (no
    dual-write, no shadow fold). L3a: the record substrate (inbox
    sequence, conversations, rounds, hypothesis ids, cursors, wake rows)
    and the Texture-research conversation (landed). L3b: Texture and
    management on records (management reads its inbox by cursor and
    answers each Texture at texture:<doc>; engineering acts addressed to
    texture:* refused); a records turn's Cast to engineering is refused
    until L3b-2. L3b-2: management and engineering on records with L4's
    routing; engineering cells run in the capsule, whose request carries
    only Inbox and Updates, so Records reach engineering only with a
    capsule image update (inventory 2026-10-13). L3c: the remaining packet and work
    item machinery deleted wholesale (owner revise as a record, settlement
    gate, delivery claims, breaker, obligations counts), the checker on
    records, and work_state_epoch.
    L3 records, inbox sequence, conversations, rounds, hypothesis ids,
    cursors and wakes (1, 2); packets and work items deleted (3); closure
    records added to L1's boot phase; the planned-restart marker gains
    work_state_epoch (D7) in the same landing; the checker re-expressed on
    records (I2, I3 and deliveries read work items and packets today) with
    I6 and the Cut 1 invariants; generated verb lists and any instruction
    the landing makes false change with it (plan review C6).
    L4 routing (5), with a management turn in the story scenario (C12).
    L5 sub-researchers as runs on records (6). L6 the full prompt rewrite
    and the reverse parity test (7). L7 run acceptance on the fold (8) and
    the self-development round trip. The story scenario runs at the Cut 1
    boundary.

receipts: []
---

# Cut 1: authority

Work today lives in two mutable layers beside the records: work items,
which do six jobs, and packets, which carry delivery state and report
content. Their bugs are the Texture closure cluster, the P2d2 stall and
the crash rule broken in both directions. This cut deletes both layers.
Records become the only work state, read by cursor. Boot closes and
never starts. Engineering reaches Texture only through management, and
each desk's prompt is written once for this system.

## Decisions (proposed; mechanism delegated by decisions §12)

**Inboxes and sequence.** Every record addressed to an inbox
(`texture:<doc>`, `research:<doc>`, `management:<owner>`,
`engineering:<doc>`, a sub-researcher's parent) takes the next sequence
number from that inbox's head, inside the record's commit. An owner
revision commits an owner-emitted record that points at the revision, in
the same transaction, and that record takes the next number in
`texture:<doc>`. Texture has one cursor over both, every sequence number
belongs to a record, and the revision text stays on its chain. The head
is a cache of the fold, written only by that commit and rebuilt by the
checker. This fixes P2d's cross-trajectory
numbering (lifecycle_commit_act.go:298).

**Conversations, rounds, hypotheses.** The store stamps
`conversation_id` from (sender inbox, recipient inbox). Subject is part
of each inbox id, so the sender never names a thread. A conversation head
(open round number, closed flag) is advanced in the same commit, so a
Cast racing a final Report lands in one round or the next, never in a
closed one. A Cast with no open round opens one. Only the recipient's
final Report or a system closure closes it. An Ask and its Reply do not
close a round. The store assigns hypothesis ids per conversation and
returns them in the act receipt. Later Casts revise or withdraw by id.
Status is a query: open rounds, hypothesis states, unread records.
`CancelAct` and desk Retract are deleted; the sender steers by Cast and
may withdraw any of its hypotheses. The owner can close any open round
with an owner-emitted closure record (same shape as the restart closure,
reason "closed by the owner"); that is the stop control, not a cap.

**Cursor and wakes.** A reader's cursor ("read through #57") is a map
from inbox to sequence on its agent record; research and engineering
read one inbox per document, so they hold one entry per document. Each
unit of work shows records in (cursor, head] and advances the cursor to that head with a compare-and-set from the old
value, in the same transaction as its effects:

- **Texture.** `ApplyTextureTurn` already writes the agent record in its
  transaction (texture_turn.go:612-623, committed at :652). The turn's prompt lists every record
  in its range, and the turn record stores the range instead of
  `InboundUpdateIDs` and `CallerWorkItemID` (types/lifecycle.go:296-307).
- **RLM desks.** A cell's frame shows its range. The cell's acts and its
  cursor advance commit as one transaction, replacing per-act commits and
  `recoverPartialActCommit` (rlm_reduce.go:766-830).

Eligibility and activation are separate. Every addressed commit writes
one wake row for the recipient inbox, keyed by (inbox, seq). A desk is
activated by a wake row when no run of that desk is active. A turn's
range is (cursor, head] at its start, and it spends every wake row inside
that range. Rows past the range stay pending:

- after a turn commits, the controller re-checks head > cursor and starts
  one more turn if a pending row is there (a record that arrived mid-turn
  is never stranded);
- a turn that fails without committing leaves its own records unread and
  re-arms nothing, but a pending row past its range still fires once the
  desk is idle. A poison record is therefore read again only when a new
  record arrives, never in a loop, and the delivery-attempt caps and
  breaker go;
- a planned-resume wake (below) activates a desk even when head equals
  cursor, so an open round whose Cast was already read resumes.

**Restart.** At a markerless boot, before the projector starts
(runtime.go:778-779), one phase does three things:

- pre-boot pending, running and passivated runs become terminal with
  fate `interrupted_by_restart`;
- non-terminal engineering assignments are cancelled with that fate, and
  the cancel wakes no one;
- each open round gets one closure record. The closure is a system
  Report, final, with id `<conversation_id>:r<round>:restart` and
  addressed to the sender. It does not wake anyone.

Four existing phases stay (runtime.go:750-761): platform_update_resume
(it acts only under a marker), selfdev_close_after_crash,
source_schedules_close_after_crash (a markerless boot closes active
schedules with the fate and mints no wake; a planned boot keeps the
pending wake), and selfdev_materialization_reconcile (it advances
operations already decided, and never starts desk work).

Pre-boot wake rows are refused, as handler.go already does. Unread
records wait for the next post-boot commit. A second boot finds no open
round and writes nothing.

A marker resumes only when its work-state schema epoch equals the
booting binary's. The marker type gains the field
(`PlannedRestart{Reason, Target, RequestedAt}` today); a marker without
it is from before the cut and counts as a crash. Under a marker, boot terminalizes runs with fate
`restarted_for_update` and mints one wake per desk with an open round or
unread records, and that wake activates the desk even when head equals
cursor. Engineering re-provisions its capsule once, as attempt
n+1. The marker is consumed first (planned_restart.go:94), so one resume
per planned restart. Cut 1 bumps the epoch, which makes the cutover boot
a crash boot (decisions §12.2). Old work items, packets and runs stay in
`og_objects` as archived history and nothing reads them. After a crash,
Autopaper restarts only when the owner restarts it by hand (§13.3).

**Routing.** Management's conversation with a Texture carries Texture's
Cast. Management Casts engineering in its own conversation. Engineering's
final Report closes that round, and management relays to Texture in one
of two ways:

- `Forward(recordID)`, a Report that carries the engineering record's
  content verbatim and cites it (Jev's pass-through);
- its own Report.

There is no hold state, and none is added. Engineering's module set is
WriteFile, Exec, Complete, Freeze, Report, Ask, Reply and Note; Cast is
gone. Engineering may Ask its sender (management) inside an open round.
Its addressee is always its conversation's sender. The reducer refuses an
engineering act addressed to `texture:*`.

**Self-development.** The operation record stays. It is the effect
journal of one candidate bundle (digest, verification, decision,
materialization, route receipt), not a work queue: it says what was
built and whether it was applied, never what work is open. The path:

1. Management Casts engineering in their conversation.
2. Engineering works in its capsule and calls Freeze. Freeze binds to the
   engineering run's capsule and trajectory as today, opening the
   operation through `OpenFreezeCandidate` when none exists
   (tools_capsule.go:251-270). Engineering's final Report names the
   operation id and bundle digest.
3. Management reads that Report by cursor and runs Verify. Verify binds
   to the operation id and digest named in the Report it answers,
   replacing the lookup by management's own trajectory
   (rlm_reduce.go:1388-1405); a digest mismatch is refused as today.
4. The materializer tells management that a verified operation awaits
   the owner's decision with a system record of deterministic id
   (self_development_materializer.go:858-883 already mints one); it now
   takes an inbox sequence like any record. A failed materialization is
   reported the same way instead of re-arming a retry wake; management
   decides what to do.
5. The owner decides through the existing approval route. The
   materializer applies through the updater and writes a marker; the
   apply restart resumes once. Restore stays as it is.

What goes is the work-state around it: the public start-operation API,
the one-open lock, auto-advance and auto-fail driven by engineering
fates, and prompt special-casing. Run acceptance reads capsule
checkpoints from the Freeze and Verify records.

**Report body.** A typed body on the record replaces the raw packet JSON
in `Prediction.Hypothesis`. The schema may still change until the first
permalink (§12.1).

## Packet uses and what replaces each

| Use today (03499ef9) | Replaced by |
|---|---|
| record → packet mint, caller-trajectory seq (lifecycle_commit_act.go:298-377) | inbox seq on the record |
| producer reports bound to open work (lifecycle_commit_act.go:455-721; tools_worker_update.go:506-527) | Report in an open round; authority = open round addressed to me |
| Texture control accumulator minting work, packet and cast (lifecycle_control_issue.go:100-401) | Cast records committed with the turn |
| management bound report via QueueLifecycleUpdate (tools_engineering_assignment.go:123-298; lifecycle.go:3575) | Report or Forward in management's conversation with that Texture |
| engineering report packets and the path-B retarget (engineering_assignments.go:1719-1823) | engineering Report to management |
| wake derived from pending packet (lifecycle.go:548-599) | wake row per addressed commit |
| choir.Updates, Inbox, Emits; envelope mailbox and cursor (tools_desk.go:468-575; rlm_reduce.go:312-494) | one view of records in (cursor, head] |
| delivery binding, claims, attempts, stranded repair, breaker (lifecycle_update_delivery.go:32; texture_controller.go:615-631, :771-913; management_controller.go:1240) | cursor; nothing to claim |
| consume dispositions incorporated, rejected, delivered (texture_turn_runtime.go:85-152) | the turn's range; Texture answers in its revision and next Casts |
| stale-packet expiry (lifecycle.go:1000-1059; lifecycle_expire.go:37) | deleted |
| settlement gate (lifecycle.go:2730) | deleted; living documents don't settle |
| packet work bindings and control targets (lifecycle_texture_target.go:57-275) | deleted |
| occurrence resolution and activation authority (texture_controller.go:283-450, :1167-1500; management_controller.go:1556-1797) | open round or unread records for me |
| research admission by control fingerprint (runtime.go:3007) | same |
| obligations surface counts (obligations_surface.go:230) | fold queries |
| autonomous wire publication and its work item (tools_texture.go:445; wire_publication.go:21-260; internal/wirepublish; proxy/wire_platform_publish.go) | deleted; Cut 2's per-version publication replaces it |
| wire-processor decision work items (wire_publication.go:149-260) | deleted with the Universal Wire path |
| self-development decision observation (self_development_materializer.go:858-883) | the same record as a system emitter with an inbox sequence |

The first construct re-greps these on the base commit. Anything not in
this table is added before deletion starts (standing question 3).

## Invariants added to Cut 0's checker

These extend `scripts/reeng_invariants.mjs`; Cut 0's I6 (cursors only
move forward) is implemented here.

1. Inbox sequences are gapless and unique.
2. A cursor is never past its head, and only a commit that names its
   range advances it.
3. A conversation has at most one open round.
4. Only a recipient's final Report or a system closure closes a round.
5. Hypothesis ids are unique per conversation, and each settles or drops
   at most once.
6. No engineering record is addressed to `texture:*`.
7. After a markerless boot, every pre-boot open round has exactly one
   closure, and no run starts before a post-boot commit.
8. The inbox and conversation heads equal their rebuild from records.
9. No pending wake row is older than the end of the last turn of an
   idle desk (no record is stranded).

## Standing questions

- **Provenance.** The owner's §2 and §12; mechanism D1-D9 is this file's
  proposal.
- **Conformance.** No new store. Records stay in the computer's object
  graph.
- **Deletion citers.** The inventory and grep gate cover code, docs,
  specs and skills in the same commits.
- **Single authority.** The record tape. The heads are rebuildable
  caches.
- **Artifact.** Fold queries and receipts through the product API.
- **Fate-sharing.** The fold reads only the computer's own store.
- **Restart.** See above.
- **No SSH.** Every acceptance goes through the product API and Cut 0's
  export.
- **Registry hygiene.** At terminal, update ACTIVE.md, mission-graph and
  the authority manifest.

## Ways it can fail, written before the code

- A cache (an inbox or conversation head, or the cursor) drifts into
  authority. It is written only in the record commit, and the checker
  rebuilds and compares it. A fix that needs more mutable state trips the
  redesign tripwire.
- A record committed during a Texture turn is lost or read twice. It
  lands past the turn's head, and the cursor compare-and-set refuses a
  stale advance.
- One cell transaction for all acts and the cursor is too slow or too
  large. Measure on the tracer. Tray quotas already bound it at 16
  intents (intent.go:86).
- A Cast races a final Report into a closed round. The conversation head
  serializes them.
- A recipient never sends a final Report. The round stays visibly open.
  Nothing gates on it, the sender can steer, and a crash closes it.
- The closure record wakes a desk at boot. Closures are system records
  that mint no wake, and invariant 7 checks this.
- A re-dispatcher missing from the deleted list re-arms old state. The
  crash drill's quiet window catches it.
- The cutover deploy carries a platform-update marker and resumes old
  work. The epoch check prevents this, and nothing reads old objects.
- The owner's computer breaks under the cutover. Accepted risk (decisions
  §13.1): it takes each cut like a test computer, and a breakage there is
  a breakage-log entry, fixed forward.
- A record that lands during a turn is stranded because no wake fires
  after the turn. Rows past a turn's range stay pending, the controller
  re-checks after every commit, and invariant 9 checks it.
- Several records in one turn are answered as one. The prompt lists each
  record, and the acceptance checks that each is cited or declined.
- Management and Texture feed each other without end. No bound is added;
  a breaker comes if a loop is observed (§1.4).
- Engineering finds another way to Texture. The module set, the reducer
  refusal and invariant 6 block it.
- Removing self-development work-state breaks the apply path. The
  operation stays as the effect journal, Verify is re-bound to the
  Report's operation id, and the self-development acceptance runs Freeze
  through restore on a fresh computer.
- Run acceptance silently loses a checkpoint. It must reach the absolute
  target in artifact 0 on the round trip, and the tracer story must
  reach at least Cut 0's recorded level.
- Sub-researchers flood the parent inbox or outlive it. Concurrency 4 and
  the spacer stay, and child runs are runs, closed at a crash.
- Prompts pass parity by losing instructions desks need. Each deletion
  names the reality it now matches, and the tracer run reads the desks'
  behaviour.
- Cut 2 rewrites the research and Texture prompts again. This cut
  writes all four prompts' cores; generated verb lists change by
  themselves; Cut 2 adds role text only where its new verbs (Write,
  search results, Judge) need judgement.
- The egress budget is claimed twice. This cut deletes it
  (egress_budget.go); Cut 2 only builds async search on what remains.

## Open questions

Answered after the convergent review (decisions §13); none remain open
for this file.

1. Owner's computer: not held (owner, decisions §13.1). It takes the cut
   at its next boot; nothing on it is exported or preserved.
2. Owner revisions: yes, owner-emitted records pointing at the revision
   (D1), so every inbox sequence belongs to a record.
3. Jev: Forward in this cut; a management-only `choir.Judge` in Cut 2.
4. Materializer: yes, Freeze, Verify and the materializer read operation
   states. The operation stays as the effect journal and Verify is
   re-bound to records ("Self-development").
5. `CancelAct` and desk Retract are deleted; senders withdraw hypotheses
   by Cast; the owner closes any round with an owner-emitted closure
   record. Decided under the §12 delegation, not by an explicit owner
   statement.
6. Engineering inbox: per document (`engineering:<doc>`), keeping
   conversation identity across rounds.
