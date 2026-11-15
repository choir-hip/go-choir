# scripts/land: the runtime shards time out on every landing that reaches them

Date: 2026-11-11. Class: yellow (scripts). Status: fix in the same commit.

## Problem

When a landing changes a package that `internal/agentcore` and
`internal/textureowner` import, but not those two packages themselves,
`scripts/land` runs them on the build VM as
`TOTAL_SHARDS=2 PARALLEL_SHARDS=1 scripts/go-test-runtime-shards`. Both
shards fail with `panic: test timed out after 10m0s` in `internal/agentcore`,
and nothing is pushed.

## Evidence

Every land result on choir-agents that ran this step failed the same way,
both shards at about 600 s in agentcore, and textureowner passing in about
65 s:

- `land-877b86ed9c` (2026-11-10 19:21Z)
- `land-af162cbfc5` (2026-11-11 19:56Z, the web golden-document fix)
- `land-dfcdd62740` (2026-11-11 20:14Z, the same lane rerun)

At the timeout, the tests still running had been running for 48 s
(`TestAddFeedStartsOwnerStampedScheduleWithNoRegistryEntry`) and 4 s
(`TestCancelRunTrajectoryDrainsMoreThanOneActivePage`). No single test hung;
the package ran out of its budget.

## Belief

Two differences from CI, which passes the same tests:

1. The shards get no `-timeout`, so go test's default of 10 minutes applies.
   CI splits the work into 8 shards on separate runners. Two shards, each
   holding half of agentcore and running at the same time on one VM, need
   more than 10 minutes.
2. `QUARANTINE_FILE` is not set. CI keeps the load-sensitive tests in
   `scripts/go-test-scale-tests` out of the shards and runs them alone. The
   landing ran them inside a loaded shard.

## Fix

The shard step sets `QUARANTINE_FILE=scripts/go-test-scale-tests`, passes
`-timeout 30m`, and is followed by a run of the scale tests alone (15 m
timeout, no retry). Rollback: revert.

## Remaining error

The shard run time is not measured. If two shards take more than 30 minutes,
the next step is more shards run one after another, not a longer timeout.
The landings that changed agentcore itself took the race path with a
45-minute timeout and were unaffected.
