# MissionGradient: Promotion Queue v0

Status: next runnable mission document
Date: 2026-05-13

## Real Artifact

Promotion queue v0: the product/runtime bridge that turns background VM patchset exports into reviewable candidate promotions with verifier contracts, rollback evidence, changed files, and explicit user-mediated promotion.

## Value Criterion

Maximize safe throughput from background VM work into canonical Choir while minimizing hidden mutation, merge ambiguity, user review burden, verifier Goodharting, and rollback cost.

## Invariants

- Background VM work enters canonical state only through candidate promotion records.
- Every candidate record names owner, run, VM, snapshot, base SHA, worker head SHA, patchset, manifest, changed files, verifier contracts, and rollback point.
- Promotion is unavailable until verification passes.
- Foreground divergence blocks direct promotion and requires an integration/reverify path.
- Workers cannot push or promote directly.
- Existing user work is not reverted or overwritten.

## Homotopy

- Store candidate promotion records.
- Expose internal platform APIs for create/list/detail/verify/promote.
- Wire `delegate_worker_vm` export results into candidate records.
- Add a minimal review UI or trace surface.
- Dogfood one launcher/uploads/themes patch through the queue.

## Dense Feedback

- Store/API tests for candidate records and state transitions.
- Promotion package tests reused for integration, verification, divergence, and rollback.
- Runtime/tool tests proving worker exports become queue entries.
- UI or trace test proving the user can inspect the promotion report.
- Full `go test ./...` and frontend build when frontend code changes.

## Stopping Condition

Stop when a background-VM-style patch can move from export to review queue to verified promotion with evidence and rollback, or when a blocked report names the failed invariant and next smallest probe.
