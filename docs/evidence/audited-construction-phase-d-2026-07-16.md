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

## G3 rejection checkpoint: route bypass and realization splice

- Frozen candidate: base `3a51e00b`; patch `/tmp/choir-g3-verifier-promotion.patch`; SHA-256 `8bb9d2dd510a076d10dd76640493e72522b05789d776d45ad121f51403bd1202`; nine staged paths.
- Gate packet: `/tmp/choir-g3-consensus-trusted`; six reviewers completed. Cursor, Gemini, and GLM accepted. Codex and GPT-5.5 independently returned `repair`; minority blockers govern.
- Route-bypass evidence: the generic vmctl transition endpoint accepts a raw generation-correct `TransitionCommand`. SQL evidence verification joins only evidence kind, route slot, and candidate `ComputerVersion`; it does not require the frozen candidate ID, independent verification receipt, certificate payload, or an accepted G3 adjudication. An internal caller can therefore pin structurally valid self-hashed envelopes and reach route CAS without preparation or gate acceptance.
- Realization-splice evidence: the prepare endpoint accepts a caller-supplied `ConstructionResult`; verification passes its `BootReceipt.HostURL` to `VMConstructionLauncher.Observe` and separately inspects the disk receipt. Neither operation independently resolves the VM identity/epoch/host/device attachment. A caller can pair disk A with endpoint B when semantic state and geometry match. `Ext4Backend.Inspect` also lacks the stronger realization/device path identity check already used by reclaim.
- Approval-provenance evidence: `OwnerApproved` is inferred from a self-hashed approval envelope. The payload is not decoded or joined to an owner-issued record, frozen candidate, or G3 acceptance.
- Consequence: the frozen candidate does not authorize a first D-ROUTE CAS. It remains pre-CAS; no evidence was pinned and no route was changed.
- Required source repair: make pre-G3 output a non-executable frozen plan; require an accepted gate artifact and exact candidate/certificate/verifier joins in every promote CAS; authenticate/join typed owner approval; independently resolve live VM/epoch/host/device identity; strengthen device-path authorization and preserve the VM/device observation join.
- Protected surfaces: verifier/Trace evidence, owner approval, D-ROUTE promotion and rollback, candidate computer identity, vmctl, production route CAS.
- Conjecture delta: server-side verifier execution closes receipt forgery but not caller-supplied realization splicing; vmctl-only CAS ownership does not itself enforce G3 acceptance.
- Heresy delta: discovered `3`; introduced `0`; repaired `0` at this checkpoint.
- Rollback: retain `origin/main@0dc3fea3` and discard the rejected Phase D worktree patch if repair cannot close all minority blockers.
