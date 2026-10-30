# S2a on the Paper: Every Management Task Refused by the Vocabulary Fence

Date: 2026-10-14. Found by the S2a acceptance check on the paper
(computer-1355b83b…, refreshed onto 9a0da89b at 13:01 UTC), with
`scripts/management_latency_probe.py --start 2026-10-14T13:01:00Z`.
Status: documented; fix follows in a separate commit.

## Evidence

Window 13:01-13:20 UTC: management delegated 6 intake tasks; all 6 closed
with reason `cancelled`, none started a run, 0 clean. Every task's final
report carries the same error:

    the task's run did not start: persist agent: vocab write guard:
    vocabfence: og:choir.agent:metadata.role="management_task" not in v2 vocabulary

The task reports returned their captures as residuals (18 residual
capture entries). Management then disposed of captures itself (4 opened,
5 fed); the paper kept publishing (13:25 UTC). Two delegations covered the
same captures 45 s apart (13:16:16 and 13:17:01), so a refused task's
captures are delegated again.

## Cause

S2a registered the `management_task` role in agentprofile, yaegikernel and
modelpolicy (a9d197568) but not in the V2 vocabulary
(`internal/vocabmigrate/fence.go`, `v2LiveSet`). Persisting the task's
agent object goes through the object-graph write guard
(`internal/store/vocab_migrate_og.go` `vocabWriteGuard`), which is active
only after the vocabulary cutover. Unit and package tests never set the
cutover, so they could not see it; only a computer past the cutover can.

## Effect

No harm to the paper beyond lost delegation: residual captures go back to
management, which disposes of them as before S2a. Wasted work: one
refused delegation per arrival, and repeat delegations of the same
captures.

## Fix direction

Add `management_task` to the V2 live set, with a test that a cutover store
persists a management_task agent. Then refresh the paper (owner-approved)
and re-run the S2a acceptance probe. A test gap to close: S2a's store tests
should run with the vocabulary cutover active.
