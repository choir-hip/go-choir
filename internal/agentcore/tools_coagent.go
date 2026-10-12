package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func RegisterCoAgentTools(registry *toolregistry.ToolRegistry, rt *Runtime, _ agentprofile.Policy) error {
	return registry.Register(newCancelAgentTool(rt))
}

func newCancelAgentTool(rt *Runtime) toolregistry.Tool {
	type args struct {
		AgentID string `json:"agent_id"`
	}
	return toolregistry.Tool{Name: "cancel_agent",
		Description: "Cancel the latest active loop for an existing agent by agent id.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"agent_id": map[string]any{"type": "string"},
		}, []string{"agent_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode cancel_agent args: %w", err)
			}
			ownerID := toolregistry.ExecutionContextFrom(ctx).OwnerID
			if ownerID == "" {
				return "", fmt.Errorf("cancel_agent missing owner context")
			}
			agentID := strings.TrimSpace(in.AgentID)
			var target types.RunRecord
			targetFromCallerSlot := false
			if toolregistry.ExecutionContextFrom(ctx).Profile == agentprofile.Management {
				callerTrajectoryID := trajectoryIDForRun(toolregistry.ExecutionContextFrom(ctx).RunRecord)
				slot, found, err := rt.store.EngineeringSlotByAgentAndTrajectory(ctx, ownerID, callerTrajectoryID, agentID)
				if err != nil {
					return "", fmt.Errorf("lookup co-super slot before cancel: %w", err)
				}
				if !found {
					return "", fmt.Errorf("agent not active in caller trajectory: %s", in.AgentID)
				}
				slotRun, err := rt.getRunForComputer(ctx, ownerID, strings.TrimSpace(slot.RunID))
				if err != nil {
					return "", fmt.Errorf("lookup co-super slot run before cancel: %w", err)
				}
				if !slotRun.State.Active() {
					return "", fmt.Errorf("agent not active in caller trajectory: %s", in.AgentID)
				}
				target = slotRun
				targetFromCallerSlot = true
			}
			if !targetFromCallerSlot {
				if resident, found, err := rt.activeRunByAgent(ctx, ownerID, agentID); err != nil {
					return "", fmt.Errorf("lookup resident agent run: %w", err)
				} else if found {
					target = resident
				} else {
					latest, err := rt.latestActiveRunByAgent(ctx, ownerID, agentID)
					if err != nil {
						if err == store.ErrNotFound {
							return "", fmt.Errorf("agent not found: %s", in.AgentID)
						}
						return "", fmt.Errorf("lookup active agent run: %w", err)
					}
					target = latest
				}
			}
			if err := rt.CancelRun(ctx, target.RunID, ownerID); err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(map[string]any{
				"agent_id": in.AgentID,
				"loop_id":  target.RunID,
				"status":   "cancelled",
			})
		}}
}
