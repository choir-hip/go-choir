# S3 Actor Runtime Embedding Removal Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I11-actorruntime-embedding-removal`
- Dispatch nonce: `s3-runtime-dissolution-i11-nonce-01`
- Transition: `s3-i11-dispatch-intent-193`
- Canonical parent: `ccc5c91a`
- Mutation class: orange
- Rollback: `ccc5c91a`

## Problem Record

S3-I9 and S3-I10 extracted the storage-independent tool-loop state machine, batch executor, and typed execution context into `internal/toolregistry`, but `internal/actorruntime.Adapter` still anonymously embeds `*runtime.Runtime`. That embedding promotes every runtime method onto the production adapter, hides the actual remaining caller boundary, and preserves the exact wrapper topology forbidden by S3 step 2. The production sandbox currently uses only explicit runtime capabilities plus `Start`; the anonymous promotion envelope is broader than the live caller contract.

This is substrate boundary debt, not an app symptom. Adding forwarding methods would recreate the wrapper surface; removing `runtime` wholesale would cross into ordered S3 steps 3-6. The smallest clean step-2 cut is to make the transitional runtime core a named, non-embedded field and migrate every caller to that field directly, leaving `Adapter` ownership only for actor lifecycle and dispatch.

## Exact Mutation Lock

Replace the anonymous `*runtime.Runtime` embed in `internal/actorruntime.Adapter` with one named private field. Update all adapter internals, handler construction, options, tests, and `cmd/sandbox` callers to use that field or an already-existing adapter-owned lifecycle method. Do not add accessors, forwarding methods, aliases, interfaces, optional/fallback cores, or a second runtime instance.

Preserve runtime construction, ActorBridge dispatch wiring, trace option application, Start/Stop/Drain order, actor log durability/recovery, API route wiring, tool installation/profile lookup, product-event emission, and every existing sandbox behavior. Do not move API/config/bootstrap ownership, delete `apihandler`, remove direct sandbox runtime imports, modify tools/routes/state/models/apps, or begin step 3.

## Acceptance

- `Adapter` contains no anonymous `*runtime.Runtime` field and no promoted runtime method set;
- one named private core points at the existing runtime instance;
- production/test callers use the explicit core boundary without new wrappers or compatibility seams;
- focused actorruntime and sandbox build/tests pass;
- ratchet wrapper count decreases and every other gated authority count is non-increasing;
- independent verification, full CI, staging identity/product smoke, consensus, and adjudication pass.
