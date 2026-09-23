# Document-Cast Settlement Consumed by Stray Complete Cell

Date: 2026-09-23
Status: documented before fix, per the problem-documentation-first invariant.
Found by the M2 post-landing probe (second real document-channel cast).
Mutation class of this document: green (analysis; no runtime change).

## Observed

On staging (`computer-03335285269bdba4f94377e56879f9e6`, deployed build
`4de7fdf9`), a second owner-authored revision on the engineering-bound
Texture document opened and bound assignment
`assignment-e77f7940-7075-545b-95a6-1f5a1cd9f0a3` (trajectory
`a5c99d30-a9ed-570c-8cbb-ebdd6485ef1e`, bound 2026-09-23T16:55:08Z).

The assigned run executed the task (wrote and ran
`/workspace/platform/fibdemo/main.go`, verified Fibonacci output) and
finished `state=completed` at 17:23:57Z. Its result text reports that a
**stray `Complete` cell** — a one-character `"e"` cell that compiled and
returned a receipt instead of a compile error — was accepted and reduced,
consuming the assignment's single terminal settlement before the model's
real `choir.Complete` could attach summary and evidence.

Canonical state after the run finished:

- Trajectory events stop at `co_super_assignment_bound` (seq 3). No
  `co_super_assignment_reported`, no `co_super_assignment_completed`, no
  `co_super_capsule_disposition_set` rows.
- Assignment: `disposition=bound`, `capsule_disposition=active`,
  `bound_loop_id` set — permanently, ~5h observed with no change.
- Run metadata carries `lifecycle_terminal_settlement_requested: true`,
  but the settlement never reached the assignment record.

## Root cause (hypothesis, unverified)

Two candidate substrates, both plausible from the evidence:

1. **Stray-cell terminal consumption.** A malformed/empty eval cell
   (`"e"`) reached `Tray.Complete` with default/empty arguments, passed
   validation (empty verdict coerces to `none` under `ad7b5194`), and
   committed the saga. The saga then failed to record the report — same
   stranded shape as the first strand — OR recorded it against an
   identity the trajectory view doesn't surface.
2. **Report commit still fails silently post-fix.** The stray Complete
   committed freeze/revoke, then `RecordCoSuperAssignmentReport` failed
   and the failure is not surfaced as a trajectory event — leaving
   `bound`/`active` forever with no retry path that produces events.

Either way the observable defect is the same: **a completed run leaves
its assignment permanently `bound` with no terminal events and no
recovery sweep that fires.** The first strand's fixes repaired the
verdict-validation path; this strand shows the settlement path still has
a hole — either in stray-cell admission (a one-character cell should not
be able to consume a terminal settlement) or in report-commit failure
visibility.

## Evidence

- Trajectory `a5c99d30-a9ed-570c-8cbb-ebdd6485ef1e`: events 1–3 only
  (`trajectory_started`, `co_super_assignment_opened`,
  `co_super_assignment_bound`).
- Assignment `assignment-e77f7940-7075-545b-95a6-1f5a1cd9f0a3`:
  `disposition=bound`, `capsule_disposition=active`.
- Run `run:assignment-e77f7940-…`: `state=completed`, result text names
  the stray `"e"` cell and the consumed settlement;
  `input_tokens=3194958`, `lifecycle_terminal_settlement_requested=true`.
- Contrast: trajectory `ad04eaba-53e9-5c5c-9116-f8c18ae6dce5` (the
  accepted M2 proof, same computer, same build) reached
  `co_super_assignment_reported` + `disposition=completed` — the happy
  path works; this strand is a distinct failure mode.

## Fix direction

1. Reject or quarantine degenerate eval cells (empty/near-empty source)
   before they can stage a `Complete` intent — a stray keystroke cell
   must not be able to consume a terminal settlement.
2. Make report-commit failure visible: a failed
   `RecordCoSuperAssignmentReport` after freeze/revoke must emit a
   trajectory event or retry via the stranded-frozen sweep, not strand
   silently.
3. Add a sweep for `run.state=completed` + `assignment.disposition=bound`
   + `capsule_disposition=active` — the run is terminal, the assignment
   is not; that pair should reconcile.
