# Corpus Dolt throttled at MemoryHigh; the front page times out (2026-10-29)

Status: open; mitigated at runtime (not durable). Class of a fix: orange
(store sizing) or red if it becomes a store-engine change. Found by the
owner opening choir.news at 04:43 UTC ("Loading the front page..." forever).
Related: the data rearchitecture goal (Dolt as the bottleneck), and
`store-a-weekly-squash-2026-10-20.md`.

## Evidence (read-only, 04:43 to 04:47 UTC)

- The paper was up and publishing all night (corpusd lines from the paper
  per hour 00-04 UTC: 23, 17, 30, 35, 26+); only the front page failed.
- proxy: `platform front stories: call corpusd: .../publications/front?limit=60:
  context deadline exceeded` at 04:43:20 and 04:43:31. `/health`:
  `platform_front.list` 22 ok, 16 `platform_error`, max 30007 ms.
- corpusd: the window query is fast (`op=front_publications ms=24-74
  rows=61`); the per-story object-graph reads fail with `platform
  objectgraph: list edges by kind: context canceled` (13 in hour 04; the
  first at 04:41:26; 1-3 per hour earlier at 21, 22 and 02 UTC).
- The same call from the host, direct to corpusd: no answer in 60 s, three
  times.
- `go-choir-corpus-dolt` (Store B, 13307): memory.current 5462 MiB over
  `MemoryHigh=5G` (`MemoryMax=7G`); its cgroup memory.pressure `full
  avg10=69 avg60=64`. Host: free 0, available 8 GiB, host memory PSI full
  avg60 45, `kswapd0` active, load 7. Platform Dolt (Store A) was under
  its own MemoryHigh with PSI near 0.
- Corpus Dolt was SIGKILLed at 02:15:28 (`status=9/KILL`, peak 5.6G over
  20 h 19 min, 332 GB read from disk) and restarted; it reached 5.4 GiB in
  2.5 h after that. Who sent the 02:15 kill is not known (unread; it was
  under MemoryMax).

## Mitigation (04:46 UTC, runtime only)

`systemctl set-property --runtime go-choir-corpus-dolt.service
MemoryHigh=6656M`. The front call from the host answered 200 in 0.64,
0.38, 0.36 s; the cgroup's full PSI avg10 fell from 69 to 3. Reverts at the
next unit restart or reboot; MemoryMax stays 7G. At 04:47 the store was at
6035 MiB and growing, so this buys time, not a fix.

## Open questions

- Why Store B's resident size grows to the limit within hours (cache
  sizing vs. a leak vs. the paper's publish volume, 367 publishes since the
  proxy's last restart).
- What killed it at 02:15, and whether the 7G MemoryMax will kill it next.
- The front page has no degraded mode: one slow store read stalls the whole
  page instead of showing the stories already resolved.
