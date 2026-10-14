# A Planned Deploy Refresh Closed the Live Paper's Schedules

Date: 2026-10-14, 07:05 UTC. Found while preparing L2a's paper enable.
Computer: the live paper, computer-1355b83bced1613eb048ee28d66688a8
(owner e5962310…), now on build 9ace75e5. Status: documented;
schedules restored by the owner act at 07:05; cause not yet established.

## What happened

- At 07:00 all 14 of the paper's source schedules read
  `interrupted_by_restart`, generation 8, `updated_at` 2026-10-14T05:48:47Z.
  The paper's last run was created at 05:44:30. So no story was opened or
  revised from about 05:48 to 07:05 (about 77 minutes).
- vmctl's journal for the balloon deploy (11a94193): vmctl restarted at
  05:47:58; then `POST /internal/vmctl/deploy-refresh caller=ci.deploy-refresh`;
  at 05:48:10 a destructive kill of `vm-1669020adcece51c2f5ff9b6df399ef5`
  (`cause=refresh epoch=13065`); at 05:48:48 `refreshed VM … (new_epoch=13066,
  deploy-image-refresh)`, `outcome=refreshed planned_restart=true`.
- `/api/runtime/obligations` on the paper at 06:49 reports
  `restart: {kind: planned, reason: platform update}`. That is the latest
  boot; the paper was refreshed again by the 06:03 and 06:37 deploys,
  while its schedules were already closed.
- Restored at 07:05 by the owner act (`POST /api/autopaper/sources/schedules`
  for each source with its previous interval and max_items): 14 of 14
  active. The pre-restore list is saved at
  `~/.config/choir-qa/traces/paper-schedules-before-restore-0705.json`.

## Why this is wrong

- Owner rule (AGENTS.md): a planned restart for an update may resume work;
  deploys refresh idle computers with a marker. And (ruling 6, 10-14):
  "we need paper publishing 24/7, even after crashes"; standing intents
  survive even a crash.
- The code agrees for planned boots: `closeSourceSchedulesAfterCrash`
  (internal/agentcore/source_schedule.go:272) returns at once when
  `BootWasPlannedRestart()` is true. Yet the schedules were closed during
  the 05:48 boot.

## Hypotheses (unconfirmed until the guest's boot log is read)

1. The 05:48 boot did not see the marker (written to the wrong place,
   consumed early, or refused for a target or epoch mismatch), so it ran as
   a crash boot.
2. Another path closes schedules on a planned boot (for example the
   actor handler's pre-boot gate or the wake projector closing a
   `source_poll_due` wake, which `RestartWorkKinds` lists).
3. The schedules were closed by an earlier crash boot whose `updated_at`
   was rewritten at 05:48:47.

Even with a correct marker, ruling 6 says schedules must survive a crash
too (until L1's code lands, a crash closes them). So every unmarked
restart of the paper stops publishing until someone restarts the
schedules.
