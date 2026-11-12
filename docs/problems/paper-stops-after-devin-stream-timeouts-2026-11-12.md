# The paper stopped publishing at 15:19Z and its liveness alert reached no one

Date: 2026-11-12. Class: green (record only; nothing changed; the paper was
not touched). Found on the 16:35Z hourly Autopaper tick, read-only.

## Problem

The paper (computer-1355b83bced1613eb048ee28d66688a8, VM vm-1669020a...)
published its last front-page story at 15:19:55Z and made its last model
call at 15:28:05Z. At 16:36Z it was still idle. The host's
`go-choir-paper-liveness-probe` has logged `ALERT paper made no inference
call for 30 min` every five minutes since 15:58Z, but only to the journal;
nobody was told.

## Evidence

- Gateway: the paper's last three model calls (15:19:14Z, 15:24:02Z,
  15:28:02Z) all failed with `devin: read stream frame: context deadline
  exceeded`. In the 70 minutes after, the paper made 4 Devin requests, 6
  Jev calls and 1 search, against about 1,200 to 2,200 calls an hour
  earlier in the day.
- vmctl: health checks of the paper VM failed at 15:21, 15:25, 15:26,
  15:27, 15:40 and 15:42Z. 137 health checks failed today, the first at
  01:54Z, while the paper kept publishing. At 16:36Z the health endpoint
  answered 200 in about 40 ms.
- No deploy or restart: gateway and vmctl active since 2026-11-11 21:34Z;
  deploy receipt 26f9d66f3 activated 21:35Z. No OOM or kill in the kernel
  log 15:00 to 15:50Z. Host memory 18 GB available.
- Liveness probe before the stop: `ok ... inference_calls_30m=638` at
  15:20Z.

## Belief

Something inside the paper stalled around 15:20Z, possibly while its
running turns waited on Devin streams that timed out, and its schedule has
not started new work since. Unconfirmed: the cause is in the paper's own
ledger and logs, which the host does not keep and this read did not reach.

## Remaining error

- Why no new work starts (a stuck turn, a closed schedule, a held queue) is
  unknown.
- The liveness probe detects the stop but sends nothing; a stop of the
  paper is seen only by a person reading the journal or by this hourly
  tick.
- Acting on the paper (a restart or a refresh) is the owner's call.

## Update 17:50Z: schedules wedged, not closed

Read-only, through `choir paper status` and the code; the guest's own logs
and turns were not reachable.

- All 56 source schedules are `active`; none is closed, paused or in the
  crash state. Every `updated_at` is 15:26:07Z or earlier and 54 are
  `overdue` (next due 15:27Z to 16:22Z). One,
  `social:mastodon:newsie.social:tag:news`, shows `poll_in_flight_since`
  15:26:07Z with no result.
- The overdue sweep (`sweepOverdueSourceSchedules`,
  `internal/agentcore/source_schedule.go`, every 5 minutes, re-arms a
  schedule more than 15 minutes overdue) should have re-armed them from
  about 15:41Z. Nothing has been written for two hours, so the sweep is
  blocked or dead. Leading hypothesis: a store or actor write stuck while
  holding the process-wide `sourceScheduleMu`, which every schedule write
  and the sweep take; reads still answer.
- Ruled out: a crash-loop closure, a guest crash-boot (no schedule
  rewrite), and vmctl acting on the unhealthy VM (`checkAllHealth` only
  logs). vmctl still reports the VM busy.
- It is not expected to resume by itself. The smallest owner act is a
  planned refresh, `choir paper restart --reason ...` (marker-backed;
  re-arms overdue schedules at boot). It interrupts in-flight turns and
  destroys the live evidence of the wedge; if the wedge is a deadlock it
  can recur. Starting schedules one by one would take the same lock and
  likely hang.
