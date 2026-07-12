# S3 Runtime Test-Only Helper Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I3-runtime-test-only-wrappers`
- Dispatch nonce: `s3-runtime-dissolution-i3-nonce-01`
- Transition: `s3-i3-dispatch-intent-98`
- Canonical parent: `5f981886`
- Mutation class: orange
- Rollback: revert the atomic implementation landing to `5f981886`; do not restore wrappers through aliases.
- Protected surfaces: none. No live Trace authority, run-lifecycle state transition, promotion/rollback, Wire, candidate computer, auth/session, vmctl, gateway/provider, or deployment-routing surface may change.
- Conjecture delta: three exported runtime conveniences have only test callers; direct same-package test setup can preserve meaningful behavior without retaining production wrappers.
- Heresy delta at dispatch: `discovered: three in-scope test-only runtime wrappers plus one wider-caller wrapper deferred`; `introduced: none`; `repaired: pending`.

## Problem Record

The current inventory and symbol/reference graph identify three in-scope exported `runtime.go` helpers with no production caller:

- `WithToolRegistry` — one same-package tool-loop test caller;
- `(*Runtime).TraceStore` — one same-package trace wiring test caller;
- `(*Runtime).CompactRunMemory` — one integration test calls this unregistered manual wrapper; automatic durable compaction is covered through the live run-memory path.

The initial probe also classified `(*Runtime).StartRun` as test-only, but implementation reconciliation found more than `48` runtime-test callers across files outside the original mutation lock. It is deferred to its own caller-complete slice; this iteration must not delete it or change any caller. Tests must not keep otherwise unused production APIs alive, but caller discovery must precede scope. The meaningful tool-loop, trace wiring, and automatic run-memory compaction contracts remain required; only wrapper-specific assertions may disappear.

## Exact Mutation Lock

Allowed production file: `internal/runtime/runtime.go`, deleting exactly the three in-scope wrappers and attached comments. `(*Runtime).StartRun` is forbidden in this iteration.

Allowed focused test files only where existing callers require direct canonical setup/entry-point rewrites:

- `internal/runtime/toolloop_test.go`;
- `internal/runtime/trace_wiring_test.go`;
- `internal/runtime/run_memory_integration_test.go`.

`internal/actorruntime/adapter_test.go` and all `StartRun` callers are inspection-only and must remain unchanged. `docs/runtime-dissolution-inventory.yaml` is parent-owned and changes only after implementation proof.

Forbidden: replacement production helper, alias, forwarding method, exported test seam, route/tool registration change, state-authority change, live execution-core move, Browser extraction, promotion/candidate mutation, unrelated cleanup, or deletion of meaningful observable behavior coverage.

## Acceptance

1. every in-scope pre-delete caller is rewritten to direct same-package setup or the wrapper-only assertion is removed without losing automatic behavior coverage;
2. all three in-scope production symbols are absent with no alias or replacement, while `StartRun` and every caller remain unchanged;
3. focused runtime/tool-loop/trace/run-memory tests and default package tests pass;
4. registered routes, tool registrations, and state authorities remain unchanged;
5. ratchet exports, unused debt, and production LOC decrease with no gated growth;
6. independent verification, full CI, staging identity/product smoke, consensus, and adjudication pass.
