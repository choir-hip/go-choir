---
definition_version: 4
readiness: executable
# Authority to execute: the launch first pass metamission, station FP10
# (docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md), item
# RES-17 of docs/security-density-architecture-checklist-2026-10-21.md.
---

# FP10 / RES-17: desk-worker idle reaper and a smaller worker binary

Mutation class: orange (runtime behavior of the desk worker pool; no
provider routing, no VM lifecycle). Rollback: revert the commits; the
reaper is a timer on the pool and the dedicated binary is optional (the
daemon falls back to re-executing itself when `desk-worker` is absent).

## The problem

`reclaimLocked` (internal/agentcore/tools_desk.go) runs only inside
`acquire` and `done`. A computer that goes quiet keeps up to
`deskSessionMaxIdle` (12) idle workers alive past their 30 minute
retention until the next cell runs anywhere
(docs/learn/02-desk-runtime.md 4.3). Each worker is the whole autoputer
binary re-executed with `desk-session`, so each idle worker carries the
daemon's Dolt engine and gRPC in its image.

## Metric (value criterion)

1. Idle worker RSS reclaimed: after retention elapses with no cell on the
   computer, the pool holds 0 idle workers without any acquire or done.
2. Per-worker idle RSS (VmRSS of a freshly spawned, idle worker, read from
   /proc, in the agents VM, Go build with the repo dev shell):
   - **before: 82,752 kB (80.8 MiB)** with the autoputer binary
     (188,551,216 bytes, unstripped dev build).
3. Worker binary size, **before: 188,551,216 bytes** (the autoputer).

## Plan

1. This file, with the before numbers (this commit).
2. Reaper: tests first (idle past retention is reaped with no
   acquire/done; a busy session is never reaped, including one whose cell
   outlasts retention; the reaper stops on closeAll), then the code.
3. Dedicated worker binary `cmd/desk-worker` sharing the entrypoint with
   `autoputer desk-session` (internal/deskworker); built CGO_ENABLED=0.
   The daemon prefers a `desk-worker` next to its own executable.

## now

Station started; before numbers recorded above.
