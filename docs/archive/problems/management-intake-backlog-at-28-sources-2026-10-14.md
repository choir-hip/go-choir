# Management Intake Falls Behind at 28 Sources (S2a, One Task at a Time)

Date: 2026-10-14. Found by `scripts/management_latency_probe.py --start
2026-10-14T14:28:00Z` on the paper (computer-1355b83b…) at 15:02 UTC,
after the paper went from 14 to 20 (14:15) and 28 (14:27) source
schedules. Status: documented; S2b (K = 4) is the planned fix.

## Evidence (14:28-15:02 UTC)

- 36 arrival records, 154 captures; dispositions 5 (1 open, 4 decline);
  149 undisposed, 81 of them older than 15 minutes (oldest 19.6 min).
- Management tasks: 10 delegated, 8 done (all clean, 0 residuals), 2 open;
  queue wait p50 425 s, p95 1,506 s; task duration p50 241 s, p95 435 s;
  one task at a time (`managementTaskSlots = 1`,
  internal/agentcore/management_tasks.go:40).
- Parent turns are fine (p50 33 s); the parent is busy 37% of the window.
- The paper still publishes (stories at 15:00-15:01 UTC).

## Reading

Intake is about 270 captures an hour at 28 sources; one task at a time
disposes of far fewer (8 tasks in 34 minutes). The backlog grows, so
captures are disposed of late (stale) rather than lost. This is the
throughput S2a deliberately left for S2b
(docs/reports/management-delegation-design-2026-10-14.md §13: K = 4, the
durable slot starter and queue, guards so two tasks never open the same
story at once).

## Options

1. Build and land S2b (K = 4) — the designed fix; red class.
2. Interim, if the backlog keeps growing: lower `max_items` or lengthen
   the interval on the highest-volume, lowest-yield schedules (e.g. the
   Hacker News lists, which also cause most re-captures per
   docs/problems/l2a-merge-recall-on-the-paper-2026-10-14.md) instead of
   removing the new sections' sources.
