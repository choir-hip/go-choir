# A failed actor handler is redelivered at once and dead-letters within a second; wake latency is measured from the assignment's open; the first spawn wake waits for management's turn (2026-10-22)

Lane lane/fp4-actor-retry-backoff (FP4, base origin/main 273d139c9).
Problem documentation first: this commit has no code. The evidence is the
SEC-06 run on deploy S, test computer 71afbf14, as recorded in
docs/problems/capsule-coldstart-18s-2026-10-22.md ("The other waits" and
"Not cold-start, but found here" a to c; on lane/fp4-capsule-coldstart).
Each finding is marked **confirmed** (code read, arithmetic on logged
timestamps) or **hypothesis**.

## Evidence (guest console, 71afbf14, booted 16:09:39)

```
16:12:43 actorruntime: engineering admission wake latency mailbox=management:c7f51f0f-… waited=13.479s content={"assignment_id":"assignment-da297e3be55ce27dfdf377a0","attempt":1}
16:13:01 runtime: actor wake latency kind=initial_dispatch target=engineering:assignment-da297e3be55ce27dfdf377a0 waited=31.224s dispatch=1ms
16:13:34 runtime: actor wake latency kind=assigned_engineering_fate_deadline target=management:c7f51f0f-… waited=1m4.1s dispatch=1ms
16:13:34-35 three more fate-deadline lines (1m4.564s, 1m5.131s, 1m5.327s)
16:13:34-35 seven "dispatcher: handle c7f51f0f-…" lines, then "dispatcher: poison c7f51f0f-…"
```

## F1: a handler error is redelivered with no backoff, so a brief conflict is a dead letter

**Confirmed (code, origin/main 273d139c9).**

- internal/actor/dispatcher.go, `activate`: on a handler error that is not
  the last allowed attempt, the dispatcher logs `dispatcher: handle
  <agent>/<update>: <err>` and `break`s. The activation commits what it
  folded before the failure and returns.
- The activation goroutine's deferred `d.Notify()` wakes `Run` at once;
  `dispatchPending` re-reads the projection, the update is still due, and a
  new activation redelivers it. Nothing waits between deliveries.
- `RecordAttempt` counts every delivery; the kernel's `MaxAttempts` is 8
  (internal/actorruntime/adapter.go, `kernelDispatcherOptions`). The eighth
  failed delivery is poisoned: `delivery_failed` to the error sink and the
  update incorporated (dropped).
- Only `ErrDeferUnprocessed` gets a backoff (`deferralBackoff`, re-armed by
  `DeferUpdate` with the attempt rolled back).
- So eight deliveries take as long as eight handler runs plus eight SQLite
  round trips. On staging that was about one second (seven "handle" lines
  and one "poison" line at 16:13:34-35).

**Hypothesis (from the cold-start record):** the poisoned update is one of
the four `assigned_engineering_fate_deadline` wakes on management, whose
handler (`resumeStrandedFateAssignmentIfPending`) races the engineering
activation's own fate and report commits on the assignment's
`LifecycleVersion` CAS. The same conflict was logged at 10:11:20 on deploy J
("terminal report attempt failed … lifecycle command digest conflict"). The
excerpt cuts the line after the agent id, so the update id and error are
not in it; round 4 (lane/fp4-r4) adds the cause to the poison line.

Harm: a conflict that would clear in milliseconds or seconds burns the
whole budget and becomes a dead letter. The fate-deadline wake is the G9
re-drive of a pending fate; losing it leaves the assignment's fate to the
other writer, or to the next wake that happens to come.

Fix to build: a bounded exponential backoff with jitter between
redeliveries of a failed handler, in the dispatcher, keeping the attempt
limit. Constraints:

- other mailboxes are not blocked, and no lock is held while waiting (the
  activation returns and frees its slot; the wait is a timestamp the
  dispatcher reads);
- tape order inside the failing mailbox is kept, as it is today: the
  dispatcher stops at the first failure so a later update is not
  incorporated past an unprocessed earlier one. A per-update `not_before`
  (the deferral's mechanism) would let later updates of the same mailbox
  run past the failed one during the wait, which today never happens for a
  handler error. So the wait holds the mailbox, not the update;
- the crash rule is unchanged: the wait is process memory, so a restart
  (crash or planned) forgets it, and the boot's own rules (crash closes
  restart work by kind, `preBootWorkOccurrence`; a planned restart resumes
  once) decide what is delivered. The durable attempt count keeps the
  budget across a restart.

## F2: wake latency is measured from the assignment's open time

**Confirmed (code + arithmetic).**

- `actorWakeOutbox` (internal/store/lifecycle.go) sets the wake's
  `CreatedAt` to `source.CreatedAt`. For an engineering assignment that is
  the open time, which every later version keeps
  (`lifecycleObject(…, assignmentObj.CreatedAt, now)` in each rewrite,
  internal/store/engineering_assignments.go).
- `sweepActorWakeOutbox` (internal/agentcore/runtime.go) logs `actor wake
  latency … waited=dispatchStarted - wake.CreatedAt`.
- 16:13:01 minus 31.224 s and 16:13:34 minus 1m4.1s are both 16:12:29.9,
  the open commit. `dispatch=1ms` on every line: the wakes went out within a
  millisecond of their mint. The 31 s and 1 m 4 s are the assignment's age.
- The wake's own mint time is the source version's commit time, which the
  outbox row already stores as its `UpdatedAt` (`lifecycleObject(…,
  source.CreatedAt, source.UpdatedAt)`).

Constraint on the fix: the wake body's `CreatedAt` also feeds the crash
rule (`preBootWorkWake(kind, CreatedAt)`) and the body comparison in
`appendActorWakeOutboxes`. Changing the body would change the crash rule
and make a re-derived wake differ from an old row's body (a spurious
`ErrConcurrentStateChange` across a deploy). So the fix reads the mint time
from the row, outside the body, and only the latency line uses it.

Observation, not changed here: `preBootWorkWake` therefore asks whether the
*source* was created before boot, not the wake. A wake minted after a crash
boot by a later version of a pre-boot source is treated as pre-boot work.
On a crash boot every open assignment is closed anyway, so no case is known
where this differs; it is recorded so the crash rule's input is named.

## F3: the first spawn wake waits for management's turn to end

**Confirmed (code + timestamps).** The residual of
docs/problems/fp4-nudge-waits-behind-management-activation-2026-10-22.md.

- The open commit mints the `:spawn` wake (`appendWake(":spawn",
  assignment.Binding.ParentAgentID, …)`, internal/store/lifecycle.go) to
  the casting agent, management.
- The dispatcher runs one activation per mailbox, and management's cast
  cell runs inside its activation's tool loop, so the wake waits for that
  loop to end: 13.479 s here (16:12:43 minus 13.479 s is the open).

Why it is on management's today (read):

- The wake predates the admission mailbox (7953b8407 made the spawn
  obligation atomic with the open; cf7ed5072 added `engineering-admission`
  for the retry, the nudge and the planned-boot resume). The nudge record's
  disposition names it as the one wake left behind, because the store mints
  it and the store cannot see agentcore's mailbox constant.
- The handler (`HandleDelegatedAssignmentSpawnDeadline`) already accepts the
  admission mailbox for the same assignment. Nothing in the saga reads
  management's actor state: it takes `engineeringAssignmentOpenMu`, re-reads
  the row, and writes under the lifecycle CAS.
- The crash rule and the apply hold key on the update kind
  (`delegated_assignment_spawn_deadline` is restart work), not on the
  mailbox.
- Dedup: an update id is derived from (mailbox, kind, content). The open
  wake's content is `{"assignment_id","attempt"}`; every other admission
  wake carries a retry count, a boot or a nudge, so moving the open wake to
  the admission mailbox does not collide with them.

What changes if it moves: the first admission attempt runs beside
management's still-open activation instead of after it. That pair already
exists for every retry and nudge (the nudge record's "Cancel against
admission" residual: both write under the CAS, the loser fails closed). A
second cast in the same management turn can now wait on the executor's
`e.mu` (held for the whole spawn, cold-start record "Adjacent findings")
through `ClaudeDelegateStatus`; before, it never overlapped a spawn. That
is a latency shift for management, not a deadlock: the spawn does not wait
on anything management holds.

Rollback of the move: a wake already minted to `engineering-admission` is
still handled by the reverted binary (the handler accepts that mailbox).

## Plan

Tests first (yellow, failing on main by behaviour), then the fix (red: the
actor runtime substrate and the wake outbox), then this record's
disposition.
