# An engineering cell that never returns hides until the activation deadline (2026-11-03)

Status: open; cause identified, fix built (see "The cause: a host thread
exit SIGKILLs the broker launcher"). The host-side bound landed
(062c669da). The memory hypothesis is refuted (see "The memory hypothesis
is refuted").
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

## A second occurrence: the broker is dead while its capsule lives (read 2026-11-04 00:01Z)

Deepseek arm, computer-e80d229b0d78c35d47656b8c66426a43, release b8cb1ba3
(without 062c669da), assignment-87f6f9554ea8cb4169f85e8f, capsule
capsule-eb9b6fbf-6f96-5f06-8ccd-614cc82dd97a. Job job-de63a7165f2e
(`node scripts/choir-shot.mjs --only smoke`, 20 min) waited for host memory
and started at 23:32:21Z. The evidence comes from
`/api/runtime/observability?computer_id=...&memory=detail`:

- The capsule's cgroup holds `anon_bytes 0`, with 193 MiB of page cache
  only. No process in it holds memory, so the broker, node and Chromium are
  all gone, but the capsule was not destroyed.
- The guest's process list has no broker, node or chromium.
- `steps.in_flight` shows `cell.go_eval` from 23:51:01Z, still waiting
  after 9 minutes or more: a re-dial into the held listener's backlog.
- The capsule's last activity is an egress line at 23:32:39Z. The next
  guest line is `agentcore: cache return mode=ceiling ... steps=19` at
  23:32:53Z.

Candidate 1 is confirmed: the broker died, and the host's held listener hid
the death.

## The cause: a host thread exit SIGKILLs the broker launcher (confirmed in code)

- The host starts the broker launcher with `Pdeathsig: SIGKILL`
  (executor.go, startBrokerLocked). The launcher starts the broker with
  Pdeathsig as well.
- Linux sends Pdeathsig when the parent THREAD that forked the child exits,
  not when the process exits (prctl(2); golang/go#27505).
- Go ends an OS thread when a goroutine that called `runtime.LockOSThread`
  returns without unlocking. The guest does this on purpose:
  - guestmem's cache return (reclaim.go: "Never unlocked: the thread ends
    with this goroutine"; it lowers the thread's priority);
  - tapestore compaction (lockLowPriorityThread).
- If such a goroutine runs on the M that forked the launcher, the thread
  ends with it. The launcher is SIGKILLed, the broker is killed by its own
  Pdeathsig, and the whole capsule PID namespace dies with it. This leaves
  the observed state: an empty cgroup, `oom_kill 0`, no log line, and a
  listener the host still holds.
- **The trigger.** ab1e95c04, deployed at about 22:11Z on 2026-11-03, made
  the guest memory ceiling act through a live capsule on test arms
  (through_work). Before it, the cache return stood aside while any capsule
  was live. Since then it runs about once a minute during jobs:
  - crouton: 22:21:44, 22:22:44 and 22:23:44, during job-d807dd9c4be9;
  - deepseek: 23:32:53, 32 s into job-de63a7165f2e.

  Each run ends one thread. Only the thread that forked the launcher is
  fatal, so a death is likely but not certain. That fits the second crouton
  capsule, whose broker survived its own cache returns.
- **Test.** internal/childthread's
  TestPdeathsigFiresWhenTheStartingThreadExits reproduces the kill without
  privileges: a child started from a thread that then ends is SIGKILLed.

## The fix (this change)

- **`internal/childthread.Start`.** It forks the child from a goroutine
  locked to its OS thread and keeps that goroutine until the child exits.
  It waits with waitid(WNOWAIT), which does not reap, so the caller's
  cmd.Wait is unchanged. Every Pdeathsig child the guest starts uses it:
  - on the host: the broker launcher, desk session workers, doctool
    helpers and the seccomp self-test;
  - inside the capsule: the broker (from the launcher), the session
    worker, the exec verb, job supervisors and the Claude delegate.

  The capsule-job supervisor forks its command with raw syscall.ForkExec,
  and that process never ends a thread, so it is left as it is.
- **Reclaim and compaction threads.** They still end their thread. That is
  the safer option: the thread's nice level and IO class were lowered, and
  unlocking would hand those lowered attributes back to the scheduler's
  pool for unrelated goroutines. With every Pdeathsig child on a held
  thread, ending a thread is harmless again. A dedicated long-lived
  low-priority thread would also work, but it is not needed.
- **The probes**, so the next death is diagnosed and handled:
  - The broker's exit is watched after readiness (broker_exit_linux.go).
    The executor logs the status, the signal and the broker's last lines.
    Every broker call then fails at once with "the capsule broker exited
    (<status>)", a call in flight included. The host's listener copy is
    closed, and the job watcher ends the running job as `failed` with the
    broker's death, which wakes the run.
  - The capsule receipt adds pids.max, pids.peak, pids.events max, cpu.stat
    nr_throttled and throttled_usec, and the broker's exit when it died.
  - The broker's own stderr is kept per capsule: the last 200 lines, plus
    the first fatal lines. The observability route returns it as
    `capsule_broker_log`.
- **pids.max.** Engineering and Claude-mode capsules go from 256 and 512 to
  1024. 256 was a size default from 7dfe782d3 (2026-08-08), with no
  recorded security reason. Memory still bounds a thread bomb.
- **The job clock was checked; no change.** A job's `minutes` count from its
  start in the broker (capsulejob Manager.Start). A host-memory wait runs
  before job_start, so the wait does not eat the job's budget.

## Remaining error field

- Fix (next commit): every engineering cell has a host wall-clock limit
  (5 minutes; the model's timeout_ms plus a 15 s grace when smaller; never
  past the activation deadline less 30 s). The broker read has its own
  bound: go_eval reads for the broker's cell bound plus 15 s (75 s when
  timeout_ms is unset), job verbs for 60 s, and the error says "the
  capsule broker did not answer" and names the job. The model's tool
  result names the running job (not stopped) and the next step. A
  "capsule_cell_running" progress event fires every 2 minutes.
- Root cause: a host thread exit SIGKILLed the broker launcher through
  Pdeathsig (above). The fix and the probes are built. Acceptance is on
  staging: a test arm runs a choir-shot job through several cache returns,
  and its broker answers until the job's own end.
- Residual: while the broker stays silent, the host's job watcher cannot
  see the job end, so "the platform wakes you" depends on the broker
  coming back. The cell now answers within about 75 s instead of 60 min,
  and the model can choir.Stop or end the round.
