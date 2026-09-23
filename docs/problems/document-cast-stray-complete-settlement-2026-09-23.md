# Document-Cast Settlement: Freeze-Before-Validate Strands the Assignment

Date: 2026-09-23 (revised same-day after 9-agent consensus review,
`.agentic-consensus/agentic-consensus-20260923-191530`)
Status: documented before fix, per the problem-documentation-first invariant.
Found by the M2 post-landing probe (second real document-channel cast).
Mutation class of this document: green (analysis; no runtime change).

## Observed

On staging (`computer-03335285269bdba4f94377e56879f9e6`, deployed build
`4de7fdf9`), a second owner-authored revision on the engineering-bound
Texture document opened and bound assignment
`assignment-e77f7940-7075-545b-95a6-1f5a1cd9f0a3` (trajectory
`a5c99d30-a9ed-570c-8cbb-ebdd6485ef1e`, bound 2026-09-23T16:55:08Z).

The assigned run executed the task (wrote and ran
`/workspace/platform/fibdemo/main.go`, verified Fibonacci output) and
finished `state=completed` at 17:23:57Z. Canonical state after the run:

- Trajectory events stop at `co_super_assignment_bound` (seq 3). Zero
  `co_super_capsule_disposition_set`, zero `co_super_assignment_reported`.
- Assignment: `disposition=bound`, `capsule_disposition=active` — wedged
  ~5h observed with no change.
- Run metadata: `lifecycle_terminal_settlement_requested=true`; the
  settlement reconcile exits early because the work item is still open.
- The model's result text claims a stray `Complete` call in an "e" cell
  "compiled and returned a receipt instead of a compile error", was
  "accepted and reduced, ending the assignment", and that subsequent
  cells were rejected with "assignment capsule is not active".

## Root cause (consensus-confirmed)

**The model's account is refuted by canonical state.** Any accepted
`Complete` reduce writes `freeze_requested` to the store *before*
touching the executor (`cosuper_assignment_fate.go:697-704` precedes
`ExtractGranted` at `:723`; the store write emits
`co_super_capsule_disposition_set`). Zero disposition events means no
Complete saga ever committed.

**The actual mechanism is `choir.Freeze`.** `ExtractGranted` — the only
production caller of `Capsule.Quiesce` — has exactly two callers: the
Complete saga (store write first) and `freezeCapsuleEffectBundle`
(`tools_capsule.go:235`), which freezes the executor as its *first*
effect, before validating receipts, before `GetByTrajectory` (line 266),
before the head check. A document trajectory has no selfdev operation,
so the call fails post-freeze — leaving executor `Frozen` (or wedged
`Quiescing`), store `active`, zero events. Every later cell is refused
by `validateAssignedCoSuperExecution` at
`cosuper_assignment_tools_overlay.go:124-126` ("assignment capsule is
not active" — an executor-state check; a store-terminal assignment
would fail earlier at `:101` with a different message — the quoted
error is itself the fingerprint).

**Why the model called Freeze: the prompt commands it.**
`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml:61`
instructs every engineering-desk assignment — including document-channel
casts — "Freeze the capsule diff with choir.Freeze. Finish the
assignment with one choir.Complete call inside a cell." The overlay was
written for the selfdev flow (freeze → verify → complete); document
casts carry no selfdev operation. The model did not hallucinate the
call; it followed the prompt. Likely cell shape: `Freeze` then
`Complete` in one cell — `commit()` aborts on the Freeze error
(`rlm_reduce.go:461-463`), so Complete never runs, producing exactly
zero events plus a dead capsule, which the model then narrated as "my
Complete was rejected."

**Why it wedges:** the Freeze intent path arms no fate watchdog (and
`armAssignedCoSuperFateWatchdog` no-ops without a `PendingProposal`,
which Freeze never writes); `resumeStrandedFrozenAssignmentCommits`
requires a pending proposal; the settlement reconcile exits early on
the open work item. "Permanently" is overstated: restart reconcile
(`cosuper_assignment_fate.go:418-432`, `assignedCoSuperCapsuleUsable`)
and the 6h deadline sweep (`enforceCoSuperAssignmentDeadlines`) both
cancel this strand — but both run only inside the persistent-Super
selection path (`super_controller.go:333`), with no periodic ticker.
On a quiet document-parented computer the wedge lasts until restart or
the next Super selection past 6h.

## Secondary findings (consensus)

1. **Classifier-reject is a success-shaped copy of this strand.**
   `BuildBundleFromDiff` can return `record.Rejected` with `err=nil`
   *after* the executor froze (`tools_capsule.go:259-260`) — the cell
   "succeeds" with a receipt-shaped result, no store write. This fits
   the model's "returned a receipt" wording better than a
   `GetByTrajectory` tool error; the exact failing step is unpinned
   without the cell's reduce error.
2. **`RunCompleted` without terminal assignment is itself a hole.**
   `executeWithToolLoop` marks `RunCompleted` when the tool loop ends
   (`runtime.go:3431`) with no guard that a bound CoSuper assignment
   reached terminal disposition. `completeSuccessfulRunWorkItems`
   returns early for lifecycle-trajectory runs
   (`super_controller.go:961-968`), so the work item stays open by
   design — sealing the strand behind a "completed" run.
3. **`Quiesce` failure wedges `StateQuiescing` permanently.**
   `capsule.go:126-127` returns on cgroup-freeze error without
   restoring `StateActive` (the ctx-cancel path at `:120-122` does).
   Same observed symptoms, same fix class.
4. **Freeze reduce uses plain `ctx`** (`rlm_reduce.go:436`) while
   Complete gets `WithoutCancel` + 3min (`:424`) — cancellation during
   `Cgroup.Freeze` can produce the stuck-`Quiescing` case.
5. **`Thaw` exists with zero callers** (`capsule.go:134`) — any quiesce
   not followed by a successful saga is irreversible without revoke.
6. **Same-class hole on the legit selfdev path:** the bundle freezes
   physically, then mutates selfdev op state later — a crash mid-bundle
   leaves op `Executing` + capsule frozen.
7. **One-live-slot damage:** the strand holds the bound non-terminal
   slot until a backstop fires; the deadline cancel "fails the
   assignment, never the work" — nothing is lost, but admission is
   blocked meanwhile.

## Evidence

- Trajectory `a5c99d30-a9ed-570c-8cbb-ebdd6485ef1e`: events 1–3 only.
- Assignment `assignment-e77f7940-7075-545b-95a6-1f5a1cd9f0a3`:
  `disposition=bound`, `capsule_disposition=active`.
- Run `run:assignment-e77f7940-…`: `state=completed`, result text names
  the stray cell and the consumed settlement;
  `input_tokens=3194958`, `lifecycle_terminal_settlement_requested=true`.
- Contrast: trajectory `ad04eaba-53e9-5c5c-9116-f8c18ae6dce5` (the
  accepted M2 proof, same computer, same build) reached
  `co_super_assignment_reported` + `disposition=completed`.
- Consensus review: 9 panelists (codex, claude-opus, cursor, devin,
  gpt-6-sol, gpt-6-luna, gemini-3.8-flash, cursor-grok-4.6, glm-5.3-flash)
  — unanimous confirmation of the Freeze mechanism and the Complete
  refutation; medium confidence on the exact post-freeze failing step.

## Fix direction (adjudicated)

1. **Reorder `freezeCapsuleEffectBundle`** — move the selfdev authority
   checks (`GetByTrajectory`, `StateExecuting`, head check, authority
   presence; `tools_capsule.go:248-282`) *before* `ExtractGranted`.
   Receipt resolution must stay after — `ResolveGrantedExecutionReceipts`
   requires `StateFrozen` (`executor.go:926-928`). Fix in the shared
   body so both the intent path and any future caller are covered.
2. **Fix the prompt** — document-channel casts must not be served an
   overlay that mandates `choir.Freeze`; the selfdev freeze instruction
   belongs behind a selfdev-operation gate in the prompt assembly.
3. **Gate at the effect boundary** — `commitFreezeIntent` should
   require a resolvable selfdev operation before any executor effect.
   `Tray.Freeze` cannot gate (no operation store); slot-gating cannot
   distinguish document vs selfdev implementation assignments.
4. **Reconcile the split** — run the existing
   `assignedCoSuperCapsuleUsable` → revoke+cancel path in-process
   (Super-selection sweep or a periodic tick), keyed on store-`active`
   XOR executor-non-`Active`, for live *and* terminal runs. Cancel, not
   complete — no report was ever authored.
5. **Watchdog coverage** — arm on every terminal-saga failure inside
   the `freeze_requested` block (`cosuper_assignment_fate.go:718-761`,
   `766-810`), not only the tail commit; those strands carry
   `PendingProposal` so the watchdog can fire. The zero-event Freeze
   strand is (4)'s patient, not the watchdog's.
6. **Same patch class:** restore `StateActive` on `Quiesce` cgroup
   failure; give the Freeze reduce the same `WithoutCancel` treatment
   Complete gets; decide `Thaw`-on-failure vs accept-`Frozen`-for-
   Complete-only-cells.
