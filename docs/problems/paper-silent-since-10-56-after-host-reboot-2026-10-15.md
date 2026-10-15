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
