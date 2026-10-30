# S2-e: layering exec pointer is a global file outside the release dir — rollback does not revert exec

**Date:** 2026-10-04
**Status:** fixed by this commit series (see Resolution).
**Mutation class of this record:** green (problem documentation only).
**Station receiving it:** S2 layering-runtime-from-release, slice S2-e
(rollback atomicity).

## Evidence

Source-traced during the 2026-10-05 director review of the vm-3dc68688
crash-loop:

- `internal/updater/closure.go` `materializeReleaseClosure` wrote the
  release's exec pointer at `$CHOIR_UPDATER_ROOT/layering-entrypoint` — a
  single global file outside `releases/<digest>` and outside the `current/`
  swap (former lines 418, 457-462).
- `nix/autoputer-vm.nix` `autoputerRuntimeExec` read that global file to
  choose the release binary.
- `internal/updater/updater.go` `restorePrior` swaps `current` back to the
  prior release but never touched the global entrypoint.

Consequence (two single-state-authority violations):

1. A health-failing layered apply restored `current/` to the predecessor's
   release dir, but the next `go-choir-autoputer.service` start re-exec'd
   the *failed* release's binary — the likely cause of the observed silent
   crash-loop on staging (recovery swap published, exec unchanged).
2. A plain file release applied over a layered one left the stale global
   entrypoint in place unless the next apply happened to clear it.

## Resolution (this commit)

- `stageRelease` writes `releases/<digest>/layering-entrypoint` before the
  release tree is sealed read-only; `materializeReleaseClosure` verifies the
  declared entrypoint materialized under the private store and matches the
  staged pointer instead of writing a global file.
- The wrapper reads `$CHOIR_UPDATER_ROOT/current/layering-entrypoint`, so
  pointer swap and exec selection are one authority; `restorePrior` reverts
  exec with the same swap.
- `ensureReleaseEntrypoint` backfills pre-S2-e staged dirs on
  `restorePrior`, `RestagePinnedRelease`, and replayed applies; an
  unresolvable entrypoint degrades to base exec with a logged cause.
- The legacy `$CHOIR_UPDATER_ROOT/layering-entrypoint` file is deleted on
  every apply so it cannot resurrect stale exec state.
- Boot-loop guard in `autoputerRuntimeExec`: ≥3 starts of the same release
  dir within 60s refuses that release's exec, writes a host-readable receipt
  under `$CHOIR_UPDATER_ROOT/bootguard/<key>.tripped`, and serves the base
  runtime with `CHOIR_LAYERING_BOOTGUARD` set; `/health` then reports
  `status=layering_bootguard` at 503 so the apply probe fails closed
  (recovery path) and hosts see the trip.
