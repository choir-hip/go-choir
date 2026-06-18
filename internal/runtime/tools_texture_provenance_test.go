package runtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestBuildAppagentRevisionProvenanceSystemAttributed(t *testing.T) {
	now := time.Date(2026, 6, 18, 15, 0, 0, 0, time.UTC)
	revMeta, err := json.Marshal(map[string]any{
		"source": "patch_texture",
		"source_entities": []types.SourceEntity{
			{EntityID: "src_aaaa", Kind: "content_item", Target: types.SourceEntityTarget{TargetKind: "content_item", ContentID: "ci-1"}},
			{EntityID: "src_bbbb", Kind: "youtube_video", Target: types.SourceEntityTarget{TargetKind: "content_item", ContentID: "ci-2"}},
		},
	})
	if err != nil {
		t.Fatalf("marshal revMeta: %v", err)
	}
	rec := &types.RunRecord{Metadata: map[string]any{"model": "test-model", "provider": "fireworks"}}

	raw := buildAppagentRevisionProvenance(rec, revMeta, now)
	if len(raw) == 0 {
		t.Fatalf("expected non-empty provenance")
	}

	var prov types.Provenance
	if err := json.Unmarshal(raw, &prov); err != nil {
		t.Fatalf("unmarshal provenance: %v", err)
	}
	if prov.SchemaVersion != types.ProvenanceSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", prov.SchemaVersion, types.ProvenanceSchemaVersion)
	}
	if prov.AuthoringModel.Model != "test-model" || prov.AuthoringModel.Provider != "fireworks" {
		t.Errorf("AuthoringModel = %+v, want fireworks/test-model", prov.AuthoringModel)
	}
	if !prov.AuthoredAt.Equal(now) {
		t.Errorf("AuthoredAt = %v, want %v", prov.AuthoredAt, now)
	}
	if len(prov.Sources) != 2 {
		t.Fatalf("Sources len = %d, want 2", len(prov.Sources))
	}
	// Canonical output sorts sources by EntityID.
	if prov.Sources[0].EntityID != "src_aaaa" || prov.Sources[1].EntityID != "src_bbbb" {
		t.Errorf("source order = %q,%q want src_aaaa,src_bbbb", prov.Sources[0].EntityID, prov.Sources[1].EntityID)
	}
}

func TestBuildAppagentRevisionProvenanceNoSources(t *testing.T) {
	now := time.Date(2026, 6, 18, 15, 0, 0, 0, time.UTC)
	revMeta := json.RawMessage(`{"source":"patch_texture"}`)
	raw := buildAppagentRevisionProvenance(&types.RunRecord{}, revMeta, now)
	if len(raw) == 0 {
		t.Fatalf("expected non-empty provenance even with no sources")
	}
	var prov types.Provenance
	if err := json.Unmarshal(raw, &prov); err != nil {
		t.Fatalf("unmarshal provenance: %v", err)
	}
	if prov.SchemaVersion != types.ProvenanceSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", prov.SchemaVersion, types.ProvenanceSchemaVersion)
	}
	if len(prov.Sources) != 0 {
		t.Errorf("expected no sources, got %d", len(prov.Sources))
	}
}
