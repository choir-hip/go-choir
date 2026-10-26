# U4 stack rebase, build delta (2026-10-26): stopped at step 2, the mint

Status: **build stopped before tests and code.** Reading the code for station X3
showed that step 2 of the U4 design ("Management casts one engineering assignment
of a new kind, `rebase`") has no mint path in the code. This document records
what exists, the structural finding, the options, and the failure modes, so the
choice is made on the record and nothing is improvised. Class when built: red
(cast gate, a new assignment kind, reconcile at boot), review tier 2. This commit
is green (docs only).

Design: `docs/design/computer-update-architecture-rnd-2026-10-24.md`, "U4 design:
rebasing an applied stack onto a new base". Problem: `docs/problems/promoted-stack-stays-base-moved-2026-10-26.md`;
residual SD-01.

## What already exists (read from code at 618727c97)

- Detection: `sourceproj.reconcileStack` answers `stack_base_moved`, keeps the
  projection commit on the old base `B` as HEAD, and `Result.Refusal()` makes the
  cast refuse (`internal/sourceproj/sourceproj.go`). `Contained` (three-way) already
  retires an upstreamed stack.
- The cast gate: `rt.sourceProjectionCastGate` (`source_projection.go`), called once,
  from `planEngineeringCast` (`engineering_assignment_runtime.go`), before
  `PreflightSourceSnapshot(ctx, "")`, which reads the repository HEAD.
- The apply check and its conflict naming: `capsule.CheckPatchAppliesToTree`,
  `CarriedPatchConflictError{Paths, Clean}`, reached through
  `Executor.CheckCarriedPatchApplies(patch, retryBaseRef)`.
- The offer, its staging and its remembered decision: `offerCarriedPatch`,
  `stageCarriedPatch`, `<root>/carried/offers/<assignment>-<attempt>.json`
  (`internal/agentcore/carried_patch.go`). Its history source is
  `storeCarriedHistory.CarriedCandidates(trajectory)`. An applied operation is not
  `roundCarriable` (only rejected or failed), so the stack top is never a candidate
  for it. That is by design ("an ordinary assignment still never carries an applied
  operation").
- Freeze: `composeFreezePatch` makes the bundle patch cumulative from the stack
  base using the capsule's pinned commit (`sourceproj.Cumulative`); `openFreezeCandidate`
  opens the operation keyed by `store.EngineeringFreezeIdempotencyKey(assignment)`.
  `CapsuleEffectBundle` has no lineage field; adding one changes
  `ComputeContentDigest` for bundles that carry it (omitempty keeps old ones).
- Settings: `store.KnownComputerSettings` (`computer_setting.go`) and
  `rt.computerSettingOn`. `source.rebase_offer` is not present anywhere.
- Crash rule: a crash boot closes open assignments "interrupted by a restart"
  (`engineering_assignment_boot_test.go`, `closeSelfDevelopmentOperationsAfterCrash`).
- Host side: `hostbuild.baseRefusal` already refuses a host build whose patch base
  is not the booted image's build commit (`ReasonBaseCommitMismatch`). There is
  **no** base check in the guest materializer (`self_development_materializer.go`);
  the design's "a U4 check, not read to exist today" is confirmed.
- Nothing partly built: no `rebase_offer`, `rebase_of`, `RebaseOf` or rebase
  assignment kind in `internal/`, `cmd/`.

## Structural finding: step 2 cannot be expressed on the records route

The design says management casts a `rebase` assignment, automatically, when the
switch is on and reconcile says `stack_base_moved`. In the code, the only way an
engineering assignment is ever created is a management turn's Cast on the records
route, and every layer of that path requires a Texture request:

1. `store.OpenEngineeringAssignment` refuses anything that is not a records
   binding: "only a records cast opens an assignment" (`engineering_assignments.go`).
2. `EngineeringAssignmentBinding.Validate` for a records binding requires
   `Kind == implementation` and `Attempt == 1`, a `ParentConversationID` that is
   management's conversation with a `texture:` inbox, and a positive `ParentRound`
   (`internal/types/engineering_assignment.go`). Kind `rebase` fails the
   `implementation or verification` check and the records check.
3. `requireEngineeringRecordsAdmission` requires the Cast's `ParentID` to be a
   Texture's Cast into management, its round still open, and the Texture document's
   `TrajectoryID` to equal the binding's trajectory.
4. `planEngineeringCast` reads that parent record and document, and the objective
   is read back from the Cast record (`assignmentObjective`). The final Report goes
   back through the parent round; progress relays go to the Texture inbox
   (`engineeringProgressRelays`, `ParentConversationID`).
5. `refuseCastOntoOpenAssignment`, the capsule tool gate
   (`requireCapsuleMutationRole`, kind must be implementation or verification),
   the tools overlay, run context, run acceptance and the store's report rules all
   switch on `assignment_kind` (about a dozen sites).

A boot-time or reconcile-time rebase has no Texture request, no open round, no
management cell and no Cast record, so none of the above can be satisfied. Also:

- The applied operation's own Texture document and trajectory are no help: that
  round is long closed, and a settled document refuses a new revise (TX-01,
  `docs/problems/settled-document-refuses-owner-revision-2026-10-26.md`).
- The capsule must start from `B'`, but the cast's preflight reads the repository
  HEAD, which at `stack_base_moved` is the projection commit on `B`. A rebase round
  needs `PreflightSourceSnapshot` on an explicit `capsule-source-git:<B'>` ref
  (the resume path of that function accepts one; the cast path does not use it).
- The carry's history is keyed by trajectory. The stack top lives on another
  trajectory, so the "one new source" for the offer is a new branch in
  `offerCarriedPatch`, not a reuse of the selection.

So the design's "reuse the existing cast gate and assignment kinds" understates the
work: the new thing is a **non-Texture mint** of an engineering assignment, which
touches the store's open command (tape writes, a new binding variant, a trajectory
the platform would have to create), the Report path, and about a dozen kind
switches. That is the red, tier 2 core of the station, not a small addition.

## Options (for FP0 or the owner; none is chosen here)

- **A. Platform-minted assignment (the design's intent).** A new
  `CastAuthority` (for example `platform_rebase`), a new `OpenEngineeringAssignment`
  variant that takes the objective from the binding, a platform-created trajectory,
  a Report path to management without a Texture round, and `Kind` or a lineage field
  threaded through the kind switches. Honest size: a store change plus a Validate
  change plus the report path; new concurrency on the assignment open. Tier 2.
- **B. Texture-triggered rebase (smaller, not automatic).** The owner, or a platform
  notice to the Texture of the stack operation, asks for the rebase as an ordinary
  Cast with `spec {"rebase_of": "<operation id>"}` on the existing records route
  (Kind stays `implementation`; the lineage is a binding or spec field). The gate
  exception admits a Cast whose `rebase_of` names the stack top and whose preflight
  is pinned to `B'`. Needs a live Texture round, so it is blocked whenever TX-01
  bites; "automatic" is lost.
- **C. Host-side rebase first (the design's later half).** The host replays the
  stack and returns a rebased bundle; no guest assignment at all. Out of scope for
  X3 but it removes the mint problem; the guest then only adopts an offered bundle.

Recommendation: A is the design's mechanism and is buildable, but it is a larger red
slice than the design says and deserves its own definition (store slice first, with
tests). B is not what the design describes and is not built here.

## Failure modes (written before any code)

The design's six, with the test that would pin each:

1. *Double apply* (patch partly in `B'`): hunks already present fail
   `CheckPatchAppliesToTree`, are named as conflicts, never applied twice. Test: a
   fixture base with one of two hunks already present; the check names that hunk's
   path.
2. *Base moves again*: the candidate is pinned to `B'`; the guest check that the
   materializer refuses a candidate whose `SourcePatchBaseCommit` is not the booted
   base is not built in U4 (it is `lane/base-join`, below; do not re-add it here). Test: apply of a bundle based on a non-booted commit is
   refused with a named reason; the next reconcile starts a new round.
3. *Loop*: one automatic round per (stack operation, booted base) pair, decided once
   and remembered under `<root>/rebase/`, like the carried offers; a failed or
   unfinished round does not re-cast at the next boot. Test: two reconciles and a
   simulated reboot cast once.
4. *Crash*: the crash boot closes the round "interrupted by a restart" and that
   counts as the pair's round; no boot-time resumer is added. Test: crash boot
   leaves the pair marked and cast count stays one.
5. *Rollback semantics*: rolling back the rebased operation restores the old stack on
   `B` and the computer returns to `stack_base_moved`, named in the receipt. Test:
   apply rebased, roll back, reconcile answers `stack_base_moved`.
6. *Gate exception widens*: admit a rebase round only if its pinned base equals the
   booted base and `rebase_of` names the current stack top; anything else keeps the
   refusal. Test: wrong base, wrong top, switch off, projection off: all refused.

Found while reading (additional):

7. *Switch off or `source.projection` off*: `source.rebase_offer` requires
   `source.projection`; with either off nothing is cast and the refusal text is
   today's. Test: both combinations.
8. *Held decision*: an open self-development operation on the computer already
   blocks a Freeze; a rebase round waits behind it (admission's
   `openOperationHold`). Test: rebase request queued while an operation is open does
   not freeze.
9. *Wrong source for the capsule*: the capsule must start from `B'`, not HEAD (the
   projection commit on `B`). If it started from HEAD the round would "rebase" onto
   the old base. Test: the rebase capsule's pinned commit equals the booted base.
10. *`B'` objects missing* (`base_objects_missing`, residual R3): no check and no
    round; the state stays as it is. Test: reconcile with missing objects casts
    nothing.
11. *Freeze patch cumulation*: Freeze must produce the stack's patch on `B'`, not a
    cumulative patch through the old projection commit. Test: rebased bundle's
    `SourcePatchBaseCommit` is `B'` and its patch applies to `B'` to give the same
    tree as `P` on `B` plus the named resolutions.
12. *Lineage in the bundle*: a new bundle field changes the content digest of bundles
    that carry it; verifiers that re-validate old bundles must be unchanged
    (omitempty). Test: an old bundle's digest is stable.
13. *Owner sees it*: the round must be visible (a Texture item or supervision
    document), or the owner will not know why the computer is casting by itself.
    Needs the same Texture binding as option A's trajectory; unresolved here.

## Files that would change (option A, for the next definition)

`internal/types/engineering_assignment.go` (binding variant and Validate),
`internal/store/engineering_assignments.go` (open variant),
`internal/store/computer_setting.go` (`source.rebase_offer`),
`internal/sourceproj/sourceproj.go` (a rebase-eligible state or flag on the result),
`internal/agentcore/source_projection.go` (gate exception, rebase pair record),
`internal/agentcore/engineering_assignment_runtime.go` (mint and `B'` preflight),
`internal/agentcore/carried_patch.go` (the stack-top source and notice),
`internal/agentcore/tools_capsule.go`, `engineering_assignment_tools_overlay.go`,
`tool_profiles.go`, `run_context.go`, `run_acceptance.go` (kind switches),
`internal/agentcore/self_development_materializer.go` (booted-base check),
`internal/capsule/transaction/builder.go` (lineage field, if kept in the bundle).

## Slices (proposed, after the mint is decided)

1. Store and types: the platform-minted binding and its open, with tests first.
2. Gate exception, switch, pair record, `B'` preflight (guest only, tests first).
3. Offer from the stack top, notice, conflict naming.
4. Materializer base check.
5. Staging proof on the test computer d@e per the design's "Proof on staging".

## Decision (2026-10-26, FP0 design worker): route D, the root Texture RLM is the parent

Revision 2. This version answers design review 303 (Devin and DeepSeek,
`_runs/bunch303`). Both reviewers confirmed that route D works and that the cast
gate cannot be forged. They sent the doc back before any build. "What changed
after review 303", at the end of this section, lists every change.

Owner rulings this section follows: "we need to rebase not reject security
updates. Security comes first." And on the route: "it shouldn't be a
platform-minted assignment. Use the root texture rlm." Terminology (owner): the
**Texture desk** is all of a computer's Texture RLMs, one desk per computer; the
master document's own RLM, `texture:<masterDoc>`, is **the root Texture RLM**.
Read against code at `618727c97`, which is unchanged under `internal/` since.

**Option A** (a platform-minted assignment) is ruled out by the owner. **B** stays
available as an owner act; it is not automatic. **C** (host-side rebase) is still
the later half of U4 and does not answer the automatic guest round.

**Not in U4: the base join.** A self-development apply that does not check its
candidate's base against the booted base is its own security slice. It is built
unconditionally (not behind any U4 switch) on `lane/base-join`, from
`docs/problems/selfdev-apply-has-no-base-join-2026-10-26.md` (that problem doc
lives on `lane/base-join` and reaches `main` when the lane lands; landing order:
`lane/base-join` first, then the U4 staging proof, slice 7). U4 depends on it
for failure mode 2 (the base moves again before apply), and the U4 staging proof
runs only after it has landed.

The first version of this doc overstated the security case. The accurate claim
has two parts:
- the apply has a provenance gap: the materializer never joins the frozen
  bundle's `SourcePatchBaseCommit` to the booted base;
- H1: computers on a self-development stack do not receive the platform's
  frontend fixes (R&D doc §1.1).

At `stack_base_moved`, the computer already serves the old base's frontend, and
an open operation blocks a newer Freeze. So a stale candidate there does not
revert a live fix. A revert needs the narrower state in which a newer-base
release is already current while an old-base candidate is frozen (DeepSeek's
trace).

### The route in one paragraph

1. When reconcile answers `stack_base_moved` with the switch on, and the computer
   has stayed up a short interval, the runtime mints one **rebase notice**.
2. The notice is a system Note into the root Texture RLM's inbox, with the
   owner-act record's shape. It wakes the root through the ordinary records wake.
3. The root opens a round with management: an ordinary Texture Cast whose packet
   names the notice.
4. Management casts an ordinary `implementation` assignment. Its spec gives the
   usual `parent` and also names the notice, the operation and the base. Its
   objective names the notice.
5. `planEngineeringCast` checks the live facts: the current stack top and the
   booted base. It then pins the capsule to `B'` instead of HEAD.
6. The store's records admission binds the notice to this one assignment inside
   the open's own transaction.
7. The rebase offer mounts the stack top's patch `P`. Freeze, verify, the owner's
   decision and the reports run on the existing path.
8. At materialization, the rebase path checks that the candidate really contains
   `P`, and writes the verified lineage as a receipt.

There is no new cast authority and no new assignment kind. There is no binding
field and no bundle field. The store gains one additive object kind, the notice
claim, and no table or column changes.

If the root or management misses its turn, or a crash comes before any
assignment exists, the pair is re-offered under a new attempt id. There are at
most three attempts per pair.

### The questions, answered from the code

1. **Can the root Texture RLM open a round on an event that is not owner text?**
   Yes. Three pieces of code already allow it:
   - A Texture records turn needs no owner text and no open work item
     (`internal/textureowner/texture_agent_revision.go:664-666`,
     `request_source: inbox_record`).
   - System records already wake Texture RLMs: support failures and owner acts
     (`internal/store/management_tasks.go:66-100`).
   - A Texture's Cast to `management` is an ordinary records route
     (`internal/agentcore/inbox_records.go:66-70`).

   The master document's trajectory is the one parent that never settles. Its
   standing work item is clamped open
   (`internal/textureowner/texture_turn_runtime.go:212-214`). A dead master is
   replaced at the next digest (`texture_root.go:427-473`).

   The records admission accepts the root's Cast unchanged
   (`internal/store/engineering_assignments.go:742-800`):
   - the parent is a Texture's Cast into `management:<owner>`;
   - its round is open;
   - the document names the binding's trajectory.

   Admission does not check that the trajectory is live (Devin's nit).
   `requireEngineeringParentAuthority` does (`:702`), and the open runs it right
   after admission.

   Discovered (not introduced): the root's ban on casting is one prompt sentence,
   "You never cast to research or management"
   (`internal/textureprompts/texture_root.txt:7`). No code check backs it. Route D
   adds a prompt exception for rebase notices only; the code checks that matter
   are the gate and admission below.
2. **What delivers to it.** A records wake, not a digest.
   - The notice is committed with `CommitInboxRecords`, which writes its own
     `inbox_record` wake row. The actor-wake outbox delivers it ("the sole
     post-restart delivery path", `internal/agentcore/runtime.go:975-980`).
   - The notice is `EmitterSystem`, so it joins the closed allowlist in
     `store.RecordWakesReader`. The precedent is the owner-act record
     (`internal/types/commitment_owner_act.go`,
     `internal/agentcore/selfdev_owner_act_wake.go:15-29`).
   - A digest is the wrong carrier for three reasons:
     - it is coalesced and debounced;
     - it lives only in memory;
     - it has no record id that the gate and admission can check.
   - The root's records turns always start a fresh run, never a parked one
     (`texture_controller.go:683-692`).
3. **The restart rules hold.**
   - *Never inside boot.* Boot reconcile runs after the crash closures
     (`runtime.go:936-942`, then `:963`). Even so, reconcile only arms an
     in-memory timer, due at `bootedAt` plus the stay-up interval (default 5
     min). A crash forgets the timer, and the next boot recomputes everything from
     the tape.
   - *Crash before the root reads a notice.* The pending wake is a pre-boot
     `inbox_record`, so the generic gate interrupts it
     (`planned_restart.go:163-207`). The gate and admission refuse a notice
     committed before `rt.bootedAt` on a markerless boot, with the fate
     "interrupted by a restart". That attempt stays closed forever. After the
     stay-up interval, the timer may mint the next attempt (question 4).
   - *Crash after an assignment claimed the notice.* The crash boot closes the
     round and the assignment as today. The pair is then spent: capsule work has
     started, and that work may be what crashes the computer. A spent pair needs
     the owner's retry act (slice 8) or the next base.
   - *Why a re-mint after a crash boot is allowed.* It is new work, not resumed
     work, by the schedule precedent in CLAUDE.md and
     `recoverSourceSchedulesAtBoot` (`internal/agentcore/source_schedule.go:323-333`):
     "standing owner intents survive a crash; their occurrences do not", and the
     next tick is "armed one interval after boot, never at once: new work on a
     computer that has stayed up that long".
     - The standing intent is the owner's switch, `source.rebase_offer`.
     - The occurrence is one attempt.
     - An attempt never reopens an earlier attempt's record, round or wake. It is
       a new record under a new id, minted by a live timer after the stay-up
       interval.
     - The bound mirrors the schedule's crash-loop limit (two crashes): three
       attempts per pair, and none after an assignment exists.
     - No boot-time resumer is added.
   - *Planned restart.* A planned (marker) restart parks and resumes a live round
     as the existing rules do. If the base also moved to `B''`, the base join
     (`lane/base-join`) refuses the stale candidate at apply. That makes a new
     pair, whose notice waits until the stale round is terminal.
4. **How attempts are remembered, and the re-mint bound.** On the tape, as the
   one state authority.
   - *The notice.* Attempt `n` of pair (operation, `B'`) has the record id
     `rebase-notice:<op>:<B' full sha>:<n>`, for `n` = 1, 2, 3. The cap is 3. It
     is committed with **command id equal to record id**.
     - The same command replays.
     - A retry under any other command id is refused by the store's record-id
       uniqueness: `planInboxRecords` returns "record ... is already committed"
       (`internal/store/inbox.go:486-490`).
     - So both ids are pinned, and slice 1 tests both.
   - *The claim.* `rebase_notice_claim`, keyed by pair (not by attempt), is
     written once by the assignment open (question 5). Its existence means the
     pair is spent.
   - *At timer fire, the next attempt `n+1` is minted only when all of these
     hold:*
     - the switches are on and the state still holds (question 7, slice 2);
     - no claim exists for the pair;
     - `n < 3`;
     - attempt `n` is closed, meaning any one of:
       - it predates a markerless boot, or
       - no Cast from the current root names it within the answer window
         (default 30 min after its commit), or
       - a Cast named it but that round closed with no claim (management missed
         its turn, or a crash closed the round).
   - The gate and admission admit only the **latest** attempt, so an older
     attempt's late Cast is refused (`rebase_notice_superseded`).
   - After attempt 3 closes, or once a claim exists, nothing more is automatic.
   - A product restore that rewinds the tape past these records makes the pair
     undecided again. That is correct, because the restore is the owner's act.
5. **Management's Cast is bound, and one notice opens one assignment (atomic).**
   - *What management must send.*
     - Spec: `{"parent": <root Cast id>, "rebase_notice": <latest attempt id>,
       "rebase_of": <operation id>, "base": <B' full sha>}`.
     - Objective: must contain the notice id.
     - The root's Cast packet must carry the same `rebase_notice`.
   - *The runtime gate (`planEngineeringCast`)* checks the live facts the store
     cannot see:
     - `stack_base_moved`, with the booted base's objects present;
     - spec `rebase_of` equals `result.OperationID`, and spec `base` equals
       `result.BootedBase`;
     - the switches are on.
   - *The store's records admission (`requireEngineeringRecordsAdmission`)*
     checks the rest inside the open's own transaction. `OpenEngineeringAssignment`
     holds `trajectoryMu` from replay check to batch
     (`internal/store/engineering_assignments.go:1169-1261`). When management's
     Cast spec names a `rebase_notice`, admission requires all of these:
     - the notice record exists and passes `IsStackRebaseNotice`;
     - its inbox is the parent Cast's sender;
     - the parent's packet names the same notice id;
     - management's spec `rebase_of` and `base` equal the notice's subject refs;
     - the objective contains the notice id;
     - it is the latest attempt for its pair;
     - it is not before `bootedAt` on a markerless boot (the runtime passes
       `bootedAt` and `planned` in the request; the store does not read the
       clock).
     - no `rebase_notice_claim` exists for the pair.

     The open's batch then creates the claim (pair key, assignment id, attempt)
     under an object condition. Two racing opens cannot both commit: the mutex
     serializes them and the condition makes it atomic.

     This replaces the gate-side scan of the first version, which had a
     time-of-check gap (Devin).
   - *Spec missing.* A spec without `rebase_notice` takes today's path, so at
     `stack_base_moved` it is refused exactly as now.
   - `rebaseOfAssignment` reads the lineage from management's Cast spec, one hop
     from `Binding.ParentControlID`. Admission has already proved that it agrees
     with the notice.
6. **The carried offer can come from the stack top without ordinary assignments
   ever carrying an applied operation.**
   - The branch is chosen only for an assignment whose pair claim names it.
   - The source is `rt.boundSourcePatch(top)`: `P` and its base `B`.
   - The ordinary path (`selectCarriedCandidate`, `roundCarriable` at
     `carried_patch.go:221`) is untouched.
   - A rebase offer mounts `P` even on conflict, and names `ConflictPaths` and
     `CleanPaths`.
   - *Rollback (DeepSeek 8, refused, not documented away).* Rebase offers are
     written at `<root>/carried/rebase-offers/<assignment>-<attempt>.json` with
     schema `choir.rebase_patch_offer.v1`. They are never written at
     `carriedOfferPath`.
     - Old code reads only `carried/offers/` with schema
       `choir.carried_patch_offer.v1` (`carried_patch.go:427-429`, `:687`). It
       never sees a rebase offer, so it can never present `P` as an ordinary
       carry.
     - After a full revert, an admitted rebase assignment would start from `B'`
       with no mount. The rollback procedure cancels open rebase assignments
       first.
7. **Content is verified at materialization (DeepSeek 4).**
   - When an operation's assignment (`freeze-open:<assignment>`,
     `internal/store/engineering_assignments.go:905`) holds a pair claim, the
     materializer runs `verifyRebaseContent` before apply. It never runs for
     `journalOnly`.
   - In a scratch index from the guest repository (which holds `B` and
     `B'`), it computes three trees:
     - `T_P` = `B'` with `P` applied using `git apply --cached --3way` in a
       scratch index, recording which paths applied cleanly. `sourceproj`'s
       `patchedTree`/`scratchIndex` are index-only (`git apply --cached`,
       `git.go:122-140`) and do not do a 3-way apply today, so the per-path
       clean/conflict classification is a new helper in `sourceproj`, not a reuse;
     - `T_C` = `B'` with the bundle's patch applied;
     - and it requires `SourcePatchBaseCommit == B'`.
   - It requires all of these:
     - every clean path of `P` is byte-equal in `T_C` and `T_P`;
     - every conflict path of `P` (the offer's `ConflictPaths`) differs in `T_C`
       from `B'`, so it was resolved and not dropped. **Named residual
       (re-review 307, finding 2):** content cannot tell "resolved by keeping
       `B'`'s version" from "dropped". A candidate that legitimately keeps `B'`
       content at a conflict path is refused `rebase_content_mismatch` with the
       verdict `kept_base_unverifiable` and that path named. It is fail-closed, not
       silent. There is no escape in U4 (no marker channel is designed); the pair
       waits for the next base or slice 8's owner retry. A marker (a per-path
       `kept-base` line in the Freeze receipt) is the candidate remedy for a later
       slice, and is not built here;
     - paths the bundle changes outside `P` are allowed and listed.
   - A failure refuses the apply with `rebase_content_mismatch`, naming the
     paths. The computer keeps its release.
   - On success it writes the lineage receipt
     `<root>/rebase/lineage/<operation id>.json`, schema
     `choir.rebase_lineage.v1`, holding:
     - `rebase_of`, the attempt id, `stack_base` `B` and `base` `B'`;
     - the digests of `P` and of the bundle patch;
     - the clean, resolved and extra paths;
     - the verdict.
   - The receipt's sha256 goes into a `source_projection_rebased` fact, and the
     owner-act "apply" record body names it, so it reaches the master document.
   - The same check runs once at Freeze, so a bad candidate fails before the
     owner sees it. The materializer's run is the authority.
   - No bundle field is added, because the materializer decodes with
     `DisallowUnknownFields` (`self_development_materializer.go:331`).

### Failure modes added by route D (written before code; the original 13 stand)

- **D1. The root Texture RLM does not cast, or copies the notice id wrong.** The
  gate or admission refuses (`rebase_notice_mismatch`), the attempt closes, and
  the next attempt follows after the answer window. Bounded at 3.
  *Test:* a wrong id is refused, and attempt 2 is minted after the window.
- **D2. Management does not cast, or casts without the notice fields.** Without
  `rebase_notice` the spec takes today's refusal. A missing turn closes the
  attempt as in D1. *Test:* a spec without the notice is refused
  `stack_base_moved`.
- **D2a. Management names the notice but sends unrelated work.** Each of these
  is refused at admission (DeepSeek 3):
  - an objective without the notice id;
  - a spec whose `rebase_of` or `base` differs from the notice.

  *Tests:* both, under a valid latest notice. The objective test is the one the
  reviewers asked for.
- **D3. An owner request on the master document, cast by the root without a
  notice.** It goes down today's path and is refused at `stack_base_moved`.
- **D4. Progress relays and owner acts wake the root on every report, and each
  root run reads a full ledger refresh.** These wakes bypass the digest's
  debounce. *Measured* in the staging proof. If the count is more than about 12,
  a follow-up narrows the root's records turns.
- **D5. The master document is replaced between the notice and the Cast.**
  Refused, because the notice's inbox is not the parent's sender. The attempt
  closes and the next attempt goes to the new master.
- **D6. A notice from before a crash is acted on after it.** Refused, "interrupted
  by a restart". The next attempt follows after the stay-up interval.
- **D7. A forged notice.** Agents' records are always `EmitterAgent` from their
  own inbox (`inboxRecordForIntent`, `inbox_records.go:394-405`, and
  `outwardSender` at `:448-453`). `IsStackRebaseNotice` requires all of these:
  - `EmitterSystem`;
  - sender `selfdev-rebase:<op>`;
  - a record id equal to `StackRebaseNoticeID(op, base, n)` with `n` in 1..3;
  - matching subject refs.
- **D8. The root Texture RLM is off, or the marker is missing.** Nothing is
  minted, and no attempt is used up.
- **D9. Two opens race on one notice.** One commits and the other fails the claim
  condition (question 5). *Test:* two concurrent opens in the store, and exactly
  one claim.
- **D10. A re-mint loop.** Attempts are capped at 3, and none is minted after a
  claim. *Test:* repeated misses mint exactly 3 attempts, and later timer fires
  mint none.
- **D11. A rebased candidate that is not `P`.** Refused at materialization with
  `rebase_content_mismatch` (question 7). *Tests:* a dropped clean hunk is
  refused; an unresolved conflict path is refused; extra paths are listed and
  allowed.

### The switch

`source.rebase_offer`, a computer setting in `store.KnownComputerSettings`,
default false. It is effective only when `source.projection` and
`texture.root_desk` are also on (`rt.rebaseOfferOn`). The proof also turns on
`texture.owner_act_wakes`, so the owner's decision reaches the master document.
With the switch off:
- no timer runs and no attempt is minted;
- the gate never admits a rebase spec, so today's refusal text is unchanged;
- the content check is skipped, because no claim can exist;
- the root prompt's exception is inert.

The base join is not behind this switch (`lane/base-join`).

### The rollback

- *One act:* turn `source.rebase_offer` off. It takes effect at the next timer
  fire or gate read. A round already admitted runs on as an implementation round;
  its candidate still passes the content check, because the claim exists. The
  candidate still needs the owner's approval, and the owner can cancel the
  assignment.
- *Full:* first cancel open rebase assignments, then revert the slice commits.
  What old code sees afterwards:
  - Notices on the tape are inert to old code: they show as `closure`, and
    `RecordWakesReader` returns false.
  - Claim objects are an unknown kind that old code never reads.
  - Rebase offers sit at a path old code never reads (question 6).
  - No table, column, bundle or binding changed.

### Slices, in order (tests first in every slice)

**Slice 1 (red, tier 2): the notice record, the claim kind and the switch.**
- *Files:*
  - `internal/types/commitment_rebase_notice.go` (new):
    - `StackRebaseNoticeID(op, base, n)` and `StackRebasePairKey(op, base)`;
    - `StackRebaseSenderInbox(op)` = `selfdev-rebase:<op>`;
    - `NewStackRebaseNotice(...)`: a system Note with subject refs `operation_id`,
      `booted_base`, `stack_base`, `attempt` and `doc_id`. Its body states the
      exact root Cast, with the id;
    - `IsStackRebaseNotice(rec)`.
  - `internal/store/management_tasks.go`: `RecordWakesReader` returns true for a
    notice. Fix the stale comment at `:55-65`: it says "five" and lists four;
    list all six, adding the owner act and the notice.
  - `internal/store/inbox.go:270-277`: replace the stale three-item list with a
    pointer to `RecordWakesReader`'s comment, so there is one list.
  - `internal/agentcore/inbox_records.go` `inboxRecordKind`: `rebase_notice`,
    before `closure`.
  - `internal/store/rebase_notice_claim.go` (new): the
    `ogKindRebaseNoticeClaim` object and its get.
  - `internal/store/computer_setting.go`: `SettingSourceRebaseOffer`.
- *Failure modes:* D7; another system record starts waking; a duplicate notice
  under a new command id.
- *Tests:*
  - the D7 predicate table;
  - `RecordWakesReader` is true for a notice and false for a closure;
  - the same command replays;
  - a **new command id with the same record id is refused** (DeepSeek 7);
  - an unknown setting key is still refused.

**Slice 2 (red, tier 2): the trigger and the bounded re-mint.**
- *Files:* `internal/agentcore/rebase_notice.go` (new) and
  `internal/agentcore/source_projection.go`:
  - after `ReconcileSourceProjection`, call `rt.armRebaseNotice(result)`;
  - the timer is a mutex-guarded singleton;
  - the stay-up interval (5 min) and the answer window (30 min) are fields that
    tests override.
- *At fire:*
  - check the switches;
  - reconcile again;
  - require `StateBaseMoved` with `sourceproj.HasCommit(repo, booted)` and a
    readable non-genesis top;
  - require a live root, read through the store;
  - require no claim for the pair;
  - require no non-terminal rebase of the same operation (defer if one exists);
  - compute the latest attempt `n` from the tape;
  - mint attempt 1 if none exists, or attempt `n+1` under question 4's rule;
  - re-arm while the state holds and attempts remain.
- *Failure modes:*
  - a mint inside boot;
  - a mint after a crash before the stay-up interval;
  - D8, D10;
  - a mint at `base_objects_missing`.
- *Tests:*
  - the due time is no earlier than `bootedAt + interval`;
  - two reconciles and a simulated reboot with attempt 1 answered mint once;
  - on a crash boot with an unanswered pre-boot attempt 1, attempt 2 is minted
    only after the interval, and attempt 1 stays refused;
  - after a claim, nothing more is minted;
  - the cap is 3;
  - each switch off mints nothing;
  - objects missing mints nothing.

**Slice 3 (red, tier 2): the gate exception, admission binding and the `B'`
preflight.**
- *Files:*
  - `internal/agentcore/source_projection.go`:
    `sourceProjectionCastGate(ctx, spec rebaseSpec) (sourceRef string, err error)`.
    Update every caller and test.
  - `internal/agentcore/engineering_assignment_runtime.go:166-172` and `:258-261`:
    - `engineeringCastSpec` gains `RebaseNotice`, `RebaseOf` and `Base`;
    - preflight `sourceRef` when it is set;
    - pass `bootedAt` and `planned` into the open request.
  - `internal/types/engineering_assignment.go`: the open request (not the
    binding) carries `RebaseBootedAt` and `RebasePlannedBoot`, which are inputs
    to the command digest.
  - `internal/store/engineering_assignments.go`:
    - `requireEngineeringRecordsAdmission` runs the notice checks (question 5);
    - `OpenEngineeringAssignment` adds the claim object and its condition to the
      batch.
  - `internal/capsule/executor.go`: `PreflightSourceCommit(ctx, commit)`, with a
    stub in `stub_other.go`.
- *Failure modes:* failure modes 6, 9 and 10; D1, D2, D2a, D3, D5, D6, D9.
- *Tests:*
  - each admission condition is false in turn, and each is refused;
  - an unrelated objective under a valid notice is refused;
  - a superseded attempt is refused;
  - two concurrent opens produce one claim;
  - the admitted binding's `SourceArtifactRef` commit equals `B'`.

**Slice 4 (red, tier 1): the rebase offer and its notice.**
- *Files:* `internal/agentcore/carried_patch.go`:
  - `offerRebasePatch` writes to `carried/rebase-offers/`. Before writing it
    checks the patch digest, its size (`CarriedPatchMaxBytes`) and its paths, as
    the ordinary offer does. `ConflictPaths` and `CleanPaths` come from
    `CheckCarriedPatchApplies(P, B' ref)` (`CarriedPatchConflictError{Paths,
    Clean}`), and the patch is mounted even on conflict. An over-bound patch, a
    digest mismatch or a root mismatch writes no offer and the assignment starts
    from `B'` with no mount, with the reason on the tape as the ordinary offer
    does;
  - the spawn mount reads that path for a claimed assignment;
  - `carriedPatchNotice` gets the rebase wording.
- *Failure modes:* failure modes 1, 11 and 12; an ordinary assignment carrying an
  applied operation; old code reading a rebase offer; an over-bound or
  digest-mismatched `P`.
- *Tests:*
  - a `B'` already holding one of `P`'s two hunks names that path as a conflict;
  - an ordinary assignment on an applied trajectory gets no offer;
  - `carried/offers/` holds nothing for a rebase assignment;
  - the rebased Freeze's `SourcePatchBaseCommit == B'`, with `Cumulative`'s
    passthrough (`sourceproj/compose.go:46-49`).

**Slice 5 (yellow, tier 1): the prompts.**
- *Files:*
  - `internal/textureprompts/texture_root.txt`: one exception sentence for a
    `rebase_notice` record. Make the Cast its body gives, exactly once, and list
    it under "Waits on the owner".
  - `internal/runtimeprompts/overlays/rlm_management_runtime.yaml`: a root Cast
    naming a `rebase_notice` gets an engineering Cast with the spec of question 5,
    and an objective that contains the notice id.
- *Tests:* the root section contains the sentence;
  `TestPromptCoreRetiresTheOldWriteSurface` is unchanged.

**Slice 6 (red, tier 2): rebase content verification.**
- *Files:*
  - `internal/agentcore/rebase_content.go` (new): `verifyRebaseContent`,
    reusing `sourceproj`'s scratch-tree helpers;
  - `internal/agentcore/self_development_materializer.go`: a call after the
    bundle validates, `!journalOnly`, rebase path only;
  - the Freeze path, the same call;
  - the `source_projection_rebased` fact kind.
- *Failure modes:* D11; refusing a correct resolution; running on non-rebase
  operations or `journalOnly`.
- *Tests:*
  - a clean rebase passes and writes the receipt;
  - a dropped clean hunk is refused;
  - an unresolved conflict path is refused;
  - extra paths are listed;
  - a non-rebase operation is never checked;
  - `journalOnly` never checks.

**Slice 7 (proof): staging on the test computer d@e** (below), after
`lane/base-join` has landed on `main`.

**Slice 8 (orange, tier 1, later): the owner retry act.**
- An owner product API, not on management's ProductAPI allowlist, re-opens a
  spent pair under a fourth and later attempt with an owner-act stamp. Those
  notices amend `IsStackRebaseNotice`: its `n` bound of 1..3 is the automatic
  bound; an owner-act-stamped notice carries `n` above 3 and the predicate then
  also requires the stamp (slice 8 changes the predicate, with its own tests).
- The claim then records attempt-scoped history: the claim key becomes
  (pair, owner generation).
- Until this lands, a spent pair waits for the next base.

All slices: `scripts/go-test-runtime-shards` for `internal/agentcore`, plus the
repo gates (`check-v1-writer-purity`, `check-decode-roots`,
`check-test-hang-guards`, `selftest-guest-token`), then the Landing Loop.

### Staging proof on d@e (no Node B edits; nothing on the paper or the owner's computers)

`scripts/proof-u4-rebase` is new, modelled on `scripts/proof-pd3-root-desk`. It
has a dry run and writes one JSON artifact with every id.

1. On d@e, set through `/api/computer-settings` as the owner:
   `source.projection`, `texture.root_desk`, `texture.owner_act_wakes` and
   `source.rebase_offer`, all on.
2. In a fresh document, request a small source change A. Approve it and see it
   applied. `/health` shows `projected`, with the operation `opA` and base `B`.
3. Land an unrelated `main` commit, giving `B'`. d@e refreshes, and `/health`
   shows `booted_base = B'` and `stack_base_moved`.
4. After the stay-up interval, check the ledger:
   - attempt 1, `rebase-notice:opA:B':1`, is in `texture:<master>`'s inbox;
   - the root's Cast names it;
   - management's spec names the notice, `opA` and `B'`;
   - the claim exists;
   - `SourceArtifactRef` is at `B'`;
   - the rebase offer is under `carried/rebase-offers/`.
5. Freeze. The bundle's base is `B'`, and its tree equals A applied to `B'`. The
   master document lists it under "Waits on the owner".
6. Approve it and see it applied:
   - the lineage receipt exists with verdict pass;
   - the "apply" owner-act record names its sha256;
   - `/health` shows `projected` on `B'`;
   - a fresh ordinary cast is admitted.
7. Conflict run: apply A2, then land `B''` that edits one file of A2. The offer
   names exactly that path in `ConflictPaths`. The receipt lists it as resolved.
8. Negatives, while d@e is at `stack_base_moved`:
   - switch off;
   - an ordinary document's engineering request;
   - a hand-made Cast that names a wrong or superseded notice id.

   Each is refused with today's or the named text.
9. Measure D4: count root runs between attempt 1 and the final Report.

Crash and re-mint receipts come from the runtime tests in slices 2 and 3. A
deployed crash probe runs only if a no-SSH crash act exists; otherwise it stays
a named residual.

### Class, heresy delta, residuals

- *Class:* red. Protected surfaces:
  - the cast gate and records admission (run acceptance);
  - a waking system record and a claim object (Trace/evidence, store);
  - the root Texture RLM's prompt (the Texture control plane, with no
    canonical-write change);
  - the materializer's rebase content check.
- *Review tiers:* slices 1, 2, 3 and 6 are tier 2 (Devin and DeepSeek); slice 4
  is tier 1; slice 5 is tier 1.
- *This commit:* green.
- *Heresy delta:*
  - discovered: the root's cast ban is prompt-only (`texture_root.txt:7`); the
    apply has no base join (`docs/problems/selfdev-apply-has-no-base-join-2026-10-26.md`,
    fixed on `lane/base-join`); the stale wake-allowlist comments
    (`internal/store/inbox.go:270-277`, `internal/store/management_tasks.go:55-65`).
  - introduced: none.
  - repaired: none yet.
- *Residuals:*
  - a pair spent after its claim, or after 3 attempts, needs slice 8;
  - D4's cost, until measured;
  - the host check lives at `internal/hostbuild/service.go:388-401`. It covers
    only host-built candidates, which is why the base join is its own lane.

### What changed after review 303

1. The base check left U4. The old switch-gated slice 6 is gone; the base join is
   now `lane/base-join`, unconditional. The security claim is restated as the
   provenance gap plus H1.
2. A bounded re-mint was added: attempt ids `:<n>`, a cap of 3, closure rules,
   and only the latest attempt is admissible. It is checked against the restart
   rule (question 3).
3. Management's Cast is bound: its spec must name the notice, operation and base,
   and its objective must name the notice. There is a test for an unrelated
   objective.
4. Rebase content is verified at materialization (and at Freeze), and a lineage
   receipt is written.
5. "One assignment per notice" moved into the store's open transaction as a claim
   object under an object condition, replacing the gate-side scan.
6. The record id and the command id are pinned, with a new-command-id test.
   Rebase offers moved to a path old code never reads.
7. Nits:
   - citations corrected (`:905`, `:212-214`, `:664-666`, `:66-70`, `:702`,
     `internal/hostbuild/service.go:388-401`);
   - both stale allowlist comments are fixed in slice 1;
   - tiers set: slices 1-3 and 6 at tier 2, including the gate exception and the
     new slice 6;
   - option A is reduced to the owner's ruling.

## Build status (2026-10-26, Sonnet worker; paused by FP0 for density work)

Resume cold from here. Lane `lane/u4-rebase`; class red; the whole feature is
behind `source.rebase_offer` (default off).

**How the trigger and `lane/base-join` meet.** The U4 trigger is state-based: it
reads `stack_base_moved` from reconcile (the effective release's stack against the
booted base), never an operation's outcome, so it behaves the same with or without
base-join. With base-join landed, an apply whose base moved settles `failed`; the
stack and `current` do not change, so the state still holds and a new pair
(operation, newer base) gets a new notice. A rebase round whose own candidate is
refused that way leaves its claim in place: that pair is spent, and the newer base
is a new pair. Without base-join, slice 3's content verification still refuses a
rebase candidate whose `SourcePatchBaseCommit` is not the claim's booted base
(and, at apply, not the booted base now), so a rebase operation never applies on a
stale base even before base-join lands. Landing order stays base-join first, then
the U4 staging proof.

**Built and pushed (slice 1, `947e0a288`):** notice type and predicate, claim kind
(registered in `objectgraph`), `source.rebase_offer`, the wake predicate, the timer
and bounded re-mint (`internal/agentcore/rebase_notice.go`), settings hook.

**Slice 2 (this push):** store admission and the claim in the open's transaction
(`requireRebaseNoticeAdmission`, `OpenEngineeringAssignmentRequest.Rebase`); the
runtime gate `sourceProjectionCastGateFor` (the old `sourceProjectionCastGate(ctx)`
is a wrapper, a deliberate deviation from the doc's "change every caller");
`PreflightSourceCommit` on the capsule executor (linux test written, not run on
darwin: CI runs it); `rebaseLineage` derived from the tape; the rebase offer under
`carried/rebase-offers/`; the root and management prompt text (slice 5 of the doc).

**Not built, next (slice 3):** `verifyRebaseContent` (`internal/agentcore/
rebase_content.go`) at materialization and Freeze, the `rebase_content_mismatch`
refusal, the lineage receipt and the `source_projection_rebased` fact, the `--cached
--3way` per-path helper in `sourceproj`, and the base-equals-claim check; then the
doc's "defer while a non-terminal rebase of the same operation exists" check in the
timer (needs `ListRebaseNoticeClaims`, already in the store), then slice 7 proof
(`scripts/proof-u4-rebase`). Known residual: a replayed rebase open after a restart
has a different `RebaseAdmission.BootedAt`, so its command digest differs and the
replay fails closed.
