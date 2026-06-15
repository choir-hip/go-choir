package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

type semanticMergeTestProvider struct {
	response string
	req      ToolLoopRequest
	calls    int
}

func (p *semanticMergeTestProvider) ProviderName() string { return "semantic-merge-test" }

func (p *semanticMergeTestProvider) Execute(ctx context.Context, task *types.RunRecord, emit EventEmitFunc) error {
	task.Result = p.response
	return nil
}

func (p *semanticMergeTestProvider) CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error) {
	p.calls++
	p.req = req
	return &ToolLoopResponse{
		ID:         "model-response-1",
		StopReason: "end_turn",
		Text:       p.response,
		Usage:      TokenUsage{InputTokens: 321, OutputTokens: 123},
		Model:      "semantic-test-model",
	}, nil
}

func TestCleanVTextToolContentRemovesWrapperTags(t *testing.T) {
	input := " <payload>\nStaging smoke after RSS title extraction works.\n</payload> "
	if got := cleanVTextToolContent(input); got != "Staging smoke after RSS title extraction works." {
		t.Fatalf("cleanVTextToolContent() = %q", got)
	}
}

func TestCleanVTextToolContentRemovesDanglingClosingMarker(t *testing.T) {
	for _, input := range []string{
		"VText wrapper cleanup works.</\n",
		"VText wrapper cleanup works.</妮>",
	} {
		if got := cleanVTextToolContent(input); got != "VText wrapper cleanup works." {
			t.Fatalf("cleanVTextToolContent(%q) = %q", input, got)
		}
	}
}

func TestCleanVTextToolContentPreservesOrdinaryText(t *testing.T) {
	input := "The paragraph mentions <payload> as literal text inside the body."
	if got := cleanVTextToolContent(input); got != input {
		t.Fatalf("cleanVTextToolContent() = %q, want original", got)
	}
}

func TestMaterializeVTextToolEditRequiresRationaleForLongRewrite(t *testing.T) {
	current := types.Revision{
		RevisionID: "rev-long",
		Content:    strings.Repeat("long section\n", 1300),
	}
	_, err := materializeVTextToolEdit(editVTextArgs{
		BaseRevisionID: "rev-long",
		Operation:      "replace_all",
		Content:        "short replacement",
	}, current)
	if err == nil || !strings.Contains(err.Error(), "requires rationale") {
		t.Fatalf("replace_all long doc err = %v, want rationale guard", err)
	}

	got, err := materializeVTextToolEdit(editVTextArgs{
		BaseRevisionID: "rev-long",
		Operation:      "replace_all",
		Content:        "short replacement",
		Rationale:      "Owner explicitly requested a full summary rewrite.",
	}, current)
	if err != nil {
		t.Fatalf("replace_all with rationale: %v", err)
	}
	if got.Operation != "replace_all" || got.Rationale == "" || got.EditCount != 1 {
		t.Fatalf("materialized rewrite metadata = %+v", got)
	}
}

func TestVTextEditRevisionMetadataRecordsOperationEvidence(t *testing.T) {
	now := time.Now().UTC()
	raw := addVTextEditRevisionMetadata(json.RawMessage(`{"existing":"kept"}`), materializedVTextEdit{
		Operation:      "apply_edits",
		SourceTool:     "patch_texture",
		BaseRevisionID: "rev-1",
		EditCount:      2,
		BaseChars:      100,
		ResultChars:    124,
		DeltaChars:     24,
	}, &types.RunRecord{
		RunID:     "run-1",
		Prompt:    "revise paragraph",
		CreatedAt: now.Add(-1500 * time.Millisecond),
		UpdatedAt: now,
	})
	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("metadata json: %v", err)
	}
	if meta["existing"] != "kept" {
		t.Fatalf("existing metadata not preserved: %+v", meta)
	}
	if meta["source"] != "patch_texture" || meta["texture_edit_tool"] != "patch_texture" {
		t.Fatalf("texture write source metadata missing: %+v", meta)
	}
	if meta["vtext_edit_operation"] != "apply_edits" || int(meta["vtext_edit_count"].(float64)) != 2 {
		t.Fatalf("edit operation metadata missing: %+v", meta)
	}
	if int(meta["vtext_run_prompt_chars"].(float64)) != len("revise paragraph") {
		t.Fatalf("prompt chars metadata missing: %+v", meta)
	}
	if int(meta["vtext_edit_delta_chars"].(float64)) != 24 {
		t.Fatalf("delta metadata missing: %+v", meta)
	}
}

func TestVTextSemanticMergeUsesProviderBackedJSON(t *testing.T) {
	provider := &semanticMergeTestProvider{response: `{
		"summary": ["Older version has a stronger client-control framing."],
		"suggestions": [{
			"id": "client_control_frame",
			"label": "Restore client-control framing",
			"description": "Bring the sharper control argument into the Primary draft while keeping current evidence.",
			"status": "Clean merge",
			"source": "rev-source",
			"preview": "client control"
		}]
	}`}
	rt := New(Config{}, nil, nil, provider)
	source := types.Revision{RevisionID: "rev-source", Content: "# Proposal\n\nClients control the system."}
	target := types.Revision{RevisionID: "rev-target", Content: "# Proposal\n\nThe system has current evidence."}

	result, evidence, err := rt.callVTextSemanticMergeModel(context.Background(), "owner-1", source, target, types.DiffResult{AddedLines: 1, RemovedLines: 1}, "compare", nil, "v4", "v5")
	if err != nil {
		t.Fatalf("model semantic compare: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	if provider.req.ToolChoice != "none" {
		t.Fatalf("tool choice = %q, want none", provider.req.ToolChoice)
	}
	if provider.req.Model == "" || provider.req.Provider == "" {
		t.Fatalf("provider/model not resolved in request: %+v", provider.req)
	}
	if got := result.Suggestions[0].Label; got != "Restore client-control framing" {
		t.Fatalf("suggestion label = %q", got)
	}
	if strings.Contains(strings.ToLower(result.Suggestions[0].ID+" "+result.Suggestions[0].Label), "restore_glossary") {
		t.Fatalf("semantic merge returned old hard-coded suggestion: %+v", result.Suggestions[0])
	}
	if evidence["model_input_tokens"] != 321 || evidence["model_output_tokens"] != 123 {
		t.Fatalf("token evidence missing: %+v", evidence)
	}
}

func TestVTextSemanticMergePromotesModelSummaryToSuggestion(t *testing.T) {
	result, err := normalizeModelSemanticMergeResult(vtextModelSemanticMergeResult{
		Summary: []string{"Earlier draft has a sharper ownership argument."},
	}, types.Revision{RevisionID: "rev-source"}, types.Revision{RevisionID: "rev-target"}, false)
	if err != nil {
		t.Fatalf("normalize summary-only model result: %v", err)
	}
	if len(result.Suggestions) != 1 {
		t.Fatalf("suggestions = %d, want 1", len(result.Suggestions))
	}
	if result.Suggestions[0].ID != "model_finding_1" || result.Suggestions[0].Description != result.Summary[0] {
		t.Fatalf("summary-derived suggestion mismatch: %+v", result.Suggestions[0])
	}
	if strings.Contains(strings.ToLower(result.Suggestions[0].Label), "glossary") {
		t.Fatalf("summary fallback reintroduced domain stub: %+v", result.Suggestions[0])
	}
}

func TestApplyVTextModelMergeEditsStripsVisibleProvenance(t *testing.T) {
	target := "# Proposal\n\nCurrent paragraph.\n\n<!-- VText merge preview provenance\n- leaked metadata\n-->\n"
	content, applied, err := applyVTextModelMergeEdits(target, []vtextModelMergeEdit{{
		SuggestionID: "client_control_frame",
		Operation:    "replace_exact",
		OldText:      "Current paragraph.",
		NewText:      "Current paragraph with restored client-control framing.",
		Rationale:    "Selected source concept improves framing.",
	}})
	if err != nil {
		t.Fatalf("apply edits: %v", err)
	}
	if strings.Contains(content, "VText merge preview provenance") || strings.Contains(content, "<!--") {
		t.Fatalf("visible provenance leaked into content: %q", content)
	}
	if !strings.Contains(content, "restored client-control framing") {
		t.Fatalf("model edit not applied: %q", content)
	}
	if len(applied) != 1 || applied[0]["operation"] != "replace_exact" {
		t.Fatalf("applied edit evidence mismatch: %+v", applied)
	}
}
