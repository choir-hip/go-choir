# A frozen candidate is lost when the assignment's Complete cites one execution ref twice (2026-10-22)

Found 14:02 UTC by the self-development monitor, in the FP4 round 3 re-run
of steps 4 and 6 on the test computer
`computer-71afbf1461dffabcce8eeebe558d03bd`. Build d206fdd40 (deploy Q).
This commit only records the problem; there is no fix in it. The cause is
read from the trace and the code. The one claim neither confirms is marked
as a hypothesis.

## Evidence

Request A: `assignment-6daa96ea503df3de8adbba70`, trajectory
`a85bd572-42c1-5512-89ee-6e000c49c88d`. Events and run events come from
the product API and `choir trace export`.

1. **14:00:59: A's Freeze succeeded.** The cell staged `rlm:freeze:0` and
   operation `selfdev-f8778d8a37e0d85be0dc7ff42f45efb4` went `frozen`
   (14:01:03).
   - Events: capsule `freeze_requested` at 14:01:06.99, then `frozen` at
     14:01:07.80.
2. **14:01:06.7: A called `choir.Complete("completed", "none", …)`** with
   five execution refs. `capsule-exec:sha256:58d86c70…46a8` appears
   **twice**:
   - `58d86c70…`, `f6b0fb2f…`, `b68afca0…`, `58d86c70…` (all
     `capsule-exec:`);
   - `capsule-go-eval:sha256:95399949…`.
3. **14:01:08.26: the Complete was refused** with
   `tool_error: reduce: persist tray-1: assignment command evidence
   unavailable after durable freeze: %!w(<nil>)`.
4. **14:01:15 and 14:01:24: every later cell was refused.** First
   `choir.Pack()`, then `choir.Complete("blocked", …)`. Both got
   `tool_error: assigned capsule obligation is no longer executable:
   assignment fate is not active and bound`.
5. **14:01:12 and 14:01:18: the completion guard retried twice**
   (`loop.retry reason=engineering_overlay_terminal_fate_required`).
6. **14:01:29.73: A was system-cancelled.** Event
   `engineering-system-cancel:assignment-6daa96ea…:1:1`. Reason: `tool
   loop: completion guard "engineering_overlay_terminal_fate_required"
   was not satisfied after 2 retries`.
7. **The candidate closed `failed`.** Operation `selfdev-f8778d8a…` reads
   `failed` at 14:01:29.91, error equal to the guard reason. A finished
   build and a frozen candidate were lost.

## Code read (d206fdd40)

- `reduceComplete` (internal/agentcore/rlm_reduce.go, about :1718) takes
  the execution refs with `trimNonEmptyStrings`, which does not remove
  duplicates. One recorded command is made per ref, so there are 5.
- After the durable freeze, `bindFrozenAssignmentExecutionReceipts`
  (internal/agentcore/engineering_assignment_fate.go:1362-1373) resolves
  those refs through `ResolveGrantedExecutionReceiptsFor`. That uses
  `bindGrantedReceipts` (internal/capsule/granted_receipt.go:65-68), where
  "Duplicate refs collapse", so it returns 4 receipts.
  - `if err != nil || len(receipts) != len(refs)` then fails with
    `err == nil`. That is the `%!w(<nil>)` in the message.
- Once the capsule is `frozen`, a capsule cell refuses with "assignment
  fate is not active and bound". The model has no cell in which to call
  Complete again with corrected refs.
- The guard then cancels, and the ended-assignment close fails the frozen
  operation.

## Cause

- Confirmed by code and trace:
  - A duplicate execution ref passes `Complete`'s parse.
  - The ref list then fails the post-freeze evidence binding by a count
    mismatch.
  - A failed Complete after a durable freeze cannot be retried from any
    cell, because the capsule is frozen.
- Hypothesis: the `capsule-go-eval:` ref did not cause the mismatch.
  Opening it would have returned a non-nil error, as A's first Freeze
  attempt at 14:00:45 did for another go-eval ref. The duplicate alone
  accounts for 5 against 4.

## Harm

- One repeated ref in a model-written Complete throws away a finished
  candidate. Here the release build ran for about 20 minutes, with two
  OOM kills in the capsule (see below).
- The candidate closes `failed` with the guard's reason, not the binding
  error. Neither the owner nor management can tell why from the row.
- `%!w(<nil>)` is a formatting bug: the error names no cause.
- FP4 drill step 6 cannot run as planned in this re-run. It needs A's
  candidate open, and this ended it. Step 4 was judged anyway: A's
  terminal_at was 14:01:29.73 and B bound at 14:01:46.89, 17.2 s later.

## Related observation (same capsule, recorded here)

- A's capsule `capsule-3b63529a-9bc6-55e8-a589-e8decdcfe8cb` teardown
  receipt (deploy P observe policy), guest log 14:01:27:
  `policy=observe request=2048MiB admitted=2048MiB high=- max=2048MiB
  peak=2048MiB events high=0 max=385311 oom=73 oom_kill=2
  doctrine_would_throttle=true`.
- The release build ran at its 2 GiB ceiling, and the kernel OOM-killed
  inside it twice.
- Three `capsule_go_eval` calls hit the 30 s broker read timeout, at
  13:44:16, 13:44:57 and 13:58:54. Hypothesis: these timeouts are linked to
  the memory pressure. That is not confirmed.

- At the same time, a management wake for the fate deadline was
  dead-lettered three times. Guest log:
  - 14:01:07: `actor wake latency kind=assigned_engineering_fate_deadline
    … waited=23m1s`;
  - 14:01:10, 14:01:27 and 14:01:29: `dispatcher: poison` and
    `dead letter: delivery_failed for update
    58a5b36c-e071-538b-82cc-70b5bdfb6cc7` (then `#redrive-1`,
    `#redrive-2`).
  - The handler's error is not logged.
  - Hypothesis: the fate-deadline handler failed on the same unbindable
    report. Its log does not show this.

## Next probes

1. In `reduceComplete`, deduplicate execution refs, or refuse duplicates
   at parse time before any freeze-sensitive work. Then make the count
   check name the cause instead of wrapping a nil error.
2. Decide what a refused Complete after a durable freeze may do. Either
   keep a report-only cell open while the capsule is `frozen`, or check
   the Complete before the freeze commits.
3. A focused test: Freeze, then Complete with a repeated
   `capsule-exec:` ref. It should report `completed`, or refuse in a way
   that can be retried, and never lose the candidate.
