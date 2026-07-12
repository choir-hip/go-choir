# S3 Runtime Test-Only Helper Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I3-runtime-test-only-wrappers`
- Dispatch nonce: `s3-runtime-dissolution-i3-nonce-01`
- Transition: `s3-i3-dispatch-intent-98`
- Canonical parent: `5f981886`
- Mutation class: orange
- Rollback: revert the atomic implementation landing to `5f981886`; do not restore wrappers through aliases.
- Protected surfaces: none. No live Trace authority, run-lifecycle state transition, promotion/rollback, Wire, candidate computer, auth/session, vmctl, gateway/provider, or deployment-routing surface may change.
- Conjecture delta: four exported runtime conveniences have only test callers or no caller; direct canonical test setup/entry points can preserve meaningful behavior without retaining production wrappers.
- Heresy delta at dispatch: `discovered: four test-only/declaration-only runtime wrappers`; `introduced: none`; `repaired: pending`.

## Problem Record

The current inventory and symbol/reference graph identify four exported `runtime.go` helpers with no production caller:

- `WithToolRegistry` — one same-package tool-loop test caller;
- `(*Runtime).TraceStore` — one same-package trace wiring test caller;
- `(*Runtime).StartRun` — a metadata-free forwarding wrapper used by runtime tests; production uses the metadata-bearing canonical entry point. The similarly named actorruntime adapter method is a distinct API and is out of scope;
- `(*Runtime).CompactRunMemory` — one integration test calls this unregistered manual wrapper; automatic durable compaction is covered through the live run-memory path.

Tests must not keep otherwise unused production APIs alive. The meaningful tool-loop, trace wiring, run execution, activation-budget, and automatic run-memory compaction contracts remain required; only wrapper-specific assertions may disappear. This problem record precedes implementation.

## Exact Mutation Lock

Allowed production file: `internal/runtime/runtime.go`, deleting exactly the four named wrappers and attached comments.

Allowed focused test files only where existing callers require direct canonical setup/entry-point rewrites:

- `internal/runtime/toolloop_test.go`;
- `internal/runtime/trace_wiring_test.go`;
- `internal/runtime/config_test.go`;
- `internal/runtime/toolloopvalidation_test.go`;
- `internal/runtime/run_memory_integration_test.go`.

`internal/actorruntime/adapter_test.go` is inspection-only: its adapter `StartRun` API is distinct and must remain unchanged. `docs/runtime-dissolution-inventory.yaml` is parent-owned and changes only after implementation proof.

Forbidden: replacement production helper, alias, forwarding method, exported test seam, route/tool registration change, state-authority change, live execution-core move, Browser extraction, promotion/candidate mutation, unrelated cleanup, or deletion of meaningful observable behavior coverage.

## Acceptance

1. every pre-delete caller is either a distinct API or rewritten to a canonical entry point/direct same-package setup;
2. all four production symbols are absent with no alias or replacement;
3. focused runtime/tool-loop/trace/config/run-memory tests and default package tests pass;
4. registered routes, tool registrations, and state authorities remain unchanged;
5. ratchet exports, unused debt, and production LOC decrease with no gated growth;
6. independent verification, full CI, staging identity/product smoke, consensus, and adjudication pass.
