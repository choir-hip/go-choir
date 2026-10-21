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

## After (agents VM, same method)

- Idle worker RSS: **20,128 kB (19.7 MiB)** with `cmd/desk-worker`
  built `CGO_ENABLED=0 -trimpath -ldflags='-s -w'`, against 82,752 kB: 61.1
  MiB less per worker (the checklist estimate was 40-60). A real cell
  (`print(40+2)`) evaluates in it.
- Worker binary: 20,426,914 bytes against 188,551,216.
- Cause of the size: `internal/gomemlimit` imported Dolt's memlimit (cgo,
  ICU), so any binary linking it carried the Dolt engine. `ApplyDaemon` now
  takes Dolt's `Init`/`NodeCacheSize` from its caller
  (`internal/autoputer/run.go`); the worker's dependency list has no Dolt.
- Reaper: `armReaperLocked` arms one timer (at `done`) for the earliest idle
  session's retention deadline; `reap` closes idle sessions past retention
  and re-arms. A busy session is never watched or reaped; `closeAll` stops
  the timer. Tests: `tools_desk_reaper_test.go` (fake clock and timer; run
  also under `-race`).

## FP0 steps (not done here)

1. Packaging is done in flake.nix (commit "the autoputer package carries the
   desk-worker binary beside it"), unverified by a nix build here (no nix in
   the agents VM). FP0 to confirm: the nix build of `desk-worker` does not
   link ICU (`CGO_ENABLED=0` if it does), and what the layered-release path
   (`nix/autoputer-vm.nix`, finds no sibling) does to the saving.
2. Measure on the paper computer: `VmRSS` of the `desk-session` children
   before and after, and that an idle computer holds 0 workers 30 minutes
   after its last cell (trace: pool count).

## now

Ported onto FP2 (lane/res17-on-fp2). `internal/deskworker` Run() does FP2
startup: gomemlimit.ApplyDeskWorker, ConfineDeskSessionWorker (exit 3 on
error), RunDeskCellSelfTestIfRequested, then serving; cmd/desk-worker and
`autoputer desk-session` both go through it; deskworker_test.go fails if it
serves unconfined. `release` stops the reaper timer when no sessions remain.
Packaging landed. Remaining: FP0 steps 1-2 above.
