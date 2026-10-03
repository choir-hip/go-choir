# S0m finding: bound carrier consumes the work but the control packet is never marked incorporated

**Status**: open — in-contract blocker for the S0m finish contract.
**Date observed**: 2026-10-03 (mechanism corrected after trace inspection).
**Computer**: `computer-03335285269bdba4f94377e56879f9e6` (owner guest)
**Deployed when observed**: autoputer `ed406f45` (the ActiveRunID fix already live).
**Mutation class**: red when repaired (lifecycle consume/marking — protected surface).

## Symptom

After the `TerminalizeRun` `ActiveRunID` fix (ed406f45), the stranded-bound
rebind is proven: texture `ApplyTexture{open_researcher}` → control `3fbf9bb4`
bound to carrier `cf8c0a9e` → probe cancelled it mid-bind →
`claim_released_pending` → **`rebound_live_run`** minted fresh carrier
`c55287d3` (correct agent `research:9bdc8055`, work `e320d40d`).

But the bound control packet `3fbf9bb4` stayed `pending` — it was never
marked incorporated — and on terminalize a second `claim_released_pending`
freed it again. No `system:reducer` resolve could fire.

Evidence: `docs/evidence/s0m-stranded-bound-rebind3-2026-10-03.json` —
`rebound_live_run` (run `c55287d3`) → `claim_released_pending` for the same
update_id.

## Corrected mechanism (after trace inspection of `c55287d3`)

The earlier framing "carrier completed unconsumed" was wrong on the agency
axis. The carrier's run record (`api/runs` `c55287d3`) shows it **did the
work**: it ran, did the research ("today's UTC date is 2026-10-03"), marked
work item `e320d40d` `completed`, and reported to texture
(`lifecycle_activation_versions` binds update `3fbf9bb4` + work `e320d40d`;
`lifecycle_control_bindings` lists the same `update_id`). The carrier's turn
did NOT no-op — it incorporated the work.

The defect is narrower and mechanical: **the bound control packet's
disposition never flipped to incorporated.** Consume-marking
(`UpdateIncorporated` via the `expectedConsumed`/`ConsumedDeliveryUpdateIDs`
batch in `internal/store/lifecycle.go:3454-3513`, gated on `memorySeen` =
`lifecycleUpdateIDsInAuthenticatedRunMemory`) did not mark `3fbf9bb4`. So the
packet stayed `pending`, and on carrier terminalize `bindTerminalRunOutcome`
(terminalizeRelease=true) released it back — a second `claim_released_pending`.

Consequence: the carrier's report is emitted but the control packet is never
marked incorporated, so the record→packet→bind→**consume**→report→resolve
chain breaks at the marking step. `mechanicalResolveForReport` /
`system:reducer` resolve cannot fire on a packet that never registered as
consumed. This is a substrate defect, not agency: the carrier did the work;
the ledger didn't record the consume.

Separability confirmed: `consumeIdleTextureTrigger` is `Texture`-profile-gated
(`tools_desk.go`) — it cannot affect research carrier `c55287d3`. This defect
(A) and the idle-mask (B) are distinct; (A) is the in-contract blocker.

## First probe target

`internal/store/lifecycle.go:3454-3513` — the `expectedConsumed`/
`ConsumedDeliveryUpdateIDs` gate and `memorySeen` check. The carrier's report
commit either didn't list `3fbf9bb4` in `ConsumedDeliveryUpdateIDs`, or
`memorySeen` was false for it (the control packet never entered the run's
authenticated memory even though its work item did). Repro: re-run
`s0m_stranded_bound_probe`, then read the rebound carrier's run-memory entries
for update `3fbf9bb4`.

## Next boundary

Repair the consume-marking linkage: a bound control whose carrier consumes the
work and emits a report must be marked incorporated (`UpdateIncorporated`) so
the record can mechanically resolve. Then re-run the stranded-bound probe to a
terminal (consumed or `delivery_attempts_exhausted` — both contract-legal) and
a clean non-cancel leg to `system:reducer` resolve.
