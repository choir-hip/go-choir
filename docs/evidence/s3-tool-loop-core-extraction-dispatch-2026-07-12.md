# S3 Tool-Loop Core Extraction Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I9-tool-loop-core-extraction`
- Dispatch nonce: `s3-runtime-dissolution-i9-nonce-01`
- Transition: `s3-i9-dispatch-intent-165`
- Canonical parent: `a28b590a`
- Mutation class: orange
- Rollback: revert the isolated extraction commit, then `a28b590a`

## Problem Record

S3 step 1 is complete, but the live provider/tool execution loop remains owned by `internal/runtime/toolloop.go`. This prevents `internal/runtime` extinction and leaves `actorruntime.Adapter` dependent on an embedded `*runtime.Runtime`. The current `internal/toolregistry` package already owns `Tool`, `ToolFunction`, `ToolRegistry`, and their canonical constructors, while `runtime/toolloop.go` consumes those types through aliases. The dependency graph therefore supports `internal/toolregistry` as the smallest existing package for the storage-independent tool-loop engine; creating `internal/agentcore` is not justified for this leaf extraction.

This is a substrate problem, not an app symptom. Patching another runtime compatibility alias would extend the superseded package instead of extracting authority.

## Exact Mutation Lock

## Fresh Caller-Graph Boundary Correction

The first mechanical extraction probe found that `RunToolLoop` directly calls
runtime-private `executeTools`. That batch executor contains app/profile policy:
planned duplicate-side-effect suppression, Texture single-write enforcement,
sequential versus parallel ordering, output projection/capping, and event
payload construction. Moving that policy into `toolregistry` would silently
move app authority and violate the original mutation lock; keeping the direct
call is impossible without an import cycle.

S3 explicitly allows boundaries to adjust from fresh caller evidence. The
correct boundary is dependency inversion: `toolregistry` owns the
storage-independent provider/tool-loop state machine and a narrow batch-executor
function contract; runtime temporarily owns and supplies the existing
app/profile-aware batch executor unchanged. This is not a compatibility facade:
there is no old `runtime.RunToolLoop`, no forwarder, and no alternate execution
path. A later step-2 slice moves or dissolves the remaining executor policy
before runtime embedding is removed.

The executor contract may carry only the existing `(context, registry, calls,
event emitter) -> ordered results` behavior. It must not expose runtime types,
storage, routes, state, models, profiles, or app policy, and it must not add a
fallback path. `RunToolLoop` must require exactly one executor when a registry
and tool calls are present; tests may use the authoritative generic executor
provided by `toolregistry` only where the old behavior was already generic.

Extract the complete storage-independent tool-loop engine from `internal/runtime/toolloop.go` into `internal/toolregistry`, including its behavioral tests and private helpers. Migrate every production and test caller to the authoritative package. Delete the old runtime declarations rather than aliasing or forwarding them.

Allowed source scope:

- `internal/runtime/toolloop.go` and directly affected runtime callers/tests;
- `internal/toolregistry/**`;
- `internal/provider/**` only for direct tool-loop call/type migration;
- `docs/runtime-dissolution-inventory.yaml` after source behavior passes.

Forbidden:

- replacement runtime aliases, wrappers, facades, or test seams;
- changes to provider request semantics, tool execution order, retry/budget/park behavior, event payloads, registrations, routes, state, models, or app tools;
- `actorruntime` embedding removal in this slice (that follows after the leaf dependency is extracted);
- unrelated cleanup.

## Acceptance

- `internal/runtime/toolloop.go` is deleted or contains no tool-loop engine declarations;
- authoritative tool-loop API and implementation live only in `internal/toolregistry`;
- all build-tag caller paths compile with no runtime alias or replacement seam;
- focused tool-loop behavior tests pass, including boundaries, terminal transitions, required-next-tool protocol, provider fallback, budgets, memory hooks, and park/passivation;
- production LOC and runtime-owned symbol debt decrease without growth in routes, tools, production importers, wrappers, compatibility markers, store calls, interface candidates, or caller edges;
- independent verification, full CI, staging identity/product smoke, consensus, and adjudication pass.

## S3-I9 Implementation Receipt

- Integrated implementation: `820fa74d` (isolated commit `57b782894b1f148a776d57a9fc0fa3e0bc9f3782`).
- `internal/runtime/toolloop.go` moved to authoritative `internal/toolregistry/toolloop.go`; behavioral coverage moved to `internal/toolregistry/toolloop_test.go`.
- `internal/toolregistry.ToolBatchExecutorFunc` is the required acyclic execution-policy boundary. Runtime supplies its unchanged `executeTools`; generic provider/gateway callers supply `toolregistry.ExecuteToolBatch`. No fallback executor or runtime compatibility surface exists.
- Focused `internal/toolregistry`, `internal/provider`, `internal/gatewayruntime`, runtime tool-loop/executor/memory tests, and integration-tag provider compilation passed on canonical integration.
- Residual runtime-qualified tool-loop surface search returned empty.
- Ratchet passed: Go files `148 -> 147`, production files `78 -> 77`, production LOC `46931 -> 45272`, test LOC `53044 -> 50232`, exports `1140 -> 1062`, caller edges `601 -> 559`, compatibility markers `15 -> 13`, and initial unused-export debt `22 -> 19`; every other gated authority count remained flat.
- The remaining app/profile-aware batch execution policy is explicit later step-2 work, not hidden completion of runtime dissolution.

## S3-I9 Independent Verification Blockers

- Independent `S3I9Verifier` returned `BLOCKING` at confidence `0.99`.
- Extraction-induced comprehensive-tag compile failures prove three runtime test helpers were deleted with the migrated unit suite while comprehensive runtime tests still consume them: `rawMessagesForTest`, `extractLastUserMessage`, and `extractTextFromContent`.
- The post-receipt inventory is also stale by one historical-evidence citer (`226 -> 227`).
- Smallest repair: retain the three helpers as test-only runtime integration helpers, regenerate the final inventory, then rerun comprehensive compilation, focused tests, ratchet, and independent verification. No production correction or behavior change is required.
