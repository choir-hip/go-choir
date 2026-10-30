# vmctl pressure reclaim hibernates a computer inside the desk respawn gap

**Status:** DIAGNOSED 2026-09-28 on staging (deployed `30cf14f0`); fix commit
follows per `docs/memo-problem-documentation-first.md`.

## Symptom

M11 episode probe op `selfdev-6fdef55efc151e74c83aa2ed6062b041`,
computer `computer-86baf095dbad2a21c2dcee4af7c4d821`
(VM `vm-1b772524c2c5215a1d6eaa696046253d`, user
`1ad3a622-a75f-4bc8-8460-5f12bf004a0e`): hibernated at 14:34:41 with
`reason=pressure` while the texture desk still owed the document another
`apply_owner_revision` activation.

## Root cause

`guestBusy()` (`internal/vmctl/ownership.go`) reads only
`running_runs > 0` from guest `/health`. The desk's activation cadence
leaves long windows with zero in-flight runs — between a failed/succeeded
desk run and the controller's next dispatch (~45 s on staging, longer
while the desk passivates or while pending mutations wait for the next
reconcile tick). During any such window, pressure reclaim sees an
"eligible" guest and suspends the computer mid-episode.

The busy check introduced for the idle sweep (`6f1417ef`) and extended to
the pressure path (`7a0ec819`) is correct but too narrow: it covers
*executing* work only. A computer with pending desk mutations —
`texture_agent_mutations.state='pending'` — is owed another activation by
the controller and is not idle in any product sense, but `running_runs`
reads 0 between activations.

## Fix shape (separate commit)

- `store.CountPendingAgentMutations(owner, computer)` — scoped count.
- Guest `/health` gains `desk_pending_mutations` from
  `rt.pendingDeskMutations()` (same owner/computer scope as
  `RunningCountByProfile`).
- `guestBusy()` busy predicate becomes
  `running_runs > 0 || desk_pending_mutations > 0`, closing the
  respawn-gap window on both the idle-sweep and pressure-reclaim paths
  (both route through `idleOwnershipCandidates`/`rankPressureCandidates`
  → `guestBusy()`).

## Residuals

- The desk respawn loop itself remains; the busy signal now only prevents
  host lifecycle from racing it. Stale `pending` mutations that can never
  dispatch (no matching open work, terminal trajectory) still pin the
  computer awake — a controller-level terminalization rule is a separate
  fix.
