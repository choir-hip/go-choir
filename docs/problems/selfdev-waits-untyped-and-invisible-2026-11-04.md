# Self-development waits are untyped, and most are invisible (2026-11-04)

Status: open. Mutation class of the fix: red (the agent loop's tool
results, management's cell frame, a platform record to Texture's inbox
through the engineering progress relay). The inventory and design in this
doc are green.

Owner ruling, 2026-11-04 ~01:00Z: "no stopping until we eradicate the
waiting except for true resource contention. and even then, that should be
semantically exposed to eng, mgmt, and texture."

## Evidence (night of 2026-11-03)

- Build jobs waited 6 to 22 minutes for host memory. Each wait was one
  free-text `reason` on a job in state `waiting`. Management and Texture saw
  nothing.
- Broker calls hung for 34 to 48 minutes. They are now bounded (062c669da).
  The root cause, the broker being killed with its job's cgroup, is in
  review on branch broker-own-cgroup.
- The refresh-between-rounds hold kept new rounds waiting for up to 20
  minutes (3bc6a794c, `refreshHoldMax`).
- The engineering admission queue logged `live` and `queued_ahead` waits
  only to the guest log.
- A delegate wait-park job (`choir-delegate wait --id X --park`, run with
  choir.Start) uses almost no memory. It declared no `memory_mib`, so it
  claimed a full build peak (3 GiB) and waited for host memory like a
  build.
- A turn parked on a job in state `waiting`. The park events carry `cause`
  and `reason`, but not what the job is waiting for. On resume the
  instruction gives only the job's outcome.

## Part 1: inventory

Scope: internal/agentcore, internal/capsule, internal/vmctl,
internal/autoputer, internal/yaegikernel, internal/claudedelegate, plus
cmd/choir-delegate. Lines are at origin/main 2cb0e1687 + f55111291.

Classes:

- **a**: true resource contention: host memory against other jobs, a build
  slot, a provider rate limit, a genuinely serial resource.
- **b**: an artificial or policy wait: a refresh hold, a whole-life
  reservation, a fixed backoff, a poll interval, a queue policy.
- **c**: a hang risk: a wait with no bound of its own.

"Surfaced" says whether the reason reaches the model, management or
Texture as data, not just as a log line.

### Self-development path

| file:line | waits for | bound | class | surfaced |
|---|---|---|---|---|
| capsule/jobs.go:136-148, 227-286 `waitForHostMemory` | A build-class job's start, until host PSI clears and the headroom covers its need beyond its capsule's reservation credit | Polls every 5 s (`hostMemoryWaitPoll`), stops at 30 min (`hostMemoryMaxWait`) | a, with a b poll | Free text only: `Status.Reason` plus `jobHoldersNote` (this computer's jobs). Other computers' holders are not named. |
| capsule/jobs.go:258-260 | A waiting job whose capsule is frozen or quiescing (`continue`) | 30 min cap | b | The reason from the previous tick stays, so the text is stale |
| capsule/job_shot_default.go:157 + capsule_size.go:131-143 | A job with no `memory_mib` claims a full build peak (3 GiB), including a pure waiter such as `choir-delegate wait --park` | as above | **b (artificial)**: the waiter needs no host memory | The text hints `memory_mib` |
| capsule/jobs.go:117-128 | One job per capsule: a second start is refused while the first runs | does not wait | b | Error text |
| capsule/executor.go:495-507, autoputer/capsule_host_memory.go:114 `AdmitBuild` | Host build admission at capsule spawn. The reservation is held for the capsule's whole life. | 10 s per call | a at admission; b for the whole-life hold (branch reserve-per-job is changing this) | `HostMemoryRefusal{Reason}` text |
| agentcore/engineering_admission.go:290-306, 329-336 | A spawn the host refused for memory: the request stays open and retries | 30 min (`engineeringHostMemoryMaxWait`); retries 30-60 s | a | One ledger record to management with the host's text (engineering_admission_hold.go:103). Texture is not told. |
| agentcore/engineering_admission.go:219-224 `live` | Another assignment holds the computer's one engineering slot | Assignment deadline; retries 10 s to 1 min | b: the doctrine I26 sequential phase | Log only |
| agentcore/engineering_admission.go:225-230 `queued_ahead` | An older open request goes first (FIFO) | Assignment deadline | b: queue policy | Log only |
| agentcore/engineering_admission.go:232-245 `candidate_pending` (carried_patch.go:403) | The trajectory's newest candidate awaits its decision | Assignment deadline; the owner's decision has no bound | b | Log only |
| agentcore/engineering_admission.go:247-256 `operation_open` (carried_patch.go:418) | Any open self-development operation on the computer | Assignment deadline | b | After 3 retries, a ledger record to management (engineering_admission_hold.go:50) |
| agentcore/engineering_admission.go:238-245 `carry_unreadable` | A store read error, retried as a wait | Assignment deadline | b: an error dressed as a wait | Log only |
| agentcore/engineering_admission.go:258-271, refresh_hold.go:124-145 `refresh_pending` | A deploy refresh the host asked for | 20 min per episode (`refreshHoldMax`); lapses 5 min after the last ask | **b (artificial)** | One ledger record per attempt and target (refresh_hold.go:156) |
| agentcore/engineering_assignment_runtime.go:516-519, 590-602 `source_settling` | The booted base's fetch, or an apply in flight | Assignment deadline | b | Busy text only |
| agentcore/engineering_admission.go:128-141 | The backoff between admission retries | 10 s doubling to 1 min, no retry cap | b: poll | Log |
| agentcore/capsule_jobs.go:342-358 `waitJobEnd` (park, 400-456) | A turn parked on its job's end | `RunningJob`'s deadline (jobs.go:365-369) plus 1 min. For a job still `waiting` the deadline starts now, so a park on a waiting job ends before the job starts, as "outlived". | b | `capsule_job_park` events; the wait's cause is not carried |
| agentcore/capsule_jobs.go:97 | The executor's StartJob and StopJob | 2 min | b | Error text |
| agentcore/engineering_cell_bound.go:117-142 | One engineering cell | 5 min host bound (062c669da) | b | Yes, on timeout |
| agentcore/tools_desk.go:203-224 `acquire` | A desk's one REPL session, busy with another run's cell | The caller's ctx only | a (a shared session); c if the ctx has no deadline | Error text on ctx end |
| capsule/broker_client.go:164-236 | Broker RPCs | 5 s dial; 60 s job verbs; cell bound + 15 s for go_eval | bounded (was c until 062c669da) | `ErrBrokerNoAnswer` |
| capsule/claude_delegate.go:149-167 | Gateway attestation after a reboot (409) | 12 s total | b | `ClaudeDelegateStatus.Reason` |
| cmd/choir-delegate/main.go:89-115 `wait` / `wait --park` | The delegate run's end | 25 s, or 30 min / 45 min with `--park` | b: a poll by design | Prints its status |
| claudedelegate/server.go:410, 459-460 | The delegate run's wall clock | `WallClockMin` | bounded | `Status.Reason` |
| agentcore/continuation_schedule.go:217, engineering_assignment_fate.go:552, inbox_records.go:766 `engineeringAssignmentOpenMu` | The spawn saga mutex, held across spawn and bind | Unbounded `Lock`; only as long as the holder | a (a serial saga); c if a spawn under it hangs | No |
| agentcore/self_development_materializer.go:173, 397 `selfdevMaterializeMu` | Materialize or apply | Unbounded `Lock` | a; c if an apply hangs | No |
| agentcore/selfdev_settle_open.go:508-512 | A settle's TryLock on the materializer | 3 s | b | 409 "try again" |
| agentcore/hibernate.go:162-182 | Work admission during a hibernate fence | The fence's expiry | b | Log |
| agentcore/self_development_restore_restart.go:156, 184-190 | A restart into a restored release: a sleep, then an idle poll | 1 h window; `time.Sleep` ignores shutdown | b; c for shutdown | Returned text |
| agentcore/run_end_retry.go:16-19, 57 | Run-end write retries | 11 fixed delays (about 15 min); `time.Sleep` | b; c for shutdown | No |
| agentcore/memory_cache_return.go:322 | 60 s sleep after a reading | `time.Sleep`, no ctx | b | No |
| agentcore/inbox_records.go:969-972 | The management task queue's free slot | A slot-freed wake | b | No |
| agentcore/engineering_assignment_fate.go:1314-1322 | The dispatcher's deferral backoff, outside MaxAttempts | Backoff | b | No |
| agentcore/support_judge.go:60, 128-136 | A support judge slot | 2 min, then dropped | a | Log |
| agentcore/intake_labels.go:203-220, research_children.go:69-176 | Provider and child-research semaphores | ctx | a: provider concurrency | No |
| vmctl/capsule_reservations.go:348-386 | Build admission: refuses at once, never blocks | n/a | a | `Reason` text with totals; holders are not named |
| vmctl/capsule_reservations.go:164-172 | A build set's hold until it stops being reported | 15 min TTL | b: a whole-life hold (reserve-per-job) | Health view |
| vmctl/host_admission.go:255-270 | Start reservations through the settle window | 5 min settle, 15 min TTL | b | Health view |
| vmctl/host_admission_make_room.go:247 `makeRoomMu` | One make-room decider at a time | Unbounded `Lock` | a; c risk | No: nothing records the holder or since when |
| vmctl/host_admission_make_room.go:494-512 | Freed memory to appear after a hibernate | 10 s, 250 ms poll | a/b | The decision record |
| vmctl/ownership.go:1366-1376 | Lifecycle claim: refuses, never blocks | n/a | a | The holder's name in the error |
| vmctl/deploy_refresh_retry.go:44, 84-95, 533-546 | The deferred-refresh sweep | 1 min, backing off to 1 h after 30 tries | b; c for an entry that never clears | `Pending` entries |
| vmctl/ring_walk.go:609, 755 | A ring release waiting for healthy exposure | No time bound | b | `Waiting` text |
| autoputer/capsule_host_memory.go:194-205 | The reservation report heartbeat | 5 s; 10 s per call | b: poll | Facts kept for jobs |
| yaegikernel/jobs.go:215-232 `Poll` | Never blocks: returns the cell's snapshot and records the job as awaited | n/a | b | `State` plus `Reason` (a free string) |
| yaegikernel/worker_mux.go:146-160 | A broker response frame inside a cell | ctx only; ends early if the socket closes | c if the cell has no deadline (the host cell bound covers engineering) | Error text |

Counts, self-development path, rows above:

| class | count |
|---|---|
| a, true contention | 10 (host memory jobs and spawn, spawn admission, host build admission, the desk session, the saga and materializer mutexes, provider and judge slots, lifecycle claim, make-room settle) |
| b, artificial or policy | 31 |
| c, hang risk (also counted in a or b) | 9 (the two saga mutexes, makeRoomMu, desk `acquire` without a deadline, three `time.Sleep` loops that ignore shutdown, the cell response frame, a refresh entry that never clears) |

Of the waits a self-development round actually spent time in tonight, only
host memory is class a. The refresh hold, the whole-life build reservation,
the full-peak claim of a pure waiter, and the admission retry cadence are
class b.

## Root cause

There is no wait substrate. Every wait point formats its own sentence: a
job `Reason`, a busy refusal's `BlockedBy`, three hand-written ledger
records (host memory, refresh, operation hold), and log lines. So:

1. The model cannot act on a wait. A `Reason` string does not say what is
   needed, what is free, who holds it, or when the next check is. The model
   cannot weigh Stop against waiting against other work.
2. Management sees three of the eleven admission waits, as prose, and never
   a job wait. It cannot tell "blocked on host memory" from "stuck".
3. Texture sees none of them. The engineering progress relay
   (engineering_progress_relay.go) carries only engineering's own Reports.
4. No one can attribute cycle time. A round's minutes waited by cause are
   not recorded anywhere.

## Part 2: design. One typed wait record.

`types.WaitState` (internal/types/wait_state.go) is the single structure:

```
WaitState{
  kind            host_memory | engineering_slot | queued_ahead |
                  candidate_pending | selfdev_operation | platform_update |
                  source_settling | read_error | ...
  class           contention | policy
  subject         what waits ("job j-…", "assignment a-…")
  resource        "host memory", "the engineering slot", ...
  need, have      plain text, plus need_bytes and have_bytes for memory
  holders[]       {who, what, bytes, since}
  since           when this wait began
  until           when the wait gives up (its bound), if it has one
  check_every_s   how often it is re-checked
  detail          the wait point's own text (for example the host's reason)
}
```

`WaitState.Sentence()` renders it as one plain sentence. For example:
"waiting for host memory: this job needs 3072 MiB; 600 MiB is free beyond
the floor; held by job j-1 (job_lease, 1536 MiB) and another computer
(job_lease, batch, 3072 MiB); checking every 5s; waited 4m; gives up in
26m".

`types.WaitTally` sums the time waited by kind. It renders as "waited
12 min: host memory 9 min, platform update 3 min".

Every class a and class b wait that stays produces a WaitState. The record
surfaces in three places.

1. **Engineering.**
   - `JobView.Wait` carries the record. The choir.Start result carries it
     under `wait`, with the sentence under `waiting`.
   - choir.Poll returns it on the job.
   - A park on a waiting job emits it on `capsule_job_park`. The park's
     deadline then covers the job's wait bound and its wall clock, not the
     wall clock from now.
2. **Management.**
   - The runtime keeps one tracker per assignment: the current WaitState,
     plus the totals by kind.
   - Admission sets the tracker from the busy refusal. Admission clears it
     when the request binds.
   - The capsule executor's job-wait hook sets and clears it for job waits.
   - Management's cell frame (`ActingPack.Waits`, read by choir.Pack())
     lists each open assignment's current wait and its minutes by kind.
3. **Texture.**
   - A wait that has lasted a minute mints one platform Note into the inbox
     of the Texture the assignment serves. It goes through the engineering
     progress relay's path and is gated by the same setting
     (`engineering.progress_to_texture`). For example: "Waiting 1 min for
     host memory held by other computers' builds".
   - Its record id is per wait episode (`progress:wait:<assignment>:<attempt>:<n>`),
     so further checks append nothing. That is the "in place" property on
     an append-only ledger.
   - When the wait ends, one closing Note follows: "Waited 9 min for host
     memory; the build started". It carries the round's totals by kind in
     its `subject_refs`.
   - Texture remains the only writer of the document (I2). It renders the
     Notes or ignores them.
   - When the assignment ends, the round's "minutes waited by kind" go
     into a system record addressed to management, related to the
     assignment, beside its report. The report struct itself is not
     changed: it is store-validated, and the data system cutover is
     rewriting the store.

## Part 3: what is removed or bounded

- Removed (b): a pure waiter job no longer waits for host memory. Examples
  are `choir-delegate wait` and `sleep`. They hold no meaningful memory, so
  the full-peak claim was artificial.
- Bounded (c): a park on a job still waiting for host memory gets a
  deadline that covers the wait bound plus the job's wall clock.
- Left alone, and rendered through WaitState instead:
  - The whole-life build reservation. Branch reserve-per-job has since
    replaced it with per-job leases (098468166). The job wait record now
    renders vmctl's lease refusal (`memory_wait`) and derives nothing
    itself.
  - The broker lifecycle. Branch broker-own-cgroup owns that.
  - The refresh hold. Its expected wait is one vmctl sweep (one minute),
    and the 20-minute bound fires only when a refresh fails to land.
    Removing it brings back the starvation in
    docs/problems/self-opened-rounds-starve-the-deploy-refresh-2026-11-03.md.
    It now shows as kind `platform_update` with `until` set to its bound.
- Named residuals:
  - The class c rows, checked in Part 4 below.
  - Another computer's holders are rendered as "another computer"
    with their kind, class and bytes, as vmctl sends them. No id is
    included.
  - When a new attempt of an assignment starts, one in-flight job-wait
    event can be attributed to the wrong attempt. Separately, an
    assignment's close can race an admission observation, which can
    leave a stale entry. That entry is reaped after 6 h. Both are review
    round 2, F1 and F3.

## Part 4: the nine class c rows, checked (2026-11-04)

Owner ruling, 2026-11-04: "eradicate the waiting except for true resource
contention, and expose it to eng, mgmt and texture." Each class c row was
read against the code at origin/main 751eb988a: who holds it, for how long,
and whether a waiter can block without end in practice.

| # | row | real? | evidence | bound |
|---|---|---|---|---|
| 1 | `engineeringAssignmentOpenMu` (continuation_schedule.go, engineering_assignment_fate.go, inbox_records.go) | yes | The holder is a spawn saga or a trajectory reconcile. The spawn is bounded at 90 s, but the saga's store and tape calls run under the actor handler's context, and the actor dispatcher gives handlers no deadline. A wedged append (seen before: the comment on `failSelfdevOperation`) holds the lock, and every admission wake, inline spawn and reconcile waits behind it, with nothing naming the holder. | A holdlock waiter gives up at 5 min, naming the holder and its hold time. An assignment's wait past 2 s is its typed wait, kind `lock`, class contention. Management's frame and Texture's progress relay show it, through the existing tracker. |
| 2 | `selfdevMaterializeMu` (self_development_materializer.go) | yes | The post-commit observer runs in its own goroutine for every committed event on the computer. The drain cleared its pending flag before each pass, so each event during a pass launched another drain, which then blocked on the pass's lock. An apply that takes minutes piles up one goroutine per event, and each of them runs a whole pass afterwards. The crash boot's marking also waited out a whole pass, the apply included. | One drain runs at a time (a running flag), and a trigger during a pass only marks it pending. The pass's wait is bounded at 5 min and names the holder. The abandon settle's 20 ms TryLock poll becomes one bounded wait. The crash marks get a mutex of their own, so marking never waits for a pass. |
| 3 | `makeRoomMu` (vmctl/host_admission_make_room.go) | yes, latent | `admitHostMemory` calls make-room with `context.Background()`, so a start that waits behind another start's make-room has no bound but the holder. The holder runs up to 10 hibernates, each with a settle of up to 60 s. | A waiter gives up at 5 min, naming the holder ("the make-room of a start of vm-…"). |
| 4 | desk session `acquire` (tools_desk.go) | partly | The caller's context is the activation's, and `ExecuteActivationSync` always sets the activation budget (60 min). So the wait was bounded, but only at an hour. The real defect was a different one: when a worker died mid-cell, `release` deleted the session without closing its `freed` channel. A run waiting for that session then waited out its whole activation for a session that no longer existed. | `close` wakes the waiters. A waiter gives up at 10 min, naming the run whose cell holds the session and for how long. |
| 5 | restore restart `time.Sleep` (self_development_restore_restart.go) | yes, for shutdown | The 2 s delay and the 15 s idle poll, with a window of up to 1 h, slept through `Stop`, and then restarted the guest from a stopped runtime. | `sleepOrStop` ends the sleep at `Stop`, and the goroutine returns. |
| 6 | run-end retry `time.Sleep` (run_end_retry.go) | yes, for shutdown | About 15 min of retries wrote to a stopped store. | The wait reports the stop, and the retry returns its last error. |
| 7 | cache-return post reading `time.Sleep` (memory_cache_return.go) | yes, for shutdown, minor | A 60 s sleep, then a read of /proc after `Stop`. | `sleepOrStop`. |
| 8 | broker frame read (yaegikernel/worker_mux.go) | no | The worker evaluates under `context.Background`. Its bound is the host: `EvalFrame` ends at its context (the activation budget, or the cell's bound), kills the worker, and the socket closes, which wakes every waiter. | Unchanged. A test pins that a waiting call returns when the socket closes. |
| 9 | deploy refresh entry (vmctl/deploy_refresh_retry.go) | yes | An entry stays until its computer takes the update. A refusal that never clears was asked hourly for ever. An entry whose route refused was kept without counting an attempt, and so was asked every minute for ever. That held even after the computer served the target, because the route check came before the served check. | An entry lives 24 h from its first deferral of its target. Past that, the sweep forgets it, logged, and the next deploy records it again. The served check runs before the route check, so an entry is cleared when its condition resolves. |

Real: 1, 2, 3, 4 (as a lost wake-up, not as a missing deadline), 5, 6, 7 and
9. Not real: 8.

Mechanism: `internal/holdlock` is a one-slot channel semaphore with a holder
record. It replaces `sync.Mutex` at rows 1 to 3. `Acquire(ctx, lock, who,
max)` returns a `HeldError`: "waiting for the self-development materialize
lock held by a materializer pass for 4m10s: gave up after 5m0s: context
deadline exceeded". `types.WaitKindLock` is the typed wait for an
assignment that waits on the open lock.

No lock was deleted. Each one guards a real serial section:

- the open lock: one saga per spawn window (a duplicate wake would write
  divergent fate intents);
- the materializer: one pass moves an operation at a time;
- make-room: one decider, so no computer is hibernated twice.

What was deleted:

- the pile of drains behind the materializer's lock;
- the crash marking's wait for a pass;
- the abandon settle's sleep poll.
