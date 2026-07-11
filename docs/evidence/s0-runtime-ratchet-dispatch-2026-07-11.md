# S0 Runtime Inventory And Ratchet Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S0-runtime-inventory-ratchet-01`
- Dispatch nonce: `s0-runtime-inventory-ratchet-01-nonce-01`
- Transition: `s0-runtime-inventory-ratchet-dispatch-intent-01`
- Canonical parent: `1a9a90b63f6541fcb8d96502e85a158b8446d14e`
- Mutation class: yellow

## Target

Create the mechanically checkable S0 inventory and ratchet for `internal/runtime`. Exact mutation targets are:

- `cmd/runtime-ratchet/**` (new command and focused tests);
- `docs/runtime-dissolution-inventory.yaml` (new complete baseline/disposition artifact).

Do not modify `internal/runtime`, runtime callers, routes, registrations, lifecycle authority, Wire authority, promotion authority, suite/registry documents, CI, or deployment configuration.

## Change

1. Reuse repository Go and YAML conventions; do not introduce dependencies.
2. Inventory every production and test Go file under `internal/runtime`, every exported declaration, every route registration and tool registration owned by that package, every production external importer, every `*runtime.Runtime` / `*runtime.APIHandler` wrapper or embed, compatibility markers, relevant state writers, and every literal `internal/runtime` citer across the suite-declared surfaces.
3. Give every required item an explicit disposition. Production files/exports/routes/tools/callers use only `delete`, `core`, or a concrete domain. Citers use only `delete`, `redirect_to_successor`, `deletion_target_reference`, `historical_evidence`, or `block`. No `later`, wildcard, or silent allowlist.
4. Make the command derive the current inventory from Go syntax and repository contents, compare it to the checked-in baseline, and fail on missing/extra items, count increases, invalid dispositions, wrappers/aliases, new production symbols without production callers, or unclassified citers. The baseline may establish current nonzero debt; it must prevent growth and expose exact drift.
5. Emit deterministic, reviewable output suitable for every S3 iteration. Preserve exact paths and declaration/registration identities.
6. Add focused behavioral tests for a clean baseline and plausible regressions: an added runtime production file/export/importer/citer, an undispositioned item, and an invalid disposition.

## Acceptance

- `go test ./cmd/runtime-ratchet` passes.
- The ratchet command passes against the checked-in baseline and prints the fresh counts required by S0.
- A focused test demonstrates each named regression fails with an actionable diagnostic.
- Inventory has no unclassified required item and no `later` bucket.
- The implementation agent does not run project-wide tests or formatters and returns the implementation commit/patch plus exact focused commands and results.
