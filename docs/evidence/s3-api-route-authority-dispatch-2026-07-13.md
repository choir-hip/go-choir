# S3 API Route Authority Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I16-api-route-authority`
- Dispatch nonce: `s3-runtime-dissolution-i16-nonce-01`
- Transition: `s3-i16-dispatch-intent-243`
- Canonical parent: `560f0942`
- Mutation class: orange
- Rollback ref: `560f0942`

## Problem and Boundary Map

S3-I15 deleted the `internal/apihandler` wrapper and moved complete process composition to `internal/sandbox`, but `internal/runtime` still owns the live HTTP route table through `RegisterRoutes`, `RegisterTextureRoutes`, and `RegisterCandidatePackageReviewSurfaceRoutes`. This is the first concrete step-3 authority residue: transport ownership remains in the god package even though bootstrap and the product API tool already have canonical external owners.

The smallest deletion-bearing extraction is route-table ownership only. Move the exact registration table to the existing `internal/apihandler` package, keep `runtime.APIHandler` methods and business behavior unchanged, delete all three runtime registrar declarations, and switch the sole production caller in `internal/sandbox/run.go` to the new canonical registrar. Inline the candidate and Texture registrations into that one table; do not create sub-registrars or dual route authority. Pass the already available `provideriface.Config.EnableTestAPIs` boolean from sandbox rather than adding a runtime config accessor.

A compile-proven risk is same-package runtime tests: `internal/apihandler` may import runtime for the handler type, while runtime tests currently call `RegisterRoutes`. The implementation must not solve this with a test-only duplicate registrar, interface, callback table, reflection, alias, wrapper, or forwarding function. It may migrate exact affected route integration tests/helpers to an external-package or apihandler-owned test surface only when that preserves their behavioral contract. If no clean acyclic migration exists within the exact slice, return `BLOCKING` with the dependency proof before editing rather than normalizing duplicate authority.

## Exact Mutation Lock

Allowed production files:

- `internal/apihandler/routes.go` (create; sole canonical route registrar)
- `internal/runtime/api.go` (delete registrar declarations only)
- `internal/runtime/api_candidate_package_intake.go` (delete registrar declaration only)
- `internal/sandbox/run.go` (one caller cutover and existing config boolean)

Allowed tests are only direct route-registration coverage that must move or change to compile after the production cutover. `docs/runtime-dissolution-inventory.yaml` may change only after focused proof.

Forbidden: any `APIHandler` method/body move; app/domain/route behavior or path change; runtime method/config/provider/store/trace/actor/lifecycle change; new package; interface; callback/function table; generic constraint facade; reflection; alias; accessor; forwarder; wrapper; test-only registrar copy; compatibility shim; dual registration path; second server/mux; step-4 behavior.

## Acceptance

- Exactly one production registrar exists in `internal/apihandler`; runtime registrar declarations and callers are zero.
- Exact health override, all public/internal/test-gated paths, route order, method dispatch, and Texture prefix behavior are unchanged.
- Test routes remain gated by the existing sandbox-loaded `EnableTestAPIs` value with no new runtime accessor.
- Same canonical `server.Server` remains bound to the product API tool.
- Focused apihandler/runtime/sandbox tests and runtime ratchet pass; live route slots and tools are flat, while runtime-scoped routes, production LOC, exports, and caller edges decrease. Every other authority count is non-increasing except classified durable citers and the exact transitional `internal/apihandler/routes.go` production-importer/wrapper heuristic entries required for the canonical owner to name the not-yet-extracted `runtime.APIHandler`; those entries remain explicit deletion debt and do not weaken the final zero-importer/zero-wrapper extinction gate.
- Independent verification, full CI, deploy identity, authenticated public product-path smoke, consensus, and adjudication pass before closure.

## S3-I16 Implementation Receipt

- Integrated isolated commit `1794f26cfa32390d205ebaf29b3f565556fd7030` as canonical `3b10893c`.
- `internal/apihandler/routes.go` now owns the sole live registrar. Runtime `RegisterRoutes`, `RegisterTextureRoutes`, and `RegisterCandidatePackageReviewSurfaceRoutes` declarations/calls are zero; sandbox has the sole production call and passes its already loaded test-API boolean.
- A prospective overlay reproduced the predicted same-package test import cycle. The repair moved registrar ownership coverage to `internal/apihandler/routes_test.go`; runtime behavior tests now invoke concrete handler methods without importing apihandler or duplicating a route table. The obsolete spawn route-absence file was consolidated into the canonical apihandler inventory.
- Programmatic before/after comparison expanded old subregistrars and proved `46` slots in exact equal order: one health override plus `45` routes. Focused inventory and test-gate coverage exercises all slots, exclusions, and test APIs both disabled and enabled.
- Focused apihandler/runtime/sandbox behavior tests and the runtime ratchet pass after canonical integration. The same server is passed to the route registrar and product API tool.
- Ratchet: Go files `144 -> 143`, test files `69 -> 68`, production LOC `44024 -> 43944`, test LOC `50141 -> 50065`, exports `1012 -> 1006`, caller edges `365 -> 363`, runtime-scoped routes `47 -> 2`; tools, unused-export debt, compatibility markers, store calls, and interface candidates are flat. Citers increased only from classified dispatch evidence (`245 -> 249`).
- Independent verifier adjudication: production importers `3 -> 4` and wrapper heuristic `3 -> 4` are truthful necessary transitional dependencies because the new canonical apihandler registrar must name the not-yet-extracted `runtime.APIHandler`. No forwarding wrapper, interface, callback, accessor, or compatibility seam exists. The slice-local overstrict sentence is amended to permit exactly `internal/apihandler/routes.go`; both inventory entries remain explicit `delete` debt, global no-growth remains binding, and final runtime extinction still requires zero production importers and wrappers.

## S3-I16 CI Attempt 2 Timeout

- GitHub Actions run `29232081536` attempt `2` passed every completed default, integration, runtime-race, docs, and heresy lane, but the non-runtime race lane `86758741778` exhausted its command timeout while `internal/server.TestHealthHandlerIncludesAddrAfterStart` was still running.
- The log reports no race detector finding or failed test; the lane summary classifies every completed test `pass` and explicitly reports the one still-running server test at timeout.
- This is not attributed to the route-authority change. A failed-lane-only retry is running; no source fix is authorized unless the retry produces a reproducible change-attributable failure.

## S3-I16 CI, Deploy, and Acceptance

- Attempt `3` was cancelled by the subsequent documentation checkpoint under main-branch concurrency, not by a test result.
- GitHub Actions run `29232081536` attempt `4` passed every default, integration, race, ratchet, SBOM, and deploy gate for head `3b10893c13a9d79b7ab4219dc6b9377c6d0ed1fd`.
- The previously timed-out non-runtime race lane passed in `11m1s` on attempt `4`.
- Deploy job `86763987981` completed successfully.
- Authenticated public `GET https://choir.news/api/texture/documents` returned `200`.
