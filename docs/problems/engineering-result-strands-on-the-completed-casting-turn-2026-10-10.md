# An engineering result never wakes management: it is delivered to the casting turn, which has already completed (2026-10-10)

Status: documented, not fixed. Mutation class of the fix: `red` (persistent
management wake and lifecycle packet delivery).

## Evidence

arXiv demo on staging at e1bcb393 (test computer demo-arxiv-2026-10-10c,
vm-82cf0bb7…; traces in the session scratchpad under traces/arxiv6, event
pages read at limit=1000). This is the first delegated engineering run that
ran to the end:

- Management run 1f6de2b4 cast engineering at 21:14:18 and completed at
  21:14:21, as its prompt tells it to.
- Engineering run `run:delegated-sha256:b66e551d…` bound at 21:14:33. It
  chose arXiv:2209.10652 ("Toy Models of Superposition"), installed numpy
  through the L2 proxy despite the capsule defects in
  capsule-workload-cannot-run-a-shell-or-install-packages-2026-10-10.md,
  ran a 12-point density sweep, and passed 5 of its 5 pre-declared gating
  checks. It wrote REPORT.md and SELECTION.md under
  `experiments/toy-superposition-2209.10652/`, staged an
  `execution_result` packet at 21:34:32, and completed at 21:37:20.
- Guest log at 21:37:20: `actorruntime: persistent Management live
  occurrence received agent=management:be4649cf… from=engineering:delegated-sha256:b66e551d…`,
  then `runtime: persistent-Management rewarm delivered-pending-runs=0`,
  then nothing. No management run exists after 21:13:50. Texture's
  document stopped at revision 5 (21:21:50), so the owner never sees the
  result.

## Cause (from source; branch inferred from the log lines, not yet from a ledger read)

1. The occurrence is a producer report. `ResolvePersistentManagementLiveOccurrence`
   (`internal/agentcore/management_controller.go`) finds no undelivered
   match, then `persistentManagementReportOccurrencePending` finds the
   report in `ListDeliveredPendingProducerReports`: it was already
   delivered (DeliveredToRunID set) to the run that asked for it. For a
   delegated cast, that run is the casting management run, and it has
   completed. The handler takes `ErrPersistentManagementReportOccurrence`
   to `resumeGeneric`.
2. `resumeGeneric` (`internal/actorruntime/handler.go`) resumes a parked
   run. The run is terminal, so it calls `reconcileCoagentWake` →
   `reconcilePersistentManagementActorLocked`. That path mints a fresh turn
   only from pending Texture controls, admissible directives or the
   execution backlog. `filterPersistentManagementExecutionUpdates` drops
   producer reports by design ("a producer report: never mint execution").
   It returns no run, and the handler acknowledges the occurrence.
3. The design assumed the casting turn would be parked, waiting for its
   report. The management prompt tells management to end its turn after
   casting, and runs complete within seconds of the cast. So every
   delegated engineering result lands on a completed run.

## Clustering assessment

This is the fifth defect today on the delegated-cast path. All five
anchor on a turn instead of a durable object:

- reaped before spawn;
- authority dies with the turn;
- parent work found by scan;
- bound run built with document-cast parent fields;
- and now the result is delivered to the casting turn.

The first four are fixed and deployed. The substrate fact is that the
persistent management desk is long-lived while its runs are short turns.
Anything addressed to "the run that cast" must instead reach the
persistent desk and start a new turn.

No unwired replacement was found. `unbindStrandedLifecycleControls`
releases controls and directives bound to dead carriers, but it skips
producer reports. For a turn that completed, it settles the packet as
delivered-without-consume to avoid an endless mint. That rule is right
for a Texture control the turn already answered, and wrong for a report
that arrived after the turn ended. In that case the packet was never
injected into any turn.

## Belief state

Before: once a delegated engineering run binds, its result returns to
management. After: the result is durable, but nothing turns it into a
management turn, so the owner never hears of it. Remaining error field:
whether the report's DeliveredToRunID was set when it was produced (the
parent run binding) or by a later delivery attempt. A ledger read of the
report packet settles this before the fix.
