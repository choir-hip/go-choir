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
- 2026-10-30 08:36–13:30 UTC (FP0). C5, the fleet cutover, on Node B.
  - 08:36:17 the paper was healthy (0.76 s), then stopped through vmctl. 08:36:24 the old writers stopped: corpusd, vmctl, checkpointd and its timer. No clients remained on the Dolt ports.
  - Old keep-set, 38 s: captures.distinct_blobs 157,465 (missing 0), og_blobs.distinct_body_refs 22,112 (missing 0), textures.publications 3,479, versions 12,012.
  - The paper's guest store, migrated offline from its mounted image (step 4b), 08:38–10:10: 26 tables and 11,477,450 rows, counts and digests equal, ok=true. computer_event_index 5,128,721 rows, og_objects 3,920,901, run_memory_entries 1,710,687. state.db is 30.6 GB beside the 31 GB Dolt workspace; the Dolt directories go at C7.
  - Problem: the go-choir-dolt-liveness-probe timer killed host copy attempt 1.
    - Evidence: under the receipts scan, Store A missed the probe's 5 s deadline three times. The timer restarted platform-dolt at 08:41:03 and SIGKILLed corpus-dolt after its stop timed out at 08:43:01. The copy failed with "invalid connection".
    - The timer was stopped for the cutover; the deploy deletes it.
  - Problem: one ORDER BY scan of Store A's computer_event_append_receipts (about 6M rows) through the Dolt sql-server stops returning rows.
    - Evidence: attempt 1 stalled at 2.75M rows. Attempt 2 (08:44) paused 5.5 min at 2.75M, then stopped at 5.5M for 85 min. Dolt held one core at 95% with no I/O, PostgreSQL's COPY waited on ClientRead, and the Dolt CLI could not open a session. A SIGQUIT dump kept only the first 50 lines in the journal, so the spinning goroutine is not known (hypothesis: Dolt's long-lived iterator, not the data).
    - No rehearsal had copied this table in full: rehearsal 1 was cancelled in it, and the live-source rule held it for C5.
    - First fix, keyset pages (125aa70bc and two follow-ups): a single OR of all key levels was planned as a scan of the rest of the table, so attempt 3 got only the first page. One contiguous range per query (k1 = ? AND k2 > ?, then k1 > ?) worked but slowed from 1,300 to about 140 rows a second (attempt 4: 1.46M rows by 12:25). Paging stays opt-in, used only for a sql-server source.
    - Fix that worked (200af7867): the host migrator reads the stopped Dolt stores directly through embedded Dolt (source dolt-dir:<dir>#<database>), the engine that streamed the paper. Built on choir-agents with the guest bundle's libraries.
  - Attempt 5 (12:28:30, both Dolt servers stopped): ok=true in 10m33s. Control: 33 tables and 6,211,826 rows, including computer_event_append_receipts 6,173,497 rows at about 10,500 a second with no stall. Graph: 7 tables and 1,177,526 rows (og_objects 671,543, og_edges 505,983). Every count and digest equal. Dropped as in the 07:55 rehearsal.
  - Problem: both Dolt servers restarted while the paper's image was mounted, so their private mount namespaces kept its ext4 mounted after the host unmount (loop0 autoclear pending). Both units were stopped at 12:28 for attempt 5, which released it, so the paper never boots on an image still mounted on the host.
  - New keep-set (12:41): 95 of 96 lines equal. og_objects.ids_sha256 differed only because the Dolt CSV client prints an empty content_hash as "" and psql prints nothing. After normalizing the 589 such rows, the old and new og_objects lists are identical, and PostgreSQL holds '' (0 NULLs). Keep-set green.
  - Pushed the squash 871ab6531 to main at 12:43.
  - Problem: main CI failed on race non-runtime shard 3, so the deploy was skipped. TestValidatorFirstPassAndRepeatPassOnManyRows split its log lines at the midpoint to find the repeat pass, but progress lines are timed, and under the race detector the slower first pass logged more, so its own "6000 re-derived" line crossed the midpoint. A test defect, not a product one. Fixed in 16e6d80d7 (the check reads the lines logged after the repeat pass starts), pushed to main at 12:58.
  - Problem: the deploy of 16e6d80d7 failed its disk preflight: 40.6 GiB free after the nix reclaim, 60 GiB required. The paper's 30.6 GB state.db now sits beside its 31 GB Dolt workspace until C7.
    - With the differential and keep-set green (the owner's condition for deleting old data), FP0 deleted the most-verified old data first: Store B (the corpus Dolt, 18 GB, whose graph tables and keep-set checks matched) and the rehearsal scratch (the rehearsal PostgreSQL, 9.3 GB, and d@e's rehearsal copy). Free: 61.5 GiB. Store A and the paper's guest Dolt stay until after C6. The deploy was re-run.
  - Deployed 16e6d80d7 at 13:29:52 (CI run 41059427856, attempt 2). Staging /health reports it. The Dolt units are gone; PostgreSQL, vmctl, corpusd, gateway, auth and maild are active, and checkpointd runs from its timer.
- 2026-10-30 13:25–13:58 UTC (FP0). C6 on the deployed release.
  - The paper booted at 13:25 on 16e6d80d7 and found its offline-migrated state.db (store: open fresh=false), ready at /health in 11 ms. The crash-boot rule held: 142 source-poll wakes in flight at the 08:36 stop became dead letters ("the source schedule sweep re-arms the schedule"), and every source schedule stayed active with its next tick 15 to 30 min after boot.
    - The derived-key validation runs a full pass at each boot (actor_wake_outbox 157,144 rows at 44 rows/s, about an hour). It held the store once (store_stalled at 13:27:13). The old release logged it every 1 to 2 min before the cutover and took 52 min for the same pass at 08:27, so this is not new. It is a phase-2 item (persist the validated state).
  - The owner's computer migrated at first boot: OK, not skipped, 13:25:36 to 13:29:43 (4m07s). Ready on 16e6d80d7. Its console shows only Firecracker's RTC-port boot noise.
  - d@e: the deploy refresh was deferred by selfdev_active_operations=1. Its operation rollback-4dca8039 has been "rollback_pending held: guest apply attempts spent" since at least 2026-10-29 21:37, before the cutover. FP0 forced the refresh (d@e is the drill ring). It migrated at first boot: OK, not skipped, 7 s.
    - Residual problem: the held rollback came over with its state. It blocks d@e's hibernate ("the guest is busy") and its deploy refreshes. Clearing it needs d@e's owner act.
  - Drills on d@e:
    - boot, wake and serve: ready after the refresh. Hibernate and wake are blocked by the held operation.
    - kill -9: Firecracker SIGKILLed at 13:36:36, resolved, booted at 13:36:45. The store reopened (fresh=false, no re-migration) and is ready. The crash boot closed the open work and marked the held rollback "interrupted ... never re-applied".
    - Owed: no acknowledged write lost (needs a write acknowledged just before the kill), a stale-epoch append refused, and bounded migration memory. None has a harness on the real system.
  - Keep-set on the live system after the deploy: 96 of 96 lines equal to the post-copy snapshot. The owner's mail tables are among them.
  - The paper publishes on the new system. Its first source ticks came due at 13:41. By 13:58: publications 3,479 to 3,482, versions 12,012 to 12,023, og_objects 671,543 to 672,506, the newest created at 13:58:16. The story gate still refuses single-outlet stories.
  - Mail: maild restarted on the new release at 13:25. The last inbound webhook was at 00:55, before the cutover, so mail arrival on the new system is not yet observed.
  - guestmem defect found by CI on PR #108 and fixed (550531ff2): a page-cache tree rooted at a file was read as a directory, so the store split rooted at state.db reported 0 bytes.
  - Review: Devin, DeepSeek v4.1 flash and Claude Sonnet review 8e1aa3076..16e6d80d7 against the plan and this runtime record (started 13:57).
  - Next: C7 deletes Store A and the guest Dolt workspaces (the paper's in its image, the rest through the cutover/guest-dolt-delete boot hook). Phase 2 restores VM_BOOT_READY_TIMEOUT to 30m, persists the derived-key validation, and adds the owed drills.
- 2026-10-30 14:00–14:40 UTC (FP0). Review findings acted on, and the paper store measured.
  - Problem (found by the Sonnet and DeepSeek review seats, independently): the host copy never advanced the one identity sequence. computer_key_escrow_transparency.seq is GENERATED ALWAYS AS IDENTITY; COPY wrote seq 1..4014 and the sequence stayed at 1 (is_called false). The next escrow append (realization key delivery, pre-genesis replacement, operator reveal) would take a primary-key violation. None had run since the cutover (newest row 08:35). Fixed on the live database at 14:13 with setval to max(seq) (now 4014, is_called true). Rollback: setval back. The migrate package is deleted at C7 and no further host copy is planned, so the copier is not patched.
  - Paper store per kind (14:19, a private reflink snapshot, sizes only): og_objects carries about 8.9 GB of body and metadata across 4.1 M objects; choir.event (run telemetry) is 2.49 M rows and 4.86 GB of it; no object of any kind is tombstoned or superseded. computer_event_index holds 5.15 M rows and run_memory_entries 1.72 M. The 30.7 GB file is retention not built, not SQLite overhead. R0f-04 (paper 3 GB [1, 10]) is falsified for that cause. The per-table byte count (dbstat) is owed: its watchdog stopped it when the paper's own validator stalled the store (store_stalled 14:18:47).
  - Steady state (PostgreSQL statistics, 14:16 to 14:31): 9.6 appends a second fleet-wide; about 14 control commits per append; 4 sequential scans of the 54-row control_key_history per append.
  - Security (not a cutover defect, found in the redesign reading): a bound guest could write public routes and publications through the generic object route. Problem record docs/problems/guest-can-write-public-routes-2026-10-30.md (9adadd947); fix 3a1232242. Routes in production are all host-written.
  - Design: docs/design/object-graph-redesign-2026-10-30.md (9adadd947), the redesign of both graphs and the bridge.
  - Deployed: the guest route fix 3a1232242 (CI 41072790737, deployed 14:42:00). After it, corpusd and the proxy logged zero refusals, and the paper's typed writes went on (19 captures, 47 support judgements, one publish). The escrow reveal fix 7812e5571 is deployed inside 03f11f5ec (CI 41075058960 green, deployed 15:01:47). Its own run 41074827690 failed on a flaky TestManagementDelegatesIntakeToTasks (agentcore, not touched; 20 of 20 pass locally), and its partial rerun failed SBOM candidate validation. Both are superseded by the full green run on 03f11f5ec.
  - Review panel (Devin, DeepSeek, Sonnet) is complete: scratchpad review/run1. Confirmed and acted on: the escrow sequence (live setval), the reveal ordering (7812e5571). Open, and C7's old-data deletion waits on them: no production snapshot producer, and the missing-disk check at autoputer/projection_base.go:151 is always true. These two land together (design doc §7, phase 0). Also open: the witness and checkpoint publish hold the append mutex and the sole write connection; 40001 errors become 400 errors; the d@e deploy-refresh retries without end; migrated prepared rows have null bodies (latent); the differential passes over dropped columns.
- C7, host (15:17 to 15:25): a compressed pg_dump of control (2.4 GB, 148 list entries) and graph (312 MB, 59 entries) sits in /var/lib/go-choir/backups/pg-2026-10-30, unit exit 0. Store A (/var/lib/go-choir/platform-dolt, 56 GB) is deleted: no process held it, and the gateway unit names it only in InaccessiblePaths with the ignore-if-missing dash. fstrim returned 121 GiB; host free went from 57 GB to 109 GB. The drop audit behind this: every host table the copy did not carry was empty, derived, or unread, receipt columns live in the signed JSON (decision 10), and guest event bodies come from the tape (decision 8). Guest Dolt deletion: 337c659c2 with the zero-byte guard 63b41761a (CI 41079644578).
- Deployed e2be9bdad (CI 41081850282, deployed 15:50:11): the event append retries 40001 inside the host and answers 503 for one that outlives the retries; the guest security epoch 4 -> 5 (C7's migrate-hook commits had no Security-Effect trailers, CI 41079644578 failed on it); the checkpoint witness on one read snapshot with no append hold (Builder B; a computer whose head moves during the scan still holds appends for one scan on the read pool, until the platform accepts a checkpoint at an older head); the snapshot producer (Builder A). After it: the paper's planned refresh deleted its guest Dolt at 15:49:32; the owner's computer took its first snapshot at 15:54:09 (head 596106, 2.06 GB sealed, 44 s). The producer copies with VACUUM INTO a temporary file capped at 8 GiB, so the paper (30 GB) is refused: the design doc's streamed chunks are still owed. Interim: a crash-consistent reflink copy of the paper's data image at /var/lib/go-choir/backups/paper-interim (not a snapshot; never restored from, never leaves the host; delete when the paper records one). The producer's first lookup of the newest snapshot walks the whole event index when none exists (no event_kind index; adding one changes the v3 schema hash, so it waits for phase 1).
- Defect: the paper's derived-key validator ran at 47 rows a second (console log 15:33), still on its third family two hours after boot. Each 200-row page compared on the busy write connection, and the 0.25 duty cycle slept three times the wait. Fix: compare on the read pool, repair only differing ids on the write connection. This replaces the design doc's phase-0 "persist the validated state", which carried the stale-marker hazard.
- f82f9ad2e deployed 16:05:08 (CI 41085327612). The paper's validator on the read pool runs at 81 rows a second (from 47): the write connection was part of the cost, not most of it. The rest is row layout: og_objects puts updated_at after body and metadata, so each compared row reads its whole body, overflow pages included. A covering index or a column reorder is DDL, which witness v3 forbids until phase 1; the validator now blocks no writer and phase 1 deletes it. Layout rule for the class tables: fixed-size columns first, bodies last or in their own table.
- Defect (review F6/F12): vmctl's deploy-refresh sweep asked a computer whose guest keeps refusing (d@e's held rollback) every minute without end and said nothing. Fix: after 30 attempts the gap doubles from 2 minutes to an hour, never dropped, with one escalation log line naming the reason.
- Phase-1 panel (Sonnet and DeepSeek in; Devin running), scratchpad panel-phase1/run1. Both keep snapshot choice (b), guest-streamed and sealed, and say to stream it now; Builder C is building page-chunked streaming from a read snapshot. filecas.SealChunk uses a synthetic nonce, so unchanged page ranges dedupe. Both split phase 1: the logical witness, the manifest on the existing choir.app-package/v1 seam and the enforced writer first; the class tables later. They disagree on the witness: Sonnet keeps LtHash plus a hash chain for logs, DeepSeek recomputes at snapshot time with per-class Merkle roots. DeepSeek: the store does not make a precommitment record immutable (only the minting caller's Exists:false condition does); an overwrite still lands on the receipted tape, so it is visible, not silent. Phase 1 needs an immutable act class enforced by the substrate. No phase anchors heads, which the precommitment doc relies on against backdating.
- Owner, 2026-10-30 ~16:40: choir.event takes the default (split by kind; trace and progress kinds leave the working store, and stay on the sealed tape: the trim is a tape event so replay agrees). d@e: "There's no important state on D at E", make it current and usable for testing; go-ahead to reset it. The paper runs without a backup tonight ("we're just going to have to risk it"). The tape must become immutable and tamper-proof (computer signatures; anchoring to be added to the design). Agent traces in user computers are the users' data: kept, sealed, preserved. Upcoming: hypervisor bake-off, a local Mac app (later Windows, Linux) running the user's computer and syncing state with the host; enterprise hosts on their own metal or cloud, syncing to employee workstations. The tape protocol is therefore a replication protocol, and snapshots must be hypervisor-neutral.
- Checkpoint cadence, measured: the periodic cadence (2,500 events or 60 min) runs only where vmctl names it (VM_CHECKPOINT_CADENCE_ONLY = one computer plus class:test). The paper and the owner's computer bind checkpoints only at planned platform updates and self-development applies.
- d@e made current (16:32): an operator force refresh (vmctl /internal/vmctl/refresh, a crash boot) booted 3c1d0767a. The existing crash-boot settlement bound closed rollback-4dca8039 degraded ("the settlement did not finish on two boots"), so it stays visible for an owner act and no longer counts as active work; d@e is ready with 0 runs and 0 capsules. No code change was needed. d@e's chain: 12744 events, 89 checkpoints, no snapshot root before this boot. The drills owed on it (kill -9 under load, stale epoch, hibernate and wake, bounded memory) are running.
- Defect: the snapshot producer copied the store with VACUUM INTO a temporary file capped at 8 GiB, so the paper (30 GB) could not record one (evidence: the e2be9bdad line above). Fix: one read transaction on the read pool reads the head first, streams the database pages through sqlite_dbpage (compiled in from modernc.org/sqlite v1.49.0; v1.48.2 lacks it, so go.mod and the vendorHash move with this change), computes the witness in the same transaction, and uploads fixed page-range chunks the host does not already hold (HEAD on the chunk route, which also claims the chunk against the GC grace window). No copy, no size cap, no 1.25x rule; a 30 minute time budget (CHOIR_SNAPSHOT_MAX_DURATION) bounds the WAL pin. The record format is unchanged (same 4 MiB cut; page_size, page_count, pages_per_chunk are added), so snapshots recorded today still restore.
