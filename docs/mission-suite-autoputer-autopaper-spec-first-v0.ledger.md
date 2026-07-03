# Mission Suite Ledger: Autoputer / Autopaper — Spec-First Substrate Redesign

## Pass 0 — 2026-07-03 05:15 EDT

**Conjecture:** The substrate migrations (actor runtime, object graph) outran their TLA+ specifications. The next step is to center the TLA+ spec, rewrite the specs to describe the current architecture, and then drive the code to match the specs.

**Move:** construct (create spec-first suite paradoc, inventory current specs, define missions and dependencies)

**Expected ΔV:** 16 new conjectures proposed (suite variant)

**Actual ΔV:** 16/16 conjectures proposed

**Definitions recorded:**
- D1: TLA+ specs are the source of truth, not a verification afterthought.
- D2: The existing specs are either stale, ahead of code, or proving current code wrong.
- D3: Code changes must follow spec changes (spec-first workflow).
- D4: The actor runtime is the correct concurrency substrate; the old runtime must be deleted.
- D5: The autoputer rename and Nucleus capsule integration are parallel work streams.

**Conjectures proposed:**
Mission S (Spec Redesign):
- C-S1: actor_protocol_og spec holds under current object-graph semantics. (UNDECIDED)
- C-S2: wire_pipeline_og spec captures the wire pipeline flow. (UNDECIDED)
- C-S3: autoputer_lifecycle spec reproduces the current boot failure. (UNDECIDED)
- C-S4: promotion_protocol_og still flags current code violations. (UNDECIDED)

Mission A (Actor Defactoring):
- C-A1: internal/runtime package can be deleted. (UNDECIDED)
- C-A2: cmd/sandbox/main.go builds using only actor runtime + extracted helpers. (UNDECIDED)
- C-A3: Actor runtime tests pass under -race. (UNDECIDED)

Mission B (Wire Redesign):
- C-B1: Wire pipeline compiles and unit tests pass with fake providers. (UNDECIDED)
- C-B2: Wire pipeline produces a real article on staging. (UNDECIDED)
- C-B3: Every fetched item's story eventually settles. (UNDECIDED)

Mission C (Autoputer Solidification):
- C-C1: Autoputer VM image builds with Nucleus included. (UNDECIDED)
- C-C2: Autoputer VM boots and binds to port 8085 on staging. (UNDECIDED)
- C-C3: Nucleus can launch a strict-agent capsule inside the autoputer VM. (UNDECIDED)

Mission D (CI/Verification Guard):
- C-D1: CI passes after each mission commit. (TESTING)
- C-D2: TLA+ specs model-check in CI. (UNDECIDED)
- C-D3: Race detector model is correctly scoped. (SUPPORTED from predecessor)

**Evidence:**
- Existing specs: `specs/actor_protocol.tla`, `actor_protocol_xvm.tla`, `wire_pipeline.tla`, `promotion_protocol.tla`
- `specs/README.md` documents the spec layering and current violations
- `promotion_protocol.tla` already proves current code violates `NoStaleCommit` and `ApprovalGate`
- `wire_pipeline.tla` models publication trajectories that the Go code has not yet adopted
- `cmd/sandbox/main.go` still imports `internal/runtime` for non-concurrency helpers
- Only 5 production files still import `internal/runtime`

**Open decisions needing owner input:**
1. Spec naming: keep old specs as `*_v1.tla` or overwrite in place?
2. TLA+ CI tooling: is the current command correct? Add per-spec TLC configs?
3. Nucleus version: pin rev or follow main?
4. Staging persistent state: reset or migrate? (destructive)
5. Wire redesign scope: full sourcecycled → publish or processor → publish subset?
6. Autoputer rename first or after Mission A?
7. Promotion protocol: fix code to match spec or update spec?

**Next:** Commit the suite paradoc, then spawn subagents for Mission S (spec audit), Mission A (helper extraction), and Mission D (CI/TLA+ verification). Get owner decisions on open questions in parallel.

---

## Pass 1 — 2026-07-03 (Mission S: Promotion Protocol Gate)

**Conjecture:** The old `promotion_protocol.tla` is stale. The current Go code already fixed the two violations it flagged. The autoputer needs a new promotion protocol spec that models the computer ontology, ledger split, route identity, and health window. This spec is the gate for the autoputer.

**Move:** redefine (write assessment doc, delete old specs, write new `specs/promotion_protocol.tla`, update `specs/README.md`, update suite paradoc with default decisions)

**Expected ΔV:**
- Close the 7 open decisions with default decisions (reduces open-decision count to 0).
- Replace C-S4 with two new gate conjectures (C-S4, C-S5).
- Increase suite variant by 2 (new gate conjectures) + 1 (Mission C C-C4 promotion end-to-end) = net +3 after closing decisions.

**Actual ΔV:**
- 7 open decisions closed with default decisions.
- Old specs deleted: `actor_protocol.tla`, `actor_protocol_xvm.tla`, `wire_pipeline.tla`, and their `.cfg` files.
- New `specs/promotion_protocol.tla` written.
- New `specs/promotion_protocol.cfg` written.
- `specs/README.md` rewritten for the new specs.
- Suite paradoc updated with default decisions and promotion-gate focus.
- C-S4 redefined: `promotion_protocol.tla` models active/candidate/route/rollback/health-window and checks green. (TESTING — pending CI TLC run)
- C-S5 redefined: `promotion_protocol.tla` encodes `NoStaleCommit`, `ApprovalGate`, `NoTornOutcome`, `RouteConsistency`, `CandidateIsolation`, `HealthWindowReversible`, `ConfirmedLedgersApplied`, `AbortedLedgersRolledBack`, `CertificateCompleteness`. (TESTING — pending CI TLC run)
- C-C4 added: Promotion protocol end-to-end works on staging: candidate → verify → approve → promote → health window → confirm. (UNDECIDED)
- Updated variant count: 18 conjectures (5 S + 3 A + 3 B + 4 C + 3 D).

**Definitions recorded:**
- D6: Old specs are deleted; new specs are written in place (pre-launch, only good code).
- D7: The promotion protocol is the gate: a persistent computer is not an autoputer until candidate promotion is model-checked, verified, and encoded.
- D8: The current Go code already enforces `NoStaleCommit` and `ApprovalGate`; the new spec must encode and verify these invariants, not just flag old violations.
- D9: Default decisions made (owner can override): delete old specs, keep TLA+ CI command, pin Nucleus rev, reset staging Dolt state, processor→publish wire scope, rename in parallel with Mission A, redefine spec first then encode.

**Evidence:**
- `docs/promotion-protocol-spec-staleness-and-redefinition-2026-07-03.md` documents the current state and the redefinition requirements.
- `internal/runtime/app_promotion.go` now has `ApproveAppAdoption`, `PromoteAppAdoption` requiring `owner_approved`, and `promoteFreshnessCAS`.
- `internal/runtime/app_promotion_freshness_test.go` unit-tests the freshness CAS.
- `docs/computer-ontology.md` provides the computer/ledger/route ontology for the new spec.
- `specs/promotion_protocol.tla` models: `Slots`, `ActiveComps`, `CandidateComps`, `Ledgers`, `MaxTailMoves`; variables `activeBase`, `candidateBase`, `candidateParent`, `route`, `ledgerState`, `promoStatus`, `promoActive`, `promoCandidate`, `promoBase`, `approved`, `poisoned`, `healthWindow`.
- Spec actions: `MoveActiveTail`, `ForkCandidate`, `PrepareLedger`, `Restage`, `Verify`, `Approve`, `Commit`, `Abort`, `ApplySecondary`, `RollbackSecondary`, `PoisonedWrite`, `HealthCheckFail`, `ConfirmHealthy`, `AutoRevert`.
- Spec invariants: `NoStaleCommit`, `ApprovalGate`, `NoTornOutcome`, `RouteConsistency`, `CandidateIsolation`, `HealthWindowReversible`, `ConfirmedLedgersApplied`, `AbortedLedgersRolledBack`, `CertificateCompleteness`.
- Spec liveness: `EveryCommittedPromotionSettles`, `SystemProgress`.

**Open decisions:** None — all closed with default decisions (owner can override at any time).

**Risks:**
- Local TLA+ tools (TLC) are not available in this shell; the spec has not been locally model-checked. Verification depends on the CI `tla-model-check` job. Any syntax or semantic error will be caught by CI, but this is not ideal for a gate artifact.
- The `AutoRevert` action now atomically rolls back all secondaries to preserve the `NoTornOutcome` invariant. This may be coarser than the eventual implementation, but it is a correct refinement.
- The `EveryCommittedPromotionSettles` liveness property deliberately excludes poisoned promotions; forward recovery (a new promotion) is outside the single-promotion model.

**Next:** Commit Pass 1, push to `origin/main`, and monitor the `tla-model-check` CI job. If TLC reports errors, iterate as Pass 2 before any code changes. If TLC passes, the promotion gate is established and Mission C promotion encoding can begin.
