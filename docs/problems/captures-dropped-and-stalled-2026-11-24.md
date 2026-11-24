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
  start for about 13 hours. Cause not established; the ledger was not read.
- At most four task runs at once; the four slots are about 55% busy, so they
  saturate near twice today's arrivals.
- A task makes 1.22 model calls on average with a median 67,000 input tokens:
  about 0.4 to 0.5 calls per decided capture.

## Belief

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
- Why the 11-19 and 11-20 spike happened, and why 163 tasks waited 13 hours on
  11-23, is not known. Both need the run ledger read.
- Whether any of the 1,760 mattered (were about a story the paper then
  missed) is not measured.
- Research reads and later desks are not counted.
