# Ledger: storage solutions, part 3 (store once)

Standing: built | kind: happened | scope: in part | settled-by: the ledger of docs/definitions/choir-storage-solutions-3-store-once-2026-11-14.md (a512e78d5, 824a01aab; now.status working at line 248) | note: T1 and T2 were design lanes at the last card

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

- 2026-11-14T15:44Z. Stored-result step 1 landed 12:46Z (a1ec394fe, b2494ce52, d90131351), CI 43596488443 green on rerun (two unrelated timing tests), deployed 13:12Z; C83 true. Receipts step 2a and 2b landed 13:16Z (tip 61c92229b), CI 43598955268 green, deployed 13:33Z; the format-2 walk reached the paper about 14:04Z and stood at 1.25 M of 5.1 M rows at 15:34Z, table +2.1 GB, the paper's ten-minute append p95 never above 7 ms (C84 open). Flake lane (six tests, feeda0550) is landing. Run-memory retirement R-1: round 2 DeepSeek ACCEPT, Gemini REPAIR (cursor-only runs took carry slots; reader and selector disagreed); repaired at f776f375b, round 3 with both seats, the last round. Stored-result step 2: delta review REPAIR (create and fork answered 400; a proof step that could not fail), repaired, Gemini ACCEPT at 1bdf94fe9, lane tip 6299ddc61. **Step 2 is held, not landed**: N2 needs the d@e proof before the paper takes it, and a landing cannot give that. The deploy expedites any release touching internal/agentcore, the ring soak floors are zero and there is no hold or pin (internal/vmctl/ring_walk.go; .github/scripts/node-b-ci-deploy:639-646), so the paper takes a landed build within about ten minutes. The only per-computer route that exists is a self-development apply on d@e, which rebuilds bin/autoputer from a patch the computer's own engineering desk freezes against the booted base; no verb applies a branch, and the candidate-on-d@e design (docs/design/candidate-on-de-before-main-2026-11-10.md) is not built. The drill's restore and the apply's revert are owner acts. Residual, named: the route is the owner's decision (self-development apply on d@e; restore the ring-0 soak floor; or build the candidate pin).

- 2026-11-14T22:14Z. Since 15:44Z, all landed, expedited and deployed: flake fixes (afb7ec449, 16:20Z, tests only, no deploy); run-memory retirement (d46fb88b7, 18:11Z, CI 43625197867, both seats ACCEPT after four rounds); Keenable 403 tolerance (5af04baad, 18:13Z, CI 43625363952); stored-result step 2 under N6 (fdbe766d6, 20:21Z, CI 43637074875); prune at boot (c1ea7a418, 21:20Z, CI 43642246845; freed 5.4 GB on the paper, 10.6 GB on d@e). Paper: lifecycle command 5.8 KB per op against 31.1 KB; retirement deletes 5,000 runs a pass, census 127,704 runs and 4.12 M entries at 22:01Z (from 149,956 and 5.09 M), every wake so far cut short by a deploy restart. C83, C84 true. d@e: step 2 re-submits pass before and after a restart; replay-completeness answers not_equivalent on the event index only, not a step 2 effect, cause unknown. Receipts index swap run 22:04Z to 22:12Z: partial index built, two old indexes dropped, 5.03 GB of index gone, paper appends flat; C88 true. Open: C85, C86, C87; proof steps 6 and 7 (owner); the event-index divergence.

- 2026-11-15T04:20Z. Restore works on d@e. The canonical checkpoint witness (version 4) landed 03:27Z as 304a81277 (red, two seats, expedited; CI 43672725876), deployed 03:47Z. d@e bound checkpoint 998cd98d... at 04:04:55Z (sequence 114,093); restore answered 200 in 283.1 s with `witness_matched: true` (base 108,786, 5,307 tail events); replay completeness reads `equivalent` at sequence 114,177. C91 partial: the realization epoch stays 13709 across an in-place restore, my clause that it advances was wrong. The paper's compact copy was adopted at the 03:43Z planned restart, 46.6 GB to 38.8 GB. Open: restore from a chain root replays up to 360,000 events and cannot fit one request (lane 3, restore as an accepted act with a status poll); checkpoint lookup still re-encodes on idempotent replay. In landing: capture-bodies report and the snapshot producer (C92, C94).

- 2026-11-15T06:01Z. Capture-bodies report and snapshot producer landed 04:50Z (57ff86475, c7eabe010), deployed 05:33Z under cc8ea803d after a CI repair (a restore-lane test took 501 s under race). C94 true: producer block on d@e and the paper, d@e restore 200 again. C92 false: the first report finds 46 eligible capture bodies, 5.8 MB, of 416,916; the owner had already ruled deletion stays off, and the report says there is nothing to gain. Owner, 05:50Z: solve paper restore storage-efficiently going forward; restore is not urgent, Autopaper data is not precious yet. Design proposed: docs/design/restore-bases-store-once-2026-11-15.md (page-delta bases, the compact copy as the full base), with two seats; its first measurement (C97) is on the build VM. Correction from that design: the paper replays at 77 events a second, measured, not 113; its tail is about 68 minutes. Restore as an accepted act (lane/restore-act) is being built at low priority. Node A: agents VM at 4 GiB, 534 GiB collected.

- 2026-11-15T14:28Z. Since 06:01Z. Restore: the bases design drew three REPAIR rounds and is set aside; the restore-act lane drew three more and is stopped (two structural assessments under docs/problems/). Reading the boot path showed a restore is undone by the next boot or append in every form built so far, so both lanes are parked until the forward restore event of the 2026-10-30 design exists (owner decision, default no). Capture on a static copy (docs/design/capture-static-copy-2026-11-15.md, accepted in round 3; code accepted by DeepSeek and Sonnet) landed 13:57Z as 842e0fda6 on the second attempt (CI 43721684380), deployed to Node B, the paper and d@e by 14:23Z; expedite. No computer has captured with it yet: C99 (paper pin at most 775 s against the 1,551 s baseline of the capture at head 10978774; the newest capture's own figure was lost to a restart and log rotation) and C100 (d@e) are open. Gemini answered 429 on four review attempts; seats were DeepSeek and Sonnet. C97 (page-churn bench) not yet run. The `now` card was rewritten: T0 and T7 are done, T4's walk and index swap are done, T1 and T2 are the unbuilt levers and each starts as a design-only lane.

- 2026-11-15T15:34Z. T1 (carry by reference) was designed with no code, drew REPAIR from both seats in rounds 1 and 2 and from DeepSeek in round 3 (Sonnet ACCEPT), and is parked by the three-round rule: docs/problems/carry-by-reference-design-three-review-rounds-structural-assessment-2026-11-15.md. Re-rank from the paper's counters since 10:31Z: carried messages are 12.8% of tape bytes (23.5 of 182.7 MB), choir.run 21 to 23%, choir.event 17%. T2 goes first. On d@e the run record is 70% metadata, most of it the growing list texture_available_source_entities; no read path shows single writes, so a counter of run writes by site, state and changed field (lane/t2-counter-land 7fcd5d715, yellow, C101) is in its landing run. My goal-file edit at 14:40Z used a status word the gate refuses and failed that lane's first landing at the gates; repaired in 77c31da3d.

- 2026-11-15T18:21Z. T2, first slice, deployed. The run-write counter landed 15:48Z (6387e5dff, yellow, CI 43731926836, expedite) and read on the paper for 1.6 h: 1,373 run-record writes, 17.25 MB of body, 81.8% of it the metadata key texture_available_source_entities, rewritten unchanged on most writes; counter ops equal the tape's. The design (docs/design/run-record-written-less-2026-11-15.md) began as a new object kind, drew REPAIR from both seats, and was repaired by deletion: the list is not stored at all, the four self-development join ids derived from it are. Both seats ACCEPT the design (round 2) and the code; Sonnet ACCEPT on the precedence delta. Landed 17:44Z as 8d81b7fe5 (orange, CI 43742756636, expedite), on Node B and the paper by 18:21Z. First reading on the new code, about 15 minutes: 740 writes, mean 3,254 bytes a write against 12,562 before; the list remains in 18 writes of runs started before the deploy. C101 void (its six-hour window was ended by this landing); C102 open until two uninterrupted hours have passed. Also recorded: tape object ops carry more than twice their payload (docs/problems/tape-object-ops-carry-more-than-twice-their-payload-2026-11-15.md), read from code, not measured on disk.

