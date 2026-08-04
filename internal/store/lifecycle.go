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

var (
	ErrLifecycleCommandConflict   = errors.New("lifecycle command digest conflict")
	ErrLifecycleInvalidTransition = errors.New("lifecycle invalid transition")
	ErrLifecycleCursorExpired     = errors.New("lifecycle cursor expired")
)

func lifecycleScopedKey(computerID, key string) string {
	return strings.TrimSpace(computerID) + "\x00" + strings.TrimSpace(key)
}

func lifecycleDigest(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return objectgraph.SHA256(payload), nil
}

func ComputeStartLifecycleRequestDigest(req types.StartLifecycleRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.StartRequestDigest = "", "", ""
	req.InitialWork.OwnerID, req.InitialWork.ComputerID = "", ""
	req.InitialWork.CreatedByRunID = ""
	req.InitialWork.CreatedAt, req.InitialWork.UpdatedAt = time.Time{}, time.Time{}
	req.InitialDocument.OwnerID, req.InitialDocument.ComputerID = "", ""
	req.InitialDocument.CreatedAt, req.InitialDocument.UpdatedAt = time.Time{}, time.Time{}
	req.InitialRevision.OwnerID, req.InitialRevision.ComputerID = "", ""
	req.InitialRevision.CreatedAt = time.Time{}
	if len(req.InitialRevision.Metadata) != 0 {
		var metadata map[string]any
		if err := json.Unmarshal(req.InitialRevision.Metadata, &metadata); err != nil {
			return "", fmt.Errorf("start lifecycle revision metadata: %w", err)
		}
		delete(metadata, "conductor_loop_id")
		delete(metadata, "prompt_unix_ts")
		req.InitialRevision.Metadata, _ = json.Marshal(metadata)
	}
	req.Agent.OwnerID, req.Agent.ComputerID, req.Agent.SandboxID = "", "", ""
	req.Agent.CreatedAt, req.Agent.UpdatedAt = time.Time{}, time.Time{}
	return lifecycleDigest(req)
}

func normalizeApplyLifecycleRevisionDigest(revision *types.Revision) error {
	revision.OwnerID, revision.ComputerID, revision.CreatedAt = "", "", time.Time{}
	if len(revision.Provenance) == 0 {
		return nil
	}
	var provenance map[string]any
	if err := json.Unmarshal(revision.Provenance, &provenance); err != nil {
		return fmt.Errorf("apply lifecycle revision provenance: %w", err)
	}
	delete(provenance, "authored_at")
	normalized, err := json.Marshal(provenance)
	if err != nil {
		return fmt.Errorf("normalize apply lifecycle revision provenance: %w", err)
	}
	revision.Provenance = normalized
	return nil
}

func normalizeApplyLifecycleSourceGraphDigest(graph TextureSourceGraphWriteSet) TextureSourceGraphWriteSet {
	normalized := TextureSourceGraphWriteSet{
		SourceEntities: append([]TextureSourceEntityGraphRecord(nil), graph.SourceEntities...),
		SourceRefs:     append([]TextureSourceRefGraphRecord(nil), graph.SourceRefs...),
	}
	for i := range normalized.SourceEntities {
		normalized.SourceEntities[i].OwnerID = ""
		normalized.SourceEntities[i].ComputerID = ""
		normalized.SourceEntities[i].CreatedAt = time.Time{}
	}
	for i := range normalized.SourceRefs {
		normalized.SourceRefs[i].OwnerID = ""
		normalized.SourceRefs[i].ComputerID = ""
		normalized.SourceRefs[i].CreatedAt = time.Time{}
	}
	return normalized
}

func normalizeApplyLifecycleDigestRequest(req types.ApplyLifecycleUpdateRequest) (types.ApplyLifecycleUpdateRequest, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	req.UpdateID, req.ChannelID, req.Role, req.SourceRunID, req.Content = "", "", "", "", ""
	req.MessageSeq = 0
	req.Packet = types.CoagentSourcePacketPayload{}
	if err := normalizeApplyLifecycleRevisionDigest(&req.Revision); err != nil {
		return req, err
	}
	req.WorkItemID = strings.TrimSpace(req.WorkItemID)
	var err error
	if req.WorkDisposition, err = normalizeUpdateWorkDisposition(req.WorkDisposition); err != nil {
		return req, err
	}
	req.RelatedUpdates = append([]types.ApplyLifecycleRelatedUpdate(nil), req.RelatedUpdates...)
	for i := range req.RelatedUpdates {
		related := &req.RelatedUpdates[i]
		related.TargetAgentID = strings.TrimSpace(related.TargetAgentID)
		related.ProducerAgentID = strings.TrimSpace(related.ProducerAgentID)
		related.ProducerUpdateID = strings.TrimSpace(related.ProducerUpdateID)
		related.UpdateID = strings.TrimSpace(related.UpdateID)
		related.UpdateID = ""
		related.DispositionRef = strings.TrimSpace(related.DispositionRef)
		related.WorkItemID = strings.TrimSpace(related.WorkItemID)
		related.WorkResultRef = strings.TrimSpace(related.WorkResultRef)
		related.Reason = strings.TrimSpace(related.Reason)
		if related.WorkDisposition, err = normalizeUpdateWorkDisposition(related.WorkDisposition); err != nil {
			return req, err
		}
	}
	sort.Slice(req.RelatedUpdates, func(i, j int) bool {
		left := req.RelatedUpdates[i].TargetAgentID + "\x00" + req.RelatedUpdates[i].ProducerAgentID + "\x00" + req.RelatedUpdates[i].ProducerUpdateID
		right := req.RelatedUpdates[j].TargetAgentID + "\x00" + req.RelatedUpdates[j].ProducerAgentID + "\x00" + req.RelatedUpdates[j].ProducerUpdateID
		return left < right
	})
	return req, nil
}

func ComputeApplyLifecycleUpdateDigest(req types.ApplyLifecycleUpdateRequest) (string, error) {
	req, err := normalizeApplyLifecycleDigestRequest(req)
	if err != nil {
		return "", err
	}
	return lifecycleDigest(req)
}

func ComputeApplyLifecycleUpdateWithSourceGraphDigest(req types.ApplyLifecycleUpdateRequest, graph TextureSourceGraphWriteSet) (string, error) {
	req, err := normalizeApplyLifecycleDigestRequest(req)
	if err != nil {
		return "", err
	}
	return lifecycleDigest(struct {
		Request types.ApplyLifecycleUpdateRequest `json:"request"`
		Graph   TextureSourceGraphWriteSet        `json:"source_graph"`
	}{Request: req, Graph: normalizeApplyLifecycleSourceGraphDigest(graph)})
}

func ComputeCommitLifecycleArtifactHeadDigest(req types.CommitLifecycleArtifactHeadRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	req.Revision.OwnerID, req.Revision.ComputerID, req.Revision.CreatedAt = "", "", time.Time{}
	return lifecycleDigest(req)
}

func ComputeOpenLifecycleWorkDigest(req types.OpenLifecycleWorkRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	return lifecycleDigest(req)
}

func ComputeAmendLifecycleWorkDigest(req types.AmendLifecycleWorkRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	return lifecycleDigest(req)
}

func ComputeReplaceLifecycleActivationDigest(req types.ReplaceLifecycleActivationRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	return lifecycleDigest(req)
}

func ComputeRecordLifecycleRefsDigest(req types.RecordLifecycleRefsRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	req.ArtifactRefs, req.EvidenceRefs = normalizeLifecycleRefs(req.ArtifactRefs), normalizeLifecycleRefs(req.EvidenceRefs)
	return lifecycleDigest(req)
}

func ComputeQueueLifecycleUpdateDigest(req types.QueueLifecycleUpdateRequest) (string, error) {
	workDisposition, err := normalizeUpdateWorkDisposition(req.WorkDisposition)
	if err != nil {
		return "", err
	}
	return lifecycleDigest(struct {
		CommandID, TrajectoryID, TargetAgentID, ProducerAgentID string
		ProducerUpdateID, PayloadDigest, WorkItemID             string
		WorkDisposition                                         types.WorkItemStatus
	}{
		CommandID: strings.TrimSpace(req.CommandID), TrajectoryID: strings.TrimSpace(req.TrajectoryID),
		TargetAgentID: strings.TrimSpace(req.TargetAgentID), ProducerAgentID: strings.TrimSpace(req.ProducerAgentID),
		ProducerUpdateID: strings.TrimSpace(req.ProducerUpdateID), PayloadDigest: strings.TrimSpace(req.PayloadDigest),
		WorkItemID: strings.TrimSpace(req.WorkItemID), WorkDisposition: workDisposition,
	})
}

func ComputeSettleLifecycleWorkDigest(req types.SettleLifecycleWorkRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	req.ActingAgentID = strings.TrimSpace(req.ActingAgentID)
	return lifecycleDigest(req)
}

func ComputeRefuseLifecycleWorkDigest(req types.RefuseLifecycleWorkRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	req.ActingAgentID = strings.TrimSpace(req.ActingAgentID)
	return lifecycleDigest(req)
}
func ComputeSettleLifecycleTrajectoryDigest(req types.SettleLifecycleTrajectoryRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	return lifecycleDigest(req)
}

func ComputeCancelLifecycleDigest(req types.CancelLifecycleRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	return lifecycleDigest(req)
}

func ComputeArchiveLifecycleArtifactDigest(req types.ArchiveLifecycleArtifactRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	return lifecycleDigest(req)
}

func ComputeLifecycleUpdatePayloadDigest(packet types.CoagentSourcePacketPayload, content string) (string, error) {
	return lifecycleDigest(struct {
		Packet  types.CoagentSourcePacketPayload `json:"packet"`
		Content string                           `json:"content"`
	}{Packet: packet, Content: content})
}

func lifecycleCanonicalID(kind objectgraph.ObjectKind, ownerID, computerID, key string) (string, error) {
	return objectgraph.BuildCanonicalID(kind, strings.TrimSpace(ownerID), objectgraph.StableSuffixFromKey(lifecycleScopedKey(computerID, key)))
}

func lifecycleObject(kind objectgraph.ObjectKind, ownerID, computerID, key string, body any, metadata map[string]any, createdAt, updatedAt time.Time) (objectgraph.Object, error) {
	canonicalID, err := lifecycleCanonicalID(kind, ownerID, computerID, key)
	if err != nil {
		return objectgraph.Object{}, err
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return objectgraph.Object{}, fmt.Errorf("marshal %s: %w", kind, err)
	}
	metadataJSON, err := objectgraph.NormalizeMetadata(metadata)
	if err != nil {
		return objectgraph.Object{}, fmt.Errorf("metadata %s: %w", kind, err)
	}
	if createdAt.IsZero() {
		createdAt = updatedAt
	}
	return objectgraph.Object{
		CanonicalID: canonicalID,
		ObjectKind:  kind,
		OwnerID:     strings.TrimSpace(ownerID),
		ComputerID:  strings.TrimSpace(computerID),
		ContentHash: objectgraph.ContentHash(kind, bodyJSON, metadataJSON),
		Body:        bodyJSON,
		Metadata:    metadataJSON,
		CreatedAt:   createdAt.UTC(),
		UpdatedAt:   updatedAt.UTC(),
	}, nil
}

func lifecycleMetadata(idField, id, computerID, trajectoryID string, seq int64) map[string]any {
	return map[string]any{
		idField:           id,
		"computer_id":     computerID,
		"trajectory_id":   trajectoryID,
		"reducer_version": types.LifecycleReducerVersion,
		"reducer_seq":     seq,
	}
}

func (s *Store) lifecycleGraph() objectgraph.Store {
	if s.ogReadStore != nil {
		return s.ogReadStore
	}
	return s.ogStore
}

func (s *Store) lifecycleGetObject(ctx context.Context, kind objectgraph.ObjectKind, ownerID, computerID, key string) (objectgraph.Object, error) {
	graph := s.lifecycleGraph()
	if graph == nil {
		return objectgraph.Object{}, fmt.Errorf("store: object graph not initialized")
	}
	id, err := lifecycleCanonicalID(kind, ownerID, computerID, key)
	if err != nil {
		return objectgraph.Object{}, err
	}
	obj, err := graph.GetObject(ctx, id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return objectgraph.Object{}, ErrNotFound
	}
	return obj, err
}

func decodeLifecycleObject[T any](obj objectgraph.Object) (T, error) {
	var rec T
	if err := json.Unmarshal(obj.Body, &rec); err != nil {
		return rec, err
	}
	return rec, nil
}

func normalizeLifecycleScope(ownerID, computerID string) (string, string, error) {
	ownerID = strings.TrimSpace(ownerID)
	computerID = strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return "", "", fmt.Errorf("lifecycle: owner_id and computer_id are required")
	}
	return ownerID, computerID, nil
}

// validateLifecycleSettlementRule enforces the closed durable-work/v1 predicate vocabulary.
func validateLifecycleSettlementRule(rule types.SettlementRule, subjectRefs map[string]string) error {
	if rule.Version != types.LifecycleReducerVersion || !rule.RequireNoOpenWorkItems || len(rule.RequiredSubjectRefs) == 0 {
		return fmt.Errorf("lifecycle settlement rule requires version %q, no-open-work, and subject refs: %w", types.LifecycleReducerVersion, ErrLifecycleInvalidTransition)
	}
	seen := make(map[string]struct{}, len(rule.RequiredSubjectRefs))
	for _, rawKey := range rule.RequiredSubjectRefs {
		key := strings.TrimSpace(rawKey)
		if key == "" || strings.TrimSpace(subjectRefs[key]) == "" {
			return fmt.Errorf("lifecycle settlement rule subject ref %q is unavailable: %w", rawKey, ErrLifecycleInvalidTransition)
		}
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("lifecycle settlement rule subject ref %q is duplicated: %w", key, ErrLifecycleInvalidTransition)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// StartLifecycle is a legacy lifecycle writer. Supervision owns lifecycle mutation.
func (s *Store) StartLifecycle(ctx context.Context, req types.StartLifecycleRequest) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, ErrLifecycleAuthorityRequired
}

func (s *Store) lifecycleTrajectoryExists(ctx context.Context, ownerID, trajectoryID string) (bool, error) {
	ownerID, trajectoryID = strings.TrimSpace(ownerID), strings.TrimSpace(trajectoryID)
	if ownerID == "" || trajectoryID == "" {
		return false, nil
	}
	objs, err := s.ogListAllByMetadata(ctx, ogKindTrajectory, "trajectory_id", trajectoryID)
	if err != nil {
		return false, err
	}
	for _, obj := range objs {
		trajectory, decodeErr := decodeLifecycleObject[types.TrajectoryRecord](obj)
		if decodeErr != nil {
			return false, decodeErr
		}
		if trajectory.OwnerID == ownerID && trajectory.TrajectoryID == trajectoryID && trajectory.LifecycleVersion > 0 {
			return true, nil
		}
	}
	return false, nil
}
func (s *Store) GetLifecycleTrajectory(ctx context.Context, ownerID, computerID, trajectoryID string) (types.TrajectoryRecord, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindTrajectory, ownerID, computerID, trajectoryID)
	if err != nil {
		return types.TrajectoryRecord{}, err
	}
	return decodeLifecycleObject[types.TrajectoryRecord](obj)
}

func (s *Store) GetLifecycleDocument(ctx context.Context, ownerID, computerID, docID string) (types.Document, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindTexDoc, ownerID, computerID, docID)
	if err != nil {
		return types.Document{}, err
	}
	return decodeLifecycleObject[types.Document](obj)
}

func (s *Store) GetLifecycleRevision(ctx context.Context, ownerID, computerID, revisionID string) (types.Revision, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindTexRev, ownerID, computerID, revisionID)
	if err != nil {
		return types.Revision{}, err
	}
	return decodeLifecycleObject[types.Revision](obj)
}
func (s *Store) GetLifecycleRun(ctx context.Context, ownerID, computerID, runID string) (types.RunRecord, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindRun, ownerID, computerID, runID)
	if err != nil {
		return types.RunRecord{}, err
	}
	run, err := decodeLifecycleObject[types.RunRecord](obj)
	if err != nil {
		return types.RunRecord{}, err
	}
	if run.OwnerID != strings.TrimSpace(ownerID) || run.SandboxID != strings.TrimSpace(computerID) || run.RunID != strings.TrimSpace(runID) {
		return types.RunRecord{}, ErrLifecycleInvalidTransition
	}
	return run, nil
}

// UpdateLifecycleDocumentTitleAuthority is a legacy lifecycle writer.
func (s *Store) UpdateLifecycleDocumentTitleAuthority(ctx context.Context, ownerID, computerID, docID, title string) (types.Document, error) {
	return types.Document{}, ErrLifecycleAuthorityRequired
}

// ListLifecycleRunsByState returns lifecycle-owned runs in one computer with
// the requested state. A non-empty ownerID narrows the result; the boot
// recovery caller intentionally exhausts all owners. Legacy runs are excluded.
func (s *Store) ListLifecycleRunsByState(ctx context.Context, ownerID, computerID string, state types.RunState) ([]types.RunRecord, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if computerID == "" {
		return nil, fmt.Errorf("list lifecycle runs: computer_id is required")
	}
	if !state.Valid() {
		return nil, fmt.Errorf("list lifecycle runs: invalid state %q", state)
	}
	objs, err := s.ogListAllByMetadata(ctx, ogKindRun, "state", string(state))
	if err != nil {
		return nil, fmt.Errorf("list lifecycle runs: %w", err)
	}
	runs := make([]types.RunRecord, 0, len(objs))
	for _, obj := range objs {
		if (ownerID != "" && obj.OwnerID != ownerID) || obj.ComputerID != computerID {
			continue
		}
		run, decodeErr := decodeLifecycleObject[types.RunRecord](obj)
		if decodeErr != nil || !lifecycleRunProjection(obj, run) ||
			(ownerID != "" && run.OwnerID != ownerID) || run.SandboxID != computerID || run.State != state {
			continue
		}
		runs = append(runs, run)
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].RunID < runs[j].RunID
		}
		return runs[i].CreatedAt.Before(runs[j].CreatedAt)
	})
	return runs, nil
}

func (s *Store) ListLifecycleTrajectoriesByOwner(ctx context.Context, ownerID, computerID string, limit int) ([]types.TrajectoryRecord, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return nil, err
	}
	graph := s.ogReadStore
	if graph == nil {
		graph = s.ogStore
	}
	if graph == nil {
		return nil, fmt.Errorf("lifecycle trajectories: object graph not initialized")
	}
	objs, err := graph.ReadObjectSnapshot(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	trajectories := make([]types.TrajectoryRecord, 0)
	for _, obj := range objs {
		if obj.ObjectKind != ogKindTrajectory {
			continue
		}
		trajectory, decodeErr := decodeLifecycleObject[types.TrajectoryRecord](obj)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if trajectory.OwnerID == ownerID && trajectory.ComputerID == computerID && trajectory.LifecycleVersion > 0 {
			trajectories = append(trajectories, trajectory)
		}
	}
	sort.Slice(trajectories, func(i, j int) bool {
		if !trajectories[i].UpdatedAt.Equal(trajectories[j].UpdatedAt) {
			return trajectories[i].UpdatedAt.After(trajectories[j].UpdatedAt)
		}
		return trajectories[i].TrajectoryID < trajectories[j].TrajectoryID
	})
	if limit > 0 && len(trajectories) > limit {
		trajectories = trajectories[:limit]
	}
	return trajectories, nil
}

func (s *Store) ListLifecycleRunsByOwner(ctx context.Context, ownerID, computerID string, limit int) ([]types.RunRecord, error) {
	return s.listLifecycleRunsByScope(ctx, ownerID, computerID, limit, nil)
}

func (s *Store) ListLifecycleRunsByChannel(ctx context.Context, ownerID, computerID, channelID string, limit int) ([]types.RunRecord, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return nil, ErrLifecycleInvalidTransition
	}
	return s.listLifecycleRunsByScope(ctx, ownerID, computerID, limit, func(run types.RunRecord) bool {
		return strings.TrimSpace(run.ChannelID) == channelID
	})
}

func (s *Store) listLifecycleRunsByScope(ctx context.Context, ownerID, computerID string, limit int, match func(types.RunRecord) bool) ([]types.RunRecord, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return nil, err
	}
	graph := s.ogReadStore
	if graph == nil {
		graph = s.ogStore
	}
	if graph == nil {
		return nil, fmt.Errorf("lifecycle runs: object graph not initialized")
	}
	objs, err := graph.ReadObjectSnapshot(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	runs := make([]types.RunRecord, 0)
	for _, obj := range objs {
		if obj.ObjectKind != ogKindRun {
			continue
		}
		run, decodeErr := decodeLifecycleObject[types.RunRecord](obj)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if run.OwnerID == ownerID && run.SandboxID == computerID && (match == nil || match(run)) {
			runs = append(runs, run)
		}
	}
	sort.Slice(runs, func(i, j int) bool {
		if !runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].CreatedAt.After(runs[j].CreatedAt)
		}
		return runs[i].RunID < runs[j].RunID
	})
	if limit > 0 && len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

func (s *Store) ListActiveLifecycleRunsByTrajectory(ctx context.Context, ownerID, computerID, trajectoryID string, limit int) ([]types.RunRecord, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return nil, err
	}
	trajectoryID = strings.TrimSpace(trajectoryID)
	if trajectoryID == "" {
		return nil, ErrLifecycleInvalidTransition
	}
	graph := s.ogReadStore
	if graph == nil {
		graph = s.ogStore
	}
	if graph == nil {
		return nil, fmt.Errorf("lifecycle runs: object graph not initialized")
	}
	objs, err := graph.ReadObjectSnapshot(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	runs := make([]types.RunRecord, 0)
	for _, obj := range objs {
		if obj.ObjectKind != ogKindRun {
			continue
		}
		run, decodeErr := decodeLifecycleObject[types.RunRecord](obj)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if run.OwnerID == ownerID && run.SandboxID == computerID && run.TrajectoryID == trajectoryID && run.State.Active() {
			runs = append(runs, run)
		}
	}
	sort.Slice(runs, func(i, j int) bool {
		if !runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].CreatedAt.Before(runs[j].CreatedAt)
		}
		return runs[i].RunID < runs[j].RunID
	})
	if limit > 0 && len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

func (s *Store) GetLifecycleWorkItem(ctx context.Context, ownerID, computerID, workItemID string) (types.WorkItemRecord, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindWorkItem, ownerID, computerID, workItemID)
	if err != nil {
		return types.WorkItemRecord{}, err
	}
	return decodeLifecycleObject[types.WorkItemRecord](obj)
}

// ListOpenAssignedLifecycleWorkItems returns every open lifecycle-owned work
// item assigned in one computer. The boot caller intentionally exhausts all
// owners so a process restart cannot strand a durable obligation.
func (s *Store) ListOpenAssignedLifecycleWorkItems(ctx context.Context, computerID string, limit int) ([]types.WorkItemRecord, error) {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return nil, fmt.Errorf("list open assigned lifecycle work items: computer_id is required")
	}
	if limit < 0 {
		return nil, fmt.Errorf("list open assigned lifecycle work items: limit must not be negative")
	}
	objs, err := s.ogListAllObjectsByKind(ctx, ogKindWorkItem)
	if err != nil {
		return nil, fmt.Errorf("list open assigned lifecycle work items: %w", err)
	}
	items := make([]types.WorkItemRecord, 0, len(objs))
	for _, obj := range objs {
		if obj.ComputerID != computerID {
			continue
		}
		item, decodeErr := decodeLifecycleObject[types.WorkItemRecord](obj)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if item.ComputerID != computerID || item.Status != types.WorkItemOpen ||
			item.LifecycleVersion <= 0 || strings.TrimSpace(item.AssignedAgentID) == "" {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].WorkItemID < items[j].WorkItemID
		}
		return items[i].UpdatedAt.Before(items[j].UpdatedAt)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (s *Store) ListLifecycleEvents(ctx context.Context, ownerID, computerID, trajectoryID string) ([]types.LifecycleEvent, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return nil, err
	}
	graph := s.ogReadStore
	if graph == nil {
		graph = s.ogStore
	}
	if graph == nil {
		return nil, fmt.Errorf("lifecycle events: object graph not initialized")
	}
	objs, err := graph.ReadObjectSnapshot(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	events := make([]types.LifecycleEvent, 0)
	for _, obj := range objs {
		if obj.ObjectKind != ogKindLifecycleEvent {
			continue
		}
		event, decodeErr := decodeLifecycleObject[types.LifecycleEvent](obj)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if event.TrajectoryID == trajectoryID {
			events = append(events, event)
		}
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].ReducerSeq == events[j].ReducerSeq {
			return events[i].EventID < events[j].EventID
		}
		return events[i].ReducerSeq < events[j].ReducerSeq
	})
	return events, nil
}
func (s *Store) ListLifecycleEventPage(ctx context.Context, ownerID, computerID, trajectoryID string, after int64, limit int) (types.LifecycleEventPage, error) {
	if after < 0 {
		return types.LifecycleEventPage{}, fmt.Errorf("lifecycle events: after must be non-negative")
	}
	if limit <= 0 {
		limit = 100
	}

	if limit > 1000 {
		limit = 1000
	}
	all, err := s.ListLifecycleEvents(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return types.LifecycleEventPage{}, err
	}
	watermark := int64(0)
	if len(all) > 0 {
		watermark = all[len(all)-1].ReducerSeq
	}
	page := types.LifecycleEventPage{Schema: types.DurableWorkSchemaV1, Events: make([]types.LifecycleEvent, 0, limit), NextCursor: after, Watermark: watermark}
	if after > watermark || (len(all) > 0 && after+1 < all[0].ReducerSeq) {
		return types.LifecycleEventPage{Schema: types.DurableWorkSchemaV1, CursorExpired: true, ReplayRequired: true, NextCursor: after, Watermark: watermark}, ErrLifecycleCursorExpired
	}
	for _, event := range all {
		if event.ReducerSeq <= after {
			continue
		}
		page.Events = append(page.Events, event)
		page.NextCursor = event.ReducerSeq
		if len(page.Events) == limit {
			break
		}
	}
	return page, nil
}

func (s *Store) ListLifecycleSubjects(ctx context.Context, computerID string) ([]types.AgentRecord, error) {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return nil, fmt.Errorf("list lifecycle subjects: computer_id is required")
	}
	if s.ogStore == nil {
		return nil, fmt.Errorf("list lifecycle subjects: object graph not initialized")
	}
	var objs []objectgraph.Object
	after := ""
	for {
		page, err := s.ogStore.ListObjectsPage(ctx, string(ogKindAgent), after, 1000)
		if err != nil {
			return nil, err
		}
		for _, obj := range page {
			if obj.ComputerID == computerID {
				objs = append(objs, obj)
			}
		}
		if len(page) < 1000 {
			break
		}
		after = page[len(page)-1].CanonicalID
	}
	subjects := make([]types.AgentRecord, 0, len(objs))
	for _, obj := range objs {
		agent, decodeErr := decodeLifecycleObject[types.AgentRecord](obj)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if agent.LifecycleVersion > 0 && agent.OwnerID != "" && agent.ComputerID != "" {
			subjects = append(subjects, agent)
		}
	}
	sort.Slice(subjects, func(i, j int) bool {
		if subjects[i].OwnerID != subjects[j].OwnerID {
			return subjects[i].OwnerID < subjects[j].OwnerID
		}
		if subjects[i].ComputerID != subjects[j].ComputerID {
			return subjects[i].ComputerID < subjects[j].ComputerID
		}
		return subjects[i].AgentID < subjects[j].AgentID
	})
	return subjects, nil
}

func (s *Store) ListPendingLifecycleUpdates(ctx context.Context, ownerID, computerID, targetAgentID string, limit int) ([]types.CoagentSourcePacket, error) {
	if limit <= 0 {
		limit = 100
	}
	graph := s.ogReadStore
	if graph == nil {
		graph = s.ogStore
	}
	if graph == nil {
		return nil, fmt.Errorf("lifecycle updates: object graph not initialized")
	}
	objects, err := graph.ReadObjectSnapshot(ctx, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	updates := make([]types.CoagentSourcePacket, 0)
	for _, obj := range objects {
		if obj.ObjectKind != ogKindWorkerUpdate {
			continue
		}
		update, decodeErr := decodeLifecycleObject[types.CoagentSourcePacket](obj)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if update.LifecycleVersion > 0 && update.TargetAgentID == targetAgentID && update.Disposition == types.UpdatePending {
			updates = append(updates, update)
		}
	}
	sort.Slice(updates, func(i, j int) bool {
		if updates[i].ReducerSeq != updates[j].ReducerSeq {
			return updates[i].ReducerSeq < updates[j].ReducerSeq
		}
		return updates[i].UpdateID < updates[j].UpdateID
	})
	if len(updates) > limit {
		updates = updates[:limit]
	}
	return updates, nil
}

func (s *Store) GetLifecycleUpdate(ctx context.Context, ownerID, computerID, trajectoryID, targetAgentID, producerAgentID, producerUpdateID string) (types.CoagentSourcePacket, error) {
	key := strings.TrimSpace(trajectoryID) + "\x00" + strings.TrimSpace(targetAgentID) + "\x00" + strings.TrimSpace(producerAgentID) + "\x00" + strings.TrimSpace(producerUpdateID)
	obj, err := s.lifecycleGetObject(ctx, ogKindWorkerUpdate, ownerID, computerID, key)
	if err != nil {
		return types.CoagentSourcePacket{}, err
	}
	return decodeLifecycleObject[types.CoagentSourcePacket](obj)
}

func (s *Store) GetLifecycleSnapshot(ctx context.Context, ownerID, computerID, trajectoryID string) (types.LifecycleSnapshot, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return types.LifecycleSnapshot{}, err
	}
	trajectoryID = strings.TrimSpace(trajectoryID)
	if trajectoryID == "" {
		return types.LifecycleSnapshot{}, fmt.Errorf("lifecycle snapshot: trajectory_id is required")
	}
	graph := s.ogReadStore
	if graph == nil {
		graph = s.ogStore
	}
	if graph == nil {
		return types.LifecycleSnapshot{}, fmt.Errorf("lifecycle snapshot: object graph not initialized")
	}
	objects, err := graph.ReadObjectSnapshot(ctx, ownerID, computerID)
	if err != nil {
		return types.LifecycleSnapshot{}, err
	}
	snapshot := types.LifecycleSnapshot{Schema: types.DurableWorkSchemaV1}
	documents := make(map[string]types.Document)
	revisions := make(map[string]types.Revision)
	trajectoryFound := false
	var activationCreatedAt time.Time
	for _, obj := range objects {
		switch obj.ObjectKind {
		case ogKindTrajectory:
			trajectory, decodeErr := decodeLifecycleObject[types.TrajectoryRecord](obj)
			if decodeErr != nil {
				return types.LifecycleSnapshot{}, decodeErr
			}
			if trajectory.TrajectoryID == trajectoryID && trajectory.LifecycleVersion > 0 {
				snapshot.Trajectory = trajectory
				trajectoryFound = true
			}
		case ogKindTexDoc:
			document, decodeErr := decodeLifecycleObject[types.Document](obj)
			if decodeErr != nil {
				return types.LifecycleSnapshot{}, decodeErr
			}
			documents[document.DocID] = document
		case ogKindTexRev:
			revision, decodeErr := decodeLifecycleObject[types.Revision](obj)
			if decodeErr != nil {
				return types.LifecycleSnapshot{}, decodeErr
			}
			revisions[revision.RevisionID] = revision
		case ogKindWorkItem:
			work, decodeErr := decodeLifecycleObject[types.WorkItemRecord](obj)
			if decodeErr != nil {
				return types.LifecycleSnapshot{}, decodeErr
			}
			if work.TrajectoryID == trajectoryID && work.LifecycleVersion > 0 {
				snapshot.WorkItems = append(snapshot.WorkItems, work)
			}
		case ogKindRun:
			run, decodeErr := decodeLifecycleObject[types.RunRecord](obj)
			if decodeErr != nil {
				return types.LifecycleSnapshot{}, decodeErr
			}
			if run.TrajectoryID == trajectoryID && (activationCreatedAt.IsZero() || run.CreatedAt.After(activationCreatedAt)) {
				activationCreatedAt = run.CreatedAt
				snapshot.Activation = types.LifecycleActivationProjection{AgentID: run.AgentID, RunID: run.RunID, State: run.State}
			}
		case ogKindAgent:
			agent, decodeErr := decodeLifecycleObject[types.AgentRecord](obj)
			if decodeErr != nil {
				return types.LifecycleSnapshot{}, decodeErr
			}
			if agent.LifecycleVersion > 0 && lifecycleObjectTrajectoryID(obj) == trajectoryID {
				snapshot.Agents = append(snapshot.Agents, agent)
			}
		case ogKindWorkerUpdate:
			update, decodeErr := decodeLifecycleObject[types.CoagentSourcePacket](obj)
			if decodeErr != nil {
				return types.LifecycleSnapshot{}, decodeErr
			}
			if update.TrajectoryID == trajectoryID && update.LifecycleVersion > 0 {
				snapshot.Updates = append(snapshot.Updates, update)
			}
		case ogKindLifecycleEvent:
			event, decodeErr := decodeLifecycleObject[types.LifecycleEvent](obj)
			if decodeErr != nil {
				return types.LifecycleSnapshot{}, decodeErr
			}
			if event.TrajectoryID == trajectoryID {
				snapshot.Events = append(snapshot.Events, event)
			}
		}
	}
	if !trajectoryFound {
		return types.LifecycleSnapshot{}, ErrNotFound
	}
	docID := strings.TrimSpace(snapshot.Trajectory.SubjectRefs["doc_id"])
	document, ok := documents[docID]
	if !ok {
		return types.LifecycleSnapshot{}, fmt.Errorf("lifecycle snapshot: bound document %q not found", docID)
	}
	headRevisionID := document.CurrentRevisionID
	if snapshot.Trajectory.Status != types.TrajectoryLive && strings.TrimSpace(snapshot.Trajectory.TerminalArtifactHeadRef) != "" {
		headRevisionID = snapshot.Trajectory.TerminalArtifactHeadRef
	}
	head, ok := revisions[headRevisionID]
	if !ok {
		return types.LifecycleSnapshot{}, fmt.Errorf("lifecycle snapshot: bound head revision %q not found", headRevisionID)
	}
	snapshot.Document, snapshot.HeadRevision = document, head
	if document.CurrentRevisionID != headRevisionID {
		current, currentOK := revisions[document.CurrentRevisionID]
		if !currentOK {
			return types.LifecycleSnapshot{}, fmt.Errorf("lifecycle snapshot: current document head revision %q not found", document.CurrentRevisionID)
		}
		snapshot.CurrentDocumentHead = &current
	}
	sort.Slice(snapshot.WorkItems, func(i, j int) bool { return snapshot.WorkItems[i].WorkItemID < snapshot.WorkItems[j].WorkItemID })
	sort.Slice(snapshot.Agents, func(i, j int) bool { return snapshot.Agents[i].AgentID < snapshot.Agents[j].AgentID })
	sort.Slice(snapshot.Updates, func(i, j int) bool {
		if snapshot.Updates[i].ReducerSeq == snapshot.Updates[j].ReducerSeq {
			return snapshot.Updates[i].UpdateID < snapshot.Updates[j].UpdateID
		}
		return snapshot.Updates[i].ReducerSeq < snapshot.Updates[j].ReducerSeq
	})
	sort.Slice(snapshot.Events, func(i, j int) bool {
		if snapshot.Events[i].ReducerSeq == snapshot.Events[j].ReducerSeq {
			return snapshot.Events[i].EventID < snapshot.Events[j].EventID
		}
		return snapshot.Events[i].ReducerSeq < snapshot.Events[j].ReducerSeq
	})
	if snapshot.Activation.AgentID == "" && len(snapshot.Agents) > 0 {
		snapshot.Activation = types.LifecycleActivationProjection{
			AgentID: snapshot.Agents[0].AgentID,
			State:   types.RunPassivated,
		}
	}
	if len(snapshot.Events) > 0 {
		snapshot.Watermark = snapshot.Events[len(snapshot.Events)-1].ReducerSeq
		snapshot.SnapshotCursor = snapshot.Watermark
	}
	return snapshot, nil
}

func lifecycleObjectTrajectoryID(obj objectgraph.Object) string {
	var metadata struct {
		TrajectoryID string `json:"trajectory_id"`
	}
	if json.Unmarshal(obj.Metadata, &metadata) != nil {
		return ""
	}
	return strings.TrimSpace(metadata.TrajectoryID)
}

func (s *Store) lifecycleTransitionObjects(ctx context.Context, kind objectgraph.ObjectKind, trajectoryID, ownerID, computerID string) ([]objectgraph.Object, error) {
	objs, err := s.ogListAllByMetadata(ctx, kind, "trajectory_id", trajectoryID)
	if err != nil {
		return nil, err
	}
	filtered := objs[:0]
	for _, obj := range objs {
		if obj.OwnerID == ownerID && obj.ComputerID == computerID {
			filtered = append(filtered, obj)
			continue
		}
		var scope struct {
			OwnerID    string `json:"owner_id"`
			ComputerID string `json:"computer_id"`
		}
		if json.Unmarshal(obj.Body, &scope) == nil && scope.OwnerID == ownerID && scope.ComputerID == computerID {
			filtered = append(filtered, obj)
		}
	}
	return filtered, nil
}

func workItemTerminal(status types.WorkItemStatus) bool {
	return status == types.WorkItemCompleted || status == types.WorkItemCancelled || status == types.WorkItemRefused
}

func (s *Store) lifecycleSettlementReady(ctx context.Context, trajectory types.TrajectoryRecord, prospectiveWork *types.WorkItemRecord, prospectiveUpdate *types.CoagentSourcePacket) (bool, error) {
	if err := validateLifecycleSettlementRule(trajectory.SettlementRule, trajectory.SubjectRefs); err != nil {
		return false, err
	}
	if strings.TrimSpace(trajectory.TerminalArtifactHeadRef) == "" {
		return false, nil
	}
	for _, key := range trajectory.SettlementRule.RequiredSubjectRefs {
		if strings.TrimSpace(trajectory.SubjectRefs[key]) == "" {
			return false, nil
		}
	}
	workObjs, err := s.lifecycleTransitionObjects(ctx, ogKindWorkItem, trajectory.TrajectoryID, trajectory.OwnerID, trajectory.ComputerID)
	if err != nil {
		return false, err
	}
	for _, obj := range workObjs {
		rec, decodeErr := decodeLifecycleObject[types.WorkItemRecord](obj)
		if decodeErr != nil {
			return false, decodeErr
		}
		if prospectiveWork != nil && rec.WorkItemID == prospectiveWork.WorkItemID {
			rec = *prospectiveWork
		}
		if !workItemTerminal(rec.Status) {
			return false, nil
		}
	}
	updateObjs, err := s.lifecycleTransitionObjects(ctx, ogKindWorkerUpdate, trajectory.TrajectoryID, trajectory.OwnerID, trajectory.ComputerID)
	if err != nil {
		return false, err
	}
	for _, obj := range updateObjs {
		update, decodeErr := decodeLifecycleObject[types.CoagentSourcePacket](obj)
		if decodeErr != nil {
			return false, decodeErr
		}
		if prospectiveUpdate != nil && update.UpdateID == prospectiveUpdate.UpdateID {
			update = *prospectiveUpdate
		}
		if update.Disposition == "" || update.Disposition == types.UpdatePending {
			return false, nil
		}
	}
	return true, nil
}

// OpenLifecycleWork is a legacy lifecycle writer.
func (s *Store) OpenLifecycleWork(ctx context.Context, req types.OpenLifecycleWorkRequest) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, ErrLifecycleAuthorityRequired
}

// ReplaceLifecycleActivation is a legacy lifecycle writer.
func (s *Store) ReplaceLifecycleActivation(ctx context.Context, req types.ReplaceLifecycleActivationRequest) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, ErrLifecycleAuthorityRequired
}

// ProjectTerminalLifecycleRun is a legacy lifecycle writer.
func (s *Store) ProjectTerminalLifecycleRun(ctx context.Context, req types.ReplaceLifecycleActivationRequest) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, ErrLifecycleAuthorityRequired
}

// AmendLifecycleWork is a legacy lifecycle writer.
func (s *Store) AmendLifecycleWork(ctx context.Context, req types.AmendLifecycleWorkRequest) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, ErrLifecycleAuthorityRequired
}

func normalizeLifecycleRefs(refs []string) []string {
	seen := make(map[string]struct{}, len(refs))
	normalized := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		normalized = append(normalized, ref)
	}
	sort.Strings(normalized)
	return normalized
}

// RecordLifecycleRefs is a legacy lifecycle writer.
func (s *Store) RecordLifecycleRefs(ctx context.Context, req types.RecordLifecycleRefsRequest) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, ErrLifecycleAuthorityRequired
}

func normalizeUpdateWorkDisposition(value types.WorkItemStatus) (types.WorkItemStatus, error) {
	disposition := types.WorkItemStatus(strings.TrimSpace(string(value)))
	switch disposition {
	case "", types.WorkItemOpen, types.WorkItemCompleted, types.WorkItemRefused:
		return disposition, nil
	default:
		return "", fmt.Errorf("lifecycle update work_disposition must be open, completed, or refused")
	}
}

func validateUpdateWorkConsequence(disposition types.WorkItemStatus, workItemID, operation string) error {
	hasWorkItem := strings.TrimSpace(workItemID) != ""
	if disposition == types.WorkItemCompleted || disposition == types.WorkItemRefused {
		if !hasWorkItem {
			return fmt.Errorf("%s: terminal work disposition requires work_item_id", operation)
		}
		return nil
	}
	if hasWorkItem && disposition == "" {
		return fmt.Errorf("%s: work_item_id requires explicit work disposition", operation)
	}
	return nil
}

// QueueLifecycleUpdate is a legacy lifecycle writer.
func (s *Store) QueueLifecycleUpdate(ctx context.Context, req types.QueueLifecycleUpdateRequest) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, ErrLifecycleAuthorityRequired
}

func unscopedGraphObject(kind objectgraph.ObjectKind, ownerID, identityKey string, body any, metadata map[string]any, now time.Time) (objectgraph.Object, error) {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return objectgraph.Object{}, err
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return objectgraph.Object{}, err
	}
	metadataJSON, err = objectgraph.NormalizeMetadata(metadataJSON)
	if err != nil {
		return objectgraph.Object{}, err
	}
	canonicalID, err := objectgraph.BuildCanonicalID(kind, ownerID, objectgraph.StableSuffixFromKey(identityKey))
	if err != nil {
		return objectgraph.Object{}, err
	}
	return objectgraph.Object{
		CanonicalID: canonicalID, ObjectKind: kind, OwnerID: ownerID,
		ContentHash: objectgraph.ContentHash(kind, bodyJSON, metadataJSON),
		Body:        bodyJSON, Metadata: metadataJSON, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func lifecycleSourceGraphCanonicalID(kind objectgraph.ObjectKind, ownerID, computerID, identityKey string) (string, error) {
	return objectgraph.BuildCanonicalID(kind, ownerID, objectgraph.StableSuffixFromKey(lifecycleScopedKey(computerID, identityKey)))
}

func lifecycleSourceGraphObject(kind objectgraph.ObjectKind, ownerID, computerID, identityKey string, body any, metadata map[string]any, now time.Time) (objectgraph.Object, error) {
	metadata["computer_id"] = computerID
	obj, err := unscopedGraphObject(kind, ownerID, lifecycleScopedKey(computerID, identityKey), body, metadata, now)
	if err != nil {
		return objectgraph.Object{}, err
	}
	obj.ComputerID = computerID
	return obj, nil
}

func validateLifecycleSourceEntityIdentity(rec TextureSourceEntityGraphRecord) error {
	var metadata struct {
		SourceKind string `json:"source_kind"`
		Target     struct {
			Identity string `json:"identity"`
		} `json:"target"`
	}
	if err := json.Unmarshal(rec.Metadata, &metadata); err != nil {
		return fmt.Errorf("lifecycle source entity identity metadata: %w", err)
	}
	ownerScope := rec.OwnerID + "\x00" + rec.ComputerID
	expected, err := BuildTextureSourceEntityCanonicalID(rec.OwnerID, ownerScope, metadata.SourceKind, metadata.Target.Identity)
	if err != nil {
		return fmt.Errorf("lifecycle source entity identity: %w", err)
	}
	if rec.CanonicalID != expected {
		return fmt.Errorf("lifecycle source entity canonical_id %q does not match computer-scoped identity %q", rec.CanonicalID, expected)
	}
	return nil
}

func validateLifecycleSourceRefIdentity(rec TextureSourceRefGraphRecord) error {
	var metadata struct {
		IdentityKey string `json:"identity_key"`
	}
	if err := json.Unmarshal(rec.Metadata, &metadata); err != nil {
		return fmt.Errorf("lifecycle source ref identity metadata: %w", err)
	}
	expected, err := BuildTextureSourceRefCanonicalIDByScope(rec.OwnerID, rec.ComputerID, rec.TextureRevisionID, metadata.IdentityKey)
	if err != nil {
		return fmt.Errorf("lifecycle source ref identity: %w", err)
	}
	if rec.CanonicalID != expected {
		return fmt.Errorf("lifecycle source ref canonical_id %q does not match computer-scoped identity %q", rec.CanonicalID, expected)
	}
	return nil
}

func (s *Store) lifecycleSourceGraphBatch(ctx context.Context, rev types.Revision, graph TextureSourceGraphWriteSet, now time.Time) ([]objectgraph.Object, []objectgraph.ObjectCondition, error) {
	objects := make([]objectgraph.Object, 0, len(graph.SourceEntities)+len(graph.SourceRefs))
	conditions := make([]objectgraph.ObjectCondition, 0, len(graph.SourceEntities)+len(graph.SourceRefs)*2)
	entityIDs := make(map[string]objectgraph.Object, len(graph.SourceEntities))
	for _, input := range graph.SourceEntities {
		rec, err := normalizeTextureSourceEntityGraphRecord(input, rev.OwnerID, rev.ComputerID, now)
		if err != nil {
			return nil, nil, err
		}
		if rec.ComputerID != "" {
			if err := validateLifecycleSourceEntityIdentity(rec); err != nil {
				return nil, nil, err
			}
		}
		identityKey := rec.CanonicalID + "\x00" + rec.VersionID
		obj, err := lifecycleSourceGraphObject(TextureSourceEntityObjectKind, rec.OwnerID, rec.ComputerID, identityKey, rec, map[string]any{
			"canonical_id": rec.CanonicalID, "version_id": rec.VersionID,
			"entity_version_key": entityVersionKey(rec.CanonicalID, rec.VersionID),
			"created_at":         rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		}, now)
		if err != nil {
			return nil, nil, err
		}
		if existing, getErr := s.lifecycleGraph().GetObject(ctx, obj.CanonicalID); getErr == nil {
			if existing.ContentHash != obj.ContentHash {
				return nil, nil, fmt.Errorf("lifecycle source entity conflict for %s/%s", rec.CanonicalID, rec.VersionID)
			}
			conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: existing.CanonicalID, Exists: true, ExpectedContentHash: existing.ContentHash})
			entityIDs[entityVersionKey(rec.CanonicalID, rec.VersionID)] = existing
		} else if errors.Is(getErr, objectgraph.ErrNotFound) {
			conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID})
			objects = append(objects, obj)
			entityIDs[entityVersionKey(rec.CanonicalID, rec.VersionID)] = obj
		} else {
			return nil, nil, getErr
		}
	}
	for _, input := range graph.SourceRefs {
		rec, err := normalizeTextureSourceRefGraphRecord(input, rev, now)
		if err != nil {
			return nil, nil, err
		}
		if rec.ComputerID != "" {
			if err := validateLifecycleSourceRefIdentity(rec); err != nil {
				return nil, nil, err
			}
		}
		entityKey := entityVersionKey(rec.SourceEntityCanonicalID, rec.SourceEntityVersionID)
		if _, ok := entityIDs[entityKey]; !ok {
			entityCanonicalID, buildErr := lifecycleSourceGraphCanonicalID(TextureSourceEntityObjectKind, rec.OwnerID, rec.ComputerID, rec.SourceEntityCanonicalID+"\x00"+rec.SourceEntityVersionID)
			if buildErr != nil {
				return nil, nil, buildErr
			}
			entityObj, getErr := s.lifecycleGraph().GetObject(ctx, entityCanonicalID)
			if getErr != nil {
				return nil, nil, fmt.Errorf("texture source ref: missing source entity version %s/%s", rec.SourceEntityCanonicalID, rec.SourceEntityVersionID)
			}
			conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: entityObj.CanonicalID, Exists: true, ExpectedContentHash: entityObj.ContentHash})
		}
		identityKey := rec.CanonicalID + "\x00" + rec.VersionID
		obj, err := lifecycleSourceGraphObject(TextureSourceRefObjectKind, rec.OwnerID, rec.ComputerID, identityKey, rec, map[string]any{
			"canonical_id": rec.CanonicalID, "version_id": rec.VersionID, "ref_version_key": identityKey,
			"doc_id": rec.DocID, "texture_revision_id": rec.TextureRevisionID,
			"created_at": rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		}, now)
		if err != nil {
			return nil, nil, err
		}
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID})
		objects = append(objects, obj)
	}
	return objects, conditions, nil
}

// CommitLifecycleArtifactHead is a legacy lifecycle writer.
func (s *Store) CommitLifecycleArtifactHead(ctx context.Context, req types.CommitLifecycleArtifactHeadRequest) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, ErrLifecycleAuthorityRequired
}

// ApplyLifecycleUpdate is a legacy lifecycle writer.
func (s *Store) ApplyLifecycleUpdate(ctx context.Context, req types.ApplyLifecycleUpdateRequest) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, ErrLifecycleAuthorityRequired
}

// ApplyLifecycleUpdateWithSourceGraph is a legacy lifecycle writer.
func (s *Store) ApplyLifecycleUpdateWithSourceGraph(ctx context.Context, req types.ApplyLifecycleUpdateRequest, graph TextureSourceGraphWriteSet) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, ErrLifecycleAuthorityRequired
}

// SettleLifecycleWork is a legacy lifecycle writer.
func (s *Store) SettleLifecycleWork(ctx context.Context, req types.SettleLifecycleWorkRequest) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, ErrLifecycleAuthorityRequired
}

// RefuseLifecycleWork is a legacy lifecycle writer.
func (s *Store) RefuseLifecycleWork(ctx context.Context, req types.RefuseLifecycleWorkRequest) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, ErrLifecycleAuthorityRequired
}

// SettleLifecycleTrajectory is a legacy lifecycle writer.
func (s *Store) SettleLifecycleTrajectory(ctx context.Context, req types.SettleLifecycleTrajectoryRequest) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, ErrLifecycleAuthorityRequired
}

// ArchiveLifecycleArtifact is a legacy lifecycle writer.
func (s *Store) ArchiveLifecycleArtifact(ctx context.Context, req types.ArchiveLifecycleArtifactRequest) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, ErrLifecycleAuthorityRequired
}

// CancelLifecycleTrajectory is a legacy lifecycle writer.
func (s *Store) CancelLifecycleTrajectory(ctx context.Context, req types.CancelLifecycleRequest) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, ErrLifecycleAuthorityRequired
}
