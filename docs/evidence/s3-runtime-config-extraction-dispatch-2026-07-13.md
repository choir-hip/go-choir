# S3 Runtime Config Extraction Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I12-runtime-config-extraction`
- Dispatch nonce: `s3-runtime-dissolution-i12-nonce-01`
- Transition: `s3-i12-dispatch-intent-202`
- Canonical parent: `27e1567d`
- Mutation class: orange
- Rollback: `27e1567d`

## Problem Record

The unanimous S3 step-2 gate authorized step 3: move real API/config/bootstrap ownership, remove the `apihandler` wrapper, and remove direct `cmd/sandbox` runtime imports. `provideriface.Config` already owns the pure runtime configuration schema, but `internal/runtime/config.go` still re-exports that type, owns every default and environment loader/normalizer, and is the config source used by sandbox bootstrap. This leaves config authority in the package targeted for extinction and retains a compatibility alias explicitly classified `delete` by the executable inventory.

This is step-3 substrate debt, not an app-domain cutover. `provideriface` is the existing acyclic owner of the schema consumed by runtime, actorruntime, provider, and bootstrap; creating a second config package or leaving aliases would add authority. The smallest first step-3 slice is a clean move of config defaults/loading/normalization into `provideriface`, with every caller migrated and the runtime config file deleted. API and bootstrap-wrapper deletion remain later step-3 slices.

## Exact Mutation Lock

Move all live declarations and behavior from `internal/runtime/config.go` into `internal/provideriface` beside the canonical `Config` schema. Move the complete behavioral tests from `internal/runtime/config_test.go` to the owning package. Migrate every production, test, build-tag, and external caller directly to `provideriface.Config`, `provideriface.LoadConfig`, and provideriface-owned defaults. Delete runtime's `Config` alias, config file, and all config helper declarations; leave no aliases, forwarders, duplicate defaults, wrappers, or fallback loaders.

Preserve every environment name, default value, normalization rule, filesystem derivation, explicit-zero behavior, config field value, app-promotion command, provider timeout, activation budget, model policy, trace, Qdrant/Ollama, and test behavior byte-for-byte. Do not alter runtime construction, API/routes, `apihandler`, cmd/sandbox bootstrap topology beyond direct config symbol qualification, tools, state, models, apps, provider routing, or begin app/domain step 4.

## Acceptance

- one canonical config schema/default/loader/normalizer authority exists in `internal/provideriface`;
- `internal/runtime/config.go` and `config_test.go` are deleted;
- no runtime config alias/forwarder/default/helper or replacement config path remains;
- all default, focused config, runtime, actorruntime, provider, gatewayruntime, and cmd/sandbox caller paths compile and focused behavior tests pass;
- runtime production/test LOC, exports, caller edges, compatibility markers, and unused-export debt decrease while every gated authority count is non-increasing;
- independent verification, full CI, staging identity/product smoke, consensus, and adjudication pass.
