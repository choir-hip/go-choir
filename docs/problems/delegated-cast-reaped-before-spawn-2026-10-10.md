# Texture → management → engineering can never reach a capsule: the restart reconciler reaps every delegated assignment in the second it opens (2026-10-10)

## Evidence

All-agents traces of both demo reruns (18:05–18:40Z; test computers
computer-abd38983… "make a Minesweeper game" and computer-4332a5b0… the
arXiv replication). Both computers: Texture's V1 opened management;
management staged `choir.Cast("engineering", …)`; the cast committed. No
engineering run ever started on either computer.

Minesweeper trajectory bedf91a6…: five delegated assignments
(`delegated-sha256:…`, one per committed cast, including the duplicates
from docs/problems/responses-stream-duplicates-tool-calls-2026-10-10.md),
each `co_super_assignment_opened` and `co_super_assignment_cancelled` in the
same second, `disposition_reason: "restart acknowledged absent pre-bind
assignment capsule"`, capsule never bound. The computer never restarted.

## Cause

Opening an assignment commits, atomically, the assignment (Open, Unbound),
its engineering work item (open, v1) and two wakes: the parent's
`delegated_assignment_spawn_deadline` (the deferred spawn saga) and the work
item's `lifecycle_work_assigned`. The work wake runs
`ReconcileLifecycleWorkAssignment`, whose engineering branch calls the
restart reconciler `ReconcileEngineeringAssignmentsForTrajectory`. That
reconciler treats any Open+Unbound assignment as a restart strand and
revokes and cancels it. Its saga mutex protects only the synchronous saga;
the delegated spawn is deferred, so nothing holds the window. The work wake
wins the race every time.

The self-development path survived because the document-channel cast
(`reconcileEngineeringCast`) opens and binds under the saga mutex.

## Fix (orange → red-adjacent: assignment lifecycle)

The runtime records `bootedAt`. The restart reconciler closes an unbound,
pre-bind assignment only when it was opened before this boot. One this boot
opened belongs to its spawn wake. A crash restart still closes every pre-boot
strand (owner rule: crash restarts end work).
