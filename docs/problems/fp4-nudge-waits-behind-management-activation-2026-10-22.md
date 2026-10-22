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
