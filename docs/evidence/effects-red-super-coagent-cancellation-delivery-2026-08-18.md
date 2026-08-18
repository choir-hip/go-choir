# Effects persistent Super missing CoSuper cancellation delivery — 2026-08-18

**Boundary:** execute. Problem documentation first. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` reports `fcee593843386e027f78cecb81442ce317d8b69e` (deployed `2026-08-18T00:15:48Z`).

## Live observation

Following the deploy of `fcee5938` (completed-run cancel fix), the epoch-294 owner refresh cleanly cancelled leftover `assignment-c60a8912` without any boot reconcile errors.

However, retrying the operations POST started fresh Super runs (`dc29cc4f` and `8da73ba3`) that both completed without opening a new CoSuper assignment:

```
Status: no new CoSuper assignment opened because the same operation already has an existing bound implementation assignment recorded from the prior activation: assignment-c60a8912
```

## Root cause

Two substrate defects prevented persistent Super from learning that `assignment-c60a8912` was cancelled:

1. **Dead parent run delivery stamping:** `buildCoSuperReturnPacket` in `internal/store/cosuper_assignments.go` unconditionally stamped `DeliveredToRunID: assignment.Binding.ParentRunID, DeliveredAt: &now` on the cancellation return packet. Because the parent run (`c4cd7200`) was already terminal at the time of restart cancellation, stamping it as delivered to `c4cd7200` caused all future Super runs to treat the packet as already delivered, leaving it unconsumed in the mailbox.

2. **Control-only update filter on rewake:** In `internal/agentcore/super_controller.go`, when persistent Super runs with `request_source == "lifecycle_texture_control"`, `pendingCoagentUpdatesForRun` only queried `listPendingLifecyclePacketsDeliveredToRun` (which returns only Texture controls) and ignored pending CoSuper reports (`LifecyclePacketDirectionProducerReport`) in the trajectory mailbox. Additionally, `validateTargetBoundLifecycleControls` failed on any pending producer reports in `ListAllPendingLifecycleUpdates`.

Consequently, persistent Super's conversation memory retained the earlier message that it had opened `assignment-c60a8912`, but never received any update indicating the assignment had been cancelled. When Texture re-woke Super with an execution request, Super avoided opening a duplicate assignment.

## Repair

1. In `internal/store/cosuper_assignments.go`, `buildCoSuperReturnPacket` only sets `DeliveredToRunID` when `!parentRun.State.Terminal()`. For cancellations and late reports on terminal parents, `DeliveredToRunID` is left empty so the packet remains pending in the parent agent's mailbox.
2. In `internal/agentcore/super_controller.go`, `listPendingPersistentSuperLifecycleControls` filters for `Direction == Control`, and `pendingCoagentUpdatesForRun` includes pending admissible CoSuper producer reports for persistent Super runs in that trajectory.
3. In `internal/agentcore/selfdev_texture_join.go` and `internal/runtimeprompts/overlays/super_runtime.yaml`, prompt overlays and rewake execution request notes explicitly instruct Super to open a fresh CoSuper assignment when prior attempts on the operation are terminal.

## State

Operation `selfdev-b090bcd72d300fed17cb3f5a142f8595` stays `executing`, no bundle. Constructed freeze `7122f279` unchanged. Mode `propose_only` gen 1. No mail. This is not a freeze.
