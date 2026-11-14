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
- 2026-11-14T04:04Z. Coalescing deployed 02:37Z. The paper's first hour
  (02:37Z to 03:35Z): 13,369 events, 44,409 ops, 192 MB; 3.32 ops per event
  (was 1.47), 1.5 corpusd calls per op (was 4.1). Lossless (refused 0,
  unknown outcome 0) but below C68's claim: 8,344 groups, mean 1.37 members,
  77% with one. Cause read from the code: the leader takes its group when it
  wins the token, before it waits behind a telemetry seal and the appender,
  so later arrivals miss it; about 30% of writes never group (conditional
  batches, telemetry seals, other kinds). Builder on lane/late-take. Honest
  bytes by kind for that hour: choir.event 20.3%, choir.run 19.4% (23.5 KB
  per op), source_ref 4.8%, lifecycle_command 4.2%, texture_revision 3.6%.
  The store-table reading landed through the runner (d4a27b2a2, CI
  43552949573, deployed 03:57Z; expedite). d@e's store is 1.11 GB:
  run_memory_entries 38%, og_bodies 30%, og_objects 25% (two thirds of that
  its indexes). On the paper (47.4 GB) the reading saw 8.9% in 67 s: the
  flat budget share gave each big table 2 s. C73 stays open; builder on
  lane/store-only so a request can name the trees. Updater prune round 3 is
  with both seats; its suite run is queued behind the counters' on the
  agents VM.
- 2026-11-14T04:20Z. Updater prune round 3: Gemini REPAIR (the bounded hold
  starves against the chunk loop), DeepSeek ACCEPT. Third REPAIR, so by N1
  it does not land. Assessment:
  docs/problems/updater-prune-three-rounds-structural-assessment-2026-11-14.md.
  Seven of eight blocking findings were the cross-process coordination; the
  keep rule was verified sound. Changed approach: prune as a boot unit
  before the updater and the runtime start, so no hold exists. C78's odds
  fall (a new candidate, new review).

