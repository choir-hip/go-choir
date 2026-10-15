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
