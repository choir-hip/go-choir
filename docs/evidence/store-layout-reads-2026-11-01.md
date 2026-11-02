# Store layout reads, S0 (2026-11-01)

Synthetic store, 100,000 `choir.event` rows, bodies 40% 200-900 B, 35%
1-3.9 KB, 20% 4-20 KB, 5% 20-120 KB (about 740 MiB); page size 4096;
SQLite 3.51.2 on the agents VM; read volume is `/proc/self/io` rchar per
statement with mmap off and the default 2 MiB page cache. Script:
`scripts/store-layout-reads.py` (`S0_CACHE=1` reproduces the cold-cache run).

Layouts: A today (body mid-row, the real indexes minus the 2026-11-01
workarounds), A+ today with the kind and phase expression indexes, B body
last, C header table plus `og_bodies(obj INTEGER PRIMARY KEY, body)` keyed by
the header rowid, event kind and phase as typed columns.

| read (MiB) | A | A+ | B | C |
|---|---|---|---|---|
| retirement slice, ~130 rows | 9.80 | 0.59 | 2.68 | 0.23 |
| newest 200 by kind and owner | 1.50 | 1.69 | 0.66 | 0.16 |
| census of every header | 737 | 748 | 214 | 19.0 (header scan) |
| refs of one owner, 25k rows | 84.7 | 84.5 | 84.8 | 19.0 |
| 500 point body reads (3.4 MiB of body) | 6.77 | 7.09 | 6.76 | 6.80 |

The header table is 17 MiB of pages; the whole-row table is 729 MiB.

Findings:
- Body last removes the overflow-chain reads but not the inline bodies: a
  1-4 KB body stays on the leaf page, so a header scan still reads it (B
  census 214 MiB). The split removes both.
- Body reads do not regress under the split.
- Two shapes to forbid in the build: a `WITHOUT ROWID` body table (point
  body reads 14 -> 51 MiB cold), and a plan that walks an index and looks up
  the header row per entry for a whole-table answer (C census through
  `idx_ev_typed` read 234 MiB against 19 MiB for the scan). Plan tests pin
  both.
