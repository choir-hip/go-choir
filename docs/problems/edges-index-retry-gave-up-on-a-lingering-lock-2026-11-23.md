# The edges index retry gave up when the timed-out build's session still held the lock

Date: 2026-11-23. Class: orange (runtime behavior, `internal/platform/edges_created_index.go`).

## Problem

`ensureEdgesToKindCreatedIndex` took the build lock with
`pg_try_advisory_lock` and returned nil when it missed ("another process is
building it"). After a build that timed out, the timed-out session can still
be on the server for a moment (its cancel is in flight, or its backend has
not noticed the closed connection) and still holds the session lock. A retry
in that window missed the lock, returned nil, and left the invalid index in
place until the next periodic step.
`TestEdgesIndexBuildTimesOutLeavesAnInvalidIndexAndTheNextTryRepairsIt` failed
intermittently with "after the retry: exists true valid false".

## Evidence

- CI run 45112957973 on bd643bc55, job "Go Test (standard, non-runtime shard
  3)": capture_ready_test.go:701, "after the retry: exists true valid false".
  The retry returned no error, so it did not fail; it did not build.
- Reproduced on the build VM under CPU pressure (8 busy loops, 6 race-built
  test processes at GOMAXPROCS=1, 25 runs each): 12 failures in 150 runs.
- A deterministic test
  (`TestEdgesIndexRetryWaitsForALingeringHolderOfTheBuildLock`: another
  connection holds the lock and lets go after 300 ms, the index is invalid)
  fails before the change with the same message and passes after.
- After the change, the same stress: 0 failures in 150 runs.

## Belief

The test is sound; production had the defect, and the fix is to wait for the
lock (bounded by the call's own deadline) and then read the index state, so a
holder that finished the build leaves nothing to do and a dead holder is
followed by the drop and rebuild. All callers run in the background
(`kickEdgesToKindCreatedIndex`), so waiting holds up no request or loop.

## Remaining error

- A holder that builds for the full 20-minute deadline makes a waiting call
  fail at its own deadline; the periodic step retries.
- Not shown on production Postgres; the proof is the test cluster.
- The held-lock window in production is rarer than in the test (the retry is
  a whole periodic step later), so the defect was low impact there.
