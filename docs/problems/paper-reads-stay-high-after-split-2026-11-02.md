# The paper's disk reads stay high after the header/body split (2026-11-02)

Mutation class: green (record). Mission: docs/definitions/choir-store-header-body-split-2026-11-02.md (S6).

## Observation

The host memory meter (/var/lib/go-choir/growth-meter/mem-2026-11.jsonl, read_bytes per minute for vm-1669020adcece51c2f5ff9b6df399ef5):

| Window (UTC) | Median read MiB/min | Notes |
|---|---|---|
| 2026-11-01 19:40 to 2026-11-02 01:15 | 4.9 | before the split; Synthetic serving, ChatGPT out of quota |
| 2026-11-02 01:30 to 04:02 | 2.9 | before the split; both providers out of quota, the paper nearly idle |
| 04:20 to 04:40 | 540 | after the split; boots at 04:14, 04:24 and 04:37 (migration, then two deploy refreshes) |
| 04:40 to 05:00 | 439 | includes the scheduled whole-store snapshot at 04:50 to 04:55 (about 40 GB in reads) |
| 05:00 to 05:28 | 85 to 170 | no boot since 04:37 |

Guest memory as the host sees it (anon) moved from about 1.5 GiB before the split to 2.7 to 4.4 GiB after it.

## What it is not (checked)

- **Not a query-plan regression.** EXPLAIN QUERY PLAN on a reflink copy of the paper's disk, for every periodic and hot reader, matches the design. No reader scans og_objects or og_bodies where it should not, and body reads come only after the filter. Sonnet probe, 2026-11-02.
- **Not the retirement pass.** 7 ops in 1 min 6 s, the same as before the split.
- **Not the per-minute run-memory scan alone.** run_memory_agent_list (about 17.6k rows) ran the same way before the split.
- **Not a whole-file job in the 05:00 to 05:28 window.** The only one tonight was the 04:50 snapshot.

## Hypotheses (unverified)

1. **The hot set exceeds the guest page cache.**
   - og_objects headers are 2.75 GB and its indexes 5.3 GB.
   - run_memory_entries and its indexes are 4.7 GB.
   - internal/dbx sets no cache_size or mmap_size, so SQLite keeps about 2 MiB per connection, and the guest page cache is a few GB.
   - Random header lookups therefore miss.
   - Open question: why did the old layout miss less? The old file held the same headers inside bigger rows, and it had been warm for hours with no reboot.
2. **The confounded baseline.** The before windows were quiet. Both providers were short of quota, and from 01:30 the paper was nearly idle. After 03:50 ChatGPT serves 50 to 80 calls a minute, so the paper does more work per minute than in either before window.
3. **The cold cache after three boots** is still settling (the trend is flat to falling).

## Next probes

- The S6 meter day (2026-11-02T05:00Z to 2026-11-03T05:00Z), with read_bytes normalized by model calls per minute (gateway journal) so that work done is separated from reads per unit of work.
- Then a cache trial on d@e first: PRAGMA cache_size and mmap_size on the read connections. It needs a guest deploy, so it waits for the meter day to end.
- The guest's rchar against read_bytes, to separate page-cache hits from disk misses.
