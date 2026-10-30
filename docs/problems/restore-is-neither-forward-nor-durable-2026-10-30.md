# A computer restore is neither a forward state change nor durable (2026-10-30)

Class: green (problem record). Surfaces: checkpoint/route projection,
tape (red). Found by the restore and audit design pass
(`docs/design/computer-history-restore-audit-2026-10-30.md` §1.2 G1).

## Evidence

- [V] `restoreComputer` appends `restore_requested` (the canonical head
  becomes N+1), then `RematerializeFromTape` rebuilds the guest store at the
  target head T and rebinds it (`internal/agentcore/rematerialize.go:383-390`).
- [V] Afterwards the embedded projection head is T while the platform's
  canonical head is N+1. `appendLocked` compares them and refuses every
  append with `ErrNeedsProjectionRepair` when they differ
  (`internal/computerevent/appender.go:733-743`).
- [V] No `restore_applied` event kind exists (`computerevent/event.go:35-62`);
  boot recovery targets the canonical head
  (`internal/autoputer/projection_base.go:115-140`,
  `internal/recoveryplan/recovery_plan.go:81`) with no restore awareness.
- [V] The restore test asserts only the state immediately after restore
  (`internal/agentcore/rematerialize_test.go:447-528`).

## Belief state

- [I] After a restore, the computer cannot append (no owner edit, no desk
  work) until a projection repair runs, and the next boot replays (T, N+1]
  and undoes the restore, including after the planned restart a changed exec
  requests. Not observed on staging.
- Precommitted probe R0k-01 (round 0k): on d@e, restore, make one owner
  edit, restart, read the projection head. p = 0.8 that it reproduces.

## Remaining error

- Unknown whether any production restore has run; if one has, its computer
  is either stuck refusing appends or silently un-restored.
- The fix is the design's `restore_applied` forward event (the state at T,
  plus carried-forward records), not a boot-time patch; it lands with the
  restore design, after this record.
