# S3 Runtime Declaration-Only Export Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I4-declaration-only-exports`
- Dispatch nonce: `s3-runtime-dissolution-i4-nonce-01`
- Transition: `s3-i4-dispatch-intent-111`
- Canonical parent: `bc1419fc`
- Mutation class: orange
- Rollback: revert the atomic implementation landing to `bc1419fc`; do not restore exports through aliases.
- Protected surfaces: none. No route, tool registration, run state, Trace, Wire, promotion/rollback, candidate computer, auth/session, vmctl, gateway/provider, or deployment-routing surface may change.
- Conjecture delta: one exported prompt declaration has no caller; deleting it removes false API surface without changing active overlays or templates.
- Heresy delta at dispatch: `discovered: one in-scope declaration-only export plus two wider build-tag caller surfaces deferred`; `introduced: none`; `repaired: pending`.

## Problem Record

The executable ratchet and initial LSP reference checks classified three exports as declaration-only. Implementation reconciliation then found `15` comprehensive-build-tag calls to `(*Runtime).ChannelPost` and `(*Runtime).ChannelRead` across `agent_tools_test.go`, `concurrent_workers_test.go`, and `failure_isolation_test.go`. Both channel methods and every caller are therefore deferred to a dedicated caller-complete slice and forbidden here.

`textureprompts.RevisionMediaSourceResearchRequired` in `internal/runtime/textureprompts/prompts.go` remains the sole in-scope export: repository-wide and build-tag-aware checks find no production or test caller rendering the overlay through this declaration.

A declaration with no caller is not product capability. Keeping this export preserves misleading prompt API surface and unused-export debt. This amended problem record precedes implementation commit.

## Exact Mutation Lock

Allowed production file only:

- `internal/runtime/textureprompts/prompts.go`: delete exactly `RevisionMediaSourceResearchRequired` and its attached comment.

`docs/runtime-dissolution-inventory.yaml` is parent-owned and changes only after implementation proof. No test file is authorized.

Forbidden: `ChannelPost`, `ChannelRead`, every channel caller, replacement helper, alias, forwarding method, exported test seam, active overlay/template deletion, route/tool registration change, state-authority change, unrelated cleanup, or package extraction.

## Acceptance

1. repository-wide symbol/reference and build-tag searches find no caller before deletion and no residual symbol afterward;
2. exactly the one prompt declaration/comment is removed with no caller or test change, while both channel methods and every caller remain unchanged;
3. default runtime and textureprompts packages compile and focused prompt tests pass;
4. active overlays/templates, channel APIs/callers, routes, tools, and state authorities remain unchanged;
5. ratchet production LOC, exports, and unused-export debt decrease with no gated growth;
6. independent verification, full CI, staging identity/product smoke, consensus, and adjudication pass.

## S3-I4 Implementation Receipt

- Integrated implementation: `710d2046` (isolated commit `4888b0775f5bfb34baa07aefa663696dfa36b8fd`).
- Exact production diff: `internal/runtime/textureprompts/prompts.go`, five deletions, no insertions.
- `ChannelPost`, `ChannelRead`, `channel_store.go`, all callers, tests, embedded overlay YAML, wildcard embedding, and generic renderer remain unchanged.
- `go test ./internal/runtime/textureprompts -count=1` and default runtime package compilation passed.
- Ratchet passed: production LOC `46949 -> 46944`, exports `1145 -> 1144`, and initial unused-export debt `27 -> 26`; test LOC, caller edges, routes, tools, production importers, wrappers, compatibility markers, store calls, interface candidates, and citers remained gated.

## S3-I4 Independent Verification Repair

- `S3I4Verifier` confirmed the source deletion, channel preservation, focused tests, and default compilation, but returned procedural `BLOCKING` because the implementation receipt added two historical-evidence citers after the prior baseline.
- The inventory was regenerated; ratchet and ratchet unit tests pass with `citers=204`. No source correction was required.

## S3-I4 Final Verification, CI, Deploy, and Acceptance

- Independent `S3I4Verifier` final recheck returned `PASS` at confidence `0.99` with no findings on canonical `c0f075ba`.
- GitHub Actions run `29199070620`, attempt `2`, passed every selected normal/race gate and deployed checkpoint `fe4a1bc480687963546c774ad6f81fa425d91ba8`.
- Deployment job `86668756969` published the activation receipt at `2026-07-12T16:13:48Z`; sandbox and gateway artifacts were active at `fe4a1bc480687963546c774ad6f81fa425d91ba8`.
- Staging health returned `200`/`status=ok`; authenticated `GET https://choir.news/api/agent/loops` returned `200`, proving the registered run-list product path remained live after prompt-export deletion.
- Residual risk: the unchanged `15`-call comprehensive-tag `ChannelPost`/`ChannelRead` graph remains deferred to a caller-complete slice.
