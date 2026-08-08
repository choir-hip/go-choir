package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
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

func RegisterPersistentSuperReportTools(registry *toolregistry.ToolRegistry, rt *Runtime) error {
	return registry.Register(newReportPersistentSuperToTextureTool(rt))
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
		ScopeDigest      string                      `json:"scope_digest"`
		SubjectDigest    string                      `json:"subject_digest"`
		ParentWorkItemID string                      `json:"parent_work_item_id"`
	}
	return toolregistry.Tool{
		Name: "assign_co_super", Description: "Open one exact durable assignment and, only after its bind receipt commits, wake a writable networkless capsule CoSuper.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"objective":    map[string]any{"type": "string"},
			"kind":         map[string]any{"type": "string", "enum": []string{"implementation", "verification"}},
			"scope_digest": map[string]any{"type": "string"}, "subject_digest": map[string]any{"type": "string"},
			"parent_work_item_id": map[string]any{"type": "string"},
		}, []string{"objective", "kind", "scope_digest", "subject_digest", "parent_work_item_id"}, false),
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
				Objective: input.Objective, Kind: input.Kind,
				ScopeDigest: input.ScopeDigest, SubjectDigest: input.SubjectDigest,
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
		Attempt      uint64 `json:"attempt"`
		Reason       string `json:"reason"`
	}
	return toolregistry.Tool{
		Name: "cancel_co_super_assignment", Description: "Durably revoke and executor-acknowledge one exact assignment capsule before cancellation.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"assignment_id": map[string]any{"type": "string"}, "attempt": map[string]any{"type": "integer"}, "reason": map[string]any{"type": "string"},
		}, []string{"assignment_id", "attempt", "reason"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			parent, err := requirePersistentSuperExecution(ctx)
			if err != nil {
				return "", err
			}
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			result, err := rt.cancelAssignedCoSuper(ctx, *parent, strings.TrimSpace(input.AssignmentID), input.Attempt, input.Reason)
			if err != nil {
				return "", err
			}
			if !result.Replay && result.Update != nil {
				rt.wakeUpdatedCoagent(ctx, *result.Update)
			}
			assignment := result.Assignment
			return toolregistry.ResultJSON(map[string]any{"assignment_id": assignment.AssignmentID, "attempt": assignment.Binding.Attempt,
				"disposition": assignment.Disposition, "capsule_disposition": assignment.CapsuleDisposition, "receipt": result.Receipt, "replay": result.Replay})
		},
	}
}

func rejectReportAuthorityInputs(actions []types.CoagentPacketAction) error {
	forbidden := map[string]bool{
		"ownerid": true, "computerid": true, "trajectoryid": true, "agentid": true, "targetagentid": true,
		"sourcerunid": true, "runid": true, "loopid": true, "workitemid": true, "targetworkitemid": true,
		"producerworkitemid": true, "controlbindingid": true, "controlid": true, "updateid": true,
		"producerupdateid": true, "channelid": true,
	}
	canonicalKey := func(key string) string {
		return strings.Map(func(r rune) rune {
			if r >= 'A' && r <= 'Z' {
				return r + ('a' - 'A')
			}
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, strings.TrimSpace(key))
	}
	var walk func(any) error
	walk = func(value any) error {
		switch typed := value.(type) {
		case map[string]any:
			for key, nested := range typed {
				if forbidden[canonicalKey(key)] {
					return fmt.Errorf("report_to_texture action inputs cannot author lifecycle authority key %q", key)
				}
				if err := walk(nested); err != nil {
					return err
				}
			}
		case []any:
			for _, nested := range typed {
				if err := walk(nested); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, action := range actions {
		if err := walk(action.Inputs); err != nil {
			return err
		}
	}
	return nil
}

func newReportPersistentSuperToTextureTool(rt *Runtime) toolregistry.Tool {
	type args struct {
		Kind            string                      `json:"kind"`
		Summary         string                      `json:"summary"`
		Claims          []types.CoagentPacketClaim  `json:"claims"`
		Sources         []types.CoagentPacketSource `json:"sources"`
		Actions         []types.CoagentPacketAction `json:"actions"`
		Questions       []string                    `json:"questions"`
		Notes           []string                    `json:"notes"`
		WorkDisposition types.WorkItemStatus        `json:"work_disposition"`
	}
	return toolregistry.Tool{
		Name:        "report_to_texture",
		Description: "Report typed progress, evidence, a blocker, or a terminal result from this exact persistent-Super control run back to its lifecycle Texture owner. Runtime derives all agent/run/trajectory/work identities.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"kind":             map[string]any{"type": "string", "enum": []string{"evidence_update", "execution_result", "blocker", "question", "proposal", "decision_request"}},
			"summary":          map[string]any{"type": "string"},
			"claims":           map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			"sources":          map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			"actions":          map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			"questions":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"notes":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"work_disposition": map[string]any{"type": "string", "enum": []string{"open", "completed"}},
		}, []string{"kind", "summary", "claims", "sources", "actions", "questions", "notes", "work_disposition"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			parent, err := requirePersistentSuperExecution(ctx)
			if err != nil {
				return "", err
			}
			var input args
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&input); err != nil {
				return "", fmt.Errorf("report_to_texture typed payload: %w", err)
			}
			if err := decoder.Decode(&struct{}{}); err != io.EOF {
				return "", fmt.Errorf("report_to_texture typed payload must contain one JSON object")
			}
			if err := rejectReportAuthorityInputs(input.Actions); err != nil {
				return "", err
			}
			packet := normalizeCoagentSourcePacketPayload(types.CoagentSourcePacketPayload{
				SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: input.Kind, Summary: input.Summary,
				Claims: input.Claims, Sources: input.Sources, Actions: input.Actions, Questions: input.Questions, Notes: input.Notes,
			})
			if err := validateCoagentSourcePacketPayload(packet); err != nil {
				return "", err
			}
			if input.WorkDisposition != types.WorkItemOpen && input.WorkDisposition != types.WorkItemCompleted {
				return "", fmt.Errorf("report_to_texture work_disposition must be open or completed")
			}
			trajectoryID := lifecycleControlTrajectoryForRun(parent)
			if trajectoryID == "" {
				return "", fmt.Errorf("report_to_texture requires one exact delivered lifecycle control trajectory")
			}
			delivered, err := rt.store.ListLifecycleControlsDeliveredToRun(ctx, parent.OwnerID, parent.SandboxID, trajectoryID, parent.AgentID, parent.RunID, 100)
			if err != nil {
				return "", err
			}
			controls := make([]types.CoagentSourcePacket, 0, len(delivered))
			for _, packet := range delivered {
				if packet.Direction == types.LifecyclePacketDirectionControl {
					controls = append(controls, packet)
				}
			}
			if len(controls) == 0 {
				return "", fmt.Errorf("report_to_texture requires an exact delivered control binding")
			}
			control := controls[0]
			if control.TargetWorkItemID == "" || control.AgentID == "" || control.SourceRunID == "" {
				return "", fmt.Errorf("report_to_texture control binding is incomplete")
			}
			for _, candidate := range controls[1:] {
				if candidate.TrajectoryID != control.TrajectoryID || candidate.TargetWorkItemID != control.TargetWorkItemID ||
					candidate.AgentID != control.AgentID || candidate.SourceRunID != control.SourceRunID || candidate.ChannelID != control.ChannelID {
					return "", fmt.Errorf("report_to_texture delivered controls span more than one trajectory/work/Texture scope")
				}
				if candidate.ReducerSeq > control.ReducerSeq {
					control = candidate
				}
			}
			targetRun, err := rt.store.GetLifecycleRun(ctx, parent.OwnerID, parent.SandboxID, control.SourceRunID)
			if err != nil {
				return "", err
			}
			targetWorkSet := lifecycleControlWorkIDsForRun(&targetRun)
			if len(targetWorkSet) != 1 {
				return "", fmt.Errorf("report_to_texture Texture source run must bind exactly one lifecycle work item")
			}
			targetWorkID := ""
			for id := range targetWorkSet {
				targetWorkID = id
			}
			execution := toolregistry.ExecutionContextFrom(ctx)
			if strings.TrimSpace(execution.ToolCallID) == "" {
				return "", fmt.Errorf("report_to_texture requires authenticated provider tool-call identity")
			}
			occurrence := objectgraph.SHA256([]byte(strings.Join([]string{"choir:persistent-super-report:v1", parent.OwnerID, parent.SandboxID, parent.RunID, execution.ToolCallID}, "\x00")))
			producerUpdateID := "super-report:" + occurrence
			content := strings.TrimSpace(packet.Summary)
			payloadDigest, err := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
			if err != nil {
				return "", err
			}
			req := types.QueueLifecycleUpdateRequest{
				OwnerID: parent.OwnerID, ComputerID: parent.SandboxID, CommandID: "queue-" + producerUpdateID,
				TrajectoryID: trajectoryID, TargetAgentID: control.AgentID, ProducerAgentID: parent.AgentID,
				ControlBindingID: control.UpdateID, TargetWorkItemID: targetWorkID,
				ProducerUpdateID: producerUpdateID, UpdateID: "result:" + occurrence,
				ChannelID: control.ChannelID, Role: "super", SourceRunID: parent.RunID,
				Packet: packet, Content: content, WorkDisposition: input.WorkDisposition,
				WorkItemID: control.TargetWorkItemID, PayloadDigest: payloadDigest,
			}
			req.CommandDigest, err = store.ComputeQueuePersistentSuperReportDigest(req)
			if err != nil {
				return "", err
			}
			queued, err := rt.store.QueueLifecycleUpdate(ctx, req)
			if err != nil {
				return "", err
			}
			if !queued.Replay && queued.Update != nil && queued.Update.Disposition == types.UpdatePending {
				rt.wakeUpdatedCoagent(ctx, *queued.Update)
			}
			return toolregistry.ResultJSON(map[string]any{"receipt": queued.Receipt, "update": queued.Update, "replay": queued.Replay})
		},
	}
}
