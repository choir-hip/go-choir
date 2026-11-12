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
