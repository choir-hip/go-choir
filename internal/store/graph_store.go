package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Object graph kind constants for VM store records.
const (
	ogKindAgent          = objectgraph.ObjectKind("choir.agent")
	ogKindRun            = objectgraph.ObjectKind("choir.run")
	ogKindEvent          = objectgraph.ObjectKind("choir.event")
	ogKindTrajectory     = objectgraph.ObjectKind("choir.trajectory")
	ogKindWorkItem       = objectgraph.ObjectKind("choir.work_item")
	ogKindChannelMsg     = objectgraph.ObjectKind("choir.channel_message")
	ogKindWorkerUpdate   = objectgraph.ObjectKind("choir.worker_update")
	ogKindLifecycleEvent = objectgraph.ObjectKind("choir.lifecycle_event")
	ogKindLifecycleCmd   = objectgraph.ObjectKind("choir.lifecycle_command")
	ogKindLifecycleSeq   = objectgraph.ObjectKind("choir.lifecycle_sequence")
	ogKindInboxDeliv     = objectgraph.ObjectKind("choir.inbox_delivery")
	ogKindRunMemory      = objectgraph.ObjectKind("choir.run_memory_entry")
	ogKindRunAccept      = objectgraph.ObjectKind("choir.run_acceptance")
	ogKindRunContin      = objectgraph.ObjectKind("choir.run_continuation")
	ogKindTexDoc         = objectgraph.ObjectKind("choir.texture_document")
	ogKindTexRev         = objectgraph.ObjectKind("choir.texture_revision")
	ogKindTexDecision    = objectgraph.ObjectKind("choir.texture_decision")
	ogKindEvidence       = objectgraph.ObjectKind("choir.agent_evidence")
	ogKindContentItem    = objectgraph.ObjectKind("choir.content_item")
	ogKindPodcastSub     = objectgraph.ObjectKind("choir.podcast_subscription")
	ogKindBrowserSess    = objectgraph.ObjectKind("choir.browser_session")
	ogKindCoagentMail    = objectgraph.ObjectKind("choir.coagent_mailbox")
	ogKindDesktopSess    = objectgraph.ObjectKind("choir.desktop_session")
	ogKindDesktopApp     = objectgraph.ObjectKind("choir.desktop_app_instance")
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

const ogMetadataPageSize = 256

// ogListAllByMetadata exhausts the canonical-ID keyset for one metadata
// equality query. Callers must apply any body-level filters only after every
// page has been collected.
func (s *Store) ogListAllByMetadata(ctx context.Context, kind objectgraph.ObjectKind, metadataField, value string) ([]objectgraph.Object, error) {
	return s.ogListAllByMetadataPageSize(ctx, kind, metadataField, value, ogMetadataPageSize)
}

func (s *Store) ogListAllByMetadataPageSize(ctx context.Context, kind objectgraph.ObjectKind, metadataField, value string, pageSize int) ([]objectgraph.Object, error) {
	store := s.ogReadStore
	if store == nil {
		store = s.ogStore
	}
	if store == nil {
		return nil, fmt.Errorf("store: object graph not initialized")
	}
	if pageSize <= 0 {
		pageSize = ogMetadataPageSize
	}

	var objects []objectgraph.Object
	afterCanonicalID := ""
	for {
		page, err := store.ListObjectsByMetadataPage(ctx, string(kind), "$."+metadataField, value, afterCanonicalID, pageSize)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			return objects, nil
		}
		nextCursor := page[len(page)-1].CanonicalID
		if nextCursor == "" || nextCursor <= afterCanonicalID {
			return nil, fmt.Errorf("store: metadata page did not advance canonical ID cursor")
		}
		objects = append(objects, page...)
		afterCanonicalID = nextCursor
	}
}

func (s *Store) ogListAllObjectsByKind(ctx context.Context, kind objectgraph.ObjectKind) ([]objectgraph.Object, error) {
	store := s.ogReadStore
	if store == nil {
		store = s.ogStore
	}
	if store == nil {
		return nil, fmt.Errorf("store: object graph not initialized")
	}
	var objects []objectgraph.Object
	afterCanonicalID := ""
	for {
		page, err := store.ListObjectsPage(ctx, string(kind), afterCanonicalID, ogMetadataPageSize)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			return objects, nil
		}
		nextCursor := page[len(page)-1].CanonicalID
		if nextCursor == "" || nextCursor <= afterCanonicalID {
			return nil, fmt.Errorf("store: object page did not advance canonical ID cursor")
		}
		objects = append(objects, page...)
		afterCanonicalID = nextCursor
	}
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

func scopedAgentIdentityKey(computerID, agentID string) string {
	return strings.TrimSpace(computerID) + "\x00" + strings.TrimSpace(agentID)
}

func scopedAgentCanonicalID(ownerID, computerID, agentID string) (string, error) {
	suffix := objectgraph.StableSuffixFromKey(scopedAgentIdentityKey(computerID, agentID))
	return objectgraph.BuildCanonicalID(ogKindAgent, strings.TrimSpace(ownerID), suffix)
}

// UpsertAgentOG stores or updates an agent record in the object graph.
// The agent_id is stored in metadata for point lookups.
func (s *Store) UpsertAgentOG(ctx context.Context, rec types.AgentRecord) error {
	rec.OwnerID = strings.TrimSpace(rec.OwnerID)
	rec.AgentID = strings.TrimSpace(rec.AgentID)
	rec.ComputerID = strings.TrimSpace(rec.ComputerID)
	rec.SandboxID = strings.TrimSpace(rec.SandboxID)
	if rec.ComputerID == "" {
		rec.ComputerID = rec.SandboxID
	}
	if rec.SandboxID == "" {
		rec.SandboxID = rec.ComputerID
	}
	if rec.OwnerID == "" || rec.ComputerID == "" || rec.AgentID == "" {
		return fmt.Errorf("store: upsert agent requires owner_id, computer_id, and agent_id")
	}
	if strings.HasPrefix(rec.AgentID, "texture:") || rec.LifecycleVersion > 0 || strings.TrimSpace(rec.ActiveRunID) != "" {
		return ErrLifecycleAuthorityRequired
	}
	if s.ogStore == nil {
		return fmt.Errorf("store: object graph not initialized")
	}
	canonicalID, err := scopedAgentCanonicalID(rec.OwnerID, rec.ComputerID, rec.AgentID)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 8; attempt++ {
		now := rec.UpdatedAt
		if now.IsZero() {
			now = time.Now().UTC()
		}
		candidate := rec
		candidate.UpdatedAt = now
		metadata := map[string]any{}
		condition := objectgraph.ObjectCondition{CanonicalID: canonicalID}
		createdAt := candidate.CreatedAt
		existingObj, getErr := s.ogStore.GetObject(ctx, canonicalID)
		switch {
		case getErr == nil:
			existing, decodeErr := decodeLifecycleObject[types.AgentRecord](existingObj)
			if decodeErr != nil {
				return decodeErr
			}
			if existing.LifecycleVersion > 0 {
				return ErrLifecycleAuthorityRequired
			}
			if strings.TrimSpace(existing.ActiveRunID) != "" &&
				(candidate.Profile != existing.Profile ||
					candidate.Role != existing.Role ||
					candidate.ChannelID != existing.ChannelID) {
				return ErrLifecycleAuthorityRequired
			}
			candidate.ActiveRunID = existing.ActiveRunID
			if candidate.CreatedAt.IsZero() {
				candidate.CreatedAt = existing.CreatedAt
			}
			createdAt = existingObj.CreatedAt
			if err := json.Unmarshal(existingObj.Metadata, &metadata); err != nil {
				return fmt.Errorf("store: decode agent metadata: %w", err)
			}
			condition = objectgraph.ObjectCondition{
				CanonicalID: canonicalID, Exists: true, ExpectedContentHash: existingObj.ContentHash,
			}
		case errors.Is(getErr, objectgraph.ErrNotFound):
			if candidate.CreatedAt.IsZero() {
				candidate.CreatedAt = now
			}
			createdAt = candidate.CreatedAt
		default:
			return getErr
		}
		metadata["agent_id"] = candidate.AgentID
		metadata["computer_id"] = candidate.ComputerID
		metadata["sandbox_id"] = candidate.SandboxID
		metadata["profile"] = candidate.Profile
		metadata["role"] = candidate.Role
		metadata["channel_id"] = candidate.ChannelID
		metadata["created_at"] = candidate.CreatedAt.UTC().Format(time.RFC3339Nano)
		metadata["updated_at"] = now.UTC().Format(time.RFC3339Nano)
		obj, buildErr := lifecycleObject(
			ogKindAgent, candidate.OwnerID, candidate.ComputerID, candidate.AgentID,
			candidate, metadata, createdAt, now,
		)
		if buildErr != nil {
			return buildErr
		}
		if err := s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{condition}, objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
			if errors.Is(err, objectgraph.ErrConflict) {
				continue
			}
			return err
		}
		return nil
	}
	return ErrConcurrentStateChange
}

// GetAgentByScopeOG retrieves an agent by its complete durable identity.
func (s *Store) GetAgentByScopeOG(ctx context.Context, ownerID, computerID, agentID string) (types.AgentRecord, error) {
	id, err := scopedAgentCanonicalID(ownerID, computerID, agentID)
	if err != nil {
		return types.AgentRecord{}, err
	}
	graph := s.ogReadStore
	if graph == nil {
		graph = s.ogStore
	}
	if graph == nil {
		return types.AgentRecord{}, fmt.Errorf("store: object graph not initialized")
	}
	obj, err := graph.GetObject(ctx, id)
	if err != nil {
		if errors.Is(err, objectgraph.ErrNotFound) {
			return types.AgentRecord{}, ErrNotFound
		}
		return types.AgentRecord{}, err
	}
	var rec types.AgentRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.AgentRecord{}, err
	}
	if rec.OwnerID != strings.TrimSpace(ownerID) || rec.ComputerID != strings.TrimSpace(computerID) || rec.AgentID != strings.TrimSpace(agentID) {
		return types.AgentRecord{}, ErrNotFound
	}
	return rec, nil
}

// ResolveLegacyAgentScopeOG resolves a pre-scoping mailbox identity only when
// every surviving agent and run witness names one owner on the current
// computer. It is a migration seam, not transition authority; ambiguity fails
// closed.
func (s *Store) ResolveLegacyAgentScopeOG(ctx context.Context, computerID, agentID string) (types.AgentRecord, error) {
	computerID, agentID = strings.TrimSpace(computerID), strings.TrimSpace(agentID)
	if computerID == "" || agentID == "" {
		return types.AgentRecord{}, fmt.Errorf("store: resolve legacy agent scope requires computer_id and agent_id")
	}
	graph := s.ogReadStore
	if graph == nil {
		graph = s.ogStore
	}
	if graph == nil {
		return types.AgentRecord{}, fmt.Errorf("store: object graph not initialized")
	}
	var match *types.AgentRecord
	after := ""
	for {
		page, err := graph.ListObjectsPage(ctx, string(ogKindAgent), after, 1000)
		if err != nil {
			return types.AgentRecord{}, err
		}
		for _, obj := range page {
			if obj.Tombstone || obj.ComputerID != computerID {
				continue
			}
			var candidate types.AgentRecord
			if err := ogDecode(obj, &candidate); err != nil {
				return types.AgentRecord{}, err
			}
			if strings.TrimSpace(candidate.AgentID) != agentID ||
				strings.TrimSpace(candidate.ComputerID) != computerID ||
				strings.TrimSpace(candidate.OwnerID) == "" {
				continue
			}
			if match != nil {
				return types.AgentRecord{}, fmt.Errorf("store: legacy agent scope is ambiguous for computer %q agent %q", computerID, agentID)
			}
			selected := candidate
			match = &selected
		}
		if len(page) < 1000 {
			break
		}
		next := page[len(page)-1].CanonicalID
		if next == "" || next <= after {
			return types.AgentRecord{}, fmt.Errorf("store: legacy agent scope pagination did not advance")
		}
		after = next
	}

	// Some pre-scoping computers retain actor mailboxes after their old agent
	// row has disappeared. Runs are independent durable owner/computer witnesses
	// for the executing agent. Every surviving witness must name one owner;
	// never prefer an agent row over a contradictory run.
	runObjects, err := s.ogListAllByMetadata(ctx, ogKindRun, "agent_id", agentID)
	if err != nil {
		return types.AgentRecord{}, err
	}
	ownerID := ""
	if match != nil {
		ownerID = strings.TrimSpace(match.OwnerID)
	}
	for _, obj := range runObjects {
		if obj.Tombstone {
			continue
		}
		var run types.RunRecord
		if err := ogDecode(obj, &run); err != nil {
			return types.AgentRecord{}, err
		}
		if strings.TrimSpace(run.AgentID) != agentID ||
			strings.TrimSpace(run.SandboxID) != computerID ||
			strings.TrimSpace(run.OwnerID) == "" {
			continue
		}
		candidateOwner := strings.TrimSpace(run.OwnerID)
		if ownerID != "" && ownerID != candidateOwner {
			return types.AgentRecord{}, fmt.Errorf("store: legacy agent scope is ambiguous for computer %q agent %q", computerID, agentID)
		}
		ownerID = candidateOwner
	}
	if match != nil {
		return *match, nil
	}
	if ownerID == "" {
		return types.AgentRecord{}, fmt.Errorf("store: legacy agent scope not found for computer %q agent %q: %w", computerID, agentID, ErrNotFound)
	}
	return types.AgentRecord{
		AgentID: agentID, OwnerID: ownerID, ComputerID: computerID, SandboxID: computerID,
	}, nil
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
	if rec.AgentID != "" && rec.SandboxID != "" {
		agentID, buildErr := scopedAgentCanonicalID(rec.OwnerID, rec.SandboxID, rec.AgentID)
		if buildErr == nil {
			_ = s.ogPutEdge(ctx, obj.CanonicalID, agentID, ogEdgeRunAgent, nil)
		}
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

func (s *Store) getRunObjectByOwnerOG(ctx context.Context, ownerID, runID string) (objectgraph.Object, error) {
	suffix := objectgraph.StableSuffixFromKey(runID)
	canonicalID, err := objectgraph.BuildCanonicalID(ogKindRun, ownerID, suffix)
	if err != nil {
		return objectgraph.Object{}, err
	}
	graphStore := s.ogReadStore
	if graphStore == nil {
		graphStore = s.ogStore
	}
	if graphStore == nil {
		return objectgraph.Object{}, fmt.Errorf("store: object graph not initialized")
	}
	obj, err := graphStore.GetObject(ctx, canonicalID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return objectgraph.Object{}, ErrNotFound
		}
		return objectgraph.Object{}, err
	}
	return obj, nil
}

func lifecycleRunProjection(obj objectgraph.Object, _ types.RunRecord) bool {
	return strings.TrimSpace(obj.ComputerID) != ""
}

func (s *Store) getLegacyRunObjectByID(ctx context.Context, runID string) (objectgraph.Object, error) {
	objs, err := s.ogListAllByMetadata(ctx, ogKindRun, "run_id", strings.TrimSpace(runID))
	if err != nil {
		return objectgraph.Object{}, err
	}
	for _, obj := range objs {
		var rec types.RunRecord
		if err := ogDecode(obj, &rec); err != nil {
			return objectgraph.Object{}, err
		}
		if rec.RunID == strings.TrimSpace(runID) && !lifecycleRunProjection(obj, rec) {
			return obj, nil
		}
	}
	return objectgraph.Object{}, ErrNotFound
}

// GetRunByOwnerOG retrieves a run directly by its owner-scoped canonical ID.
func (s *Store) GetRunByOwnerOG(ctx context.Context, ownerID, runID string) (types.RunRecord, error) {
	obj, err := s.getRunObjectByOwnerOG(ctx, ownerID, runID)
	if err != nil {
		return types.RunRecord{}, err
	}
	var rec types.RunRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.RunRecord{}, err
	}
	return rec, nil
}

// GetRunOG retrieves a run by ID from the object graph.
func (s *Store) GetRunOG(ctx context.Context, runID string) (types.RunRecord, error) {
	obj, err := s.getLegacyRunObjectByID(ctx, runID)
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
	// Fetch the existing object to preserve identity and created_at. Legacy
	// callers without owner identity retain the global not-found behavior.
	var existing objectgraph.Object
	var err error
	if rec.OwnerID == "" {
		existing, err = s.getLegacyRunObjectByID(ctx, rec.RunID)
	} else {
		existing, err = s.getRunObjectByOwnerOG(ctx, rec.OwnerID, rec.RunID)
	}
	if err != nil {
		if err == ErrNotFound || err == objectgraph.ErrNotFound {
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
	objs, err := s.ogListAllObjectsByKind(ctx, ogKindRun)
	if err != nil {
		return nil, err
	}
	runs := make([]types.RunRecord, 0, min(limit, len(objs)))
	for _, obj := range objs {
		if obj.OwnerID != ownerID {
			continue
		}
		var rec types.RunRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if lifecycleRunProjection(obj, rec) {
			continue
		}
		runs = append(runs, rec)
	}
	sort.Slice(runs, func(i, j int) bool {
		if !runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].CreatedAt.After(runs[j].CreatedAt)
		}
		return runs[i].RunID < runs[j].RunID
	})
	if len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

// ListRunsByStateOG lists runs by state from the object graph.
// Uses metadata JSON_EXTRACT to filter by state.
func (s *Store) ListRunsByStateOG(ctx context.Context, state types.RunState, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	runs, err := s.ListAllRunsByStateOG(ctx, state)
	if err != nil {
		return nil, err
	}
	if len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

// ListAllRunsByStateOG exhausts the canonical-ID keyset for one run state.
func (s *Store) ListAllRunsByStateOG(ctx context.Context, state types.RunState) ([]types.RunRecord, error) {
	return s.listAllRunsByStateOG(ctx, state, ogMetadataPageSize)
}

func (s *Store) listAllRunsByStateOG(ctx context.Context, state types.RunState, pageSize int) ([]types.RunRecord, error) {
	objs, err := s.ogListAllByMetadataPageSize(ctx, ogKindRun, "state", string(state), pageSize)
	if err != nil {
		return nil, err
	}
	runs := make([]types.RunRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.RunRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if lifecycleRunProjection(obj, rec) {
			continue
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
	objs, err := s.ogListAllObjectsByKind(ctx, ogKindRun)
	if err != nil {
		return nil, err
	}
	runs := make([]types.RunRecord, 0, min(limit, len(objs)))
	for _, obj := range objs {
		var rec types.RunRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if lifecycleRunProjection(obj, rec) {
			continue
		}
		runs = append(runs, rec)
	}
	sort.Slice(runs, func(i, j int) bool {
		if !runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].CreatedAt.After(runs[j].CreatedAt)
		}
		return runs[i].RunID < runs[j].RunID
	})
	if len(runs) > limit {
		runs = runs[:limit]
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

func (s *Store) getTrajectoryObjectOG(ctx context.Context, ownerID, trajectoryID string) (objectgraph.Object, error) {
	suffix := objectgraph.StableSuffixFromKey(trajectoryID)
	canonicalID, err := objectgraph.BuildCanonicalID(ogKindTrajectory, ownerID, suffix)
	if err != nil {
		return objectgraph.Object{}, err
	}
	store := s.ogReadStore
	if store == nil {
		store = s.ogStore
	}
	if store == nil {
		return objectgraph.Object{}, fmt.Errorf("store: object graph not initialized")
	}
	obj, err := store.GetObject(ctx, canonicalID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return objectgraph.Object{}, ErrNotFound
		}
		return objectgraph.Object{}, err
	}
	return obj, nil
}

// CreateTrajectoryIfAbsentOG creates a trajectory if it doesn't exist.
// Returns the stored record (existing or newly created).
func (s *Store) CreateTrajectoryIfAbsentOG(ctx context.Context, rec types.TrajectoryRecord) (types.TrajectoryRecord, error) {
	if rec.LifecycleVersion > 0 {
		return types.TrajectoryRecord{}, ErrLifecycleAuthorityRequired
	}
	if exists, err := s.lifecycleTrajectoryExists(ctx, rec.OwnerID, rec.TrajectoryID); err != nil {
		return types.TrajectoryRecord{}, err
	} else if exists {
		return types.TrajectoryRecord{}, ErrLifecycleAuthorityRequired
	}
	existing, err := s.getTrajectoryObjectOG(ctx, rec.OwnerID, rec.TrajectoryID)
	if err == nil {
		var existingRec types.TrajectoryRecord
		if err := ogDecode(existing, &existingRec); err != nil {
			return types.TrajectoryRecord{}, err
		}
		return existingRec, nil
	}
	if err != ErrNotFound {
		return types.TrajectoryRecord{}, err
	}

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
	metadata := trajectoryMetadata(rec)

	_, err = s.ogPut(ctx, ogKindTrajectory, rec.OwnerID, rec.TrajectoryID, rec, metadata, now)
	if err != nil {
		return types.TrajectoryRecord{}, err
	}
	return rec, nil
}

// GetTrajectoryOG retrieves a trajectory by ID from the object graph.
func (s *Store) GetTrajectoryOG(ctx context.Context, ownerID, trajectoryID string) (types.TrajectoryRecord, error) {
	obj, err := s.getTrajectoryObjectOG(ctx, ownerID, trajectoryID)
	if err != nil {
		return types.TrajectoryRecord{}, err
	}
	var rec types.TrajectoryRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.TrajectoryRecord{}, err
	}
	if rec.LifecycleVersion > 0 {
		return types.TrajectoryRecord{}, ErrNotFound
	}
	return rec, nil
}

// ListTrajectoriesByOwnerOG lists trajectories by owner.
func (s *Store) ListTrajectoriesByOwnerOG(ctx context.Context, ownerID string, limit int) ([]types.TrajectoryRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.ogListAllObjectsByKind(ctx, ogKindTrajectory)
	if err != nil {
		return nil, err
	}
	sort.Slice(objs, func(i, j int) bool {
		if !objs[i].UpdatedAt.Equal(objs[j].UpdatedAt) {
			return objs[i].UpdatedAt.After(objs[j].UpdatedAt)
		}
		return objs[i].CanonicalID < objs[j].CanonicalID
	})
	trajs := make([]types.TrajectoryRecord, 0, len(objs))
	for _, obj := range objs {
		if obj.OwnerID != ownerID {
			continue
		}
		var rec types.TrajectoryRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.LifecycleVersion > 0 {
			continue
		}
		trajs = append(trajs, rec)
		if len(trajs) == limit {
			break
		}
	}
	return trajs, nil
}

// UpdateTrajectoryStatusOG updates the status of a trajectory.
func (s *Store) UpdateTrajectoryStatusOG(ctx context.Context, ownerID, trajectoryID string, status types.TrajectoryStatus) (types.TrajectoryRecord, error) {
	obj, err := s.getTrajectoryObjectOG(ctx, ownerID, trajectoryID)
	if err != nil {
		return types.TrajectoryRecord{}, err
	}
	var rec types.TrajectoryRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.TrajectoryRecord{}, err
	}
	if rec.LifecycleVersion > 0 {
		return rec, ErrLifecycleAuthorityRequired
	}
	if rec.Status == status {
		return rec, nil
	}
	if rec.Status != types.TrajectoryLive ||
		(status != types.TrajectorySettled && status != types.TrajectoryCancelled) {
		return rec, fmt.Errorf("update trajectory status: %w", ErrConcurrentStateChange)
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
	updated, err := trajectoryObject(rec, existing)
	if err != nil {
		return types.TrajectoryRecord{}, err
	}
	if err := s.ogStore.PutObject(ctx, updated); err != nil {
		return types.TrajectoryRecord{}, err
	}
	return rec, nil
}

func trajectoryMetadata(rec types.TrajectoryRecord) map[string]any {
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
	return metadata
}

func trajectoryObject(rec types.TrajectoryRecord, existing objectgraph.Object) (objectgraph.Object, error) {
	bodyJSON, err := json.Marshal(rec)
	if err != nil {
		return objectgraph.Object{}, fmt.Errorf("marshal trajectory: %w", err)
	}
	metadataJSON, err := json.Marshal(trajectoryMetadata(rec))
	if err != nil {
		return objectgraph.Object{}, fmt.Errorf("marshal trajectory metadata: %w", err)
	}
	existing.Body = bodyJSON
	existing.Metadata = metadataJSON
	existing.UpdatedAt = rec.UpdatedAt
	return existing, nil
}

// =========================================================================
// Work Items — object graph implementation
// =========================================================================

// CreateWorkItemOG creates a work item in the object graph.
func (s *Store) CreateWorkItemOG(ctx context.Context, rec types.WorkItemRecord) (types.WorkItemRecord, error) {
	if rec.WorkItemID == "" {
		return types.WorkItemRecord{}, fmt.Errorf("create work item: work_item_id is required")
	}
	if exists, err := s.lifecycleTrajectoryExists(ctx, rec.OwnerID, rec.TrajectoryID); err != nil {
		return types.WorkItemRecord{}, err
	} else if exists {
		return types.WorkItemRecord{}, ErrLifecycleAuthorityRequired
	}
	if rec.LifecycleVersion > 0 {
		return types.WorkItemRecord{}, ErrLifecycleAuthorityRequired
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
	metadata := workItemMetadata(rec)

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

func (s *Store) getWorkItemObjectOG(ctx context.Context, ownerID, workItemID string) (objectgraph.Object, error) {
	suffix := objectgraph.StableSuffixFromKey(workItemID)
	canonicalID, err := objectgraph.BuildCanonicalID(ogKindWorkItem, ownerID, suffix)
	if err != nil {
		return objectgraph.Object{}, err
	}
	store := s.ogReadStore
	if store == nil {
		store = s.ogStore
	}
	if store == nil {
		return objectgraph.Object{}, fmt.Errorf("store: object graph not initialized")
	}
	obj, err := store.GetObject(ctx, canonicalID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return objectgraph.Object{}, ErrNotFound
		}
		return objectgraph.Object{}, err
	}
	return obj, nil
}

// GetWorkItemOG retrieves a work item by ID.
func (s *Store) GetWorkItemOG(ctx context.Context, ownerID, workItemID string) (types.WorkItemRecord, error) {
	obj, err := s.getWorkItemObjectOG(ctx, ownerID, workItemID)
	if err != nil {
		return types.WorkItemRecord{}, err
	}
	var rec types.WorkItemRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.WorkItemRecord{}, err
	}
	if rec.LifecycleVersion > 0 {
		return types.WorkItemRecord{}, ErrNotFound
	}
	return rec, nil
}

// ListWorkItemsByTrajectoryOG lists work items for a trajectory.
func (s *Store) ListWorkItemsByTrajectoryOG(ctx context.Context, ownerID, trajectoryID string, openOnly bool) ([]types.WorkItemRecord, error) {
	objs, err := s.ogListAllByMetadata(ctx, ogKindWorkItem, "trajectory_id", trajectoryID)
	if err != nil {
		return nil, err
	}
	items := make([]types.WorkItemRecord, 0, len(objs))
	for _, obj := range objs {
		var rec types.WorkItemRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.LifecycleVersion > 0 {
			continue
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
	obj, err := s.getWorkItemObjectOG(ctx, ownerID, workItemID)
	if err != nil {
		return types.WorkItemRecord{}, err
	}
	var rec types.WorkItemRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.WorkItemRecord{}, err
	}
	if rec.LifecycleVersion > 0 {
		return rec, ErrLifecycleAuthorityRequired
	}
	if rec.Status == status {
		return rec, nil
	}
	if rec.Status != types.WorkItemOpen ||
		(status != types.WorkItemCompleted && status != types.WorkItemCancelled) {
		return rec, fmt.Errorf("update work item status: %w", ErrConcurrentStateChange)
	}

	rec.Status = status
	rec.UpdatedAt = time.Now().UTC()
	updated, err := workItemObject(rec, obj)
	if err != nil {
		return types.WorkItemRecord{}, err
	}
	if err := s.ogStore.PutObject(ctx, updated); err != nil {
		return types.WorkItemRecord{}, err
	}
	return rec, nil
}

func workItemMetadata(rec types.WorkItemRecord) map[string]any {
	return map[string]any{
		"work_item_id":          rec.WorkItemID,
		"trajectory_id":         rec.TrajectoryID,
		"status":                string(rec.Status),
		"assigned_agent_id":     rec.AssignedAgentID,
		"objective_fingerprint": rec.ObjectiveFingerprint,
		"created_by_run_id":     rec.CreatedByRunID,
		"created_at":            rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":            rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func workItemObject(rec types.WorkItemRecord, existing objectgraph.Object) (objectgraph.Object, error) {
	bodyJSON, err := json.Marshal(rec)
	if err != nil {
		return objectgraph.Object{}, fmt.Errorf("marshal work item: %w", err)
	}
	metadataJSON, err := json.Marshal(workItemMetadata(rec))
	if err != nil {
		return objectgraph.Object{}, fmt.Errorf("marshal work item metadata: %w", err)
	}
	existing.Body = bodyJSON
	existing.Metadata = metadataJSON
	existing.UpdatedAt = rec.UpdatedAt
	return existing, nil
}

func (s *Store) cancelTrajectoryAuthorityOG(ctx context.Context, ownerID, trajectoryID string) (types.TrajectoryRecord, error) {
	trajectoryObj, err := s.getTrajectoryObjectOG(ctx, ownerID, trajectoryID)
	if err != nil {
		return types.TrajectoryRecord{}, err
	}
	var trajectory types.TrajectoryRecord
	if err := ogDecode(trajectoryObj, &trajectory); err != nil {
		return types.TrajectoryRecord{}, err
	}
	if trajectory.LifecycleVersion > 0 {
		return trajectory, ErrLifecycleAuthorityRequired
	}
	if trajectory.Status == types.TrajectorySettled || trajectory.Status == types.TrajectoryCancelled {
		return trajectory, nil
	}
	if trajectory.Status != types.TrajectoryLive {
		return trajectory, fmt.Errorf("cancel trajectory authority: %w", ErrConcurrentStateChange)
	}

	objs, err := s.ogListAllByMetadata(ctx, ogKindWorkItem, "trajectory_id", trajectoryID)
	if err != nil {
		return types.TrajectoryRecord{}, fmt.Errorf("cancel trajectory authority: list work items: %w", err)
	}
	now := time.Now().UTC()
	batch := objectgraph.Batch{Objects: make([]objectgraph.Object, 0, len(objs)+1)}
	for _, obj := range objs {
		var item types.WorkItemRecord
		if err := ogDecode(obj, &item); err != nil {
			return types.TrajectoryRecord{}, err
		}
		if item.OwnerID != ownerID || item.TrajectoryID != trajectoryID || item.Status != types.WorkItemOpen {
			continue
		}
		item.Status = types.WorkItemCancelled
		item.UpdatedAt = now
		updated, err := workItemObject(item, obj)
		if err != nil {
			return types.TrajectoryRecord{}, err
		}
		batch.Objects = append(batch.Objects, updated)
	}

	trajectory.Status = types.TrajectoryCancelled
	trajectory.UpdatedAt = now
	updatedTrajectory, err := trajectoryObject(trajectory, trajectoryObj)
	if err != nil {
		return types.TrajectoryRecord{}, err
	}
	batch.Objects = append(batch.Objects, updatedTrajectory)
	if s.ogStore == nil {
		return types.TrajectoryRecord{}, fmt.Errorf("cancel trajectory authority: object graph not initialized")
	}
	if err := s.ogStore.PutBatch(ctx, batch); err != nil {
		return types.TrajectoryRecord{}, fmt.Errorf("cancel trajectory authority: %w", err)
	}
	return trajectory, nil
}

func (s *Store) ListOpenWorkItemsByKindOG(ctx context.Context, kind string, limit int) ([]types.WorkItemRecord, error) {
	objs, err := s.ogListAllByMetadata(ctx, ogKindWorkItem, "status", string(types.WorkItemOpen))
	if err != nil {
		return nil, fmt.Errorf("list open work items by kind: %w", err)
	}
	items := make([]types.WorkItemRecord, 0)
	for _, obj := range objs {
		var item types.WorkItemRecord
		if err := ogDecode(obj, &item); err != nil {
			return nil, err
		}
		itemKind, _ := item.Details["kind"].(string)
		if item.Status != types.WorkItemOpen || itemKind != kind {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].UpdatedAt.Before(items[j].UpdatedAt)
		}
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].WorkItemID < items[j].WorkItemID
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
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
	if rec.LifecycleVersion > 0 {
		return ErrLifecycleAuthorityRequired
	}
	if strings.TrimSpace(rec.ComputerID) != "" && strings.TrimSpace(rec.TrajectoryID) != "" {
		if _, err := s.GetLifecycleTrajectory(ctx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID); err == nil {
			return ErrLifecycleAuthorityRequired
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
	}
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
	if rec.SourceRunID != "" {
		metadata["source_run_id"] = rec.SourceRunID
	}
	if rec.SourceOutcomeSHA256 != "" {
		metadata["source_outcome_sha256"] = rec.SourceOutcomeSHA256
	}
	if rec.DeliveredToRunID != "" {
		metadata["delivered_to_run_id"] = rec.DeliveredToRunID
	}
	_, err := s.ogPut(ctx, objectgraph.ObjectKind("choir.worker_update"), rec.OwnerID, rec.UpdateID, rec, metadata, now)
	return err
}

// GetWorkerUpdateOG retrieves a worker update by ID, scoped to the given owner.
func (s *Store) GetWorkerUpdateOG(ctx context.Context, ownerID, updateID string) (types.CoagentSourcePacket, error) {
	// Exhaust matching update IDs before filtering by owner and lifecycle
	// authority. A limited pre-filter window can hide the valid legacy record.
	objs, err := s.ogListAllByMetadata(ctx, objectgraph.ObjectKind("choir.worker_update"), "update_id", updateID)
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
		if rec.LifecycleVersion > 0 {
			continue
		}
		return rec, nil
	}
	return types.CoagentSourcePacket{}, ErrNotFound
}

// ListWorkerUpdatesBySourceRunOG exhausts terminal-bound updates for one run.
func (s *Store) ListWorkerUpdatesBySourceRunOG(ctx context.Context, ownerID, sourceRunID string) ([]types.CoagentSourcePacket, error) {
	objs, err := s.ogListAllByMetadata(ctx, objectgraph.ObjectKind("choir.worker_update"), "source_run_id", sourceRunID)
	if err != nil {
		return nil, err
	}
	updates := make([]types.CoagentSourcePacket, 0, len(objs))
	for _, obj := range objs {
		var rec types.CoagentSourcePacket
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.LifecycleVersion > 0 {
			continue
		}
		if rec.OwnerID == ownerID && rec.SourceRunID == sourceRunID {
			updates = append(updates, rec)
		}
	}
	return updates, nil
}

// ListWorkerUpdatesByTrajectoryOG lists worker updates for a trajectory.
func (s *Store) ListWorkerUpdatesByTrajectoryOG(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.CoagentSourcePacket, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.ogListAllByMetadata(ctx, objectgraph.ObjectKind("choir.worker_update"), "trajectory_id", trajectoryID)
	if err != nil {
		return nil, err
	}
	packets := make([]types.CoagentSourcePacket, 0, len(objs))
	sort.Slice(objs, func(i, j int) bool {
		if !objs[i].UpdatedAt.Equal(objs[j].UpdatedAt) {
			return objs[i].UpdatedAt.After(objs[j].UpdatedAt)
		}
		return objs[i].CanonicalID < objs[j].CanonicalID
	})
	for _, obj := range objs {
		var rec types.CoagentSourcePacket
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.LifecycleVersion > 0 {
			continue
		}
		if rec.OwnerID != ownerID {
			continue
		}
		packets = append(packets, rec)
		if len(packets) == limit {
			break
		}
	}
	return packets, nil
}

// ListPendingWorkerUpdatesOG lists pending (undelivered) worker updates
// for a target agent.
func (s *Store) ListPendingWorkerUpdatesOG(ctx context.Context, ownerID, targetAgentID string, limit int) ([]types.CoagentSourcePacket, error) {
	if limit <= 0 {
		limit = 100
	}
	// Exhaust the target keyset before excluding lifecycle and delivered rows;
	// applying a fixed window first can mask an older valid legacy update.
	objs, err := s.ogListAllByMetadata(ctx, objectgraph.ObjectKind("choir.worker_update"), "target_agent_id", targetAgentID)
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
		if rec.LifecycleVersion > 0 {
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
	if strings.TrimSpace(rec.TrajectoryID) != "" {
		return ErrLifecycleAuthorityRequired
	}
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
	if strings.TrimSpace(rec.TrajectoryID) != "" {
		return types.Document{}, ErrLifecycleAuthorityRequired
	}
	return rec, nil
}

// ListTextureDocumentsByOwnerOG lists documents by owner.
func (s *Store) ListTextureDocumentsByOwnerOG(ctx context.Context, ownerID string, limit int) ([]types.Document, error) {
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.ogListAllObjectsByKind(ctx, ogKindTexDoc)
	if err != nil {
		return nil, err
	}
	docs := make([]types.Document, 0, len(objs))
	for _, obj := range objs {
		if obj.OwnerID != ownerID {
			continue
		}
		var rec types.Document
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if strings.TrimSpace(rec.TrajectoryID) != "" {
			continue
		}
		docs = append(docs, rec)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].UpdatedAt.After(docs[j].UpdatedAt) })
	if len(docs) > limit {
		docs = docs[:limit]
	}
	return docs, nil
}

// ListTextureDocumentsByScopeOG lists every document visible to one computer.
func (s *Store) ListTextureDocumentsByScopeOG(ctx context.Context, ownerID, computerID string, limit int) ([]types.Document, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return nil, fmt.Errorf("list texture documents by scope: owner_id and computer_id are required")
	}
	if limit <= 0 {
		limit = 100
	}
	objs, err := s.ogListAllObjectsByKind(ctx, ogKindTexDoc)
	if err != nil {
		return nil, err
	}
	docs := make([]types.Document, 0, len(objs))
	for _, obj := range objs {
		if obj.OwnerID != ownerID || strings.TrimSpace(obj.ComputerID) != computerID {
			continue
		}
		var rec types.Document
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if strings.TrimSpace(rec.ComputerID) == computerID {
			docs = append(docs, rec)
		}
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
	objs, err := s.ogListAllObjectsByKind(ctx, ogKindTexDoc)
	if err != nil {
		return nil, err
	}
	docs := make([]types.Document, 0, len(objs))
	for _, obj := range objs {
		var rec types.Document
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if strings.TrimSpace(rec.TrajectoryID) != "" {
			continue
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
	if strings.TrimSpace(existing.TrajectoryID) != "" {
		return ErrLifecycleAuthorityRequired
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
	if strings.TrimSpace(rec.TrajectoryID) != "" {
		return ErrLifecycleAuthorityRequired
	}
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
	if strings.TrimSpace(rec.ComputerID) != "" || strings.TrimSpace(rec.TrajectoryID) != "" {
		return types.Revision{}, ErrLifecycleAuthorityRequired
	}
	return rec, nil
}

// ListTextureRevisionsByDocOG lists revisions for a document.
func (s *Store) ListTextureRevisionsByDocOG(ctx context.Context, ownerID, docID string, limit int) ([]types.Revision, error) {
	if limit <= 0 {
		limit = 1000
	}
	objs, err := s.ogListAllByMetadata(ctx, ogKindTexRev, "doc_id", docID)
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
		if strings.TrimSpace(rec.ComputerID) != "" || strings.TrimSpace(rec.TrajectoryID) != "" {
			continue
		}
		revisions = append(revisions, rec)
	}
	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].VersionNumber > revisions[j].VersionNumber
	})
	if len(revisions) > limit {
		revisions = revisions[:limit]
	}
	return revisions, nil
}

// ListTextureRevisionsByScopeOG lists revisions visible to one computer.
func (s *Store) ListTextureRevisionsByScopeOG(ctx context.Context, ownerID, computerID, docID string, limit int) ([]types.Revision, error) {
	ownerID, computerID, docID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(docID)
	if ownerID == "" || computerID == "" || docID == "" {
		return nil, fmt.Errorf("list texture revisions by scope: owner_id, computer_id, and doc_id are required")
	}
	if limit <= 0 {
		limit = 1000
	}
	objs, err := s.ogListAllByMetadata(ctx, ogKindTexRev, "doc_id", docID)
	if err != nil {
		return nil, err
	}
	revisions := make([]types.Revision, 0, len(objs))
	for _, obj := range objs {
		if obj.OwnerID != ownerID || strings.TrimSpace(obj.ComputerID) != computerID {
			continue
		}
		var rec types.Revision
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if strings.TrimSpace(rec.ComputerID) == computerID && rec.DocID == docID {
			revisions = append(revisions, rec)
		}
	}
	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].VersionNumber > revisions[j].VersionNumber
	})
	if len(revisions) > limit {
		revisions = revisions[:limit]
	}
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
	if strings.TrimSpace(obj.ComputerID) != "" {
		return TextureSourceEntityGraphRecord{}, ErrNotFound
	}
	var rec TextureSourceEntityGraphRecord
	if err := ogDecode(obj, &rec); err != nil {
		return TextureSourceEntityGraphRecord{}, err
	}
	if strings.TrimSpace(rec.ComputerID) != "" {
		return TextureSourceEntityGraphRecord{}, ErrNotFound
	}
	return rec, nil
}

// ListTextureSourceEntitiesByOwnerOG lists all source entities for an owner.
// The choir.source_entity kind is shared with sourcecycled web captures, so
// we filter to only texture source entities by checking for the
// entity_version_key metadata field that texture source entities carry.
func (s *Store) ListTextureSourceEntitiesByOwnerOG(ctx context.Context, ownerID string, limit int) ([]TextureSourceEntityGraphRecord, error) {
	return s.ListTextureSourceEntitiesByScopeOG(ctx, ownerID, "", limit)
}

func (s *Store) ListTextureSourceEntitiesByScopeOG(ctx context.Context, ownerID, computerID string, limit int) ([]TextureSourceEntityGraphRecord, error) {
	if limit <= 0 {
		limit = 500
	}
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	objs, err := s.ogListAllObjectsByKind(ctx, TextureSourceEntityObjectKind)
	if err != nil {
		return nil, err
	}
	sort.Slice(objs, func(i, j int) bool {
		if !objs[i].UpdatedAt.Equal(objs[j].UpdatedAt) {
			return objs[i].UpdatedAt.After(objs[j].UpdatedAt)
		}
		return objs[i].CanonicalID < objs[j].CanonicalID
	})
	out := make([]TextureSourceEntityGraphRecord, 0, min(limit, len(objs)))
	for _, obj := range objs {
		if obj.OwnerID != ownerID || strings.TrimSpace(obj.ComputerID) != computerID {
			continue
		}
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
		if rec.OwnerID != ownerID || strings.TrimSpace(rec.ComputerID) != computerID {
			continue
		}
		out = append(out, rec)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// TextureSourceEntityVersionExistsOG checks if a source entity version exists in OG.
func (s *Store) TextureSourceEntityVersionExistsOG(ctx context.Context, canonicalID, versionID string) (bool, error) {
	obj, err := s.ogGetByKey(ctx, TextureSourceEntityObjectKind, "entity_version_key", entityVersionKey(canonicalID, versionID))
	if err == objectgraph.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(obj.ComputerID) != "" {
		return false, nil
	}
	var rec TextureSourceEntityGraphRecord
	if err := ogDecode(obj, &rec); err != nil {
		return false, err
	}
	return strings.TrimSpace(rec.ComputerID) == "", nil
}

// PutTextureSourceRefOG stores a texture source ref in the object graph.
func (s *Store) PutTextureSourceRefOG(ctx context.Context, rec TextureSourceRefGraphRecord) error {
	if strings.TrimSpace(rec.DocID) != "" {
		doc, err := s.GetTextureDocumentOG(ctx, rec.OwnerID, rec.DocID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(doc.TrajectoryID) != "" {
			return ErrLifecycleAuthorityRequired
		}
	}
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
	obj, err := s.ogGetByKey(ctx, TextureSourceRefObjectKind, "ref_version_key", canonicalID+"\x00"+versionID)
	if err == objectgraph.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(obj.ComputerID) != "" {
		return false, nil
	}
	var rec TextureSourceRefGraphRecord
	if err := ogDecode(obj, &rec); err != nil {
		return false, err
	}
	return strings.TrimSpace(rec.ComputerID) == "", nil
}

// ListTextureSourceRefsByRevisionOG lists source refs for a specific revision.
func (s *Store) ListTextureSourceRefsByRevisionOG(ctx context.Context, ownerID, docID, revisionID string, limit int) ([]TextureSourceRefGraphRecord, error) {
	return s.ListTextureSourceRefsByRevisionAndScopeOG(ctx, ownerID, "", docID, revisionID, limit)
}

func (s *Store) ListTextureSourceRefsByRevisionAndScopeOG(ctx context.Context, ownerID, computerID, docID, revisionID string, limit int) ([]TextureSourceRefGraphRecord, error) {
	if limit <= 0 {
		limit = 500
	}
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	objs, err := s.ogListAllByMetadata(ctx, TextureSourceRefObjectKind, "texture_revision_id", revisionID)
	if err != nil {
		return nil, err
	}
	out := make([]TextureSourceRefGraphRecord, 0, min(limit, len(objs)))
	for _, obj := range objs {
		var rec TextureSourceRefGraphRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID != ownerID || strings.TrimSpace(rec.ComputerID) != computerID || rec.DocID != docID {
			continue
		}
		out = append(out, rec)
		if len(out) >= limit {
			break
		}
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
