# evidence.graph on a copy of the paper's store, 2026-11-03

Question (switch audit, group 3): what would turning `evidence.graph` on cost on the paper? Metadata only; no row contents were printed or saved.

## Result in one line

The paper's `agent_evidence` table holds 5 rows and 4.6 KB. The import moves them in 30 ms. The feared cost (a large table imported under `graphHomeMu`) does not exist on the paper today.

## Numbers

| Measure | Paper (computer-1355b83b...) | d@e (computer-59c902c5...) |
|---|---|---|
| agent_evidence rows | 5 | 0 |
| row bytes (content+title+uri+metadata) | 4,627 total | 0 |
| row size p50 / p95 / max | 308 B / 3,612 B / 3,612 B (sorted: 102, 297, 308, 308, 3612) | n/a |
| created-at span | 2026-10-14 .. 2026-10-29 (rows on 10-14 x2, 10-23, 10-28, 10-29) | n/a |
| growth, last 7 days | 0 rows/day (newest row 5 days old) | 0 |
| growth, lifetime (20 days) | 0.25 rows/day, about 0.2 KB/day | 0 |
| og_objects in the store (context) | 3,686,178; 0 of kind choir.agent_evidence | 18,784; 0 |
| real import (5 rows) | moved 5, error none, table left 0, 5 objects, wall 30 ms, graphHomeMu held about 30 ms, peak RSS 43 MB (harness total) | moved 0, wall 0.12 ms |
| tape written by the 5-row import | about 24 KB gross at SQLite page granularity (index pages 16 KB, bodies 8 KB), 2 events | 0 |

Per-row cost of an import on the real code path (synthetic rows, same code, scratch store with the test tape; no row refused in any case):

| Case | wall | graphHomeMu held | gross bytes written | per row | peak RSS |
|---|---|---|---|---|---|
| 1 x 4 KB | 9 ms | 9 ms | 8 KB net of table | n/a | 46 MB |
| 1,000 x 800 B | 6.9 s | 6.9 s | about 4.4 MB (net +3.6 MB after the table delete) | about 4.4 KB (0.8 payload + about 3.6 overhead) | 70 MB |
| 1,000 x 4 KB | 9.1 s | 9.1 s | about 7.7 MB | about 7.7 KB (4.1 payload + about 3.6 overhead) | 95 MB |
| 10,000 x 800 B | 104 s | 104 s | about 43.8 MB | about 4.4 KB | 196 MB |
| 2,000 x 64 KB | 37.7 s | 37.7 s | about 139 MB | about 70 KB | 565 MB |
| 1 row 1 MiB / 8 MiB / 70 MiB | 0.11 s / 1.0 s / 8.4 s | same | body size plus 8 KB index | n/a | 590 MB / 840 MB / 2.07 GB |

Reading the table: the import costs about 7 to 10 ms and about 3.6 KB of non-payload tape (event index, object header, 4 indexes, edge) per row, linear in rows. It refused nothing, even at 70 MiB (the 64 MiB plaintext constant in computerevent was not hit by this path; peak RSS of a 70 MiB row is 2 GB, about 30x the row). The 25 s POST bound would cut an import off at roughly 3,500 rows of 800 B; the inline path in `CreateEvidence` has no bound.

Steady state: 0.25 rows/day lifetime times about 4.4 KB per row is about 1 KB/day of tape, and 0 over the last week. Even at 100x the lifetime rate (25 rows/day, at p95 size) it would be below 0.2 MB/day. The paper's tape holds 1.88M `choir.event` objects; 5 evidence objects are not visible next to that.

## Method

- Locate: the paper's data disk is `vm-1669020a.../data.img` (computer-1355b83b via fc-config.json), a 128 GiB sparse ext4 image (47 GB allocated) holding `state.db` (SQLite, 23 GB, store schema v6). d@e is `vm-70070cf7.../data.img` (32 GiB sparse, 1.1 GB allocated).
- Copy: `cp --reflink=always` of each live `data.img` into `/var/lib/go-choir/drill-scratch/evidence-graph-20261103/` (btrfs, no extra space, no guest stopped, live file never opened for writing). The copies were loop-mounted read-write (they are copies; the ext4 journal replays on the copy) and queried with `sqlite3 -readonly`. Free disk stayed at 146-150 GB, far above the 40 GB floor. MemAvailable stayed at 16-19 GB; the harness peaked under 2.1 GB RSS, only in the 70 MiB single-row probe. All runs used `nice -n 19 ionice -c3`.
- Import harness: a scratch copy of the worktree (rsynced to Node B, outside the repo) with one added test file. It builds a store with the same tape wiring as `selfdevReplayRuntime`, turns `evidence.graph` on, seeds `agent_evidence` (for the real case by `ATTACH`ing the paper copy read-only and `INSERT ... SELECT` in-process; contents never left the process), and runs `Store.ImportEvidenceToGraph`. Bytes are the `dbstat` page-size delta per table after a WAL checkpoint; lock hold is the wall time of the call (the function takes `graphHomeMu` at entry and holds it throughout), confirmed by a concurrent `ImportEngineeringSlotsToGraph` loop whose longest wait equalled the import wall time; RSS is `getrusage` maxrss of the process (cumulative across cases).
- Not done: the harness did not open the paper's 23 GB `state.db` as the store (the tape appender needs the paper's live chain keys and lease), so tape bytes use the test tape, not the production tape (signatures, receipts, host-side CAS copies are not counted). Treat bytes as a lower bound by a small constant factor; the shape (linear, about 4 KB per small row) is the finding.
- Cleanup: unmounted, detached loops, deleted the whole scratch dir; disk back to 148 GB free.

## Recommendation

Turn `evidence.graph` on: the paper's table has 5 rows (4.6 KB) and has had no new row in 5 days, so the import is a 30 ms, 24 KB event and the steady-state tape cost is about 1 KB/day, which does not undo the storage and memory work. Do it on the paper after bounding the import, because the unbounded inline path in `CreateEvidence` and the 25 s POST path still hold `graphHomeMu` for roughly 7 to 10 ms per row (a 10,000-row table would block slot claims for 104 s), and a single 70 MiB row would peak at 2 GB RSS: cap the rows per lock hold (the page loop already exists, so take the lock per 50-row page) and cap a row's size (for example refuse above 1 MiB with a named error). Because the paper has barely used `save_evidence`, the value of the switch is replay eligibility (the table must be empty for the manifest), not volume; if the owner prefers deletion, the alternative with the same effect is to delete the table path and the setting after importing these 5 rows once. Keep it off nowhere: no computer in the measured set has enough evidence for the cost to matter.
