---
definition_version: 3
definition_id: choir-commitment-ledger-desk-carrier-draft-2026-09-23
execution_mode: mission_orchestrator
draft: true

start:
  captured_at: '2026-09-23T21:00:00Z'
  source:
    canonical_ref: main@b0adf6f7
    deploy_identity: staging https://choir.news build.commit=4de7fdf9
  worktrees:
    - path: /Users/wiz/go-choir
      status: unknown
      class: unknown
      owner: unknown
      touch: read_only
      recovery: reconcile at charter
  predecessor:
    mission: choir-ontology-kernel-draft-2026-09-22
    disposition: >-
      partial dependency — the commitment ledger schema and the yaegi
      carrier generalization do not need the kernel; desk wakes do. R2 may
      start in parallel with K but must not claim live-desk acceptance
      before K lands.
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md
  observed_artifact:
    - claim: >-
        The yaegi carrier serves only engineering (capsule_go_eval sole
        tool); the choir package surface is fixed, not per-desk; the
        read-only research scope cannot message (choir.go:40-60,117-145).
      claim_scope: current
      evidence_ref: internal/yaegikernel/choir.go
    - claim: >-
        update_coagent's packet machinery is load-bearing across ~60 files:
        texture evidence ingestion, revision metadata (worker_updates_*),
        actor park/resume, Super delivery, terminal child-outcome fallback,
        replay goldens. The tool registration is retired for engineering;
        the packet contract is not.
      claim_scope: current
      evidence_ref: internal/agentcore/tools_worker_update.go
    - claim: >-
        Delegated cast has no admission authority: the document-cast API
        requires an AuthorUser revision; assignment identity and parent
        control derive from it (cosuper_assignment_runtime.go:134-183).
        choir.Spawn is a staged spawn_request envelope, not admission;
        choir.Assign is a synchronous broker path — a third delegation
        shape.
      claim_scope: current
      evidence_ref: internal/agentcore/cosuper_assignment_runtime.go

finish:
  deliver: >-
    One agent substrate: every desk runs yaegi Go cells in a subprocess
    with a desk-specific choir module set; inter-desk communication is a
    semantic-act verb surface where claims are commitments on an OG ledger
    that resolve and score.
  artifact: >-
    (a) The commitment record type: an OG object with provenance edges
    (never a third store), carrying the memo's full schema — frozen
    prediction, observation, scorer identity, discrepancy class,
    specificity; scalar scores are derived views. (b) The yaegi carrier
    generalized: per-desk module sets, desk cells in a killable subprocess
    with restricted stdlib exports (not in-process), research desk gets
    message authority (read-only world access ≠ read-only messaging).
    (c) The semantic-act verbs: choir.Cast (delegation — new admission
    authority for desk-authored casts), choir.Report (typed claim +
    evidence assertion, names its resolver), choir.Ask, choir.Precommit,
    choir.Resolve, choir.Cancel/Withdraw, choir.Escalate, choir.Note (raw,
    unscored). choir.Outcome folds into Report; choir.Assign deleted or
    folded into Cast; choir.Message demoted to Note. Complete/Freeze/Verify
    stay engineering effect verbs. (d) update_coagent migrated for the four
    desks — the packet schema survives as Report's body; texture's
    worker_updates_* metadata path migrated; processor/reconciler keep the
    tool until their phase (explicit boundary). (e) The execution_request
    packet kind gets a verb or an explicit death (D13).
  acceptance:
    - action: >-
        A management desk cell stages choir.Cast(engineering, objective);
        the reducer opens an engineering assignment under the new delegated
        admission authority (not an owner revision); the assignment binds
        and the cast records a commitment on the OG ledger.
      proves: delegated cast works end-to-end on canonical evidence
      evidence_class: local test + deployed proof
    - action: >-
        A research desk cell stages choir.Report with evidence refs; the
        report lands as a typed act that asserts OG evidence nodes and
        resolves on the named resolver's acceptance.
      proves: the semantic-act surface carries evidence, not just transport
      evidence_class: local test
    - action: >-
        A desk cell staging choir.Precommit records a frozen prediction
        with resolver and deadline; resolution writes the outcome and
        accrues to the desk's score; the score never enters the acting
        agent's context.
      proves: the commitment ledger is live and the epistemic boundary holds
      evidence_class: local test
    - action: >-
        The four desks' registries contain zero tool-loop tools; every
        agent-to-agent act is a staged choir verb; update_coagent is absent
        from management/engineering/research/texture registries and present
        only in processor/reconciler.
      proves: one carrier, one comm substrate for the four desks
      evidence_class: local test
  rollback: git revert + redeploy; the ledger is additive (new OG kinds),
    the carrier is a new path alongside the old until cutover.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the number of agent-communication substrates (currently two
    non-uniform channels + a third sync path) to one semantic-act surface,
    while preserving every consumer the packet machinery feeds.
  goodharting_would_be: >-
    A verb surface that wraps update_coagent packets without the commitment
    semantics — transport renamed, not semantics built. Or a ledger that
    records acts but never resolves them (a write-only scoreboard).

homotopy:
  realism_axis: >-
    Substrate coverage: from engineering-only yaegi cells (current) through
    four desks on cells with the verb surface (this mission) to all actors
    on cells (world-wire phase adds processor/reconciler or deletes them).
    The low rung is the same carrier with fewer desks — same interfaces,
    same semantics.

boundaries:
  mutation_class: red
  authority_sources:
    - docs/desk-rlm-rectification-plan-2026-09-23.md
    - docs/Precommitment Records — Engineering Memo.md
    - docs/archive/choir-event-driven-rlm-ontology-minimal-2026-09-15.md
    - AGENTS.md
  must_preserve:
    - the capsule boundary for engineering mutation
    - the assignment fate saga as host machinery
    - texture's evidence pipeline continuity during the update_coagent
      migration (worker_updates_* metadata must not break mid-flight)
    - the epistemic boundary: scores never enter the acting agent's context
    - processor/reconciler on the tool loop until their phase
  excluded:
    - live desk actors waking on revisions (R3 — needs K's derivable wakes)
    - texture writing doc revisions from the ledger (R3)
    - score surfacing and context packs (R4)
    - durable vocabulary rename (R5)
    - the strand-2 patch (R0 — lands first, independently)
  protected_surfaces:
    - canonical event appends and the reducer commit path
    - the assignment admission authority (new delegated-cast contract)
    - the capsule executor boundary
    - texture canonical writes (AuthorAppAgent authority unchanged)
    - the object graph (new kinds; no third store)

now:
  status: working
  slice: consumer census + commitment record schema landed (in-flight)
  source_ref: main@60046751
  deploy_identity: staging https://choir.news build.commit=ce28e407
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: r2-semantic-act-substrate
    claim: >-
      One semantic-act verb surface over a commitment ledger can replace
      update_coagent + raw choir.Message for the four desks without losing
      any consumer the packet machinery feeds, and the delegated-cast
      admission authority can open engineering assignments without an
      owner-authored revision.
    test: >-
      The acceptance probes: delegated cast opens an assignment; Report
      asserts evidence; Precommit resolves and scores; the four desks have
      zero tool-loop tools.
    edge: missing_oracle — the update_coagent consumer census may be
      incomplete; a consumer discovered mid-migration extends the scope.
    delta_o: >-
      A full consumer inventory before any deletion: every file referencing
      CoagentSourcePacket, worker_updates, or the packet kinds, classified
      as migrate/keep/delete.
    scope_if_supported: >-
      The four desks run one substrate; processor/reconciler remain the
      only tool-loop survivors with an explicit deletion boundary.
    status: active
    evidence_refs:
      - docs/desk-rlm-rectification-plan-2026-09-23.md
      - internal/agentcore/tools_worker_update.go
  decision:
    what: >-
      Commitment record = OG object with provenance edges (never a third
      store — Phase 2a rule, D12 settled). Desk cells in subprocess with
      restricted stdlib (D2 corrected). Delegated cast = new admission
      authority with its own identity scheme (not the v3 seed). Report
      names its resolver; resolver never the claimant for engineering
      claims (D7). Full memo schema for records, scalar views derived (D8).
    kind: architecture
    status: proposal
    evidence_ref: docs/desk-rlm-rectification-plan-2026-09-23.md
    owner_ratification_ref: pending — plan under owner review
  belief:
    believed_state: >-
      The carrier generalization is mechanical; the delegated-cast
      admission authority and the update_coagent consumer migration are
      the hard parts.
    main_uncertainty: >-
      The full update_coagent consumer inventory — texture's evidence
      pipeline depth may exceed the ~60-file estimate.
    next_observation: >-
      The consumer census result; the delegated-cast admission contract's
      first test.
  blocker_or_risk: >-
    K (ontology kernel) gates the live-desk half; the ledger + carrier +
    verbs half may proceed in parallel.
  next_action: >-
    In progress — substrate landed (commits 60046751→f7df9213). DONE:
    (a) update_coagent consumer census (40 files, migrate/keep/delete) —
    docs/evidence/r2-update-coagent-consumer-census-2026-09-25.md; (b)
    commitment record schema — internal/types/commitment.go + OG kind
    choir.commitment_record (graph_store.go) + append path
    Store.AppendCommitmentRecord (store/commitment.go); (c) semantic-act
    verb surface — StagedIntent kinds cast/ask/note/reply/cancel/escalate/
    precommit/report/resolve + Tray methods + ChoirScope verbs + choir
    module exports (yaegikernel/intent.go, choir.go); (d) reducer wiring —
    validateSemanticActIntent + commitActIntent mint the ledger record and
    mail addressed envelopes (rlm_reduce.go). NEXT (red-class core):
    delegated-cast admission authority — a new admission path that opens an
    engineering assignment without an owner revision, under a new identity
    scheme (not choir:co-super-assignment:v3), with the reducer commit made
    atomic and the one-live-assignment ledger enforced. IMPLEMENTATION MAP
    (researched 2026-09-25): the caster (persistent Management) run already
    carries assignment_trajectory_id + lifecycle_control_bindings metadata
    (lifecycle_control_delivery.go:499,682), so the delegated parent
    authority resolves through the EXISTING requireEngineeringParentAuthority
    run path (engineering_assignments.go:599) — ParentRunID set to the
    caster's run — NOT a new join. Needed: (1) a delegated mint path beside
    startAssignedEngineeringForDocument (engineering_assignment_runtime.go)
    that derives the binding from the cast intent + the caster's live run
    instead of an owner revision, mints ParentDecisionID under a delegated
    scheme (e.g. choir:delegated-decision:v1, not co-super-decision:v3), and
    resolves subject/capsule provenance; (2) commitActIntent for IntentCast
    calls OpenEngineeringAssignment (in place of/alongside the cast envelope)
    and returns the assignment receipt; (3) one-live-assignment admission
    ledger + atomic commit (reducer commit currently envelopes+cursor, not
    transactional with the ledger write — consensus binding precondition);
    (4) the spawn/bind/activate saga must run async after commit, not inside
    the cell reducer. THEN per-desk yaegi module sets, the four-desk
    update_coagent migration, and the execution_request verb/death.

  delegated_cast_landed_2026_09_25: >-
    Delegated-cast admission authority landed (commit 09057831).
    CastAuthority on EngineeringAssignmentBinding (types/engineering_assignment.go);
    requireEngineeringDelegatedParentAuthority (store/engineering_assignments.go)
    authenticates the caster's run + open work item and treats the cast's
    commitment record (ParentControlID = its canonical ID, Provenance.AgentID
    == caster) as the delegated parent control. startDelegatedCastAssignment
    (engineering_assignment_runtime.go) mints the binding under
    choir:delegated-decision:v1 / choir:delegated-cast-request:v1 and runs the
    shared spawnBindActivateAssignment saga synchronously (the restart
    sweeper cancels open-but-unbound opens). commitActIntent(IntentCast)
    (rlm_reduce.go) mints the commitment record, opens the assignment, then
    mails the cast envelope. Per-desk module sets landed (7dda0a26):
    ChoirScope.desk + deskModuleSets; research gains message authority
    (read-only world != read-only messaging). REMAINING before
    finish.acceptance: (1) update_coagent cutover off the 4 desks —
    RegisterCoagentUpdateTools removal from management+research registries
    (tool_profiles.go) requires porting ~70 test call sites that Execute the
    tool on desks (update_coagent_source_packet_test.go, survivor_contract,
    cutover, authority, tools_test, rlm_replay_*) onto the staged choir verb
    path, plus migrating texture's worker_updates_* metadata path; reverting
    a bare registry drop leaves the suite red, so this is one scoped commit;
    (2) execution_request verb/death (D13); (3) the reducers' commitment/OG
    write inside commit must be made transactional with the cursor (atomic
    commit — consensus precondition); (4) acceptance probes.

  status_2026_09_25: >-
    R2 substrate + admission core landed and tested on main. Committed
    chain: 60046751 census → 74460602 commitment schema → 2f6f490c verb
    surface → f7df9213 ledger write path → 09057831 delegated-cast admission
    authority → 7dda0a26 per-desk module sets → bfce0398 delegated-cast
    authority tests (all pass: opens under caster authority, rejects
    foreign/absent control). Deploy: bfce0398 diffs deployed ce28e407→head
    so its staging deploy carries the cumulative R2 runtime delta (earlier
    per-commit pushes were superseded; the surviving head run covers them).
    IN PROGRESS / NOT DONE — update_coagent consumer migration is a
    producer+consumer relocation, not a registry drop: removing
    RegisterCoagentUpdateTools from management+research breaks ~70
    desk-facing tests that Execute the tool and the worker_updates_*
    consumption path must migrate to commitment records. Next commit.

  delegated_spawn_async_2026_09_25: >-
    Consensus precondition (3) repaired: the delegated-cast spawn/bind/activate
    saga no longer runs inside the cell reducer. openDelegatedCastAssignment
    (engineering_assignment_runtime.go) now commits only the durable open —
    record + binding + deterministic capability — and returns. commitActIntent
    (rlm_reduce.go) arms delegated_assignment_spawn_deadline via
    armDelegatedCastSpawn (continuation_schedule.go) under kernel mode; the
    actorruntime dispatcher (handler.go case delegated_assignment_spawn_deadline
    → HandleDelegatedAssignmentSpawnDeadline) re-drives
    resumeDelegatedCastAssignment post-commit. The saga re-derives the
    committed subject/capability digests fail-closed, so a replayed wake is a
    no-op. Non-kernel test runtimes keep a synchronous fallback. Remaining:
    update_coagent consumer migration, execution_request verb/death (D13),
    acceptance probes.

receipts: []
---

## The verb surface (adjudicated)

```go
// Operational acts — tracked, not scored.
choir.Cast(desk, objective, spec)   -> Handle   // delegated admission
choir.Ask(to, question)             -> Handle   // query; resolves on answer
choir.Note(to, body)                -> Receipt  // raw transport, unscored
choir.Cancel(handle) / Withdraw     -> Receipt  // retract a commitment
choir.Escalate(to, issue)           -> Receipt  // to management or owner

// Epistemic acts — claims that resolve and score.
choir.Precommit(statement, resolve) -> Handle   // frozen prediction
choir.Report(to, claim, evidence)   -> Receipt  // evidence assertion
choir.Resolve(handle, outcome)      -> Receipt  // the resolver's act
choir.Reply(to, answer)             -> Receipt  // Ask's counterpart

// Engineering effect verbs — unchanged.
choir.Complete(...)  choir.Freeze(...)  choir.Verify(...)  choir.InspectBundle()
```

## Hard requirements carried from the consensus review

- The commitment record is an OG object with provenance edges — never a
  third store.
- The reducer commit must become atomic for staged acts (currently
  sequential, `rlm_reduce.go:405-475`) or carry explicit partial-commit
  recovery — a half-committed cast is the strand-2 shape.
- The delegated-cast identity scheme must not reuse
  `choir:co-super-assignment:v3` without a version bump + replay story.
- Management's cast surface implements the one-live-assignment admission
  ledger (doctrine: one live engineering assignment per computer).
- Research desk: read-only world access is separate from message
  authority — the current read-only scope can't message at all.
- The `execution_request` packet kind (privileged control) needs a verb
  or an explicit death (D13).
