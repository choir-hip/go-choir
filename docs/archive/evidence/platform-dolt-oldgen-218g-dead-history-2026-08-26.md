# Platform Dolt 218G Commit-History Accumulation (per-mutation DOLT_COMMIT; no GC collects reachable history)

- **Date:** 2026-08-26
- **Class:** platform problem (storage substrate), problem-documentation-first receipt
- **Status:** documented before fix; cleanup executed same day after agentic-consensus adjudication (12 models)
- **Mutation class of the fix:** red (canonical platform store maintenance)

## Problem

`/var/lib/go-choir/platform-dolt/platform` (the `go-choir-platform-dolt`
`dolt sql-server`, Dolt 2.1.9, port 13306) held **225G**: 218G in
`.dolt/noms/oldgen` across 113 `.darc` archives (largest single archive 169G),
plus 7G of leaked `buffered_file_byte_sink_*` temps in `.dolt/tmp` (Aug 22).
Live logical HEAD was ~4.5G. Reclaimed: **224G** (host disk 367G used → 143G
used; free 105G → 328G).

## Root Cause (consensus-corrected)

**Initial hypothesis (wrong):** shallow auto-GC never collects oldgen; a
`dolt gc --full` would reclaim it. Eleven of twelve consensus models approved
this premise.

**Correct root cause (panel dissent, verified against repo and host):**
`internal/platform/store.go:488` and `internal/cycle/storage.go:56` call
`CALL DOLT_COMMIT('-Am', ...)` on essentially every mutation. The store
accumulated **6,903,253 commits** since 2026-05-16. Dolt GC — shallow *or*
`--full` — preserves everything reachable from any commit on any branch, so
the entire commit graph (months of superseded row versions of 3.3M
`og_objects` longblob bodies and event receipts) is unreachable-by-no-GC.
`dolt gc --full` would have walked the 218G, rewritten all reachable history,
reclaimed nothing, and risked filling the 116G-free disk mid-run. The dissent's
abort-gate measurement (`SELECT COUNT(*) FROM dolt_log` → 6.9M) falsified the
approval premise before execution.

Auto-GC was running correctly throughout (78 successful shallow GCs since
2026-08-01, every ~45 min). Dolt 2.x archiving (`.darc`) packs chunks but does
not drop history.

## Executed Cleanup (history squash, per amended consensus procedure)

1. Stopped writers: `go-choir-sourcecycled`, `go-choir-corpusd` (vmctl, auth,
   gateway, maild, proxy left up).
2. First dump attempt via MySQL protocol (`SELECT *` over 3.3M longblob rows)
   **OOM-killed the dolt server** (unit memory peak 27.3G; kernel OOM-kill at
   04:52:37Z; auto-restarted clean — read-only kill, ACID intact). Lesson:
   never full-scan this store through the SQL server; use offline `dolt dump`.
3. Stopped `go-choir-platform-dolt`; offline `dolt dump -fn platform-dump.sql`
   → **11.3G** logical dump at `/var/lib/go-choir/dump-20260826/` (54/54
   tables, "Successfully exported data"). True live HEAD ≈ 11G dump / 4.5G
   chunk storage — the earlier "45M live" figure was only the newgen journal.
4. `mv platform platform.bak-2026-08-26`; fresh `dolt init`; imported the dump
   (`dolt sql < dump`, ~40 min); counts verified against pre-cleanup live
   server counts: og_objects 3,316,888 (≥3,316,558), og_edges 1,661,660,
   fetches 940,973, items 831,219, ingestion_events 1,591,811,
   cycle_events 66,034, **computer_event_append_receipts 133,319 (exact)**,
   computer_event_heads 1.
5. `CALL DOLT_COMMIT('-Am', 'history squash 2026-08-26...')` → dolt_log
   **6,903,253 → 2 commits**. Offline `dolt gc` compacted 29G journal →
   **4.5G** store.
6. Restarted platform-dolt → corpusd → sourcecycled. Smoke: corpusd
   `/health` `{"status":"ok","store":"ok"}` at commit c3314c59; public
   `https://choir.news/health` ok; live-server row count matches.
7. Deleted `platform.bak-2026-08-26` → **224G reclaimed**
   (476G disk: 367G used/105G free → 143G used/328G free). The 11.3G logical
   dump is retained at `/var/lib/go-choir/dump-20260826/` until a later deploy
   cycle.

## Recurrence Control

- Repo-tracked `go-choir-platform-dolt-history-audit.service/.timer`
  (nix/node-b.nix): daily audit; fails loudly when `dolt_log` exceeds 1M
  commits or oldgen exceeds 50 GiB, pointing at the squash runbook (this
  receipt). Thresholds tunable via `GO_CHOIR_DOLT_MAX_COMMITS` /
  `GO_CHOIR_DOLT_MAX_OLDGEN_KIB`.
- Architectural fix (overhauls-scale, owner direction 2026-08-26): stop
  committing every mutation; post-overhauls the platform moves to regular
  snapshotting instead of unbounded per-event commit history. "All that event
  buildup and garbage for just one user is a fatal error that can't be
  tolerated."

## Notes

- The sealed computer `0333528` guest event tape lives in guest persistent
  disks, not this store — untouched. `computer_event_append_receipts`
  (133,319 rows) and `computer_event_heads` are preserved exactly.
- `candidate-fleet-d03dacaa` (22G, live `universal-wire-platform` computer) is
  a guest VM state dir — untouched.
- Dolt 2.0/2.1's improved GC and `.darc` archiving reduce *storage per chunk*
  (preflight measured 88% archive reduction) but do not and cannot collect
  reachable commit history. Version upgrade was never the fix for this class.
