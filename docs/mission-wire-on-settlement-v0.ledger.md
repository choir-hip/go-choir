# M5 Wire on Settlement — Parallax Mission Ledger

This is the append-only Parallax mission ledger for the M5 paradoc,
`docs/archive/mission-wire-on-settlement-v0.md`.

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

## 2026-06-12 — Public Surface Probe: Publication Corpus Live, Cycle Predicate Still Auth-Gated

Claim/scope: unauthenticated staging can still provide product-path observer
evidence about public platformd publication health, but it cannot by itself
settle M5 because the Universal Wire edition/cycle predicate is auth-gated.

Move: shift observer from prompt-bar/authenticated owner APIs to public
platformd publication, retrieval, and export APIs on deployed staging commit
`b8f33087ce099d11054447d852e788453379a787`.

Expected ΔV: 0-1. Either discover a public product predicate that can reduce
the product-proof blocker, or prove that authenticated owner proof is still
the first gate.

Actual ΔV: 0. Observer evidence improved, but all six settlement blockers
remain. Public retrieval/resolve/export are alive and useful receipts;
Universal Wire story/edition proof and cycle linkage still require an
authenticated owner session.

Receipt:
- `curl -fsS https://choir.news/health | jq .` reports proxy and sandbox
  `build.commit` / `deployed_commit`
  `b8f33087ce099d11054447d852e788453379a787`, deployed at
  `2026-06-12T23:11:18Z`.
- `curl -i https://choir.news/api/universal-wire/stories` returned HTTP 401
  `{"error":"authentication required"}`.
- `curl -fsS 'https://choir.news/api/platform/retrieval/search?q=Universal%20Wire'`
  returned HTTP 200 with zero results.
- `curl -fsS 'https://choir.news/api/platform/retrieval/search?q=wire'`
  returned HTTP 200 with 15 public publication results.
- Public resolve for
  `/pub/vtext/climate-change-raises-bilateral-trade-costs-through-maritime-shipping-disruption-boe-research-fi-pub09e4bf037`
  returned an active public route plus consent, review, and attestation ids.
- Public export for that route returned publication
  `pub-09e4bf03-7cf8-43ea-88f1-191c6f68bc1b`, version
  `pubver-1b8910c7-ab8e-43e5-9570-346ea94e35ca`, Markdown content length
  `4390`, `private_material_omitted=true`, source revision hash
  `9a1f53d16ada1e0bd3f1683b11ba16a04995695325c00bbf90d120aadbcb1fa1`,
  two source manifest entries, and two transclusions.
- Public search showed duplicate-looking titles with distinct publication ids
  and distinct source revision hashes, including four variants of
  `Climate Change Raises Bilateral Trade Costs Through Maritime Shipping
  Disruption, BoE Research Finds.vtext`. This is a front-page quality
  discriminator to check under auth, not a standalone accounting-leak verdict.

Open edge: resume with an authenticated owner session and prove the actual
cycle link: trace/vtext/publication/front-page receipts, sourcecycled timing,
duplicate/stale-publication interpretation, and production maxProc>1 behavior.

## 2026-06-12 — Independent Review Falsifier: Revision Metadata Merge Not Serialized

Claim/scope: the landed M5/M5a stack is not ready for final handoff if a
concurrent publication metadata patch can drop another revision metadata key.
Scope is code review evidence before any code fix.

Move: prover/review shift over the landed M5 range, using the store JSON
merge/concurrency edge as the falsifier.

Expected ΔV: either 0 if the prior review claim holds, or negative if the
review finds an unfixed accounting substrate problem.

Actual ΔV: -1. Current V increases from 6 to 7 until the problem is fixed and
verified. The paradoc previously claimed Store JSON merge patches are
serialized, and trajectory/work-item patches do take `Store.jsonPatchMu`, but
revision metadata patches do not.

Receipt:
- `internal/store/store.go:60` defines `jsonPatchMu`.
- `internal/store/trajectory.go:150` `UpdateTrajectorySubjectRefs` locks
  `s.jsonPatchMu` around read/merge/write of `subject_refs_json`.
- `internal/store/trajectory.go:380` `UpdateWorkItemDetails` locks
  `s.jsonPatchMu` around read/merge/write of `details_json`.
- `internal/store/vtext.go:844` `PatchRevisionMetadata` reads the revision,
  unmarshals `metadata_json`, merges patch keys, and writes the whole JSON
  object back without taking `s.jsonPatchMu`.
- `internal/runtime/wire_platform_publish.go:169` uses
  `PatchRevisionMetadata` to persist `platformd_route_path` and
  `platformd_publication_ref`, so a concurrent metadata patch can lose the
  publication ref or the other patch key.

Open edge: fix `PatchRevisionMetadata` with the same Store-instance
serialization guard, add a concurrent regression test, then rerun focused
store/runtime checks before returning to authenticated product proof.

## 2026-06-12 — Construct + Verify: Revision Metadata Merge Serialized

Claim/scope: the independent-review falsifier is discharged locally if VText
revision metadata merge patches share the same Store-instance serialization
guard as trajectory subject refs and work item details, and a concurrent test
preserves all patch keys. Scope is local code/test evidence for the
metadata-merge edge, not staging product proof.

Move: construct. `PatchRevisionMetadata` now locks `Store.jsonPatchMu` before
reading, merging, and writing `metadata_json`; added
`TestVTextRevisionMetadataConcurrentMergePatchesPreserveKeys`.

Expected ΔV: +1 by removing the unfixed revision-metadata merge blocker.

Actual ΔV: +1 locally. Current V returns to 6. Push/CI/deploy evidence for
the fix is still pending, and authenticated product proof remains open.

Receipt:
- Commit `0d4d57a5` (`fix: serialize vtext metadata merge patches`).
- `nix develop -c go test ./internal/store -run TestVTextRevisionMetadataConcurrentMergePatchesPreserveKeys -count=1`
  passed.
- `nix develop -c go test ./internal/store -count=1` passed in `56.994s`.
- `nix develop -c go test ./internal/runtime -run 'TestWire|TestVText|TestTrajectory|TestProcessor' -count=1`
  passed.
- `nix develop -c go test ./internal/runtime ./internal/cycle ./internal/store ./cmd/sourcecycled -count=1`
  passed: `internal/runtime` 22.374s, `internal/cycle` 2.825s,
  `internal/store` 60.112s, `cmd/sourcecycled` 5.561s.
- `git diff --check` passed.

Open edge: commit this paradoc update, push the fix stack, monitor CI/deploy,
verify staging identity for the new behavior SHA, then resume authenticated
product-path Universal Wire proof.

## 2026-06-12 — Landing + Product-Proof Boundary: Metadata Fix Deployed, Auth Gate Remains

Claim/scope: the metadata-merge fix is landed only if the behavior-changing
stack is pushed, CI-green, deployed to staging, and staging reports the new
commit. M5 settlement still requires an authenticated product-path wire-cycle
proof and a production multi-story maxProc>1 cycle.

Move: land and probe. Pushed the stack through `4b4562a2`, monitored CI/deploy,
verified `/health`, then retried the smallest allowed unauthenticated
product-path probes plus public publication controls.

Expected ΔV: 0. Landing proof should strengthen the evidence class but not
remove the authenticated product-proof or production-cycle blockers.

Actual ΔV: 0. Metadata fix is deployed; V remains 6. Product-path cycle proof
is still blocked on owner authentication.

Receipt:
- Pushed `4b4562a2e01549291a3ff2080ec2a187ef5f365f` to `origin/main`.
- CI run `27449221402` completed successfully.
- CI jobs included runtime shards 0/1/2/3, non-runtime tests, Go vet/build,
  integration smoke, TLA+ model check, deploy-impact, and staging deploy.
- Deploy job `81140982346` succeeded in 21s.
- FlakeHub publish run `27449221388` succeeded.
- `curl -fsS https://choir.news/health | jq .` reported proxy and sandbox
  `build.commit` / `deployed_commit`
  `4b4562a2e01549291a3ff2080ec2a187ef5f365f`, deployed at
  `2026-06-12T23:37:50Z`.
- `curl -i https://choir.news/api/universal-wire/stories` returned HTTP 401
  `{"error":"authentication required"}`.
- `curl -i -X POST https://choir.news/api/prompt-bar ...` returned HTTP 401
  `{"error":"authentication required"}`.
- `curl -fsS 'https://choir.news/api/platform/retrieval/search?q=wire'`
  returned HTTP 200 with 15 public publication results.
- Public export for
  `/pub/vtext/climate-change-raises-bilateral-trade-costs-through-maritime-shipping-disruption-boe-research-fi-pub09e4bf037`
  returned publication `pub-09e4bf03-7cf8-43ea-88f1-191c6f68bc1b`,
  version `pubver-1b8910c7-ab8e-43e5-9570-346ea94e35ca`, Markdown content
  length `4390`, `private_material_omitted=true`, and source revision hash
  `9a1f53d16ada1e0bd3f1683b11ba16a04995695325c00bbf90d120aadbcb1fa1`.

Open edge: owner-authenticated session required for Universal Wire
cycle/front-page/trace proof. Do not call M5 settled from public corpus health.

## 2026-06-12 — Blocked Exit: Owner Session Still Signed Out

Claim/scope: M5 cannot descend past V=6 from the current observer if no
authenticated owner session is available. Scope is product-path proof
readiness after the metadata fix landed on staging.

Move: shift observer from unauthenticated curl to the user's Chrome profile
using the existing `choir.news` tab. Read visible page state only; did not
inspect cookies, local storage, profiles, or internal routes.

Expected ΔV: 0 or +1. If Chrome carried an owner session, the next probe could
attempt authenticated Universal Wire cycle evidence. If Chrome was still
signed out, the mission should exit blocked with a precise owner obligation.

Actual ΔV: 0. Chrome still rendered the signed-out preview, so the missing
owner-authenticated session remains the first blocker. Current V remains 6.

Receipt:
- `git status --short` was clean; `HEAD == origin/main ==
  c2be9b7c7980f5b972230e7a2b4a8accf5c732a1`.
- Staging `/health` still reports proxy and sandbox deployed behavior commit
  `4b4562a2e01549291a3ff2080ec2a187ef5f365f`, deployed at
  `2026-06-12T23:37:50Z`.
- Chrome open tabs included `https://choir.news/`.
- After claiming and refreshing that tab, visible page text included
  `Choir Preview` and `Local preview - sign in to save`.
- The page did not expose a usable owner session for authenticated
  `/api/universal-wire/stories` or prompt-bar product proof.

Blocked obligation: owner must open or provide an authenticated
`https://choir.news` session, then run one real product-path Universal Wire
cycle. Public platformd corpus health and signed-out preview state remain
insufficient for M5 settlement.

## 2026-06-12 — Portfolio Sequencing Correction: Defer Wire Product Gate

Claim/scope: M5 should not spend owner attention on whether Universal Wire is
currently product-complete if the portfolio's real next move is durable actors
and old-code deletion. Scope is mission sequencing and proof authority, not a
code change and not a Universal Wire bug verdict.

Move: update the portfolio and this paradoc. The previous route treated M5 as
the next active gate after M1 because settlement accounting can be modeled
before M2-M4. The owner corrected the operating expectation: get durable
actors working and remove the old coordination/continuation code before
worrying about Universal Wire product completeness.

Expected ΔV: 0. The M5 settlement blockers remain, but they are parked behind
M2-M4 instead of being chased through the current empty front page.

Actual ΔV: 0. Current V remains 6; status changes from blocked-on-auth to
deferred-on-portfolio-sequencing. The metadata fix remains landed at
`4b4562a2e01549291a3ff2080ec2a187ef5f365f`. The authenticated Universal Wire
empty-state observation is retained as a later discriminator, not an active
mission blocker.

Receipt:
- Updated `docs/mission-portfolio-2026-06-11.md` so the recommended order is
  M9 -> M1 -> M2 -> M3 -> M4 -> M5 -> M6+M7 -> M8.
- Updated M5 dependencies: substrate work can consume M1, but the
  product-facing route-switch evidence gate depends on M4.
- Updated this paradoc's status to `deferred` and rewrote the resume goal
  string to send future agents through M2-M4 before Universal Wire product
  proof.
- Owner-provided discriminator: an authenticated browser session can be
  supplied, and the owner observed Universal Wire showing no articles; this
  does not settle or falsify M5 before durable actors/old-code deletion.

Open edge: compile or resume M2 as the next active mission. Return to M5 only
after M2-M4 make durable actors operational and remove the old continuation /
parent-child control paths.
