# A dated switch gate, and two tests with a clock in them, turned main red at midnight UTC

2026-11-19. Recorded with the repair.

## Problem

CI run 44426306206 on `23c09aaa8` (searching for need, pushed 00:06Z) failed
in three jobs. None of the three failures is in that change. Until they are
fixed every CI run on main fails and nothing deploys.

## Evidence

Read from the run's log.

1. **Vocabulary Gates, "switch-registry lint".** Five lines of the form
   `FAIL: calibration_e1_restate: grandfather date 2026-11-18 has passed and
   calibration_e1_restate is still off for every computer; turn it on at R0
   or delete it`, for `calibration_e1_restate`, `_pull`, `_stub`,
   `_two_stage` and `_baseline`. The date is a field of each entry in
   `switches/registry.yaml`; `scripts/check-switch-registry` fails once it
   has passed. It passed at 00:00Z.
2. **`TestR4_TheDayIsTheListenersDay`** (`internal/agentcore`, added
   2026-11-18 with the heard index), at 00:08Z: `(R4) no-zone rows
   ["recent: 3 days ago | Quarry fire | …"]`, want "2 days ago". The test
   seeds a record 49 hours before now and the code counts days by UTC dates
   (`dayPhrase`, as its comment says). 49 hours before 00:08Z is three UTC
   dates back. The test fails in the first hour of every UTC day.
3. **`TestDeskSessionWorkerSelfExitRecordsItsOwnCause`**
   (`internal/yaegikernel`): `spawn: desk session worker ready: frame: read
   header: EOF (… fatal error: test crash)`. The test's helper process
   crashes itself 200 ms after it starts, to prove a cell reports its
   worker's death. Under the race detector on a loaded runner the worker was
   not ready within 200 ms, so the spawn failed instead of the cell.

Why the landing did not see them: `scripts/land` ran its gates at 23:5xZ on
2026-11-18, before the date; and on the lead's Mac (UTC−5)
`scripts/check-switch-registry` used the local date, so it still passed
after midnight UTC, when CI did not.

## Belief

- The first is a gate doing what it was built to do, at a time nobody was
  watching. The five switches are the arms of calibration experiment E1
  (`docs/design/calibration-experiment-e1-2026-11-04.md`). Their entries say
  "E1 starts by the owner's act". It was never started, and in two weeks
  nobody walked the date. The gate's two answers are "turn it on at R0" and
  "delete it". Neither is the lead's to choose at midnight: starting an
  experiment is the owner's act by the entry's own words, the arms are
  alternatives that cannot all be on for one computer, and deleting E1 is a
  red change to `rlm_reduce.go` that throws away a design the owner set up.
- The second and third are tests that read the clock and assumed a quiet
  one.

## What was done

- The five entries' `grandfathered_off_until` moved from 2026-11-18 to
  2026-11-25, which is each entry's own `expires` date, and each entry's
  `hold` says so and says it is not to be moved again. **This walks past the
  gate once.** It is the conservative unblock, not the answer.
- `scripts/check-switch-registry` takes today's date in UTC, so a run on a
  developer's machine and a CI run of the same minute agree.
- `TestR4_TheDayIsTheListenersDay` seeds by UTC date (noon, two and three
  days back).
- `TestDeskSessionWorkerSelfExitRecordsItsOwnCause` gives its worker three
  seconds before the crash.

## Remaining error

- **The owner's decision, due before 2026-11-25:** start E1 or delete it.
  On 2026-11-25 nine switch entries reach a walk-by date (these five and four
  others) and 52 entries already print an expiry warning that nothing reads.
  Without a decision main goes red again that midnight.
- A dated gate fails at a time, not at a change, so no landing check can
  catch it. What would: the hourly status printing any switch date within
  three days. Not built.
- The crash timer in the worker test still starts at process start; three
  seconds is a margin, not a fix. Starting it at the first cell would need
  the helper to see the cell.
- Other tests may read the clock near a boundary. Not searched for.
