# S3 Sandbox Runtime Import Cutover Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I13-sandbox-runtime-import-cutover`
- Dispatch nonce: `s3-runtime-dissolution-i13-nonce-01`
- Transition: `s3-i13-dispatch-intent-210`
- Canonical parent: `6ee86dd1`
- Mutation class: orange
- Rollback: `6ee86dd1`

## Problem Record

S3-I12 moved config authority to `provideriface`, but `cmd/sandbox/main.go` still directly imports `internal/runtime`, violating the explicit S3 step-3 cutover criterion. The remaining named uses are mechanically narrow: stub provider construction, agent-profile identifiers, model-policy path derivation, and the `ToolRegistry` type. Profile and registry authority already live in `agentprofile` and `toolregistry`; config/bootstrap path authority now lives in `provideriface`. Stub construction is the only behavior-bearing runtime dependency and must move to its real provider/bootstrap owner without losing its conductor-to-Texture decision behavior.

This is bootstrap/provider substrate debt, not an API or app-domain cutover. Adding an actorruntime/sandbox forwarder would hide the import rather than remove authority. The smallest atomic cut is to move the live stub provider and its exact behavior-supporting pure helpers to the smallest existing acyclic provider owner justified by callers, move model-policy path derivation to the config owner, migrate profile/registry uses to their existing owners, and delete every direct `cmd/sandbox -> internal/runtime` symbol edge.

## Exact Mutation Lock

Remove the direct `internal/runtime` import from `cmd/sandbox/main.go` and its tests. Replace runtime profile and registry compatibility symbols with `agentprofile` and `toolregistry`. Move `DefaultModelPolicyPath` to `provideriface` beside config ownership and migrate every caller, deleting the runtime declaration. Move `StubProvider`/`NewStubProvider` to the smallest existing provider package that can own the complete current behavior without importing runtime; move only mechanically required pure helpers and tests, migrate every caller, and delete runtime's declarations. Do not add aliases, forwarders, wrappers, callbacks, duplicate stub paths, fallback constructors, or a new package.

Preserve stub delay/cancellation/progress/error/result/policy behavior, conductor-to-Texture decision payload/title/seed semantics, gateway-vs-stub selection, tool profile counts/installation, model policy paths, runtime construction, API/routes, `apihandler`, state/models/apps, provider routing semantics, and lifecycle ordering. Do not begin API handler movement or app/domain step 4.

## Acceptance

- `cmd/sandbox` has no direct `internal/runtime` import or runtime-qualified symbol;
- agent profiles and ToolRegistry come directly from canonical owners;
- provideriface solely owns model-policy path derivation;
- one non-runtime package solely owns the complete stub provider behavior and runtime declarations are deleted;
- no alias/forwarder/wrapper/callback/duplicate/fallback/replacement import path remains;
- focused stub, provider policy, sandbox bootstrap, runtime, actorruntime, gatewayruntime, and build-tag caller behavior passes;
- runtime production LOC/exports/caller edges/production importers decrease and every gated authority count is non-increasing;
- independent verification, full CI, staging identity/product smoke, consensus, and adjudication pass.
