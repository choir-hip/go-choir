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
  reviewer: none yet (convergent panel next)
  frozen_ref: none
  verdict: none
  evidence_ref: none

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
    the other desks; path B is gone; management verifies an engineering
    Report against the operation and bundle it names; sub-researchers are
    runs on records (L5); run acceptance folds from records (L7); packets
    and work items are deleted from production code, with a CI gate that
    keeps them at zero.
  artifact: >-
    a heresy-detector row (enforce: zero) for the cut-1 vocabulary green on
    main; a deployed self-development round trip on records on a minted
    test computer; the paper and crash scenarios passing on the final
    deletion commit
  acceptance:
    - action: scripts/check-heresies.sh --fail-on-regression with the C1e row (WorkItem|work_item, \bCoagentSourcePacket\b, update_coagent, engineering Assign, choir.Message, choir.Emit, textureSupervision) at zero in production code, excluding tests, vocabmigrate historical decode and .claude/worktrees
      proves: the deletion holds and cannot regrow
      evidence_class: CI
    - action: a tracer scenario on a minted test computer where management casts engineering on records, engineering returns a Report naming an operation and bundle digest, management Verifies it, and the self-development decision materializes
      proves: engineering and self-development work on records end to end, with no work item or packet minted (I7 widened to every route)
      evidence_class: deployed proof
    - action: the paper scenario (scripts/reeng_tracer.mjs --scenario paper) on the commit that deletes the Autopaper residue (step 7) and again on the final commit
      proves: Autopaper still opens, writes and publishes stories after the packets and work items it used are gone
      evidence_class: deployed proof
    - action: the crash scenario on the final commit, with I2 and I3 re-expressed on records
      proves: crash closure still holds and the checker still sees it once its old inputs are deleted
      evidence_class: deployed proof
  rollback: revert per step (each step is its own commit or batch); tag the last-good commit before step 7 and before step 8
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
    - docs/definitions/choir-reeng-cut1-authority-2026-10-12.md scope_split (convergent panel 2026-10-13, .agentic-consensus/c1-scope-2026-10-13)
    - docs/definitions/choir-reeng-metamission-2026-10-12.md (C1e station; runs after C2's autopaper_live and L1a)
    - docs/choir-doctrine.md (Texture sole canonical writer; management admits engineering through delegated casts; engineering capsule-bound mutation)
  must_preserve:
    - Texture is the sole writer of canonical document revisions
    - engineering mutates only inside a capsule management admitted
    - a crash boot never resumes or re-drives work (L1a)
    - Autopaper keeps publishing through every step; a step that breaks the paper scenario is reverted, not patched forward
  excluded:
    - the packet body CoagentSourcePacketPayload (the live Report content); rename it only if the gate needs it, as its own commit
    - schedules option B, restart subsystem L1b/L1c work, and the vocabmigrate historical decoders
  protected_surfaces: [run acceptance, self-development materialization, Texture canonical writes, Trace/evidence (the checker), management admission of engineering]
  admissible_evidence: deployed proof on a minted test computer on staging for behavior; CI for the gate; unit and reducer tests shape a step but never prove it
  heresy_delta:
    discovered: [engineering refused on records turns since C1-core, path B dual routing, checker invariants bound to the tables being deleted]
    introduced: []
    repaired: []

now:
  status: blocked
  slice: >-
    Drafted from the inventory; needs a convergent review before it runs.
    Steps in the inventory's order: (1) engineering conversation on
    records, (2) records-authorized assignments, (3) delete path B, (4)
    management Verify and the self-development round trip, (5) L5 beside
    the deletions, (6) L7 after path B, (7) Autopaper residue with its own
    paper runs, (8) store and types, then enforce the gate
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
    test: step 1 and 2 land with the reducer tests and a deployed round trip, and no step needs a new table or record kind
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
    next_observation: what management reads today from a terminal-outcome packet that a records Report does not carry
  blocker_or_risk: >-
    Not reviewed yet. Riskiest coupling: store/lifecycle.go and
    management_controller.go on the live Autopaper path. Second: deleting
    I2/I3's inputs before re-expressing them on records would blind C1e's
    own acceptance instrument
  next_action: run a convergent panel on this file; adopt or answer its findings; promote to reviewed; then step 1

receipts: []
---

# C1e: engineering on records, then the deletions

The scope is cut 1's `scope_split`; the map is the inventory. This file
tracks execution.

Order of work (inventory §5):

1. **Engineering conversation on records.** Engineering joins
   `recordsDesk` and `recordsRoute` (management to engineering); the
   capsule gets `Records`; engineering loses `Inbox`, `Updates`, `Emits`,
   `Assign`, `Message` and `Emit`; the refusal at
   `inbox_records.go:360` goes; engineering's Report goes to management
   instead of `buildEngineeringReturnPacket`.
2. **Records-authorized assignments.** Conversation and round authority
   replace `ParentWorkItemID`/`AssignedWorkItemID`;
   `RequireNoOpenWorkItems` goes.
3. **Delete path B.** Retarget, supervision mint, `selfdev_texture_join.go`,
   recast and judge, Texture exclusions, the handler entry, the path-B
   branch in `run_acceptance.go`.
4. **Management Verify** bound to the operation id and bundle digest the
   engineering Report names; then the self-development round trip on
   records (acceptance 2).
5. **L5**: sub-researchers as runs (independent; beside 1-4).
6. **L7**: run acceptance on the fold (after 3).
7. **Autopaper residue**: Texture's work-item fallbacks, `Updates`, the
   terminal-outcome packets, spawned work items and the assigned-work
   sweep, actorruntime occurrence wakes, management's packet activation.
   Own paper runs (acceptance 3).
8. **Store and types**, the checker re-expressed on records first, then
   the gate set to `enforce: zero` (acceptance 1 and 4).
