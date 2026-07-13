# S3 Candidate Intake Harness Deletion Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I17-candidate-intake-harness-deletion`
- Dispatch nonce: `s3-runtime-dissolution-i17-nonce-01`
- Transition: `s3-i17-dispatch-intent-252`
- Canonical parent: `3f485404`
- Mutation class: orange
- Protected surfaces: none; the live route table is unchanged
- Rollback ref: `3f485404`

## Boundary Decision

Post-S3-I16 mapping found `20` production files and `128` methods on `runtime.APIHandler`. Go requires every method on a type to remain in the declaring package, so the eventual handler ownership cutover must be atomic. Smaller live receiver moves, a second handler type, alias, wrapper, interface, callback table, generic facade, accessor layer, forwarding shim, and duplicated test registrar are invalid.

The dependency map also found the final two runtime-scoped route registrations are a dormant opt-in candidate-package mutation harness. `RegisterCandidatePackageIntakeRoutes` is not called by production; its only callers are tests. It preserves eleven HTTP receiver methods that are superseded by cohesive runtime candidate-intake operations. Two independent architecture reviewers recommended deleting this residue before the atomic handler move; one recommended immediate atomic cutover, and one review was inconclusive. Deletion-first governs: do not carry dead transport authority into the cutover.

## Exact Mutation Lock

Production deletion in `internal/runtime/api_candidate_package_intake.go`:

- `RegisterCandidatePackageIntakeRoutes`
- `candidatePackageIntakeWriteRoutesDisabled`
- `HandleCandidatePackageIntakesRoot`
- `HandleCandidatePackageIntakeDetail`
- `handleCandidatePackageIntakeReview`
- `handleCandidatePackageIntakeAdoptionBoundary`
- `handleCandidatePackageIntakePublicationDraft`
- `handleCandidatePackageIntakeAdoptionReviewCreate`
- `handleCandidatePackageIntakeAdoptionReviewDecision`
- `handleCandidatePackageIntakePromotionSwitch`
- `handleCandidatePackageIntakePromotionSwitchRollback`
- `handleCandidatePackageIntakePromotionSwitchRollForward`
- `handleCandidatePackageIntakePromotionAcceptance`

Retain the live read-only review surface exactly:

- `HandleCandidatePackageReviewSurfaceReadOnly`
- `handleCandidatePackageIntakePromotionReviewSurface`

Delete tests and helpers whose sole contract is the dormant mutation registrar, including `candidatePackageIntakeTestServer`. Preserve candidate-intake domain invariants through direct calls to the existing runtime business operations; build live read-only review-surface fixtures through those operations. Do not add exported test helpers, a registrar, route table, facade, or HTTP shim.

Allowed files: `internal/runtime/api_candidate_package_intake.go`, direct candidate-intake tests, `internal/apihandler/routes_test.go` only if an exclusion assertion is required, and `docs/runtime-dissolution-inventory.yaml` after focused proof. Everything else is forbidden.

## Acceptance

- Zero `RegisterCandidatePackageIntakeRoutes`, `candidatePackageIntakeWriteRoutesDisabled`, deleted receiver declarations, or callers under any build tag.
- Runtime-scoped routes `2 -> 0`; APIHandler receiver methods `128 -> 117`; runtime production LOC, exports, and caller edges decrease; no importer/wrapper/accessor/interface growth.
- The canonical `46`-slot live apihandler table is unchanged; only its existing read-only candidate review route remains.
- Former mutation paths are absent from every production and test server.
- Read-only review authorization, method rejection, owner scope, and no-mutation behavior remain covered.
- Direct domain tests preserve creation, review, adoption boundary/review, promotion switch, rollback, roll-forward, acceptance, and review-surface behavior.
- Focused candidate-intake, apihandler, and ratchet tests pass. No staging deployment is manufactured because live behavior does not change; independent verification, consensus, adjudication, and durable closure remain required.
