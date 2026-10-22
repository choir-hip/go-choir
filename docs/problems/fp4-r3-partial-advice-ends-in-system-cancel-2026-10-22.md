# The R3 freeze refusal tells engineering to report "partial", and a partial report ends in a completion-guard cancel (2026-10-22)

Found 10:19 UTC by the self-development monitor in FP4's staging drill,
step 6, on the test computer `computer-71afbf1461dffabcce8eeebe558d03bd`.
Build 5289387c9 (deploy K, cold-booted at 10:16:04 by a planned refresh).
This commit only records the problem. Each claim cites trace or code, and
the one design reading that goes beyond them is labelled.

## Evidence

Assignment B: `assignment-932f7ac2c41ba20992edb935`, trajectory
`17c9e9fb-0125-5746-bb10-8d64053147cd`. A's candidate
`selfdev-0593136f3ac2d894e21abd97132276d1` was `awaiting_approval`.

1. 10:16:48.8: B's `choir.Freeze` was refused, as R3 intends. The tool
   error read: "self-development candidate: assignment
   assignment-932f7ac2… cannot freeze until the open candidate settles
   (another self-development operation is open on this computer:
   selfdev-0593136f… is awaiting_approval); a frozen, verified or
   awaiting_approval candidate waits for management's Verify and the
   owner's decision, which this assignment cannot make; **report the work
   as partial or failed instead of retrying the freeze**".
   - Guest log, 10:16:49: "selfdev: assignment assignment-932f7ac2… cannot
     freeze: another self-development operation is open on this computer:
     selfdev-0593136f… is awaiting_approval".
2. 10:17:02.5 and 10:17:09.8: B ran `choir.Complete("partial", "none",
   "Request B applied the specified one-line comment … Freeze attempted
   once and refused: …")`.
   - That gave two `co_super_assignment_reported` events (reducer_seq 5
     and 6).
3. 10:17:18.9 and 10:17:19.0: capsule disposition set to
   `revoke_requested`, then `revoked`.
4. 10:17:19.08: `co_super_assignment_cancelled`, event_id
   `engineering-system-cancel:assignment-932f7ac2…:1:1`. Reason: `tool
   loop: completion guard "engineering_overlay_terminal_fate_required" was
   not satisfied after 2 retries`.
5. The engineering run for B ended `cancelled`, created 10:16:25.6 and
   updated 10:17:19.08. B's trajectory `work_settled` follows.
6. The operations list after the drill shows no operation for B. Start
   refused it with `ErrOperationOpen`. A's candidate is still
   `awaiting_approval`, untouched.

The drill prompt (docs/evidence/fp4-admission-queue-deployed-proof-2026-10-22.md)
asked B to "Complete as partial" on refusal. It followed R3's own advice,
so the engineering desk did what both the prompt and the tool error said.

## Code read (origin/main 5289387c9)

- The refusal is `internal/agentcore/freeze_candidate.go:50-60`. It
  advises "partial or failed".
- A partial report is not terminal:
  `terminal := report.Result != types.EngineeringResultPartial`
  (`internal/agentcore/engineering_assignment_fate.go:853`).
  - It records a progress report and leaves the assignment open.
- The engineering desk's completion guard is `internal/agentcore/runtime.go`,
  about lines 3728-3742. A turn may end only after a tool result carries
  `fate_terminal: true`; `engineeringOverlayTerminalFateCommitted` (:4275)
  checks for it.
  - Otherwise it retries with the instruction "Freeze … then Complete …
    or commit the corresponding terminal failure fate". After 2 retries
    the tool loop fails, and the system cancels the assignment.
- So of R3's two options, one ("partial") cannot end the run. It leads
  to a system cancel whose reason names the guard, not the open candidate.

## Harm

- The assignment's recorded fate is `cancelled` by the system, with a
  guard reason.
  - The true cause was that the freeze was refused because candidate
    selfdev-0593136f is open.
  - Round 2 committed to true cancel reasons (503d2942c), and this path
    still yields a cancel reason that is not the cause.
- Management and the owner read "cancelled: completion guard not
  satisfied" rather than "failed: cannot freeze until
  selfdev-0593136f settles". The refusal text sits only inside B's
  partial reports.
- Any model that follows R3's "partial" advice spends two guard retries
  first: about 17 s here (10:17:02 to 10:17:19), and longer on a slower provider.

## Options (not chosen here)

1. Change R3's advice to "report the work as failed (Complete with result
   failed and the refusal as the reason)". This is a text-only change.
2. Make the refused freeze itself a terminal failure fate, so the guard
   is satisfied without a second call.
3. Let the guard accept a partial report whose last freeze was refused by
   an open operation, and close it as failed with that reason.

Design reading (hypothesis): option 1 is the smallest change and keeps
R3's intent. Options 2 and 3 move fate decisions into the runtime and
need the round-2 reviewers.

## Cause confirmed and fix (lane/fp4-r3-nudge, 2026-10-22)

Tests b557b2d64 (failed on main 7448bf899), fix cf7ed5072.

The code read above holds: a partial report is not a terminal fate
(`terminal := report.Result != types.EngineeringResultPartial`), so the
completion guard retries and then fails the tool loop, and the system
cancel takes the guard's reason.

Fix, both halves of what FP0 asked for:

1. Option 1 of the record: the refusal now reads "report the work as
   failed: Complete with result failed and this refusal as the reason,
   instead of retrying the freeze". It no longer mentions partial
   (`TestFreezeRefusalAdvisesAFailedReport`).
2. The guard's reason names the candidate when the run's freeze was
   refused by one, so a model that still reports partial ends with a fate
   that says why: `tool loop: completion guard
   "engineering_overlay_terminal_fate_required; the freeze was refused
   because self-development candidate selfdev-… was open (state
   awaiting_approval)" was not satisfied after 2 retries` (wording as of
   round 3b; round 3 added "and waits for management's Verify and the
   owner's decision", which was wrong for states other than
   awaiting_approval). Its reminder then tells the run to Complete failed with the
   refusal. The reason is read from the run's tool results
   (`engineeringTerminalFateGuardReason`), and a run with no refused freeze
   keeps the old reason (`TestCompletionGuardReasonNamesTheOpenCandidate`).

Options 2 and 3 (the runtime deciding the fate) were not taken. The guard
still decides nothing new: it ends the run exactly as before, with a true
reason.

Disposition: repaired. The fate is still a system cancel when the model
ignores the advice, but its reason now names the open candidate.

## Round 3b (review bunch 51)

- DeepSeek 5: the guard's reason now names the candidate and the state
  the refusal gave, and claims nothing about what that state waits for
  (1b21dc6c8, test f9732b3de). Text only; fate unchanged.
- Closed in FP4 round 4 (4e6f96833: the guard reads only refusals after
  the last successful freeze). Was: residual: the guard keeps the run's last refusal even if a later freeze
  succeeded, so a stale refusal can name a candidate in the reason. A
  successful freeze leaves no mark in the cell output the guard reads, so
  scoping the scan is not a few lines. It is not text only: whenever the
  reason is not the default, the guard also switches the model-facing
  instruction to "Complete with result failed" (internal/agentcore/runtime.go,
  the completion guard, about lines 3744-3750). So in the rare run where
  B's freeze is refused, A's candidate then settles, B's retried freeze
  succeeds, and B ends its turn without Complete, the guard tells B to
  report failed for work whose freeze succeeded: a wrong instruction
  toward a failed fate (review bunch 54, Sonnet F2). It stays a residual;
  the repair is to scope the scan to refusals after the run's last
  successful freeze. A model that echoes the refusal pattern in cell
  output can also shape this text and instruction.
- DeepSeek 1: the drill's request B said "partial", so it could not test
  this repair. The evidence doc keeps that record and adds a re-run plan
  whose request B reports FAILED with the refusal as the reason
  (docs/evidence/fp4-admission-queue-deployed-proof-2026-10-22.md).
