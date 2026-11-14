# Ledger: storage solutions, part 3 (store once)

- 2026-11-14T01:15Z. Authored on the owner's word that the refactors are the
  top priority ahead of Autoradio. Start: main eecafd3f1. In flight from part
  2: coalescing (landing), the updater prune (review round 2), the per-table
  store reading (one seat accepted). The design is unreviewed; T0 sends it to
  two seats.
- 2026-11-14T01:30Z. Both seats sent the design back. The per-kind table
  came from a counter that gives a batch to its first op; three of five
  named kinds are written once; retirement already runs daily; payloads are
  already compressed; receipts have more readers than a retry. C72 settled
  false. Design revision 2: the duplicates are copies (the carried
  conversation tail, the lifecycle command's stored result, tool output),
  plus the run record written whole. Stations re-cut; the four size targets
  withdrawn until T0's counters and reading exist. C74 entered. The per-op
  counters builder was told to attribute by every op.
