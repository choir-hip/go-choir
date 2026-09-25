package textureowner

import (
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)


func TestRecordTextureDecisionToolDescriptionKeepsDecisionsOffDocument(t *testing.T) {
	tool := newRecordTextureDecisionTool(&Handler{})
	if !strings.Contains(tool.Description, "outside the canonical document") ||
		!strings.Contains(tool.Description, "owner explicitly asks Texture to record an off-document decision note") ||
		!strings.Contains(tool.Description, "Do not use it for ordinary sentence-level edits") ||
		!strings.Contains(tool.Description, "do not put agent process rationale into document text") {
		t.Fatalf("record_texture_decision description is too weak: %q", tool.Description)
	}
	if _, ok := tool.Parameters["properties"].(map[string]any)["decision_kind"]; !ok {
		t.Fatalf("record_texture_decision schema missing decision_kind: %#v", tool.Parameters)
	}
}




func TestTextureContentItemSourceEntityDefaultsToWholeResource(t *testing.T) {
	// After the D3 cutover, content-item source entities are whole_resource by
	// default; text_quote selectors (and their quote-match validation) come from
	// typed researcher findings, never from regex-scraping prose context.
	entity := contentItemRefToSourceEntity(types.ContentItem{
		ContentID:    "content-cloud-audit",
		Title:        "Cloud auditability source",
		SourceURL:    "https://example.com/cloud-audit",
		CanonicalURL: "https://example.com/cloud-audit",
		ContentHash:  "sha256-cloud-audit",
	})
	if entity.Kind != "content_item" ||
		entity.Target.TargetKind != "content_item" ||
		entity.Target.ContentID != "content-cloud-audit" ||
		entity.Display.OpenSurface != "source" ||
		entity.Evidence.ResearchState != "represented" ||
		len(entity.Selectors) != 1 ||
		entity.Selectors[0].SelectorKind != "whole_resource" ||
		entity.Selectors[0].ContentHash != "sha256-cloud-audit" ||
		entity.Selectors[0].TextQuote != "" {
		t.Fatalf("derived content item source entity = %#v", entity)
	}
}


func TestInitialTextureToolChoiceOnlyConstrainsMechanicalContinuations(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		want     string
	}{
		{
			name: "current factual work starts unconstrained",
			metadata: map[string]any{
				"type":            "texture_agent_revision",
				"original_prompt": "what is the weather in boston now",
			},
			want: "",
		},
		{
			name: "mutable product work starts unconstrained",
			metadata: map[string]any{
				"type":            "texture_agent_revision",
				"original_prompt": "debug and fix the runtime gateway",
			},
			want: "",
		},
		{
			name: "community wire operational proof starts unconstrained",
			metadata: map[string]any{
				"type":        "texture_agent_revision",
				"seed_prompt": "Universal Wire staging proof request: run the existing source-refresh/research/projection/publication flow, create or approve an Article Texture, update universal-wire/Wire.texture, then leave evidence ids and verifier proof.",
			},
			want: "",
		},
		{
			name: "creative direct document work starts unconstrained",
			metadata: map[string]any{
				"type":            "texture_agent_revision",
				"original_prompt": "tell me a story about computers",
			},
			want: "",
		},
		{
			name: "explicit decision note starts unconstrained",
			metadata: map[string]any{
				"type":            "texture_agent_revision",
				"original_prompt": "Create a short Texture document. Record an off-document Texture decision note with decision_kind no_worker_needed first.",
			},
			want: "",
		},
		{
			name: "direct user-authored revise requires durable action but not exact patch",
			metadata: map[string]any{
				"type":                "texture_agent_revision",
				"request_intent":      "revise",
				"current_author_kind": string(types.AuthorUser),
				"original_prompt":     "Research this and show visible work state while evidence is pending.",
			},
			want: "required",
		},
		{
			name: "scheduled non-coagent run leaves texture free to choose",
			metadata: map[string]any{
				"type":                  "texture_agent_revision",
				"original_prompt":       "research the sources and run one command",
				"scheduled_message_seq": int64(3),
			},
			want: "",
		},
		{
			name: "grounded integrate wake requires a durable action",
			metadata: map[string]any{
				"type":                  "texture_agent_revision",
				"original_prompt":       "integrate the researcher findings",
				"scheduled_message_seq": int64(3),
				"request_source":        "update_coagent",
			},
			want: "required",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := initialTextureToolChoice(&types.RunRecord{
				Metadata: tc.metadata,
			})
			if got != tc.want {
				t.Fatalf("initialTextureToolChoice = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTextureActorToolLoopBudgetDefaultsAndOverrides(t *testing.T) {
	rec := &types.RunRecord{
		ChannelID: "doc-channel",
		Metadata: map[string]any{
			"type":   "texture_agent_revision",
			"doc_id": "doc-budget",
		},
	}
	budget := textureActorToolLoopBudget(rec)
	if budget.Label != "texture:doc-budget" {
		t.Fatalf("label = %q, want texture:doc-budget", budget.Label)
	}
	if budget.MaxProviderCalls != defaultTextureActorMaxProviderCalls {
		t.Fatalf("max provider calls = %d, want default %d", budget.MaxProviderCalls, defaultTextureActorMaxProviderCalls)
	}
	if budget.MaxTotalTokens != defaultTextureActorMaxTotalTokens {
		t.Fatalf("max total tokens = %d, want default %d", budget.MaxTotalTokens, defaultTextureActorMaxTotalTokens)
	}
	if budget.MaxElapsed != defaultTextureActorMaxElapsed {
		t.Fatalf("max elapsed = %s, want %s", budget.MaxElapsed, defaultTextureActorMaxElapsed)
	}

	rec.Metadata["actor_budget_max_provider_calls"] = int64(7)
	rec.Metadata["actor_budget_max_input_tokens"] = int64(1000)
	rec.Metadata["actor_budget_max_output_tokens"] = int64(2000)
	rec.Metadata["actor_budget_max_total_tokens"] = int64(3000)
	rec.Metadata["actor_budget_max_elapsed_seconds"] = int64(90)
	rec.Metadata["actor_budget_spent_provider_calls"] = int64(2)
	rec.Metadata["actor_budget_spent_input_tokens"] = int64(100)
	rec.Metadata["actor_budget_spent_output_tokens"] = int64(200)
	budget = textureActorToolLoopBudget(rec)
	if budget.MaxProviderCalls != 7 ||
		budget.MaxInputTokens != 1000 ||
		budget.MaxOutputTokens != 2000 ||
		budget.MaxTotalTokens != 3000 ||
		budget.MaxElapsed != 90*time.Second ||
		budget.SpentProviderCalls != 2 ||
		budget.SpentInputTokens != 100 ||
		budget.SpentOutputTokens != 200 {
		t.Fatalf("override budget = %+v", budget)
	}
}

func TestExplicitNoWorkerDecisionParsesWithoutNarrativeRouteOracle(t *testing.T) {
	prompt := strings.Join([]string{
		"Create a short Texture document for a deployed staging proof.",
		"Because this task is fully supplied and requires no research or execution worker,",
		"record an off-document Texture decision note with decision_kind no_worker_needed.",
		"Then write the concise reader-facing Texture revision.",
	}, " ")
	if !texturePromptExplicitlyRequestsDecisionNote(prompt) {
		t.Fatal("test prompt should explicitly request a decision note")
	}
	if !texturePromptExplicitlyRequestsNoWorkerDecision(prompt) {
		t.Fatal("no-worker decision note prompt should parse as an explicit decision request")
	}

	if texturePromptExplicitlyRequestsNoWorkerDecision("Debug and fix the runtime gateway, run tests, and verify the staging proof.") {
		t.Fatal("ordinary mutation prompt must not parse as a no-worker decision")
	}
}

func TestExplicitNoWorkerDecisionPromptParsesInitialDecision(t *testing.T) {
	prompt := strings.Join([]string{
		"Create a short Texture document titled M32_TEXTURE_DECISION_ROUTE_TEST.",
		"Because this task is fully supplied and requires no research or execution worker,",
		"record an off-document Texture decision note with decision_kind no_worker_needed,",
		"exact reason M3.2 staging proof: user supplied the needed content and requested no research or execution worker.,",
		"evidence ref staging-marker:M32_TEXTURE_DECISION_ROUTE_TEST,",
		"next action Write the concise reader-facing Texture revision.",
		"Then write the concise reader-facing Texture revision.",
	}, " ")
	decision, ok := explicitNoWorkerDecisionRequestFromPrompt(prompt)
	if !ok {
		t.Fatal("proof-style prompt should parse as an explicit initial decision")
	}
	if decision.DecisionKind != "no_worker_needed" {
		t.Fatalf("decision kind = %q", decision.DecisionKind)
	}
	if decision.Reason != "M3.2 staging proof: user supplied the needed content and requested no research or execution worker." {
		t.Fatalf("reason = %q", decision.Reason)
	}
	if len(decision.EvidenceRefs) != 1 || decision.EvidenceRefs[0] != "staging-marker:M32_TEXTURE_DECISION_ROUTE_TEST" {
		t.Fatalf("evidence refs = %#v", decision.EvidenceRefs)
	}
	if decision.NextAction != "Write the concise reader-facing Texture revision" {
		t.Fatalf("next action = %q", decision.NextAction)
	}
}
