# An orphan frozen candidate holds every deploy refresh (2026-11-09)

Class: orange/red (self-development lifecycle). Found on d@e
(computer-59c902c5d46caecc0a180a8557164e72, staging).

## Evidence

- Trajectory cfaa8ee1-d7fd-5544-8dca-db2a2876f225 (the FRED report) settled at
  10:30:58Z (`trajectory_settled`); its last engineering assignment 70dc19c9
  completed 10:29:13.
- Its self-development operation selfdev-5b5db94e5e6979245b76774c1d28522b
  (capsule-383fa61f-..., created 10:08:05, updated 10:09:12) is still `frozen`
  with `last_intent_error` "verify tray-1: legacy screens bundle; shots are
  taken after the host build". Its three siblings on the same capsule closed
  `failed` by independent judgment.
- vmctl defers every refresh of d@e: `outcome=deferred
  reason=selfdev_awaiting_decision` (the correct busy refusal,
  `internal/agentcore/planned_restart.go:769`), until the candidate is decided,
  2 h after its last update, or 6 h after its creation. d@e has run an old build
  since 08:34 while three expedited releases shipped.

## Cause

1. A permanent verify refusal leaves the operation open.
   `verifyShotRefusal` (`internal/agentcore/selfdev_shot_review.go:156`) refuses
   a platform bundle that carries screens and no shot plan, and a plan with too
   many views. Both depend only on the frozen bundle, so no retry can pass.
   `commitVerifyIntent` (`internal/agentcore/rlm_reduce.go:1886-1893`) turns the
   refusal into a stamped intent error and an error return, with no verdict. The
   operation stays `frozen`, nothing fixes it, and the engineer is never given
   the rejection a fail verdict would carry.
2. Settlement does not look at self-development. The settlement reducer
   (`internal/store/lifecycle.go:5286` `SettleLifecycleTrajectory`, triggered
   from `ReconcileLifecycleSettlementForTerminalRun`, `internal/store/store.go:345`)
   checks work items, records and updates, never the trajectory's open
   operations. The store is below `selfdev`, so the close belongs to the
   runtime. The close paths that exist (`failSelfdevOperation`,
   `internal/agentcore/engineering_assignment_fate.go:523`, used at assignment
   cancel and crash boot; the owner abandon act,
   `internal/agentcore/selfdev_settle_open.go:500`) fire on an assignment
   ending or the owner acting, never on the trajectory settling. The crash-boot
   sweep `closeOrphanedFrozenCandidatesAtCrashBoot` skips completed assignments.

## Belief

High for both causes (read from code and the d@e operation rows). Unverified:
no trace was read of why management settled the trajectory without retrying
the Verify; the settle itself is legitimate once all work is terminal.

## Fix shape (smallest substrate fix)

- A trajectory that is settled or cancelled closes its `frozen` and `verified`
  operations through the existing `failSelfdevOperation`, at run end (the
  persistence point where settlement lands). `awaiting_approval` is left to
  the owner's decision. This closes work; it never resumes any.
- A permanent verify refusal records a `fail` verdict instead of stamping an
  error, so the operation gets a terminal fate and the engineer the rejection.
- d@e's existing orphan needs the owner abandon act (or the 2 h timeout); this
  fix does not reach rows already orphaned.
