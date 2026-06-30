# Parallax: Qdrant Derived Index Pipeline

## Status

O2 implementation branch ready for verifier review, with real local-Qdrant
verification blocked by no service at `127.0.0.1:6333` on 2026-06-26. The O0
prototype was preserved at `preserve/o0-qdrant-prototype-2026-06-26`
(`4c1b28be`). O2 narrows it from sample-object prototype to objectgraph-derived
package code.

## Mission conjecture

If Qdrant indexes objectgraph `Object` records through a rebuildable
shadow-collection pipeline, and the active alias is switched by one atomic
Qdrant alias transaction, then Choir gains a semantic retrieval substrate
without creating a parallel source of truth.

## Deeper goal

The vector database is the semantic navigation layer of the object graph. The deeper goal is to prove that Qdrant can index objects, support alias-based transactional updates, and be rebuilt from canonical state. This is a prerequisite for agentic search and the citation economy.

## Witness / spec

Deliver a prototype with:

- A local Qdrant instance running via Docker or Podman (or via Nix if available).
- A collection schema with payload fields: `canonical_id`, `object_kind`, `content_hash`, `owner_id`, `text`, `embedding_model`, `embedding_version`, `metadata`.
- A Go package that projects `internal/objectgraph.Object` values into Qdrant
  points. Sample-only objects are not acceptable for O2.
- A shadow-collection build flow: create new collection, verify counts/sample queries, atomically switch alias, keep old collection for TTL.
- A verification that the alias switch is atomic and does not lose points.
- An embedding boundary that accepts any provider/model implementation by
  capability and vector size. Deterministic hash embedding is test-only.
- Documentation of rebuild, rollback, and old-collection cleanup.

## Invariants / qualities / domain ramp

- Qdrant is derived state, not canonical memory. The pipeline must be rebuildable from the object graph.
- All points must reference canonical IDs and content hashes.
- No mutation of canonical stores in this prototype.
- Keep the prototype local; no production deployment.
- Do not hard-code role/provider assumptions into the production embedder
  boundary.
- Use the Qdrant placement guidance from `@/Users/wiz/go-choir/docs/handoff-hybrid-computer-capsule-architecture-2026-06-10.md:386-423`.

## Authority / bounds

- Yellow/orange mutation class: infrastructure and derived index, no canonical state change.
- No production routing change.
- Branch: `codex/o2-qdrant-derived-index`.
- Worktree: `/Users/wiz/.codex/worktrees/fb93/go-choir`.

## Bridge conjecture + sub-conjectures

- Main conjecture: Qdrant is the right derived index for the object graph.
- Sub-conjecture 1: alias-based collection switching provides transactional
  index updates only if the switch is sent as one alias transaction. The O0
  prototype used an `update_alias` action shape; O2 treats that as a
  prototype-fit blocker and repairs it with combined `delete_alias` plus
  `create_alias` actions.
- Sub-conjecture 2: the payload schema is sufficient for agentic search across object kinds.
- Sub-conjecture 3: the pipeline can be rebuilt from canonical state if Qdrant is lost.

## Ledger / move log

- Move 0: Read Qdrant planning docs and architecture guidance.
- Move 1: Review preserved prototype alias-switch and source-of-truth
  boundaries.
- Move 2: Implement objectgraph projection and payload mapping.
- Move 3: Implement shadow-collection build and alias switch with explicit
  rollback handle.
- Move 4: Add hermetic unit tests and a local-Qdrant integration test that
  skips when Qdrant is unavailable.
- Move 5: Document rebuild, rollback, and provider/embedder boundary.
- Move 6: Commit focused docs and code changes, then stop for independent O2
  verifier review. Local Qdrant integration proof is deferred until a safe
  local service is available.

## Version / lineage

- Predecessor: `@/Users/wiz/go-choir/docs/object-graph-schema-2026-06-23.md` section 7 and `@/Users/wiz/go-choir/docs/handoff-hybrid-computer-capsule-architecture-2026-06-10.md:386-423`.
- Successor link: this pipeline will be wired to the object service and source entity migration.

## Learning state

- Retained: Qdrant collection schema, alias-switch flow, objectgraph projection,
  embedding boundary, rebuild/rollback path.
- Promoted outward: none until independent verifier accepts O2.

## Settlement

O2 worker stop condition: branch contains focused implementation and tests, but
does not claim O2 complete. A separate verifier must review schema, alias
switch, and source-of-truth boundaries before O2 can be accepted.

## Rebuild And Rollback Path

Rebuild:

1. Read the canonical corpus from `internal/objectgraph.Service` or another
   objectgraph-backed source with an owner/kind filter.
2. Project each non-tombstoned object to index text plus payload carrying
   `canonical_id`, `object_kind`, `content_hash`, owner/computer/version
   identifiers, embedding model metadata, and object metadata.
3. Create a new versioned shadow collection sized to the embedder model.
4. Upsert all embedded points and verify count/search before changing the
   active alias.
5. Switch the alias to the new collection with one Qdrant alias operation.
6. Retain the previous collection as disposable rollback state until the
   confidence window expires, then delete it.

Rollback:

1. If verification fails before alias switch, delete the shadow collection and
   leave the current alias untouched.
2. If a post-switch issue is found while the previous collection is retained,
   switch the alias back to the previous collection with one alias operation.
3. If Qdrant data is lost, rebuild from objectgraph rather than treating Qdrant
   as recovery source.
