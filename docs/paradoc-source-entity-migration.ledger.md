# Source Entity Migration Ledger

## 2026-06-26 - O3 Phase 1 Store Boundary Checkpoint

Move: route decision before implementation.

Expected delta V: remove the Phase 1 route-choice obligation and resolve the
reviewer P2 canonical-ID ambiguity.

Actual delta V: chose Texture-store source tables behind the objectgraph
contract for Phase 1, and constrained source entity/source ref canonical IDs to
a single URL-safe suffix compatible with `objectgraph.BuildCanonicalID` and
`objectgraph.ParseCanonicalID`.

Evidence: `internal/store/texture.go` `CreateRevision` already wraps revision
insert, revision hash computation, and document-head advancement in one
transaction. `internal/objectgraph/object.go` canonical IDs parse exactly four
colon-separated parts and require the suffix to be one URL-safe component.

Next move: implement a focused store boundary and tests; do not migrate
producers, frontend/source-open, publication/export, or Qdrant projections in
this worker.

## 2026-06-26 - O3 Phase 2 Shadow-Write Producer Checkpoint

Move: route decision before implementation.

Expected delta V: remove the Phase 2 route-choice obligation for a single
narrow producer/tool path without expanding O3 into frontend, source-open, or
Qdrant behavior.

Actual delta V: chose the Texture appagent edit tool commit point
(`patch_texture` / `rewrite_texture` through `commitTextureToolEdit`) as the
only Phase 2 shadow-write path for this worker.

Evidence: `commitTextureToolEdit` already materializes structured
`SourceEntities`, stores a Texture revision, and preserves legacy revision
reads through `texture_revisions.source_entities_json`. Phase 1 added
`CreateRevisionWithSourceGraph` as the same-transaction write boundary.

Next move: implement graph `choir.source_entity` shadow writes for that tool
path only, add focused compatibility tests, and avoid public route,
frontend/source-open, Qdrant, provider, auth/session, deploy, or staging
changes.

## 2026-06-26 - O3 Phase 3 Source Ref Edge Checkpoint

Move: route decision before implementation.

Expected delta V: remove the Phase 3 resolution-rule ambiguity for the selected
Texture tool path without expanding O3 into graph reads or product source-open.

Actual delta V: chose to resolve body `source_ref.attrs.source_entity_id`
against the graph source entity records derived from the same materialized
`SourceEntities` array in `commitTextureToolEdit`, then write pinned
`choir.source_ref` records through `CreateRevisionWithSourceGraph`.

Evidence: Phase 2 already routes `patch_texture` / `rewrite_texture` through
`textureToolSourceGraphWriteSet` and `CreateRevisionWithSourceGraph` while
preserving legacy revision JSON reads. Phase 1 already makes source graph writes
part of the Texture revision transaction and rejects refs pointing at a missing
source entity version before advancing the document head.

Failure mode: if a body `source_ref` cannot resolve to a graph source entity
record, the Texture tool edit must fail and document head advancement must not
occur. The producer resolver should report the unresolved legacy source id; the
store transaction remains the final head-stability guard.

Next move: implement source_ref graph record construction for the selected tool
path only, add focused legacy-compatibility and unresolved-ref/head-stability
tests, and avoid public route, frontend/source-open, Qdrant, provider,
auth/session, deploy, staging, and graph-first-read changes.
