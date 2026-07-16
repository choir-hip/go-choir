# Audited Construction Phase D Evidence — 2026-07-16

## Problem checkpoint: constructor accepts an unverified CodeClosure

- Mutation class: red.
- Substrate classification: immutable-input authority substrate.
- Trigger: the first deterministic run of the independent realization verifier against the accepted Phase C constructor.
- Command: `go test ./internal/computerversion ./internal/routeledger ./internal/vmctl -count=1 && go vet ./internal/computerversion ./internal/routeledger ./internal/vmctl`.
- Receipt: `TestProductionMaterializerConstructsAndReadsBackLiveState` failed with `realization verifier: CodeRef binding: computer input resolver: incomplete code closure`.
- Source evidence: `ProductionMaterializer.Construct` checks the resolved `CodeRef` and the single sandbox artifact name, but does not call `CodeClosure.Verify` before disk creation and launch (`internal/computerversion/production_materializer.go`). Its success fixture therefore used a ref-only closure that the independent verifier correctly refused.
- Consequence: a constructor can reach disk and VM mutation with an immutable input record that does not prove its source commit, closure digest, URI, and artifact digest binding. Phase D cannot freeze a promotion candidate from that construction.
- Existing-fix connection: the newly implemented independent verifier already invokes `CodeClosure.Verify` and fails closed. The source repair is to invoke the same immutable value contract in the constructor before any disk or VM mutation and to make the production success fixture use a complete closure; no alternate store, route writer, or compatibility path is needed.
- Protected surfaces: immutable CodeRef authority, construction receipt, realization verification, promotion authorization. No route CAS or production mutation occurred.
- Conjecture delta: independent post-construction verification exposed a missing pre-mutation constructor invariant; a matching ref and artifact count are not an immutable closure proof.
- Heresy delta: discovered `1`; introduced `0`; repaired `0` at this checkpoint.
- Rollback: revert the subsequent constructor validation change and Phase D verifier/promotion candidate commit. Existing D-ROUTE state remains untouched.
