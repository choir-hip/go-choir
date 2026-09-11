# Testing/CI Speedup Work — Report 2026-09-11

**Scope:** local test throughput + CI sharding for `internal/store` (300+ embedded-Dolt tests).
**Commits:** `e22b99d4` (tmpfs + 8 shards, pre-existing HEAD) → `58d4e2dc` (store bootstrap batch) → `278263ab` (race workflow 4 shards).
**Status:** landed, CI green with measured speedup. Mutation class orange(ci/store).

---

## 1. The loop that wasted 30 minutes

Full `go test ./internal/store/ -count=1` was run six times locally with 300/600/900s timeouts. Every run was killed by Go's test-binary timeout while verbose tails showed steady progress (~5–10 more `TestOG*` tests per 5 minutes). Signature: throughput-bound single process, not a hang. The repo never runs this shape anywhere — `scripts/go-test-non-runtime-shards` exists precisely because the package is too heavy for one process. Repeating the run could not converge, with or without any code change (stashed baseline behaved identically).

## 2. What was changed

### 2.1 `58d4e2dc` — batch bootstrap column discovery (`internal/store/store.go`)

`Store.bootstrap()` ran ~40 sequential `ensureColumn()` calls, each a ~95ms `information_schema.columns` round-trip (~3.8s total). Replaced with one batched `ensureColumns()`: a single `SELECT table_name, column_name ... WHERE table_schema = DATABASE() AND table_name IN (...)` over the 12 tables, then `ALTER TABLE ADD COLUMN` only for missing columns, in declaration order (index dependency preserved). Same identifier validation, same scoping, same fail-closed ALTER wrapping. Follow-through in the same commit: deleted the now-unused singular helper, fixed the stale bootstrap comment, added a duplicate-entry guard restoring old-loop skip semantics.

Measured (local, darwin arm64): fresh `Open` 6.6s → 3.2s; reopen 4.4s → 1.2s; 3-test subset 26s → 12.7s.

### 2.2 `278263ab` — race workflow to 4 non-runtime shards (`.github/workflows/race.yml`)

The weekly `Race Detector` ran non-runtime tests with `TOTAL_SHARDS=3`, one below the shard script's `>=4` store-sub-sharding threshold — so all 348 store tests ran single-process with `-race` on one 30-min shard (~16 min pre-fix). Bumped matrix to `[0, 1, 2, 3]` and `TOTAL_SHARDS` to 4: store now splits across 2 test-name sub-shards per the script's design point. Cost: one extra weekly runner.

## 3. How it was proved (no local full-package run)

- **Local gate (run once):** `go vet ./internal/store/` clean; 5-test Open/migration/reopen subset ok in ~15–19s (incl. `TestOpenMigratesWorkerUpdatesBeforeDeliveryIndex`, `TestOpenMigratesRunsRequestedByRunIDBeforeIndex`, recovery-across-reopen tests).
- **Independent review:** 10-model convergent consensus panel (codex, claude/opus, cursor, opencode, omp-gpt56-sol/luna, omp-glm53-flash, omp-cursor-grok46, omp-muse-spark). Unanimous: no blocking bug; full-package loop is the wrong proof; land store-file-only orange commit; CI shards 0–5 are the proof. Raw outputs in gitignored `.agentic-consensus/testing-ci-finish-20260911/`.
- **CI proof:** run `34622877021` on `58d4e2dc` — green end-to-end (race matrices, all 8+8 shards, scale, vet+build, staging deploy). Store package-elapsed per sub-shard vs baseline run `34609638815` (`e22b99d4`, same race mode + topology):

| shard | baseline | candidate | Δ |
|-------|----------|-----------|---|
| 0 | 173.7s | 98.6s | −43% |
| 1 | 189.1s | 83.7s | −56% |
| 2 | 107.7s | 96.5s | −10% |
| 3 | 190.7s | 92.2s | −52% |
| 4 | 170.4s | 83.4s | −51% |
| 5 | 130.4s | 76.0s | −42% |
| **sum / max** | **962s / 191s** | **530s / 99s** | **−45% / −48%** |

- **Race-lane proof:** manually dispatched `Race Detector` run `34626445187` on `278263ab` to exercise the new 4-shard topology immediately instead of waiting for Monday's schedule.

## 4. What was deliberately not done

- No local full-package run after the fix (non-proof; CI owns exhaustive coverage).
- No serial 6-shard local sweep (CI runs all six on push within minutes).
- No new permanent test: existing migration tests assert consumer-visible schema outcomes; the batch helper's empty-input branch is unreachable from bootstrap.
- No `TMPDIR=/dev/shm` on darwin (Linux-only); no `-race` locally (CI lane governs).
- Case-folding of scanned identifiers and `rows.Close()` error context were reviewed and rejected as landing blockers (reopen path proves current casing; close-before-ALTER is the required pattern).

## 5. Rollback

- Store batch: `git revert 58d4e2dc` (restores per-column loop; additive columns stay, harmless).
- Race shards: `git revert 278263ab`.
- Staging: schema outcome identical pre/post; deploy health is a no-regression check, not a migration event.

## 6. Residuals

- **Deploy time** (from mission-2 report, untouched): NixOS closure build on node-b ~13 min. Binary cache or CI-built closure would cut it to <2 min. Next realism axis, not this work.
- **Weekly race lane** is now shard-covered; Monday 2026-09-14 schedule is the standing detector.
- Heresy delta: none. Protected surfaces touched: none (store bootstrap queries are orange, not red).
