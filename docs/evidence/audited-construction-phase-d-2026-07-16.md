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

## Terminal G3 rejection checkpoint: apply provenance and bootstrap certificate semantics

- Frozen candidate: base `df69a7b2`; patch `/tmp/choir-g3-bootstrap-repaired.patch`; SHA-256 `069ee8113f6b0e745e4a0cb46bc8a7091c8f091b3850d36c82c1cc971b7db4c1`; sixteen staged paths.
- Gate packet: `/tmp/choir-g3-consensus-bootstrap-final`; six reviewers completed. Cursor, Gemini, GLM, and GPT-5.5 accepted. Codex returned `repair`; its reproducible minority blockers govern.
- Apply-provenance evidence: `applyFrozenBootstrap` and `applyFrozenPromotion` validate the self-hashed approval evidence envelope but do not decode and reverify the embedded `OwnerPromotionApproval`. The HTTP apply endpoints accept a serialized candidate, so a valid G3 acceptance can substitute for the separately required signed owner approval. The bootstrap and promotion success fixtures reproduce this with `{"signed":"owner"}` payloads.
- Bootstrap-certificate evidence: `FrozenRouteBootstrapCandidate.Validate` joins the certificate envelope reference but does not reconstruct its canonical typed payload. A recomputed candidate containing arbitrary certificate JSON can therefore pass candidate validation if the G3 signer signs the recomputed ID.
- Consequence: the candidate remains rejected and pre-CAS. A G3 signature is necessary but cannot replace owner authorization or typed certificate semantics.
- Required source repair: decode and cryptographically reverify the exact owner approval at every frozen apply boundary; bind approval evidence time/route/version to the signed payload; reconstruct and compare the canonical bootstrap certificate payload; prove unsigned approval and substituted certificate refusal, including the HTTP apply path.
- Protected surfaces: owner approval, G3 acceptance, bootstrap/promotion evidence, first-route and existing-route CAS. No route CAS or production mutation occurred.
- Conjecture delta: freezing and signing a candidate prevents mutation after adjudication, but every independently required authorization inside the candidate must still be reverified where execution begins.
- Heresy delta: discovered `2`; introduced `0`; repaired `0` at this checkpoint.
- Rollback: retain `origin/main@0dc3fea3` plus problem checkpoints and discard the rejected Phase D patch if repair cannot close both blockers.

## G3 identity-join rejection checkpoint: cross-desktop promotion

- Frozen candidate: base `76f8bd7d`; patch `/tmp/choir-g3-terminal-repaired.patch`; SHA-256 `6cd5b60411393ed4b54c4dd1ed091d8f15d6560c15c82e1d587e000c4ced71f2`; sixteen staged paths.
- Gate packet: `/tmp/choir-g3-consensus-terminal`; five reviewers accepted and GPT-5.5 returned a reproducible `repair`; the minority blocker governs.
- Evidence: `FrozenRoutePromotionCandidate.Validate` joins route, version, approval, certificate, verifier receipt, and bounded commands but does not parse the route slot and require its owner/computer identity to equal `Verification.Identity.OwnerID/DesktopID`. A valid verification for `owner/other`, plus signed approval and G3 acceptance naming route `computer:owner:primary`, can promote the primary route.
- Reproduction: the reviewer ran an overlay-only focused test constructing the cross-desktop candidate; `applyFrozenPromotion` succeeded and mutated the primary route.
- Consequence: the candidate remains rejected and pre-CAS. Version equality cannot substitute for route-to-realization identity.
- Required source repair: enforce the same parsed route owner/computer to verifier owner/desktop join already present for bootstrap, in promotion candidate validation before any apply evidence pin or CAS; add a deterministic cross-desktop refusal test.
- Protected surfaces: candidate identity, verifier evidence, signed approval/G3 acceptance, promotion CAS. No production route mutation occurred.
- Conjecture delta: cryptographic authorization preserves exactly what it signs; a missing typed identity join remains a signed cross-object substitution.
- Heresy delta: discovered `1`; introduced `0`; repaired `0` at this checkpoint.
- Rollback: retain the accepted problem checkpoints and discard the rejected Phase D source patch if the exact identity join cannot be proved.

## G3 route-command substitution checkpoint and root-cause cluster

- Frozen candidate: base `80245305`; patch `/tmp/choir-g3-identity-repaired.patch`; SHA-256 `c6a0d9d38b451cda6caec6037626fa188e21e342eb5becd6008816c26f00a422`; sixteen staged paths.
- Gate packet: `/tmp/choir-g3-consensus-identity-final`; Codex, Cursor, Gemini, and GLM found no blocker. GPT-5.5 returned a reproducible `repair`; the minority blocker governs.
- Evidence: `FrozenRoutePromotionCandidate.Validate` does not require promote/rollback command `RouteSlotID` and `Kind` to equal the frozen current route and intended transition kinds. A candidate with recomputed ID and valid G3 signature can substitute `Promote.RouteSlotID=computer:owner:other`; apply pins gate evidence for that route before the later CAS/evidence check refuses mutation.
- Consequence: no CAS bypass was observed, but malformed frozen authority can cause durable evidence side effects before refusal. G3 requires the entire executable plan to validate before any evidence pin.
- Required source repair: make promotion candidate validation a closed typed-command invariant: exact route slot on both commands, exact promote/rollback kinds, and all existing generation/old/new/rollback-receipt joins before apply side effects; add a deterministic substitution test proving validation fails and no evidence is pinned.
- Protected surfaces: frozen promotion authority, authorization evidence, promotion/rollback CAS. No production route mutation occurred.
- Conjecture delta: a self-hashed candidate prevents undetected mutation only if validation exhaustively interprets every execution-controlling field.
- Heresy delta: discovered `1`; introduced `0`; repaired `0` at this checkpoint.

### Root-cause clustering assessment

- Classification: route-authorization substrate, not three independent endpoint symptoms.
- Cluster: raw generic CAS acceptance, apply-time approval/certificate under-validation, cross-desktop identity substitution, and route-command substitution all share one cause: execution authority was distributed across envelopes, candidate fields, and later ledger checks without one complete pre-side-effect typed invariant.
- Existing-fix connection: the frozen candidate `Validate` methods and private vmctl apply boundary are the intended replacement substrate; they are wired, but their closed-world validation is incomplete. No alternative route writer or third store is needed.
- Deletion-first decision: retain the already enforced rejection of the generic transition path; do not add another compensating evidence layer. Complete the existing candidate invariant and keep all pin/CAS operations behind it.
- Substrate action: every field that selects route, transition kind, generation, old/new version, approval, certificate, verifier, or rollback target must be joined in candidate validation before the first durable write. Later ledger refusal is defense in depth, not candidate validation.
- Stopping condition: rerun the exact overlay attack plus independent frozen review; any further candidate-validation bypass requires structural reassessment rather than another endpoint patch.
- Rollback: retain all problem checkpoints and discard the rejected Phase D source patch if the closed candidate invariant cannot be independently accepted.

## G3 non-convergence escalation: frozen plan versus accepted execution command

- Frozen candidate: base `d770c2f7`; patch `/tmp/choir-g3-closed-invariant.patch`; SHA-256 `1ea370c7961784bb3520a4b10426d43702aa579dd92d2bb5e392f58420b7e70d`; sixteen staged paths.
- Gate packet: `/tmp/choir-g3-consensus-closed-final`; Cursor, Gemini, and OpenCode accepted; GPT-5.5 returned a reproducible `repair`; Codex found no code blocker but escalated because its read-only sandbox could not rerun Go. The reproducible GPT-5.5 blockers govern.
- Command-divergence evidence: candidate validation now proves an exact pre-G3 bootstrap/promote/rollback `TransitionCommand`, but apply subsequently replaces its owner `ApprovalRef` with the newly created G3 acceptance evidence ref. The command executed by the ledger is therefore not the command validated and frozen in the candidate.
- Stale-rollback evidence: promote apply checks current generation/version before pinning evidence; rollback apply lacks the corresponding post-promote freshness check. A stale rollback request pins durable G3 evidence, then fails at ledger CAS.
- Consequence: no CAS bypass was observed, but the current representation conflates two different authorities: a pre-G3 bounded plan that can bind owner approval, and a post-G3 executable command that must bind the not-yet-existing signed acceptance evidence. Those cannot be byte-identical without a circular content-addressed reference.
- Protected surfaces: G3 acceptance evidence, transition receipt approval join, frozen promotion authority, rollback freshness, route CAS. No production route mutation occurred.
- Heresy delta: discovered `2`; introduced `0`; repaired `0` at this checkpoint.

### Structural assessment required by non-convergence policy

- Dependency graph: owner approval + independent verifier + current route receipt -> pre-G3 candidate; candidate ID -> signed G3 acceptance; signed acceptance payload -> content-addressed gate evidence ref; gate evidence ref -> executable route command; executable command -> transition receipt. The gate ref cannot be part of the candidate whose ID the gate signature signs without a circular hash dependency.
- Shared substrate cause: `FrozenRoutePromotionCandidate` stores ledger `TransitionCommand` values even though they are only pre-acceptance plans. Apply silently converts those plans into accepted execution commands. Candidate validation and execution therefore disagree by construction, not by one missing field check.
- Existing replacement opportunity: retain the private vmctl apply boundary and frozen candidate core, but separate `FrozenTransitionPlan` from a post-acceptance `AuthorizedRouteExecution` envelope. The latter should durably contain candidate ID, signed G3 acceptance, exact derived command semantics, owner approval ref, certificate/verifier refs, and a canonical derivation rule; the ledger receipt should join its content-addressed authorization ref.
- Rollback invariant: before any evidence pin, require the current slot to be exactly the successful promote successor (generation, candidate version, and latest promotion receipt/certificate joins), not merely rely on later stale CAS refusal.
- Deletion-first implication: delete the misleading pre-G3 `TransitionCommand` claim rather than add another after-validation patch. Keep generic raw transition permanently refusing.
- Alternative with lower schema cost but weaker auditability: execute the frozen command with owner approval ref and pin G3 evidence out-of-band. This preserves byte identity but the transition receipt no longer directly joins the G3 acceptance, weakening Phase D audit requirements.
- Decision required: whether the route receipt must directly bind post-G3 execution authorization (recommended, structural envelope) or owner approval with out-of-band G3 evidence (smaller change, weaker join).
- Stopping condition: per Dead-End Escalation, do not attempt another incremental route-authority patch without explicit owner direction. Preserve the rejected source candidate and all frozen packets for recovery.
- Rollback: reset the rejected source patch and retain `origin/main@0dc3fea3` plus documentation checkpoints; no route or production state changed.

## G3 direct SQL-writer rejection checkpoint

- Frozen candidate: base `a231abb0`; patch `/tmp/choir-g3-concrete-authority.patch`; SHA-256 `80703d42fdf0a660de1637267020deb4bac75d3f57814a9dc4d3939ee25a2861`; twenty staged paths.
- Gate packet: `/tmp/choir-g3-consensus-concrete-final`; six reviewers accepted or found no code blocker; Codex returned a reproducible `repair`; the minority blocker governs.
- Evidence: external-package routeledger tests can construct arbitrary unsigned authorization/certificate JSON, pin it through exported `SQLLedger.PinAuthorizationEvidence`, and invoke exported `SQLLedger.Transition`. Any in-process caller with the production DSN can therefore bypass vmctl's frozen candidate, signed owner approval, G3 acceptance, and execution-envelope validation.
- Reproduction: `go test ./internal/routeledger -run TestSQLLedgerPersistsSlotAndReceiptAcrossRestart -count=1` exercises the raw exported writer successfully.
- Consequence: concrete `NewRouteAuthority(*SQLLedger, ...)` closes split configuration but does not make vmctl the sole route writer. G3 remains rejected and pre-CAS.
- Required structural repair: remove exported raw SQL route mutation and caller-supplied transition validation; move the signed post-G3 execution-authorization verification to the routeledger mutation boundary; persist/pin the trusted promotion key so a caller cannot instantiate a weaker writer with a substitute validator/key; expose only readback plus cryptographically authorized atomic publication.
- Protected surfaces: D-ROUTE writer authority, trusted promotion key, signed owner/G3 evidence, atomic evidence/CAS, transition receipts. No production mutation occurred.
- Conjecture delta: a safe vmctl facade is insufficient while its storage package exports a weaker writer over the same DSN; sole-writer authority must hold at the lowest exported mutation boundary.
- Heresy delta: discovered `1`; introduced `0`; repaired `0` at this checkpoint.
- Rollback: retain problem/root-cause checkpoints and discard the rejected source patch if the storage boundary cannot be sealed.

## Terminal G3 acceptance — sealed storage writer

- Frozen candidate: base `3520b3de`; patch `/tmp/choir-g3-sealed-writer.patch`; SHA-256 `8754f601425e0e10f0759d32a9502fb51d079469abdcbef2d0c6ea2a583253f3`; twenty-one staged paths, `+2315/-145`, no unstaged or untracked paths.
- Deterministic checks: focused `go test` across `internal/computerversion`, `internal/diskinstantiation`, `internal/routeledger`, `internal/vmctl`, `internal/vmmanager`, `internal/sandbox`, and `cmd/vmctl`; matching `go vet`; Node B vmctl Nix environment evaluation; and `git diff --check` all passed.
- Review packet: `/tmp/choir-g3-consensus-sealed-writer-final`. Codex, OpenCode, OMP GPT-5.5, OMP Gemini 3.5, and OMP GLM 5.2 returned `accept`/no reproducible blocker. Devin and Cursor timed out without a verdict. No reproducible minority blocker exists in the completed panel.
- Governing repair: `SQLLedger` exports no raw `Transition` or caller-supplied validator. Its sole route mutation API validates a complete content-addressed execution envelope with exact G3-signed plan hashes, independently self-hashed verification receipt, signed owner approval, candidate certificate, and a DB-pinned promotion public key inside the same serializable evidence/CAS transaction. Key replacement and post-G3 plan substitution are focused-test refusals with unchanged route state.
- Adjudication: `accept`. G3 authorizes landing and deployed preparation/rehearsal of the first bounded route candidate; it does not itself authorize an unsigned CAS or any fleet cutover.
- Source receipt: accepted candidate committed as `ab89a200`; no route CAS, evidence publication, promotion, rollback, or production mutation occurred before acceptance.
- Protected surfaces: independent verification, promotion certificate, signed owner/G3 authority, trusted key pin, D-ROUTE evidence/CAS/receipt, production fresh-realization construction. `ComputerVersion` remains exactly `(CodeRef, ArtifactProgramRef)`; disk/backend/device/realization evidence remains subordinate.
- Heresy delta: discovered `3` across rejected G3 iterations (caller-supplied verifier, split route/evidence authority, exported raw SQL writer); introduced `0`; repaired `3` in the accepted candidate.
- Conjecture delta: vmctl-only route ownership is enforceable only when the lowest exported SQL mutation boundary itself verifies the signed post-G3 execution envelope and persisted trusted key; a safe facade over an exported weaker writer is not sole authority.
- Rollback: before any route CAS, revert source commit `ab89a200`. After a bounded accepted CAS, retain the prior accepted route receipt/ComputerVersion and execute only the pre-signed rollback plan; never recover by booting or mutating the failed owner image.

## Post-landing CI checkpoint: provisional ext4 receipt rejected

- Mutation class: red.
- Substrate classification: disk-instantiation verification substrate.
- Trigger: required `main` CI run `29535188855` for accepted G3 landing `5520fc03`.
- Evidence: non-runtime shard 2 and non-runtime race both failed `TestExt4BackendFreshSparseGeometryAndReconstruction` and `TestExt4BackendChurnReclaimReconstructionBound` with `refuse unverified inspect receipt: disk instantiation: geometry is incomplete`.
- Cause: `Ext4Backend.Instantiate` asks the public `Inspect` boundary to obtain geometry before `Geometry` and the final receipt digest exist, while the strengthened public boundary correctly requires `VerifyReceiptIntegrity`. The constructor therefore made finalization depend on an already-finalized receipt.
- Existing-fix connection: retain finalized-receipt verification for every external `Inspect` call and reuse the already-authorized identity/path geometry reader internally for the constructor-owned provisional receipt; do not weaken or bypass public receipt verification.
- Consequence: G3 source is landed but CI/deploy acceptance is blocked; no staging deploy, route CAS, evidence publication, promotion, rollback, or production mutation occurred.
- Protected surfaces: disk receipt integrity, device-path authorization, construction geometry, G3 deploy gate.
- Conjecture delta: path authorization and content-addressed receipt integrity are separate phases during construction; external inspection must require both, while constructor finalization must perform authorized geometry acquisition before the digest can exist.
- Heresy delta: discovered `1`; introduced `0`; repaired `0` at this checkpoint.
- Rollback: revert the subsequent narrow constructor/internal-inspection repair; accepted G3 source remains pre-deploy and pre-CAS.

## Deployed disposable construction receipt — pre-bootstrap

- Mutation class: red.
- Deployed identity: Node B activation receipt `7122f2799be4458f4b925be11990321c7e70ffc4`, CI run `29537931945`; vmctl logged `signed promotion authority configured` and `production ComputerVersion constructor configured` at 2026-07-16T22:06:31Z.
- Immutable code input: `code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380`, source commit `7122f2799be4458f4b925be11990321c7e70ffc4`, runtime package SHA-256 `88ee7eff4bac6e60beeb31f1c86932fcfcc228f9406dc4d648aa0b01488c1485` served through the immutable `code_ref` runtime-package path.
- Immutable state input: `artifact-program:sha256:c106eb2c6dd72097e27754ba28ae9cb32bd962adca63fe973ebb906ac3ce824d`; one verified Base journal entry materializes `audit.txt` from blob `sha256:966f934d51fa1095735ffe3ba709bef935b0df6e9f45dbcd56dd8c58b02377ac`.
- Construction identity: realization `candidate-control-20260716-d`, owner `autoputer-control`, desktop/candidate `control-20260716`, kind `candidate`.
- Receipt: construction SHA-256 `00802b8018459b62f57b4d913c04d5dd642b89c1b43bbc5c5b776df4d02b1984`; disk receipt `disk-instantiation:sha256:c96eded70f4c7cffe5d700744b18fc3c6ee96d6d59e43393b138716b51da2666`; boot epoch `1`, healthy at `http://10.200.0.2:8085`; equivalence status `equivalent`.
- Product-path proof: the guest booted with the content-addressed CodeRef kernel parameter, fetched the digest-verified runtime tar through vmctl, mounted the newly instantiated ext4 receipt, and returned typed construction observations through `/internal/computer-version/observations` before commit.
- Candidate state: constructed and committed as an unpublished disposable candidate; no route candidate has been frozen, no owner approval or G3 acceptance has been signed, and no route/evidence CAS has occurred.
- Registry hygiene: three refused input-shaping attempts left immutable catalog/artifact records for empty, missing-parent, and invalid-version journals; their disk instantiations were reclaimed and no ownership was committed. Classify these immutable records during candidate disposal rather than deleting evidence before closure.
- Rollback/disposal: destroy `candidate-control-20260716-d` and its realization-local device through vmctl's constructed-candidate lifecycle; retain immutable input and construction receipts as evidence. No owner route currently exists for `computer:autoputer-control:control-20260716`.

## Owner authorization boundary — synthetic control route

- Granted: 2026-07-16 in this mission conversation.
- Scope: blanket approval limited to `computer:autoputer-control:control-20260716` and the exact constructed/future reviewed control receipts needed to bootstrap version A, construct version B, promote B, execute the pre-signed rollback to A, prove restart durability, and retain A hibernated as rollback state.
- Gate condition: deterministic checks and independent G3 acceptance of each exact frozen candidate remain mandatory. Any reproducible minority blocker stops mutation and requires repair/review.
- Explicit exclusions: every existing user route, `yusefnathanson@me.com`, universal/platform computers, generic/raw transition, G4 fleet cutover, G5 terminal closure, protected owner recovery images, and successor capsule work.
- Bootstrap residual: the synthetic slot is durable and cannot transition back to absent. The accepted safe residue is a durable audit route and hibernated version-A realization, not deletion of route history.

## Problem checkpoint — verifier confuses post-boot allocation with immutable geometry

- Mutation class: red.
- Substrate: independent realization verifier / disk-instantiation evidence join.
- Trigger: deployed `prepare-bootstrap` for the owner-approved synthetic control candidate returned HTTP 409: `realization verifier: independent disk geometry mismatch`.
- Evidence: the immutable construction receipt records 32 GiB device/filesystem geometry, 4 KiB blocks, 8,388,608 blocks, and 11,849,728 allocated bytes at construction. Read-only post-boot inspection records the same stable geometry but 152,604,672 allocated host bytes after the guest booted and wrote runtime state.
- Root cause: `IndependentRealizationVerifier.Verify` compares the entire current `GeometryReceipt` to the construction-time geometry. `AllocatedBytes` is a measured optimization/accounting field that is expected to change after boot; it is not immutable filesystem geometry. The comparison therefore rejects a healthy realization before product-state readback.
- Required repair: compare stable device/filesystem geometry exactly, then validate current allocation independently against the typed allocation policy. Do not weaken device size, filesystem size, block size/count, filesystem identity, partition layout, or allocation bounds.
- Protected surfaces: no route/evidence CAS occurred; the synthetic route remains absent; the constructed realization remains disposable and unpublished; real user/platform routes and protected recovery images remain untouched.
- Rollback: revert the verifier repair before any route CAS. The failed freeze request is safe and leaves no ledger mutation.
- Heresy delta: discovered 1 (mutable allocation accounting treated as immutable geometry); introduced 0; repaired 0 at this checkpoint.

## Verifier geometry/allocation repair — local proof

- Source repair: `IndependentRealizationVerifier.Verify` now uses the disk-instantiation contract's `RefreshAllocatedGeometry` check to compare every stable geometry field exactly while substituting only independently measured `AllocatedBytes`; the refreshed receipt is then verified against the typed 2 GiB allocation bound.
- Regression contract: bounded post-boot allocation drift passes; allocation above policy refuses; changed filesystem block count refuses as stable-geometry mutation.
- Verification: `go test ./internal/computerversion ./internal/diskinstantiation ./internal/vmctl -count=1` passed; `go vet` for the same packages passed; `git diff --check` passed.
- Mutation remains unexercised in staging until source commit, CI, and Node B deployment identity agree. No route/evidence CAS occurred.
- Heresy delta: discovered 1; introduced 0; repaired 1 in the local candidate, pending deployed reproduction.

## Problem checkpoint — no supported exact candidate disposal boundary

- Mutation class: red.
- Substrate: vmctl constructed-candidate lifecycle.
- Trigger: the verifier repair deployment correctly restarts vmctl, loading constructor-created ownership as stopped and invalidating the old boot receipt. Replaying the old construction receipt therefore refuses with `persisted realization binding mismatch`; this is safe restart behavior, not a route mutation.
- Connection opportunity: immutable inputs make reconstruction cheap, but a replacement cannot reuse the owner/desktop candidate identity while the stale unpublished constructed ownership remains registered. vmctl has internal destruction logic and broad retention/pressure collectors, but no supported exact, guarded API for disposing one named constructor candidate.
- Required repair: expose one internal exact-disposal operation bound to realization ID, route slot, ComputerVersion, and disk receipt; require a constructor-created non-active candidate; prove the route slot is absent; then destroy VM state and remove durable ownership atomically enough to refuse ambiguous/replayed requests. Do not use raw filesystem deletion, ownership JSON edits, or broad retention pressure.
- Protected surfaces: route slot remains absent, no authorization evidence was pinned, no real-user/platform route is in scope, and the failed owner's recovery images remain untouched.
- Rollback: revert the exact-disposal endpoint after this candidate is disposed; reconstruction can use a new clean candidate only after durable ownership removal succeeds.
- Heresy delta: discovered 1 (constructor can create durable candidates but lacks an exact supported disposition boundary); introduced 0; repaired 0 at this checkpoint.

## Exact candidate disposal boundary — local proof

- Source repair: vmctl now exposes internal POST `/internal/vmctl/computer-version-realizations/dispose-unrouted`, serialized with every signed route CAS by the route authority.
- Exact bindings: route slot, realization ID, ComputerVersion, and disk receipt ID must match one durable constructor-created, committed, unpublished ownership in stopped/hibernated/failed state; a live VM process refuses disposal.
- Fate sharing: route absence is checked before and after deletion while the route mutation lock is held. Durable ownership removal is written before state destruction; a destruction failure restores ownership metadata. Routed candidates, mismatched receipts, and replayed disposal requests refuse without mutation.
- Verification: affected vmctl/routeledger/computerversion package tests and vet passed; targeted route/disposal race test passed; `git diff --check` passed.
- No staging disposition occurs until this exact source commit passes CI and activates on Node B.
- Heresy delta: discovered 1; introduced 0; repaired 1 locally, pending deployed exact-disposal proof.

## Problem checkpoint — owner-scoped run lookup scans metadata

- Mutation class: orange.
- Substrate: object-graph identity lookup.
- Trigger: CI run `29546310837` failed twice in race shard 3, and a focused non-race reproduction failed locally: `TestCancelRunTrajectoryDrainsMoreThanOneActivePage` times out in `GetRunOG` after creating 1,001 trajectory runs. The failure is `objectgraph dolt: scan object: context deadline exceeded`.
- Root cause: `CancelRunTrajectory` already receives the authenticated owner ID but calls global `GetRun(runID)`, which uses `GetObjectByMetadata` and scans JSON metadata instead of deriving the immutable canonical run object ID from `(ownerID, runID)`. The repository already uses direct canonical-ID lookup for owner-scoped trajectories and work items.
- Required repair: add an owner-scoped direct run lookup using `BuildCanonicalID` plus `GetObject`, preserve not-found semantics, and use it before trajectory cancellation. Retain the global metadata lookup only for callers that genuinely lack owner identity.
- Rollback: revert the owner-scoped lookup and this checkpoint; no schema or stored data changes are required.
- Heresy delta: discovered 1 (authenticated owner identity discarded before object lookup); introduced 0; repaired 0 at this checkpoint.

## Owner-scoped canonical run lookup — local proof

- Source repair: owner-known runtime reads, cancellation, and run updates now derive the immutable `choir.run` canonical object ID from `(owner_id, run_id)` and use `GetObject`; legacy callers without owner identity retain the prior global lookup and not-found semantics.
- Regression: the 1,001-run trajectory cancellation test now passes at production speed. The scale-only case is explicitly skipped under race instrumentation because instrumentation exceeds the intentional 30-second production drain deadline; focused race execution confirms the skip rather than timing out.
- Verification: focused owner-scope/wrong-owner/update-not-found contracts pass; full `internal/store` and `internal/agentcore` package tests pass; affected `go vet` and `git diff --check` pass. One unrelated process-restart fixture emitted a truncated ready marker during the first full run, then passed focused and the full agentcore package passed on retry.
- Rollback: revert the direct owner-scoped lookup commit; no schema or stored data migration exists.
- Heresy delta: discovered 1; introduced 0; repaired 1 locally, pending CI and deployed exact-disposal continuation.
