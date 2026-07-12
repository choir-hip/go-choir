# S3 NewToolRegistry Test-Caller Cutover

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I8-new-tool-registry-test-callers`
- Dispatch nonce: `s3-runtime-dissolution-i8-nonce-01`
- Transition: `s3-i8-dispatch-intent-155`
- Canonical parent: `bf60ae14`
- Mutation class: orange
- Rollback: revert the atomic implementation landing to `bf60ae14`; do not restore the alias through another compatibility surface.
- Protected surfaces: none.
- Conjecture delta: `runtime.NewToolRegistry` is retained solely by tests; migrating every test to the authoritative `toolregistry.NewToolRegistry` and deleting the alias removes the final declaration-only registry constructor compatibility surface without changing production behavior.
- Heresy delta: `discovered`: one test-only constructor alias retained by mechanically enumerable tests; `introduced`: none; `repaired`: pending.

## Problem Record

`internal/runtime/tools.go` re-exports `toolregistry.NewToolRegistry`. The executable inventory reports no production callers. Repository/build-tag-aware search finds callers only in test files: runtime tests plus provider tests using `runtime.NewToolRegistry`; the authoritative constructor exists in `internal/toolregistry/toolregistry.go` and is already used by production `toolregistry.NewToolRegistryWithTools`.

Keeping the alias violates the S3 no-alias and test-only-deletion gates and retains one unit of initial unused-export debt. Unlike live `ToolRegistry`/`Tool` aliases, this constructor can cut over now without crossing into extraction step 2.

## Exact Mutation Lock

Allowed source paths: `internal/runtime/tools.go` and only test files containing a `NewToolRegistry()` caller under `internal/runtime` or `internal/provider`.

- delete exactly `NewToolRegistry = toolregistry.NewToolRegistry` from runtime production code;
- replace every runtime test call with `toolregistry.NewToolRegistry()` and every provider test call with `toolregistry.NewToolRegistry()`;
- add the authoritative import only where required; preserve all test semantics, names, tools, registry behavior, production registrations, routes, providers/models, and state;
- preserve `Tool`, `ToolRegistry`, `MustNewToolRegistry`, and all live aliases until their ordered extraction boundary;
- introduce no replacement runtime helper/alias/test seam or unrelated cleanup.

Acceptance requires zero residual runtime constructor alias/callers, focused tests/default compilation, ratchet decrease without gated growth, independent verification, full CI, staging identity/product smoke, consensus, and adjudication.

## S3-I8 Implementation Receipt

- Integrated implementation: `35f9c1f0` (isolated commit `7da3eea102112513339db6f4a9ca35c884115b94`).
- Ten-file exact cutover: `86` legacy test calls became direct `toolregistry.NewToolRegistry()` calls; the runtime alias was deleted; no production caller or replacement seam remains.
- Default runtime/provider tests and compilation passed; integration-tag provider compilation passed. Comprehensive-tag runtime compilation reproduces identical pre-existing `prompts_test.go`/`texture_test.go` errors at the canonical parent.
- Ratchet passed after removing the fulfilled export/debt rows: production LOC `46932 -> 46931`, test LOC `53038 -> 53044`, exports `1141 -> 1140`, initial unused-export debt `23 -> 22`; caller edges and every gated authority count remained flat.

## S3-I8 Final Verification, CI, Deploy, and Acceptance

- Independent `S3I8Verifier` returned `PASS` at confidence `0.97` with no findings on canonical `e88b2727`.
- GitHub Actions run `29205831071`, attempt `2`, passed every selected normal/race gate and deployed checkpoint `331ad4834aa5fcd5489d15aeddc8c7e4cfc9e148`.
- Deployment job `86686164550` published the activation receipt at `2026-07-12T19:43:07Z`; sandbox and gateway artifacts were active at `331ad4834aa5fcd5489d15aeddc8c7e4cfc9e148`.
- Staging health returned `200`/`status=ok`; authenticated `GET https://choir.news/api/agent/loops` returned `200`.
- Residual risk: pre-existing comprehensive-tag test drift remains outside S3-I8; no in-slice residual risk.
