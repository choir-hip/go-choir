---
definition_version: 4

# R2: the restart rule in both directions. Station of
# definitions/choir-reeng-metamission-2026-10-12.md (R1 truthful prompts
# and deletions, R2 restart rule, R3 records as work, R4 async search and
# research, R5 Texture, R6 one document model). Owner-agreed design:
# docs/re-engineering-decisions-2026-10-12.md §1.6, §2, §9, §10.2.
# Problem record (documented first, 486f9201):
# docs/problems/crash-restart-leaves-work-open-and-recasts-engineering-2026-10-12.md
readiness: drafted

review:
  reviewer: none
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
      The marker exists and is consumed once: the host writes it before a
      planned restart (platform update, agentcore/platform_update.go:263;
      self-development apply and rollback,
      self_development_materializer.go:315, :390); boot consumes it
      (autoputer/run.go:531-537, planned_restart.go:94-121) and every
      closer reads rt.BootWasPlannedRestart() (planned_restart.go:147).
    - >-
      A markerless boot refuses pre-boot actor wakes of work kinds with
      interrupted_by_restart (actorruntime/handler.go:97-118, :168-171), and
      closes self-development operations and source schedules
      (self_development_materializer.go:88-112, source_schedule.go:214-243).
      Those two closers are the pattern R2 extends.
    - >-
      It does not close the obligations behind the wakes. Boot passivates
      pending/running runs as "runtime_restarted" (resumable, not closed) and
      ensures a spawned work item for research, management and engineering
      runs (runtime.go:2382-2425, :2404-2405). No boot path cancels a work
      item or a pending packet; the only lifecycle writer of a cancelled work
      item and a cancelled packet is trajectory cancel
      (store/lifecycle.go:5782-5826).
    - >-
      What survives is re-dispatched on the next post-boot wake, not at boot:
      the Texture wake reconcile arms pre-crash pending packets and stranded
      deliveries (textureowner/texture_controller.go:615), wakes on open
      Texture work (:638-660), and reactivates the passivated run (:694,
      :942). The Texture boot itself does nothing (:128-132).
    - >-
      Engineering resumes at boot. The capsule sweep cancels any assignment
      whose capsule is absent with reason "restart revoked absent assignment
      capsule" and wakes the parent with the cancel packet
      (engineering_assignment_fate.go:530-556); the Texture boot then calls
      ReconcileEngineeringDesk for every engineering desk subject
      (texture_controller.go:110-125), whose restart-recast branch re-opens
      the cast at attempt n+1, up to 3, with no marker check
      (engineering_desk.go:44-50, :213-279). Capsules are tmpfs, absent after
      any guest restart (problems/m11-restart-passivation-cancels-desk-strands-op-2026-09-27.md).
    - >-
      Owner revise reopens Texture work only when the latest Texture work
      item is completed; a cancelled item answers 409 "no open desk target
      work item" (textureowner/texture_agent_revision.go:134-160, :263-291).
      The document response reports an interrupted turn only from a
      passivated run (textureowner/texture.go:1055-1088).
    - >-
      Records have no fate act and no system emitter
      (types/commitment.go:41-71, :262-265). Study B §6 warns the record
      fields cannot be re-keyed once written; they are settled in R3.
    - >-
      Wire-publication claim recovery cancels the trajectory of any open
      wire_publication work item on every boot (wire_publication.go:21-43):
      a closer on the dormant legacy stack, not a re-dispatcher.
    - >-
      The owner restart route stops and re-resolves the VM without a marker
      (proxy/computer_lifecycle.go:147-157), so it is a no-SSH crash boot.

finish:
  deliver: >-
    A crash never resumes work and a planned update restart resumes it
    once. After a markerless boot every obligation that existed before the
    boot (run, engineering assignment, work item, pending packet) is
    terminal with the visible fate "interrupted by a restart", nothing
    created before the boot dispatches afterwards, and the owner can still
    revise every document that was busy. After a boot with a marker the
    pre-boot work resumes once.
  artifact: >-
    One crash-close boot phase, gated on the marker and run after every
    boot phase that can mint a packet and before the projector starts
    (runtime.go:748-776): (1) pre-boot pending/running runs end in a
    terminal state with fate interrupted_by_restart instead of passivated,
    and mint no spawned work item; (2) every non-terminal engineering
    assignment is revoked and cancelled with the fate reason, usable
    capsule or not, and its cancel packet does not wake anyone; (3) every
    open work item and pending packet created before the boot is
    terminalized with that reason by one lifecycle command per trajectory
    (and per directive inbox), reusing the trajectory-cancel loops without
    cancelling the trajectory; (4) the restart recast matches only the
    planned-boot reason, so a crash-cancelled assignment stays dead;
    (5) owner revise reopens Texture work whose latest item was closed by
    the fate, and the document response reports the fate from the terminal
    run. Records are not written: R3 stage 4 replaces (3) with fate
    records and R3 stage 6 deletes it with work items (named deletions).
  acceptance:
    - >-
      Staging crash: on a disposable test computer, open a Texture turn, a
      research Cast in flight, and engineering work on both paths (A:
      management delegated cast; B: document cast, M11 probe), then call
      the owner restart route. Through the product API only: every pre-boot
      run, assignment, work item and pending packet is terminal with fate
      interrupted_by_restart; no assignment attempt n+1 exists; no run is
      created in a 10-minute quiet window; the Texture document reports the
      interruption. Artifact: a JSON receipt of before/after obligation
      counts by kind, with ids.
    - >-
      Staging owner path after the crash: an owner revise on the busy
      document returns 2xx (not 409) and starts exactly one Texture turn.
    - >-
      Staging planned restart: a restart carrying the marker (platform
      update or self-development apply) on a test computer with pre-boot
      work delivers each pre-boot wake once and recasts a restart-cancelled
      assignment once; a markerless boot right after closes it with the
      fate. Artifact: the same receipt shape.
    - >-
      Idempotence: a second markerless boot changes nothing and writes no
      second fate (deterministic command ids per item).
  rollback: >-
    git revert of the station commits. Obligations already closed stay
    closed (the rule's intended state). After a revert, crash-cancelled
    assignments carry a reason the old recast ignores (safe: they stay
    dead), but old owner revise answers 409 on Texture work closed by the
    fate; the revert must keep the reopen predicate or the owner restarts
    those documents.
  landing:
    required_receipts: [problem_doc_update, pushed_commit, ci, deploy, environment_identity, deployed_acceptance, consensus_review]

value:
  better_means: >-
    the count of pre-boot obligations that remain open, or dispatch, after
    a markerless boot goes to zero, while a planned boot still resumes once
    and the owner can still revise every document
  goodharting_would_be: >-
    closing work-item rows while a passivated run or pending packet still
    re-arms on the next wake; closing so broadly that a document refuses
    the owner (409) or its trajectory is cancelled; proving it with a
    simulated boot in a unit test instead of a staging restart
  realism_axis: >-
    focused tests of each closer under a markerless and a marked boot ->
    staging owner restart of a test computer with one Texture turn open ->
    plus research and engineering paths A and B open -> staging planned
    restart with marker -> the owner computer after its next real crash
  mutation_class: red
  authority_sources:
    - CLAUDE.md "Restarts End Work (Crash) Or Resume It (Planned Update)" (owner, 2026-10-09)
    - docs/re-engineering-decisions-2026-10-12.md §1.6 (rule unchanged), §2 (crash boot appends a fate; work is records), §9 (path B dissolves, deleting the recast)
  ceremony:
    conjecture_delta: >-
      from "a crash closes wakes, so it ends work" to "a crash ends work
      only when every obligation behind a wake is terminal, because the
      next wake re-arms whatever is left open"
    protected_surfaces:
      - run lifecycle and boot (internal/agentcore/runtime.go Start, passivateInterruptedActivations)
      - lifecycle ledger: work items, packets, lifecycle events (internal/store/lifecycle.go)
      - engineering capsule authority (engineering_assignment_fate.go, engineering_desk.go)
      - owner revise authority (internal/textureowner/texture_agent_revision.go, texture.go)
    admissible_evidence: >-
      staging product-API reads after a real owner restart and a real
      marked restart of a test computer, joined to trace and lifecycle
      events; unit tests shape each closer but prove nothing on their own
    heresy_delta:
      discovered:
        - crash boot passivates runs instead of closing them, and ensures spawned work items for them (runtime.go:2404-2405)
        - the restart cancel of an assignment wakes its parent after boot (engineering_assignment_fate.go:555)
        - owner revise refuses a document whose Texture work was cancelled (texture_agent_revision.go:263-291)
        - pre-crash packets and passivated runs re-dispatch on the next post-boot wake (texture_controller.go:615, :638-660, :694)
        - wire-publication claim recovery cancels trajectories on planned boots too (wire_publication.go:21-43)
      introduced:
        - a temporary work-item and packet closer; deletion named in R3 stages 4 and 6
      repaired: []
  must_preserve:
    - one resume per planned restart; the marker is consumed before resuming
    - the owner can revise every document after a crash
    - nothing is closed silently; every closed item names the fate
    - only Texture writes canonical revisions; R2 writes none
    - no new record kinds before R3 settles the schema
  excluded:
    - fate records and the record schema (R3 stage 2 and 4)
    - deleting path B (decisions §9; R7 in the metamission draft, intent, after R3)
    - the wire-publication legacy stack (deleted with the old Universal Wire)
    - hibernate and wake, and a management-owned resume subsystem (CLAUDE.md "Later")
  read_only_dependencies: [internal/actorruntime/handler.go, internal/agentcore/planned_restart.go, internal/types/commitment.go]

now:
  status: drafted
  slice: authored from the problem record, studies B §1(f) and G §4, and code reads at fbb58c23; nothing built
  source_ref: fbb58c23
  deploy_identity: staging b172895b
  candidate: none
  conjecture:
    id: R2-1
    claim: >-
      If a markerless boot leaves no pre-boot run, assignment, work item or
      pending packet open, and the recast ignores crash fates, then no
      pre-boot work dispatches after a crash, crash-restart loops are
      impossible by construction, and R3 can swap the closer for fate
      records without changing what the owner sees.
    test: the staging crash receipt (zero open pre-boot obligations, zero runs in the quiet window) and the planned-restart receipt
    edge: missing_oracle
    delta_o: >-
      an end-of-boot count of pre-boot obligations by kind, logged and
      readable through the obligations API, so a re-dispatcher missing from
      the list above shows up as a non-zero count
    scope_if_supported: guest crash boots of user computers on staging; not hibernate or host-only restarts
    status: proposed
    evidence_refs: []
  decision:
    what: >-
      Close with existing terminal states and the reason
      interrupted_by_restart now; do not write fate records (no fate act or
      system emitter exists, and record fields cannot be re-keyed later).
      Gate the recast by the cancel reason written at the source, not by
      deleting path B (about 800 lines and the M11 entry move, out of
      scale for this station; the metamission draft gives it to R7, after
      R3). (O1) settled: the owner agreed path B dissolves (decisions
      §9), so R7 deletes it with the recast and R2's reason gate is
      interim. (O2) open: whether a crash closes pending finished reports
      (research results not yet read by Texture) or only obligations to
      do work. Default: close their delivery packets, rule literal; the
      report records stay on the ledger and remain readable on Texture's
      next owner-started turn, and the R3 inbox fold decides re-surfacing.
    kind: design
    status: proposed
    evidence_ref: this file, start.observed
  belief:
    believed_state: wakes are crash-closed; runs, assignments, work items and packets are not, and engineering recasts at boot
    main_uncertainty: >-
      whether a planned restart can reach a busy test computer through a
      product path (deploys skip busy computers); if not, the planned half
      is proved on an idle computer with a pending wake plus the recast
      test, and that edge is named for the owner
    next_observation: the staging crash receipt before any code (baseline counts after an owner restart)
  blocker_or_risk: red station; review before execution
  next_action: >-
    Review this file. Then, before code: append the five discovered
    heresies to the problem doc (Problem Documentation First), and take the
    baseline crash receipt on a staging test computer.

receipts: []
---

# R2: the restart rule in both directions

The owner's rule is that a crash ends work and a planned update may resume
it once. Today a crash ends only the wake: the run is passivated, its work
item and packets stay open, and the next wake picks them all back up. The
engineering desk does not even wait for a wake; it re-casts the assignment
at boot. R2 closes everything that existed before a crash boot, with the
fate shown to the owner. It changes the recast so that only a planned
restart resumes. It builds nothing new into work items beyond a closer
that R3 replaces with fate records.

Ways it can fail, written before the code:

- a boot phase after the closer mints a packet (terminal-outcome repair,
  the engineering cancel) and it dispatches (the closer runs last, before
  the projector, and the end-of-boot count catches the rest);
- a re-dispatcher not in this file's list re-arms surviving state (the
  quiet-window check on staging finds it);
- closing Texture work refuses the owner's next revise (the reopen
  predicate accepts the fate);
- the closer cancels a trajectory and the document dies (it reuses the
  cancel loops without touching the trajectory);
- a planned boot closes work (every closer reads the marker);
- a resume after a planned boot crashes and loops (the marker is consumed
  first; the next boot is markerless and closes it);
- a half-finished closer leaves mixed state (each item has a
  deterministic command id; the next markerless boot finishes it);
- the fate is invisible (the receipt reads it through the product API,
  not logs);
- the proof is a unit test of a simulated boot (acceptance is a staging
  owner restart).
