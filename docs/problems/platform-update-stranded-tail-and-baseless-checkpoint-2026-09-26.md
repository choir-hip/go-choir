# Platform-update tail: stranded after `applied`, and baseless computers cannot mint checkpoints

Date: 2026-09-26
Mission: choir-platform-update-push-restore-2026-09-26 (M9a)
Discovered by: deployed probe `scripts/m9a_platform_update_probe.mjs` run 15
(computer-bb375349f9b8d0e4ec67e4c21712b233, update
upd-m9a-platform-update-1790451579800-a)
Mutation class: red (canonical event surface, checkpoint authority,
route-projection tail)

## Evidence

Guest tape after the run (corpusd `events/replay`):

```text
1 genesis_imported          lifecycle-bootstrap-chain:...
2 effect_accepted           platform-update-accepted-...
3 materialization_started   platform-update-started-...
4 materialization_applied   platform-update-applied-...
```

No `checkpoint_published`, no `route_projection_updated`, no
`materialization_failed`. Route slot stayed absent (`route_absent: true`,
generation 0). Guest log:

```text
19:40:01 runtime: platform update resume: re-driving update upd-...-a after guest restart
19:40:02 runtime: platform update resume: self-development checkpoint:
  replay completeness: projection base refused: no advertised base
```

## Problem 1 — the tail is a dead zone

`ApplyPlatformUpdate` commits `materialization_applied` and immediately clears
`PendingTransitionRef` (correct transition semantics: pending tracks
materialization only). After that point:

- `resumePendingPlatformUpdate` returns early — its predicate is
  `PendingTransitionRef != ""`. Nothing re-drives the tail.
- A re-pushed identical offer hits the `applied`-exists early return
  (`Replayed`), which returns before the checkpoint/route section.
- Tail errors return bare `err` — no `materialization_failed` event, no
  residual marker. The tape says "applied" while the served route is the old
  version, and no retry can ever reach the tail.

Result: any crash or error in checkpoint mint / checkpoint event / route
projection / route event permanently strands the update. This is the
applied-but-unpromoted class the deploy-time resume fixed for the
pending-bound half of the update; the post-applied half is uncovered.

## Problem 2 — fresh computers cannot mint checkpoints at all

`checkpointRestoreBindings` runs `ReplayCompleteness`, which calls
`openProbeReplayStore` → `resolveRecoveryTarget` → `src.Watermark`. No
advertised base → `ErrBaseRefused` → checkpoint mint fails → the platform
update tail dies.

The only base publisher is `cmd/choir-rebuild-base`, an offline host tool run
by ops (see
`docs/evidence/choir-rlm-restore-zero-deployed-proof-2026-09-09.md`: only
computer-03335285269bdba4f94377e56879f9e6 has a watermark, seq 148431).
Freshly provisioned computers — including every probe computer the deployed
proof registers — have no base, so checkpoint, restore, and platform update
all refuse. `Rematerialize` has the same hard requirement via
`installStagedBase`.

The fail-closed design intent ("no lifetime-replaying the prefix") is about
*unbounded* replay. A fresh computer's chain is tens of events; replaying it
in full is cheap and *stronger* evidence than a base install — it verifies
the whole chain reproduces live state rather than trusting a base snapshot.

## Fix direction

- Tail: make the post-applied section resumable. `applied`-exists no longer
  short-circuits to `Replayed` unless the route event committed; otherwise
  the tail re-drives with inputs recovered from the tape (applied event
  payload carries `ApplyResult`; its `DecisionRef` is the accepted digest).
  Sweep predicate extends to "latest `platform-update-applied-*` event
  without a matching `platform-update-route-projection-updated-*`".
- Checkpoint recovery without re-mint: verifier certificates embed server
  time, so re-minting under the same idempotency key conflicts. Fetch by
  idempotency key instead (`GET /internal/computers/checkpoints`), falling
  back to mint only when no checkpoint exists.
- Baseless bounded replay: when no base is advertised, replay the full tape
  from genesis into the staged/probe store, bounded by
  `MaxRecoveryTailEvents` — the existing cap that "publication must keep W
  close to H". Identical refusal posture for long chains; fresh computers
  gain checkpoint/restore/update.
