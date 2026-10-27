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
