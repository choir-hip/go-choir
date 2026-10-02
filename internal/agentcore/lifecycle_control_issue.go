package agentcore

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// LifecyclePacketIDForRecord derives the canonical lifecycle control packet ID
// for an immutable commitment record. Re-minting the same record therefore
// targets the same durable packet identity.
func LifecyclePacketIDForRecord(recordID string) string {
	return recordID + ":packet"
}

func lifecyclePacketIDForRecord(recordID string) string {
	return LifecyclePacketIDForRecord(recordID)
}

// issueLifecycleControl queues controls from the exact active Texture run and
// wakes each newly-created target through the normal lifecycle queue path.
func (rt *Runtime) issueLifecycleControl(ctx context.Context, rec *types.RunRecord, controls []types.TextureTurnControl, reason string) ([]types.CoagentSourcePacket, error) {
	if rt == nil || rt.store == nil || rec == nil {
		return nil, fmt.Errorf("issue lifecycle control: runtime, store, and caller run are required")
	}
	documentID := strings.TrimSpace(rec.ChannelID)
	if documentID == "" {
		return nil, fmt.Errorf("issue lifecycle control: caller run has no document channel")
	}
	snapshot, err := rt.store.GetLifecycleSnapshot(ctx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID)
	if err != nil {
		return nil, fmt.Errorf("issue lifecycle control: load trajectory snapshot: %w", err)
	}
	caller, err := rt.store.GetAgentByScope(ctx, rec.OwnerID, rec.ComputerID, rec.AgentID)
	if err != nil {
		return nil, fmt.Errorf("issue lifecycle control: load caller agent: %w", err)
	}
	controlIDs := make([]string, 0, len(controls))
	for _, control := range controls {
		controlIDs = append(controlIDs, strings.TrimSpace(control.ControlID))
	}
	req := types.IssueLifecycleControlRequest{
		OwnerID: rec.OwnerID, ComputerID: rec.ComputerID,
		CommandID:    "issue-lifecycle-control:" + rec.RunID + ":" + strings.Join(controlIDs, ","),
		TrajectoryID: rec.TrajectoryID, DocumentID: documentID,
		CallerAgentID: rec.AgentID, CallerRunID: rec.RunID,
		ExpectedLifecycleVersion:       snapshot.Trajectory.LifecycleVersion,
		ExpectedCallerLifecycleVersion: caller.LifecycleVersion,
		Controls:                       controls, Reason: reason,
	}
	req.CommandDigest, err = store.ComputeIssueLifecycleControlDigest(req)
	if err != nil {
		return nil, fmt.Errorf("issue lifecycle control: digest request: %w", err)
	}
	result, err := rt.store.IssueLifecycleControl(ctx, req)
	if err != nil {
		return nil, err
	}
	if !result.Replay {
		for _, control := range result.Controls {
			rt.wakeUpdatedCoagent(ctx, control)
		}
	}
	return result.Controls, nil
}
