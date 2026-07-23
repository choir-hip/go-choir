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

type lifecycleSequence struct {
	TrajectoryID string `json:"trajectory_id"`
	ReducerSeq   int64  `json:"reducer_seq"`
}

func lifecycleTerminalTrajectoryRef(trajectoryID string) string {
	return "trajectory:" + strings.TrimSpace(trajectoryID)
}

func lifecycleScopedKey(computerID, key string) string {
	return strings.TrimSpace(computerID) + "\x00" + strings.TrimSpace(key)
}

func (s *Store) nextPostTerminalSequence(ctx context.Context, ownerID, computerID string, trajectory types.TrajectoryRecord, now time.Time) (int64, objectgraph.Object, objectgraph.ObjectCondition, error) {
	sequenceObj, err := s.lifecycleGetObject(ctx, ogKindLifecycleSeq, ownerID, computerID, trajectory.TrajectoryID)
	sequence := lifecycleSequence{TrajectoryID: trajectory.TrajectoryID, ReducerSeq: trajectory.ReducerSeq}
	createdAt := now
	condition := objectgraph.ObjectCondition{}
	if err == nil {
		if sequence, err = decodeLifecycleObject[lifecycleSequence](sequenceObj); err != nil {
			return 0, objectgraph.Object{}, objectgraph.ObjectCondition{}, err
		}
		if sequence.TrajectoryID != trajectory.TrajectoryID || sequence.ReducerSeq < trajectory.ReducerSeq {
			return 0, objectgraph.Object{}, objectgraph.ObjectCondition{}, ErrLifecycleInvalidTransition
		}
		createdAt = sequenceObj.CreatedAt
		condition = objectgraph.ObjectCondition{CanonicalID: sequenceObj.CanonicalID, Exists: true, ExpectedContentHash: sequenceObj.ContentHash}
	} else if !errors.Is(err, ErrNotFound) {
		return 0, objectgraph.Object{}, objectgraph.ObjectCondition{}, err
	}
	sequence.ReducerSeq++
	updated, err := lifecycleObject(ogKindLifecycleSeq, ownerID, computerID, trajectory.TrajectoryID, sequence, lifecycleMetadata("trajectory_id", trajectory.TrajectoryID, computerID, trajectory.TrajectoryID, sequence.ReducerSeq), createdAt, now)
	if err != nil {
		return 0, objectgraph.Object{}, objectgraph.ObjectCondition{}, err
	}
	if condition.CanonicalID == "" {
		condition.CanonicalID = updated.CanonicalID
	}
	return sequence.ReducerSeq, updated, condition, nil
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

func requireLifecycleDigest(got string, computed string, err error) error {
	if err != nil {
		return err
	}
	if strings.TrimSpace(got) != computed {
		return fmt.Errorf("lifecycle: command digest does not match canonical request: %w", ErrLifecycleCommandConflict)
	}
	return nil
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

func (s *Store) replayLifecycleCommand(ctx context.Context, ownerID, computerID, commandID, digest string) (types.LifecycleResult, bool, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindLifecycleCmd, ownerID, computerID, commandID)
	if errors.Is(err, ErrNotFound) {
		return types.LifecycleResult{}, false, nil
	}
	if err != nil {
		return types.LifecycleResult{}, false, err
	}
	receipt, err := decodeLifecycleObject[types.LifecycleCommandReceipt](obj)
	if err != nil {
		return types.LifecycleResult{}, false, err
	}
	if receipt.CommandDigest != digest {
		return types.LifecycleResult{}, true, ErrLifecycleCommandConflict
	}
	if receipt.StoredResult != nil {
		stored := receipt.StoredResult
		receipt.StoredResult = nil
		return types.LifecycleResult{
			Receipt: receipt, Trajectory: stored.Trajectory, Schema: stored.Schema,
			WorkItem: stored.WorkItem, Agent: stored.Agent, Update: stored.Update,
			Events: stored.Events, Replay: true, Document: stored.Document, Revision: stored.Revision,
		}, true, nil
	}
	result := types.LifecycleResult{Receipt: receipt, Replay: true}
	if receipt.TrajectoryID != "" {
		trajectory, getErr := s.GetLifecycleTrajectory(ctx, ownerID, computerID, receipt.TrajectoryID)
		if getErr != nil {
			return types.LifecycleResult{}, true, getErr
		}
		result.Trajectory = trajectory
		snapshot, snapshotErr := s.GetLifecycleSnapshot(ctx, ownerID, computerID, receipt.TrajectoryID)
		if snapshotErr != nil {
			return types.LifecycleResult{}, true, snapshotErr
		}
		result.Document, result.Revision = &snapshot.Document, &snapshot.HeadRevision
	}
	for _, ref := range receipt.ResultEventRefs {
		eventObj, getErr := s.lifecycleGraph().GetObject(ctx, ref)
		if getErr != nil {
			return types.LifecycleResult{}, true, getErr
		}
		event, decodeErr := decodeLifecycleObject[types.LifecycleEvent](eventObj)
		if decodeErr != nil {
			return types.LifecycleResult{}, true, decodeErr
		}
		result.Events = append(result.Events, event)
	}
	if receipt.Kind == types.LifecycleCommitArtifactHead && len(result.Events) == 1 {
		refs := result.Events[0].ArtifactRefs
		if len(refs) == 2 {
			revisionObj, getErr := s.lifecycleGetObject(ctx, ogKindTexRev, ownerID, computerID, refs[1])
			if getErr != nil {
				return types.LifecycleResult{}, true, getErr
			}
			revision, decodeErr := decodeLifecycleObject[types.Revision](revisionObj)
			if decodeErr != nil {
				return types.LifecycleResult{}, true, decodeErr
			}
			result.Revision = &revision
		}
	}
	return result, true, nil
}

func validateLifecycleCommand(commandID, digest, trajectoryID string) error {
	if strings.TrimSpace(commandID) == "" || strings.TrimSpace(digest) == "" || strings.TrimSpace(trajectoryID) == "" {
		return fmt.Errorf("lifecycle: command_id, command digest, and trajectory_id are required")
	}
	return nil
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

// StartLifecycle atomically creates one trajectory, initial work item, durable agent activation, event, and command receipt. Effects remain disabled.
func (s *Store) StartLifecycle(ctx context.Context, req types.StartLifecycleRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID = strings.TrimSpace(req.CommandID)
	req.StartRequestDigest = strings.TrimSpace(req.StartRequestDigest)
	req.TrajectoryID = strings.TrimSpace(req.TrajectoryID)
	if err := validateLifecycleCommand(req.CommandID, req.StartRequestDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	computedStartDigest, startDigestErr := ComputeStartLifecycleRequestDigest(req)
	if err := requireLifecycleDigest(req.StartRequestDigest, computedStartDigest, startDigestErr); err != nil {
		return types.LifecycleResult{}, err
	}
	docID := strings.TrimSpace(req.InitialDocument.DocID)
	expectedAgentID := "texture:" + docID
	if req.InitialWork.WorkItemID == "" || docID == "" || strings.TrimSpace(req.InitialRevision.RevisionID) == "" {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle start: initial doc_id, revision_id, and work_item_id are required")
	}
	if strings.TrimSpace(req.Agent.AgentID) != expectedAgentID ||
		strings.TrimSpace(req.Agent.Profile) != "texture" ||
		strings.TrimSpace(req.Agent.Role) != "texture" ||
		strings.TrimSpace(req.Agent.ChannelID) != docID {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle start: durable subject must be texture:%s with Texture profile/role and document channel: %w", docID, ErrLifecycleInvalidTransition)
	}
	if assigned := strings.TrimSpace(req.InitialWork.AssignedAgentID); assigned != "" && assigned != expectedAgentID {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle start: initial work assignment must target %s: %w", expectedAgentID, ErrLifecycleInvalidTransition)
	}
	if req.InitialRevision.DocID != "" && strings.TrimSpace(req.InitialRevision.DocID) != strings.TrimSpace(req.InitialDocument.DocID) {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle start: initial revision doc_id mismatch")
	}

	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	subjectRefs := make(map[string]string, len(req.SubjectRefs)+1)
	for key, value := range req.SubjectRefs {
		subjectRefs[key] = value
	}
	if err := validateLifecycleSettlementRule(req.SettlementRule, subjectRefs); err != nil {
		return types.LifecycleResult{}, err
	}
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.StartRequestDigest); found || replayErr != nil {
		return replay, replayErr
	}
	now := time.Now().UTC()
	trajectory := types.TrajectoryRecord{
		TrajectoryID: req.TrajectoryID, OwnerID: ownerID, ComputerID: computerID,
		Kind: req.Kind, SubjectRefs: subjectRefs, Status: types.TrajectoryLive,
		SettlementRule: req.SettlementRule, LifecycleVersion: 1, ReducerSeq: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	if trajectory.Kind == "" {
		trajectory.Kind = types.TrajectoryKindTask
	}
	if trajectory.SubjectRefs == nil {
		trajectory.SubjectRefs = make(map[string]string)
	}
	trajectory.SubjectRefs["doc_id"] = strings.TrimSpace(req.InitialDocument.DocID)
	workInput := req.InitialWork
	if strings.TrimSpace(workInput.AssignedAgentID) == "" {
		workInput.AssignedAgentID = strings.TrimSpace(req.Agent.AgentID)
	}
	work, err := normalizeLifecycleWork(workInput, ownerID, computerID, req.TrajectoryID, now)
	if err != nil {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle start: initial work: %w", err)
	}
	work.LifecycleVersion, work.LastReducerSeq = 1, 1
	agent := req.Agent
	agent.OwnerID, agent.ComputerID = ownerID, computerID
	if strings.TrimSpace(agent.Profile) != strings.TrimSpace(agent.Role) {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle start: agent profile and role must match: %w", ErrLifecycleInvalidTransition)
	}
	switch strings.TrimSpace(agent.Profile) {
	case "texture", "researcher", "processor", "reconciler":
	default:
		return types.LifecycleResult{}, fmt.Errorf("lifecycle start: effects-capable agent profile is not admissible: %w", ErrLifecycleInvalidTransition)
	}
	if strings.TrimSpace(work.AuthorityProfile) == "" {
		work.AuthorityProfile = agent.Profile
	}
	if strings.TrimSpace(work.AuthorityProfile) != strings.TrimSpace(agent.Profile) {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle start: initial work authority does not match assigned agent: %w", ErrLifecycleInvalidTransition)
	}
	if agent.SandboxID == "" {
		agent.SandboxID = computerID
	}
	agent.LifecycleVersion, agent.LastReducerSeq = 1, 1
	if agent.CreatedAt.IsZero() {
		agent.CreatedAt = now
	}
	agent.UpdatedAt = now
	document := req.InitialDocument
	document.DocID, document.OwnerID, document.ComputerID, document.TrajectoryID = strings.TrimSpace(document.DocID), ownerID, computerID, req.TrajectoryID
	document.CurrentRevisionID, document.CreatedAt, document.UpdatedAt = strings.TrimSpace(req.InitialRevision.RevisionID), now, now
	revision := req.InitialRevision
	revision.RevisionID, revision.DocID = strings.TrimSpace(revision.RevisionID), document.DocID
	revision.OwnerID, revision.ComputerID, revision.TrajectoryID = ownerID, computerID, req.TrajectoryID
	revision.VersionNumber, revision.ParentRevisionID, revision.CreatedAt = 0, "", now
	if revision.AuthorKind == "" {
		revision.AuthorKind = types.AuthorAppAgent
	}
	expectedRevisionHash := types.ComputeStructuredRevisionHash("", revision.Content, revision.BodyDoc, revision.SourceEntities, revision.Provenance)
	if revision.RevisionHash != "" && revision.RevisionHash != expectedRevisionHash {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle start: initial revision hash mismatch: %w", ErrLifecycleInvalidTransition)
	}
	revision.RevisionHash = expectedRevisionHash

	event := types.LifecycleEvent{
		EventID: req.CommandID + ":1", OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: req.TrajectoryID, WorkItemID: work.WorkItemID,
		Kind: types.LifecycleTrajectoryStarted, ReducerVersion: types.LifecycleReducerVersion,
		ReducerSeq: 1, CommandID: req.CommandID, CommandDigest: req.StartRequestDigest,
		ArtifactRefs: []string{document.DocID, revision.RevisionID}, CreatedAt: now,
	}

	trajectoryObj, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, 1), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	workObj, err := lifecycleObject(ogKindWorkItem, ownerID, computerID, work.WorkItemID, work, lifecycleMetadata("work_item_id", work.WorkItemID, computerID, req.TrajectoryID, 1), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	agentMeta := lifecycleMetadata("agent_id", agent.AgentID, computerID, req.TrajectoryID, 1)
	agentMeta["channel_id"] = agent.ChannelID
	agentObj, err := lifecycleObject(ogKindAgent, ownerID, computerID, agent.AgentID, agent, agentMeta, agent.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	documentMeta := lifecycleMetadata("doc_id", document.DocID, computerID, req.TrajectoryID, 1)
	documentMeta["current_revision_id"] = document.CurrentRevisionID
	documentObj, err := lifecycleObject(ogKindTexDoc, ownerID, computerID, document.DocID, document, documentMeta, now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	revisionMeta := lifecycleMetadata("revision_id", revision.RevisionID, computerID, req.TrajectoryID, 1)
	revisionMeta["doc_id"] = document.DocID
	revisionMeta["revision_hash"] = revision.RevisionHash
	revisionObj, err := lifecycleObject(ogKindTexRev, ownerID, computerID, revision.RevisionID, revision, revisionMeta, now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	revisionDocumentEdgeID, err := objectgraph.BuildEdgeID(revisionObj.CanonicalID, documentObj.CanonicalID, ogEdgeDocRevision, json.RawMessage(`{}`))
	if err != nil {
		return types.LifecycleResult{}, err
	}
	revisionDocumentEdge := objectgraph.Edge{EdgeID: revisionDocumentEdgeID, FromID: revisionObj.CanonicalID, ToID: documentObj.CanonicalID, Kind: ogEdgeDocRevision, Metadata: json.RawMessage(`{}`), CreatedAt: now}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, 1), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	receipt := types.LifecycleCommandReceipt{
		CommandID: req.CommandID, CommandDigest: req.StartRequestDigest, Kind: types.LifecycleStart,
		OwnerID: ownerID, ComputerID: computerID, TrajectoryID: req.TrajectoryID,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: 1,
		ResultEventRefs: []string{eventObj.CanonicalID}, CreatedAt: now,
	}
	receiptObj, err := lifecycleObject(ogKindLifecycleCmd, ownerID, computerID, req.CommandID, receipt, lifecycleMetadata("command_id", req.CommandID, computerID, req.TrajectoryID, 1), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}

	legacyTrajectoryID, err := objectgraph.BuildCanonicalID(ogKindTrajectory, ownerID, objectgraph.StableSuffixFromKey(req.TrajectoryID))
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID}, {CanonicalID: workObj.CanonicalID},
		{CanonicalID: documentObj.CanonicalID}, {CanonicalID: revisionObj.CanonicalID},
		{CanonicalID: eventObj.CanonicalID}, {CanonicalID: receiptObj.CanonicalID},
	}
	if legacyTrajectoryID != trajectoryObj.CanonicalID {
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: legacyTrajectoryID})
	}
	if existingAgent, getErr := s.lifecycleGetObject(ctx, ogKindAgent, ownerID, computerID, agent.AgentID); getErr == nil {
		storedAgent, decodeErr := decodeLifecycleObject[types.AgentRecord](existingAgent)
		if decodeErr != nil {
			return types.LifecycleResult{}, decodeErr
		}
		if storedAgent.AgentID != agent.AgentID || storedAgent.OwnerID != ownerID ||
			storedAgent.ComputerID != computerID || storedAgent.SandboxID != computerID ||
			storedAgent.Profile != agent.Profile || storedAgent.Role != agent.Role ||
			storedAgent.ChannelID != agent.ChannelID {
			return types.LifecycleResult{}, fmt.Errorf("lifecycle start: existing durable subject binding conflicts with %s", agent.AgentID)
		}
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: existingAgent.CanonicalID, Exists: true, ExpectedContentHash: existingAgent.ContentHash})
		agent.CreatedAt = storedAgent.CreatedAt
		agentObj, err = lifecycleObject(ogKindAgent, ownerID, computerID, agent.AgentID, agent, agentMeta, existingAgent.CreatedAt, now)
		if err != nil {
			return types.LifecycleResult{}, err
		}
	} else if errors.Is(getErr, ErrNotFound) {
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: agentObj.CanonicalID})
	} else {
		return types.LifecycleResult{}, getErr
	}
	result := types.LifecycleResult{Receipt: receipt, Trajectory: trajectory, WorkItem: &work, Agent: &agent, Document: &document, Revision: &revision, Events: []types.LifecycleEvent{event}}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.StartRequestDigest, conditions,
		[]objectgraph.Object{documentObj, revisionObj, trajectoryObj, workObj, agentObj, eventObj, receiptObj},
		result, revisionDocumentEdge)
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

// UpdateLifecycleDocumentTitleAuthority changes only the owner-visible title
// projection. It preserves lifecycle state, artifact head, and reducer
// sequencing while conditionally replacing the scoped document object.
func (s *Store) UpdateLifecycleDocumentTitleAuthority(ctx context.Context, ownerID, computerID, docID, title string) (types.Document, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return types.Document{}, err
	}
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return types.Document{}, ErrNotFound
	}

	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	documentObj, err := s.lifecycleGetObject(ctx, ogKindTexDoc, ownerID, computerID, docID)
	if err != nil {
		return types.Document{}, err
	}
	document, err := decodeLifecycleObject[types.Document](documentObj)
	if err != nil {
		return types.Document{}, err
	}
	now := time.Now().UTC()
	document.Title, document.UpdatedAt = title, now
	var metadata map[string]any
	if err := json.Unmarshal(documentObj.Metadata, &metadata); err != nil {
		return types.Document{}, fmt.Errorf("decode lifecycle document metadata: %w", err)
	}
	metadata["title"], metadata["updated_at"] = title, now.Format(time.RFC3339Nano)
	updated, err := lifecycleObject(ogKindTexDoc, ownerID, computerID, docID, document, metadata, documentObj.CreatedAt, now)
	if err != nil {
		return types.Document{}, err
	}
	condition := objectgraph.ObjectCondition{
		CanonicalID: documentObj.CanonicalID, Exists: true, ExpectedContentHash: documentObj.ContentHash,
	}
	if err := s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{condition}, objectgraph.Batch{Objects: []objectgraph.Object{updated}}); err != nil {
		if errors.Is(err, objectgraph.ErrConflict) {
			return types.Document{}, ErrConcurrentStateChange
		}
		return types.Document{}, err
	}
	return document, nil
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

func (s *Store) lifecycleTransitionReceipt(now time.Time, ownerID, computerID, trajectoryID, commandID, digest string, kind types.LifecycleCommandKind, seq int64, events []objectgraph.Object) (types.LifecycleCommandReceipt, objectgraph.Object, error) {
	refs := make([]string, 0, len(events))
	for _, event := range events {
		refs = append(refs, event.CanonicalID)
	}
	receipt := types.LifecycleCommandReceipt{
		CommandID: commandID, CommandDigest: digest, Kind: kind,
		OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: seq,
		ResultEventRefs: refs, CreatedAt: now,
	}
	obj, err := lifecycleObject(ogKindLifecycleCmd, ownerID, computerID, commandID, receipt, lifecycleMetadata("command_id", commandID, computerID, trajectoryID, seq), now, now)
	return receipt, obj, err
}

func (s *Store) commitLifecycleTransition(ctx context.Context, ownerID, computerID, commandID, digest string, conditions []objectgraph.ObjectCondition, objects []objectgraph.Object, result types.LifecycleResult, edges ...objectgraph.Edge) (types.LifecycleResult, error) {
	if s.ogStore == nil {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle: object graph not initialized")
	}
	storedReceipt := result.Receipt
	storedReceipt.StoredResult = &types.LifecycleStoredResult{
		Trajectory: result.Trajectory, Schema: result.Schema, WorkItem: result.WorkItem,
		Agent: result.Agent, Update: result.Update, Events: result.Events,
		Document: result.Document, Revision: result.Revision,
	}
	receiptObj, err := lifecycleObject(ogKindLifecycleCmd, ownerID, computerID, commandID, storedReceipt,
		lifecycleMetadata("command_id", commandID, computerID, result.Receipt.TrajectoryID, result.Receipt.ReducerSeq),
		result.Receipt.CreatedAt, result.Receipt.CreatedAt)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	replacedReceipt := false
	for i := range objects {
		if objects[i].CanonicalID == receiptObj.CanonicalID {
			objects[i] = receiptObj
			replacedReceipt = true
			break
		}
	}
	if !replacedReceipt {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle: transition receipt missing from atomic batch")
	}
	if err := s.ogStore.PutBatchConditional(ctx, conditions, objectgraph.Batch{Objects: objects, Edges: edges}); err != nil {
		if errors.Is(err, objectgraph.ErrConflict) {
			if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, commandID, digest); found || replayErr != nil {
				return replay, replayErr
			}
			return types.LifecycleResult{}, ErrConcurrentStateChange
		}
		return types.LifecycleResult{}, err
	}
	return result, nil
}

func (s *Store) lifecycleTrajectoryObject(ctx context.Context, ownerID, computerID, trajectoryID string) (objectgraph.Object, types.TrajectoryRecord, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindTrajectory, ownerID, computerID, trajectoryID)
	if err != nil {
		return objectgraph.Object{}, types.TrajectoryRecord{}, err
	}
	rec, err := decodeLifecycleObject[types.TrajectoryRecord](obj)
	return obj, rec, err
}

func (s *Store) lifecycleWorkObject(ctx context.Context, ownerID, computerID, workItemID string) (objectgraph.Object, types.WorkItemRecord, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindWorkItem, ownerID, computerID, workItemID)
	if err != nil {
		return objectgraph.Object{}, types.WorkItemRecord{}, err
	}
	rec, err := decodeLifecycleObject[types.WorkItemRecord](obj)
	return obj, rec, err
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

// QueueLifecycleUpdate atomically accepts one producer-scoped update into the
// durable backlog. Incorporation is a separate reducer transition.
func normalizeLifecycleWork(work types.WorkItemRecord, ownerID, computerID, trajectoryID string, now time.Time) (types.WorkItemRecord, error) {
	work.WorkItemID = strings.TrimSpace(work.WorkItemID)
	work.Objective = strings.TrimSpace(work.Objective)
	work.AssignedAgentID = strings.TrimSpace(work.AssignedAgentID)
	if work.WorkItemID == "" || work.Objective == "" || work.AssignedAgentID == "" {
		return types.WorkItemRecord{}, fmt.Errorf("lifecycle work: work_item_id, objective, and assigned_agent_id are required")
	}
	work.OwnerID, work.ComputerID, work.TrajectoryID = ownerID, computerID, trajectoryID
	work.Status, work.ResultRef = types.WorkItemOpen, ""
	work.ObjectiveFingerprint = objectgraph.SHA256([]byte(work.Objective))
	work.CreatedByRunID = ""
	if work.CreatedAt.IsZero() {
		work.CreatedAt = now
	}
	work.UpdatedAt = now
	return work, nil
}

func (s *Store) requireLifecycleAssignedAgent(ctx context.Context, ownerID, computerID, agentID string) (types.AgentRecord, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return types.AgentRecord{}, ErrLifecycleInvalidTransition
	}
	agent, err := s.GetAgentByScope(ctx, ownerID, computerID, agentID)
	if err != nil {
		return types.AgentRecord{}, err
	}
	switch strings.TrimSpace(agent.Profile) {
	case "texture", "researcher", "processor", "reconciler":
	default:
		return types.AgentRecord{}, ErrLifecycleInvalidTransition
	}
	if strings.TrimSpace(agent.Role) != strings.TrimSpace(agent.Profile) {
		return types.AgentRecord{}, ErrLifecycleInvalidTransition
	}
	if strings.HasPrefix(agentID, "texture:") && agent.LifecycleVersion <= 0 {
		return types.AgentRecord{}, ErrLifecycleInvalidTransition
	}
	return agent, nil
}

func lifecycleWorkFingerprintAvailable(snapshot types.LifecycleSnapshot, workItemID, fingerprint string) bool {
	for _, existing := range snapshot.WorkItems {
		if existing.WorkItemID != workItemID && existing.ObjectiveFingerprint == fingerprint && existing.Status == types.WorkItemOpen {
			return false
		}
	}
	return true
}

func (s *Store) OpenLifecycleWork(ctx context.Context, req types.OpenLifecycleWorkRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest, req.TrajectoryID = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest), strings.TrimSpace(req.TrajectoryID)
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	computedDigest, digestErr := ComputeOpenLifecycleWorkDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if trajectory.Status != types.TrajectoryLive {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	now := time.Now().UTC()
	work, err := normalizeLifecycleWork(req.WorkItem, ownerID, computerID, req.TrajectoryID, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	assignedAgent, agentErr := s.requireLifecycleAssignedAgent(ctx, ownerID, computerID, work.AssignedAgentID)
	var resultAgent *types.AgentRecord
	if errors.Is(agentErr, ErrNotFound) {
		switch strings.TrimSpace(work.AuthorityProfile) {
		case "researcher", "processor", "reconciler":
		default:
			return types.LifecycleResult{}, fmt.Errorf("lifecycle open work: assigned agent profile: %w", ErrLifecycleInvalidTransition)
		}
		if strings.HasPrefix(work.AssignedAgentID, "texture:") {
			return types.LifecycleResult{}, fmt.Errorf("lifecycle open work: assigned agent: %w", ErrLifecycleInvalidTransition)
		}
	} else if agentErr != nil {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle open work: assigned agent: %w", agentErr)
	} else {
		if strings.TrimSpace(work.AuthorityProfile) != strings.TrimSpace(assignedAgent.Profile) {
			return types.LifecycleResult{}, fmt.Errorf("lifecycle open work: authority profile does not match assigned agent: %w", ErrLifecycleInvalidTransition)
		}
		resultAgent = &assignedAgent
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if !lifecycleWorkFingerprintAvailable(snapshot, work.WorkItemID, work.ObjectiveFingerprint) {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	nextSeq := trajectory.ReducerSeq + 1
	work.LifecycleVersion, work.LastReducerSeq = 1, nextSeq
	trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = nextSeq, trajectory.LifecycleVersion+1, now
	event := types.LifecycleEvent{
		EventID: req.CommandID + ":1", OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: req.TrajectoryID, WorkItemID: work.WorkItemID, Kind: types.LifecycleWorkOpened,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: nextSeq,
		CommandID: req.CommandID, CommandDigest: req.CommandDigest, CreatedAt: now,
	}
	workObj, err := lifecycleObject(ogKindWorkItem, ownerID, computerID, work.WorkItemID, work, lifecycleMetadata("work_item_id", work.WorkItemID, computerID, req.TrajectoryID, nextSeq), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	trajectoryUpdated, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, nextSeq), trajectoryObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, nextSeq), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleOpenWork, nextSeq, []objectgraph.Object{eventObj})
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		{CanonicalID: workObj.CanonicalID}, {CanonicalID: eventObj.CanonicalID}, {CanonicalID: receiptObj.CanonicalID},
	}
	objects := []objectgraph.Object{trajectoryUpdated, workObj, eventObj, receiptObj}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects, types.LifecycleResult{
		Receipt: receipt, Trajectory: trajectory, WorkItem: &work, Agent: resultAgent, Events: []types.LifecycleEvent{event},
	})
}

// ReplaceLifecycleActivation atomically advances the durable subject to a new
// ephemeral run and records that run in the same object-graph transaction.
func (s *Store) ReplaceLifecycleActivation(ctx context.Context, req types.ReplaceLifecycleActivationRequest) (types.LifecycleResult, error) {
	return s.projectLifecycleRun(ctx, req)
}

// ProjectTerminalLifecycleRun records the terminal state of an activation after
// its trajectory has already become terminal. It updates only the run
// projection: trajectory, work, update, event, and receipt authority are
// deliberately untouched.
func (s *Store) ProjectTerminalLifecycleRun(ctx context.Context, req types.ReplaceLifecycleActivationRequest) (types.LifecycleResult, error) {
	return s.projectLifecycleRun(ctx, req)
}

func (s *Store) projectLifecycleRun(ctx context.Context, req types.ReplaceLifecycleActivationRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.TrajectoryID, req.AgentID = strings.TrimSpace(req.TrajectoryID), strings.TrimSpace(req.AgentID)
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	computedDigest, digestErr := ComputeReplaceLifecycleActivationDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}
	run := req.Run
	run.RunID, run.OwnerID, run.AgentID = strings.TrimSpace(run.RunID), strings.TrimSpace(run.OwnerID), strings.TrimSpace(run.AgentID)
	run.SandboxID, run.TrajectoryID = strings.TrimSpace(run.SandboxID), strings.TrimSpace(run.TrajectoryID)
	if run.RunID == "" || run.OwnerID != ownerID || run.SandboxID != computerID ||
		run.AgentID != req.AgentID || run.TrajectoryID != req.TrajectoryID || !run.State.Valid() {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}

	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if trajectory.Status == types.TrajectoryCancelled && run.State != types.RunCancelled {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	if trajectory.Status != types.TrajectoryLive && run.State.Active() {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	runCanonicalID, err := lifecycleCanonicalID(ogKindRun, ownerID, computerID, run.RunID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	runCondition := objectgraph.ObjectCondition{CanonicalID: runCanonicalID}
	existingRunObj, getRunErr := s.lifecycleGraph().GetObject(ctx, runCanonicalID)
	if getRunErr == nil {
		storedRun, decodeErr := decodeLifecycleObject[types.RunRecord](existingRunObj)
		if decodeErr != nil {
			return types.LifecycleResult{}, decodeErr
		}
		if storedRun.OwnerID != ownerID || storedRun.SandboxID != computerID ||
			storedRun.TrajectoryID != req.TrajectoryID || storedRun.AgentID != req.AgentID {
			return types.LifecycleResult{}, ErrLifecycleCommandConflict
		}
		if storedRun.State.Terminal() && storedRun.State != run.State &&
			!(trajectory.Status == types.TrajectoryCancelled && run.State == types.RunCancelled) {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		run.CreatedAt = storedRun.CreatedAt
		runCondition.Exists, runCondition.ExpectedContentHash = true, existingRunObj.ContentHash
	} else if !errors.Is(getRunErr, objectgraph.ErrNotFound) {
		return types.LifecycleResult{}, getRunErr
	} else if trajectory.Status != types.TrajectoryLive {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}

	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		runCondition,
	}
	if errors.Is(getRunErr, objectgraph.ErrNotFound) {
		agentObj, agentErr := s.lifecycleGetObject(ctx, ogKindAgent, ownerID, computerID, req.AgentID)
		if agentErr == nil {
			agent, decodeErr := decodeLifecycleObject[types.AgentRecord](agentObj)
			if decodeErr != nil {
				return types.LifecycleResult{}, decodeErr
			}
			if agent.OwnerID != ownerID || agent.ComputerID != computerID || agent.AgentID != req.AgentID ||
				strings.TrimSpace(agent.Profile) != strings.TrimSpace(run.AgentProfile) ||
				strings.TrimSpace(agent.Role) != strings.TrimSpace(run.AgentRole) ||
				(agent.Profile == "texture" && agent.LifecycleVersion <= 0) {
				return types.LifecycleResult{}, ErrLifecycleInvalidTransition
			}
			conditions = append(conditions, objectgraph.ObjectCondition{
				CanonicalID: agentObj.CanonicalID, Exists: true, ExpectedContentHash: agentObj.ContentHash,
			})
		} else if errors.Is(agentErr, ErrNotFound) {
			workItemIDValue, _ := run.Metadata["lifecycle_work_item_id"].(string)
			workItemID := strings.TrimSpace(workItemIDValue)
			if workItemID == "" {
				return types.LifecycleResult{}, ErrLifecycleInvalidTransition
			}
			workObj, work, workErr := s.lifecycleWorkObject(ctx, ownerID, computerID, workItemID)
			if workErr != nil {
				return types.LifecycleResult{}, workErr
			}
			if work.TrajectoryID != req.TrajectoryID || work.Status != types.WorkItemOpen ||
				strings.TrimSpace(work.AssignedAgentID) != req.AgentID {
				return types.LifecycleResult{}, ErrLifecycleInvalidTransition
			}
			conditions = append(conditions, objectgraph.ObjectCondition{
				CanonicalID: workObj.CanonicalID, Exists: true, ExpectedContentHash: workObj.ContentHash,
			})
		} else {
			return types.LifecycleResult{}, agentErr
		}
	}

	runMetadata := map[string]any{
		"run_id": run.RunID, "agent_id": run.AgentID, "channel_id": run.ChannelID,
		"requested_by_run_id": run.RequestedByRunID, "trajectory_id": run.TrajectoryID,
		"agent_profile": run.AgentProfile, "agent_role": run.AgentRole, "sandbox_id": run.SandboxID,
		"state": string(run.State), "created_at": run.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at": run.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	runBody, err := json.Marshal(run)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	runMetadataJSON, err := objectgraph.NormalizeMetadata(runMetadata)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	runObj := objectgraph.Object{
		CanonicalID: runCanonicalID, ObjectKind: ogKindRun, OwnerID: ownerID, ComputerID: computerID,
		ContentHash: objectgraph.ContentHash(ogKindRun, runBody, runMetadataJSON), Body: runBody, Metadata: runMetadataJSON,
		CreatedAt: run.CreatedAt.UTC(), UpdatedAt: run.UpdatedAt.UTC(),
	}
	if getRunErr == nil {
		runObj.CreatedAt = existingRunObj.CreatedAt
		if runObj.ContentHash == existingRunObj.ContentHash {
			return types.LifecycleResult{Trajectory: trajectory, Replay: true}, nil
		}
	}
	if err := s.ogStore.PutBatchConditional(ctx, conditions, objectgraph.Batch{Objects: []objectgraph.Object{runObj}}); err != nil {
		if errors.Is(err, objectgraph.ErrConflict) {
			return types.LifecycleResult{}, ErrConcurrentStateChange
		}
		return types.LifecycleResult{}, err
	}
	return types.LifecycleResult{Trajectory: trajectory}, nil
}

func (s *Store) AmendLifecycleWork(ctx context.Context, req types.AmendLifecycleWorkRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest, req.TrajectoryID, req.WorkItemID = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest), strings.TrimSpace(req.TrajectoryID), strings.TrimSpace(req.WorkItemID)
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	computedDigest, digestErr := ComputeAmendLifecycleWorkDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}

	if req.WorkItemID == "" || req.ExpectedLifecycleVersion <= 0 {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle amend work: work_item_id and expected_lifecycle_version are required")
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	workObj, existing, err := s.lifecycleWorkObject(ctx, ownerID, computerID, req.WorkItemID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if trajectory.Status != types.TrajectoryLive || existing.TrajectoryID != req.TrajectoryID || existing.LifecycleVersion != req.ExpectedLifecycleVersion || existing.Status != types.WorkItemOpen {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}
	now := time.Now().UTC()
	work, err := normalizeLifecycleWork(req.WorkItem, ownerID, computerID, req.TrajectoryID, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if work.WorkItemID != req.WorkItemID {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	if _, err := s.requireLifecycleAssignedAgent(ctx, ownerID, computerID, work.AssignedAgentID); err != nil {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle amend work: assigned agent: %w", err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if !lifecycleWorkFingerprintAvailable(snapshot, work.WorkItemID, work.ObjectiveFingerprint) {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	nextSeq := trajectory.ReducerSeq + 1
	work.CreatedAt = existing.CreatedAt
	work.LifecycleVersion, work.LastReducerSeq = existing.LifecycleVersion+1, nextSeq
	trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = nextSeq, trajectory.LifecycleVersion+1, now
	event := types.LifecycleEvent{
		EventID: req.CommandID + ":1", OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: req.TrajectoryID, WorkItemID: work.WorkItemID, Kind: types.LifecycleWorkAmended,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: nextSeq,
		CommandID: req.CommandID, CommandDigest: req.CommandDigest, Reason: work.Reason, CreatedAt: now,
	}
	workUpdated, err := lifecycleObject(ogKindWorkItem, ownerID, computerID, work.WorkItemID, work, lifecycleMetadata("work_item_id", work.WorkItemID, computerID, req.TrajectoryID, nextSeq), workObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	trajectoryUpdated, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, nextSeq), trajectoryObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, nextSeq), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleAmendWork, nextSeq, []objectgraph.Object{eventObj})
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		{CanonicalID: workObj.CanonicalID, Exists: true, ExpectedContentHash: workObj.ContentHash},
		{CanonicalID: eventObj.CanonicalID}, {CanonicalID: receiptObj.CanonicalID},
	}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, []objectgraph.Object{trajectoryUpdated, workUpdated, eventObj, receiptObj}, types.LifecycleResult{
		Receipt: receipt, Trajectory: trajectory, WorkItem: &work, Events: []types.LifecycleEvent{event},
	})
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

func (s *Store) RecordLifecycleRefs(ctx context.Context, req types.RecordLifecycleRefsRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest, req.TrajectoryID, req.WorkItemID = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest), strings.TrimSpace(req.TrajectoryID), strings.TrimSpace(req.WorkItemID)
	req.ArtifactRefs, req.EvidenceRefs = normalizeLifecycleRefs(req.ArtifactRefs), normalizeLifecycleRefs(req.EvidenceRefs)
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	computedDigest, digestErr := ComputeRecordLifecycleRefsDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}
	if len(req.ArtifactRefs) == 0 && len(req.EvidenceRefs) == 0 && len(req.SubjectRefs) == 0 {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle record refs: artifact_refs, evidence_refs, or subject_refs are required")
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if trajectory.Status != types.TrajectoryLive {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	if req.WorkItemID != "" {
		work, workErr := s.GetLifecycleWorkItem(ctx, ownerID, computerID, req.WorkItemID)
		if workErr != nil {
			return types.LifecycleResult{}, workErr
		}
		if work.TrajectoryID != req.TrajectoryID {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
	}
	now := time.Now().UTC()
	if trajectory.SubjectRefs == nil {
		trajectory.SubjectRefs = make(map[string]string)
	}
	for key, value := range req.SubjectRefs {
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if key == "" || value == "" {
			return types.LifecycleResult{}, fmt.Errorf("lifecycle record refs: subject ref keys and values must be non-empty")
		}
		trajectory.SubjectRefs[key] = value
	}
	nextSeq := trajectory.ReducerSeq + 1
	trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = nextSeq, trajectory.LifecycleVersion+1, now
	event := types.LifecycleEvent{
		EventID: req.CommandID + ":1", OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: req.TrajectoryID, WorkItemID: req.WorkItemID, Kind: types.LifecycleRefsRecorded,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: nextSeq,
		CommandID: req.CommandID, CommandDigest: req.CommandDigest,
		ArtifactRefs: req.ArtifactRefs, EvidenceRefs: req.EvidenceRefs, Reason: strings.TrimSpace(req.Reason), CreatedAt: now,
	}
	trajectoryUpdated, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, nextSeq), trajectoryObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, nextSeq), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleRecordRefs, nextSeq, []objectgraph.Object{eventObj})
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		{CanonicalID: eventObj.CanonicalID}, {CanonicalID: receiptObj.CanonicalID},
	}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, []objectgraph.Object{trajectoryUpdated, eventObj, receiptObj}, types.LifecycleResult{
		Receipt: receipt, Trajectory: trajectory, Events: []types.LifecycleEvent{event},
	})
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

func (s *Store) QueueLifecycleUpdate(ctx context.Context, req types.QueueLifecycleUpdateRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.TrajectoryID, req.UpdateID = strings.TrimSpace(req.TrajectoryID), strings.TrimSpace(req.UpdateID)
	req.TargetAgentID, req.ProducerAgentID = strings.TrimSpace(req.TargetAgentID), strings.TrimSpace(req.ProducerAgentID)
	req.ProducerUpdateID, req.PayloadDigest = strings.TrimSpace(req.ProducerUpdateID), strings.TrimSpace(req.PayloadDigest)
	req.WorkItemID = strings.TrimSpace(req.WorkItemID)
	req.WorkDisposition, err = normalizeUpdateWorkDisposition(req.WorkDisposition)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if err := validateUpdateWorkConsequence(req.WorkDisposition, req.WorkItemID, "lifecycle queue update"); err != nil {
		return types.LifecycleResult{}, err
	}
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	if req.UpdateID == "" || req.TargetAgentID == "" || req.ProducerAgentID == "" || req.ProducerUpdateID == "" || req.PayloadDigest == "" {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle queue update: update_id, target_agent_id, producer_agent_id, producer_update_id, and payload_digest are required")
	}
	payloadDigest, digestErr := ComputeLifecycleUpdatePayloadDigest(req.Packet, req.Content)
	if digestErr != nil {
		return types.LifecycleResult{}, digestErr
	}
	if payloadDigest != req.PayloadDigest {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle queue update: payload digest mismatch: %w", ErrLifecycleCommandConflict)
	}
	computedCommandDigest, commandDigestErr := ComputeQueueLifecycleUpdateDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computedCommandDigest, commandDigestErr); err != nil {
		return types.LifecycleResult{}, err
	}

	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	agentObj, err := s.lifecycleGetObject(ctx, ogKindAgent, ownerID, computerID, req.TargetAgentID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	agent, err := decodeLifecycleObject[types.AgentRecord](agentObj)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if agent.LifecycleVersion <= 0 || agent.Profile != "texture" || agent.Role != "texture" {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	updateKey := req.TrajectoryID + "\x00" + req.TargetAgentID + "\x00" + req.ProducerAgentID + "\x00" + req.ProducerUpdateID
	updateCanonicalID, err := lifecycleCanonicalID(ogKindWorkerUpdate, ownerID, computerID, updateKey)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if existingObj, getErr := s.lifecycleGraph().GetObject(ctx, updateCanonicalID); getErr == nil {
		stored, decodeErr := decodeLifecycleObject[types.CoagentSourcePacket](existingObj)
		if decodeErr != nil {
			return types.LifecycleResult{}, decodeErr
		}
		if stored.PayloadDigest != req.PayloadDigest || strings.TrimSpace(stored.WorkItemID) != req.WorkItemID ||
			stored.WorkDisposition != req.WorkDisposition {
			return types.LifecycleResult{}, ErrLifecycleCommandConflict
		}
		return types.LifecycleResult{Trajectory: trajectory, Agent: &agent, Update: &stored, Replay: true}, nil
	} else if !errors.Is(getErr, objectgraph.ErrNotFound) {
		return types.LifecycleResult{}, getErr
	}
	now := time.Now().UTC()
	if trajectory.Status != types.TrajectoryLive {
		nextSeq, sequenceUpdated, sequenceCondition, sequenceErr := s.nextPostTerminalSequence(ctx, ownerID, computerID, trajectory, now)
		if sequenceErr != nil {
			return types.LifecycleResult{}, sequenceErr
		}
		update := types.CoagentSourcePacket{
			UpdateID: req.UpdateID, ProducerUpdateID: req.ProducerUpdateID,
			OwnerID: ownerID, ComputerID: computerID, AgentID: req.ProducerAgentID,
			TargetAgentID: req.TargetAgentID, TrajectoryID: req.TrajectoryID,
			ChannelID: req.ChannelID, Role: req.Role, SourceRunID: req.SourceRunID,
			WorkItemID: req.WorkItemID, WorkDisposition: req.WorkDisposition,
			MessageSeq: nextSeq, PayloadDigest: req.PayloadDigest,
			Disposition: types.UpdateLate, DispositionRef: lifecycleTerminalTrajectoryRef(req.TrajectoryID),
			DispositionReason: "trajectory is terminal", LifecycleVersion: 1, ReducerSeq: nextSeq,
			Packet: req.Packet, Content: req.Content, CreatedAt: now,
		}
		event := types.LifecycleEvent{
			EventID: req.CommandID + ":1", OwnerID: ownerID, ComputerID: computerID,
			TrajectoryID: req.TrajectoryID, UpdateID: req.UpdateID, Kind: types.LifecycleUpdateLate,
			ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: nextSeq,
			CommandID: req.CommandID, CommandDigest: req.CommandDigest,
			Reason: update.DispositionReason, CreatedAt: now,
		}
		updateMeta := lifecycleMetadata("update_id", req.UpdateID, computerID, req.TrajectoryID, nextSeq)
		updateMeta["producer_update_id"], updateMeta["target_agent_id"] = req.ProducerUpdateID, req.TargetAgentID
		updateObj, buildErr := lifecycleObject(ogKindWorkerUpdate, ownerID, computerID, updateKey, update, updateMeta, now, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		eventObj, buildErr := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, nextSeq), now, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		receipt, receiptObj, buildErr := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleQueueUpdate, nextSeq, []objectgraph.Object{eventObj})
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		conditions := []objectgraph.ObjectCondition{
			sequenceCondition,
			{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
			{CanonicalID: updateObj.CanonicalID}, {CanonicalID: eventObj.CanonicalID}, {CanonicalID: receiptObj.CanonicalID},
		}
		return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, []objectgraph.Object{sequenceUpdated, updateObj, eventObj, receiptObj}, types.LifecycleResult{
			Receipt: receipt, Trajectory: trajectory, Agent: &agent, Update: &update, Events: []types.LifecycleEvent{event},
		})
	}
	var workCondition *objectgraph.ObjectCondition
	if req.WorkItemID != "" {
		workObj, work, workErr := s.lifecycleWorkObject(ctx, ownerID, computerID, req.WorkItemID)
		if workErr != nil {
			return types.LifecycleResult{}, workErr
		}
		if work.TrajectoryID != req.TrajectoryID || work.Status != types.WorkItemOpen ||
			strings.TrimSpace(work.AssignedAgentID) != req.ProducerAgentID {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		condition := objectgraph.ObjectCondition{
			CanonicalID: workObj.CanonicalID, Exists: true, ExpectedContentHash: workObj.ContentHash,
		}
		workCondition = &condition
	}
	nextSeq := trajectory.ReducerSeq + 1
	update := types.CoagentSourcePacket{
		UpdateID: req.UpdateID, ProducerUpdateID: req.ProducerUpdateID,
		OwnerID: ownerID, ComputerID: computerID, AgentID: req.ProducerAgentID,
		TargetAgentID: req.TargetAgentID, TrajectoryID: req.TrajectoryID,
		ChannelID: req.ChannelID, Role: req.Role, SourceRunID: req.SourceRunID,
		WorkItemID: req.WorkItemID, WorkDisposition: req.WorkDisposition,
		PayloadDigest: req.PayloadDigest, Disposition: types.UpdatePending,
		MessageSeq:       nextSeq,
		LifecycleVersion: 1, ReducerSeq: nextSeq, Packet: req.Packet,
		Content: req.Content, CreatedAt: now,
	}
	trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = nextSeq, trajectory.LifecycleVersion+1, now
	agent.LastReducerSeq, agent.LifecycleVersion, agent.UpdatedAt = nextSeq, agent.LifecycleVersion+1, now
	event := types.LifecycleEvent{
		EventID: req.CommandID + ":1", OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: req.TrajectoryID, UpdateID: req.UpdateID, Kind: types.LifecycleUpdateQueued,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: nextSeq,
		CommandID: req.CommandID, CommandDigest: req.CommandDigest, CreatedAt: now,
	}
	trajectoryUpdated, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, nextSeq), trajectoryObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	agentUpdated, err := lifecycleObject(ogKindAgent, ownerID, computerID, agent.AgentID, agent, lifecycleMetadata("agent_id", agent.AgentID, computerID, req.TrajectoryID, nextSeq), agentObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	updateMeta := lifecycleMetadata("update_id", req.UpdateID, computerID, req.TrajectoryID, nextSeq)
	updateMeta["producer_update_id"] = req.ProducerUpdateID
	updateMeta["target_agent_id"] = req.TargetAgentID
	updateObj, err := lifecycleObject(ogKindWorkerUpdate, ownerID, computerID, updateKey, update, updateMeta, now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, nextSeq), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleQueueUpdate, nextSeq, []objectgraph.Object{eventObj})
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		{CanonicalID: agentObj.CanonicalID, Exists: true, ExpectedContentHash: agentObj.ContentHash},
		{CanonicalID: updateObj.CanonicalID}, {CanonicalID: eventObj.CanonicalID}, {CanonicalID: receiptObj.CanonicalID},
	}
	if workCondition != nil {
		conditions = append(conditions, *workCondition)
	}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, []objectgraph.Object{trajectoryUpdated, agentUpdated, updateObj, eventObj, receiptObj}, types.LifecycleResult{Receipt: receipt, Trajectory: trajectory, Agent: &agent, Update: &update, Events: []types.LifecycleEvent{event}})
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

// ApplyLifecycleUpdate incorporates one producer-scoped update exactly once.
// CommitLifecycleArtifactHead advances a live lifecycle artifact head under
// trajectory/document/head CAS and emits the durable reducer event atomically.
func (s *Store) CommitLifecycleArtifactHead(ctx context.Context, req types.CommitLifecycleArtifactHeadRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.TrajectoryID, req.ExpectedHeadRevisionID = strings.TrimSpace(req.TrajectoryID), strings.TrimSpace(req.ExpectedHeadRevisionID)
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	if req.ExpectedLifecycleVersion <= 0 || req.ExpectedHeadRevisionID == "" {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle commit head: expected lifecycle version and head are required")
	}
	computedDigest, digestErr := ComputeCommitLifecycleArtifactHeadDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if trajectory.LifecycleVersion != req.ExpectedLifecycleVersion {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}
	if (trajectory.Status == types.TrajectoryLive) == req.Unbound {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	docID := strings.TrimSpace(trajectory.SubjectRefs["doc_id"])
	if docID == "" {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	documentObj, err := s.lifecycleGetObject(ctx, ogKindTexDoc, ownerID, computerID, docID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	document, err := decodeLifecycleObject[types.Document](documentObj)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if document.ArchivedAt != nil || document.TrajectoryID != req.TrajectoryID || document.CurrentRevisionID != req.ExpectedHeadRevisionID {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}
	headObj, err := s.lifecycleGetObject(ctx, ogKindTexRev, ownerID, computerID, req.ExpectedHeadRevisionID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	head, err := decodeLifecycleObject[types.Revision](headObj)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	now := time.Now().UTC()
	revision := req.Revision
	revision.OwnerID, revision.ComputerID = ownerID, computerID
	if req.Unbound {
		revision.TrajectoryID = ""
	} else {
		revision.TrajectoryID = req.TrajectoryID
	}
	revision.DocID, revision.ParentRevisionID = docID, req.ExpectedHeadRevisionID
	revision.CreatedAt = now
	revision, _, _, err = prepareTextureRevisionV2(revision)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	document, revision, err = commitTextureHeadAuthority(document, &head, revision, now)
	if errors.Is(err, ErrStaleDocumentHead) {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if req.Unbound {
		return s.commitUnboundTextureArtifactHead(ctx, req, trajectoryObj, trajectory, documentObj, document, headObj, revision, now)
	}
	nextSeq := trajectory.ReducerSeq + 1
	trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = nextSeq, trajectory.LifecycleVersion+1, now
	revisionMeta := lifecycleMetadata("revision_id", revision.RevisionID, computerID, req.TrajectoryID, nextSeq)
	revisionMeta["doc_id"], revisionMeta["revision_hash"] = docID, revision.RevisionHash
	revisionObj, err := lifecycleObject(ogKindTexRev, ownerID, computerID, revision.RevisionID, revision, revisionMeta, now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	documentMeta := lifecycleMetadata("doc_id", docID, computerID, req.TrajectoryID, nextSeq)
	documentMeta["current_revision_id"] = revision.RevisionID
	documentUpdated, err := lifecycleObject(ogKindTexDoc, ownerID, computerID, docID, document, documentMeta, documentObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	trajectoryUpdated, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, nextSeq), trajectoryObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	event := types.LifecycleEvent{
		EventID: req.CommandID + ":" + fmt.Sprintf("%d", nextSeq), OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: req.TrajectoryID, Kind: types.LifecycleArtifactHeadAdvanced,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: nextSeq,
		CommandID: req.CommandID, CommandDigest: req.CommandDigest,
		ArtifactRefs: []string{docID, revision.RevisionID}, CreatedAt: now,
	}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, nextSeq), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleCommitArtifactHead, nextSeq, []objectgraph.Object{eventObj})
	if err != nil {
		return types.LifecycleResult{}, err
	}
	edgeMetadata := json.RawMessage(`{}`)
	documentEdgeID, err := objectgraph.BuildEdgeID(revisionObj.CanonicalID, documentUpdated.CanonicalID, ogEdgeDocRevision, edgeMetadata)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	parentEdgeID, err := objectgraph.BuildEdgeID(revisionObj.CanonicalID, headObj.CanonicalID, ogEdgeRevParent, edgeMetadata)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		{CanonicalID: documentObj.CanonicalID, Exists: true, ExpectedContentHash: documentObj.ContentHash},
		{CanonicalID: headObj.CanonicalID, Exists: true, ExpectedContentHash: headObj.ContentHash},
		{CanonicalID: revisionObj.CanonicalID}, {CanonicalID: eventObj.CanonicalID}, {CanonicalID: receiptObj.CanonicalID},
	}
	result := types.LifecycleResult{Receipt: receipt, Trajectory: trajectory, Document: &document, Revision: &revision, Events: []types.LifecycleEvent{event}}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions,
		[]objectgraph.Object{trajectoryUpdated, documentUpdated, revisionObj, eventObj, receiptObj}, result,
		objectgraph.Edge{EdgeID: documentEdgeID, FromID: revisionObj.CanonicalID, ToID: documentUpdated.CanonicalID, Kind: ogEdgeDocRevision, Metadata: edgeMetadata, CreatedAt: now},
		objectgraph.Edge{EdgeID: parentEdgeID, FromID: revisionObj.CanonicalID, ToID: headObj.CanonicalID, Kind: ogEdgeRevParent, Metadata: edgeMetadata, CreatedAt: now},
	)
}

func (s *Store) commitUnboundTextureArtifactHead(
	ctx context.Context,
	req types.CommitLifecycleArtifactHeadRequest,
	trajectoryObj objectgraph.Object,
	trajectory types.TrajectoryRecord,
	documentObj objectgraph.Object,
	document types.Document,
	headObj objectgraph.Object,
	revision types.Revision,
	now time.Time,
) (types.LifecycleResult, error) {
	ownerID, computerID := req.OwnerID, req.ComputerID
	nextSeq, sequenceUpdated, sequenceCondition, sequenceErr := s.nextPostTerminalSequence(ctx, ownerID, computerID, trajectory, now)
	if sequenceErr != nil {
		return types.LifecycleResult{}, sequenceErr
	}
	revisionMeta := lifecycleMetadata("revision_id", revision.RevisionID, computerID, "", nextSeq)
	revisionMeta["doc_id"], revisionMeta["revision_hash"] = document.DocID, revision.RevisionHash
	revisionObj, err := lifecycleObject(ogKindTexRev, ownerID, computerID, revision.RevisionID, revision, revisionMeta, now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	documentMeta := lifecycleMetadata("doc_id", document.DocID, computerID, req.TrajectoryID, nextSeq)
	documentMeta["current_revision_id"] = revision.RevisionID
	documentUpdated, err := lifecycleObject(ogKindTexDoc, ownerID, computerID, document.DocID, document, documentMeta, documentObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	event := types.LifecycleEvent{
		EventID: req.CommandID + ":" + fmt.Sprintf("%d", nextSeq), OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: req.TrajectoryID, Kind: types.LifecycleArtifactHeadAdvanced,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: nextSeq,
		CommandID: req.CommandID, CommandDigest: req.CommandDigest,
		ArtifactRefs: []string{document.DocID, revision.RevisionID}, CreatedAt: now,
	}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, nextSeq), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(
		now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest,
		types.LifecycleCommitArtifactHead, nextSeq, []objectgraph.Object{eventObj},
	)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	edgeMetadata := json.RawMessage(`{}`)
	documentEdgeID, err := objectgraph.BuildEdgeID(revisionObj.CanonicalID, documentUpdated.CanonicalID, ogEdgeDocRevision, edgeMetadata)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	parentEdgeID, err := objectgraph.BuildEdgeID(revisionObj.CanonicalID, headObj.CanonicalID, ogEdgeRevParent, edgeMetadata)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		{CanonicalID: documentObj.CanonicalID, Exists: true, ExpectedContentHash: documentObj.ContentHash},
		{CanonicalID: headObj.CanonicalID, Exists: true, ExpectedContentHash: headObj.ContentHash},
		{CanonicalID: revisionObj.CanonicalID},
		{CanonicalID: eventObj.CanonicalID},
		{CanonicalID: receiptObj.CanonicalID},
		sequenceCondition,
	}
	result := types.LifecycleResult{
		Receipt: receipt, Trajectory: trajectory, Document: &document, Revision: &revision,
		Events: []types.LifecycleEvent{event},
	}
	return s.commitLifecycleTransition(
		ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions,
		[]objectgraph.Object{documentUpdated, revisionObj, eventObj, receiptObj, sequenceUpdated}, result,
		objectgraph.Edge{EdgeID: documentEdgeID, FromID: revisionObj.CanonicalID, ToID: documentUpdated.CanonicalID, Kind: ogEdgeDocRevision, Metadata: edgeMetadata, CreatedAt: now},
		objectgraph.Edge{EdgeID: parentEdgeID, FromID: revisionObj.CanonicalID, ToID: headObj.CanonicalID, Kind: ogEdgeRevParent, Metadata: edgeMetadata, CreatedAt: now},
	)
}

func (s *Store) ApplyLifecycleUpdate(ctx context.Context, req types.ApplyLifecycleUpdateRequest) (types.LifecycleResult, error) {
	return s.applyLifecycleUpdate(ctx, req, TextureSourceGraphWriteSet{}, false)
}

func (s *Store) ApplyLifecycleUpdateWithSourceGraph(ctx context.Context, req types.ApplyLifecycleUpdateRequest, graph TextureSourceGraphWriteSet) (types.LifecycleResult, error) {
	return s.applyLifecycleUpdate(ctx, req, graph, true)
}

func (s *Store) applyLifecycleUpdate(ctx context.Context, req types.ApplyLifecycleUpdateRequest, sourceGraph TextureSourceGraphWriteSet, includeSourceGraph bool) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.TrajectoryID, req.UpdateID = strings.TrimSpace(req.TrajectoryID), strings.TrimSpace(req.UpdateID)
	req.TargetAgentID, req.ProducerAgentID = strings.TrimSpace(req.TargetAgentID), strings.TrimSpace(req.ProducerAgentID)
	req.ProducerUpdateID, req.PayloadDigest = strings.TrimSpace(req.ProducerUpdateID), strings.TrimSpace(req.PayloadDigest)
	req.WorkItemID = strings.TrimSpace(req.WorkItemID)
	req.WorkDisposition, err = normalizeUpdateWorkDisposition(req.WorkDisposition)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if err := validateUpdateWorkConsequence(req.WorkDisposition, req.WorkItemID, "lifecycle apply update"); err != nil {
		return types.LifecycleResult{}, err
	}
	req.RelatedUpdates = append([]types.ApplyLifecycleRelatedUpdate(nil), req.RelatedUpdates...)
	seenRelatedUpdates := map[string]struct{}{}
	seenWorkItems := map[string]struct{}{}
	if req.WorkItemID != "" && req.WorkDisposition != types.WorkItemOpen {
		seenWorkItems[req.WorkItemID] = struct{}{}
	}
	for i := range req.RelatedUpdates {
		related := &req.RelatedUpdates[i]
		related.TargetAgentID = strings.TrimSpace(related.TargetAgentID)
		related.ProducerAgentID = strings.TrimSpace(related.ProducerAgentID)
		related.ProducerUpdateID = strings.TrimSpace(related.ProducerUpdateID)
		related.UpdateID = strings.TrimSpace(related.UpdateID)
		related.DispositionRef = strings.TrimSpace(related.DispositionRef)
		related.WorkItemID = strings.TrimSpace(related.WorkItemID)
		related.WorkResultRef = strings.TrimSpace(related.WorkResultRef)
		related.Reason = strings.TrimSpace(related.Reason)
		related.WorkDisposition, err = normalizeUpdateWorkDisposition(related.WorkDisposition)
		if err != nil {
			return types.LifecycleResult{}, err
		}
		if err := validateUpdateWorkConsequence(related.WorkDisposition, related.WorkItemID, "lifecycle apply related update"); err != nil {
			return types.LifecycleResult{}, err
		}
		if related.TargetAgentID == "" || related.ProducerAgentID == "" || related.ProducerUpdateID == "" || related.UpdateID == "" {
			return types.LifecycleResult{}, fmt.Errorf("lifecycle apply related update: target_agent_id, producer_agent_id, producer_update_id, and update_id are required")
		}
		if related.TargetAgentID != req.TargetAgentID ||
			(related.Disposition != types.UpdateIncorporated && related.Disposition != types.UpdateRejected) {
			return types.LifecycleResult{}, fmt.Errorf("lifecycle apply related update must explicitly incorporate or reject an update for the primary target")
		}
		if related.Disposition == types.UpdateRejected && related.Reason == "" {
			return types.LifecycleResult{}, fmt.Errorf("lifecycle apply related rejected update requires reason")
		}
		relatedKey := related.TargetAgentID + "\x00" + related.ProducerAgentID + "\x00" + related.ProducerUpdateID
		if _, duplicate := seenRelatedUpdates[relatedKey]; duplicate ||
			(related.TargetAgentID == req.TargetAgentID && related.ProducerAgentID == req.ProducerAgentID && related.ProducerUpdateID == req.ProducerUpdateID) {
			return types.LifecycleResult{}, fmt.Errorf("lifecycle apply related update is duplicated")
		}
		seenRelatedUpdates[relatedKey] = struct{}{}
		if related.WorkItemID != "" && related.WorkDisposition != types.WorkItemOpen {
			if _, duplicate := seenWorkItems[related.WorkItemID]; duplicate {
				return types.LifecycleResult{}, fmt.Errorf("lifecycle apply related updates reuse terminal work_item_id %q", related.WorkItemID)
			}
			seenWorkItems[related.WorkItemID] = struct{}{}
		}
	}
	sort.Slice(req.RelatedUpdates, func(i, j int) bool {
		left := req.RelatedUpdates[i].TargetAgentID + "\x00" + req.RelatedUpdates[i].ProducerAgentID + "\x00" + req.RelatedUpdates[i].ProducerUpdateID
		right := req.RelatedUpdates[j].TargetAgentID + "\x00" + req.RelatedUpdates[j].ProducerAgentID + "\x00" + req.RelatedUpdates[j].ProducerUpdateID
		return left < right
	})
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	if req.UpdateID == "" || req.TargetAgentID == "" || req.ProducerAgentID == "" || req.ProducerUpdateID == "" || req.PayloadDigest == "" {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle apply update: update_id, target_agent_id, producer_agent_id, producer_update_id, and payload_digest are required")
	}
	payloadDigest, digestErr := ComputeLifecycleUpdatePayloadDigest(req.Packet, req.Content)
	if digestErr != nil {
		return types.LifecycleResult{}, digestErr
	}
	if payloadDigest != req.PayloadDigest {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle apply update: payload digest mismatch: %w", ErrLifecycleCommandConflict)
	}
	var computedCommandDigest string
	var commandDigestErr error
	if includeSourceGraph {
		computedCommandDigest, commandDigestErr = ComputeApplyLifecycleUpdateWithSourceGraphDigest(req, sourceGraph)
	} else {
		computedCommandDigest, commandDigestErr = ComputeApplyLifecycleUpdateDigest(req)
	}
	if err := requireLifecycleDigest(req.CommandDigest, computedCommandDigest, commandDigestErr); err != nil {
		return types.LifecycleResult{}, err
	}
	requestedDisposition := req.Disposition
	if requestedDisposition == "" {
		requestedDisposition = types.UpdateIncorporated
	}
	if requestedDisposition != types.UpdateIncorporated && requestedDisposition != types.UpdateRejected {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle apply update: unsupported disposition %q", requestedDisposition)
	}

	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if trajectory.Status != types.TrajectoryLive {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	agentObj, err := s.lifecycleGetObject(ctx, ogKindAgent, ownerID, computerID, req.TargetAgentID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	agent, err := decodeLifecycleObject[types.AgentRecord](agentObj)
	if err != nil {
		return types.LifecycleResult{}, err
	}

	updateKey := req.TrajectoryID + "\x00" + req.TargetAgentID + "\x00" + req.ProducerAgentID + "\x00" + req.ProducerUpdateID
	updateCanonicalID, err := lifecycleCanonicalID(ogKindWorkerUpdate, ownerID, computerID, updateKey)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	existingUpdate, err := s.lifecycleGraph().GetObject(ctx, updateCanonicalID)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return types.LifecycleResult{}, ErrNotFound
	}
	if err != nil {
		return types.LifecycleResult{}, err
	}
	update, err := decodeLifecycleObject[types.CoagentSourcePacket](existingUpdate)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if update.UpdateID != req.UpdateID || update.PayloadDigest != req.PayloadDigest ||
		update.ProducerUpdateID != req.ProducerUpdateID || update.AgentID != req.ProducerAgentID ||
		update.TargetAgentID != req.TargetAgentID {
		return types.LifecycleResult{}, ErrLifecycleCommandConflict
	}
	expectedWorkDisposition := update.WorkDisposition
	if requestedDisposition == types.UpdateRejected && expectedWorkDisposition != types.WorkItemOpen {
		expectedWorkDisposition = types.WorkItemRefused
	}
	if strings.TrimSpace(update.WorkItemID) != req.WorkItemID ||
		expectedWorkDisposition != req.WorkDisposition {
		return types.LifecycleResult{}, ErrLifecycleCommandConflict
	}
	if update.Disposition != "" && update.Disposition != types.UpdatePending {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	updateCreatedAt := existingUpdate.CreatedAt

	now := time.Now().UTC()
	nextSeq := trajectory.ReducerSeq + 1
	update.Disposition = requestedDisposition
	eventKind := types.LifecycleUpdateApplied
	if requestedDisposition == types.UpdateRejected {
		eventKind = types.LifecycleUpdateRejected
	}
	update.DispositionRef = strings.TrimSpace(req.DispositionRef)
	update.DispositionReason = strings.TrimSpace(req.Reason)
	update.LifecycleVersion++
	update.ReducerSeq = nextSeq

	var artifactObjects []objectgraph.Object
	var artifactConditions []objectgraph.ObjectCondition
	var artifactEdges []objectgraph.Edge
	var resultDocument *types.Document
	var resultRevision *types.Revision
	artifactRefs := []string{}
	if requestedDisposition == types.UpdateIncorporated && req.ReferenceExistingArtifact {
		docID := strings.TrimSpace(trajectory.SubjectRefs["doc_id"])
		documentObj, getErr := s.lifecycleGetObject(ctx, ogKindTexDoc, ownerID, computerID, docID)
		if getErr != nil {
			return types.LifecycleResult{}, getErr
		}
		document, decodeErr := decodeLifecycleObject[types.Document](documentObj)
		if decodeErr != nil {
			return types.LifecycleResult{}, decodeErr
		}
		if document.ArchivedAt != nil {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		if docID == "" || strings.TrimSpace(req.DispositionRef) == "" || document.CurrentRevisionID != strings.TrimSpace(req.DispositionRef) {
			return types.LifecycleResult{}, ErrConcurrentStateChange
		}
		headObj, getErr := s.lifecycleGetObject(ctx, ogKindTexRev, ownerID, computerID, document.CurrentRevisionID)
		if getErr != nil {
			return types.LifecycleResult{}, getErr
		}
		head, decodeErr := decodeLifecycleObject[types.Revision](headObj)
		if decodeErr != nil {
			return types.LifecycleResult{}, decodeErr
		}
		artifactConditions = append(artifactConditions,
			objectgraph.ObjectCondition{CanonicalID: documentObj.CanonicalID, Exists: true, ExpectedContentHash: documentObj.ContentHash},
			objectgraph.ObjectCondition{CanonicalID: headObj.CanonicalID, Exists: true, ExpectedContentHash: headObj.ContentHash},
		)
		artifactRefs = []string{docID, document.CurrentRevisionID}
		resultDocument, resultRevision = &document, &head
	} else if requestedDisposition == types.UpdateIncorporated {
		docID := strings.TrimSpace(trajectory.SubjectRefs["doc_id"])
		if docID == "" || strings.TrimSpace(req.Revision.RevisionID) == "" {
			return types.LifecycleResult{}, fmt.Errorf("lifecycle incorporate update: doc_id subject and revision_id are required")
		}
		documentObj, getErr := s.lifecycleGetObject(ctx, ogKindTexDoc, ownerID, computerID, docID)
		if getErr != nil {
			return types.LifecycleResult{}, getErr
		}
		document, decodeErr := decodeLifecycleObject[types.Document](documentObj)
		if decodeErr != nil {
			return types.LifecycleResult{}, decodeErr
		}
		if document.ArchivedAt != nil {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		headObj, getErr := s.lifecycleGetObject(ctx, ogKindTexRev, ownerID, computerID, document.CurrentRevisionID)
		if getErr != nil {
			return types.LifecycleResult{}, getErr
		}
		head, decodeErr := decodeLifecycleObject[types.Revision](headObj)
		if decodeErr != nil {
			return types.LifecycleResult{}, decodeErr
		}
		revision := req.Revision
		revision.RevisionID, revision.DocID = strings.TrimSpace(revision.RevisionID), docID
		revision.OwnerID, revision.ComputerID, revision.TrajectoryID = ownerID, computerID, req.TrajectoryID
		if revision.ParentRevisionID == "" {
			revision.ParentRevisionID = document.CurrentRevisionID
		}
		document, revision, buildErr := commitTextureHeadAuthority(document, &head, revision, now)
		if errors.Is(buildErr, ErrStaleDocumentHead) {
			return types.LifecycleResult{}, ErrConcurrentStateChange
		}
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		revision.OwnerID, revision.ComputerID, revision.TrajectoryID = ownerID, computerID, req.TrajectoryID
		revisionMeta := lifecycleMetadata("revision_id", revision.RevisionID, computerID, req.TrajectoryID, nextSeq)
		revisionMeta["doc_id"], revisionMeta["revision_hash"] = docID, revision.RevisionHash
		revisionObj, buildErr := lifecycleObject(ogKindTexRev, ownerID, computerID, revision.RevisionID, revision, revisionMeta, now, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		documentMeta := lifecycleMetadata("doc_id", docID, computerID, req.TrajectoryID, nextSeq)
		documentMeta["current_revision_id"] = revision.RevisionID
		documentUpdated, buildErr := lifecycleObject(ogKindTexDoc, ownerID, computerID, docID, document, documentMeta, documentObj.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		edgeMetadata := json.RawMessage(`{}`)
		documentEdgeID, buildErr := objectgraph.BuildEdgeID(revisionObj.CanonicalID, documentUpdated.CanonicalID, ogEdgeDocRevision, edgeMetadata)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		parentEdgeID, buildErr := objectgraph.BuildEdgeID(revisionObj.CanonicalID, headObj.CanonicalID, ogEdgeRevParent, edgeMetadata)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		artifactObjects = []objectgraph.Object{documentUpdated, revisionObj}
		artifactConditions = []objectgraph.ObjectCondition{
			{CanonicalID: documentObj.CanonicalID, Exists: true, ExpectedContentHash: documentObj.ContentHash},
			{CanonicalID: revisionObj.CanonicalID},
		}
		artifactEdges = []objectgraph.Edge{
			{EdgeID: documentEdgeID, FromID: revisionObj.CanonicalID, ToID: documentUpdated.CanonicalID, Kind: ogEdgeDocRevision, Metadata: edgeMetadata, CreatedAt: now},
			{EdgeID: parentEdgeID, FromID: revisionObj.CanonicalID, ToID: headObj.CanonicalID, Kind: ogEdgeRevParent, Metadata: edgeMetadata, CreatedAt: now},
		}
		if includeSourceGraph {
			sourceObjects, sourceConditions, sourceErr := s.lifecycleSourceGraphBatch(ctx, revision, sourceGraph, now)
			if sourceErr != nil {
				return types.LifecycleResult{}, fmt.Errorf("lifecycle incorporate source graph: %w", sourceErr)
			}
			artifactObjects = append(artifactObjects, sourceObjects...)
			artifactConditions = append(artifactConditions, sourceConditions...)
		}
		if update.DispositionRef == "" {
			update.DispositionRef = revision.RevisionID
		} else if update.DispositionRef != revision.RevisionID {
			return types.LifecycleResult{}, ErrLifecycleCommandConflict
		}
		artifactRefs = []string{docID, revision.RevisionID}
		resultDocument, resultRevision = &document, &revision
	} else if requestedDisposition == types.UpdateRejected && update.DispositionRef == "" {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle reject update: disposition_ref is required")
	}
	if trajectory.SubjectRefs == nil {
		trajectory.SubjectRefs = make(map[string]string)
	}
	for key, value := range req.SubjectRefs {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			trajectory.SubjectRefs[key] = strings.TrimSpace(value)
		}
	}
	event := types.LifecycleEvent{
		EventID: req.CommandID + ":1", OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: req.TrajectoryID, UpdateID: req.UpdateID, Kind: eventKind,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: nextSeq,
		CommandID: req.CommandID, CommandDigest: req.CommandDigest,
		ArtifactRefs: artifactRefs, Reason: update.DispositionReason, CreatedAt: now,
	}
	events := []types.LifecycleEvent{event}
	mutationSeq := nextSeq
	var resultWork *types.WorkItemRecord
	if req.WorkItemID != "" && req.WorkDisposition != types.WorkItemOpen {
		if requestedDisposition == types.UpdateIncorporated && req.WorkDisposition != types.WorkItemCompleted {
			return types.LifecycleResult{}, fmt.Errorf("lifecycle incorporate update work consequence requires completed work_disposition")
		}
		if requestedDisposition == types.UpdateRejected && req.WorkDisposition != types.WorkItemRefused {
			return types.LifecycleResult{}, fmt.Errorf("lifecycle reject update work consequence requires refused work_disposition")
		}
	}
	if strings.TrimSpace(req.WorkItemID) != "" && req.WorkDisposition != types.WorkItemOpen {
		workObj, work, getErr := s.lifecycleWorkObject(ctx, ownerID, computerID, strings.TrimSpace(req.WorkItemID))
		if getErr != nil {
			return types.LifecycleResult{}, getErr
		}
		if work.TrajectoryID != req.TrajectoryID || workItemTerminal(work.Status) {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		if strings.TrimSpace(work.AssignedAgentID) != req.ProducerAgentID {
			return types.LifecycleResult{}, fmt.Errorf("lifecycle update work consequence producer %q does not own assigned work %q: %w", req.ProducerAgentID, work.WorkItemID, ErrLifecycleInvalidTransition)
		}
		mutationSeq++
		workEventKind := types.LifecycleWorkSettled
		workEventRefs := []string{}
		if requestedDisposition == types.UpdateRejected {
			work.Status, work.Reason, work.ResultRef = types.WorkItemRefused, strings.TrimSpace(req.Reason), strings.TrimSpace(update.DispositionRef)
			workEventKind = types.LifecycleWorkRefused
			workEventRefs = append(workEventRefs, work.ResultRef)
		} else {
			if strings.TrimSpace(req.WorkResultRef) == "" {
				return types.LifecycleResult{}, fmt.Errorf("lifecycle incorporate update work consequence: work_result_ref is required")
			}
			work.Status, work.ResultRef = types.WorkItemCompleted, strings.TrimSpace(req.WorkResultRef)
			workEventRefs = append(workEventRefs, work.ResultRef)
		}
		work.LifecycleVersion++
		work.LastReducerSeq, work.UpdatedAt = mutationSeq, now
		workUpdated, buildErr := lifecycleObject(ogKindWorkItem, ownerID, computerID, work.WorkItemID, work, lifecycleMetadata("work_item_id", work.WorkItemID, computerID, req.TrajectoryID, mutationSeq), workObj.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		artifactConditions = append(artifactConditions, objectgraph.ObjectCondition{CanonicalID: workObj.CanonicalID, Exists: true, ExpectedContentHash: workObj.ContentHash})
		artifactObjects = append(artifactObjects, workUpdated)
		events = append(events, types.LifecycleEvent{
			EventID: req.CommandID + ":2", OwnerID: ownerID, ComputerID: computerID,
			TrajectoryID: req.TrajectoryID, WorkItemID: work.WorkItemID, UpdateID: req.UpdateID, Kind: workEventKind,
			ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: mutationSeq,
			CommandID: req.CommandID, CommandDigest: req.CommandDigest,
			ArtifactRefs: workEventRefs, Reason: work.Reason, CreatedAt: now,
		})
		resultWork = &work
	}
	for _, related := range req.RelatedUpdates {
		relatedKey := req.TrajectoryID + "\x00" + related.TargetAgentID + "\x00" + related.ProducerAgentID + "\x00" + related.ProducerUpdateID
		relatedCanonicalID, getErr := lifecycleCanonicalID(ogKindWorkerUpdate, ownerID, computerID, relatedKey)
		if getErr != nil {
			return types.LifecycleResult{}, getErr
		}
		relatedObj, getErr := s.lifecycleGraph().GetObject(ctx, relatedCanonicalID)
		if errors.Is(getErr, objectgraph.ErrNotFound) {
			return types.LifecycleResult{}, ErrNotFound
		}
		if getErr != nil {
			return types.LifecycleResult{}, getErr
		}
		relatedUpdate, decodeErr := decodeLifecycleObject[types.CoagentSourcePacket](relatedObj)
		if decodeErr != nil {
			return types.LifecycleResult{}, decodeErr
		}
		if relatedUpdate.UpdateID != related.UpdateID ||
			relatedUpdate.ProducerUpdateID != related.ProducerUpdateID ||
			relatedUpdate.AgentID != related.ProducerAgentID ||
			relatedUpdate.TargetAgentID != related.TargetAgentID ||
			strings.TrimSpace(relatedUpdate.WorkItemID) != related.WorkItemID {
			return types.LifecycleResult{}, ErrLifecycleCommandConflict
		}
		expectedWorkDisposition := relatedUpdate.WorkDisposition
		if related.Disposition == types.UpdateRejected && expectedWorkDisposition != types.WorkItemOpen {
			expectedWorkDisposition = types.WorkItemRefused
		}
		if related.WorkDisposition != expectedWorkDisposition {
			return types.LifecycleResult{}, ErrLifecycleCommandConflict
		}
		if relatedUpdate.Disposition != "" && relatedUpdate.Disposition != types.UpdatePending {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		dispositionRef := related.DispositionRef
		if dispositionRef == "" {
			dispositionRef = strings.TrimSpace(req.DispositionRef)
		}
		if dispositionRef == "" || dispositionRef != strings.TrimSpace(req.DispositionRef) {
			return types.LifecycleResult{}, ErrLifecycleCommandConflict
		}
		mutationSeq++
		relatedUpdate.Disposition = related.Disposition
		relatedUpdate.DispositionRef = dispositionRef
		relatedUpdate.DispositionReason = related.Reason
		relatedUpdate.LifecycleVersion++
		relatedUpdate.ReducerSeq = mutationSeq
		relatedMeta := lifecycleMetadata("update_id", related.UpdateID, computerID, req.TrajectoryID, mutationSeq)
		relatedMeta["producer_update_id"] = related.ProducerUpdateID
		relatedMeta["target_agent_id"] = related.TargetAgentID
		relatedUpdated, buildErr := lifecycleObject(ogKindWorkerUpdate, ownerID, computerID, relatedKey, relatedUpdate, relatedMeta, relatedObj.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		artifactConditions = append(artifactConditions, objectgraph.ObjectCondition{
			CanonicalID: relatedObj.CanonicalID, Exists: true, ExpectedContentHash: relatedObj.ContentHash,
		})
		artifactObjects = append(artifactObjects, relatedUpdated)
		updateEventKind := types.LifecycleUpdateApplied
		if related.Disposition == types.UpdateRejected {
			updateEventKind = types.LifecycleUpdateRejected
		}
		events = append(events, types.LifecycleEvent{
			EventID: req.CommandID + ":" + fmt.Sprintf("%d", len(events)+1),
			OwnerID: ownerID, ComputerID: computerID, TrajectoryID: req.TrajectoryID,
			UpdateID: related.UpdateID, WorkItemID: related.WorkItemID, Kind: updateEventKind,
			ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: mutationSeq,
			CommandID: req.CommandID, CommandDigest: req.CommandDigest,
			ArtifactRefs: artifactRefs, Reason: related.Reason, CreatedAt: now,
		})
		if related.WorkItemID == "" {
			continue
		}
		relatedWorkObj, relatedWork, workErr := s.lifecycleWorkObject(ctx, ownerID, computerID, related.WorkItemID)
		if workErr != nil {
			return types.LifecycleResult{}, workErr
		}
		if relatedWork.TrajectoryID != req.TrajectoryID || workItemTerminal(relatedWork.Status) {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		if strings.TrimSpace(relatedWork.AssignedAgentID) != related.ProducerAgentID {
			return types.LifecycleResult{}, fmt.Errorf("lifecycle related update producer %q does not own assigned work %q: %w", related.ProducerAgentID, relatedWork.WorkItemID, ErrLifecycleInvalidTransition)
		}
		if related.WorkDisposition == types.WorkItemOpen {
			if related.WorkItemID != req.WorkItemID || req.WorkDisposition == types.WorkItemOpen {
				artifactConditions = append(artifactConditions, objectgraph.ObjectCondition{
					CanonicalID: relatedWorkObj.CanonicalID, Exists: true, ExpectedContentHash: relatedWorkObj.ContentHash,
				})
			}
			continue
		}
		artifactConditions = append(artifactConditions, objectgraph.ObjectCondition{
			CanonicalID: relatedWorkObj.CanonicalID, Exists: true, ExpectedContentHash: relatedWorkObj.ContentHash,
		})
		mutationSeq++
		workEventKind := types.LifecycleWorkSettled
		workEventRefs := []string{}
		if related.Disposition == types.UpdateRejected {
			if related.WorkDisposition != types.WorkItemRefused {
				return types.LifecycleResult{}, fmt.Errorf("lifecycle reject related update work consequence requires refused disposition")
			}
			relatedWork.Status, relatedWork.Reason, relatedWork.ResultRef = types.WorkItemRefused, related.Reason, dispositionRef
			workEventKind = types.LifecycleWorkRefused
			workEventRefs = append(workEventRefs, relatedWork.ResultRef)
		} else {
			if related.WorkDisposition != types.WorkItemCompleted || related.WorkResultRef == "" {
				return types.LifecycleResult{}, fmt.Errorf("lifecycle incorporate related update terminal work consequence requires completed disposition and work_result_ref")
			}
			relatedWork.Status, relatedWork.ResultRef = types.WorkItemCompleted, related.WorkResultRef
			workEventRefs = append(workEventRefs, relatedWork.ResultRef)
		}
		relatedWork.LifecycleVersion++
		relatedWork.LastReducerSeq, relatedWork.UpdatedAt = mutationSeq, now
		relatedWorkUpdated, buildErr := lifecycleObject(ogKindWorkItem, ownerID, computerID, relatedWork.WorkItemID, relatedWork, lifecycleMetadata("work_item_id", relatedWork.WorkItemID, computerID, req.TrajectoryID, mutationSeq), relatedWorkObj.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		artifactObjects = append(artifactObjects, relatedWorkUpdated)
		events = append(events, types.LifecycleEvent{
			EventID: req.CommandID + ":" + fmt.Sprintf("%d", len(events)+1),
			OwnerID: ownerID, ComputerID: computerID, TrajectoryID: req.TrajectoryID,
			WorkItemID: relatedWork.WorkItemID, UpdateID: related.UpdateID, Kind: workEventKind,
			ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: mutationSeq,
			CommandID: req.CommandID, CommandDigest: req.CommandDigest,
			ArtifactRefs: workEventRefs, Reason: relatedWork.Reason, CreatedAt: now,
		})
	}
	trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = mutationSeq, trajectory.LifecycleVersion+1, now
	agent.LastReducerSeq, agent.LifecycleVersion, agent.UpdatedAt = mutationSeq, agent.LifecycleVersion+1, now
	trajectoryUpdated, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, trajectory.ReducerSeq), trajectoryObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	agentUpdated, err := lifecycleObject(ogKindAgent, ownerID, computerID, agent.AgentID, agent, lifecycleMetadata("agent_id", agent.AgentID, computerID, req.TrajectoryID, mutationSeq), agentObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	updateMeta := lifecycleMetadata("update_id", req.UpdateID, computerID, req.TrajectoryID, nextSeq)
	updateMeta["producer_update_id"] = req.ProducerUpdateID
	updateMeta["target_agent_id"] = req.TargetAgentID
	updateObj, err := lifecycleObject(ogKindWorkerUpdate, ownerID, computerID, updateKey, update, updateMeta, updateCreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	eventObjs := make([]objectgraph.Object, 0, len(events))
	for _, lifecycleEvent := range events {
		eventObj, buildErr := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, lifecycleEvent.EventID, lifecycleEvent, lifecycleMetadata("event_id", lifecycleEvent.EventID, computerID, req.TrajectoryID, lifecycleEvent.ReducerSeq), now, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		eventObjs = append(eventObjs, eventObj)
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleApplyUpdate, trajectory.ReducerSeq, eventObjs)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		{CanonicalID: agentObj.CanonicalID, Exists: true, ExpectedContentHash: agentObj.ContentHash},
		{CanonicalID: receiptObj.CanonicalID},
		{CanonicalID: existingUpdate.CanonicalID, Exists: true, ExpectedContentHash: existingUpdate.ContentHash},
	}
	conditions = append(conditions, artifactConditions...)
	objects := []objectgraph.Object{trajectoryUpdated, agentUpdated, updateObj}
	objects = append(objects, artifactObjects...)
	for _, eventObj := range eventObjs {
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID})
		objects = append(objects, eventObj)
	}
	objects = append(objects, receiptObj)
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects, types.LifecycleResult{Receipt: receipt, Trajectory: trajectory, WorkItem: resultWork, Agent: &agent, Document: resultDocument, Revision: resultRevision, Events: events}, artifactEdges...)
}

func (s *Store) settleOrRefuseLifecycleWork(ctx context.Context, ownerID, computerID, commandID, digest, trajectoryID, workItemID, actingAgentID, resultRef, reason string, refusal bool) (types.LifecycleResult, error) {
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if trajectory.Status != types.TrajectoryLive {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	workObj, work, err := s.lifecycleWorkObject(ctx, ownerID, computerID, workItemID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if work.TrajectoryID != trajectoryID || workItemTerminal(work.Status) {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	if strings.TrimSpace(actingAgentID) == "" || strings.TrimSpace(work.AssignedAgentID) != strings.TrimSpace(actingAgentID) {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle resolve work actor %q does not own assigned work %q: %w", actingAgentID, work.WorkItemID, ErrLifecycleInvalidTransition)
	}
	now := time.Now().UTC()
	nextSeq := trajectory.ReducerSeq + 1
	eventKind := types.LifecycleWorkSettled
	commandKind := types.LifecycleSettleWork
	if strings.TrimSpace(resultRef) == "" {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle resolve work: result_ref is required")
	}
	work.ResultRef = strings.TrimSpace(resultRef)
	if refusal {
		work.Status, work.Reason = types.WorkItemRefused, reason
		eventKind, commandKind = types.LifecycleWorkRefused, types.LifecycleRefuseWork
	} else {
		work.Status = types.WorkItemCompleted
	}
	work.LifecycleVersion, work.LastReducerSeq, work.UpdatedAt = work.LifecycleVersion+1, nextSeq, now
	trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = nextSeq, trajectory.LifecycleVersion+1, now
	event := types.LifecycleEvent{
		EventID: commandID + ":1", OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: trajectoryID, WorkItemID: workItemID, Kind: eventKind,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: nextSeq,
		CommandID: commandID, CommandDigest: digest, ArtifactRefs: []string{work.ResultRef},
		Reason: reason, CreatedAt: now,
	}
	events := []types.LifecycleEvent{event}
	trajectoryUpdated, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, trajectoryID, trajectory, lifecycleMetadata("trajectory_id", trajectoryID, computerID, trajectoryID, trajectory.ReducerSeq), trajectoryObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	workUpdated, err := lifecycleObject(ogKindWorkItem, ownerID, computerID, workItemID, work, lifecycleMetadata("work_item_id", workItemID, computerID, trajectoryID, work.LastReducerSeq), workObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	eventObjs := make([]objectgraph.Object, 0, len(events))
	for _, lifecycleEvent := range events {
		eventObj, buildErr := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, lifecycleEvent.EventID, lifecycleEvent, lifecycleMetadata("event_id", lifecycleEvent.EventID, computerID, trajectoryID, lifecycleEvent.ReducerSeq), now, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		eventObjs = append(eventObjs, eventObj)
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, trajectoryID, commandID, digest, commandKind, trajectory.ReducerSeq, eventObjs)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		{CanonicalID: workObj.CanonicalID, Exists: true, ExpectedContentHash: workObj.ContentHash},
		{CanonicalID: receiptObj.CanonicalID},
	}
	objects := []objectgraph.Object{trajectoryUpdated, workUpdated}
	for _, eventObj := range eventObjs {
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID})
		objects = append(objects, eventObj)
	}
	objects = append(objects, receiptObj)
	return s.commitLifecycleTransition(ctx, ownerID, computerID, commandID, digest, conditions, objects, types.LifecycleResult{Receipt: receipt, Trajectory: trajectory, WorkItem: &work, Events: events})
}

func (s *Store) SettleLifecycleWork(ctx context.Context, req types.SettleLifecycleWorkRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.TrajectoryID, req.WorkItemID = strings.TrimSpace(req.TrajectoryID), strings.TrimSpace(req.WorkItemID)
	req.ActingAgentID, req.ResultRef = strings.TrimSpace(req.ActingAgentID), strings.TrimSpace(req.ResultRef)
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	if req.ActingAgentID == "" {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle settle work: acting_agent_id is required")
	}
	computedDigest, digestErr := ComputeSettleLifecycleWorkDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	return s.settleOrRefuseLifecycleWork(ctx, ownerID, computerID, strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest), strings.TrimSpace(req.TrajectoryID), strings.TrimSpace(req.WorkItemID), req.ActingAgentID, req.ResultRef, "", false)
}

func (s *Store) RefuseLifecycleWork(ctx context.Context, req types.RefuseLifecycleWorkRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.TrajectoryID, req.WorkItemID, req.ActingAgentID = strings.TrimSpace(req.TrajectoryID), strings.TrimSpace(req.WorkItemID), strings.TrimSpace(req.ActingAgentID)
	req.Reason, req.RefusalRef = strings.TrimSpace(req.Reason), strings.TrimSpace(req.RefusalRef)
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	computedDigest, digestErr := ComputeRefuseLifecycleWorkDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}
	if req.ActingAgentID == "" || req.Reason == "" || req.RefusalRef == "" {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle refuse work: acting_agent_id, reason, and refusal_ref are required")
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	return s.settleOrRefuseLifecycleWork(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, req.TrajectoryID, req.WorkItemID, req.ActingAgentID, req.RefusalRef, req.Reason, true)
}

// CancelLifecycleTrajectory atomically cancels the trajectory, every open work
func (s *Store) SettleLifecycleTrajectory(ctx context.Context, req types.SettleLifecycleTrajectoryRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.TrajectoryID, req.ExpectedHeadRevisionID = strings.TrimSpace(req.TrajectoryID), strings.TrimSpace(req.ExpectedHeadRevisionID)
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	computedDigest, digestErr := ComputeSettleLifecycleTrajectoryDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}
	if req.ExpectedLifecycleVersion <= 0 || req.ExpectedHeadRevisionID == "" {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle settle trajectory: expected_lifecycle_version and expected_head_revision_id are required")
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if trajectory.Status != types.TrajectoryLive || trajectory.LifecycleVersion != req.ExpectedLifecycleVersion {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}
	docID := strings.TrimSpace(trajectory.SubjectRefs["doc_id"])
	if docID == "" {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	documentObj, err := s.lifecycleGetObject(ctx, ogKindTexDoc, ownerID, computerID, docID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	document, err := decodeLifecycleObject[types.Document](documentObj)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if document.CurrentRevisionID != req.ExpectedHeadRevisionID {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}
	headObj, err := s.lifecycleGetObject(ctx, ogKindTexRev, ownerID, computerID, document.CurrentRevisionID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	head, err := decodeLifecycleObject[types.Revision](headObj)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	prospective := trajectory
	prospective.TerminalArtifactHeadRef = document.CurrentRevisionID
	ready, err := s.lifecycleSettlementReady(ctx, prospective, nil, nil)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if !ready {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	now := time.Now().UTC()
	nextSeq := trajectory.ReducerSeq + 1
	settledAt := now
	trajectory.Status, trajectory.SettledAt = types.TrajectorySettled, &settledAt
	trajectory.TerminalArtifactHeadRef = document.CurrentRevisionID
	trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = nextSeq, trajectory.LifecycleVersion+1, now
	event := types.LifecycleEvent{
		EventID: req.CommandID + ":1", OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: req.TrajectoryID, Kind: types.LifecycleTrajectorySettled,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: nextSeq,
		CommandID: req.CommandID, CommandDigest: req.CommandDigest,
		ArtifactRefs: []string{document.DocID, document.CurrentRevisionID}, CreatedAt: now,
	}
	trajectoryUpdated, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, nextSeq), trajectoryObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, nextSeq), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleSettleTrajectory, nextSeq, []objectgraph.Object{eventObj})
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		{CanonicalID: eventObj.CanonicalID},
		{CanonicalID: documentObj.CanonicalID, Exists: true, ExpectedContentHash: documentObj.ContentHash},
		{CanonicalID: headObj.CanonicalID, Exists: true, ExpectedContentHash: headObj.ContentHash},
		{CanonicalID: receiptObj.CanonicalID},
	}
	objects := []objectgraph.Object{trajectoryUpdated, eventObj, receiptObj}
	workObjs, err := s.lifecycleTransitionObjects(ctx, ogKindWorkItem, req.TrajectoryID, ownerID, computerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	updateObjs, err := s.lifecycleTransitionObjects(ctx, ogKindWorkerUpdate, req.TrajectoryID, ownerID, computerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	for _, obj := range append(workObjs, updateObjs...) {
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID, Exists: true, ExpectedContentHash: obj.ContentHash})
	}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects, types.LifecycleResult{
		Receipt: receipt, Trajectory: trajectory, Document: &document, Revision: &head, Events: []types.LifecycleEvent{event},
	})
}

// ArchiveLifecycleArtifact records logical archival while retaining the
// document, immutable revision history, source graph, and lifecycle evidence.
func (s *Store) ArchiveLifecycleArtifact(ctx context.Context, req types.ArchiveLifecycleArtifactRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.TrajectoryID, req.ExpectedHeadRevisionID = strings.TrimSpace(req.TrajectoryID), strings.TrimSpace(req.ExpectedHeadRevisionID)
	req.Reason = strings.TrimSpace(req.Reason)
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	computedDigest, digestErr := ComputeArchiveLifecycleArtifactDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}
	if req.ExpectedLifecycleVersion <= 0 || req.ExpectedHeadRevisionID == "" {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle archive artifact: expected_lifecycle_version and expected_head_revision_id are required")
	}

	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if trajectory.LifecycleVersion != req.ExpectedLifecycleVersion {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}
	if trajectory.Status == types.TrajectoryLive {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	docID := strings.TrimSpace(trajectory.SubjectRefs["doc_id"])
	if docID == "" {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	documentObj, err := s.lifecycleGetObject(ctx, ogKindTexDoc, ownerID, computerID, docID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	document, err := decodeLifecycleObject[types.Document](documentObj)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if document.ArchivedAt != nil {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	if document.CurrentRevisionID != req.ExpectedHeadRevisionID {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}
	headObj, err := s.lifecycleGetObject(ctx, ogKindTexRev, ownerID, computerID, document.CurrentRevisionID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	head, err := decodeLifecycleObject[types.Revision](headObj)
	if err != nil {
		return types.LifecycleResult{}, err
	}

	now := time.Now().UTC()
	nextSeq, sequenceUpdated, sequenceCondition, sequenceErr := s.nextPostTerminalSequence(ctx, ownerID, computerID, trajectory, now)
	if sequenceErr != nil {
		return types.LifecycleResult{}, sequenceErr
	}
	document.ArchivedAt, document.UpdatedAt = &now, now
	documentMeta := lifecycleMetadata("doc_id", docID, computerID, req.TrajectoryID, nextSeq)
	documentMeta["current_revision_id"] = document.CurrentRevisionID
	documentMeta["archived_at"] = now.Format(time.RFC3339Nano)
	documentUpdated, err := lifecycleObject(ogKindTexDoc, ownerID, computerID, docID, document, documentMeta, documentObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	event := types.LifecycleEvent{
		EventID: req.CommandID + ":1", OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: req.TrajectoryID, Kind: types.LifecycleArtifactArchived,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: nextSeq,
		CommandID: req.CommandID, CommandDigest: req.CommandDigest,
		ArtifactRefs: []string{docID, document.CurrentRevisionID}, Reason: req.Reason, CreatedAt: now,
	}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, nextSeq), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleArchiveArtifact, nextSeq, []objectgraph.Object{eventObj})
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions := []objectgraph.ObjectCondition{
		sequenceCondition,
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		{CanonicalID: documentObj.CanonicalID, Exists: true, ExpectedContentHash: documentObj.ContentHash},
		{CanonicalID: headObj.CanonicalID, Exists: true, ExpectedContentHash: headObj.ContentHash},
		{CanonicalID: eventObj.CanonicalID},
		{CanonicalID: receiptObj.CanonicalID},
	}
	objects := []objectgraph.Object{sequenceUpdated, documentUpdated, eventObj, receiptObj}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects, types.LifecycleResult{
		Receipt: receipt, Trajectory: trajectory, Document: &document, Revision: &head, Events: []types.LifecycleEvent{event},
	})
}

// item, and every pending update in the same reducer transition.
func (s *Store) CancelLifecycleTrajectory(ctx context.Context, req types.CancelLifecycleRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.TrajectoryID, req.Reason = strings.TrimSpace(req.TrajectoryID), strings.TrimSpace(req.Reason)
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	req.ExpectedHeadRevisionID = strings.TrimSpace(req.ExpectedHeadRevisionID)
	computedDigest, digestErr := ComputeCancelLifecycleDigest(req)
	if req.ExpectedLifecycleVersion <= 0 || req.ExpectedHeadRevisionID == "" {
		return types.LifecycleResult{}, fmt.Errorf("cancel lifecycle: expected lifecycle version and head revision are required")
	}
	if err := requireLifecycleDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if trajectory.Status != types.TrajectoryLive {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	docID := strings.TrimSpace(trajectory.SubjectRefs["doc_id"])
	if docID == "" {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	documentObj, err := s.lifecycleGetObject(ctx, ogKindTexDoc, ownerID, computerID, docID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	document, err := decodeLifecycleObject[types.Document](documentObj)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	headObj, err := s.lifecycleGetObject(ctx, ogKindTexRev, ownerID, computerID, document.CurrentRevisionID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	head, err := decodeLifecycleObject[types.Revision](headObj)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	now := time.Now().UTC()
	nextSeq := trajectory.ReducerSeq + 1
	if trajectory.LifecycleVersion != req.ExpectedLifecycleVersion || document.CurrentRevisionID != req.ExpectedHeadRevisionID {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}
	cancelledAt := now
	trajectory.Status, trajectory.CancelledAt = types.TrajectoryCancelled, &cancelledAt
	trajectory.TerminalArtifactHeadRef = document.CurrentRevisionID
	trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = nextSeq, trajectory.LifecycleVersion+1, now
	trajectoryUpdated, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, nextSeq), trajectoryObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		{CanonicalID: documentObj.CanonicalID, Exists: true, ExpectedContentHash: documentObj.ContentHash},
		{CanonicalID: headObj.CanonicalID, Exists: true, ExpectedContentHash: headObj.ContentHash},
	}
	objects := []objectgraph.Object{trajectoryUpdated}

	workObjs, err := s.lifecycleTransitionObjects(ctx, ogKindWorkItem, req.TrajectoryID, ownerID, computerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	for _, obj := range workObjs {
		work, decodeErr := decodeLifecycleObject[types.WorkItemRecord](obj)
		if decodeErr != nil {
			return types.LifecycleResult{}, decodeErr
		}
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID, Exists: true, ExpectedContentHash: obj.ContentHash})
		if workItemTerminal(work.Status) {
			continue
		}
		work.Status, work.Reason = types.WorkItemCancelled, strings.TrimSpace(req.Reason)
		work.LifecycleVersion, work.LastReducerSeq, work.UpdatedAt = work.LifecycleVersion+1, nextSeq, now
		updated, buildErr := lifecycleObject(ogKindWorkItem, ownerID, computerID, work.WorkItemID, work, lifecycleMetadata("work_item_id", work.WorkItemID, computerID, req.TrajectoryID, nextSeq), obj.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		objects = append(objects, updated)
	}
	updateObjs, err := s.lifecycleTransitionObjects(ctx, ogKindWorkerUpdate, req.TrajectoryID, ownerID, computerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	for _, obj := range updateObjs {
		update, decodeErr := decodeLifecycleObject[types.CoagentSourcePacket](obj)
		if decodeErr != nil {
			return types.LifecycleResult{}, decodeErr
		}
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID, Exists: true, ExpectedContentHash: obj.ContentHash})
		if update.Disposition != "" && update.Disposition != types.UpdatePending {
			continue
		}
		update.Disposition, update.DispositionReason = types.UpdateCancelled, strings.TrimSpace(req.Reason)
		update.DispositionRef = lifecycleTerminalTrajectoryRef(req.TrajectoryID)
		update.LifecycleVersion, update.ReducerSeq = update.LifecycleVersion+1, nextSeq
		meta := lifecycleMetadata("update_id", update.UpdateID, computerID, req.TrajectoryID, nextSeq)
		meta["producer_update_id"] = update.ProducerUpdateID
		key := req.TrajectoryID + "\x00" + update.TargetAgentID + "\x00" + update.AgentID + "\x00" + update.ProducerUpdateID
		updated, buildErr := lifecycleObject(ogKindWorkerUpdate, ownerID, computerID, key, update, meta, obj.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		objects = append(objects, updated)
	}
	event := types.LifecycleEvent{
		EventID: req.CommandID + ":1", OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: req.TrajectoryID, Kind: types.LifecycleTrajectoryCancelled,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: nextSeq,
		CommandID: req.CommandID, CommandDigest: req.CommandDigest,
		Reason: strings.TrimSpace(req.Reason), CreatedAt: now,
		ArtifactRefs: []string{document.DocID, document.CurrentRevisionID},
	}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, nextSeq), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleCancelTrajectory, nextSeq, []objectgraph.Object{eventObj})
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID}, objectgraph.ObjectCondition{CanonicalID: receiptObj.CanonicalID})
	objects = append(objects, eventObj, receiptObj)
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects, types.LifecycleResult{Receipt: receipt, Trajectory: trajectory, Document: &document, Revision: &head, Events: []types.LifecycleEvent{event}})
}
