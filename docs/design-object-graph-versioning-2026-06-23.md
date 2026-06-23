# Design: Object Graph Versioning

This document defines how versioning works in the Choir object graph. It addresses the current split between Dolt storage versioning, Git code versioning, and ad-hoc app revision IDs. It is part of the conceptual refactor docset.

## 1. The problem

Versioning in Choir is currently fragmented:

- Dolt versions the underlying app state.
- Git versions the code.
- Texture has its own revision numbers (`v1`, `v2`, `v3`).
- App state is serialized and restored without a strong object-version notion.
- Publications currently float to the latest version instead of pinning a specific object version.
- The `vtext` → `texture` rename is painful because the object model has no `supersedes` or `kind migration` concept.

The object graph needs a single versioning model that works across storage, code, and user semantics.

## 2. Core principles

1. **Identity is stable; name and kind are mutable.** A canonical ID does not change when an object is renamed or rekinded.
2. **Every object has a version chain.** A version is a snapshot of the object's bytes and metadata at a point in time.
3. **A revision is a morphism.** It takes one object version and produces another. Revisions are edges in the graph.
4. **References are version-pinned by default.** When one object references another, the reference includes the target version ID unless the reference is explicitly live.
5. **Supersession is a first-class edge.** An object, a kind, or a schema can be superseded by a successor. History is preserved.

## 3. Object versioning

### 3.1 Version chain

```text
Object:
  canonical_id: obj:tex:doc:abc123
  object_kind: choir.texture_doc
  versions:
    v1: { content_hash, metadata, body, created_at, revision_id }
    v2: { content_hash, metadata, body, created_at, revision_id }
    v3: { content_hash, metadata, body, created_at, revision_id }
  current_version: v3
  published_version: v1 | null
```

Each version is immutable. A new version is produced by a revision morphism. The `current_version` pointer can move forward. The `published_version` pointer is set explicitly by a publish action and does not float.

### 3.2 Revision edge

```text
choir.texture_revision:
  canonical_id: obj:tex:rev:abc123-v2
  object_kind: choir.texture_revision
  metadata:
    doc_id: obj:tex:doc:abc123
    revision_number: 2
    previous_revision_id: obj:tex:rev:abc123-v1
    agent_id: researcher-xyz
    run_id: run-123
    source_entity_ids: [obj:src:ent:456]
```

A revision is an object too. It is the morphism that carries provenance. This is why the current Texture bug is an object-graph bug: the source entities are not being carried as objects from one revision to the next.

## 4. Kind supersession

The `vtext` → `texture` rename is a kind-level migration. The object graph handles this with a `supersedes` edge:

```text
choir.object_kind:
  canonical_id: kind:choir.texture_doc
  metadata:
    name: texture_doc
    schema_version: v1
    superseded_by: null
    supersedes: [kind:choir.vtext_doc]
```

Rules:

- An old object kind remains valid in the graph as long as objects of that kind exist.
- A migration is a mutation transaction that rewrites old-kind objects to the new kind.
- The migration preserves canonical IDs and records a `was_kind` edge for provenance.
- References to old-kind objects continue to resolve via the supersession chain.

This is how the `vtext` → `texture` rename should have happened: a durable migration transaction, not a global search-and-replace that leaves the docs checker firing for years.

## 5. Version-pinned references

A reference from one object to another must include a version pin unless the reference is explicitly live.

```text
reference:
  from: obj:pub:doc:xyz
  to: obj:tex:doc:abc123
  to_version: v2
  kind: publishes
  live: false
```

This is the fix for the version-preserving publishing bug. A publication is an object that references a specific version of a texture document. If the texture document later advances to `v3`, the publication still points to `v2`. Updating the publication is a separate mutation.

Live references exist for things like the desktop shell showing the current state of a document. The default is pinned.

## 6. Publication as a versioned object

```text
choir.publication:
  canonical_id: obj:pub:doc:xyz
  metadata:
    source_doc_id: obj:tex:doc:abc123
    source_version: v2
    published_at: timestamp
    route: string
    bundle_hash: string
  body: the publication bundle
```

The publication object is immutable. It records exactly which version of the source document was published. The public route resolves to the publication object, not the live document.

## 7. Storage mapping

| Layer | Versioning mechanism | Maps to object graph |
|---|---|---|
| Dolt | Branch/commit/merge | Object graph canonical state, version chains |
| Git | Commit/branch | Code version for the runtime and functors |
| Blob store | Content hash | Object body versions |
| Qdrant | Collection alias | Derived index over a snapshot of the graph |
| Mutation transaction | Durable transaction object | Records the promotion/rollback refs |

Dolt's branch and commit model is the right storage substrate for the object graph. The object graph exposes a higher-level API: object versions, revision edges, supersession, and pinned references.

## 8. In-place changes as transactions

A rename, schema change, or object-kind migration is a `MutationTransaction`. See `@/Users/wiz/go-choir/docs/design-mutation-transaction-2026-06-23.md`.

Example: `vtext` → `texture` migration.

1. **Begin**: record base refs (Dolt commit, Git commit, blob manifest).
2. **Stage**: create a Dolt branch for the object graph migration.
3. **Execute**: rewrite all `choir.vtext_doc` objects to `choir.texture_doc` with `was_kind` edges.
4. **Verify**: run tests and docs checker to confirm `vtext` vocabulary is only historical.
5. **Commit**: merge Dolt branch, update code, switch route.
6. **Rollback**: keep old Dolt commit and Git commit until TTL.

The key is that the rename becomes a graph-level migration, not a codebase-wide search-and-replace. The old vocabulary lives in historical objects and evidence docs; the current vocabulary lives in current objects.

## 9. Implications for the docs checker

Most of the 975 doccheck warnings are retired vocabulary (`vtext`, `continuation-level`, `AI workspace`, `chat`, etc.) appearing in current docs. The versioning model explains why this happens: the object model changed, but the docs were not migrated as a transaction. They drifted.

The fix is the same as the object graph fix: classify docs as current, evidence, or historical; run a migration transaction on the docs; pin the current truth.

## 10. App-as-functor consequence

If apps are functors over the graph, then an app version is a version of the functor. Publishing an app change is the same as publishing a document change: pin the version of the functor that renders the objects. The desktop shell can render an old document with an old functor if needed, or migrate it forward.

## 11. First implementation target

The first versioned object to implement is `choir.source_entity` with a version chain and a `source_ref` edge that pins the source version used by a texture revision. This directly fixes both the source-citation bug and the version-preserving publishing bug.

## 12. Open questions

- Should versions be linear or DAG-shaped (e.g. for collaborative edits)?
- Should the canonical ID encode the current kind, or should kind be mutable metadata?
- How do we version the object graph schema itself?
- How do we represent a user-approved promotion vs. an auto-promoted low-risk transaction?

These are answered by the first source-entity migration, not by this doc.
