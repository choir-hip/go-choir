# A queued engineering request is not admitted when the live one ends; it waits behind management's own activation (2026-10-22)

Found 10:19 UTC by the self-development monitor, running FP4's staging
drill on the test computer `computer-71afbf1461dffabcce8eeebe558d03bd`
(vm-c5c801f8…). Build a84b39bac, deploy J, FP4 round 2. Problem
documentation only; there is no fix in this commit. The cause is read
from logs and code, and the parts not confirmed by trace are marked as
hypotheses.

## Evidence

FP4 drill step "(4) let A finish; B binds within seconds (nudge), not at
the next minute retry". `scripts/proofs/fp4-selfdev-admission.sh --step 5`:

```
INFO B bound 305.32s after A's terminal_at (a_terminal_at=2026-10-22T10:11:20.297299733Z
     b_bound_at=2026-10-22T10:16:25.618146264Z bound=30s)
FAIL B's co_super_assignment_bound event follows A's terminal_at by 0..30s
```

- A: `assignment-a6fa724300f99ebc10afa93a` (trajectory 76c9cbb8…).
  - Bound at 09:54:42.
  - Reported `completed` at 10:11:20.297, capsule revoked.
- B: `assignment-932f7ac2c41ba20992edb935` (trajectory 17c9e9fb…).
  - Opened at 09:55:35.
  - Bound at 10:16:25.618.
- B was bound only after a planned restart. Rows polled every 5 s show B
  `open`/`unbound` from 10:11:21 through 10:16:10. The host's deploy K
  refresh restarted the guest at 10:15:57 (planned, marker-backed). The
  planned boot logged at 10:16:07: "planned boot: queued engineering
  request assignment-932f7ac2… kept and resumed". B was bound 18 s later.
- Without that restart, nothing in the log shows B would have been
  admitted any sooner.

Guest console (Node B, read-only), runtime lines:

- 09:55:38 to 10:10:48: "engineering admission: assignment B waits behind
  A (live); retry N in …". Retries 1-18 back off 10 s, 20 s, 40 s, then
  1 min. The last one, at 10:10:48, is "retry 18 in 1m0s".
- No admission line after 10:10:48: no "waits behind", no "nudge for B
  found the slot busy", no bind. The retry due at about 10:11:48 left no
  trace.
- 10:11:15 to 10:11:20: four `actor wake latency
  kind=assigned_engineering_fate_deadline target=management:… waited=16m56s
  to 17m1s`, then "dispatcher: handle <owner>" (management takes A's fate
  and report).
- 10:11:20: "assignment A terminal report attempt failed: disposition=bound
  capsule=revoked version=6 err=lifecycle command digest conflict". A
  retry recorded the report at 10:11:20.297.
- 10:11:28 to 10:15:34: one tool loop, iterations 1 to 15.
- 10:15:39: "round 1 of conv:management:…|texture:e0822a85… (B's document)
  stays open … an assignment it opened has an outcome management has not
  read".
- 10:15:39: **"dispatcher: activation wait
  agent=management:c7f51f0f-… waited=4m19s updates=30 slots=2/8
  due_agents=2"**.
- 10:15:57: "host requested a planned restart (platform update 5289387c9…;
  allow_busy=false, interrupting 0 running run(s))".

## Code read (origin/main a84b39bac)

- When an assignment ends, `wakeNextQueuedEngineeringRequest`
  (internal/agentcore/engineering_admission.go:299) arms a one-shot spawn
  wake, due now, for the first queued request.
- Each admission retry is also a spawn wake
  (`deferEngineeringAdmission`, :167).
- Both go through `rt.scheduleActor(..., next.Binding.ParentAgentID, …)`.
  For B, `binding.parent_agent_id` is
  `management:c7f51f0f-1b2a-4d0e-b932-d552bb361c30`.
- So B's admission runs only when management's actor lane delivers the
  update.

## Cause (hypothesis, supported by the log lines above)

- The nudge and the minute retry were armed on time.
- They were queued as updates to management, and management was in an
  activation. It is most likely the 10:11:28-10:15:34 tool loop, reading
  A's terminal report; the console does not name the loop's agent.
  - It began with the 10:11:20 "dispatcher: handle" lines for A's fate
    wakes.
  - It is in neither trajectory's runs list. The next run in A's list is
    Texture on A's document, created 10:15:33, which is where management's
    reading of A would lead.
- Management's next activation waited 4 min 19 s with 30 updates queued
  (the 10:15:39 dispatcher line).
  - `logActivationWait` (internal/actor/dispatcher.go:589-616) measures
    "waited" from the oldest due update in the batch.
  - 10:15:39 minus 4 min 19 s is 10:11:20. That is A's terminal_at, the
    moment the nudge was armed due-now.
  - Slots were not the limit: `slots=2/8`. What held the batch was
    management's own open activation.
- That next activation had still not bound B when the planned restart
  came 18 s later. Its first tool-loop iteration was at 10:15:48. The order
  in which the handler takes the 30 updates is not logged.
- **When an assignment ends, management always has an activation to run:
  reading the report. So the nudge lands behind exactly that run every
  time.** "Within seconds" holds only if management is idle when the
  assignment ends, and it never is.
- Not confirmed: that the 4 min 19 s wait held B's nudge in particular.
  The pre-restart actor queue was lost in the restart. A dispatch-latency
  line for `delegated_assignment_spawn_deadline` would settle it, but none
  was logged before the restart.

## Harm

- A queued engineering request waits as long as management's
  report-reading run takes, on top of the slot being free. Here that was
  more than 4 minutes, and it was cut short only by an unrelated planned
  restart.
- During that wait the computer sits idle on engineering, while vmctl
  reads it as busy and defers deploy refreshes.
  - Guest /health showed `running_runs` 1 from 10:11:30 through 10:15:52.
    The log records changes only, so 1 held for that whole span.
  - vmctl's retry sweep deferred deploy K once a minute until 10:14:57.
- The nudge's stated purpose (fd07ab19f, "B binds within seconds, not at
  the next minute retry") is not met on staging.
- The focused test can't see the delay.
  `TestEngineeringAdmissionSlotFreeWakesTheNextQueuedRequest`
  (engineering_admission_test.go:538) captures the wake when it is
  scheduled and checks that it is due now.
  - It never dispatches the wake through management's mailbox.

## Next probes

1. Log the dispatch latency of `delegated_assignment_spawn_deadline`
   wakes, as `actor wake latency` does for other kinds. One drill then
   shows where the nudge waited.
2. Decide whether admission belongs on management's actor lane at all.
   Admission runs under the saga mutex and does not need management's
   model. A host-side (runtime) wake target would not wait behind
   management's activation.
3. A focused test that holds management in an activation while the
   nudge fires, and asserts B's bind within the nudge bound.

## Cause confirmed (lane/fp4-r3-nudge, 2026-10-22)

The hypothesis holds, in code and in a test.

- Code: the dispatcher runs one activation per mailbox at a time
  (`dispatchPending` skips a mailbox in `d.running`,
  internal/actor/dispatcher.go), and a management activation runs its
  whole tool loop inside `HandleUpdate` (internal/actorruntime/handler.go).
  An update addressed to management therefore waits until the current
  activation returns. The nudge, the backoff retry and the planned-boot
  resume were all addressed to `binding.ParentAgentID`, which is
  management.
- Test: `TestEngineeringAdmissionNudgeDoesNotWaitBehindManagementActivation`
  (internal/agentcore/engineering_admission_dispatch_test.go, 80634bd09)
  runs a real actor dispatcher over a SQLite kernel log and holds
  management's mailbox in one activation while A ends. On main the nudge
  is armed on time to `management:owner-admission`, does not run within
  5 s, and runs 18 ms after management's activation is released.

## What admission needs from management's lane

Nothing. Read before choosing the fix:

- Locks: `HandleDelegatedAssignmentSpawnDeadline` takes
  `engineeringAssignmentOpenMu`, a process mutex, whatever mailbox
  delivers the wake.
- Rows: it re-reads the assignment; the expiry cancel, the terminal
  capsule reclaim and the bind are store commits, each guarded by the
  assignment's `ExpectedLifecycleVersion` CAS.
- Actor state: it returns management's memory unchanged.
- Restart rules: the crash rule (`preBootWorkOccurrence`, the outbox's
  `preBootWorkWake`) and the apply hold key on the update kind
  (`delegated_assignment_spawn_deadline` is restart work), not on the
  mailbox.

## Fix: the second of the two shapes (admission on its own mailbox)

Tests b557b2d64 (failed on main 7448bf899), fix cf7ed5072.

Admission's own wakes (the backoff retry, the nudge, the planned-boot
resume) go to a mailbox of their own, `engineering-admission`
(`agentcore.EngineeringAdmissionMailbox`), with a dispatcher lane of one,
as source polls have. The handler accepts a wake on either that mailbox or
the casting agent's.

Why this shape and not admitting B inline at A's terminal commit:

- `wakeNextQueuedEngineeringRequest` is called from paths that already
  hold `engineeringAssignmentOpenMu` (the trajectory reconcile's cancels)
  and from A's report tool call. An inline admission would re-enter the
  mutex (deadlock) or run B's capsule spawn and bind inside A's report
  commit, with its failures folded into A's fate.
- The mailbox change keeps every rule where it was: admission still runs
  under the saga mutex, after expiry (R1, R4), against the one-slot rule,
  and a crash boot still drops the wake as restart work.

`TestEngineeringAdmissionWakeOffManagementKeepsTheSlotRule` shows a nudge
on the new mailbox still leaves B open and unbound while A holds the slot.

Every admission wake now logs, at the handler: `actorruntime: engineering
admission wake latency mailbox=<mailbox> update=<id> waited=<due to
handled> content=<wake>`.

## Disposition

Repaired for the nudge, the retry and the boot resume. Residual: the
open-time spawn wake is minted by the store in the open commit
(internal/store/lifecycle.go, `appendWake(":spawn",
assignment.Binding.ParentAgentID, …)`) and still goes to management's
mailbox, so a request's first admission attempt waits for the casting
activation to end. Moving it changes the store's outbox target; a request
refused busy is re-armed onto the admission mailbox from then on.

Further residuals (review bunch 51, DeepSeek 2, Devin F4):

- **The emitter's commit boundary.** A wake scheduled inside an actor
  activation is buffered and becomes durable only when that activation
  commits (internal/actorruntime/adapter.go, the `EmissionsFromCtx` branch
  of the dispatch path, about line 423). Hypothesis, reasoned from code
  (internal/agentcore/rlm_reduce.go, about line 1070, passes the
  activation's context down to `wakeNextQueuedEngineeringRequest`) and not
  drilled: a nudge minted from a management-initiated Cancel is emitted
  inside management's activation, so it waits for that activation to
  commit whatever mailbox it targets, and the mailbox move does not remove
  that wait. The staging case is fixed
  because A's terminal report, and so the nudge, is emitted inside A's own
  engineering activation (`commitAssignedEngineeringReport`,
  internal/agentcore/engineering_assignment_fate.go), not management's.
  The drill's "let A finish or Cancel it" step therefore covers only the
  finish branch; the Cancel branch is the uncovered path, and the re-run
  plan lets A finish. A buffered emission also dies if the emitting
  activation's handler errors or defers, and the redelivered report is a
  Replay, which does not re-mint the nudge (`!result.Replay`,
  engineering_assignment_fate.go, about line 1426). B then falls back to
  its own retry chain, up to about 60 s, not a few seconds; B is not
  stranded (review bunch 54, Sonnet F4).
- **Cancel against admission is a new concurrent pair.** A management
  cell-verb Cancel (internal/agentcore/tools_engineering_assignment.go →
  `cancelAssignedEngineering`) does not take `engineeringAssignmentOpenMu`.
  While admission ran on management's mailbox the two could not overlap;
  now they can. Both write the assignment under its `LifecycleVersion`
  CAS, so the loser gets a command conflict. A cancel that loses returns
  the conflict to management's reducer (rlm_reduce.go, about lines
  1070-1072), so management's cell sees an error, and a re-read happens
  only if management re-issues the Cancel; a bind that loses goes through the saga's capsule cleanup, which
  destroys the just-spawned capsule and records the revoke. It fails
  closed: the end state is a lawful fate (cancelled unbound, or bound then
  cancelled), and no capsule is stranded.
- **Known test limit** (review bunch 54, Sonnet F3):
  `TestKernelDispatcherHasTheAdmissionLane` checks
  `kernelDispatcherOptions`, not that `New` uses it, so inlining lane-less
  options back into `New` would leave it green.

Rollback: revert the fix commit. A wake already on the admission mailbox
is then delivered to a handler that drops a mailbox other than the casting
agent's, so the request loses that wake. A planned restart (the deploy
that carries the revert) resumes kept queued requests on management's
mailbox; a crash boot closes them interrupted_by_restart. Either way the
request gets a fate.
