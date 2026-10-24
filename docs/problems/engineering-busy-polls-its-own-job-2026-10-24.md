# Engineering busy-polls its own capsule job, and a prompt rule has not stopped it (2026-10-24)

Date: 2026-10-24, 03:5x UTC. Found by: FP0, from the self-development drill
fp0-selfdev-1024a (evidence: [selfdev-drill-2026-10-24](../evidence/selfdev-drill-2026-10-24.md)).
Class of this record: green. Class of the fix it leads to: orange (the
engineering in-cell API; the reducer and the desk loop). Status: fix
built on lane `lane/fp0-job-park` (design below), not deployed or drilled.

## What happened

The drill (`--change go-served`, staging 0cbc52edc, a fresh jailed class-test
computer) passed all 19 checks. A full cycle works: a capsule change, Freeze,
verify, the pre-apply checkpoint, approval, the host build, apply, the new
route served, a restore, and the route gone. Its one failure was round 1 of
engineering:

- Engineering started the `bin/autoputer` Go build as a capsule job, then
  called `choir.Poll` on it 30 times in 73 seconds (03:08:53 to 03:10:06),
  each a model turn showing `go: downloading …` progress. The build was
  healthy. Module fetch through the host goproxy worked under host-only DNS.
- It tried `choir.Complete("blocked")`. The reducer refused it ("job … is
  still running; Complete waits until it ends … or choir.Stop it"; the
  10-20 fix). It then `choir.Stop`ped the healthy build and completed
  blocked. Management re-cast it, and round 2 passed.
- Round 1 cost 51 model turns and 1.68 M input tokens. Round 2 polled 57
  times over a build of about 7 minutes (71 turns, 1.54 M input tokens) and
  this time waited the job out. Polling is most of the cycle's model spend:
  about 3.2 M input tokens for one self-development change.

## The cluster

The same behaviour, three drills, five days, two models' worth of prompt
changes:

| Drill | Polls | Outcome |
|---|---|---|
| selfdev-jobs-0101 (10-15) | about 100 cells, 2 s apart, over 4 min | compaction, a refused second start, Complete blocked while the build ran |
| selfdev-jobs-0654 (10-15) | 126 cells every 2.5 s, about 4 KB tail each | compaction at 166 k tokens (emergency_fallback); the desk forgot its own build |
| fp0-selfdev-1024a (10-24) | 30 then 57 | Stop of a healthy build, round 1 lost; about 3.2 M input tokens |

What the desk is told today. **[V]**
- The `Poll` help (`internal/yaegikernel/surface.go:47`): "It never waits:
  end your turn and you are woken when the job ends."
- The engineering overlay (`internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml:52`):
  "Do not poll a job in a loop … A running job is never a blocker: end your
  turn and wait to be woken; do not Complete "blocked" because a build or
  test is still running."

The model read the help in round 1 and polled anyway. The 10-20 cluster
record ([engineering-completion-path-cluster](engineering-completion-path-cluster-2026-10-20.md),
failure 23) already ruled that "a prompt change would only shift the odds. A
refusal closes the path for every model". It added the Complete refusal.
Tonight the model went around that refusal with Stop.

## Classification

Substrate, by this repo's rule: three or more symptoms in one subsystem, and
the prompt rule meant to prevent it exists and does not hold. Nothing in the
loop makes waiting the easy path. A cell that polls a running job returns at
once, and the tool loop offers the model another turn. Ending the turn is a
choice the model has to make against the grain of the loop, every time.

## What a fix has to do (not decided; failure modes first)

The fix should make "end the turn and be woken" what happens, not what is
asked. Candidate shape: a cell whose only job-related effect is a `Poll`
that sees the run's own job still running ends the turn after it reports.
The desk parks on the job wake, and the next activation starts from the
job's outcome. Failure modes any shape must answer:

1. The job wake never arrives (lost wake, restart): the desk must not sleep
   forever. Jobs carry a wall clock (at most 45 min). A park needs a
   deadline that re-wakes the desk, and the deadline must survive a planned
   restart and close on a crash, per the restart rule.
2. A desk that polls a job that is already done must see the outcome in the
   same turn (no park).
3. Stop as an escape: a Stop of a running job in the same activation that
   polled it, followed by Complete blocked, is today's escape. Decide
   whether Stop needs a reason the reducer records, or whether the park
   simply removes the pressure that leads to it.
4. Other work while a job runs (reading files, writing a note) must still be
   possible in the cells before the park, so engineering is not forced idle
   when it has real work.
5. Desks without capsule jobs (management, Texture, research) are
   unchanged.
6. The turn-commit ledger must record the park and the wake, so a trace
   shows the desk waited, and was not dropped or starved (the "Review the
   traces" rule).

## Also seen, not explained

- Right after the restore, the operation read back as `awaiting_approval`
  (its state at the restored sequence 930). The `restored` witness after the
  restart reads `applied`. One read was taken before the restart and one
  after. Not looked into.
- The host build took 17 min 10 s (go-served-1143 on 10-15: 13 min), the
  longest stage of the cycle. A warm build cache is the obvious lever. Not
  measured.

## Heresy delta

discovered: the overlay's no-poll rule and the 10-20 refusal do not prevent
a desk from stopping a healthy build to end its round. introduced: none.
repaired: none.

## Design: a Poll of the run's own running job parks the turn (FP0 lane fp0-job-park)

Class: orange (the engineering in-cell API, the capsule_go_eval tool, the
tool loop's engineering options). Rollback: revert the lane's commits; no
store schema, tape event, boot path or provider route changes.

### What the code does today **[V]**

- A job's end does wake the desk, but only inside the same activation and
  only after the model ends its turn. `handleCapsuleJobDone`
  (`internal/agentcore/capsule_jobs.go`) commits a `job:<id>` record into
  `engineering:<assignment>` and signals an in-process channel
  (`signalJobWake`). The only reader of that signal is
  `engineeringJobWait`, run by the engineering completion guard
  (`runtime.go`, `WithCompletionGuard`) when a model turn ends with no tool
  call and no terminal fate. It waits for the job's end (bounded by the
  job's wall clock plus one minute and the run's context) and resumes the
  loop with `jobEndedInstruction(view)`: state, exit code, elapsed, the
  receipt_ref and what to do with it. There is no separate activation, no
  durable park and no outbox wake: the "park" is the tool loop blocked in
  one goroutine.
- The tool loop (`internal/toolregistry/toolloop.go`) calls the model again
  after every tool batch unless the batch was terminal or passivating. So a
  cell that polls a running job is followed by a model turn, and only a
  model that chooses to end its turn reaches the wait. That is the grain the
  record names.
- A crash closes jobs as `interrupted` at the next guest boot
  (`closeInterruptedJobs`), and runs are closed by the existing boot rule.
  A planned restart refuses while an engineering capsule is live
  (`plannedRestartWork.LiveCapsules`, `planned_restart.go`), so no wait on a
  job ever spans a planned restart.

### The shape

1. The cell reports what it waited on. `choir.Poll` records each job it
   returned in a non-terminal state (`running`, or `starting` for a job the
   cell itself staged). The cell result carries them as `jobs_awaited`
   (kernel `SessionResult`, the broker, `capsule.GoEvalResult`), the same
   way `records_shown` already rides back.
2. The host decides the park. After the cell's reduction, `capsule_go_eval`
   sets `job_park` to the first awaited job unless the cell committed a
   terminal fate or staged a `choir.Stop`. The model sees the field in the
   cell result.
3. The loop does not offer another model turn. A new tool-loop option,
   installed for the engineering overlay only, runs after a non-terminal
   batch: when the cell result names `job_park` (or the cell was the
   Complete-while-the-job-runs refusal), it waits for the job's end with the
   same wait the completion guard uses, then appends the job-ended
   instruction as the next user turn and calls the model. A park counts
   against the guard's wait bound (32 per run), not its retries.
4. The park is in the run's event trace. `capsule_job_park` (job id, cause
   `poll` or `complete_refused`, deadline) is emitted when the park starts,
   and `capsule_job_park_ended` (job id, how it ended, the job's state,
   waited ms) when it ends. Both are run progress events, appended like
   `completion_guard`.
5. Nothing durable is added: no new store row, tape event, outbox entry or
   boot path. The park dies with the process exactly as the existing guard
   wait does.

Why this shape: it reuses the wait that already works (round 2 of the 10-24
drill and the 10-15 drills show `engineeringJobWait` resuming turns) and
moves its trigger from "the model ended its turn" to "the cell looked at a
job that is still running". That is the one decision the model kept getting
wrong. It needs no new authority and no new durable state, and a desk that
never polls a running job sees no difference.

### The six failure modes

1. **The wake never arrives.** The wait is bounded by the job's wall clock
   plus one minute (`jobWaitSlack`) and by the run's context. A stale signal
   from an earlier job does not release it (it re-checks the job). At the
   deadline the turn resumes with an instruction that the job outlived its
   wall clock. A crash kills the process: the job closes `interrupted` at
   boot and the run closes under the existing rule, so no resumer is added.
   A planned restart cannot happen while the capsule is live.
2. **A job that is already done.** The cell-start snapshot shows it done, so
   Poll does not record it and the cell does not park. If the job ends
   between the cell and the park, the park sees no running job and resumes
   at once with the job's outcome, without waiting.
3. **Stop as an escape.** A cell that stages a Stop does not park (the job is
   ending). Stop is otherwise unchanged: it is the legitimate way out of a
   hung or wrong job. The park removes the pressure that led to tonight's
   Stop. After its first look at a running job, the desk next speaks when
   the job has ended, so it never sees "still running" twice. The refused
   Complete (failure 23) also parks, so the cell that tried to finish early
   is followed by the job's outcome, not by a fresh chance to Stop.
4. **Other work while a job runs.** Only a cell whose Poll saw a running
   job parks. Cells that read, write, Exec, Note or Report and do not poll
   run as before. A desk that polls once to read the tail parks there, and
   that is the intent.
5. **Desks without capsule jobs.** The option is installed only with the
   engineering overlay. Other desks have no jobs in their cell snapshot, so
   their Poll never records one. Management, Texture and research are
   unchanged.
6. **The trace shows the wait.** `capsule_job_park` and
   `capsule_job_park_ended` sit in the run's events between the cell's
   `tool.result` and the next model call, with the job id, the cause and how
   the park ended. A trace can tell a desk that waited from one that was
   dropped or starved.

### Residuals

- A desk can still Stop a healthy job the first time it sees it running, in
  the polling cell itself. The park makes that unlikely, not impossible.
  If a drill shows it, the next step is a Stop that records a reason the
  reducer keeps, not another prompt rule.
- The park is in-process, as the guard wait is. A durable park (a parked
  run woken by the job record through the outbox) would be needed only if
  engineering runs come to outlive a process. Today they cannot (a live
  capsule refuses a planned restart).
- The park reads `job_park` from the cell result the model sees, which the
  tool loop cuts at 100 KiB. A cell that prints more than that loses the
  field (and `fate_terminal`, which has the same exposure), so it does not
  park. Tails are about 4 KiB, so this is unlikely, and the outcome is
  today's behaviour, not a new failure.
- The broker tests (`cmd/capsule-broker`, linux only) were vetted for linux
  but not run on the lane's macOS host. CI runs them.
