# Crash boots re-drive platform updates and self-development applies

Status: open; L1a-3a (steps 1-2, platform updates) in progress, L1a-3b (step 3, self-development) next (L1a-3 of docs/definitions/choir-ledger-l1-restart-2026-10-13.md)
Found: 2026-10-13, code reading against restart design v3 §6 item 1 and §9
findings 5-7. Mutation class: red (platform update, self-development
materialization, run lifecycle).

## The rule

A crash boot (no planned-restart marker) never resumes or re-drives work;
each open obligation closes with a visible fate (CLAUDE.md, "Restarts End
Work (Crash) Or Resume It (Planned Update)").

## What happens today

Start (`internal/agentcore/runtime.go` ~750-790) gates run closure, round
closure, pre-decision self-development closure and source schedule closure
on the boot kind. Three paths run on every boot regardless:

1. **Platform-update resume** (`platform_update.go:813`). With a pending
   transition it re-pushes the stored offer through `ApplyPlatformUpdate`,
   which calls `updater.Apply` again. With an applied update whose tail is
   stranded it re-drives the checkpoint publish and route projection
   (`resumeStrandedPlatformUpdateTail`). It discharges only an offer pinned to
   a retired realization.
2. **Self-development reconcile** (`self_development_materializer.go:128`),
   both the boot trigger and the post-commit observer. Crash closure fails
   only pre-decision ops (`selfdev.PreDecisionStates`); `awaiting_approval`,
   `accepted`, `materializing`, `rollback_pending` and `degraded` stay open
   and the reconciler re-runs `updater.Apply` or the rollback for them.
3. **`selfdev_materialization_retry` wakes** are exempt from the pre-boot gate
   (`planned_restart.go` RestartWorkKinds comment; `actorruntime/handler.go`
   `preBootWorkOccurrence`), so a pre-boot retry still triggers the
   reconciler after a crash.

Engineering capsule reconcile (`engineering_assignment_fate.go:573`) was on
the design's list but only revokes and cancels; it is closure, not re-drive.

## Why the obvious fix is wrong

Settling every crash-boot transition as "failed" would be false and could
wedge the computer:

- The reducer clears `PendingTransitionRef` on `materialization_failed` only
  with `RestoredPriorEffective` (`computerevent/reducer.go` ~103-109).
  `recordPlatformUpdateFailed` always sets it. After a crash mid-apply that
  is true only when the updater journal shows nothing changed or a verified
  recovery receipt; when the journal is terminal `applied`, the new release
  is live and the honest record is `applied`; when the journal is not
  terminal (`pointer_swapped`, `restart_requested`, `recovering`) the
  effective state is unknown and the updater daemon may still finish it.
- Leaving the transition pending refuses every later update
  (`ErrPlatformUpdateStaleHead`) until an owner act.

So crash settlement must read the updater journal (`ReadJournalOutcome`) and
record what it says, never call `updater.Apply`.

## A second defect found on the way: the marker leaks

`markPlannedRestart` clears the marker only when the apply returns an error.
A resumed apply replays from the updater journal and returns success without
restarting, so the marker stays on disk (up to its 15-minute validity) and
any crash in that window boots as planned. Same pattern in the materializer's
apply and rollback calls.

## Found while fixing: a lost apply response reads as a refusal

`ApplyPlatformUpdate` treats an updater error with an empty `Outcome` as
"refused before any mutation" and records failed with the prior state
restored (`platform_update.go`, the `result.Outcome == ""` branch). A
transport error (socket closed, timeout) after the updater finished also
arrives as an empty result, so a live caller would record a completed apply
as failed. In production the caller dies with the restart, so this path is
rare; the crash settler below reads the journal instead of the response and
does not share the defect. Not fixed in L1a-3a; recorded as a discovered
heresy for L1c, where planned-ness moves to the host.

## Planned work (L1a-3)

1. Clear the marker whenever the apply returns without restarting.
2. A crash settler that reads the updater journal and records applied,
   failed-with-restored-prior, or leaves the transition visibly pending with
   the fate "interrupted by a restart"; it never calls `updater.Apply`.
3. Start, on crash boots: the settler instead of the platform-update resume
   and stranded-tail re-drive; crash closure extended to post-decision
   self-development ops, run before the reconcile trigger and the post-commit
   observer; the reconciler skips ops marked interrupted.
4. No separate gate for `selfdev_materialization_retry`: its handler only
   triggers the reconciler, which finds nothing to drive once step 3 settles
   every post-decision op on a crash boot.
5. Deferred to L1c: whether a guest-self-marked boot counts as planned. Today
   the post-restart re-push is the second half of every apply, so treating
   self-marked boots as crashes would break every update until the host
   attests planned boots.

## Evidence

Code reading on main fdc70329 (file:line references above); no live
incident. The L1a-2 attempt budget (c08ead07) already bounds self-marked
replays at three per apply.


## Merged from docs/problems/crash-restart-leaves-work-open-and-recasts-engineering-2026-10-12.md (2026-10-12)

**A crash restart leaves work open and can resume engineering work.**
Found 2026-10-12 during read-only design research (studies B and G). Status: documented, not fixed. Both halves contradict the standing rule in CLAUDE.md, "Restarts End Work (Crash) Or Resume It (Planned Update)": at a markerless boot every open obligation (run, wake, control, directive, assignment, work item) is closed with the visible fate "interrupted by a restart", never re-dispatched.
- Status: documented, not fixed. Both halves contradict the standing rule in
- decision, and **re-casts the assignment at the next attempt**, up to

(Full text in git history at the path above.)
