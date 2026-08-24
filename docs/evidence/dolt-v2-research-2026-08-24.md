# Dolt 2.0 upgrade research — 2026-08-24

Authority record promoted from the librarian research agent
(`agent://DoltTwoResearch`, 18 primary sources, 2026-08-24). This document is
the in-tree citation for the pre-flight definition's Dolt upgrade decisions.
Full raw output: agent://DoltTwoResearch (session artifact).

## Version map (as of 2026-08-24)

| Component | Current (chosen) | Target | Notes |
|---|---|---|---|
| Embedded driver | `github.com/dolthub/driver v1.84.1` | `github.com/dolthub/driver/v2 v2.2.0` (2026-07-15) | Import path change to `/v2` (v2.1+); API/DSN unchanged |
| Dolt Go | `v0.40.5-0.20260326074512-005921bdd8ca` | `v0.40.5-0.20260715172757-a6690826d767` | Same module path; newer pseudo-version (no dolt/go v2 tag exists) |
| go-mysql-server | `v0.20.1-0.20260325173633-83a7fba2790f` | `v0.20.1-0.20260713210757-6d01d00bbbf3` | Moves with driver |
| Vitess | `v0.0.0-20260309181228-a99af9c518ab` | `v0.0.0-20260624214226-81d034e0fde8` | Moves with driver |
| go-icu-regex | `v0.0.0-20250916051405-78a38d478790` | `v0.0.0-20260610153742-72563bc7ca83` | cgo + ICU4C still required |
| gozstd | `v0.0.0-20240423170813-23a2903bca63` | unchanged | C/zstd linkage |
| Go | 1.25.6 | **>= 1.26.2** | Hard requirement (go.mod directive of driver/v2 + dolt/go) |
| CLI (reference) | — | v2.0.0 GA 2026-05-07; latest v2.3.1 2026-08-19 | CLI milestone; embedded feed stops at v2.2.0 |

- "Dolt 2.0" is a CLI/repository milestone (v1.88.1..v2.0.0 = 9 commits, mostly
  CI/toolchain + PR 11017 adaptive-encoding flip), not a separately versioned Go
  module.

## What 2.0 brings

- Archive storage (`.darc`, dictionary-zstd): 30-50% footprint reduction; async
  assembly; constant-time reads. Archive INDEX size is material (~350 MB growth
  on a 41 GB archive, ~1 GB total — PR #8078); mmap index option exists (PR
  #9579) but is NOT exposed by the embedded driver Config.
- Auto-GC "on by default" — SERVER scope only: the embedded driver does not
  install the `AutoGCController` (verified in driver v2.2 `openEngineWithRetry`
  + engine `sqlengine.go`: background GC only when `config.AutoGCController !=
  nil`). Manual `CALL DOLT_GC()` remains; without the session-aware controller
  it can invalidate the calling connection (close/reopen after).
- Adaptive encoding: inline small TEXT/BLOB/JSON/GEOMETRY (`TARGET_ROW_SIZE`),
  up to +40% scans for large types; applies to newly created tables/columns.
- Sysbench: +13% writes / +5% reads vs MySQL — SQL-server measure, not an
  embedded 7-9 GiB workspace on a 4 GiB Firecracker guest.

## Compatibility and the write-path verdict

- 1.x databases open under 2.x with NO migrate; **2.x-written databases are not
  readable by 1.x clients in all cases** (release note verbatim) — one-way.
- Go API unchanged: `Config`/`ParseDSN`/`NewConnector`/`Connect` and the DSN
  params (`commitname`, `commitemail`, `database`, `multistatements`,
  `clientfoundrows`) are identical in v2.2 source.
- **The per-event fsync ceiling is NOT fixed**: v2.3.1 `journal_writer.go`
  still calls `journal.Sync()` per committed root (`commitRootHashUnlocked`),
  same 64 MiB unsync threshold as v0.40. No official proof models our
  constrained-guest write path.
- Non-negotiable safety: use the late v2.x graph (the initial v2.0.0 predates
  PR #11058, a branch-control security bypass fix via session table cache).

## License / build

- Apache-2.0 for Dolt + driver (unchanged). go-icu-regex requires ICU4C + a C++
  toolchain + cgo (Nix guest build has `pkgs.icu` today; confirm under the new
  flake). gozstd link unchanged.

## Risks

1. One-way format after any 2.x write → snapshot-fence discipline; downgrade
   tests only on untouched snapshots.
2. Archive-index memory (~350 MB-1 GB) in the 4 GiB guest; not configurable via
   embedded Config — must be measured (PF-2) and, if unsafe, archives disabled
   or the verdict recorded.
3. Embedded path does NOT get server auto-GC — the guest's GC policy keeps
   relying on the explicit guard/env (PF-4 re-validates under v2 semantics).
4. Module-path trap: v2.0.0/v2.0.7 used the old path; v2.2 README has a stale
   import snippet — authority is v2.2 `go.mod` + example.
5. Known side discussions (#10849 idle CPU/RSS, #5997 import garbage) are
   server-side, not embedded v2 regressions; no embedded-specific 2.0 regression
   found.

## Verdict

Upgrade for correctness/current graph/storage/adaptive encoding/feature
maintenance — validated first (snapshot smoke, like-for-like 4 GiB-guest
measurement, archive/RSS/OOM check, one-way rehearsal). NOT the fix for the
3-6s per-event guest ceiling (per-root Sync unchanged; no embedded benchmark).
The ceiling's candidate fix remains the appender per-page batching (dissent D)
with per-event nonfsync (C) evaluated as the lower-risk alternative.

## Sources (primary)

dolthub/dolt releases/tag/v2.0.0; dolthub.com/blog/2026-05-11-dolt-2-dot-0/;
dolthub/driver go.mod@v2.2.0 + config.go/connector.go/parse_dsn.go/driver.go@v2.2.0;
dolthub/dolt go/store/nbs/journal_writer.go@v2.3.1 +
go/libraries/doltcore/engine/sqlengine.go@v2.3.1; dolthub.com/blog/2025-02-28-
announcing-automatic-gc-in-sql-server/; dolthub.com/blog/2026-05-20-adaptive-
encoding-in-dolt/; dolthub/dolt issues/9579, issues/8078, issues/10849,
issues/5997, PR 11058; dolthub/go-icu-regex README; proxy.golang.org
(yaegi v0.16.1.mod go 1.21).
