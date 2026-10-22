---
definition_version: 4

# Station FP4 of the launch first pass: the self-development architecture
# items of the security, density and architecture checklist (SDC). Owner,
# 2026-10-21 (relayed by the orchestrator): "this pass works only the
# security and density checklist". This replaces the station's first
# direction (LP S6, the commit gate: docs/definitions/choir-lp-s6-commit-gate-2026-10-20.md),
# which is not on the SDC; no S6 commit was made on this lane. Items, in
# order: ARCH-10's launch half (launch item L3-08), then ARCH-04, then ARCH-03.
# Run as `/goal docs/definitions/choir-launch-fp4-selfdev-architecture-2026-10-21.md`.
readiness: drafted

review:
  reviewer: none yet; the orchestrator sends the red candidates (ARCH-10, and ARCH-03's red part if it lands) to a panel before landing
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T18:30:00Z
  source:
    canonical_ref: lane/b10-selfdev at f399a2e8c (origin/main when the lane branched)
    deploy_identity: >-
      unknown to this lane (no staging access). The metamission records staging
      serving f2c4a3ec3 with c893a956a deploying at 17:50Z.
  worktrees:
    - path: /private/tmp/claude-501/-Users-wiz-go-choir/3631f12d-c64d-40d6-a462-dc5bc4bd8b0b/scratchpad/lanes/b10-selfdev
      status: clean
      class: goal_candidate
      owner: FP4 lane worker
      touch: goal_owned
      recovery: branch lane/b10-selfdev
    - path: /Users/wiz/go-choir and every other worktree and stash
      status: unknown
      class: other_agent_wip
      owner: not this lane
      touch: forbidden
      recovery: leave in place
  observed:
    - >-
      Item ARCH-10 (SDC section 4): a new engineering assignment cancels the
      live one. spawnBindActivateAssignment calls
      reclaimSupersededAssignmentCapsules (engineering_assignment_runtime.go:480,
      function at :651-701), which calls cancelAssignedEngineering on every
      other non-terminal assignment that holds a capsule, with the reason
      "superseded by a fresh implementation assignment". Doctrine
      (choir-doctrine.md, the management admission ledger) says FIFO with a
      retryable refusal and work left pending. Hypothesis from code reading
      until the staging drill: docs/problems/engineering-admission-supersedes-live-assignment-2026-10-21.md.
    - >-
      Item ARCH-04: the object graph registers each kind with an Owner that
      names a package (OwnerVMStore, OwnerCorpusd, OwnerAutoradio;
      internal/objectgraph/registry.go:24-44), and CheckWrite (the write gate)
      checks only that a kind is registered. Texture's sole-writer rule is
      enforced separately, in the Texture turn's shape check
      (internal/store/texture_turn.go:179-180, AuthorAppAgent). No general
      rule names which desk writes which kind.
    - >-
      Item ARCH-03: the S7 app package manifest (frontend module, optional
      backend, recipe, data contract) has no desk. The S7-0 branch
      origin/feat/s7-0-app-packages (4bc762102) predates app desks and is
      831 commits behind main. ARCH-02 (the desk manifest) is another lane's
      work (FP2).

finish:
  deliver: >-
    A second engineering request never destroys the first: it waits its turn,
    in arrival order, and is refused retryably while the slot is busy. Each
    object kind has exactly one writer desk, declared in one table the write
    path reads. An app package declares its app desk in a design that the
    package and desk-manifest work can build to.
  artifact: >-
    ARCH-10: the spawn saga's admission function and retry wake, with tests
    that open a second assignment while one is live and see the first
    untouched; the supersede cancel deleted. ARCH-04: a writer column on the
    kind registry (one value per kind from a closed set), a coverage test
    that holds every registered kind to exactly one writer and every writer to
    a known desk, and a write-gate refusal for a write whose declared writer
    is not the kind's writer, where the call site declares one. ARCH-03: a
    design note, docs/design/app-package-carries-app-desk-2026-10-21.md.
  acceptance:
    - action: >-
        Staging drill on a test computer (the orchestrator runs it): Cast two
        engineering requests in a row while the first capsule is working;
        read the assignment rows and the runtime log.
      proves: >-
        The first assignment stays bound and unrevoked; the second stays open
        with no capsule, retries, and is admitted after the first goes
        terminal; the log names the assignment it waits behind.
      evidence_class: deployed proof
    - action: go test ./internal/agentcore -run EngineeringAdmission (focused), and go test ./internal/objectgraph
      proves: The admission order, the busy refusal, no cancel of live work, expiry and the restart rule; one writer per kind.
      evidence_class: local test
  rollback: >-
    Revert the fix commits. ARCH-10 adds no store schema, event kind or
    persisted field (the queue is the assignment rows already on the tape and
    an ordinary scheduled wake); reverting restores supersede-on-spawn.
    ARCH-04 adds a registry column and a coverage test; reverting removes the
    column and the refusal.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the ways a later request, a second writer or a new app can
    displace or bypass the single authority for existing work (a live
    assignment, an object kind, a desk), while preserving the deadline that
    bounds a stuck assignment, the crash rule that closes queued work, and
    the one-writer rule for Texture documents.
  goodharting_would_be: >-
    A FIFO that is a line in a document; tests that never open a second
    assignment while the first is bound; an admission check placed after the
    cancel; a writer column filled by guess with no refusal path and no
    coverage test; a design note that names a desk manifest no one reads.

homotopy:
  realism_axis: >-
    The real saga with a fake capsule runtime on a real store (focused test),
    then a test computer on staging with two real Casts, then the paper's
    engineering load. Every step uses the production spawn saga and wake
    handler; no mock admission.

boundaries:
  mutation_class: >-
    red for ARCH-10 (engineering Cast authority and capsule-bound mutation),
    orange for ARCH-04 (object graph write gate), green then red for ARCH-03
    (the red part only if clear and testable in this batch)
  authority_sources:
    - owner 2026-10-21, owner notes sections 36 and 37 (the SDC; app agents are desks)
    - docs/choir-doctrine.md (the management admission ledger; I26 app desks; C2)
    - docs/security-density-architecture-checklist-2026-10-21.md, items ARCH-03, ARCH-04, ARCH-10
    - docs/computer-ontology.md (management admission)
    - CLAUDE.md (Restarts End Work; Problem Documentation First; Testing)
  must_preserve:
    - A crash restart never resumes work; a queued request's wake is a restart-work kind and is dropped as interrupted_by_restart.
    - The assignment deadline still fails a stuck bound assignment closed; a queued request expires on the same deadline.
    - Terminal assignments' leaked capsules are still revoked before a fresh spawn (budget reclaim).
    - Texture remains the sole writer of canonical document text; the owner's edits keep their own writer class.
    - One canonical state authority per fact: the assignment rows on the tape; no queue table, no second admission store.
  excluded:
    - N parallel capsules per assignment and an admission ledger with overcommit (ARCH-10 P2 half; requires a release gate after sequential proof)
    - ARCH-02, the desk manifest (lane FP2); this lane reads its shape and does not build it
    - the S6 commit-gate slices (dropped by the owner ruling; the S6 goal file is unchanged)
    - the app package loader, S7-0, and the recovered branch feat/s7-0-app-packages
  protected_surfaces: [engineering assignment admission and the spawn saga, capsule revoke fate path, object graph write gate, Texture canonical writes (read only)]
  ceremony: >-
    Red commits carry conjecture delta, protected surfaces, admissible evidence
    class, rollback path and heresy delta (discovered, introduced, repaired).
    Problem documentation first: the problem record
    docs/problems/engineering-admission-supersedes-live-assignment-2026-10-21.md
    is this station's first commit with this file.

standing_questions:
  q1_who_settled: >-
    Owner-settled: the SDC items and the order (owner notes 36, 37); the
    doctrine paragraph on FIFO admission; app agents as desks (I26). Choices
    made here, binding only as proposals: arrival order is (created_at,
    assignment id) rather than a new ordinal field; a stuck live assignment
    is waited out to its deadline rather than superseded.
  q2_conformance: >-
    No third store, no new event kind. The queue is the assignment rows on
    the tape. The registry writer column is in objectgraph, which both stores
    already call through CheckWrite.
  q3_deletion_citers: >-
    Deleted: the supersede branch of reclaimSupersededAssignmentCapsules.
    Citers to grep and repair in the same commit: engineering_assignment_reclaim_test.go,
    the SDC ARCH-10 text (plan update owed: supersede versus FIFO as a doctrine
    deviation, now repaired), docs/problems/c1e-b1-engineering-assignments-cancelled-on-arrival-2026-10-14.md
    (different cause; unchanged), and any prompt text that tells management a
    new Cast supersedes.
  q4_consumers: >-
    The spawn wake handler (kernel mode), the inline spawn (non-kernel, tests),
    ReconcileEngineeringAssignmentsForTrajectory, the fate deadline, and
    management's Cancel. Only the spawn saga changes.
  q5_single_authority: >-
    The assignment rows. Order and liveness are derived from them at each
    attempt; nothing is cached in the process.
  q6_artifact: >-
    The assignment rows read after the drill, and the log lines naming the
    blocker. Not a process's narration.
  q7_fate_sharing: >-
    The admission decision fate-shares with the store read and the saga mutex
    it already needs; the retry is a durable wake, so it survives a process
    restart that is planned and is dropped by a crash.
  q8_restart: >-
    A crash drops the queued request's retry wake and the reconcile cancels
    the open assignment as interrupted_by_restart. A planned restart keeps the
    open, unbound row (review B1: the reconcile used to cancel it as a
    pre-bind strand) and the boot sweep arms its one resume, a spawn wake with
    the boot's identity; a resume that crashes leads to a markerless boot that
    closes it. Non-kernel runtimes keep the old strand closure.
  q9_no_ssh: >-
    The drill reads assignment rows and the runtime log through the product
    API and the tracer; no SSH is needed.

heresy_delta:
  discovered: >-
    Code supersedes where doctrine says FIFO with a retryable refusal;
    the object kind owner is a package, not a desk; the app package manifest
    has no desk.
  introduced: none
  repaired: none until the slices land

now:
  status: checkpoint
  slice: >-
    Capsule memory (lane/fp4-capsule-memory, base main 71ec8e704): A's
    release build in the round 3 re-run ran at its capsule's 2 GiB ceiling
    with 2 OOM kills. Cause confirmed in code: the capsule's /sys mask hides
    cpu.max from Go, so go build runs -p 4 in one CPU of quota, with no
    GOMEMLIMIT and a cold cache. Fix (orange, the 2 GiB cap kept): capsule
    Go workloads get GOMAXPROCS from the quota and GOMEMLIMIT at 5/8 of the
    throttling limit (no bound under 64 MiB); in a local measurement only,
    a capsule-shaped cold build went from an anon peak of 1.27 GiB to
    846 MiB and from 9 m 32 s to 7 m 53 s. The deployed proof (the FP4
    drill re-run on 71afbf14) is pending. Problem record 3680eacc3, tests
    55e9ac1ae and 6d937d4fe, fix fd8337941 and c68c49a99 (review round 2:
    linux build tags, the floor, no ambient carry); sizing in
    docs/evidence/res04-capsule-memory-sizing-2026-10-22.md.
    Before that: round 3 repair from the deployed drill on 71afbf14 (lane/fp4-r3-nudge,
    base main 7448bf899). P1: the nudge waited behind management's own
    activation (B bound 305 s after A ended); the cause is confirmed in a
    test, and admission's own wakes now go to an engineering-admission
    mailbox with its own dispatcher lane, and every admission wake logs its
    dispatch latency. P2: the R3 refusal advised "partial", which ends in a
    completion-guard cancel; it now advises failed with the refusal as the
    reason, and the guard's reason names the open candidate. Tests first
    (yellow), then the fix (red), then this record. Round 3b (review
    bunch 51: three accepts, edits only): a re-run plan whose request B
    reports failed, residuals for the emitter's commit boundary and the
    cancel/admission race, real-handler and lane tests, a range for the
    latency test, a state-neutral guard reason. Earlier: round 2 (R1 to
    R6), batch 1 (B1 to B3), writer-gate counts, the nudge.
  source_ref: 5c2c8032c on lane/fp4-r3-nudge (base main 7448bf899)
  deploy_identity: unknown to this lane
  candidate: {id: arch10-admission-r3, state: ready_for_review, ref: lane/fp4-r3-nudge, base: 7448bf899, digest: none, scope: [internal/agentcore/engineering_admission.go, internal/agentcore/engineering_assignment_runtime.go, internal/agentcore/engineering_assignment_fate.go, internal/agentcore/continuation_schedule.go, internal/agentcore/api.go, internal/agentcore/freeze_candidate.go, internal/agentcore/runtime.go, internal/actorruntime/adapter.go, internal/actorruntime/handler.go, internal/objectgraph/writer.go]}
  conjecture:
    id: FP4-C1
    claim: >-
      The supersede cancel is the only path by which a new engineering Cast
      destroys a live assignment. Replacing it with admission in the spawn
      saga makes concurrent requests safe without a new store, event kind or
      schema, provided every consumer of the open, unbound state reads it as a
      request (the restart reconcile was the consumer the first candidate
      missed) and expiry is a precondition of admission.
    test: >-
      Focused tests on a real store (EngineeringAdmission*,
      EngineeringBoundDeadline*), then the staging drill below.
    edge: missing_oracle
    delta_o: >-
      The drill's assignment rows after each step, the log lines naming the
      blocker, and the time from the first terminal to the second bind.
    scope_if_supported: >-
      Sequential admission of records Casts on one computer, kernel mode;
      not N concurrent capsules, not the legacy coagent spawn path.
    status: testing
    evidence_refs:
      - docs/problems/engineering-admission-supersedes-live-assignment-2026-10-21.md (review findings section)
      - "local: the B1, B2 and B3 tests failed on b0f872bd6 and pass at 059924c7e; go test ./internal/agentcore -run 'Engineering|Assignment|Restart|Boot|C1e|PreBind|Deadline|Records|Cast' ok; go test ./internal/objectgraph ok"
      - "local: the round 3 tests (engineering_admission_dispatch_test.go, freeze_refusal_fate_test.go, actorruntime admission_wake_latency_test.go) failed on 7448bf899 and pass at cf7ed5072; on main the nudge ran only 18 ms after management's held activation was released, at the fix 95 ms after A ended with management still held; GOMAXPROCS=2 go test -race ./internal/agentcore -run 'Engineering|Admission|Assignment|Selfdev|Freeze|Obligation|Nudge|Guard|Completion' ok; go test ./internal/actor ok; go test ./internal/actorruntime ok"
      - "local: the round 2 tests (engineering_admission_review2_test.go) failed on c494f5aac and pass at 11dfea955; GOMAXPROCS=2 go test -race ./internal/agentcore -run 'Engineering|Admission|Assignment|Selfdev|Freeze|Obligation' ok; go test ./internal/objectgraph ok; go vet agentcore, objectgraph, platform clean"
  decision:
    what: >-
      Queue by durable open request and retry the spawn; never supersede. A
      planned boot keeps queued requests and resumes each once; a crash boot
      closes them. Expiry precedes admission. The bound deadline counts from
      bind (grant attestation GrantedAt); the open arms no deadline.
    kind: architecture
    status: proposal
    evidence_ref: docs/choir-doctrine.md management admission ledger; docs/computer-ontology.md:378-385
    owner_ratification_ref: >-
      The FIFO doctrine is owner-ratified (I26, 2026-10-21). Refusing at open
      versus queueing at spawn is not ratified: open question below, default
      in force.
  belief:
    believed_state: >-
      The queue's lifecycle now agrees with the restart rules and the
      deadline: kept across a planned restart, closed by a crash, expired
      before admission, and bound runs get their full deadline from bind.
    main_uncertainty: >-
      The planned-restart resume on a real refresh (the wake survives, the
      boot sweep runs after the adapter binds scheduleActor), which only the
      drill shows.
    next_observation: The staging drill below, with a planned refresh between the Casts.
  blocker_or_risk: >-
    A stuck live assignment blocks the queue until its deadline (six hours
    from bind) or an explicit Cancel; accepted residual. After a planned
    restart a surviving pre-boot retry chain and the boot resume chain can
    both run until admission (bounded; both no-op after bind). After A
    completes with a frozen candidate, B binds but cannot freeze until the
    owner decides A (R3; residual for L3-07). A scheduleActor failure after
    the bind commit, without a restart, still leaves a bound run without
    its deadline wake (R2 residual). The open-time spawn wake is still minted
    on management's mailbox by the store, so a request's first admission
    attempt waits for the casting activation (P1 residual). Hypothesis,
    from code, not drilled: a nudge minted from a management-initiated
    Cancel becomes durable only when management's activation commits, so
    it still waits for that activation; a nudge whose emitting activation
    errors or defers is lost and B waits for its own retry (up to about
    60 s); a management Cancel and an admission can now race, and the
    lifecycle CAS makes the loser fail closed (P1 residuals b, c).
  next_action: >-
    Capsule memory: FP0 lands fd8337941 and c68c49a99 with the problem record and tests,
    then runs the deployed proof in
    docs/evidence/res04-capsule-memory-sizing-2026-10-22.md ("Deployed
    proof"). That is an FP4 request A re-run on 71afbf14. Its cell and job
    both print GOMAXPROCS=1 GOMEMLIMIT=1342177280. Pass: A's capsule_memory
    receipt shows oom_kill=0. Record the build job's wall time from A's
    trace against the earlier run's about 20 minutes. It can share a run
    with the round 3 re-run below. Earlier: FP0: review the round 3 and 3b commits on lane/fp4-r3-nudge, land, then
    re-run drill steps 4 and 6 with the re-run plan in
    docs/evidence/fp4-admission-queue-deployed-proof-2026-10-22.md ("Re-run
    plan for steps 4 and 6 after FP4 round 3"). Precondition: no
    self-development operation is open on 71afbf14 before A is cast (check
    with `choir self-dev show/head`; the first run's selfdev-0593136f was
    rejected by FP0 at 11:32 UTC on 2026-10-22). Its request B tells the desk
    to report FAILED with the refusal as the reason, so the new refusal text
    is the branch under test; the original request B said "partial" and
    cannot test it. (4) Let A finish on its own (not the "or Cancel it"
    variant, which this repair does not cover); B binds within seconds, not
    at the next minute retry, while management is still reading A's report;
    the guest log has "actorruntime: engineering admission wake latency
    mailbox=engineering-admission … waited=…" for B's nudge (content carries
    "nudge":"<A>"), waited a few seconds at most. (6) B's freeze is refused
    with text naming A's operation, "cannot freeze until" and "Complete with
    result failed", never "partial"; the guest log has "selfdev: assignment
    B cannot freeze"; B's terminal report is failed with the refusal as its
    reason and there is no system cancel. If the guard still cancels B, its
    reason names A's candidate ("the freeze was refused because
    self-development candidate selfdev-… was open (state
    awaiting_approval)"), never the bare guard name, and that is a finding,
    not a pass. Steps 0 to 2 and 5 passed on deploys J and K; step 3 is
    unreachable by design (evidence doc). Crash half: on a throwaway
    computer, kill the guest with B queued; after boot B is cancelled
    interrupted_by_restart.
slices:
  ARCH-10:
    what: >-
      Engineering admission does not cancel live work. In the spawn saga, before
      any cancel: refuse (typed, retryable) while another assignment holds a
      capsule or an older, unexpired request is open; leave the request open;
      re-arm the spawn wake with a retry count and a backoff; admit when the
      slot is free and the request is first in (created_at, assignment id)
      order; expire a request queued past the assignment deadline. Delete the
      supersede cancel. Terminal-capsule reclaim stays.
    proof: >-
      Focused tests (engineering admission); the staging drill above. The
      second half of ARCH-10, N capsules per assignment and serialized apply,
      is a design question this lane answers in the closing report if its
      shape is clear; no code for it without a release gate.
  ARCH-04:
    what: >-
      One writer per object kind. A writer column on the registry from a
      closed set (the desk names plus owner, system and corpusd), a coverage
      test, and a write-gate check for call sites that declare a writer.
    proof: go test ./internal/objectgraph (coverage, refusal); the existing write-gate tests unchanged.
  ARCH-03:
    what: >-
      An app package carries its app desk. Green: a design note that adds the
      desk to the S7 package manifest by reference to ARCH-02's desk manifest.
      Red: only if the shape is clear and testable in this batch.
    proof: The design note, reviewed against the S7 definition, D1, and the desk manifest lane.

open_for_the_owner:
  - >-
    Refuse at open or queue at spawn? Default in force: queue at spawn. The
    refusal at open is simpler and leaves the retry to management's model; it
    gives no durable order.
  - >-
    A stuck live assignment: wait to the six-hour deadline, or let management's
    explicit Cancel free it? Default in force: both are already true (the
    deadline fails it closed; Cancel is an explicit fate). Nothing supersedes.

receipts:
  - id: arch10-admission
    boundary: implement
    identity: 6ce19af5a
    proof_refs: [internal/agentcore/engineering_admission_test.go]
    rollback_ref: git revert 6ce19af5a
  - id: arch04-writer-gate
    boundary: implement
    identity: 5b4578e73
    proof_refs: [internal/objectgraph/writer_gate_test.go, internal/store/texture_writer_gate_test.go]
    rollback_ref: git revert of the ARCH-04 commit; the default mode is observe, so a revert changes no behavior
  - id: arch10-b1fix
    boundary: implement
    identity: e832951b0 (problem record), 7e24b0889 (B1), d5055214a (B2), a46ed9a9b (B3), 059924c7e (nudge)
    proof_refs: [internal/agentcore/engineering_admission_test.go]
    rollback_ref: git revert of the five commits in reverse order; no schema, no new kind
  - id: arch10-r2
    boundary: implement
    identity: 783cd578c (findings), 6f1861830 (tests), 11dfea955 (fix)
    proof_refs: [internal/agentcore/engineering_admission_review2_test.go]
    rollback_ref: git revert 11dfea955 (no schema, no new kind)
  - id: arch10-r3
    boundary: implement
    identity: b557b2d64 (tests), cf7ed5072 (fix), on lane/fp4-r3-nudge
    proof_refs: [internal/agentcore/engineering_admission_dispatch_test.go, internal/agentcore/freeze_refusal_fate_test.go, internal/actorruntime/admission_wake_latency_test.go]
    rollback_ref: git revert cf7ed5072 (no schema, no new kind; a wake already on the admission mailbox is dropped and the request gets its fate at the next planned or crash boot)
  - id: arch10-r3b
    boundary: implement
    identity: f55f1588a (dispatcher options extracted), f9732b3de (tests), 1b21dc6c8 (guard reason text)
    proof_refs: [internal/actorruntime/admission_mailbox_test.go, internal/actorruntime/admission_wake_latency_test.go, internal/agentcore/freeze_refusal_fate_test.go]
    rollback_ref: git revert 1b21dc6c8 (text only); the other two change no behaviour
  - id: capsule-go-bounds
    boundary: implement
    identity: 3680eacc3 (problem record), 55e9ac1ae and 6d937d4fe (tests), fd8337941 and c68c49a99 (fix), on lane/fp4-capsule-memory; deployed proof pending
    proof_refs: [internal/yaegikernel/go_bounds_test.go, internal/capsule/go_bounds_test.go, cmd/capsule-broker/exec_direct_test.go, docs/evidence/res04-capsule-memory-sizing-2026-10-22.md]
    rollback_ref: git revert c68c49a99 fd8337941 (no schema, no stored state; workloads go back to Go's defaults)
  - id: arch04-observability
    boundary: implement
    identity: bbd19192c
    proof_refs: [internal/agentcore/writer_gate_observability_test.go]
    rollback_ref: git revert bbd19192c (an additive JSON field)
  - id: arch03-design
    boundary: define
    identity: 58f369309
    proof_refs: [docs/design/app-package-carries-app-desk-2026-10-21.md]
    rollback_ref: git revert of the docs commit
---

# FP4: the self-development architecture items

This station builds three items of the security and density checklist in the
order the owner set: engineering admission that does not cancel live work
(ARCH-10, the launch half of launch item L3-08), one writer per object kind
(ARCH-04), and an app package that carries its app desk (ARCH-03).

The first slice has a problem record, and the code reading is a hypothesis
until the staging drill reads the assignment rows. Read the problem record
before the code: it lists the failure modes of the fix before the tests.
