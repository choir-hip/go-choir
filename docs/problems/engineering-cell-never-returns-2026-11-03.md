# An engineering cell that never returns hides until the activation deadline (2026-11-03)

Status: open. The host-side bound landed (062c669da); the root cause is
not found. The memory hypothesis is refuted (see "The memory hypothesis is
refuted").
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

## Why the broker did not answer (first hypothesis, refuted below)

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

## The memory hypothesis is refuted (read 2026-11-03 23:42Z)

Read with `choir trace export --computer
computer-848b21d55df1dd1a953ae798b8edc696` (the boot layer's recent_log),
release 5ea2a61b07ed. The job ran in capsule-1673af6c (spawned 22:18:42Z,
destroyed 23:19:12Z), not in capsule-ae78252b (that is the next
assignment's, spawned 23:20:51Z). Its memory receipt at destroy:

    capsule capsule-1673af6c-... memory: policy=observe request=2048MiB
    admitted=2048MiB high=- max=2048MiB peak=1426MiB events high=0 max=0
    oom=0 oom_kill=0 doctrine_would_throttle=false

- memory.high was not set (policy observe), so nothing in the capsule could
  be throttled at memory.high.
- memory.events `max 0`: the capsule never reached memory.max, so no task in
  it, the broker included, ever entered reclaim against the capsule's limit.
- `oom 0`, `oom_kill 0`: no OOM kill in the capsule's subtree (cgroup v2
  counts a kill by any OOM killer, the global one included, in the victim's
  memory.events).
- The peak was 1426 MiB of 2048 MiB. No job memory boost was taken for
  job-d807dd9c4be9. No boost line names it in the log.

Control run on choir-agents (kernel 6.18, a delegated user scope; the
harness is a scratch program and is not in the repo). A broker stand-in
(Go, verifies Ed25519 per request, writes a status file, runs a job with a
15 s wall clock that kills the job's group) answered 44 of 44 requests,
1 per second, with the host holding the listener as the executor does:

| run | capsule limit | memory.events / pids.events after | broker answers | wall clock fired |
|---|---|---|---|---|
| memory hog, same cgroup | memory.max 256 MiB, swap 0 | max 42, oom 1, oom_kill 1 | 44/44 | yes |
| memory hog, broker in a sibling leaf (job leaf 224 MiB, broker memory.min 32 MiB) | same | max 39, oom 1, oom_kill 1 | 44/44 | yes |
| fork hog, same cgroup | pids.max 64 | pids max 4 | 44/44 | yes |

With no swap, a job at memory.max is OOM-killed within about a second and
leaves `max` and `oom_kill` counts in memory.events. The crouton receipt has
neither. Moving the broker to its own leaf is not the fix for this incident,
and it was not built.

## What the evidence does establish (confirmed from code)

- **A dead broker looks like a hung one.** The host keeps its own copy of the
  broker's listening socket open (`caps.listener`,
  `startBrokerLocked`, internal/capsule/executor.go). If the broker process
  exits, the socket still accepts connections into its backlog and nobody
  reads them. A host call then fails as `read unix ... broker.sock: i/o
  timeout`, the exact error seen, never as "connection refused".
- **A broker exit after readiness is never recorded.** `caps.processErr` is
  read only before readiness ("capsule broker exited before readiness").
  After that, the launcher's exit status goes unread: no log line, no job
  end, and RPCs keep going to the backlog. The broker is PID 1 of the
  capsule's PID namespace, so its death also kills the job. That fits a job
  that never ended in the host's view.
- **The broker's own log is not readable on the product route.** Its stderr
  goes to the autoputer's stderr (the journal), and the observability
  recent_log does not include it. No `capsule-broker:` line appears in it,
  not even the ready line at 22:18:42Z. The window from 22:21Z to 23:19Z has
  no broker-side evidence.
- **The job's processes were all gone at destroy:** `teardown memory:
  current=417MiB->15MiB ... file=387MiB anon=0MiB`. That is read after
  cgroup.kill, so it does not say when they died.

## Candidate causes still open (hypotheses, no trace confirms one)

1. **The broker process died** (exited or crashed), and the held listener
   hid it. One concrete path: an engineering capsule has `pids.max` 256
   (`engineeringAssignmentPidsMax`). The job ran vite, then Chromium with
   Playwright, then an autoputer built inside the capsule (choir-shot starts
   one over /computer). These are all thread-heavy, and cgroup v2 counts
   threads. When a Go process cannot create an OS thread, it dies with
   "runtime: failed to create new OS thread", and it needs one whenever a
   goroutine blocks in a syscall and no idle thread is parked. The control
   above (a warm broker under a fork hog) did not die, so this path is
   unproven. The capsule receipt does not record pids.events, so the
   incident cannot settle it.
2. **The broker was alive but unable to serve,** in a way not yet found.
   Every broker-side wait the RPC path reaches is bounded: the session eval
   timer is 60 s, the readiness wait is bounded, and m.mu and j.mu are held
   briefly. The second assignment's capsule ran the same `choir-shot --only
   01` job (with a 1536 MiB boost, so memory.max was 3584 MiB). Its broker
   answered, and the job ended `failed` at 23:32:22Z.

## Next probes (before any substrate change)

- Record the broker's exit: log the launcher's exit status and time when the
  broker dies after readiness, and fail the capsule's broker calls at once
  with "the capsule broker exited: <status>" instead of letting them wait in
  the held listener's backlog. Also end the running job ("the capsule broker
  exited").
- Add `pids.events` (max), `pids.peak` and `cpu.stat` (throttled_usec) to the
  capsule receipt next to memory.events.
- Keep the broker's own stderr lines (a bounded tail per capsule) where the
  observability route can return them.
- The next recurrence then says which candidate it was. If it is pids, the
  substrate change is the same shape as the one asked for here, but for
  pids: the job runs in a child leaf whose `pids.max` is the capsule's limit
  less a reserve for the broker.

## Remaining error field

- Fix (next commit): every engineering cell has a host wall-clock limit
  (5 minutes; the model's timeout_ms plus a 15 s grace when smaller; never
  past the activation deadline less 30 s). The broker read has its own
  bound: go_eval reads for the broker's cell bound plus 15 s (75 s when
  timeout_ms is unset), job verbs for 60 s, and the error says "the
  capsule broker did not answer" and names the job. The model's tool
  result names the running job (not stopped) and the next step. A
  "capsule_cell_running" progress event fires every 2 minutes.
- Residual (root cause, open): the memory-in-the-job's-cgroup cause is
  refuted (memory.events above). The broker died or went silent for a
  cause not yet observed. A dead broker is invisible to the host, because
  the host holds the listener and never reads the exit status. The next
  step is the probes above, not a cgroup split.
- Residual: while the broker stays silent, the host's job watcher cannot
  see the job end, so "the platform wakes you" depends on the broker
  coming back. The cell now answers within about 75 s instead of 60 min,
  and the model can choir.Stop or end the round.
