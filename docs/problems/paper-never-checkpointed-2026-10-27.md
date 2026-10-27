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

## Correction: the seed base opens (FP0, 2026-10-27 21:0x UTC)

S1 (6d7a37d07, deployed 21:02:30) re-admitted the paper's blocked row on
the new build. Generation 22 is `running`. It installed the seed
`abacf6b0…` and is replaying from event 34660, at about 1024 events per
30 s; the replay page fetch alone takes 8 to 9.5 s. So the seed is not
damaged. The 10-15 refusal came from code that has since been fixed, and
the row sat blocked for twelve days only because a blocked row never
retried. The design pass's damage hypothesis is refuted, and S0 is moot.

What follows:
- The replay must cover about 4.38M events, roughly 36 hours at this rate.
  S1's 8-minute job deadline ends each attempt. A failed row re-enqueues
  under a new generation and prunes its scratch, so a host replay of the
  paper never converges. It only burns up to 8 minutes of a core per
  attempt, under the backoff.
- S2, the host capture, stays the route to the paper's first base. The
  fetch time is a residual: about 9 s per 1024 events from the artifacts
  store, investigated separately.

## Owner decision, S2 (2026-10-27 20:43 UTC)

Owner: "Yes, Paper's first checkpoint is a copy of its live desk [disk].
Notice that the auto paper actually doesn't have unique data. It doesn't
have its own file system. So it's really something which can be run in a
very lightweight way. I mean, I could even imagine it running on a
ephemeral sandbox ... not that I want to go that way right now, but yes,
first copy."

S2 is authorized: the paper's first base is a host capture of its live
data image, typed `live_captured`. A second point carries weight: the paper
holds no data that exists nowhere else; everything it has comes from its
sources and its tape. So a captured base being wrong costs a rebuild, not
data loss. That lowers S2's risk class for the paper. The ephemeral-sandbox
idea is recorded, not pursued.

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
