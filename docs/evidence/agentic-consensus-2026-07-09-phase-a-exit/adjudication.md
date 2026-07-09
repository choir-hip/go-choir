# Phase A Exit Gate Adjudication

## Round 1 panel (2026-07-09)

- **Panel:** codex (CLI error), devin (no output), cursor, opencode, omp-gpt55, omp-gemini35, omp-glm52
- **Raw outputs:** `docs/evidence/agentic-consensus-2026-07-09-phase-a-exit/`
- **Verdict:** `conditional` (clear majority)

### Category-(a) findings and resolutions

| # | Finding | Category | Resolution |
|---|---|---|---|
| A1 | Missing I4 `DOLT_RESET` / destructive-rollback detector in `docs/heresy-detectors.md` | Phase A exit defect | Added `I4` row with pattern `CALL DOLT_RESET` and per-row `exclude:` glob support in `scripts/check-heresies.sh` to ignore `*_test.go` and `dolt_promotion_adapter.go`. |
| A2 | `Variant` metric `heresy_families_without_ci_detector: 31` contradicted the claimed W1 CI discovery ship | truth defect | Split the metric: `heresy_families_without_ci_detector: 0` (discovery wired for 12 aggregate families) and `heresy_families_without_ci_enforcement: 12` (fail-on-regression deferred per phase). |
| A3 | `Determined State Snapshot` still listed W3/H031 as open while evidence showed them closed | truth defect | Refreshed the snapshot; moved W3, H031, S1, and cross-substrate relabeling to `settled`; only `storage-fork (D-STORE)` remains in `open`. |
| A4 | `docs/current-architecture.md` and `README.md` still described D-PROMO as "under test" and W2 as "pending" | truth defect | Rolled both truth-doc claims forward to reflect D-PROMO settled at the pinned-connection/single-writer assumption level and W2 proven in staging. |
| A5 | `docs/choir-grip-checkpoint-2026-07-07.md` was not registered as a narrative doc | truth defect | Added it to `docs/README.md` as a narrative grip checkpoint and registered it in `docs/doc-authority-manifest.yaml` with `doc_role: narrative` and `authority: none`. |

### Category-(c) noise

- `docs/missions/substrate-hardening-v0.md` had a duplicate closing code fence (fixed in `6e6f3753`).
- Heresy detector counts include manifest/docs self-hits; discovery-mode counts are evidence, not production violations.

## Round 2 panel (delta, 2026-07-09)

- **Panel:** cursor, opencode, omp-gpt55, omp-gemini35, omp-glm52
- **Raw outputs:** `docs/evidence/agentic-consensus-2026-07-09-phase-a-exit-delta/`
- **Verdicts:** `conditional` (cursor), `clear` (gpt55, gemini35, glm52); opencode produced logs without a final verdict.

### Category-(a) findings and resolutions

| # | Finding | Category | Resolution |
|---|---|---|---|
| B1 | I3 `bounded-request-path` invariant still listed `status: settled (definition) / violated (implementation)` with observables for the pre-fix 180s state | truth defect | Updated I3 to `settled (definition and implementation)` with current observables (`DefaultClientTimeout` / `DefaultVmctlTimeout` 60s, `http.Server` Read/Write timeouts, `sandboxResolveRetryWindow` reconciled, staging 504 within 60s). |
| B2 | Evidence ledger still said D-PROMO was "strengthened to testing" and the pinned-connection test was "not yet independently reproduced" | truth defect | Updated evidence-ledger entries to reflect D-PROMO settled by `go test ./internal/computerversion -run TestDoltEmbeddedBranchIsolationPinnedConnection -count=10` (10/10 passes). |
| B3 | No committed Phase Gate adjudication table | process defect | This file. |

## Final disposition

All round 1 and round 2 category-(a) findings are resolved. Phase A exit is cleared subject to a follow-up delta panel confirming the above resolutions.
