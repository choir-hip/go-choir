# Ledger: News Live + PR Merge + Model Default + Doc Cleanup

**Mission:** `docs/mission-news-live-pr-merge-model-default-v0.md`
**Started:** 2026-06-29

---

## Pass 0 — Mission Start

**Conjecture:** Mission conjecture stated (7 conjectures, V=7).
**Move:** construct (mission document authored).
**Expected ΔV:** 0 (mission start).
**Actual ΔV:** 0.
**Receipt:** `docs/mission-news-live-pr-merge-model-default-v0.md` created.
**Edge:** none.

---

## Pass 1 — Post-Implementation Review + Mission Extension (2026-06-30)

**Position:** Orchestrator resumed on `beads/parallax-v2` (wrong branch);
reoriented to the mission. Found all three tracks implemented on separate
branches, partially landed on main.

**Findings:**
- C1 settled: PRs #19/#20/#21/#23/#26/#28 on main; CI green per commit
  messages. Model policy C3 settled: `22270323` on main (low default,
  super=high), tests pass locally.
- C4 settled (diagnosis): Universal Wire edition alias
  `universal-wire/Wire.texture` never bootstrapped in production;
  `autonomousPublishWireArticleToEdition` silently no-oped on
  `store.ErrNotFound`. Documented in
  `docs/problem-universal-wire-edition-alias-not-bootstrapped-v0.md`
  (problem-doc-first honored: diagnosis commit `ae53581e` before fix
  `b533b258`).
- C5 partial (fix written, not landed): `b533b258` adds
  `Runtime.ensureUniversalWireEdition` self-healing bootstrap + regression
  test `TestWireAutonomousPublishBootstrapsEditionWhenAliasMissing`. Local
  `go test ./internal/runtime -run Wire` green. Staging proof pending.
- C2 not landed: circular pair #22+#27 on `combined-trace-runtime`
  (`7ebfc59b`) + CI speedup (`fc94f981`).
- C6/C7 not landed: doc cleanup on `doc-cleanup-audit` (`e4700d79`).

**Branch hygiene hazard identified:**
`combined-trace-runtime` contains stale/duplicate commits:
- `530caa2c` wire fix lacks the regression test that canonical `b533b258`
  has (production code identical, test missing).
- `57504f38` doc cleanup bundles non-docs work; canonical docs-only commit
  is `e4700d79` on `doc-cleanup-audit`.
- `a782caab` model policy has same patch-id as `22270323` already on main
  (duplicate).
Landing plan uses the canonical commits, NOT the combined-branch versions.

**Mission extension (Track D):** Added Track D — graph-native `/api/v1/`
surface + `choir` CLI. Rationale: CLI-first ordering makes the news
pipeline headlessly testable and makes the GUI fall out (same API,
different consumer). Clarified that `nucleus-cli-v0` (the CLI) does NOT
depend on `nucleus-capsule-runtime-v0` (Nucleus-the-container-tech as
capsule substrate); the naming conflation is flagged for later cleanup.
API key auth prerequisite is already implemented
(`internal/auth/store.go`, `internal/auth/handlers.go`).

**Move:** construct (mission doc updated with Parallax State, Track D,
branch-state table, references).
**Expected ΔV:** -2 (C1, C3 already settled by main landing).
**Actual ΔV:** -2.
**Receipt:** `docs/mission-news-live-pr-merge-model-default-v0.md` updated.
**Edge:** beads system on `beads/parallax-v2` is a SEPARATE PR for codex
review; explicitly excluded from this mission's landing plan.
**Next:** land canonical Track B/C/circular-pair commits to main, then
build Track D, then landing loop + staging acceptance proof via CLI.
