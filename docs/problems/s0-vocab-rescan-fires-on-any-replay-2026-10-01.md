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

## 2026-10-16: the head-position bug is fixed; a one-row replay still pays the full rescan

The predicate is now rows applied this boot
(`appender.ReplaySnapshot().AppliedRows > 0`, `internal/autoputer/run.go`).
The cost of a boot that applies even one row is unchanged.

Evidence (the paper, `computer-1355b83b…`, 2026-10-16):

| Boot | Tail | Server start → indexes ensured (vocabulary pass behind the gate) |
|---|---|---|
| 16:29:59 (b8feaa147 refresh) | 1 | 16:30:00 → 16:33:44: **3 min 44 s** |
| 16:45:16 (rerun refresh) | 0 | 16:45:16 → 16:45:16: under 1 s |

Two of the paper's twelve boots on 2026-10-16 had a tail (12:49, 16:29).
Each costs about 4 minutes of paper downtime per deploy, and the
replaying boot is the one that ran into the vocabulary refusals
(`paper-boot-refused-vocabulary-migration-capture-label-override-2026-10-16.md`).
The deploy refresh then outlived vmctl's 120 s write timeout
(`deploy-refresh-empty-reply-vmctl-write-timeout-2026-10-16.md`).

**Why the pass is whole-table.** `MigrateAndFenceServingVocabulary`
plans over every `og_objects` and `og_edges` row and keeps every row that
holds an `obj:`/`edge:` reference. It then checks each kept leaf against
every key of the rename map, for up to 8 fixpoint passes, and runs the
serving fence over every row. None of this depends on how many rows
replay wrote.

**A second gap, found reading the code.** The fast path trusts
`FencedAt` and this process's applied-row count. A replay can span
processes: the 30-minute resume quantum, or a crash after a durable
replay checkpoint. Rows the earlier process replayed are then never
checked if the later process applies none. Nothing durable records that
a replay opened after the fence.

**Fix direction (red: boot vocabulary fence).** Check exactly the rows
replay wrote, and keep the full rescan as the fallback:

- On a fenced store, the first replayed deposit in a process durably
  marks the report "replay open" before it commits.
- The replay path records the object and edge IDs it writes.
- At the boot fence, on a fenced store whose open replay belongs to
  this process, re-read those rows. If no leaf migrates, no reference
  hits the rename map (report renames plus deposit-upcast aliases), no
  edge endpoint is renamed, and the role fields pass the serving fence,
  then the plan's fixpoint is unchanged. Stamp the fence and clear the
  marker.
- Otherwise, including a marker left by another process, run the full
  pass.

Rows written live since the fence are trusted exactly as on today's
zero-replay fast path; the narrowed path neither widens nor narrows that
trust.
