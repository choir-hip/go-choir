# S3 Tool-Loop Test-Only Option Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I5-toolloop-test-option`
- Dispatch nonce: `s3-runtime-dissolution-i5-nonce-01`
- Transition: `s3-i5-dispatch-intent-123`
- Canonical parent: `b1e2d214`
- Mutation class: orange
- Rollback: revert the atomic implementation landing to `b1e2d214`; do not restore the export through an alias.
- Protected surfaces: none. No route, tool registration, run state, provider/model, Trace, Wire, promotion/rollback, candidate computer, auth/session, vmctl, gateway, or deployment-routing surface may change.
- Conjecture delta: `WithCompletionGuard` is an exported convenience with one same-package test caller and no production caller; direct same-package option setup preserves completion-guard behavior without retaining false production API.
- Heresy delta at dispatch: `discovered: one test-only exported option wrapper`; `introduced: none`; `repaired: pending`.

## Problem Record

Repository-wide and build-tag-aware searches identify exactly one caller of `WithCompletionGuard`: `TestRunToolLoopCompletionGuardRetriesEndTurn` in `internal/runtime/toolloop_test.go`. Production never supplies this option. The test exercises completion-guard behavior, which remains meaningful; only the exported constructor wrapper is dead.

Tests must not keep otherwise unused production APIs alive. The test can construct the same `ToolLoopOption` closure directly in the same package while preserving guard retries, recorded attempts, provider calls, and terminal behavior. This problem record precedes implementation.

## Exact Mutation Lock

Allowed files only:

- `internal/runtime/toolloop.go`: delete exactly `WithCompletionGuard` and its attached comment.
- `internal/runtime/toolloop_test.go`: replace the one wrapper call with a direct same-package `ToolLoopOption` closure assigning `opts.completionGuard = guard`.

`docs/runtime-dissolution-inventory.yaml` is parent-owned and changes only after implementation proof.

Forbidden: completion-guard field/type/behavior deletion, replacement production helper, alias, exported test seam, tool-loop control-flow change, provider/model change, route/tool registration change, state-authority change, unrelated test cleanup, or package extraction.

## Acceptance

1. the only pre-delete caller is the one named same-package test and no residual symbol remains after deletion;
2. the exported wrapper/comment are deleted and the test uses direct same-package option setup;
3. focused completion-guard/tool-loop tests and default runtime compilation pass;
4. completion-guard behavior, tool-loop flow, providers, routes, tools, and state authorities remain unchanged;
5. ratchet production LOC, exports, and unused-export debt decrease with no gated growth;
6. independent verification, full CI, staging identity/product smoke, consensus, and adjudication pass.
