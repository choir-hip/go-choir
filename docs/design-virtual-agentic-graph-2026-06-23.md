# Design: The Object Graph is Virtual and Agentic

This document refines the object graph model. The graph is not a graph database. It is a virtual graph: a patchwork of objects stored across many data systems, embedded into a shared high-dimensional space, and navigated by agentic search.

## 1. The graph is not a graph database

An early temptation was to model Choir as a hypergraph or graph database. That would mean a single store with explicit nodes and edges and traversal APIs. That is not what we are building.

The Choir object graph is virtual:

- Objects live in Dolt, SQLite, blob stores, mail databases, source cycle databases, and other specialized stores.
- Edges are stored as objects, metadata fields, or provenance records.
- The vector index (Qdrant) embeds objects and their metadata in high-dimensional space.
- Navigation is agentic search, not node-to-node traversal.

The graph is an abstraction, not a database product. The schema is the contract. The stores are the implementation.

## 2. Hypergraph inspiration, not implementation

A hypergraph lets an edge connect many nodes. That is the right mental model for a citation: one `source_ref` can connect a paragraph, a source entity, a selector, and a revision. But we do not need a hypergraph database to express this.

We express it with:

- A `choir.source_ref` object that references multiple related objects by canonical ID.
- Metadata fields that name the relationship.
- Provenance edges that record the chain of custody.

The hypergraph is the logical structure. The physical storage is ordinary objects in ordinary stores.

## 3. The graph is embedded in high-dimensional space

The vector database is not a side feature. It is the primary navigation layer of the graph.

When an object enters the graph, it is embedded. The embedding captures:

- The object's body or text.
- Its metadata (kind, owner, timestamps, source URL).
- Its relationships (citation history, revision chain, authorship).
- Its provenance (run ID, agent ID, trace refs).

This means the graph has a dual form:

- **Discrete form**: canonical objects with IDs, stored in Dolt/SQLite/blob stores.
- **Continuous form**: dense vectors in Qdrant, representing semantic neighborhood.

Agentic search operates on both forms at once.

## 4. Agentic search is the traversal engine

Node-to-node graph traversal is shallow. It follows explicit edges. Agentic search is deeper because it can combine:

- Semantic similarity from embeddings.
- Structured filters from metadata.
- Citation history from edges.
- Provenance from Trace.
- User intent from the prompt.

An agent can ask: "find sources that contradict this claim" or "find code changes that touched the same schema" or "find news stories about the same event from different publishers." These are not simple graph traversals. They are semantic, contextual, and cross-domain.

This is why the graph does not need to be a graph database. The vector index plus agentic reasoning is the traversal engine.

## 5. Edges are still real

Saying the graph is virtual does not mean edges are imaginary. Edges are durable objects or metadata fields. A `source_ref` is a real object. A `revises` edge is a real object. A `supersedes` edge is a real object. These edges are the audit trail and the citation substrate.

But edges are not the primary navigation mechanism. They are the provenance mechanism. Navigation happens through the vector index and agentic search.

## 6. Many indices, combined

The graph is navigated by combining several indices:

| Index | Role | Store |
|---|---|---|
| Vector | Semantic similarity | Qdrant |
| Structured | Kind, owner, timestamps, version | Dolt / SQLite |
| Full-text | Searchable text | SQLite / Dolt / external |
| Citation | Source refs and source entities | Dolt / SQLite |
| Provenance | Trace / run IDs / agent IDs | Trace store |
| Content | Content hash lookup | Blob store |

No single index is enough. The power is in the combination. An agentic query can start with vector similarity, filter by structured metadata, cross-reference citation edges, and verify provenance in Trace.

## 7. Storage can change without the graph minding

The object graph schema does not depend on SQLite or Dolt or Qdrant or Postgres. The host state may move from SQLite to Postgres. The vector index may move from Qdrant to another engine. The blob store may change.

As long as the object service presents the same canonical object API, the graph is intact. The stores are interchangeable behind the object abstraction.

This is the benefit of the virtual graph: it is not tied to a single storage technology.

## 8. Within-domain edge walking

There are cases where node-to-node traversal is correct:

- Following a `revises` chain in a Texture document.
- Following `source_ref` edges to reconstruct the citation graph of a document.
- Following `supersedes` edges to resolve an old object kind.
- Following `contains` edges to list paragraphs in a document.

These are within-domain provenance walks. They are explicit, structured, and bounded. They do not require a graph database. They require object IDs and a few query patterns.

## 9. Implementation consequence

We do not need to build a graph database. We need:

1. A canonical ID scheme.
2. An object kind registry.
3. A small object service that knows how to read and write objects across the stores.
4. A Qdrant indexing pipeline that embeds objects and their metadata.
5. An agentic query interface that combines vector search with structured filters.

This is much smaller than building a graph engine. It is the right size for the object graph.

## 10. Relation to the schema

`@/Users/wiz/go-choir/docs/object-graph-schema-2026-06-23.md` defines the logical schema. This document says that the physical graph is the embedding of that schema across existing stores. The schema is the contract. The stores are the implementation. The vector index is the navigation layer.

## 11. Relation to self-developing software

`@/Users/wiz/go-choir/docs/design-self-developing-software-2026-06-23.md` says the system must be able to improve itself. The virtual graph makes this possible because the same navigation and storage model works for code objects, news objects, source objects, and transaction objects. The vector index does not care whether it is embedding a code diff or a news story.

## 12. Relation to mutation transactions

`@/Users/wiz/go-choir/docs/design-mutation-transaction-2026-06-23.md` says every change is a transaction. The virtual graph makes transactions portable: a transaction can record base refs across Dolt, Git, Qdrant, and VM snapshots, and the object graph records the transaction object itself.

## 13. Open questions

- How do we keep the vector index consistent with the canonical stores in near-real-time?
- How do we version the embedding model itself?
- What is the query language for agentic graph search?
- How do we represent multi-hop provenance queries without a graph traversal engine?

These are answered by the first source-entity migration and the first Qdrant indexing pipeline.
