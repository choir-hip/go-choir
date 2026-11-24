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

## Remaining error

- The copy is torn; counts near the gap and the tail are approximate.
- Why the 11-19 and 11-20 spike happened, and why 163 tasks waited 13 hours on
  11-23, is not known. Both need the run ledger read.
- Whether any of the 1,760 mattered (were about a story the paper then
  missed) is not measured.
- Research reads and later desks are not counted.
