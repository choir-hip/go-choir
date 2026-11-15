# Node B storage review, 2026-11-12 20:15Z

Tags: [O] observed (source named), [I] inferred, [?] not determined. Read-only: no
change was made to the repo or to any host. Nothing under secrets was listed or read.
Raw copies of the growth ledger sit beside this file (gm11.jsonl, gm10.jsonl) in the scratchpad.

## 1. Last week's storage work, and where it stands

### Which mission
There is no single "storage mission" last week. Three records make it:

1. **Lean system** (docs/definitions/choir-lean-system-2026-10-30.md, ledger and lanes beside it), 2026-10-30 to 11-01.
   Stations L0 (free Node A and B disks), L1 (paper's state.db from 31 GB to at most 3 GB, relaxed by the owner to "what is realistic"),
   L5 (streamlined off-host backup). Status in docs/ACTIVE.md line 94: "paused 2026-11-01, checkpoint_incomplete". [O]
2. **Disk incidents and the clustering assessment**, 2026-11-11:
   docs/problems/node-b-deploy-refused-disk-headroom-2026-11-11.md (commit 1abd0ebab) and
   docs/problems/node-b-storage-clustering-assessment-2026-11-11.md (commit 68f950d15). Owner: "clean up the disk and improve our mechanisms for storage release." [O]
3. Ledger claims C48/C49 (docs/calibration/claude-ledger.md): the metadata balance and the re-run deploy.

### What landed (commits and ledger) [O]
- Paper state.db 32.4 GB to 20.4 GB (S5, planned restart 10-31 12:53Z), incremental auto_vacuum.
- Receipt compaction active on the paper, the owner's computer and d@e (new paper receipt bodies about 469 B vs 1460 B; ledger 11-01). The paper backfill rewrote 0 early rows at that time.
- Snapshot sweep (lane/snapshot-sweep, proved 10-31 19:41Z gen 1649: 1.4 GB + 0.64 GB discarded).
- Release dedupe, retention by reference, jail cleanup, nix retention after activation, layered image.
- Destroyed-computer tape sweep: dry run, then real deletion on 10-31 (docs/reports/start-here-2026-10-20.md: 162 computers, 760k files, 2.3 GB; docs/reports/the-day-the-paper-got-lighter-2026-10-31.md).
- Loose tape (10.9M files) packed into segments and deleted by `taperepack` on 11-01 (clustering assessment Q4). Node A backup repo pruned 426 to 114.6 GiB (ledger 10-31 17:20Z).
- tapestore.CompactSealed built (ledger 10-31 18:15Z: "Not run on Node B" at the time).
- Growth meter timer (hourly, metadata only) armed 10-31.
- 11-11: metadata balance by hand (C48 settled FALSE: expected at least 85 GiB free, got 79.8; metadata 25.1 to 9.5 GiB allocated). C49 true.
- Since 11-11 no storage commit has landed (git log since 2026-11-11 shows only docs, texture, scripts). [O]

### What the plan promised but did not land (clustering assessment section 7 vs main) [O unless noted]
- **B1** make go-choir-btrfs-headroom balance metadata: not built. nix/node-b.nix:2100 still runs only `balance start -dusage=20,limit=20`; today's journal shows "relocate 9 of 433 chunks" (a data balance).
- **B2** forecast alarm: not built. go-choir-host-disk-alarm still prints only "host disk ok: free 83 GiB, btrfs unallocated 33 GiB, metadata 36%".
- **B3** dead-computer reaper (F2/F3), **B5** capture-bodies retention (F7), **B4/F5** root thinning, **B6** host gate for compact copy, **B7** tape tiering, **B8** receipts folding, **B9** hibernate idle test computers: none landed.
- **taperepack** has no timer (systemctl list-timers shows none). Tape sweep is on-demand.
- Dead config `VMCTL_STORE_COMPACT_*` (B10) not checked today.

### The "storage sweep dry run"
I found no work item with that exact name. The nearest items, confirmed or corrected:
- The **destroyed-computer tape sweep** dry run was run (and lost to deploy restarts twice, then completed) on 10-31, and real deletion followed. So it was not carried over unrun. But it is an on-demand tool, and dead state has re-accumulated: 93 tape-seg dirs / 6.7 GiB and 203 file-cas-chunks dirs / 28.2 GiB belong to computers absent from vm-state/ownerships.json (my match of the 9 `computer-...` ids in that file against directory names; see section 2) [O]. A second dry run is due.
- The lean ledger's **tape-seg-compact "d@e dry run and apply with --compsize"** (lanes.md line 25, ledger 10-31 18:15Z "Not run on Node B"): no record that I found of it running as designed. The 11-01 pack ran instead. [I; I did not check every ledger line]
- The clustering assessment's A2 (dead computers' chunks, dry-run join first) is the sweep that is owed now.

## 2. Measurements today (Node B, 2026-11-12 ~20:10Z, all [O])

### Volume (`df`, `btrfs filesystem usage /`, `btrfs filesystem df /`)
- /dev/md127 476 GiB, used 380 GiB, **avail 85 GiB** (82%). Free (estimated) 84.25, min 67.6.
- Data, single: 423.0 GiB allocated, 372.0 used (88%). **Metadata DUP: 9.5 GiB allocated, 3.44 used (36%)**. Unallocated 33.3 GiB. Compression zstd:3, discard=async (per assessment).
- 7 subvolume list lines (`btrfs subvolume list /`), no snapshots of note.
- Swap file 16 G, 183 MiB used.

### /var/lib/go-choir (du, GiB; reflinks/hardlinks mean sums exceed df)
| Holder | GiB | Note |
|---|---|---|
| total (secrets excluded) | 354 | |
| platform-artifacts | 209 | |
| - sha256/file-cas-chunks | 84.9 | paper 53.0 (54,321 MiB), owner (0333) 2.3 |
| - tape-seg | 86.7 | paper 69.4 (71,051 MiB by du), 93 unregistered dirs 6.7 |
| - capture-bodies | 21.6 | no collector |
| - file-cas-roots | 6.4 | |
| - platform-update | 6.8 | |
| - computer-inputs / artifact-gc-log / tape-sweep-archive | 0.7 / 0.5 / 0.3 | |
| vm-state | 128.5 | hardlinked into .jail; real images sum 82.5 (below) |
| guest-releases / tools / bases | 8.6 / 2.4 / 1.2 | 6 releases |
| taperepack-work | 1.8 | 11-01 manifest, audit evidence |
| services 1.2, offhost-backup 0.3, gateway 0.3, files 0.3 | | |
| /nix/store | 25.9 | stable floor; GC works |
| /var/lib/postgresql | 32.0 | control 27 GB, graph 5.0 GB |
| /root | 5.2 | go modcache 3.4, cutover 0.5, npm 0.4 |
| /var/log/journal | 0.26 | |

Data images (growth meter, last reading 20:01Z): paper 49.2 GiB; vm-70070cf7 26.7; vm-8bee33d5 4.6; vm-48bc0981 3.1; vm-9b5b54a2 0.5; vm-90c9b5bb 0.5. The four computers created 11-03 and the candidate dirs from last week are gone from vm-state and from the registry (retention worked). Sum of images 84.5 GiB.

### PostgreSQL (read-only transaction, no row content)
- control 27 GB, graph 5038 MB (now 4.9 GiB by meter).
- control top table: `computer_event_append_receipts` **27 GB total** (heap 17 GB + indexes 9.9 GB), 12,333,262 rows. Next: computer_lifecycle_receipts 63 MB; everything else under 9 MB.
- graph: og_objects 2.0 GB, proj_capture_terms 1.2 GB, og_edges 1.1 GB, proj_capture_index 305 MB, og_lookup_keys 285 MB.
- Per computer receipts: paper 10,935,946 rows (89% of the table), max sequence 10,935,946; 404,699 in the last 24 h; 3,622,639 in 7 days. Of the paper's last-24 h receipts 402,292 are `projection_batch_recorded` (99.4%), 2,363 artifact_produced, 80 file_root_committed. About 1.9 KB average row (pg_column_size of 20,000 rows).
- Paper tape sequence 10.9M events matches the owner's figure. State.db and WAL are inside the guest image; **not visible from host metadata** [?]. The paper image (49.2 GiB allocated, 128 GiB apparent) is the only host-side proxy. The guest log's `wal 737817872 bytes` is not something I can verify from the host.

### Timers (journalctl -u, last 3 days)
- go-choir-btrfs-headroom (daily 00:46Z): data balance only. 11-11 run: unallocated 25 GiB, relocate 9 of 433 chunks, 34 GiB after (33.2 GB written, 1m48s). 11-12 run: 52 GiB unallocated, no balance needed. This unit did not restore the metadata; the 11-11 hand balance did.
- go-choir-disk-gc (daily ~00:20Z): 11-11 run freed 9.6 GiB of nix store (350 paths) and 165 MiB journals, df Avail 72G; 11-12 run: "0 store paths deleted", Avail 124G. No `vms_reclaimed`/state release lines this window.
- go-choir-guest-release-retention (every 30 min): working, 6 releases, deleted 2, dedupe 427 MB.
- go-choir-host-disk-alarm (15 min): "host disk ok: free 82 to 90 GiB, unallocated 33 GiB, metadata 36%". Warns only below 60 GiB (the same number as the deploy refusal, per assessment).
- go-choir-checkpointd (1 min): idle settling, no errors.
- go-choir-growth-meter (hourly): items=387, errors=0. It does not meter file-cas-chunks, capture-bodies or platform-update (no such keys in items) [O].
- Backup (journal): nightly restic added 40.9 GB (11-11) and 34.3 GB (11-12). Node A repo size not measured.

## 3. Headroom: now and trend (growth meter, 11-01 to 11-12; daily last reading, GiB)
| day | avail last / min | nix | tape-seg | PG control | PG graph | paper image | all images |
|---|---|---|---|---|---|---|---|
| 11-01 | 193.6 / 193.6 | 35.1 | 48.9 | 18.2 | 2.5 | 23.7 | 28.1 |
| 11-04 | 168.8 / 143.9 | 25.4 | 57.4 | 20.1 | 3.0 | 28.6 | 56.7 |
| 11-07 | 130.6 / 121.0 | 25.8 | 69.7 | 22.8 | 3.4 | 34.9 | 68.3 |
| 11-09 | 111.6 / 66.8 | 26.0 | 76.8 | 24.5 | 3.9 | 38.0 | 81.4 |
| 11-10 | 67.2 / 67.2 | 26.7 | 80.7 | 25.4 | 4.2 | 42.8 | 109.7 |
| 11-11 | 123.9 / 48.6 | 25.9 | 84.4 | 26.3 | 4.6 | 46.2 | 81.4 |
| 11-12 (20Z) | 85.6 / 84.3 | 25.9 | 86.7 | 27.0 | 4.9 | 49.2 | 84.5 |

Free space: 193.6 (11-01) to 48.6 minimum (11-11 morning), then metadata balance plus image fall brought it to 124 by 11-11 20Z. **From 11-12 08Z (133 GiB) to 16Z (84 GiB) free fell 49 GiB in 8 hours** [O hourly ledger]. It is 85 GiB now, 25 GiB above the 60 GiB deploy floor.

### The new fast drain: the paper's file-cas chunks [O find mtimes + gc log]
- Chunks in computer-1355b83b... on disk grew from 21.5 GiB (assessment, 11-11) to **53.0 GiB**. Files with mtime today: 52.8 GiB in 87,873 files.
- By hour (UTC, apparent bytes): 08Z 2.9, 09Z 6.0, 10Z 6.1, 11Z 6.2, 12Z 6.1, 13Z 6.1, 14Z 6.2, 15Z 2.7, (16Z and 17Z none; the paper froze at 15:28Z per recent problems commits), 18Z 3.5, 19Z 6.9 GiB.
- The collector deleted 18.2 GiB of the paper's chunks today (27,979 files) in the same log. Net about +34 GiB apparent. Collector keeps a chunk for 24 h at least and roots 7 days (assessment, code).
- A steady 6.1 GiB per hour, about 9,400 files an hour, looks like a loop, not user work [I]. The cause is not determined [?]. The 11-11 assessment saw an 18.7 GiB burst and guessed a post-compaction snapshot (p=0.6). That is not enough to explain 6 GiB an hour for 12 hours. Needs the owner's session or the guest log to settle; I did not call the guest.

### Growth rates (GiB/day) and days of headroom
| Category | Rate | Basis |
|---|---|---|
| tape-seg, all | 3.3 (last 5.8 days 2.9) | [O] meter 11-02 to 11-12 |
| paper tape alone | 2.4 | [O] 43.8 to 69.4 over 10.8 d |
| PG control (receipts) | 0.75 | [O] 18.9 to 27.0 |
| PG graph | 0.19 | [O] |
| Data images, paper only | 2.2 (last 4.8 d); sawtooth +20 to +35 transients | [O] |
| nix store | 0 net (floor 25.9; sawtooth to 39) | [O] |
| file-cas-roots | 0 (bounded 6 to 8) | [O] |
| capture-bodies | 0.3 rising to 2.0 per assessment | [O by earlier du; today's rate not measured] |
| file-cas chunks (paper) | about +34 net today; the previous days were near 0 net | [O today only] |
| Whole disk, 11-01 to 11-10 | 14 GiB/day free-space decline (193.6 to 67.2 over 9 days) | [O] |

Days left above the 60 GiB floor (25 GiB of room), by scenario [I]:
- Chunk loop continues (about 35 to 40 GiB/day observed 11-12): **under one day**.
- Whole-disk 14 GiB/day: **1.8 days** to the floor; 6 days to empty.
- Steady growers only, no chunk loop (tape 3.3 + PG 0.9 + images 2.2 + capture ~1 = about 7.5): **3 days** to the floor, 11 to empty.
Transients of +33 GiB (paper compact copy on 11-09) can break the floor at any time.

## 4. Mission numbers then and now
| Measure | Mission (date) | Now |
|---|---|---|
| free on / | 90 GB (10-30 start, lean doc); 227 GiB (11-01, assessment); 51 GiB (11-11) | 85 GiB (124 at 11-11 20Z) |
| metadata allocated / used | 25.1 / 3.3 GiB (11-11) | 9.5 / 3.44 GiB |
| data allocated / used | 405.0 / 372.6 | 423.0 / 372.0 |
| tape-seg | 27 GB (10-30), 82.8 GiB (11-11) | 86.7 GiB |
| paper tape | 66.0 (11-11) | 69.4 |
| file-cas chunks (total) | 82 GB (10-30); 53.6 GiB (11-11) | 84.9 GiB |
| of which paper | 21.5 | 53.0 |
| unregistered chunk dirs | 199 dirs / 26.9 GiB | 203 dirs / 28.2 GiB (registry now 9 ids) |
| unregistered tape dirs | 89 / 1.3 GiB | 93 / 6.7 GiB |
| capture-bodies | 19.5 | 21.6 |
| PG control / receipts rows | 25.9 GiB / 11.7M | 27 GB / 12.33M |
| paper image | 44.7 | 49.2 |
| all VM images | 113.9 (12 dirs) | 84.5 (6 images) |
| nix store | 35 to 39 sawtooth | 25.9 |
| guest releases | 12.75 listed, 5 kept | 8.6, 6 kept |
| paper state.db | 31 GB to 20.4 GB (10-31) | not visible from host [?] |
| journals | 181 MiB | 268 MB (du), vacuumed daily |

Reading: the mission's biggest gain held (loose tape is gone, nix and releases stay bounded, dead VM images are gone). Metadata is fixed by hand but the unit that should keep it fixed is not changed. The tape, receipts and the paper's image still grow without sink, and file-cas chunks for the paper moved from a sideshow to the fastest drain.

## 5. Improvements, ranked

Values: S = measured, E = estimate. Class and rollback per CLAUDE.md. "Owner" = production deletion is the owner's call.

1. **Find and stop the paper's 6 GiB/hour chunk writes.** Saves: up to about 35 GiB/day of apparent growth (S: today's mtimes, gc log). Mechanism: read the paper's snapshot/artifact trace and collector log to learn what writes about 9,400 chunks an hour (likely a repeating snapshot or file-root chunking [I]), then fix the writer so unchanged content is not re-chunked; do not just shorten the collector window. Class: orange to red (file CAS, depends on the writer). Rollback: revert. Owner: no for investigation (read the traces first, CLAUDE.md rule 1); a fix touching the paper is the owner's go. Not built.
2. **Wire metadata balance into go-choir-btrfs-headroom (assessment B1).** Saves: prevents re-stranding the 25 GiB (S: it stranded 38 GiB, the hand balance recovered about 16 GiB of allocation, 25.1 to 9.5, while `df` free rose 55.4 to 79.8). Mechanism: add `-musage=40,limit=10` when metadata size exceeds 3x used. Class: orange. Rollback: revert. Owner: no. Built: no; one hour of work. Also set the ledger straight: C48 was false, so the "about 38 GiB" figure was an overestimate.
3. **Dead-computer reaper plus a fresh dry run (B3 / A2).** Saves 28.2 GiB chunks + 6.7 GiB tape = about 35 GiB now (S, from a match on the 9 ids in ownerships.json; the match is mine and unverified against the control DB), then about 0.17 GiB per dead test computer. Mechanism: extend the existing destroyed-tape sweep (internal/platform/tape_purge.go) and the artifact collector to chunks, roots, tape of computers with no ownership, no account, and no event for 7 days; dry-run report first. Class: red mechanism; the one-time delete is black. Rollback: none for deleted data, so the dry-run report is the gate. Owner: yes for the first delete. Partly built (tape sweep exists and ran on 10-31); the chunk/root side is not.
4. **Forecast alarm (B2).** Saves nothing; ends surprises. Today it fires at the same 60 GiB that refuses the deploy, and the fastest drain of the week (49 GiB in 8 h) is only visible in the meter. Add days-to-floor from the daily-minimum slope plus top-five 24 h growers, and meter file-cas-chunks, capture-bodies, platform-update so growth shows. Class: yellow. Rollback: revert. Owner: no.
5. **Tape tiering to the archive repository (B7).** Saves about 28 GiB of cold segments now (E, from the 14-day hot window; 83 GiB total) and stops local tape growth of 3.3 GiB/day. Mechanism: sealed segments past the last verified snapshot plus margin are written to a never-pruned restic archive repo, verified, then dropped locally; needs a lazy read path (does not exist). Class: red, design first. Rollback: restore from archive. Owner: yes (tape is canonical, never delete). Not built; backup already holds a copy of every sealed segment (assessment).
6. **Receipts folding (B8).** Saves: the table is 27 GB, 89% paper, 99.4% projection batches, +0.75 GiB/day (S). Folding receipts older than the last checkpoint into one row per segment could free most of 17 GB heap plus 9.9 GB indexes (E, unmeasured). Compaction already active on three computers shrinks new bodies (469 B vs 1460 B) but not row count or index size, since rows is the bulk. Class: red (chain of acceptance), two seats. Rollback: existing expand setting. Owner: design decision. Not built.
7. **capture-bodies retention (B5/F7).** 21.6 GiB, no collector, accelerating (E 2 GiB/day by the assessment, not re-measured). Class: red (evidence). No rollback. Owner: yes, a number N of days. Not built.
8. **Swap 16 GiB to 4 GiB (A3).** +12 GiB; swap uses 183 MiB (S). Class: orange. Rollback: recreate. Owner: yes (memory safety). Not run.
9. **Hygiene and small releases.** /root modcache etc. 5.2 GiB (go clean -modcache; yellow, owner glance); taperepack-work 1.8 GiB manifest after archiving (red, owner); dead `VMCTL_STORE_COMPACT_*` config (green); put the bound table (assessment section 8) into docs/ACTIVE.md (green).
10. **Gate the in-guest compact copy on host free space (B6).** The compact copy has peaked at +19 to +33 GiB on the host and checks only guest space. At 25 GiB above the floor, one copy can cross it. Class: red. Rollback: revert. Not built.
11. **Bigger disk or second device for platform-artifacts (B11).** The tape and receipts alone grow about 4 GiB/day (S). If 5 and 6 are not built, this is the fallback. Owner's decision, cost unknown [?].

What to leave alone: sealed tape segments of registered computers, b@c's image, the paper's data.img and latest roots (see assessment 7c).

## 6. Caveats
- Registered-vs-dead sizes use my regexp match of 9 `computer-<32 hex>` ids from vm-state/ownerships.json against directory names; the owner computer and others are included when present in that file. Not cross-checked with the control database or retention-plan (needs the internal token).
- du figures are allocated bytes with hardlinks/reflinks; vm-state is double-counted against .jail.
- The paper's state.db size, the WAL size and the cause of the chunk loop need the guest or the owner's session; not asked.
- "Days left" are projections from the observed rates, not forecasts of transient events.
