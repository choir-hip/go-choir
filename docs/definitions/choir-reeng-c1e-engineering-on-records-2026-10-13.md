---
definition_version: 4

# Station C1e of the re-engineering metamission
# (docs/definitions/choir-reeng-metamission-2026-10-12.md): engineering and
# self-development on records, then the wholesale deletion of packets and
# work items. Scope is cut 1's scope_split (C1e items) in
# docs/definitions/choir-reeng-cut1-authority-2026-10-12.md; the code map
# and the dependency order are docs/reports/c1e-deletion-inventory-2026-10-13.md
# (read-only inventory on 18d01cc0). This file is the executable station
# over that scope; it adds no scope.
readiness: drafted

review:
  reviewer: agentic-consensus convergent round 1 on 79d60be4 (7 of 7 send_back; every blocking finding adopted below, round 2 next)
  frozen_ref: none (round 2 binds it)
  verdict: send_back
  evidence_ref: .agentic-consensus/c1e-station-convergent-r1-2026-10-13/

start:
  captured_at: 2026-10-13T22:00:00Z
  source:
    canonical_ref: main 29bbab97
    deploy_identity: staging 18d01cc0 (29bbab97 deploying)
  worktrees:
    - path: /Users/wiz/go-choir (untracked `sourcecycled` at the repo root)
      status: dirty
      class: other_agent_wip
      owner: not this session
      touch: forbidden
      recovery: leave in place
  observed:
    - >-
      Work items live in 58 production files (about 1,846 lines); the
      delivery packet (types.CoagentSourcePacket, kind choir.worker_update)
      in 31. The packet body types.CoagentSourcePacketPayload is still the
      live Report content (rlm_reduce.go, inbox_records.go) and is not in
      scope for deletion (inventory §1).
    - >-
      Records cover Texture, research and management (recordsDesk);
      recordsRoute has no engineering branch and commitRecordsRoute refuses
      engineering casts (agentcore/inbox_records.go:360). Until C1e,
      management refuses engineering casts on records turns, so execution
      requests come back blocked on staging and on the owner's computer.
    - >-
      Path B (a document cast with no parent run retargets its engineering
      report to texture:<doc> and mints work:texture-supervision:<doc>) is
      still wired: store/engineering_assignments.go:1747-1823,
      texture_controller.go, selfdev_texture_join.go, the recast and Jev
      judge, run_acceptance.go:166-188 (inventory §1).
    - >-
      store/lifecycle.go and management_controller.go are shared with the
      live Autopaper path: Texture and management still receive terminal
      outcomes as packets, and actorruntime wakes Texture on producer-report
      occurrences (inventory §5, riskiest coupling).
    - >-
      The reeng checker's I2 and I3 read work items and packets
      (scripts/reeng_invariants.mjs); I7 asserts none are minted on a
      records route. On a crash, I2 fails by design while one work item is
      open (c1-core-crash, l1a3-crash-2).
    - >-
      The ledger metamission's L1a (crash boots run no re-driver) is landed
      at 18d01cc0, the precondition the metamission set for C1e (one
      shared-runtime cutover at a time).

finish:
  deliver: >-
    Engineering and self-development conversations run on records like
    the other desks, with engineering's prompts and generated verb list
    teaching only its post-C1e exports; path B is gone; management
    verifies an engineering Report against the operation and bundle it
    names; sub-researchers are runs on records (L5); run acceptance folds
    from records (L7); packets and work items are deleted from production
    code, with a CI gate that keeps them at zero.
  artifact: >-
    a heresy-detector row (enforce: zero) for the cut-1 vocabulary green on
    main; a deployed self-development round trip on records on a minted
    test computer; the paper and crash scenarios passing on the final
    deletion commit
  acceptance:
    - action: >-
        a docs/heresy-detectors.md row, enforce: zero, in the checker's own
        syntax: scripts/check-heresies.sh counts literal substrings, not
        regexes, so the row lists one backticked literal per name —
        `WorkItem`, `workItem`, `work_item`, `CoagentSourcePacket`,
        `worker_update`, `update_coagent`, `"Assign"`, `choir.Message`,
        `choir.Emit`, `textureSupervision`, `RequireNoOpenWorkItems` —
        with exclude: docs/**, *.md, *_test.go, internal/vocabmigrate/**
        (historical decode of old tapes),
        scripts/reeng_invariants.mjs (I7 must name the kinds it asserts are
        never minted). The live body type CoagentSourcePacketPayload is
        renamed in its own commit before the row lands, so the literal
        `CoagentSourcePacket` cannot match it. The row lands in report mode
        at step 1 (its count is the progress measure) and turns to
        enforce: zero at step 8, with a planted-hit commit on a branch
        showing CI fail
      proves: the deletion holds and cannot regrow, and the gate is not vacuous
      evidence_class: CI
    - action: >-
        engineering's prompts (internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml
        and its desk overlay) and generated verb list name only its
        post-step-1 exports, with the reverse parity test
        (internal/yaegikernel/choir_parity_test.go) extended to engineering
      proves: engineering is not taught verbs it no longer has
      evidence_class: CI
    - action: >-
        node scripts/reeng_tracer.mjs --scenario engineering --expect-sha
        <step-4 sha>, a scenario authored in step 4: on a minted test
        computer, management casts engineering on records, engineering
        returns a Report naming an operation id and bundle digest,
        management Verifies it, the self-development decision materializes,
        and run acceptance for the round-trip trajectory is synthesized
        from the fold (L7); the checker reports I7 (widened to every route)
        pass, with no work item or packet minted, and the accepted
        trajectory and run ids recorded
      proves: engineering and self-development work on records end to end, and run acceptance reads the fold
      evidence_class: deployed proof
    - action: >-
        node scripts/reeng_tracer.mjs --scenario research --expect-sha
        <step-5 sha>: a research cast spawns sub-researchers that appear
        as runs on records; the all-children barrier in
        research_children.go is gone; the parent reads its children's
        Reports from records
      proves: L5 on the live research path
      evidence_class: deployed proof
    - action: >-
        node scripts/reeng_tracer.mjs --scenario paper --expect-sha <sha>
        after the pushes for steps 2, 5 and 7 and on the final commit
      proves: Autopaper still opens, writes and publishes stories at every step that touches store/lifecycle.go, research_children.go or the Autopaper residue
      evidence_class: deployed proof
    - action: >-
        the crash scenario on the final commit, with I2 and I3 re-expressed
        on records before their old inputs are deleted; the expected
        verdicts are fixed in advance: I3 pass, and I2 reading open rounds
        on records, where a round open at the crash closes with fate
        interrupted_by_restart, so I2 passes
      proves: crash closure still holds and the checker still sees it once its old inputs are deleted
      evidence_class: deployed proof
  rollback: >-
    revert per step (each step is its own commit or batch); tag the
    last-good commit before step 4, before step 7 and before step 8. Step 8
    keeps the registry kinds choir.work_item and choir.worker_update
    decodable (vocabmigrate historical decode) so old tapes still replay
    and a revert alone restores the prior build on a computer whose tape
    moved on
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Fewer authorities for the same fact: one work state (records) for every
    desk, measured as production lines naming work items or packets going
    to zero while the deployed scenarios keep passing.
  goodharting_would_be: >-
    Renaming the types or hiding them behind aliases so the grep reads
    zero; deleting the checker inputs (I2, I3) instead of re-expressing
    them; passing the gate by excluding the files that still use them;
    leaving management's engineering refusal in place and calling
    engineering "on records" because nothing reaches it.

homotopy:
  realism_axis: >-
    unit and reducer tests -> CI with the gate in report mode -> a minted
    test computer on staging (self-development round trip, paper, crash)
    -> the paper computer -> the owner's computer after an operator
    refresh

boundaries:
  mutation_class: red
  authority_sources:
    - docs/re-engineering-decisions-2026-10-12.md (§2, §9, §10, §12; D9 for the Verify binding)
    - docs/definitions/choir-reeng-cut1-authority-2026-10-12.md scope_split (convergent panel 2026-10-13, .agentic-consensus/c1-scope-2026-10-13)
    - docs/definitions/choir-reeng-metamission-2026-10-12.md (C1e station; runs after C2's autopaper_live and L1a)
    - docs/choir-doctrine.md (Texture sole canonical writer; management admits engineering through delegated casts; engineering capsule-bound mutation)
  must_preserve:
    - Texture is the sole writer of canonical document revisions
    - engineering mutates only inside a capsule management admitted
    - a crash boot never resumes or re-drives work (L1a)
    - Autopaper keeps publishing through every step; a step that breaks the paper scenario is reverted, not patched forward
    - old tapes still replay: historical choir.work_item and choir.worker_update objects stay decodable after step 8
    - self-development materialization stays refused for engineering-on-records work until management Verify (step 4) lands; steps 1-4 go out in one push
  excluded:
    - the packet body CoagentSourcePacketPayload (the live Report content); rename it only if the gate needs it, as its own commit
    - schedules option B, restart subsystem L1b/L1c work, and the vocabmigrate historical decoders
  protected_surfaces: [run acceptance, self-development materialization, Texture canonical writes, Trace/evidence (the checker), management admission of engineering]
  admissible_evidence: deployed proof on a minted test computer on staging for behavior; CI for the gate; unit and reducer tests shape a step but never prove it
  known_costs:
    - the cutover bumps the work-state epoch (cut 1 D7), so the first boot after it is treated as a crash boot; any engineering work item or packet open on a computer then closes with fate interrupted_by_restart instead of carrying over
    - engineering's records surface reaches a computer only with a capsule image update; step 1 is not proven until acceptance 3 runs on a computer that has it
  heresy_delta:
    discovered: [engineering refused on records turns since C1-core, path B dual routing, checker invariants bound to the tables being deleted, a deletion gate that a literal-substring checker cannot express as drafted]
    introduced: []
    repaired: []

now:
  status: blocked
  slice: >-
    Drafted from the inventory; convergent round 1 sent it back 7 of 7
    and every blocking finding is adopted (gate in the checker's literal
    syntax with a planted-hit proof, the engineering prompt and parity
    test, acceptance lines for L5 and L7 and a named engineering scenario,
    paper runs at steps 2, 5 and 7, step 2 narrowed, instruments first as
    step 0). Round 2 next
  source_ref: 29bbab97
  deploy_identity: staging 18d01cc0
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: C1e-1
    claim: >-
      Records already carry everything the work-item and packet tables
      carry for engineering (conversation, round, Report, cursor), so
      moving engineering onto records and deleting path B removes the last
      readers of both tables without a new mechanism.
    test: steps 1-4 land with the reducer tests and a deployed round trip (acceptance 3), and no step through step 7 needs a new table or record kind
    edge: missing_oracle
    delta_o: the I7 checker widened to every route, reading zero minted work items and packets on a deployed engineering round trip
    scope_if_supported: engineering and self-development on staging
    status: untested
    evidence_refs: [docs/reports/c1e-deletion-inventory-2026-10-13.md]
  decision: {what: "run the inventory's eight steps in order, each its own commit, steps 1-4 batched per push", kind: operational, status: proposal, evidence_ref: docs/reports/c1e-deletion-inventory-2026-10-13.md, owner_ratification_ref: not_applicable}
  belief:
    believed_state: >-
      Nothing of C1e is landed. L1a is landed, so the boot re-drivers C1e
      shares are settled. The gate's regex trap (CoagentSourcePacket
      matching the live Payload type) is known and avoided with word
      boundaries.
    main_uncertainty: whether management_controller.go's packet activation can be removed in step 7 without a records-side replacement for terminal outcomes reaching management
    next_observation: what management reads today from a terminal-outcome packet that a records Report does not carry; answered in writing, with records-side delivery, cursor and wake evidence for succeeded, failed and cancelled outcomes, before step 7 starts
  blocker_or_risk: >-
    Not reviewed yet. Riskiest coupling: store/lifecycle.go and
    management_controller.go on the live Autopaper path. Second: deleting
    I2/I3's inputs before re-expressing them on records would blind C1e's
    own acceptance instrument
  next_action: convergent round 2 on the repaired file; on accept, promote to reviewed and repoint the metamission's C1e station here; then step 0 (the payload rename and the gate row in report mode), then step 1

receipts: []
---

# C1e: engineering on records, then the deletions

The scope is cut 1's `scope_split`; the map is the inventory. This file
tracks execution.

Order of work (inventory §5, repaired after convergent round 1):

0. **Instruments first.** Rename `CoagentSourcePacketPayload` (own
   commit); land the gate row in report mode; widen I7 to every route and
   re-express I2/I3 on records, so every later step is measured by an
   instrument that already landed.
1. **Engineering conversation on records.** Engineering joins
   `recordsDesk` and `recordsRoute` (management to engineering); the
   capsule gets `Records`; engineering loses `Inbox`, `Updates`, `Emits`,
   `Assign`, `Message` and `Emit`; the refusal at
   `inbox_records.go:360` goes; engineering's Report goes to management
   instead of `buildEngineeringReturnPacket`; engineering's prompts and
   verb list teach only what it has, with the reverse parity test
   (acceptance 2).
2. **Records-authorized assignments.** Conversation and round authority
   replace `ParentWorkItemID`/`AssignedWorkItemID`.
   `RequireNoOpenWorkItems` leaves the engineering seed and
   `selfdev_texture_join.go` only; its live-path setters
   (`texture_lifecycle_create.go`, `texture_handoff.go`,
   `coagent_route.go`, `story_open.go`) and its enforcement die in step 8
   with the tables. Paper run after this push.
3. **Delete path B.** Retarget, supervision mint, `selfdev_texture_join.go`,
   recast and judge, Texture exclusions, the handler entry, the path-B
   branch in `run_acceptance.go`.
4. **Management Verify** bound to the operation id and bundle digest the
   engineering Report names; then the self-development round trip on
   records, with the new `--scenario engineering` (acceptance 3). Steps 1-4
   ship in one push; materialization stays refused until Verify is in.
5. **L5**: sub-researchers as runs (acceptance 4), with a paper run after
   its push: `research_children.go` is on the live research path.
6. **L7**: run acceptance on the fold (after 3; proven inside acceptance 3).
7. **Autopaper residue**, gated on the written answer to the main
   uncertainty: Texture's work-item fallbacks, `Updates`, the
   terminal-outcome packets, spawned work items and the assigned-work
   sweep, actorruntime occurrence wakes, management's packet activation.
   Own paper runs (acceptance 5).
8. **Store and types**, keeping historical decode, then the gate turns to
   `enforce: zero` with a planted-hit proof (acceptance 1) and the crash
   scenario runs (acceptance 6).
