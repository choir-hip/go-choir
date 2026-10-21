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
    wake (the same marker rules as every restart-work kind).
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
  status: working
  slice: >-
    ARCH-10 launch half: the problem record and this file (first commit), then
    the failing tests, then the admission function, the retry wake, and the
    deletion of the supersede cancel.
  source_ref: f399a2e8c
  deploy_identity: unknown to this lane
  candidate: {id: none, state: none, ref: lane/b10-selfdev, base: f399a2e8c, digest: none, scope: []}
  conjecture:
    id: FP4-C1
    claim: >-
      The supersede cancel is the only path by which a new engineering Cast
      destroys a live assignment. Replacing it with admission in the spawn
      saga (refuse and retry while a live or older queued assignment exists)
      makes concurrent requests safe without a new store, event kind or
      schema.
    test: >-
      A focused test that opens a second assignment while the first is bound
      (fake capsule runtime, real store) and sees the first non-terminal with
      its capsule unrevoked; then the staging drill reads the same rows.
    edge: missing_oracle
    delta_o: >-
      The log line naming the blocker and the retry count, and the drill's
      read of both assignment rows after each step.
    scope_if_supported: >-
      Sequential admission of records Casts on one computer, kernel mode;
      not N concurrent capsules, not the legacy coagent spawn path.
    status: proposed
    evidence_refs: [docs/problems/engineering-admission-supersedes-live-assignment-2026-10-21.md]
  decision:
    what: >-
      Queue by durable open request and retry the spawn; do not refuse the
      Cast at open (a refused open writes nothing and leaves the retry to a
      model).
    kind: architecture
    status: proposal
    evidence_ref: docs/choir-doctrine.md management admission ledger; docs/computer-ontology.md:378-385
    owner_ratification_ref: >-
      The FIFO doctrine is owner-ratified (I26, 2026-10-21). The choice between
      refusing at open and queueing at spawn is not ratified: named as an open
      question below, default in force.
  belief:
    believed_state: >-
      Admission correctness is a small change in the spawn saga; the harder
      parts are the wake's retry identity and the stuck-live-assignment wait.
    main_uncertainty: >-
      Whether anything depends on supersede to recover a stuck assignment
      (the staging drill and the paper's engineering history would show it).
    next_observation: The failing focused test, then the drill.
  blocker_or_risk: >-
    A stuck live assignment now blocks the queue until its six-hour deadline or
    an explicit Cancel; accepted, recorded as a residual.
  next_action: >-
    Commit the problem record and this file, then write the failing admission
    tests.

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

receipts: []
---

# FP4: the self-development architecture items

This station builds three items of the security and density checklist in the
order the owner set: engineering admission that does not cancel live work
(ARCH-10, the launch half of launch item L3-08), one writer per object kind
(ARCH-04), and an app package that carries its app desk (ARCH-03).

The first slice has a problem record, and the code reading is a hypothesis
until the staging drill reads the assignment rows. Read the problem record
before the code: it lists the failure modes of the fix before the tests.
