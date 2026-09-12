package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// CoSuperAssignmentSeed is the exact parent Super/trajectory authority used by
// assignment command tests and runtime restart tests.
type CoSuperAssignmentSeed struct {
	OwnerID, ComputerID, TrajectoryID                 string
	ParentAgentID, ParentRunID, ParentWorkID          string
	ParentDecisionID, ParentControlID                 string
	AssignedAgentIDs, AssignedWorkIDs, AssignedRunIDs []string
}

// SeedCoSuperAssignmentAuthority installs the immutable parent Super run,
// trajectory, document, and Super work required to open assigned CoSuper work.
func SeedCoSuperAssignmentAuthority(s *Store, ownerID, computerID string, count int) (CoSuperAssignmentSeed, error) {
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	f := CoSuperAssignmentSeed{
		OwnerID: ownerID, ComputerID: computerID, TrajectoryID: "trajectory-assignment",
		ParentAgentID: "super:" + ownerID, ParentRunID: "run-super-assignment", ParentWorkID: "work-super-assignment",
		ParentDecisionID: "decision:" + objectgraph.SHA256([]byte("decision-assignment")), ParentControlID: "control-assignment",
	}
	trajectory := types.TrajectoryRecord{
		TrajectoryID: f.TrajectoryID, OwnerID: f.OwnerID, ComputerID: f.ComputerID,
		Kind: types.TrajectoryKindTask, Status: types.TrajectoryLive,
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"subject"}},
		SubjectRefs:    map[string]string{"subject": "artifact:subject", "doc_id": "document-assignment"}, LifecycleVersion: 1, ReducerSeq: 1, CreatedAt: now, UpdatedAt: now,
	}
	document := types.Document{DocID: "document-assignment", OwnerID: f.OwnerID, ComputerID: f.ComputerID, Title: "Assignment authority", CurrentRevisionID: "revision-assignment", CreatedAt: now, UpdatedAt: now}
	revision := types.Revision{RevisionID: "revision-assignment", DocID: document.DocID, OwnerID: f.OwnerID, ComputerID: f.ComputerID, AuthorKind: types.AuthorAppAgent, AuthorLabel: "Choir", Content: "assignment authority", CreatedAt: now}
	parentAgent := types.AgentRecord{
		AgentID: f.ParentAgentID, OwnerID: f.OwnerID, ComputerID: f.ComputerID,
		Profile: "super", Role: "super", ChannelID: f.ParentAgentID, ActiveRunID: f.ParentRunID,
		LifecycleVersion: 0, CreatedAt: now, UpdatedAt: now,
	}
	parentWork := types.WorkItemRecord{
		WorkItemID: f.ParentWorkID, TrajectoryID: f.TrajectoryID, OwnerID: f.OwnerID, ComputerID: f.ComputerID,
		Objective: "coordinate delegated assignments", AuthorityProfile: "super", Status: types.WorkItemOpen,
		AssignedAgentID: f.ParentAgentID, LifecycleVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	parentRun := types.RunRecord{
		RunID: f.ParentRunID, AgentID: f.ParentAgentID, ChannelID: f.ParentAgentID,
		AgentProfile: "super", AgentRole: "super", OwnerID: f.OwnerID, ComputerID: f.ComputerID,
		State: types.RunRunning, Prompt: "coordinate", CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{
			"assignment_trajectory_id": f.TrajectoryID, "work_item_ids": []string{f.ParentWorkID},
			"lifecycle_control_bindings": []any{map[string]any{"trajectory_id": f.TrajectoryID,
				"target_work_item_id": f.ParentWorkID, "update_id": f.ParentControlID, "producer_agent_id": "texture:document-assignment"}},
		},
	}
	must := func(kind objectgraph.ObjectKind, key string, body any, metadata map[string]any) (objectgraph.Object, error) {
		return lifecycleObject(kind, f.OwnerID, f.ComputerID, key, body, metadata, now, now)
	}
	trajObj, err := must(ogKindTrajectory, f.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", f.TrajectoryID, f.ComputerID, f.TrajectoryID, 1))
	if err != nil {
		return CoSuperAssignmentSeed{}, err
	}
	agentObj, err := must(ogKindAgent, f.ParentAgentID, parentAgent, map[string]any{"agent_id": f.ParentAgentID, "computer_id": f.ComputerID})
	if err != nil {
		return CoSuperAssignmentSeed{}, err
	}
	workObj, err := must(ogKindWorkItem, f.ParentWorkID, parentWork, lifecycleMetadata("work_item_id", f.ParentWorkID, f.ComputerID, f.TrajectoryID, 1))
	if err != nil {
		return CoSuperAssignmentSeed{}, err
	}
	docObj, err := must(ogKindTexDoc, document.DocID, document, map[string]any{"doc_id": document.DocID, "computer_id": f.ComputerID})
	if err != nil {
		return CoSuperAssignmentSeed{}, err
	}
	revObj, err := must(ogKindTexRev, revision.RevisionID, revision, map[string]any{"revision_id": revision.RevisionID, "doc_id": document.DocID, "computer_id": f.ComputerID})
	if err != nil {
		return CoSuperAssignmentSeed{}, err
	}
	objects := []objectgraph.Object{trajObj, agentObj, workObj, docObj, revObj}
	for i := 0; i < count; i++ {
		f.AssignedAgentIDs = append(f.AssignedAgentIDs, fmt.Sprintf("co-super:assignment-%02d", i))
		f.AssignedWorkIDs = append(f.AssignedWorkIDs, fmt.Sprintf("work-cosuper-assignment-%02d", i))
		f.AssignedRunIDs = append(f.AssignedRunIDs, fmt.Sprintf("run-cosuper-assignment-%02d", i))
	}
	if err := s.ogStore.PutBatch(ctx, objectgraph.Batch{Objects: objects}); err != nil {
		return CoSuperAssignmentSeed{}, err
	}
	if err := s.CreateRunOG(ctx, parentRun); err != nil {
		return CoSuperAssignmentSeed{}, err
	}
	return f, nil
}
