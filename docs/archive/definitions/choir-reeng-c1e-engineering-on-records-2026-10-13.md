---
definition_version: 4

# Station C1e of the re-engineering metamission
# (docs/archive/definitions/choir-reeng-metamission-2026-10-12.md): engineering and
# self-development on records, then the wholesale deletion of packets and
# work items. Scope is cut 1's scope_split (C1e items) in
# docs/archive/definitions/choir-reeng-cut1-authority-2026-10-12.md; the code map
# and the dependency order are docs/archive/reports/c1e-deletion-inventory-2026-10-13.md
# (read-only inventory on 18d01cc0). This file is the executable station
# over that scope; it adds no scope.
readiness: reviewed

review:
  reviewer: agentic-consensus convergent rounds 1 (79d60be4, 7 of 7 send_back), 2 (1617a002, send_back), 3 (ac4885d1; claude, codex, gemini, deepseek, glm accept; gpt-6.1-sol send_back, its three findings and the accepters' non-blocking notes adopted after the round; opencode failed to run)
  frozen_ref: git blob d7b956d944653201f1d1daf3754c0d598301cb7c (this file at ac4885d1; sha256 e533a369…)
  verdict: accept
  evidence_ref: .agentic-consensus/c1e-station-convergent-r1-2026-10-13/, -r2-2026-10-13/, -r3-2026-10-13/

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
    Commit order is the nine steps (0-8) below; deploy order is six batches,
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
        frozen kind-membership table for historical vocabulary
        (DecodeOGObjectKind recognizes retired kinds; it decodes no
        work-item or packet object into a live type). Every
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
      proves: Freeze, Verify, decision, apply and restore work on records and, on B5, with work items and packets deleted; the operation is opened by Freeze as the candidate bundle's effect journal, with no public start-operation route, no one-open-operation lock and no engineering-fate auto-advance or auto-fail; the apply restart resumes once and the restore lands on the prior head; on B3 and B5, run acceptance derives from the fold and the round trip reaches the absolute target in cut 1's artifact 0
      evidence_class: deployed proof (operation states, route receipt and event heads through the product API; accepted trajectory, run and acceptance ids)
    - action: >-
        node scripts/reeng_tracer.mjs --scenario research, extended in step
        5: research spawns two or more sub-researchers; run on B2's and
        B5's identities
      proves: child runs leave events, report only to the parent inbox, and the parent cell does not block on them (cut 1 L5); the all-children barrier in research_children.go, kept on C2's research path until now, is gone
      evidence_class: deployed proof
    - action: >-
        L7's story baseline: on B3's and B5's identities, the tracer story
        scenario's fold-derived run acceptance is fetched and compared with
        Cut 0's recorded level (docs/archive/reports/reeng-cut0-measurements-2026-10-12.md
        §4), recording trajectory, acceptance id, level reached and the
        comparison
      proves: the tracer story reaches at least Cut 0's recorded level with acceptance derived from the fold (cut 1 L7)
      evidence_class: deployed proof
    - action: >-
        node scripts/reeng_tracer.mjs --scenario paper on B0's, B1's, B2's,
        B4's and B5's identities
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
    the failing batch and mint fresh test computers. Live computers (the
    paper computer, the owner's) are refreshed by the deploy itself when
    idle (.github/workflows/ci.yml), before any acceptance run, so a
    failing batch can reach them first: their recovery is forward-only.
    Before B1, B4 and B5 the paper computer's publications are recorded
    (count, routes, latest versions) and re-read after its refresh, so the
    cutover shows its published history retained
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
    - docs/archive/definitions/choir-reeng-cut1-authority-2026-10-12.md scope_split (convergent panel 2026-10-13, .agentic-consensus/c1-scope-2026-10-13)
    - docs/archive/definitions/choir-reeng-metamission-2026-10-12.md (C1e station; runs after C2's autopaper_live and L1a)
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
  status: working
  slice: >-
    2026-10-14 05:25 UTC: B1 is deployed (e9d727a7 with the homepage
    change; CI 38509908096 success; 549 of 549 health probes 200 through
    the deploy; rollback tag reeng-c1e-pre-b1 = bf7e855f). Acceptance:
    - paper scenario: first run (arxiv:cs_ai) fetched 0 items because
      arXiv's cs.AI feed was empty at 04:22 UTC (environmental, read
      directly); rerun on rss:hn_frontpage passed: 3 stories opened and
      published in auto mode, public routes resolve (receipt
      ~/.config/choir-qa/traces/c1e-b1-paper-hn-0438/).
    - engineering drill (scenario authored tonight, 0f398fd2): FAILED.
      Management Casts engineering on records with the right parent, but
      each assignment ends with a cancel report about 10 s later and
      engineering never freezes (three Casts, drill timed out). Problem
      doc docs/archive/problems/c1e-b1-engineering-assignments-cancelled-on-arrival-2026-10-14.md
      (a60591f5); diagnosis from the trace is in progress.
    Earlier (B1 build): steps 1-4 on branch c1e-b1, rebased onto main
    with L2a (no semantic clash; management has Stories, Feed, Decline
    and Verify), net about -3,700 lines; open points: the store cancel
    cannot stop a live loop (stopCancelledAssignmentRun); the reconcile
    and the run-end settle can each mint the final Report; leftover dead
    code goes to a follow-up deletion.
  source_ref: e9d727a7
  deploy_identity: staging e9d727a7
  candidate: {id: none, state: none, ref: none, base: none, digest: none, scope: []}
  conjecture:
    id: C1e-1
    claim: >-
      Records already carry everything the work-item and packet tables
      carry for engineering (conversation, round, Report, cursor), so
      moving engineering onto records and deleting path B removes the
      engineering readers of both tables without a new mechanism (the
      Autopaper readers go in step 7).
    test: steps 1-4 land with the reducer tests and a deployed round trip (acceptance 3), and no step through step 7 needs a new table or record kind
    edge: missing_oracle
    delta_o: the I7 checker route-scoped to the engineering pairs from B1, reading zero minted work items and packets on the deployed engineering drill; zero everywhere from B4
    scope_if_supported: engineering and self-development on staging
    status: weakened  # contradicted on the code read (packet-reads report); deployed test is B1's round trip
    evidence_refs: [docs/archive/reports/c1e-deletion-inventory-2026-10-13.md, docs/archive/reports/c1e-management-packet-reads-2026-10-13.md]
  decision: {what: "run steps 0-8 in order, each its own commit, deployed as batches B0-B5", kind: operational, status: proposal, evidence_ref: docs/archive/reports/c1e-deletion-inventory-2026-10-13.md, owner_ratification_ref: not_applicable}
  belief:
    believed_state: >-
      B0 is landed and its instrument is proven deployed. The census
      serves on 19e56c54 (0 work items, 0 packets on a fresh paper
      computer), and the checker reads I7 pass from it on the B0 paper
      bundle. I7 was unknown on the pre-B0 bundle (freemodels-paper-3).
      The packet that reaches management is engineering's
      assignment-report producer_report, not the synthetic run-outcome
      packet. A records Report lacks the assignment identity, a typed
      outcome (completed, partial, failed, blocked; verdict; cancelled),
      the trajectory, work and control join, and a per-run read mark.
      The records wake starts a fresh management run where the packet
      path resumes the parked caster.
    main_uncertainty: >-
      Whether B1 carries the assignment identity and typed outcome on the
      Report body, or has management read them from the assignment row
      (no new record kind either way). Also whether the records wake can
      resume a parked casting run without a second wake mechanism. The
      uncertainty moved from step 7 to B1.
    next_observation: >-
      B1's design note answers both before step 1's code, and is
      reviewed. The evidence half (records-side delivery, cursor and wake
      evidence for succeeded, failed and cancelled outcomes) comes from
      B1's deployed engineering round trip.
  blocker_or_risk: >-
    Riskiest coupling: store/lifecycle.go and management_controller.go on
    the live Autopaper path. Second: acceptance (ConsumedDeliveryUpdateIDs
    and the unseen-packet refusal of a terminal WorkDisposition) has no
    records counterpart; it must be redefined against the cursor or gain
    a per-run read mark. Observed on the B0 paper bundle, and also on
    freemodels-paper-3, so not B0's: I1 fails on a version-0 revision
    authored by system:story-open (the story seed revision, not written
    through the Texture desk cell path). This is a discovered heresy or a
    checker classification gap, and a residual for Cut 2's owner. I2's
    run half fails because a research run is still running at the
    paper scenario's export, which is expected mid-flight.
  next_action: >-
    The B1 design note is accepted (round 4, 7d218d6f, after send_backs in
    rounds 1-3). Steps 1-4 are being built in a separate worktree, branch
    c1e-b1, concurrently with L2a's design (owner's overnight plan
    2026-10-14). B1 deploys after L2a's deploy is verified, one
    shared-runtime cutover at a time. Before the B1 push: tag
    reeng-c1e-pre-b1 at the then-current main, and record the paper
    computer's publications (scripts/l2a_paper_window.py gives the
    counts).

receipts:
  - id: c1e-b1
    batch: B1
    pushed_commit: e9d727a7
    commits: [4a3ef2f2, 2caafead, ceef3a5e, 811bedc5, 37bb2c6b, 363b9645]
    ci: "38509908096 success"
    deploy: staging deployed_commit e9d727a7, 2026-10-14 about 04:20 UTC
    rollback_ref: reeng-c1e-pre-b1 (bf7e855f)
    deployed_acceptance: >-
      paper scenario on rss:hn_frontpage: pass (c1e-b1-paper-hn-0438).
      Engineering drill: fail (c1e-b1-engineering-0444): assignments end
      cancelled about 10 s after the Cast; see the problem doc.
  - id: c1e-b0
    batch: B0
    pushed_commit: 19e56c54
    commits: [5766c87c, 90f8b94e, 291ad648, 86e7ea0f, bce3d7bd, 19e56c54]
    ci: "38481001401 success (38476951550 on 86e7ea0f failed: crash-settle test race, fixed in 19e56c54)"
    deploy: staging deployed_commit 19e56c54, deployed_at 2026-10-13T23:04:18Z
    rollback_ref: reeng-c1e-pre-b0 (d2324a55)
    deployed_acceptance: >-
      node scripts/reeng_tracer.mjs --scenario paper --label c1e-b0-paper
      --expect-sha 19e56c54: pass. Computer
      computer-fc0f063d27cf207b30b977c4334e1773 (stopped, 201); 12 stories
      opened, 8 published in auto mode (one at v2); public bundle 200 with
      0 citations. Bundle sha256 f5bdd1f2... in
      ~/.config/choir-qa/traces/c1e-b0-paper/
    checker_run: >-
      node scripts/reeng_invariants.mjs bundle-a1.json --require I7: I7
      pass (census present, 0 objects), I6 pass. I1 and I2 fail as on
      freemodels-paper-3 (see blocker_or_risk). The I2, I3 and I7
      missing-input failures are shown on planted snapshots in
      scripts/reeng_invariants_test.mjs (44 tests pass)
    mutation_class: orange (census route, checker, tracer); red-adjacent observer change in computerevent (sync only)
    heresy_delta: {discovered: [I1 system:story-open seed revision], introduced: [], repaired: []}
---

# C1e: engineering on records, then the deletions

The scope is cut 1's `scope_split`; the map is the inventory. This file
tracks execution.

Order of work (inventory §5, repaired after convergent rounds 1-3).
Steps are commits; batches B0-B5 are deploys (see `finish.batches`).

0. **Instruments (B0).** Rename `CoagentSourcePacketPayload` (own
   commit) to a name containing no gate literal, keeping every JSON tag,
   so the rename is wire-neutral. Land the gate row in report mode with
   the exact cap and budget literals. Make the exporter serve a
   forbidden-object census whose kind names come from
   `internal/vocabmigrate/families.go` (gate-excluded, so the census
   outlives B5) and make the checker fail on missing inputs or unknown
   verdicts; re-express I2/I3 on records; re-point
   `scripts/reeng_tracer.mjs` off `work_items`. Write the answer to the
   main uncertainty (what management reads from a terminal-outcome
   packet) if it is cheap now; it gates step 7. The checker reads only
   tracked files, so `.claude/worktrees/` copies never count. Paper run.
1. **Engineering conversation on records (B1).** Engineering joins
   `recordsDesk` and `recordsRoute` (management to engineering); the
   capsule gets `Records`; engineering's module set becomes WriteFile,
   Exec, Complete, Freeze, Report, Ask, Reply, Note, losing `Cast`,
   `Inbox`, `Updates`, `Emits`, `Assign`, `Message` and `Emit`; the
   refusal at `inbox_records.go:360` goes; the six other verbs it has today
   (`CancelAct`, `Escalate`, `Precommit`, `ReportPacket`, `Resolve`,
   `Disagreement`) go too; engineering's Report goes to
   management instead of `buildEngineeringReturnPacket`; its prompts and
   verb list teach only that set, with the parity test (acceptance 2).
2. **Records-authorized assignments (B1).** Conversation and round
   authority replace `ParentWorkItemID`/`AssignedWorkItemID`.
   `RequireNoOpenWorkItems` is removed from the engineering seed and
   `selfdev_texture_join.go` and nowhere else; its live-path setters
   (`texture_lifecycle_create.go`, `texture_handoff.go`,
   `coagent_route.go`, `story_open.go`) and its enforcement die in step 8.
3. **Delete path B and self-development work state (B1).** Retarget,
   supervision mint, `selfdev_texture_join.go`, recast and judge, Texture
   exclusions, the handler entry, the path-B branch in
   `run_acceptance.go`. With it, cut 1's self-development deletions: the
   public start-operation API (`api_self_development.go`), the
   one-open-operation lock (`selfdev/operations.go`), engineering-fate
   auto-advance and auto-fail (`engineering_assignment_fate.go`) and the
   prompt special-casing. The operation stays only as the candidate
   bundle's effect journal, opened by Freeze; approval, materialization
   and restore routes stay.
4. **Management Verify (B1)** bound to the operation id and bundle digest
   the engineering Report names; the `--scenario engineering` drill
   (acceptance 3). B1 deploys here, with a paper run.
5. **L5 (B2)**: sub-researchers as runs; the research scenario extended
   (acceptance 4); paper run.
6. **L7 (B3)**: run acceptance on the fold; the engineering drill again
   with fold-derived acceptance, and the story baseline against Cut 0.
7. **Autopaper residue (B4)**, gated on the written answer to the main
   uncertainty, with records-side delivery, cursor and wake evidence for
   succeeded, failed and cancelled outcomes: Texture's work-item
   fallbacks, `Updates` and `update_coagent`, the terminal-outcome
   packets, spawned work items and the assigned-work sweep, actorruntime
   occurrence wakes, management's packet activation; I7 becomes zero
   everywhere; paper run.
8. **Store and types (B5).** The work-item and packet tables, types,
   registry kinds, the typed migrators in `internal/vocabmigrate`
   (`MigrateWorkItemRecord` and its kin), the kind literals in
   `internal/store/vocab_migrate_og.go`'s `ogIdentityFormulas`, the
   retirement guards that name the dropped tables (`store.go`
   `retiredRuntimeTables`, `replay_eligibility.go`), the retired-verb
   stubs in `yaegikernel/choir.go` (a stale call then gets the
   unknown-name hint), comment and identifier scrubs the gate finds, the
   frontend specs' `open_work_items`/`worker_updates_consumed`
   assertions, `cmd/choir/trace.go`; then the gate turns to `enforce: zero`
   with its planted-hit proof, and the engineering, research, paper and
   crash scenarios run on B5's identity.
