# Parallax: Design the Source Entity Object Migration

## Status

Open. Not yet started.

## Mission conjecture

If we design a detailed migration plan that turns `texture_available_source_entities` metadata into durable `choir.source_entity` objects cited by `choir.source_ref` edges, then we will have the blueprint for the first object-kind migration into the graph and a direct fix for the Texture source-citation bug.

## Deeper goal

Source entities are the first object kind that should be promoted from metadata to first-class objects. The deeper goal is to prove the migration pattern: define the object, define the edge, update the producer, update the consumer, add the index, verify, deprecate. This pattern will be reused for mail, web captures, and slide decks.

## Witness / spec

Deliver a design document with:

- Exact schema for `choir.source_entity` and `choir.source_ref`.
- Mapping from current `CoagentPacketSource` and `EvidenceRecord` types to the new object schema.
- Changes required in the researcher agent to write source entity objects.
- Changes required in the runtime to carry source entity objects across revisions.
- Changes required in the Texture agent to read and cite source entity objects.
- Changes required in the frontend to render source refs.
- Indexing plan for Qdrant.
- Verifier tests, including a regression test for `TestTextureAgentRevisionRegistersMediaSourceEntities`.
- Rollback plan.
- A phased implementation plan (schema → runtime → agent → frontend → deprecation).

## Invariants / qualities / domain ramp

- Source entity canonical IDs must be stable across revisions.
- A Texture revision must cite source entities via version-pinned `source_ref` edges.
- The migration must not break existing source entities in already-persisted revisions.
- The old metadata representation can remain for backward compatibility during transition, but new code must use objects.
- No regression to markdown source links or synthetic IDs.

## Authority / bounds

- Red/orange mutation class: runtime behavior, product APIs, and canonical Texture state.
- This is a protected surface; follow `AGENTS.md` landing loop.
- Branch: `design/source-entity-migration` or `feature/source-entity-objects`.
- Worktree: `source-migration-plan`.

## Bridge conjecture + sub-conjectures

- Main conjecture: promoting source entities to objects fixes the carry-forward bug and makes citations durable.
- Sub-conjecture 1: `choir.source_ref` edge objects can replace inline metadata for citations.
- Sub-conjecture 2: the researcher agent can be updated to write source entity objects without losing the current packet format.
- Sub-conjecture 3: the runtime can carry source entity objects across revisions using the object service.

## Ledger / move log

- Move 0: Read `@/Users/wiz/go-choir/docs/object-graph-schema-2026-06-23.md`, `@/Users/wiz/go-choir/docs/design-object-graph-versioning-2026-06-23.md`, and the current runtime source-entity handling.
- Move 1: Draft the object schema and migration mapping.
- Move 2: Identify all producer and consumer sites in the code.
- Move 3: Write the phased implementation plan.
- Move 4: Define verifier tests.
- Move 5: Write the rollback plan.
- Move 6: Commit the design document.

## Version / lineage

- Predecessor: `@/Users/wiz/go-choir/docs/object-graph-schema-2026-06-23.md` and the Texture source-entity fixes from commit `1025fb84`.
- Successor link: this design will be implemented as the first object-graph migration.

## Learning state

- Retained: the migration pattern for object kinds.
- Promoted outward: the source entity schema will be added to the object-graph schema.

## Settlement

Done when the design document is approved and the implementation plan is ready to execute.
