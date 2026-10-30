# Ledger: choir-data-system-build-2026-10-30

Append-only. One terse entry per pass. Never edit earlier entries.

- 2026-10-30 ~06:15 UTC (FP0). Goal authored on the owner's word ("start
  building this ... iron out the kinks first"). The selection goal was
  superseded. Frame review round 1 at 3f4dafb7e:
  - Devin: accept_with_notes. Five paragraph-scale fixes, F1 to F5: the PSA
    proof bound, replay semantics against the §12 stake, a closed effect
    set, outbox backpressure, and fence before stop.
  - Sonnet: send_back. Seven blocking findings, B1 to B7:
    - B1, custody and audit overclaim; the DEK sits on disk.
    - B2, what the host reads; the class is writer-declared.
    - B3, PSA cadence, signing and per-object keys.
    - B4, the effect set and host-mediator barrier; file-sync loss.
    - B5, refusal, writer epoch, fsync and receipt durability; semantic
      kinds stay synchronous.
    - B6, class-pure events, chain integrity, restore composition and
      release.
    - B7, deletion safety: pending uploads, refcount, quarantine.
  - DeepSeek: still running.

  Output is in .agentic-consensus/frame-r1/out/. The overlap is high, and
  the next move is adjudication plus frame revision 2.
- 2026-10-30 ~06:30 UTC (FP0). Owner: "Remember, we are working at agentic
  speed. We can't let code review slow us down. We reviewed the plan a bunch
  of times for a reason. So we can do the migration in bulk, and let runtime
  checks be the verifier, not code review." Recorded in the goal:
  - D0 folds round 1 once; there is no round 2 and no Codex;
  - D1 and D2 run without panels;
  - B1 to B7 are built in bulk, enabled for the proof ring only;
  - verification is by B0's drills, the hard gates, the differential and
    deployed acceptance;
  - the CLAUDE.md review tiers are superseded for this mission.
- 2026-10-30 ~06:45 UTC (FP0). Owner: "Skip tonight's backup. Accelerate the
  migration cutover. Yes, do the fleet cutover, and delete old data.
  Downtime is acceptable ... Better to rapidly get it 85% there, and iterate
  for the last mile."
  - The backup is skipped: the timer was stopped and a runtime drop-in
    added (skip-during-cutover.conf).
  - The goal is restructured:
    - Phase 1 runs D0 (fold round 1, once), D1 (the build sheet), C0 (the
      drills), C1 to C3 in parallel (guest SQLite, PostgreSQL, tape v2),
      C4 (the migration, on d@e first) and C5 (the fleet cutover and
      deletion of the old data).
    - Phase 2, I1 to I7, iterates the rest.
  - Owner gates are removed.
  - The last off-host copy of the old data is the restic snapshot of
    10-29 03:28 UTC.
- 2026-10-30 ~07:05 UTC (FP0). D0 complete.
  - DeepSeek round 1 landed with send_back, B1 to B7, converging with the
    other two seats. Its distinct points:
    - the key file on the data image;
    - egressgrant as the effect taxonomy;
    - blob ownership per class;
    - the capacity arithmetic, which put the trigger near 15 to 20 heavy
      computers;
    - an audit-export consistency proof.
  - Frame revision 2 is 85eb4a55b; ledger round 0m is 5294d3ec6.
  - D1 started, with the Dolt-coupling census running.
- 2026-10-30 ~07:35 UTC (FP0). D1 complete: docs/design/data-system-build-sheet-2026-10-30.md. The integration branch cutover/data-system replaces landing on main; main gets the branch at C5. Next: WP0, the dbx seam, built by FP0.
- 2026-10-30 ~08:20 UTC (FP0). WP0 landed on cutover/data-system: dbx (70a850d31), objectgraph.NewSQLStore (a525e75e5), store.StateDBPath and LegacyDoltWorkspacePath (48048f0e6). Six Sonnet builders launched from 48048f0e6, one worktree and branch each (cutover/wp-a to wp-f), split by package boundary per the build sheet work order.
- 2026-10-30 ~06:50 UTC, Node B clock (FP0). The earlier entries' "~07:xx/~08:20" times were estimates; Node B's clock is the reference from here on.
  - The C0 keep-set snapshot (scripts/cutover/keepset-snapshot.sh, 4d3daebda on cutover/data-system) ran a dry baseline against live Store B in under a minute:
    - 3,473 publications and 11,989 versions;
    - 156,882 capture blobs, 0 missing;
    - 22,069 og body_refs, 0 missing;
    - the owner's mailbox has 255 messages.
  - The rehearsal plan changed. Node B has 9 GB of memory available, too little for a second pair of Dolt servers on reflink copies. So the host migrator reads the live Store A and B servers read-only, with one snapshot-isolated transaction per table, into a scratch PostgreSQL 17:
    - unit choir-rehearsal-pg, user nobody, MemoryMax=3G;
    - socket in /var/lib/go-choir-rehearsal/sock;
    - databases control and graph.
  - d@e's guest store was copied from a reflink of its data.img (ro, noload) to /var/lib/go-choir-rehearsal/de-guest: 600 MB of Dolt at state.texture/texture.
  - The guest layout: the store path is the empty file `state`, so StateDBPath puts the new store at state.db. state-actor.db is already SQLite.
- 2026-10-30 07:16 UTC (FP0). Merged into cutover/data-system so far:
  - WP-B, the guest object graph on SQLite (33cf3eebb). objectgraph tests pass, including -race.
  - WP-D and WP-E, Store A to PostgreSQL control and Store B to PostgreSQL graph (89eced3be). The full ./internal/platform suite passes on PostgreSQL.
  - WP-A, the guest store on SQLite (075b91a7a). ./internal/store passes.
  - FP0 commits: ad91b2cd5 ports three e2e tests off embedded Dolt; 46bfd9403 makes the projection sealer refuse a nil cipher.
  - go build ./... is red only in internal/projectionbase, which WP-C is deleting. WP-C and WP-F are running.
  - Recorded decisions:
    - Decision 10: the append receipts keep the signed receipt JSON; columns duplicated inside it are dropped.
    - The key-escrow transparency append takes pg_advisory_xact_lock, so concurrent appenders cannot fork the chain.
    - og object times are whole seconds, which the front cursor compares.
    - JSON stays byte-exact TEXT on PostgreSQL.
    - state.texture stays as the guest's sidecar directory (store-schema.json, floor, lease marker) and must not be deleted at C5.
  - Runtime checks owed on staging:
    - Event-body lookups that miss the guest's 1,024-entry cache now fetch from the host tape: watch latency on the paper's hot paths.
    - A 40001 serialization failure in routeledger is not yet mapped to a stale-transition result.
- 2026-10-30 07:30 UTC (FP0). All six WPs are merged into cutover/data-system (WP-F at 23d9b89a9, WP-C at 5da33247c, the hook untagged at 2b947216f), and `go build ./...` is green. Two Sonnet fixers are running: cutover/fix-tests covers the full suites and repo gates; cutover/fix-host covers snapshot chunks as file-CAS GC roots, the vmctl compaction leftovers and checkpointd.
  - nix/postgresql.nix landed on main as 07d74c278 (additive; CI and deploy running), so the production PostgreSQL exists before C5.
  - C4, guest rehearsal: d@e's store was copied from a reflink of its data image and migrated with scripts/cutover/guest-migrate-dryrun.sh. 26 tables and 26,275 rows; every count and digest equal; about 50 s.
    - 600 MB of Dolt became a 63 MB state.db.
    - Dropped columns are only computer_event_index event_json and event_head_receipt_json (decision 8) and the 8 og generated columns, which are now VIRTUAL.
  - C4, host rehearsal: the live Store A and B are going into the scratch PostgreSQL. Receipts copy at about 25K rows a second.
    - Problem: during the copy, the Store A Dolt server ran at 330% CPU and the paper's /health took 24 s (it was 200 before the copy), with a deploy nix build also running.
    - The copy is bounded to minutes, so it continues under watch. At C5 the paper is stopped, so this contention does not recur.
    - The dry-run mode does not bootstrap the target schema, so it reports every source table as dropped. Use the real copy into a scratch database instead.
