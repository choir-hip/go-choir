# The paper's busy_cold act frees cache the guest refills at once; the host gets a sixth

Date: 2026-11-01. Status: recorded; the paper is back to observe. Outcome B of
the D6 design (docs/design/density-d6-page-cache-return-2026-10-26.md, section 10
step 2): stop, then S6's gate.

## Evidence (Node B and the paper's guest memory route, metadata only)

- 07:34Z, the first active act on the paper (vmctl journal): guest_target 1024 MiB,
  guest_reclaimed 1031 MiB, firecracker RSS 7748 -> 7574 MiB (174 MiB, 17%),
  reportable free 434 -> 608 MiB, refaults 0 before and after, settle_exit=cap
  at 5 s. RSS was still 7575 MiB a few minutes later.
- Minutes after the act the guest had 160 MiB free in all (free_blocks by order:
  72 MiB at order 10, the rest scattered). page_reporting order is 9 (2 MiB).
- Guest vmstat since its 05:30 boot: pgsteal_kswapd 482,006 pages (1.84 GiB),
  workingset_refault_file 202,590 pages (791 MiB), pgsteal_proactive 0.
- d@e, for contrast: 266 MiB inactive file, under the 512 MiB keep; its active
  pass returns nothing_to_return.

## Reading

The paper's cache is not idle. The guest keeps reading its 20.7 GB state.db and
kswapd keeps it full: whatever an act frees, the next reads take. Only freed
2 MiB blocks reach the host, and few form before the refill. So the act costs a
re-read of what it evicts and returns about a sixth of it.

Hypothesis, not yet confirmed: the host caches the paper's data.img too
(io_engine Sync), so state.db pages are held twice, once in each page cache.

## Directions (not started)

- Bound the guest cache instead of trimming it: memory.high on the guest's
  daemon cgroup, or a balloon under RES-09 (design S6), so freed memory stays
  free and coalesces into reportable blocks.
- Find what reads state.db as a stream (the paper's queries) and stop the scan.
- The double cache: O_DIRECT for data.img, or host-side drop of the image's
  pages, weighed against read latency.
