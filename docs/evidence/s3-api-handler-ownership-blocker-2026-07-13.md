# S3 APIHandler Ownership Structural Blocker

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I18-atomic-api-handler-ownership`
- Dispatch nonce: `s3-runtime-dissolution-i18-nonce-01`
- Canonical implementation parent: `edd2d5517537980f63706163f095c947cc2155f8`
- Problem-record parent: `4923c48c87c6f10bd33d900562f0a135b16d8351`
- Mutation class: red mission-boundary finding; this receipt is documentation-only
- Protected surfaces implicated: candidate/promotion, Texture canonical behavior, run acceptance, provider/model routing, Browser control, product events, persistent Super reconciliation
- Substrate classification: package/authority-boundary substrate, not an individual HTTP-handler symptom

## Problem

The S3-I18 dispatch authorized a mechanical atomic move of the one `runtime.APIHandler` type, constructor, and all 128 HTTP receiver bodies into `internal/apihandler`, while forbidding app/domain/state/provider/store/lifecycle behavior movement, new public seams, wrappers, callbacks, accessors, forwarders, aliases, dual handlers, and out-of-lock production changes.

A mechanically faithful compile diagnostic proved those constraints are mutually unsatisfiable. Moving the locked declarations exposes package-private dependencies whose ownership and test disposition were not included in the dispatch. The result is not a small missing import or isolated prerequisite: it is the package boundary the slice was intended to establish.

No implementation commit exists. The diagnostic prototype was discarded, the isolated worktree is clean, and canonical product behavior is unchanged.

## Evidence

The isolated implementer moved all 128 receivers across 20 files plus their direct declaration closure solely to compile the proposed boundary.

- `go test -gcflags=all=-e ./internal/apihandler -run '^$'`: failed with 238 errors, including 170 undefined-symbol occurrences, 77 unique private declaration classes, 66 private `Runtime` member occurrences across 21 unique members, and two private return-type mismatches. Seventeen of the twenty moved files failed. Full output: `artifact://2607`. Categorized output: `artifact://2610`.
- The 77 missing declaration classes comprise 20 types, 21 constants, and 36 functions owned by 18 production files outside the exact mutation lock.
- The 21 private `Runtime` members comprise five fields and sixteen methods. Existing `Store()` and `EventBus()` operations and handler-owned configuration/synchronization can resolve only a minority. Provider policy, prompt-bar completion, Texture route/revision/merge operations, Browser operations, product events, persistent-Super reconciliation, app-adoption preview, and system-prompt rendering have no approved cohesive public operation boundary.
- `go test -gcflags=all=-e ./internal/runtime -run '^$'` against the diagnostic split failed with 38 errors across eleven same-package test files. Those tests directly name the old handler and private HTTP support declarations; `runtime` cannot import `apihandler` because `apihandler` imports `runtime`.
- After discarding the diagnostic, `go test ./internal/apihandler ./internal/runtime ./internal/sandbox -run '^$'` passed (`artifact://2618`), focused route tests passed (`artifact://2625`), and the isolated worktree was clean at `edd2d551` (`artifact://2616`).
- Canonical boundary remains: one runtime handler type, one runtime constructor, 128 runtime receivers, and zero apihandler handler types/constructors/receivers.

## Dependency Graph

```text
cmd/sandbox
  -> apihandler.RegisterRoutes
  -> runtime.NewAPIHandler
  -> runtime.APIHandler (128 HTTP receivers)
       -> runtime private HTTP DTOs/helpers
       -> runtime private fields/methods
       -> candidate/promotion operations
       -> provider/model policy and prompt construction
       -> run acceptance and Wire resolution
       -> Browser control and events
       -> Texture diagnosis/revision/lineage/merge
       -> product events and persistent Super

runtime same-package tests
  -> runtime.APIHandler + private HTTP support
  -X-> apihandler (would create apihandler -> runtime -> apihandler cycle)
```

The existing `internal/apihandler` package is wired and canonical for route-table ownership and the product API tool, but it is not a dormant replacement for the complete handler. Connecting it already removed duplicate registrar authority; it does not resolve the package-private business boundary.

## Why Incremental Patches Are Rejected

1. Exporting 77 declarations or raw provider/config/mutex/controller accessors would create the broad facade prohibited by doctrine and the dispatch.
2. Copying declarations into `apihandler` would create dual business/helper implementations.
3. Adding one-line wrappers around sixteen private methods would be a forwarder seam, not a cohesive business boundary.
4. Moving the 18 owner files crosses candidate/promotion/Texture/provider/tool authority and silently begins S3 step 4.
5. Test aliases, callbacks, duplicate registrars, or a test-only handler would preserve the old boundary under another name.
6. Another narrow deletion slice cannot solve the substrate. S3-I16 exposed route ownership, S3-I17 proved dormant handler deletion impossible under the ratchet, and S3-I18 now proves the whole mechanical move impossible. This is the third non-convergent iteration around the same boundary.

## Structural Assessment

The substrate problem is that HTTP transport and domain orchestration are co-located inside package-private receiver bodies. The mission order assumes transport ownership can move before cohesive runtime business operations exist. The compile proof falsifies that assumption.

A replacement implementation does not exist unwired. `internal/apihandler` is the correct destination, but only route-table ownership and one product tool have been connected. The missing object is an explicitly designed, finite business-operation boundary between transport and runtime—not another handler shim.

## Required Parent-Plan Decision

Before another implementation attempt, replace S3-I18 with one of these authority-level routes:

1. **Cohesive prerequisite operations, then atomic handler cutover.** Define a small finite set of typed runtime business operations covering the protected domains, their invariants, and their tests. Authorize a bounded prerequisite ratchet exception, then move the transport shell and test ownership atomically.
2. **One broadened red atomic landing.** Include the complete operation/type/test disposition and handler move in one mutation lock. This has a much larger review and rollback surface.
3. **Reorder S3 step 4 before handler ownership.** Extract domain operations first, then move the thin transport shell. This changes the settled S3 order and requires explicit owner authority.

The recommended route is option 1. It preserves the intended destination and clean cutover while making the hidden semantic boundary explicit. It must not authorize a list of raw exports/accessors. Every one of the 20 types, 21 constants, 36 helpers, 21 private members, 18 production owner files, and eleven affected test files needs a named disposition.

Because this changes a red protected-surface boundary and the mission's ordered graph, the orchestrator cannot self-ratify it. S3-I18 is `blocked_incomplete` pending owner authority. Per dead-end escalation, no fourth incremental APIHandler patch is authorized.

## Post-Blocker Consensus

Four independent reviewers returned unanimous support for the structural stop, the `blocked_incomplete` state, the need for owner authority, the absence of an unwired complete replacement, and route 1 as the preferred replacement. Confidence ranged from 0.91 to 1.0. Two reviewers ranked step-4 reordering second and two ranked the broadened atomic landing second; that dissent does not affect the unanimous first choice.

The smallest proposed authority object is `S3-I18R finite runtime business-operation boundary and two-landing cutover rule`: authorize one bounded red prerequisite landing containing only a closed set of cohesive typed runtime operations with production callers and behavior tests, followed by one atomic transport/type/test ownership cutover. Require a complete disposition table using only `move_to_apihandler_http_only`, `keep_private_runtime_domain`, `delete_as_dead`, or `replace_with_cohesive_runtime_operation`. Raw exports, accessors, wrappers, callbacks, aliases, forwarders, duplicate handlers, and indefinite dual paths remain forbidden.

Consensus artifact: `/tmp/choir-s3-i18-blocker-consensus-20260713`.

## Conjecture And Heresy Delta

- Conjecture delta: `S3_step3_transport_ownership_can_move_mechanically_before_business_operation_extraction` is falsified.
- Discovered heresies: package-private transport/domain co-location across 77 declaration classes and 21 private runtime members; same-package tests act as a second ownership boundary.
- Introduced: none.
- Repaired: none.

## Rollback And Residual State

- Source rollback: none required; no source commit exists.
- Dispatch rollback ref: `edd2d5517537980f63706163f095c947cc2155f8`.
- Canonical problem-record parent: `4923c48c87c6f10bd33d900562f0a135b16d8351`.
- Residual risk: runtime still owns the handler type, constructor, and all 128 receivers. Canonical runtime ratchet source counts are unchanged; documentation citer drift remains a known non-source baseline mismatch.
