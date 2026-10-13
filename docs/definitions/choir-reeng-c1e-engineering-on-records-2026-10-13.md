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
  reviewer: agentic-consensus convergent rounds 1 (79d60be4, 7 of 7 send_back) and 2 (1617a002, send_back; repairs confirmed, remaining findings adopted below); round 3 next
  frozen_ref: none (round 3 binds it)
  verdict: send_back
  evidence_ref: .agentic-consensus/c1e-station-convergent-r1-2026-10-13/, .agentic-consensus/c1e-station-convergent-r2-2026-10-13/

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
    a heresy-detector row (enforce: zero) for cut 1's deletion vocabulary
    green on main; the cut-1 self-development drill passing on records on a
    minted test computer; the paper, research, engineering and crash
    scenarios passing on the final deletion commit
  batches: >-
    Commit order is the eight steps below; deploy order is five batches,
    each fenced with --expect-sha on its own deployed identity. B0 = step 0
    (instruments). B1 = steps 1-4 in one push. B2 = step 5. B3 = step 6.
    B4 = step 7. B5 = step 8. A batch is rolled back whole, never a step
    inside it.
  acceptance:
    - action: >-
        The deletion gate: a docs/heresy-detectors.md row in the checker's
        own syntax (scripts/check-heresies.sh counts literal substrings, so
        one backticked literal per name): `WorkItem`, `workItem`,
        `work_item`, `workitem`, `CoagentSourcePacket`, `worker_update`,
        `update_coagent`, `RequireNoOpenWorkItems`, `"Assign"`,
        `choir.Message`, `choir.Emit`, `textureSupervision`,
        `MaybeAutonomousPublishWireArticle`, `wire_publication`, and the
        round-cap and budget identifiers cut 1 L2 named, their exact
        literals fixed in step 0 from what C1-core left. The row's notes
        end the exclude list with a semicolon so the parser takes only
        patterns: `exclude: docs/**, *.md, *_test.go, *_test.mjs,
        frontend/tests/**, internal/vocabmigrate/families.go;` — docs and
        tests are history and instruments, not code; families.go is the
        frozen list of retired kind names, which decodes nothing. Every
        other hit is deleted, including scripts/reeng_tracer.mjs and
        scripts/reeng_invariants.mjs once re-expressed. The live body type
        CoagentSourcePacketPayload is renamed in its own commit in step 0,
        so `CoagentSourcePacket` cannot match it. The row lands in report
        mode in B0 (its count is the progress measure) and turns to
        enforce: zero in B5, with a planted-hit commit on a branch shown
        failing CI once
      proves: the deleted names have no non-test citer, old objects stay in og_objects as untyped history and no code decodes them (cut 1), and the gate is not vacuous
      evidence_class: CI
    - action: >-
        engineering's prompts (internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml
        and its desk overlay) and generated verb list name only its
        post-step-1 module set (WriteFile, Exec, Complete, Freeze, Report,
        Ask, Reply, Note; cut 1 Routing), with the reverse parity test
        (internal/yaegikernel/choir_parity_test.go) extended to
        engineering and shown failing once on a planted stale verb
      proves: engineering is not taught verbs it no longer has
      evidence_class: CI
    - action: >-
        The cut-1 self-development drill, node scripts/reeng_tracer.mjs
        --scenario engineering (authored in step 4) on a fresh test
        computer: management Casts engineering on records; engineering
        Freezes; management Verifies against the bundle digest named in
        engineering's final Report; the owner approves; the materializer
        applies (planned restart with marker); then a restore to the
        pre-apply head. Run on B1's identity (without the fold-derived
        acceptance, which lands in B3), on B3's identity with run
        acceptance synthesized from the fold for the round-trip
        trajectory, and on B5's identity
      proves: Freeze, Verify, decision, apply and restore work on records and, on B5, with work items and packets deleted; the apply restart resumes once and the restore lands on the prior head; on B3 and B5, run acceptance derives from the fold and the round trip reaches the absolute target in cut 1's artifact 0
      evidence_class: deployed proof (operation states, route receipt and event heads through the product API; accepted trajectory, run and acceptance ids)
    - action: >-
        node scripts/reeng_tracer.mjs --scenario research, extended in step
        5: research spawns two or more sub-researchers; run on B2's and
        B5's identities
      proves: child runs leave events, report only to the parent inbox, and the parent cell does not block on them (cut 1 L5); the all-children barrier in research_children.go, kept on C2's research path until now, is gone
      evidence_class: deployed proof
    - action: >-
        node scripts/reeng_tracer.mjs --scenario paper on B1's, B2's, B4's
        and B5's identities
      proves: Autopaper still opens, writes and publishes stories after every batch that touches store/lifecycle.go, research_children.go or the Autopaper residue
      evidence_class: deployed proof
    - action: >-
        The checker without false passes, in B0: the exporter serves an
        authoritative census of forbidden objects (work items and packets
        minted, by route) and I2, I3 and I7 fail when a mandatory input is
        missing or a verdict is unknown, shown once each on a planted
        snapshot. I7 is route-scoped (its records routes gain the
        engineering pairs in B1) until B4, when it becomes zero minted
        everywhere. I2 and I3 read open rounds on records
      proves: every later acceptance is measured by an instrument that already landed and that cannot pass on missing evidence
      evidence_class: CI plus one deployed checker run on B0's identity
    - action: >-
        the crash scenario on B5's identity, with verdicts fixed in
        advance: I3 pass; I2 pass, a round open at the crash closing with
        fate interrupted_by_restart
      proves: crash closure still holds once the checker's old inputs are deleted
      evidence_class: deployed proof
  rollback: >-
    Forward-fix (decisions §12.3; cut 1 rollback). Tag the last-good commit
    before each batch: reeng-c1e-pre-b0 ... reeng-c1e-pre-b5. Staging test
    computers are disposable and are re-minted, never reverted: old code
    cannot read new-format records. Break-glass: redeploy the tag before
    the failing batch and mint fresh test computers. The paper computer and
    the owner's computer take each batch at their next idle refresh like
    any other; a batch that breaks the paper scenario is fixed forward or
    its tag redeployed before the paper computer refreshes
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
    - Autopaper keeps publishing through every batch
    - old work-item and packet objects stay in og_objects as untyped history; no code decodes them after B5 (cut 1)
    - self-development materialization is never reachable without management Verify: steps 1-4 deploy together as B1
  excluded:
    - deleting the packet body: CoagentSourcePacketPayload is the live Report content; step 0 renames it and nothing more
    - schedules option B and restart subsystem L1b/L1c work
  protected_surfaces: [run acceptance, self-development materialization, Texture canonical writes, Trace/evidence (the checker), management admission of engineering]
  admissible_evidence: deployed proof on a minted test computer on staging for behavior; CI for the gate; unit and reducer tests shape a step but never prove it
  known_costs:
    - B1 and B5 bump the work-state epoch (cut 1 D7), so the first boot after each is treated as a crash boot; any engineering work item or packet open on a computer then closes with fate interrupted_by_restart instead of carrying over
    - engineering's records surface reaches a computer only with a capsule image update; step 1 is not proven until acceptance 3 runs on a computer that has it
  heresy_delta:
    discovered: [engineering refused on records turns since C1-core, path B dual routing, checker invariants bound to the tables being deleted, a deletion gate that a literal-substring checker cannot express as drafted]
    introduced: []
    repaired: []

now:
  status: blocked_incomplete
  slice: >-
    Drafted from the inventory. Round 1 sent it back 7 of 7; round 2
    confirmed those repairs and sent it back on what remained, all adopted:
    cut 1's full self-development drill and L5 discriminators restored,
    commit order separated from five deploy batches, forward-fix rollback
    and untyped history per cut 1, a checker that fails on missing
    evidence, the gate's full cut-1 name list with a parser-safe exclude
    list, engineering's Cast removed. Round 3 next
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
    status: proposed
    evidence_refs: [docs/reports/c1e-deletion-inventory-2026-10-13.md]
  decision: {what: "run the inventory's eight steps in order, each its own commit, steps 1-4 batched per push", kind: operational, status: proposal, evidence_ref: docs/reports/c1e-deletion-inventory-2026-10-13.md, owner_ratification_ref: not_applicable}
  belief:
    believed_state: >-
      Nothing of C1e is landed. L1a is landed, so the boot re-drivers C1e
      shares are settled. The gate is a literal-substring row, so the live
      CoagentSourcePacketPayload is renamed in step 0 before the row lands.
    main_uncertainty: whether management_controller.go's packet activation can be removed in step 7 without a records-side replacement for terminal outcomes reaching management
    next_observation: what management reads today from a terminal-outcome packet that a records Report does not carry; answered in writing, with records-side delivery, cursor and wake evidence for succeeded, failed and cancelled outcomes, before step 7 starts
  blocker_or_risk: >-
    Not reviewed yet. Riskiest coupling: store/lifecycle.go and
    management_controller.go on the live Autopaper path. Second: deleting
    I2/I3's inputs before re-expressing them on records would blind C1e's
    own acceptance instrument
  next_action: convergent round 3 on the repaired file; on accept, promote to reviewed and repoint the metamission's C1e station here; then B0

receipts: []
---

# C1e: engineering on records, then the deletions

The scope is cut 1's `scope_split`; the map is the inventory. This file
tracks execution.

Order of work (inventory §5, repaired after convergent rounds 1 and 2).
Steps are commits; batches B0-B5 are deploys (see `finish.batches`).

0. **Instruments (B0).** Rename `CoagentSourcePacketPayload` (own
   commit). Land the gate row in report mode with the exact cap and budget
   literals. Make the exporter serve a forbidden-object census and the
   checker fail on missing inputs or unknown verdicts; re-express I2/I3 on
   records; re-point `scripts/reeng_tracer.mjs` off `work_items`. Write the
   answer to the main uncertainty (what management reads from a
   terminal-outcome packet) if it is cheap now; it gates step 7.
1. **Engineering conversation on records (B1).** Engineering joins
   `recordsDesk` and `recordsRoute` (management to engineering); the
   capsule gets `Records`; engineering's module set becomes WriteFile,
   Exec, Complete, Freeze, Report, Ask, Reply, Note, losing `Cast`,
   `Inbox`, `Updates`, `Emits`, `Assign`, `Message` and `Emit`; the
   refusal at `inbox_records.go:360` goes; engineering's Report goes to
   management instead of `buildEngineeringReturnPacket`; its prompts and
   verb list teach only that set, with the parity test (acceptance 2).
2. **Records-authorized assignments (B1).** Conversation and round
   authority replace `ParentWorkItemID`/`AssignedWorkItemID`.
   `RequireNoOpenWorkItems` leaves the engineering seed and
   `selfdev_texture_join.go` only; its live-path setters
   (`texture_lifecycle_create.go`, `texture_handoff.go`,
   `coagent_route.go`, `story_open.go`) and its enforcement die in step 8.
3. **Delete path B (B1).** Retarget, supervision mint,
   `selfdev_texture_join.go`, recast and judge, Texture exclusions, the
   handler entry, the path-B branch in `run_acceptance.go`.
4. **Management Verify (B1)** bound to the operation id and bundle digest
   the engineering Report names; the `--scenario engineering` drill
   (acceptance 3). B1 deploys here, with a paper run.
5. **L5 (B2)**: sub-researchers as runs; the research scenario extended
   (acceptance 4); paper run.
6. **L7 (B3)**: run acceptance on the fold; the engineering drill again
   with fold-derived acceptance.
7. **Autopaper residue (B4)**, gated on the written answer to the main
   uncertainty, with records-side delivery, cursor and wake evidence for
   succeeded, failed and cancelled outcomes: Texture's work-item
   fallbacks, `Updates` and `update_coagent`, the terminal-outcome
   packets, spawned work items and the assigned-work sweep, actorruntime
   occurrence wakes, management's packet activation; I7 becomes zero
   everywhere; paper run.
8. **Store and types (B5).** The work-item and packet tables, types,
   registry kinds, the typed migrators in `internal/vocabmigrate`
   (`MigrateWorkItemRecord` and its kin) and `internal/store/vocab_migrate_og.go`'s
   uses, `cmd/choir/trace.go`; then the gate turns to `enforce: zero`
   with its planted-hit proof, and the engineering, research, paper and
   crash scenarios run on B5's identity.
