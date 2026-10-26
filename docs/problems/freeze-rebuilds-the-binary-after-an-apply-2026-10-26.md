# Freeze rebuilds the Go binary for a frontend-only follow-up after an apply

Date: 2026-10-26. Status: open; documented before the fix. Mutation class of the fix: orange (capsule Freeze reuse rule, additive).

## Evidence

Trace reviews round 6 (R6-4) and round 7 (R7-4), docs/evidence/selfdev-trace-reviews-2026-10-25.md: on b@c, a follow-up round that changed only frontend files had its first Freeze refused, "capsule release must contain executable bin/autoputer: the running binary was built from a03d5266 but the capsule source is pinned to 0f9269ee; build bin/autoputer", and engineering ran `go build` in the capsule: 4m48s in round 7, 7.6 minutes in round 6. Round 3, before any apply, reused the running binary.

## Cause (read in code at origin/main; one link is a hypothesis)

- The refusal is `PlanReleaseBinaryReuseWith` (internal/capsule/release_binary.go:160-204), the branch at about 182-184 taken when the differ is nil. The differ comes from `reuseDiffer()` (internal/capsule/executor.go:288-293), which is nil unless `reuseByInputDiff` is set.
- The binary-input reuse rule already exists: `BinaryInputPath` (release_binary.go:73-95: go.mod, go.sum, go.work*, flake files, cmd/, internal/, vendor/, nix/, non-test .go) and `GitPathDiffer` (release_binary.go:222-250) return `ReuseProofBinaryInputsEqual` when no changed path is an input; `ValidateReleaseBinary` accepts that proof, and `CheckReusedBinaryIsRunning` re-checks at apply.
- It is switched on once, at executor construction: internal/autoputer/capsule_executor_linux.go:71-73 calls `SetBinaryReuseByInputDiff(true)` only `if sourceproj.Enabled()`. The executor is built early in boot (internal/autoputer/run.go:252); the per-computer setting `source.projection` is loaded later in `Runtime.Start` (internal/agentcore/runtime.go:957, source_projection.go:37), and the runtime toggle (`applySourceProjectionSetting`, source_projection.go:43-51) never reaches the executor. No nix file sets `CHOIR_SOURCE_PROJECTION`.
- So on a computer where projection is on by setting only (b@c), the capsule pin moves to the projection commit after an apply while the running binary keeps the booted base commit, and the reuse rule that would prove "no Go input changed" is never consulted. Hypothesis to confirm: b@c's autoputer boot log lacks "capsule executor: source projection on; binary reuse by binary-input diff".
- No `//go:embed` covers frontend/ (eleven embeds, all under internal/), so a frontend-only change leaves the binary unchanged and reuse is correct.

## Fix (next)

Make the gate live instead of read once: the executor asks a gate function (installed by the linux constructor as `sourceproj.Enabled`) each time it builds the differ. Fails closed as today when the base objects are missing (the differ errors and Freeze asks for a build). Rollback: revert, or turn source.projection off.
