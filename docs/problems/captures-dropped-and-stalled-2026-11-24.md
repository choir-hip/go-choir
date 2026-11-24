# Captures are dropped by expiry and by tasks that close without deciding them (2026-11-24)

## Problem

The owner asked whether the paper is building a backlog of captures. It is
not building a live one, but it has been losing captures: some aged out of the
queue undecided, and about 2% a day are left undecided by a management task
that failed, was passivated or finished without them.

## Evidence

Measured from a copy of the paper's database taken 2026-11-24T02:08Z to the
external archive drive (owner's permission; nothing written on Node B). The
copy was dumped from a live image and **failed `integrity_check`**; the rows
were recovered by a tolerant scan, with one small gap on 2026-11-21 and the
last two hours unusable. Cutoff T0 = 2026-11-24T00:40Z. Polled captures only.

| Measure | 24 hours | 7 days |
| --- | --- | --- |
| Captures arrived | 3,871 (161/h) | 25,608 (152/h) |
| Time from arrival to decision, median | 236 s | 198 s |
| Same, 90th percentile | 3,526 s | 1,314 s |
| Same, longest | 13.8 h | 23.9 h |
| Management tasks | 1,262 | 8,119 |
| Tasks carrying one capture | 610 | 3,983 |
| Tasks carrying the full twelve | 153 | 1,043 |
| Task run time, median / 90th percentile | 105 / 329 s | 103 / 332 s |

- **Undecided at T0: 1,861 captures.** About 100 are under a day old (the live
  queue). 1,760 are older than a day and will never be decided.
- **1,352 captures aged out as `expired_in_queue`**: 1,110 that arrived on
  11-19 and 242 on 11-20. None since.
- **476 captures in seven days sit on a task that closed without deciding
  them**: 198 on a failed run, 164 on a passivated run, 89 on a task that
  finished and left them, 19 on a completed run, 6 on a deadline.
- **A 13-hour stall on 11-23**: 163 tasks created from about 08:40Z did not
  start for up to 13.7 hours. Explained below.
- At most four task runs at once; the four slots are about 55% busy, so they
  saturate near twice today's arrivals.
- A task makes 1.22 model calls on average with a median 67,000 input tokens:
  about 0.4 to 0.5 calls per decided capture.

## The two events, from the run ledger (read 2026-11-24)

Read from the same torn copy: task rows (`choir.management_task` objects) and
run events (`choir.event` objects) in `og_objects`, by rowid range. The
record's counts reconcile (481 expired tasks, 1,352 captures, 1,110 and 242).

**Both are one mechanism, and neither was a halt.** Runs of every desk share
a dispatcher. When its slots are full, a run waits between `loop.submitted`
and `loop.started`. A management task is claimed, and holds one of the four
task slots, before its run waits for the dispatcher
(`internal/agentcore/management_tasks.go:414`, `managementTaskHoldsSlot`), so
dispatcher wait slows task turnover. A free slot goes to the newest queued
task (`NewestQueuedFirst`, `internal/store/management_tasks.go:754`), so when
arrivals exceed turnover the oldest tasks starve. A task queued 24 hours is
closed `expired_in_queue` (`managementTaskQueueMaxAge`,
`management_tasks.go:361`).

| | 11-19 and 11-20: 1,352 captures expired | 11-23: 163 tasks waited |
| --- | --- | --- |
| What the task rows show | all 481 tasks: no start, no run, closed 24.0 to 24.15 h after creation; created 11-19 09Z to 20Z (25 to 46 an hour) and 11-20 01Z to 10Z | created 08:41Z to 16:40Z, claimed 18:11Z to 22:22Z, longest wait 13.69 h; 161 of 163 then closed `done`, none expired |
| Dispatcher wait (submitted to started) | median 1 to 6 s until 11-19 07Z, then 110 to 350 s for every desk until about 11-21 02Z | median 120 to 230 s from 08Z to 16Z; normal from 17Z |
| Task turnover | 22 to 48 started an hour against 30 to 70 created; a task's own run time unchanged (median 45 s) | 36 to 50 claimed an hour against 47 to 67 created; 73 to 101 an hour from 17Z, which drained the queue, oldest last |
| What the host was doing | `loop.retry` events with reason `provider_quota` begin 11-19 08:24Z in hourly bursts; research turns show gaps of 100 to 600 s between a tool result and the next progress event. No restart closed tasks in the window | research run starts rose from 10 to 15 an hour to 36 to 77 an hour from 08Z to 16Z, then fell to 10 |
| Cause | **correlated, not proved**: long research and Texture turns filled the dispatcher. The quota refusals start at the same hour, but continue at similar rates on later days with no backlog, so they are not enough alone | **proved to the level of data plus the commit's own words**: the research-frame change (`3dcddbe86..33fc8ff10`, 11-23 06:10Z) multiplied research runs; its revert `771fcc088` (13:32Z, "runs rose from 13 to 62 an hour after it deployed") ended it. Deploy times were not read |
| Why nothing was lost on 11-23 | | the load ended before any task was 24 hours old |

**Can it recur today? Yes.** The four task slots, the slot held during
dispatcher wait, newest-first order and the 24-hour expiry are all on
`origin/main` unchanged (the two rules date from 11-08 and 11-09, before both
events). Any day in which arrivals exceed task turnover repeats the loss, and
any spike in research or Texture runs can cause that. The 11-23 trigger
itself is reverted.

**Not established.** Whether the sub-call spacer (`callSpacer`,
`internal/agentcore/research_children.go`) contributed on 11-19: it leaves no
event in the ledger; a 600-second gap would need about 400 queued calls, so
it is unlikely, not excluded. Which deploy or restart, if any, started the
11-19 backlog: the database holds no deploy marker.

**Limits of the read.** `og_objects` aborts on its kind indexes; rows were
read by rowid range without them. One chunk (rowids 11,800,000 to 11,840,000)
is malformed, so 11-23 after about 23:13Z may be short, and 11-21's task rows
are incomplete (862). Gap counts are per chunk and miss runs that span a
boundary. No row content was read out: counts, times, ids and reason values
only.

## Belief

The ledger read adds a third property: intake shares a dispatcher with every
other desk and gives way to it silently, so a busy research hour becomes lost
captures a day later with no error anywhere.

The intake design loses work in two ways that no one sees: a queue rule that
discards what waits too long, and a task that can close with captures still
on it and nothing to hand them on. Both are properties of the management-task
path the capture desk replaces (design mission, G4 and G5). The capture desk
must be built so that a capture cannot leave the queue without a decision
record row saying what happened to it.

## The fix on today's code (`lane/intake-starve`, 2026-11-24)

This is the smallest fix for the mechanism; the capture desk still replaces
the path.

- **A task's run never waits for a shared slot.** Its inbox has its own
  dispatcher lane, as wide as the most task slots. This is smaller than
  moving the slot claim to after admission: the dispatcher already has lanes
  (source polls, engineering admission), one line adds one, and the claim,
  bind and close order is untouched. The invariant: a held task slot has a
  run the dispatcher starts at once.
- **Oldest first.** Newest first was chosen on 2026-11-08 because the queue
  outran its slots and only two-day-old news was served. The cause then was
  slots held by dead rows, and on 11-19 slots slowed by other desks; the
  order hid both by starving the old tasks.
- **The expiry: removed, not kept with a record.** The two options were to
  remove the 24-hour expiry, or to keep a bound whose expiry writes a counted
  record per capture and returns the captures to intake. Removing it is the
  conservative one, as nothing is dropped while the computer runs (a crash
  boot closes what is queued as interrupted by a restart, by the restart
  rule; a planned restart resumes it), and it is also the smaller: a
  returned capture would be delegated again into the same queue, which is
  the same wait with more records. The cost is that a queue which outruns
  its slots grows without bound; that is now visible within a minute (the
  queue line) instead of silent for a day. Whether old captures should be
  declined by a decision is management's to judge, with a record, in the
  capture desk design.
- **The queue line**, once a minute: depth, oldest wait, running, slots.
- **The first hours after the deploy.** Tasks already queued, however old,
  now run, oldest first. A backlog the size of 2026-11-23's (163 tasks) at
  four slots and a median run of 105 seconds drains in about 1.2 hours
  (163 x 105 / 4 seconds), with new arrivals behind it; the queue line shows
  it falling.
- **A known inefficiency, left.** Every closure names the oldest queued task
  for a start, so several closures in a row send several start wakes to one
  task's inbox. The task starts once (the claim is the guard) and the
  starter fills every free slot on any wake, so no slot idles; the extra
  wakes are waste, not a fault.

Not changed: the 476 captures in seven days left on a task that closed
without deciding them (a failed run, a passivated run, a deadline). That is
the second loss in this record and needs its own change.

## Remaining error

- The copy is torn; counts near the gap and the tail are approximate.
- The trigger of the 11-19 backlog is correlated, not proved (below).
- Whether any of the 1,760 mattered (were about a story the paper then
  missed) is not measured.
- Research reads and later desks are not counted.

Security effect (review 1, seat 2): peak concurrent runs rise from 12 to up to 20 at eight task slots, and queued tasks never expire, so the queue and its capture leases can grow without bound; both are logged. No authority, credential, desk read or write, route, custody or isolation change.
