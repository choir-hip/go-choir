# Object Graph Schema

This document is the first concrete schema for the Choir object graph. It defines object kinds, edge kinds, identity, capabilities, and persistence mapping. It is intended to be revised in place as objects migrate into the graph.

## 1. Identity

Every object has a stable identity.

| Field | Meaning | Example |
|---|---|---|
| `canonical_id` | Globally unique, opaque, URL-safe identifier | `obj:tex:doc:abc123` |
| `object_kind` | Registered object type | `choir.texture_doc` |
| `owner_id` | User or computer that owns the object | `user:alice` |
| `computer_id` | Machine or VM where the object is currently materialized | `vm:alice-macbook` |
| `version_id` | For versioned objects, the revision identifier | `v3` |
| `content_hash` | Hash of canonical serialized bytes | `sha256:deadbeef...` |
| `created_at` | Timestamp of first materialization | ISO 8601 |
| `updated_at` | Timestamp of last mutation | ISO 8601 |

`canonical_id` is the primary key. `content_hash` is used for deduplication, content verification, and vector-index pointers. `version_id` is optional; immutable objects do not need it.

## 2. Base object structure

```text
Object:
  canonical_id: string
  object_kind: string
  owner_id: string
  computer_id: string
  version_id: string | null
  content_hash: string
  body: bytes | null              // canonical bytes, often encrypted at rest
  metadata: JSON                 // kind-specific metadata
  created_at: timestamp
  updated_at: timestamp
  tombstone: bool                // soft deletion
  superseded_by: canonical_id | null
```

`body` is optional because some objects are pure edges or containers. `metadata` is always JSON and must be forward-compatible.

## 3. Object kinds

### 3.1 Core document objects

#### `choir.texture_doc`

A structured document composed of paragraph objects.

```text
metadata:
  title: string
  schema_version: string
  published_version_id: string | null
  revision_count: int
```

#### `choir.texture_paragraph`

A single paragraph node inside a texture document.

```text
metadata:
  doc_id: canonical_id
  paragraph_id: string  // stable within the doc
  order: int
  style: string | null
```

body: plain text or structured JSON depending on schema version.

#### `choir.texture_revision` (frame)

A snapshot of a document at a point in time.

```text
metadata:
  doc_id: canonical_id
  revision_number: int
  revision_role: string  // input, working, canonical
  previous_revision_id: canonical_id | null
  agent_id: string | null
  run_id: string | null
  source_entity_ids: [canonical_id]
```

### 3.2 Source objects

#### `choir.source_entity`

The durable substrate for citations. This is the first object kind to migrate into the graph fully.

```text
metadata:
  kind: string              // web_url, content_item, media, whole_resource
  display_title: string
  display_url: string | null
  provenance: JSON
  selectors: [JSON]         // text_quote, whole_resource, etc.
  content_hash: string
  media_type: string | null
  owner_id: string
  run_id: string | null
  agent_id: string | null
```

body: the actual content blob if small; otherwise a reference to a `choir.file_blob`.

#### `choir.source_ref`

An edge object that represents an inline citation inside a texture revision.

```text
metadata:
  doc_id: canonical_id
  revision_id: canonical_id
  paragraph_id: string
  entity_id: canonical_id
  selector_index: int
  offset: int | null
  length: int | null
```

### 3.3 Communication objects

#### `choir.mail_message`

An immutable email object.

```text
metadata:
  message_id: string          // SMTP Message-ID
  thread_id: canonical_id
  folder: string
  from_address: string
  to_addresses: [string]
  subject: string
  date: timestamp
  headers: JSON
  body_hash: string
  attachments: [canonical_id]
```

#### `choir.mail_thread`

A container object grouping mail messages.

```text
metadata:
  subject: string
  participant_ids: [canonical_id]
  last_message_at: timestamp
```

#### `choir.contact`

A person or organization object.

```text
metadata:
  display_name: string
  email_addresses: [string]
  avatar_blob_id: canonical_id | null
  source_entity_id: canonical_id | null
```

### 3.4 Capture objects

#### `choir.web_capture`

A captured web page or URL artifact.

```text
metadata:
  url: string
  canonical_url: string
  title: string
  fetched_at: timestamp
  fetch_status: int
  content_blob_id: canonical_id
  extracted_text_blob_id: canonical_id | null
  embedding_model: string | null
  embedding_version: string | null
```

This is the missing object for Universal Wire and for Texture web sources.

#### `choir.calendar_event`

A time-bounded object.

```text
metadata:
  summary: string
  start_at: timestamp
  end_at: timestamp
  timezone: string
  attendee_ids: [canonical_id]
  source_entity_id: canonical_id | null
```

### 3.5 Blob and file objects

#### `choir.file_blob`

Content-addressed binary storage.

```text
metadata:
  media_type: string
  byte_length: int
  filename: string | null
  sha256: string
```

body: the raw bytes.

### 3.6 Supervision objects

See `@/Users/wiz/go-choir/docs/design-conductor-supervision-protocol-2026-06-23.md` for the full protocol.

#### `choir.supervision_observation`

```text
metadata:
  trajectory_id: string
  sensor_kind: string
  subject_id: canonical_id
  subject_kind: string
  payload: JSON
  schema_version: int
```

#### `choir.supervision_finding`

```text
metadata:
  trajectory_id: string
  fingerprint: string
  invariant: string
  severity: string
  actor: string
  subject_id: canonical_id
  evidence_hash: string
  expected_response_shape: string
  state: string
  resolution_at: timestamp | null
  resolved_by: string | null
```

#### `choir.supervision_message`

```text
metadata:
  trajectory_id: string
  finding_id: canonical_id
  to_actor: string
  to_address: string
  message_kind: string
  payload: JSON
  sent_at: timestamp
```

## 4. Edge kinds

Edges are also objects. They are first-class and citeable.

| Edge kind | Source | Target | Meaning |
|---|---|---|---|
| `contains` | container | object | Parent-child, e.g. doc -> paragraph |
| `cites` | source_ref | source_entity | Inline citation |
| `revises` | revision | revision | Document frame successor |
| `authored_by` | object | actor/user | Provenance |
| `references` | object | object | Generic typed reference |
| `depends_on` | work_item | object | Required before work can settle |
| `responds_to` | supervision_message | finding | Nudge is a response to a finding |
| `has_avatar` | contact | file_blob | Contact avatar |
| `has_attachment` | mail_message | file_blob | Email attachment |
| `captured_from` | web_capture | source_entity | Capture is a source |
| `belongs_to_thread` | mail_message | mail_thread | Thread membership |

## 5. Capability model

A capability is a token that grants a scoped right over a slice of the graph.

```text
Capability:
  capability_id: string
  grantor: canonical_id        // user or system
  grantee: string               // actor, app, or user
  object_id: canonical_id | null
  object_kind: string | null
  edge_kind: string | null
  rights: [read, write, cite, delete, admin]
  expires_at: timestamp | null
  conditions: JSON | null
```

Examples:
- The email app gets `read,write` on `choir.mail_message` objects owned by the user.
- A researcher agent gets `cite` on `choir.source_entity` objects and `write` on proposed `choir.source_entity` objects.
- The trajectory supervisor gets `read` on everything and `write` only on `choir.supervision_*` objects.

## 6. Persistence mapping

The object graph is the logical model. Storage is implementation-specific.

| Concern | Store | Notes |
|---|---|---|
| Canonical app state | Dolt | Versioned, branchable, auditable |
| Host lightweight state | SQLite | sourcecycled.db, maild.db, etc. |
| Binary content | Blob store | Content-addressed, `sha256` |
| Vector index | Qdrant | Derived, alias-switchable, rebuildable |
| Real-time views | In-memory cache | Derived from graph |
| Trajectory trace | Trace/evidence store | Immutable, append-only |

All stores must reference objects by `canonical_id`. Qdrant payloads must include `canonical_id`, `object_kind`, `content_hash`, and `owner_id`.

## 7. Vector index shape

Qdrant collections are derived from the object graph.

```text
Collection naming: choir_{owner_id}_{object_kind}_v{index_version}

Point payload:
  canonical_id: string
  object_kind: string
  content_hash: string
  owner_id: string
  text: string | null
  embedding_model: string
  embedding_version: string
  metadata: JSON
```

Update flow:
1. Select canonical corpus and embedding model/version.
2. Build a shadow collection.
3. Verify counts, hashes, sample queries, metadata coverage, latency.
4. Atomically switch alias to the new collection.
5. Keep the old collection for a rollback TTL.
6. Garbage-collect after confidence window.

## 8. First migration: source entities

The first object kind to migrate into the graph is `choir.source_entity`.

Current state: source entities are embedded as JSON arrays in run metadata and revision metadata. They are not durable objects.

Target state:
- Every source entity is a `choir.source_entity` object with a `canonical_id`.
- Texture revisions cite them via `choir.source_ref` edge objects.
- Researchers create `choir.source_entity` objects and propose `choir.source_ref` edges.
- The vector index contains the embeddings of source entity text.

This migration directly fixes the Texture source-citation bug and gives the trajectory supervisor a real object to validate.

## 9. Schema evolution

The schema is versioned. Object kinds and edge kinds are registered in a registry. Migrations are typed deltas:

1. Add a new object kind.
2. Add a new edge kind.
3. Backfill existing data into the new object kind.
4. Update the app that consumes the old representation to read the new object.
5. Deprecate the old representation.

No big-bang rewrites. Every change is a schema delta with a rollback path.

## 10. Open questions

- Should object IDs be UUIDs, content hashes, or a composite like `kind:owner:hash`?
- Should paragraphs be separate objects or embedded in the document body?
- How do we represent encrypted objects (e.g. private emails) in the graph?
- Should Qdrant collections be per-user, per-computer, or per-object-kind?
- What is the canonical serialization format for `content_hash`?

These are answered in the first migration, not in this doc.
