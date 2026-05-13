# Choir-in-Choir Deformation v0 Dogfood Report

Date: 2026-05-13
Mission: `docs/mission-choir-in-choir-deformation-v0.md`

## Result

This run converted the mission geometry into a first executable slice:

- durable promotion candidate queue records;
- durable run continuation records;
- runtime verification/promotion methods over candidate-world patchsets;
- automatic continuation from completed runs when bounded continuation metadata is present;
- first-class `vsuper` role/profile and default worker-VM delegation target;
- delegate-worker export collection that queues promotion candidates;
- one launcher/uploads/themes candidate patch dogfooded through queued verification and explicit promotion in tests.

## Promotion Queue Proof

New queue state:

- `promotion_candidates` table in the runtime store;
- `types.PromotionCandidateRecord` with candidate identity, owner, source loop, VM/snapshot, base SHA, worker head, patchset paths, verifier contracts, report JSON, status, and error;
- store methods for upsert, update, get, and owner-scoped list;
- runtime methods:
  - `QueuePromotionCandidate`;
  - `VerifyPromotionCandidate`;
  - `PromotePromotionCandidate`.

Verification/promotion semantics:

- queued candidates require verifier contracts before verification;
- verification imports the patchset onto an integration branch;
- destination branch remains at rollback base until promotion;
- promotion requires explicit `approved=true`;
- promotion blocks dirty/diverged canonical state through the existing promotion package.

## Vsuper Proof

`vsuper` is now a runtime profile:

- prompt default: `internal/runtime/prompt_defaults/vsuper.md`;
- canonical aliases include `vsuper`, `v-super`, `vm-super`, and `candidate-super`;
- internal worker runtime submissions may start `vsuper`;
- `delegate_worker_vm` defaults to `vsuper`;
- `vsuper` can mutate candidate state and export patchsets, but does not get foreground VM control tools or promotion authority.

## Continuation Proof

New continuation state:

- `run_continuations` table in the runtime store;
- `types.RunContinuationRecord`;
- `SelectRunContinuation` compacts source run memory before recording a next objective;
- `StartRunContinuation` starts a bounded child run with an allowed authority profile;
- completed runs can auto-select and auto-start a continuation when metadata includes:
  - `continuation_objective`;
  - `continuation_authority_profile`;
  - `continuation_lease_seconds`;
  - `continuation_auto_start`.

Allowed automatic continuation profiles are currently `vsuper`, `co-super`, and `researcher`.

## Dogfood Patch

The product-pressure patch was intentionally narrow: a launcher/uploads/themes onboarding marker in `frontend/src/lib/Launcher.svelte` inside a temp candidate repo.

The runtime dogfood test proves:

- worker/candidate patch is exported as a patchset;
- candidate is queued;
- verifier contract checks `launch-with-uploads-themes`;
- integration branch receives the patch;
- main remains unchanged before promotion;
- explicit promotion fast-forwards main;
- source run emits queued, verified, and promoted events.

Primary test: `TestRuntimePromotionQueueDogfoodsLauncherUploadsThemesPatch`.

## Verification

Focused verification passed:

```sh
CGO_CFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_CXXFLAGS='-I/opt/homebrew/opt/icu4c@78/include' \
CGO_LDFLAGS='-L/opt/homebrew/opt/icu4c@78/lib' \
go test ./internal/store ./internal/runtime ./internal/promotion
```

Observed result:

- `internal/store`: pass
- `internal/runtime`: pass
- `internal/promotion`: pass

## Residual Risks

- Promotion queue is runtime/store-visible, not yet a desktop UI.
- Automatic continuation currently uses explicit metadata, not full objective synthesis from mission docs, queue state, failed candidates, and product gaps.
- The dogfood product patch is a narrow marker proof, not the real app launcher, file upload UI/API, or theme editor.
- Background VM rollback is represented through git/base/promotion records; VM lease/snapshot rollback still needs live vmctl proof.
- Playwright bootstrap through the actual Choir desktop/prompt bar remains a next run target.
