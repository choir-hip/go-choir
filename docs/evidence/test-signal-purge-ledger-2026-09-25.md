# Test-Signal Purge — Deletion Ledger (mission 1.5)

Date: 2026-09-25. Definition: `docs/definitions/choir-test-signal-purge-2026-09-24.md`.
Metric (owner): safely minimize test quantity AND test wallclock duration —
remove tests whose failure would not reveal a real defect the E2E suite misses.

Method: 13 disjoint package-scope audits run as `omp` subprocesses
(gemini-3.8-flash / cursor-grok-4.6-high) against a shared rubric
(`.agentic-consensus/purge-template.md`). Per-scope ledgers:
`.agentic-consensus/purge-ledgers/*.md`.

## Totals (diff-verified)

- **729 test/benchmark functions deleted** (`git diff '*_test.go'` removed `func Test`/`func Benchmark`).
- **13 `*_test.go` files removed entirely** (every member test was low-signal):
  agentcore {capsule_go_eval_dispatch, choir_event_consumers, observability,
  podcast_unit, prompts_policy}, autoputer/handlers, computerversion/
  base_current_state_projection, modelcatalog/catalog, sourcecontract
  {evidence, reader_artifact, selector, source_kind}, store/media.
- **134 test files modified** (partial deletions + unused-import cleanup).
- ~21.8k test lines removed (278 added are helper rewrites for surviving tests).
- 4 `+func Test*` in the diff are repositioned same-name tests (moved), not additions.

## Per-scope counts (deleted / kept-borderline)

| scope | deleted | kept-borderline | files removed | failing found |
|---|---|---|---|---|
| agentcore-ae | 13 | 50 | 2 | 0 |
| agentcore-fr | 67 | 146 | 3 | 0 |
| agentcore-sz | 7 | 114 | 0 | 0 |
| capsule | 5 | 5 | 0 | 0 |
| computerversion | 12 | 87 | 1 | 0 |
| platform-maild | 0 | 152 | 0 | 0 |
| proxy | 59 | 134 | 0 | 0 |
| store | 74 | 276 | 1 | 0 |
| textureowner | 16 | 108 | 0 | 0 |
| vmctl-autoputer-yaegi | 106 | 15 | 1 | 0 |
| tail-cmd-1 | 177 | 543 | 1 | 0 |
| tail-cmd-2 | 104 | 18 | 4 | 0 |
| tail-cmd-3 | 30 | 18 | 0 | 1 |

## Deletion criteria applied (rubric)

Deleted: implementation pins (wiring/defaults/field-copies/mock-echoes),
incidental-behavior pins (log wording, internal ordering), tautologies /
not-throw / length-grew, duplicate-input coverage (kept one representative),
and tests of test-only scaffolding not reflecting production wiring.

Kept: concurrency fences (epoch/serial-per-actor/CAS), store invariants &
event-sourcing/projection correctness, poison/dead-letter & timeout handling,
due-index/not_before semantics, error-type contracts, acceptance/verifier gates,
fencing/signature verification — the contracts E2E cannot reach. Borderline
keeps are named in each ledger.

## Deviations / notes

- `platform-maild` deleted 0 (152 kept-borderline): the auditor judged that
  package's tests contract-bearing; conservative per rubric's "when in doubt,
  keep". Re-audit candidate if more reduction wanted.
- Pre-existing failure flagged (not purge-caused, not deleted):
  `internal/desktopstate/handler_test.go: TestCloneStatePersistsOwnerScopedDesktopCopy`
  is `//go:build comprehensive`-gated and calls `h.CloneState` which no longer
  exists — stale build-tagged test; outside the deletion rule (currently
  failing for a real reason → reported, not deleted).

## Verification

- `go build ./...` clean; `go vet ./...` clean (no unused-import/compile break
  from deletions).
- `go test` green on spot high-deletion packages: vmctl, autoputer,
  yaegikernel, proxy, capsule.
- Full suite runs sharded in CI; local single-process `go test ./...` exceeds
  the wallclock window this mission is reducing.
