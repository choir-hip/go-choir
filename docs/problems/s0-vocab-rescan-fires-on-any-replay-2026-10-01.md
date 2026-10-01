# S0 finding: vocabulary rescan fires whenever a replay applies any rows

Date: 2026-10-01
Discovered by: S0a boundary panel (claude + omp-gemini38 + omp-glm53-flash,
independently) on receipts s0a-boot-timeline-owner-sized(-post-refresh)-2026-10-01.json
Station receiving it: S3 (boot cost) — this is the dominant owner-sized boot cost

## Evidence

`internal/autoputer/run.go` computes

```go
replayed := appender.ReplaySnapshot().Sequence > 0
```

`ReplaySnapshot().Sequence` is the **replay head position** — the last
sequence the local projection reached — not a count of events applied
this boot. `internal/computerevent/appender.go` sets it in
`setReplayProgress` on every applied event (`apply` →
`setReplayProgress(record.Request.Event.Sequence, …)`). On a tape that is
240,542 events long, applying even one new deferral row during a refresh
boot pushes `Sequence` to 240542.

The predicate then feeds
`db.MigrateAndFenceServingVocabulary(bootstrapCtx, replayed, gate.tick)`,
whose fast path (`internal/store/vocab_migrate.go`, `if !replayed && prior
!= nil && prior.FencedAt != "" { … return prior, nil }`) only runs when
`replayed` is false. So **any** boot that applied a single new row —
which is nearly every refresh on a computer that gets deferral churn —
takes the full-table rescan across all 240k events.

Staged measurements (same owner computer, same data.img):

- epoch 994, zero rows applied → `replayed=false` → 9.9s to healthy.
- epoch 995, handful of rows applied → `replayed=true` → 662.7s to
  healthy. The 653s gap between `replay_begin` and `replay_done` is the
  vocabulary rescan, not event apply.

## Boundary

This is not the deploy's fault and not S0's instrument; it's a pre-existing
defect in the replay→vocab handoff the instrument surfaced. The honest
predicate is *events applied this boot* — `RecordApplied` already fires
per applied row — not the head position.

## Not a fix here

Documented per problem-documentation-first. The fix changes the fence
gate (vocabulary migration is a red surface); S0a fixes it as part of the
instrument loop and records the receipt evidence showing the fast path
taken when zero rows are applied.
