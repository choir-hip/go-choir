# Promotion Queue Product Bridge Dogfood

Date: 2026-05-13
Mission: `docs/mission-choir-grand-deformation-v0.md`
Related mission: `docs/mission-promotion-queue-v0.md`

## Slice

This slice turns promotion queue v0 from runtime/store methods into a visible product/runtime bridge:

- `/api/promotions` lists authenticated user's candidate-world patchsets;
- `/api/promotions/{candidate_id}` returns owner-scoped candidate detail;
- `/internal/promotions` queues candidate records for platform callers only;
- `/internal/promotions/{candidate_id}/verify` verifies a queued candidate against a repo path;
- `/internal/promotions/{candidate_id}/promote` promotes a verified candidate only when explicitly approved;
- Settings now shows a read-only Promotion queue panel, so candidate-world patchsets become product-visible without exposing internal mutation endpoints to the browser.
- Playwright seeds a candidate through the sandbox internal API, then verifies the browser renders it through Settings without making any `/internal` browser requests.

Follow-up owner review slice:

- `/api/promotions/{candidate_id}/approve` records owner approval for a verified candidate;
- `/api/promotions/{candidate_id}/reject` records owner rejection;
- Settings exposes Approve/Reject controls for eligible candidates;
- internal promotion requires recorded owner approval as well as verifier success.

## Existing Substrate Confirmed

`delegate_worker_vm` already scans worker run events for `export_patchset` tool results and queues one promotion candidate per worker export. The result returned to super includes both `export_patchsets` and `promotion_queue` entries.

## Verification

Commands run:

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test ./internal/runtime -run 'TestPromotionCandidatePublicListAndDetailAreOwnerScoped|TestInternalPromotionRoutesRequireInternalCallerAndQueueCandidate|TestHandlePromptBarCreatesServerOwnedConductorRun'
```

Result: passed.

```text
cd frontend && npx playwright test trace-settings-registry.spec.js --grep "Settings renders queued promotion candidates" --workers=1 --timeout=120000
```

Result: passed.

```text
cd frontend && npx playwright test desktop-shell-core.spec.js file-browser.spec.js trace-settings-registry.spec.js --workers=1 --timeout=120000
```

Result: `40 passed`.

Rerun after owner-review controls were added:

```text
cd frontend && npx playwright test desktop-shell-core.spec.js file-browser.spec.js trace-settings-registry.spec.js --workers=1 --timeout=120000
```

Result: `41 passed`.

```text
cd frontend && npm run build
```

Result: build passed. Existing warning remains for the large `ghostty-web` chunk.

```text
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' go test ./...
```

Result: all packages passed.

```text
git diff --check
```

Result: passed.

## Boundary

The browser can inspect queue state and record owner approve/reject decisions, but verification and canonical promotion remain internal-only. That preserves the authority boundary: human review is product state, while repo mutation still requires verified internal promotion.

The next Choir-in-Choir proof still needs a live product-path loop where a prompt/super run requests or uses a background worker VM, produces an exported patchset, queues it, verifies it, and then promotes after explicit approval.

## Next Deformation

Use Playwright to drive Choir through the prompt bar toward a bounded candidate-world task. The local launcher now starts `vmctl`, so the next blocker is not control-plane absence; it is proving the product path can cause super to request a worker, delegate work, export a patchset, queue the candidate, verify, and promote through the visible queue.
