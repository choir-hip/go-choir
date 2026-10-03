# S0m finding: freed control packet never rebinds — `delivered_at` invariant break on unbind

Written 2026-10-03. Mutation class: red (protected surface — lifecycle control
reducer `ReconcileUpdateDelivery` + the coagent pending-scan invariant).

## Symptom

Deployed probe `scripts/s0m_stranded_bound_probe.mjs` on guest `39e924aa`
(staging, healthy, post-live-lock-repair). The ask→control→bind→carrier-kill
chain runs cleanly: texture mints control packet `6e4e3069` targeting
`research:e0768ae7`, it binds to carrier run `4ab7b076`, the probe POSTs
`/api/runs/4ab7b076/cancel`, and the packet's claim releases back to
`disposition=pending, delivered_to_run_id=null, delivery_attempts=1`.

**The release leg is proven.** The rebind leg is not: the freed packet stays
`pending` with `delivered_to_run_id=null` for the full 8-minute recovery window
and beyond — no fresh `research:e0768ae7` carrier run is ever minted. The desk
wake that should re-drive the reconcile produces **zero log lines** (no
`unbind stranded`, no `wake coagent`, no `reconcile`, no mint).

Probe result: `ok=false`, `final={"disposition":"pending"}` — the packet is
stranded as *pending but never deliverable*.

## Root cause — verified in source

`bindTerminalRunOutcome` → `unbindStrandedLifecycleControls`
(`management_controller.go:1131`) frees a dead run's bound control packet by
sending `ReconcileUpdateDelivery` with `TargetRunID: ""` (line 1180). In the
reducer `internal/store/lifecycle_update_delivery.go:129-133`, the non-exhausted
branch unconditionally writes:

```go
update.DeliveredToRunID = strings.TrimSpace(req.TargetRunID) // ""
update.DeliveredAt        = &now                            // re-stamped!
update.DeliveryAttempts++
```

So on a **pure unbind** (`TargetRunID==""`, `Exhaust=false`) the reducer clears
the run claim but **re-stamps `DeliveredAt=&now`** — a delivery timestamp with
no bound run — and bumps the attempt counter.

The coagent pending scan then excludes the freed packet.
`ListPendingLifecycleUpdates` (`internal/store/lifecycle.go:1955`) filters:

```go
update.Disposition == types.UpdatePending && update.DeliveredAt == nil &&
strings.TrimSpace(update.DeliveredToRunID) == ""
```

`DeliveredAt != nil` ⇒ the freed packet is **invisible** to
`ListAllPendingLifecycleUpdates` → `reconcileUpdatedCoagentActor`
(`management_controller.go:1838`) sees zero pending controls → `updates` is
empty → line 1930 `return nil, nil` → **no carrier minted, no error, no log**.

The wake (`wakeUpdatedCoagent` → `coagent_result` occurrence →
`reconcileCoagentWake` → `ReconcileCoagentWake` → `reconcileUpdatedCoagentActor`)
*does* fire — but the reconcile finds nothing to bind because the scan dropped
the exact packet it was woken for. The occurrence consumes cleanly
(`acknowledgeDurablyTerminal`/`nil`), so nothing retries, and the defect is
fully silent.

## The invariant it breaks

`DeliveredAt` is the **claim timestamp** — `ListActionablePendingLifecycleUpdates`
(`lifecycle.go:1991`) exists precisely because "a bound-but-pending packet is
still an exact pending trigger whose claim run may have died without consuming
it." The pending-scan contract is:

> `Disposition==Pending && DeliveredAt==nil && DeliveredToRunID==""`
> ⟺ an unclaimed, deliverable packet.

The unbind path violates it by leaving `DeliveredAt` non-nil on a now-unclaimed
row: the packet reads as *claimed* (DeliveredAt set) while being *unclaimed*
(DeliveredToRunID empty) — a state that matches neither the pending scan (needs
`DeliveredAt==nil`) nor the bound scan (`ListBoundPendingUpdatesForTarget`,
which needs `DeliveredToRunID != ""`). It falls between both and strands.

## Why the attempt count also lies

`DeliveryAttempts++` on the unbind branch charges the freed packet for the
*dead* binding, so a second strand bumps it toward `delivery_attempts_exhausted`
even though no live carrier ever consumed it. The attempt counter is meant to
bound *delivery retries*, not count how many times a dead run's claim was
reclaimed.

## Fix direction (substrate, not trigger)

Restore the invariant: **a non-exhausted `TargetRunID==""` reconcile (pure
unbind) must clear `DeliveredAt=nil` and must not count as a delivery
attempt.** The exhausted branch already does this correctly (sets
`DeliveredAt=nil` + terminal fate). The rebind/append path
(`bindLifecycleControlsToRun`) stamps `DeliveredAt` when it *actually* binds a
live run — so the fix is narrowly the unbind branch: on `TargetRunID==""` and
not exhausted, write `DeliveredToRunID="", DeliveredAt=nil` and skip the
`DeliveryAttempts++` (a dead-claim reclaim is not a delivery attempt).

Alternative considered: switch `reconcileUpdatedCoagentActor` to
`ListActionablePendingLifecycleUpdates`. That would also surface the freed
packet, but it widens the scan to include genuinely-bound rows and leaves the
`DeliveredAt` semantic broken for every other consumer — the unbind branch is
the one writing a false claim timestamp, so the write is what should be
corrected.

## Evidence

- Probe `docs/evidence/s0m-stranded-bound-2026-10-03.json`: carrier
  `4ab7b076` cancelled → `claim_released_pending` → `final={"disposition":"pending"}`.
- Trajectory `0e65e932-2e7b-504e-b6b5-32a49635ba40` (staging): packet
  `6e4e3069` `disposition=pending, delivered_to_run_id=null, delivery_attempts=1`,
  `delivered_at=12:14:03` (the unbind's re-stamp).
- Guest `research:e0768ae7` run list: only the cancelled `4ab7b076`; no fresh
  carrier minted after the free.
- Guest console `go-choir-vmctl` 12:12–12:20: texture turns logged, zero
  stranded/unbind/wake/reconcile lines for `6e4e3069`/`e0768ae7`.
- Code refs: `internal/store/lifecycle_update_delivery.go:122-133`,
  `internal/store/lifecycle.go:1955,1997`,
  `internal/agentcore/management_controller.go:1131,1180,1838,1930,3057`,
  `internal/agentcore/research_checkpoint_fallback.go:53-59`.
