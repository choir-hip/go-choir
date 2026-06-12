# M5 Wire on Settlement — Parallax Mission Ledger

This is the append-only Parallax mission ledger for the M5 paradoc,
`docs/mission-wire-on-settlement-v0.md`.

Historical passes before 2026-06-12 are retained inline in the mission
document under `ledger / move log`. Future passes should append here and
rewrite only the mission document's compact `Parallax State`.

## 2026-06-12 — Resume Checkpoint

Claim/scope: the local M5/M5a substrate is ready for the handoff/settlement
boundary, not settled. Scope is local repo verification only.

Move: settle-or-handoff preparation; update Parallax State with variant,
budget, next move, ledger pointer, and current evidence.

Expected ΔV: 0 direct settlement decrease; observer evidence improves the
next run by naming the exact remaining blockers.

Actual ΔV: local verification debt decreased, but settlement V remains 9
because landing, staging, production cycle, independent review, honest-and-
full instrumentation, and final core-claim verdict are still open.

Receipt:
- `nix develop -c go test ./internal/runtime ./internal/cycle ./internal/store ./cmd/sourcecycled` passed.
- `nix develop -c go test -tags comprehensive -count=1 ./internal/runtime` passed at `102.400s` package / `113.71s` wall.
- `nix develop -c go vet ./...` passed.
- `nix develop -c go vet -tags comprehensive ./...` passed.
- `git diff --check` passed.

Open edge: `internal/store/trajectory.go` read-merge-write JSON patch helpers
still overclaim concurrent key preservation; attempted CAS was not
Dolt-compatible and was backed out. Treat this as the first independent-review
falsifier before landing.

## 2026-06-12 — Review Finding: Stale Reset Crossed Verdict/Runtime Axes

Claim/scope: sourcecycled's split `status` and `runtime_status` axes must not
be re-fused by recovery code. Scope is local sourcecycled storage/reconcile
behavior.

Move: review/probe. Read `cmd/sourcecycled/main.go` and
`internal/cycle/storage.go` after the runtime-status split.

Expected ΔV: either discharge one review blocker or expose a real blocker
before code landing.

Actual ΔV: found a real bug. `ResetProcessorRequestSubmission` and
`ResetStaleSubmittedProcessorRequests` still reset both `status` and
`runtime_status` from rows selected by `runtime_status='submitted'`. That can
erase a request verdict that was already projected to `completed` or
`deferred` while the runtime-capacity slot remained submitted.

Receipt: `internal/cycle/storage.go` stale/orphan reset queries before fix.

Open edge: fix must preserve terminal request verdicts while releasing or
requeuing only the runtime-capacity axis.

## 2026-06-12 — Construct + Verify: Review Blockers Removed Locally

Claim/scope: the two pre-landing review blockers can be removed without
changing the M5 settlement claim scope. Scope is local repo behavior.

Move: construct. Serialize Store JSON merge patches within one Store instance
and make sourcecycled stale/orphan runtime recovery preserve already projected
request verdicts while releasing the runtime-capacity axis.

Expected ΔV: reduce local review blockers, but do not reduce staging or
production evidence blockers.

Actual ΔV: local review blockers removed. V remains 9 because the remaining
blockers are landing/CI/staging/production evidence, honest-and-full
instrumentation, maxProc>1 cycle proof, processor-phase admission scope,
non-fetch deferred wake policy, and final rearchitecture verdict.

Receipt:
- `nix develop -c go test ./internal/store -run 'TestTrajectorySubjectRefs.*Merge|TestWorkItemDetails.*Merge'` passed.
- `nix develop -c go test ./internal/cycle -run 'TestProcessorRequestRuntimeStatusCanDivergeFromVerdictStatus|TestResetProcessorRequestSubmissionPreservesProjectedVerdict|TestResetStaleSubmittedProcessorRequestsPreservesProjectedVerdicts'` passed.
- `nix develop -c go test ./cmd/sourcecycled -run 'TestIngestionRuntimeDispatcher'` passed.
- `nix develop -c go test ./internal/runtime ./internal/cycle ./internal/store ./cmd/sourcecycled` passed.
- `nix develop -c go test -tags comprehensive -count=1 ./internal/runtime` passed at `103.344s` package / `120.82s` wall.
- `nix develop -c go vet ./...` passed.
- `nix develop -c go vet -tags comprehensive ./...` passed.
- `git diff --check` passed.

Open edge: this is still local proof. M5 remains open_handoff until landing,
staging build identity, product-path wire-cycle evidence, and the production
maxProc>1 evidence gate are recorded.
