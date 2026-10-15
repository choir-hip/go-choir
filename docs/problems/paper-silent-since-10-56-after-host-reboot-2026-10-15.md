# Paper computer made no inference calls for 1h45m after the host reboot (2026-10-15)

Date: 2026-10-15, observed 12:39 UTC. Paper computer
`vm-1669020adcece51c2f5ff9b6df399ef5` /
`computer-1355b83bced1613eb048ee28d66688a8`. Status: documented; hypothesis
only, not investigated further by the lander.

## Observed

- Gateway `inference succeeded for autoputer vm-1669…` per UTC hour:
  08h 409, 09h 598, 10h 136, last call 10:56:07, then none through 12:39.
- Newest stories on `/api/platform/publications/front`: `updated_at`
  10:55:43 and 10:53:12; the next older are 09:40. So the paper published
  nothing after 10:56.
- Paper boots: 10:51:10 (host reboot, planned), 11:20, 11:37 and 12:22
  (deploy refreshes, each shown as "VM is unhealthy" for about 12 s at start).
  The paper's own `/health` is `ready`, `running_runs 0`, build current. After
  each boot the console shows `source_schedules_after_restart` ending in about
  25 ms and then no further runtime lines for a source poll.
- Staging /health is ok; disk 37%; the paper is running.

## Hypotheses (unverified)

1. The standing source schedule did not resume after the host reboot or after
   the deploy-time restarts: boots with no planned-restart marker close
   in-flight work ("Restarts End Work"), and `recoverSourceSchedulesAtBoot`
   should re-arm the schedule's next tick; the 25 ms phase may mean it found
   nothing to arm, or that the schedule was closed with a cause after two
   consecutive restart-crashes (the 10:51 reboot plus 11:20).
2. The 11:20, 11:37 and 12:22 restarts kept resetting the poll interval
   (about 10 min) before it could fire.

Needs the paper's schedule record and the run ledger (product route or the
paper's tape), not the host journal; the Landing Loop's "paper healthy" check
(health + disk) did not catch this because the computer is healthy but idle.

## Confirmed (coordinator, 12:40 UTC)

- Read with the paper credential (states and timings only):
  `GET /api/autopaper/sources/schedules` listed 28 schedules, all `active`.
  **26 were `overdue`.** Their `next_due` values were 10:04:38–10:05:31 UTC
  with `poll_in_flight_since` zero, so each schedule's saved tick fell
  inside the platform Dolt outage (about 09:49–10:47;
  [deadlock doc](platform-dolt-deadlock-hangs-checkpointd-and-crashloops-vmctl-2026-10-15.md)).
- The paper's console from 09:56:06 to 10:47:37 shows a poison loop:
  `actorruntime: handler: unknown update kind "delivery_failed" for agent
  computer-1355…` and `dispatcher: poison e5962310-…`. `delivery_failed`
  has no handler, a known residual from the raced-activation work.
- All five boots after that (10:50 host reboot, 11:05, 11:20, 11:37, 12:22)
  were `restart kind=planned`. `recoverSourceScheduleAfterPlannedBoot`
  (internal/agentcore/source_schedule.go) re-arms the saved tick through
  `armSourcePoll`, which "collapses into the pending wake". The planned
  path never moves `next_due` or the generation, and a lost wake is not
  re-created. So the schedules stayed active and overdue with nothing
  queued, through every boot.
- Hypothesis 1 (no re-arm after a crash) is refuted: no boot was a crash.
  The cause is wakes lost during the Dolt outage, plus a planned-boot
  re-arm that does not repair a wake already marked delivered or
  poisoned. The exact wake disposition (poisoned, or deferred and dropped)
  is not read; it would need the actor mailbox rows.

## Recovered (owner act, 12:42–12:49 UTC)

The coordinator restarted the 26 overdue schedules with
`POST /api/autopaper/sources/schedules`, keeping each source's interval and
max_items, staggered 15 s apart. The reason recorded was "owner act: re-arm
after wakes lost in the 2026-10-15 platform Dolt outage". Generations went
up by 1, and first polls were due 12:42:45–12:49:15. Result: the gateway
logged 202 paper `inference succeeded` calls between 12:41 and about 12:52.

## Defects to fix (not fixed)

1. `delivery_failed` has no handler. A failed source-poll wake is
   poisoned, and nothing re-arms the schedule.
2. A planned boot re-arms an overdue schedule's saved tick into whatever
   wake the actor thinks is pending. It should re-arm any schedule more
   than one grace period overdue as a new tick, so a lost wake is
   re-created.
3. Detection: the list's `overdue` flag existed and nobody read it.
   Add a paper liveness check that alerts when 2 or more schedules are
   overdue, or when no paper inference has happened for 30 minutes, to
   the Landing Loop's "paper healthy" step and as a host timer.

## Failure modes considered

Written before the fix (branch `fix/paper-schedule-lost-wake`). Each is a way
the planned-boot repair, the runtime sweep, or the `delivery_failed` handler
could make the paper worse than the silence it repairs. The tests name them.

Design in one paragraph: a schedule that is active, past
`sourceScheduleOverdueGrace`, and has no live poll in flight is re-armed as a
new tick: `Generation++`, `NextDue = now + i*15s`, saved, then armed. A wake
carries its generation, and `HandleSourcePollDue` refuses a wake whose
generation or `NextDue` is not the saved one, so any stale or poisoned wake of
the old chain is dead. The wake's update id is derived from its content
(source, generation, due), so the new tick is a new tape row, never a collapse
into the lost one. Two callers: the planned-boot recovery (once, at boot) and a
periodic sweep (every few minutes while the runtime lives). Crash-boot
semantics are unchanged.

1. **Double polls.** The repair must never leave two live chains for one
   source. Guard: the generation bump happens in the same `sourceScheduleMu`
   critical section as the overdue judgement, and the old wake's handler
   re-reads the schedule under that mutex before marking the poll in flight,
   so it sees the new generation and returns. The only residual is an old
   poll that is genuinely still running past the abandoned-mark bound (1 h);
   it finishes with `live=false`, records usage, and arms nothing.
2. **Stagger storm.** 26 schedules re-armed together would poll 26 feeds in
   one second and then stay phase-locked, repeating the burst every interval.
   Guard: tick `i` of one repair pass is due `now + i*15s`, so 26 sources
   spread over 6.5 minutes. Residual: the spread persists as each source's
   own interval re-arms it; that is the intended steady state.
3. **Sweep racing a live poll.** A poll marks `PollInFlightSince` under the
   mutex and records its result under the mutex. The sweep skips any
   schedule with a fresh in-flight mark, and a poll that finishes after the
   sweep re-armed sees `Generation` changed, so it records usage and arms
   nothing. A wake that is delivered but waiting on the mutex while the sweep
   runs is refused by generation.
4. **Generation churn.** A sweep that fired every pass would bump the
   generation forever. Guard: a repaired schedule is due in the future
   (`now + stagger`), so it is not overdue past the 15-minute grace again
   until a full grace has passed with no poll. A schedule whose arm keeps
   failing is retried at most once per grace window, not once per sweep.
5. **Crash-loop interaction.** The repair must not hide a poll that crashes
   the computer, and must not count a planned boot as a crash. Guard: the
   repair never touches `CrashesWithPollInFlight`; a repaired tick that
   crashes the computer is a crash boot with the in-flight mark, counted and
   closed at `sourceScheduleCrashLoopLimit` exactly as before. A schedule
   closed with that cause is not active, so neither the planned-boot repair
   nor the sweep resurrects it (an owner act restarts it).
6. **Sweep during a Dolt outage.** Every list or put fails for an hour. The
   sweep must back off, not spin or flood the log. Guard: a failed pass
   doubles the wait before the next pass (5 min, 10, 20, 40, capped at 1 h)
   and logs one line per failed pass; the first failed save ends the pass, so
   26 schedules cost one failed write, not 26; a clean pass resets the wait.
   A pass makes no model call and polls nothing itself.
7. **Stuck in-flight mark.** A panic or hung handler can leave
   `PollInFlightSince` set with no poll running; skipping such a schedule
   forever is the same silence. Guard: a mark older than one hour is treated
   as abandoned and the schedule is repaired (see 1 for the double-poll
   bound).
8. **Non-active schedules.** Stopped, `interrupted_by_restart`, or a zero
   `NextDue` are owner or crash-loop decisions; the repair ignores them.
9. **`delivery_failed` poisoning.** The kind is emitted by the dispatcher into
   the `delivery-poison` sink mailbox. Today the sink's default branch logs
   `unknown update kind` and consumes it, so it is already dead-lettered once
   but looks like a handler bug and records nothing about what died. Guard:
   an explicit handler logs one line naming the failed update and its
   mailbox, never returns an error (an erroring sink handler would itself
   retry), and leaves a source-poll wake's recovery to the sweep. Open
   question, not decided here: whether the failed wake's own handler should
   re-arm on error (it cannot while the store is down, which is the case
   that matters), so the sweep is the single recovery owner.
10. **Detection false alarms.** The host timer must not page on a healthy
    paper that is legitimately quiet (feeds with no new items make no model
    call) or on a paper that is mid-deploy. Guard: it alerts on "no inference
    for 30 minutes", logs to the journal only, and does not restart anything.

## Fix status (branch `fix/paper-schedule-lost-wake`, not landed)

1. Planned-boot re-arm: a schedule more than the 15-minute grace overdue is
   re-armed as a new tick (generation+1, `next_due = now + i*15s`).
2. Runtime sweep: every 5 minutes, same repair for an active overdue schedule
   with no live poll; backs off to hourly while the store fails.
3. `delivery_failed` / `delivery_invalid` dead-letter once with a log line
   naming the failed update and mailbox. **Open question:** the poisoned
   source-poll wake itself is not re-armed by the sink (the sink must not
   touch the store, which is what is down). The sweep is the single recovery
   owner. Whether `HandleSourcePollDue` should also re-arm on error is left
   open; it cannot help while the store is down.
4. Detection: `go-choir-paper-liveness-probe` timer (every 5 minutes) alerts at
   journal priority err when the paper made no inference call for 30 minutes;
   `verify-host-hardening.py` check `paper-probe`. The paper's vm id is
   resolved from the platform computer in `computer-tiers.json` through
   vmctl's `ownerships.json` (the class itself is not stored in
   `ownerships.json`: the paper's row there says `primary`). Not covered by
   the probe: the overdue-schedule count needs the paper credential, so it
   stays a manual Landing Loop step (runbook `node-b-host-reboot.md`).
   Residual: a paper that is legitimately quiet for 30 minutes (every feed
   unchanged) would alert; at the observed 400 to 600 calls per hour that
   has not happened.
