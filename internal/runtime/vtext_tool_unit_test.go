package runtime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

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
