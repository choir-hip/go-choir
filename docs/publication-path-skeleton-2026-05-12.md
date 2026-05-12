# Publication Path Skeleton

Date: 2026-05-12

This is the forward-compatible publication boundary for Choir. It is not a
publication implementation and it does not introduce the citation economy,
paywalls, collaboration markets, or CHIPS mechanics.

## State Boundaries

User desktop state is private by default.

- Per-user embedded Dolt stores private `vtext` documents, revisions, citations
  attached to private revisions, appagent state, findings, evidence metadata,
  and publication staging metadata.
- The per-user snapshot filesystem stores uploads, large media, artifacts,
  working trees, generated outputs, and filesystem aliases for Dolt-backed
  documents.
- A user desktop can keep producing local private versions after a publication
  event. Publishing a revision does not freeze the private document.

Platform publication state is public or platform-visible by explicit event.

- Platform Dolt stores publication records, public artifact metadata, published
  revision refs, citation graph edges, public routing records, compute
  accounting, and later CHIPS state.
- Platform Dolt is a ledger and index, not a hot-path mailbox or a dump of every
  private user event.
- Large public artifacts should live in platform storage with content-addressed
  hashes and platform Dolt rows pointing at them.

The first invariant: publication copies selected immutable refs from a private
trust domain into a platform-visible trust domain. It does not give the platform
desktop write access to the user's private live document.

## Platform Dolt Appliance Boundary

The platform Dolt appliance is the durable publication ledger.

Initial tables should be conceptually equivalent to:

```text
published_vtexts
  publication_id
  owner_id
  source_desktop_id
  source_doc_id
  source_revision_id
  source_revision_hash
  title
  publication_state
  visibility
  created_at
  published_at

published_vtext_versions
  publication_version_id
  publication_id
  source_revision_id
  source_revision_hash
  content_hash
  artifact_ref
  edition_label
  created_at

published_artifacts
  artifact_id
  publication_id
  content_hash
  media_type
  storage_ref
  byte_size
  created_at

publication_events
  event_id
  publication_id
  event_type
  actor_id
  source_trace_id
  payload_json
  created_at
```

Do not use platform Dolt as the live editor, actor mailbox, or cross-VM message
bus. It records durable publication facts and supports public queries.

## First Publish Operation

The first publish operation is deliberately narrow:

```text
selected user VText revision
  -> validate source ownership and revision immutability
  -> compute content hash and gather citation/artifact refs
  -> create a publication staging record in user embedded Dolt
  -> copy selected content/artifact refs to platform storage
  -> create platform Dolt publication rows
  -> expose the published snapshot in a platform publishing desktop
  -> emit Trace/publication events tying the operation to source doc/revision
```

Properties:

- The unit is one selected `vtext` revision, not the whole mutable document.
- The source revision remains private-user-owned.
- The platform version is immutable once published.
- Later local revisions can remain unpublished or become later public editions.
- Rollback hides or supersedes a publication record; it does not rewrite the
  source private revision history.

## Platform Publishing Desktop

The platform publishing desktop is the public/read-optimized runtime for
published artifacts.

It can host:

- public VText readers
- rendered published snapshots
- cached Pretext/transclusion renderers
- public media references
- citation previews
- later public collaboration/submission flows

It should not host:

- private user appagent state
- private drafts not selected for publication
- active user desktop mutation flows
- super/cosuper mutable development work

## Forward-Compatible Open Questions

Version publishing:

- Do users publish one revision, a selected range, all versions up to N, or all
  versions by default?
- Should "publish all versions" remain a purity mode while selected snapshots
  are the default UX?

Redaction/projection:

- Should redaction produce a separate public projection with its own hash?
- How do we preserve provenance when a public version omits private sections?

Paywalls and release timing:

- Should authors support delayed public release, paid latest versions, or
  subscriber-only versions?
- How do paywalled versions interact with citation visibility?

Collaboration:

- Should third-party submissions be public proposals, private inbox items, or
  economic offers?
- What is the approval/deny flow and how does it affect contributor credit?

Citations:

- What is the first public citation edge: manual citation, transclusion ref, or
  agent-proposed citation candidate?
- How do citations attach to exact version hashes rather than mutable titles?

CHIPS economics:

- Do not implement token mechanics yet.
- Preserve publication, citation, artifact, and compute records so CHIPS can be
  priced over real provenance later.

## Non-Goals For The Current Mission

- No CHIPS wallets, staking, or token billing.
- No public citation scoring.
- No paywall implementation.
- No collaboration marketplace.
- No full platform Dolt deployment.
- No migration of private user VText state into platform Dolt.
