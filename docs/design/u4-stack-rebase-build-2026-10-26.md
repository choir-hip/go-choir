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
2. *Base moves again*: the candidate is pinned to `B'`; add a guest check that the
   materializer refuses a candidate whose `SourcePatchBaseCommit` is not the booted
   base (none exists today). Test: apply of a bundle based on a non-booted commit is
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

Owner rulings this section follows: "we need to rebase not reject security
updates. Security comes first." And on the route: "it shouldn't be a
platform-minted assignment. Use the root texture rlm." Terminology (owner): the
**Texture desk** is all of a computer's Texture RLMs, one desk per computer; the
master document's own RLM, `texture:<masterDoc>`, is **the root Texture RLM**.
Read against `1a7cac668` (code at `618727c97`, unchanged since).

**Option A** (a platform-minted assignment) is ruled out by the owner. **B** stays
available as an owner act; it is not automatic. **C** (host-side rebase) is still
the later half of U4 and does not answer the automatic guest round.

### The route in one paragraph

When reconcile answers `stack_base_moved` with the switch on, and the computer
has stayed up a short interval, the runtime mints one **rebase notice**: a system
Note into the root Texture RLM's inbox, with the owner-act record's shape. The
notice wakes the root Texture RLM through the ordinary records wake. The root
opens a round with management: an ordinary Texture Cast whose packet names the
notice. Management casts an ordinary `implementation` assignment with the usual
`{"parent": <the root's Cast>}`. `planEngineeringCast` finds the notice through
that parent and admits the cast while the computer is at `stack_base_moved`, but
only for the current stack top and booted base. It pins the capsule to `B'`
instead of HEAD. The carried offer mounts the stack top's patch `P`. Freeze,
verify, the owner's decision and the reports all run on the existing path.
There is no new cast authority, no new assignment kind, no binding field, no
bundle field and no store schema change. `rebase_of` is never typed by a model.
It is derived from the tape: assignment, then management's Cast, then its
`ParentID` (the root's Cast), then the packet's `rebase_notice`, then the notice
record, which names the operation and `B'`.

### The six questions, answered from the code

1. **Can the root Texture RLM open a round on an event that is not owner text?**
   Yes. Three pieces of code already allow it:
   - A Texture records turn needs no owner text and no open work item
     (`textureowner/texture_agent_revision.go:663-670`, `request_source: inbox_record`).
   - System records already wake Texture RLMs: support failures and owner acts
     (`store/management_tasks.go:66-100`).
   - A Texture's Cast to `management` is an ordinary records route
     (`agentcore/inbox_records.go:57-69`).

   The master document's trajectory is the one parent that never settles. Its
   standing work item is clamped open (`textureowner/texture_turn_runtime.go:207-213`),
   so TX-01 cannot bite. A dead master is replaced at the next digest
   (`texture_root.go:427-473`).

   The records admission is satisfied unchanged
   (`store/engineering_assignments.go:742-800`):
   - the parent is a Texture's Cast (`texture:<masterDoc>`) into
     `management:<owner>`;
   - its round is open;
   - the master document's `TrajectoryID` is the binding's trajectory, and that
     trajectory is live.

   The only thing that stops the root from casting today is a prompt sentence:
   "You never cast to research or management" (`textureprompts/texture_root.txt:7`).
   No code check backs it; the root's only code clamps are the work disposition
   and the 48 KiB bound. Discovered, not introduced: the ban is prompt-only, so the
   root could already cast ordinary engineering from owner text on the master
   document. Route D narrows the prompt exception to a rebase notice. The code
   check that matters is the gate exception below.
2. **What delivers to it.** A records wake, not a digest. The notice is committed
   with `CommitInboxRecords`. That commit writes its own `inbox_record` wake row,
   and the actor-wake outbox delivers it (`runtime.go:982-987`: "the actor-wake
   outbox projector is the sole post-restart delivery path"). The notice is
   `types.CommitmentEmitterSystem`, so it must join the closed allowlist in
   `store.RecordWakesReader` (`management_tasks.go:66-100`). The precedent is the
   owner-act record (`types/commitment_owner_act.go`, `agentcore/selfdev_owner_act_wake.go:15-29`):
   a system Note from a non-desk sender, which opens and closes no round. A
   digest is the wrong carrier for three reasons:
   - it is coalesced and debounced, and holds documents, not facts;
   - it lives in memory and starts a full ledger refresh;
   - it carries no record id for the gate to check against.

   The root's records turns always start a fresh run, never a parked one
   (`texture_controller.go:683-692`), so the run reads the ledger and the notice
   together.
3. **The restart rules hold.**
   - *Never at boot.* Boot reconcile runs after the crash closures
     (`runtime.go:936-942`, then `:963`). Even so, the notice is never minted
     inside boot. Reconcile only arms an in-memory timer, due at `bootedAt` plus
     a stay-up interval (default 5 min). A crash forgets the timer, and the next
     boot recomputes the state from the tape.
   - *Crash before the root reads the notice.* The pending wake is a pre-boot
     `inbox_record`, so the generic gate interrupts it (`planned_restart.go:163-207`).
     The unread notice stays on the tape, but the gate exception refuses a notice
     committed before `rt.bootedAt` on a markerless boot, with the fate
     "interrupted by a restart". The same `createdAt.Before(rt.bootedAt)` test is
     in `planned_restart.go:207`.
   - *Crash during the round.* The crash boot closes the round and the assignment
     as today (`closeOpenRoundsAfterCrash`, `closePreBootRunsAfterCrash`). The
     notice exists, so the pair is decided and nothing is re-minted. One crash
     spends the pair's automatic round, as the design's failure mode 4 requires.
   - *Why minting after a crash boot is allowed.* It is new work, not resumed
     work, by the schedule precedent in CLAUDE.md and
     `recoverSourceSchedulesAtBoot` (`source_schedule.go:323-333`): "standing
     owner intents survive a crash; their occurrences do not", and the schedule's
     next tick is "armed one interval after boot, never at once: new work on a
     computer that has stayed up that long".
     - The standing intent here is the owner's switch `source.rebase_offer`
       ("keep my stack on the booted base").
     - The occurrence is one round per (stack operation, booted base).
     - It reopens nothing that was open before the boot, and it fires only after
       the computer has stayed up.
     - It is stricter than the schedule's two-crash rule: a pair whose round
       crashes the computer is never tried again automatically.
     - No boot-time resumer is added.
     - The comment above `inboxRecordWakes` (`store/inbox.go:270-277`) lists the
       waking system records "minted only by live work, never at a crash boot".
       The notice joins that list in those words, because it is minted by a live
       timer after the stay-up interval.
   - *Planned restart.* A planned (marker) restart parks and resumes a live round
     as the existing rules do. The notice check passes, because a planned boot
     keeps pre-boot work. If the planned restart also moved the base to `B''`, the
     resumed round is stale; the materializer check refuses it at apply (below).
4. **How "one automatic round per pair" is remembered.** On the tape, not in a
   guest file, so there is one state authority (standing question 4).
   - The notice's record id is `rebase-notice:<operation id>:<B' full sha>`.
     Its existence means the pair is decided.
   - `CommitInboxRecords` with that command id replays and never mints twice.
   - The gate admits at most one assignment per notice: it scans the master
     trajectory's assignments and derives each one's notice.
   - The timer does not mint while any non-terminal rebase of the same operation
     exists on another base. It re-arms instead, so the rebase onto `B''` waits
     for the stale round onto `B'` to end.
   - A product restore that rewinds the tape past the notice makes the pair
     undecided again. That is correct, because the restore is the owner's act.
5. **The carried offer can come from the stack top without ordinary assignments
   ever carrying an applied operation.**
   - The new branch is chosen only when `rebaseOfAssignment` derives a notice
     from the tape chain. That needs a system notice, a parent sent by the
     current root Texture RLM, and the notice's own record id in that parent's
     packet.
   - The source is `rt.boundSourcePatch(top)`: `P` and its base `B`, both already
     read by `sourceProjectionInput`.
   - The ordinary path (`selectCarriedCandidate`, `roundCarriable` at
     `carried_patch.go:221`) is not touched, so an applied operation is still
     never an ordinary candidate.
   - Unlike an ordinary carry, a rebase offer mounts `P` even when it conflicts
     with `B'`, and names `ConflictPaths` and `CleanPaths`. The engineer redoes
     only the conflicting paths (design step 4).
   - The offer file gains `RebaseOf` and `RebaseNotice` with omitempty. The decode
     is a plain `json.Unmarshal` (`carried_patch.go:690`), so older code reads the
     file.
6. **The materializer's base check.**
   - Where: in `materializeSelfDevelopmentOperation`, after the bundle validates
     and before the host-build decision (`self_development_materializer.go:331-354`).
   - When: never when `journalOnly` (a crash-boot settlement), and only with the
     switch on. Under those conditions it refuses a bundle whose
     `SourcePatchBaseCommit` is set and is not `rt.bootedBaseCommit()`. The
     refusal goes through `recordMaterializationFailed` with reason
     `base_moved_since_freeze`; the computer keeps its release.
   - Why the host check is not enough: the host already refuses Go candidates
     (`hostbuild/service.go:387-399`, `base_commit_mismatch`). The guest check
     covers frontend-only candidates, which never reach the host, and names the
     reason earlier.
   - After a refusal: the stack top is unchanged, so reconcile answers
     `stack_base_moved` for (`P`'s operation, `B''`). That is a new pair, and the
     next notice follows.
   - Lineage needs no bundle field. The materializer decodes with
     `DisallowUnknownFields` (`:331`), so a lineage field would make the bundle
     undecodable after a platform rollback. The lineage is read from the tape:
     operation, then `freeze-open:<assignment>` (`store/engineering_assignments.go:904`),
     then `rebaseOfAssignment`.

### D against A (one line, as asked)

A was ruled out by the owner. For the record, the comparison is lopsided:
- *Size of new authority:* D adds a waking system record kind and one gate
  exception, and both are refusal-shaped. A adds a cast authority, a store open
  variant and a report path.
- *Protected surfaces:* D touches the cast gate, the root Texture RLM's prompt, a
  wake allowlist and the materializer refusal. A also touches the store schema,
  Validate, and about a dozen kind switches.
- *New failure modes:* D's are model steps that fail visibly in the ledger
  (D1-D3 below). A's are silent ones in the store.

### Failure modes added by route D (written before code; the original 13 stand)

- **D1. The root Texture RLM does not cast, or copies the notice id wrong.** The
  gate refuses with the code `rebase_notice_mismatch`, management reports it, and
  the master document shows it. The pair is spent; a retry is an owner act (slice
  8). *Test:* a root Cast that names an unknown id, or another pair's id, is
  refused.
- **D2. Management does not cast, or casts without the root's parent.** The
  ordinary gate refuses `stack_base_moved` as today. *Test:* an ordinary parent at
  `stack_base_moved` is still refused.
- **D3. An owner request on the master document, cast by the root without a
  notice.** It goes down today's path and is refused at `stack_base_moved`.
  Nothing is misread as a rebase. *Test:* a root Cast with no `rebase_notice` is
  refused.
- **D4. Progress relays and owner acts wake the root on every report, and each
  root run reads a full ledger refresh.** This cost bypasses the digest's
  debounce. *Measured* in the staging proof (root runs during one rebase round).
  If the count is more than about 12, a follow-up gives root records turns the
  documents their records name instead of a full refresh.
- **D5. The master document is replaced between the notice and the Cast.** The
  notice's inbox is no longer the current root's, so the gate refuses
  (`rebase_notice_mismatch`) and the pair is spent. This is a residual; it is rare
  because replacement needs a settle that the clamp missed.
- **D6. A notice from before a crash is read after it.** It is refused,
  "interrupted by a restart" (question 3).
- **D7. A forged notice.** Agents' records are always `EmitterAgent` with their own
  sender (`inboxRecordForIntent`, `inbox_records.go:394-405`).
  `IsStackRebaseNotice` requires all of these:
  - `EmitterSystem`;
  - sender `selfdev-rebase:<op>`;
  - a record id equal to `StackRebaseNoticeID(op, base)`;
  - subject refs naming both.

  *Test:* each mismatch fails the predicate.
- **D8. The root Texture RLM is off, or the marker is missing.** Nothing is
  minted, the pair stays undecided, and the timer re-arms each interval while
  the state holds. *Test:* with `texture.root_desk` off, no notice is minted and
  the cast refusal is unchanged.

### The switch

`source.rebase_offer`, a computer setting in `store.KnownComputerSettings`,
default false. It is effective only when `source.projection` and
`texture.root_desk` are also on, and the runtime reads all three together
(`rt.rebaseOfferOn`). The proof also turns on `texture.owner_act_wakes`, so the
owner's decision reaches the master document. With the switch off:
- no timer runs and no notice is minted;
- the gate exception never admits, so today's refusal text is unchanged;
- the materializer check is skipped;
- the root prompt's exception is inert, because no notice exists to act on.

### The rollback

- *One act:* turn `source.rebase_offer` off. It takes effect at the next timer
  fire, gate read or apply. A round already admitted runs as an ordinary
  implementation round; its candidate still needs the owner's approval, and the
  owner can cancel the assignment.
- *Full:* revert the slice commits. Notices already on the tape are inert to old
  code: `inboxRecordKind` shows them as `closure`, and `RecordWakesReader` returns
  false. No schema, bundle or binding changed, so a platform rollback decodes
  everything.

### Slices, in order (each red unless marked; tests first in every slice)

**Slice 1 (red, tier 2): the notice record and the switch.**
- *Files:*
  - `internal/types/commitment_rebase_notice.go` (new): `StackRebaseNoticeID(op, base)`,
    `StackRebaseSenderInbox(op)` = `selfdev-rebase:<op>`, `NewStackRebaseNotice(...)`
    and `IsStackRebaseNotice(rec)`. The record is a system Note with subject refs
    `operation_id`, `booted_base`, `stack_base` and `doc_id`, and its body states
    the exact call the root makes, with the record id in it.
  - `internal/store/management_tasks.go`: `RecordWakesReader` returns true for a
    notice.
  - `internal/store/inbox.go:270-277`: the comment lists the notice.
  - `internal/agentcore/inbox_records.go` `inboxRecordKind`: `rebase_notice`,
    placed before the `closure` case.
  - `internal/store/computer_setting.go`: `SettingSourceRebaseOffer = "source.rebase_offer"`,
    false, with its doc comment.
- *Failure modes:* a notice that does not wake; any other system record starting
  to wake; a record id that is not deterministic; the predicate accepting an agent
  record.
- *Tests:*
  - the D7 predicate table;
  - `RecordWakesReader` is true for a notice and false for a closure;
  - a store commit of the same notice twice replays;
  - an unknown key is still refused.

**Slice 2 (red, tier 2): the trigger.**
- *Files:* `internal/agentcore/rebase_notice.go` (new) and
  `internal/agentcore/source_projection.go`:
  - after `ReconcileSourceProjection` stores its result, call
    `rt.armRebaseNotice(result)`;
  - the timer is a mutex-guarded singleton on `Runtime`, and its stay-up interval
    is a field that tests override.
- *At fire:*
  - check the switches;
  - reconcile again with reason `rebase`;
  - require `StateBaseMoved` with `sourceproj.HasCommit(repo, booted)` (not the
    objects-missing branch, failure mode 10);
  - require a non-genesis top with a readable patch;
  - read the live root through the store: `GetTextureRoot`, the document, its
    trajectory live and the standing item open (the `textureRootLive` logic);
  - stop if the notice id already exists (decided);
  - re-arm while a non-terminal rebase of the same operation exists;
  - then commit the notice.
- *Failure modes:* a mint inside boot; a mint after a crash before the computer
  has stayed up; a second mint per pair; a mint while the root is off; a mint at
  `base_objects_missing`; a timer storm.
- *Tests:*
  - due time is no earlier than `bootedAt + interval`;
  - two reconciles and a simulated reboot mint once (failure mode 3);
  - a crash-boot runtime with the notice already present mints nothing (failure
    mode 4);
  - each switch off mints nothing (failure mode 7, D8);
  - objects missing mints nothing (failure mode 10);
  - a live rebase of the same operation defers the mint.

**Slice 3 (red, tier 2): the gate exception and the `B'` preflight.**
- *Files:*
  - `internal/agentcore/source_projection.go`:
    `sourceProjectionCastGate(ctx, parent *types.CommitmentRecord) (sourceRef string, err error)`.
    `""` means HEAD as today. Update every caller and test.
  - `internal/agentcore/rebase_notice.go`: `rebaseForParent(ctx, parent, result)`
    and `rebaseOfAssignment(ctx, assignment)`.
  - `internal/agentcore/engineering_assignment_runtime.go:258-261`: pass the
    parent, and preflight `sourceRef` when it is set.
  - `internal/capsule/executor.go`: `PreflightSourceCommit(ctx, commit)` gives
    `commitDigest` and `capsule-source-git:<commit>:sha256:<digest>`, with a stub
    in `stub_other.go`.
- *The exception admits only when all of these hold:*
  - `result.State == stack_base_moved` and the switch is on;
  - the parent's sender is the current root's inbox;
  - its packet's `rebase_notice` loads a notice that passes `IsStackRebaseNotice`;
  - the notice's inbox is that sender;
  - the notice's operation is `result.OperationID` and its base is
    `result.BootedBase`;
  - the notice is not before `bootedAt` on a markerless boot;
  - no other assignment derives this notice (replay excepted);
  - no non-terminal rebase of the operation exists.

  Anything else refuses with the existing `stack_base_moved:` text plus a named
  sub-reason.
- *Failure modes:* failure modes 6 and 9, D1-D3, D5, D6.
- *Tests:* each condition is false in turn, and each is refused. The admitted
  binding's `SourceArtifactRef` commit equals `B'`, and the capsule's
  `emitSourcePatch` base is `B'`.

**Slice 4 (red, tier 1): the offer from the stack top, and its notice.**
- *Files:* `internal/agentcore/carried_patch.go`:
  - `offerCarriedPatchForAssignment` branches on `rebaseOfAssignment`;
  - a new `offerRebasePatch` reads `P` through `boundSourcePatch`, checks digest,
    size (`CarriedPatchMaxBytes`) and paths, and runs `CheckCarriedPatchApplies(P, B' ref)`;
  - the offer is recorded once with `RebaseOf`, `RebaseNotice`, `ConflictPaths`
    and `CleanPaths`, and is mounted even on a conflict;
  - `carriedPatchNotice` gets the rebase wording: what `P` is, the bases, and
    `git apply --3way /selfdev/carried/source.patch`, then resolve only the named
    paths, rebuild, test and Freeze.
- *Failure modes:* failure modes 1, 11 and 12; an ordinary assignment carrying
  an applied operation; a patch over the bound; a root mismatch between the
  bundle patch and `/workspace/platform` paths.
- *Tests:*
  - a fixture `B'` already holding one of `P`'s two hunks names that path as a
    conflict (failure mode 1);
  - an ordinary assignment on a trajectory whose round is applied gets no offer;
  - the rebased Freeze's `SourcePatchBaseCommit == B'`, and `Cumulative` leaves
    the patch unchanged when the pin is not a projection commit
    (`sourceproj/compose.go:46-49`, failure mode 11);
  - an old bundle's digest is unchanged (failure mode 12, trivially: no field).

**Slice 5 (yellow, tier 1): the prompts.**
- *Files:*
  - `internal/textureprompts/texture_root.txt`: one exception sentence. "A
    `rebase_notice` record asks you to open one round: make the Cast its body
    gives, exactly once, and list it under 'Waits on the owner'."
  - `internal/runtimeprompts/overlays/rlm_management_runtime.yaml`: "a Cast from
    the root Texture RLM that names a rebase_notice is a rebase request: cast it
    to engineering with {\"parent\": r.RecordID} and its objective unchanged."
- *Failure modes:* the prompt core grows; the root reads the exception as leave
  to cast for other things.
- *Tests:* `textureprompts` tests contain the sentence;
  `TestPromptCoreRetiresTheOldWriteSurface` is unchanged (the root section is not
  in the core).

**Slice 6 (red, tier 1): the materializer base check.**
- *Files:* `internal/agentcore/self_development_materializer.go` (question 6).
- *Failure modes:* failure mode 2; refusing ordinary projected candidates;
  running on `journalOnly`.
- *Tests:*
  - a bundle on a non-booted base is refused `base_moved_since_freeze`, and the
    next reconcile is a new pair;
  - a bundle on the booted base passes;
  - with the switch off, the old behavior holds;
  - `journalOnly` never checks.

**Slice 7 (proof): staging on the test computer d@e** (below), with the
receipts recorded here and in the R&D doc's U4 section.

**Slice 8 (orange, later, optional): the owner retry act.** An owner product API,
not on management's ProductAPI allowlist, mints `rebase-notice:<op>:<B'>:<n>` for a
spent pair. Until it lands, a spent pair waits for the next base move. This is a
named residual.

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
   applied. `/health` shows `source_projection.state = projected`, with the
   operation `opA` and base `B`.
3. Land an unrelated `main` commit, giving `B'`. d@e refreshes at idle, and
   `/health` shows `booted_base = B'` and `stack_base_moved`.
4. After the stay-up interval, check the ledger (`choir trajectory` on the
   master):
   - the notice `rebase-notice:opA:B'` is in `texture:<master>`'s inbox;
   - the root's Cast names it;
   - management's Cast has `parent` set to the root's Cast;
   - the assignment's `SourceArtifactRef` commit is `B'`;
   - the offer is `offered` with `rebase_of = opA`.
5. Freeze. The candidate's `SourcePatchBaseCommit` is `B'`, and its tree equals
   A applied to `B'` (the script recomputes this with `git apply` in a scratch
   clone). The master document lists it under "Waits on the owner".
6. Approve it and see it applied. `/health` shows `projected` on `B'` with the
   new operation, and a fresh ordinary cast in another document is admitted.
7. Conflict run: apply A2, then land `B''` that edits one file of A2. The offer
   names exactly that path in `ConflictPaths`, and the rest are `CleanPaths`.
8. Negatives, run while d@e is at `stack_base_moved`. The cast refusal text is
   today's when:
   - the switch is off;
   - an ordinary document requests engineering;
   - the root's Cast names a wrong notice id.
9. Measure D4: count root runs between the notice and the final Report.

Crash-path receipts are the runtime tests in slices 2 and 3 (failure modes 3 and
4, D6). A deployed crash probe runs only if a no-SSH crash act exists; otherwise
it stays a named residual.

### Class, heresy delta, residuals

- *Class:* red. Protected surfaces: the cast gate (run acceptance path), a waking
  system record (Trace/evidence), the root Texture RLM's prompt (the Texture
  control plane, with no canonical-write change) and the materializer refusal.
- *Review:* tier 2 (Devin and DeepSeek) for slices 1-3; tier 1 for slices 4-6.
- *This commit:* green.
- *Heresy delta:*
  - discovered: the root Texture RLM's cast ban is prompt-only
    (`texture_root.txt:7`). Also a hypothesis to investigate before slice 6: a
    frontend-only candidate frozen on `B` could apply over `B'` with no base check
    at all, and could revert part of a security fix in `B'`. Not yet traced.
  - introduced: none.
  - repaired: none yet.
- *Residuals:*
  - a spent pair needs an owner act (slice 8);
  - D5 (master replaced mid-flight);
  - D4's cost, until measured;
  - widening the materializer base check from `source.rebase_offer` to every
    `source.projection` computer, after this proof.
