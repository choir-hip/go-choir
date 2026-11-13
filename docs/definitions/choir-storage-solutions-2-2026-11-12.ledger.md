# Storage solutions, part 2: ledger

Append-only. One terse entry per pass.

- 2026-11-12T20:30Z. Authored on the owner's direction. Start: 81 GiB free,
  draining 6.2 GiB an hour from failed paper snapshots. Done before
  authoring: root's stale package caches deleted on Node B (about 4.4 GiB);
  destroyed-computer sweep dry run started (ts-20261112T202150Z-a31549a9,
  no archive, fork lineage acknowledged); lanes appender-wedge,
  snapshot-loop and storage-guards in build; write census running. Claims
  C57 to C59 entered; C59 settled partial.
- 2026-11-12T20:50Z. Write census in (docs/evidence/tape-write-census-2026-11-12.md).
  Group commit exists behind tape.group_commit, default off, ruled deleted
  by the owner on 2026-11-03 for its loss windows. 62% of events are
  run-memory writes, one entry each; an event is four corpusd calls. C58
  settled false (ephemeral share about 15%). S5 rescoped; decision D8 added.
  lane/storage-guards built, in one-seat review.
- 2026-11-12T21:10Z. Owner rulings: D1 default (no local tape dropped;
  revisit before Autoradio goes to TestFlight); D2 to D7 defaults; D8 group
  commit should be on (lossless coalescing only). Jev: use the vendor's
  limits, not our 30,000-byte guard. lane/storage-guards repaired after a
  REPAIR verdict (two blocking findings) and is in scripts/land.
- 2026-11-12T21:30Z. lane/storage-guards landed (cca135073), not yet
  deploy-verified. Throughline gains the status `observing`; this mission
  carries an observation plan and hands the live station to Autoradio when
  it enters that status.
- 2026-11-12T21:40Z. Guards deployed (cca135073). The deploy restarted the
  platform service and killed the first sweep dry run after 32 minutes;
  re-run as ts-20261112T205401Z-5755efcc. Swap file resized by hand to the
  configured 4 GiB: free 73 to 84.7 GiB. Goal file moved to throughline
  version 5: stations carry status and carrier, conjecture bound to claim
  C61, S5 rewritten from the census, S9 (status scripts) added.
- 2026-11-12T21:50Z. Correction: the new throughline format is version 7, not 5.
  Old files already carry versions 5 and 6 from earlier experiments.
- 2026-11-12T21:15Z. Seat 1 on lane/snapshot-loop: REPAIR. Blocking: the
  3-hour chunk window can delete chunks of a capture that spans hibernation,
  and the root is recorded unchecked; the backoff was counted from the
  attempt's start, so a 90-minute abort retried at once. The window change
  is dropped from S0 and moves to S4 behind root-time verification. Four
  skill-text commits each triggered CI and a deploy; lane/ci-docs-like
  built to stop that. Free 78.9 GiB at 21:08Z.
- 2026-11-12T21:35Z. S1 proved: forecast at 21:25Z reads free 77 GiB, slope
  -12.2 GiB a day, 1.4 days to the floor; 24-hour growers: paper chunks
  +27.8 GiB, nix store +13.4, paper image +9.3, tape +2.6. C60 true. The
  fast-drain line said ok at 11.8 GiB in 6 hours, so its threshold is too
  lenient for today's drain; and the nix store grew 13.4 GiB in a day of
  many deploys. Seat 2 accepted the repaired snapshot tip 67288e86d; seat 1
  re-review running. C62 entered. The second sweep dry run died at the
  0d0c4f164 deploy (a skill-text commit); it is re-run after the snapshot
  deploy.
- 2026-11-12T22:35Z. S0 landed as 0d2f0bd46 (both seats ACCEPT on the tip),
  deployed 22:31Z, expedited. The paper has not taken it: every planned
  refresh is refused while a snapshot streams, and on the old code one
  always is (docs/problems/snapshot-loop-blocks-its-own-fix-2026-11-12.md).
  Free 67.8 GiB. Sweep dry run ts-20261112T223140Z-d1a3ccd3 running. An act
  on the paper is needed; asked the owner.
- 2026-11-12T23:30Z. On the owner's word ("stop and restart the paper and
  do what works") the paper's VM was stopped on the host at 22:52:11Z
  (POST /internal/vmctl/stop); the always-on sweep booted it at 22:52:48Z on
  0d2f0bd46. First snapshot on the new code recorded at 23:28:18Z, head
  10978774: 44.1 GB of pages in 25m51s (witness 22m37s, stream 3m02s), 10,524
  chunks, 5,858 uploaded (4.04 GB sealed), 4,666 already held. Free space
  69.2 GiB at 22:55Z, 79.7 at 23:05Z after the hourly collector, 75.1 after
  the snapshot's upload. The paper published a story at 23:20Z; its call
  rate after the boot is being watched. Sweep dry run still in references
  after 58 minutes.
- 2026-11-13T08:00Z. Overnight landings, each its own deploy. S9 status
  scripts 29666dc47 (02:07Z). S2 wedge fix d6b88cd63 (05:55Z; both seats on
  round 3; the first land run timed out on a loaded build VM, the rerun
  passed); the corpusd-crash drill on d@e passed with no re-send or catch-up
  line in the guest's log, so the lost-reply path did not run on staging
  (C63 partial). S7 sweep progress 421288af7 (06:15Z). S4 part 1 root-time
  chunk check afe50a926 (06:54Z; both seats on round 3). S5 part 1 counters
  e392ac824 (07:54Z; CI needed three attempts: a timing test in
  internal/toolregistry failed once, then a partial rerun tripped the SBOM
  acceptance job). C62 true: free space rose 6.7 GiB in the six hours to
  05:35Z. lane/restart-cancels-capture parked after five rounds (C64 false;
  assessment in docs/problems/snapshot-loop-blocks-its-own-fix-2026-11-12.md).
- 2026-11-13T08:00Z. The sweep cannot finish: dry run
  ts-20261113T065544Z-903217d8 walked the paper at 13.3 minutes per 50,000
  events; the reader probes every segment for every event
  (docs/problems/tape-sweep-walk-probes-every-segment-per-event-2026-11-13.md).
  Repair in lane/sweep-walk-one-view. Dry run ts-20261113T075407Z-73f33629
  keeps adding progress meanwhile. First counters on d@e: 7.2 corpusd calls
  per tape event (two heads and two pins per append), appender depth 1 for
  139 of 165 appends.
- 2026-11-13T08:00Z. Station change, as told to the owner overnight: Autoradio
  is the live station; S7, S5 and S4 continue behind it; scripts/status-hourly
  is the tripwire.
