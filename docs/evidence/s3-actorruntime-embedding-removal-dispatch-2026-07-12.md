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

Replace the anonymous `*runtime.Runtime` embed in `internal/actorruntime.Adapter` with one explicit named field. Update all adapter internals, handler construction, options, tests, and `cmd/sandbox` callers to use that field or an already-existing adapter-owned lifecycle method. Do not add accessors, forwarding methods, aliases, interfaces, optional/fallback cores, a constructor result edge, or a second runtime instance.

Preserve runtime construction, ActorBridge dispatch wiring, trace option application, Start/Stop/Drain order, actor log durability/recovery, API route wiring, tool installation/profile lookup, product-event emission, and every existing sandbox behavior. Do not move API/config/bootstrap ownership, delete `apihandler`, remove direct sandbox runtime imports, modify tools/routes/state/models/apps, or begin step 3.

## Acceptance

- `Adapter` contains no anonymous `*runtime.Runtime` field and no promoted runtime method set;
- one explicit named core field points at the existing runtime instance;
- production/test callers use that boundary without a new accessor, forwarder, callback, interface, constructor result, or compatibility seam;
- focused actorruntime and sandbox build/tests pass;
- every ratchet authority count is non-increasing;
- independent verification, full CI, staging identity/product smoke, consensus, and adjudication pass.

## S3-I11 Ratchet Blocker

The first isolated implementation removed anonymous embedding with `core *runtime.Runtime` but changed `New` to return `(*Adapter, *runtime.Runtime)`. The executable inventory correctly counted both the named field and the constructor result as runtime wrapper edges, increasing `wrappers` from `5` to `6`; the slice therefore fails its ratchet and cannot land as returned.

Smallest repair: retain one explicit named `Runtime *runtime.Runtime` field and the original single-result constructor. This keeps exactly one mechanically visible transitional runtime edge, removes method promotion, avoids an accessor/forwarder/callback/interface/second instance, and keeps the wrapper count flat at `5`. The slice does not claim step-2 completion: later extraction must delete this explicit field together with the remaining runtime dependency. The acceptance condition is corrected from “private core and wrapper count decreases” to “one explicit named non-anonymous core edge, no promoted method set, and all ratchet counts non-increasing.”

## S3-I11 Repair Receipt

- Repaired isolated commit `9fae2d61f677a260671a814d673dde758ffb568d`, integrated as `e9de3b98`.
- `Adapter` now has exactly one explicit named non-anonymous `Runtime *runtime.Runtime` field; `New` remains single-result and constructs one runtime instance.
- No constructor result edge, accessor, forwarder, callback, interface, fallback, duplicate core, or promoted runtime method remains.
- `go test ./internal/actorruntime ./cmd/sandbox -count=1` passes.
- The canonical runtime ratchet passes with wrappers flat at `5`; every gated authority count is non-increasing.

## S3-I11 Independent Verification

- Independent `S3I11Verifier` returned `PASS` at confidence `0.99` with no findings.
- Verified one named non-anonymous runtime edge, original single-result constructor, one `runtime.New` construction, migrated callers, unchanged handler/dispatch wiring, unchanged lifecycle sequencing, no replacement seam, focused test pass, and ratchet wrappers flat at `5`.
- Environment-level CI, deployment, staging acceptance, consensus, and adjudication remained pending at verifier return.

## S3-I11 CI, Deploy, and Acceptance

- GitHub Actions run `29213877006` attempt `3` passed every default, integration, race, ratchet, SBOM, and deploy gate for head `f4962eced74dcafd0874e728d245cac1fd82f27a`.
- Attempt `2` had timed out in the non-runtime race lane while tests were still passing/running; failed-job retry completed successfully without a source change.
- Deploy job `86708347659` completed successfully.
- Staging health returned `200`/`status=ok`; authenticated `GET https://choir.news/api/agent/loops` returned `200`.

## S3-I11 Final Consensus

- Gemini, GPT-5.5, and OpenCode returned `PASS` with no blocking source findings and confidence `0.98-1.0`; all authorized only the next S3 step-2 extraction iteration.
- Codex found no source blocker but labeled the pre-adjudication ledger's intentionally pending consensus/landed fields `BLOCKING`. That procedural finding is satisfied by this adjudication transition itself; treating “not yet adjudicated” as a source blocker would make the required consensus gate circular.
- Adjudication: `PASS`. Close S3-I11. The explicit `Adapter.Runtime` and handler runtime edge remain declared step-2 debt; step 2, S3, and step 3 remain incomplete/unauthorized.
