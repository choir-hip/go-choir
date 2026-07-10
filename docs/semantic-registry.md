# Choir Semantic Registry

**Status:** derived navigation for the doctrine; not a second doctrine.

This registry makes the durable semantic kernel readable without creating a
parallel authority. Each record is a compact extraction of an already-settled
doctrine or domain-contract claim. If this document disagrees with its cited
source, the cited source wins. New global semantics must be promoted through
Choir Doctrine, not added here first.

| ID | Settled semantic | Scope and boundary | Source / settlement | Evidence or enforcement seam | Invalidation / successor |
| --- | --- | --- | --- | --- | --- |
| `SEM-01` | A Choir user owns a persistent **computer**, not a disposable sandbox or chat session. | Includes multiple ledgers and route identity; excludes any claim that one VM, database, browser tab, or Git checkout is the whole product. | Doctrine `C1`; [computer ontology](computer-ontology.md). | Computer ontology and protected VM/promotion paths; staging proof is required for shared platform behavior. | A promoted replacement ontology; code-service use of `sandbox` alone is not an invalidation. |
| `SEM-02` | Versioned artifacts are canonical user-facing truth; Texture owns canonical document revisions. | Findings, search results, worker updates, and rendered views are non-canonical until incorporated in a typed artifact domain. | Doctrine `C2`, `I1–I2b`; [Texture contract](texture-agentic-invariants-2026-06-13.md). | Texture revision/state-machine tests and product-path proof. | An explicit doctrine/domain-contract promotion that assigns a different canonical owner. |
| `SEM-03` | Risky mutation occurs against a candidate ComputerVersion and becomes canonical only by verified, approved promotion with rollback. | A candidate is a forked `(CodeRef, ArtifactProgramRef)`, never a VM; capsules execute effects but do not own agency or promotion authority. | Doctrine `C3`, `C8`; [computer ontology](computer-ontology.md). | MutationTransaction/promotion contracts, verifier/owner evidence, and route flip records. | A promoted change to the computer/promotion contract; a new materializer does not change this law. |
| `SEM-04` | Authority is an envelope of obligations, evidence, scope, and settlement criteria, not a persona or free-form workflow script. | Profiles may bound capabilities; they do not create a second product ontology. | Doctrine `C5`; [runtime invariants](runtime-invariants.md). | Capability boundaries, typed tool calls, and acceptance evidence. | A promoted doctrine replacement. |
| `SEM-05` | Trajectories and work items are the intended causality model; parent/child control and continuation synthesis are retired control paths. | Provenance edges may remain, but they are not liveness/control truth. Current implementation residue is explicitly transitional. | Doctrine `C6–C7`, `I3–I5`; [current architecture](current-architecture.md). | Runtime tests, detector inventory, and the active product Definition's deletion work. | Completion or an explicit doctrine revision; neither is implied by a legacy code reference. |
| `SEM-06` | Shared-platform claims are evidence-scoped. Claim strength is bounded by staging, verifier contracts, and owner review. | Local tests, status prose, and historical proof cannot establish live shared behavior. | Doctrine `C9`, `I8–I9`; `AGENTS.md`. | Deployed commit identity, staging acceptance proof, verifier contracts, owner review. | A changed evidence policy promoted in doctrine/operating contract. |
| `SEM-07` | External source bytes are evidence, not instructions; provenance is structured artifact identity rather than link-shaped prose. | Source Service, ContentItem, researcher, Texture, and publication have distinct ownership boundaries. | Doctrine `I14–I15`; [source/publication contract](source-external-data-publication.md). | Source hashes/selectors, `source_entities`, citation policy, publication records. | A promoted source/publication contract revision. |
| `SEM-08` | **Autopaper is a reopened provisional product object, beginning with one authoritative activation per typed ingestion handoff.** | It may compose scheduled Sources, processor evidence, canonical Texture artifacts, and explicit edition publication inside a Choir computer. It is not a separate service, may not bypass Texture/provenance authority, and its wider schedule plus personal/platform publication semantics remain unsettled. | Owner-restated 2026-07-10; [product-completion Definition](definitions/choir-product-completion-2026-07-10.md). | Source-cycle activation identity, runtime run creation, Texture/edition publication evidence, and the Definition's open edges. | Owner re-tables the product or a promoted Definition replaces this provisional boundary. |
| `SEM-09` | **Choir Base is a partial storage/sync substrate, not a competing canonical app-state authority.** | Its append-only journal, derived tree, blobs, API helpers, and File Provider integration may support source/blob/materialization concerns. Embedded Dolt remains canonical for private product/app state. Product wiring is blocked until exact-byte, stable-identity, owner-scoped conflict/cursor semantics are proven. | [computer ontology](computer-ontology.md); [product-completion Definition](definitions/choir-product-completion-2026-07-10.md); [NOW](NOW.md). | `internal/base`, `internal/desktop`, `/api/base/*` helpers, and the two-device exact-byte kernel. | A promoted Base product-wiring Definition with explicit ownership and store boundaries replaces this provisional kernel boundary. |

## What Is Deliberately Not Here

- Current deployment health, commit identity, live error rates, and feature
  readiness — use [`NOW.md`](NOW.md) with its freshness boundary.
- Service names, commands, model/provider choices, browser/desktop surface
  details, or materializer implementation.
- Active mission status and mutable task state — use [`ACTIVE.md`](ACTIVE.md).
- Raw evidence, historical rationale, or a proposal that has not been promoted.
- The implementation choice to use a particular database or store, unless it
  changes a semantic contract above.

## Registry Change Rule

Edit a record only when its cited authority changes, then update the citation
and enforcement seam in the same change. Adding a record requires a stable ID,
scope, non-definition, settlement source, enforcement/evidence seam, successor,
and invalidation trigger. The documentation-authority Definition owns the
initial registry migration; Choir Doctrine remains the promotion authority.
