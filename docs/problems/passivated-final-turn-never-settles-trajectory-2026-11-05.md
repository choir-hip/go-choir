# A trajectory whose last Texture run passivates never settles (2026-11-05)

Status: open; documented first, the fix follows in the same commit (mission
tempo rule 6). Class: red (lifecycle settlement).

## Evidence

d@e, trajectory f62bea6c-a40a-5ca4-9a2b-a608e3b3819a (build 9806c0506,
started 19:35:09Z): events trajectory_started, four texture_turn_committed,
work_settled at 19:37:52Z, then nothing for 40+ minutes; status live,
lifecycle_version 5. Runs: research 91428e1d completed 19:37:49Z; Texture
35990207 (the turn that settled the work) passivated 19:37:53Z with
passivated_reason durable_transition_success and no settle flag. Earlier
trajectories 2585dfb7 and 54a49cd5 settled only because a later Texture run
with no work item took one more turn and ended completed.

## Mechanism

Settlement is reachable from one place,
`Store.ReconcileLifecycleSettlementForTerminalRun` (internal/store/store.go),
called only when a run persists in a terminal state (completed, failed,
cancelled), and only for a run whose metadata carries
`lifecycle_terminal_settlement_requested`, set only when a terminal run held
the agent's active slot (internal/store/lifecycle.go). A work-bound Texture
run is made to passivate after its committed turn
(`WithPassivatingToolResult`, reason durable_transition_success), and
`RunPassivated` is not terminal. So when the turn that settles the last work
item passivates and nothing wakes another run, nothing ever settles the
trajectory. No sweep, timer or boot repair exists. Pre-existing; not a
regression from 2026-11-05's commits (fadd941ca only declares the store
reader).

## Fix

A passivation that releases the active slot is an end of activation like a
terminal state: it carries the settlement request and reconciles settlement
the same way. Readiness is unchanged (no open work items, required subject
refs), so a run that passivates waiting on open work does not settle.

Built: `lifecycleRunEndsActivation` (terminal or passivated) marks the
settlement request and triggers the reconcile; a request carries over only
from a terminal run, and a passivated run settles only while it is still the
agent's activation (a replacement run has taken over otherwise;
TestStaleTerminalRunCannotSettleReplacementActivation). Tests:
TestPassivatedRunAfterLastWorkSettlesTrajectory,
TestPassivatedRunWithOpenWorkDoesNotSettle. Residual: a Texture turn that
settles its work item while a Cast to research is still unanswered now
settles the trajectory at its passivation, so the late reply lands in a
settled trajectory (before, the next completed run settled it after the
reply). The stuck trajectory f62bea6c is not rescued (no run end follows).
