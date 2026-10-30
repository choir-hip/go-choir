# Corpus Dolt OOM-killed twice during the engine measurement (2026-10-30)

Class: green (problem record). Host: Node B. Unit: `go-choir-corpus-dolt`
(Store B, :13307), cgroup `MemoryHigh` 6.5 GiB, `MemoryMax` 7 GiB.

## Evidence [V]

- `journalctl -u go-choir-corpus-dolt`: `Failed with result 'oom-kill'` at
  2026-10-30 00:06:03 UTC (7G peak, 35G read from disk over 1h27m) and at
  00:39:51 (7G peak, 19.6G read over 33m). systemd restarted it within 4 s
  each time; `NRestarts=2`. At 01:59 it was active, at 5.8 GB of a 6.5 GiB
  `MemoryHigh`.
- The windows overlap the round-0f engine measurement
  (`docs/evidence/new-engine-storage-measurement-2026-10-30.md`), which read
  the corpus keep set. Its author reports no live query against Store B at
  the moment of either kill.
- The paper's health returned intermittent `store_stalled` 503s during the
  measurement; five probes at 01:59 returned 200.

## Belief state

- Hypothesis [I], not confirmed: the measurement's earlier full reads of
  Store B grew the Dolt process's caches toward the cgroup limit, and the
  paper's own reads tipped it over. The 35G read in the first window is
  consistent with a full export. The alternative, that the paper's readers
  alone OOM Store B, is not ruled out; the trace (query log by client)
  decides, not inference.
- Already on the deferred list: a GOMEMLIMIT for corpus Dolt below its
  cgroup limit, so the Go runtime collects before the kernel kills.

## Remaining error

- No per-client query attribution for the two windows.
- The owner has said not to spend on Dolt problems before the engine move;
  this record exists so the move's acceptance includes "no OOM restarts of
  the corpus store under a full keep-set read".
