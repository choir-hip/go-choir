# Cut 1 breakage log

One log per cut window (docs/re-engineering-decisions-2026-10-12.md §12.4):
problems found and fixed in the same commit during Cut 1 of
`docs/definitions/choir-reeng-cut1-authority-2026-10-12.md`. Problems that
are not fixed in the commit that finds them get their own problem doc.

## B1. A crash boot starts a new research run that takes a pre-boot Cast

Found 2026-10-13 by the Cut 0 tracer, run `c0-fast-1` (build 287734c5,
bundle sha256 2306081c…, trajectory 2b5b9631). Invariant I3 failed with ten
post-boot dispatches; the first two are the defect.

Trace, from the bundle's lifecycle events and runs:

1. 05:11:27 the owner restart route restarts the computer while research run
   3c3ea752 is in a cell; boot at 05:11:34 has no planned-restart marker.
2. Boot passivation (`passivateInterruptedActivations`) parks 3c3ea752 and,
   for it, opens a spawned work item (lifecycle event seq 10,
   `lifecycle-open-spawned-work:3c3ea752…`).
3. That work item's wake is minted after the boot, so the actor handler's
   crash-boot refusal (`preBootWorkOccurrence`, which only refuses
   occurrences recorded before the boot) lets it through.
4. A new research run 2127efc8 starts at 05:11:35 and takes Texture's second
   pre-boot Cast, control packet 93b04b27 (seq 11,
   `bind-control-delivery:2127efc8…:93b04b27…`). It runs until 05:19:42 with no
   owner input.

So a crash restart resumed work, against the owner rule (CLAUDE.md,
"Restarts End Work"). The accept-1 bundle shows the same first link (a
spawned work item opened one second after boot for a pre-boot Texture run).

Fix (landing L1, same commit): a markerless boot no longer passivates. It
cancels every pre-boot pending, running, parked or blocked run with fate
`interrupted_by_restart`, opens no work, and wakes no one
(`closePreBootRunsAfterCrash`; Engineering cancel wakes and the boot
Engineering recast are suppressed on a crash boot). Planned boots keep the
park-and-resume path. Pre-boot pending control packets stay pending until
the owner's next act; Cut 1 landing L3 deletes packets.
Test: `TestCrashBootClosesPreBootRunsWithFate` (fails on the old boot:
runs passivated, parked and blocked runs left open).

## B2. A crash boot misses an open run behind 1,024 newer runs

Found 2026-10-13 by the convergent plan review (codex), confirmed by test.
Boot lists the owner's runs with `ListRunsByOwnerStates(..., 1024)`, and
that list applies its limit to run headers before it filters states
(internal/store/graph_store.go:1039, :1057). An old pending, running,
parked or blocked run behind more than 1,024 newer finished runs is never
seen, so L1's claim "every pre-boot run is closed" fails on any computer
with a long history (the owner's computer takes every cut).

Fix (same commit): boot's owner-scoped list scans every run header
(`preBootRunHeaderScan`; headers carry no bodies and only matching runs
are loaded). Test: `TestCrashBootClosesOpenRunBehindManyFinishedRuns`
(one old running run behind 1,030 finished runs; failed with the run
still running before the fix).
