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
- 2026-11-14T02:22Z. Coalescing landed on main (30c9b1f34), suites green on
  the agents VM. Node A gained a runner VM for landings (choir-runner, no
  logins; scripts/land takes LAND_VM_MIRROR, e6bdc2db6); the store-table
  reading is the first landing through it. Owner rulings N1 to N5 recorded.
  In flight: updater prune round 3, per-op counters, the lifecycle command's
  stored result, receipts first steps (the table is 30.1 GB, the paper 27.6,
  twice what the design said). Carry by reference waits on the counters:
  only the Texture document agent carries a conversation forward.
