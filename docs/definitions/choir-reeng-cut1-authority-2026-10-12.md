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
readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-12T19:30:00Z
  source:
    canonical_ref: main 03499ef9
    deploy_identity: staging choir.news b172895b
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty (this session's doc edits in progress; decisions doc and metamission)
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
      internal/types/evidence.go:178-240; 29 non-test files, 367 lines) carry
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
      terminal settlement repair (runtime.go:2478-2551). The Texture boot
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
      (:44-46, :238), children get the parent's prompt (:201, :213) and emit
      no events (:210-215).
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
    One coherent cutover, landed as internal commits behind one deployed
    acceptance (inventory first, then store, then desks, then prompts).
    (1) Records: conversation_id and round stamped by the store; Cast
    hypotheses with store-assigned ids (H1, H2, ...), probability, stakes
    and crux, open uncertainties with stakes, general hypotheses (seed name,
    probability, operational meaning of the key word), revise and withdraw
    by id; a typed Report body (claims with sources, findings tagged
    bears_on <id> or outside, settled[{id, outcome}], dropped[{id, reason}],
    stop_reason, final); a system emitter kind and a closure record with a
    deterministic id; subject, EmitterKind and ComputerID on desk-authored
    records; per-inbox sequence numbers; indexed columns for inbox,
    inbox_seq, conversation_id, round and record kind. The store validates
    well-formedness only and scores nothing.
    (2) Delivery by cursor (decisions below): inbox heads, reader cursors on
    agent records, wakes derived from record commits, Texture's turn commit
    and each RLM cell commit advancing the cursor in the same transaction
    as the work.
    (3) Deleted with every reader (inventory in the prose): work items,
    packets, delivery claims and dispositions, stranded repair, Texture
    breaker, stale-packet expiry, the envelope mailbox and its cursor,
    owner-revise reopen and the 409 class, document settlement, the one
    management Cast per turn rule.
    (4) Boot: a markerless boot terminalizes pre-boot runs and engineering
    assignments with fate interrupted_by_restart, appends one closure record
    per open round, and dispatches nothing. Deleted: boot passivation and
    passivated-run resume, spawned-work replay, terminal settlement repair,
    Texture wake re-arm, wire-publication claim recovery, the engineering
    restart recast. A planned-restart marker of the same work-state schema
    epoch resumes once.
    (5) Routing: path B, Assign, Message, Emit, Emits deleted; engineering's
    module set without Cast and Ask, its addressee forced to its
    conversation's sender (management), the reducer refusing any
    engineering act addressed to texture:*; management relays an
    engineering Report to Texture by forward (pass-through) or its own
    Report; the runtime Jev judge and self-development scaffolding removed
    (public start-operation API, one-open lock, auto-advance and auto-fail
    from engineering fates, notifySelfDevelopmentDecisionBoundary, prompt
    special-casing) while freeze, verifier, decision binding, updater,
    materializer, marker, platform update, route projection and restore stay.
    (6) Research: round cap, egress budget, Texture and engineering
    tool-loop budgets, forced status report, next_instruction and every
    update_coagent name deleted; research reports carry final. Sub-researchers
    are runs with events, their own short prompt and read-only surface, and
    report to the parent research inbox through records; choir.Research
    returns at once; the all-children barrier, window, share and noEmit go.
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
    - action: reverse parity test (fails on a planted stale verb, shown once) and a grep gate on WorkItem|work_item, CoagentSourcePacket, update_coagent, Assign, choir.Message, choir.Emit, textureSupervision, round cap and budget names
      proves: the deleted names have no non-test citer outside listed decode shims
      evidence_class: CI
    - action: run acceptance synthesized for the engineering round-trip trajectory
      proves: acceptance derives from the fold and reaches the level it reached before the cut for the same path
      evidence_class: deployed proof
  rollback: >-
    Forward-fix (decisions §12.3). Before the cutover push, tag the Cut 0
    landed commit reeng-cut1-last-good. Staging test computers are
    disposable and are re-minted, never reverted: old code cannot read
    new-format records. The owner's computer stays on last-good until the
    cut is proven, so its snapshot is its pre-cut checkpoint. Break-glass:
    redeploy the tag and mint fresh test computers.
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
    - published versions and the owner's computer and Textures untouched until the cut is proven
    - no budgets, round caps, reminders, runtime scoring or circuit breakers are added
    - substrate stays (decisions §1.2) - child concurrency 4 and the 1.5 s spacer, tray quotas, poison-free wakes, the 60-minute run limit as a labelled prototyping constraint
  excluded:
    - async search, the research desk's new verbs, Texture's write verb, captures, permalinks, Autopaper (Cut 2)
    - trace export, tracer, invariant checker, fresh computers, request-id fix, one REPL owner per desk, survival register (Cut 0)
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
    introduced: none intended; two cache objects (inbox head, conversation head) written only by the record commit and rebuilt by the checker
    repaired: none yet; the documented list is the target

now:
  status: drafted
  slice: authored from decisions §2, §9, §10, §12 and the R1-R4 drafts' findings; nothing built
  source_ref: 03499ef9
  deploy_identity: staging b172895b
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
    test: the acceptance list, then five consecutive clean tracer runs and no new work-state entry in the breakage log after its last fix
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
      advanced in the record's commit; owner revisions take the next number
      in Texture's inbox but stay on the revision chain. D2 conversation id
      is derived from (sender inbox, recipient inbox); round from a
      conversation head in the same commit. D3 a reader's cursor lives on
      its agent record; Texture's turn commit and each RLM cell commit (one
      transaction for all its acts) advance it from the value the work saw.
      D4 every addressed commit mints one wake for the recipient inbox;
      failed work never re-wakes; boot mints wakes only under a marker. D5
      the closure record is a system Report, final, reason
      interrupted_by_restart, addressed to the sender, id
      <conversation_id>:r<round>:restart, never waking anyone. D6 the store
      assigns hypothesis ids. D7 a marker resumes only within the same
      work-state schema epoch; Cut 1 bumps the epoch, so the cutover boot is
      a crash boot. D8 a planned resume wakes each desk with an open round
      or unread records once; runs are never resumed.
    kind: architecture
    status: proposal
    evidence_ref: docs/re-engineering-decisions-2026-10-12.md §2, §12
    owner_ratification_ref: decisions §2 and §12 ratify the direction and delegate the mechanism (§12, "I trust your judgement"); D1-D8 are this file's proposal
  belief:
    believed_state: every code claim above holds at 03499ef9; Cut 0 not yet landed
    main_uncertainty: whether one cutover this size converges inside the redesign tripwire, and how the owner's computer is held at last-good while staging deploys
    next_observation: Cut 0's survival register and push-to-healthy-computer time, then the deletion inventory re-grepped on Cut 0's landed commit
  blocker_or_risk: >-
    Depends on Cut 0 (tracer, checker, crash drill, fresh computers,
    request-id fix, one REPL owner per desk). No hold mechanism for the
    owner's computer is known in code; Cut 0's register must name one or
    Cut 1 does not push. Red throughout.
  next_action: convergent panel on this file with the metamission and the Cut 0 and Cut 2 files

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
revision takes the next number in `texture:<doc>` too, so Texture has one
cursor over both. The head is a cache of the fold, written only by that
commit and rebuilt by the checker. This fixes P2d's cross-trajectory
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

**Cursor and wakes.** A reader's cursor ("read through #57") is a field
on its agent record. Each unit of work shows records in (cursor, head]
and advances the cursor to that head with a compare-and-set from the old
value, in the same transaction as its effects:

- **Texture.** `ApplyTextureTurn` already writes the agent record in its
  transaction (texture_turn.go:289). The turn's prompt lists every record
  in its range, and the turn record stores the range instead of
  `InboundUpdateIDs` and `CallerWorkItemID` (types/lifecycle.go:296-307).
- **RLM desks.** A cell's frame shows its range. The cell's acts and its
  cursor advance commit as one transaction, replacing per-act commits and
  `recoverPartialActCommit` (rlm_reduce.go:766-830).

Records that commit mid-work have a higher sequence and wait. Every
addressed commit writes one wake row for the recipient inbox, keyed by
(inbox, seq). The controller starts work only if head > cursor and no run
of that desk is active. A turn that fails without committing leaves its
records unread but mints no wake. So a poison record cannot loop, and the
delivery-attempt caps and breaker go.

**Restart.** At a markerless boot, before the projector starts
(runtime.go:778-779), one phase does three things:

- pre-boot pending, running and passivated runs become terminal with
  fate `interrupted_by_restart`;
- non-terminal engineering assignments are cancelled with that fate, and
  the cancel wakes no one;
- each open round gets one closure record. The closure is a system
  Report, final, with id `<conversation_id>:r<round>:restart` and
  addressed to the sender. It does not wake anyone.

Pre-boot wake rows are refused, as handler.go already does. Unread
records wait for the next post-boot commit. A second boot finds no open
round and writes nothing.

A marker resumes only when its work-state schema epoch equals the
booting binary's. Under a marker, boot terminalizes runs with fate
`restarted_for_update` and mints one wake per desk with an open round or
unread records. Engineering re-provisions its capsule once, as attempt
n+1. The marker is consumed first (planned_restart.go:94), so one resume
per planned restart. Cut 1 bumps the epoch, which makes the cutover boot
a crash boot (decisions §12.2). Old work items, packets and runs stay in
`og_objects` as archived history and nothing reads them. After a crash,
Autopaper restarts only by a deliberate management act (§12.9).

**Routing.** Management's conversation with a Texture carries Texture's
Cast. Management Casts engineering in its own conversation. Engineering's
final Report closes that round, and management relays to Texture in one
of two ways:

- `Forward(recordID)`, a Report that carries the engineering record's
  content verbatim and cites it (Jev's pass-through);
- its own Report.

There is no hold state, and none is added. Engineering's module set is
WriteFile, Exec, Complete, Freeze, Report, Ask, Reply and Note. Its
addressee is always its conversation's sender. The reducer refuses an
engineering act addressed to `texture:*`.

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

The first construct re-greps these on the base commit. Anything not in
this table is added before deletion starts (standing question 3).

## Invariants added to Cut 0's checker

These extend `scripts/reeng_invariants.mjs`; Cut 0's I5 (cursors only
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

## Standing questions

- **Provenance.** The owner's §2 and §12; mechanism D1-D8 is this file's
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
- The owner's computer takes the cutover deploy. Do not push until Cut
  0's register names the hold.
- Several records in one turn are answered as one. The prompt lists each
  record, and the acceptance checks that each is cited or declined.
- Management and Texture feed each other without end. No bound is added;
  a breaker comes if a loop is observed (§1.4).
- Engineering finds another way to Texture. The module set, the reducer
  refusal and invariant 6 block it.
- Removing the self-development state machine breaks the update apply
  path. Only the listed scaffolding goes. If apply reads a deleted
  transition, keep it and log it.
- Run acceptance silently loses a checkpoint. Compare the before and
  after levels on the same path.
- Sub-researchers flood the parent inbox or outlive it. Concurrency 4 and
  the spacer stay, and child runs are runs, closed at a crash.
- Prompts pass parity by losing instructions desks need. Each deletion
  names the reality it now matches, and the tracer run reads the desks'
  behaviour.
- Cut 2 rewrites the research and Texture prompts again. Generated verb
  lists change by themselves; Cut 2 touches role text only where its new
  verbs need judgement.

## Open questions

1. How the owner's computer is held at last-good while staging deploys.
   No mechanism is visible in code. Cut 0's survival register
   (docs/reports/reeng-cut0-survival-register-2026-10-12.md) records
   what must survive but not how it is held, so the register must name a
   mechanism.
2. Should owner revisions become owner-emitted records? This file shares
   only the sequence. Study B recorded an earlier "not yet".
3. Does Jev get a management verb in Cut 1, or only in Cut 2 for support
   judgement? Default: Forward only.
4. Does the materializer's apply path depend on operation states this
   file deletes?
5. Settled 2026-10-12 (owner delegation, decisions §12): `CancelAct` and
   desk Retract are deleted; senders withdraw hypotheses by Cast; the
   owner closes any round with an owner-emitted closure record.
6. Is an engineering inbox per document the right granularity, or should
   it be per management round?
