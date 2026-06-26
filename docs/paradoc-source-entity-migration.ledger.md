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
