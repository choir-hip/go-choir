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
