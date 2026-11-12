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
