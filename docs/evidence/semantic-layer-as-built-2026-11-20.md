# The semantic layer as built: a survey for the unified events-and-entities design (2026-11-20)

Class: green (a record; nothing changed but this file). Read-only survey of
`origin/main` at `b90c13e7c` (the primary checkout, read through the lane
`lead-docs`) and of production on 2026-11-20 between 22:22Z and 22:30Z. Code
facts carry `file:line`. Production facts are counts read from corpusd's
`graph` database through `ssh node-b` and `sudo -n -u postgres psql -At -d
graph` with `set default_transaction_read_only=on;` (bounded counts by kind,
no text scans), and from public reads of `https://choir.news`. Where I did not
read a thing I say so (last section). A name, a comment or a design doc is not
treated as evidence that a thing is built.

Tags: **M** measured on 2026-11-20 by me; **E** a number from a record in
`docs/evidence` that I did not re-measure; **C** read from code at `origin/main`;
**U** not determined.

## The answer on one page

**What is unified.** One registry (`internal/objectgraph/registry.go:72-398`)
names 109 object kinds and 86 edge kinds, every durable write is checked
against it (`registry.go:420`), every kind has exactly one custodian derived
from the desk and kernel-service manifests (`objectgraph/custody.go:44-50`),
and every object id has the one shape `obj:<kind>:<base64 owner>:<suffix>`
(`objectgraph/object.go:124-135`). The same code runs on two stores: each
computer's own SQLite `state.db` and corpusd's PostgreSQL `graph` database
(the Autopaper object graph). That is a real, enforced substrate. It holds
2,332,859 objects and 1,731,783 edges in the shared graph today (**M**), and
86% of the rows are publication plumbing (transclusions, checks,
provenance, manifests: 16 kinds); the semantic facts about the world
are a small part of it.

**What is ad hoc.** The facts a reader of the news would call semantic are held
in at least nine places in six shapes, and only some are graph kinds:

| Fact | Where it is held | Graph kind? | Rows today |
|---|---|---|---|
| a capture (page, text, labels) | `choir.web_capture` in corpusd; text blobs and a per-computer FTS5 file; `proj_capture_index`, `proj_capture_terms` side tables | yes (plus side tables) | 237,050 |
| a capture's meaning | one halfvec(256) of title plus lead passage in `vec_captures` | no, a side table | 237,073 |
| a story and its versions | `choir.publication`, `choir.publication_version` and 17 kinds of plumbing; story passages in `vec_story_passages` | yes (plus side table) | 10,903 and 36,458 |
| story cites capture | `cites_capture` edge from a version to a capture | yes | 202,741 edges; 51,501 distinct captures (21.7% of captures) |
| a world event, and a capture's membership | `choir.world_event`, `choir.event_membership` on the paper computer's own `state.db`, custodian management | yes, but **on the computer, not in the shared graph** | about 10,153 events at 05:26Z (E); none in the graph DB |
| an entity | `choir.entity` in corpusd, owner-scoped, from the research desk's `story.entities` tape record | yes | 6,995 |
| story about entity | `mentions_entity` edge, role central or incidental, publication to entity | yes | 23,966 edges; 5,775 stories |
| capture about entity | nothing (E3 not built) | no | 0 |
| capture in event, story reports event | `in_event`, `reports_event` | `in_event` registered, `reports_event` not; neither written | 0 |
| a capture's resolution state | `proj_capture_resolution` | no, a side table | 0 rows (written by tests only) |
| related stories | `related_story` edge from a research desk's judgment; `relies_on`; plus a walk over shared citations and shared central entities | yes | 4 and 1 edges |
| a capture's topic and event "label" | `choir.capture_label`, a Jev answer on the computer (event: story, sibling or new; topic: a story) | yes, on the computer | U |
| search leads | `search_leads` table in corpusd, never evidence | no, a side table | 43,766 |
| podcast clip to story | `story_route` string inside `choir.podcast_clip` | kind, but no edge | 496 |
| front importance and section | a JSON file a host timer writes; corpusd reads it | no, a file | U |

**The five largest inconsistencies.**

1. **Events are not in the graph the rest of the layer lives in.** Events and
   memberships are objects of the paper computer's own store (id
   `ev-<20 hex>` from the computer id and the opening capture,
   `agentcore/event_layer.go:296-301`), custodian management, read by research
   through an in-memory substring match over the newest 3,000 events
   (`researchtools/corpus_events.go:54-80`; there were about 10,153 events on
   2026-11-20). Entities, stories and captures are in corpusd under owner-scoped
   canonical ids. No edge joins them: `in_event` has zero rows
   (`platform/capture_ready.go:247` reads it, nothing writes it), a story has no
   event id, and `basis.events` of the public related answer is always 0
   (**M**: `{"captures":25,"events":0}` on the front story). The public
   `/api/autopaper/events` answers 401 without the paper's key (**M**), so the
   instrument the design needs does not exist in public form yet.
2. **The events are almost all singletons, so they carry little.** 282 of the
   300 newest events had one member (E, `research-runs-with-nothing-new-2026-11-20.md:201`);
   84.1% of the questions over 03:28Z to 05:12Z minted a new event and 2.8%
   joined (E, `event-join-rate-hand-read-2026-11-20.md` section 1). Of 50 hand-read
   minted events, 10 should have joined an existing one (E).
3. **Entity identity is the normalised name, and quality is poor.** `ent:` plus
   24 hex of a hash of the normalised name (`types/story_entity.go:87-130`);
   4 of 6,995 entities have a Wikidata id (**M**); of 200 sampled attachments
   29.0% are not entities and 23.9% of the entity rows have a second id for the
   same thing (E, `entity-layer-baseline-hand-read-2026-11-20.md` section 2).
   Nothing can retire or merge an entity. The front list shows `Markets` and
   `IPO` as central entities of the lead story today (**M**).
4. **One concept, several id forms that do not join in one expression.** A
   story is `pub-<32 hex>` on the tape, the front list and the entity page;
   `obj:choir.publication:<b64 owner>:key-<64 hex>` in the graph, the vectors and
   every edge (`platform/publication_related_story.go:200-205`); a Texture
   `doc_id` on the computer; a route path in a podcast clip. A capture is
   `obj:choir.web_capture:...` in corpusd, a frame handle `r1.c1` in a Texture
   cell. An outlet is computed by two different functions (`outlet.Of`, which
   folds `bbc.co.uk` into `bbc.com` and drops aggregators; `registrableDomain`
   from the public suffix list, `platform/capture_ready.go:292`) and is a
   string, never an object. "Source" means four things. A "subject"
   (`choir.subject`) is a user principal, not a topic.
5. **Edges are written that nothing reads, and reads exist for edges nothing
   writes.** Write-only in non-test code: `was_derived_from`,
   `was_associated_with`, `attested`, `granted_consent`, `authored_review`,
   `derived_from_proposal`, `owns`, `has_agent` (each 22 to 36,461 rows, only
   `platform/publication_graph.go` names them). Read, never written:
   `in_event` (0 rows), and the capture-side `mentions_entity` the ready list
   reads (0 rows). Registered with no rows in the graph DB: 56 of 86 edge kinds (about 40 are computer-side and are expected to be absent; graph-side and empty: `cites`, `captured_from`, `derived_from`, `has_media`, `has_transcript`, `contains`, `was_generated_by`, `transcludes_from`, `in_event`), plus object kinds
   `choir.take_outcome` and `choir.package_force_request`. `relies_on` has one
   row and "ranks nothing and no desk sees it" (`registry.go:357-359`);
   `related_story` has four.

**What an agent cannot ask today.** An agent's cell has no verb that reads an
entity, an event by id, an outlet, or a graph edge. `choir.SearchWireCorpus`
returns events (substring), captures and published pieces, none of them with
entity ids; `choir.Relate` writes entities and reads none; the public entity
page, the related-stories answer and the front list are HTTP reads that a
research cell cannot call. So an agent cannot ask: which published stories
report this event; what do we hold about this entity since T; which captures of
this event are newer than the story's last version; who are the central
entities of the stories citing this outlet; which stories contradict each
other; which captures mention this entity; what is the event's follow-up chain.
Section 4 lists each with what is missing.

**The design consequence in one sentence.** The substrate for a unified layer
exists (one registry, one id shape, one custody rule, two stores, one
shared database with vectors and graph in the same Postgres), but the
semantic facts were each added by a different feature with its own home, id and
reader, so the unification is mostly a matter of choosing one home for events,
joining them to the corpusd ids that stories and captures already use, and
giving agents one query verb over it, not of building a new store.

---

## 1. The object graph as built

### 1.1 What it is, where it lives

| Fact | Detail | Evidence |
|---|---|---|
| Two stores, one schema family | A computer's own SQLite `state.db` (headers, `og_bodies`, generated columns; `objectgraph/sql_store.go:231-300`) and corpusd's PostgreSQL `graph` database (`platform/corpus_schema.go:96-135`). `docs/computer-ontology.md` "Store Taxonomy" calls the second "the Autopaper object graph" and the first "a projection of the computer's tape, rebuilt by replay". | C |
| Per-computer private state | Everything under the 66 `OwnerVMStore` kinds below is written to the computer's `state.db`, replays from the tape, and is restored with the computer. It is never in the graph DB (the graph DB holds 0 rows of the VM-store kinds, **M**). The paper's world events and memberships are here. | C, M |
| The Autopaper object graph | The 40 `OwnerCorpusd` kinds below (and 3 reserved `OwnerAutoradio` kinds with no writer) (captures, publications and their plumbing, entities, podcast, takes, library) live in `graph`, outside a computer's restore set. Captures are the one shared kind written by many computers (35 owners, 35 computers hold `choir.web_capture`; one holds 233,339 of 237,050) (**M**). | C, M |
| Registry | `DefaultRegistry()` `registry.go:72-398`. The kind's custodian is not written there; `DeriveCustody` (`custody.go`) takes it from `deskmanifest.OwnedKinds` and `kernelservice` manifests, and the load fails when a kind has none or two (`registry.go:268`). App-package kinds live under the `choir.app.` namespace (`package_kinds.go`) and are not in this table. | C |
| Write gate | `CheckWrite` (`registry.go:420`): an unregistered object or edge kind is refused by both stores before anything else. A read gate checks the declared reader against custody (`reader.go:195-260`); corpusd kinds are readable by any declared reader (`reader.go:212`). | C |
| Object id | `obj:<kind>:<base64url owner>:<suffix>`; suffix is `key-<sha256 of the identity key>` for external-key kinds or a content hash (`object.go:124-135`, `object.go:214`). The owner is inside every id: the same page captured by two owners, or "Anthropic" for two owners, is two objects. | C |
| Edge id | a hash of from, to, kind and normalised metadata (`object.go:218`): the same edge written twice is one row, and a role change is a new edge (the old one is tombstoned, `publication_story_entities.go:123-135`). | C |

### 1.2 Tables, columns, indexes and collations (graph DB, **M** from `information_schema` and `pg_indexes`)

| Table | Columns | Indexes | Collation |
|---|---|---|---|
| `og_objects` | `canonical_id` (PK), `object_kind`, `owner_id`, `computer_id`, `version_id`, `content_hash`, `body bytea`, `body_ref`, `body_size`, `metadata text` (JSON as text, not jsonb), `created_at`, `updated_at`, `tombstone`, `superseded_by` | `og_objects_pkey`, `(object_kind, owner_id)`, `(object_kind, owner_id, computer_id, updated_at)`, `(updated_at)` | `canonical_id` is **COLLATE "C"**; every other text column is the database default (en_US.UTF-8) |
| `og_edges` | `edge_id` (PK), `from_id`, `to_id`, `kind`, `metadata text`, `created_at`, `tombstone` | `(from_id)`, `(to_id)`, `(to_id, kind)`, partial `(to_id, kind, created_at DESC, edge_id) WHERE NOT tombstone` | **none is C**: `from_id`, `to_id`, `kind` are default collation |
| `og_lookup_keys` | `object_kind`, `lookup_key`, `canonical_id` | PK `(object_kind, lookup_key, canonical_id)`, `(canonical_id)` | `lookup_key` and `canonical_id` are **C** |
| `proj_capture_index` | `capture_id`, `owner_id`, `computer_id`, `url_key`, `canonical_key`, `url`, `canonical_url`, `host`, `source_id`, `item_id`, `language`, `title`, `has_blocks`, `fetched_at`, `published_at`, `updated_at` | PK `capture_id`; `(owner, computer, url_key, fetched_at DESC)`, `(canonical_key)`, `(fetched_at)`, `(host)`, `(source_id)` | default (a copy of the capture id in the default collation) |
| `proj_capture_terms` | `owner_id`, `computer_id`, `term`, `capture_id`, `fetched_at` | PK `(owner, computer, term, capture_id)`, `(capture_id)`, a covering index | `term` is **C**; `capture_id` default |
| `proj_capture_resolution` | `capture_id`, `owner_id`, `computer_id`, `outcome`, `record_id`, `resolved_at` | PK `capture_id`, `(owner, computer, resolved_at DESC)` | default |
| `vec_captures` | `capture_id`, `owner_id`, `computer_id`, `text_sha`, `day`, `model`, `embedding halfvec`, `refused`, `embedded_on` | PK, `(owner, computer, day)`, HNSW `halfvec_cosine_ops`, partial `(model) WHERE refused<>''` | default |
| `vec_story_passages` | `story_id`, `version_id`, `ordinal`, `text_sha`, `day`, `model`, `embedding halfvec`, `refused`, `embedded_on` | PK `(story_id, version_id, ordinal)`, `(version_id)`, `(day)`, HNSW | default |
| `search_leads` | `owner_id`, `computer_id`, `url_key`, `url`, `title`, `snippet`, `published_at`, `first/last_returned_at`, `times_returned`, `query_keys[]`, `conversation_ids[]`, `tried_at`, `capture_id`, `tsv` | PK `(owner, computer, url_key)`, GIN `tsv`, `(last_returned_at)` | default |
| `publication_transclusions`, `publication_version_proposals`, `platform_texture_documents`, `platform_texture_revisions`, `proposal_delivery_records` | publication plumbing, not semantic | | |

What the collations mean for a designer. `og_objects.canonical_id` is C and
`og_edges.from_id/to_id` are not, so a byte-prefix range such as
`from_id >= 'obj:choir.publication:' AND from_id < 'obj:choir.publication;'` is
empty on production (':' sorts after ';' in en_US), which is exactly the
production defect recorded in `docs/problems/entity-read-lists-no-stories-in-production-2026-11-20.md`
and pinned by `platform/edge_collation_test.go:1-16`. Equality joins across the
two columns work: I ran `og_edges e JOIN og_objects o ON o.canonical_id =
e.from_id` as a read-only count and it answered (317 rows for the last hour of
`cites_capture` edges) (**M**). Every reader of edges by prefix must use
`starts_with()`, `LIKE` or an exact id list, as `ListEdgesFromAny` does
(`platform/entity_page.go:411`). The vector tables and `proj_capture_*` carry
the capture id in the default collation, so a capture id compared against
`og_objects.canonical_id` is an equality only.

### 1.3 Object kinds (all 109), their custodian, id mode and rows

"Store side" says where the kind is written: `computer` (the computer's own
`state.db`; `OwnerVMStore`), `corpusd` (`graph`; `OwnerCorpusd`) or
`autoradio(reserved)` (no writer). Custodian is derived from
`deskmanifest/manifest.go:350-480` (desk `OwnedKinds`) and
`kernelservice/kernelservice.go:66-124` (service `OwnedKinds`). Rows are in the
shared graph DB only (**M**, 22:2xZ); `-` means a computer-side kind that never
appears there. The last column lists the first non-test files that name the
kind as a literal or constant (writer and reader files are not separated; read
the file to see which). Semantic kinds are marked in the next subsection.

| reg line | kind | store side | identity | custodian | rows in graph DB | writer/reader files (first, non-test) |
|---|---|---|---|---|---|---|
| 76 | choir.source_entity | computer | content | desk:texture | - | store/texture_source_graph.go |
| 77 | choir.source_ref | computer | content | desk:texture | - | objectgraph/derived_keys.go store/texture_source_graph.go |
| 78 | choir.web_capture | corpusd | content | system:corpusd | 237082 | objectgraph/web_capture.go |
| 81 | choir.media_item | autoradio(reserved) | content | system:autoradio | - | store/snapshot_to.go |
| 82 | choir.audio_recording | autoradio(reserved) | content | system:autoradio | - |  |
| 83 | choir.transcript | autoradio(reserved) | content | system:autoradio | - | sourcecontract/source_kind.go |
| 86 | choir.agent | computer | key | system:kernel | - | store/lifecycle_commit_act.go store/lifecycle_control_issue.go store/engineering_assignments.go store/texture_turn.go store/lifecycle.go store/story_research_request.go |
| 87 | choir.run | computer | key | system:kernel | - | objectgraph/derived_keys.go store/lifecycle_commit_act.go store/engineering_assignments.go store/store.go store/run_write_counters.go store/texture_turn.go |
| 88 | choir.event | computer | content | system:kernel | - | computerevent/projection_batch.go store/store.go store/stream_seq_lease.go store/event_table.go store/telemetry_retire.go store/telemetry_sealer.go |
| 89 | choir.channel_message | computer | content | system:kernel | - | store/graph_store.go |
| 90 | choir.inbox_delivery | computer | content | system:kernel | - | store/graph_store.go |
| 91 | choir.run_memory_entry | computer | content | system:kernel | - | store/graph_store.go store/snapshot_to.go |
| 92 | choir.trajectory | computer | key | system:kernel | - | routekinds/routekinds.go store/lifecycle_commit_act.go store/lifecycle_control_issue.go store/engineering_assignments.go store/texture_turn.go store/lifecycle_control_delivery.go |
| 93 | choir.work_item | computer | key | system:kernel | - | store/lifecycle_control_issue.go store/engineering_assignments.go store/trajectory.go store/texture_turn.go store/lifecycle.go store/lifecycle_continue.go |
| 94 | choir.run_acceptance | computer | key | system:kernel | - | store/graph_store.go |
| 95 | choir.run_continuation | computer | key | system:kernel | - | store/graph_store.go |
| 96 | choir.texture_document | computer | key | desk:texture | - | objectgraph/custody.go textureowner/notebook_owner.go textureowner/writer.go textureowner/texture_evidence_embed.go agentcore/story_entities_verb.go agentcore/capture_read_gate.go |
| 97 | choir.texture_revision | computer | content | desk:texture | - | objectgraph/custody.go agentcore/texture_owner_directions_cast.go agentcore/texture_frame.go agentcore/story_analysis.go routekinds/routekinds.go lease/capsule.go |
| 98 | choir.texture_decision | computer | key | desk:texture | - | agentcore/runtime.go store/graph_store.go |
| 99 | choir.agent_evidence | computer | key | desk:research | - | objectgraph/reader.go objectgraph/custody.go textureowner/texture_evidence_sources.go agentcore/subcall_judge.go store/graph_store.go store/snapshot_to.go |
| 100 | choir.content_item | computer | content | system:shell | - | objectgraph/reader.go sourcecontract/source_kind.go store/graph_store.go |
| 101 | choir.podcast_subscription | computer | key | system:shell | - | routekinds/routekinds.go store/graph_store.go |
| 102 | choir.browser_session | computer | key | system:shell | - | store/graph_store.go |
| 103 | choir.desktop_session | computer | key | system:shell | - | store/graph_store.go store/snapshot_to.go |
| 104 | choir.desktop_app_instance | computer | key | system:shell | - | store/graph_store.go store/snapshot_to.go |
| 105 | choir.worker_update | computer | key | system:kernel | - | store/lifecycle_commit_act.go store/lifecycle_control_issue.go store/store.go store/texture_turn.go store/lifecycle_control_delivery.go store/lifecycle.go |
| 106 | choir.coagent_mailbox | computer | key | system:kernel | - | store/graph_store.go |
| 107 | choir.source_schedule | computer | key | desk:management | - | agentcore/source_schedule.go agentcore/tools_product_api.go routekinds/routekinds.go store/source_schedule.go |
| 111 | choir.platform_poll | computer | key | system:kernel | - | agentcore/platform_poll.go store/platform_poll.go |
| 112 | choir.computer_setting | computer | key | system:shell | - | textureowner/texture_owner_directions.go agentcore/computer_settings.go autoputer/file_objects.go store/computer_setting.go store/snapshot_to.go |
| 115 | choir.engineering_lineage | computer | key | desk:engineering | - | agentcore/engineer_lineage.go routekinds/routekinds.go store/engineering_lineage.go |
| 118 | choir.texture_root | computer | key | desk:texture | - | agentcore/rebase_notice.go store/texture_root.go |
| 122 | choir.rebase_notice_claim | computer | key | system:kernel | - | store/rebase_notice_claim.go |
| 128 | choir.capture_label | computer | key | desk:management | - | objectgraph/reader.go agentcore/intake_labels.go store/capture_label.go |
| 129 | choir.capture_label_override | computer | key | desk:management | - | agentcore/intake_overrides.go store/capture_label.go |
| 131 | choir.world_event | computer | key | desk:management | - | objectgraph/reader.go agentcore/event_layer.go routekinds/routekinds.go store/world_event.go |
| 132 | choir.event_membership | computer | key | desk:management | - | objectgraph/reader.go agentcore/event_layer.go store/world_event.go |
| 140 | choir.desk_package_event | computer | key | system:kernel | - | store/desk_package_events.go |
| 141 | choir.lease | computer | key | system:lease | - | lease/capsule.go lease/lease.go store/lease.go |
| 142 | choir.package_install | computer | key | system:kernel | - | store/package_install.go |
| 143 | choir.package_install_pending | computer | key | system:kernel | - | store/package_install_pending.go |
| 144 | choir.release_adoption | computer | key | system:kernel | - | store/package_install_legacy.go |
| 149 | choir.file | computer | key | system:owner_files | - | computerversion/base_tree.go texturewrite/media_tokens.go texturewrite/apply.go texturedoc/embed.go autoputer/file_objects.go base/tree/tree.go |
| 150 | choir.file_path | computer | key | system:owner_files | - | store/file_object.go |
| 158 | choir.stream_seq_lease | computer | key | system:store | - | objectgraph/writer.go store/stream_seq_lease.go |
| 163 | choir.actor_wake_outbox | computer | key | system:kernel | - | objectgraph/derived_keys.go store/actor_wake_signal.go store/lifecycle.go store/graph_store.go store/lifecycle_expire.go |
| 164 | choir.capture_disposition | computer | key | desk:management | - | agentcore/management_tasks.go store/management_tasks.go store/inbox.go |
| 165 | choir.citation_check | computer | key | desk:texture | - | agentcore/calibration_feedback.go store/support.go |
| 166 | choir.commitment_record | computer | key | system:kernel | - | objectgraph/sql_store.go ledgerwitness/walk.go store/lifecycle_commit_act.go store/lifecycle_control_issue.go store/commitment.go store/inbox.go |
| 167 | choir.conversation_head | computer | key | system:kernel | - | store/engineering_assignments.go store/management_tasks.go store/inbox.go |
| 168 | choir.inbox_command | computer | key | system:kernel | - | store/inbox.go |
| 169 | choir.inbox_cursor | computer | key | system:kernel | - | store/inbox.go |
| 170 | choir.inbox_head | computer | key | system:kernel | - | store/inbox.go |
| 171 | choir.lifecycle_cancel_intent | computer | key | system:kernel | - | routekinds/routekinds.go store/lifecycle.go store/lifecycle_continue.go store/graph_store.go |
| 172 | choir.lifecycle_command | computer | key | system:kernel | - | routekinds/routekinds.go store/graph_store.go |
| 173 | choir.lifecycle_event | computer | key | system:kernel | - | routekinds/routekinds.go store/lifecycle_commit_act.go store/lifecycle_control_issue.go store/engineering_assignments.go store/texture_turn.go store/lifecycle_control_delivery.go |
| 174 | choir.lifecycle_sequence | computer | key | system:kernel | - | store/graph_store.go |
| 175 | choir.management_task | computer | key | desk:management | - | agentcore/capture_arrival.go agentcore/management_tasks.go store/management_tasks.go store/inbox.go |
| 176 | choir.migration_marker | computer | key | system:kernel | - | store/lifecycle.go store/graph_store.go |
| 177 | choir.scheduler_sequence | computer | key | system:kernel | - | store/graph_store.go |
| 178 | choir.story_library_head | computer | key | desk:texture | - | store/story_index.go store/management_tasks.go store/story_open.go |
| 180 | choir.story_index | computer | key | desk:texture | - | objectgraph/reader.go routekinds/routekinds.go store/story_index.go store/story_open.go |
| 181 | choir.support_verdict | computer | key | desk:texture | - | agentcore/support_judge.go agentcore/calibration_feedback.go routekinds/routekinds.go store/support.go |
| 184 | choir.co_super_assignment | computer | key | system:kernel | - | store/engineering_assignments.go |
| 185 | choir.co_super_assignment_report | computer | key | system:kernel | - | store/engineering_assignments.go |
| 186 | choir.co_super_subject_candidate | computer | key | system:kernel | - | store/engineering_assignments.go |
| 187 | choir.co_super_capability_claim | computer | key | system:kernel | - | store/engineering_assignments.go |
| 188 | choir.co_super_capsule_claim | computer | key | system:kernel | - | store/engineering_assignments.go |
| 192 | choir.co_super_slot | computer | key | desk:engineering | - | store/graph_home.go store/snapshot_to.go |
| 193 | choir.co_super_run_claim | computer | key | system:kernel | - | store/engineering_assignments.go |
| 196 | choir.subject | corpusd | key | system:corpusd | 22 | platform/publication_graph.go |
| 197 | choir.publication | corpusd | key | system:corpusd | 10904 | types/trajectory.go textureowner/algorithm2_owner.go textureowner/texture_next_actor.go agentcore/trajectory.go platform/publication_related_story.go platform/service.go |
| 198 | choir.publication_version | corpusd | key | system:corpusd | 36461 | platform/service.go platform/publication_version_read.go platform/publication_oped.go platform/retrieval_schema.go platform/publication_versions.go platform/publication_links.go |
| 199 | choir.publication_proposal | corpusd | key | system:corpusd | 36461 | platform/retrieval_schema.go platform/publication_graph.go |
| 203 | choir.oped_verdict | corpusd | key | system:corpusd | 2 | platform/publication_oped.go |
| 204 | choir.changelog_notice | corpusd | key | system:corpusd | 7 | platform/publication_links.go |
| 205 | choir.public_route | corpusd | key | system:corpusd | 10904 | platform/publication_reliance.go platform/service_publication_read.go platform/objectgraph_store.go platform/retrieval_schema.go platform/publication_graph.go |
| 206 | choir.artifact_manifest | corpusd | content | system:corpusd | 36461 | platform/publication_graph.go |
| 207 | choir.artifact_blob | corpusd | content | system:corpusd | 36074 | platform/publication_graph.go |
| 208 | choir.provenance_entity | corpusd | content | system:corpusd | 72922 | platform/publication_graph.go |
| 209 | choir.provenance_activity | corpusd | key | system:corpusd | 36461 | platform/publication_graph.go |
| 210 | choir.provenance_agent | corpusd | key | system:corpusd | 22 | platform/publication_graph.go |
| 211 | choir.consent_record | corpusd | key | system:corpusd | 36461 | platform/publication_graph.go |
| 212 | choir.review_record | corpusd | key | system:corpusd | 36461 | platform/publication_graph.go |
| 213 | choir.retrieval_source | corpusd | content | system:corpusd | 36461 | platform/publication_graph.go |
| 214 | choir.retrieval_span | corpusd | content | system:corpusd | 36461 | platform/publication_graph.go |
| 215 | choir.retrieval_manifest | corpusd | content | system:corpusd | 36461 | platform/publication_graph.go |
| 216 | choir.publication_source_entity | corpusd | key | system:corpusd | 219440 | platform/publication_graph.go |
| 217 | choir.publication_transclusion | corpusd | content | system:corpusd | 509212 | platform/publication_graph.go |
| 218 | choir.publication_policy | corpusd | content | system:corpusd | 23 | platform/publication_graph.go |
| 219 | choir.verifier_attestation | corpusd | key | system:corpusd | 36461 | platform/publication_graph.go |
| 221 | choir.publication_resolved_blocks | corpusd | key | system:corpusd | 36457 | platform/publication_graph.go |
| 222 | choir.publication_check | corpusd | key | system:corpusd | 506987 | platform/publication_checks.go platform/retrieval_schema.go |
| 223 | choir.support_judgement | corpusd | key | system:corpusd | 305454 | platform/publication_checks.go |
| 230 | choir.entity | corpusd | key | system:corpusd | 6996 | platform/publication_entity_learning.go platform/retrieval_schema.go platform/publication_story_entities.go platform/publication_entity_candidates.go |
| 234 | choir.entity_review | corpusd | key | system:corpusd | 12815 | platform/publication_entity_learning.go |
| 235 | choir.entity_stopword | corpusd | key | system:corpusd | 1168 | platform/publication_entity_learning.go platform/publication_entity_candidates.go |
| 240 | choir.podcast_episode | corpusd | key | system:corpusd | 614 | platform/podcast.go |
| 241 | choir.podcast_transcript | corpusd | key | system:corpusd | 531 | platform/podcast.go |
| 242 | choir.podcast_feed | corpusd | key | system:corpusd | 61 | platform/podcast_feeds.go |
| 243 | choir.podcast_index | corpusd | key | system:corpusd | 246 | platform/podcast_feeds.go |
| 244 | choir.podcast_clip | corpusd | key | system:corpusd | 496 | platform/podcast.go platform/podcast_read.go |
| 248 | choir.take | corpusd | key | system:corpusd | 2 | calibration/autoradio_take.go calibration/autoradio.go agentcore/api_autoradio_programme_place.go platform/takes.go runsheet/runsheet.go |
| 249 | choir.take_audio | corpusd | key | system:corpusd | 1 | platform/takes.go |
| 250 | choir.take_outcome | corpusd | key | system:corpusd | 0 | platform/takes.go |
| 254 | choir.software_release | corpusd | key | system:corpusd | 4 | platform/package_store.go |
| 259 | choir.package_release | corpusd | key | system:corpusd | 36 | pkgrelease/listing.go pkgrelease/record.go platform/packages.go platform/retrieval_schema.go |
| 260 | choir.release_listing | corpusd | key | system:corpusd | 43 | pkgrelease/listing.go platform/packages.go platform/retrieval_schema.go |
| 263 | choir.package_force_request | corpusd | key | system:corpusd | 0 | platform/packages_platform.go |


### 1.4 Which of those kinds carry semantic facts about the world

Of the 109 kinds, these are the ones a designer needs. Everything else is
runtime state (runs, leases, desktop), publication plumbing, the library or
the phone.

| Kind | Meaning | Id form | Written by, when | Read by |
|---|---|---|---|---|
| `choir.web_capture` (`registry.go:78`) | one fetched or polled page: URL, title, language, source id, published and fetched times, body and block blob ids | `obj:choir.web_capture:<owner>:key-<hash>`; content hash for the body (`objectgraph/web_capture.go:11-45`) | the capture route, a source poll, a research `choir.Read` (all end in `indexCaptures`, `platform/capture_index.go:189`) | corpus search, `ReadCapture`, ranking, ready list, vectors, the event layer (as `CapturedItem`) |
| `choir.publication`, `choir.publication_version` (`registry.go:197-198`) | a story and each published version | publication `pub-<32 hex>` public id, canonical id from its hash (`platform/publication_versions.go:279-286`); version `pub-...-v<n>` | corpusd on a Texture publish (`platform/publication_graph.go`) | front list, story page, related walk, ranking, vectors |
| `choir.publication_source_entity`, `choir.publication_transclusion`, `choir.retrieval_span` and 13 more plumbing kinds | what a version cites and how it is displayed; **not** `choir.entity` | `src_<16 hex>` (a "source entity" is a cited source, not a world entity) | publish | the story page's citation cards |
| `choir.entity` (`registry.go:230`) | a person, organization, place, product, work or law a published story is about | `ent:<24 hex of the normalised name hash>` or `wd:Q<n>` as the entity key; canonical `obj:choir.entity:<owner>:key-...` (`platform/publication_story_entities.go:66-70`) | corpusd, as the export of the research desk's `story.entities` tape record, when the story publishes (`agentcore/story_entities_source.go:178-257`, `platform/publication_story_entities.go:93-150`) | the entity page, related walk, candidates and lexicon, front `entities` |
| `choir.entity_review`, `choir.entity_stopword` (`registry.go:234-235`) | a review's counts and what it taught; a phrase marked not-an-entity | key | corpusd from the same export (`platform/publication_entity_learning.go`) | the lexicon, rebuilt at each corpusd start (`publication_entity_candidates.go:1-12`) |
| `choir.world_event`, `choir.event_membership` (`registry.go:131-132`) | an event and one capture's membership of it, with p, outlet, counted origin | event `ev-<20 hex>` of (computer, opening capture) (`event_layer.go:296`); membership keyed by (computer, capture) (`store/world_event.go:71`) | the event layer's host loop, as management, one flush a minute after the first queued capture (`event_layer.go:1153-1195`); on the **computer** | the events route (`agentcore/event_api.go:18`), corpus search (`researchtools/corpus_events.go:54`), the layer's own index |
| `choir.capture_label`, `choir.capture_label_override` (`registry.go:128-129`) | Jev's answer for a capture: event story, sibling or new; the story that shares its topic; language and script | key by (computer, capture, version) (`store/capture_label.go:36-60`) | the intake labeller while the computer setting `intake.labels` is on (default true, `store/computer_setting.go:187`; `agentcore/intake_labels.go:19-40`); on the computer | corpus search's `topic` field (`researchtools/corpus_search.go:249`) |
| `choir.source_entity`, `choir.source_ref` (`registry.go:76-77`) | Texture's own source cards for a document | `src_` ids | the Texture desk (`store/texture_source_graph.go:15`) | Texture |
| `choir.podcast_episode`, `_transcript`, `_clip`, `_feed`, `_index` | an episode, its timed transcript, a clip of it that sources a story | key | corpusd's podcast service (`platform/podcast.go`, `podcast_feeds.go`); the match runs in `podcast-worker` | the story page, Autoradio |
| `choir.subject` (`registry.go:196`) | a user principal (`subject_kind: user`), not a topic | `obj:choir.subject:<owner>:self` | corpusd at publish (`platform/publication_graph.go:45`) | publication graph |

Kinds registered but never written: `choir.media_item`, `choir.audio_recording`,
`choir.transcript` (reserved, `registry.go:81-83`); `choir.software_release`
(retired, 4 rows remain); `choir.take_outcome` and `choir.package_force_request`
have a writer and 0 rows. `choir.event_projection` and `choir.entity_merge`
(the design's new kinds) are not registered.

Ontology plan versus built. `docs/reports/claims-events-entities-ontology-2026-10-14.md`
D3 names `choir.capture_extraction`, `choir.entity`, `choir.world_event` with
`scale: topic`, `choir.proposition`, `choir.graph_judgment`, `choir.search_result`.
Only `choir.entity` is built in the graph; `choir.world_event` is built, but
on the computer and without `scale`; the other four do not exist. There is no
claim or proposition kind at all.

### 1.5 Edge kinds (all 86 registered; rows in the shared graph DB, **M**)

Edge rows in the graph DB total 1,731,783. Computer-side kinds show 0 because
the computer's edges are in its own `state.db`. The last column names the
non-test files that mention the kind; one file means the writer is also the
only reader.

| reg line | edge kind | owner | rows in graph DB | non-test code naming it |
|---|---|---|---|---|
| 277 | cites | - | 0 | agentcore/api_autoradio_programme_lines.go, agentcore/api_autoradio_programme_heard.go, agentcore/api_autoradio_programme_records.go, agentcore/api_autoradio_programme.go |
| 278 | captured_from | - | 0 |  |
| 279 | derived_from | - | 0 |  |
| 280 | has_media | - | 0 |  |
| 281 | has_transcript | - | 0 | platform/podcast.go, podcastworker/candidates.go |
| 282 | contains | - | 0 |  |
| 283 | references | - | 5 | sourcefetch/readability.go, platform/service.go, platform/tape_sweep.go, platform/source_metadata.go |
| 286 | has_version | - | 36461 | platform/service_publication_read.go, platform/publication_versions.go, platform/publication_graph.go, platform/publication_related.go |
| 287 | supersedes | - | 25557 | platform/service_publication_read.go, platform/publication_graph.go, cmd/doccheck/main.go |
| 288 | derived_from_proposal | - | 36461 | platform/publication_graph.go |
| 289 | routes_to | - | 10904 | platform/service_publication_read.go, platform/publication_graph.go |
| 290 | has_manifest | - | 36461 | platform/service_publication_read.go, platform/publication_version_read.go, platform/publication_front.go, platform/publication_graph.go |
| 291 | contains_blob | - | 36461 | platform/service_publication_read.go, platform/publication_version_read.go, platform/publication_graph.go |
| 292 | owns | - | 10904 | platform/publication_graph.go |
| 293 | has_agent | - | 22 | platform/publication_graph.go |
| 296 | document_revision | - | 0 | agentcore/texture_lifecycle_api.go, store/graph_store.go |
| 297 | revision_parent | - | 0 | store/graph_store.go |
| 298 | run_agent | - | 0 | store/graph_store.go |
| 299 | run_trajectory | - | 0 | store/graph_store.go |
| 300 | run_parent | - | 0 | store/graph_store.go |
| 301 | event_run | - | 0 | store/graph_store.go |
| 302 | message_from_run | - | 0 | store/graph_store.go |
| 303 | message_to_run | - | 0 | store/graph_store.go |
| 304 | work_item_trajectory | - | 0 | store/graph_store.go |
| 305 | work_item_assigned_agent | - | 0 | store/graph_store.go |
| 306 | acceptance_run | - | 0 | store/graph_store.go |
| 307 | acceptance_trajectory | - | 0 | store/graph_store.go |
| 308 | continuation_from_run | - | 0 | store/graph_store.go |
| 309 | continuation_to_run | - | 0 | store/graph_store.go |
| 310 | decision_document | - | 0 | store/graph_store.go |
| 311 | decision_run | - | 0 | store/graph_store.go |
| 312 | evidence_agent | - | 0 | store/graph_store.go |
| 313 | subscription_content | - | 0 | store/graph_store.go |
| 314 | browser_session_run | - | 0 | store/graph_store.go |
| 315 | package_source_computer | - | 0 |  |
| 316 | adoption_package | - | 0 |  |
| 317 | adoption_target_computer | - | 0 |  |
| 318 | session_desktop | - | 0 | store/graph_store.go |
| 319 | app_instance_desktop | - | 0 | store/graph_store.go |
| 322 | was_derived_from | - | 36461 | platform/publication_graph.go |
| 323 | was_generated_by | - | 0 |  |
| 324 | was_associated_with | - | 36461 | platform/publication_graph.go |
| 325 | generated | - | 36461 | keyescrow/sealed.go, platform/publication_graph.go |
| 326 | attested | - | 36461 | platform/publication_graph.go |
| 327 | attests_to | - | 36461 | platform/service_publication_read.go, platform/publication_graph.go |
| 328 | granted_consent | - | 36461 | platform/publication_graph.go |
| 329 | consent_for | - | 36461 | platform/service_publication_read.go, platform/publication_graph.go |
| 330 | authored_review | - | 36461 | platform/publication_graph.go |
| 331 | reviews | - | 36461 | platform/service_publication_read.go, platform/publication_graph.go, cmd/choir/package.go |
| 332 | contains_span | - | 72922 | platform/service_publication_read.go, platform/publication_graph.go |
| 333 | has_retrieval_manifest | - | 36461 | platform/service_publication_read.go, platform/publication_graph.go |
| 334 | has_retrieval_source | - | 36461 | platform/service_publication_read.go, platform/publication_graph.go |
| 335 | references_entity | - | 219440 | platform/service_publication_read.go, platform/publication_graph.go |
| 336 | transcludes | - | 509219 | platform/service_publication_read.go, platform/publication_graph.go |
| 337 | transcludes_from | - | 0 |  |
| 338 | has_policy | - | 36461 | platform/service_publication_read.go, platform/publication_graph.go |
| 345 | has_resolved_blocks | Corpusd | 36457 | platform/service_publication_read.go, platform/publication_graph.go |
| 346 | is_version_of | Corpusd | 36461 | platform/service.go, platform/service_publication_read.go |
| 350 | cites_capture | Corpusd | 202741 | platform/publication_related_edges.go |
| 355 | related_story | Corpusd | 4 | platform/publication_related_story.go |
| 359 | relies_on | Corpusd | 1 | objectgraph/reliance.go |
| 363 | mentions_entity | Corpusd | 23966 | platform/publication_story_entities.go |
| 367 | in_event | Corpusd | 0 | platform/capture_ready.go |
| 370 | co_super_assignment_trajectory | VMStore | 0 | store/engineering_assignments.go |
| 371 | co_super_assignment_parent | VMStore | 0 | store/engineering_assignments.go |
| 372 | co_super_assignment_parent_run | VMStore | 0 | store/engineering_assignments.go |
| 373 | co_super_assignment_parent_work | VMStore | 0 | store/engineering_assignments.go |
| 374 | co_super_assignment_agent | VMStore | 0 | store/engineering_assignments.go |
| 375 | co_super_assignment_work | VMStore | 0 | store/engineering_assignments.go |
| 376 | co_super_assignment_run | VMStore | 0 | store/engineering_assignments.go |
| 377 | co_super_report_assignment | VMStore | 0 | store/engineering_assignments.go |
| 378 | co_super_report_candidate | VMStore | 0 | store/engineering_assignments.go |
| 379 | commitment_record_packet | VMStore | 0 | store/lifecycle_commit_act.go |
| 382 | document_alias | - | 0 | store/graph_store.go |
| 383 | document_mutation | - | 0 | store/graph_store.go |
| 384 | document_checkpoint | - | 0 | store/graph_store.go |
| 385 | coagent_mailbox | - | 0 | store/graph_store.go |
| 386 | super_slot | - | 0 | store/graph_store.go |
| 387 | computer_lineage | - | 0 | store/graph_store.go |
| 388 | media_progress | - | 0 | agentcore/replay_eligibility.go, agentcore/residue_import.go, store/graph_store.go, store/snapshot_to.go |
| 389 | media_recent | - | 0 | store/graph_store.go |
| 390 | user_preference | - | 0 | store/graph_store.go |
| 391 | desktop_state | - | 0 | agentcore/replay_eligibility.go |
| 392 | desktop_workspace | - | 0 |  |
| 393 | window_placement | - | 0 |  |
| 394 | worker_update | - | 0 |  |

### 1.6 The tape and how a record becomes a graph row

The tape (each computer's append-only event chain) is the authority for what a
computer decided; the graph rows in corpusd are a one-way, re-runnable export
(`types/task.go:266-284`, `docs/design/entity-layer-2026-11-20.md` section 3.2).
Tape record kinds that carry a semantic fact:

| Tape kind | What it holds | Storage class | Export to corpusd |
|---|---|---|---|
| `story.entities` (`types/task.go:284`; phases `recorded`, `analysis_unanswered`, `platform_failed`) | a story's entity attachments: entity id or new name and type, role, the p read from the System 1 answer, whether the desk overrode it and why | object graph (`types/event_storage.go:72`) | when the story **publishes**: `publishStoryVersion` then `exportStoryEntities` read the channel's tape and POST `/internal/platform/story-entities` (`agentcore/story_entities_source.go:178-257`); validate-only first, then the write (`agentcore/story_publish.go:546-556`); corpusd accepts a platform-tier computer only (`platform/publication_story_entities.go:187-195`) |
| `story.relation` (`types/task.go:274`) | a judged pair of stories and the relation (`same_incident`, `later_development`, `same_subject`) | object graph | `exportStoryRelations` (`agentcore/story_relation_verb.go:283`) to `/internal/platform/story-relations`; 4 rows in production |
| `system1.judgment` (`types/task.go:247`) | every System 1 answer: the receipt the two records above cite | object graph | none; stays on the computer |
| `search.served` (`types/search_served.go:14`) | which provider served a web search | object graph | none (search leads are written by a separate host call, `agentcore/search_leads.go`) |
| `texture.search.queried` (`types/task.go:362`) | the owner's Texture search | object graph | none |
| the event layer's records | none: `choir.world_event` and `choir.event_membership` are **objects**, written by the host loop through `PutWorldEvent` and `PutEventMembership` (`agentcore/event_layer.go:733,776`) with no tape event naming the judgment (the Jev receipt is a field of the membership: model, cost; the question and the shortlist are **not** recorded, `event-join-rate-hand-read-2026-11-20.md` section 2) | the computer's graph | none today; E5 designs a projection |
| the capture arrival and disposition records | what a poll captured and what management did with it | object graph | none |

Who exports what, and when:

| Export | Trigger | Path | Idempotent by |
|---|---|---|---|
| captures | every capture write (route, poll, research read) | corpusd writes the object itself; `indexCaptures` then writes `proj_capture_index`, `proj_capture_terms`, the FTS file and wakes the vector worker (`platform/capture_index.go:185-189`) | content-hash or key identity |
| story versions and their citations | a Texture publish | `platform/publication_graph.go`; one `cites_capture` edge from the version to each capture its anchors cite (`publication_related_edges.go:17-60`, backfill `publication_related_backfill.go:156-197`) | edge id from fixed metadata |
| story entities | publish | as above | deterministic edge id (`mentionMetadata` is exactly `{"role":...}`, `publication_story_entities.go:73`); a role change tombstones the old edge |
| event membership | a flush, one minute after the first queued capture (`eventFlushInterval`, `event_layer.go:39`) | none to corpusd | key by (computer, capture) |
| vectors | a capture write kicks the embed worker (landed 2026-11-20, station E1); the worker derives its own gap; stories: every version, every passage | corpusd only (`platform/vector_worker.go`, `vector_store.go:1-30`) | `text_sha` and model hash |

Restart behaviour (CLAUDE.md "Restarts"): the event layer's queue and index are
memory only; a crash loses queued captures, which stay without a membership,
and nothing resolves them at boot (`event_layer.go:32-35`). The index is rebuilt
from the store on first use, capped at 10,000 events and 20,000 memberships
(`event_layer.go:85-86`), so at about 10,153 events the index has already begun
to drop the oldest. The lexicon is rebuilt from `choir.entity` rows at corpusd's
start.

---

## 2. Every place a semantic fact is held

Legend for "home": **graph** is a registered kind or edge in the shared graph
DB; **computer graph** is a registered kind on the computer's `state.db`;
**side table** is a derived table beside the graph; **file** is a blob or file;
**tape** is a tape record only; **memory** is process state.

| # | Fact | Home | Shape and writer | Identity shared with other facts? | Provenance |
|---|---|---|---|---|---|
| 1 | Capture (the page) | graph `choir.web_capture`; text and block blobs in the content store; per-computer FTS5 file (`internal/capturefts`, `platform/capture_fulltext.go`); `proj_capture_index` (host, source id, language, titles, times) and `proj_capture_terms` (3,035,571 term rows) | writer: capture route, source poll, research Read. Capture id `obj:choir.web_capture:<owner>:key-<hash>`; the same id is the key of `vec_captures`, `proj_*`, `search_leads.capture_id` | yes: one id space across five tables (the id is copied into each) | url, fetched and published times, source id, extractor, doc hash, body blob |
| 2 | Capture text and passages | blobs; FTS5 passages of about 300 tokens (`capture_fulltext.go`); vector lead passage = first span (`vector_worker.go:196`) | corpusd, after each capture write | by capture id | none beyond the capture |
| 3 | Capture vector | side table `vec_captures`: one halfvec(256) of title plus lead passage, 237,073 rows, 0 refused (**M**) | corpusd worker | capture id | `text_sha`, `model` hash, `embedded_on` |
| 4 | Story and versions | graph `choir.publication`, `choir.publication_version` (+16 plumbing kinds); versions per story avg 3.34, max 132 (**M**) | corpusd, at publish | `pub-<32 hex>` (public) and canonical id (graph) | publication check, support judgement, review and consent records, attestation per version |
| 5 | Story passages and vectors | side table `vec_story_passages`: 65,113 rows over 36,092 versions of 10,747 stories (**M**); every version, not the latest only | corpusd worker | `story_id` and `version_id` are **canonical** ids (`obj:choir.publication...`), not the public `pub-` id | `text_sha`, model |
| 6 | Citation (story to capture) | graph edge `cites_capture` version to capture, `{"relation":"cites"}` (202,741) | corpusd at publish; backfilled | canonical ids on both ends | the anchor's `GraphTransclusion` (`choir.publication_transclusion`, 509,219) |
| 7 | Cited source card | graph `choir.publication_source_entity` (219,440) with `references_entity` edges | corpusd at publish | a separate `src_<hex>` space | `created_by: texture_write` |
| 8 | Outlet and source | not an object. Strings: `outlet.Of(url)` (computed in the event layer and the publish gate, `agentcore/event_layer.go:774`, `story_publish.go:313`), `registrableDomain` (ready list, `platform/capture_ready.go:292`), `host` and `source_id` in `proj_capture_index`, `choir.source_schedule` (the poll definition, computer graph) | computed at use | **no**: two outlet functions that disagree (aggregators dropped and `bbc.co.uk` folded in one, neither in the other) | none |
| 9 | World event | computer graph `choir.world_event` | the event layer, management custodian, on the paper's `state.db` | `ev-<20 hex>`, a computer-scoped id; no owner part, no graph id | headline, opening capture, `follows`; the Jev question and shortlist are not kept |
| 10 | Capture to event membership | computer graph `choir.event_membership` (one per capture) | same | key (computer, capture id) | p, model, cost, outlet, wire origin, tentative flag |
| 11 | Capture label (event and topic answers) | computer graph `choir.capture_label` | intake labeller | by capture id | model, tokens, the candidates shown |
| 12 | Story entities | graph `choir.entity` + `mentions_entity` role edges + tape `story.entities` | see 1.4, 1.6 | entity key `ent:`/`wd:`; canonical id owner-scoped | the tape record (p, override reason); `entity_review` rows |
| 13 | `proj_capture_resolution` | side table, 0 rows | tests only today | capture id | `record_id` of the tape record |
| 14 | Related stories | (a) edges `related_story` (4) and `relies_on` (1); (b) a **computed walk** over `cites_capture`, central entity edges and a title-word floor (`platform/publication_related.go:593`) with cache in process memory; the answer's `basis.mode` on production is `captures+relations+entities+floor`, `events` 0 (**M**) | walk is read-time; edges by `RelateStories` | canonical publication ids | the walk is not stored; `story.relation` has p and reason |
| 15 | Search leads | side table `search_leads` (43,766 rows, 3,173 with a capture id, 4,175 tried) | host, one write per search | URL key; capture id when read | query keys, conversation ids, times |
| 16 | Lexicon | process memory in corpusd, rebuilt from `choir.entity` rows (and `entity_stopword`) at start; "9,120 lexicon keys" in the growth read | corpusd | entity key | rows |
| 17 | Vocabulary and role guards | `types.EntityRoles`, `EntityTypes` (`types/story_entity.go:35-60`) and `maxStoryEntitiesPerRequest = 12`; `vocab_migrate_og.go` is **not** one: it is the 2026 desk-name migration (V1 to V2) | code constants | | |
| 18 | Podcast clip entity gate | `podcastmatch` extracts capitalised "names" and rare "topic" tokens from the story's title and dek (`podcastmatch/auto.go:406-462`); it never reads `choir.entity`. A clip points at its story by `story_route` and version, a string in the clip body (`platform/podcast.go:138-150`), with no edge | `podcast-worker` | **no**: a route string | rule outcome |
| 19 | Notebook | a Texture document `My notebook`: markdown lines `[L7 · heard 9 of 11]` (`docs/reference/AUTORADIO-NOTEBOOK.md`); per-listener, free text | Texture, the listener | none | in-line tags |
| 20 | "Topic" or "subject" | (a) the label's topic answer (a story id, 11); (b) front **sections**: P(yes) per question in a JSON scores file a host timer writes, applied with thresholds at load (`platform/publication_front_importance.go:1-60`, `internal/sectionlist`); (c) the phone's topic-tree lane (UI); (d) `choir.subject` which is a user; (e) the ontology's `world_event scale: topic`, not built | mixed | no shared id | |
| 21 | Importance and front ranking | the same JSON scores file: `importance` 0-10, `promo`, section P; read by corpusd, written by `cmd/front-importance` | a host timer every ten minutes | keyed by publication id and revision | none stored per judgment |
| 22 | Claims, propositions, contradictions | **nothing** is built (`choir.commitment_record` is a desk precommitment ledger, not world claims) | | | |

Duplications and inconsistencies, concretely:

1. **The same concept held twice.** "This capture is about that event" is a
   capture label's `event` answer (story, sibling, new), an event
   membership (a Jev answer with p), the vector neighbours the event layer asks
   corpusd for (`event_layer.go:858`), and the `via_stories` reason of a ranked
   capture. Four mechanisms, two stores, no shared record.
2. **"Same incident / related story" held three ways.** `related_story` edges
   (4 rows), event ids (not joined to stories), and the read-time walk. The
   public related answer is entirely the walk plus the floor (**M**: three of
   three links were `reason: entity`, one shared entity each).
3. **Ids that cannot be joined in one query.** Events (`ev-...`, on the
   computer) to anything in `graph`; a story's public id to a vector row's
   canonical story id (derivable with `publicationCanonicalFor`, not
   joinable in SQL); a podcast clip's route string to a publication; an
   outlet string to a source schedule's `source_id`; a Texture `doc_id` to a
   publication (`publicationIdentityFor`); the Texture frame handle `r1.c1` to a
   capture id.
4. **Facts with no provenance.** An event's shortlist and the Jev question are
   not stored; a related-walk link is not stored; an outlet has no row; a
   front section is a P in a file with no record of the judgment; the lexicon
   is memory.
5. **Kinds registered but unused.** `in_event` (registered at `registry.go:367`,
   read at `capture_ready.go:256`, never written), `transcludes_from`,
   `was_generated_by`, `has_media`, `captured_from`, `derived_from`, `contains`
   (registered, 0 rows, no non-test writer), `choir.take_outcome`,
   `choir.package_force_request`, the three Autoradio reserved kinds.
6. **Edges written but not read.** The eight listed on the one-page answer, and
   `relies_on` ("data only", `registry.go:357-359`). `cites_capture` is read only
   by the related walk, ranking (`platform/capture_rank.go:629`) and the
   story page (`service_publication_read.go:423`).
7. **A switch in the way.** Story entities are attached only while the computer
   setting `research.related_story` is on; it defaults false
   (`store/computer_setting.go:190`) and the paper has it on (inferred from the
   growth read, **U** whether read from the setting). Three environment kill
   switches (`CORPUSD_RELATED`, `CORPUSD_RELATED_STORY`, `CORPUSD_RELATED_ENTITIES`)
   gate the related walk's signals (`platform/publication_related.go:60-64`,
   `publication_related_entities.go:25-32`). The ranked-search design
   deletes all four; none is deleted.
8. **Quality of the identity.** 6,991 of 6,995 entity keys are minted from a
   name; four are Wikidata (`Q49417`, `Q179200`, `Q754`, `Q817`); 1,587 rows carry
   aliases; 1,168 stopwords; 4,889 entities (70%) are attached to one story, 721
   to five or more; the most central entity has 417 central edges (**M**). The
   entity types: organization 2,430, person 1,891, place 1,535, product 609,
   work 428, law 102.
9. **Capture-side entity and event links are read before they are written.**
   `captureLinkEdges` reads `mentions_entity` and `in_event` from captures
   (`capture_ready.go:249-290`), and the candidate step uses the neighbours'
   entities (`capture_ready.go:484`), but no capture has either edge: all
   23,966 `mentions_entity` edges start at a publication (**M**).

---

## 3. The vector side

| What | Embedded how | Where | Rows (**M**) | Notes |
|---|---|---|---|---|
| a capture | one vector of `DocumentInput(title, lead passage)`; the lead is the first span of the capture's text (`vector_worker.go:193-211`); a capture with no text uses its title | `vec_captures`, HNSW, halfvec(256) | 237,073 (all public captures; 0 refused) | model EmbeddingGemma 2 (GGUF, llama.cpp b12259), `docs/reference/EMBEDDINGGEMMA.md`; the lead is **one** passage, so a long page's body beyond its first span has no vector |
| a story | every passage of every version (1,800 runes, at most 2,400, `vector_worker.go:249-300`) | `vec_story_passages`, HNSW | 65,113 over 36,092 versions | search keeps only passages of the **latest** version (`capture_rank.go:14-20`) |
| a query | embedded once per ranked search on a second embedder unit on port 8091 (landed 2026-11-19, `current-architecture.md` row "Vector index") | memory | | 4 s deadline (`capture_rank.go:79`); past it the answer is BM25's and says so |
| a passage of a published event | **not embedded**: events have no vector; the event layer asks for the nearest captures of each queued capture and maps them to events (`event_layer.go:858`, neighbour route `platform/vector_neighbours.go`) | | | an event's "vector" is the set of its members' captures |
| a search lead | not embedded (`search_leads.go`, the one-verb definition) | | | matched by words only |
| an entity | not embedded; no name vectors (entity design "deliberately not built") | | | |

The ranked answer (`platform/capture_rank.go:1-110, 352`): one query, one
answer. (1) The query is embedded; the nearest story passages and capture
leads are merged by cosine, a capture before a story on a tie. A story counts
once, by its best passage, and only when the passage is of its latest version;
it is replaced by the captures that version cites through `cites_capture`
(at most 8 a story, `rankStoryCitedMax`, `capture_rank.go:57`). (2) BM25's top
three pages are guaranteed a place. (3) A capture reached more than one way is
listed once with every reason (`text`, `meaning`, `story`). Every capture passes
the owner, computer, private, tombstone, superseded and window checks.
Reciprocal-rank fusion measured 0.856 recall at ten and is not used; the built
rule measured 0.940 on 2026-11-19T20:16Z (E, `current-architecture.md`).

Vectors and graph in one query: **yes, once**, inside corpusd: the story half of
the ranking walks `cites_capture` from vector-found stories to captures in the
same request (`capture_rank.go:629`). Nowhere else do they meet: entities are
not an input to ranking; events are not an input (the event layer calls the
neighbours route and then maps captures to events in the computer's memory);
the related walk uses graph edges only; the search leads use words only. There
is no query that says "captures near this text that are in this event" or
"captures near this text whose entity is X": `in_event` and capture
`mentions_entity` do not exist as data.

---

## 4. What an agent can ask today

### 4.1 Verbs that read world knowledge

Every verb is a method of the `choir` package a desk's cell calls
(`yaegikernel/choir.go:379-490`; signatures and prose in
`yaegikernel/surface.go:57-128`). The desk's verb set is computed from its
manifest grants (`deskmanifest/grants.go:91-151`). There is no
`docs/reference` page for the module surface; the surface is the Go table and
the `Help` card (`yaegikernel/surface_cards.go:20-45` shows each desk's hot
verbs).

| Verb | Signature | Returns | Who holds it (grant) | Scope and custodian rule | Cost |
|---|---|---|---|---|---|
| `SearchWireCorpus` | `(query string, limit int)`; since/until/order inside the query | `published` (route_path, title, snippet, kind, author, read_url, **publication_id**), `events` (event_id, headline, outlets, origins, corroborated, first_seen, last_seen, capture_ids, matched), `captures` (capture_id, title, url, outlet, source_id, language, topic, fetched_at, published_at, passage, reasons text/meaning/story, via_stories), `stories`, notes (`researchtools/researchtools.go:134-165`, `corpus_search.go:45-70`) | research and research_web (`world.read`) | this computer's own corpus: its captures, events and **every** computer's published pieces; owner-private captures never listed | one corpusd call (text half mean 189 ms, title-label half median 37 ms, 8 to 14 s for a query that matches about 1,500 captures, 4 s vector deadline; the host's brief runs it 3 at a time under an 8 s deadline); events: an in-memory substring match over the newest 3,000 events and 6,000 memberships on the computer (`corpus_events.go:21-54`), no model |
| `Read` | `(url string)` | `capture_id`, `url`, `title`, `block_count`, first blocks, `next_block`, `headings` | research, research_web (`world.read`) | fetches the web into a shared capture (a world fetch); owner-private never | an egress fetch; writes a capture, wakes the embed worker, queues an event resolution (`tool_profiles.go:390`) |
| `ReadCapture` | `(captureID string, fromBlock int, maxChars int)` | JSON: `capture_id`, `url`, `title`, `blocks[{id, kind, text, page}]`, `next_block` | every desk that holds `captures.read` (management, Texture, research, tasks, app desks) | any capture by id from any computer; an owner-private capture only to its label's custodian, the owner and the kernel (`agentcore/capture_read_gate.go:13-60`) | one corpusd read, no model |
| `SearchStart`, `Search`, `SearchWait`, `Searches`, `Captures` | web search (`choir.Search(query, fresh)`) and the session's own search and read history | `Search` with results and `Reused`/`Refused`; `Captures()` the run's own reads | research (`world.read`) | web providers; a held-back search returns unread pages instead | a gateway call to providers; a corpus-first hold exists in code for unread pages (research prompt v16); the one-verb design replaces it (landings L1 done, L2 not on main) |
| `Records` | `()` | the desk's unread inbox records (Casts, Asks, Feeds, Declines, Reports), a capture's id and title under `r.Captures.Items` | every desk | the desk's own inbox | local |
| `Stories` | `(limit int, before string)` | this computer's stories: doc_id, inbox, headline, lede, updated_at, version | management and its tasks (`intake` tier) | the computer's own Texture story index (`choir.story_index`, readable by management by `supplementalKindReaders`, `reader.go:180`) | local; no cap |
| `StoryAnalysis` | `()` | the desk's own story's entity candidates (existing and extracted), each with System 1 role and kind answers and receipts; novelty | research (`story.relate`) | preloaded at the first cell; names and titles only, never story text | a Jev call by the host before the cell (`story_analysis.go:206-380`); questions are asked in more than one call (the entity design counts "two calls of twelve") |
| `System1` | `(questions)` | a distribution over each question's choices | research (`system1.call`) | the judge sees only the capture ids the call names and passages the cell passes | a model call per `System1`; the verb text says calls are bounded per run and per day; that text predates the no-limits rule and I did not read the code that enforces it (**U**) |
| `SearchTexture` | `(query, limit)` | refs and snippets labelled `owner_private:texture` | the research Texture child only, under a lease the Texture custodian issues | the owner's own stories | guarded by the custodian |
| `ReadDoc` | `()` | the Texture desk's own document | Texture | its own | local |
| `Get`, `List` | `(kind, id)`, `(kind, limit)` | a live object of the app package's own kinds | app desks | the package's own namespace `choir.app.*` | local |
| `Feed`, `ReadEvidence`, `ListEvidence`, `RunMemoryEntry`, `PollSource`, `SourceAPI` | feed captures to a story; research's own evidence; poll a source | | various | | |

**Write verbs that carry semantic facts** (not reads, but the only way a fact
enters): `Relate(story, entities)` (the story's central and incidental
entities, at most 12 a call, each citing a System 1 answer; the probability is
read from that answer, never supplied) and `RelateStories(pairs)` (at most 12
pairs), both under `story.relate` (`deskmanifest/grants.go:147`) and both
refused when the computer setting is off (`story_entities_verb.go:131`).

What the verbs do not return. **No verb takes or returns an entity id as a
query**: `StoryAnalysis` shows only one's own story's candidates; no verb lists
what is known about an entity. **No verb reads an event by id**; events arrive
only as `SearchWireCorpus` hits. **There is no `Related` verb** (the related
walk is an HTTP route behind the proxy: `/api/platform/publications/related`,
`proxy/handlers.go:978`); a cell reaches the world only through the host-served actions in
`deskmanifest.verbDispatch` (`manifest.go:289-320`), and none of them is a read of the
platform's entity, related or front routes. Management's
`ProductAPI` verb calls the product API but is held by management, not research
(`deskmanifest.verbDispatch`, `manifest.go:289-320`; I did not enumerate its
allowed routes, **U**). The entity read, the front list and the related answer
are public HTTP: `GET /api/platform/entities/{id}` (aliases, counts, newest
public stories with role, co-entities), `GET /api/platform/entities/growth`,
`GET /api/platform/publications/front`, `GET /api/platform/publications/related?route=`
(**M**, each answered 200 on 2026-11-20).

### 4.2 Questions the data could answer and an agent cannot ask

| Question | What exists | What is missing |
|---|---|---|
| Which published stories report this event? | `cites_capture` (story to capture) and `choir.event_membership` (capture to event) both exist, in two stores | the link: no `in_event`, no `reports_event`, no event in corpusd; **no verb** to ask it. Station E5 (lane `story-events-e5`, uncommitted) adds the edges and a consolidation read |
| What do we hold about this entity since T? | `mentions_entity` from stories to entities, entity rows with aliases; the capture text is searchable | no capture-to-entity edge (E3); no verb that takes an entity; the entity page is HTTP and public only, shows stories (not captures), cuts at 50 stories and 8 s; `since` does not exist on it |
| Which captures of this event are newer than the story's last version? | event members (computer), story version times (graph), `cites_capture` | no join of the two stores; no verb; the events route has no `since` and caps at 300 (`event_api.go:26-30`) |
| Who are the central entities of the stories citing this outlet? | stories to captures (`cites_capture`), captures to hosts (`proj_capture_index.host`), stories to central entities | an outlet is not an object and the two outlet functions differ; no verb takes an outlet; a three-hop join no route computes |
| Which stories contradict each other on X? | `related_story` relation set is `same_incident`, `later_development`, `same_subject` only; `choir.support_judgement` and `publication_check` judge a story against its sources | no claim, proposition or contradiction kind; no relation `contradicts`; `RelateStories` holds 4 pairs |
| Which entities does this capture mention? | nothing for captures | station E3 |
| What are the events this entity is in? | nothing | entities and events share no edge |
| What happened after this event (its follow-ups)? | `WorldEvent.Follows` (a later development links to an earlier event, `store/world_event.go:34-44`) | not exposed by `SearchWireCorpus` (`CorpusEvent` has no `follows`); no verb |
| Which stories did we publish about this event's entity in the last day, by outlet? | the pieces separately | the joins above |
| Is this entity the same as that one (aliases, "U.S." and "United States")? | alias lists on 1,587 entity rows; `candidate` lookups (`publication_entity_candidates.go`) | no merge; the lexicon refuses a name another entity holds; the design's `choir.entity_merge` is unbuilt |
| How many distinct outlets corroborate this event? | `Origins` and `Corroborated` computed from members (`event_layer.go:158-195`) | only in the computer's events route and the substring search; not stored as a fact |

---

## 5. Desk and custody rules a re-architecture must respect

Doctrine (`docs/choir-doctrine.md` I26 and I27, lines 406-470; CLAUDE.md
"Desks"):

- each data kind has one custodian desk with sole read and write; desks reach
  each other only through precommitment records; a desk's grant over a kind it
  does not hold flows through the custodian's lease; research has read-only
  world authority; management supplies coherence and holds no document or event
  mutation authority; engineering mutates only in capsules;
- one object graph per computer, one home per kind; "a copy of a kind's data
  into a second kind, table, file convention or browser store is a second home";
- the shared Autopaper graph lives in corpusd's `graph` database and is outside
  a computer's restore set (`computer-ontology.md` Store Taxonomy);
- labels (`owner_private:<kind>`, `world_untrusted`, `platform`, `publisher:<id>`)
  ride every record and capture-derived text; the disclosure gate admits an
  owner-private label only to its custodian or a lease holder and sinks
  `publication`, `mail_send`, `egress`, `desk_message`, `library_release`
  (`types/labels.go:1-60`, `disclosure/rules.go:20-60`);
- private computers' data never crosses owners: every corpusd table carries
  owner and computer, every object id carries the owner, the entity and related
  routes accept a platform-tier computer only
  (`publication_story_entities.go:187-195`), a person entity is served publicly
  only with a Wikidata id or as central in two public stories.

How today's stores comply:

| Store | Custodian by manifest | Verdict |
|---|---|---|
| `choir.entity`, `entity_review`, `entity_stopword`, `mentions_entity` | corpusd kernel service (`kernelservice.go:84-96`); the judgment is the research desk's tape record (`story.relate` grant) | complies in form: one writer, tape is authority, export one-way. Departures: the gate is a switch (CLAUDE.md "No switches"); the judgment record rides the `system1:<computer>` channel as `story.entities`, not as a precommitment record; no private-owner path (entities are the paper's only) |
| `choir.world_event`, `choir.event_membership`, `choir.capture_label` | desk management (`manifest.go:362`); research and Texture admitted as readers (`reader.go:180-190`) | complies as a kind. **Tension to settle**: doctrine I26 says management "holds no document/event mutation authority" (meaning the runtime event, I read) while the manifest gives it custody of `choir.world_event`; the writes come from a host loop that declares management, not from a desk run (`registry.go:122-127`). The events are also not in the shared graph, so no other owner or the public can read them |
| `choir.web_capture` | corpusd | complies; shared by many owners' computers through owner-scoped ids; a private computer's capture carries an `owner_private` label and is served only to its custodian |
| `vec_*`, `proj_*`, `search_leads`, FTS5 | corpusd, derived and rebuildable | **side tables, not kinds**; no custodian entry. They are declared derived ("a lost table costs a re-embed", `vector_store.go:15-27`; "derived and rebuildable", `capture_resolution.go:12-14`; the design says `choir.story_index` is the precedent for a declared projection, `reader.go:175-190`). Whether I27 counts a derived table as a second home is the owner's call |
| `choir.podcast_*` | corpusd; its own registry comment says "residual R1: a desk should be" the writer (`registry.go:236-239`) | does not comply with one custodian desk |
| front importance and sections | a JSON file a host timer writes, corpusd reads | not a kind; no custodian; no tape record |
| `choir.related_story`, `relies_on` | corpusd; `relies_on` has no reader | one-way exports; `relies_on` is data no desk sees |
| the notebook | the Texture desk (a document) | complies |
| the lexicon (memory) | corpusd | derived |

The red surfaces any unification touches: Texture canonical writes; tape and
evidence (a new record kind such as the design's `capture.entities`);
checkpoint and route projection; gateway and provider calls (the event layer's
one Jev call per flush, `agentcore/jev_job.go`; CLAUDE.md "no limits on our own
model calls", "as many questions as possible in the same call"); the corpusd
export routes behind `requireInternalOrBoundGuestWrite`.

---

## 6. Production facts (read-only, 2026-11-20 22:22Z to 22:30Z unless said)

### 6.1 Counts in the shared graph DB (`graph`, **M**)

| Measure | Value |
|---|---|
| objects / edges | 2,333,135 / 1,731,783 (grew while I read) |
| plumbing kinds (16) | 2,014,695 objects, 86.4% |
| world kinds (captures, entities, publications, routes, podcast) | 318,278 objects, 13.6% |
| object kinds in use | 38 of 109 registered; edge kinds in use 30 of 86 |
| `choir.web_capture` | 237,050; 35 owners; the largest owner holds 233,339 |
| `choir.publication` / `publication_version` | 10,903 (10,742 of one owner, the paper) / 36,458 |
| `choir.entity` | 6,995; 4 with a Wikidata id (0.06%); newest minted 22:26Z |
| `mentions_entity` | 23,966 edges (23,833 live): central 13,764, incidental 10,069; all from publications; 5,775 stories have any; avg 4.13 a story (max 32), 2.38 central; 5,597 stories have a central entity |
| entity degree | of 6,995 entities with edges: 4,889 on one story, 1,385 on two to four, 721 on five or more; top central degrees 417, 247, 213, 207, 187 |
| `cites_capture` | 202,741 edges, 33,313 versions, 51,501 distinct captures (21.7% of 237,050) |
| `related_story` / `relies_on` | 4 (two `same_incident`, two `later_development`) / 1; 3 stories carry a related edge |
| `choir.entity_review` / `entity_stopword` | 12,814 / 1,168 |
| podcast: episodes / transcripts / clips / feeds | 614 / 531 / 496 / 61 |
| `proj_capture_index` / `proj_capture_terms` / `proj_capture_resolution` | 237,050 / 3,035,571 / 0 |
| `vec_captures` / `vec_story_passages` | 237,073 / 65,113 (36,092 versions, 10,747 stories) |
| `search_leads` | 43,766 (3,173 with a capture; 4,175 tried; oldest 2026-11-20) |
| `og_lookup_keys` | 572,271 |
| `in_event` edges, `reports_event` edges | 0, kind not registered |

### 6.2 Per day (UTC; the graph's `created_at`, which starts on 2026-11-12 in this database, **M**)

| Day | New publications | New versions | New captures |
|---|---|---|---|
| 2026-11-13 | 456 | 1,733 | 9,762 |
| 11-14 | 350 | 1,546 | 7,337 |
| 11-15 | 349 | 1,449 | 8,401 |
| 11-16 | 514 | 1,910 | 11,558 |
| 11-17 | 535 | 1,981 | 11,834 |
| 11-18 | 561 | 1,968 | 12,458 |
| 11-19 | 288 | 849 | 9,763 |
| 11-20 (to 22:25Z) | 445 | 971 | 10,837 |

Entities minted a day: about 300 to 450 since 2026-11-09, 252 on 2026-11-19 (E,
entity design section 1).

### 6.3 Events (the computer's store; not readable from the graph DB)

I could not read the events: the route answered 401 without the paper's key and
the store is the paper computer's `state.db`. What the records say:

- total: 10,153 events on 2026-11-20 05:26Z (E, join-rate record section 2);
- members per event: 282 of the 300 newest events (19:40Z to 20:18Z) have one
  member and 18 have two to six; 116 have an intake member and 198 a
  `web:read` member; 3% have an intake capture newer than the first research
  read of the same event (E, `research-runs-with-nothing-new-2026-11-20.md:201-207`);
- joins: 03:28Z to 05:12Z, 96 flushes asked 899 questions; joined 25 (2.8%),
  minted 756 (84.1%), tentative 94 (10.5%), unresolved 24 (2.7%); 704 of 899
  (78%) were about a capture with no stored vector; 656 were research reads
  (E, join-rate record section 1). The embed kick landed after that span
  (E1, 2026-11-20), so the share since is not measured by me (**U**);
- of the 50 newest minted events hand-read, 25 correctly minted, 10 should have
  joined, 15 unclear (pages that are not one happening: a home page, a bot-block
  page, titleless posts) (E, section 2a).

### 6.4 Entity quality (hand read of 200 attachments of the 300 newest front stories, E)

29.0% (58 of 200) are not entities (25 common nouns or roles, 16 demonyms, 14
fragments, 3 events); of the 142 entity rows 23.9% (34) have a second id in the
window; 2 of 142 (1.4%) are the wrong entity for the story; the deterministic
detector flags 25 of 58 non-entities (recall 43%, precision 100%). The design's
own tally of 240 front attachments had 17% not entities and 7 duplicate pairs.
I saw the same faults today in the lead story's central entities (`Markets`,
`IPO`) (**M**, public front list).

### 6.5 Search and research context (E)

840 web searches from 232 research runs in 6 h 48 min after 13:30Z on
2026-11-20; 210 of the 232 runs (90%) were the first run their story had
(`research-runs-with-nothing-new-2026-11-20.md`); the paper opens about 34
stories an hour; searches 2,764 by 20:26Z for the day against the owner's 1,000
a day. Corpus search: 87 of 87 corpus searches in the hour 23:00Z to 24:00Z on
2026-11-18 returned a capture with a page-text hit (E, `current-architecture.md`).
Recall at ten of the ranked answer 0.940 (E).

---

## 7. In-flight work, and what each assumes

**Lane `entity-e3`** (captures get entity ids in the event flush; red). At
22:2xZ the lane's tree is at `origin/main` with no commit and no edit (**M**,
`git status` and `git log origin/main..HEAD` empty), so nothing is built yet.
The design (`docs/design/entity-layer-2026-11-20.md`, station E3) assumes
entities stay in corpusd, owner-scoped, as `choir.entity` rows with
`mentions_entity` from captures (role central, incidental or the new
`tentative`), custodian the corpusd kernel service, authority the computer's
tape. The judgment is made by the event layer's host loop, as management, in
the **same Jev call** as the event question (one call per flush, both
questions), reading `proj_capture_resolution` (the ready list, not the memory
queue) so unresolved captures are retried; it adds a `capture.entities` tape
record per flush and a qualified-id rule (a second entity with the same
normalised name gets an id that includes its opening mention) replacing the
same-name refusal. It assumes judgments are canonical (not reproducible) and
rows derived (re-exportable at zero calls), and that merges ride the same
flush (E4, `choir.entity_merge`, resolved at read, never rewriting edges).

**Lane `story-events-e5`** (a published story carries its event ids;
`reports_event`; red). One untracked file, `internal/types/story_event.go`
(**M**): the request types for `POST /internal/platform/story-events`, a
`StoryEventsRequest` that replaces a story's earlier set, roles `primary` and
`secondary` for a story, `member` and `tentative` for a capture, an
`EventPagePrefix` `/api/platform/events/`, and a consolidation read
(`/internal/platform/story-events/stories`) that returns the published stories
reporting the same events as a story, a set of captures or named events. It
assumes the computer stays the events' custodian and corpusd holds a **public
projection** (`choir.event_projection`, `in_event`, `reports_event`), with no
model call: a story reports an event when two of its cited captures or 30% are
counted members, the primary one having the most. The design itself says the
coverage depends on the join rate, because with 84% of captures minting their
own event few stories will reach the two-capture rule (design section 5).

**Lane `search-leads-l2`** (not on main; one commit `1e925aa37`, orange and
yellow). Adds a `Sufficiency` block to the ranked corpus answer (best cosine,
strong captures within a horizon, unread strong captures, unread leads, same
terms searched recently), a `search_query_times` table with one index, held-lead
matching by words and conversation, one host log line per web search and a
replay script (`scripts/search-sufficiency-replay`). It assumes leads are never
evidence and never embedded, the thresholds are provisional and gate nothing in
L2 (L3, the verb, reads them), and "unread" is a computer-wide mark, not the
run's own set, because the per-run set would widen the desk protocol (a red
change).

**`docs/design/entity-layer-2026-11-20.md`** (proposal, not ratified). Premise
corrected by the author: an entity layer already exists for stories. Entities are
shared behind corpusd and owner-scoped, judgments on the computer's tape, rows
derived; capture links come from one flush call that also resolves events;
events remain on the computer and corpusd holds a public projection (declared a
projection to satisfy I27); a story's event ids are derived deterministically at
publish; merges are records resolved at read; the first reader is research's
corpus search, then the story page, related, management's feed-or-open, the
research brief and the Autoradio DJ; Wikidata linking and name vectors are
deliberately not built. Stations E0 to E8; E0, E1 and E2 are landed.

**`docs/design/corpus-first-retrieval-and-research-2026-11-18.md`**. The
corpus is the first place research looks, the web is for the difference. One
vector index (stories by passage, captures by lead), one ranked answer
(vector first, BM25's top three guaranteed), events for every captured page by
a vector shortlist, a story's event ids exported as `reports_event` edges, entity
merges as `choir.entity_merge`, and a "neighbourhood read" (stories on the same
events, headline vectors at 0.86 or more, related by merged central entities,
held captures on those events, top passages) that **extends the existing walk
rather than writing a second one** (`RelatedStories`,
`publication_related.go:593`). It lists deletions: the setting
`research.related_story`, the three `CORPUSD_RELATED*` switches, the guest's
substring event search (`corpus_events.go:54`), the title-overlap picks, the
walk's title-word filler, and the capture term table's search role.

**`docs/definitions/corpus-first-search-one-verb-2026-11-19.md`**. One verb
`choir.Search(query, why ...string)` asks the corpus first and decides, from
one ranked corpus call and a `Sufficiency` block, whether to hold the web search
(unread strong captures or enough unread leads) or send it (with `since` set to
the newest strong capture minus a lag). Search leads (one row per owner,
computer and URL key, never evidence) make a web search's returned results
reusable; custodian corpusd; thresholds from the 200-question recall set and a
replay of a logged day. No cap, no switch, no extra model call. It assumes the
corpus and the leads are the only memory the verb consults: nothing in it reads
events or entities, so the "only search for what is new" question is, in its own
words, decided by captures and leads alone.

---

## What I could not determine

- **Events in production, in detail.** The count, members per event
  distribution and join rate of the current events are not read by me: the route
  needs the paper's key (401 without it) and the store is on the paper's disk.
  The numbers above are the records' (05:26Z and 19:40Z to 20:18Z). The share
  of questions with no stored vector after the embed kick (E1) is not measured.
- **Whether the paper's `research.related_story` setting is on.** Inferred from
  the growth read and the related answer's mode, not read from the setting
  (`current-architecture.md` says the same). Whether `intake.labels` is on for
  the paper is likewise not read (its default is true); how many
  `choir.capture_label` rows exist is not known.
- **Counts on the computer's `state.db`** (labels, memberships, capture
  dispositions, any `story.entities` tape records): unreadable from the graph
  DB, and I did not touch the paper VM.
- **`ProductAPI`'s allowed routes for management**, and whether the code that
  bounds `System1` per run and per day still exists (the verb text says it
  does; CLAUDE.md forbids such limits on our own calls).
- **Which of the 109 kinds are read, as opposed to named, by non-test code.**
  The table's file list is a literal-or-constant match, not a call graph; a
  kind named in one file may be both written and read there. Kinds reached only
  through a constant I did not search for (for example one built with
  `fmt.Sprintf`) may show no files.
- **Whether a Postgres equality between `og_edges.from_id` (default collation)
  and `og_objects.canonical_id` (C) is safe in every query.** One read-only
  count ran; `edge_collation_test.go` records the range-predicate failure. I did
  not test ordering or `LIKE`.
- **Front importance and section scores in production** (the JSON file): its
  contents and age were not read.
- **Exact verb list per desk.** Verb sets are computed from grants at run time
  (`deskmanifest.Desk.Verbs`); the table above is my reading of
  `grants.go` and `manifest.go`, not an enumeration from a running desk.
- **The share of captures that are private** or carry an `owner_private` label
  in the graph DB (the label is a field of the capture record, not a column;
  I did not scan bodies).
