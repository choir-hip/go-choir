# A Planned Deploy Refresh Closed the Live Paper's Schedules

Date: 2026-10-14, about 06:50 UTC. Found while preparing L2a's paper enable.
Computer: the live paper, computer-1355b83bced1613eb048ee28d66688a8
(owner e5962310…), now on build 9ace75e5. Status: documented;
schedules restored by the owner act at about 06:49; cause not yet established.

## What happened

- At 07:00 all 14 of the paper's source schedules read
  `interrupted_by_restart`, generation 8, `updated_at` 2026-10-14T05:48:47Z.
  The paper's last run was created at 05:44:30. So no story was opened or
  revised from about 05:48 to 06:49 (about an hour).
- vmctl's journal for the balloon deploy (11a94193): vmctl restarted at
  05:47:58; then `POST /internal/vmctl/deploy-refresh caller=ci.deploy-refresh`;
  at 05:48:10 a destructive kill of `vm-1669020adcece51c2f5ff9b6df399ef5`
  (`cause=refresh epoch=13065`); at 05:48:48 `refreshed VM … (new_epoch=13066,
  deploy-image-refresh)`, `outcome=refreshed planned_restart=true`.
- `/api/runtime/obligations` on the paper at 06:49 reports
  `restart: {kind: planned, reason: platform update}`. That is the latest
  boot; the paper was refreshed again by the 06:03 and 06:37 deploys,
  while its schedules were already closed.
- Restored at about 06:49 by the owner act (`POST /api/autopaper/sources/schedules`
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

## Findings (2026-10-14, from the guest console and vmctl journal)

Status: cause established. Hypothesis 1 is confirmed in a specific form;
hypotheses 2 and 3 are ruled out.

**The 05:48 boot read as a crash, by design (D7 work-state epoch).** The
guest console (`/var/lib/go-choir/vm-state/vm-1669020a…/console.log`)
shows the whole sequence on the old binary and then the new one:

```text
05:48:10 runtime: host requested a planned restart (platform update 11a9419345a5…)
05:48:43 agentcore: planned restart marker has work-state epoch 1, this binary reads 2; treating boot as a crash
05:48:43 autoputer: restart kind=crash_or_stop
05:48:47 source schedule: crash boot: rss:hn_best closed: interrupted by a restart
         … (14 schedules, 05:48:47, matching updated_at)
```

- The marker was written (the old binary answered the host's
  `/internal/runtime/planned-restart` at 05:48:10) and consumed. It was
  refused because the old binary was pre-C1e-B1 (work-state epoch 1), and
  11a94193 includes a06191f9 (C1e B1, `CurrentWorkStateEpoch = 2`).
  `ConsumePlannedRestartMarker` treats an epoch-mismatched marker as a
  crash on purpose (D7: nothing resumes into a new schema).
- Why this refresh: the paper had run a pre-B1 binary since its 23:51
  boot on 10-13. The 04:21 (e9d727a7) and 05:26 (d5139fd6) refreshes
  carried B1 but were deferred (`busy`, 2 and 1 running runs). 05:48 was
  the first refresh to land, so it was the paper's epoch-cutover boot.
- vmctl logs `planned_restart=true` because the host's request succeeded;
  it cannot see that the guest's own epoch check overruled it. The
  divergence is visible only in the guest console and in
  `/api/runtime/obligations` (`restart.kind`) for that boot.
- The only writer of `interrupted_by_restart` on a schedule is
  `closeSourceSchedulesAfterCrash`, and the console names it. No other path
  (hypothesis 2) and no rewritten older closure (hypothesis 3).
- Later boots (06:03 5e4b6fa7, 06:36 9ace75e5) were read as planned
  (`restart kind=planned`), as expected for same-epoch refreshes.

**A second closure the same night, also by design.** The schedules had
already been closed once: vmctl hibernated the paper at 21:00 on 10-13
(`reason=pressure`, a destructive kill with no snapshot); its wake at
23:51 was a markerless boot (`restart kind=crash_or_stop`) that closed
all 14 schedules at 23:51:47. The restart design draft predicted this
("every idle hibernate wake today is a crash boot that closes
schedules"). The paper was also silent through the hibernation itself
(21:00 to 23:51).

## Belief state

- The marker path works as written. The failure is a rule, not a bug:
  under today's code every crash-classified boot (a real crash, a
  hibernate wake, an epoch-cutover refresh) stops the paper until an owner
  act. Epoch bumps and pressure hibernates are routine, so the paper stops
  routinely.
- Ruling 6 (owner, 2026-10-14) and restart design option B (§3.4, adopted
  by all six panelists) remove the class: a crash closes only the poll in
  flight; the schedule stays active and its next tick, at now plus the
  interval, is new work; a schedule whose poll was in flight at two
  crash boots in a row is closed with that cause (the crash-loop bound).
  Pending arrivals stay closed at a crash, as today (they are announced
  only by a poll, and the design changes only the schedule's fate).
- The D7 epoch rule stays as it is. It still guards the work it was built
  for; schedules just stop depending on whether a boot is planned.

## Remaining error field

- Fix: schedules survive crash boots (station L1, "schedules survive
  crash"), in a separate commit on a worktree branch, not yet deployed.
- Residuals: vmctl's `planned_restart=true` can disagree with the
  guest's boot kind (no host-side receipt of the guest's verdict); the
  pressure hibernate of the paper computer itself (about three hours
  silent) is a separate problem; the crash-loop bound counts only crashes
  with a poll in flight, so a crash a poll causes after it records its
  result is not attributed to it.

## After the fix deployed (ce59d406, 07:46 UTC)

- The fix is live on the platform, and new and idle computers get it on
  their next refresh.
- The paper itself has not been refreshed: at 08:28 it was still on
  9ace75e5, 14 of 14 schedules active (generation 9, the 06:49 restore).
  With intake.labels on and 14 sources polling every 15 minutes, it is
  never idle, and deploys never restart a busy computer. That confirms
  the hypothesis in the cost-track report: a 24/7 paper does not take
  updates by itself. Its schedules are safe meanwhile: a refresh at the
  same work-state epoch is planned, so it keeps them even under the old
  code.
- Open: how a always-busy computer takes updates. Options: a short
  update window (pause polling at a quiet minute, let the refresh run,
  resume, which is allowed once per planned restart); the paper split
  across several computers that update in turn; or the snapshot path
  (S3) for a fast restart. For the owner, with the multi-computer
  question.
