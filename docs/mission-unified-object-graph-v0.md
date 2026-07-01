# Mission: Unified Object Graph — Audited Computer + Publication Layer

**Status:** planned — design phase
**Date:** 2026-07-01
**Beads:** `choir-bqbg` (epic)
**Related:** `choir-2oi` (agent-api-graph-native), `choir-zwk` (texture-kinds-objectgraph), `choir-avo6` (mutation-transaction-coordinator), `choir-udjd` (store consolidation, settled PR #37)

## Objective

Fold all relational data models into a single object graph (`og_objects` + `og_edges`) in every Dolt instance. The VM's object graph IS the audited computer. corpusd's object graph IS the platform publication record. Same schema, same query patterns, same API surface.

## What's Broken

Today there are three parallel data models:

1. **VM embedded Dolt `store.Store`** — ~36 relational tables for runs, agents, trajectories, texture documents, revisions, events, channel messages, desktop state, app promotion, media, browser sessions. This is the "audited computer" but it's not modeled as a graph.

2. **corpusd publication tables** — ~20 relational tables: publications, publication_versions, public_routes, artifact_manifests, artifact_blobs, provenance_entities, provenance_activities, provenance_agents, provenance_edges, consent_records, review_records, retrieval_sources, retrieval_spans, retrieval_manifests, citation_edges, publication_source_entities, publication_transclusions, publication_policies, verifier_attestations, platform_subjects, publication_proposals. These are a parallel world from the object graph.

3. **corpusd object graph** — `og_objects` + `og_edges` for web captures, source entities, ingestion handoffs only. A small island in a sea of relational tables.

The object graph should be the single substrate. Everything else is a symptom of the relational model that preceded it.

## The Unified Model

### Core Schema (already exists)

```sql
og_objects (
  canonical_id  VARCHAR(255) PRIMARY KEY,
  object_kind   VARCHAR(128),
  owner_id      VARCHAR(255),
  computer_id   VARCHAR(255),
  version_id    VARCHAR(255),
  content_hash  VARCHAR(128),
  body          LONGBLOB,
  metadata      LONGTEXT,      -- JSON
  created_at    DATETIME,
  updated_at    DATETIME,
  tombstone     BOOLEAN,
  superseded_by VARCHAR(255)
)

og_edges (
  edge_id    VARCHAR(255) PRIMARY KEY,
  from_id    VARCHAR(255),
  to_id      VARCHAR(255),
  kind       VARCHAR(128),
  metadata   LONGTEXT,      -- JSON
  created_at DATETIME,
  tombstone  BOOLEAN
)
```

### Object Kinds

#### VM Store (audited computer)

| Kind | Replaces | Body | Metadata |
|------|----------|------|----------|
| `choir.agent` | `agents` | — | profile, role, sandbox_id, channel_id |
| `choir.run` | `runs` | — | state, prompt, result, error, agent_profile, agent_role, sandbox_id, finished_at |
| `choir.event` | `events` | payload_json | seq, stream_seq, kind, phase, ts |
| `choir.channel_message` | `channel_messages` | content | from_name, role, seq |
| `choir.inbox_delivery` | `inbox_deliveries` | content | role, delivered_at |
| `choir.run_memory_entry` | `run_memory_entries` | message_json | seq, kind, role, summary, model |
| `choir.trajectory` | `trajectories` | — | kind, subject_refs, status, settlement_rule |
| `choir.work_item` | `work_items` | — | objective, reason, authority_profile, step_budget, status |
| `choir.run_acceptance` | `run_acceptances` | — | acceptance_level, state, ci_run_id, deploy_run_id, evidence_refs |
| `choir.run_continuation` | `run_continuations` | — | objective, lease_seconds, status |
| `choir.texture_document` | `texture_documents` | — | title, current_revision_id |
| `choir.texture_revision` | `texture_revisions` | content, body_doc_json | version_number, revision_hash, author_kind, source_entities, citations, provenance |
| `choir.texture_decision` | `texture_decisions` | — | decision_kind, reason, evidence_refs, next_action |
| `choir.agent_evidence` | `agent_evidence` | content | kind, source_uri, title |
| `choir.content_item` | `content_items` | text_content | source_type, media_type, title, source_url, content_hash |
| `choir.podcast_subscription` | `podcast_subscriptions` | — | feed_url, title, author, artwork_url |
| `choir.browser_session` | `browser_sessions` | text_snapshot, html_snapshot | provider, mode, state, current_url, title, screenshot |
| `choir.app_change_package` | `app_change_packages` | manifest_json | app_id, status, source_candidate_ref, trace_id |
| `choir.app_adoption` | `app_adoptions` | — | app_id, status, target_computer_id, merge_strategy, trace_id |
| `choir.desktop_session` | `desktop_sessions` | — | device_id, viewport_profile, visibility_state |
| `choir.desktop_app_instance` | `desktop_app_instances` | app_context_json | app_id, title, lifecycle, shared_stack_rank |

#### corpusd (publication layer)

| Kind | Replaces | Body | Metadata |
|------|----------|------|----------|
| `choir.subject` | `platform_subjects` | — | subject_kind, display_name, canonical_uri |
| `choir.publication` | `publications` | — | handle, slug, title, state, latest_version_id |
| `choir.publication_version` | `publication_versions` | — | edition_label, source_doc_id, source_revision_id, source_revision_hash, projection_hash, artifact_manifest_id, published_at, supersedes_version_id |
| `choir.publication_proposal` | `publication_proposals` | — | source_doc_id, source_revision_id, projection_hash, title, state |
| `choir.public_route` | `public_routes` | — | handle, route_path, target_kind, target_id, target_version_id, state |
| `choir.artifact_manifest` | `artifact_manifests` | manifest_json | subject_kind, subject_id, media_type, manifest_hash |
| `choir.artifact_blob` | `artifact_blobs` | — | hash_algorithm, media_type, byte_size, storage_ref |
| `choir.provenance_entity` | `provenance_entities` | — | entity_kind, canonical_uri |
| `choir.provenance_activity` | `provenance_activities` | — | activity_kind, trace_id, run_id, started_at, ended_at |
| `choir.provenance_agent` | `provenance_agents` | — | agent_kind, subject_id, model, provider, vm_id |
| `choir.consent_record` | `consent_records` | — | target_kind, target_id, action, state, evidence_ref |
| `choir.review_record` | `review_records` | body | target_kind, target_id, decision |
| `choir.retrieval_source` | `retrieval_sources` | — | source_kind, canonical_uri, license, visibility, state |
| `choir.retrieval_span` | `retrieval_spans` | — | source_version_id, selector_kind, selector_json, text_hash, token_count |
| `choir.retrieval_manifest` | `retrieval_manifests` | — | output_kind, output_id, query_or_objective_hash, selected_refs, rejected_refs |
| `choir.publication_source_entity` | `publication_source_entities` | entity_json | kind, target_kind, target_id, display_policy, open_surface |
| `choir.publication_transclusion` | `publication_transclusions` | snapshot_text | host_selector_json, source_selector_json, relation_type, default_display_mode, access_policy, export_policy |
| `choir.publication_policy` | `publication_policies` | — | access_policy_json, export_policy_json |
| `choir.verifier_attestation` | `verifier_attestations` | evidence_json | target_kind, target_id, verifier_kind, result, predicate_type, subject_digest |

#### Already registered (keep)

| Kind | Status |
|------|--------|
| `choir.source_entity` | registered |
| `choir.source_ref` | registered |
| `choir.web_capture` | registered |
| `choir.universal_wire_story_cluster` | registered |
| `choir.media_item` | registered |
| `choir.audio_recording` | registered |
| `choir.transcript` | registered |
| `choir.autoradio_run_sheet` | registered |

### Edge Kinds

#### Structural edges

| Kind | From → To | Replaces |
|------|-----------|----------|
| `has_version` | publication → publication_version | FK in publication_versions |
| `supersedes` | publication_version → publication_version | supersedes_version_id |
| `derived_from_proposal` | publication_version → publication_proposal | proposal_id FK |
| `routes_to` | public_route → publication/publication_version | public_routes.target_* |
| `has_manifest` | publication_version → artifact_manifest | artifact_manifest_id FK |
| `contains_blob` | artifact_manifest → artifact_blob | artifact_blobs.artifact_manifest_id |
| `owns` | subject → publication | publications.owner_id |
| `has_agent` | subject → provenance_agent | provenance_agents.subject_id |
| `document_revision` | texture_revision → texture_document | texture_revisions.doc_id |
| `revision_parent` | texture_revision → texture_revision | parent_revision_id |
| `run_agent` | run → agent | runs.agent_id |
| `run_trajectory` | run → trajectory | runs.trajectory_id |
| `run_parent` | run → run | runs.requested_by_run_id |
| `event_run` | event → run | events.loop_id |
| `message_from_run` | channel_message → run | channel_messages.from_loop_id |
| `message_to_run` | channel_message → run | channel_messages.to_loop_id |
| `work_item_trajectory` | work_item → trajectory | work_items.trajectory_id |
| `work_item_assigned_agent` | work_item → agent | work_items.assigned_agent_id |
| `acceptance_run` | run_acceptance → run | run_acceptances.loop_id |
| `acceptance_trajectory` | run_acceptance → trajectory | run_acceptances.trajectory_id |
| `continuation_from_run` | run_continuation → run | run_continuations.source_loop_id |
| `continuation_to_run` | run_continuation → run | run_continuations.next_loop_id |
| `decision_document` | texture_decision → texture_document | texture_decisions.doc_id |
| `decision_run` | texture_decision → run | texture_decisions.loop_id |
| `evidence_agent` | agent_evidence → agent | agent_evidence.agent_id |
| `subscription_content` | podcast_subscription → content_item | podcast_subscriptions.content_id |
| `browser_session_run` | browser_session → run | browser_sessions.source_loop_id |
| `package_source_computer` | app_change_package → computer | app_change_packages.source_computer_id |
| `adoption_package` | app_adoption → app_change_package | app_adoptions.package_id |
| `adoption_target_computer` | app_adoption → computer | app_adoptions.target_computer_id |
| `session_desktop` | desktop_session → desktop | desktop_sessions.desktop_id |
| `app_instance_desktop` | desktop_app_instance → desktop | desktop_app_instances.desktop_id |

#### Provenance edges (already edges in relational model)

| Kind | From → To | Replaces |
|------|-----------|----------|
| `was_derived_from` | provenance_entity → provenance_entity | provenance_edges |
| `was_generated_by` | provenance_entity → provenance_activity | provenance_edges |
| `was_associated_with` | provenance_activity → provenance_agent | provenance_edges |
| `generated` | provenance_activity → provenance_entity | provenance_edges |
| `attested` | provenance_agent → verifier_attestation | implied |
| `attests_to` | verifier_attestation → any | verifier_attestations.target_* |
| `granted_consent` | subject → consent_record | implied |
| `consent_for` | consent_record → any | consent_records.target_* |
| `authored_review` | subject → review_record | implied |
| `reviews` | review_record → any | review_records.target_* |
| `contains_span` | retrieval_source → retrieval_span | retrieval_spans.source_id |
| `has_retrieval_manifest` | any → retrieval_manifest | retrieval_manifests.output_* |
| `references_entity` | publication_version → publication_source_entity | publication_source_entities.publication_version_id |
| `transcludes` | publication_version → publication_transclusion | publication_transclusions.publication_version_id |
| `transcludes_from` | publication_transclusion → publication_source_entity | publication_transclusions.source_entity_id |
| `has_policy` | publication_version → publication_policy | publication_policies.publication_version_id |

#### Already registered (keep)

| Kind | Status |
|------|--------|
| `cites` | registered |
| `captured_from` | registered |
| `derived_from` | registered |
| `has_media` | registered |
| `has_transcript` | registered |
| `contains` | registered |
| `references` | registered |

### Tables that become edges only (no object)

| Table | Edge kind | From → To |
|-------|-----------|-----------|
| `provenance_edges` | (use edge_kind column) | from_id → to_id |
| `citation_edges` | (use relation_type column) | from_id → to_id |
| `texture_document_aliases` | `document_alias` | owner/path → texture_document |
| `texture_agent_mutations` | `document_mutation` | run → texture_document |
| `texture_controller_checkpoints` | `document_checkpoint` | owner → texture_document |
| `coagent_mailboxes` | `coagent_mailbox` | agent → channel |
| `co_super_slots` | `super_slot` | trajectory → run |
| `computer_source_lineages` | `computer_lineage` | owner → computer |
| `media_progress` | `media_progress` | owner → media_identity |
| `media_recents` | `media_recent` | owner → media_identity |
| `user_preferences` | `user_preference` | owner → preference_key |
| `desktop_state` | `desktop_state` | owner → desktop |
| `desktop_workspaces` | `desktop_workspace` | owner → desktop |
| `desktop_window_placements` | `window_placement` | session → app_instance |
| `worker_updates` | `worker_update` | agent → target_agent |

## Implementation Plan

### Phase 1: Registry + Schema (green/yellow)

Register all new object kinds and edge kinds in `objectgraph.DefaultRegistry()`. No runtime behavior change.

### Phase 2: corpusd Publication Store (orange)

Implement `PublicationGraphStore` in `internal/platform/` that writes publications, versions, routes, manifests, blobs, attestations, provenance, citations, consent, review, retrieval, transclusions, policies as objects + edges. Replace the relational `Store` methods called by `Service.PublishTexture` and related handlers.

Migration: write to both relational tables and object graph in parallel. Once verified, drop the relational tables.

### Phase 3: VM Store (orange)

Implement `GraphStore` in `internal/store/` that writes runs, agents, trajectories, events, messages, texture documents, revisions, decisions, evidence, desktop state, app promotion as objects + edges. Replace the relational `Store` methods.

Migration: same dual-write pattern as Phase 2.

### Phase 4: Unified API (orange)

Expose the object graph as the primary API surface in both corpusd and VMs. Agents query and mutate the graph directly (choir-2oi). The relational query helpers become thin wrappers over graph queries.

### Phase 5: Drop Relational Tables (yellow)

Once all reads and writes go through the object graph, drop the relational tables. The object graph is the only substrate.

## Mutation Class

- Phase 1: green (registry only)
- Phase 2-4: orange (runtime behavior, data model)
- Phase 5: orange (schema migration, data deletion)

## Rollback Path

- Phases 1-4: dual-write means relational tables remain valid. Roll back by switching reads back to relational.
- Phase 5: before dropping tables, take a Dolt snapshot. Roll back by restoring the snapshot and switching reads back to relational.

## Conjecture

The relational model was necessary for rapid development, but it creates parallel data models that are hard to query across boundaries. The object graph unifies them: every entity is an object, every relationship is an edge, every Dolt instance has the same schema. The VM's object graph is the audited computer — a complete, traversable record of what happened. corpusd's object graph is the platform publication record — durable, publicly addressable, verifiable.
