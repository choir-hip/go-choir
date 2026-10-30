# B. Work items as records: is the work abstraction needed?

Read-only research, 2026-10-12, against `main` a401c878. No repo file was
changed. Owner questions: "what is our work abstraction doing? Is it needed?
Can work be implicit in the precommitment records protocol — everything is
work?" and "how can we use precommitment records for tracking ongoing
progress?" Constraint from the owner: no budgets, no runtime-owned jobs;
desks keep going until diminishing returns and report progress every step
through records.

Evidence labels used below:

- **[V]** verified by reading the code at the cited line (or by grep);
- **[D]** taken from an existing design or problem doc (its author verified it; I did not re-verify every line);
- **[I]** my inference or proposal.

## Summary

- **What work items do today.** They do six jobs that have little in common
  (section 1). They hold activation authority: a run must bind open work.
  They are the anchor that binds a packet to a target. They gate settlement.
  They dedupe by fingerprint. They decide owner-revise targeting and
  reopening. They are the parent/assigned anchors for engineering
  assignments. Many writers mutate them. They are not crash-closed: no boot
  path writes `cancelled` [V]. And they are the recurring bug locus in the
  Texture loop: a clustering assessment found five closure bugs in one day
  [D, `docs/archive/problems/clustering-texture-obligation-closure-2026-10-09.md`],
  and P2d2 is blocked on the one-work-item run model.
- **What records already do.** A record is immutable. It carries a typed
  act, an addressee, a parent (`ParentID`) and related ids, and evidence
  refs. A derived packet, committed atomically with it, wakes the addressee
  (section 2). The ledger has threads (`TargetRef` → `ParentID`/`RelatedIDs`)
  and a reducer-minted mechanical resolve (`system:reducer`). It does not
  record when a Cast is finished: that lives on the packet's
  `WorkDisposition` and on the work item's `Status`.
- **Can work be a fold over records?** Mostly yes (section 3). "Open work
  for D on S" becomes: Casts addressed to `D:S` with no terminal record in
  their thread. "Progress" becomes the chain of Reports and Notes in the
  thread. "Crash close" becomes appending a fate record to each open thread.
  "Activation authority" becomes: an open thread addressed to me, or an
  unconsumed record addressed to me.
- **What records lack to do this:**
  1. a terminal marker on Report (`final`);
  2. a fate act with a system emitter;
  3. a store-stamped thread root id;
  4. a subject on desk-authored addresses;
  5. indexed columns, because record queries today are `JSON_EXTRACT` scans.
- **Casts with hypotheses (owner notes, section 3b).** The schema already
  has prediction slots: Hypothesis, Assumptions, Alternatives and typed
  Questions (`internal/types/commitment.go:115-123`). No Cast fills them
  today [V]. The proposal:
  - **Cast:** carries H1..Hn with probabilities and cruxes, plus a
    catch-all H0.
  - **Progress Reports:** tag each finding by the hypothesis it bears on,
    or as `outside` all of them.
  - **Belief updates:** separate records by the caster or a third party,
    citing the Reports.
  - **H0:** resolves through a materiality-trigger record. The trigger
    links H0, the outside-finding Report, and the belief change
    (|Δp| ≥ δ) or the lede/structure revision. The reporter can never be
    the judge.
  - **Close:** resolves each Hi. H0 resolves by count of its triggers.

  Progress becomes belief movement, so "diminishing returns" can be
  measured without a budget.
- **P2d2.** With the Texture desk's obligation derived from its inbox fold,
  a directive to `texture:<doc>` is just another record addressed there. The
  three mechanisms shape D adds are not needed: reopen-in-act, the
  completion guard and the dead-work candidate rule. The per-addressee
  sequence fix is still needed (section 5).
- **Cost.** This is a red, multi-station substrate migration (section 6).
  It deletes far more than it adds: about 1,900 matching lines in 62
  non-test files, by grep [V]. It must not be started as a patch inside
  P2d2.

---

## 1. Inventory: what work items are used for

Work items are objectgraph objects of kind `choir.work_item`
(`internal/store/graph_store.go:24`, registry
`internal/objectgraph/registry.go:68`). They are mutable: status, details,
`LifecycleVersion`. The type is `types.WorkItemRecord`
(`internal/types/trajectory.go:85-105`). Its statuses are open, completed,
cancelled and refused (`:74-79`).

There are two access stacks [V]:

- **The lifecycle stack (live).**
  - Read through `GetLifecycleWorkItem` (`internal/store/lifecycle.go:1931`)
    and the `LifecycleSnapshot.WorkItems`.
  - Written only inside lifecycle reducer commands, which also append
    `work_opened`, `work_settled` and similar events and receipts
    (`types/lifecycle.go:43-80`). So the work item is already a mutable
    projection beside an append-only event tape.
- **The generic stack (legacy).**
  - Functions: `CreateWorkItem`, `UpdateWorkItemStatus`,
    `UpdateWorkItemDetails`, `FindWorkItemByFingerprint`
    (`internal/store/trajectory.go:153-385`, `graph_store.go:1473-1730`).
  - It hides lifecycle items (`graph_store.go:1550`, `:1569`, `:1593`).
  - Its remaining callers are the dormant wire-publication paths and
    legacy run completion.
- **The SQL table `work_items`** (`internal/store/store.go:379-395`,
  indexes `:527-529`) is touched only by the vocabulary migration
  (`internal/store/vocab_migrate.go:222-241`, `:375`, `:659`, `:735-740`).
  No product path reads or writes it. It is dead DDL.

Size [V, grep]: about 1,900 non-test lines in 62 files match
`WorkItem|work_item`. This includes packet fields such as `TargetWorkItemID`.

### (a) Creation and opening [V]

- **`StartLifecycle` `InitialWork`** (`lifecycle.go:1393-1434`, `:1504`).
  The ids are `uuidSHA1(key+":work:initial")` in
  `textureowner/texture_handoff.go:223`, `texture_lifecycle_create.go:82`
  and `coagent_route.go:166`. `normalizeLifecycleWork` (`lifecycle.go:2779-2794`)
  forces `open` and sets fingerprint = sha256(objective).
- **`OpenLifecycleWork`** (`lifecycle.go:2831-2930`) dedupes by objective
  fingerprint (`lifecycleWorkFingerprintAvailable`, `:2822-2829`). Callers:
  - spawned coagent work (`agentcore/runtime.go:1389-1407`);
  - the owner-revise reopen (`textureowner/texture_agent_revision.go:296-312`).
    It uses a constant objective so only one concurrent reopen wins.
- **Texture-turn openers** (`researcher-work`, `persistent-super-work`):
  - built in `textureowner/texture_turn_runtime.go:320-372`;
  - minted inside the turn or control commit by the control accumulator
    (`store/lifecycle_control_issue.go:203-290`), which stamps
    `requested_by_*` provenance (`:247-251`);
  - P1 mints the cast record in the same batch (`:345-401`).
- **`/management/open`** uses `"mgmt-work-"+commandID`
  (`textureowner/management_open.go:175`, `:196-209`).
- **Engineering assignments** use `"work:"+assignmentID[":attempt-N"]`
  (`agentcore/engineering_assignment_runtime.go:401-411`, `:547`). The
  supervision anchor is `work:texture-supervision:<doc>`
  (`store/engineering_assignments.go:1765-1823`), minted inside the
  report/cancel commits.
- **Legacy wire fingerprints** (`internal/workitem/fingerprint.go:23-60`)
  are used by `agentcore/wire_publication.go`. The processor and story
  paths have no callers. The live entry, `MaybeAutonomousPublishWireArticle`,
  is gated to the platform owner (`wirepublish/eligibility.go:32-55`), so
  it is effectively inert [I].

### (b) Status transitions [V]

These are the only setters that run in production:

- **Texture caller work → completed.** Only alongside a revision outcome
  (`store/texture_turn.go:149-153`, `:464-477`). The disposition comes from
  the model's `ApplyTexture` input (`texture_turn_runtime.go:439-452`).
- **Producer work → completed or refused.** Set when a Texture turn names
  the report as incorporated or rejected (`store/texture_turn.go:199-229`,
  `:542-590`).
  - The default consumption is `delivered`, which leaves the work open
    (`texture_turn_runtime.go:137-150`).
  - This is the "research work never settles" defect [D, cluster doc].
- **Engineering terminal projection** (`engineering_assignments.go:1895-1915`).
- **Cancel all open work on trajectory cancel**
  (`CancelLifecycleTrajectory`, `lifecycle.go:5782-5800`).
- **Owner-revise reopen** when the latest Texture work is completed
  (`texture_agent_revision.go:139-155`, `:271-291`).

`SettleLifecycleWork`, `RefuseLifecycleWork` (`lifecycle.go:5057`, `:5084`),
`AmendLifecycleWork` (`:3293`) and the `ApplyLifecycleUpdate` work
consequences have **no production caller**. Only tests call them.

### (c) Activation authority and run binding [V]

- **Run metadata.** `lifecycle_work_item_id` (Texture, one item) or
  `work_item_ids` (management, research, engineering).
  - `management_controller.go:376-379`, `:2094-2154`
  - `runtime.go:1467`, `:2908-2917`
  - `engineering_assignment_runtime.go:191-197`
  - Texture binds the *first open* item by id
    (`texture_agent_revision.go:500-509`), without excluding the
    supervision prefix.
- **Store gate.** `projectLifecycleRun` requires each bound id to be open,
  assigned to the agent and lifecycle-versioned
  (`lifecycle.go:2949-2980`, `:3118-3160`).
- **Runtime gates.**
  - `ValidateActivationAuthority` (`textureowner/texture_controller.go:389-397`)
  - `ValidateOccurrenceActivationAuthority` (`:422-425`), where the run's
    work must equal the occurrence's resolved work
  - `validateOccurrenceReconciliationCandidates` (`:310`, `:323`)
  - management `validateTargetBoundLifecycleControls`
    (`management_controller.go:1559-1571`)
  - `hydrateLifecycleControlWorkItems` (`:1750-1789`)
  - activation keys that include the work version (`:185-225`)
  - research provider admission (`runtime.go:3113-3160`)
  - engineering overlay checks (`engineering_assignment_tools_overlay.go:47`,
    `:111-125`)
- **Producer report authority.** `lifecycleAssignedWorkItemID`
  (`agentcore/tools_worker_update.go:506-527`) requires open work and exact
  `requested_by_*` provenance (`:375-388`, `:438-455`).
  `commitLifecycleProducerReportAct` re-checks it
  (`store/lifecycle_commit_act.go:487-505`).
- **Control targeting.** `ValidateLifecycleTextureControlTarget`
  (`store/lifecycle_texture_target.go:57-180`) requires the target work to
  be open, assigned and opened by this Texture (`:198-232`).
- **Tool loop.** Whether a run passivates or terminates keys on
  `lifecycle_work_item_id` (`runtime.go:3515`).

### (d) Packet and report binding [V]

- `ResolveLifecyclePacketWorkBindings` (`lifecycle_texture_target.go:250-275`)
  handles only `""`, `producer_report` and `control`, so a `directive` is
  refused. This is the P2d2 blocker.
- Reports carry `WorkItemID` and `WorkDisposition` on the packet
  (`agentcore/rlm_reduce.go:1475-1510`).
- `ApplyTextureTurn` requires the report's target work to equal the
  caller's work (`texture_turn.go:503-516`).
- Occurrence resolution falls back from the occurrence's target work, to
  the active run's work, to the one open Texture item
  (`texture_controller.go:1404-1440`).
- Stale-packet expiry classifies a packet as dead when its work is not open
  (`lifecycle.go:1006-1019`).

### (e) Settlement [V]

- `lifecycleSettlementReady` (`lifecycle.go:2730-2775`) requires every work
  item to be terminal and no packet pending.
- Every document lifecycle sets `RequireNoOpenWorkItems`
  (`texture_lifecycle_create.go:97`, `texture_handoff.go:254`,
  `coagent_route.go:218`, `selfdev_texture_join.go:73`).
- A terminal run requests settlement (`lifecycle.go:3180-3195`). The
  request is consumed by `ReconcileLifecycleSettlementForTerminalRun`
  (`store/store.go:1326-1365`), on each terminal run update and at boot
  (`runtime.go:2510-2514`).
- The legacy `EvaluateTrajectorySettlement` (`agentcore/trajectory.go:86-145`)
  uses the generic stack, so it never counts lifecycle work.

### (f) Crash and boot [V]

- **What a markerless boot closes.** Only actor wakes of work kinds, with
  the fate `interrupted_by_restart`
  (`internal/actorruntime/handler.go:97-118`, `:168-171`).
- **What survives.** No boot path sets a work item to `cancelled`. The
  only `WorkItemCancelled` writers are trajectory cancel and the
  engineering terminal projection. Open work and pending packets survive a
  crash. This contradicts the CLAUDE.md rule, which lists "work item" among
  the obligations a crash closes.
- **Boot paths that *read* open work and act on it:**
  - spawned-work open replay (`runtime.go:2405-2410`);
  - terminal settlement repair (`runtime.go:2510-2551`);
  - wire-publication claim recovery (`wire_publication.go:21-43`);
  - the Texture wake re-arming on open desk work
    (`texture_controller.go:638-660`, `:735-748`).

### (g) Rendering [V]

- **Management and research prompts.**
  `buildAssignedWorkItemPromptForCarrier` (`runtime.go:2953-2995`) tells the
  desk to `choir.Report` with `work_disposition=completed` or `open`. It
  lists work ids and, when set, `step_budget` and `token_budget`.
- **Texture prompt.** It says "You never name work items"
  (`textureprompts/overlays/run_system.yaml:31`).
- **Obligations surface.** Counts open work
  (`agentcore/obligations_surface.go:230`, through
  `ListOpenAssignedLifecycleWorkItems`, `lifecycle.go:1942`).
- **API responses.** `management_open.go:220`,
  `texture_lifecycle_create.go:125`, `texture_handoff.go:286-303`.
- **Owner revise.** Returns 409 when there is no open desk work, or when
  there is more than one (`texture_agent_revision.go:134-160`, `:230-261`).

### (h) Engineering [V]

- **Parent work.**
  - Document cast: the single open engineering work
    (`engineering_assignment_runtime.go:301-314`).
  - Delegated cast: the caster run's `lifecycle_work_item_id` (`:210-225`).
- **Assigned work.** One work item per assignment attempt.
- **Fate path.** Uses `assigned_work_item_id`
  (`engineering_assignment_fate.go:1088`).
- **Resume objective.** Read from the work item
  (`continuation_schedule.go:235`).
- **Supervision item.** Excluded by prefix at
  `texture_agent_revision.go:241`, `:283` and `texture_controller.go:71`,
  `:642`.
- **Closure [I].** The supervision item closes only on trajectory cancel or
  Texture caller completion. If neither happens, it blocks settlement
  indefinitely.

### (i) Budgets [V]

`StepBudget` and `TokenBudget` are **not enforced anywhere**. They appear
only in prompt text (`runtime.go:2983-2989`) and in the dead SQL
DDL/scanner (`store.go:386-387`, `trajectory.go:388`, `:451`). The only
live bound on research is the Texture-side round counter
(`texture_turn_runtime.go:161`, `:187-200`). That counter is already a fold
over the snapshot: controls since the latest owner input.

### (j) Run acceptance [V]

`SynthesizeRunAcceptance` derives from runs and trace events
(`agentcore/run_acceptance.go:58-111`). Neither it nor
`store/run_acceptance.go` reads work items.

### What the inventory says about "is it needed?"

**What work items actually do.** They do six jobs:

1. activation authority;
2. a packet binding anchor;
3. a settlement gate;
4. objective dedupe;
5. owner-revise targeting and reopen;
6. engineering anchors.

None of these needs a *mutable* row. Each is a question about acts: was this
asked, by whom, of whom, is it still owed, what happened.

**The evidence that this layer is the problem.**

- **Mutable copies of record facts.** The mutable status is a second copy
  of facts that records already hold, or should hold. Where the two
  disagree, bugs follow.
- **Five closure bugs in one day** [D: cluster doc;
  `texture-settled-work-refuses-owner-revise`,
  `owner-revision-supervision-desk-target-409`,
  `texture-research-loop-never-idles`,
  `texture-research-opener-dropped-silently`, P2d-review3].
- **About 52 commits since 2026-09-21** whose messages match work, open
  work or reopen [V, git log grep, approximate].
- **Dead code.** Half the transition API is dead (Settle, Refuse and Amend
  have no production caller).
- **Unenforced fields.** The budget fields are not enforced.
- **No crash close.** Work items are not crash-closed.

---

## 2. The record side as built today

### 2.1 What a record carries [V]

`types.CommitmentRecord` (`internal/types/commitment.go:189-252`):

- `RecordID`, deterministic: `cell:kind:local` for desk acts
  (`internal/agentcore/rlm_reduce.go:1084`), `control:<hash>` for P1
  control casts (`internal/store/lifecycle_control_issue.go:338-341`).
- `Kind`: one of precommit, report, resolve, disagreement or directive
  (`commitment.go:41-47`). A directive has a closed `Subtype`: note,
  escalate, cast or retract (`:56-59`).
- `Directive{Subtype, Body, Objective, TargetRef}` (`:66-71`). A Cast is
  `Kind=directive, Subtype=cast, Objective, Body=statement`
  (`rlm_reduce.go:1133-1135`).
- `Precommit{Question, Distribution, Resolver}`. An Ask is a precommit with
  `Resolver=ToDesk` (`rlm_reduce.go:1112-1127`).
- `Resolve{Verdict, EvidenceRefs}` with `Discrepancy` (`:98-101`,
  `rlm_reduce.go:1167-1188`).
- `Addressee` (`:228`), the ledger-side target desk, set from `ToDesk`
  (`rlm_reduce.go:1766-1771`). It is a bare profile ("research") or an
  agent id ("research:<doc>").
- `ParentID`, `ChildIDs`, `RelatedIDs` and `RetrievedByIDs` (`:240-243`).
  Any act with a `TargetRef` gets `ParentID = TargetRef`
  (`rlm_reduce.go:1201-1203`). Report and Reply also set
  `RelatedIDs = [TargetRef]` (`:1145-1154`).
- `EmitterKind` (agent, owner, edition or source), `ComputerID` and
  `SubjectRefs` (`:250-252`, `:262-265`). Only P1 control casts fill these,
  with
  `SubjectRefs{doc_id, trajectory_id, control_id, work_item_id, command_id}`
  (`lifecycle_control_issue.go:373-382`). Desk-authored records built by
  `commitmentRecordForIntent` set none of them (`rlm_reduce.go:1080-1205`).
- **Absent:** a finished/terminal flag on Report, a thread root id, a fate
  act, a system emitter kind, and any `WorkDisposition`. A Report does not
  need a `TargetRef` (`validateSemanticActIntent`,
  `rlm_reduce.go:161-221`, has no Report case requiring one).

### 2.2 How a record wakes its addressee [V]

- **One transaction.** `CommitLifecycleAct`
  (`internal/store/lifecycle_commit_act.go:219-423`) mints the record object
  (kind `choir.commitment_record`), a packet (kind `choir.worker_update`,
  `UpdateID = recordID + ":packet"`, `SourceRecordID = recordID`,
  `Disposition = pending`), a `control_queued` lifecycle event, a
  packet→record edge, a caller-agent version bump and a receipt.
  - If any part fails, the whole act fails (`:14-19`).
  - Records with no addressee are ledger-only (`:86-89`).
- **Producer reports.** These go through `commitLifecycleProducerReportAct`
  (`:429-721`). It requires the producer run to bind the reported work item
  and that work to be open (`:487-505`). It also folds the bound pending
  control into `incorporated` in the same commit (`:620-666`).
- **The wake.** It is derived from the packet, not the record:
  `actorWakeOutboxFromWorkerUpdate` (`internal/store/lifecycle.go:548-599`)
  emits a `coagent_result` wake for any pending packet with a target.
  - The runtime also fires a live trigger after commit
    (`rlm_reduce.go:1922-1927`).
- **The inbox.** `ListPendingLifecycleUpdates` (`lifecycle.go:2101-2140`)
  scans every packet of the owner, capped at 65,536
  (`lifecycle_control_delivery.go:67`), and filters by target and
  `pending`.
- **Texture controls (P1).** The control accumulator mints a cast record
  per control, and the record carries `SubjectRefs.work_item_id`
  (`lifecycle_control_issue.go:345-401`). The work item is opened in the
  same batch (`:203-290`). So record → work is already a recorded edge;
  work → record is not.

### 2.3 Threads and resolution [V]

- `commitmentRecordLinks` is `ParentID` plus `RelatedIDs`
  (`lifecycle_commit_act.go:186-202`).
- `commitmentRecordResolvesTarget` treats a record as closing a target when
  it is a Resolve linking the target (`:204-216`).
- `mechanicalResolveForReport` (`:136-184`) is already a fold. On every
  Report it lists up to 10,000 records (`:144`), finds a linked
  precommit/Ask with no resolve, and mints a `system:reducer` resolve with
  `Discrepancy=answered` in the same transaction.
- So the ledger already derives one status by fold: whether an Ask is
  answered. It does it only for precommits, not for Casts. A Cast's record
  stays `Discrepancy=unresolved` forever. Its "done" lives on the work item
  and on the report packet's `WorkDisposition`.
- **Retract.** `CancelAct` (`IntentCancel`) mints a retract directive whose
  addressee is copied from the target record (`rlm_reduce.go:1839-1843`).
  No code path cancels the target's work item on a retract. Grep for
  `CommitmentDirectiveRetract` finds only record construction and packet
  rendering (`rlm_reduce.go:1136-1138`, `:1684-1687`).

### 2.4 Query cost today [V]

- `ListCommitmentRecords`, `GetCommitmentRecord` and
  `ListCommitmentClaimRecords` use `ogListByOwnerAndBody`
  (`internal/store/commitment.go:83-219`), which becomes a
  `JSON_EXTRACT(CAST(body AS JSON))` predicate over `(object_kind, owner_id)`
  (`internal/objectgraph/dolt_store.go:845-870`).
- Only `run_id`, `trajectory_id` and `channel_id` have generated, indexed
  metadata columns (`dolt_store.go:150-162`). The comment there records
  `JSON_EXTRACT` full scans measured at about 1.4s per call under drain.
- `GetCommitmentRecord` by id is therefore a body scan. So is every
  addressee query. Commitment records do not carry `trajectory_id` in their
  metadata (`lifecycle_commit_act.go:307-317`), so the indexed columns do
  not help.

---

## 3. Each work-item purpose as a fold over records

**Definitions [I].** A *thread* is a root act that asks for work (Cast, Ask
or Escalate) plus every record whose thread root is that act. A record is
*terminal* for its thread when it is one of:

- a Report with `final: true`;
- a Reply to an Ask;
- a Resolve;
- a Retract by the root's emitter;
- a fate record (system emitter: interrupted by a restart, refused,
  trajectory closed, superseded).

A thread is *open* when it has no terminal record. A record addressed to D is
*unconsumed* while no turn receipt of D names it, or while its packet is
`pending`.

| Work-item purpose (today) | As a fold (proposal) [I] | Missing from records | Deleted |
|---|---|---|---|
| Open obligation per desk ("open work for D on S") | `open(D,S)` = root acts addressed to `D:S` with no terminal record | **Subject** on desk addresses (desk acts set only `Addressee`); a **thread root id** stamped by the store; a **terminal flag** on Report | `WorkItemRecord.Status`; `OpenLifecycleWork`; open-work fingerprints (`lifecycleWorkFingerprintAvailable`, `lifecycle.go:2822`) |
| "Cast to open work continues it" (design §3) | At commit, if `open(D,S)` is non-empty, stamp the new Cast's thread root = that root, else = itself | Same thread root id | `target_work_item_id` plumbing; `ValidateLifecycleTextureControlTarget` work checks (`lifecycle_texture_target.go:157-169`, `:198-208`) |
| Activation authority (a run must bind open work) | A run is authorized for D while `open(D,·)` ∪ `unconsumed(D)` is non-empty. The run binds a **consumed-through watermark**, not a work id | Per-addressee sequence; P2d already found the cross-trajectory numbering defect | `lifecycle_work_item_id` / `work_item_ids` checks; `ValidateActivationAuthority` work clause (`texture_controller.go:389-397`) |
| Packet → work binding (producer, target, control binding) | Packet → `SourceRecordID` (exists); record → thread root | Thread root id | `ResolveLifecyclePacketWorkBindings` (`lifecycle_texture_target.go:250-275`); `ProducerWorkItemID`, `TargetWorkItemID`, `WorkDisposition` on packets |
| Completion ("research work never settles", cluster item 4) | Thread closes on its own terminal record: the answering desk's `final` Report or the caster's Retract | `Report.Final` (today it is the packet's `WorkDisposition`, invisible on the ledger) | `SettleLifecycleWork`, `RefuseLifecycleWork`, `AmendLifecycleWork`; the inbound-disposition work consequences in `ApplyTextureTurn` |
| Progress | `progress(T)` = ordered Reports and Notes in thread T: count, last-report age, new evidence refs per report (marginal yield), revisions citing the thread | Nothing: all of it is derivable once thread ids exist | Nothing to delete (no progress tracking exists today) |
| Crash close at a markerless boot | For each open thread: append a fate record "interrupted by a restart" (system emitter, deterministic id `root:fate:boot:<bootID>`) and terminalize its pending packets. A planned-restart marker skips this | Fate act and system emitter kind (only agent, owner, edition and source exist, `commitment.go:262-265`) | Nothing today closes work at boot, so this **closes a gap** rather than deleting one (§1(f)) |
| Trajectory settlement (`RequireNoOpenWorkItems`) | `settled-ready` = no open thread rooted in the trajectory's subject and no unconsumed record. Or drop settlement for documents: [I] a live document should always be revisable (cluster option C) | None | `lifecycleSettlementReady` work loop (`lifecycle.go:2730-2758`) |
| Owner-revise target and reopen (`ownerRevisionTarget`, `textureWorkSettledByDesk`, `reopenTextureWorkForOwner`) | Owner input stays on the revision chain (owner decision: owner revisions do not become records yet). The Texture inbox fold is "owner head moved since my last turn" ∪ unconsumed records. Nothing reopens | None | All three functions, and the 409 "multiple open desk target work items" class |
| Supervision anchor `work:texture-supervision:<doc>` | Engineering reports are records whose thread root is the Cast that opened the assignment. Texture (via management, per the owner's routing answer) reads them as records | Thread root id | `TextureSupervisionWorkItemPrefix` and every exclusion site that names it |
| Engineering assignment parent/assigned work | `EngineeringAssignment` stays: it holds real external state (capsule disposition, grants). Its `ParentWorkItemID`/`AssignedWorkItemID` become thread root ids | Thread root id | Two work items per assignment |
| Budgets (`StepBudget`, `TokenBudget`) | None. The owner rejects budgets. See §1(i) for whether anything enforces them | None | The fields |
| Rendering ("what is owed") | The obligations surface counts open threads by addressee and age instead of open work rows | Index | `ListOpenAssignedLifecycleWorkItems` (`lifecycle.go:1942`) |

**What genuinely stays mutable [I].**

- **Delivery state.** A packet's `pending → delivered → incorporated`
  disposition and the wake outbox derived from it. This is per-recipient
  delivery, not an obligation. It could later shrink to a per-agent
  consumed watermark, but it works today and P1 already derives it from
  records.
- **Run state.**
- **The engineering assignment's capsule state.**

Work items are the only layer that is both mutable and duplicates what the
records say.

---

## 3b. What a Cast carries, and how progress and closing work (owner notes, 2026-10-12)

The owner's direction:

- A Cast carries the caster's **hypotheses and uncertainties**: what it
  believes, how confident it is, and what would change its mind. A bare
  "find X, p=0.8" is not enough.
- Progress Reports update those hypotheses, and closing resolves them.
- A Cast may also carry a **catch-all hypothesis**: "there are important
  things I haven't mentioned", with a probability. It resolves true if a
  Report brings a finding outside the listed hypotheses that the caster, or
  a third party (never the reporter), judges material. Material means the
  caster revises its lede or structure, or moves a stated probability by
  more than a set delta. The resolution must link the catch-all to the
  Reports and revisions that triggered it.

### What exists [V]

- **The schema already has the slots.**
  - `CommitmentPrediction{Hypothesis, Assumptions, Alternatives, Questions[]TypedQuestion, CommittedAt}`
    (`internal/types/commitment.go:115-123`).
  - `TypedQuestion{Question, Answers, Probabilities, Weight}` (`:75-83`).
  - `CommitmentRevision{Weakened, Strengthened, Hypothesis, RevisedAt}`
    (`:162-169`).
- **Casts do not use them.** A desk Cast fills only `Directive{Objective, Body}`
  (`internal/agentcore/rlm_reduce.go:1133-1135`). A P1 control cast fills
  only the directive and `SubjectRefs`
  (`internal/store/lifecycle_control_issue.go:358-382`). No Cast record
  carries a prediction today.
- **The Expectation is still a draft.** The design §4 `Expectation`
  (1 to 4 typed questions, `By`, `Resolver`) is drafted for P3 and not
  built.
- **The existing revision field cannot be used.** `CommitmentRevision` is a
  field *inside* the record. Design §10 says a record is immutable once
  committed and that scores are separate records. So a hypothesis update
  cannot write into the Cast record. It has to be its own record.

### Proposed shape [I]

**Cast (root of a thread).**

- `Prediction.Hypothesis`: the caster's working belief, in prose.
- `Prediction.Questions`: the hypotheses H1..Hn, each a TypedQuestion with
  a probability. They need a **stable `id`**, which `TypedQuestion` lacks
  today. Answering records cannot link to a question without one.
- `Prediction.Assumptions`: the uncertainties the caster is resting on.
- **Cruxes**, one per hypothesis: what would change the caster's mind (a
  new field, or `Answers` used as the named outcomes). Nothing in the
  schema holds this today.
- **H0, the catch-all.** A TypedQuestion with a reserved kind `catch_all`
  ("material findings outside H1..Hn exist", p), plus the materiality rule
  it will be judged by: a lede or structure change, or |Δp| ≥ δ on any
  Hi. δ is a configured constant.

**Report (progress, from the answering desk).**

- `final` (bool): true ends the thread.
- `findings[]`, each tagged `bears_on: Hi | outside`, with evidence refs.
- Optionally the *reporter's* own probability for each Hi. These are its
  view, and they never trigger the catch-all.

**Belief update (new, from the caster or a third party).** This is a
separate record whose thread root is the Cast:

- `updates[]`: Hi with p_before and p_after;
- `cites[]`: the Report ids that moved it;
- optionally the Texture revision id whose lede or structure changed
  because of them.

For Texture, the turn that commits the revision can mint this record in the
same transaction. The turn already lists the inbound packets it consumed
(`TextureTurnRecord.InboundUpdateIDs`, `internal/types/lifecycle.go:296-308`),
so `cites` can be derived, not authored.

**Materiality trigger for H0 (new).** A record of kind resolve (or a
`materiality` subtype) linking:

- `H0 (question id on the Cast)` → `the Report(s) with an "outside" finding` → `the belief update and/or the revision that changed`.

The store refuses it if the judge's emitter equals the emitter of any cited
Report. That enforces "never the reporter".

It can be minted mechanically, as `mechanicalResolveForReport` already
mints `system:reducer` resolves (`lifecycle_commit_act.go:136-184`) [V]:
when a belief update cites a Report with an `outside` finding and either
|Δp| ≥ δ or a structural revision. A third party (management or the owner)
can also mint one by judgment.

**Close.**

- The thread is closed by the reporter's `final` Report, the caster's
  Retract, or a fate record.
- At close, a resolve record per hypothesis (or one with per-question
  answers) compares the caster's last belief and the evidence against the
  frozen Cast probabilities.
- **H0 resolves by count, with no model call.** It is true if at least one
  materiality trigger exists in the thread, and false otherwise. Its
  resolve record lists the trigger ids, so the link chain
  `H0 → trigger → Reports → revision` survives in the ledger.
- A fate close ("interrupted by a restart") resolves every open Hi and H0 as
  `unresolved`, with the fate as the observation.

### What this changes in the fold (§3)

- **Progress is belief movement, not source counts.** For a thread, the fold
  returns:
  - each Hi's probability path (from belief updates);
  - |Δp| per Report;
  - the outside-finding count;
  - the H0 trigger list.
- **"Diminishing returns" becomes measurable without a budget.** Belief has
  stopped moving (Σ|Δp| over the last k Reports is near 0), and no new H0
  trigger has appeared. The reporting desk still makes the call to send
  `final`. Supervisors see the same series.
- **Missing pieces now include:**
  - question ids on `TypedQuestion`;
  - a cruxes slot;
  - the `catch_all` question kind with its δ;
  - `bears_on` tags on Report findings;
  - a belief-update record;
  - the materiality-trigger record and its not-the-reporter rule.

  All are additive JSON. They must be settled before stage 2 writes
  them, because records become cross-computer track record (design §10).

---

## 4. Worked example: record status as a query

Owner opens document D ("What changed with Haiku 5.5?"). The ids are
illustrative. δ = 0.15.

| # | Record | Emitter → addressee | Thread root | Notes |
|---|---|---|---|---|
| 0 | owner revision v0 (revision chain, not a record) | owner → D | — | Texture inbox fold: owner head moved |
| 1 | `c1` Cast. Hypothesis: "5.5 is a speed/price release, not a capability jump". H1 an official announcement exists, p=0.85 (crux: none found within the vendor's own channels). H2 price per token fell, p=0.6 (crux: a pricing page showing the same price). H3 benchmark gains under 5 points, p=0.55 (crux: a vendor eval table). Assumption: press coverage reflects the vendor post. **H0 catch-all**, p=0.3 | texture:D → research:D | c1 (opens) | no open thread for research:D, so c1 opens one |
| 2 | T1: revision v1 "researching…"; consumes owner head v0 | — | — | Texture inbox empty |
| 3 | `p1` Report: vendor post found (bears_on H1); final=false | research:D → texture:D | c1 | progress |
| 4 | `p2` Report: pricing page, price down 40% (H2); final=false | research:D → texture:D | c1 | progress |
| 5 | T2 consumes p1 and p2; revision v2; belief update `u1`: H1 0.85→0.99, H2 0.6→0.95, cites p1 and p2 | texture:D (ledger) | c1 | progress = belief moved |
| 6 | `c2` Cast "check the eval table too" (adds a crux for H3) | texture:D → research:D | **c1** (continues) | the thread is open, so c2 joins it |
| 7 | `p3` Report: eval table shows +11 points on coding (H3); **outside**: a new agent-tool API shipped alongside | research:D → texture:D | c1 | — |
| 8 | T3 consumes p3; revision v3 **rewrites the lede** around the tool API; `u2`: H3 0.55→0.1, cites p3 | texture:D | c1 | — |
| 9 | `t1` materiality trigger for **H0**: links H0 → p3 (outside finding) → u2 (\|Δp\|=0.45 ≥ δ) and v3 (lede change); judge = system:reducer, not research:D | system:reducer | c1 | H0 has fired; the link is recorded |
| 10 | `m1` Cast "story X has 4 new captures" | management:owner → texture:D | m1 | P2d2 case: just another record addressed to texture:D |
| 11 | `p4`…`p6` Reports, minor detail; T4 and T5 consume them with m1; `u3`, `u4` with Σ\|Δp\| = 0.03 | — | c1 | belief has stopped moving |
| 12 | `p7` Report final=true: "last three reports moved nothing; the outside finding is covered" | research:D → texture:D | c1 | terminal |
| 13 | `r1` resolve: H1 true (confirmed), H2 true (confirmed), H3 false (contradicted), **H0 true, triggers [t1]**; discrepancy qualified | system:reducer / scorer | c1 | closed and scored |

Status of c1, as queries over this tape:

| Asked at | Query result |
|---|---|
| after 1 | `open`: 0 reports, beliefs at the Cast values |
| after 5 | `in progress`: 2 reports; H1 0.99, H2 0.95, H3 0.55; Σ\|Δp\| last report 0.35 |
| after 9 | `in progress`: H3 0.1; **H0 triggered by t1 (p3 → v3 lede)**; steered by c2 |
| after 11 | `in progress, converging`: Σ\|Δp\| over the last 3 reports is 0.03 |
| after 12 | `answered` (final report p7) |
| after 13 | `resolved`: 2 confirmed, 1 contradicted, catch-all fired. The caster was surprised, so a lesson is owed (design §5) |
| if the computer crashed between 9 and 12 | `interrupted by a restart`: fate record f1 is appended at boot, the thread closes, and the open Hi resolve `unresolved` (H0 stays true, because t1 already exists). Texture sees f1 on its next input and may Cast again |
| if Texture retracted c1 at 11 | `retracted`; hypotheses are resolved on the evidence so far, or left unresolved |
| if open past `By` | `overdue`: a derived label, with no record and no budget |

Texture's own state is also a query:

- **owed a turn** when the owner head is newer than its last consumed
  head, or when an unconsumed record is addressed to it;
- **idle** otherwise.

The document is busy while any thread rooted in its subject is open.

**No budgets [I].**

- **Who decides to stop.** The research desk ends with `final` when its own
  judgment says returns have diminished. The prompt already asks for this:
  "Stop when marginal returns diminish"
  (`internal/textureprompts/overlays/run_system.yaml:31`) [V].
- **What the ledger supplies.** The signal is now explicit: belief movement
  per Report, and H0 triggers.
- **What replaces a cap.** Supervisors, not caps. Texture or management
  reads the thread's belief path in its pack and can Retract c1, or Cast a
  steer into the same thread.
- **What disappears.** Today's research-round bound
  (`textureResearchRoundsPerOwnerRequest = 4`,
  `internal/textureowner/texture_turn_runtime.go:161`) [V] exists because
  closure was invisible (cluster item 5). It becomes a supervision choice,
  not a runtime cap.
- **Silence.** A thread with no record for a while is shown as silent: a
  derived fact. The engineering progress-overdue observation is precedent
  (`EngineeringProgressReviewWindow`, `lifecycle.go:535`) [V]. It is not a
  budget, and it ends nothing.

---

## 5. P2d2: record inbox versus shape D

**Why P2d2 is blocked.**

- **The work-item mismatch.** A Texture run binds exactly one work item. A
  directive occurrence must resolve to that same work. And the store turn
  commit rejects any directive packet, because
  `ResolveLifecyclePacketWorkBindings` has no `directive` case.
  - Verified: `lifecycle_texture_target.go:250-275`. The switch handles
    `""`, `producer_report` and `control`, and the default returns an
    error.
  - The inbox doc probe-confirmed this.
- **The idle Texture.** An idle Texture whose request work is completed has
  nothing to bind a run to [D].

**Shape D** (directive-inbox doc §3-4) keeps work items. It adds:

1. a join-or-reopen of request work inside `CommitLifecycleAct`;
2. a completion guard, so a turn cannot complete W while a directive is
   pending, enforced in both store and runtime and relying on a trajectory
   CAS bump that costs every in-flight turn one retry;
3. a dead-work candidate rule in two places;
4. a sequence check;
5. a shared reopen-objective constant.

Each one exists only to keep "the run's one work item" coherent with "inputs
that name no work".

**Record-inbox shape (call it R) [I].** The Texture desk's obligation is the
fold from §4: owner head unconsumed, or a record addressed to `texture:<doc>`
unconsumed.

- A directive is a record plus a packet, as P1 and P2 already mint them. It
  needs no work object, no join and no reopen.
- **Activation.** A Texture run is authorized while its inbox fold is
  non-empty. The run's `scheduled_message_seq` is already the consume
  watermark (the inbox doc §1 says the turn consumes every pending packet
  up to it, whatever woke it [D]).
- **The completion guard disappears.** There is no "completed" to set. A
  directive that commits during a turn is simply unconsumed after it. Its
  packet stays pending, the wake re-arms, and the next turn takes it. No CAS
  bump is needed to make a guard hold.
- **Settlement** is a fold: no open thread rooted in the document and no
  unconsumed record. Alternatively, documents do not settle at all (open
  owner question 3 in the inbox doc).
- **The turn contract can stay intact in the first step.** Texture keeps
  one *standing* per-document anchor in place of request work items:
  - `CallerWorkItemID` is the standing anchor, and the disposition is
    always `open`;
  - owner revise stops opening and reopening Texture work.

  This is shape A from the inbox doc. A was rejected for its nine exclusion
  sites and because it never settles. Those costs exist because A
  *coexists* with request work items. Under R the request items are
  removed:
  - `ownerRevisionTarget`, `textureWorkSettledByDesk`, the initial-work
    wake, the resolver fallback and submit's selection become "the standing
    anchor" or the fold;
  - settlement moves to the fold.

  The exclusions become deletions [I].
- **Sequence numbering.** The cross-trajectory sequence defect P2d found
  (management's packet numbered on the caller's trajectory) still has to be
  fixed under R. R makes the sequence per addressee, not per caller
  trajectory.

**Comparison [I].**

| | D | R (Texture slice) |
|---|---|---|
| New mechanisms | 4 to 5 (reopen-in-act, completion guard ×2, dead-work rule ×2, sequence check) | 1 (inbox fold predicate) |
| Deleted | none | Texture request work items; owner reopen; the 409 class; supervision prefix exclusions |
| Turn-contract change | none | none in step 1 (standing anchor); later delete `CallerWorkItemID` |
| Prerequisites | none | thread id, index columns, fate act (stages 1 and 2 below) |
| Time to P2d2 | shortest | one or two stations longer |
| Rollback | revert, plus drain directive packets | revert, plus drain; standing anchors read as ordinary open work by old code (needs the A-style exclusions in the *old* code, so the rollback is not free) [I] |

**Recommendation [I].**

- **Default.** If Autopaper W2 must ship this week, land D, but label its
  reopen and completion guard as scheduled for deletion by stage 3.
- **Otherwise (preferred).** Follow CLAUDE.md "prefer deletion over
  addition" and the clustering rule (five or more symptoms on one
  substrate): make P2d2 stage 3 of the migration, preceded by stages 1 and
  2.
- **Either way.** Do not add a new work-item mechanism (A, B or C) beyond D.

---

## 6. Risks and a staged migration

### Risks

- **Fold performance [V/I].**
  - Record queries are body `JSON_EXTRACT` scans (§2.4).
    `mechanicalResolveForReport` already scans up to 10,000 records on
    every Report commit (`lifecycle_commit_act.go:144`) [V]. That is a live
    O(N) per commit before any migration.
  - The packet inbox is a 65,536-row owner scan
    (`lifecycle.go:2101-2125`) [V]. So the work-item world is not cheap
    either.
  - Mitigations:
    - generated, indexed metadata columns for `addressee`, `thread_id` and
      `record_kind` (the pattern already exists, `dolt_store.go:150-200`);
    - a per-thread head row written *only* by the record-commit
      transaction, as a pure function of records. It records open or
      closed, last seq and counts.
  - That head row is a cache with one writer, rebuildable and audited by a
    rebuild-and-compare check. It is not an authority. Without the
    single-writer rule it would become a work item again.
- **Crash-close correctness [I].**
  - The fate pass must run after the planned-restart marker is consumed and
    before any wake dispatch.
  - It must be idempotent (deterministic fate id per boot).
  - It must also terminalize the pending packets of closed threads.
    Otherwise the outbox sweep re-dispatches them. The inbox doc names this
    window as a hypothesis (§5, last bullet) [D].
  - Today crash close acts only on wakes
    (`internal/actorruntime/handler.go:97-118`, `:168-171`) [V]. Work items
    and packets survive a crash [D, and no boot writer of `cancelled`, V].
    So stage 4 *adds* closure that CLAUDE.md already requires, and it will
    surface threads that used to linger silently.
- **Migration of existing rows [I].**
  - Many open work items have no root record: pre-P1 controls,
    owner-opened Texture work and supervision items.
  - Follow the protocol design's §7 rule, "nothing migrates": at cutover,
    one boot pass closes every open work item with the visible fate
    "superseded by record-native work" and records one fate observation per
    item.
  - Post-P1 work items can be joined to their cast record through
    `SubjectRefs.work_item_id` for the shadow audit.
- **Run acceptance [V].** `SynthesizeRunAcceptance` derives from runs and
  trace events (`internal/agentcore/run_acceptance.go:58-111`).
  `internal/store/run_acceptance.go` has no work-item reference. Coupling is
  low, but the surface is red. Re-run acceptance on a demo before and after
  each runtime stage.
- **Evidence ledger schema [V/I].** The new record fields are additive JSON.
  The body accepts future fields (`commitment.go:84-86`). But records are
  becoming cross-computer track record (design §10), so the thread id,
  `final` and the fate act must be settled before they are written. They
  cannot be re-keyed later.
- **Model cooperation [I].** `final` is a desk choice, and the cluster doc's
  lesson (O24) is that closure must not depend only on model cooperation.
  - The fold still closes threads without the desk: by Retract
    (supervisor), by fate (crash, refusal) and by trajectory close.
  - A never-finalized thread stays visibly open and silent. It does not
    hold anything that matters, because nothing gates on work-item status
    any more except the inbox fold, and the inbox fold is about *unconsumed
    records*, not open threads.

### Staged migration

| Stage | Change | Mutation class | Protected surfaces |
|---|---|---|---|
| 0 | Decision doc: fold semantics, terminal set, thread id rule, fate act, standing Texture anchor; owner answers on settlement and owner-revision records | green | — |
| 1 | Shadow fold: read-only `OpenThreads(addressee)` / `ThreadStatus(root)` from records, plus an audit endpoint comparing the fold with work items on staging; generated index columns for `addressee`, `record_kind` | orange (read API, index DDL); rollback is a revert, and the columns are inert | none written |
| 2 | Record schema: the store stamps `thread_id` at commit (continue-if-open rule), `Report.final` (set from the packet's `work_disposition=completed` while both exist), a `fate` directive subtype or resolve verdict, a `system` emitter kind; hypothesis-bearing Casts (question ids, cruxes, `catch_all` H0 with δ), `bears_on` tags on Report findings, belief-update records (Texture's turn mints them, citing consumed packets), and the materiality-trigger record with its not-the-reporter rule. Dual-write: work items stay authoritative | red | evidence ledger |
| 3 | Texture inbox fold, which **is P2d2**: Texture authority from the fold, a standing per-document anchor, directive packets consumed with no work, owner revise stops opening and reopening Texture work, the supervision prefix deleted | red | Texture canonical writes (turn contract), run acceptance re-check |
| 4 | Crash close by fate records at a markerless boot for every open thread, with packets terminalized; skipped on a planned restart | red | evidence ledger, lifecycle |
| 5 | Research, management and engineering: runs bind thread ids, producer reports carry `final` on the record, `WorkDisposition`/`TargetWorkItemID`/`ProducerWorkItemID` are retired, settlement is a fold, existing open work is drained with a fate; Assign folds into Cast (assign review doc) | red | evidence ledger, run acceptance, engineering capsule authority |
| 6 | Delete the `work_items` SQL DDL, `WorkItemRecord`, budgets, `OpenLifecycleWork`, `SettleLifecycleWork`, `RefuseLifecycleWork`, `AmendLifecycleWork`, `lifecycleWorkFingerprintAvailable`, and the work-item kinds in the replay table | orange (deletion), red where it touches lifecycle replay | lifecycle replay/decode (retired event kinds stay decodable, as `LifecycleOwnerInstructionQueued` shows, `types/lifecycle.go:83-87`) |

Every runtime stage runs the Landing Loop and a deployed demo, with a trace
audit that joins record → packet → turn receipt → revision.

**Heresy delta for this research.**

- **Discovered:**
  - work items are not crash-closed [V];
  - Cast records never close on the ledger [V];
  - every Report commit scans up to 10,000 records [V];
  - a retract does not cancel the retracted work [V];
  - `SettleLifecycleWork`, `RefuseLifecycleWork` and `AmendLifecycleWork`
    have no production caller [V];
  - `StepBudget` and `TokenBudget` are not enforced and appear only in
    prompt text [V];
  - the SQL `work_items` table is dead outside the vocabulary migration [V];
  - Texture's submit selects the first open work without excluding the
    supervision prefix [V, also noted in the inbox doc];
  - no Cast record carries a prediction, even though the schema has the
    slots [V].
- **Introduced:** none.
- **Repaired:** none.
