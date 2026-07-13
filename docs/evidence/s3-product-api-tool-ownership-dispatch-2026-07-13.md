# S3 Product API Tool Ownership Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I14-product-api-tool-ownership`
- Dispatch nonce: `s3-runtime-dissolution-i14-nonce-01`
- Transition: `s3-i14-dispatch-intent-225`
- Canonical parent: `9ee58796`
- Mutation class: orange
- Rollback: `9ee58796`

## Problem Record

Fresh caller mapping found a production dependency-cycle blocker beneath the false `apihandler` wrapper. The Super-only `product_api_request` tool is implemented in `internal/runtime/tools_product_api.go`; every tool call constructs a private `server.Server`, constructs the runtime `APIHandler`, and registers the complete route table again. Runtime therefore remains a production caller of its own HTTP constructor and registrar. Moving HTTP ownership to `internal/apihandler` now would create the illegal cycle `apihandler -> runtime -> apihandler`, while deleting or renaming only the current wrapper would not reduce authority.

The duplicate per-call route table is also false product authority. The sandbox already owns one production `server.Server` with the canonical route table. The product tool must execute against that server rather than constructing a second mux whose route set can drift.

This is a substrate prerequisite, not S3 step-3 completion. The wrapper remains until a later atomic HTTP-ownership extraction; step 4 remains unauthorized.

## Boundary Review

A four-runner architecture panel completed. Codex identified the hidden production caller and recommended removing it as the smallest deletion-bearing prerequisite at confidence `0.92`. Gemini recommended moving the larger sandbox bootstrap into the existing sandbox package; OpenCode recommended an actor-adapter registrar. Those alternatives either miss the runtime-owned duplicate route constructor or relocate the false seam. The orchestrator adjudicates the hidden production caller as the first dependency because deleting it makes the later ownership graph acyclic without adding a facade.

## Exact Mutation Lock

- Move the complete `product_api_request` tool implementation and behavioral tests from runtime to `internal/apihandler`.
- Bind the tool to the already constructed canonical `*server.Server`; it must not construct a server, mux, handler, or route table.
- Register the tool exactly once into the existing Super registry from `cmd/sandbox/main.go` after default tool installation and canonical route registration.
- Remove runtime's default-tool registration of `newProductAPIRequestTool`.
- Delete `internal/runtime/tools_product_api.go` and `internal/runtime/tools_product_api_test.go` after all behavior assertions move.
- Use canonical `toolregistry.Tool` directly. Preserve name, description, JSON schema, method/path normalization, product-route allowlist, execution-context/profile/owner enforcement, auth headers, JSON content type, response body cap, truncation marker, status/error result shape, and tool-disabled behavior.
- Preserve one route set and all API/domain behavior. Do not alter `runtime.APIHandler`, the current `apihandler.Handler`, routes, app/domain code, actor lifecycle, provider selection, state, models, or step 4.
- Do not add aliases, interfaces, callbacks, forwarders, accessors, fallback registrations, a second server/mux, or a new package.
- Update `docs/runtime-dissolution-inventory.yaml` only after focused behavior passes and classify this durable dispatch citer.

## Acceptance

- Runtime has no production call to `NewAPIHandler` or `RegisterRoutes` outside their declarations/registration implementation.
- `product_api_request` uses the canonical server passed at startup; no per-call mux exists.
- Allowed owner-scoped public Texture request succeeds with authenticated owner identity.
- Internal, test, agent, prompt-config, raw-event, non-Super, missing-owner, invalid-method/path, oversized-body, and oversized-response cases retain exact rejection/truncation behavior.
- Disabled tools leave the product API tool absent; enabled tools register it once, and duplicate registration fails explicitly.
- Focused `internal/apihandler`, runtime, sandbox, and actor-runtime tests pass.
- Runtime production LOC, API constructor caller edges, and runtime-scoped tool declarations decrease; routes and the enabled product tool catalog remain flat; wrappers do not increase; every other gated authority count is non-increasing except classified durable citers.
- Independent verification, full CI, deploy identity, authenticated public product-path smoke, consensus, and adjudication pass before closure.

## S3-I14 Implementation Receipt

- Integrated isolated commit `d72d86a93576fdc10e757b1986907dd3940c4665` as canonical `ca9b3142`.
- `internal/apihandler` now owns `product_api_request` as a canonical `toolregistry.Tool` bound to the already constructed production server. No per-call server, mux, handler, route registrar, callback, interface, accessor, alias, or fallback remains.
- Sandbox installs default registries, registers the one canonical route table, then registers the server-bound tool exactly once in the existing Super registry only when tools are enabled.
- Runtime's product tool implementation/test files and default registration are deleted. The runtime default-catalog regression now asserts that server-bound transport is absent; apihandler tests cover canonical-server identity, owner/email headers, schema, nil/duplicate registration, allowlist/rejections, size limits, truncation, and result shape.
- Focused tests passed for apihandler, runtime, sandbox, and actor-runtime. The runtime ratchet passes.
- Ratchet deltas: Go files `146 -> 144`, production files `76 -> 75`, test files `70 -> 69`, production LOC `44216 -> 44032`, test LOC `50223 -> 50142`, exports `1014 -> 1012`, caller edges `372 -> 368`, runtime-scoped tool declarations `49 -> 48`; routes, production importers, wrappers, compatibility markers, store calls, interface candidates, and initial unused-export debt are flat. The enabled product catalog remains behaviorally flat because sandbox registers the same tool after runtime's duplicate constructor-backed registration is deleted. Three durable dispatch/suite citers are classified (`241 -> 244`).

## S3-I14 Independent Verification

- Independent `S3I14Verifier` returned `PASS` at confidence `0.99` with no remaining source, test, inventory, ownership, registration, schema, identity, size, result-shape, or error-parity blocker.
- The verifier initially read the pre-evidence source commit's stale “tools flat” sentence, then re-read canonical `1453405e`, withdrew the finding, and confirmed the canonical runtime-scoped `49 -> 48` versus enabled-catalog-flat distinction.

## S3-I14 CI, Deploy, and Acceptance

- GitHub Actions run `29225212648` attempt `2` passed every lane except the non-runtime race job, which reached its job deadline while tests were still running and reported no test failure. Attempt `3` retried the failed lane and passed every default, integration, race, ratchet, SBOM, and deploy gate for head `ca9b314254bd5fb92333ceffe7daee8831f364ad`.
- Deploy job `86740654409` completed successfully.
- Staging health returned `200`/`status=ok`; authenticated public `GET https://choir.news/api/texture/documents` returned `200`.
