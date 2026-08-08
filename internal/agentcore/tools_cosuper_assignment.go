package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// RegisterAssignedCoSuperTools adds only the exact persistent-Super assignment
// path. Generic lifecycle Super activation remains refused in StartCoagentRun.
func RegisterAssignedCoSuperTools(registry *toolregistry.ToolRegistry, rt *Runtime) error {
	for _, tool := range []toolregistry.Tool{newAssignCoSuperTool(rt), newCancelAssignedCoSuperTool(rt)} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func requirePersistentSuperExecution(ctx context.Context) (*types.RunRecord, error) {
	execution := toolregistry.ExecutionContextFrom(ctx)
	rec := execution.RunRecord
	if rec == nil || rec.AgentID != persistentSuperAgentID(rec.OwnerID) || rec.AgentProfile != "super" || rec.AgentRole != "super" || rec.TrajectoryID != "" {
		return nil, fmt.Errorf("assigned CoSuper tools require the exact non-lifecycle persistent Super")
	}
	return rec, nil
}

func newAssignCoSuperTool(rt *Runtime) toolregistry.Tool {
	type args struct {
		Objective        string                      `json:"objective"`
		Kind             types.CoSuperAssignmentKind `json:"kind"`
		ParentWorkItemID string                      `json:"parent_work_item_id"`
		CandidateID      string                      `json:"candidate_id,omitempty"`
	}
	return toolregistry.Tool{
		Name: "assign_co_super", Description: "Open one exact durable assignment and, only after its bind receipt commits, wake a writable networkless capsule CoSuper.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"objective":           map[string]any{"type": "string"},
			"kind":                map[string]any{"type": "string", "enum": []string{"implementation", "verification"}},
			"parent_work_item_id": map[string]any{"type": "string"},
			"candidate_id":        map[string]any{"type": "string", "description": "Required only for verification; exact candidate returned by a completed implementation assignment."},
		}, []string{"objective", "kind", "parent_work_item_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			parent, err := requirePersistentSuperExecution(ctx)
			if err != nil {
				return "", err
			}
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			execution := toolregistry.ExecutionContextFrom(ctx)
			started, err := rt.startAssignedCoSuper(ctx, parent.RunID, parent.OwnerID, StartAssignedCoSuperRequest{
				Objective: input.Objective, Kind: input.Kind, CandidateID: input.CandidateID,
				ParentWorkItemID: input.ParentWorkItemID, ToolCallID: execution.ToolCallID,
			})
			if err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(map[string]any{
				"assignment_id": started.Assignment.AssignmentID, "attempt": started.Assignment.Binding.Attempt,
				"assigned_work_item_id": started.Assignment.Binding.AssignedWorkItemID,
				"loop_id":               started.Run.RunID, "kind": started.Assignment.Binding.Kind,
				"disposition": started.Assignment.Disposition, "replay": started.Replay,
			})
		},
	}
}

func newCancelAssignedCoSuperTool(rt *Runtime) toolregistry.Tool {
	type args struct {
		AssignmentID string `json:"assignment_id"`
		Reason       string `json:"reason"`
	}
	return toolregistry.Tool{
		Name: "cancel_co_super_assignment", Description: "Durably revoke and executor-acknowledge one exact assignment capsule before cancellation.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"assignment_id": map[string]any{"type": "string"}, "reason": map[string]any{"type": "string"},
		}, []string{"assignment_id", "reason"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			parent, err := requirePersistentSuperExecution(ctx)
			if err != nil {
				return "", err
			}
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			assignment, err := rt.cancelAssignedCoSuper(ctx, *parent, strings.TrimSpace(input.AssignmentID), 1, input.Reason)
			if err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(map[string]any{"assignment_id": assignment.AssignmentID, "attempt": assignment.Binding.Attempt,
				"disposition": assignment.Disposition, "capsule_disposition": assignment.CapsuleDisposition})
		},
	}
}
