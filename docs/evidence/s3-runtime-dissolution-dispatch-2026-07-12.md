# S3 Runtime Dissolution Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I1-dead-api-handlers`
- Dispatch nonce: `s3-runtime-dissolution-i1-nonce-01`
- Transition: `s3-i1-dispatch-intent-62`
- Canonical parent: `b1cc1e55`
- Mutation class: orange
- Rollback: revert the atomic deletion commit

## Entry Gate

S2 completed at sandbox artifact `b7b1262e455a779ca00c8d968ef28b3fa6af9b50`. The current runtime ratchet passes with `148` Go files, `47` runtime routes, `1,199` exports, `604` export-caller edges, `39` initial unused exports, `15` compatibility markers, four production importers, and five wrappers. The S1 exception table names every S1-added runtime surface and the baseline includes the bounded exception.

Five read-only S3 scouts were dispatched for dead-surface, execution-core, API/bootstrap, Browser, and S1-exception analysis; all failed before inspection with external `402 Insufficient account balance`. They contribute no findings. The first slice therefore uses the mechanically generated unused-export inventory plus gopls production/test reference evidence.

## S3-I1 — Delete Unregistered Runtime API Handlers

The ratchet marks these `internal/runtime/api.go` methods `disposition: delete`:

- `HandleRunSubmission` — gopls finds declaration only;
- `HandleSpawn` — declaration only;
- `HandleRunStatus` — declaration plus test-only `waitForTaskCompletion` helper;
- `HandleChannelMessageList` — declaration only;
- `HandleRunStatusByID` — declaration only;
- `HandleTopology` — declaration only.

None appears in the current runtime route inventory or `RegisterRoutes`. They are not product endpoints; live run submission/status/list/cancel surfaces use prompt-bar and the registered owner-scoped routes. `waitForTaskCompletion` must poll the store/runtime directly instead of preserving an unregistered HTTP handler for tests.

Allowed paths: `internal/runtime/api.go`, `internal/runtime/test_helpers_test.go`, directly dependent focused runtime tests, and `docs/runtime-dissolution-inventory.yaml` after implementation proof. No other runtime behavior, route, config, package, or domain extraction is authorized.

Change: delete all six methods, delete response/request structs or helpers only if they become unused, and rewrite the test-only completion helper against the canonical runtime/store API. Do not register replacement routes, add wrappers, aliases, deprecated shims, or copy behavior.

Acceptance:

1. syntax-aware references find no production caller before deletion and no residual symbol afterward;
2. focused tests covering the rewritten completion helper and registered run/prompt routes pass;
3. runtime ratchet decreases production exports and LOC with no increase in routes, wrappers, compatibility markers, production importers, or unused debt;
4. independent verifier confirms no registered product route or test-only production API was removed;
5. CI and sandbox deployment pass; deployed health and an existing owner-scoped CLI/product run observation remain green;
6. post-implementation consensus has no confirmed blocker.

This is deletion-only S3 order item 1. It does not authorize Browser extraction, live execution-core movement, API/bootstrap ownership changes, `apihandler` removal, or any new package.
