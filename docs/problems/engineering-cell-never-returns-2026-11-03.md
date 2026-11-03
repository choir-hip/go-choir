# An engineering cell that never returns hides until the activation deadline (2026-11-03)

Status: open; fix lands in the next commit.
Mutation class of the fix: red (the engineering agent loop's tool path).

## What was seen

Crouton self-development arm, computer
computer-848b21d55df1dd1a953ae798b8edc696, assignment
assignment-6af03a6b6f61e705ac8203a8, trajectory
26f41d68-9411-5739-88ba-4bfacae6e0a3, model openrouter stealth/crouton-alpha.
Read from the trace export (`choir trace export`, run_events seq 99-104):

- 22:21:35Z the cell `choir.Start(node scripts/choir-shot.mjs ...,
  minutes: 10)` started job-d807dd9c4be9 (wall clock 600 s).
- 22:21:54Z a 46-token Poll cell (`v, _ := choir.Poll("job-d807dd9c4be9")`)
  saw it running; the turn parked (`capsule_job_park`, deadline 22:32:35Z).
- 22:32:35Z the park ended on its deadline: `reason: deadline,
  state: running, waited_ms: 641056`. The job was still running 41 s past
  its own 600 s wall clock in the host's view.
- 22:32:38Z iteration 50: the model called capsule_go_eval once
  (`output_tokens: 46`, the same size as the Poll cell before it).
- Nothing after: no tool.result, no capsule_job_park, no further provider
  call. At 22:52Z (20 minutes later) the run is still `running` and the
  run's last event is still seq 104.

## Confirmed: what the cell blocked on

The cell returned at 23:18:42Z (run_events seq 105), `duration_ms
2764161` (46.07 min), at the activation deadline (bound 22:18:42Z + 60
min). Its source was a plain Poll:

    v, _ := choir.Poll("job-d807dd9c4be9")

and it failed with `tool_error: failed to read broker response: read unix
@->/proc/self/fd/11/broker.sock: i/o timeout`. choir.Poll reads the job
snapshot the host put in the request (`GoEvalRequest.Jobs`), so the cell's
code did not block; the host's go_eval RPC to the capsule broker got no
answer for 46 minutes. The assignment was then cancelled (seq 106,
`loop.cancelled: context deadline exceeded`). No job-end record ever
reached the engineering inbox: the job never ended in the host's view.

## Why nothing ended the wait earlier (confirmed from code)

`newCapsuleGoEvalTool` (internal/agentcore/tools_capsule.go) bounds a cell
only when the model passes `timeout_ms`. Without it, the eval context is
the activation's own context. `BrokerClient.call` sets its read deadline
to the context's deadline when there is one, so the host read the
broker's answer until the activation deadline, not for the broker's own
60 s cell bound plus a margin. The other host steps around the call are
unbounded too: the shared-connection lock (`connMu`, not
context-aware) and the post-eval worktree digest
(`digestCapsuleWorktree(ctx, ...)`). The only other watchdog is
`armCellTerminalDeadline`, which is the activation deadline as well.
The desk path (tools_desk.go) has a host deadline for management cells;
engineering's capsule path has none.

## Why the broker did not answer (hypothesis, the leading one)

The broker went silent for every RPC, not one: the host's job_status
polls (a fresh connection after each 30 s failure) also got no answer
from about 22:31Z, so the host still saw the job running 41 s past its
600 s wall clock at 22:32:35Z, and no job end ever arrived. The job's
own wall-clock stop is a timer inside the broker, and it never took
effect either. The broker serves each connection on its own goroutine;
no broker lock is held for a job's duration (job manager `m.mu` is held
only in Start and a lookup; `j.mu` only briefly). So this was not one
blocked handler or a single-threaded broker.

The broker process runs in the capsule's own cgroup, the same one as the
job (internal/capsule/executor.go, `UseCgroupFD`/`CgroupFD` at broker
start). The job was `node scripts/choir-shot.mjs` (a vite build, then
Chromium). A job pressing the cgroup's memory.high throttles every task
in the cgroup, the broker included; with no swap, reclaim also evicts
the broker's text pages. That stalls the whole broker, the wall-clock
timer included. This fits every observation, but it is unconfirmed: it
needs the capsule's memory.events (`high`, `max`, `oom` counts, in the
capsule memory receipt at destroy) or the guest log for 22:30-23:19Z.

## Remaining error field

- Fix (next commit): every engineering cell has a host wall-clock limit
  (5 minutes; the model's timeout_ms plus a 15 s grace when smaller; never
  past the activation deadline less 30 s). The broker read has its own
  bound: go_eval reads for the broker's cell bound plus 15 s (75 s when
  timeout_ms is unset), job verbs for 60 s, and the error says "the
  capsule broker did not answer" and names the job. The model's tool
  result names the running job (not stopped) and the next step. A
  "capsule_cell_running" progress event fires every 2 minutes.
- Residual (root cause, not small): the broker shares the job's cgroup.
  Moving it to its own leaf (or the job to a child cgroup) has to respect
  cgroup v2's no-internal-processes rule, the Quiesce freeze (which today
  freezes the broker with the capsule) and the capsule's memory
  accounting. Confirm with memory.events first.
- Residual: while the broker stays silent, the host's job watcher cannot
  see the job end, so "the platform wakes you" depends on the broker
  coming back. The cell now answers within about 75 s instead of 60 min,
  and the model can choir.Stop or end the round.
