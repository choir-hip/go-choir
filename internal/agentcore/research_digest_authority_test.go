package agentcore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The research desk is not the custodian of Texture's documents (objectgraph reader gate
// X1). The gate ENFORCES as the product ships it (this package's TestMain runs fixtures in
// observe, so these tests set enforce themselves): a research read of a story's document
// or revision is refused. So no research-context path may read the document, and the
// story's text reaches research inside the Texture's own Cast
// (store.attachStoryToResearchCasts).
func textureCrossings() int {
	n := 0
	for k, v := range objectgraph.ReaderGateCounts() {
		if strings.Contains(k, "choir.texture_") && strings.Contains(k, "|desk:research|") {
			n += int(v)
		}
	}
	return n
}

func TestInEnforceTheDigestKeepsItsStoryAndReadsNoTextureDocument(t *testing.T) {
	rt, _, owner, computer, doc, _ := digestFixture(t)
	objectgraph.ResetReaderGateCounts()
	d := buildDigest(t, rt, briefRunRecord(owner, computer, doc, "run-auth"))
	if d == nil || !strings.Contains(d.text, "Dockworkers struck on Monday") || !strings.Contains(d.text, "Main point: Whether the dock strike ends") {
		t.Fatalf("the digest lost the story's text or main point under enforce:\n%v", d)
	}
	for _, deg := range d.degraded {
		if deg == "story" || deg == "texture_text" || deg == "conversation" {
			t.Fatalf("a story section degraded under enforce: %v", d.degraded)
		}
	}
	if n := textureCrossings(); n != 0 {
		t.Fatalf("the digest read a Texture document or revision as the research desk (%d crossings): %v", n, objectgraph.ReaderGateCounts())
	}
}

// The deployed story-research context (main-point landing) took the story's main point
// with StoryPointOf under the research reader. Under enforce that read is refused and the
// main point is EMPTY: this test is the evidence for the problem record
// docs/problems/story-research-context-main-point-empty-under-enforce-2026-11-22.md.
func TestUnderEnforceAResearchReaderGetsNoMainPointAndNoDocument(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-enforce", rt.cfg.ComputerID
	t0 := time.Now().UTC().Add(-time.Hour)
	doc := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Dock strike", "Whether the dock strike ends and on what terms", "", []string{"c1"}, t0)

	custodian := objectgraph.WithCustodianReader(context.Background(), "choir.texture_revision")
	if point, _ := s.StoryPointOf(custodian, owner, computer, doc); point == "" {
		t.Fatal("the custodian reads no main point: the control is broken")
	}
	defer objectgraph.SetModeForTest(objectgraph.WriterGateEnforce)()
	research := withDeskReader(context.Background(), "research")
	if _, err := s.GetLifecycleDocument(research, owner, computer, doc); err == nil {
		t.Error("the research desk read a story's document under enforce")
	}
	if point, _ := s.StoryPointOf(research, owner, computer, doc); point != "" {
		t.Errorf("StoryPointOf under the research reader returned %q under enforce", point)
	} else {
		t.Log("StoryPointOf under the research reader returns \"\" under enforce: the deployed story context loses its main point")
	}
}

func TestADigestWithoutTheStoryInTheCastHoldsNoStoryText(t *testing.T) {
	rt, s, owner, computer, _, _ := digestFixture(t)
	// A story whose research Cast carries no account of the story (a Cast written before
	// the field, or by another path): the digest has no story section, and it does not go
	// to the document for one.
	t0 := time.Now().UTC().Add(-time.Hour)
	other := openStoryOf(t, s, owner, computer, "cell-8:cast:o1", "Unrelated vote", "Who wins the unrelated vote and by how much", "", nil, t0)
	researchCastAt(t, s, owner, computer, other, "cell-9:cast:r9", t0.Add(time.Minute))
	objectgraph.ResetReaderGateCounts()
	d := buildDigest(t, rt, briefRunRecord(owner, computer, other, "run-nostory"))
	if d != nil && (strings.Contains(d.text, "[This story]") || strings.Contains(d.text, "Unrelated vote")) {
		t.Fatalf("a story the Cast does not carry was read from the document:\n%s", d.text)
	}
	if n := textureCrossings(); n != 0 {
		t.Fatalf("the digest read a Texture document (%d crossings)", n)
	}
}
