# Phase A Exit Gate Adjudication

## Round 1 panel (2026-07-09)

- **Panel:** codex (CLI failed), cursor (`conditional`), devin (`conditional`), opencode (`conditional`), omp-gpt55 (`conditional`), omp-gemini35 (`conditional`), omp-glm52 (`conditional`).
- **Raw outputs:** `docs/evidence/agentic-consensus-2026-07-09-phase-a-exit/`
- **Verdict:** `conditional` (unanimous among returning agents; codex failed)

### Category-(a) findings and resolutions

| # | Finding | Category | Resolution |
|---|---|---|---|
| A1 | Missing I4 `DOLT_RESET` / destructive-rollback detector in `docs/heresy-detectors.md` | Phase A exit defect | Added `I4` row with pattern `CALL DOLT_RESET` and per-row `exclude:` glob support in `scripts/check-heresies.sh` to ignore `*_test.go` and `dolt_promotion_adapter.go`. |
| A2 | `Variant` metric `heresy_families_without_ci_detector: 31` contradicted the claimed W1 CI discovery ship | truth defect | Split the metric: `heresy_families_without_ci_detector: 0` (discovery wired for 12 aggregate families) and `heresy_families_without_ci_enforcement: 12` (fail-on-regression deferred per phase). |
| A3 | `docs/heresy-detectors.md` header and deferred-enforcement prose still described the detector manifest as "not yet a CI-enforced check" even though the discovery script and CI job were already wired | truth defect | Updated the header and `Deferred Enforcement` section to show the discovery CI check is wired and fail-on-regression/allowlists are deferred per phase. |
| A4 | `Determined State Snapshot` still listed W3/H031 as open while evidence showed them closed | truth defect | Refreshed the snapshot; moved W3, H031, S1, and cross-substrate relabeling to `settled`; only `storage-fork (D-STORE)` remains in `open`. |
| A5 | `docs/current-architecture.md` and `README.md` still described D-PROMO as "under test" and W2 as "pending" | truth defect | Rolled both truth-doc claims forward to reflect D-PROMO settled at the pinned-connection/single-writer assumption level and W2 proven in staging. |
| A6 | `docs/choir-grip-checkpoint-2026-07-07.md` was not registered as a narrative doc | truth defect | Added it to `docs/README.md` as a narrative grip checkpoint and registered it in `docs/doc-authority-manifest.yaml` with `doc_role: narrative` and `authority: none`. |
| A7 | W3 evidence / Run Checkpoint used a stale `67fff296` as the "current deployed SHA" and the timeout evidence doc used the same stale identity | truth defect | Updated the W3 evidence ledger to record the deployed SHA progression (`67fff296` first 60s observation, `1ed41f2b` later deploy, and `14f56211` at the time of the round-3 review). Time-scoped the identity claim so it is not asserted as an evergreen "current" value. |

### Category-(c) noise

- `docs/missions/substrate-hardening-v0.md` had a duplicate closing code fence (fixed in `6e6f3753`).
- Heresy detector counts include manifest/docs/evidence self-hits; discovery-mode counts are evidence, not production violations.
- The `I4` detector reports non-zero hits because the literal `CALL DOLT_RESET` appears in the manifest, in the panel output evidence files, and in the adapter source; the row's `exclude:` list removes `*_test.go` and `dolt_promotion_adapter.go`, and its `target` is `0 production (non-test, non-adapter) hits`.

## Round 2 panel (delta, 2026-07-09)

- **Panel:** cursor (`conditional`), opencode (logs only, no final verdict), omp-gpt55 (`clear`), omp-gemini35 (`clear`), omp-glm52 (`clear`).
- **Raw outputs:** `docs/evidence/agentic-consensus-2026-07-09-phase-a-exit-delta/`
- **Verdicts:** `conditional` (cursor); `clear` (majority of returning agents)

### Category-(a) findings and resolutions

| # | Finding | Category | Resolution |
|---|---|---|---|
| B1 | I3 `bounded-request-path` invariant still listed `status: settled (definition) / violated (implementation)` with observables for the pre-fix 180s state | truth defect | Updated I3 to `settled (definition and implementation)` with current observables (`DefaultClientTimeout` / `DefaultVmctlTimeout` 60s, `http.Server` Read/Write timeouts, `sandboxResolveRetryWindow` reconciled, staging 504 within 60s). |
| B2 | Evidence ledger still said D-PROMO was "strengthened to testing" and the pinned-connection test was "not yet independently reproduced" | truth defect | Updated evidence-ledger entries to reflect D-PROMO settled by `go test ./internal/computerversion -run TestDoltEmbeddedBranchIsolationPinnedConnection -count=10` (10/10 passes). |
| B3 | No committed Phase Gate adjudication table | process defect | This file. |

## Round 3 panel (delta-2, 2026-07-09)

- **Panel:** cursor (`clear`), opencode (`clear`), omp-gpt55 (`conditional`), omp-gemini35 (`clear`), omp-glm52 (`clear`).
- **Raw outputs:** `docs/evidence/agentic-consensus-2026-07-09-phase-a-exit-delta-2/`
- **Verdicts:** `clear` (cursor, opencode, gemini35, glm52); `conditional` (gpt55)

### Category-(a) findings and resolutions

| # | Finding | Category | Resolution |
|---|---|---|---|
| C1 | Adjudication table under-counted round 1 findings (listed devin as "no output", omitted A3/A7, and did not record opencode round 1 as `conditional`) | process defect | This updated adjudication table: now records codex failed, devin conditional, opencode round 1 `conditional`, all panel verdicts, and adds the A3 heresy-detector prose and A7 time-scoped deployed SHA findings. |
| C2 | W3 evidence ledger still used a non-time-scoped "current deployed SHA" claim (`1ed41f2b`) that was superseded by live staging at `14f56211` at the time of the round-3 review | truth defect | Replaced the evergreen "current deployed SHA" wording with a time-scoped observation: `67fff296` first 60s observation, `1ed41f2b` at 2026-07-09T05:12:21Z, `14f56211` at 2026-07-09T05:42:19Z; all deploys show the same 60s timeout bound. |

## Round 4 panel (delta-3, 2026-07-09)

- **Panel:** cursor (`conditional`), opencode (`conditional`), omp-gpt55 (`conditional`), omp-gemini35 (`clear`), omp-glm52 (`clear`).
- **Raw outputs:** `docs/evidence/agentic-consensus-2026-07-09-phase-a-exit-delta-3/`
- **Verdicts:** `clear` (gemini35, glm52); `conditional` (cursor, opencode, gpt55)

### Category-(a) findings and resolutions

| # | Finding | Category | Resolution |
|---|---|---|---|
| D1 | Adjudication table still inaccurately recorded opencode round 1 as `logs only` and round 3 as `logs only`, and under-counted round 3 verdicts as `3 clear / 1 conditional` | process defect | Updated adjudication: round 1 opencode is `conditional`; round 3 opencode is `clear`; round 3 verdicts are `4 clear / 1 conditional`. |
| D2 | `Determined State Snapshot` still asserted a present-tense `the deployed SHA is 1ed41f2b` without a timestamp or scroll sequence | truth defect | Rewrote the snapshot W3 claim to use the time-scrolled sequence `67fff296` (first 60s timeout), `1ed41f2b` (2026-07-09T05:12:21Z), and `14f56211` (2026-07-09T05:42:19Z), matching the evidence ledger. |

## Final disposition

All round 1–4 category-(a) findings are resolved. Phase A exit is cleared subject to a follow-up delta panel confirming the D1/D2 resolutions.
