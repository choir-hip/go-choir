# Parallax: Prototype the Qdrant Indexing Pipeline

## Status

Open. Not yet started.

## Mission conjecture

If we stand up a local Qdrant instance, embed sample Choir objects, and verify alias-based collection switching, then we will have a working derived index for the object graph and a pipeline for indexing new objects as they enter the graph.

## Deeper goal

The vector database is the semantic navigation layer of the object graph. The deeper goal is to prove that Qdrant can index objects, support alias-based transactional updates, and be rebuilt from canonical state. This is a prerequisite for agentic search and the citation economy.

## Witness / spec

Deliver a prototype with:

- A local Qdrant instance running via Docker or Podman (or via Nix if available).
- A collection schema with payload fields: `canonical_id`, `object_kind`, `content_hash`, `owner_id`, `text`, `embedding_model`, `embedding_version`, `metadata`.
- A script or Go program that embeds a few sample objects (text docs, source entities, web captures) and inserts them into Qdrant.
- A shadow-collection build flow: create new collection, verify counts/sample queries, atomically switch alias, keep old collection for TTL.
- A verification that the alias switch is atomic and does not lose points.
- Documentation of the embedding model choice and cost/latency notes.

## Invariants / qualities / domain ramp

- Qdrant is derived state, not canonical memory. The pipeline must be rebuildable from the object graph.
- All points must reference canonical IDs and content hashes.
- No mutation of canonical stores in this prototype.
- Keep the prototype local; no production deployment.
- Use the Qdrant placement guidance from `@/Users/wiz/go-choir/docs/handoff-hybrid-computer-capsule-architecture-2026-06-10.md:386-423`.

## Authority / bounds

- Yellow/orange mutation class: infrastructure and derived index, no canonical state change.
- No production routing change.
- Branch: `prototype/qdrant-index`.
- Worktree: `qdrant-pipeline`.

## Bridge conjecture + sub-conjectures

- Main conjecture: Qdrant is the right derived index for the object graph.
- Sub-conjecture 1: alias-based collection switching provides transactional index updates.
- Sub-conjecture 2: the payload schema is sufficient for agentic search across object kinds.
- Sub-conjecture 3: the pipeline can be rebuilt from canonical state if Qdrant is lost.

## Ledger / move log

- Move 0: Read Qdrant planning docs and architecture guidance.
- Move 1: Start local Qdrant.
- Move 2: Create collection schema and payload mapping.
- Move 3: Embed sample objects.
- Move 4: Implement shadow-collection build and alias switch.
- Move 5: Verify counts, sample queries, metadata coverage.
- Move 6: Document model choice and pipeline shape.
- Move 7: Commit and push the prototype.

## Version / lineage

- Predecessor: `@/Users/wiz/go-choir/docs/object-graph-schema-2026-06-23.md` section 7 and `@/Users/wiz/go-choir/docs/handoff-hybrid-computer-capsule-architecture-2026-06-10.md:386-423`.
- Successor link: this pipeline will be wired to the object service and source entity migration.

## Learning state

- Retained: Qdrant collection schema, alias-switch flow, embedding model choice.
- Promoted outward: the indexing pipeline design.

## Settlement

Done when the prototype can embed objects, switch aliases atomically, and pass a verification script.
