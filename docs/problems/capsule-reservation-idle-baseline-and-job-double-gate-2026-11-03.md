# A stale idle baseline overcommits the build ledger, and capsule jobs wait twice (2026-11-03)

Status: open. Mutation class of this doc: green. Evidence: a read-only
investigation of Node B at 2026-11-03 23:36Z (vmctl health and journal
metadata, firecracker RSS), and code.

## What is correct

Node B's capsule memory accounting is correct in design. Per VM, a live
build set holds `outstanding = idle + Σ build reservations − rss`, clamped
to what the guest's ceiling can still grow
(`buildReservationSet.outstanding`, `internal/vmctl/capsule_reservations.go`),
and that amount joins the reserved term every start and every build spawn
is admitted against.

## Problem 1: the idle baseline goes stale (a bug)

Evidence: VM `04a4e430…` (pool arm, computer `4d38420c…`) carried a settled
idle RSS of 418 MiB recorded at 11:25Z on an older release. Its real idle on
the current release is about 1.8 GB. Its build was charged on top of 418 MiB,
so it was under-reserved by about 1.4 GB, and the ledger drifted
overcommitted against MemAvailable.

Cause, from the code:

- `idle` is `VMOwnership.MemorySettledRSSBytes`, density D4's settled RSS.
  D4 captures it only when an *admitted* start's settle window ends
  (`admissionTicket.finish` → `captureSettledLocked` → `sampleVMMemory`).
  A deploy refresh (`refreshVMForDesktop` → `RefreshVM`) and the other
  unadmitted boots open no admission reservation, so a refreshed computer
  keeps the settled RSS of its previous release for as long as it stays up.
- `ReportCapsuleReservations` takes that value as `idle` whenever it is
  non-zero, and overwrites the set's `idle` with it on every heartbeat, so
  even a fresh pre-capsule RSS reading is replaced by the stale value.
- Nothing records which realization (boot epoch) a settled reading belongs
  to, or bounds its age.

## Problem 2: a build capsule's jobs are gated twice (owner decision: fix)

A build capsule's spawn is admitted against host memory for its whole
reservation (3 GiB). Each of its jobs then waits until the host facts'
headroom covers the job's whole need (`HostMemoryWaitReasonFor`,
`internal/capsule/capsule_size.go`), even when the need fits inside the
reservation its own capsule was already admitted for. The facts' headroom
excludes other computers' reservations but not this one's, so the second
gate only binds when MemAvailable has fallen below what the ledger
promised, which is the overcommit of problem 1 (or another computer's
unreserved growth). Then the job waits for memory its capsule already
holds.

Owner decision: a job needs host headroom only for
`need − its own capsule's unrealised reservation`, floored at 0. This is
safe only while the ledger is not overcommitted, so problem 1 is fixed in
the same change.

## Remaining risk

Concurrent peaks beyond the admitted reservations (a capsule's growth past
its 3 GiB, other computers' unreserved growth) are still bounded only by the
host floor, the PSI pressure gate a job still reads, and pressure reclaim.
