:# Promotion Protocol: Spec Staleness and Autoputer Redefinition

**Status:** problem documentation / assessment (docs-only, green class)  
**Date:** 2026-07-03  
**Mutation class:** green (no code change; this is the doc-first assessment)  
**Protected surfaces:** promotion/rollback, computer lineage, route identity

---

## The Problem

The existing TLA+ spec `specs/promotion_protocol.tla` and its design companion `docs/choir-promotion-protocol-conjecture-2026-06-11.md` claim that the current Go implementation violates two intended invariants:

- `NoStaleCommit` — promotion can fire against a foreground lineage that moved since verification.
- `ApprovalGate` — promotion can fire without owner approval because `owner_approved` is a dead status.

That diagnosis was correct on 2026-06-11. It is no longer correct. The Go code has been repaired:

1. `ApproveAppAdoption` (internal/runtime/app_promotion.go:411-435) now enforces the `owner_approved` transition.
2. `PromoteAppAdoption` (internal/runtime/app_promotion.go:456-505) now rejects any status other than `owner_approved`.
3. `promoteFreshnessCAS` (internal/runtime/app_promotion.go:441-454) now records `lineage_ref_at_verification` and compares it against the current `ActiveSourceRef` at promotion time. It is unit-tested in `internal/runtime/app_promotion_freshness_test.go`.

The old spec is now a model of a prior architecture, not the running system. It does not describe the autoputer. It does not model the computer ontology from `docs/computer-ontology.md`. It does not model the ledger split, the promotion certificate, the route identity, or the Nucleus capsule boundary. It is stale and must be replaced.

---

## What the Current Go Code Actually Does

The current implementation is a partial, single-ledger promotion protocol focused on `source_build` state:

1. **Publish** (`PublishAppChangePackage`) — a candidate produces a typed `AppChangePackage` with source deltas, verifier contracts, and artifact digests. This is the prepare artifact for the source/build ledger.
2. **Create adoption** (`CreateAppAdoption`) — a target computer records a candidate ref and captures the active source ref at candidate-start. This is the fork point.
3. **Verify** (`VerifyAppAdoption`) — the recipient side rebuilds the candidate from source and runs verifier contracts. This is the `source_build` ledger prepare/verify step.
4. **Approve** (`ApproveAppAdoption`) — owner authorizes the verified promotion.
5. **Promote** (`PromoteAppAdoption`) — checks owner approval, checks artifact digests, checks the freshness CAS, then flips the target computer's `ActiveSourceRef`, `RuntimeDigest`, `UIDigest`, and `RouteProfile` in `ComputerSourceLineage`. This is the route-pointer commit.
6. **Rollback** (`RollbackAppAdoption`) — restores the previous active source ref, runtime digest, UI digest, and route profile from the rollback profile JSON.

What is missing from the current implementation (and must be in the new spec):

- **Dolt/app ledger** — app state, textures, prompts, traces are not part of the promotion certificate.
- **VM/OS ledger** — the running VM image and runtime services are not included.
- **Blob/content ledger** — generated media and uploads are not tracked.
- **Artifact/provenance ledger** — verifier results, trace refs, and promotion certificates are not durable graph objects.
- **Promotion certificate** — there is no durable certificate object that records all ledger states, merge results, conflicts, and route transition.
- **Health window / auto-revert** — after promotion, there is no explicit post-commit health window or auto-revert mechanism.
- **Poisoned-write / N-1 rollback** — the rollback window is not tracked, so a torn rollback from a schema change is possible.
- **Capsule isolation** — risky effects are not captured inside Nucleus capsules.

---

## Why the Promotion Protocol Is the Autoputer Gate

`docs/computer-ontology.md` defines the product object as a persistent **computer**, not a sandbox. A computer is composed of heterogeneous ledgers that must be promoted together or explicitly excluded. The user experiences one active computer at a time; a candidate computer is a speculative fork that may become the active computer.

The autoputer is the implementation service that hosts these persistent computers. It is not an autoputer until it can:

1. Fork an active computer into a candidate.
2. Apply typed mutations to the candidate in isolation (ideally inside Nucleus capsules for risky effects).
3. Verify the candidate against independent contracts.
4. Require owner approval before the candidate can become active.
5. Atomically flip the route identity from the active computer to the candidate.
6. Roll back if the post-commit health window fails.
7. Close the rollback window once the candidate has written data the previous active computer cannot read (poisoned write / N-1 rule).

That is exactly the promotion protocol. Until the promotion protocol is model-checked and encoded, the autoputer is just a renamed sandbox with a borked runtime. The promotion protocol is the gate.

---

## Redefinition Requirements for the New Spec

The new `specs/promotion_protocol.tla` must:

1. Model the computer ontology:
   - a set of **active computers**;
   - a set of **candidate computers** derived from active computers;
   - a **route pointer** that maps a user/cloud slot to the currently active computer.
2. Model the ledger split with per-ledger prepare/apply states:
   - `source_build` (git-like source and build artifacts);
   - `dolt_app` (app state, textures, prompts, traces);
   - `vm_os` (machine image and runtime service state);
   - `blob_content` (uploads and generated media);
   - `artifact_graph` (verifier results, trace refs, promotion certificates);
   - `route_identity` (the active pointer).
3. Model the promotion certificate as a durable record of the fork point, candidate base, merge results, conflicts, verifier results, and route transition.
4. Model the freshness CAS: at commit time, the active computer's base must equal the candidate's fork base.
5. Model the approval gate: commit requires an explicit `approved` state.
6. Model the health window: after commit, the system is in a `committed` state until either `confirmed` (healthy) or `reverted` (unhealthy).
7. Model poisoned writes: a `committed` promotion may become `poisoned`, which disables auto-revert and forces forward recovery (a new corrective promotion).
8. Model rollback: before commit, abort is safe; after commit, revert is only safe while the rollback window is open.

---

## Invariants the New Spec Must Check

- **NoStaleCommit** — a promotion may not commit if the active computer's base has moved since the candidate was verified.
- **ApprovalGate** — a promotion may not commit without explicit owner approval.
- **NoTornOutcome** — settled promotions are uniform across all participating ledgers; no ledger may be `applied` while another is `rolled_back` for the same promotion.
- **RouteConsistency** — the route pointer always names a computer that is either the pre-promotion active computer or the post-promotion candidate; never both, never neither, and never a half-promoted state.
- **HealthWindowReversible** — a `committed` promotion may revert to the previous active computer only if no poisoned write has occurred.
- **CertificateCompleteness** — every committed promotion has a durable certificate recording the fork base, candidate base, merge results, verifier results, and route transition.
- **CandidateIsolation** — before commit, candidate mutations do not affect the active computer's route-visible state.

---

## What the New Spec Will Prove (and What It Will Catch When Sabotaged)

When the spec is model-checked green, it proves that the autoputer promotion protocol is safe under all interleavings of:
- active computer mutation during candidacy;
- candidate verification success/failure;
- owner approval/rejection;
- commit/abort timing;
- health window success/failure;
- poisoned write timing;
- rollback attempts.

When sabotaged, the spec must reproduce known failure modes as short counterexamples:
- Drop the freshness CAS → active computer changes are silently overwritten.
- Drop the approval gate → unreviewed changes become visible.
- Allow auto-revert after a poisoned write → torn rollback, old version reads data it cannot interpret.
- Update route before all ledgers prepare → route points to a computer whose state is inconsistent with the promoted ledgers.

---

## Path to Encoding

1. Write the new `specs/promotion_protocol.tla` and model-check it with TLC.
2. Update `specs/README.md` with the new spec's story and invariants.
3. Design the Go promotion package (`internal/autoputer/promotion` or `internal/promotion`) to match the spec state machine.
4. Migrate the existing `AppAdoption` logic into the new promotion package as the `source_build` ledger prepare step.
5. Add the missing ledgers incrementally, starting with `dolt_app` and `route_identity`.
6. Add the promotion certificate as a durable object-graph record.
7. Add the health window and poisoned-write tracking.
8. Wire Nucleus capsules as the execution environment for candidate mutations.

---

## Residual Risks

- The current `PromoteAppAdoption` only flips string pointers in `ComputerSourceLineage`. The new spec assumes a real route flip that changes what the running autoputer serves. The gap between pointer flip and actual running service swap is a known risk.
- The Dolt/app ledger is not yet part of the promotion. A source-only promotion that does not touch app state is incomplete for an autoputer.
- The health window requires health checks that the current system does not define.
- Nucleus capsule integration is not yet implemented; the spec will model it as an assumption that candidate mutations are isolated, but the implementation must honor that assumption.

---

## Next Action

Write the new `specs/promotion_protocol.tla` that satisfies the requirements above, then model-check it with TLC before any code changes.
