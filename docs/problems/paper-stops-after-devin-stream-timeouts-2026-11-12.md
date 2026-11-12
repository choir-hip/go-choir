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

## Update 19:10Z: the whole guest froze, not only the schedules

Read-only, from the schedule list, the Node B gateway journal and the code.

- The schedule list now shows 66 schedules, all `active`, 64 `overdue`.
  Only newsie carries an in-flight mark (15:26:07Z); the other 63 overdue
  ones carry none. Their wakes never got past marking themselves in flight,
  and the sweep never re-armed them: with no in-flight mark the sweep
  repairs a schedule at once (`sourceScheduleNeedsRepair`), and its write
  would have changed `next_due` and the generation. Neither changed.
- The paper made no gateway call of any kind after 15:28:05Z: no model
  call, no Jev call, no failure. Its last calls were ordinary (Devin
  successes, Jev bulk calls at about 120 ms). So the freeze is not limited
  to the schedule path. Every desk went quiet about two minutes after the
  newsie poll began.
- The schedule list read and the health endpoint still answer, so HTTP
  serving and store reads work.
- Every schedule write, and the sweep, runs under the process-wide
  `sourceScheduleMu`. The poll's long work runs without it, and inside an
  activation the next-wake arm only buffers. A holder that never released
  the lock would explain the schedules, but not the silence of every other
  desk.

Belief, revised: a store-level or actor-kernel block (a write that never
returns, or a dispatcher that stopped delivering) froze all writers at
about 15:26 to 15:28Z. The schedule lock is a casualty, not the cause.
Unconfirmed.

Evidence capture: `GET /api/runtime/observability?goroutines=1` on the
guest gives the goroutine dump that would settle it. The unauthenticated
host read that worked on 2026-11-08 now answers `authentication required`.
The dump needs the paper owner's session. A restart without it loses the
evidence. Guest logs do not reach the Node B journal.

## Update 19:20Z: cause found, an event appender wedge; refreshed

The goroutine dump and recent log (the paper owner's
`/api/runtime/observability?goroutines=1`, read with the paper's owner key
after renewing it with `scripts/qa_renew_key.mjs`) settle it. The 19:10Z
belief (a blocked store or actor call) was wrong in kind: nothing was
blocked. The store refused every write.

- `computer event appender: wedge began` at 15:26:30Z with
  `heads differ after in-place recovery`: local head 10914341, platform
  head 10914342. The platform's canonical chain held one event the local
  projection never applied. More than 1,600 refusals followed, one or more
  a minute, each `in-place prepared recovery (heads differ): err=<nil>`
  without closing the gap.
- Every write then failed with `computer event projection repair required`.
  The sweep logged `sweep failed (6 in a row): re-arm
  social:mastodon:newsie.social:tag:news: ... append projection batch:
  computer event projection repair required` and backed off to hourly.
- The dump has 45 goroutines and none blocked since 15:26; the sweep loop
  sat in its backoff wait. No deadlock, no stuck lock.
- `recoverPreparedInPlaceLocked` only discards orphaned prepared rows; it
  never fetches the missing platform event. The boot replay does
  (`source.Events` after the local head, then a head check). The code
  comment records the same wedge on the paper twice on 2026-11-02, "until
  its next boot".

A second defect: `choir paper restart` (the operator path) answered 502
`the restart receipt could not be recorded; nothing was restarted`. It
records its receipt in the guest's store before restarting
(`internal/proxy/platform_operator_computers.go`), so it cannot restart
a computer whose store refuses writes, the one case where a restart is the
repair.

Act: the owner asked for capture and restart. A planned refresh as the
paper's owner (`POST /api/computers/<paper>/lifecycle/refresh`) returned
201 at 19:18:54Z, epoch 13677 to 13678.

Remaining error:

- Why the local projection missed event 10914342 is not known. It came
  with the Devin stream deadlines (15:19 to 15:28Z), but a link is not
  shown. Three wedges of this kind on the paper in ten days.
- The running computer cannot heal this wedge: in-place recovery should
  replay from the platform as the boot does. Until it does, every such
  wedge stops the paper until a person restarts it, and the liveness alert
  reaches no one.
