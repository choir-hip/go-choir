# Effects Super continuation storm after CoSuper cancel — 2026-08-19

**Boundary:** diagnose. Not freeze. Not promote. No live send.
No restore. No HTTP Super-start / operations POST. No
`maxToolLoopIterations` patch. Do not cancel Super `999bd208`
(that would restorm immediately on current code).

**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

**Prior wake proof:** `docs/evidence/effects-red-super-continuation-after-cosuper-cancel-2026-08-19.md`
(`9bc99f90` deployed as `51b18f54`, epoch 327).

## Live observation (2026-08-19T19:17Z)

Staging `/health` `deployed_commit`
`51b18f5440d9b3acec2713f71786db695263c37c` (`deployed_at` 18:54:54Z).
Retained computer `computer-03335285269bdba4f94377e56879f9e6` epoch
**327**, `propose_only` generation 1. Effects remain OFF.

Super continuation `b57705fd-6e39-4fc6-9a2a-4aa8f0caac3d` **failed**
at 19:11:37Z:

```text
tool loop: exceeded 200 iterations without end_turn
```

Created 18:58:39Z. Prompt remains `Process pending coagent update
packets for privileged execution.` `request_source=lifecycle_texture_control`.
`requested_by_agent_id=co-super:assignment-97191e37-657c-5acf-af18-f1c80d09def2`.
Same Super work `fd43ecca-cb82-53cf-91b5-dbe6f2412f97`. No
`input_tokens`/`output_tokens` in run metadata. No new CoSuper run in
`/api/runs`.

One second later, Super `999bd208-9aa6-4646-910b-746e63763b7d`
**started** (19:11:38Z), still `running` as of 19:17:06Z with
`updated_at` frozen at 19:11:38Z. Identical:

- prompt
- `request_source=lifecycle_texture_control`
- `requested_by_agent_id=co-super:assignment-97191e37-…`
- `assignment_trajectory_id=5242ca03-7513-5809-be58-4d43cbeab18f`
- `work_item_ids` (`fd43ecca`, `4671d318`, `38b96770`)

Operation `selfdev-ccf0f1ec0e851750f253fe5f5ed97974` remains
`executing`, empty `bundle_digest`, `updated_at` still 17:45:05Z.
Texture document `c273a57b` still version 1 (18:58:47Z). No HTTP
operations POST. Pre-A checkpoint `99949fe2` unchanged.

This is not an iteration-0 gateway hang. Super ran ~13 minutes then
hit the same 200-iteration ceiling as CoSuper `97191e37`. Super did
not bind a new assignment.

## Causal chain

`9bc99f90` starts Super from pending admissible CoSuper
`producer_report`s as `lifecycle_texture_control` **without**
`bindLifecycleControlsToRun` and **without** `worker_update_ids`.
That is deliberate: reports are not Texture Control packets.

The 2026-08-18 injector still delivers cancel text because
`pendingCoagentUpdatesForRun` appends pending admissible reports
from `ListAllPendingLifecycleUpdates`, which keeps only packets with
empty `DeliveredToRunID` and `DeliveredAt` (`internal/store/lifecycle.go`).
`TestPersistentSuperContinuesFromCoSuperSystemCancellationWithoutTextureRewake`
asserts that the cancel report remains undelivered and that injector
text includes the 200-iteration reason. It does not fail the
continuation Super and re-reconcile.

When continuation Super terminals, `handleExecutionError` calls
`maybeContinuePersistentSuperInbox`. That function stamps mailbox
delivery only for `request_source=update_coagent` completed inbox
runs (`markPersistentSuperRunUpdatesDelivered`). It then **always**
calls `reconcilePersistentSuperActor`.

`listPendingPersistentSuperAdmissibleReports` still finds the same
CoSuper `97191e37` blocker (`DeliveredToRunID` empty). Reconcile
starts another Super with the same prompt, requester, trajectory,
and work IDs. Live interval: `b57705fd` failed 19:11:37.715Z →
`999bd208` created 19:11:38.064Z.

`MarkWorkerUpdatesDelivered` skips `LifecycleVersion > 0`, so the
existing mailbox stamp cannot consume a lifecycle producer_report.
`BindLifecycleControlDelivery` is Control-only.
`ListLifecycleControlsDeliveredToRunPage` lists controls delivered
to a run, not producer_reports. There is no production stamp that
attaches a CoSuper `producer_report` to a continuation Super without
treating it as Texture Control.

## What this is not

- Not a miss of the 9bc99f90 wake. Continuation Super did start
  without HTTP Super-start.
- Not the 2026-08-18 dead-parent delivery stamp. `DeliveredToRunID`
  is still empty by design of that repair and of 9bc99f90.
- Not a reason to raise `maxToolLoopIterations`. A longer Super loop
  would only lengthen each storm cycle.
- Not a reason to HTTP Super-start or cancel `999bd208`. Cancel would
  terminalize and restorm immediately.

## Required join (source; live re-probe after deploy)

A CoSuper `producer_report` that starts continuation Super must be
consumed for **wake** so a terminal Super cannot start another Super
from the same packet, while remaining injectable into **that** Super
run (cancel text). That must not be `BindLifecycleControlDelivery`
and must not stamp `DeliveredToRunID` (that list is the injector
source). Continuation Super records `producer_report_ids`;
`listPendingPersistentSuperAdmissibleReports` skips IDs already
claimed by a terminal `lifecycle_texture_control` Super.

Covered by extending
`TestPersistentSuperContinuesFromCoSuperSystemCancellationWithoutTextureRewake`:
after continuation Super is marked failed with the 200-iteration
error, `maybeContinuePersistentSuperInbox` / reconcile must not
create a third Super; the cancel report stays undelivered for the
injector.

Super 200-iteration looping without `assign_co_super` remains a
separate residual. Do not raise the cap to paper over it.

## Residual (still unpaid)

- Super `999bd208` in-flight; expect a third Super when it 200-fails
  unless this stamp deploys first.
- Super continuation burned 200 iterations without a new CoSuper.
  Run events are not exposed on `/api/runs/{id}/events` (HTTP 405).
- `maxToolLoopIterations=200` still aborts capsule authorship.
- Deploy skipped active VM refresh; `deploy-impact` still last-push-range.

## Forbidden until Super binds a new CoSuper after processing the blocker once

- freeze / propose / promote
- live send
- restore
- HTTP Super-start / operations POST
- raising `maxToolLoopIterations`
- cancelling Super `999bd208` as a substitute for the stamp
- SQL-empty or replace the retained computer

## Rollback

No product-state rollback in this receipt. Checkpoint `99949fe2`
remains the pre-A fence. This file is docs-only.
