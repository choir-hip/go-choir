# S3 Sandbox Bootstrap Ownership Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I15-sandbox-bootstrap-ownership`
- Dispatch nonce: `s3-runtime-dissolution-i15-nonce-01`
- Transition: `s3-i15-dispatch-intent-233`
- Canonical parent: `3c651e58`
- Mutation class: orange
- Rollback: `3c651e58`

## Problem Record

After S3-I14 removed runtime's duplicate product-tool route constructor, the only production caller of the `apihandler` wrapper is the sandbox binary. The wrapper still embeds runtime's API handler and forwards construction and route registration without owning HTTP behavior. The sandbox binary also owns roughly 260 lines of real bootstrap composition: config derivation, store lifecycle, provider selection, trace mounting, actor-runtime construction, file-event wiring, tool registration, readiness probes, and process startup. That composition belongs in the existing `internal/sandbox` package, which already owns sandbox configuration, source workspace, shell, files, and terminal surfaces.

Deleting only the wrapper and making the command import runtime directly would satisfy a filename count while preserving false command-level authority. Adding an adapter registrar, interface, callback, alias, or renamed facade would hide the same seam. Moving the complete bootstrap into the existing sandbox package deletes both the false wrapper and the command's composition authority without moving app/domain behavior or creating a package cycle.

This is a deletion-bearing step-3 prerequisite. Runtime still owns `APIHandler` methods and route registration after this slice; their later atomic HTTP-ownership extraction remains open. Step 4 remains unauthorized.

## Exact Mutation Lock

- Move all nontrivial bootstrap composition and its helper tests from `cmd/sandbox/main.go` / `main_test.go` into new `internal/sandbox/run.go` / `run_test.go` within the existing package.
- Reduce `cmd/sandbox/main.go` to process entry only: preserve the `zot-session` mode and invoke the sandbox-owned bootstrap for normal mode.
- Inside sandbox bootstrap, construct runtime's API handler and register runtime routes directly on the one canonical server; register the apihandler-owned product API tool on that same server and Super registry.
- Delete `internal/apihandler/api.go` completely. Keep `internal/apihandler/product_api_tool.go` as real transport ownership.
- Migrate `internal/runtime/texture_live_llm_workflow_test.go` from the deleted wrapper to its package-local runtime handler constructor.
- Remove false deprecation comments in runtime API declarations that point to the deleted wrapper; do not rename, alias, wrap, forward, duplicate, or otherwise change those declarations in this slice.
- Preserve exact bootstrap order, config/default behavior, provider routing, store/trace lifecycle, file-event behavior, tool catalogs, readiness semantics, route table, startup/shutdown, logs, and `zot-session` behavior.
- Do not move API handlers or app/domain logic; do not change actor lifecycle, provider semantics, state, models, routes, health policy, deployment configuration, or step 4.
- Do not add packages, interfaces, callbacks, accessors, aliases, forwarding methods, fallback bootstrap paths, duplicate servers/muxes, or compatibility shims.
- Update `docs/runtime-dissolution-inventory.yaml` only after focused behavior passes and classify durable citers.

## Acceptance

- `internal/apihandler/api.go` is deleted and no `Handler` embed, `NewAPIHandler`, `RegisterRoutes`, or `RegisterTextureRoutes` wrapper remains outside runtime.
- `cmd/sandbox/main.go` has no runtime, actor-runtime, provider, server, store, trace, toolregistry, health, events, or apihandler composition imports; it only selects process mode and delegates normal bootstrap to `internal/sandbox`.
- Existing sandbox package is the sole bootstrap composition owner; no second startup path exists.
- Runtime API declarations remain one path and have no false apihandler deprecation pointer.
- Focused sandbox command/package, runtime live workflow, apihandler product tool, and actor-runtime tests pass.
- Wrapper count and command-level authority edges decrease; routes/tools/product behavior remain flat; every other gated authority count is non-increasing except classified durable citers.
- Independent verification, full CI, deploy identity, authenticated public product-path smoke, consensus, and adjudication pass before closure.

## S3-I15 Compile-Proven Cycle Blocker

- The exact bootstrap move exposed an existing reverse dependency: runtime Texture import code calls sandbox-owned `ResolveFilesRoot` at three production sites. Once the existing sandbox package owns bootstrap and imports runtime, that reverse edge creates an illegal Go package cycle.
- This is a pre-existing ownership defect revealed by the cutover, not a reason to restore the wrapper.
- Smallest deletion-bearing repair: move `DefaultFilesRoot` and `ResolveFilesRoot` into the existing provider-interface configuration authority, migrate sandbox and runtime callers, add exact explicit/environment/default precedence coverage, and delete the sandbox declarations. No alias, forwarder, duplicate resolver, callback, interface, or new package is permitted.

## S3-I15 Implementation Receipt

- Integrated isolated commit `2c950a7eb6439cd4148ce7b87554676c70d00609` as canonical `887bbdde`; it descends from the canonical cycle problem record `5d427cc3`.
- `cmd/sandbox` is process entry only. The existing sandbox package owns the complete original startup sequence and dedicated `zot-session` operation.
- The apihandler wrapper file is deleted. Sandbox bootstrap wires runtime's one handler/registrar path directly on the canonical server, then binds the apihandler-owned product tool to that same server and Super registry.
- The compile-proven reverse dependency is deleted: files-root default/resolution authority moved to provider-interface configuration; sandbox and runtime callers converge there with exact explicit/environment/default precedence tests and no alias or forwarder.
- Runtime's false apihandler deprecation pointers are removed, and the build-tagged live workflow uses the one runtime handler constructor directly.
- Focused provider-interface, sandbox command/package, apihandler, actor-runtime, integration-tagged live workflow, full runtime, runtime-ratchet tests, sandbox build, and end-to-end `zot-session` entry smoke pass.
- Ratchet deltas: production LOC `44032 -> 44024`, test LOC `50142 -> 50141`, caller edges `368 -> 365`, wrappers `5 -> 3`, compatibility markers `10 -> 8`; files, exports, debt, routes, tools, importers, store calls, and interface candidates are flat. One durable citer is classified (`244 -> 245`).
