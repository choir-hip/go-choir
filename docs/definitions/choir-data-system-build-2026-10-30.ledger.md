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
- 2026-10-30 07:33 UTC (FP0). Problem: the live-source host rehearsal made the paper unhealthy.
  - Evidence: the 6M-row computer_event_append_receipts copy, with --parallel 2 and the deploy's guest-image nix build running at the same time, pushed Node B load to 14.6 and Store A Dolt to 330% CPU.
    - The paper's /health took 24 s, then returned 503.
    - vmctl logged it unhealthy from 07:31:01 to 07:31:56.
  - FP0 killed the migrator at 2.5 minutes. The paper returned to ready at 0.6 s.
  - Before the kill, the rehearsal verified 26 control tables with counts and digests equal: escrows, transparency, lifecycle receipts, checkpoints, file roots, heads and the rest.
  - Rule from here:
    - Never copy the receipts table from the live source. It is copied at C5, when the paper is stopped.
    - Rehearse the graph only with --parallel 1, under a watchdog that kills the copy when the paper's /health is slow or not 200.
- 2026-10-30 07:55 UTC (FP0). C4 host rehearsal, with --parallel 1 and a paper-health watchdog, into the scratch PostgreSQL:
  - Graph: 7 tables and 1,176,822 rows, counts and digests equal (og_objects, og_edges and the rest).
    - The source-only tables are either empty leftovers (counted live: 0 rows each) or derived retrieval tables (og_lookup_keys, proj_capture_*), which corpusd rebuilds from og_objects at start.
  - Control: 32 tables and 38,287 rows equal, every table except computer_event_append_receipts, which waits for C5.
  - Problem found and fixed: the migrator's bootstrap did not create vmctl's route ledger and input catalog tables (computer_version_*). The copy would have dropped route slots, transition receipts and authorization evidence.
    - Fix: on cutover/data-system ("bootstrap also creates vmctl's route ledger"), with a test.
    - Dropped on purpose: verifier_attestations and computer_version_route_authority_config, which nothing reads in the old or the new code.
  - The deploy of 07d74c278 did a planned refresh of the paper: killed at 07:34:26, booted at 07:35:02, ready at 07:38. Production PostgreSQL is active, with databases control and graph and the role and password set.
  - Devin reviewed the cutover's .github diff for the security-epoch gate: VERDICT no-security-effect. The epoch was raised from 3 to 4 on the branch (4b80413f9).
- 2026-10-30 08:20 UTC (FP0). Problem: the paper's guest store does not fit an in-boot migration.
  - Evidence: a rehearsal on a reflink of the paper's data image (34 GB, of which 31 GB is the Dolt workspace) ran choir-migrate-guest on the host, capped at CPUQuota=200% and MemoryMax=8G.
    - About 1,050 MB of SQLite was written in 4.5 min, roughly 230 MB a minute. Anonymous memory was about 2 GB.
    - The live paper's /health rose to 35–38 s and host IO pressure reached 12% full. The idle IO class did not protect it, so FP0 stopped the rehearsal at 08:02:51, partway through computer_event_index.
    - The extrapolated total is 25 min or more, against vmctl's 30-minute boot-ready wait. A failed wait deletes the partial file and boots again, so the migration would loop.
  - Decision:
    - At C5 the paper's VM is stopped through vmctl before the writers. Its image is migrated offline from the host, with full resources, while nothing else runs (runbook step 4b). Its boot hook then finds state.db and skips.
    - VM_BOOT_READY_TIMEOUT is raised to 90m for the cutover on cutover/data-system (9cf2bc493), so other computers, such as the owner's 5.2 GB image and the 146 hibernated ones as they wake, migrate at first boot without looping. It goes back to 30m in phase 2.
  - Guest Dolt deletion (C7) is on branch cutover/guest-dolt-delete (159b0f999). A boot deletes the legacy Dolt databases only when that computer's own migration report says ok, and it keeps the sidecars. The hourly fstrim gives the space back to the host. It lands after staging acceptance.
  - cutover/fix-tests is merged into cutover/data-system (8933d1ba9), and PR #108 CI is re-running. The fixes cover:
    - store guards;
    - per-test temp directories for the SQLite helpers, the cause of the bulk agentcore failures;
    - the routeledger retry on 40001, which closes the earlier owed check;
    - the decode-roots and switch-registry gates;
    - the CI lane lists.
  - The rehearsal copy and loop devices are removed.
