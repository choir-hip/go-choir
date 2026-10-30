# A cancelled run skips the post-turn hook, so its rounds stay open (2026-10-14)

Status: found in code while building management delegation S2a
(`docs/archive/reports/management-delegation-design-2026-10-14.md`). Not seen on
staging; hypothesis about frequency only. S2a handles the case for
management task runs; the general case is open.

## What the code does (code, branch `management-delegation-s2a`, base `e3437f1cd`)

- `Runtime.terminalizeRun` (`internal/agentcore/runtime.go`, reached from
  `CancelRun` and from `HandleActivationBudgetDeadline`, the durable
  activation-budget wake) persists the run as cancelled, then cancels the
  activation's context.
- The activation's tool loop then returns a context error, and
  `handleExecutionError` returns at once when the stored state is already
  terminal ("a stored terminal state always wins").
- So neither path calls `recheckRecordsInbox`, the post-turn hook that runs
  `closeRoundsOfEndedRun` (B6) and the D4 re-check.

Consequence: a research or management run cancelled by the owner, or ended
by its activation budget through the durable wake, leaves the rounds it is
the recipient of open, with no closure and no wake for the caster, until the
next crash boot closes them. The in-process budget path (the activation
context's own deadline firing first) does run the hook, so which path wins
the race decides whether the Texture hears back.

## Why it matters for delegation

A management task's deadline is its activation budget, and CancelTask is
`CancelRun` (design R2-20). Without the hook, a deadline or cancel would
leave the task round open and the lease held: failure mode 11 in the note.

## Handling in S2a

`terminalizeRun` closes the task of a management task run after it
terminalizes (`closeManagementTaskOfEndedRun`, idempotent). The general
fix, running the post-turn hook for every records desk from
`terminalizeRun`, is a separate change: it would close Texture rounds on
cancels that today stay open, which needs its own review of D5/D6 (a
cancelled management run with an engineering child).

## Residual

Named residual R-CANCEL: research and management runs ended through
`terminalizeRun` still skip `closeRoundsOfEndedRun`.
