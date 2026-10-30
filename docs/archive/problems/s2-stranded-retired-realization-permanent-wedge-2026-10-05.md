# A stranded accepted event with a retired realization wedges the computer permanently
Date: 2026-10-05 · Station: S2 · Surface: `internal/agentcore/platform_update.go` `resumePendingPlatformUpdate`

## Symptom
Computer `computer-6450a253b8b6ebc0866471973694f5be` refused **every** platform update with `platform update: base event head is stale`, indefinitely, across refreshes and deploys. Each boot logged the same pair:

```
runtime: platform update resume: re-driving update s2acc-apply-20261005T070526Z after guest restart
runtime: platform update resume: platform update: offer binds a different realization
```

## Mechanism
`ApplyPlatformUpdate` commits `effect_accepted` (opening `PendingTransitionRef`) **before** it calls the updater, and the updater's apply commonly kills its own caller (the guest restarts). If the guest restarts in that window, the accepted event is on the tape with the transition still open.

At the next boot `resumePendingPlatformUpdate` re-drives the **pinned** offer. But:
- realizations are epoch-monotonic (`vm-…-epoch-12873`), so after any VM epoch rotation the pinned `realization_id` can never match again;
- the accepted event's target commitment binds the offer digest, so the offer cannot be re-minted either.

The offer is therefore permanently inapplicable, yet the resume path only logged the error and returned. `PendingTransitionRef` stayed set forever, so `ApplyPlatformUpdate`'s resume gate took the `head.PendingTransitionRef != ""` branch for every *subsequent* offer, found no accepted event under that new offer's idempotency key, and returned `ErrPlatformUpdateStaleHead` — which presented as a head problem and sent diagnosis down the wrong path for hours.

## Why the existing fix did not cover it
`6fcb05e5` discharges a **pre-mutation refusal**: the updater refuses, no release took effect, so `materialization_failed` clears the transition. The stranded case is different — the updater was never reached, the guest died mid-flight, and no refusal ever existed to discharge.

## Fix
- `ErrPlatformUpdateRealizationBound` names the provably-permanent case, distinct from the transient `ErrPlatformUpdateStaleHead`.
- The resume sweep discharges a realization-bound strand with `materialization_failed` (nothing was mutated, so the prior effective state holds and `RestoredPriorEffective` clears the transition). Any other resume error stays a retry — transient failures must not write off an update that could still land.

The asymmetry that decides it: staying wedged makes the computer permanently unable to take *any* platform update, needing host surgery to clear. Discharging loses one update, which the next deploy re-offers. Losing one update is recoverable; a wedged computer is not.

## Regression
`TestPlatformUpdateBootSweepDischargesRetiredRealization` seeds an accepted event whose offer binds a retired realization and asserts the sweep clears the transition, commits exactly one `materialization_failed`, and that a later correctly-bound offer applies. Fails on the parent commit with the staging log line reproduced verbatim.