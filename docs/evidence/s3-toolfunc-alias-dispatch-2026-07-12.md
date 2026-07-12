# S3 ToolFunc Alias Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I6-toolfunc-alias`
- Dispatch nonce: `s3-runtime-dissolution-i6-nonce-01`
- Transition: `s3-i6-dispatch-intent-134`
- Canonical parent: `50ff30bd`
- Mutation class: orange
- Rollback: revert the atomic implementation landing to `50ff30bd`; do not restore the alias through a second compatibility surface.
- Protected surfaces: none. No route, tool registration, runtime state, Trace, Wire, promotion/rollback, candidate computer, auth/session, vmctl, gateway/provider, model, or deployment-routing surface may change.
- Conjecture delta: the runtime `ToolFunc` alias is a declaration-only compatibility residue; deleting it removes false runtime API surface while the authoritative `toolregistry.ToolFunc` and all active tool behavior remain unchanged.
- Heresy delta at dispatch: `discovered`: one declaration-only backward-compatibility alias; `introduced`: none; `repaired`: pending.

## Problem Record

`internal/runtime/tools.go` still re-exports `toolregistry.ToolFunc` as `runtime.ToolFunc` under a backward-compatibility comment. LSP reference analysis reports exactly one reference: the declaration itself at `internal/runtime/tools.go:24`. Repository-wide source and build-tag-aware text searches find no caller. The authoritative type remains `internal/toolregistry.ToolFunc`, used by `toolregistry.Tool.Func`.

The executable runtime dissolution ratchet classifies `internal/runtime/tools.go:type:ToolFunc` as `delete`. Keeping the alias preserves an unused compatibility surface and one unit of initial unused-export debt.

## Exact Mutation Lock

Only `internal/runtime/tools.go` may change in source:

- delete exactly the `ToolFunc = toolregistry.ToolFunc` alias line;
- preserve `Tool`, `ToolRegistry`, every constructor alias, imports, tool schemas, registrations, routes, state, providers/models, and all behavior;
- introduce no alias, helper, test seam, forwarding method, package extraction, or unrelated cleanup;
- tests need no source edits because there is no caller.

After implementation, regenerate `docs/runtime-dissolution-inventory.yaml`. Acceptance requires source/default compilation, executable ratchet decrease without gated growth, independent verification, full CI, staging identity/product smoke, consensus, and adjudication.

## S3-I6 Implementation Receipt

- Integrated implementation: `5736341f` (isolated commit `be27ca99782624dd57d1f024a00d3ca60419dc59`).
- Exact source diff: one deletion in `internal/runtime/tools.go`; `ToolFunc = toolregistry.ToolFunc` was removed and every surrounding byte remained unchanged.
- Pre/post all-Go-source and build-tag-aware scans found no caller; after deletion only authoritative `internal/toolregistry.ToolFunc` remains.
- Default runtime compilation passed. Comprehensive-tag compilation has identical pre/post unrelated failures in `prompts_test.go` and `texture_test.go`; this is documented residual drift outside S3-I6.
- Ratchet passed after removing the fulfilled debt row: production LOC `46934 -> 46933`, exports `1143 -> 1142`, initial unused-export debt `25 -> 24`; test LOC, caller edges, routes, tools, production importers, wrappers, compatibility markers, store calls, interface candidates, and citers remained gated.

## S3-I6 Independent Verification Blocker

- Independent `S3I6Verifier` returned procedural `BLOCKING`: source scope, caller absence, authoritative type preservation, and default compilation pass, but this implementation receipt added one historical-evidence citer after the prior baseline.
- Smallest repair: regenerate the inventory so `citers=214`, then rerun the executable ratchet and request final independent reverification. No source correction is required.

## S3-I6 Final Verification, CI, Deploy, and Acceptance

- Independent `S3I6Verifier` final recheck returned `PASS` at confidence `1.0` with no findings on canonical `e22644a1`; executable ratchet, ratchet tests, and default runtime compilation passed with `citers=214`.
- GitHub Actions run `29202509590`, attempt `2`, passed every selected normal/race gate and deployed checkpoint `626400430bcf4bd04cccbb8a8bf60f7b83d110e6`.
- Deployment job `86677607396` published the activation receipt at `2026-07-12T18:02:35Z`; sandbox and gateway artifacts were active at `626400430bcf4bd04cccbb8a8bf60f7b83d110e6`.
- Staging health returned `200`/`status=ok`; authenticated `GET https://choir.news/api/agent/loops` returned `200`, proving the registered run-list product path remained live after alias deletion.
- Residual risk: pre-existing comprehensive-tag `prompts_test.go`/`texture_test.go` drift remains outside S3-I6; no in-slice residual risk.
