package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

func RegisterWireProcessorTools(registry *toolregistry.ToolRegistry, rt *Runtime) error {
	return registry.Register(newRecordWireProcessorDecisionTool(rt))
}

type recordWireProcessorDecisionArgs struct {
	Decision       string   `json:"decision"`
	Summary        string   `json:"summary"`
	SourceItemIDs  []string `json:"source_item_ids,omitempty"`
	CoveredByDocID string   `json:"covered_by_doc_id,omitempty"`
}

func newRecordWireProcessorDecisionTool(rt *Runtime) Tool {
	return Tool{
		Name:        "record_wire_processor_decision",
		Description: "Record a typed non-publication decision for the current Universal Wire processor request. Use this when no Texture story should open, so the request does not disappear behind terminal run state or a prose-only checkpoint.",
		Parameters: jsonSchemaObject(map[string]any{
			"decision": map[string]any{
				"type": "string",
				"enum": []string{
					string(wireProcessorDecisionAlreadyCovered),
					string(wireProcessorDecisionNotNewsworthy),
					string(wireProcessorDecisionInsufficientSignal),
					string(wireProcessorDecisionDeferred),
				},
			},
			"summary": map[string]any{
				"type":        "string",
				"description": "Short owner-readable reason for the explicit non-publication decision.",
			},
			"source_item_ids": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "The exact source item ids this decision covers. Required when the processor request contains multiple source items.",
			},
			"covered_by_doc_id": map[string]any{
				"type":        "string",
				"description": "For decision=already_covered: the published Texture document id that already covers these source items.",
			},
		}, []string{"decision", "summary"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in recordWireProcessorDecisionArgs
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode record_wire_processor_decision args: %w", err)
			}
			runRec := toolregistry.ExecutionContextFrom(ctx).RunRecord
			if runRec == nil {
				return "", fmt.Errorf("record_wire_processor_decision missing run context")
			}
			if agentprofile.Canonical(agentProfileForRun(runRec)) != agentprofile.Processor {
				return "", fmt.Errorf("record_wire_processor_decision requires a processor run")
			}
			decision := wireProcessorDecisionVerdict(strings.TrimSpace(in.Decision))
			switch decision {
			case wireProcessorDecisionAlreadyCovered, wireProcessorDecisionNotNewsworthy, wireProcessorDecisionInsufficientSignal, wireProcessorDecisionDeferred:
			default:
				return "", fmt.Errorf("decision must be one of already_covered, not_newsworthy, insufficient_evidence, deferred")
			}
			summary := strings.TrimSpace(in.Summary)
			if summary == "" {
				return "", fmt.Errorf("summary must not be empty")
			}
			sourceItemIDs, err := resolveWireProcessorSourceItemIDs(runRec, trimNonEmpty(in.SourceItemIDs), true)
			if err != nil {
				return "", err
			}
			item, err := rt.recordWireProcessorDecision(ctx, runRec, wireProcessorDecisionUpdate{
				Verdict:        decision,
				Summary:        summary,
				SourceItemIDs:  sourceItemIDs,
				CoveredByDocID: strings.TrimSpace(in.CoveredByDocID),
				Complete:       false,
			})
			if err != nil {
				return "", err
			}
			return toolResultJSON(map[string]any{
				"trajectory_id":     runRec.TrajectoryID,
				"work_item_id":      item.WorkItemID,
				"decision":          decision,
				"source_item_ids":   sourceItemIDs,
				"covered_by_doc_id": strings.TrimSpace(in.CoveredByDocID),
				"status":            item.Status,
			})
		},
	}
}
