# S3 Tool Registry With-Tools Alias Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I7-toolregistry-with-tools-alias`
- Dispatch nonce: `s3-runtime-dissolution-i7-nonce-01`
- Transition: `s3-i7-dispatch-intent-146`
- Canonical parent: `72299d24`
- Mutation class: orange
- Rollback: revert the atomic implementation landing to `72299d24`; do not restore the alias through a second compatibility surface.
- Protected surfaces: none.
- Conjecture delta: `runtime.NewToolRegistryWithTools` is a test-only compatibility alias; deleting it and pointing its sole test directly at the authoritative `toolregistry.NewToolRegistryWithTools` removes false runtime API surface without changing registry behavior.
- Heresy delta at dispatch: `discovered`: one test-only constructor alias; `introduced`: none; `repaired`: pending.

## Problem Record

`internal/runtime/tools.go` still re-exports `toolregistry.NewToolRegistryWithTools` under a backward-compatibility block. Repository/build-tag-aware search finds exactly one caller, `TestNewToolRegistryWithTools` in `internal/runtime/tools_test.go`; no production or external caller exists. The authoritative constructor already exists in `internal/toolregistry/toolregistry.go` and is used by `MustNewToolRegistry` there.

The executable dissolution inventory classifies `internal/runtime/tools.go:var:NewToolRegistryWithTools` as `delete`. Keeping it retains a redundant compatibility surface and one unit of unused-export debt.

## Exact Mutation Lock

Allowed source paths only: `internal/runtime/tools.go` and `internal/runtime/tools_test.go`.

- delete exactly `NewToolRegistryWithTools = toolregistry.NewToolRegistryWithTools` from the runtime alias block;
- update the one test caller to invoke `toolregistry.NewToolRegistryWithTools`, adding only the required import;
- preserve `Tool`, `ToolRegistry`, `NewToolRegistry`, `MustNewToolRegistry`, all registry behavior/tests, imports not made unused, schemas, registrations, routes, providers/models, and state;
- introduce no replacement runtime alias/helper/test seam or unrelated cleanup.

After implementation, regenerate `docs/runtime-dissolution-inventory.yaml`. Acceptance requires focused/default compilation, ratchet decrease without gated growth, independent verification, full CI, staging identity/product smoke, consensus, and adjudication.

## S3-I7 Implementation Receipt

- Integrated implementation: `4a2c8bd9` (isolated commit `5987bcc721baafaaa7ceddbceeb57a91faf60248`).
- Exact source diff: delete the runtime constructor alias, add the authoritative `internal/toolregistry` import to `tools_test.go`, and change the sole test caller to `toolregistry.NewToolRegistryWithTools`; `2` insertions and `2` deletions.
- Focused `TestNewToolRegistryWithTools` and default runtime compilation passed before and after; all-source/build-tag-aware scans found no production/external caller and no replacement runtime seam.
- Ratchet passed after removing the fulfilled export/debt rows: production LOC `46933 -> 46932`, test LOC `53037 -> 53038`, exports `1142 -> 1141`, initial unused-export debt `24 -> 23`; caller edges and every gated authority count remained flat.
