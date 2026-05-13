# Promotion Queue Owner Review Proof

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`

## Slice

Promotion review now has a product-visible owner decision without giving the browser promotion authority:

- `POST /api/promotions/{candidate_id}/approve` records owner approval for a verified candidate;
- `POST /api/promotions/{candidate_id}/reject` marks an owner-scoped candidate rejected;
- Settings renders Approve/Reject controls for eligible candidates;
- browser traffic stays on `/api/promotions/*` and never calls `/internal/*`;
- internal promotion now requires both verifier success and recorded owner approval.

This closes one important gap in the candidate-world bridge: human approval is now durable product state, not only an internal boolean supplied by a privileged caller.

## Invariant Preserved

The browser can review but cannot promote canonical state. Promotion still requires the internal promotion path, a verified promotion report, a clean destination branch, and explicit internal approval. The new owner approval step is necessary but not sufficient for promotion.

## Verification

Commands run:

```text
cd frontend && npm run build
```

Result: build passed. Existing warning remains for the large `ghostty-web` chunk.

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime -run 'TestRuntimePromotionQueueDogfoodsLauncherUploadsThemesPatch|TestPromotionCandidatePublicListAndDetailAreOwnerScoped|TestPromotionCandidatePublicReviewIsOwnerScopedAndNonPromoting|TestInternalPromotionRoutesRequireInternalCallerAndQueueCandidate'
```

Result: passed.

```text
cd frontend && npx playwright test trace-settings-registry.spec.js --workers=1 --timeout=120000
```

Result: `3 passed`.

```text
cd frontend && npx playwright test desktop-shell-core.spec.js file-browser.spec.js trace-settings-registry.spec.js --workers=1 --timeout=120000
```

Result: `41 passed`.

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test -count=1 ./internal/runtime ./internal/store ./internal/promotion
```

Result: passed.

```text
git diff --check
```

Result: passed.

## Boundary

The approval proof still does not show Choir producing a candidate through its own prompt/super/worker loop. It makes that loop reviewable once the worker export arrives.

## Next Deformation

Drive a narrow product prompt through VText or super so the runtime itself requests a worker, delegates a candidate task, exports a patchset, queues it, and lands in this owner-review surface.
