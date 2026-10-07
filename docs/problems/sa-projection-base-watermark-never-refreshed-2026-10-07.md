# SA: projection base watermark never refreshed — recovery tail grows unbounded

**Status:** confirmed on staging, reproducible. Mutation class: orange → red
(recovery path). File under SA slice 0.

## Found

2026-10-07. Owner VM `candidate-fleet-e15cb89f25d963c220319b7b`
(`computer-03335285269bdba4f94377e56879f9e6`) entered a boot loop:

```
autoputer: required projection base refused: refusing genesis fallback:
recovery tail 423720 events exceeds 10000;
publish a fresher base (local=0 W=148431 H=572151)
```

## Root cause

`computer_replay_watermarks` is written only by
`POST /internal/computers/files/watermark` (`HandleFileCASWatermark` →
`RecordReplayWatermark`). The sole caller is the manual `choir-rebuild-base`
CLI via `projectionbase.AdvertiseWatermark`. There is no scheduled job,
timer, or trigger anywhere in `corpusd`, `vmctl`, or `autoputer` that
re-publishes a fresh projection base as the canonical head advances.

The advertised watermark for this computer was last written when a
`choir-rebuild-base` run published seq 148431. Since then ~423k additional
events have been appended. `MaxRecoveryTailEvents = 10000`
(`internal/projectionbase/recovery_plan.go:11`), so the gap `(W, H]` = 423720
far exceeds the tail cap, and the VM can never boot past
`materializeProjectionBaseIfNeeded` without manual operator intervention.

## Fix shape

One of:

1. **Scheduled watermark refresh in corpusd** — a periodic task that runs
   `projectionbase.Rebuilder` (or equivalent) for each active computer and
   calls `RecordReplayWatermark` when the new base is published.
   Cadence: every N events (e.g. every 5000) or every T hours, whichever
   comes first.

2. **Incremental base extension** — rather than full replay, extend the
   last published base by appending only the tail events since the last
   watermark. This requires the rebuilder to accept a seed store instead of
   always starting from scratch.

3. **Watermark-aware event appender gate** — after each append batch, if
   `(canonical_head_seq - watermark_seq) > threshold`, trigger async base
   rebuild. This ties watermark freshness to write volume.

## Immediate remediation (this incident)

`choir-rebuild-base` patched to add `--source http` so it uses
`/internal/computers/events/replay` (which returns correct `TransitionInput`
via `replayTransitionInput` in `event_replay.go`) instead of
`DiskEventSource` (which fabricates `TargetStateCommitment` from
`event.ResultingEffectiveCommitment` — empty for `effect_accepted` events,
causing "accepted effect is not fully bound" at seq 474642).

Run command:
```
choir-rebuild-base \
  --computer computer-03335285269bdba4f94377e56879f9e6 \
  --target-head 5fdd38ccd576442923d7ec07cc10f0fe15d6fa84956f7ee06449fff7a731bf40 \
  --source http \
  --owner 5bd6de97-3b58-408c-bf89-c42c81b083de \
  --platform-url http://127.0.0.1:8086 \
  --artifacts-root /var/lib/go-choir/platform-artifacts \
  --key-hex <guest-privacy-key> \
  --advertise \
  --capability <bearer-or-internal>
```

Guest privacy key extracted from `data.img` at
`/choir-credentials/privacy-key` (ext4, inode 2614).

## Secondary defect — DiskEventSource cannot replay effect_accepted events

`DiskEventSource.EventsPage` (internal/projectionbase/source.go:133)
fabricates `TransitionInput.TargetStateCommitment` as
`event.ResultingEffectiveCommitment`, which is empty on `effect_accepted`
events. The reducer rejects it ("accepted effect is not fully bound").

The correct commitment for `effect_accepted` is
`next.DesiredStateCommitment` — available only from the stored
`event_head_receipt_json` in `computer_event_append_receipts`, not from the
event bytes or the payload. Self-dev commitments additionally require
`operation.BaseHead` which lives only in the guest store.

**The `--source http` flag added to `choir-rebuild-base` sidesteps this by
reading `/internal/computers/events/replay` (corpusd has the receipts).**
`DiskEventSource` remains usable only for chains with no `effect_accepted`
events — a durable fix requires either receipts in the artifact store or a
seeded scratch store from the guest's data.img.


## Rollback

None — record only. The deployed repair path (manual `bootstrap-chain`
POST) is available to any holder of `computer:lifecycle` scoped to the
computer. The watermark POST is idempotent (monotonic insert).
