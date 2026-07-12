# S3 Runtime Declaration-Only Export Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I4-declaration-only-exports`
- Dispatch nonce: `s3-runtime-dissolution-i4-nonce-01`
- Transition: `s3-i4-dispatch-intent-111`
- Canonical parent: `bc1419fc`
- Mutation class: orange
- Rollback: revert the atomic implementation landing to `bc1419fc`; do not restore exports through aliases.
- Protected surfaces: none. No route, tool registration, run state, Trace, Wire, promotion/rollback, candidate computer, auth/session, vmctl, gateway/provider, or deployment-routing surface may change.
- Conjecture delta: three exported declarations have no caller; deleting them removes false API surface while preserving the active replacement paths.
- Heresy delta at dispatch: `discovered: three declaration-only exports`; `introduced: none`; `repaired: pending`.

## Problem Record

The executable ratchet and independent LSP reference checks identify three exported declarations whose only reference is their own declaration:

- `(*Runtime).ChannelPost` in `internal/runtime/channel_store.go`; addressed wake/delivery is already owned by `ChannelCast`, the active replacement to which this wrapper forwards.
- `(*Runtime).ChannelRead` in `internal/runtime/channel_store.go`; active channel readers use owner-scoped store queries and actor message paths, not this ownerless wrapper.
- `textureprompts.RevisionMediaSourceResearchRequired` in `internal/runtime/textureprompts/prompts.go`; no production or test caller renders this overlay through the declaration.

A declaration with no caller is not product capability. Keeping these exports preserves misleading ownership surfaces and unused-export debt. This problem record precedes implementation.

## Exact Mutation Lock

Allowed production files only:

- `internal/runtime/channel_store.go`: delete exactly `ChannelPost`, `ChannelRead`, and attached comments.
- `internal/runtime/textureprompts/prompts.go`: delete exactly `RevisionMediaSourceResearchRequired` and its attached comment.

`docs/runtime-dissolution-inventory.yaml` is parent-owned and changes only after implementation proof. No test file is authorized: no caller rewrite is expected.

Forbidden: replacement helper, alias, forwarding method, exported test seam, active overlay/template deletion, `ChannelCast` change, owner-scoped store-query change, actor message delivery change, route/tool registration change, state-authority change, unrelated cleanup, or package extraction.

## Acceptance

1. repository-wide symbol/reference and build-tag searches find no caller before deletion and no residual symbol afterward;
2. exactly the three declarations/comments are removed with no caller or test change;
3. default runtime and textureprompts packages compile and focused channel/prompt tests pass;
4. `ChannelCast`, owner-scoped channel reads, actor delivery, overlays/templates, routes, tools, and state authorities remain unchanged;
5. ratchet production LOC, exports, and unused-export debt decrease with no gated growth;
6. independent verification, full CI, staging identity/product smoke, consensus, and adjudication pass.
