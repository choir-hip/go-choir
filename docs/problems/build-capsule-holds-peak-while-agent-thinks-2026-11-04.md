# A build capsule holds a build's peak of host memory while its agent thinks (2026-11-04)

Status: open. Mutation class of this doc: green. Owner ruling,
2026-11-04 ~01:00Z: "no stopping until we eradicate the waiting except for
true resource contention. and even then, that should be semantically
exposed to eng, mgmt, and texture."

## Problem

A build-class capsule reserves `BuildPeakBytes` (3 GiB) on vmctl's host
memory ledger from its spawn to its destroy (`Executor.Spawn` →
`HostMemory.AdmitBuild`; `ReleaseBuild` at destroy). Most of a capsule's
life is its agent thinking, often 20 to 40 minutes, with no job running and
nothing growing into the reservation. The ledger still charges the whole
3 GiB as `outstanding = idle + Σ builds − rss` and every other start and
build spawn is admitted against it.

On Node B (31.8 GiB, floor about 4.8 GiB, burst pool 1 GiB) that caps
concurrent self-development arms at about three, whatever they are doing.

## Evidence

Node B vmctl journal, metadata only (2026-11-03, host clock UTC):

- `refuse_build` decisions every 30 s for the same arm from 22:23 to 22:40
  and again at 23:23 to 23:24; 58 refusals in ten hours.
- 23:23:30: `refuse_build need_mib=3398 available_mib=14747
  reserved_mib=6767 floor_mib=4779 headroom_mib=1024`. The host had
  14.7 GiB available, and 6.7 GiB of it was promised to live build capsules
  that had not grown into it. The spawn waited for memory that sat free.
- The orchestrator's arms waited 6 to 22 minutes at spawn that night.

## Cause

The reservation's unit is the capsule, but the memory is used by jobs
(a frontend build, a Go link, choir-shot, tests). A capsule's idle
processes, its broker and shell, use tens of MiB
(docs/reports/playwright-in-capsules-memory-2026-10-15.md: a capsule cgroup
at rest held 43 to 95 MiB).

Fix 2 of
docs/problems/capsule-reservation-idle-baseline-and-job-double-gate-2026-11-03.md
(the job's own-reservation credit, 67f9ec664) patched the job-side double
gate that this unit mismatch creates; it does not shorten the time memory
is held.

## Direction

A host reservation exists only while memory can be used:

- a build capsule's spawn reserves only its baseline, the memory its idle
  processes use, re-measured from its cgroup while no job runs;
- a job takes a lease for its need when it starts, synchronously admitted
  by vmctl under the same ledger lock as every start and spawn, and gives it
  back when it ends; the capsule's cgroup memory.max is held to its baseline
  usage plus the lease while the job runs;
- a refused lease waits with its reason as data (need, room, holders), so
  "waiting for host memory" happens only when jobs actually contend.

The credit of fix 2 becomes unnecessary: a job's admission is its lease.

## Residual risk

Growth outside a job (a `choir.Exec` command, a Claude delegate's own
growth) is unleased, as other computers' growth always was: it is bounded
by the capsule's cgroup limit, the host floor, PSI and pressure reclaim,
and the next heartbeat re-measures it into the capsule's baseline.
