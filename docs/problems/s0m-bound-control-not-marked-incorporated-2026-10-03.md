# S0m finding: bound carrier consumes the work but the control packet is never marked incorporated

**Status**: closed by `d61c9b1b` (2026-10-03 18:24 UTC) — consume-marking
at report-commit landed `commitLifecycleProducerReportAct`
(`lifecycle_commit_act.go:621-670`), verified live same day. `3fbf9bb4`'s
rebound carrier reported ~17:41 UTC, ~43 min BEFORE the fix — the incident
predates the repair, not a live gap.
**Date observed**: 2026-10-03 (mechanism corrected; resolved by panel
convergent verdict agentic-consensus-20261003-190021).
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

## Resolution — panel convergent verdict (agentic-consensus-20261003-190021)

**The repair already landed today in `d61c9b1b`.** `commitLifecycleProducer
ReportAct` (`lifecycle_commit_act.go:621-670`) folds every bound-pending
control for the reported work item into the same atomic commit as the
producer report: `Disposition=incorporated`, `DispositionRef=<report id>`,
reason `"bound lifecycle control incorporated by carrier producer report"`.
Regression `TestCommitLifecycleActProducerReportMarksBoundControl
Incorporated` passes; the 5 incorporated controls seen live carry exactly
that reason.

`3fbf9bb4` is **pre-fix evidence, not a live gap**: its rebound carrier
`c55287d3` committed its report ~17:41 UTC, ~43 min before `d61c9b1b`
(18:24 UTC) was committed/deployed. At that moment no consume-marking
existed for research carriers — correct diagnosis, since repaired.

### Why the panel rejected the other framings
- **Not (a) report-carried binding:** the run's `lifecycle_activation_versions`
  fingerprint already carries the binding authoritatively
  (`version.UpdateID == control.UpdateID`, matched on `ProducerWorkItemID`);
  a caller-supplied `ControlBindingID` would be a second, weaker, digest-arm
  source that must be re-validated anyway. Same field, different
  `TargetWorkItemID` meaning per path — the smell.
- **Not (b) consume-at-texture-read:** `report commit → carrier terminalize
  → texture consume` leaves the re-pend/rebind window open; consume must be
  atomic with the report, not its later read. (b)'s *authority* (fingerprint
  derivation) is right, but its *commit point* is too late.
- **Not (c) carrier-terminalize settle:** consumes the instruction even if
  the carrier never reported (crash/empty turn) — masks real failure, breaks
  the open-work retry contract, and `runtime.go:3090` requires
  `pending|incorporated` for the texture consume check anyway.

### Consume-point invariant (settled)
A research `control` is **consumed when its carrier commits a
`producer_report` for that control's target work item** — atomically in the
same reducer commit as the report packet. A partial report still consumes
the delivery (the instruction was acted on); a carrier that terminalizes
*without* reporting still releases the control for rebind. This is the same
rule the persistent-Management path already followed.

### Residual coverage flag (worth one audit, not a blocker)
Two store entry paths can write a research `producer_report`: the record-
native `CommitLifecycleAct` (consume-marked, `commitLifecycleProducerReport
Act`) and the legacy `QueueLifecycleUpdate` research `else` branch
(`lifecycle.go:3516`, which omits consume-marking). A lifecycle-bound
carrier's `IntentReport` routes to `commitLifecycleActIntent`
(`rlm_reduce.go:919`) first and only falls to `commitAddressedPacketIntent`
on the `errLifecycleActLegacyCaller` sentinel — which requires the caller
run to be absent from the lifecycle store, a state that cannot hold a bound
control. So the gap is believed **unreachable for bound controls**; the only
residuals are (i) the `update_coagent` tool path (`tools_worker_update.go:248`,
same `QueueLifecycleUpdate` research branch, no consume fields) and (ii)
pre-cutover legacy runs. If those can still mint a bound control, port the
same fold to `QueueLifecycleUpdate`'s research else-branch for path parity.

## Deployed re-check (2026-10-03, autoputer d61c9b1b)

Post-recovery control-packet scan on the owner computer (post guest reboot
+ outbox drain): **5 bound control packets `disposition=incorporated` with
reason `bound lifecycle control incorporated by carrier producer report`**
(`1db9be5a`, `bc34f0fc`, `c1e9e1cf`, `99a04fff`, `eed8bc04`) — the
`d61c9b1b` consume-marking is live and working. (Earlier draft said
`"...by carrier process"`; the actual reason string is `"...by carrier
producer report"` — provenance confirmed in source.) `3fbf9bb4`'s
non-incorporation was a pre-fix event.
