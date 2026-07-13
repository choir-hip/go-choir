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

## S3-I12 Implementation Receipt

- Integrated implementation `58593d85` from isolated commit `c435257234137e4aaa16ed63c171168a7c9630dd`.
- `provideriface` now solely owns `Config`, all defaults, `LoadConfig`, `NormalizeConfig`, and private parsing/filesystem helpers; runtime config source and tests are deleted or moved to their behavioral owner.
- All mechanically located production, default-test, integration-tag, comprehensive-tag, actorruntime, provider/gatewayruntime, and sandbox callers use provideriface directly; no runtime alias, forwarder, duplicate helper/default, or fallback loader remains.
- Focused provideriface config, sandbox bootstrap-value, runtime activation/run-memory, actorruntime, and integration-tag runtime checks pass. Comprehensive-tag compilation reproduces only the identical pre-existing `prompts_test.go`/`texture_test.go` failures.
- Ratchet passed after canonical regeneration: Go files `147 -> 146`, production files `77 -> 76`, production LOC `44680 -> 44338`, test LOC `50266 -> 50172`, exports `1061 -> 1031`, caller edges `549 -> 520`, compatibility markers `13 -> 12`; every other gated authority count is flat.

## S3-I12 Independent Verification

- Independent `S3I12Verifier` returned `PASS` at confidence `0.98` with no findings.
- Verified exact config declaration/caller relocation, absence of runtime aliases/forwarders/fallbacks/duplicates, preserved defaults/normalization/test behavior, focused compilation/tests, ratchet counts, and no out-of-scope semantic change.
- Full CI, comprehensive-tag parent drift disposition, deployment, and staging acceptance remained external gates at verifier return.

## S3-I12 CI, Deploy, and Acceptance

- GitHub Actions run `29216971462` attempt `2` passed every default, integration, race, ratchet, SBOM, and deploy gate for head `5958b290cf76b8340e454030e00e7f40436bd0be`.
- Deploy job `86716121905` completed successfully.
- Staging health returned `200`/`status=ok`; authenticated `GET https://choir.news/api/agent/loops` returned `200`.
- Comprehensive-tag parent drift remains the documented pre-existing `prompts_test.go`/`texture_test.go` failure set; default and integration-tag CI paths are green.
