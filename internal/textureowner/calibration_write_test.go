package textureowner

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// A Write's pSupported reaches the revision metadata and, through the
// turn's transaction, the new citation's check memo with the authoring
// model (docs/archive/reports/icrl-calibration-experiment-2026-10-14.md).
func TestWriteSelfScoreReachesTheCitationCheck(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "selfscore")
	ctx := context.Background()
	if rec.Metadata == nil {
		rec.Metadata = map[string]any{}
	}
	rec.Metadata["provider"], rec.Metadata["model"] = "chatgpt", "gpt-6-luna"
	base := headOf(t, s, rec)
	p := 0.8
	body := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits:      "@replace b2\nThe court ruled that the merger may proceed {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}.",
		Why:        "state the ruling",
		PSupported: &p,
	}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, body, "selfscore-1"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	head := headOf(t, s, rec)
	if got, ok := decodeRevisionMetadata(head.Metadata)[store.RevisionMetaPSupported].(float64); !ok || got != 0.8 {
		t.Fatalf("revision metadata = %s", head.Metadata)
	}
	checks, err := s.ListCitationChecks(ctx, rec.OwnerID, rec.ComputerID, 50)
	if err != nil || len(checks) == 0 {
		t.Fatalf("checks = %d, %v", len(checks), err)
	}
	found := false
	for _, c := range checks {
		if c.Value.Pair.RevisionID == head.RevisionID {
			found = true
			if c.Value.SelfP == nil || *c.Value.SelfP != 0.8 || c.Value.ModelID != "chatgpt/gpt-6-luna" {
				t.Fatalf("check = %+v", c.Value)
			}
		}
	}
	if !found {
		t.Fatalf("no check memo for revision %s: %+v", head.RevisionID, checks)
	}
}
