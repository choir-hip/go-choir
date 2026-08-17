# Effects CoSuper completed-run cancel aborts boot reconcile — 2026-08-17

**Boundary:** execute. Problem documentation first. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` reports `a1f3d2cf93a84cf6c20e246202e6b74b90a6e932`, deployed `2026-08-17T21:44:54Z`. Guest observability `deployed_at 2026-08-17T21:45:46Z`.

## Live observation

Staging is on `a1f3d2cf` (capsule PATH + overlay writable) at epoch **293**, computer `active`, mode `propose_only` generation 1. The previous CoSuper `assignment-c60a8912-a578-547e-9293-5a922a3de040` is still `bound` with capsule `revoked`, work item `open`, and `run:assignment-c60a8912` already **completed**. The CoSuper agent has no `active_run_id`.

Boot reconcile at `2026-08-17T21:45:47Z` aborted the whole trajectory:

```
runtime: boot CoSuper assignment capsule sweep trajectory e826402d-b666-503f-93ba-72b3bcf51e8d: co-super assignment invalid transition
runtime: boot work-item sweep owner=5bd6de97-3b58-408c-bf89-c42c81b083de agent=co-super:assignment-c60a8912-a578-547e-9293-5a922a3de040 trajectory=e826402d-b666-503f-93ba-72b3bcf51e8d: co-super assignment invalid transition
```

`ReconcileCoSuperAssignmentsForTrajectory` reaches the bound-but-unusable assignment, revokes the already-revoked capsule, then `CancelCoSuperAssignment`. `projectCoSuperTerminal` refuses because:

1. Work is still `open`, but completing the run already released `agent.ActiveRunID`, so `ActiveRunID != BoundRunID`.
2. The run is already terminal (`completed`) while cancel wants `cancelled`.

This is the same fail-fast-per-trajectory shape as `cec68e23`: one settled-but-mismatched obligation aborts recovery of the rest. The capsule-tools environment is deployed and is not the current blocker.

## State

Operation `selfdev-b090bcd72d300fed17cb3f5a142f8595` stays `executing`, no bundle. Super `c4cd7200` is terminal. Constructed freeze `7122f279` unchanged. Mode `propose_only` gen 1. No mail. This is not a freeze.

## Next

Repair `projectCoSuperTerminal` so restart cancel can close an assignment whose bound run already completed and whose agent already released `ActiveRunID`. Then refresh and retry the same operations POST. Do not retry Super while `assignment-c60a8912` remains bound.
