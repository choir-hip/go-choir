# The engineer has no clock (2026-11-04)

Status: open. Mutation class of the fix: orange (the engineer's cell
context and one harness line on its tool result; no store, no settlement, no
auth). This note is green.

## Evidence

- live8 trace, 2026-11-04. Every self-development request tells the engineer
  "Freeze by minute 40"; the assignment's activation budget is 60 minutes
  (`provideriface.DefaultActivationBudget`). Three of the last five rounds
  froze at 41 to 45 minutes. The judgment was due inside the 60-minute limit.
- `choir.Context()` (`yaegikernel/choir.go`) returns computer id, activation
  id and slot. The engineer's run context carries no elapsed time. Only job
  results carry `elapsed_sec` (`yaegikernel/jobs.go`), and that is the job's.
- An engineer cannot keep a minute budget it cannot see.
- Management's Cast states `freeze_minutes`, settled by the runtime from the
  Cast record's `committed_at` to the Freeze (`engineering_round_predictions.go`,
  calibration cut 1). That is the anchor a clock must share.

## Fix

1. `choir.Context()` in the engineer's cell adds `elapsed_min`, `budget_min`,
   `budget_left_min`, and `revision` once engineer reuse is on main. Elapsed
   is measured from the same anchor as the freeze_minutes settlement: the
   Cast record's `committed_at`, else the assignment's `created_at`
   (`roundClockAnchor`, shared with `roundPredictionRecords`).
2. One bounded harness line on a tool result when elapsed crosses budget
   minus 20 minutes: "N minutes used of M; Freeze within the next K minutes
   or say why in a Report." Once per assignment attempt (and per revision).
3. Management's `freeze_minutes` prediction is not shown to the engineer: the
   clock carries elapsed and budget only; the engineer's view of its Cast
   already drops the predictions block.

## Failure modes (written before the code)

- C1 The clock disagrees with the settlement (a different anchor). Both read
  `roundClockAnchor`; a test pins them to the same instant.
- C2 The clock freezes at cell start while a cell runs for minutes. The host
  sends elapsed-at-cell-start; the kernel adds its own monotonic time since
  the cell began when `Context()` is called; the frame also carries the anchor
  instant, and the kernel takes the larger reading so broker queue and transit
  time are counted (under-reporting shows more budget than exists).
- C3 The anchor is unreadable (store error, no Cast, a verifier slot, a
  non-assignment run, a Cast record that cannot be read, a future anchor). No
  clock is sent, never a silent fallback to a different anchor: `Context()` has no clock keys
  rather than a zero that reads as "just started". Never an error to the
  cell.
- C4 The line repeats every tool result. Fired once per assignment attempt
  and revision from an in-memory set; marked only when the line is attached
  to a result that is returned (a failed cell loses nothing).
- C5 The line is unbounded or shouts after the budget. One fixed sentence;
  K is clamped to 0 and the words say so.
- C6 The prediction leaks. The clock type has no prediction field; a test
  asserts the frame JSON and Context() carry none.
- C7 A crash restart resumes the nag. The set is in memory: a new boot
  closes the work (CLAUDE.md restart rule); a planned-restart resume may
  fire the line once more, which is acceptable.

## Not done

The run-context turn is not changed (not cheap: it is assembled per provider
turn, not per cell). Engineer reuse is not on main at this base; `revision`
is plumbed as an optional field and stays absent until it lands.
