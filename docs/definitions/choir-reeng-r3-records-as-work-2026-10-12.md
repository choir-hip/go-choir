---
definition_version: 4

# R3 — records as work. Station of
# definitions/choir-reeng-metamission-2026-10-12.md. Owner-agreed direction:
# docs/re-engineering-decisions-2026-10-12.md §2-3 (wins over the studies).
# Main study: docs/reports/re-engineering-studies-2026-10-12/B-work-items-as-records.md
# (§6 staged migration, used here as slices S1-S6). Carries the protocol
# metamission's P2d2 (definitions/choir-protocol-p2d-texture-addressing-2026-10-11.md)
# as slice S3; shape D (docs/texture-directive-inbox-design-2026-10-11.md) is
# not built. Unblocks Autopaper W2 (definitions/choir-autopaper-w2-living-textures-2026-10-11.md).
readiness: drafted

review:
  reviewer: none yet
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-12T18:10:00Z
  source:
    canonical_ref: main fbb58c23
    deploy_identity: staging choir.news b172895b
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      No Go file changed between study B's base (a401c878) and fbb58c23
      (git diff --stat), so the study's file:line cites hold. Spot checks
      below were re-read at fbb58c23.
    - >-
      Work items are mutable objectgraph rows doing six jobs (activation
      authority, packet binding, settlement gate, objective dedupe,
      owner-revise reopen, engineering anchors; study B §1). Texture submit
      binds the first open work item and refuses without one
      (internal/textureowner/texture_agent_revision.go:502-509). Settlement
      requires no open work (lifecycle.go:2730 lifecycleSettlementReady;
      RequireNoOpenWorkItems set at texture_lifecycle_create.go:97,
      texture_handoff.go:254, coagent_route.go:218, selfdev_texture_join.go:73).
      OpenLifecycleWork (lifecycle.go:2831) is live; SettleLifecycleWork
      (:5057), RefuseLifecycleWork (:5084) and AmendLifecycleWork (:3293)
      have no production caller (study B §1(b)).
    - >-
      Records cannot yet carry work. CommitmentRecord kinds are precommit,
      report, resolve, disagreement, directive (internal/types/commitment.go:42-46);
      directive subtypes include cast and retract (:58-59); emitter kinds
      are agent, owner, edition, source, with no system kind (:262-265).
      There is no final flag on Report, no thread id and no fate act.
      Desk-authored records set no EmitterKind or SubjectRefs
      (commitmentRecordForIntent, internal/agentcore/rlm_reduce.go:1080).
      Completion lives on the packet's WorkDisposition, defaulted to open
      (rlm_reduce.go:1483-1484, :1510).
    - >-
      The record object's metadata already holds record_kind and addressee
      (internal/store/lifecycle_commit_act.go:307-317, :562-565), but only
      run_id, trajectory_id and channel_id are generated, indexed columns
      (internal/objectgraph/dolt_store.go:154-162); record queries are
      JSON_EXTRACT scans. Every Report commit lists up to 10,000 records
      (mechanicalResolveForReport, lifecycle_commit_act.go:136-145).
    - >-
      Casts never fill the prediction slots that exist:
      CommitmentPrediction{Hypothesis, Assumptions, Alternatives, Questions}
      (commitment.go:115-122) and TypedQuestion{Question, Answers,
      Probabilities, Weight} (:75-82), which has no stable id, crux or
      stakes field.
    - >-
      P2d2's blocker is live: ResolveLifecyclePacketWorkBindings handles
      only "", producer_report and control, and errors on directive
      (internal/store/lifecycle_texture_target.go:244-275), so a Texture
      turn commit fails while a directive is pending (probe-confirmed,
      inbox design §2). Directive packets are numbered from the caller's
      trajectory seq (lifecycle_commit_act.go:298, :371). P2d1 (only a Cast
      to engineering opens an engineering assignment) landed as 6990b09d
      (castOpensEngineeringAssignment, rlm_reduce.go:1796).
    - >-
      A markerless boot closes actor wakes of restartWorkKinds only
      (internal/actorruntime/handler.go:97-126); no boot path closes work
      items or packets (docs/problems/crash-restart-leaves-work-open-and-recasts-engineering-2026-10-12.md).
      R2 owns that rule; this station only re-expresses it as fate records.
    - >-
      The Texture initial-work wake arms on any open non-supervision
      Texture work item (texture_controller.go:638-645); the supervision
      prefix has 6 non-test references. The obligations surface is served
      at /api/runtime/obligations (internal/apihandler/routes.go:47).

finish:
  deliver: >-
    What a desk owes, what it has done and whether it may act are read
    from the precommitment record tape, not from mutable work items. A Cast
    opens a thread; Reports in the thread are progress; a final Report, a
    Retract, or a system fate record closes it. Management can feed any
    same-owner Texture through records, and a Texture takes every record
    addressed to it, several at once, in one turn. Work items are deleted.
  artifact: >-
    Six slices, landed in order, each with its own acceptance and Landing
    Loop.
    S1 (orange) shadow fold: generated indexed columns for addressee and
    record_kind; read-only store functions ThreadStatus(root) and
    OpenThreads(addressee, subject) computed from records only; an audit
    view on /api/runtime/obligations (and choir CLI) that compares the fold
    with work items per computer and classifies each disagreement into a
    named class (research work never settles; work with no root record:
    pre-P1 controls, owner-opened Texture work, supervision anchors;
    retract that left work open; unclassified). Before S2, thread roots are
    found by walking ParentID/RelatedIDs and a Report is provisionally
    terminal when its packet says work_disposition=completed, labelled as
    a pre-S2 read.
    S2 (red, evidence ledger) record schema, dual-write: the store stamps
    thread_id at commit (TargetRef present: the target's thread; a Cast or
    Ask to D:S with an open thread there: that thread, per protocol design
    §3; else itself) with an indexed column; Report.final (set from the
    packet's work_disposition=completed while both exist); a fate directive
    subtype with a new system emitter kind and deterministic ids;
    EmitterKind, ComputerID and SubjectRefs{doc_id, trajectory_id} stamped
    on desk-authored records; mechanicalResolveForReport reads by thread_id
    instead of listing 10,000 records. Hypothesis-shaped Casts as optional
    additive fields (see now.decision): hypotheses with id, statement,
    probability, stakes and crux; open uncertainties with stakes and no
    probability; general hypotheses (kind general, seed name from
    decisions §3.1, probability, the operational meaning of the key word);
    findings on Reports tagged bears_on <hypothesis id> or outside; a
    belief-update record (default: a ledger-only Note whose thread is the
    Cast, carrying updates[] with p_before/p_after and cites[]) that may
    also judge a general hypothesis material, refused at commit when the
    judge's emitter equals a cited Report's emitter. The store validates
    well-formedness only (ids unique, probabilities in [0,1], bears_on
    names a hypothesis on the thread root). Work items stay authoritative.
    S3 (red, Texture) the Texture inbox, which is P2d2: Texture's
    obligation is the fold "owner head newer than its last consumed head,
    or an unconsumed record addressed to texture:<doc>"; activation
    authority and the wake read that fold; the run binds the consume
    watermark (scheduled_message_seq, texture_turn_runtime.go:87). For the
    turn contract's CallerWorkItemID a standing texture-inbox:<doc> anchor
    is used transitionally (never settled by a turn, deleted in S6).
    needTo accepts texture:<doc> from management for the same owner and
    computer, and Ask, Escalate and Reply to a Texture stay refused;
    directive packets are numbered per addressee on the document's
    trajectory; ResolveLifecyclePacketWorkBindings gains a directive case
    with no work; a directive occurrence resolves by its record (exists,
    not retracted, addressee and emitter checked); one consume predicate
    covers producer reports and directives; the revision request's
    worker-message slot carries every directive the turn consumes (record
    id, emitter, objective, body, evidence refs); a refused occurrence
    terminalizes its packet with the reason; a retracted pending directive
    terminalizes cancelled; management's overlay teaches texture:<docID>.
    Owner revise stops opening and reopening Texture work
    (ownerRevisionTarget, textureWorkSettledByDesk, reopenTextureWorkForOwner,
    texture_agent_revision.go:134-160, :230-312, and the 409 class are
    deleted); the supervision prefix and its exclusions are deleted;
    Texture trajectory settlement is the fold. P2d's four-directive bound
    is not carried (decisions §1.2, §1.4).
    S4 (red, lifecycle and ledger) crash close as fate records: if R2 left
    crash close as an interim (status writes on work items or packets), a
    markerless boot instead appends one fate record "interrupted by a
    restart" per open thread (id <root>:fate:restart:<bootID>), after the
    planned-restart marker check and before any wake dispatch, and
    terminalizes those threads' pending packets; a planned restart skips
    it. If R2 already writes fate records, S4 is the deployed check only.
    S5 (red, ledger, run acceptance, engineering authority) research,
    management and engineering on threads: runs bind thread ids or the
    watermark instead of work_item_ids (management_controller.go:376-379,
    :2094-2154; runtime.go:1467, :2908-2917; engineering_assignment_runtime.go:191-197);
    producer-report authority comes from the open thread
    (tools_worker_update.go:506-527, lifecycle_commit_act.go:487-505);
    choir.Report takes final and work_disposition retires with
    TargetWorkItemID and ProducerWorkItemID; EngineeringAssignment keeps its
    capsule state and its parent and assigned anchors become thread ids;
    settlement is the fold everywhere; a one-time, idempotent cutover
    closes each remaining open work item with the fate "superseded by
    record-native work". Prompts change in the same commits (affordance
    invariant, decisions §1.1).
    S6 (orange; red where it touches lifecycle replay) deletion: the
    work_items SQL DDL and its vocabulary migration (store.go:379-395,
    :527-529; vocab_migrate.go), WorkItemRecord, StepBudget and
    TokenBudget, OpenLifecycleWork, Settle/Refuse/AmendLifecycleWork,
    lifecycleWorkFingerprintAvailable, the generic work-item stack
    (store/trajectory.go:153-385, graph_store.go:1473-1730), the standing
    anchor and CallerWorkItemID, and every citer in docs, specs and skills.
    Retired lifecycle event kinds stay decodable.
  acceptance:
    - action: S1 audit view on two or more live staging test computers after the AI news demo
      proves: the record fold agrees with work items except for named work-item defects and no-record origins; unclassified disagreements are zero
      evidence_class: deployed proof (audit JSON exported as an artifact)
    - action: S2 audit re-run reading thread_id and final from records only; an old binary decodes new records; a hypothesis-bearing Cast committed on a test computer renders in the obligations view; malformed hypotheses and a reporter judging its own finding are refused
      proves: the schema is the fold's sole input and is backward-readable
      evidence_class: deployed proof plus store tests written before the code
    - action: "S3, P2d2's original acceptance: on a fresh test computer an owner-prompted management turn casts to texture:<doc>; the Texture's next revision answers it; the audit joins record, packet and consuming turn"
      proves: management feeds a Texture through a record
      evidence_class: deployed proof
    - action: S3, three or more management directives to one Texture committed before its turn are consumed by one turn whose prompt shows all of them, plus a directive to an idle Texture whose request is done, a forged directive, a retracted directive and an owner revise after a completed turn (no 409)
      proves: Texture's obligation is its inbox; several directives per turn; no reopen path needed
      evidence_class: deployed proof (Playwright or CLI spec with exported join)
    - action: S4, R2's crash probe on a test computer with an open research thread, then a second crash boot
      proves: the thread closes by one fate record, nothing re-dispatches, a repeat boot mints no duplicate fate
      evidence_class: deployed proof
    - action: S5, a full owner -> Texture -> research thread with three or more progress Reports, closed by research's final Report
      proves: the thread opens, progresses and closes on the tape; no work item is minted for it; the fold reports closed
      evidence_class: deployed proof with trace audit record -> packet -> turn receipt -> revision
    - action: S5, an engineering assignment through management on a test computer, and run acceptance synthesized for the demo before and after S3 and S5
      proves: capsule authority and run acceptance are unchanged
      evidence_class: deployed proof
    - action: S6 grep gate on WorkItem|work_item in non-test code and docs, plus decoding a pre-R3 staging lifecycle tape
      proves: work items are gone except listed decode shims
      evidence_class: CI plus deployed proof
  rollback: >-
    git revert per slice. S1: inert columns, read-only views. S2: additive
    JSON; written records stay valid (old readers ignore unknown fields,
    commitment.go:84-86); records are immutable, so the field names must
    pass review before S2 lands. S3: before reverting, a drain command
    terminalizes pending directive packets addressed to Textures (reason
    rollback) and closes the standing anchors, because old code re-arms on
    both. S4: revert; fate records already written stay as true history.
    S5: revert; work items closed by the cutover stay closed and old code
    opens new ones. S6: revert restores the code; dropped DDL was dead.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the obligation facts held outside the append-only record tape
    (mutable status columns, packet-only dispositions, implicit reopen
    rules) toward zero, while preserving Texture as sole writer, the
    restart rule, authority boundaries and one authority per fact.
  goodharting_would_be: >-
    a fold tuned to agree with work items, reproducing their bugs; a
    per-thread head row or packet field that quietly becomes the new
    mutable status; deleting work-item names while the same jobs move to
    another mutable row; a Texture inbox that consumes directives and
    ignores them.

homotopy:
  realism_axis: >-
    shadow fold read-only on staging test computers -> records carry
    thread, final and fate (dual-write) -> Texture runs from its inbox
    (P2d2) -> crash fate closes threads -> every desk on threads -> work
    items deleted -> Autopaper W2 feeding story Textures

boundaries:
  mutation_class: red overall; S1 orange; S2-S5 red; S6 orange, red where lifecycle replay is touched
  authority_sources:
    - docs/re-engineering-decisions-2026-10-12.md §2-3, §10 (owner 2026-10-12)
    - CLAUDE.md "Restarts End Work (Crash) Or Resume It (Planned Update)"
    - docs/precommitment-protocol-design-2026-10-11.md §3 (addressing), where not replaced
  must_preserve:
    - only the Texture desk writes a Texture
    - a crash restart never resumes work; a planned restart may, once
    - producer_report and document_revision occurrences resolve as today until S5 changes them deliberately
    - engineering's delegated Cast still opens its assignment; capsule state stays on EngineeringAssignment
    - records are immutable; no runtime scoring or materiality judgment (decisions §2, §9)
    - no budgets, round caps or directive bounds are added (decisions §1.2, §1.4)
  excluded:
    - research's and Texture's use of hypothesis fields in prompts and reasoning (R4 research, R5 Texture)
    - Assign (deleted in R1) and management routing, Jev and path B (R7)
    - owner revisions as records (owner revisions stay on the revision chain)
    - runtime-minted materiality triggers and resolve-by-count (study B §3b, withdrawn by decisions §9)
    - circuit breakers before a loop is observed
  protected_surfaces:
    - evidence ledger: internal/types/commitment.go, internal/store/commitment.go, internal/store/lifecycle_commit_act.go, internal/objectgraph/dolt_store.go
    - lifecycle reducer: internal/store/lifecycle.go, internal/store/lifecycle_texture_target.go, internal/store/lifecycle_control_issue.go, internal/types/lifecycle.go
    - Texture canonical writes and activation: internal/store/texture_turn.go, internal/textureowner/texture_controller.go, texture_turn_runtime.go, texture_agent_revision.go, internal/actorruntime/adapter.go, internal/actorruntime/handler.go
    - desk act path and prompts: internal/agentcore/rlm_reduce.go, internal/agentcore/runtime.go, internal/agentcore/management_controller.go, internal/agentcore/tools_worker_update.go, internal/runtimeprompts/overlays
    - engineering authority: internal/agentcore/engineering_assignment_runtime.go, internal/store/engineering_assignments.go
    - run acceptance (read-only check): internal/agentcore/run_acceptance.go, internal/store/run_acceptance.go
  admissible_evidence: >-
    deployed proof on choir.news test computers with an exported trace join
    (record -> packet -> turn receipt -> revision) and the fold audit;
    store and controller tests written before the code shape slices but
    never settle one. Local runs prove nothing about wakes, boots or live
    desks.
  conjecture_delta: >-
    Architectural-mode change (throughline Stuck rule): the authority for
    open work, activation and completion moves from mutable work items to
    a fold over immutable records. P2d's conjecture P2d-1 (a directive
    minted with its own work item) is superseded by RC-1 below.
  heresy_delta:
    discovered: >-
      (study B, recorded, not counted as repair) work items are not
      crash-closed; Cast records never close on the ledger; a retract
      leaves its work open; every Report commit scans up to 10,000 records;
      three work transitions have no production caller; budgets are prompt
      text only; the work_items SQL table is dead; Texture submit binds the
      first open work without excluding supervision work; no Cast carries
      a prediction
    introduced: >-
      two named transitional duplicates, each with its deletion slice:
      dual-write of final and work_disposition (S2 to S5) and the standing
      texture-inbox anchor (S3 to S6)
    repaired: none yet (the list above is the target at S6)

now:
  status: drafted
  slice: authored from decisions §2-3 and study B; nothing built
  source_ref: fbb58c23
  deploy_identity: staging b172895b
  candidate: none
  conjecture:
    id: RC-1
    claim: >-
      If open work, activation authority, completion and Texture's
      obligation are derived from the record tape alone, with the shadow
      audit explaining every disagreement on live test computers, then the
      work-item closure bug class stops recurring and management can feed
      any Texture, which unblocks Autopaper W2.
    test: S1 audit, then S3 and S5 deployed acceptance, then two weeks of problem docs with no new closure bug in the Texture loop
    edge: frame_lock
    delta_o: >-
      the audit classifies disagreements by named work-item defect instead
      of counting them, so a fold that is right where work items are wrong
      is not pushed back toward them
    scope_if_supported: the four root desks on staging test computers
    status: proposed
    evidence_refs: [docs/reports/re-engineering-studies-2026-10-12/B-work-items-as-records.md, docs/problems/clustering-texture-obligation-closure-2026-10-09.md]
  decision:
    what: >-
      The hypothesis shape's schema belongs in R3 (S2): question ids,
      stakes, cruxes, open uncertainties, general hypotheses, bears_on tags
      and the belief-update record are record fields, and records cannot be
      re-keyed once written, so they land with thread_id, final and fate in
      one reviewed schema change. R4 owns research's use (writing and
      answering hypothesis-bearing Casts, tagging findings, stopping at
      diminishing returns); R5 owns Texture's. R3 validates well-formedness
      only and scores nothing. Defaults chosen here for the file review:
      new Casts continue an open thread to the same addressee and subject
      (design §3); the belief update is a ledger-only Note; schema_id stays
      commitment_record.v1 with additive fields; documents keep settlement,
      as the fold.
    kind: architecture
    status: proposal
    evidence_ref: docs/re-engineering-decisions-2026-10-12.md §2-3; study B §3b, §6
    owner_ratification_ref: decisions §2-3 ratify the direction; the placement is this file's proposal
  belief:
    believed_state: records already thread, wake and resolve Asks by fold; they lack final, fate, thread id, subject and indexes
    main_uncertainty: whether continue-if-open threading fits desks that want two independent asks to one addressee at once
    next_observation: S1's audit on live test computers
  blocker_or_risk: depends on R1 (prompts, Assign, round cap, breaker) and R2 (restart rule); red throughout; the Texture controller has the most churn
  next_action: >-
    after R1 lands, agentic-consensus file review of this file (focus: S2
    field names, threading rule, S3 transitional anchor); S1 may run after
    R1; S2 onward waits for R2 to land

receipts: []
---

# R3 — records as work

Work items are mutable rows that hold six unrelated jobs and copy facts
the records already hold or should hold. Five Texture closure bugs in one
day came from that layer, and P2d2 stalled on its rule that one Texture
run binds one work item. This station makes work a fold over the
append-only precommitment records and deletes work items. Texture's inbox
is P2d2's delivery, so management can feed story Textures for Autopaper.

**The fold.** A thread is a root act that asks for work (Cast, Ask,
Escalate) plus every record whose thread_id is that root. A thread
closes on a final Report, a Reply to an Ask, a Resolve, a Retract by the
root's emitter, or a system fate record. Open work for D on S is the set
of threads addressed to D:S with no closing record. A desk may act while
it has an open thread or an unconsumed record addressed to it. Progress
is the ordered Reports and belief updates in the thread. "Silent" and
"overdue" are derived labels; they end nothing.

**Standing questions.** One authority: the record tape. A per-thread head
row is built only if S1 measures fold latency that needs it, and then
only as a single-writer cache rebuilt and compared by the audit. Fate
sharing: the fold reads the computer's own store only. Restart: S4
closes threads by fate after the marker check. No SSH: the audit and
every acceptance run through /api/runtime/obligations and the choir CLI.
Deletion citers: S6 greps docs, specs and skills as well as code.

Ways it can fail, written before the code:

- the fold is tuned to match work items and inherits their bugs (the
  audit classifies disagreements; it does not count them down to zero);
- a thread never gets a final Report because the desk forgets (it stays
  visibly open and silent; Retract, fate and trajectory close still close
  it; nothing gates on open threads except the owner's view);
- continue-if-open joins an unrelated Cast to an old thread (the Cast's
  TargetRef wins; the review decides whether a desk may open a parallel
  thread);
- the thread_id stamp races two concurrent Casts into two roots (stamped
  inside the commit transaction under the addressee's condition);
- schema field names are wrong after records ship to other computers
  (field review before S2; additive only after);
- the fold is too slow without indexes (S1 adds them and measures before
  any runtime reads the fold);
- a directive commits while a Texture turn is in flight and is lost (it
  stays unconsumed past the turn's watermark and re-arms the wake);
- a directive is consumed by a turn whose prompt never showed it (the
  consume predicate and the prompt read the same watermark);
- several directives in one turn are answered as one (the prompt lists
  each record; the acceptance checks each is cited or declined);
- management and Texture feed each other without end (no bound is added;
  if a loop is observed, a breaker is added then, per decisions §1.4);
- a forged or retracted directive wakes a Texture (resolved by its record;
  refusals terminalize the packet so it cannot re-arm);
- research Notes to their own Texture, stranded today, change meaning at
  S3 (pre-landing count of pending directive packets to texture:*, and
  the first consumption is recorded);
- crash fate runs before the marker check, or re-dispatches packets of
  closed threads (ordered after the marker; packets terminalized in the
  same pass; deterministic fate id per boot);
- the cutover closes work a live run is bound to (S5 cuts over only on an
  idle computer, per the deploy rule that never restarts a busy one);
- engineering loses capsule authority when its anchors move to thread
  ids (assignment state stays where it is; only the anchor field changes);
- run acceptance silently depended on work items (re-checked on a demo
  before and after S3 and S5);
- a rollback leaves directive packets or standing anchors that old code
  re-arms on (the drain runs first).
