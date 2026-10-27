# The paper is never checkpointed (2026-10-27)

Status: open; documented before any fix. Station D8 of
docs/definitions/choir-consolidate-and-scale-metamission-2026-10-26.md.
Owner, 2026-10-27: "We need to have auto checkpointing on. We need to
figure out how to make it resource efficient and do it the right way, but
it's an essential feature"; "We need snapshotting and hibernation
everywhere automatically." Investigation: Sonnet, read-only, 19:3x.

## Evidence

- The paper's boot: `projection recovery resume (local=4394600 W=34660
  H=4394600)`; its host projection base sits 4.36M events behind its tape.
- `go-choir-checkpointd` (timer, every minute) logged succeeded jobs today
  for d@e (`checkpoint-verify:<digest>`, from d@e's guest cadence) and b@c
  (`cadence`), none for the paper. The only paper line in 7 days:
  `checkpoint reconcile: enqueue computer-1355b83b…: context canceled`
  (10-27 00:26:50); nothing else in 40 days of journal.
- The service hit `start operation timed out` on 10-26 20:02 and 10-27
  00:13, 00:26, 08:38; the owner's computer 03335285 (595k events) has a
  failing job (seed-witness extraction).

## Why (code; verified unless marked)

- Host cadence has no class or size filter: `ReconcileProjectionJobs`
  (`internal/platform/projection_jobs.go:~285-320`) enqueues every computer
  where `checkpointDue` holds; the paper is due.
- Hypothesis A: the paper's job row is `blocked` or `failed`; a blocked row
  with unchanged inputs returns silently and a failed row waits out a
  backoff up to 16 h, neither logging. Hypothesis B: the job is killed by
  `TimeoutStartSec=10min`, `MemoryMax=6G` or the 5-min lease before it logs;
  a first paper job replays about 4.36M events. The job row
  (`GET /projection_jobs/<id>`) decides.
- The guest-side cadence (`VM_CHECKPOINT_CADENCE_ONLY`, `nix/node-b.nix:1777`,
  `internal/vmmanager/checkpoint_cadence.go`) is on for d@e only and refuses
  `critical_protected` and `public_platform` by design until the live
  extract's append hold is measured on the paper (design
  `apply-checkpoint-without-replay-2026-10-26.md`, R2). On d@e one
  checkpoint took 5.5 s and 118 MB.

## Hypothesis A confirmed (FP0, 2026-10-27, corpusd job read)

The paper's projection-base job row reads `status: blocked`,
`reason: cadence`, `failures: 9`, `updated_at: 2026-10-15T02:42:29Z`,
`target_sequence: 402176`, `watermark_sequence: 34660`,
`cold_tail: 4365298`, `alert: urgent_tail,repeated_failure`, with the error
`projection base refused: installed base does not open: runtime store:
bootstrap: apply schema: Error 1049: database not found: texture`, on the
seed base `abacf6b0…`. The job has been blocked for twelve days; a blocked
row with unchanged inputs never retries and logs nothing, so the alert
reached no one. Next: why the seed base lacks the `texture` database, and
the smallest reseed (design pass running).

## Design pass (Opus, read-only, 2026-10-27)

Verified:
- The seed `abacf6b0…` belongs to the paper alone: sequence 34660,
  vocabulary v2, 462 MB, made 2026-10-13 18:43. Its texture workspace is
  only `.darc` archive files, which the repo's own GC no longer writes
  (archive level 0 since 10-19, `squash.go:251`).
- The engine finds the `texture` database but cannot load it: the error comes
  from `verifyInstalledHead` (`install.go:274-281`) via bootstrap. The
  likely cause is damage, from Dolt bug #11747 or archive-version skew; this
  is a hypothesis until S0 opens it.
- Turning on the guest cadence today would not help: `checkpointBaseSeed`
  (`checkpoint_verdicts.go:174-177`) hands the host the same broken base to
  verify against.
- The guest bind runs a full `SELECT *` under the append hold
  (`checkpoint_restore_bindings.go:87-135`, `dolt_state_extractor.go:389`).
  That is 2.2 s on d@e and minutes on the paper, so it cannot be the paper's
  checkpoint as built.
- A host genesis rebuild runs about 42 to 50 events/s, which is 24 to 29
  hours for 4.39M events. The 10-minute service run, the up-to-16 h failure
  backoff and the scratch prune on re-enqueue keep it from finishing.
- Every publish writes a full tar (`publisher.go:63-127`). At the paper's
  size that would be terabytes a day.
- Periodic memory snapshots of a busy computer buy nothing safe: restoring
  one after a crash resumes work, which the restart rule forbids. A snapshot
  belongs only to a planned stop.

Slices, in order:
- S0, operator read: open the seed with the current binary in checkpointd
  scratch, to confirm the cause.
- S1, orange, tier 1:
  - quarantine a base refused twice and fall back to an older one;
  - re-admit a blocked row when the build changes;
  - log blocked rows hourly;
  - give checkpointd a per-job deadline below its service timeout.
- S2, red, tier 2, owner decision: a host capture as the paper's first base.
  - Hold the guest's appends, pause it, reflink its data image, and resume.
  - On the host, at low priority, compact and check that the head equals H.
  - Publish the result typed `live_captured`.
  - From there, incremental replay at about 50 events/s keeps up with the
    paper's 7 to 12.
- S3, red, tier 2: witness v2, a hash-only bind (per-table Dolt root hashes)
  in milliseconds. Lift the class refusal; cadence for every computer.
- S4, red, tier 2: incremental base publication (content-addressed table
  files plus journal segments).
- S5, red, tier 2: a busy hibernate on planned host stops (hold, pause,
  snapshot, stop; on wake, resume).
- S6, orange, tier 1: diff snapshots for every non-protected computer.

## Consequences

Any rebuild, rematerialize or fresh realization of the paper replays from
event 34660, much of it V1-era tape (the vocabulary migration then runs in
full). Hibernation and snapshots for the paper (D3, D8) have no recent base
to stand on.

## Fix direction (not chosen)

1. Read the paper's job row (A or B).
2. Seed the paper's base near its head once: a one-time checkpointd run with
   raised limits in a window with disk headroom (the data image is at 93%),
   or the guest cadence for the paper after measuring the append hold on
   d@e.
3. Then the ordinary cadence keeps it current; the same policy for every
   computer, with hibernation and snapshots on top (D2, D3).
