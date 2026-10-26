package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Continuing a settled document (F1,
// docs/problems/settled-document-refuses-owner-revision-2026-10-26.md). A
// trajectory is one episode of work; a document outlives its episodes. When
// a document's trajectory has settled, the next episode is a new live
// trajectory bound to the same document:
//
//   - the settled trajectory is not touched: settled stays terminal, its
//     events, work, updates and assignments stay its own and its snapshot
//     still reads (on its terminal head);
//   - the new trajectory carries the document's binding (doc_id, artifact,
//     the settlement rule's required refs) and the continues link
//     (types.TrajectoryRefContinues), and nothing else of the episode;
//   - the document's trajectory pointer and its doc-scoped desks
//     (texture:<doc>, and research:<doc> when it is bound to the settled
//     trajectory) move to the new trajectory, in the same commit, so the new
//     snapshot has its desk and the old one does not;
//   - the head is carried by reference: the document's current revision is
//     the new trajectory's head; no revision is written here;
//   - one open Texture work item, so the owner's revise that follows has its
//     target.
//
// It is a start (command kind start, event trajectory_started with
// [doc_id, head] like StartLifecycle's), so no lifecycle vocabulary is added.
// Ids come from the caller; the owner revise derives them from the settled
// trajectory, so concurrent revises on one settled document send the same
// command and all but one replay.

// lifecycleLineageMax bounds a lineage read.
const lifecycleLineageMax = 64

// ComputeContinueLifecycleDocumentDigest is the canonical digest of a
// continue command: scope and timestamps are not part of it, so a retry
// from any caller with the same intent replays.
func ComputeContinueLifecycleDocumentDigest(req types.ContinueLifecycleDocumentRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	req.InitialWork.OwnerID, req.InitialWork.ComputerID, req.InitialWork.CreatedByRunID = "", "", ""
	req.InitialWork.CreatedAt, req.InitialWork.UpdatedAt = time.Time{}, time.Time{}
	return lifecycleDigest(req)
}

// continuedDeskProfiles are the desks whose lifecycle subject is the
// document (agent id <profile>:<doc_id>) and so move with it.
var continuedDeskProfiles = []string{agentprofile.Texture, agentprofile.Research}

// ContinueLifecycleDocument starts the next live trajectory on a document
// whose trajectory settled. Refusals: the predecessor is live or cancelled,
// or not this document's (ErrLifecycleInvalidTransition); the document moved
// on or its head changed (ErrConcurrentStateChange); the document is
// archived, is a story, has no live Texture desk on the predecessor, or is
// the deleted engineering-bound kind (ErrLifecycleInvalidTransition); a desk
// still owns a pending or running activation (ErrConcurrentStateChange).
func (s *Store) ContinueLifecycleDocument(ctx context.Context, req types.ContinueLifecycleDocumentRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.TrajectoryID, req.PredecessorTrajectoryID = strings.TrimSpace(req.TrajectoryID), strings.TrimSpace(req.PredecessorTrajectoryID)
	req.DocID, req.ExpectedHeadRevisionID = strings.TrimSpace(req.DocID), strings.TrimSpace(req.ExpectedHeadRevisionID)
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	if req.DocID == "" || req.PredecessorTrajectoryID == "" || req.ExpectedHeadRevisionID == "" || req.PredecessorTrajectoryID == req.TrajectoryID {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle continue: doc_id, predecessor_trajectory_id, a new trajectory_id and expected_head_revision_id are required: %w", ErrLifecycleInvalidTransition)
	}
	computedDigest, digestErr := ComputeContinueLifecycleDocumentDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computedDigest, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}
	deskAgentID := agentprofile.Texture + ":" + req.DocID
	if assigned := strings.TrimSpace(req.InitialWork.AssignedAgentID); assigned != "" && assigned != deskAgentID {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle continue: initial work must target %s: %w", deskAgentID, ErrLifecycleInvalidTransition)
	}
	if profile := strings.TrimSpace(req.InitialWork.AuthorityProfile); profile != "" && profile != agentprofile.Texture {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle continue: initial work authority must be texture: %w", ErrLifecycleInvalidTransition)
	}

	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	predecessorObj, predecessor, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, req.PredecessorTrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	// Settled only: live has nothing to continue, and cancelled is a stop.
	if predecessor.Status != types.TrajectorySettled || predecessor.Kind == types.TrajectoryKindStory ||
		strings.TrimSpace(predecessor.SubjectRefs["doc_id"]) != req.DocID {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	if _, intentErr := s.lifecycleGetObject(ctx, ogKindLifecycleCancelIntent, ownerID, computerID, predecessor.TrajectoryID); intentErr == nil {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	} else if !errors.Is(intentErr, ErrNotFound) {
		return types.LifecycleResult{}, intentErr
	}
	documentObj, err := s.lifecycleGetObject(ctx, ogKindTexDoc, ownerID, computerID, req.DocID)
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
	if document.TrajectoryID != predecessor.TrajectoryID || document.CurrentRevisionID != req.ExpectedHeadRevisionID {
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
	// The engineering-bound document (path B) is deleted (C1e step 3); one
	// that survives keeps refusing rather than gaining a Texture desk.
	if engObj, engErr := s.lifecycleGetObject(ctx, ogKindAgent, ownerID, computerID, agentprofile.Engineering+":"+req.DocID); engErr == nil {
		if lifecycleObjectTrajectoryID(engObj) == predecessor.TrajectoryID {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
	} else if !errors.Is(engErr, ErrNotFound) {
		return types.LifecycleResult{}, engErr
	}

	now := time.Now().UTC()
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: predecessorObj.CanonicalID, Exists: true, ExpectedContentHash: predecessorObj.ContentHash},
		{CanonicalID: documentObj.CanonicalID, Exists: true, ExpectedContentHash: documentObj.ContentHash},
		{CanonicalID: headObj.CanonicalID, Exists: true, ExpectedContentHash: headObj.ContentHash},
	}
	var objects []objectgraph.Object
	var textureDesk *types.AgentRecord
	for _, profile := range continuedDeskProfiles {
		agentID := profile + ":" + req.DocID
		agentObj, getErr := s.lifecycleGetObject(ctx, ogKindAgent, ownerID, computerID, agentID)
		if errors.Is(getErr, ErrNotFound) {
			continue
		}
		if getErr != nil {
			return types.LifecycleResult{}, getErr
		}
		if lifecycleObjectTrajectoryID(agentObj) != predecessor.TrajectoryID {
			// Not this episode's desk (absent from its snapshot): leave it.
			continue
		}
		agent, decodeErr := decodeLifecycleObject[types.AgentRecord](agentObj)
		if decodeErr != nil {
			return types.LifecycleResult{}, decodeErr
		}
		if agent.AgentID != agentID || agent.OwnerID != ownerID || agent.ComputerID != computerID ||
			strings.TrimSpace(agent.Profile) != profile || strings.TrimSpace(agent.Role) != profile || agent.ChannelID != req.DocID {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: agentObj.CanonicalID, Exists: true, ExpectedContentHash: agentObj.ContentHash})
		// A desk run still holding the activation would rewrite the desk's
		// binding back to the settled trajectory when it ends
		// (projectLifecycleRun, TerminalizeRun): refuse while it owns it,
		// and release a stale hold.
		if activeRunID := strings.TrimSpace(agent.ActiveRunID); activeRunID != "" {
			runObj, runErr := s.lifecycleGetObject(ctx, ogKindRun, ownerID, computerID, activeRunID)
			if runErr != nil && !errors.Is(runErr, ErrNotFound) {
				return types.LifecycleResult{}, runErr
			}
			if runErr == nil {
				run, runDecodeErr := decodeLifecycleObject[types.RunRecord](runObj)
				if runDecodeErr != nil {
					return types.LifecycleResult{}, runDecodeErr
				}
				if lifecycleRunOwnsActivation(run.State) {
					return types.LifecycleResult{}, ErrConcurrentStateChange
				}
				conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: runObj.CanonicalID, Exists: true, ExpectedContentHash: runObj.ContentHash})
			}
			agent.ActiveRunID = ""
		}
		if agent.LifecycleVersion <= 0 {
			if profile == agentprofile.Texture {
				return types.LifecycleResult{}, ErrLifecycleInvalidTransition
			}
			continue
		}
		agent.LifecycleVersion++
		agent.LastReducerSeq, agent.UpdatedAt = 1, now
		agentMeta := lifecycleMetadata("agent_id", agent.AgentID, computerID, req.TrajectoryID, 1)
		agentMeta["channel_id"] = agent.ChannelID
		agentUpdated, buildErr := lifecycleObject(ogKindAgent, ownerID, computerID, agent.AgentID, agent, agentMeta, agentObj.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		objects = append(objects, agentUpdated)
		if profile == agentprofile.Texture {
			desk := agent
			textureDesk = &desk
		}
	}
	if textureDesk == nil {
		// Only a document whose Texture desk worked this episode continues.
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}

	subjectRefs := map[string]string{"doc_id": req.DocID, types.TrajectoryRefContinues: predecessor.TrajectoryID}
	carried := append([]string{"artifact"}, predecessor.SettlementRule.RequiredSubjectRefs...)
	for _, key := range carried {
		key = strings.TrimSpace(key)
		if value := strings.TrimSpace(predecessor.SubjectRefs[key]); key != "" && value != "" {
			subjectRefs[key] = value
		}
	}
	if err := validateLifecycleSettlementRule(predecessor.SettlementRule, subjectRefs); err != nil {
		return types.LifecycleResult{}, err
	}
	trajectory := types.TrajectoryRecord{
		TrajectoryID: req.TrajectoryID, OwnerID: ownerID, ComputerID: computerID,
		Kind: predecessor.Kind, SubjectRefs: subjectRefs, Status: types.TrajectoryLive,
		SettlementRule: predecessor.SettlementRule, LifecycleVersion: 1, ReducerSeq: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	if trajectory.Kind == "" {
		trajectory.Kind = types.TrajectoryKindTask
	}
	workInput := req.InitialWork
	workInput.AssignedAgentID, workInput.AuthorityProfile = deskAgentID, agentprofile.Texture
	work, err := normalizeLifecycleWork(workInput, ownerID, computerID, req.TrajectoryID, now)
	if err != nil {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle continue: initial work: %w", err)
	}
	work.LifecycleVersion, work.LastReducerSeq = 1, 1
	document.TrajectoryID, document.UpdatedAt = req.TrajectoryID, now

	event := types.LifecycleEvent{
		EventID: req.CommandID + ":1", OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: req.TrajectoryID, WorkItemID: work.WorkItemID,
		Kind: types.LifecycleTrajectoryStarted, ReducerVersion: types.LifecycleReducerVersion,
		ReducerSeq: 1, CommandID: req.CommandID, CommandDigest: req.CommandDigest,
		ArtifactRefs: []string{document.DocID, head.RevisionID}, CreatedAt: now,
	}
	trajectoryObj, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, 1), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	legacyTrajectoryID, err := objectgraph.BuildCanonicalID(ogKindTrajectory, ownerID, objectgraph.StableSuffixFromKey(req.TrajectoryID))
	if err != nil {
		return types.LifecycleResult{}, err
	}
	workObj, err := lifecycleObject(ogKindWorkItem, ownerID, computerID, work.WorkItemID, work, lifecycleMetadata("work_item_id", work.WorkItemID, computerID, req.TrajectoryID, 1), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	documentMeta := lifecycleMetadata("doc_id", document.DocID, computerID, req.TrajectoryID, 1)
	documentMeta["current_revision_id"] = document.CurrentRevisionID
	documentUpdated, err := lifecycleObject(ogKindTexDoc, ownerID, computerID, document.DocID, document, documentMeta, documentObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, 1), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleStart, 1, []objectgraph.Object{eventObj})
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions = append(conditions,
		objectgraph.ObjectCondition{CanonicalID: trajectoryObj.CanonicalID},
		objectgraph.ObjectCondition{CanonicalID: workObj.CanonicalID},
		objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID},
		objectgraph.ObjectCondition{CanonicalID: receiptObj.CanonicalID},
	)
	if legacyTrajectoryID != trajectoryObj.CanonicalID {
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: legacyTrajectoryID})
	}
	objects = append(objects, trajectoryObj, workObj, documentUpdated, eventObj, receiptObj)
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects, types.LifecycleResult{
		Receipt: receipt, Trajectory: trajectory, WorkItem: &work, Agent: textureDesk,
		Document: &document, Revision: &head, Events: []types.LifecycleEvent{event},
	})
}

// LifecycleTrajectoryLineage is trajectoryID followed by the trajectories it
// continues (types.TrajectoryRefContinues), newest first, at most limit (0:
// lifecycleLineageMax). A link is followed only to a trajectory of the same
// document, and the read stops at a missing trajectory or a cycle, so a
// hand-written or corrupt ref never reads another document's history. A
// trajectory that cannot be found at all is its own lineage.
func (s *Store) LifecycleTrajectoryLineage(ctx context.Context, ownerID, computerID, trajectoryID string, limit int) ([]string, error) {
	trajectoryID = strings.TrimSpace(trajectoryID)
	if trajectoryID == "" {
		return nil, nil
	}
	if limit <= 0 || limit > lifecycleLineageMax {
		limit = lifecycleLineageMax
	}
	out := []string{trajectoryID}
	seen := map[string]bool{trajectoryID: true}
	current, err := s.GetLifecycleTrajectory(ctx, ownerID, computerID, trajectoryID)
	if errors.Is(err, ErrNotFound) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	docID := strings.TrimSpace(current.SubjectRefs["doc_id"])
	for len(out) < limit {
		previousID := strings.TrimSpace(current.SubjectRefs[types.TrajectoryRefContinues])
		if previousID == "" || docID == "" || seen[previousID] {
			break
		}
		previous, getErr := s.GetLifecycleTrajectory(ctx, ownerID, computerID, previousID)
		if errors.Is(getErr, ErrNotFound) {
			break
		}
		if getErr != nil {
			return nil, getErr
		}
		if strings.TrimSpace(previous.SubjectRefs["doc_id"]) != docID {
			break
		}
		out = append(out, previousID)
		seen[previousID] = true
		current = previous
	}
	return out, nil
}
