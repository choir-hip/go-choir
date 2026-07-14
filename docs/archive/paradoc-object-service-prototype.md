# Parallax: Prototype the Object Graph Service

## Status

Open. Not yet started.

## Mission conjecture

If we build a small object service that can create, read, list, and reference objects across Dolt, SQLite, and blob stores with a canonical ID scheme, then the rest of the system will have a stable substrate to migrate source entities, mail objects, web captures, and slide decks into the object graph.

## Deeper goal

The object graph is the center of the Choir refactor. The deeper goal is to prove that the graph can be implemented without building a graph database, using existing stores and a simple API. The object service is the bridge between the schema and the implementation.

## Witness / spec

Deliver a prototype object service in Go with:

- `CreateObject(ctx, kind, body, metadata) (canonicalID, error)`
- `GetObject(ctx, id, version) (object, error)`
- `ListObjects(ctx, filter) ([]object, error)`
- `PutEdge(ctx, from, to, kind, metadata) (edgeID, error)`
- `QueryObjects(ctx, vector, structured, provenance) ([]object, error)` (agentic search stub)

And:
- An object kind registry loaded from a YAML file or Go consts.
- A canonical ID scheme: `obj:<kind>:<owner>:<hash-or-uuid>`.
- A Dolt-backed store for canonical app state.
- A SQLite-backed store for host-level objects.
- A blob store for content-addressed bodies.
- Unit tests for each operation.

Do not yet integrate with the runtime. The prototype is a standalone package.

## Invariants / qualities / domain ramp

- The API must be storage-agnostic: callers should not know which store an object lives in.
- The canonical ID must be stable and globally unique.
- The service must not mutate existing app state in this prototype.
- Use existing Go patterns in the repo: interfaces, structs, JSON metadata.
- Keep the package small and focused; no runtime integration yet.

## Authority / bounds

- Orange mutation class: new runtime behavior and APIs, but no product surface change.
- No database schema changes to production stores unless behind a feature flag.
- Branch: `prototype/object-service`.
- Worktree: `object-service`.

## Bridge conjecture + sub-conjectures

- Main conjecture: a simple object service is enough to implement the object graph; we do not need a graph database.
- Sub-conjecture 1: Dolt is the right canonical store for versioned app objects.
- Sub-conjecture 2: SQLite is the right store for host-level lightweight objects.
- Sub-conjecture 3: Content-addressed blob storage is the right store for bodies.

## Ledger / move log

- Move 0: Read `@/Users/wiz/go-choir/docs/object-graph-schema-2026-06-23.md` and `@/Users/wiz/go-choir/docs/design-virtual-agentic-graph-2026-06-23.md`.
- Move 1: Create package `internal/objectgraph` or similar.
- Move 2: Define interfaces and the canonical ID scheme.
- Move 3: Implement Dolt store adapter.
- Move 4: Implement SQLite store adapter.
- Move 5: Implement blob store adapter.
- Move 6: Implement the service API.
- Move 7: Write unit tests.
- Move 8: Commit and push the prototype.

## Version / lineage

- Predecessor: `@/Users/wiz/go-choir/docs/object-graph-schema-2026-06-23.md`.
- Successor link: this service will be used by the source entity migration.

## Learning state

- Retained: the canonical ID scheme, storage adapter pattern, and API surface.
- Promoted outward: the object service package will be merged into main.

## Settlement

Done when the prototype package has passing tests, a clean API, and is merged or ready for the source entity migration to build on.
