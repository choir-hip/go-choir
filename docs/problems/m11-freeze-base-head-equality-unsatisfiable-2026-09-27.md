# M11 blocker: `choir.Freeze` base-head equality is unsatisfiable on a live tape

**Date:** 2026-09-27
**Class:** red (self-development transition authority — the freeze leg decides
what the computer's canonical event chain binds)
**Status:** repaired 2026-09-27 (commit 3812ec3e): freeze now asserts the
pinned projection surface (desired/effective heads, empty pending
transition) instead of canonical-head equality; regression test
`TestFreezeAcceptsAdvancedBookkeepingHead` covers both directions.
**Deployed proof pending** — episode probe rerun required.
**First observed:** probe `M11_SELFDEV_EPISODE_1790532595839`, staging
`19e2c256` (RLM-only desk deployed), computer
`computer-330816470b1eb4386fd680e03a3dca3d`, operation
`selfdev-fb21cd3498a0f5c4efd8122985a29698`.

## Evidence

The episode ran correctly up to freeze:

- Op opened `trajectory_started` at 18:10:09Z; the `engineering:<docID>`
  occurrence incorporated; assignment `assignment-c0bf5f4d-…` opened, bound,
  ran 48 `capsule_go_eval` cells over ~10 minutes, authored
  `docs/evidence/m11-selfdev-episode-1790532595839.md` (3322 bytes,
  sha256:6cd18f9c…), verified it in-capsule (sha match, marker checks PASS).
- `choir.Freeze` was staged twice in clean cells; both reductions failed
  identically: `reduce: persist tray-1: self-development base head
  unavailable, stale, or pending` (`internal/agentcore/tools_capsule.go:262`).
- The desk reported `blocked` (terminal, honest) and the run failed at
  18:20:45Z; the operation is pinned `executing` forever.

## Root cause

`freezeCapsuleEffectBundle` requires

```go
headBefore.PendingTransitionRef == "" &&
headBefore.CanonicalEventHead == operation.BaseHead
```

`operation.BaseHead` is pinned at operation `Start` to the trajectory event's
`PreviousHead` — i.e. the head *before* `trajectory_started`. Two independent
mechanisms then guarantee `CanonicalEventHead != BaseHead` by the time any
desk cell runs:

1. The operation's own `trajectory_started` event appends on top of the pin
   (the pin is `PreviousHead`, not the event itself).
2. `projection_batch_recorded` events land continuously — every
   projection finalize unconditionally CASes `canonical_event_head`
   (`internal/store/computer_events.go:171`). The probe tape carried 298
   projection batches in 30 minutes, ~1 per 6 seconds.

On a virgin computer the pin is `genesis_imported` (seq 1); the tape head is
seq 300 by the time the desk finishes. **No self-development freeze can ever
pass on the current architecture** — the invariant predates both the
projection stream (5ae5b610, 2026-07-19) and document-cast operations.

## The correct invariant

The freeze needs "no competing state transition since the pin," not "no
events since the pin." The reducer
(`internal/computerevent/reducer.go:81-146`) moves the state surface only on
`effect_accepted`, `materialization_*`, `rollback_*`, `researcher_update`,
and genesis — `DesiredEventHead`, `EffectiveEventHead`,
`DesiredStateCommitment`, `EffectiveStateCommitment`, `PendingTransitionRef`.
Every other kind (projection batches, trajectory lifecycle, tool/message
events) is a pass-through over those fields.

The operation row already persists the pin's surface: `selfdev.Start`
(`internal/selfdev/operations.go:226`) stores `DesiredHead =
head.DesiredEventHead` and `EffectiveHead = head.EffectiveEventHead` at
creation. So the freeze check needs no new storage — compare the live head's
projection fields against the operation's pinned fields and require
`PendingTransitionRef == ""`.

## Scope of the fix

`internal/agentcore/tools_capsule.go:262`: replace the `CanonicalEventHead`
equality with projection-surface equality against the operation pin:

- `headBefore.PendingTransitionRef != ""` — refuse (a competing transition
  is in flight); unchanged.
- `operation.DesiredHead == "" || operation.EffectiveHead == ""` — refuse
  (defensive: pin lacks the surface).
- `headBefore.DesiredEventHead != operation.DesiredHead ||
  headBefore.EffectiveEventHead != operation.EffectiveHead` — refuse (a
  competing accepted/rollback/research transition landed since the pin).

`record.BaseEventHead = operation.BaseHead` stays — the pin is the audit
anchor, and `selfDevelopmentBundleMatchesOperation` (materializer:137)
re-binds the bundle to the same pin. The materializer's apply leg binds
`AcceptedEventHead` to `operation.DecisionEvent`, not the canonical head, so
no downstream CAS needs to change.

## Failure signature

- Desk cell: `reduce: persist tray-1: self-development base head
  unavailable, stale, or pending` while the operation is `executing` and the
  assignment is otherwise healthy.
- Probe shape: run terminalizes `failed` with a `blocked` report whose
  summary names the freeze precondition; tape shows only
  `trajectory_started` + `projection_batch_recorded`.

## What proves closure

`scripts/m11_selfdev_episode_probe.mjs` on staging: the desk cell calling
`choir.Freeze` on an `executing` operation succeeds (or fails with a
*next* distinct cause) after projection batches have advanced the canonical
head past `BaseHead`. A unit test replays the invariant: head advanced by
projection batches + the op's own trajectory event → freeze passes; head
advanced by a competing `effect_accepted` → freeze refuses.

## Second defect in the same leg (2026-09-27, deployed `1d60d2ad`)

Probe `M11_SELFDEV_EPISODE_1790537156070`, computer
`computer-8835bbd1b770ca8e61644b80403bdea3`, op
`selfdev-95d58528e935481c5daefe059e3cbd26`: with the head gate fixed, the
desk's `choir.Freeze` reached the bundle build and failed on
`capsule release contains no frozen runtime artifacts`
(`capsule/executor.go:1431` — `StageGrantedRelease` requires
`var/lib/artifact/release/` carrying executable `bin/autoputer` plus
`frontend/` artifacts; the desk had produced a source-only diff). Every
subsequent cell then returned `assigned capsule obligation is no longer
executable: assignment capsule is not active: %!w(<nil>)`.

Two substrate bugs:

1. **Freeze never thaws.** `ExtractGranted` quiesces the capsule to
   `StateFrozen` (`executor.go:1152`) and `Capsule.Thaw` had zero callers.
   A refused freeze — or a *successful* one — permanently stranded the
   desk: the obligation gate requires `StateActive`, so no retry, no
   `choir.Complete`, and the run ends by completion-guard exhaustion.
2. **`%!w(<nil>)`.** `validateAssignedEngineeringExecution` wrapped a nil
   error when the capsule was non-active but inspectable, hiding the real
   state from the failure string.

Fix (this session): `capsule.Executor.ThawGranted` (role-gated, linux;
stubbed elsewhere) + a deferred best-effort thaw inside
`freezeCapsuleEffectBundle` after `ExtractGranted` succeeds — every
Frozen-dependent read (diff, release stage, snapshot digest, bindings,
receipts) completes before the deferred thaw fires. The terminal fate
path (`fate.go` freeze-intent freeze) is untouched: it re-quiesces on
`StateActive`. The overlay prompt now names the release-tree precondition
so the desk builds `bin/autoputer` + `frontend/` in-capsule before
calling Freeze.

