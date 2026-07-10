package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Object graph kind constants for VM store records.
const (
	ogKindAgent        = objectgraph.ObjectKind("choir.agent")
	ogKindRun          = objectgraph.ObjectKind("choir.run")
	ogKindEvent        = objectgraph.ObjectKind("choir.event")
	ogKindTrajectory   = objectgraph.ObjectKind("choir.trajectory")
	ogKindWorkItem     = objectgraph.ObjectKind("choir.work_item")
	ogKindChannelMsg   = objectgraph.ObjectKind("choir.channel_message")
	ogKindWorkerUpdate = objectgraph.ObjectKind("choir.worker_update")
	ogKindInboxDeliv   = objectgraph.ObjectKind("choir.inbox_delivery")
	ogKindRunMemory    = objectgraph.ObjectKind("choir.run_memory_entry")
	ogKindRunAccept    = objectgraph.ObjectKind("choir.run_acceptance")
	ogKindRunContin    = objectgraph.ObjectKind("choir.run_continuation")
	ogKindTexDoc       = objectgraph.ObjectKind("choir.texture_document")
	ogKindTexRev       = objectgraph.ObjectKind("choir.texture_revision")
	ogKindTexDecision  = objectgraph.ObjectKind("choir.texture_decision")
	ogKindEvidence     = objectgraph.ObjectKind("choir.agent_evidence")
	ogKindContentItem  = objectgraph.ObjectKind("choir.content_item")
	ogKindPodcastSub   = objectgraph.ObjectKind("choir.podcast_subscription")
	ogKindBrowserSess  = objectgraph.ObjectKind("choir.browser_session")
	ogKindCoagentMail  = objectgraph.ObjectKind("choir.coagent_mailbox")
	ogKindAppPackage   = objectgraph.ObjectKind("choir.app_change_package")
	ogKindAppAdoption  = objectgraph.ObjectKind("choir.app_adoption")
	ogKindDesktopSess  = objectgraph.ObjectKind("choir.desktop_session")
	ogKindDesktopApp   = objectgraph.ObjectKind("choir.desktop_app_instance")
)

// Edge kind constants.
const (
	ogEdgeRunAgent       = objectgraph.EdgeKind("run_agent")
	ogEdgeRunTrajectory  = objectgraph.EdgeKind("run_trajectory")
	ogEdgeRunParent      = objectgraph.EdgeKind("run_parent")
	ogEdgeEventRun       = objectgraph.EdgeKind("event_run")
	ogEdgeMsgFromRun     = objectgraph.EdgeKind("message_from_run")
	ogEdgeMsgToRun       = objectgraph.EdgeKind("message_to_run")
	ogEdgeWorkItemTraj   = objectgraph.EdgeKind("work_item_trajectory")
	ogEdgeWorkItemAgent  = objectgraph.EdgeKind("work_item_assigned_agent")
	ogEdgeAcceptRun      = objectgraph.EdgeKind("acceptance_run")
	ogEdgeAcceptTraj     = objectgraph.EdgeKind("acceptance_trajectory")
	ogEdgeContinFromRun  = objectgraph.EdgeKind("continuation_from_run")
	ogEdgeContinToRun    = objectgraph.EdgeKind("continuation_to_run")
	ogEdgeDecisionDoc    = objectgraph.EdgeKind("decision_document")
	ogEdgeDecisionRun    = objectgraph.EdgeKind("decision_run")
	ogEdgeEvidenceAgent  = objectgraph.EdgeKind("evidence_agent")
	ogEdgeSubContent     = objectgraph.EdgeKind("subscription_content")
	ogEdgeBrowserRun     = objectgraph.EdgeKind("browser_session_run")
	ogEdgePackageLineage = objectgraph.EdgeKind("package_source_computer")
	ogEdgeAdoptionPkg    = objectgraph.EdgeKind("adoption_package")
	ogEdgeAdoptionTarget = objectgraph.EdgeKind("adoption_target_computer")
	ogEdgeSessionDesktop = objectgraph.EdgeKind("session_desktop")
	ogEdgeAppDesktop     = objectgraph.EdgeKind("app_instance_desktop")
	ogEdgeDocRevision    = objectgraph.EdgeKind("document_revision")
	ogEdgeRevParent      = objectgraph.EdgeKind("revision_parent")
	ogEdgeSuperSlot      = objectgraph.EdgeKind("super_slot")
	ogEdgeCoagentMailbox = objectgraph.EdgeKind("coagent_mailbox")
	ogEdgeDocAlias       = objectgraph.EdgeKind("document_alias")
	ogEdgeDocMutation    = objectgraph.EdgeKind("document_mutation")
	ogEdgeDocCheckpoint  = objectgraph.EdgeKind("document_checkpoint")
	ogEdgeComputerLin    = objectgraph.EdgeKind("computer_lineage")
	ogEdgeMediaProgress  = objectgraph.EdgeKind("media_progress")
	ogEdgeMediaRecent    = objectgraph.EdgeKind("media_recent")
	ogEdgeUserPref       = objectgraph.EdgeKind("user_preference")
)

// ogPut creates or updates an object in the graph from a Go record.
// The record is serialized to JSON as the object body. The metadata map
// stores queryable fields (record ID, state, etc.) for JSON_EXTRACT
// lookups. The recordIDField is the metadata key that holds the record's
// primary key (e.g. "agent_id", "run_id"), enabling point lookups
// without knowing the owner.
func (s *Store) ogPut(ctx context.Context, kind objectgraph.ObjectKind, ownerID, identityKey string, body any, metadata map[string]any, now time.Time) (objectgraph.Object, error) {
	if s.og == nil {
		return objectgraph.Object{}, fmt.Errorf("store: object graph not initialized")
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return objectgraph.Object{}, fmt.Errorf("store: marshal body: %w", err)
	}
	obj, err := s.og.CreateObject(ctx, objectgraph.CreateObjectRequest{
		Kind:        kind,
		OwnerID:     ownerID,
		IdentityKey: identityKey,
		Body:        bodyJSON,
		Metadata:    metadata,
		Now:         now,
	})
	if err != nil {
		return objectgraph.Object{}, fmt.Errorf("store: create object: %w", err)
	}
	return obj, nil
}

// ogGetByKey finds an object by kind + a metadata field equality check.
// This enables point lookups by record ID (agent_id, run_id, etc.)
// without knowing the owner. Uses JSON_EXTRACT on the og_objects table.
// Uses the read connection pool when available to avoid blocking during
// write transactions.
func (s *Store) ogGetByKey(ctx context.Context, kind objectgraph.ObjectKind, metadataField, value string) (objectgraph.Object, error) {
	store := s.ogReadStore
	if store == nil {
		store = s.ogStore
	}
	if store == nil {
		return objectgraph.Object{}, fmt.Errorf("store: object graph not initialized")
	}
	return store.GetObjectByMetadata(ctx, string(kind), "$."+metadataField, value)
}

// ogListByMetadata lists objects by kind + a metadata field equality
// check, with an optional owner filter.
func (s *Store) ogListByMetadata(ctx context.Context, kind objectgraph.ObjectKind, metadataField, value string, limit int) ([]objectgraph.Object, error) {
	store := s.ogReadStore
	if store == nil {
		store = s.ogStore
	}
	if store == nil {
		return nil, fmt.Errorf("store: object graph not initialized")
	}
	return store.ListObjectsByMetadata(ctx, string(kind), "$."+metadataField, value, limit)
}

func (s *Store) ogListByOwnerAndBody(ctx context.Context, kind objectgraph.ObjectKind, ownerID string, matches []objectgraph.JSONFieldMatch, limit int) ([]objectgraph.Object, error) {
	store := s.ogReadStore
	if store == nil {
		store = s.ogStore
	}
	if store == nil {
		return nil, fmt.Errorf("store: object graph not initialized")
	}
	return store.ListObjectsByOwnerAndBody(ctx, string(kind), ownerID, matches, limit)
}

// ogIsEmpty reports whether the object graph has any objects at all.
// Used to gate SQL-to-OG backfill so it only runs on the first open,
// not on every restart (which would replay stale SQL over newer OG state).
func (s *Store) ogIsEmpty(ctx context.Context) (bool, error) {
	store := s.ogReadStore
	if store == nil {
		store = s.ogStore
	}
	if store == nil {
		return true, nil
	}
	objs, err := store.ListObjects(ctx, objectgraph.ListFilter{Limit: 1})
	if err != nil {
		return false, err
	}
	return len(objs) == 0, nil
}

// ogExistsByKey reports whether an object of the given kind exists with
// the specified metadata field value. Used for put-if-absent semantics
// in backfill methods.
func (s *Store) ogExistsByKey(ctx context.Context, kind objectgraph.ObjectKind, metadataField, value string) (bool, error) {
	_, err := s.ogGetByKey(ctx, kind, metadataField, value)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, objectgraph.ErrNotFound) {
		return false, nil
	}
	return false, err
}

// ogPutEdge creates an edge between two objects.
func (s *Store) ogPutEdge(ctx context.Context, fromID, toID string, kind objectgraph.EdgeKind, metadata any) error {
	if s.og == nil {
		return fmt.Errorf("store: object graph not initialized")
	}
	_, err := s.og.PutEdge(ctx, fromID, toID, kind, metadata)
	return err
}

// ogDecode unmarshals an object's body into the target.
func ogDecode(obj objectgraph.Object, target any) error {
	if err := json.Unmarshal(obj.Body, target); err != nil {
		return fmt.Errorf("store: unmarshal object body: %w", err)
	}
	return nil
}

// ogDelete removes an object from the graph by canonical ID.
func (s *Store) ogDelete(ctx context.Context, id string) error {
	if s.ogStore == nil {
		return fmt.Errorf("store: object graph not initialized")
	}
	return s.ogStore.DeleteObject(ctx, id)
}

// =========================================================================
// Agents — object graph implementation
// =========================================================================

// UpsertAgentOG stores or updates an agent record in the object graph.
// The agent_id is stored in metadata for point lookups.
func (s *Store) UpsertAgentOG(ctx context.Context, rec types.AgentRecord) error {
	now := rec.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"agent_id":   rec.AgentID,
		"sandbox_id": rec.SandboxID,
		"profile":    rec.Profile,
		"role":       rec.Role,
		"channel_id": rec.ChannelID,
		"created_at": rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at": now.UTC().Format(time.RFC3339Nano),
	}
	_, err := s.ogPut(ctx, ogKindAgent, rec.OwnerID, rec.AgentID, rec, metadata, now)
	return err
}

// GetAgentOG retrieves an agent by ID from the object graph.
// Looks up by agent_id in metadata since the canonical ID includes
// the owner, which the caller may not know.
func (s *Store) GetAgentOG(ctx context.Context, agentID string) (types.AgentRecord, error) {
	obj, err := s.ogGetByKey(ctx, ogKindAgent, "agent_id", agentID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return types.AgentRecord{}, ErrNotFound
		}
		return types.AgentRecord{}, err
	}
	var rec types.AgentRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.AgentRecord{}, err
	}
	return rec, nil
}

// =========================================================================
// Runs — object graph implementation
// =========================================================================

// CreateRunOG inserts a new run record in the object graph.
func (s *Store) CreateRunOG(ctx context.Context, rec types.RunRecord) error {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"run_id":              rec.RunID,
		"agent_id":            rec.AgentID,
		"channel_id":          rec.ChannelID,
		"requested_by_run_id": rec.RequestedByRunID,
		"trajectory_id":       rec.TrajectoryID,
		"agent_profile":       rec.AgentProfile,
		"agent_role":          rec.AgentRole,
		"sandbox_id":          rec.SandboxID,
		"state":               string(rec.State),
		"created_at":          rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":          rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if rec.FinishedAt != nil {
		metadata["finished_at"] = rec.FinishedAt.UTC().Format(time.RFC3339Nano)
	}

	obj, err := s.ogPut(ctx, ogKindRun, rec.OwnerID, rec.RunID, rec, metadata, now)
	if err != nil {
		return err
	}

	// Write structural edges.
	if rec.AgentID != "" {
		agentSuffix := objectgraph.StableSuffixFromKey(rec.AgentID)
		agentID, _ := objectgraph.BuildCanonicalID(ogKindAgent, rec.OwnerID, agentSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, agentID, ogEdgeRunAgent, nil)
	}
	if rec.TrajectoryID != "" {
		trajSuffix := objectgraph.StableSuffixFromKey(rec.TrajectoryID)
		trajID, _ := objectgraph.BuildCanonicalID(ogKindTrajectory, rec.OwnerID, trajSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, trajID, ogEdgeRunTrajectory, nil)
	}
	if rec.RequestedByRunID != "" {
		parentSuffix := objectgraph.StableSuffixFromKey(rec.RequestedByRunID)
		parentID, _ := objectgraph.BuildCanonicalID(ogKindRun, rec.OwnerID, parentSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, parentID, ogEdgeRunParent, nil)
	}
	return nil
}

// GetRunOG retrieves a run by ID from the object graph.
func (s *Store) GetRunOG(ctx context.Context, runID string) (types.RunRecord, error) {
	obj, err := s.ogGetByKey(ctx, ogKindRun, "run_id", runID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return types.RunRecord{}, ErrNotFound
		}
		return types.RunRecord{}, err
	}
	var rec types.RunRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.RunRecord{}, err
	}
	return rec, nil
}

// UpdateRunOG updates an existing run record in the object graph.
// Since CreateObject is an upsert (ON DUPLICATE KEY UPDATE), this is
// the same as CreateRunOG but preserves the original created_at.
func (s *Store) UpdateRunOG(ctx context.Context, rec types.RunRecord) error {
	// Fetch the existing object to preserve created_at.
	existing, err := s.ogGetByKey(ctx, ogKindRun, "run_id", rec.RunID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return fmt.Errorf("%w: run %s", ErrNotFound, rec.RunID)
		}
		return fmt.Errorf("update run: %w", err)
	}
	var existingRec types.RunRecord
	if err := ogDecode(existing, &existingRec); err != nil {
		return err
	}
	// Preserve created_at from the original.
	created := existingRec.CreatedAt
	if created.IsZero() {
		created = rec.CreatedAt
	}

	now := rec.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"run_id":              rec.RunID,
		"agent_id":            rec.AgentID,
		"channel_id":          rec.ChannelID,
		"requested_by_run_id": rec.RequestedByRunID,
		"trajectory_id":       rec.TrajectoryID,
		"agent_profile":       rec.AgentProfile,
		"agent_role":          rec.AgentRole,
		"sandbox_id":          rec.SandboxID,
		"state":               string(rec.State),
		"created_at":          created.UTC().Format(time.RFC3339Nano),
		"updated_at":          now.UTC().Format(time.RFC3339Nano),
	}
	if rec.FinishedAt != nil {
		metadata["finished_at"] = rec.FinishedAt.UTC().Format(time.RFC3339Nano)
	}

	// Use PutObject directly to update in place (same canonical ID).
	updated := existing
	updated.UpdatedAt = now
	bodyJSON, _ := json.Marshal(rec)
	updated.Body = bodyJSON
	updated.Metadata = mustMarshalMetadata(metadata)
	return s.ogStore.PutObject(ctx, updated)
}

// ListRunsByOwnerOG lists runs by owner from the object graph.
func (s *Store) ListRunsByOwnerOG(ctx context.Context, ownerID string, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:    ogKindRun,
		OwnerID: ownerID,
		Limit:   limit,
	})
	if err != nil {
		return nil, err
	}
	runs := make([]types.RunRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.RunRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		runs = append(runs, rec)
	}
	return runs, nil
}

// ListRunsByStateOG lists runs by state from the object graph.
// Uses metadata JSON_EXTRACT to filter by state.
func (s *Store) ListRunsByStateOG(ctx context.Context, state types.RunState, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.ogListByMetadata(ctx, ogKindRun, "state", string(state), limit)
	if err != nil {
		return nil, err
	}
	runs := make([]types.RunRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.RunRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		runs = append(runs, rec)
	}
	return runs, nil
}

// ListAllRunsOG lists all runs from the object graph, ordered by
// created_at descending.
func (s *Store) ListAllRunsOG(ctx context.Context, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	store := s.ogReadStore
	if store == nil {
		store = s.ogStore
	}
	if store == nil {
		return nil, fmt.Errorf("store: object graph not initialized")
	}
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:  ogKindRun,
		Limit: limit,
	})
	if err != nil {
		return nil, err
	}
	runs := make([]types.RunRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.RunRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		runs = append(runs, rec)
	}
	return runs, nil
}

// =========================================================================
// Events — object graph implementation
// =========================================================================

// AppendEventOG stores an event in the object graph.
// Events use content-hash identity, so the canonical ID is derived
// from the event content. The run_id and seq are stored in metadata
// for querying.
func (s *Store) AppendEventOG(ctx context.Context, rec *types.EventRecord) error {
	now := rec.Timestamp
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"event_id":      rec.EventID,
		"seq":           rec.Seq,
		"stream_seq":    rec.StreamSeq,
		"run_id":        rec.RunID,
		"agent_id":      rec.AgentID,
		"channel_id":    rec.ChannelID,
		"trajectory_id": rec.TrajectoryID,
		"kind":          string(rec.Kind),
		"phase":         rec.Phase,
		"timestamp":     rec.Timestamp.UTC().Format(time.RFC3339Nano),
	}

	_, err := s.ogPut(ctx, ogKindEvent, rec.OwnerID, "", rec, metadata, now)
	if err != nil {
		return err
	}

	// Write edge from event to run if run_id is set.
	if rec.RunID != "" {
		// We don't know the run's owner, so we can't build the canonical
		// ID directly. Skip the edge for now — the run_id in metadata
		// is sufficient for querying.
		// TODO: when runs and events share the same owner, write the edge.
	}
	return nil
}

// ListEventsOG lists events for a run from the object graph.
func (s *Store) ListEventsOG(ctx context.Context, runID string, limit int) ([]types.EventRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.ogListByMetadata(ctx, ogKindEvent, "run_id", runID, limit)
	if err != nil {
		return nil, err
	}
	events := make([]types.EventRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.EventRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		events = append(events, rec)
	}
	return events, nil
}

// ListEventsByOwnerOG lists events by owner from the object graph.
func (s *Store) ListEventsByOwnerOG(ctx context.Context, ownerID string, limit int) ([]types.EventRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:    ogKindEvent,
		OwnerID: ownerID,
		Limit:   limit,
	})
	if err != nil {
		return nil, err
	}
	events := make([]types.EventRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.EventRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		events = append(events, rec)
	}
	return events, nil
}

// =========================================================================
// Trajectories — object graph implementation
// =========================================================================

// CreateTrajectoryIfAbsentOG creates a trajectory if it doesn't exist.
// Returns the stored record (existing or newly created).
func (s *Store) CreateTrajectoryIfAbsentOG(ctx context.Context, rec types.TrajectoryRecord) (types.TrajectoryRecord, error) {
	// Check if it already exists.
	existing, err := s.ogGetByKey(ctx, ogKindTrajectory, "trajectory_id", rec.TrajectoryID)
	if err == nil {
		var existingRec types.TrajectoryRecord
		if err := ogDecode(existing, &existingRec); err != nil {
			return types.TrajectoryRecord{}, err
		}
		return existingRec, nil
	}
	if err != objectgraph.ErrNotFound {
		return types.TrajectoryRecord{}, err
	}

	// Create new.
	if rec.Kind == "" {
		rec.Kind = types.TrajectoryKindTask
	}
	if rec.Status == "" {
		rec.Status = types.TrajectoryLive
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	metadata := map[string]any{
		"trajectory_id": rec.TrajectoryID,
		"kind":          string(rec.Kind),
		"status":        string(rec.Status),
		"created_at":    rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":    rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if rec.SettledAt != nil {
		metadata["settled_at"] = rec.SettledAt.UTC().Format(time.RFC3339Nano)
	}

	_, err = s.ogPut(ctx, ogKindTrajectory, rec.OwnerID, rec.TrajectoryID, rec, metadata, now)
	if err != nil {
		return types.TrajectoryRecord{}, err
	}
	return rec, nil
}

// GetTrajectoryOG retrieves a trajectory by ID from the object graph.
func (s *Store) GetTrajectoryOG(ctx context.Context, ownerID, trajectoryID string) (types.TrajectoryRecord, error) {
	obj, err := s.ogGetByKey(ctx, ogKindTrajectory, "trajectory_id", trajectoryID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return types.TrajectoryRecord{}, ErrNotFound
		}
		return types.TrajectoryRecord{}, err
	}
	var rec types.TrajectoryRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.TrajectoryRecord{}, err
	}
	// Verify owner matches.
	if rec.OwnerID != ownerID {
		return types.TrajectoryRecord{}, ErrNotFound
	}
	return rec, nil
}

// ListTrajectoriesByOwnerOG lists trajectories by owner.
func (s *Store) ListTrajectoriesByOwnerOG(ctx context.Context, ownerID string, limit int) ([]types.TrajectoryRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:    ogKindTrajectory,
		OwnerID: ownerID,
		Limit:   limit,
	})
	if err != nil {
		return nil, err
	}
	trajs := make([]types.TrajectoryRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.TrajectoryRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		trajs = append(trajs, rec)
	}
	return trajs, nil
}

// UpdateTrajectoryStatusOG updates the status of a trajectory.
func (s *Store) UpdateTrajectoryStatusOG(ctx context.Context, ownerID, trajectoryID string, status types.TrajectoryStatus) (types.TrajectoryRecord, error) {
	obj, err := s.ogGetByKey(ctx, ogKindTrajectory, "trajectory_id", trajectoryID)
	if err != nil {
		return types.TrajectoryRecord{}, err
	}
	var rec types.TrajectoryRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.TrajectoryRecord{}, err
	}
	if rec.OwnerID != ownerID {
		return types.TrajectoryRecord{}, ErrNotFound
	}

	rec.Status = status
	rec.UpdatedAt = time.Now().UTC()
	if status == types.TrajectorySettled && rec.SettledAt == nil {
		now := rec.UpdatedAt
		rec.SettledAt = &now
	}

	return s.upsertTrajectoryOG(ctx, rec, obj)
}

// upsertTrajectoryOG writes the trajectory record back to the object graph,
// preserving the existing object ID and created_at.
func (s *Store) upsertTrajectoryOG(ctx context.Context, rec types.TrajectoryRecord, existing objectgraph.Object) (types.TrajectoryRecord, error) {
	metadata := map[string]any{
		"trajectory_id": rec.TrajectoryID,
		"kind":          string(rec.Kind),
		"status":        string(rec.Status),
		"created_at":    rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":    rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if rec.SettledAt != nil {
		metadata["settled_at"] = rec.SettledAt.UTC().Format(time.RFC3339Nano)
	}

	bodyJSON, _ := json.Marshal(rec)
	existing.Body = bodyJSON
	existing.Metadata = mustMarshalMetadata(metadata)
	existing.UpdatedAt = rec.UpdatedAt
	if err := s.ogStore.PutObject(ctx, existing); err != nil {
		return types.TrajectoryRecord{}, err
	}
	return rec, nil
}

// =========================================================================
// Work Items — object graph implementation
// =========================================================================

// CreateWorkItemOG creates a work item in the object graph.
func (s *Store) CreateWorkItemOG(ctx context.Context, rec types.WorkItemRecord) (types.WorkItemRecord, error) {
	if rec.WorkItemID == "" {
		return types.WorkItemRecord{}, fmt.Errorf("create work item: work_item_id is required")
	}
	if rec.Status == "" {
		rec.Status = types.WorkItemOpen
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	metadata := map[string]any{
		"work_item_id":          rec.WorkItemID,
		"trajectory_id":         rec.TrajectoryID,
		"status":                string(rec.Status),
		"assigned_agent_id":     rec.AssignedAgentID,
		"objective_fingerprint": rec.ObjectiveFingerprint,
		"created_by_run_id":     rec.CreatedByRunID,
		"created_at":            rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":            rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}

	obj, err := s.ogPut(ctx, ogKindWorkItem, rec.OwnerID, rec.WorkItemID, rec, metadata, now)
	if err != nil {
		return types.WorkItemRecord{}, err
	}

	// Write edge to trajectory.
	if rec.TrajectoryID != "" {
		trajSuffix := objectgraph.StableSuffixFromKey(rec.TrajectoryID)
		trajID, _ := objectgraph.BuildCanonicalID(ogKindTrajectory, rec.OwnerID, trajSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, trajID, ogEdgeWorkItemTraj, nil)
	}
	// Write edge to assigned agent.
	if rec.AssignedAgentID != "" {
		agentSuffix := objectgraph.StableSuffixFromKey(rec.AssignedAgentID)
		agentID, _ := objectgraph.BuildCanonicalID(ogKindAgent, rec.OwnerID, agentSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, agentID, ogEdgeWorkItemAgent, nil)
	}
	return rec, nil
}

// GetWorkItemOG retrieves a work item by ID.
func (s *Store) GetWorkItemOG(ctx context.Context, ownerID, workItemID string) (types.WorkItemRecord, error) {
	obj, err := s.ogGetByKey(ctx, ogKindWorkItem, "work_item_id", workItemID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return types.WorkItemRecord{}, ErrNotFound
		}
		return types.WorkItemRecord{}, err
	}
	var rec types.WorkItemRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.WorkItemRecord{}, err
	}
	if rec.OwnerID != ownerID {
		return types.WorkItemRecord{}, ErrNotFound
	}
	return rec, nil
}

// ListWorkItemsByTrajectoryOG lists work items for a trajectory.
func (s *Store) ListWorkItemsByTrajectoryOG(ctx context.Context, ownerID, trajectoryID string, openOnly bool) ([]types.WorkItemRecord, error) {
	// Fetch a large window since ogListByMetadata orders by updated_at
	// DESC and we need all matching records to filter by owner/open
	// status. Using a small limit can miss older work items that are
	// still open.
	objs, err := s.ogListByMetadata(ctx, ogKindWorkItem, "trajectory_id", trajectoryID, 100000)
	if err != nil {
		return nil, err
	}
	items := make([]types.WorkItemRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.WorkItemRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		if openOnly && rec.Status != types.WorkItemOpen {
			continue
		}
		items = append(items, rec)
	}
	return items, nil
}

// UpdateWorkItemStatusOG updates the status of a work item.
func (s *Store) UpdateWorkItemStatusOG(ctx context.Context, ownerID, workItemID string, status types.WorkItemStatus) (types.WorkItemRecord, error) {
	obj, err := s.ogGetByKey(ctx, ogKindWorkItem, "work_item_id", workItemID)
	if err != nil {
		return types.WorkItemRecord{}, err
	}
	var rec types.WorkItemRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.WorkItemRecord{}, err
	}
	if rec.OwnerID != ownerID {
		return types.WorkItemRecord{}, ErrNotFound
	}

	rec.Status = status
	rec.UpdatedAt = time.Now().UTC()

	metadata := map[string]any{
		"work_item_id":          rec.WorkItemID,
		"trajectory_id":         rec.TrajectoryID,
		"status":                string(rec.Status),
		"assigned_agent_id":     rec.AssignedAgentID,
		"objective_fingerprint": rec.ObjectiveFingerprint,
		"created_by_run_id":     rec.CreatedByRunID,
		"created_at":            rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":            rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}

	bodyJSON, _ := json.Marshal(rec)
	obj.Body = bodyJSON
	obj.Metadata = mustMarshalMetadata(metadata)
	obj.UpdatedAt = rec.UpdatedAt
	if err := s.ogStore.PutObject(ctx, obj); err != nil {
		return types.WorkItemRecord{}, err
	}
	return rec, nil
}

// =========================================================================
// Channel Messages — object graph implementation
// =========================================================================

// AppendChannelMessageOG stores a channel message in the object graph.
// Channel messages use content-hash identity.
func (s *Store) AppendChannelMessageOG(ctx context.Context, message *types.ChannelMessage, ownerID string) error {
	now := message.Timestamp
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"channel_id":    message.ChannelID,
		"seq":           message.Seq,
		"from_agent_id": message.FromAgentID,
		"from_run_id":   message.FromRunID,
		"to_agent_id":   message.ToAgentID,
		"to_run_id":     message.ToRunID,
		"trajectory_id": message.TrajectoryID,
		"role":          message.Role,
		"timestamp":     message.Timestamp.UTC().Format(time.RFC3339Nano),
	}

	_, err := s.ogPut(ctx, ogKindChannelMsg, ownerID, "", message, metadata, now)
	return err
}

// ListChannelMessagesOG lists messages for a channel after a sequence
// number.
func (s *Store) ListChannelMessagesOG(ctx context.Context, ownerID, channelID string, afterSeq int64, limit int) ([]types.ChannelMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	// Fetch a large window since ogListByMetadata orders by updated_at
	// DESC, not by seq. We need all matching messages to filter by
	// afterSeq and sort by seq to preserve cursor-based pagination.
	// 100000 is a practical upper bound that covers all real-world
	// channels without causing excessive memory allocation.
	objs, err := s.ogListByMetadata(ctx, ogKindChannelMsg, "channel_id", channelID, 100000)
	if err != nil {
		return nil, err
	}
	msgs := make([]types.ChannelMessage, 0, len(objs))
	for _, obj := range objs {
		var rec types.ChannelMessage
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		// When ownerID is empty, return messages for all owners
		// (unscoped read used by Runtime.ChannelRead).
		if ownerID != "" && obj.OwnerID != ownerID {
			continue
		}
		if rec.Seq <= afterSeq {
			continue
		}
		msgs = append(msgs, rec)
	}
	// Sort by seq ascending to match the old SQL ORDER BY seq ASC.
	sort.Slice(msgs, func(i, j int) bool { return msgs[i].Seq < msgs[j].Seq })
	if len(msgs) > limit {
		msgs = msgs[:limit]
	}
	return msgs, nil
}

// =========================================================================
// Worker Updates — object graph implementation
// =========================================================================

// CreateWorkerUpdateOG stores a worker update in the object graph.
func (s *Store) CreateWorkerUpdateOG(ctx context.Context, rec types.CoagentSourcePacket) error {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"update_id":       rec.UpdateID,
		"agent_id":        rec.AgentID,
		"target_agent_id": rec.TargetAgentID,
		"channel_id":      rec.ChannelID,
		"trajectory_id":   rec.TrajectoryID,
		"role":            rec.Role,
		"message_seq":     rec.MessageSeq,
	}
	if rec.DeliveredToRunID != "" {
		metadata["delivered_to_run_id"] = rec.DeliveredToRunID
	}
	_, err := s.ogPut(ctx, objectgraph.ObjectKind("choir.worker_update"), rec.OwnerID, rec.UpdateID, rec, metadata, now)
	return err
}

// GetWorkerUpdateOG retrieves a worker update by ID, scoped to the given owner.
func (s *Store) GetWorkerUpdateOG(ctx context.Context, ownerID, updateID string) (types.CoagentSourcePacket, error) {
	// Use ogListByMetadata to find all updates with this update_id, then
	// filter by owner. This avoids the ogGetByKey single-match limitation
	// which could return another owner's record with the same update_id.
	objs, err := s.ogListByMetadata(ctx, objectgraph.ObjectKind("choir.worker_update"), "update_id", updateID, 100)
	if err != nil {
		return types.CoagentSourcePacket{}, err
	}
	for _, obj := range objs {
		if obj.OwnerID != ownerID {
			continue
		}
		var rec types.CoagentSourcePacket
		if err := ogDecode(obj, &rec); err != nil {
			return types.CoagentSourcePacket{}, err
		}
		return rec, nil
	}
	return types.CoagentSourcePacket{}, ErrNotFound
}

// ListWorkerUpdatesByTrajectoryOG lists worker updates for a trajectory.
func (s *Store) ListWorkerUpdatesByTrajectoryOG(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.CoagentSourcePacket, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.ogListByMetadata(ctx, objectgraph.ObjectKind("choir.worker_update"), "trajectory_id", trajectoryID, limit)
	if err != nil {
		return nil, err
	}
	packets := make([]types.CoagentSourcePacket, 0, len(objs))
	for _, obj := range objs {
		var rec types.CoagentSourcePacket
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		packets = append(packets, rec)
	}
	return packets, nil
}

// ListPendingWorkerUpdatesOG lists pending (undelivered) worker updates
// for a target agent.
func (s *Store) ListPendingWorkerUpdatesOG(ctx context.Context, ownerID, targetAgentID string, limit int) ([]types.CoagentSourcePacket, error) {
	if limit <= 0 {
		limit = 100
	}
	// Fetch a large window since ogListByMetadata orders by updated_at DESC.
	// We need all matching records to filter out delivered ones and then
	// sort by created_at before applying the caller's limit. Using a small
	// limit can miss older undelivered records when many delivered records
	// are more recently updated.
	objs, err := s.ogListByMetadata(ctx, objectgraph.ObjectKind("choir.worker_update"), "target_agent_id", targetAgentID, 100000)
	if err != nil {
		return nil, err
	}
	packets := make([]types.CoagentSourcePacket, 0, len(objs))
	for _, obj := range objs {
		var rec types.CoagentSourcePacket
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		if rec.DeliveredToRunID != "" {
			continue
		}
		packets = append(packets, rec)
	}
	sort.Slice(packets, func(i, j int) bool {
		if !packets[i].CreatedAt.Equal(packets[j].CreatedAt) {
			return packets[i].CreatedAt.Before(packets[j].CreatedAt)
		}
		return packets[i].MessageSeq < packets[j].MessageSeq
	})
	if len(packets) > limit {
		packets = packets[:limit]
	}
	return packets, nil
}

// =========================================================================
// Inbox Deliveries — object graph implementation
// =========================================================================

// CreateInboxDeliveryOG stores an inbox delivery in the object graph.
func (s *Store) CreateInboxDeliveryOG(ctx context.Context, rec types.InboxDelivery) error {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"delivery_id":         rec.DeliveryID,
		"to_agent_id":         rec.ToAgentID,
		"to_run_id":           rec.ToRunID,
		"from_agent_id":       rec.FromAgentID,
		"from_run_id":         rec.FromRunID,
		"channel_id":          rec.ChannelID,
		"role":                rec.Role,
		"trajectory_id":       rec.TrajectoryID,
		"delivered_to_run_id": rec.DeliveredToLoopID,
		"created_at":          rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	if rec.DeliveredAt != nil {
		metadata["delivered_at"] = rec.DeliveredAt.UTC().Format(time.RFC3339Nano)
	}

	_, err := s.ogPut(ctx, ogKindInboxDeliv, rec.OwnerID, rec.DeliveryID, rec, metadata, now)
	return err
}

// GetInboxDeliveryOG retrieves an inbox delivery by ID.
func (s *Store) GetInboxDeliveryOG(ctx context.Context, ownerID, deliveryID string) (types.InboxDelivery, error) {
	obj, err := s.ogGetByKey(ctx, ogKindInboxDeliv, "delivery_id", deliveryID)
	if err != nil {
		return types.InboxDelivery{}, err
	}
	var rec types.InboxDelivery
	if err := ogDecode(obj, &rec); err != nil {
		return types.InboxDelivery{}, err
	}
	if rec.OwnerID != ownerID {
		return types.InboxDelivery{}, ErrNotFound
	}
	return rec, nil
}

// =========================================================================
// Run Memory Entries — object graph implementation
// =========================================================================

// AppendRunMemoryEntryOG stores a run memory entry in the object graph.
// Run memory entries use external-key identity (entry_id).
func (s *Store) AppendRunMemoryEntryOG(ctx context.Context, rec types.RunMemoryEntry) error {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"entry_id":            rec.EntryID,
		"run_id":              rec.RunID,
		"agent_id":            rec.AgentID,
		"parent_entry_id":     rec.ParentEntryID,
		"seq":                 rec.Seq,
		"kind":                string(rec.Kind),
		"role":                rec.Role,
		"model":               rec.Model,
		"tokens_before":       rec.TokensBefore,
		"first_kept_entry_id": rec.FirstKeptEntryID,
		"created_at":          rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	}

	_, err := s.ogPut(ctx, ogKindRunMemory, rec.OwnerID, rec.EntryID, rec, metadata, now)
	return err
}

// ListRunMemoryEntriesOG lists memory entries for a run.
func (s *Store) ListRunMemoryEntriesOG(ctx context.Context, ownerID, runID string, limit int) ([]types.RunMemoryEntry, error) {
	if limit <= 0 {
		limit = 1000
	}
	objs, err := s.ogListByMetadata(ctx, ogKindRunMemory, "run_id", runID, limit)
	if err != nil {
		return nil, err
	}
	entries := make([]types.RunMemoryEntry, 0, len(objs))
	for _, obj := range objs {
		var rec types.RunMemoryEntry
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		entries = append(entries, rec)
	}
	return entries, nil
}

// =========================================================================
// Run Acceptances — object graph implementation
// =========================================================================

// CreateRunAcceptanceOG stores a run acceptance record in the object graph.
// Acceptances use external-key identity (acceptance_id).
func (s *Store) CreateRunAcceptanceOG(ctx context.Context, rec types.RunAcceptanceRecord) error {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"acceptance_id":     rec.AcceptanceID,
		"target_mission_id": rec.TargetMissionID,
		"trajectory_id":     rec.TrajectoryID,
		"run_id":            rec.RunID,
		"desktop_id":        rec.DesktopID,
		"authority_profile": rec.AuthorityProfile,
		"base_sha":          rec.BaseSHA,
		"deployment_commit": rec.DeploymentCommit,
		"ci_run_id":         rec.CIRunID,
		"deploy_run_id":     rec.DeployRunID,
		"staging_url":       rec.StagingURL,
		"health_commit":     rec.HealthCommit,
		"acceptance_level":  string(rec.AcceptanceLevel),
		"vm_mode":           rec.VMMode,
		"state":             string(rec.State),
		"created_at":        rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":        rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}

	obj, err := s.ogPut(ctx, ogKindRunAccept, rec.OwnerID, rec.AcceptanceID, rec, metadata, now)
	if err != nil {
		return err
	}

	// Write edge to run if set.
	if rec.RunID != "" {
		runSuffix := objectgraph.StableSuffixFromKey(rec.RunID)
		runID, _ := objectgraph.BuildCanonicalID(ogKindRun, rec.OwnerID, runSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, runID, ogEdgeAcceptRun, nil)
	}
	// Write edge to trajectory.
	if rec.TrajectoryID != "" {
		trajSuffix := objectgraph.StableSuffixFromKey(rec.TrajectoryID)
		trajID, _ := objectgraph.BuildCanonicalID(ogKindTrajectory, rec.OwnerID, trajSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, trajID, ogEdgeAcceptTraj, nil)
	}
	return nil
}

// GetRunAcceptanceOG retrieves a run acceptance by ID.
func (s *Store) GetRunAcceptanceOG(ctx context.Context, ownerID, acceptanceID string) (types.RunAcceptanceRecord, error) {
	obj, err := s.ogGetByKey(ctx, ogKindRunAccept, "acceptance_id", acceptanceID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return types.RunAcceptanceRecord{}, ErrNotFound
		}
		return types.RunAcceptanceRecord{}, err
	}
	var rec types.RunAcceptanceRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.RunAcceptanceRecord{}, err
	}
	if rec.OwnerID != ownerID {
		return types.RunAcceptanceRecord{}, ErrNotFound
	}
	return rec, nil
}

// GetRunAcceptanceByIDOG retrieves a run acceptance by ID without owner scoping.
func (s *Store) GetRunAcceptanceByIDOG(ctx context.Context, acceptanceID string) (types.RunAcceptanceRecord, error) {
	obj, err := s.ogGetByKey(ctx, ogKindRunAccept, "acceptance_id", acceptanceID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return types.RunAcceptanceRecord{}, ErrNotFound
		}
		return types.RunAcceptanceRecord{}, err
	}
	var rec types.RunAcceptanceRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.RunAcceptanceRecord{}, err
	}
	return rec, nil
}

// ListRunAcceptancesByTrajectoryOG lists acceptances for a trajectory.
func (s *Store) ListRunAcceptancesByTrajectoryOG(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.RunAcceptanceRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.ogListByMetadata(ctx, ogKindRunAccept, "trajectory_id", trajectoryID, limit)
	if err != nil {
		return nil, err
	}
	accepts := make([]types.RunAcceptanceRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.RunAcceptanceRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		accepts = append(accepts, rec)
	}
	return accepts, nil
}

// =========================================================================
// Run Continuations — object graph implementation
// =========================================================================

// CreateRunContinuationOG stores a run continuation record.
// Continuations use external-key identity (continuation_id).
func (s *Store) CreateRunContinuationOG(ctx context.Context, rec types.RunContinuationRecord) error {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"continuation_id":   rec.ContinuationID,
		"source_run_id":     rec.SourceRunID,
		"next_run_id":       rec.NextRunID,
		"authority_profile": rec.AuthorityProfile,
		"lease_seconds":     rec.LeaseSeconds,
		"status":            string(rec.Status),
		"created_at":        rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":        rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}

	obj, err := s.ogPut(ctx, ogKindRunContin, rec.OwnerID, rec.ContinuationID, rec, metadata, now)
	if err != nil {
		return err
	}

	// Write edges to source and next runs.
	if rec.SourceRunID != "" {
		runSuffix := objectgraph.StableSuffixFromKey(rec.SourceRunID)
		runID, _ := objectgraph.BuildCanonicalID(ogKindRun, rec.OwnerID, runSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, runID, ogEdgeContinFromRun, nil)
	}
	if rec.NextRunID != "" {
		runSuffix := objectgraph.StableSuffixFromKey(rec.NextRunID)
		runID, _ := objectgraph.BuildCanonicalID(ogKindRun, rec.OwnerID, runSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, runID, ogEdgeContinToRun, nil)
	}
	return nil
}

// GetRunContinuationOG retrieves a run continuation by ID.
func (s *Store) GetRunContinuationOG(ctx context.Context, ownerID, continuationID string) (types.RunContinuationRecord, error) {
	obj, err := s.ogGetByKey(ctx, ogKindRunContin, "continuation_id", continuationID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return types.RunContinuationRecord{}, ErrNotFound
		}
		return types.RunContinuationRecord{}, err
	}
	var rec types.RunContinuationRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.RunContinuationRecord{}, err
	}
	if rec.OwnerID != ownerID {
		return types.RunContinuationRecord{}, ErrNotFound
	}
	return rec, nil
}

// ListRunContinuationsBySourceRunOG lists continuations from a source run.
func (s *Store) ListRunContinuationsBySourceRunOG(ctx context.Context, ownerID, sourceRunID string, limit int) ([]types.RunContinuationRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.ogListByMetadata(ctx, ogKindRunContin, "source_run_id", sourceRunID, limit)
	if err != nil {
		return nil, err
	}
	contins := make([]types.RunContinuationRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.RunContinuationRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		contins = append(contins, rec)
	}
	return contins, nil
}

// =========================================================================
// Texture Documents — object graph implementation
// =========================================================================

// CreateTextureDocumentOG creates a texture document in the object graph.
// Documents use external-key identity (doc_id).
func (s *Store) CreateTextureDocumentOG(ctx context.Context, rec types.Document) error {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"doc_id":              rec.DocID,
		"title":               rec.Title,
		"current_revision_id": rec.CurrentRevisionID,
		"created_at":          rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":          rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}

	_, err := s.ogPut(ctx, ogKindTexDoc, rec.OwnerID, rec.DocID, rec, metadata, now)
	return err
}

// GetTextureDocumentOG retrieves a texture document by ID.
func (s *Store) GetTextureDocumentOG(ctx context.Context, ownerID, docID string) (types.Document, error) {
	obj, err := s.ogGetByKey(ctx, ogKindTexDoc, "doc_id", docID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return types.Document{}, ErrNotFound
		}
		return types.Document{}, err
	}
	var rec types.Document
	if err := ogDecode(obj, &rec); err != nil {
		return types.Document{}, err
	}
	if rec.OwnerID != ownerID {
		return types.Document{}, ErrNotFound
	}
	return rec, nil
}

// ListTextureDocumentsByOwnerOG lists documents by owner.
func (s *Store) ListTextureDocumentsByOwnerOG(ctx context.Context, ownerID string, limit int) ([]types.Document, error) {
	if limit <= 0 {
		limit = 100
	}
	// Fetch a large window since ListObjects orders by og_objects.updated_at
	// which may differ from Document.UpdatedAt (e.g. after backfill where
	// the OG timestamp comes from CreatedAt). We sort by Document.UpdatedAt
	// and then apply the caller's limit.
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:    ogKindTexDoc,
		OwnerID: ownerID,
		Limit:   100000,
	})
	if err != nil {
		return nil, err
	}
	docs := make([]types.Document, 0, len(objs))
	for _, obj := range objs {
		var rec types.Document
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		docs = append(docs, rec)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].UpdatedAt.After(docs[j].UpdatedAt) })
	if len(docs) > limit {
		docs = docs[:limit]
	}
	return docs, nil
}

// ListAllTextureDocumentsOG lists all texture documents across all owners,
// ordered by updated_at descending.
func (s *Store) ListAllTextureDocumentsOG(ctx context.Context, limit int) ([]types.Document, error) {
	if limit <= 0 {
		limit = 500
	}
	// Fetch a large window since ListObjects orders by og_objects.updated_at
	// which may differ from Document.UpdatedAt (e.g. after backfill where
	// object updated_at comes from creation time). We sort by Document.UpdatedAt
	// and then apply the caller's limit.
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:  ogKindTexDoc,
		Limit: 100000,
	})
	if err != nil {
		return nil, err
	}
	docs := make([]types.Document, 0, len(objs))
	for _, obj := range objs {
		var rec types.Document
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		docs = append(docs, rec)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].UpdatedAt.After(docs[j].UpdatedAt) })
	if len(docs) > limit {
		docs = docs[:limit]
	}
	return docs, nil
}

// UpdateTextureDocumentOG updates a texture document (e.g. to set
// current_revision_id).
func (s *Store) UpdateTextureDocumentOG(ctx context.Context, rec types.Document) error {
	obj, err := s.ogGetByKey(ctx, ogKindTexDoc, "doc_id", rec.DocID)
	if err != nil {
		return err
	}
	var existing types.Document
	if err := ogDecode(obj, &existing); err != nil {
		return err
	}
	if existing.OwnerID != rec.OwnerID {
		return ErrNotFound
	}

	now := rec.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"doc_id":              rec.DocID,
		"title":               rec.Title,
		"current_revision_id": rec.CurrentRevisionID,
		"created_at":          existing.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":          now.UTC().Format(time.RFC3339Nano),
	}

	bodyJSON, _ := json.Marshal(rec)
	obj.Body = bodyJSON
	obj.Metadata = mustMarshalMetadata(metadata)
	obj.UpdatedAt = now
	return s.ogStore.PutObject(ctx, obj)
}

// =========================================================================
// Texture Revisions — object graph implementation
// =========================================================================

// CreateTextureRevisionOG creates a texture revision in the object graph.
// Revisions use external-key identity (revision_id).
func (s *Store) CreateTextureRevisionOG(ctx context.Context, rec types.Revision) error {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"revision_id":        rec.RevisionID,
		"doc_id":             rec.DocID,
		"author_kind":        string(rec.AuthorKind),
		"author_label":       rec.AuthorLabel,
		"version_number":     rec.VersionNumber,
		"parent_revision_id": rec.ParentRevisionID,
		"revision_hash":      rec.RevisionHash,
		"created_at":         rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	}

	obj, err := s.ogPut(ctx, ogKindTexRev, rec.OwnerID, rec.RevisionID, rec, metadata, now)
	if err != nil {
		return err
	}

	// Write edge from revision to document.
	if rec.DocID != "" {
		docSuffix := objectgraph.StableSuffixFromKey(rec.DocID)
		docID, _ := objectgraph.BuildCanonicalID(ogKindTexDoc, rec.OwnerID, docSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, docID, ogEdgeDocRevision, nil)
	}
	// Write edge to parent revision.
	if rec.ParentRevisionID != "" {
		parentSuffix := objectgraph.StableSuffixFromKey(rec.ParentRevisionID)
		parentID, _ := objectgraph.BuildCanonicalID(ogKindTexRev, rec.OwnerID, parentSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, parentID, ogEdgeRevParent, nil)
	}
	return nil
}

// GetTextureRevisionOG retrieves a texture revision by ID.
func (s *Store) GetTextureRevisionOG(ctx context.Context, ownerID, revisionID string) (types.Revision, error) {
	obj, err := s.ogGetByKey(ctx, ogKindTexRev, "revision_id", revisionID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return types.Revision{}, ErrNotFound
		}
		return types.Revision{}, err
	}
	var rec types.Revision
	if err := ogDecode(obj, &rec); err != nil {
		return types.Revision{}, err
	}
	if rec.OwnerID != ownerID {
		return types.Revision{}, ErrNotFound
	}
	return rec, nil
}

// ListTextureRevisionsByDocOG lists revisions for a document.
func (s *Store) ListTextureRevisionsByDocOG(ctx context.Context, ownerID, docID string, limit int) ([]types.Revision, error) {
	if limit <= 0 {
		limit = 1000
	}
	objs, err := s.ogListByMetadata(ctx, ogKindTexRev, "doc_id", docID, limit)
	if err != nil {
		return nil, err
	}
	revisions := make([]types.Revision, 0, len(objs))
	for _, obj := range objs {
		var rec types.Revision
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		revisions = append(revisions, rec)
	}
	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].VersionNumber > revisions[j].VersionNumber
	})
	return revisions, nil
}

// =========================================================================
// Texture Decisions — object graph implementation
// =========================================================================

// CreateTextureDecisionOG creates a texture decision in the object graph.
// Decisions use external-key identity (decision_id).
func (s *Store) CreateTextureDecisionOG(ctx context.Context, rec types.TextureDecisionRecord) error {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"decision_id":   rec.DecisionID,
		"doc_id":        rec.DocID,
		"run_id":        rec.RunID,
		"trajectory_id": rec.TrajectoryID,
		"actor_id":      rec.ActorID,
		"decision_kind": rec.DecisionKind,
		"created_at":    rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	}

	obj, err := s.ogPut(ctx, ogKindTexDecision, rec.OwnerID, rec.DecisionID, rec, metadata, now)
	if err != nil {
		return err
	}

	// Write edge to document.
	if rec.DocID != "" {
		docSuffix := objectgraph.StableSuffixFromKey(rec.DocID)
		docID, _ := objectgraph.BuildCanonicalID(ogKindTexDoc, rec.OwnerID, docSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, docID, ogEdgeDecisionDoc, nil)
	}
	// Write edge to run.
	if rec.RunID != "" {
		runSuffix := objectgraph.StableSuffixFromKey(rec.RunID)
		runID, _ := objectgraph.BuildCanonicalID(ogKindRun, rec.OwnerID, runSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, runID, ogEdgeDecisionRun, nil)
	}
	return nil
}

// ListTextureDecisionsByDocOG lists decisions for a document.
func (s *Store) ListTextureDecisionsByDocOG(ctx context.Context, ownerID, docID string, limit int) ([]types.TextureDecisionRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.ogListByMetadata(ctx, ogKindTexDecision, "doc_id", docID, limit)
	if err != nil {
		return nil, err
	}
	decisions := make([]types.TextureDecisionRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.TextureDecisionRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		decisions = append(decisions, rec)
	}
	return decisions, nil
}

// ListTextureDecisionsByTrajectoryOG lists decisions for a trajectory.
func (s *Store) ListTextureDecisionsByTrajectoryOG(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.TextureDecisionRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.ogListByMetadata(ctx, ogKindTexDecision, "trajectory_id", trajectoryID, limit)
	if err != nil {
		return nil, err
	}
	decisions := make([]types.TextureDecisionRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.TextureDecisionRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		decisions = append(decisions, rec)
	}
	return decisions, nil
}

// =========================================================================
// Evidence — object graph implementation
// =========================================================================

// CreateEvidenceOG stores an evidence record in the object graph.
// Evidence uses external-key identity (evidence_id).
func (s *Store) CreateEvidenceOG(ctx context.Context, rec types.EvidenceRecord) error {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"evidence_id": rec.EvidenceID,
		"agent_id":    rec.AgentID,
		"kind":        rec.Kind,
		"source_uri":  rec.SourceURI,
		"title":       rec.Title,
		"created_at":  rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	}

	obj, err := s.ogPut(ctx, ogKindEvidence, rec.OwnerID, rec.EvidenceID, rec, metadata, now)
	if err != nil {
		return err
	}

	// Write edge to agent.
	if rec.AgentID != "" {
		agentSuffix := objectgraph.StableSuffixFromKey(rec.AgentID)
		agentID, _ := objectgraph.BuildCanonicalID(ogKindAgent, rec.OwnerID, agentSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, agentID, ogEdgeEvidenceAgent, nil)
	}
	return nil
}

// GetEvidenceOG retrieves an evidence record by ID.
func (s *Store) GetEvidenceOG(ctx context.Context, ownerID, evidenceID string) (types.EvidenceRecord, error) {
	obj, err := s.ogGetByKey(ctx, ogKindEvidence, "evidence_id", evidenceID)
	if err != nil {
		return types.EvidenceRecord{}, err
	}
	var rec types.EvidenceRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.EvidenceRecord{}, err
	}
	if rec.OwnerID != ownerID {
		return types.EvidenceRecord{}, ErrNotFound
	}
	return rec, nil
}

// ListEvidenceByAgentOG lists evidence for an agent.
func (s *Store) ListEvidenceByAgentOG(ctx context.Context, ownerID, agentID string, limit int) ([]types.EvidenceRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.ogListByMetadata(ctx, ogKindEvidence, "agent_id", agentID, limit)
	if err != nil {
		return nil, err
	}
	records := make([]types.EvidenceRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.EvidenceRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID {
			continue
		}
		records = append(records, rec)
	}
	return records, nil
}

// =========================================================================
// Content Items — object graph implementation
// =========================================================================

// CreateContentItemOG stores a content item in the object graph.
// Content items use external-key identity (content_id).
func (s *Store) CreateContentItemOG(ctx context.Context, rec types.ContentItem) error {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"content_id":    rec.ContentID,
		"source_type":   rec.SourceType,
		"media_type":    rec.MediaType,
		"app_hint":      rec.AppHint,
		"title":         rec.Title,
		"source_url":    rec.SourceURL,
		"canonical_url": rec.CanonicalURL,
		"content_hash":  rec.ContentHash,
		"created_at":    rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":    rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}

	_, err := s.ogPut(ctx, ogKindContentItem, rec.OwnerID, rec.ContentID, rec, metadata, now)
	return err
}

// GetContentItemOG retrieves a content item by ID.
func (s *Store) GetContentItemOG(ctx context.Context, ownerID, contentID string) (types.ContentItem, error) {
	obj, err := s.ogGetByKey(ctx, ogKindContentItem, "content_id", contentID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return types.ContentItem{}, ErrNotFound
		}
		return types.ContentItem{}, err
	}
	var rec types.ContentItem
	if err := ogDecode(obj, &rec); err != nil {
		return types.ContentItem{}, err
	}
	if rec.OwnerID != ownerID {
		return types.ContentItem{}, ErrNotFound
	}
	return rec, nil
}

// ListContentItemsByOwnerOG lists content items by owner.
func (s *Store) ListContentItemsByOwnerOG(ctx context.Context, ownerID string, limit int) ([]types.ContentItem, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:    ogKindContentItem,
		OwnerID: ownerID,
		Limit:   limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.ContentItem, 0, len(objs))
	for _, obj := range objs {
		var rec types.ContentItem
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		items = append(items, rec)
	}
	return items, nil
}

// =========================================================================
// Podcast Subscriptions — object graph implementation
// =========================================================================

// CreatePodcastSubscriptionOG stores a podcast subscription.
// Subscriptions use external-key identity (subscription_id).
func (s *Store) CreatePodcastSubscriptionOG(ctx context.Context, rec types.PodcastSubscription) error {
	// Use UpdatedAt for the OG object timestamp so that list queries
	// ordering by updated_at DESC reflect the most recent refresh, not
	// just the creation time.
	now := rec.UpdatedAt
	if now.IsZero() {
		now = rec.CreatedAt
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"subscription_id": rec.SubscriptionID,
		"feed_url":        rec.FeedURL,
		"content_id":      rec.ContentID,
		"title":           rec.Title,
		"author":          rec.Author,
		"artwork_url":     rec.ArtworkURL,
		"created_at":      rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":      rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if !rec.LastFetchedAt.IsZero() {
		metadata["last_fetched_at"] = rec.LastFetchedAt.UTC().Format(time.RFC3339Nano)
	}

	obj, err := s.ogPut(ctx, ogKindPodcastSub, rec.OwnerID, rec.SubscriptionID, rec, metadata, now)
	if err != nil {
		return err
	}

	// Write edge to content item if set.
	if rec.ContentID != "" {
		contentSuffix := objectgraph.StableSuffixFromKey(rec.ContentID)
		contentID, _ := objectgraph.BuildCanonicalID(ogKindContentItem, rec.OwnerID, contentSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, contentID, ogEdgeSubContent, nil)
	}
	return nil
}

// GetPodcastSubscriptionOG retrieves a podcast subscription by ID.
func (s *Store) GetPodcastSubscriptionOG(ctx context.Context, ownerID, subscriptionID string) (types.PodcastSubscription, error) {
	obj, err := s.ogGetByKey(ctx, ogKindPodcastSub, "subscription_id", subscriptionID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return types.PodcastSubscription{}, ErrNotFound
		}
		return types.PodcastSubscription{}, err
	}
	var rec types.PodcastSubscription
	if err := ogDecode(obj, &rec); err != nil {
		return types.PodcastSubscription{}, err
	}
	if rec.OwnerID != ownerID {
		return types.PodcastSubscription{}, ErrNotFound
	}
	return rec, nil
}

// ListPodcastSubscriptionsByOwnerOG lists subscriptions by owner.
func (s *Store) ListPodcastSubscriptionsByOwnerOG(ctx context.Context, ownerID string, limit int) ([]types.PodcastSubscription, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:    ogKindPodcastSub,
		OwnerID: ownerID,
		Limit:   limit,
	})
	if err != nil {
		return nil, err
	}
	subs := make([]types.PodcastSubscription, 0, len(objs))
	for _, obj := range objs {
		var rec types.PodcastSubscription
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		subs = append(subs, rec)
	}
	return subs, nil
}

// =========================================================================
// Browser Sessions — object graph implementation
// =========================================================================

// CreateBrowserSessionOG stores a browser session record.
// Sessions use external-key identity (session_id).
func (s *Store) CreateBrowserSessionOG(ctx context.Context, rec types.BrowserSessionRecord) error {
	// Use UpdatedAt for the OG object timestamp so that list queries
	// ordering by updated_at DESC reflect the most recent session activity,
	// not just the creation time.
	now := rec.UpdatedAt
	if now.IsZero() {
		now = rec.CreatedAt
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"session_id":         rec.SessionID,
		"provider":           rec.Provider,
		"mode":               rec.Mode,
		"execution_scope":    rec.ExecutionScope,
		"backend_session_id": rec.BackendSessionID,
		"world_kind":         rec.WorldKind,
		"vm_id":              rec.VMID,
		"snapshot_id":        rec.SnapshotID,
		"source_run_id":      rec.SourceRunID,
		"candidate_trace_id": rec.CandidateTraceID,
		"state":              rec.State,
		"current_url":        rec.CurrentURL,
		"title":              rec.Title,
		"created_at":         rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":         rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}

	obj, err := s.ogPut(ctx, ogKindBrowserSess, rec.OwnerID, rec.SessionID, rec, metadata, now)
	if err != nil {
		return err
	}

	// Write edge to source run if set.
	if rec.SourceRunID != "" {
		runSuffix := objectgraph.StableSuffixFromKey(rec.SourceRunID)
		runID, _ := objectgraph.BuildCanonicalID(ogKindRun, rec.OwnerID, runSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, runID, ogEdgeBrowserRun, nil)
	}
	return nil
}

// GetBrowserSessionOG retrieves a browser session by ID.
func (s *Store) GetBrowserSessionOG(ctx context.Context, ownerID, sessionID string) (types.BrowserSessionRecord, error) {
	obj, err := s.ogGetByKey(ctx, ogKindBrowserSess, "session_id", sessionID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return types.BrowserSessionRecord{}, ErrNotFound
		}
		return types.BrowserSessionRecord{}, err
	}
	var rec types.BrowserSessionRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.BrowserSessionRecord{}, err
	}
	if rec.OwnerID != ownerID {
		return types.BrowserSessionRecord{}, ErrNotFound
	}
	return rec, nil
}

// ListBrowserSessionsByOwnerOG lists browser sessions by owner.
func (s *Store) ListBrowserSessionsByOwnerOG(ctx context.Context, ownerID string, limit int) ([]types.BrowserSessionRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:    ogKindBrowserSess,
		OwnerID: ownerID,
		Limit:   limit,
	})
	if err != nil {
		return nil, err
	}
	sessions := make([]types.BrowserSessionRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.BrowserSessionRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		sessions = append(sessions, rec)
	}
	return sessions, nil
}

// =========================================================================
// App Change Packages — object graph implementation
// =========================================================================

// CreateAppChangePackageOG stores an app change package.
// Packages use external-key identity (package_id).
func (s *Store) CreateAppChangePackageOG(ctx context.Context, rec types.AppChangePackageRecord) error {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"package_id":              rec.PackageID,
		"app_id":                  rec.AppID,
		"status":                  string(rec.Status),
		"visibility":              rec.Visibility,
		"source_computer_id":      rec.SourceComputerID,
		"source_candidate_id":     rec.SourceCandidateID,
		"source_active_ref":       rec.SourceActiveRef,
		"candidate_source_ref":    rec.CandidateSourceRef,
		"package_manifest_sha256": rec.PackageManifestSHA256,
		"trace_id":                rec.TraceID,
		"created_at":              rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":              rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}

	_, err := s.ogPut(ctx, ogKindAppPackage, rec.OwnerID, rec.PackageID, rec, metadata, now)
	return err
}

// GetAppChangePackageOG retrieves an app change package by ID.
func (s *Store) GetAppChangePackageOG(ctx context.Context, ownerID, packageID string) (types.AppChangePackageRecord, error) {
	obj, err := s.ogGetByKey(ctx, ogKindAppPackage, "package_id", packageID)
	if err != nil {
		return types.AppChangePackageRecord{}, err
	}
	var rec types.AppChangePackageRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.AppChangePackageRecord{}, err
	}
	if rec.OwnerID != ownerID {
		return types.AppChangePackageRecord{}, ErrNotFound
	}
	return rec, nil
}

// ListAppChangePackagesByOwnerOG lists packages by owner.
func (s *Store) ListAppChangePackagesByOwnerOG(ctx context.Context, ownerID string, limit int) ([]types.AppChangePackageRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:    ogKindAppPackage,
		OwnerID: ownerID,
		Limit:   limit,
	})
	if err != nil {
		return nil, err
	}
	packages := make([]types.AppChangePackageRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.AppChangePackageRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		packages = append(packages, rec)
	}
	return packages, nil
}

// =========================================================================
// App Adoptions — object graph implementation
// =========================================================================

// CreateAppAdoptionOG stores an app adoption record.
// Adoptions use external-key identity (adoption_id).
func (s *Store) CreateAppAdoptionOG(ctx context.Context, rec types.AppAdoptionRecord) error {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"adoption_id":          rec.AdoptionID,
		"package_id":           rec.PackageID,
		"app_id":               rec.AppID,
		"target_computer_id":   rec.TargetComputerID,
		"target_computer_kind": rec.TargetComputerKind,
		"target_candidate_id":  rec.TargetCandidateID,
		"status":               string(rec.Status),
		"candidate_source_ref": rec.CandidateSourceRef,
		"route_profile":        rec.RouteProfile,
		"trace_id":             rec.TraceID,
		"created_at":           rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":           rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}

	obj, err := s.ogPut(ctx, ogKindAppAdoption, rec.OwnerID, rec.AdoptionID, rec, metadata, now)
	if err != nil {
		return err
	}

	// Write edge to package.
	if rec.PackageID != "" {
		pkgSuffix := objectgraph.StableSuffixFromKey(rec.PackageID)
		pkgID, _ := objectgraph.BuildCanonicalID(ogKindAppPackage, rec.OwnerID, pkgSuffix)
		_ = s.ogPutEdge(ctx, obj.CanonicalID, pkgID, ogEdgeAdoptionPkg, nil)
	}
	return nil
}

// GetAppAdoptionOG retrieves an app adoption by ID.
func (s *Store) GetAppAdoptionOG(ctx context.Context, ownerID, adoptionID string) (types.AppAdoptionRecord, error) {
	obj, err := s.ogGetByKey(ctx, ogKindAppAdoption, "adoption_id", adoptionID)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	var rec types.AppAdoptionRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.AppAdoptionRecord{}, err
	}
	if rec.OwnerID != ownerID {
		return types.AppAdoptionRecord{}, ErrNotFound
	}
	return rec, nil
}

// ListAppAdoptionsByOwnerOG lists adoptions by owner.
func (s *Store) ListAppAdoptionsByOwnerOG(ctx context.Context, ownerID string, limit int) ([]types.AppAdoptionRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:    ogKindAppAdoption,
		OwnerID: ownerID,
		Limit:   limit,
	})
	if err != nil {
		return nil, err
	}
	adoptions := make([]types.AppAdoptionRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.AppAdoptionRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		adoptions = append(adoptions, rec)
	}
	return adoptions, nil
}

// =========================================================================
// Desktop Sessions — object graph implementation
// =========================================================================

// SaveDesktopStateOG stores desktop state in the object graph.
// Desktop state uses external-key identity (desktop_id).
func (s *Store) SaveDesktopStateOG(ctx context.Context, state types.DesktopState) error {
	now := state.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	metadata := map[string]any{
		"desktop_id":       state.DesktopID,
		"active_window_id": state.ActiveWindowID,
		"updated_at":       now.UTC().Format(time.RFC3339Nano),
	}

	_, err := s.ogPut(ctx, ogKindDesktopSess, state.OwnerID, state.DesktopID, state, metadata, now)
	return err
}

// GetDesktopStateOG retrieves desktop state by owner + desktop ID.
func (s *Store) GetDesktopStateOG(ctx context.Context, ownerID, desktopID string) (types.DesktopState, error) {
	obj, err := s.ogGetByKey(ctx, ogKindDesktopSess, "desktop_id", desktopID)
	if err != nil {
		return types.DesktopState{}, err
	}
	var rec types.DesktopState
	if err := ogDecode(obj, &rec); err != nil {
		return types.DesktopState{}, err
	}
	if rec.OwnerID != ownerID {
		return types.DesktopState{}, ErrNotFound
	}
	return rec, nil
}

// =========================================================================
// Texture Source Graph — object graph implementation
// =========================================================================

// PutTextureSourceEntityOG stores a texture source entity in the object graph.
// Source entities are versioned by canonical_id + version_id, so the identity
// key combines both to produce a unique OG object per version.
func (s *Store) PutTextureSourceEntityOG(ctx context.Context, rec TextureSourceEntityGraphRecord) error {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	identityKey := rec.CanonicalID + "\x00" + rec.VersionID
	metadata := map[string]any{
		"canonical_id":       rec.CanonicalID,
		"version_id":         rec.VersionID,
		"entity_version_key": entityVersionKey(rec.CanonicalID, rec.VersionID),
		"created_at":         rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	_, err := s.ogPut(ctx, TextureSourceEntityObjectKind, rec.OwnerID, identityKey, rec, metadata, now)
	return err
}

// GetTextureSourceEntityOG retrieves a source entity by canonical_id + version_id.
func (s *Store) GetTextureSourceEntityOG(ctx context.Context, canonicalID, versionID string) (TextureSourceEntityGraphRecord, error) {
	obj, err := s.ogGetByKey(ctx, TextureSourceEntityObjectKind, "entity_version_key", entityVersionKey(canonicalID, versionID))
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return TextureSourceEntityGraphRecord{}, ErrNotFound
		}
		return TextureSourceEntityGraphRecord{}, err
	}
	var rec TextureSourceEntityGraphRecord
	if err := ogDecode(obj, &rec); err != nil {
		return TextureSourceEntityGraphRecord{}, err
	}
	return rec, nil
}

// ListTextureSourceEntitiesByOwnerOG lists all source entities for an owner.
// The choir.source_entity kind is shared with sourcecycled web captures, so
// we filter to only texture source entities by checking for the
// entity_version_key metadata field that texture source entities carry.
func (s *Store) ListTextureSourceEntitiesByOwnerOG(ctx context.Context, ownerID string, limit int) ([]TextureSourceEntityGraphRecord, error) {
	if limit <= 0 {
		limit = 500
	}
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:    TextureSourceEntityObjectKind,
		OwnerID: ownerID,
		Limit:   limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]TextureSourceEntityGraphRecord, 0, len(objs))
	for _, obj := range objs {
		// Skip non-texture source entities (e.g. sourcecycled web captures)
		// that share the choir.source_entity kind but don't carry the
		// entity_version_key metadata field.
		var meta map[string]any
		if err := json.Unmarshal(obj.Metadata, &meta); err != nil {
			return nil, fmt.Errorf("list texture source entities: parse metadata: %w", err)
		}
		if _, ok := meta["entity_version_key"]; !ok {
			continue
		}
		var rec TextureSourceEntityGraphRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

// TextureSourceEntityVersionExistsOG checks if a source entity version exists in OG.
func (s *Store) TextureSourceEntityVersionExistsOG(ctx context.Context, canonicalID, versionID string) (bool, error) {
	_, err := s.ogGetByKey(ctx, TextureSourceEntityObjectKind, "entity_version_key", entityVersionKey(canonicalID, versionID))
	if err == nil {
		return true, nil
	}
	if err == objectgraph.ErrNotFound {
		return false, nil
	}
	return false, err
}

// PutTextureSourceRefOG stores a texture source ref in the object graph.
func (s *Store) PutTextureSourceRefOG(ctx context.Context, rec TextureSourceRefGraphRecord) error {
	now := rec.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	identityKey := rec.CanonicalID + "\x00" + rec.VersionID
	metadata := map[string]any{
		"canonical_id":        rec.CanonicalID,
		"version_id":          rec.VersionID,
		"ref_version_key":     identityKey,
		"doc_id":              rec.DocID,
		"texture_revision_id": rec.TextureRevisionID,
		"created_at":          rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	_, err := s.ogPut(ctx, TextureSourceRefObjectKind, rec.OwnerID, identityKey, rec, metadata, now)
	return err
}

// TextureSourceRefVersionExistsOG checks if a source ref version exists in OG.
func (s *Store) TextureSourceRefVersionExistsOG(ctx context.Context, canonicalID, versionID string) (bool, error) {
	_, err := s.ogGetByKey(ctx, TextureSourceRefObjectKind, "ref_version_key", canonicalID+"\x00"+versionID)
	if err == nil {
		return true, nil
	}
	if err == objectgraph.ErrNotFound {
		return false, nil
	}
	return false, err
}

// ListTextureSourceRefsByRevisionOG lists source refs for a specific revision.
func (s *Store) ListTextureSourceRefsByRevisionOG(ctx context.Context, ownerID, docID, revisionID string, limit int) ([]TextureSourceRefGraphRecord, error) {
	if limit <= 0 {
		limit = 500
	}
	objs, err := s.ogListByMetadata(ctx, TextureSourceRefObjectKind, "texture_revision_id", revisionID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]TextureSourceRefGraphRecord, 0, len(objs))
	for _, obj := range objs {
		var rec TextureSourceRefGraphRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID || rec.DocID != docID {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

// mustMarshalMetadata converts a map to json.RawMessage, returning {} on
// error. Used internally where the metadata map is known to be valid.
func mustMarshalMetadata(m map[string]any) json.RawMessage {
	out, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(out)
}
