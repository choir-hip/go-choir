package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func RegisterCoAgentTools(registry *toolregistry.ToolRegistry, rt *Runtime, _ agentprofile.Policy) error {
	return registry.Register(newCancelAgentTool(rt))
}

func (rt *Runtime) verifyRequiredChildRuns(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rt.store == nil || rec == nil {
		return nil
	}
	required := metadataIntValue(rec.Metadata, "required_child_runs")
	if required <= 0 {
		return nil
	}
	runs, err := rt.store.ListRunsByOwner(ctx, rec.OwnerID, 500)
	if err != nil {
		return fmt.Errorf("verify required child runs: %w", err)
	}
	completed := 0
	for _, child := range runs {
		if strings.TrimSpace(child.RequestedByRunID) == strings.TrimSpace(rec.RunID) &&
			child.State == types.RunCompleted {
			completed++
		}
	}
	if completed < required {
		return fmt.Errorf("required child runs incomplete: completed %d, require %d", completed, required)
	}
	return nil
}

func (rt *Runtime) verifyRequiredTextureRevisions(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rt.store == nil || rec == nil {
		return nil
	}
	required := metadataIntValue(rec.Metadata, "required_texture_revisions")
	if required <= 0 {
		return nil
	}
	runs, err := rt.store.ListRunsByOwner(ctx, rec.OwnerID, 500)
	if err != nil {
		return fmt.Errorf("verify required Texture revisions: list runs: %w", err)
	}
	written := make(map[string]struct{})
	for _, child := range runs {
		if strings.TrimSpace(child.RequestedByRunID) != strings.TrimSpace(rec.RunID) ||
			agentprofile.Canonical(agentProfileForRun(&child)) != agentprofile.Texture ||
			metadataStringValue(child.Metadata, "request_intent") != "universal_wire_reconciler_article_revision" {
			continue
		}
		docID := strings.TrimSpace(firstNonEmpty(metadataStringValue(child.Metadata, "doc_id"), child.ChannelID))
		if docID == "" {
			continue
		}
		revisions, err := rt.store.ListRevisionsByDoc(ctx, docID, rec.OwnerID, 200)
		if err != nil {
			return fmt.Errorf("verify required Texture revisions for doc %s: %w", docID, err)
		}
		for _, revision := range revisions {
			metadata := decodeRevisionMetadata(revision.Metadata)
			if metadataString(metadata, "loop_id") != child.RunID ||
				metadataString(metadata, "revision_role") != textureRevisionRoleCanonical ||
				metadataString(metadata, "input_origin") != textureInputOriginReconcilerHandoff {
				continue
			}
			written[revision.RevisionID] = struct{}{}
		}
	}
	if len(written) < required {
		return fmt.Errorf("required reconciler Texture revisions missing: wrote %d, require %d", len(written), required)
	}
	return nil
}

func (rt *Runtime) awaitRequiredTextureRevisions(ctx context.Context, rec *types.RunRecord, timeout time.Duration) error {
	if rec == nil || metadataIntValue(rec.Metadata, "required_texture_revisions") <= 0 {
		return nil
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for {
		if err := rt.verifyRequiredTextureRevisions(ctx, rec); err == nil {
			return nil
		}
		runs, err := rt.store.ListRunsByOwner(ctx, rec.OwnerID, 500)
		if err != nil {
			return fmt.Errorf("await required Texture revisions: list runs: %w", err)
		}
		children := 0
		active := false
		for _, child := range runs {
			if strings.TrimSpace(child.RequestedByRunID) != strings.TrimSpace(rec.RunID) ||
				agentprofile.Canonical(agentProfileForRun(&child)) != agentprofile.Texture ||
				metadataStringValue(child.Metadata, "request_intent") != "universal_wire_reconciler_article_revision" {
				continue
			}
			children++
			if child.State.Active() {
				active = true
			}
		}
		if children == 0 {
			return fmt.Errorf("required reconciler Texture revisions missing: no reconciler-owned Texture handoff")
		}
		if !active {
			return fmt.Errorf("required reconciler Texture revisions missing: all %d Texture handoff(s) terminated without the required canonical write", children)
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("required reconciler Texture revisions timed out after %s", timeout)
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (rt *Runtime) awaitRequiredChildRuns(ctx context.Context, rec *types.RunRecord, timeout time.Duration) error {
	if rec == nil {
		return nil
	}
	if metadataIntValue(rec.Metadata, "required_child_runs") <= 0 {
		return rt.awaitRequiredTextureRevisions(ctx, rec, timeout)
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for {
		if err := rt.verifyRequiredChildRuns(ctx, rec); err == nil {
			return rt.awaitRequiredTextureRevisions(ctx, rec, timeout)
		}
		runs, err := rt.store.ListRunsByOwner(ctx, rec.OwnerID, 500)
		if err != nil {
			return fmt.Errorf("await required child runs: %w", err)
		}
		children := 0
		active := false
		for _, child := range runs {
			if strings.TrimSpace(child.RequestedByRunID) != strings.TrimSpace(rec.RunID) {
				continue
			}
			children++
			if child.State.Active() {
				active = true
			}
		}
		if children == 0 {
			return fmt.Errorf("required child runs missing")
		}
		if !active {
			return fmt.Errorf("all %d required child run(s) terminated before completion", children)
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("required child runs timed out after %s", timeout)
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
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
			if toolregistry.ExecutionContextFrom(ctx).Profile == agentprofile.Super {
				callerTrajectoryID := trajectoryIDForRun(toolregistry.ExecutionContextFrom(ctx).RunRecord)
				slot, found, err := rt.store.CoSuperSlotByAgentAndTrajectory(ctx, ownerID, callerTrajectoryID, agentID)
				if err != nil {
					return "", fmt.Errorf("lookup co-super slot before cancel: %w", err)
				}
				if !found {
					return "", fmt.Errorf("agent not active in caller trajectory: %s", in.AgentID)
				}
				slotRun, err := rt.store.GetRun(ctx, strings.TrimSpace(slot.RunID))
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
					latest, err := rt.store.GetLatestActiveRunByAgent(ctx, ownerID, agentID)
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
