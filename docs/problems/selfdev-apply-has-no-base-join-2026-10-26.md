# A frontend-only self-development apply never checks its base (2026-10-26)

Status: fixed and proved on staging. Landed in train 17 (1ff7a6eb5), deployed with 670f9ea50/d385633b6; staging proof on d@e 2026-10-26 18:01 (below). Found by design review 303 (Devin and
DeepSeek) of the U4 stack rebase design. Class of the fix: red (apply path,
security provenance), review tier 2.

## Problem

An owner-approved candidate that changes no Go input is not host-built
(`hostBuildRequired = consensusDecided || len(binaryInputs) > 0`,
`internal/agentcore/self_development_materializer.go:354-360`), so the host's
`baseRefusal` / `ReasonBaseCommitMismatch` (`internal/hostbuild/service.go:383-401`)
never runs for it. The guest updater does have a base check,
`checkStateCompatibility` comparing `manifest.BaseCommit` with
`bootedBaseCommit()` (`internal/updater/updater.go:466-477`), but the plain
manifest never sets `BaseCommit` (`self_development_materializer.go:433-438`;
only the layered host-built manifest does,
`self_development_host_build.go:364`), so `omitempty` skips it. The apply then
installs `bundle.RuntimeFiles` verbatim. The base the candidate was built on is
already recorded and unused: `bundle.SourcePatchBaseCommit`
(`tools_capsule.go:330`, `source_projection.go:355`).

## Severity (the two seats differ; the fix is the same)

- Devin: live; an old-base frontend bundle installed over a newer base reverts
  any frontend fix the newer base carried.
- DeepSeek: narrower today. At `stack_base_moved` the computer already serves
  the old-base frontend (`current/frontend`), and an open operation blocks a
  newer Freeze, so the stale candidate re-applies the same old frontend. The
  revert of a live fix needs a newer-base release already current while an
  old-base candidate is frozen (e.g. a platform follow on a computer with source
  projection off). The underlying live defect is H1 (computers on a
  self-development stack do not receive the platform's frontend fixes).

Security comes first (owner, 2026-10-25 and 2026-10-26), so the join is made
unconditional either way.

## Fix shape

1. The plain release carries `BaseCommit = bundle.SourcePatchBaseCommit`; the
   updater's existing check refuses a candidate whose base is not the booted
   base, as `base_moved_since_freeze`, naming both commits.
2. Guards: an empty `bootedBaseCommit()` (dev builds,
   `source_projection.go:60-79`) and a bundle with no source patch are defined
   and tested, never a silent pass or a refuse-everything; compare with
   `EqualFold`.
3. A crash-boot settlement of an operation already applied (journal terminal
   `applied`) is never refused by the new check.
4. Not gated on `source.rebase_offer`. Until U4 lands, a refused candidate is
   rebuilt by a new round; with U4, the rebase round handles it.

## Found while building the fix: a refused apply wedges the operation in `materializing` (2026-10-26)

Evidence: with the join in place, the new tests
`TestSelfDevPlainReleaseRefusedWhenTheBaseMovedSinceFreeze` and
`TestSelfDevPlainReleaseShortBaseCommitIsRefused` (agentcore) saw the updater
refuse the apply pre-mutation with `base_moved_since_freeze`, and the
operation then stay `materializing` for the whole 90 s wait, with the step
error `updater: base_moved_since_freeze: release base commit ... does not
match booted base commit ...`.

Cause (read from the code, `materializeSelfDevelopmentOperation`): after the
apply the code reads the updater journal, and for any terminal outcome other
than `applied` it replaces the apply error with `errors.New(journaled.Failure)`.
A pre-mutation refusal journals the terminal outcome `refused`, so the typed
`updater.ErrApplyRefused` is lost; the later `errors.Is(applyErr,
updater.ErrApplyRefused)` is false, the function returns the error, and the
operation stays `materializing`. Each retry replays the same journaled refusal.
`SelfDevelopmentApplyMaterializing` is true while it does, and the actor
handler holds work for it (`self_development_materializer.go`, "desks held").

Reach: every pre-mutation refusal of a live or crash-settled apply (state
compat, base digest join, source trust, manifest verify), not only the new
join. Until now these were rare for plain releases, which declared nothing to
refuse on; the join makes a refusal an expected outcome, so it must settle.

Fix (same commit series, second; settled `failed`, not `degraded`, by the Review 311 section below): a journaled terminal `refused` outcome keeps
the typed error, so the existing branch settles the operation `degraded` with
the refusal as its terminal error. The rollback path has the same override and
is left as it is (residual, named): it is not refused by the join.

## Fix as built (2026-10-26)

- `plainReleaseBaseCommit` (materializer) sets the plain manifest's `BaseCommit`
  to the bundle's `SourcePatchBaseCommit` (lower-cased). It claims none, and
  logs, when the bundle records no source patch base, or when the booted base
  names no verifiable 40/64-hex commit (dev `build_commit=local`, a dirty tree,
  or no image manifest wired); refusing there would refuse every candidate on
  that build. A short or malformed base is claimed as is, so it is refused.
- The updater's `checkStateCompatibility` compares with `EqualFold`, never a
  prefix, and refuses with `base_moved_since_freeze` naming both commits.
  Not gated on any switch. Only the plain manifest is written in one place
  (`materializeSelfDevelopmentOperation`); the layered manifest keeps the
  host's recorded base and the platform-update manifest its offer's.
- A journal-terminal `applied` operation never reaches the check: `Apply`
  returns the recorded outcome first and a crash-boot settlement reads the
  journal only (b@c's selfdev-83fbdfa9 shape; tested).
- Rollback: `rollbackApplyManifest` drops a plain pinned release's base claim,
  so a rollback is never refused by the join; a layered release keeps its base.
- Release-time consequence: a plain release minted by this build carries a
  `base_commit` in its content digest; releases minted before it are unchanged,
  and an operation mid-apply across the upgrade is not restarted by a deploy
  (CLAUDE.md: never restarted mid self-development apply).
- Rollback of the fix: revert the commit; no data migration. A plain release
  that already carries `base_commit` stays valid (the field is read by the
  updater only at apply time).

## Review 311 (2026-10-26)

Devin and DeepSeek v4.1 flash reviewed the built fix (bbf755e93). Both approve
the base join itself. FP0's ruling on the refusal settlement and the findings:

- **Ruling: `degraded` means "applied, with an unsettled tail".** It stays an
  open state (`selfdev.OpenStates`) and is repaired forward on a planned boot
  (X1's use, `lane/apply-replay-bound`). A typed apply refusal installs
  nothing, so it settles `materializing -> failed`, a settled state, with the
  refusal reason and both commits (release base, booted base) on the
  operation. A new self-development round can then open; U4 will later turn
  that into a rebase round. The earlier "settles degraded" fix was wrong:
  `degraded` is open, so `Store.Start` refused every new operation
  (`ErrOperationOpen`) and engineering admission stayed held. The "Fix" and
  "Fix as built" text above that says degraded is superseded by this section.
- **DeepSeek F1 (High): fixed** as above (refused -> failed, a following
  `Store.Start` succeeds; the crash-boot settlement of a journal-terminal
  refused op settles failed without calling `Apply` again).
- **DeepSeek F3 (medium-low): fixed.** `plainReleaseBaseCommit` distinguishes
  "no manifest path wired, or a dev build (`local`, dirty)" (no claim, apply)
  from "a path is wired but the manifest is unreadable or carries no valid
  build commit" (fail closed: the apply is refused with a typed reason and
  settles failed). Same item as Devin F6 and F7 (the host-built log line).
- **DeepSeek F2 (medium): by design, now visible.** A rollback to a plain
  release drops the base claim (rollback must stay possible). The operation's
  terminal result now records that the claim was dropped. Rollback behavior is
  unchanged.
- **Devin F1 (medium): fixed.** The rollback path's journal re-read replaced a
  journaled `refused` outcome with a plain error, so a refused rollback never
  matched `ErrApplyRefused`, stayed `rollback_pending` and re-armed a 60 s
  retry wake forever (and held every deploy through `plannedRestartBusy`). It
  now keeps the typed refusal, like the apply path.

Residuals, named, no code in this lane:

- **R1 (DeepSeek F4, Devin F5): `RestagePinnedRelease` has no base join** (a
  tape restore, `rematerialize.go`, can put an old-base frontend over a newer
  booted base). Recovery path, not a candidate apply.
- **R2 (DeepSeek F5): mid-apply journal conflict across the fix.** `base_commit`
  enters the content digest, so a non-terminal pre-fix journal replayed by a
  post-fix materializer yields `ErrIdempotencyConflict` and `refuse()`
  overwrites the prepared journal, losing `PriorReleaseTarget`. Unreachable
  while a deploy never restarts mid-apply (crash settlement is journal-only).
- **R3 (Devin F2): platform-update crash settlement** leaves a journaled
  `refused` update pending (`settlePendingPlatformUpdateAfterCrash` default
  branch, `platform_update.go`); the live path discharges it. A second
  instance of the same class, to be settled like the live path.
- **R4 (Devin F3): a degraded-refused reconcile spends durable apply attempts**
  (`spendGuestApplyAttempt`) before `Apply` returns the recorded refusal. With
  refused now settling `failed`, this no longer applies to refusals; the
  attempt-bound behavior for genuinely degraded ops belongs to
  `lane/apply-replay-bound`.
- **R5 (Devin F4): a bundle with no source patch claims no base** and installs
  unjoined. Recording the freeze-time booted base in every bundle is open.
- **R6 (Devin F8): fixed with the ruling.** A refusal now settles through
  `recordMaterializationFailed`, so a `materialization_failed` event
  discharges the head's pending transition (without it a `failed` operation
  would leave the transition pending and block the next decision).
- **R7 (discovered building the rollback tests, 2026-10-26): a rollback
  operation cannot record its own success.** `verificationEvidence` requires
  the recorded verification payload to name the operation being materialized
  (`payload.OperationID != operation.OperationID`), but a rollback operation
  carries its TARGET's verifier ref (`Store.StartRollback`), whose payload names
  the target's operation. Driven end to end in the derivable fixture, the
  rollback applies in the updater and then sticks in `rollback_pending` with
  `materializer: verification payload refused`. No existing test completes a
  rollback materialization. Hypothesis until read on a live trace; not changed
  here (red, verification join). The rollback tests in this lane rewrite the
  payload's operation id in a test shim to reach the base-claim behavior.

## Staging proof (2026-10-26 18:01 UTC, FP0)

d@e (computer-59c902c5…, a test computer) booted d385633b6 and held an older
candidate, selfdev-974729048206f364a38bf22908e186b8 (frontend only: App.svelte,
app.css, desktop-motion.css, theme.ts), frozen on base 8efa23c7e. Approved with
d@e's own key at 18:01:01. The operation settled `failed` at 18:01:12 (11 s) with
`updater refused apply: updater: base_moved_since_freeze: release base commit
8efa23c7e9f86dea8b0b2f397ebc42c612cf3dc3 does not match booted base commit
d385633b65b0c34cd790ec11ff0f7a67e74a706b`. Nothing was installed: the
operation is `failed`, not `degraded`, as the review 311 ruling requires; the
refusal is typed and names both commits. Not read in this proof: whether a new
round opened (no new operation was listed within a minute).
