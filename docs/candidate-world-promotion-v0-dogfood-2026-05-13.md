# Candidate World Promotion v0 Dogfood

Date: 2026-05-13

## Scope

This report proves `docs/mission-candidate-world-promotion-v0.md`.

The v0 artifact is `internal/promotion`: a promotion controller above `internal/shipper` that records candidate-world identity, imports worker patchsets into integration branches, runs verifier contracts, blocks foreground divergence, and mutates the destination branch only after explicit approval.

## Dogfood Path

The dogfood is `TestCandidateWorldPromotionDogfoodsLauncherPatch`.

It creates a temporary product-shaped repo with:

- `frontend/src/lib/Launcher.svelte`;
- a clean `main` base SHA;
- a cloned background-worker repo;
- a worker branch named `agent/run-cwp-v0/background-vm`.

The worker branch makes a narrow launcher product patch by adding the marker `launch-with-uploads-themes`. It then exports a patchset and manifest through `shipper.ExportPatchset`, including:

- run ID: `run-cwp-v0`;
- trace ID: `trace-cwp-v0`;
- VM ID: `vm-branch-per-candidate`;
- snapshot ID: `snapshot-before-mutation`;
- base SHA;
- worker head SHA;
- verification command proving the launcher marker exists.

`promotion.PrepareIntegrationCandidate` imports that patchset into `agent/run-cwp-v0/launcher-product-patch`, runs a verifier contract against the integration branch, and writes a promotion report. The test asserts that `main` does not contain the candidate marker before promotion.

Only after verification does `promotion.ApplyVerifiedPromotion(..., approved=true)` fast-forward `main` to the verified integration branch. The saved report records `status=promoted`, `canonical_mutated=true`, `promotion_approved=true`, and rollback metadata.

## Invariants Proven

- Foreground/canonical branch is not mutated during integration.
- Candidate identity includes owner, VM, run, snapshot, base SHA, worker head SHA, manifest path, patchset path, and integration branch.
- Integration uses branch-per-candidate geometry: `agent/<run>/<slug>`.
- Verifier contracts are records, not agent species.
- Promotion requires explicit approval.
- Promotion is blocked if the destination branch has diverged from the recorded rollback base.
- Rollback command is recorded as `git switch <destination> && git reset --hard <base_sha>`.

## Verification

Focused package test:

```sh
go test ./internal/promotion
```

Result:

```text
ok github.com/yusefmosiah/go-choir/internal/promotion
```

Full suite with local ICU flags:

```sh
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' \
go test ./...
```

Result: all packages passed.

## Residual Risks

This is a library-level v0, not a live runtime API exposed to super. The existing `delegate_worker_vm` and `export_patchset` tools produce the inputs, but platform-side import/promotion still needs a product surface or internal endpoint.

Promotion currently uses `git merge --ff-only`. That is intentionally conservative. Non-fast-forward integration should remain a separate integration-candidate flow with its own verification contract.

Verifier contracts run shell checks in the integration repo. That is sufficient for v0 but should gain richer capability profiles for Browser/Playwright, frontend build, security scans, and deployed/staging checks.
