# S3 Sandbox Runtime Import Cutover Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S3-I13-sandbox-runtime-import-cutover`
- Dispatch nonce: `s3-runtime-dissolution-i13-nonce-01`
- Transition: `s3-i13-dispatch-intent-210`
- Canonical parent: `6ee86dd1`
- Mutation class: orange
- Rollback: `6ee86dd1`

## Problem Record

S3-I12 moved config authority to `provideriface`, but `cmd/sandbox/main.go` still directly imports `internal/runtime`, violating the explicit S3 step-3 cutover criterion. The remaining named uses are mechanically narrow: stub provider construction, agent-profile identifiers, model-policy path derivation, and the `ToolRegistry` type. Profile and registry authority already live in `agentprofile` and `toolregistry`; config/bootstrap path authority now lives in `provideriface`. Stub construction is the only behavior-bearing runtime dependency and must move to its real provider/bootstrap owner without losing its conductor-to-Texture decision behavior.

This is bootstrap/provider substrate debt, not an API or app-domain cutover. Adding an actorruntime/sandbox forwarder would hide the import rather than remove authority. The smallest atomic cut is to move the live stub provider and its exact behavior-supporting pure helpers to the smallest existing acyclic provider owner justified by callers, move model-policy path derivation to the config owner, migrate profile/registry uses to their existing owners, and delete every direct `cmd/sandbox -> internal/runtime` symbol edge.

## Exact Mutation Lock

Remove the direct `internal/runtime` import from `cmd/sandbox/main.go` and its tests. Replace runtime profile and registry compatibility symbols with `agentprofile` and `toolregistry`. Move `DefaultModelPolicyPath` to `provideriface` beside config ownership and migrate every caller, deleting the runtime declaration. Move `StubProvider`/`NewStubProvider` to the smallest existing provider package that can own the complete current behavior without importing runtime; move only mechanically required pure helpers and tests, migrate every caller, and delete runtime's declarations. Do not add aliases, forwarders, wrappers, callbacks, duplicate stub paths, fallback constructors, or a new package.

Preserve stub delay/cancellation/progress/error/result/policy behavior, conductor-to-Texture decision payload/title/seed semantics, gateway-vs-stub selection, tool profile counts/installation, model policy paths, runtime construction, API/routes, `apihandler`, state/models/apps, provider routing semantics, and lifecycle ordering. Do not begin API handler movement or app/domain step 4.

## Acceptance

- `cmd/sandbox` has no direct `internal/runtime` import or runtime-qualified symbol;
- agent profiles and ToolRegistry come directly from canonical owners;
- provideriface solely owns model-policy path derivation;
- one non-runtime package solely owns the complete stub provider behavior and runtime declarations are deleted;
- no alias/forwarder/wrapper/callback/duplicate/fallback/replacement import path remains;
- focused stub, provider policy, sandbox bootstrap, runtime, actorruntime, gatewayruntime, and build-tag caller behavior passes;
- runtime production LOC/exports/caller edges/production importers decrease and every gated authority count is non-increasing;
- independent verification, full CI, staging identity/product smoke, consensus, and adjudication pass.

## S3-I13 Implementation Receipt

- Integrated source `08049400` from returned branch commit `6efcae63e2abd4a8fb83503966f137c0eefe183c`.
- `cmd/sandbox` has no runtime import; profiles use `agentprofile`, registries use `toolregistry`, model-policy path derivation is provideriface-owned, and `internal/provider` solely owns complete StubProvider behavior plus shared pure Texture-decision helpers.
- Runtime StubProvider, model-path, nine AgentProfile compatibility constants, ToolRegistry alias, and MustNewToolRegistry forwarder are deleted; every caller is migrated and no replacement seam remains. The pre-existing runtime `Tool` alias remains explicitly outside this slice.
- Focused runtime/sandbox/provider/provideriface/actorruntime/gatewayruntime and exact stub/config/bootstrap tests pass.
- Ratchet passed after canonical regeneration and removal of three fulfilled initial-unused-export debts: production LOC `44338 -> 44224`, exports `1031 -> 1014`, caller edges `520 -> 372`, initial unused-export debt `19 -> 16`, production importers `4 -> 3`, compatibility markers `12 -> 10`; routes, tools, wrappers, store calls, interface candidates, and legacy state counts are flat. Classified historical citers increased `236 -> 241` solely from this slice's durable definition/evidence references; citer growth is intentionally allowed by the ratchet and does not create runtime authority.

## Dispatch Substrate Reconciliation

The implementer returned a clean named branch commit but switched the shared repository worktree onto that branch rather than using the required isolated worktree. No uncommitted or conflicting paths existed. The orchestrator skipped the resulting empty cherry-pick, switched the shared worktree back to canonical `main`, and integrated the same commit as `08049400`. This is a discovered delegation-substrate conformance defect, not a source mutation or unresolved result conflict; the branch commit and canonical integration match.

## S3-I13 Independent Verification

- Independent `S3I13Verifier` returned `PASS` at confidence `0.96` with no findings.
- Verified sandbox import deletion, canonical profile/registry/model-path authorities, exact stub delay/cancellation/event/failure/result/policy/Texture-decision behavior, and absence of duplicate aliases, forwarders, callbacks, fallbacks, constructors, or replacement seams.
- Moving `ConductorSeedPrompt` and `InitialTextureTitle` preserves one authority: runtime declarations are deleted and all callers converge on the provider-owned pure implementations required by exact stub behavior.

## S3-I13 CI, Deploy, and Acceptance

- GitHub Actions run `29220365255` attempt `2` passed every default, integration, race, ratchet, SBOM, and deploy gate for head `7c014386aca694949516d60c380580e47b01f5b6`.
- Deploy job `86725820359` completed successfully.
- Staging health returned `200`/`status=ok`; authenticated `GET https://choir.news/api/agent/loops` returned `200`.

## S3-I13 Final Consensus Blockers

- Codex, Gemini, and OpenCode returned `PASS`; GPT-5.5 returned `BLOCKING` at confidence `0.91`.
- Source blocker: `gofmt -l` found `80` changed Go files non-clean, `53` newly unformatted relative to the parent, including malformed import grouping and an indented `done:` label. The semantic cutover passes, but canonical Go formatting is required before closure.
- Evidence blocker: the implementation receipt incorrectly said every other gate was flat. `citers` increased `236 -> 241` from canonical S3-I13 definition/evidence references. The executable ratchet intentionally permits citer growth while classifying every new citer; this is not source authority growth, but the receipt must state it exactly.
- Smallest repair: correct the receipt, format only changed S3-I13 Go files, rerun focused packages and ratchet, then request final verifier/consensus recheck.

## S3-I13 Formatter Repair Receipt

- Formatted exactly the Go files changed by `6ee86dd1..08049400`; `gofmt -l` is empty afterward.
- Focused tests passed for the runtime, sandbox command, provider, provider-interface, actor-runtime, and gateway-runtime packages.
- Canonical inventory regenerated for formatting-only line removal: production LOC `44224 -> 44216`; every semantic authority count is unchanged. The runtime ratchet passes.

## S3-I13 Repair Verification Blocker

- Independent repair verification found formatting clean, semantic diff neutral, focused tests passing, original authority contract intact, but the repair receipt's code-formatted runtime package name created one new unclassified historical citer (`241 -> 242`).
- Smallest repair: describe the focused package set without a detector-matching source path; do not normalize incidental documentation growth by regenerating the baseline.
