# S0m finding: bound carrier completes without consuming its delivered control

**Status**: open — next repair boundary after the rebind fix.
**Date observed**: 2026-10-03
**Computer**: `computer-03335285269bdba4f94377e56879f9e6` (owner guest)
**Deployed when observed**: autoputer `ed406f45` (the ActiveRunID fix already live).
**Mutation class**: red when repaired (lifecycle consume / run-terminalization).

## Symptom

After the `TerminalizeRun` `ActiveRunID` fix (ed406f45), the stranded-bound
rebind is proven: texture `ApplyTexture{open_researcher}` → control `3fbf9bb4`
bound to carrier `cf8c0a9e` → probe cancelled it mid-bind →
`claim_released_pending` → **`rebound_live_run`** minted fresh carrier
`c55287d3` (correct agent `research:9bdc8055`, work `e320d40d`).

But the rebound carrier **completed without consuming the bound control**:
packet `3fbf9bb4` stayed `pending`, and a *second* `claim_released_pending`
freed it again. The carrier ran to `completed` while its bound control was
never incorporated — no `producer_report`, so no `system:reducer` resolve.

Evidence: `docs/evidence/s0m-stranded-bound-rebind3-2026-10-03.json` —
`rebound_live_run` (run `c55287d3` `disp=pending` repeatedly) → carrier
terminalized `completed` → `claim_released_pending` for the same update_id.

## Working hypothesis

A carrier run minted to consume a bound control can terminalize (`completed`)
before/without its consume turn incorporating the delivered packet. Either the
consume cell never ran for `c55287d3`, or it ran and the delivered control was
not consumed (the same class of defect as the `consumeIdleTextureTrigger` mask
in `s0m-desk-no-authoring-act-2026-10-03.md`: a non-ApplyTexture authored cell
reads as "no act"). On terminalize the bound-but-unconsumed control is released
(`DeliveredAt` cleared) back to pending — leaving the packet re-stranded.

The rebind substrate is proven; this is the **consume** half of the finish
contract: a bound carrier must incorporate its control before completing.

## Impact

The rebind fix unblocks re-binding but not resolution — a bound carrier that
completes unconsumed re-strands the packet, so the mechanical resolve
(`consumer+producer` report → `system:reducer` resolve) still can't complete
on a cancel-heavy path.

## First probe target

`internal/agentcore/runtime.go` `ListLifecycleControlsDeliveredToRunPage` /
the consume path that marks a delivered control incorporated
(`internal/store/lifecycle.go` around the `DeliveredToRunID == run` scan, the
`consumedObjects`/`DispositionReason` "authenticated durable run-memory
delivery" batch), plus the run-terminalize path that releases a bound-but-
unconsumed control. Repro: re-run `s0m_stranded_bound_probe` on staging and
inspect whether `c55287d3`-like carriers ever execute a consume cell before
`completed`.

## Next boundary

Repair the consume leg: a run that binds a delivered control must consume it
(or the bound control must be released before the run can terminalize with the
packet still pending). This is the remaining delivery defect blocking the
finish-acceptance mechanical resolve.
