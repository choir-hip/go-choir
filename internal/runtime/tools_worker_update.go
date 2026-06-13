package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func RegisterCoagentUpdateTools(registry *ToolRegistry, rt *Runtime) error {
	return registry.Register(newUpdateCoagentTool(rt))
}

type submitCoagentUpdateArgs struct {
	UpdateID           string                         `json:"update_id"`
	Kind               string                         `json:"kind,omitempty"`
	Summary            string                         `json:"summary,omitempty"`
	AgentID            string                         `json:"agent_id,omitempty"`
	ChannelID          string                         `json:"channel_id,omitempty"`
	Findings           []string                       `json:"findings,omitempty"`
	Evidence           []researchFindingEvidenceInput `json:"evidence,omitempty"`
	EvidenceIDs        []string                       `json:"evidence_ids,omitempty"`
	Artifacts          []string                       `json:"artifacts,omitempty"`
	Refs               []string                       `json:"refs,omitempty"`
	Tests              []string                       `json:"tests,omitempty"`
	Questions          []string                       `json:"questions,omitempty"`
	Proposals          []string                       `json:"proposals,omitempty"`
	CapabilityRequests []types.CapabilityRequest      `json:"capability_requests,omitempty"`
	Notes              []string                       `json:"notes,omitempty"`
}

func newUpdateCoagentTool(rt *Runtime) Tool {
	return Tool{
		Name:        "update_coagent",
		Description: "Persist one structured non-canonical coagent update and wake the addressed owning agent. Use this for research findings, execution results, verification results, artifacts, blockers, directives, assignments, questions, proposals, and typed capability_requests. A capability request is a signal to the owner/supervisor, not automatic routing.",
		Parameters: jsonSchemaObject(map[string]any{
			"update_id":    map[string]any{"type": "string"},
			"kind":         map[string]any{"type": "string", "enum": []string{"findings", "evidence", "capability_request", "blocker", "proposal", "status", "verification", "artifact", "question", "directive", "assignment"}},
			"summary":      map[string]any{"type": "string"},
			"agent_id":     map[string]any{"type": "string"},
			"channel_id":   map[string]any{"type": "string"},
			"findings":     stringArraySchema(),
			"evidence_ids": stringArraySchema(),
			"evidence": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"kind":       map[string]any{"type": "string"},
						"source_uri": map[string]any{"type": "string"},
						"title":      map[string]any{"type": "string"},
						"content":    map[string]any{"type": "string"},
						"metadata":   map[string]any{"type": "object"},
					},
					"required":             []string{"kind", "content"},
					"additionalProperties": false,
				},
			},
			"artifacts": stringArraySchema(),
			"refs":      stringArraySchema(),
			"tests":     stringArraySchema(),
			"questions": stringArraySchema(),
			"proposals": stringArraySchema(),
			"capability_requests": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"capability":           map[string]any{"type": "string"},
						"requested_role":       map[string]any{"type": "string"},
						"objective":            map[string]any{"type": "string"},
						"why_needed":           map[string]any{"type": "string"},
						"blocking":             map[string]any{"type": "boolean"},
						"evidence_needed_for":  map[string]any{"type": "string"},
						"suggested_next_owner": map[string]any{"type": "string"},
					},
					"required":             []string{"capability", "objective"},
					"additionalProperties": false,
				},
			},
			"notes": stringArraySchema(),
		}, []string{"update_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in submitCoagentUpdateArgs
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode update_coagent args: %w", err)
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
				return "", fmt.Errorf("update_coagent missing coagent context")
			}

			evidenceIDs := trimNonEmpty(in.EvidenceIDs)
			for idx, item := range in.Evidence {
				rec, err := ensureFindingEvidence(ctx, rt.store, ownerID, agentID, updateID, idx, item)
				if err != nil {
					return "", err
				}
				evidenceIDs = append(evidenceIDs, rec.EvidenceID)
			}

			update := types.WorkerUpdateRecord{
				UpdateID:           updateID,
				OwnerID:            ownerID,
				AgentID:            agentID,
				Role:               nonEmpty(role, configuredAgentProfileForRun(ctxRunRecord(ctx))),
				Kind:               strings.TrimSpace(in.Kind),
				Summary:            strings.TrimSpace(in.Summary),
				Findings:           trimNonEmpty(in.Findings),
				EvidenceIDs:        evidenceIDs,
				Artifacts:          trimNonEmpty(in.Artifacts),
				Refs:               trimNonEmpty(in.Refs),
				Tests:              trimNonEmpty(in.Tests),
				Questions:          trimNonEmpty(in.Questions),
				Proposals:          trimNonEmpty(in.Proposals),
				CapabilityRequests: normalizeCapabilityRequests(in.CapabilityRequests),
				Notes:              trimNonEmpty(in.Notes),
				CreatedAt:          time.Now().UTC(),
			}
			if workerUpdateEmpty(update) {
				return "", fmt.Errorf("update_coagent requires summary, findings, evidence, evidence_ids, artifacts, refs, tests, questions, proposals, capability_requests, or notes")
			}

			targetAgentID, targetChannelID, err := resolveFindingsTarget(ctx, rt, strings.TrimSpace(in.AgentID))
			if err != nil {
				return "", err
			}
			if target, err := rt.store.GetAgent(ctx, targetAgentID); err == nil {
				targetProfile := canonicalAgentProfile(target.Profile)
				if targetProfile == AgentProfileEmail {
					return "", fmt.Errorf("%s cannot send arbitrary coagent updates to Email appagent %s; use a VText-owned request_email_draft artifact handoff", canonicalAgentProfile(stringFromToolContext(ctx, toolCtxProfile)), target.AgentID)
				}
				if err := enforceCoagentUpdateAuthority(ctx, rt, target, targetProfile); err != nil {
					return "", err
				}
			}
			channelID := authoritativeDeliveryChannelID(targetChannelID, in.ChannelID, stringFromToolContext(ctx, toolCtxChannelID))
			if channelID == "" {
				return "", fmt.Errorf("update_coagent could not resolve channel_id")
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
			stored, created, err := rt.store.DispatchWorkerUpdate(ctx, update, message)
			if err != nil {
				return "", err
			}
			if !created {
				if err := validateExistingWorkerUpdate(stored, update); err != nil {
					return "", err
				}
			} else {
				rt.emitChannelMessageEvent(ctx, *message, ownerID)
				rt.wakeUpdatedCoagent(ctx, stored)
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

func enforceCoagentUpdateAuthority(ctx context.Context, rt *Runtime, target types.AgentRecord, targetProfile string) error {
	if rt == nil || rt.store == nil {
		return nil
	}
	if canonicalAgentProfile(stringFromToolContext(ctx, toolCtxProfile)) != AgentProfileSuper || targetProfile != AgentProfileCoSuper {
		return nil
	}
	ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
	if ownerID == "" {
		return nil
	}
	run, err := rt.store.GetLatestActiveRunByAgent(ctx, ownerID, target.AgentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("lookup co-super active run: %w", err)
	}
	parentRunID := strings.TrimSpace(run.ParentRunID)
	if parentRunID == "" {
		return nil
	}
	parent, err := rt.store.GetRun(ctx, parentRunID)
	if err != nil {
		return fmt.Errorf("lookup co-super parent run: %w", err)
	}
	if canonicalAgentProfile(agentProfileForRun(&parent)) != AgentProfileVSuper {
		return nil
	}
	return fmt.Errorf("skip-level directive blocked: super must address co-super %s through owning vsuper %s with update_coagent", target.AgentID, parent.AgentID)
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
	return strings.TrimSpace(update.Summary) == "" &&
		len(update.Findings) == 0 &&
		len(update.EvidenceIDs) == 0 &&
		len(update.Artifacts) == 0 &&
		len(update.Refs) == 0 &&
		len(update.Tests) == 0 &&
		len(update.Questions) == 0 &&
		len(update.Proposals) == 0 &&
		len(update.CapabilityRequests) == 0 &&
		len(update.Notes) == 0
}

func buildWorkerUpdateMessage(update types.WorkerUpdateRecord) string {
	var b strings.Builder
	b.WriteString("Coagent update ready.")
	if strings.TrimSpace(update.Role) != "" {
		b.WriteString("\nRole: ")
		b.WriteString(strings.TrimSpace(update.Role))
		b.WriteString(".")
	}
	if strings.TrimSpace(update.Kind) != "" {
		b.WriteString("\nKind: ")
		b.WriteString(strings.TrimSpace(update.Kind))
		b.WriteString(".")
	}
	if strings.TrimSpace(update.Summary) != "" {
		b.WriteString("\nSummary: ")
		b.WriteString(strings.TrimSpace(update.Summary))
	}
	appendWorkerUpdateSection(&b, "Findings", update.Findings)
	appendWorkerUpdateSection(&b, "Evidence", update.EvidenceIDs)
	appendWorkerUpdateSection(&b, "Artifacts", update.Artifacts)
	appendWorkerUpdateSection(&b, "Refs", update.Refs)
	appendWorkerUpdateSection(&b, "Tests", update.Tests)
	appendWorkerUpdateSection(&b, "Questions", update.Questions)
	appendWorkerUpdateSection(&b, "Proposals", update.Proposals)
	appendCapabilityRequestSection(&b, update.CapabilityRequests)
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

func appendCapabilityRequestSection(b *strings.Builder, requests []types.CapabilityRequest) {
	if len(requests) == 0 {
		return
	}
	b.WriteString("\n\nCapability requests:\n")
	for _, request := range requests {
		b.WriteString("- capability=")
		b.WriteString(request.Capability)
		if request.RequestedRole != "" {
			b.WriteString(" requested_role=")
			b.WriteString(request.RequestedRole)
		}
		if request.Blocking {
			b.WriteString(" blocking=true")
		}
		if request.EvidenceNeededFor != "" {
			b.WriteString(" evidence_needed_for=")
			b.WriteString(request.EvidenceNeededFor)
		}
		if request.SuggestedNextOwner != "" {
			b.WriteString(" suggested_next_owner=")
			b.WriteString(request.SuggestedNextOwner)
		}
		b.WriteString("\n  objective: ")
		b.WriteString(request.Objective)
		if request.WhyNeeded != "" {
			b.WriteString("\n  why_needed: ")
			b.WriteString(request.WhyNeeded)
		}
		b.WriteString("\n")
	}
}

func validateExistingWorkerUpdate(existing, want types.WorkerUpdateRecord) error {
	if existing.AgentID != want.AgentID ||
		existing.TargetAgentID != want.TargetAgentID ||
		existing.ChannelID != want.ChannelID ||
		existing.Role != want.Role ||
		existing.Kind != want.Kind ||
		existing.Summary != want.Summary ||
		existing.Content != want.Content ||
		!stringSlicesEqual(existing.Findings, want.Findings) ||
		!stringSlicesEqual(existing.EvidenceIDs, want.EvidenceIDs) ||
		!stringSlicesEqual(existing.Artifacts, want.Artifacts) ||
		!stringSlicesEqual(existing.Refs, want.Refs) ||
		!stringSlicesEqual(existing.Tests, want.Tests) ||
		!stringSlicesEqual(existing.Questions, want.Questions) ||
		!stringSlicesEqual(existing.Proposals, want.Proposals) ||
		!capabilityRequestsEqual(existing.CapabilityRequests, want.CapabilityRequests) ||
		!stringSlicesEqual(existing.Notes, want.Notes) {
		return fmt.Errorf("update_id %s already exists with different payload", want.UpdateID)
	}
	return nil
}

func normalizeCapabilityRequests(requests []types.CapabilityRequest) []types.CapabilityRequest {
	out := make([]types.CapabilityRequest, 0, len(requests))
	for _, request := range requests {
		normalized := types.CapabilityRequest{
			Capability:         strings.TrimSpace(request.Capability),
			RequestedRole:      strings.TrimSpace(request.RequestedRole),
			Objective:          strings.TrimSpace(request.Objective),
			WhyNeeded:          strings.TrimSpace(request.WhyNeeded),
			Blocking:           request.Blocking,
			EvidenceNeededFor:  strings.TrimSpace(request.EvidenceNeededFor),
			SuggestedNextOwner: strings.TrimSpace(request.SuggestedNextOwner),
		}
		if normalized.Capability == "" && normalized.Objective == "" {
			continue
		}
		out = append(out, normalized)
	}
	return out
}

func capabilityRequestsEqual(a, b []types.CapabilityRequest) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
