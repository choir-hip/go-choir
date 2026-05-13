# MissionGradient: Candidate World Promotion v0

Status: completed in repo
Date: 2026-05-13

Completion notes:

- Implementation: `internal/promotion`
- Dogfood proof: `docs/candidate-world-promotion-v0-dogfood-2026-05-13.md`
- Next frontier: `docs/candidate-world-promotion-next-frontier-2026-05-13.md`
- Next mission doc: `docs/mission-promotion-queue-v0.md`

## Real Artifact

Candidate-world promotion v0: a background-VM-safe path where a worker mutates an isolated repo state, exports a patchset with evidence, super imports it into an integration candidate, verifier contracts run, and canonical state changes only after promotion.

## Value Criterion

Maximize verified improvement from background VM work while minimizing canonical-state corruption, merge ambiguity, rollback cost, user review burden, and unverifiable agent claims.

## Invariants

- Foreground state is never speculatively mutated.
- Every candidate has owner, VM, base SHA, worker head or patchset, verification evidence, and rollback point.
- Branch-per-candidate-VM is the default geometry unless the implementation proves a safer equivalent.
- Promotion is explicit, owner-mediated, and non-blocking.
- Verifier contracts are records with target, invariants, checks, capability profile, result schema, and evidence paths.
- User foreground divergence blocks blind patch application and forces integration-candidate verification.
- Existing user work is not reverted or overwritten.

## Homotopy

Increase difficulty in this order:

- model the candidate-world record and exported evidence shape;
- prove rollback metadata for a synthetic candidate;
- import a patchset into an integration candidate without touching canonical state;
- run verifier contracts against that candidate;
- dogfood one narrow Choir-in-Choir product patch, preferably in launcher/uploads/themes;
- produce a promotion report that a human can accept or reject.

## Dense Feedback

- Unit tests for candidate records, verifier contract records, and rollback metadata.
- Integration test for export/import without canonical mutation.
- Git-state assertions for base SHA, worker head, changed files, and dirty state.
- Verification-event assertions.
- One dogfood report with commands, patchset path, verification path, and promotion decision.

## Stopping Condition

Stop when candidate-world promotion v0 is implemented, verified by tests, and demonstrated by one narrow background-VM-style Choir-in-Choir patch, or when a blocked report names the failed invariant, rollback point, evidence, and next smallest probe.
