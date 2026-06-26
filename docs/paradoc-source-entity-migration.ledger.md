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
