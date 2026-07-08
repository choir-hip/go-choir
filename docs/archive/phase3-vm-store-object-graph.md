# Phase 3: VM Store → Object Graph (Direct Replacement)

**Status:** planned — design phase
**Date:** 2026-07-02
**Parent mission:** `docs/mission-unified-object-graph-v0.md` (Phase 3)

## Objective

Replace the VM's `store.Store` relational schema (~36 tables, 177 methods, ~15K lines) with the object graph (`og_objects` + `og_edges`). No dual-write. The object graph is the only substrate.

## Current State

### What exists today

- `internal/store/` — 177 methods on `store.Store` across 13 files, backed by embedded Dolt with ~36 relational tables
- `internal/objectgraph/` — `Store` interface, `Service`, `MemoryStore`, `HTTPStore`, registry with all object/edge kinds pre-registered
- `internal/platform/objectgraph_store.go` — corpusd's Dolt-backed `ObjectGraphStore` (writes to `og_objects`/`og_edges`)
- The VM runtime currently uses `HTTPStore` to talk to corpusd for object graph operations — this is wrong for VM-owned state. The VM's object graph must be local (embedded Dolt), not remote.

### What's missing

1. **Embedded Dolt `ObjectGraphStore`** — a `store.Store` implementation that writes to `og_objects`/`og_edges` in the VM's local embedded Dolt, not via HTTP to corpusd. The corpusd `ObjectGraphStore` in `internal/platform/objectgraph_store.go` is the reference implementation; we need a VM-local version.
2. **GraphStore adapter** — a type in `internal/store/` (or a new package) that implements the 177 `store.Store` methods by translating to object graph operations (CreateObject, PutEdge, ListObjects, ListEdges).
3. **Migration** — convert existing relational data to object graph objects+edges on first boot of the new code.

## Architecture

### Layer 1: VM-local Dolt ObjectGraphStore

Create `internal/objectgraph/dolt_store.go` — a `Store` implementation backed by the VM's embedded Dolt. This is the same schema as corpusd's `og_objects`/`og_edges`, but in the VM's local Dolt workspace.

```
internal/objectgraph/dolt_store.go
  type DoltStore struct { db *sql.DB }
  func NewDoltStore(db *sql.DB) *DoltStore
  func (s *DoltStore) PutObject(ctx, obj) error
  func (s *DoltStore) GetObject(ctx, id) (Object, error)
  func (s *DoltStore) ListObjects(ctx, filter) ([]Object, error)
  func (s *DoltStore) PutEdge(ctx, edge) error
  func (s *DoltStore) ListEdges(ctx, filter) ([]Edge, error)
  func (s *DoltStore) Close() error
  func (s *DoltStore) EnsureSchema() error  // CREATE TABLE og_objects / og_edges
```

This is a copy/adaptation of `internal/platform/objectgraph_store.go` but takes a `*sql.DB` directly instead of a `*platform.Store`. The schema DDL is identical.

### Layer 2: GraphStore (replaces store.Store)

Create `internal/store/graph_store.go` — a new `Store` replacement that delegates to `objectgraph.Service` for all persistence. This is where the 177 methods get reimplemented.

The key insight: most `store.Store` methods are CRUD over typed records. Each record type maps to an object kind:

| Relational table | Object kind | Identity mode |
|---|---|---|
| agents | choir.agent | external-key |
| runs | choir.run | external-key |
| events | choir.event | content-hash |
| channel_messages | choir.channel_message | content-hash |
| inbox_deliveries | choir.inbox_delivery | content-hash |
| run_memory_entries | choir.run_memory_entry | content-hash |
| trajectories | choir.trajectory | external-key |
| work_items | choir.work_item | external-key |
| run_acceptances | choir.run_acceptance | external-key |
| run_continuations | choir.run_continuation | external-key |
| texture_documents | choir.texture_document | external-key |
| texture_revisions | choir.texture_revision | content-hash, versioned |
| texture_decisions | choir.texture_decision | external-key |
| agent_evidence | choir.agent_evidence | content-hash |
| content_items | choir.content_item | content-hash |
| podcast_subscriptions | choir.podcast_subscription | external-key |
| browser_sessions | choir.browser_session | external-key |
| app_change_packages | choir.app_change_package | external-key |
| app_adoptions | choir.app_adoption | external-key |
| desktop_sessions | choir.desktop_session | external-key |
| desktop_app_instances | choir.desktop_app_instance | external-key |
| computer_source_lineages | edge: computer_lineage | edge-only |
| co_super_slots | edge: super_slot | edge-only |
| coagent_mailboxes | edge: coagent_mailbox | edge-only |
| media_progress | edge: media_progress | edge-only |
| media_recents | edge: media_recent | edge-only |
| user_preferences | edge: user_preference | edge-only |
| document_aliases | edge: document_alias | edge-only |
| agent_mutations | edge: document_mutation | edge-only |
| texture_controller_checkpoints | edge: document_checkpoint | edge-only |
| worker_updates | choir.worker_update (or edge) | content-hash |
| desktop_state/workspaces/placements | choir.desktop_state | external-key |

### Layer 3: Runtime wiring

`cmd/sandbox/main.go` changes:
- Replace `store.Open(path)` with `objectgraph.NewDoltStore(db)` + `objectgraph.NewService(...)` + `store.NewGraphStore(ogService)`
- The `Runtime` takes a `*store.Store`-compatible interface. Either keep the same struct shape or introduce an interface.

### Migration

On first boot with the new code:
1. Open the existing Dolt workspace
2. Check if `og_objects` table exists and has data → already migrated, skip
3. If not: read all relational tables, convert each row to an object/edge, write to `og_objects`/`og_edges`
4. Drop the relational tables

The migration is a one-time batch job. For VMs with large histories this could take seconds. The VM is offline during migration (it's the boot path).

## Execution Plan

This is too large for a single PR. Break into slices:

### Slice 1: DoltStore + schema (green)
- Create `internal/objectgraph/dolt_store.go`
- Port schema DDL from `internal/platform/store.go`
- Tests: verify PutObject/GetObject/ListObjects/PutEdge/ListEdges against embedded Dolt
- No runtime changes, no migration

### Slice 2: GraphStore core — runs, agents, events (orange)
- Create `internal/store/graph_store.go` with the `Store` struct reimagined as a wrapper around `objectgraph.Service`
- Implement: UpsertAgent, GetAgent, CreateRun, GetRun, UpdateRun, ListRuns*, AppendEvent, ListEvents*
- Implement: edge writes (run_agent, run_trajectory, event_run, run_parent)
- Tests: port existing store_test.go cases for these methods
- No runtime wiring yet

### Slice 3: GraphStore — trajectories, work items, co-super slots (orange)
- Implement: CreateTrajectoryIfAbsent, GetTrajectory, ListTrajectoriesByOwner, UpdateTrajectory*
- Implement: CreateWorkItem, GetWorkItem, ListWorkItems*, UpdateWorkItem*
- Implement: ClaimCoSuperSlot, ReleaseCoSuperSlotClaim, CoSuperSlot* (edge: super_slot)
- Tests: port existing tests

### Slice 4: GraphStore — channel messages, inbox, worker updates (orange)
- Implement: AppendChannelMessage, ListChannelMessages*
- Implement: worker_updates, coagent_mailboxes (edge: coagent_mailbox)
- Implement: DispatchWorkerUpdate, GetCoagentMailboxCursor, MarkWorkerUpdatesDelivered
- Tests: port existing tests

### Slice 5: GraphStore — texture documents, revisions, source graph (orange)
- Implement: CreateDocument, GetDocument, ListDocuments*, UpdateDocument, DeleteDocument
- Implement: CreateRevision, GetRevision, ListRevisions*, GetHistory, GetDiff, GetBlame
- Implement: texture_source_entities, texture_source_refs, document_aliases
- Implement: agent_mutations, texture_controller_checkpoints, texture_decisions, evidence
- Tests: port existing texture_test.go, texture_source_graph_test.go

### Slice 6: GraphStore — remaining tables (orange)
- run_memory_entries, run_acceptances, run_continuations
- browser_sessions, media_progress, media_recents, user_preferences, podcast_subscriptions
- content_items, app_change_packages, app_adoptions, computer_source_lineages
- desktop_state, desktop_workspaces, desktop_sessions, desktop_app_instances, desktop_window_placements
- Tests: port remaining tests

### Slice 7: Migration + runtime wiring (orange)
- Implement migration: read relational → write og_objects/og_edges → drop relational tables
- Wire `cmd/sandbox/main.go` to use GraphStore instead of store.Store
- Update `internal/runtime/` if any direct SQL queries need to change
- End-to-end test: boot VM, verify all operations work through the object graph

### Slice 8: Cleanup (yellow)
- Delete old relational schema DDL from `internal/store/store.go`
- Delete old method implementations (they're all in graph_store.go now)
- Remove `internal/store/migration.go` (legacy SQLite import — no longer relevant)
- Remove `internal/store/dolt_maintenance.go` (or adapt to GC the og_ tables)

## Risk Assessment

- **High risk:** The `store.Store` has 177 methods and ~400 call sites in `internal/runtime/`. A partial migration would leave the runtime in a broken state.
- **Mitigation:** GraphStore implements the same method signatures. The runtime doesn't need to change (unless it does raw SQL, which some methods do via `s.DB()`).
- **DB() exposure:** `store.Store.DB()` returns the raw `*sql.DB`. The trace store (`internal/trace`) uses this. We need to keep exposing the Dolt DB handle even after the relational tables are gone — the `og_objects`/`og_edges` tables are in the same DB.
- **Transactions:** Several methods use `sql.Tx` for atomic multi-table writes. The object graph `BatchStore` interface supports atomic batch writes. GraphStore should use `BatchStore` for these.
- **Query performance:** Relational indexes are purpose-built. Object graph queries use `ListObjects` with filters on `object_kind`/`owner_id`. The `og_objects` table has indexes on `(object_kind, owner_id)` and `(updated_at)`. This should be sufficient for most queries. Complex joins (e.g. GetDiff, GetBlame) may need to be reimplemented as graph traversals.

## Open Questions

1. **Should `store.Store` remain a concrete type or become an interface?** If it stays a concrete type, GraphStore replaces it entirely and the runtime doesn't change. If it becomes an interface, we can mock it in tests more easily, but we need to update all 400 call sites. Recommendation: keep concrete type, replace internals.

2. **What about `s.DB()` and `s.TexturePath()`?** The trace store uses `DB()` directly. We should keep the DB handle exposed — the `og_objects`/`og_edges` tables live in the same Dolt workspace. `TexturePath()` is used for the Dolt workspace path; keep it.

3. **Worker updates: object or edge?** Worker updates have a complex lifecycle (pending, delivered, backlog). They could be objects (choir.worker_update) with edges connecting them to runs/agents, or they could be edges with metadata. Recommendation: objects, because they have their own identity and lifecycle.

4. **Desktop state: object or edge?** Desktop state is a single record per owner+desktop. It could be an object (choir.desktop_state) or edges from owner to app instances. Recommendation: object, with edges for app_instance → desktop_session → desktop relationships.
