# S3 Atomic API Handler Ownership Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I18-atomic-api-handler-ownership`
- Dispatch nonce: `s3-runtime-dissolution-i18-nonce-01`
- Transition: `s3-i18-dispatch-intent-257`
- Canonical parent: `e20a7e01`
- Mutation class: red
- Rollback ref: `e20a7e01`

## Conjecture Delta And Protected Surfaces

**Conjecture:** the existing `internal/apihandler` owner can own the one live `APIHandler` type, constructor, HTTP-only DTOs/helpers, and every receiver method in one atomic cutover while `internal/runtime` retains business operations behind cohesive calls. This advances S3 step 3 without changing step-4 domain authority.

**Discovered heresy:** route registration moved in S3-I16, but `runtime.APIHandler` still makes the god package the implementation owner of HTTP transport; apihandler's registrar names that foreign type as explicit importer/wrapper heuristic debt. S3-I17 proved that pre-deleting dormant candidate mutation receivers strands eleven real Runtime domain exports and violates unused-export-debt authority.

**Introduced heresy budget:** zero aliases, wrappers, interfaces, callback/function tables, generic facades, accessors, forwarders, dual handler types, dual route paths, duplicate registrars, or temporary compatibility paths. One concrete `*runtime.Runtime` dependency inside the real apihandler handler may remain explicit step-4 deletion debt; it replaces the foreign `runtime.APIHandler` dependency rather than adding a second seam.

**Repaired heresy target:** zero `runtime.APIHandler`, `runtime.NewAPIHandler`, or APIHandler receiver methods in `internal/runtime`; one canonical apihandler type and constructor; all live routes retain exact behavior.

Protected surfaces carried unchanged through the move: authentication and owner scoping; internal caller authorization; Texture canonical reads/writes/revisions/merges/proposals; run lifecycle, cancellation, acceptance, trajectories, and evidence; candidate-package/promotion/adoption review and switch/rollback/roll-forward behavior; model/provider policy resolution; browser sessions; WebSocket/event delivery; persistent storage; runtime health; product API tool routing. No protected semantic mutation is authorized.

Admissible evidence class: exact source/caller/dependency proof; focused behavioral tests for every moved surface; runtime ratchet; independent red-slice verification by a distinct agent; full CI including integration/race/SBOM; staging deploy identity; authenticated public read and controlled write/read product-path acceptance; final multi-reviewer consensus and adjudication.

## Atomic Boundary

Move the `APIHandler` type and `NewAPIHandler` constructor plus all `128` receiver methods from these `20` runtime files into existing `internal/apihandler`, splitting mixed files as necessary:

- `api.go`
- `api_app_promotion.go`
- `api_candidate_package_intake.go`
- `api_costs.go`
- `api_texture_prompt_eval.go`
- `api_trajectory.go`
- `browser.go`
- `content.go`
- `desktop.go`
- `live_ws.go`
- `media_state.go`
- `podcast.go`
- `prompts.go`
- `runtime_refresh.go`
- `texture.go`
- `texture_agent_revision.go`
- `texture_import.go`
- `texture_lineage.go`
- `texture_merge.go`
- `texture_proposals.go`

Move HTTP-only request/response DTOs, authentication/JSON/path/route helpers, and handler-only free functions required by those receivers. Leave non-HTTP `Runtime` methods, domain types, state authority, stores, and business helpers in runtime.

Update `internal/apihandler/routes.go` to use its package-local handler and `internal/sandbox/run.go` to construct that handler once using the already loaded runtime and provider configuration. Carry all eleven S3-I17 candidate mutation receivers and their real calls through this cutover; do not alter the candidate domain operations or ratchet debt authority.

## Private Dependency Rule

Resolve cross-package dependencies in the same atomic landing, in this order:

1. Use existing cohesive exported Runtime APIs, `Store()`, and `EventBus()`.
2. Store already loaded `provideriface.Config` on the handler for transport response/gating needs; do not add `Runtime.Config()` or field accessors.
3. Keep HTTP-only synchronization in apihandler when it protects request deduplication rather than runtime state.
4. For a private Runtime operation needed by a moved handler, expose only a cohesive business operation with a same-landing production caller and unchanged semantics. Raw `Provider()`, `Config()`, mutex, map, controller, or field-shaped accessors are forbidden.
5. If a dependency would require moving domain ownership or adding a seam, stop and return a compile-proven blocker before widening.

## Exact Mutation Lock

Allowed production paths: `internal/apihandler/**`, the twenty named `internal/runtime` files only for moving/deleting handler-owned declarations and minimal cohesive Runtime operation visibility required by compilation, `internal/sandbox/run.go`, direct moved-handler tests, and `docs/runtime-dissolution-inventory.yaml` after focused proof.

Forbidden: changing any route/path/order/method/status/body/schema/auth/owner/internal-call behavior; candidate/promotion/provider/store/actor/lifecycle/Texture semantics; moving non-HTTP business ownership; new package; compatibility layer; generated bulk copy left beside originals; broad unrelated formatting or test churn.

The implementation must be one clean atomic commit. No intermediate commit may contain two handler types or leave the repository uncompilable. If safe automation is used, final source must be idiomatic hand-maintainable Go, not generated indirection.

## Acceptance

- Exactly one `APIHandler` and constructor exist in `internal/apihandler`; zero runtime declarations, receivers, aliases, or callers remain.
- All 128 receivers are accounted for by move or evidence-backed deletion; the eleven candidate receivers remain production callers until ordered step 4.
- Exact 46-slot live route table/order and test gating are unchanged; one sandbox construction/call path and the same canonical server/product tool remain.
- Initial unused-export debt is `<=16`. Runtime-scoped routes stay `2` until candidate step 4, while runtime production LOC, exports, and caller edges decrease materially. No new wrapper/interface/accessor/compatibility marker; the one apihandler->runtime concrete dependency is explicit deletion debt.
- Focused apihandler/runtime/sandbox and each moved domain surface tests pass; runtime ratchet passes.
- Independent verifier, full CI/deploy identity, authenticated staging controlled write/read and public read acceptance, consensus, and adjudication pass before closure.
