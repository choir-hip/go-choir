# S3 Runtime Dead Helper Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I2-declaration-only-helpers`
- Dispatch nonce: `s3-runtime-dissolution-i2-nonce-01`
- Transition: `s3-i2-dispatch-intent-79`
- Canonical parent: `f10b8d98`
- Mutation class: orange
- Rollback: revert the atomic implementation landing to `f10b8d98`; do not restore aliases or wrappers elsewhere.
- Protected surfaces: none. This slice does not touch live promotion/rollback, run lifecycle, Wire, candidate computers, auth/session renewal, vmctl, gateway/provider calls, or deployment routing.
- Conjecture delta: three exported production helpers with declaration-only references are inert compatibility/test-design residue; deleting them reduces runtime surface without changing product behavior.
- Heresy delta at dispatch: `discovered: three declaration-only runtime exports`; `introduced: none`; `repaired: pending`.

## Problem Record

The current mechanically generated inventory and gopls reference graph agree that these exports have only their declarations and no production or test caller:

- `internal/runtime/promptspec/promptspec.go: Document.MustRender`;
- `internal/runtime/runtime.go: (*Runtime).ToolRegistry`;
- `internal/runtime/tool_profiles.go: WithToolProfileRegistry`.

Keeping declaration-only helpers expands the typed runtime surface and invites new callers to depend on APIs that the live product does not use. Tests do not justify retaining them, and no replacement is required. This problem record precedes the deletion implementation.

## Exact Mutation Lock

Allowed production files:

- `internal/runtime/promptspec/promptspec.go` — delete `Document.MustRender` only;
- `internal/runtime/runtime.go` — delete `(*Runtime).ToolRegistry` only;
- `internal/runtime/tool_profiles.go` — delete `WithToolProfileRegistry` only;
- `docs/runtime-dissolution-inventory.yaml` only after implementation proof to remove the three satisfied debt rows and rebase deterministic counts/citers.

Focused test files may be changed only if compilation proves a previously hidden build-tag caller; any such caller is a blocking dependency to report before mutation rather than silently rewriting scope.

Forbidden: replacement helper, alias, wrapper, forwarding method, new package, route/config/bootstrap changes, live tool-loop movement, Browser extraction, promotion/candidate mutation, or unrelated cleanup.

## Acceptance

1. gopls/reference and repository searches show no caller before deletion and no residual symbol after deletion;
2. focused package compilation/tests for `internal/runtime/promptspec` and default `internal/runtime` pass;
3. no registered route, tool registration, state authority, or product behavior changes;
4. runtime ratchet passes with exports, unused-export debt, and production LOC decreased and no gated count increased;
5. independent verifier confirms exact deletion-only scope;
6. full CI, staging identity/health, product-path smoke, and post-implementation consensus have no confirmed blocker.
