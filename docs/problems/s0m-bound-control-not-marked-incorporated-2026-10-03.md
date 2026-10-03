# S0m finding: bound carrier consumes the work but the control packet is never marked incorporated

**Status**: open — ROOT CAUSE CONFIRMED in source 2026-10-03 (see Root cause).
The record-native `report` intent never consumes a bound control for a
research carrier.
**Date observed**: 2026-10-03 (mechanism corrected after trace inspection;
root cause in `rlm_reduce.go` confirmed same day).
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

## Root cause (confirmed in source 2026-10-03) — design gap, not a dropped write

The upstream control `3fbf9bb4` can only be released/incorporated when its
producer_report carries a `ControlBindingID` back-link, and the consume side
validates it (`texture_controller.go:1523-1535` — `producerBindingID =
canonical.ControlBindingID`, `bindingMatches` requires exactly 1 matching
control update).

But the research→texture report path **cannot carry `ControlBindingID`**:
- `commitAddressedPacketIntent` (`rlm_reduce.go:1421`) omits
  `ControlBindingID`/`TargetWorkItemID`/`ConsumedDeliveryUpdateIDs`.
- `QueueLifecycleUpdate` sends them down the `persistentManagementProducer`
  branch (`lifecycle.go:3392-3514`, which walks `deliveredPacketConsumptions`
  → `UpdateIncorporated`); the research `else` branch (`:3516-3519`)
  **rejects** a non-empty `ControlBindingID`/`TargetWorkItemID`.

So a research carrier's report always has empty `ControlBindingID`, the
consume-side `bindingMatches` check is skipped (`producerBindingID == ""`),
and the upstream control is orphaned — never marked `incorporated`, never
released. `3fbf9bb4` is exactly this: carrier `c55287d3` did the work and
reported, but its report had no back-link, so the control stayed bound until
`bindTerminalRunOutcome` released it on terminalize.

The only report that CAN carry the binding is the persistent-Management→
texture report (`tools_engineering_assignment.go:269-278`), which derives
`consumedDeliveryIDs` from the carrier's `memorySeen`∩delivered set. The 3
"incorporated by carrier process" packets seen 2026-10-03 used that path.

### Design fork (next boundary — red surface)
The binding semantics differ per path and can't be reused blindly:
- persistent-Management: `TargetWorkItemID` = the *texture-side* work item
  (validated `targetWork.AssignedAgentID == req.TargetAgentID`, texture).
- research: `TargetWorkItemID` = the *research* work item the control targets
  (`control.TargetWorkItemID` = research work, `AssignedAgentID` = research).

Options — recommend (b), which is the existing-authority path:
- **(a)** Extend the research producer-report request to carry
  `ControlBindingID` + its own (research-scoped) work-item validation +
  digest, so the report self-identifies its upstream control — then a
  consume-side release marks the control incorporated. Costs a new
  request-digest arm + a second producer-authority validation branch
  (research `TargetWorkItemID` semantics differ from Management's).
- **(b)** RECOMMENDED — consume-side inference: when the texture turn
  consumes a producer_report with empty `ControlBindingID` from a research
  producer, derive `producerBindingID` from the carrier run's
  `lifecycleActivationVersionsForRun` (`version.UpdateID` where
  `TargetWorkItemID == report.ProducerWorkItemID`). `version.UpdateID` IS
  the control's `UpdateID` (management_controller.go:199 stamps
  `update.UpdateID` into the activation version), so `bindingMatches`
  (texture_controller.go:1523-1535) then runs and the control can be marked
  incorporated. This matches existing authority:
  `ValidateLifecycleProducerReportAuthority` already derives the research
  binding from the run's activation fingerprint exactly this way when
  `ControlBindingID` is empty (texture_lifecycle_api.go:613-636) — no
  report-field change, authority stays on the durable fingerprint.

### Functional consequence (why it matters, not cosmetic)
An orphaned control stays `pending`. On carrier terminalize,
`bindTerminalRunOutcome`/`unbindStrandedLifecycleControls` releases it back
to pending; it can then **rebind to a fresh carrier and re-run already-
completed work** — the restart-recast heresy (a completed work item
re-executed as new). Marking `incorporated` on report consume closes the
control so it cannot re-pend. For `3fbf9bb4` the carrier had already
completed `e320d40d`, so a rebind would re-run finished research.

Existing regression `TestLifecycleRunTerminalizeReleasesStrandedControlAnd
RewakesDesk` covers the release half only; add a test asserting the control
is marked `incorporated`/`delivered-without-consume` when its carrier did the
work (not released to re-pend).

### Open semantic question for the fix
When does a research control count "consumed"? Two candidate commit points:
- **carrier-terminalize**: research `boundClaimRelease`-on-any-terminal
  (management_controller.go:1170-1172) is what re-pends a completed-work
  control. A `boundClaimSettle` for a carrier that completed its bound work
  (or emitted its report) would terminalize delivered-without-consume
  instead of re-pending — arguably the smallest fix, no consume-path
  change. The risk: it consumes the instruction even if the carrier never
  truly incorporated it.
- **report-consume (option b)**: mark the control when the texture desk
  consumes the carrier's report — consume is tied to evidence of delivery,
  not just carrier liveness. Stronger, but ties control fate to a downstream
  event.

The (b) consume-side inference is still recommended for authority
cleanliness; pick the commit point at implementation time with the
"what counts as consumed" invariant written into the test.

## Deployed re-check (2026-10-03, autoputer d61c9b1b)

Post-recovery control-packet scan on the owner computer (post guest reboot
+ outbox drain): **3 bound control packets reached `disposition=incorporated`
with reason `bound lifecycle control incorporated by carrier process`**
(`1db9be5a`→`8b5bc6db`, `bc34f0fc`→`27bd9a26`, `c1e9e1cf`→`c8399a11`) — the
`UpdateIncorporated`/`ConsumedDeliveryUpdateIDs` marking path works when the
bound carrier's report commit lists the update. This narrows `3fbf9bb4`: its
carrier's report commit apparently did NOT list the update (or `memorySeen`
was false for it specifically), so the defect is a per-carrier consume-mark
omission, not a dead marking path. Repro still requires the stranded-bound
probe reaching a consume on a rebound carrier — the probe's bind window
repeatedly missed the stochastic `open_researcher` desk-act under post-boot
recovery latency (desk-mint ~10-35 min behind the draining queue).

## Next boundary

Repair the consume-marking linkage: a bound control whose carrier consumes the
work and emits a report must be marked incorporated (`UpdateIncorporated`) so
the record can mechanically resolve. Then re-run the stranded-bound probe to a
terminal (consumed or `delivery_attempts_exhausted` — both contract-legal) and
a clean non-cancel leg to `system:reducer` resolve.
