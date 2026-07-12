# S3 Batch Executor Extraction Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I10-batch-executor-extraction`
- Dispatch nonce: `s3-runtime-dissolution-i10-nonce-01`
- Transition: `s3-i10-dispatch-intent-181`
- Canonical parent: `5b532da2`
- Mutation class: orange
- Rollback: revert the isolated extraction commit, then `5b532da2`

## Problem Record

S3-I9 moved the storage-independent provider/tool-loop state machine into `internal/toolregistry`, but `internal/runtime/tools.go` still owns the required batch executor supplied to that loop. The remaining executor is live execution core: invocation/result events, ordered results, sequential-versus-parallel policy, duplicate-side-effect suppression, Texture single-write enforcement, output projection/capping, and app/profile-aware skip planning. This explicit deferred finding prevents completion of S3 step 2 and keeps execution authority split across packages.

The first I9 extraction probe proved the policy depends on runtime-private profile context. Existing `internal/agentprofile` already owns canonical profile identifiers, and `internal/toolregistry` now owns the loop and executor contract. The smallest acyclic boundary is therefore: toolregistry owns typed tool-execution context plus the batch executor; runtime computes run-derived values once and installs that context. No second package or `internal/agentcore` is justified.

This is a substrate extraction, not an app symptom. Leaving a callback into runtime would preserve the split authority; copying the policy would create two execution paths.

## Exact Mutation Lock

Move the complete live batch execution policy from `internal/runtime/tools.go` into `internal/toolregistry`. Move the tool-execution context values needed by that policy to one typed context object owned by toolregistry, using `internal/agentprofile` identifiers. Migrate runtime context producers and consumers directly to the authoritative object. Delete dead helpers proved callerless and delete old runtime executor/context declarations; do not alias or forward them.

Allowed source scope:

- `internal/runtime/tools.go`, `tool_profiles.go`, and direct executor/context consumers/tests;
- `internal/toolregistry/**`;
- `internal/agentprofile/**` only if a missing canonical identifier is mechanically required;
- `docs/runtime-dissolution-inventory.yaml` after behavior passes.

Forbidden:

- changes to tool registration or tool implementations;
- changes to profile derivation, execution order, skip rules, Texture write semantics, event payloads, output projection/capping, provider/loop behavior, routes, state, models, or app behavior;
- runtime aliases, forwarding wrappers, optional/fallback executors, dual context keys, compatibility seams, or a second batch path;
- unrelated cleanup.

## Acceptance

- toolregistry solely owns `ExecuteToolBatch`, its full policy, and its typed execution context;
- production loop callers use the authoritative executor directly; no runtime executor callback remains;
- no old runtime batch/context declaration, alias, forwarder, duplicate key, or replacement seam remains;
- focused executor/context behavior tests preserve parallel and sequential order, every skip rule, Texture single-write behavior, projections/caps, events, ordered results, and error semantics;
- all-source/build-tag caller paths compile;
- ratchet decreases production LOC/runtime-owned symbols without growth in routes, tools, production importers, wrappers, compatibility markers, store calls, interface candidates, or caller edges;
- independent verification, full CI, staging identity/product smoke, consensus, and adjudication pass.

## S3-I10 Implementation Receipt

- Integrated implementation: `23b65818` (isolated commit `6e9b7267e1defe0a799a8856570c90b6c066a106`).
- Toolregistry now owns the complete batch execution policy and sole typed `ExecutionContext`; runtime computes run-derived values once and installs that object.
- Production `RunToolLoop` and supervisor-recovery callers use `toolregistry.ExecuteToolBatch` directly. Old runtime executor functions, per-field context keys/accessors, callback path, no-op transition hook, and callerless hidden delegation helpers are deleted.
- Focused owner executor/context, runtime integration, provider, gateway/gatewayruntime, and integration-tag provider checks pass. Comprehensive-tag runtime compilation reproduces only the pre-existing `prompts_test.go`/`texture_test.go` drift.
- Residual runtime executor/context seam searches return no old declarations, aliases, forwarders, duplicate keys, or replacement paths.
- Ratchet passed: production LOC `45272 -> 44681`, exports `1062 -> 1061`, and caller edges `559 -> 549`; routes, tools, production importers, wrappers, compatibility markers, store calls, interface candidates, and initial unused-export debt remain flat.

## S3-I10 Independent Verification Blocker

- Independent `S3I10Verifier` returned `BLOCKING` at confidence `0.99`.
- Although every caller now supplies the same authoritative executor, `RunToolLoop` still exposes `ToolBatchExecutorFunc` as an arbitrary callback parameter. That public injection seam violates sole executor ownership and permits a second execution policy.
- Smallest repair: delete `ToolBatchExecutorFunc`, remove the parameter from `RunToolLoop`, call `ExecuteToolBatch` directly inside the loop, and update every caller mechanically. No policy or behavior change is required.
