---
date: 2026-11-27
title: the gates no longer hand the lander's stage directory to the checks
class: yellow
---
What: `scripts/gates` unsets `LAND_QUEUE_STAGE`, `LAND_STAGE_DIR`, `LAND_RESULT_FILE` and `LAND_QUEUE_LANE` before it runs any check, and `scripts/selftest-land-timings` unsets them too. The gates header names `GATES_STALL`.

Why: the first real batch through `scripts/land-queue` (two lanes, 2026-11-27T13:35Z) landed nothing. Every attempt failed at `scripts/selftest-land-timings`: the queue's own stage variable reached the selftest, which starts the queue, and the stage-ownership check added the same day refused a stage that was not its own. A plain `scripts/land` run never sets that variable, so nine landings had passed.

Proof: `LAND_QUEUE_STAGE=/tmp/zz-none LAND_STAGE_DIR=/tmp/zz-none scripts/selftest-land-timings` printed one FAIL before and `all passed` after. The queue behaved as designed on the failure: three attempts, both lanes set aside with the failing step, nothing pushed, its own tree removed, exit 1. Not proved until a batch lands: ledger claim C257.

Replaced: checks inheriting the lander's private variables.

Undo: revert this commit; a batch then fails at that selftest again.
