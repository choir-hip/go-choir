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

## 2026-06-12 — Landing Blocker: Proxy Wire Publish Test Fixture Drift

Claim/scope: the M5 substrate can pass CI and reach staging only if the host
wire-publication choke-point test exercises the same sandbox API shape as the
handler. Scope is CI landing evidence for pushed commit `09a5dc80`.

Move: observe landing. Pushed the docs/Parallax/M5 implementation stack and
monitored GitHub Actions run `27447982144`.

Expected ΔV: landing proof should reduce deploy/staging uncertainty.

Actual ΔV: CI found a concrete blocker before deploy. Runtime shards, vet,
build, integration smoke, TLA+, and deploy-impact passed, but non-runtime Go
tests failed in `internal/proxy`.

Receipt: GitHub Actions run `27447982144`, job `81137136114`,
`TestHandleInternalWirePlatformPublishPostsToPlatformd`:
`status = 502 body = {"error":"failed to load wire document"}` after the
handler logged `proxy: wire publish fetch document: sandbox status 404`.

Open edge: inspect the proxy handler's sandbox fetch path and update the test
fixture or handler as appropriate; rerun `internal/proxy` and then push a fix
commit so deploy can proceed.

## 2026-06-12 — Construct + Verify: Proxy Fixture Matches Wire Choke Point

Claim/scope: the CI blocker was test-fixture drift, not a handler regression.
The handler reads the platform owner's document and revision through sandbox
`/internal/vtext/...` paths; the post-publish async sync still reads the public
document revisions list.

Move: construct. Update `TestHandleInternalWirePlatformPublishPostsToPlatformd`
to serve the handler's internal document/revision paths while preserving the
public revisions-list fixture used by sync.

Expected ΔV: unblock CI landing without changing runtime behavior.

Actual ΔV: local non-runtime package blocker removed; deploy/staging evidence
still pending on the next pushed CI run.

Receipt:
- `nix develop -c go test ./internal/proxy -run TestHandleInternalWirePlatformPublishPostsToPlatformd` passed.
- `nix develop -c go test ./internal/proxy` passed in `10.628s`.

Open edge: push the fix commit, monitor CI/deploy, then verify staging build
identity and product-path wire evidence.

## 2026-06-12 — Landing Edge: Green Fix Run Skipped Deploy

Claim/scope: a green CI run is not sufficient landing proof if the deploy job
is skipped after a prior behavior-changing commit failed before deploy. Scope
is the pushed stack ending at `7504e151`.

Move: observe landing rerun. Pushed `7504e151` and monitored GitHub Actions run
`27448208407`.

Expected ΔV: green CI should start staging deploy.

Actual ΔV: CI passed, but deploy was skipped. The deploy-impact job compared
the test/docs fix commit against its immediate parent and saw no deployed
artifact path change, while the previous behavior commit `09a5dc80` had never
deployed because run `27447982144` failed before deploy.

Receipt: GitHub Actions run `27448208407` passed all Go/TLA/frontend gates but
reported `Deploy to Staging (Node B)` as skipped.

Open edge: use the workflow's explicit `force_staging_deploy=true` dispatch on
`main`, then verify `/health` build identity before attempting product-path
wire proof. Do not claim staging acceptance from the green skipped-deploy run.

## 2026-06-12 — Landing Proof: Forced Deploy + Staging Identity

Claim/scope: the M5 substrate stack reached staging, but not product-path or
production-cycle acceptance. Scope is deploy identity only.

Move: force the documented deploy path after the skipped-deploy edge. Dispatch
`ci.yml` on `main` with `force_staging_deploy=true`.

Expected ΔV: discharge CI/deploy/build-identity blockers.

Actual ΔV: CI and staging identity are discharged. M5 remains open_handoff
because no authenticated product-path wire-cycle proof or production maxProc>1
cycle has been observed.

Receipt:
- Push CI run `27448208407` for `7504e151` passed all gates but skipped deploy.
- Manual workflow_dispatch run `27448287123` for `b8f33087` passed all gates.
- Deploy job `81138219091` completed successfully in `5m13s`.
- `curl -fsS https://choir.news/health | jq .` returned proxy and sandbox
  `build.commit` / `deployed_commit`
  `b8f33087ce099d11054447d852e788453379a787`, deployed at
  `2026-06-12T23:11:18Z`.

Open edge: run product-path proof from an authenticated owner session.

## 2026-06-12 — Product-Proof Blocker: No Authenticated Browser Session

Claim/scope: staging product APIs remain correctly auth-gated and cannot be
used for proof without an owner session. Scope is product-path proof readiness,
not runtime correctness.

Move: probe only allowed browser-public/product paths after staging identity.

Expected ΔV: either submit/observe a real product-path proof or identify the
first missing condition.

Actual ΔV: product-path proof is blocked on auth. Unauthenticated curl calls to
`POST /api/prompt-bar` and `GET /api/prompt-bar/submissions/nonexistent`
returned `401 {"error":"authentication required"}`. Chrome-connected staging
tabs rendered the signed-out preview ("Local preview - sign in to save").

Receipt:
- `curl -i https://choir.news/api/prompt-bar ...` -> HTTP 401.
- `curl -i https://choir.news/api/prompt-bar/submissions/nonexistent` -> HTTP
  401.
- Chrome DOM snapshots of `https://choir.news/` showed the signed-out preview,
  not the durable owner computer.

Open edge: owner must provide or open an authenticated `choir.news` session;
then resume with the goal string in the paradoc. Do not bypass auth with
internal routes.
