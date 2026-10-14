package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// Corpus search finds the paper's own stories (L2a R2-8, defect 4 of
// docs/problems/autopaper-intake-defects-2026-10-14.md). Ways it fails,
// written before the code:
//   - the "published" filter tests revision metadata for route paths no
//     revision carries, so every story is filtered out (the defect);
//   - stories are lifecycle documents (they have a trajectory), and the
//     search lists only documents without one, so deleting the filter
//     alone still finds nothing;
//   - the head revision of a story is read through the non-lifecycle
//     getter, which refuses it, so content never matches;
//   - another owner's story is found.
func TestSearchWireCorpusFindsTheOwnersStories(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	owner, computer := "owner-corpus", rt.cfg.ComputerID
	harbor := openTestStory(t, s, owner, computer, "Harbor reopens after the storm")
	openTestStory(t, s, "owner-other", computer, "Harbor strike enters second week")
	deps := researchtools.Dependencies{Store: s}
	ectx := toolregistry.WithExecutionContext(ctx, toolregistry.ExecutionContext{RunID: "research-run", OwnerID: owner, ComputerID: computer, Profile: agentprofile.Research})
	raw, err := deps.HostEgress(ectx, "search_wire_corpus", json.RawMessage(`{"query":"harbor","owner_id":"`+owner+`"}`))
	if err != nil {
		t.Fatalf("search_wire_corpus: %v", err)
	}
	var out struct {
		Results []struct {
			DocID string `json:"doc_id"`
			Title string `json:"title"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Results) != 1 || out.Results[0].DocID != harbor {
		t.Fatalf("search results = %s; want the owner's story %s alone", raw, harbor)
	}
	if strings.Contains(string(raw), "strike") {
		t.Fatalf("search found another owner's story: %s", raw)
	}
}
