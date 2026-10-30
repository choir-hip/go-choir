# S0m finding: freed control re-strand — RESOLVED (TerminalizeRun clears ActiveRunID)

**Status**: repaired, deployed, re-proven on staging.
**Date observed**: 2026-10-03
**Computer**: `computer-03335285269bdba4f94377e56879f9e6` (owner guest)
**Deployed**: autoputer `ed406f45` (the fix) — staging `/health` confirmed.
**Mutation class**: red (lifecycle run/agent projection — protected surface).

## Symptom (probe `s0m-stranded-bound-rebind2-2026-10-03.json`)

Texture desk stages `ApplyTexture{controls:[open_researcher]}` → control
`a7c94abf` mints `LifecyclePacketDirectionControl` → binds live carrier
`670d3179` (agent `research:251c4f4e-…`). Probe cancels the carrier
mid-bind. The claim is **released** — the packet returns to
`disposition=pending, delivered_to_run_id=null, delivered_at=null,
delivery_attempts=1` — exactly the state the `c31bf43a` unbind repair
targets. **But no replacement carrier mints for 8 minutes.** The freed
packet re-strands: release worked, rebind never fired.

## Reproduction (local, deterministic)

`internal/agentcore/zz_rebind_repro_test.go` —
`TestFreedLifecycleControlRebindsAfterCarrierCancel`:

```
seed control -> ReconcileCoagentWake mints carrier -> CancelRun(carrier)
  -> freed packet pending (DeliveredAt cleared, unbind fired)
  -> ReconcileCoagentWake => ErrLifecycleInvalidTransition
  -> no carrier minted
```

`agent.ActiveRunID` after cancel = `"c8243d9e…"` (the dead carrier) —
**never cleared.**

## Root cause — the ActiveRunID clear lives on the wrong reducer

Two separate reducers manage `agent.ActiveRunID`:

- `projectLifecycleRun` / `ReplaceLifecycleActivation`
  (`internal/store/lifecycle.go:2913-2935`) — the **activation** path.
  Writes `agent.ActiveRunID = run.RunID` when a run owns activation, and
  clears it (`agent.ActiveRunID = ""`) when that run terminalizes
  (`previousActiveRunID == run.RunID`).
- `TerminalizeRun` (`internal/store/lifecycle.go:4939+`) — the
  **cancel/terminalize** reducer. It writes the run object to terminal
  state and emits `LifecycleRunTerminalized`, but **never touches
  `agent.ActiveRunID`.**

So a carrier terminalized via `TerminalizeRun` (the `CancelRun` path →
`terminalizeRunCanonical`) leaves `agent.ActiveRunID` pointing at a dead
run.

`reconcileUpdatedCoagentActor` → `ResolveLifecycleControlActivation`
(`lifecycle_control_delivery.go:283-291`) reads
`agent.ActiveRunID`, loads that run, and requires
`lifecycleRunOwnsActivation(active.State)` (pending|running). A
cancelled/completed run fails → `ErrLifecycleInvalidTransition` →
the `replayErr` propagates up through `reconcileUpdatedCoagentActor`
(`management_controller.go:2042`) and the reconcile **refuses to mint a
carrier** — for the exact packet it was woken to bind.

## Why completed carriers pass and cancelled carriers strand

`ResolveLifecycleControlActivation` returns `{Active, DurablyFailed,
Completed}`:

- `resolveLifecycleControlActivationCompleted` matches `RunCompleted` →
  `replay.Completed` set → `reconcileUpdatedCoagentActor` early-returns
  `nil,nil` at `management_controller.go:2062` — **before** the
  active-run gate is reached. The CI test
  `TestLifecycleRunTerminalizeReleasesStrandedControlAndRewakesDesk`
  terminalizes a `running` carrier to `RunCompleted`, so it passes: the
  stale `ActiveRunID` is masked by the `Completed` early-return.
- A `RunCancelled` carrier is neither `Completed` nor `DurablyFailed`
  (the failure resolver only matches `RunFailed`), so the reconcile
  falls through to `ResolveLifecycleControlActivation`'s active-run
  gate → stale `ActiveRunID` → `ErrLifecycleInvalidTransition`.

The defect only bites **cancelled** (and `RunFailed`) carriers —
precisely the killed-mid-bind shape the stranded-bound rebind re-proof
forces. `RunCompleted` carriers strand the same way but are masked by
the `replay.Completed` early-return.

## Fix direction (substrate)

Clear `agent.ActiveRunID` in the `TerminalizeRun` reducer when the
terminalized run is the agent's current `ActiveRunID` — mirroring the
clear in `projectLifecycleRun`. Single-authority: `agent.ActiveRunID`
must reflect the agent's *live* activation or be empty; a terminal run
must never remain `ActiveRunID`.

Alternative considered: make `ResolveLifecycleControlActivation`
treat a terminal `agent.ActiveRunID` as no-active rather than
`ErrLifecycleInvalidTransition`. That widens the replay resolver's
contract (it's used to detect genuinely-ambiguous activations); the
write-side invariant (don't leave a dead run as ActiveRunID) is the
correct repair — same "fix the write, not the read" reasoning as the
`DeliveredAt` repair.

## Evidence

- Staging probe `docs/evidence/s0m-stranded-bound-rebind2-2026-10-03.json`
  (`ok=false`, 8-min `claim_released_pending` loop, `final_update.disposition=pending`).
- Local reproducer `zz_rebind_repro_test.go` (`ErrLifecycleInvalidTransition`,
  stale `agent.ActiveRunID=c8243d9e`).
- Code refs: `internal/store/lifecycle.go:4939` (TerminalizeRun, no
  ActiveRunID write), `internal/store/lifecycle.go:2913-2935`
  (projectLifecycleRun, the only ActiveRunID writer),
  `internal/store/lifecycle_control_delivery.go:283-291` (active-run
  gate), `internal/agentcore/management_controller.go:2042` (replayErr
  propagation).
- CI test that masks the bug: `lifecycle_control_injection_test.go:1044`
  (terminalizes `running`→`completed`, hits `replay.Completed` early-return).

## Resolution (deployed + re-proven)

`TerminalizeRun` now clears `agent.ActiveRunID` when the terminalizing run is
the agent's live activation — folded into the same atomic commit batch
(run+trajectory+event+receipt+agent). Commit `ed406f45`; deployed to staging.

Deployed re-proof `docs/evidence/s0m-stranded-bound-rebind3-2026-10-03.json`:
texture `ApplyTexture{open_researcher}` → control `3fbf9bb4` bound to carrier
`cf8c0a9e` → probe cancelled it mid-bind → `claim_released_pending` →
**`rebound_live_run`** to fresh carrier `c55287d3` (correct agent
`research:9bdc8055`, work `e320d40d`). The freed packet rebound through the
repaired path — the stranded-bound rebind is proven end-to-end.

Regression `TestFreedLifecycleControlRebindsAfterCarrierCancel`
(`lifecycle_control_rebind_cancel_test.go`): seed → mint carrier →
`CancelRun` mid-bind → assert re-pend + `ActiveRunID=""` + fresh carrier
minted. Fails on the old code (`ErrLifecycleInvalidTransition` → nil
rebound), passes on the fix.

Residual (next boundary, separate defect): the rebound carrier `c55287d3`
bound the packet then **completed without consuming it** — the packet stayed
`pending` and a second release freed it. A bound-but-unconsumed carrier is a
delivery/consume defect, not a rebind defect; rebind is verified.
