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
- 2026-11-14T05:58Z. Landed: the store reading can name its trees (801f76b3e, deployed
  05:25Z; expedite; its CI run failed only in the SBOM accept job, a cache
  race recorded in docs/problems/sbom-accept-job-failed-on-801f76b3e-2026-11-14.md).
  The paper's store, 47.5 GB: og_bodies at least 12.4 GB (cut), the
  run_memory_entries table at least 11.2 GB (cut) with 2.5 GB of indexes,
  og_objects 5.4 GB without its 19 indexes (being read). Late take: both
  seats REPAIR in round 1 (a write could pass acknowledged telemetry; a
  caps-excluded leader's write survived its panic), both ACCEPT in round 2
  (679999c8d); lands after the counters. Boot-time prune
  (lane/prune-at-boot): round 1 DeepSeek REPAIR (keep rule missed a fenced
  installed apply), round 2 DeepSeek REPAIR (a store object freed in place),
  round 3 DeepSeek ACCEPT, Gemini REPAIR (the nix unit test's CI wiring is
  classified docs-only). Both seats hold the mechanism sound; by the
  three-round rule it does not land without the owner. C80 false. Receipts
  first steps: both seats REPAIR, one step breaks revert-first; split into
  two landings. Lifecycle stored result (cfbbdd29a) is with both seats.


- 2026-11-14T08:05Z. Counters deployed 07:06Z (a21558f9b under f4108ff50; CI 43567350353, test flake rerun green, SBOM accept job failed again). Paper, first hour: carried run memory 55.9% of message bytes (C74 true); choir.run 22.6% of object bytes at 4.7 rewrites per run, 18.7 KB each; tape 150 MB/h. Node A OOM killed the agents VM at 06:42Z: landings on the runner only. Receipts step 1 accepted by both seats; step 2 stopped after a third REPAIR, assessment written, the index swap becomes a supervised script (2b) and the writer and walk stay code (2a). Prune-at-boot: both seats accept round 5 at 92241b062, should-fixes at 3ae395c89, held for the owner. Train on the runner since 07:06Z: receipts step 1, late take, census.

- 2026-11-14T09:51Z. Train 79dd71b57 (receipts step 1, late take, census) landed 08:35Z, CI 43576567543 green, deployed 08:47Z. C82 true. C79 false: late take is correct but did not lift group size (1.16); writers queue at the appender. Census on the paper: 5.14 M entries in 151,550 runs, 0 retired; newest-two-of-agent holds 1.70 M entries and run-record-not-found 1.39 M; builder started on R-1. Receipts 2a/2b first review: concrete repairs (Bootstrap index race; scripts need CI and allowlists), in repair as one stacked lane. Stored-result: code accepted both steps, test and record fixes in progress.
