package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func RegisterWorkerUpdateTools(registry *ToolRegistry, rt *Runtime) error {
	return registry.Register(newSubmitWorkerUpdateTool(rt))
}

func newSubmitWorkerUpdateTool(rt *Runtime) Tool {
	type args struct {
		UpdateID    string   `json:"update_id"`
		AgentID     string   `json:"agent_id,omitempty"`
		ChannelID   string   `json:"channel_id,omitempty"`
		Findings    []string `json:"findings,omitempty"`
		EvidenceIDs []string `json:"evidence_ids,omitempty"`
		Artifacts   []string `json:"artifacts,omitempty"`
		Refs        []string `json:"refs,omitempty"`
		Tests       []string `json:"tests,omitempty"`
		Questions   []string `json:"questions,omitempty"`
		Proposals   []string `json:"proposals,omitempty"`
		Notes       []string `json:"notes,omitempty"`
	}
	return Tool{
		Name:        "submit_worker_update",
		Description: "Persist a structured non-patch worker update and send one addressed delivery to the owning agent.",
		Parameters: jsonSchemaObject(map[string]any{
			"update_id":    map[string]any{"type": "string"},
			"agent_id":     map[string]any{"type": "string"},
			"channel_id":   map[string]any{"type": "string"},
			"findings":     stringArraySchema(),
			"evidence_ids": stringArraySchema(),
			"artifacts":    stringArraySchema(),
			"refs":         stringArraySchema(),
			"tests":        stringArraySchema(),
			"questions":    stringArraySchema(),
			"proposals":    stringArraySchema(),
			"notes":        stringArraySchema(),
		}, []string{"update_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode submit_worker_update args: %w", err)
			}
			updateID := strings.TrimSpace(in.UpdateID)
			if updateID == "" {
				return "", fmt.Errorf("update_id must not be empty")
			}
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			agentID := stringFromToolContext(ctx, toolCtxAgentID)
			runID := stringFromToolContext(ctx, toolCtxRunID)
			role := stringFromToolContext(ctx, toolCtxRole)
			if ownerID == "" || agentID == "" || runID == "" {
				return "", fmt.Errorf("submit_worker_update missing worker context")
			}

			update := types.WorkerUpdateRecord{
				UpdateID:    updateID,
				OwnerID:     ownerID,
				AgentID:     agentID,
				Role:        nonEmpty(role, configuredAgentProfileForRun(ctxRunRecord(ctx))),
				Findings:    trimNonEmpty(in.Findings),
				EvidenceIDs: trimNonEmpty(in.EvidenceIDs),
				Artifacts:   trimNonEmpty(in.Artifacts),
				Refs:        trimNonEmpty(in.Refs),
				Tests:       trimNonEmpty(in.Tests),
				Questions:   trimNonEmpty(in.Questions),
				Proposals:   trimNonEmpty(in.Proposals),
				Notes:       trimNonEmpty(in.Notes),
				CreatedAt:   time.Now().UTC(),
			}
			if workerUpdateEmpty(update) {
				return "", fmt.Errorf("submit_worker_update requires findings, evidence_ids, artifacts, refs, tests, questions, proposals, or notes")
			}

			targetAgentID, targetChannelID, err := resolveFindingsTarget(ctx, rt, strings.TrimSpace(in.AgentID))
			if err != nil {
				return "", err
			}
			channelID := authoritativeDeliveryChannelID(targetChannelID, in.ChannelID, stringFromToolContext(ctx, toolCtxChannelID))
			if channelID == "" {
				return "", fmt.Errorf("submit_worker_update could not resolve channel_id")
			}

			trajectoryID := ""
			if runRec := ctxRunRecord(ctx); runRec != nil && runRec.Metadata != nil {
				if id, _ := runRec.Metadata[runMetadataTrajectoryID].(string); strings.TrimSpace(id) != "" {
					trajectoryID = strings.TrimSpace(id)
				}
			}

			update.TargetAgentID = targetAgentID
			update.ChannelID = channelID
			update.TrajectoryID = trajectoryID
			update.Content = buildWorkerUpdateMessage(update)

			message := &types.ChannelMessage{
				ChannelID:    channelID,
				From:         runID,
				FromAgentID:  agentID,
				FromRunID:    runID,
				ToAgentID:    targetAgentID,
				TrajectoryID: trajectoryID,
				Role:         update.Role,
				Content:      update.Content,
				Timestamp:    update.CreatedAt,
			}
			delivery := types.InboxDelivery{
				DeliveryID:   uuid.NewString(),
				OwnerID:      ownerID,
				ToAgentID:    targetAgentID,
				FromAgentID:  agentID,
				FromRunID:    runID,
				ChannelID:    channelID,
				Role:         message.Role,
				Content:      message.Content,
				TrajectoryID: trajectoryID,
				CreatedAt:    update.CreatedAt,
			}

			stored, created, err := rt.store.DispatchWorkerUpdate(ctx, update, message, delivery)
			if err != nil {
				return "", err
			}
			if !created {
				if err := validateExistingWorkerUpdate(stored, update); err != nil {
					return "", err
				}
			} else {
				rt.emitChannelMessageEvent(ctx, *message, ownerID)
			}

			return toolResultJSON(map[string]any{
				"update_id":     stored.UpdateID,
				"agent_id":      stored.TargetAgentID,
				"channel_id":    stored.ChannelID,
				"cursor":        stored.MessageSeq,
				"trajectory_id": stored.TrajectoryID,
				"status":        map[bool]string{true: "submitted", false: "existing"}[created],
			})
		},
	}
}

func stringArraySchema() map[string]any {
	return map[string]any{
		"type":  "array",
		"items": map[string]any{"type": "string"},
	}
}

func ctxRunRecord(ctx context.Context) *types.RunRecord {
	runRec, _ := ctx.Value(toolCtxRunRecord).(*types.RunRecord)
	return runRec
}

func workerUpdateEmpty(update types.WorkerUpdateRecord) bool {
	return len(update.Findings) == 0 &&
		len(update.EvidenceIDs) == 0 &&
		len(update.Artifacts) == 0 &&
		len(update.Refs) == 0 &&
		len(update.Tests) == 0 &&
		len(update.Questions) == 0 &&
		len(update.Proposals) == 0 &&
		len(update.Notes) == 0
}

func buildWorkerUpdateMessage(update types.WorkerUpdateRecord) string {
	var b strings.Builder
	b.WriteString("Worker update ready.")
	if strings.TrimSpace(update.Role) != "" {
		b.WriteString("\nRole: ")
		b.WriteString(strings.TrimSpace(update.Role))
		b.WriteString(".")
	}
	appendWorkerUpdateSection(&b, "Findings", update.Findings)
	appendWorkerUpdateSection(&b, "Evidence", update.EvidenceIDs)
	appendWorkerUpdateSection(&b, "Artifacts", update.Artifacts)
	appendWorkerUpdateSection(&b, "Refs", update.Refs)
	appendWorkerUpdateSection(&b, "Tests", update.Tests)
	appendWorkerUpdateSection(&b, "Questions", update.Questions)
	appendWorkerUpdateSection(&b, "Proposals", update.Proposals)
	appendWorkerUpdateSection(&b, "Notes", update.Notes)
	return b.String()
}

func appendWorkerUpdateSection(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString("\n\n")
	b.WriteString(title)
	b.WriteString(":\n")
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(item)
		b.WriteString("\n")
	}
}

func validateExistingWorkerUpdate(existing, want types.WorkerUpdateRecord) error {
	if existing.AgentID != want.AgentID ||
		existing.TargetAgentID != want.TargetAgentID ||
		existing.ChannelID != want.ChannelID ||
		existing.Role != want.Role ||
		existing.Content != want.Content ||
		!stringSlicesEqual(existing.Findings, want.Findings) ||
		!stringSlicesEqual(existing.EvidenceIDs, want.EvidenceIDs) ||
		!stringSlicesEqual(existing.Artifacts, want.Artifacts) ||
		!stringSlicesEqual(existing.Refs, want.Refs) ||
		!stringSlicesEqual(existing.Tests, want.Tests) ||
		!stringSlicesEqual(existing.Questions, want.Questions) ||
		!stringSlicesEqual(existing.Proposals, want.Proposals) ||
		!stringSlicesEqual(existing.Notes, want.Notes) {
		return fmt.Errorf("update_id %s already exists with different payload", want.UpdateID)
	}
	return nil
}
