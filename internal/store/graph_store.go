package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Object graph kind constants for VM store records.
const (
	ogKindAgent       = objectgraph.ObjectKind("choir.agent")
	ogKindRun         = objectgraph.ObjectKind("choir.run")
	ogKindEvent       = objectgraph.ObjectKind("choir.event")
	ogKindTrajectory  = objectgraph.ObjectKind("choir.trajectory")
	ogKindWorkItem    = objectgraph.ObjectKind("choir.work_item")
	ogKindChannelMsg  = objectgraph.ObjectKind("choir.channel_message")
	ogKindInboxDeliv  = objectgraph.ObjectKind("choir.inbox_delivery")
	ogKindRunMemory   = objectgraph.ObjectKind("choir.run_memory_entry")
	ogKindRunAccept   = objectgraph.ObjectKind("choir.run_acceptance")
	ogKindRunContin   = objectgraph.ObjectKind("choir.run_continuation")
	ogKindTexDoc      = objectgraph.ObjectKind("choir.texture_document")
	ogKindTexRev      = objectgraph.ObjectKind("choir.texture_revision")
	ogKindTexDecision = objectgraph.ObjectKind("choir.texture_decision")
	ogKindEvidence    = objectgraph.ObjectKind("choir.agent_evidence")
	ogKindContentItem = objectgraph.ObjectKind("choir.content_item")
	ogKindPodcastSub  = objectgraph.ObjectKind("choir.podcast_subscription")
	ogKindBrowserSess = objectgraph.ObjectKind("choir.browser_session")
	ogKindAppPackage  = objectgraph.ObjectKind("choir.app_change_package")
	ogKindAppAdoption = objectgraph.ObjectKind("choir.app_adoption")
	ogKindDesktopSess = objectgraph.ObjectKind("choir.desktop_session")
	ogKindDesktopApp  = objectgraph.ObjectKind("choir.desktop_app_instance")
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
func (s *Store) ogGetByKey(ctx context.Context, kind objectgraph.ObjectKind, metadataField, value string) (objectgraph.Object, error) {
	if s.ogStore == nil {
		return objectgraph.Object{}, fmt.Errorf("store: object graph not initialized")
	}
	return s.ogStore.GetObjectByMetadata(ctx, string(kind), "$."+metadataField, value)
}

// ogListByMetadata lists objects by kind + a metadata field equality
// check, with an optional owner filter.
func (s *Store) ogListByMetadata(ctx context.Context, kind objectgraph.ObjectKind, metadataField, value string, limit int) ([]objectgraph.Object, error) {
	if s.ogStore == nil {
		return nil, fmt.Errorf("store: object graph not initialized")
	}
	return s.ogStore.ListObjectsByMetadata(ctx, string(kind), "$."+metadataField, value, limit)
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
	obj.Body = bodyJSON
	obj.Metadata = mustMarshalMetadata(metadata)
	obj.UpdatedAt = rec.UpdatedAt
	if err := s.ogStore.PutObject(ctx, obj); err != nil {
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
		"work_item_id":           rec.WorkItemID,
		"trajectory_id":          rec.TrajectoryID,
		"status":                 string(rec.Status),
		"assigned_agent_id":      rec.AssignedAgentID,
		"objective_fingerprint":  rec.ObjectiveFingerprint,
		"created_by_run_id":      rec.CreatedByRunID,
		"created_at":             rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":             rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
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
	objs, err := s.ogListByMetadata(ctx, ogKindWorkItem, "trajectory_id", trajectoryID, 500)
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
		"work_item_id":           rec.WorkItemID,
		"trajectory_id":          rec.TrajectoryID,
		"status":                 string(rec.Status),
		"assigned_agent_id":      rec.AssignedAgentID,
		"objective_fingerprint":  rec.ObjectiveFingerprint,
		"created_by_run_id":      rec.CreatedByRunID,
		"created_at":             rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":             rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
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

// mustMarshalMetadata converts a map to json.RawMessage, returning {} on
// error. Used internally where the metadata map is known to be valid.
func mustMarshalMetadata(m map[string]any) json.RawMessage {
	out, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(out)
}

